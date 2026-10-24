package chrome

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/felipeolliveira/ssm-helper/internal/env"
)

func TestArgs(t *testing.T) {
	target := env.Target{
		Name:      "qa-web",
		LocalPort: "8443",
		DNSHosts:  []string{"qa.example.com", "*.api.example.com"},
		OpenURLs:  []string{"qa.example.com", "http://other.example.com/x"},
	}
	want := []string{
		"--user-data-dir=" + filepath.Join("root", "chrome-qa-web"),
		"--no-first-run",
		"--host-rules=MAP qa.example.com 127.0.0.1:8443,MAP *.api.example.com 127.0.0.1:8443",
		"https://qa.example.com",
		"http://other.example.com/x",
	}
	if got := Args(target, "root"); !reflect.DeepEqual(got, want) {
		t.Errorf("Args =\n%q\nwant\n%q", got, want)
	}

	target.ChromeProfile = "qa"
	target.DNSHosts = nil
	got := Args(target, "root")
	if got[0] != "--user-data-dir="+filepath.Join("root", "chrome-qa") || len(got) != 4 {
		t.Errorf("Args with chromeProfile = %q", got)
	}
}
