package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/felipeolliveira/ssm-helper/internal/msg"
	"github.com/felipeolliveira/ssm-helper/internal/update"
)

// channel is "release" in the builds of the release workflow (-ldflags -X).
// Other builds carry the version of wails.json in the repository, which is
// not a real release, so they never look for updates.
var channel string

// afterUpdateArg starts the new executable after an update: it waits for the
// old process to exit, since the single instance lock would hand the start
// over to it.
const afterUpdateArg = "--after-update"

const (
	eventUpdateAvailable = "update:available"
	eventUpdateProgress  = "update:progress"
	updateFirstCheck     = 10 * time.Second
	updateCheckEvery     = 12 * time.Hour
)

// UpdateInfo is what the screens show about updates.
type UpdateInfo struct {
	// Enabled is false in builds that are not releases.
	Enabled bool   `json:"enabled"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
	URL     string `json:"url"`
	// Available means Latest is newer than Current and can be installed.
	Available bool `json:"available"`
	// Blocked means Latest is newer, but its signature did not match the keys
	// built into the app: it is never installed and the user is told to
	// contact support instead of downloading it.
	Blocked bool `json:"blocked"`
	// CheckedAt is empty until the first check finishes.
	CheckedAt string `json:"checkedAt"`
}

type UpdateProgress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

func (a *App) updateEnabled() bool { return channel == "release" }

func (a *App) currentVersion() string { return a.About().Version }

// releasesRepo is "owner/name" of the GitHub repository of the build.
func (a *App) releasesRepo() string {
	return strings.TrimPrefix(a.About().Repository, "https://github.com/")
}

// GetUpdateInfo returns the result of the last check.
func (a *App) GetUpdateInfo() UpdateInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.updateInfoLocked()
}

func (a *App) updateInfoLocked() UpdateInfo {
	info := UpdateInfo{Enabled: a.updateEnabled(), Current: a.version}
	if a.latest != nil {
		newer := update.Newer(a.version, a.latest.Tag)
		info.Latest = a.latest.Tag
		info.URL = a.latest.URL
		info.Blocked = newer && a.blocked == a.latest.Tag
		info.Available = newer && !info.Blocked
		info.CheckedAt = a.checkedAt.Format(time.RFC3339)
	}
	return info
}

// CheckUpdate looks for a newer release now.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	if !a.updateEnabled() {
		return a.GetUpdateInfo(), nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	r, err := update.Latest(ctx, a.releasesRepo())
	if err != nil {
		return a.GetUpdateInfo(), err
	}
	// A release whose signature does not match is never offered. Other
	// failures (network) are left to the update itself.
	if update.Newer(a.version, r.Tag) && r.Ready() {
		if _, err := update.VerifiedSums(ctx, r); errors.Is(err, msg.Err(msg.UpdateSignatureInvalid)) {
			a.block(r.Tag, err)
		}
	}
	a.mu.Lock()
	notify := update.Newer(a.version, r.Tag) && a.notified != r.Tag
	if notify {
		a.notified = r.Tag
	}
	a.latest, a.checkedAt = &r, time.Now()
	info := a.updateInfoLocked()
	a.mu.Unlock()
	if notify {
		if info.Available {
			a.logf("update", msg.New(msg.UpdateAvailable, "version", r.Tag))
		}
		wruntime.EventsEmit(a.ctx, eventUpdateAvailable, info)
	}
	return info, nil
}

// block keeps a release with a bad signature from being offered again in
// this run and records why, for the log file and the troubleshooting file.
func (a *App) block(tag string, err error) {
	a.mu.Lock()
	first := a.blocked != tag
	a.blocked = tag
	a.mu.Unlock()
	if first {
		a.logf("update", msg.From(err))
	}
}

// checkUpdatesLoop checks shortly after startup and then every few hours,
// while the preference is on. Failures (no network, a proxy, the GitHub rate
// limit) only go to the log file and panel.
func (a *App) checkUpdatesLoop() {
	if !a.updateEnabled() {
		return
	}
	wait := updateFirstCheck
	for {
		select {
		case <-time.After(wait):
		case <-a.ctx.Done():
			return
		}
		wait = updateCheckEvery
		a.mu.Lock()
		enabled := a.settings.CheckUpdates
		a.mu.Unlock()
		if !enabled {
			continue
		}
		if _, err := a.CheckUpdate(); err != nil {
			a.logf("update", msg.From(err))
		}
	}
}

// SetCheckUpdates turns the automatic check on or off.
func (a *App) SetCheckUpdates(check bool) {
	a.mu.Lock()
	a.settings.CheckUpdates = check
	a.mu.Unlock()
	a.saveSettings()
}

// ApplyUpdate downloads the latest release, checks it against SHA256SUMS.txt,
// puts it in place of the running executable and restarts the app. Open
// tunnels close with the app.
func (a *App) ApplyUpdate() error {
	a.mu.Lock()
	r := a.latest
	busy := a.applyingUpdate
	if r != nil && !busy {
		a.applyingUpdate = true
	}
	a.mu.Unlock()
	if busy {
		return nil
	}
	if r != nil && a.GetUpdateInfo().Blocked {
		a.mu.Lock()
		a.applyingUpdate = false
		a.mu.Unlock()
		return msg.Err(msg.UpdateSignatureInvalid, "version", r.Tag)
	}
	if r == nil || !update.Newer(a.version, r.Tag) {
		if r != nil {
			a.mu.Lock()
			a.applyingUpdate = false
			a.mu.Unlock()
		}
		return msg.Wrap(msg.UpdateCheckFailed, errors.New("no newer release found"))
	}
	defer func() {
		a.mu.Lock()
		a.applyingUpdate = false
		a.mu.Unlock()
	}()
	if !r.Ready() {
		return msg.Err(msg.UpdateNotReady, "version", r.Tag)
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return msg.Wrap(msg.UpdateApplyFailed, err)
	}
	newPath := exe + ".new"
	err = update.Download(a.ctx, *r, newPath, func(done, total int64) {
		wruntime.EventsEmit(a.ctx, eventUpdateProgress, UpdateProgress{Done: done, Total: total})
	})
	if errors.Is(err, msg.Err(msg.UpdateSignatureInvalid)) {
		a.block(r.Tag, err)
		wruntime.EventsEmit(a.ctx, eventUpdateAvailable, a.GetUpdateInfo())
		return err
	}
	if err != nil {
		a.logf("update", msg.From(err))
		return err
	}
	if err := update.Swap(exe, newPath); err != nil {
		os.Remove(newPath)
		a.logf("update", msg.From(err))
		return err
	}
	a.logf("update", msg.New(msg.UpdateRestarting, "version", r.Tag))
	cmd := exec.Command(exe, afterUpdateArg, strconv.Itoa(os.Getpid()))
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		// The new version is already in place: it runs when the app is opened again
		e := msg.Wrap(msg.UpdateApplyFailed, err)
		a.logf("update", msg.From(e))
		return e
	}
	go func() {
		// Lets the call return to the screen before the window closes
		time.Sleep(300 * time.Millisecond)
		wruntime.Quit(a.ctx)
	}()
	return nil
}

// afterUpdate runs before the window opens when the app was started by an
// update: it waits for the old process and reports the new version.
func afterUpdate(args []string) (updated bool) {
	if len(args) < 2 || args[0] != afterUpdateArg {
		return false
	}
	if pid, err := strconv.Atoi(args[1]); err == nil {
		update.WaitForExit(pid, 30*time.Second)
	}
	return true
}

// removeOldExecutable deletes what an earlier update left behind.
func removeOldExecutable() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if exe, err = filepath.EvalSymlinks(exe); err == nil {
		update.RemoveOld(exe)
	}
}
