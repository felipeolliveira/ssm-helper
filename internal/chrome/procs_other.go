//go:build !windows

package chrome

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const pathsIgnoreCase = false

// findProfileProcess looks in /proc for the process whose command line uses
// dir. Outside Windows this only serves development and tests.
func findProfileProcess(dir string) (int, bool) {
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}
		if isBrowserFor(strings.Split(strings.TrimRight(string(data), "\x00"), "\x00"), dir) {
			return pid, true
		}
	}
	return 0, false
}

// watchPID returns a function that waits for pid to exit, checking every second.
func watchPID(pid int) (func(), bool) {
	if syscall.Kill(pid, 0) != nil {
		return nil, false
	}
	return func() {
		for syscall.Kill(pid, 0) == nil && !zombie(pid) {
			time.Sleep(time.Second)
		}
	}, true
}

// zombie reports whether pid exited but was not reaped by its parent yet.
func zombie(pid int) bool {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	_, after, ok := strings.Cut(string(data), ") ")
	return ok && strings.HasPrefix(after, "Z")
}
