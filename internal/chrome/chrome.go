// Package chrome opens a separate Chrome instance that resolves the target
// domains to the local tunnel, without touching the Windows hosts file.
package chrome

import (
	"fmt"
	"os"
	"os/exec"
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

// Args builds the Chrome command line; each profile gets its own user data dir.
func Args(t env.Target, dataRoot string) []string {
	args := []string{
		"--user-data-dir=" + filepath.Join(dataRoot, "chrome-"+t.ChromeProfileName()),
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

// Launch opens Chrome for the target and returns the command line used.
func Launch(t env.Target, dataRoot string) (string, error) {
	path, err := findChrome()
	if err != nil {
		return "", err
	}
	args := Args(t, dataRoot)
	commandLine := fmt.Sprintf("%q %s", path, strings.Join(args, " "))
	cmd := exec.Command(path, args...)
	if err := cmd.Start(); err != nil {
		return commandLine, msg.Wrap(msg.ChromeOpenFailed, err)
	}
	go cmd.Wait()
	return commandLine, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
