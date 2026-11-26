//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

func downloadsDir(fallback string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if info, err := os.Stat(filepath.Join(home, "Downloads")); err == nil && info.IsDir() {
			return filepath.Join(home, "Downloads")
		}
	}
	return fallback
}

func revealFile(path string) { _ = openFolder(filepath.Dir(path)) }
