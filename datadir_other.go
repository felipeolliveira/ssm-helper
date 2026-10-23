//go:build !windows

package main

import "os"

// protectDir limits a folder to the current user.
func protectDir(path string) error {
	return os.Chmod(path, 0o700)
}
