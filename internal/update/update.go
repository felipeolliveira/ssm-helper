// Package update finds a newer release of the app on GitHub, downloads its
// executable, checks it against the release's SHA256SUMS.txt, whose
// signature must match a public key built into the app, and puts it in place
// of the running one.
package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

// Names of the files the release workflow attaches to each release.
const (
	ExeName  = "ssm-helper.exe"
	SumsName = "SHA256SUMS.txt"
)

// APIBase is the GitHub API; tests point it to a local server.
var APIBase = "https://api.github.com"

// Client has a timeout for the release lookup. Downloads use their own
// context. Proxies come from HTTP(S)_PROXY, as in http.DefaultTransport.
var Client = &http.Client{Timeout: 15 * time.Second}

// downloadClient only follows redirects to the hosts that serve release files.
var downloadClient = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("too many redirects")
	}
	return checkAssetURL(req.URL)
}}

// assetHosts serve the files attached to GitHub releases: github.com
// redirects to one of the others. Tests add their local server.
var assetHosts = map[string]bool{
	"github.com":                           true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
}

// allowHTTP lets tests use a plain HTTP server.
var allowHTTP = false

// maxExeSize caps the download; the executable is about 25 MB.
const maxExeSize = 200 << 20

func checkAssetURL(u *url.URL) error {
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
		return fmt.Errorf("refusing %s: not HTTPS", u.Redacted())
	}
	if !assetHosts[u.Hostname()] {
		return fmt.Errorf("refusing %s: not a GitHub release host", u.Redacted())
	}
	return nil
}

// Release is the latest published release (drafts and prereleases are skipped by GitHub).
type Release struct {
	Tag     string `json:"tag"`
	URL     string `json:"url"`
	ExeURL  string `json:"-"`
	SumsURL string `json:"-"`
	SigURL  string `json:"-"`
}

// Latest returns the latest release of repo ("owner/name").
func Latest(ctx context.Context, repo string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, APIBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ssm-helper")
	resp, err := Client.Do(req)
	if err != nil {
		return Release{}, msg.Wrap(msg.UpdateCheckFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, msg.Wrap(msg.UpdateCheckFailed, fmt.Errorf("GitHub answered %s", resp.Status))
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return Release{}, msg.Wrap(msg.UpdateCheckFailed, err)
	}
	r := Release{Tag: body.TagName, URL: body.HTMLURL}
	for _, a := range body.Assets {
		switch a.Name {
		case ExeName:
			r.ExeURL = a.URL
		case SumsName:
			r.SumsURL = a.URL
		case SigName:
			r.SigURL = a.URL
		}
	}
	if _, ok := parseVersion(r.Tag); !ok {
		return Release{}, msg.Wrap(msg.UpdateCheckFailed, fmt.Errorf("unexpected tag %q", r.Tag))
	}
	return r, nil
}

// Ready tells whether the release has the files an update needs; the
// release workflow attaches them a few minutes after the release is
// published. Releases from before signing have no signature and never are.
func (r Release) Ready() bool { return r.ExeURL != "" && r.SumsURL != "" && r.SigURL != "" }

// keys are the public keys that verify releases; tests replace them.
var keys = PublicKeys

// VerifiedSums downloads SHA256SUMS.txt and its signature and returns the
// checksums only when the signature matches a key built into the app.
func VerifiedSums(ctx context.Context, r Release) ([]byte, error) {
	trusted, err := keys()
	if err != nil {
		return nil, msg.Wrap(msg.UpdateSignatureInvalid, err, "version", r.Tag)
	}
	sums, err := fetch(ctx, r.SumsURL, 64<<10)
	if err != nil {
		return nil, msg.Wrap(msg.UpdateDownloadFailed, err)
	}
	sig, err := fetch(ctx, r.SigURL, 4<<10)
	if err != nil {
		return nil, msg.Wrap(msg.UpdateDownloadFailed, err)
	}
	if !Verify(trusted, sums, string(sig)) {
		return nil, msg.Wrap(msg.UpdateSignatureInvalid,
			fmt.Errorf("%s does not match the signature in %s", SumsName, SigName), "version", r.Tag)
	}
	return sums, nil
}

// Newer reports whether tag (v1.2.3) is a later version than current (1.2.3).
func Newer(current, tag string) bool {
	c, ok1 := parseVersion(current)
	t, ok2 := parseVersion(tag)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if t[i] != c[i] {
			return t[i] > c[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Download saves the release executable to path after checking it against
// the signed SHA256SUMS.txt. progress receives the bytes written and the
// total (-1 when unknown). On any failure path is removed.
func Download(ctx context.Context, r Release, path string, progress func(done, total int64)) (err error) {
	sums, err := VerifiedSums(ctx, r)
	if err != nil {
		return err
	}
	want, err := checksum(sums, ExeName)
	if err != nil {
		return msg.Wrap(msg.UpdateDownloadFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.ExeURL, nil)
	if err == nil {
		err = checkAssetURL(req.URL)
	}
	if err != nil {
		return msg.Wrap(msg.UpdateDownloadFailed, err)
	}
	req.Header.Set("User-Agent", "ssm-helper")
	resp, err := downloadClient.Do(req)
	if err != nil {
		return msg.Wrap(msg.UpdateDownloadFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return msg.Wrap(msg.UpdateDownloadFailed, fmt.Errorf("GitHub answered %s", resp.Status))
	}
	if resp.ContentLength > maxExeSize {
		return msg.Wrap(msg.UpdateDownloadFailed, fmt.Errorf("the executable has %d bytes, more than the %d allowed", resp.ContentLength, maxExeSize))
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return msg.Wrap(msg.UpdateDownloadFailed, err)
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = msg.Wrap(msg.UpdateDownloadFailed, cerr)
		}
		if err != nil {
			os.Remove(path)
		}
	}()
	hash := sha256.New()
	w := &progressWriter{w: io.MultiWriter(f, hash), total: resp.ContentLength, report: progress}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxExeSize+1))
	if err != nil {
		return msg.Wrap(msg.UpdateDownloadFailed, err)
	}
	if n > maxExeSize {
		return msg.Wrap(msg.UpdateDownloadFailed, fmt.Errorf("the executable is larger than the %d bytes allowed", maxExeSize))
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != want {
		return msg.Wrap(msg.UpdateChecksumMismatch, fmt.Errorf("expected %s, got %s", want, got))
	}
	return nil
}

func fetch(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err == nil {
		err = checkAssetURL(req.URL)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ssm-helper")
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// checksum finds the SHA-256 of name in a sha256sum output.
func checksum(sums []byte, name string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name && len(fields[0]) == sha256.Size*2 {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s has no checksum for %s", SumsName, name)
}

type progressWriter struct {
	w      io.Writer
	done   int64
	total  int64
	last   time.Time
	report func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	// At most a few events per second
	if p.report != nil && (time.Since(p.last) > 200*time.Millisecond || p.done == p.total) {
		p.last = time.Now()
		p.report(p.done, p.total)
	}
	return n, err
}

// OldSuffix marks the executable replaced by an update; the new one removes it.
const OldSuffix = ".old"

// Swap puts newPath in place of exe and keeps the running one as exe+".old":
// Windows lets a running executable be renamed, not overwritten. When the
// second rename fails, the first is undone.
func Swap(exe, newPath string) error {
	old := exe + OldSuffix
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return msg.Wrap(msg.UpdateApplyFailed, err)
	}
	if err := os.Rename(newPath, exe); err != nil {
		if undo := os.Rename(old, exe); undo != nil {
			err = errors.Join(err, undo)
		}
		return msg.Wrap(msg.UpdateApplyFailed, err)
	}
	return nil
}

// RemoveOld deletes the executable an update left behind. Right after an
// update it may still be locked for a moment, so it tries for a while.
func RemoveOld(exe string) {
	old := exe + OldSuffix
	for range 20 {
		if err := os.Remove(old); err == nil || errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}
