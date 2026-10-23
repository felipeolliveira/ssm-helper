// Package env reads, validates and writes the environment files (env.json and
// env.<name>.json), keeping the order in which profiles and targets are declared.
// Files in the old format (UPPER_CASE keys, from config.yaml/config.json) are
// also read, so they can be migrated.
package env

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
)

// ErrNotFound means the environment file does not exist at the expected path.
var ErrNotFound = errors.New("arquivo do ambiente não encontrado")

type Profile struct {
	Name      string `json:"name"`
	SSOURL    string `json:"ssoUrl"`
	SSORegion string `json:"ssoRegion"`
	AccountID string `json:"accountId"`
	RoleName  string `json:"roleName"`
}

type Target struct {
	Name          string   `json:"name"`
	Profile       string   `json:"profile"`
	SSMTarget     string   `json:"ssmTarget"`
	SSMRegion     string   `json:"ssmRegion"`
	SSMDocument   string   `json:"ssmDocument"`
	RemoteHost    string   `json:"remoteHost"`
	RemotePort    string   `json:"remotePort"`
	LocalPort     string   `json:"localPort"`
	DNSHosts      []string `json:"dnsHosts"`
	OpenURLs      []string `json:"openUrls"`
	ChromeProfile string   `json:"chromeProfile"`
}

// Env is one environment: the AWS profiles and the tunnel targets.
type Env struct {
	Path     string    `json:"path"`
	Title    string    `json:"title"`
	Profiles []Profile `json:"profiles"`
	Targets  []Target  `json:"targets"`
}

var (
	// NamePattern is the rule for profile, target and palette names.
	NamePattern    = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	linkPattern    = regexp.MustCompile(`^[A-Za-z0-9._~:/?#&=%+-]+$`)
	dnsPattern     = regexp.MustCompile(`^[A-Za-z0-9._~:/?#&=%+*-]+$`)
	dnsHostPattern = regexp.MustCompile(`^[A-Za-z0-9.*-]+$`)
	urlHostPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
)

// InvalidNameHint explains NamePattern in validation messages.
const InvalidNameHint = "nome inválido: use apenas letras, números, '.', '_' ou '-'"

// Field keys, with the old UPPER_CASE key each one replaced
var (
	profileFields = [][2]string{
		{"ssoUrl", "AWS_SSO_URL"},
		{"ssoRegion", "AWS_SSO_REGION"},
		{"accountId", "AWS_SSO_ACCOUNT_ID"},
		{"roleName", "AWS_SSO_ROLE_NAME"},
	}
	targetFields = map[string]string{
		"profile":       "PROFILE",
		"ssmTarget":     "SSM_TARGET",
		"ssmRegion":     "SSM_REGION",
		"ssmDocument":   "SSM_DOCUMENT",
		"remoteHost":    "SSM_REMOTE_HOST",
		"remotePort":    "SSM_REMOTE_PORT",
		"localPort":     "SSM_LOCAL_PORT",
		"dnsHosts":      "DNS_HOSTS",
		"openUrls":      "OPEN_URLS",
		"chromeProfile": "CHROME_PROFILE",
	}
	requiredTargetFields = []string{"profile", "ssmTarget", "ssmDocument", "remoteHost", "remotePort", "localPort"}
)

// Load reads an environment file. It returns ErrNotFound, a *jsonfile.FormatError,
// or the environment together with the list of validation problems (empty when valid).
func Load(path string) (*Env, []string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return Parse(path, data)
}

// Parse validates environment content (JSON; YAML is also accepted for migration); see Load.
func Parse(path string, data []byte) (*Env, []string, error) {
	root, err := jsonfile.Parse(data)
	if err != nil {
		return nil, nil, err
	}

	e := &Env{Path: path, Profiles: []Profile{}, Targets: []Target{}}
	var problems []string

	e.Title = jsonfile.Field(root, "title")
	if e.Title == "" {
		e.Title = jsonfile.Field(jsonfile.Object(root, "variables"), "MENU_TITLE")
	}
	if e.Title == "" {
		problems = append(problems, "title")
	}

	profileNames := map[string]bool{}
	if profiles := jsonfile.Object(root, "profiles"); profiles != nil {
		if len(profiles.Content) == 0 {
			problems = append(problems, "profiles (nenhum perfil definido)")
		}
		for i := 0; i+1 < len(profiles.Content); i += 2 {
			name, node := profiles.Content[i].Value, profiles.Content[i+1]
			profileNames[name] = true
			// Entries with problems are still returned, so the GUI can fix them
			e.Profiles = append(e.Profiles, Profile{
				Name:      name,
				SSOURL:    field(node, profileFields[0]),
				SSORegion: field(node, profileFields[1]),
				AccountID: field(node, profileFields[2]),
				RoleName:  field(node, profileFields[3]),
			})
			if !NamePattern.MatchString(name) {
				problems = append(problems, fmt.Sprintf("profiles.%s (%s)", name, InvalidNameHint))
				continue
			}
			for _, f := range profileFields {
				if field(node, f) == "" {
					problems = append(problems, fmt.Sprintf("profiles.%s.%s", name, f[0]))
				}
			}
		}
	} else {
		problems = append(problems, "profiles (seção ausente)")
	}

	if targets := jsonfile.Object(root, "targets"); targets != nil {
		if len(targets.Content) == 0 {
			problems = append(problems, "targets (nenhum target definido)")
		}
		for i := 0; i+1 < len(targets.Content); i += 2 {
			name, node := targets.Content[i].Value, targets.Content[i+1]
			tf := func(key string) string { return field(node, [2]string{key, targetFields[key]}) }
			t := Target{
				Name:          name,
				Profile:       tf("profile"),
				SSMTarget:     tf("ssmTarget"),
				SSMRegion:     tf("ssmRegion"),
				SSMDocument:   tf("ssmDocument"),
				RemoteHost:    tf("remoteHost"),
				RemotePort:    tf("remotePort"),
				LocalPort:     tf("localPort"),
				DNSHosts:      list(node, "dnsHosts", targetFields["dnsHosts"]),
				OpenURLs:      list(node, "openUrls", targetFields["openUrls"]),
				ChromeProfile: tf("chromeProfile"),
			}
			e.Targets = append(e.Targets, t)
			if !NamePattern.MatchString(name) {
				problems = append(problems, fmt.Sprintf("targets.%s (%s)", name, InvalidNameHint))
				continue
			}
			for _, f := range requiredTargetFields {
				if tf(f) == "" {
					problems = append(problems, fmt.Sprintf("targets.%s.%s", name, f))
				}
			}
			if t.Profile != "" && !profileNames[t.Profile] {
				problems = append(problems, fmt.Sprintf("targets.%s.profile (perfil '%s' não existe em profiles)", name, t.Profile))
			}
			for _, link := range t.DNSHosts {
				if !dnsPattern.MatchString(link) || !dnsHostPattern.MatchString(LinkToHost(link)) {
					problems = append(problems, fmt.Sprintf("targets.%s.dnsHosts (domínio inválido: %s)", name, link))
				}
			}
			for _, link := range t.OpenURLs {
				if !linkPattern.MatchString(link) || !urlHostPattern.MatchString(LinkToHost(link)) {
					problems = append(problems, fmt.Sprintf("targets.%s.openUrls (URL inválida: %s)", name, link))
				}
			}
			if t.ChromeProfile != "" && !NamePattern.MatchString(t.ChromeProfile) {
				problems = append(problems, fmt.Sprintf("targets.%s.chromeProfile (%s)", name, InvalidNameHint))
			}
		}
	} else {
		problems = append(problems, "targets (seção ausente)")
	}

	return e, problems, nil
}

// field reads a key, falling back to its old UPPER_CASE name.
func field(node *yaml.Node, keys [2]string) string {
	if v := jsonfile.Field(node, keys[0]); v != "" {
		return v
	}
	return jsonfile.Field(node, keys[1])
}

func list(node *yaml.Node, key, oldKey string) []string {
	if jsonfile.Lookup(node, key) != nil {
		return jsonfile.List(node, key)
	}
	return jsonfile.List(node, oldKey)
}

// Profile returns the profile with the given name.
func (e *Env) Profile(name string) (Profile, bool) {
	for _, p := range e.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// Target returns the target with the given name.
func (e *Env) Target(name string) (Target, bool) {
	for _, t := range e.Targets {
		if t.Name == name {
			return t, true
		}
	}
	return Target{}, false
}

// Region uses the target region, falling back to the profile's SSO region.
func (t Target) Region(p Profile) string {
	if t.SSMRegion != "" {
		return t.SSMRegion
	}
	return p.SSORegion
}

// ChromeProfileName uses chromeProfile, falling back to the target name.
func (t Target) ChromeProfileName() string {
	if t.ChromeProfile != "" {
		return t.ChromeProfile
	}
	return t.Name
}

// UsesBrowser reports whether the target has domains or URLs for Chrome.
func (t Target) UsesBrowser() bool {
	return len(t.DNSHosts) > 0 || len(t.OpenURLs) > 0
}

// LinkToURL adds https:// when the link has no scheme.
func LinkToURL(link string) string {
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	return "https://" + link
}

// LinkToHost strips scheme, path and port from a link.
func LinkToHost(link string) string {
	if _, rest, ok := strings.Cut(link, "://"); ok {
		link = rest
	}
	link, _, _ = strings.Cut(link, "/")
	link, _, _ = strings.Cut(link, ":")
	return link
}
