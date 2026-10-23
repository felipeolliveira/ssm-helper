package env

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
	"github.com/v-felipeolliveira/ssm-helper/internal/msg"
)

func TestLoadExample(t *testing.T) {
	e, problems, err := Load(filepath.Join("..", "..", "env.json.example"))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if e.Title != "Ambiente de QA" {
		t.Errorf("Title = %q", e.Title)
	}
	if got := []string{e.Profiles[0].Name, e.Profiles[1].Name}; !reflect.DeepEqual(got, []string{"qa-admin", "dev-admin"}) {
		t.Errorf("profile order = %v", got)
	}
	web, ok := e.Target("qa-web")
	if !ok {
		t.Fatal("qa-web not found")
	}
	if !reflect.DeepEqual(web.DNSHosts, []string{"qa.example.com", "api.qa.example.com"}) {
		t.Errorf("DNSHosts = %v", web.DNSHosts)
	}
	if web.LocalPort != "8443" || web.ChromeProfileName() != "qa-web" || !web.UsesBrowser() {
		t.Errorf("unexpected target: %+v", web)
	}
	if db, _ := e.Target("qa-database"); db.UsesBrowser() || db.DNSHosts == nil {
		t.Errorf("unexpected database target: %+v", db)
	}
}

// The old format (UPPER_CASE keys, lists as text) is still read, for migration.
func TestParseLegacyFormat(t *testing.T) {
	data := `
variables:
  MENU_TITLE: "QA"
profiles:
  qa:
    AWS_SSO_URL: "https://x"
    AWS_SSO_REGION: "us-east-1"
    AWS_SSO_ACCOUNT_ID: "1"
    AWS_SSO_ROLE_NAME: "r"
targets:
  web:
    PROFILE: "qa"
    SSM_TARGET: "i-1"
    SSM_DOCUMENT: "doc"
    SSM_REMOTE_HOST: "h"
    SSM_REMOTE_PORT: "443"
    SSM_LOCAL_PORT: "8443"
    DNS_HOSTS: "a.com b.com"
    OPEN_URLS: ""
`
	e, problems, err := Parse("x", []byte(data))
	if err != nil || len(problems) > 0 {
		t.Fatalf("problems=%v err=%v", problems, err)
	}
	web, _ := e.Target("web")
	if e.Title != "QA" || e.Profiles[0].SSOURL != "https://x" || web.SSMTarget != "i-1" ||
		!reflect.DeepEqual(web.DNSHosts, []string{"a.com", "b.com"}) || len(web.OpenURLs) != 0 {
		t.Errorf("legacy parse = %+v", e)
	}
}

func TestLoadNotFound(t *testing.T) {
	_, _, err := Load(filepath.Join(t.TempDir(), DefaultFileName))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, _, err := Parse("x", []byte(`{"title": `))
	if !jsonfile.IsFormatError(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseProblems(t *testing.T) {
	data := `{
  "profiles": {
    "bad name": {"ssoUrl": "x"},
    "ok": {"ssoUrl": "x", "ssoRegion": "us-east-1", "accountId": "123"}
  },
  "targets": {
    "web": {
      "profile": "missing",
      "ssmTarget": "i-1",
      "ssmDocument": "doc",
      "remoteHost": "host",
      "remotePort": "443",
      "localPort": "8443",
      "dnsHosts": ["*.qa.example.com", "bad_host!"],
      "openUrls": ["https://qa.example.com/path", "*.example.com"],
      "chromeProfile": "a b"
    }
  }
}`
	_, problems, err := Parse("x", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []msg.Message{
		msg.New(msg.EnvFieldRequired, "path", "title"),
		msg.New(msg.EnvInvalidName, "path", "profiles.bad name"),
		msg.New(msg.EnvFieldRequired, "path", "profiles.ok.roleName"),
		msg.New(msg.EnvProfileNotFound, "path", "targets.web.profile", "profile", "missing"),
		msg.New(msg.EnvInvalidDomain, "path", "targets.web.dnsHosts", "value", "bad_host!"),
		msg.New(msg.EnvInvalidURL, "path", "targets.web.openUrls", "value", "*.example.com"),
		msg.New(msg.EnvInvalidName, "path", "targets.web.chromeProfile"),
	}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("problems =\n%+v\nwant\n%+v", problems, want)
	}
}

func TestParseEmptySections(t *testing.T) {
	_, problems, err := Parse("x", []byte(`{"title": "t", "profiles": {}, "targets": {}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []msg.Message{msg.New(msg.EnvNoProfiles), msg.New(msg.EnvNoTargets)}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("problems = %+v", problems)
	}
}

func TestTargetDefaults(t *testing.T) {
	p := Profile{SSORegion: "us-east-1"}
	tg := Target{Name: "db"}
	if tg.Region(p) != "us-east-1" || tg.ChromeProfileName() != "db" || tg.UsesBrowser() {
		t.Errorf("defaults: %+v", tg)
	}
	tg = Target{Name: "db", SSMRegion: "sa-east-1", ChromeProfile: "qa"}
	if tg.Region(p) != "sa-east-1" || tg.ChromeProfileName() != "qa" {
		t.Errorf("overrides: %+v", tg)
	}
}

func TestLinks(t *testing.T) {
	cases := map[string][2]string{
		"qa.example.com":                 {"https://qa.example.com", "qa.example.com"},
		"http://qa.example.com:8080/a":   {"http://qa.example.com:8080/a", "qa.example.com"},
		"https://api.example.com/v1?x=1": {"https://api.example.com/v1?x=1", "api.example.com"},
	}
	for in, want := range cases {
		if got := LinkToURL(in); got != want[0] {
			t.Errorf("LinkToURL(%q) = %q", in, got)
		}
		if got := LinkToHost(in); got != want[1] {
			t.Errorf("LinkToHost(%q) = %q", in, got)
		}
	}
}

func TestLoadReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFileName)
	if err := os.WriteFile(path, []byte(`{"title": "t"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _, err := Load(path)
	if err != nil || e.Path != path || e.Title != "t" {
		t.Fatalf("env = %+v, err = %v", e, err)
	}
}
