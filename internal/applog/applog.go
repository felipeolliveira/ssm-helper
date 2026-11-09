// Package applog writes the messages of the log panel to one text file per
// day (ssm-helper-YYYY-MM-DD.log), so they survive the app closing.
package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

// File appends log lines to the file of the current day in Dir. It is safe
// for concurrent use.
type File struct {
	Dir string

	mu   sync.Mutex
	day  string
	file *os.File
}

func New(dir string) *File { return &File{Dir: dir} }

// FileName is the log file of the day of t.
func FileName(t time.Time) string {
	return "ssm-helper-" + t.Format(time.DateOnly) + ".log"
}

// Write appends one line; it opens the file of a new day when the date changes.
func (f *File) Write(t time.Time, source string, m msg.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	day := t.Format(time.DateOnly)
	if f.file == nil || f.day != day {
		if f.file != nil {
			f.file.Close()
			f.file = nil
		}
		if err := os.MkdirAll(f.Dir, 0o700); err != nil {
			return err
		}
		file, err := os.OpenFile(filepath.Join(f.Dir, FileName(t)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		f.file, f.day = file, day
	}
	_, err := f.file.WriteString(Format(t, source, m) + "\n")
	return err
}

// Close closes the open file; a later Write opens it again.
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

// Format renders a message as one line: the code and its parameters (the
// texts are translated only in the frontend) followed by the detail.
func Format(t time.Time, source string, m msg.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s] %s", t.Format("2006-01-02 15:04:05"), source, m.Code)
	keys := make([]string, 0, len(m.Params))
	for k := range m.Params {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, " %s=%q", k, m.Params[k])
	}
	if m.Detail != "" {
		// Keep one entry per line
		b.WriteString(" | " + strings.ReplaceAll(strings.TrimRight(m.Detail, "\r\n"), "\n", " ⏎ "))
	}
	return b.String()
}
