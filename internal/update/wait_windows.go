package update

import (
	"time"

	"golang.org/x/sys/windows"
)

// WaitForExit waits up to timeout for the process pid to end.
func WaitForExit(pid int, timeout time.Duration) {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return // already gone
	}
	defer windows.CloseHandle(process)
	windows.WaitForSingleObject(process, uint32(timeout.Milliseconds()))
}
