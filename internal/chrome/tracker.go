package chrome

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/felipeolliveira/ssm-helper/internal/env"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

// How long Clear waits for Chrome to close by itself before killing it, and
// for its files to be released after it exits (child processes linger).
var (
	closeTimeout   = 5 * time.Second
	releaseTimeout = 4 * time.Second
	releaseStep    = 250 * time.Millisecond
)

// deletingSuffix marks a profile folder renamed by Clear and not yet removed.
const deletingSuffix = ".deleting-"

// Tracker keeps the Chrome processes the app opened, one per profile folder,
// so the app can close a window and Clear can delete the folder after it.
type Tracker struct {
	mu       sync.Mutex
	procs    map[string]*process
	clearing map[string]bool
	onChange func() // called when a tracked Chrome opens or exits
}

type process struct {
	profile string
	cmd     *exec.Cmd
	done    chan struct{}
}

// NewTracker takes the function called, outside any lock, whenever the list
// of open profiles changes; it may be nil.
func NewTracker(onChange func()) *Tracker {
	if onChange == nil {
		onChange = func() {}
	}
	return &Tracker{procs: map[string]*process{}, clearing: map[string]bool{}, onChange: onChange}
}

// Launch opens Chrome for the target and returns the command line used.
func (tr *Tracker) Launch(t env.Target, dataRoot string) (string, error) {
	dir := ProfileDir(t, dataRoot)
	tr.mu.Lock()
	busy := tr.clearing[dir]
	tr.mu.Unlock()
	if busy {
		return "", msg.Err(msg.ChromeProfileBusy, "profile", t.ChromeProfileName())
	}

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
	tr.track(dir, t.ChromeProfileName(), cmd)
	return commandLine, nil
}

// track waits for cmd in the background. When Chrome is already open for the
// folder, the new process hands its URLs to it and exits at once, so the
// process kept is always the one that owns the window.
func (tr *Tracker) track(dir, profile string, cmd *exec.Cmd) {
	p := &process{profile: profile, cmd: cmd, done: make(chan struct{})}
	tr.mu.Lock()
	_, running := tr.procs[dir]
	if !running {
		tr.procs[dir] = p
	}
	tr.mu.Unlock()
	if !running {
		tr.onChange()
	}
	go func() {
		cmd.Wait()
		close(p.done)
		tr.mu.Lock()
		owner := tr.procs[dir] == p
		if owner {
			delete(tr.procs, dir)
		}
		tr.mu.Unlock()
		if owner {
			tr.onChange()
		}
	}()
}

// Running reports whether a Chrome opened by the app uses the target's profile.
func (tr *Tracker) Running(t env.Target, dataRoot string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.procs[ProfileDir(t, dataRoot)] != nil
}

// Open lists the profiles with a Chrome opened by the app, sorted.
func (tr *Tracker) Open() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	profiles := make([]string, 0, len(tr.procs))
	for _, p := range tr.procs {
		profiles = append(profiles, p.profile)
	}
	slices.Sort(profiles)
	return profiles
}

// Close closes the Chrome the app opened for the target's profile and waits
// for it to exit. closed is false when there was none.
func (tr *Tracker) Close(t env.Target, dataRoot string) (closed bool, err error) {
	dir := ProfileDir(t, dataRoot)
	tr.mu.Lock()
	busy := tr.clearing[dir]
	p := tr.procs[dir]
	tr.mu.Unlock()
	if busy {
		return false, msg.Err(msg.ChromeProfileBusy, "profile", t.ChromeProfileName())
	}
	if p == nil {
		return false, nil
	}
	p.close()
	return true, nil
}

// close asks Chrome to close, as clicking X would, and kills it if it does
// not exit within closeTimeout (a page may hold it with a "leave site?" prompt).
func (p *process) close() {
	closeProcess(p.cmd.Process)
	select {
	case <-p.done:
	case <-time.After(closeTimeout):
		killProcess(p.cmd.Process)
		select {
		case <-p.done:
		case <-time.After(closeTimeout):
		}
	}
}

// Clear closes the Chrome the app opened for the target's profile and deletes
// the profile folder: cookies, storage, cache, logins and extensions. closed
// tells whether a window was closed. A Chrome the app did not open (for
// example, from before a restart) is left alone and the folder is kept.
func (tr *Tracker) Clear(t env.Target, dataRoot string) (closed bool, err error) {
	dir := ProfileDir(t, dataRoot)
	profile := t.ChromeProfileName()
	tr.mu.Lock()
	if tr.clearing[dir] {
		tr.mu.Unlock()
		return false, msg.Err(msg.ChromeProfileBusy, "profile", profile)
	}
	tr.clearing[dir] = true
	p := tr.procs[dir]
	tr.mu.Unlock()
	defer func() {
		tr.mu.Lock()
		delete(tr.clearing, dir)
		tr.mu.Unlock()
	}()

	if p != nil {
		closed = true
		p.close()
	}

	removeLeftovers(dir)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return closed, nil
	}
	// Renaming first never leaves a half-deleted profile behind: on Windows it
	// fails while Chrome keeps any file of the folder open.
	tmp := fmt.Sprintf("%s%s%d", dir, deletingSuffix, time.Now().UnixNano())
	deadline := time.Now().Add(releaseTimeout)
	for {
		if !inUse(dir) {
			if err = os.Rename(dir, tmp); err == nil || errors.Is(err, os.ErrNotExist) {
				break
			}
		}
		if time.Now().After(deadline) {
			e := msg.Err(msg.ChromeProfileInUse, "profile", profile)
			if err != nil {
				e = msg.Wrap(msg.ChromeProfileInUse, err, "profile", profile)
			}
			return closed, e
		}
		time.Sleep(releaseStep)
	}
	if err != nil {
		return closed, nil // removed in the meantime
	}
	if err := os.RemoveAll(tmp); err != nil {
		// The profile is already out of the way: the next Chrome starts clean
		return closed, msg.Wrap(msg.ChromeProfileClearFailed, err, "profile", profile)
	}
	return closed, nil
}

// removeLeftovers deletes folders an earlier Clear renamed but could not remove.
func removeLeftovers(dir string) {
	matches, _ := filepath.Glob(dir + deletingSuffix + "*")
	for _, m := range matches {
		os.RemoveAll(m)
	}
}
