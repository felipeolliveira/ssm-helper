// Package env reads, validates and writes the environment files (env.json and
// env.<name>.json), keeping the order in which profiles and targets are declared.
// Files in the old format (UPPER_CASE keys, from config.yaml/config.json) are
// also read, so they can be migrated.
package env

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
	"github.com/v-felipeolliveira/ssm-helper/internal/msg"
)

// ErrNotFound means the environment file does not exist at the expected path.
var ErrNotFound = msg.Err(msg.EnvNotFound)

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
	// regionPattern accepts AWS region names (us-east-1, sa-east-1, us-gov-west-1).
	// The SDK builds endpoint hosts from the region without checking it, so a
	// value like "evil.example.com/x" would send tokens to another host.
	regionPattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]+$`)
	portPattern   = regexp.MustCompile(`^[0-9]{1,5}$`)
)

// ValidRegion reports whether region looks like an AWS region name.
func ValidRegion(region string) bool {
	return regionPattern.MatchString(region)
}

// ValidPort reports whether port is a number from 1 to 65535.
func ValidPort(port string) bool {
	if !portPattern.MatchString(port) {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

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
func Load(path string) (*Env, []msg.Message, error) {
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
func Parse(path string, data []byte) (*Env, []msg.Message, error) {
	root, err := jsonfile.Parse(data)
	if err != nil {
		return nil, nil, err
	}

	e := &Env{Path: path, Profiles: []Profile{}, Targets: []Target{}}
	var problems []msg.Message
	add := func(code string, params ...string) { problems = append(problems, msg.New(code, params...)) }

	e.Title = jsonfile.Field(root, "title")
	if e.Title == "" {
		e.Title = jsonfile.Field(jsonfile.Object(root, "variables"), "MENU_TITLE")
	}
	if e.Title == "" {
		add(msg.EnvFieldRequired, "path", "title")
	}

	profileNames := map[string]bool{}
	if profiles := jsonfile.Object(root, "profiles"); profiles != nil {
		if len(profiles.Content) == 0 {
			add(msg.EnvNoProfiles)
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
				add(msg.EnvInvalidName, "path", "profiles."+name)
				continue
			}
			for _, f := range profileFields {
				if field(node, f) == "" {
					add(msg.EnvFieldRequired, "path", "profiles."+name+"."+f[0])
				}
			}
			if region := e.Profiles[len(e.Profiles)-1].SSORegion; region != "" && !ValidRegion(region) {
				add(msg.EnvInvalidRegion, "path", "profiles."+name+".ssoRegion", "value", region)
			}
		}
	} else {
		add(msg.EnvSectionMissing, "section", "profiles")
	}

	if targets := jsonfile.Object(root, "targets"); targets != nil {
		if len(targets.Content) == 0 {
			add(msg.EnvNoTargets)
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
				add(msg.EnvInvalidName, "path", "targets."+name)
				continue
			}
			for _, f := range requiredTargetFields {
				if tf(f) == "" {
					add(msg.EnvFieldRequired, "path", "targets."+name+"."+f)
				}
			}
			if t.SSMRegion != "" && !ValidRegion(t.SSMRegion) {
				add(msg.EnvInvalidRegion, "path", "targets."+name+".ssmRegion", "value", t.SSMRegion)
			}
			for _, port := range [][2]string{{"remotePort", t.RemotePort}, {"localPort", t.LocalPort}} {
				if port[1] != "" && !ValidPort(port[1]) {
					add(msg.EnvInvalidPort, "path", "targets."+name+"."+port[0], "value", port[1])
				}
			}
			if t.Profile != "" && !profileNames[t.Profile] {
				add(msg.EnvProfileNotFound, "path", "targets."+name+".profile", "profile", t.Profile)
			}
			for _, link := range t.DNSHosts {
				if !dnsPattern.MatchString(link) || !dnsHostPattern.MatchString(LinkToHost(link)) {
					add(msg.EnvInvalidDomain, "path", "targets."+name+".dnsHosts", "value", link)
				}
			}
			for _, link := range t.OpenURLs {
				if !linkPattern.MatchString(link) || !urlHostPattern.MatchString(LinkToHost(link)) {
					add(msg.EnvInvalidURL, "path", "targets."+name+".openUrls", "value", link)
				}
			}
			if t.ChromeProfile != "" && !NamePattern.MatchString(t.ChromeProfile) {
				add(msg.EnvInvalidName, "path", "targets."+name+".chromeProfile")
			}
		}
	} else {
		add(msg.EnvSectionMissing, "section", "targets")
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
