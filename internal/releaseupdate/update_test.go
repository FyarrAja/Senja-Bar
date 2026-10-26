package releaseupdate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixture(t *testing.T, r Release, exe, checksum string) *http.Client {
	t.Helper()
	data, _ := json.Marshal(r)
	return &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/repos/" + Repository + "/releases/latest":
			body = string(data)
		case "/" + Repository + "/releases/download/v1.2.0/" + Installer:
			body = exe
		case "/" + Repository + "/releases/download/v1.2.0/SHA256SUMS.txt":
			body = checksum
		default:
			t.Errorf("unexpected request %s", req.URL)
			return nil, fmt.Errorf("unexpected URL")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
}
func stable() Release {
	base := "https://github.com/" + Repository + "/releases/download/v1.2.0/"
	return Release{Tag: "v1.2.0", Assets: []Asset{{Installer, base + Installer}, {"SHA256SUMS.txt", base + "SHA256SUMS.txt"}}}
}
func TestVerifiedInstallerAndNoDowngrade(t *testing.T) {
	exe := "MZ-test-installer"
	sum := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(exe)), Installer)
	for _, current := range []string{"1.1.2", "1.2.0", "1.3.0"} {
		t.Run(current, func(t *testing.T) {
			dir := t.TempDir()
			var phases []int
			path, err := Prepare(context.Background(), fixture(t, stable(), exe, sum), current, dir, func(p int) { phases = append(phases, p) })
			if err != nil {
				t.Fatal(err)
			}
			if current == "1.1.2" {
				b, e := os.ReadFile(path)
				if e != nil || string(b) != exe {
					t.Fatalf("installer %q: %v", b, e)
				}
				if phases[len(phases)-1] != Downloading {
					t.Fatal(phases)
				}
			} else {
				if path != "" || phases[len(phases)-1] != Current {
					t.Fatalf("must not downgrade/reinstall: %q %v", path, phases)
				}
			}
		})
	}
}
func TestRejectsUnverifiedAndUntrustedRelease(t *testing.T) {
	for _, kind := range []string{"hash", "missing", "foreign", "prerelease", "draft", "badversion"} {
		t.Run(kind, func(t *testing.T) {
			r := stable()
			sum := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte("different")), Installer)
			switch kind {
			case "missing":
				r.Assets = nil
			case "foreign":
				r.Assets[0].URL = "https://example.com/evil.exe"
			case "prerelease":
				r.Prerelease = true
			case "draft":
				r.Draft = true
			case "badversion":
				r.Tag = "../../bad"
			}
			dir := t.TempDir()
			path, err := Prepare(context.Background(), fixture(t, r, "MZ-test", sum), "1.1.2", dir, func(int) {})
			if err == nil || path != "" {
				t.Fatalf("accepted invalid release: %s %v", path, err)
			}
			files, _ := filepath.Glob(filepath.Join(dir, "*"))
			if len(files) != 0 {
				t.Fatalf("left failed downloads: %v", files)
			}
		})
	}
}
func TestHTTPFailureAndDownloadLimit(t *testing.T) {
	for _, code := range []int{403, 404, 500} {
		client := &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("no"))}, nil
		})}
		if _, err := Prepare(context.Background(), client, "1.1.2", t.TempDir(), func(int) {}); err == nil {
			t.Fatalf("accepted HTTP %d", code)
		}
	}
	client := &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("toolong")), ContentLength: -1}, nil
	})}
	if err := get(context.Background(), client, "https://github.com", 3, io.Discard); err == nil {
		t.Fatal("accepted oversized body")
	}
}
func TestDownloadRedirectPolicy(t *testing.T) {
	for _, raw := range []string{"http://github.com/asset", "https://example.com/asset", "https://github.com.evil.example/asset"} {
		req, _ := http.NewRequest(http.MethodGet, raw, nil)
		if err := Client().CheckRedirect(req, nil); err == nil {
			t.Fatalf("accepted redirect %s", raw)
		}
	}
}

func TestCheckOnlyRequestsReleaseMetadata(t *testing.T) {
	data, _ := json.Marshal(stable())
	for _, current := range []string{"1.1.6", "1.2.0", "1.3.0"} {
		requests := 0
		client := &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
			requests++
			if req.URL.String() != "https://api.github.com/repos/"+Repository+"/releases/latest" {
				t.Fatalf("background check downloaded an asset: %s", req.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
		})}
		r, newer, err := Check(context.Background(), client, current)
		if err != nil || requests != 1 || r.Tag != "v1.2.0" || newer != (current == "1.1.6") {
			t.Fatalf("%s: %+v %v %v requests=%d", current, r, newer, err, requests)
		}
	}
}

func TestDownloadProgressTracksWrittenBytes(t *testing.T) {
	for _, length := range []int64{10, -1} {
		client := &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, ContentLength: length, Body: io.NopCloser(strings.NewReader("0123456789"))}, nil
		})}
		var last, total int64
		calls := 0
		err := getProgress(context.Background(), client, "https://github.com/test", 100, io.Discard, func(done, size int64) {
			if done < last || done > 10 {
				t.Fatalf("invalid byte progress %d after %d", done, last)
			}
			last, total = done, size
			calls++
		})
		if err != nil || last != 10 || total != max(int64(0), length) || calls < 2 {
			t.Fatalf("%v bytes=%d total=%d calls=%d", err, last, total, calls)
		}
	}
}

func TestDownloadPublishesResponseSizeBeforeBody(t *testing.T) {
	client := &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: 10, Body: io.NopCloser(strings.NewReader("0123456789"))}, nil
	})}
	first := true
	err := getProgress(context.Background(), client, "https://github.com/test", 100, io.Discard, func(done, total int64) {
		if first {
			if done != 0 || total != 10 {
				t.Fatalf("initial measurement %d/%d", done, total)
			}
			first = false
		}
	})
	if err != nil || first {
		t.Fatal(err, first)
	}
}
func TestFailedDownloadDoesNotReportCompletion(t *testing.T) {
	client := &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: 100, Body: io.NopCloser(strings.NewReader("partial"))}, nil
	})}
	var last int64
	err := getProgress(context.Background(), client, "https://github.com/test", 5, io.Discard, func(done, total int64) { last = done })
	if err == nil || last == 100 {
		t.Fatalf("invalid successful progress %d: %v", last, err)
	}
}
