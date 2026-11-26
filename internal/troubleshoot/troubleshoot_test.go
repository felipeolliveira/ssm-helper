package troubleshoot

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const qaEnv = `{
  "title": "QA",
  "profiles": {
    "qa-admin": {
      "ssoUrl": "https://identitycenter.amazonaws.com/ssoins-1234567890abcdef",
      "ssoRegion": "us-east-1",
      "accountId": "111122223333",
      "roleName": "Admin"
    }
  },
  "targets": {
    "qa-web": {
      "profile": "qa-admin",
      "ssmTarget": "i-0abc1234def567890",
      "ssmRegion": "us-east-1",
      "ssmDocument": "AWS-StartPortForwardingSessionToRemoteHost",
      "remoteHost": "internal-alb-qa.us-east-1.elb.amazonaws.com",
      "remotePort": "443",
      "localPort": "8443",
      "dnsHosts": ["*.qa.secret-corp.com"],
      "openUrls": ["app.qa.secret-corp.com/login"],
      "chromeProfile": ""
    }
  }
}
`

var secrets = []string{
	"ssoins-1234567890abcdef",
	"111122223333",
	"i-0abc1234def567890",
	"internal-alb-qa",
	"secret-corp.com",
}

func readZip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(content)
	}
	return files
}

func TestWrite(t *testing.T) {
	root := t.TempDir()
	envDir := root
	logDir := filepath.Join(root, "ssm-helper-data", "logs")
	write := func(path, content string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(path), 0o700)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(envDir, "env.qa.json"), qaEnv)
	write(filepath.Join(envDir, "env.broken.json"), `{"title": "x", "profiles": {"p": {"ssoUrl": "https://leak.example/start"`)
	// Never read: tokens and cookies
	write(filepath.Join(root, "ssm-helper-data", "sso-cache", "token.json"), "SSO-TOKEN")
	write(filepath.Join(root, "ssm-helper-data", "chrome-qa", "Default", "Cookies"), "COOKIES")

	now := time.Date(2026, 11, 26, 15, 4, 0, 0, time.UTC)
	write(filepath.Join(logDir, "ssm-helper-2026-11-26.log"),
		`2026-11-26 10:00:00 [qa-web] TUNNEL_ACTIVE port="8443"
2026-11-26 10:00:01 [qa-web] TUNNEL_PLUGIN_OUTPUT | Starting session with SessionId i-0abc1234def567890 to internal-alb-qa.us-east-1.elb.amazonaws.com
2026-11-26 10:00:02 [qa-admin] SSO_LOGIN_FAILED | account 111122223333 at https://identitycenter.amazonaws.com/ssoins-1234567890abcdef
2026-11-26 10:00:03 [qa-web] CHROME_COMMAND | --host-rules=MAP *.qa.secret-corp.com 127.0.0.1:8443 https://app.qa.secret-corp.com/login
`)
	write(filepath.Join(logDir, "ssm-helper-2026-11-20.log"), "six days ago\n")
	write(filepath.Join(logDir, "ssm-helper-2026-11-19.log"), "seven days ago, left out\n")
	write(filepath.Join(logDir, "notes.txt"), "not a log file\n")

	var buf bytes.Buffer
	err := Write(&buf, Input{
		Now:      now,
		Docs:     map[string]any{"about.json": map[string]string{"version": "1.4.0"}, "state.json": map[string]any{"tunnels": []string{"qa-web connected on internal-alb-qa.us-east-1.elb.amazonaws.com"}}},
		EnvDir:   envDir,
		EnvNames: []string{"broken", "qa"},
		LogDir:   logDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := readZip(t, buf.Bytes())

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{
		"README.txt", "about.json",
		"environments/env.broken.json", "environments/env.qa.json",
		"logs/ssm-helper-2026-11-20.log", "logs/ssm-helper-2026-11-26.log",
		"state.json",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("entries =\n%q\nwant\n%q", names, want)
	}

	for name, content := range files {
		for _, s := range append(secrets, "SSO-TOKEN", "COOKIES", "leak.example") {
			if strings.Contains(content, s) {
				t.Errorf("%s contains %q:\n%s", name, s, content)
			}
		}
	}

	var qa envEntry
	if err := json.Unmarshal([]byte(files["environments/env.qa.json"]), &qa); err != nil {
		t.Fatal(err)
	}
	if qa.Env == nil || qa.Env.Profiles[0].AccountID != Mask+"3333" || qa.Env.Targets[0].LocalPort != "8443" || qa.Env.Path != "env.qa.json" {
		t.Errorf("masked env = %+v", qa.Env)
	}
	var broken envEntry
	if err := json.Unmarshal([]byte(files["environments/env.broken.json"]), &broken); err != nil {
		t.Fatal(err)
	}
	if broken.Env != nil || broken.Error == nil || broken.Error.Code == "" {
		t.Errorf("broken env = %+v", broken)
	}
	if !strings.Contains(files["logs/ssm-helper-2026-11-26.log"], `TUNNEL_ACTIVE port="8443"`) {
		t.Errorf("log lost its content:\n%s", files["logs/ssm-helper-2026-11-26.log"])
	}
}

func TestMaskerPrefersLongest(t *testing.T) {
	m := newMasker([]string{"qa.example.com", "https://qa.example.com/x", "abc"})
	if got := m.Replace("open https://qa.example.com/x and api.qa.example.com, abc"); got != "open "+Mask+" and api."+Mask+", abc" {
		t.Errorf("Replace = %q", got)
	}
}
