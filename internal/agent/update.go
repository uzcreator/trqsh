package agent

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

// UpdateInfo describes the latest desktop release published to the public GUI
// repo. The desktop app compares Version against its own build version to decide
// whether to prompt the user to update.
type UpdateInfo struct {
	Version string `json:"version"` // latest release tag, without the leading "v"
	Notes   string `json:"notes"`   // release notes (unused for now)
	URL     string `json:"url"`     // direct installer for this OS, else the release page
}

const (
	// We resolve the latest version from the /releases/latest *redirect* rather
	// than the GitHub API: the API is rate-limited to 60 req/h per IP
	// (unauthenticated), which 403s in practice; the plain web redirect is not,
	// and just needs the Location header to read the newest tag.
	latestReleaseURL = "https://github.com/trqsh-uz/gui/releases/latest"
	downloadBaseURL  = "https://github.com/trqsh-uz/gui/releases/download"
)

var (
	updMu    sync.Mutex
	updCache *UpdateInfo
	updAt    time.Time
)

// latestRelease returns the newest published desktop release, cached for an hour
// so launching the app (and manual checks) don't hammer GitHub.
func latestRelease(ctx context.Context) (UpdateInfo, error) {
	updMu.Lock()
	if updCache != nil && time.Since(updAt) < time.Hour {
		u := *updCache
		updMu.Unlock()
		return u, nil
	}
	updMu.Unlock()

	// Don't follow the redirect — we want to read its Location header.
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return UpdateInfo{}, err
	}
	req.Header.Set("User-Agent", "trqsh-desktop")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return UpdateInfo{}, fmt.Errorf("releases/latest: no redirect (status %s)", resp.Status)
	}
	// loc looks like https://github.com/trqsh-uz/gui/releases/tag/v0.2.3
	i := strings.LastIndex(loc, "/tag/")
	if i == -1 {
		return UpdateInfo{}, fmt.Errorf("releases/latest: unexpected location %q", loc)
	}
	tag := loc[i+len("/tag/"):]
	version := strings.TrimPrefix(tag, "v")

	info := UpdateInfo{Version: version, URL: loc}
	// Point straight at this OS's installer (asset names follow Tauri's bundler
	// convention, matching what CI publishes and the website links).
	if asset := assetName(version, runtime.GOOS); asset != "" {
		info.URL = downloadBaseURL + "/" + tag + "/" + asset
	}

	updMu.Lock()
	updCache = &info
	updAt = time.Now()
	updMu.Unlock()
	return info, nil
}

// assetName is the installer file name for a version on the given OS.
func assetName(version, goos string) string {
	switch goos {
	case "windows":
		return "trqsh_" + version + "_x64-setup.exe"
	case "darwin":
		return "trqsh_" + version + "_aarch64.dmg"
	case "linux":
		return "trqsh_" + version + "_amd64.AppImage"
	}
	return ""
}
