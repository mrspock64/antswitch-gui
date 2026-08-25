package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/minio/selfupdate"
)

const (
	updateRepoOwner   = "mrspock64"
	updateRepoName    = "antswitch-gui"
	updateHTTPTimeout = 10
)

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

// fetchLatestRelease queries the GitHub API for this project's latest release.
func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", updateRepoOwner, updateRepoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "antswitch-gui")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: unexpected status %s", resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// updateAssetName returns the release asset name expected for this platform.
func updateAssetName() string {
	name := fmt.Sprintf("antswitch-gui-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func findAsset(rel *githubRelease, name string) *githubAsset {
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			return &rel.Assets[i]
		}
	}
	return nil
}

// isNewerVersion compares two "vX.Y.Z" tags numerically. Anything that
// doesn't parse as a version (e.g. the "dev" build) is treated as older
// than any real release, so local dev builds always report an update.
func isNewerVersion(current, latest string) bool {
	cur := parseVersion(current)
	lat := parseVersion(latest)
	if lat == nil {
		return false
	}
	if cur == nil {
		return true
	}
	for i := 0; i < 3; i++ {
		if lat[i] != cur[i] {
			return lat[i] > cur[i]
		}
	}
	return false
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) == 0 {
		return nil
	}
	out := make([]int, 3)
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

// fetchChecksum downloads the release's checksums.txt (sha256sum -a 256
// format: "<hex>  <filename>" per line) and returns the digest for name.
func fetchChecksum(ctx context.Context, rel *githubRelease, name string) ([]byte, error) {
	asset := findAsset(rel, "checksums.txt")
	if asset == nil {
		return nil, nil // no checksums published; apply without verification
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums.txt: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			return hex.DecodeString(fields[0])
		}
	}
	return nil, nil
}

// downloadAsset fetches the given asset's full contents into memory.
func downloadAsset(ctx context.Context, asset *githubAsset) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: unexpected status %s", asset.Name, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// applyUpdate downloads the platform asset for rel and replaces the running
// executable with it in place (minio/selfupdate handles the atomic
// rename dance on every OS, including the Windows "delete-on-reboot" case).
func applyUpdate(ctx context.Context, rel *githubRelease) error {
	name := updateAssetName()
	asset := findAsset(rel, name)
	if asset == nil {
		return fmt.Errorf("no release asset for %s/%s (%s)", runtime.GOOS, runtime.GOARCH, name)
	}

	data, err := downloadAsset(ctx, asset)
	if err != nil {
		return err
	}

	checksum, err := fetchChecksum(ctx, rel, name)
	if err != nil {
		return err
	}

	return selfupdate.Apply(bytes.NewReader(data), selfupdate.Options{Checksum: checksum})
}
