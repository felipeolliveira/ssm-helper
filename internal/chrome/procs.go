package chrome

import (
	"path/filepath"
	"strings"
)

// isBrowserFor reports whether a command line is the main process of the
// Chrome using dir: renderer, GPU and other child processes carry --type=.
func isBrowserFor(args []string, dir string) bool {
	matches := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "--type=") {
			return false
		}
		if value, ok := strings.CutPrefix(arg, "--user-data-dir="); ok {
			matches = samePath(strings.Trim(value, `"`), dir)
		}
	}
	return matches
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if pathsIgnoreCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}
