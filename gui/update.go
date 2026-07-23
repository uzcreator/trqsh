package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// updateFeedURL points at the release channel published by Part 08's release
// workflow. It returns the latest desktop build for auto-update prompts.
const updateFeedURL = "https://downloads.trqsh.uz/desktop/latest.json"

// UpdateInfo is the result of a version check (frontend/src/lib/agent.ts).
type UpdateInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Notes     string `json:"notes"`
	URL       string `json:"url"`
}

type releaseFeed struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
	URL     string `json:"url"`
}

// CheckUpdate fetches the release feed and compares it to the running version.
// Network failures are treated as "up to date" (non-fatal) so the UI never
// blocks on connectivity.
func (s *AgentService) CheckUpdate() (UpdateInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(updateFeedURL)
	if err != nil {
		s.log.Debug("update check failed", "err", err)
		return UpdateInfo{Version: version}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{Version: version}, nil
	}
	var feed releaseFeed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return UpdateInfo{Version: version}, nil
	}
	return UpdateInfo{
		Available: newerVersion(feed.Version, version),
		Version:   feed.Version,
		Notes:     feed.Notes,
		URL:       feed.URL,
	}, nil
}

// newerVersion reports whether semver-ish string a is strictly greater than b.
// Pre-release/build suffixes are ignored; missing fields count as zero.
func newerVersion(a, b string) bool {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		n, _ := strconv.Atoi(part)
		out[i] = n
	}
	return out
}
