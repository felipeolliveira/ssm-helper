package chrome

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const appPathsKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`

// findChrome looks in App Paths (per user, then machine) and the default install folders.
func findChrome() (string, error) {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, appPathsKey, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		path, _, err := key.GetStringValue("")
		key.Close()
		if err == nil && fileExists(path) {
			return path, nil
		}
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if base := os.Getenv(env); base != "" {
			path := filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe")
			if fileExists(path) {
				return path, nil
			}
		}
	}
	return "", ErrNotFound
}
