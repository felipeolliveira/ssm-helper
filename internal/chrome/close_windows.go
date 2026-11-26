package chrome

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// closeProcess asks Chrome to close its windows, as clicking X would.
func closeProcess(p *os.Process) {
	taskkill("/PID", strconv.Itoa(p.Pid))
}

// killProcess ends Chrome and its child processes.
func killProcess(p *os.Process) {
	taskkill("/PID", strconv.Itoa(p.Pid), "/T", "/F")
}

func taskkill(args ...string) {
	cmd := exec.Command("taskkill", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	_ = cmd.Run()
}

// inUse reports whether a Chrome holds the profile: it keeps "lockfile" open
// while it runs, so the file cannot be removed. Removing it otherwise is harmless.
func inUse(dir string) bool {
	err := os.Remove(filepath.Join(dir, "lockfile"))
	return err != nil && !errors.Is(err, os.ErrNotExist)
}
