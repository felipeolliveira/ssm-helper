package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// testKey signs the fake releases; the package trusts it during the tests.
var testKey = func() ed25519.PrivateKey {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	return key
}()

func trustTestKey(t *testing.T) {
	t.Helper()
	saved := keys
	keys = func() ([]ed25519.PublicKey, error) {
		return []ed25519.PublicKey{testKey.Public().(ed25519.PublicKey)}, nil
	}
	t.Cleanup(func() { keys = saved })
}

// fakeGitHub serves a release with the given executable and checksum file,
// signed with testKey.
func fakeGitHub(t *testing.T, exe []byte, sums string) *httptest.Server {
	t.Helper()
	return fakeGitHubSigned(t, exe, sums, Sign(testKey, []byte(sums)))
}

// fakeGitHubSigned serves the release with the given signature; an empty one
// leaves SHA256SUMS.txt.sig out of the release.
func fakeGitHubSigned(t *testing.T, exe []byte, sums, sig string) *httptest.Server {
	t.Helper()
	trustTestKey(t)
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/owner/app/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent")
		}
		sigAsset := ""
		if sig != "" {
			sigAsset = fmt.Sprintf(`, {"name": "SHA256SUMS.txt.sig", "browser_download_url": "%s/dl/SHA256SUMS.txt.sig"}`, srv.URL)
		}
		fmt.Fprintf(w, `{"tag_name": "v1.4.0", "html_url": "https://example/r/v1.4.0", "assets": [
			{"name": "ssm-helper.exe", "browser_download_url": "%[1]s/dl/ssm-helper.exe"},
			{"name": "SHA256SUMS.txt", "browser_download_url": "%[1]s/dl/SHA256SUMS.txt"}%[2]s]}`, srv.URL, sigAsset)
	})
	mux.HandleFunc("/dl/SHA256SUMS.txt.sig", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, sig) })
	mux.HandleFunc("/dl/ssm-helper.exe", func(w http.ResponseWriter, r *http.Request) { w.Write(exe) })
	mux.HandleFunc("/dl/SHA256SUMS.txt", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, sums) })
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	allowLocal(t, srv)
	return srv
}

// allowLocal points the package to a local test server.
func allowLocal(t *testing.T, srv *httptest.Server) {
	t.Helper()
	base, host := APIBase, httptestHost(srv)
	APIBase, allowHTTP, assetHosts[host] = srv.URL, true, true
	t.Cleanup(func() {
		APIBase, allowHTTP = base, false
		delete(assetHosts, host)
	})
}

func httptestHost(srv *httptest.Server) string {
	u, _ := url.Parse(srv.URL)
	return u.Hostname()
}

func TestDownloadRefusesOtherHosts(t *testing.T) {
	exe := []byte("x")
	fakeGitHub(t, exe, sumOf(exe)+"  ssm-helper.exe\n")
	r, err := Latest(context.Background(), "owner/app")
	if err != nil {
		t.Fatal(err)
	}
	for name, u := range map[string]string{
		"plain http":   "http://github.com/x/ssm-helper.exe",
		"another host": "https://evil.example/ssm-helper.exe",
	} {
		allowHTTP = false
		bad := r
		bad.ExeURL = u
		if err := Download(context.Background(), bad, filepath.Join(t.TempDir(), "new"), nil); !errors.Is(err, msg.Err(msg.UpdateDownloadFailed)) {
			t.Errorf("%s: Download = %v, want %s", name, err, msg.UpdateDownloadFailed)
		}
		allowHTTP = true
	}

	// A redirect to another host is refused too
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(exe) }))
	defer other.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+"/exe", http.StatusFound)
	}))
	defer redirect.Close()
	bad := r
	bad.ExeURL = redirect.URL + "/exe" // same host as the allowed server (127.0.0.1)
	if err := Download(context.Background(), bad, filepath.Join(t.TempDir(), "new"), nil); !errors.Is(err, msg.Err(msg.UpdateDownloadFailed)) {
		t.Errorf("redirect: Download = %v, want %s", err, msg.UpdateDownloadFailed)
	}
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
	allowLocal(t, srv)
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

func TestSignatureRejected(t *testing.T) {
	exe := []byte("tampered executable")
	sums := sumOf(exe) + "  ssm-helper.exe\n"
	// Signed with a key the app does not know, as after the release files were replaced
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	fakeGitHubSigned(t, exe, sums, Sign(other, []byte(sums)))
	r, err := Latest(context.Background(), "owner/app")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifiedSums(context.Background(), r); !errors.Is(err, msg.Err(msg.UpdateSignatureInvalid)) {
		t.Fatalf("VerifiedSums = %v, want %s", err, msg.UpdateSignatureInvalid)
	}
	path := filepath.Join(t.TempDir(), "new")
	err = Download(context.Background(), r, path, nil)
	if !errors.Is(err, msg.Err(msg.UpdateSignatureInvalid)) || msg.From(err).Params["version"] != "v1.4.0" {
		t.Fatalf("Download = %v, want %s for v1.4.0", err, msg.UpdateSignatureInvalid)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("the executable was downloaded despite the bad signature")
	}
}

func TestUnsignedReleaseNotReady(t *testing.T) {
	exe := []byte("x")
	fakeGitHubSigned(t, exe, sumOf(exe)+"  ssm-helper.exe\n", "")
	r, err := Latest(context.Background(), "owner/app")
	if err != nil {
		t.Fatal(err)
	}
	if r.Ready() {
		t.Error("a release without SHA256SUMS.txt.sig is ready")
	}
}

func TestSignVerify(t *testing.T) {
	primaryPub, primary, _ := ed25519.GenerateKey(rand.Reader)
	backupPub, backup, _ := ed25519.GenerateKey(rand.Reader)
	trusted := []ed25519.PublicKey{primaryPub, backupPub}
	sums := []byte("abc  ssm-helper.exe\n")

	for name, key := range map[string]ed25519.PrivateKey{"primary": primary, "backup": backup} {
		if !Verify(trusted, sums, Sign(key, sums)+"\n") {
			t.Errorf("signature of the %s key rejected", name)
		}
	}
	if Verify(trusted, []byte("changed"), Sign(primary, sums)) {
		t.Error("signature accepted for other content")
	}
	// A plain ed25519 signature of the file, without the context, is not accepted
	if Verify(trusted, sums, base64.StdEncoding.EncodeToString(ed25519.Sign(primary, sums))) {
		t.Error("signature without the context accepted")
	}
	if Verify(trusted, sums, "not base64!") {
		t.Error("garbage accepted")
	}

	parsed, err := ParsePrivateKey(base64.StdEncoding.EncodeToString(primary.Seed()) + "\n")
	if err != nil || !parsed.Equal(primary) {
		t.Errorf("ParsePrivateKey = %v", err)
	}
	if _, err := ParsePublicKey("c2hvcnQ="); err == nil {
		t.Error("a short public key was accepted")
	}
}

// TestPublicKeysSet fails until keys.go has the primary and backup public
// keys, so no release is built without them.
func TestPublicKeysSet(t *testing.T) {
	for i, s := range publicKeys {
		if _, err := ParsePublicKey(s); err != nil {
			t.Errorf("public key %d in keys.go: %v (generate it with: go run ./tools/updatekey generate -out <name>)", i+1, err)
		}
	}
	if len(publicKeys) != 2 {
		t.Errorf("keys.go has %d public keys, want 2 (primary and backup)", len(publicKeys))
	}
}
