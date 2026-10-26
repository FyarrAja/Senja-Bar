// Package releaseupdate downloads verified Senja release installers on demand.
package releaseupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const Repository = "FyarrAja/Senja-Bar"
const Installer = "Senja-Bar.exe"
const (
	Idle = iota
	Checking
	Downloading
	Installing
	Current
	Failed
	Available
)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}
type Release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

func Version(s string) ([3]int, error) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("invalid version %q", s)
	}
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return v, fmt.Errorf("invalid version %q", s)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, err
		}
		v[i] = n
	}
	return v, nil
}

func Newer(next, current string) (bool, error) {
	a, err := Version(next)
	if err != nil {
		return false, err
	}
	b, err := Version(current)
	if err != nil {
		return false, err
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i], nil
		}
	}
	return false, nil
}

func trustedAsset(raw, tag, name string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "/"+Repository+"/releases/download/"+tag+"/"+name
}

func Client() *http.Client {
	return &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return fmt.Errorf("too many redirects")
		}
		h := req.URL.Hostname()
		if req.URL.Scheme != "https" || req.URL.User != nil || (h != "github.com" && h != "release-assets.githubusercontent.com" && h != "objects.githubusercontent.com") {
			return fmt.Errorf("untrusted download redirect")
		}
		return nil
	}}
}

func get(ctx context.Context, client *http.Client, raw string, max int64, out io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Senja-Bar-Updater")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", res.StatusCode)
	}
	if res.ContentLength > max {
		return fmt.Errorf("download exceeds size limit")
	}
	n, err := io.Copy(out, io.LimitReader(res.Body, max+1))
	if err != nil {
		return err
	}
	if n > max {
		return fmt.Errorf("download exceeds size limit")
	}
	return nil
}

// Check fetches release metadata only. It never downloads or runs an installer.
func Check(ctx context.Context, client *http.Client, current string) (Release, bool, error) {
	var data strings.Builder
	if err := get(ctx, client, "https://api.github.com/repos/"+Repository+"/releases/latest", 1<<20, &data); err != nil {
		return Release{}, false, err
	}
	var release Release
	if err := json.Unmarshal([]byte(data.String()), &release); err != nil {
		return release, false, err
	}
	if release.Draft || release.Prerelease {
		return release, false, fmt.Errorf("release is not stable")
	}
	newer, err := Newer(release.Tag, current)
	if err != nil {
		return release, false, err
	}
	if newer {
		if _, _, err = releaseURLs(release); err != nil {
			return release, false, err
		}
	}
	return release, newer, nil
}
func releaseURLs(release Release) (string, string, error) {
	var exeURL, sumURL string
	for _, a := range release.Assets {
		switch a.Name {
		case Installer:
			if !trustedAsset(a.URL, release.Tag, Installer) {
				return "", "", fmt.Errorf("untrusted installer URL")
			}
			if exeURL != "" {
				return "", "", fmt.Errorf("duplicate installer")
			}
			exeURL = a.URL
		case "SHA256SUMS.txt":
			if !trustedAsset(a.URL, release.Tag, a.Name) {
				return "", "", fmt.Errorf("untrusted checksum URL")
			}
			if sumURL != "" {
				return "", "", fmt.Errorf("duplicate checksums")
			}
			sumURL = a.URL
		}
	}
	if exeURL == "" || sumURL == "" {
		return "", "", fmt.Errorf("release installer/checksums unavailable")
	}
	return exeURL, sumURL, nil
}

// Prepare never executes a download. Only a newer stable release with matching
// checksums is returned; the caller controls the final installer handoff.
func Prepare(ctx context.Context, client *http.Client, current, dir string, report func(int)) (path string, err error) {
	report(Checking)
	release, newer, err := Check(ctx, client, current)
	if err != nil {
		return "", err
	}
	if !newer {
		report(Current)
		return "", nil
	}
	exeURL, sumURL, err := releaseURLs(release)
	if err != nil {
		return "", err
	}
	report(Downloading)
	var data strings.Builder
	if err = get(ctx, client, sumURL, 64<<10, &data); err != nil {
		return "", err
	}
	var expected []byte
	for _, line := range strings.Split(data.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == Installer {
			if expected != nil {
				return "", fmt.Errorf("duplicate installer checksum")
			}
			expected, err = hex.DecodeString(fields[0])
			if err != nil || len(expected) != sha256.Size {
				return "", fmt.Errorf("invalid installer checksum")
			}
		}
	}
	if len(expected) != sha256.Size {
		return "", fmt.Errorf("installer checksum missing")
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, ".download-*.tmp")
	if err != nil {
		return "", err
	}
	temp := file.Name()
	defer func() { file.Close(); os.Remove(temp) }()
	hash := sha256.New()
	if err = get(ctx, client, exeURL, 64<<20, io.MultiWriter(file, hash)); err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	if hex.EncodeToString(hash.Sum(nil)) != hex.EncodeToString(expected) {
		return "", fmt.Errorf("installer checksum mismatch")
	}
	// A fresh private subdirectory avoids collisions with an installer already
	// executing; failed downloads are removed without touching the running app.
	stage, err := os.MkdirTemp(dir, release.Tag+"-")
	if err != nil {
		return "", err
	}
	path = filepath.Join(stage, Installer)
	if err = os.Rename(temp, path); err != nil {
		os.Remove(stage)
		return "", err
	}
	return path, nil
}
