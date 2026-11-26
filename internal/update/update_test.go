package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		current, tag string
		want         bool
	}{
		{"1.3.0", "v1.3.0", false},
		{"1.3.0", "v1.3.1", true},
		{"1.3.0", "v1.4.0", true},
		{"1.3.9", "v1.10.0", true},
		{"1.3.0", "v2.0.0", true},
		{"1.4.0", "v1.3.9", false},
		{"1.3.0", "latest", false},
		{"dev", "v9.9.9", false},
		{"1.3", "v1.4.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.current, c.tag); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.tag, got, c.want)
		}
	}
}

// fakeGitHub serves a release with the given executable and checksum file.
func fakeGitHub(t *testing.T, exe []byte, sums string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/owner/app/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent")
		}
		fmt.Fprintf(w, `{"tag_name": "v1.4.0", "html_url": "https://example/r/v1.4.0", "assets": [
			{"name": "ssm-helper.exe", "browser_download_url": "%[1]s/dl/ssm-helper.exe"},
			{"name": "SHA256SUMS.txt", "browser_download_url": "%[1]s/dl/SHA256SUMS.txt"}]}`, srv.URL)
	})
	mux.HandleFunc("/dl/ssm-helper.exe", func(w http.ResponseWriter, r *http.Request) { w.Write(exe) })
	mux.HandleFunc("/dl/SHA256SUMS.txt", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, sums) })
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	defer func(base string) { t.Cleanup(func() { APIBase = base }) }(APIBase)
	APIBase = srv.URL
	return srv
}

func sumOf(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestLatestAndDownload(t *testing.T) {
	exe := []byte("new executable")
	fakeGitHub(t, exe, sumOf(exe)+"  ssm-helper.exe\n")

	r, err := Latest(context.Background(), "owner/app")
	if err != nil {
		t.Fatal(err)
	}
	if r.Tag != "v1.4.0" || r.URL != "https://example/r/v1.4.0" || !r.Ready() {
		t.Fatalf("Latest = %+v", r)
	}

	path := filepath.Join(t.TempDir(), "ssm-helper.exe.new")
	var last int64
	if err := Download(context.Background(), r, path, func(done, total int64) { last = done }); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != string(exe) {
		t.Errorf("downloaded %q", data)
	}
	if last != int64(len(exe)) {
		t.Errorf("last progress = %d, want %d", last, len(exe))
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	fakeGitHub(t, []byte("tampered"), sumOf([]byte("original"))+" *ssm-helper.exe\n")
	r, err := Latest(context.Background(), "owner/app")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ssm-helper.exe.new")
	err = Download(context.Background(), r, path, nil)
	if !errors.Is(err, msg.Err(msg.UpdateChecksumMismatch)) {
		t.Fatalf("Download = %v, want %s", err, msg.UpdateChecksumMismatch)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("the bad download was kept")
	}
}

func TestDownloadWithoutChecksum(t *testing.T) {
	fakeGitHub(t, []byte("x"), sumOf([]byte("x"))+"  other.exe\n")
	r, _ := Latest(context.Background(), "owner/app")
	err := Download(context.Background(), r, filepath.Join(t.TempDir(), "new"), nil)
	if !errors.Is(err, msg.Err(msg.UpdateDownloadFailed)) {
		t.Fatalf("Download = %v, want %s", err, msg.UpdateDownloadFailed)
	}
}

func TestLatestErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer srv.Close()
	defer func(base string) { APIBase = base }(APIBase)
	APIBase = srv.URL
	if _, err := Latest(context.Background(), "owner/app"); !errors.Is(err, msg.Err(msg.UpdateCheckFailed)) {
		t.Fatalf("Latest = %v, want %s", err, msg.UpdateCheckFailed)
	}
}

func TestSwap(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "ssm-helper.exe")
	newPath := exe + ".new"
	os.WriteFile(exe, []byte("v1"), 0o755)
	os.WriteFile(newPath, []byte("v2"), 0o755)
	os.WriteFile(exe+OldSuffix, []byte("v0"), 0o755) // left by an earlier update

	if err := Swap(exe, newPath); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(exe); string(data) != "v2" {
		t.Errorf("exe = %q", data)
	}
	if data, _ := os.ReadFile(exe + OldSuffix); string(data) != "v1" {
		t.Errorf("old = %q", data)
	}
	RemoveOld(exe)
	if _, err := os.Stat(exe + OldSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Error("old executable not removed")
	}
}

func TestSwapUndoes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on renaming onto a missing file")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "ssm-helper.exe")
	os.WriteFile(exe, []byte("v1"), 0o755)
	// The new file is missing, so the second rename fails
	err := Swap(exe, filepath.Join(dir, "missing.new"))
	if !errors.Is(err, msg.Err(msg.UpdateApplyFailed)) {
		t.Fatalf("Swap = %v, want %s", err, msg.UpdateApplyFailed)
	}
	if data, _ := os.ReadFile(exe); string(data) != "v1" {
		t.Errorf("exe after a failed swap = %q", data)
	}
}
