// Package troubleshoot builds the troubleshooting file the user sends to
// support: the app version and state, the environments and the recent log
// files, with the sensitive values of the environments masked. It never
// reads the SSO cache, the Chrome profiles or the WebView data.
package troubleshoot

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/felipeolliveira/ssm-helper/internal/env"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

// Mask replaces a sensitive value, as the privacy mode does on screen
// (MASK in frontend/src/privacy.tsx).
const Mask = "••••••••"

// LogDays is how many days of log files go into the file, today included.
const LogDays = 7

// minSecretLen keeps short values (such as a one-letter host) from masking
// unrelated text in the logs.
const minSecretLen = 4

// Input is what goes into the troubleshooting file.
type Input struct {
	Now time.Time
	// Docs are written as JSON files, keyed by name (about.json, state.json).
	Docs map[string]any
	// EnvDir holds the environment files; EnvNames are their names (env.List).
	EnvDir   string
	EnvNames []string
	// LogDir holds the daily log files (internal/applog).
	LogDir string
}

// envEntry is an environment as written to the file.
type envEntry struct {
	File     string        `json:"file"`
	Env      *env.Env      `json:"env,omitempty"`
	Problems []msg.Message `json:"problems,omitempty"`
	// Error is set when the file could not be loaded; its content is left out.
	Error *msg.Message `json:"error,omitempty"`
}

// Write builds the zip. Every entry goes through the masking of the
// sensitive values found in the environments.
func Write(w io.Writer, in Input) error {
	envs, secrets := loadEnvs(in.EnvDir, in.EnvNames)
	masker := newMasker(secrets)
	zw := zip.NewWriter(w)
	add := func(name string, data []byte) error {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: in.Now})
		if err != nil {
			return err
		}
		_, err = f.Write([]byte(masker.Replace(string(data))))
		return err
	}
	addJSON := func(name string, v any) error {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		return add(name, append(data, '\n'))
	}

	if err := add("README.txt", []byte(readme(in.Now))); err != nil {
		return err
	}
	names := make([]string, 0, len(in.Docs))
	for name := range in.Docs {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := addJSON(name, in.Docs[name]); err != nil {
			return err
		}
	}
	for _, e := range envs {
		if err := addJSON("environments/"+e.File, e); err != nil {
			return err
		}
	}
	for _, name := range recentLogs(in.LogDir, in.Now) {
		data, err := os.ReadFile(filepath.Join(in.LogDir, name))
		if err != nil {
			continue
		}
		if err := add("logs/"+name, data); err != nil {
			return err
		}
	}
	return zw.Close()
}

// loadEnvs reads every environment and masks its sensitive fields. It also
// returns the original values, to mask them wherever else they show up.
func loadEnvs(dir string, names []string) ([]envEntry, []string) {
	var entries []envEntry
	var secrets []string
	for _, name := range names {
		file := env.FileName(name)
		e := envEntry{File: file}
		loaded, problems, err := env.Load(filepath.Join(dir, file))
		if err != nil {
			m := msg.From(err)
			e.Error = &m
			entries = append(entries, e)
			continue
		}
		secrets = append(secrets, Secrets(loaded)...)
		maskEnv(loaded)
		loaded.Path = file
		e.Env = loaded
		e.Problems = problems
		entries = append(entries, e)
	}
	return entries, secrets
}

// Secrets lists the sensitive values of an environment: the same fields the
// privacy mode hides on screen.
func Secrets(e *env.Env) []string {
	var out []string
	for _, p := range e.Profiles {
		out = append(out, p.SSOURL, p.AccountID)
	}
	for _, t := range e.Targets {
		out = append(out, t.SSMTarget, t.RemoteHost)
		for _, h := range t.DNSHosts {
			// The logs show a wildcard domain without its "*."
			out = append(out, h, strings.TrimPrefix(h, "*."))
		}
		for _, u := range t.OpenURLs {
			out = append(out, u, strings.TrimSuffix(env.LinkToURL(u), "/"))
		}
	}
	return out
}

func maskEnv(e *env.Env) {
	for i := range e.Profiles {
		p := &e.Profiles[i]
		p.SSOURL = maskValue(p.SSOURL)
		if len(p.AccountID) > 4 {
			p.AccountID = Mask + p.AccountID[len(p.AccountID)-4:]
		} else {
			p.AccountID = maskValue(p.AccountID)
		}
	}
	for i := range e.Targets {
		t := &e.Targets[i]
		t.SSMTarget = maskValue(t.SSMTarget)
		t.RemoteHost = maskValue(t.RemoteHost)
		for j := range t.DNSHosts {
			t.DNSHosts[j] = Mask
		}
		for j := range t.OpenURLs {
			t.OpenURLs[j] = Mask
		}
	}
}

func maskValue(v string) string {
	if v == "" {
		return ""
	}
	return Mask
}

// newMasker replaces each secret with Mask, longest first, so a URL is masked
// as a whole and not only its host.
func newMasker(secrets []string) *strings.Replacer {
	seen := map[string]bool{}
	var unique []string
	for _, s := range secrets {
		if len(s) >= minSecretLen && !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	slices.SortFunc(unique, func(a, b string) int { return len(b) - len(a) })
	pairs := make([]string, 0, 2*len(unique))
	for _, s := range unique {
		pairs = append(pairs, s, Mask)
	}
	return strings.NewReplacer(pairs...)
}

// recentLogs lists the log files of the last LogDays days, oldest first.
func recentLogs(dir string, now time.Time) []string {
	entries, _ := os.ReadDir(dir)
	first := now.AddDate(0, 0, -(LogDays - 1)).Format(time.DateOnly)
	var names []string
	for _, e := range entries {
		day, ok := strings.CutPrefix(e.Name(), "ssm-helper-")
		day, isLog := strings.CutSuffix(day, ".log")
		if !ok || !isLog || e.IsDir() {
			continue
		}
		if _, err := time.Parse(time.DateOnly, day); err == nil && day >= first {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names
}

// readme is for the support team, so it is in English like the log files.
func readme(now time.Time) string {
	return fmt.Sprintf(`SSM Helper troubleshooting file
Generated: %s

about.json        version and build of the app
state.json        active environment, tunnels, SSO sessions (status only) and preferences
environments/     the environment files
logs/             log files of the last %d days

SSO URLs, account numbers (all but the last 4 digits), SSM targets, remote
hosts, domains and URLs of the environments are replaced by %s in every file.
Never included: the SSO cache (tokens), the Chrome profiles (cookies, logins)
and the WebView data.

Still visible: names of profiles, roles and targets, regions, ports, the
Windows user name in the app folders and AWS error messages, which may name
hosts or IPs that are not in the environments. Treat this file as internal:
keep it with the support team or people the user trusts, and delete it once
the issue is solved.
`, now.Format(time.RFC3339), LogDays, Mask)
}
