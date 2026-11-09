package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

func TestFormat(t *testing.T) {
	at := time.Date(2026, 11, 9, 14, 3, 7, 0, time.Local)
	got := Format(at, "web", msg.Message{
		Code:   "TUNNEL_ACTIVE",
		Params: map[string]string{"port": "8080", "b": "x y"},
		Detail: "line one\nline two\n",
	})
	want := `2026-11-09 14:03:07 [web] TUNNEL_ACTIVE b="x y" port="8080" | line one ⏎ line two`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestWriteOneFilePerDay(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	f := New(dir)
	defer f.Close()

	day1 := time.Date(2026, 11, 9, 23, 59, 0, 0, time.Local)
	day2 := day1.Add(2 * time.Minute)
	for _, w := range []struct {
		at   time.Time
		code string
	}{{day1, "A"}, {day1, "B"}, {day2, "C"}} {
		if err := f.Write(w.at, "src", msg.Message{Code: w.code}); err != nil {
			t.Fatal(err)
		}
	}

	read := func(at time.Time) string {
		data, err := os.ReadFile(filepath.Join(dir, FileName(at)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if got := read(day1); strings.Count(got, "\n") != 2 || !strings.Contains(got, "] A") || !strings.Contains(got, "] B") {
		t.Fatalf("day 1: %q", got)
	}
	if got := read(day2); strings.Count(got, "\n") != 1 || !strings.Contains(got, "] C") {
		t.Fatalf("day 2: %q", got)
	}

	// Appends after reopening
	f.Close()
	if err := f.Write(day2, "src", msg.Message{Code: "D"}); err != nil {
		t.Fatal(err)
	}
	if got := read(day2); strings.Count(got, "\n") != 2 {
		t.Fatalf("day 2 after reopen: %q", got)
	}
}
