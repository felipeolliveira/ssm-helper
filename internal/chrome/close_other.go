//go:build !windows

package chrome

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func closeProcess(pid int) { _ = syscall.Kill(pid, syscall.SIGTERM) }

func killProcess(pid int) { _ = syscall.Kill(pid, syscall.SIGKILL) }

// inUse reports whether a Chrome holds the profile: while it runs,
// SingletonLock is a link to "<host>-<pid>". A link left by a crash points
// to a process that no longer exists.
func inUse(dir string) bool {
	target, err := os.Readlink(filepath.Join(dir, "SingletonLock"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(target[strings.LastIndex(target, "-")+1:])
	if err != nil {
		return true
	}
	return syscall.Kill(pid, 0) == nil
}
