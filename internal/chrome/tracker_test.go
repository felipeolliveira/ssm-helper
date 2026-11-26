package chrome

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/felipeolliveira/ssm-helper/internal/env"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

func TestProfileDirMatchesArgs(t *testing.T) {
	target := env.Target{Name: "web", ChromeProfile: "qa"}
	if got, want := Args(target, "root")[0], "--user-data-dir="+ProfileDir(target, "root"); got != want {
		t.Errorf("Args[0] = %q, want %q", got, want)
	}
}

func writeProfile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "Default", "Local Storage"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Default", "Cookies"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestClear(t *testing.T) {
	root := t.TempDir()
	target := env.Target{Name: "web"}
	tr := NewTracker(nil)

	// Nothing to clear yet
	if closed, err := tr.Clear(target, root); err != nil || closed {
		t.Fatalf("Clear without a folder = %v, %v", closed, err)
	}

	dir := ProfileDir(target, root)
	writeProfile(t, dir)
	// A leftover from an earlier Clear that could not finish
	leftover := dir + deletingSuffix + "1"
	writeProfile(t, leftover)

	if closed, err := tr.Clear(target, root); err != nil || closed {
		t.Fatalf("Clear = %v, %v", closed, err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Errorf("left in the data folder: %v", entries)
	}
}

func TestClearClosesTrackedChrome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sleep as a stand-in for Chrome")
	}
	root := t.TempDir()
	target := env.Target{Name: "web"}
	dir := ProfileDir(target, root)
	writeProfile(t, dir)

	tr := NewTracker(nil)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tr.track(dir, "web", cmd)
	// A second launch while the first runs hands off and exits; the first stays tracked
	second := exec.Command("true")
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	tr.track(dir, "web", second)
	if !tr.Running(target, root) {
		t.Fatal("Running = false")
	}

	closed, err := tr.Clear(target, root)
	if err != nil || !closed {
		t.Fatalf("Clear = %v, %v", closed, err)
	}
	if cmd.ProcessState == nil {
		t.Error("the tracked process is still running")
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("profile folder still there: %v", err)
	}
	if tr.Running(target, root) {
		t.Error("Running = true after Clear")
	}
}

func TestClearKeepsProfileInUse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SingletonLock is how Chrome locks a profile outside Windows")
	}
	defer func(timeout time.Duration) { releaseTimeout = timeout }(releaseTimeout)
	releaseTimeout = 0

	root := t.TempDir()
	target := env.Target{Name: "web", ChromeProfile: "qa"}
	dir := ProfileDir(target, root)
	writeProfile(t, dir)
	// A Chrome the app did not open: this test process holds the lock
	if err := os.Symlink(fmt.Sprintf("host-%d", os.Getpid()), filepath.Join(dir, "SingletonLock")); err != nil {
		t.Fatal(err)
	}

	_, err := NewTracker(nil).Clear(target, root)
	if !errors.Is(err, msg.Err(msg.ChromeProfileInUse)) || msg.From(err).Params["profile"] != "qa" {
		t.Fatalf("Clear = %v, want %s for qa", err, msg.ChromeProfileInUse)
	}
	if _, err := os.Stat(filepath.Join(dir, "Default", "Cookies")); err != nil {
		t.Errorf("profile changed: %v", err)
	}

	// A lock left by a crash does not block
	os.Remove(filepath.Join(dir, "SingletonLock"))
	os.Symlink("host-999999999", filepath.Join(dir, "SingletonLock"))
	if _, err := NewTracker(nil).Clear(target, root); err != nil {
		t.Fatalf("Clear with a stale lock = %v", err)
	}
}

func TestLaunchWhileClearing(t *testing.T) {
	target := env.Target{Name: "web"}
	tr := NewTracker(nil)
	tr.clearing[ProfileDir(target, "root")] = true
	if _, err := tr.Launch(target, "root"); !errors.Is(err, msg.Err(msg.ChromeProfileBusy)) {
		t.Fatalf("Launch = %v, want %s", err, msg.ChromeProfileBusy)
	}
}

func TestCloseAndOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sleep as a stand-in for Chrome")
	}
	root := t.TempDir()
	target := env.Target{Name: "web", ChromeProfile: "qa"}
	changes := make(chan []string, 4)
	var tr *Tracker
	tr = NewTracker(func() { changes <- tr.Open() })

	if closed, err := tr.Close(target, root); err != nil || closed {
		t.Fatalf("Close without Chrome = %v, %v", closed, err)
	}

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tr.track(ProfileDir(target, root), "qa", cmd)
	if got := <-changes; !slices.Equal(got, []string{"qa"}) {
		t.Fatalf("open after launch = %v", got)
	}

	if closed, err := tr.Close(target, root); err != nil || !closed {
		t.Fatalf("Close = %v, %v", closed, err)
	}
	select {
	case got := <-changes:
		if len(got) != 0 {
			t.Errorf("open after Close = %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no change after Close")
	}
}
