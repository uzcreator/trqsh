package agent

import (
	"context"
	"encoding/json"
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
	// Desktop releases now share the main repo (uzcreator/trqsh) but carry a
	// `desktop-v*` tag prefix so they don't collide with the CLI's `v*` tags.
	// /releases/latest can't filter by prefix, so we list releases via the API and
	// pick the newest desktop-tagged one; the hourly cache below keeps this well
	// under the 60 req/h unauthenticated rate limit.
	releasesAPIURL  = "https://api.github.com/repos/uzcreator/trqsh/releases?per_page=30"
	downloadBaseURL = "https://github.com/uzcreator/trqsh/releases/download"
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPIURL, nil)
	if err != nil {
		return UpdateInfo{}, err
	}
	req.Header.Set("User-Agent", "trqsh-desktop")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return UpdateInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("releases: status %s", resp.Status)
	}

	var rels []struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		Draft   bool   `json:"draft"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return UpdateInfo{}, err
	}
	// Releases come newest-first; take the first non-draft desktop-tagged one.
	var tag, notes string
	for _, r := range rels {
		if !r.Draft && strings.HasPrefix(r.TagName, "desktop-v") {
			tag, notes = r.TagName, r.Body
			break
		}
	}
	if tag == "" {
		return UpdateInfo{}, fmt.Errorf("no desktop release found")
	}
	version := strings.TrimPrefix(tag, "desktop-v")

	info := UpdateInfo{Version: version, Notes: notes, URL: "https://github.com/uzcreator/trqsh/releases/tag/" + tag}
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
