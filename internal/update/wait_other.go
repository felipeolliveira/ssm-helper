//go:build !windows

package update

import (
	"syscall"
	"time"
)

// WaitForExit waits up to timeout for the process pid to end.
func WaitForExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
}
