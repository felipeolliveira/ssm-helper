package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// downloadsDir is the user's Downloads folder, or fallback when it is unknown.
func downloadsDir(fallback string) string {
	if dir, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0); err == nil {
		return dir
	}
	return fallback
}

// revealFile opens Explorer with the file selected. Explorer only accepts the
// path quoted after "/select,", which Go's own quoting would not produce.
func revealFile(path string) {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	_ = cmd.Start()
}
