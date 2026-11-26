package chrome

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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
	tr.track(dir, "web", cmd.Process.Pid, func() { cmd.Wait() })
	// A second launch while the first runs hands off and exits; the first stays tracked
	second := exec.Command("true")
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	tr.track(dir, "web", second.Process.Pid, func() { second.Wait() })
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
	tr.track(ProfileDir(target, root), "qa", cmd.Process.Pid, func() { cmd.Wait() })
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

func TestIsBrowserFor(t *testing.T) {
	dir := filepath.Join("data", "chrome-qa")
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"main process", []string{"chrome", "--user-data-dir=" + dir, "--no-first-run", "https://qa"}, true},
		{"quoted value", []string{"chrome", `--user-data-dir="` + dir + `"`}, true},
		{"not clean", []string{"chrome", "--user-data-dir=" + dir + string(filepath.Separator)}, true},
		{"renderer", []string{"chrome", "--type=renderer", "--user-data-dir=" + dir}, false},
		{"other profile", []string{"chrome", "--user-data-dir=" + dir + "-2"}, false},
		{"default profile", []string{"chrome", "https://qa"}, false},
	}
	for _, c := range cases {
		if got := isBrowserFor(c.args, dir); got != c.want {
			t.Errorf("%s: isBrowserFor = %v, want %v", c.name, got, c.want)
		}
	}
	if pathsIgnoreCase && !isBrowserFor([]string{"chrome", "--user-data-dir=" + strings.ToUpper(dir)}, dir) {
		t.Error("paths differing in case should match on Windows")
	}
}

func TestProfileDirs(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"chrome-qa", "chrome-web", "chrome-qa" + deletingSuffix + "1", "sso-cache"} {
		os.MkdirAll(filepath.Join(root, name), 0o700)
	}
	want := map[string]string{filepath.Join(root, "chrome-qa"): "qa", filepath.Join(root, "chrome-web"): "web"}
	if got := ProfileDirs(root); !maps.Equal(got, want) {
		t.Errorf("ProfileDirs = %v, want %v", got, want)
	}
}

// TestHelperProcess stands in for a Chrome opened outside the tracker: it
// sleeps with --user-data-dir in its command line.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("CHROME_TEST_HELPER") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func TestAdoptRunningChrome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the search on Windows looks for chrome.exe")
	}
	root := t.TempDir()
	target := env.Target{Name: "web", ChromeProfile: "qa"}
	dir := ProfileDir(target, root)
	writeProfile(t, dir)

	helper := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "--", "--user-data-dir="+dir)
	helper.Env = append(os.Environ(), "CHROME_TEST_HELPER=1")
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	// Reaped here, as Chrome would be by whoever started it
	exited := make(chan struct{})
	go func() { helper.Wait(); close(exited) }()
	defer helper.Process.Kill()

	changes := make(chan []string, 4)
	var tr *Tracker
	tr = NewTracker(func() { changes <- tr.Open() })
	tr.Adopt(ProfileDirs(root))
	if got := <-changes; !slices.Equal(got, []string{"qa"}) {
		t.Fatalf("open after Adopt = %v", got)
	}

	// Clear finds it, closes it and deletes the folder
	closed, err := tr.Clear(target, root)
	if err != nil || !closed {
		t.Fatalf("Clear = %v, %v", closed, err)
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("the running Chrome was not closed")
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("profile folder still there: %v", err)
	}
}

func TestCloseFindsChromeNotTracked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the search on Windows looks for chrome.exe")
	}
	root := t.TempDir()
	target := env.Target{Name: "web"}
	helper := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "--", "--user-data-dir="+ProfileDir(target, root))
	helper.Env = append(os.Environ(), "CHROME_TEST_HELPER=1")
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { helper.Wait(); close(exited) }()
	defer helper.Process.Kill()

	if closed, err := NewTracker(nil).Close(target, root); err != nil || !closed {
		t.Fatalf("Close = %v, %v", closed, err)
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("the running Chrome was not closed")
	}
}
