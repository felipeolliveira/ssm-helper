// Package chrome opens a separate Chrome instance that resolves the target
// domains to the local tunnel, without touching the Windows hosts file.
package chrome

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/felipeolliveira/ssm-helper/internal/env"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

// ErrNotFound means chrome.exe could not be located.
var ErrNotFound = msg.Err(msg.ChromeNotFound)

// HostRules maps each dnsHosts entry to the local tunnel port.
func HostRules(t env.Target) string {
	rules := make([]string, 0, len(t.DNSHosts))
	for _, host := range t.DNSHosts {
		rules = append(rules, fmt.Sprintf("MAP %s 127.0.0.1:%s", env.LinkToHost(host), t.LocalPort))
	}
	return strings.Join(rules, ",")
}

// ProfileDir is the user data dir of the target's Chrome profile. It does not
// depend on the environment: targets with the same profile share it.
func ProfileDir(t env.Target, dataRoot string) string {
	return filepath.Join(dataRoot, "chrome-"+t.ChromeProfileName())
}

// Args builds the Chrome command line; each profile gets its own user data dir.
func Args(t env.Target, dataRoot string) []string {
	args := []string{
		"--user-data-dir=" + ProfileDir(t, dataRoot),
		"--no-first-run",
	}
	if rules := HostRules(t); rules != "" {
		args = append(args, "--host-rules="+rules)
	}
	for _, link := range t.OpenURLs {
		args = append(args, env.LinkToURL(link))
	}
	return args
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
