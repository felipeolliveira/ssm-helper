package chrome

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const pathsIgnoreCase = true

// findProfileProcess looks for the chrome.exe whose command line uses dir.
func findProfileProcess(dir string) (int, bool) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "chrome.exe") {
			continue
		}
		line, ok := commandLine(entry.ProcessID)
		if !ok {
			continue
		}
		if args, err := windows.DecomposeCommandLine(line); err == nil && isBrowserFor(args, dir) {
			return int(entry.ProcessID), true
		}
	}
	return 0, false
}

// commandLine reads the command line of another process of the same user.
func commandLine(pid uint32) (string, bool) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(process)
	size := uint32(4096)
	for range 3 {
		buf := make([]byte, size)
		err := windows.NtQueryInformationProcess(process, windows.ProcessCommandLineInformation,
			unsafe.Pointer(&buf[0]), size, &size)
		if err == nil {
			// The buffer starts with a UNICODE_STRING pointing into itself
			return (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0])).String(), true
		}
		if err != windows.STATUS_INFO_LENGTH_MISMATCH && err != windows.STATUS_BUFFER_TOO_SMALL {
			return "", false
		}
	}
	return "", false
}

// watchPID returns a function that waits for pid to exit. It holds a handle
// to the process, so a pid reused by another program is never waited on.
func watchPID(pid int) (func(), bool) {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil, false
	}
	return func() {
		windows.WaitForSingleObject(process, windows.INFINITE)
		windows.CloseHandle(process)
	}, true
}
