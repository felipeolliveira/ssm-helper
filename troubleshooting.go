package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/felipeolliveira/ssm-helper/internal/env"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
	"github.com/felipeolliveira/ssm-helper/internal/troubleshoot"
)

// troubleshootState is state.json in the troubleshooting file.
type troubleshootState struct {
	GeneratedAt        string          `json:"generatedAt"`
	ActiveEnv          string          `json:"activeEnv"`
	Envs               []EnvInfo       `json:"envs"`
	Tunnels            []TunnelState   `json:"tunnels"`
	Profiles           []ProfileStatus `json:"profiles"`
	OpenChromeProfiles []string        `json:"openChromeProfiles"`
	Preferences        Preferences     `json:"preferences"`
}

// SaveTroubleshooting asks where to save the troubleshooting file, writes it
// and shows it in Explorer. It returns the file, or "" when the user cancels.
func (a *App) SaveTroubleshooting() (string, error) {
	now := time.Now()
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		DefaultDirectory: downloadsDir(a.dir),
		DefaultFilename:  "ssm-helper-troubleshooting-" + now.Format("2006-01-02-1504") + ".zip",
		Filters:          []wruntime.FileFilter{{DisplayName: "ZIP (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil {
		return "", msg.Wrap(msg.TroubleshootingSaveFailed, err)
	}
	if path == "" {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		path += ".zip"
	}

	names, _ := env.List(a.dir)
	list, _ := a.ListEnvs()
	state := troubleshootState{
		GeneratedAt:        now.Format(time.RFC3339),
		ActiveEnv:          a.activeName(),
		Envs:               list.Envs,
		Tunnels:            a.TunnelStates(),
		Profiles:           a.ProfileStatuses(),
		OpenChromeProfiles: a.chrome.Open(),
		Preferences:        a.GetPreferences(),
	}
	if err := writeTroubleshooting(path, troubleshoot.Input{
		Now:      now,
		Docs:     map[string]any{"about.json": a.About(), "state.json": state},
		EnvDir:   a.dir,
		EnvNames: names,
		LogDir:   a.logFile.Dir,
	}); err != nil {
		return "", msg.Wrap(msg.TroubleshootingSaveFailed, err)
	}
	a.logf("troubleshooting", msg.New(msg.TroubleshootingSaved, "file", path))
	revealFile(path)
	return path, nil
}

// writeTroubleshooting writes next to path first, so a failure never leaves a
// broken file with the chosen name.
func writeTroubleshooting(path string, in troubleshoot.Input) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := troubleshoot.Write(f, in); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
