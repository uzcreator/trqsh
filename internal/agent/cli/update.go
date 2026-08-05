package cli

// newUpdateCmd checks for and installs a newer trqsh release: it lists the
// repo's GitHub releases, picks the newest non-draft CLI tag (v* but not
// desktop-v*, which is the GUI's own release train), downloads the matching
// platform archive, verifies its checksum against the release's
// checksums.txt, and atomically swaps it in for the running binary. This
// deliberately mirrors the npm/pip wrappers' own install-on-first-run logic
// (packaging/npm/lib/install.js, packaging/pypi/src/trqsh/_runtime.py): same
// archive naming, same checksum trust model, no new dependencies (stdlib
// archive/zip + archive/tar cover both platforms' archive formats).

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

const (
	// CLI releases publish to their own repo (source stays in uzcreator/trqsh;
	// see .goreleaser.yaml). Desktop GUI releases (desktop-v* tags) live in the
	// source repo instead, so unlike before, this repo's releases are always
	// CLI-only — the desktop-v* skip in latestCLIVersion is now just a
	// defensive no-op, not load-bearing.
	updateRepo     = "uzcreator/trqshcli"
	updateMaxBytes = 200 << 20 // generous cap; real archives are a few tens of MB
)

// Overridable by tests (an httptest.Server can't be reached at the real
// github.com hosts).
var (
	updateReleasesAPIURL = "https://api.github.com/repos/" + updateRepo + "/releases?per_page=30"
	updateDownloadBase   = "https://github.com/" + updateRepo + "/releases/download"
)

func newUpdateCmd(g *globalFlags) *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install the latest trqsh release",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpdate(cmd, g, checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only check for a newer version, don't install it")
	return cmd
}

func runUpdate(cmd *cobra.Command, g *globalFlags, checkOnly bool) error {
	ctx := cmd.Context()
	current := agent.Version
	if current == "" || current == "dev" {
		fmt.Println("running a locally-built dev binary (not a release) — self-update isn't available.")
		fmt.Println("download a release from https://trqsh.uz/download")
		return nil
	}

	fmt.Println("checking for updates…")
	latest, err := latestCLIVersion(ctx)
	if err != nil {
		return fmt.Errorf("could not check for updates: %w", err)
	}

	cmpResult, ok := compareVersions(latest, current)
	if !ok {
		fmt.Printf("you have %s; latest release is %s (couldn't compare versions automatically)\n", current, latest)
		fmt.Println("see https://github.com/" + updateRepo + "/releases")
		return nil
	}
	if cmpResult <= 0 {
		fmt.Printf("trqsh %s is up to date ✓\n", current)
		return nil
	}

	fmt.Printf("a new version is available: %s → %s\n", current, latest)
	if checkOnly {
		fmt.Println("run `trqsh update` to install it.")
		return nil
	}

	target, err := runningBinaryPath()
	if err != nil {
		return err
	}
	if err := installUpdate(ctx, latest, target); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Printf("\n  ✓ updated trqsh %s → %s\n", current, latest)

	if cfg, err := g.loadConfig(cmd); err == nil && daemonAlive(controlAddr(cfg)) {
		fmt.Println("  a background daemon is still running the old build — `trqsh down` then start a new tunnel to pick up the update.")
	}
	return nil
}

// latestCLIVersion returns the newest published CLI release's version (no
// leading "v"). Releases come back newest-first; the desktop-v* skip is
// defensive (this repo is CLI-only) rather than load-bearing.
func latestCLIVersion(ctx context.Context) (string, error) {
	data, err := fetchURL(ctx, updateReleasesAPIURL, "application/vnd.github+json")
	if err != nil {
		return "", err
	}
	var rels []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(data, &rels); err != nil {
		return "", err
	}
	for _, r := range rels {
		if r.Draft || r.Prerelease {
			continue
		}
		if !strings.HasPrefix(r.TagName, "v") || strings.HasPrefix(r.TagName, "desktop-v") {
			continue
		}
		return strings.TrimPrefix(r.TagName, "v"), nil
	}
	return "", errors.New("no published CLI release found")
}

// compareVersions parses a and b as X.Y.Z and reports a<=>b, or ok=false if
// either doesn't parse (e.g. a non-release build), in which case the caller
// shouldn't attempt an automatic update decision.
func compareVersions(a, b string) (result int, ok bool) {
	pa, ok1 := parseSemver(a)
	pb, ok2 := parseSemver(b)
	if !ok1 || !ok2 {
		return 0, false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1, true
			}
			return -1, true
		}
	}
	return 0, true
}

func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 { // drop -prerelease/+build metadata
		v = v[:i]
	}
	parts := strings.Split(v, ".")
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

// installUpdate downloads, verifies, and installs the given released version
// over target (the resolved path of the currently-running binary).
func installUpdate(ctx context.Context, version, target string) error {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	ext, binName := "tar.gz", "trqsh"
	if goos == "windows" {
		ext, binName = "zip", "trqsh.exe"
	}
	archive := fmt.Sprintf("trqsh_%s_%s_%s.%s", version, goos, goarch, ext)
	base := updateDownloadBase + "/v" + version

	fmt.Printf("downloading %s...\n", archive)
	data, err := fetchURL(ctx, base+"/"+archive, "")
	if err != nil {
		return fmt.Errorf("download %s: %w", archive, err)
	}

	sums, err := fetchURL(ctx, base+"/checksums.txt", "")
	if err != nil {
		// Unlike the npm/pip installers (which tolerate a missing
		// checksums.txt for older mirrors), self-update always targets the
		// canonical GitHub release, so a verification failure here aborts
		// rather than silently installing an unverified binary.
		return fmt.Errorf("download checksums.txt: %w", err)
	}
	if err := verifyChecksum(data, archive, string(sums)); err != nil {
		return err
	}
	fmt.Println("checksum verified ✓")

	binData, err := extractBinaryFromArchive(data, ext, binName)
	if err != nil {
		return fmt.Errorf("extract %s: %w", binName, err)
	}

	return replaceBinary(target, binData)
}

// runningBinaryPath resolves the path of the currently-running executable
// (following symlinks), which self-update replaces in place.
func runningBinaryPath() (string, error) {
	target, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	return target, nil
}

func fetchURL(ctx context.Context, url, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "trqsh-cli-updater")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, errors.New("GitHub API rate limit hit — try again in a few minutes, or check https://github.com/" + updateRepo + "/releases directly")
		}
		return nil, fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, updateMaxBytes))
}

// verifyChecksum checks data's sha256 against archive's line in a
// checksums.txt (either "<hex>  <name>" or "<hex> *<name>" form).
func verifyChecksum(data []byte, archive, sums string) error {
	want := ""
	for line := range strings.SplitSeq(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == archive {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf("%s not listed in checksums.txt", archive)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s\n  expected %s\n  got      %s", archive, want, got)
	}
	return nil
}

// extractBinaryFromArchive reads just the named binary's bytes out of a zip
// or tar.gz archive held in memory. It never writes anything using an
// archive-supplied path (the caller picks the destination), so unlike a
// generic "extract everything to a directory" this has no zip-slip /
// path-traversal surface to guard against.
func extractBinaryFromArchive(data []byte, ext, binName string) ([]byte, error) {
	if ext == "zip" {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) != binName {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return io.ReadAll(io.LimitReader(rc, updateMaxBytes))
		}
		return nil, fmt.Errorf("%s not found in archive", binName)
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != binName {
			continue
		}
		return io.ReadAll(io.LimitReader(tr, updateMaxBytes))
	}
	return nil, fmt.Errorf("%s not found in archive", binName)
}

// replaceBinary atomically swaps target for newData's contents. The new
// binary is written to a temp file in the same directory first (so the final
// rename is same-filesystem, hence atomic).
//
// Unix allows renaming directly over a running executable: the process keeps
// running off the old (now-unlinked) inode. Windows won't allow overwriting a
// running .exe in place, so it's renamed aside first and the new binary takes
// its name; the old one is then a best-effort delete (it may still be mapped
// by this very process and fail to remove immediately, which is harmless —
// it's already out of the way, and gets cleaned up by the next update or a
// manual delete).
func replaceBinary(target string, newData []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".trqsh-update-*")
	if err != nil {
		return fmt.Errorf("create temp file next to %s: %w", target, err)
	}
	tmpPath := tmp.Name()
	_, writeErr := tmp.Write(newData)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write new binary: %w", firstErr(writeErr, closeErr))
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil { // no-op perm-wise on Windows, harmless
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod new binary: %w", err)
	}

	if runtime.GOOS == "windows" {
		old := target + ".old"
		_ = os.Remove(old) // drop a stale .old left by a previous update, if any
		if err := os.Rename(target, old); err != nil {
			_ = os.Remove(tmpPath)
			return permissionHint(fmt.Errorf("replace %s: %w", target, err))
		}
		if err := os.Rename(tmpPath, target); err != nil {
			_ = os.Rename(old, target) // best-effort revert so the user isn't left without a binary
			return permissionHint(fmt.Errorf("replace %s: %w", target, err))
		}
		_ = os.Remove(old) // best-effort; may still be in use, that's fine
		return nil
	}

	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return permissionHint(fmt.Errorf("replace %s: %w", target, err))
	}
	return nil
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func permissionHint(err error) error {
	if !errors.Is(err, os.ErrPermission) {
		return err
	}
	if runtime.GOOS == "windows" {
		return fmt.Errorf("%w (try running from an elevated/Administrator terminal)", err)
	}
	return fmt.Errorf("%w (try `sudo trqsh update`, or reinstall to a directory you own)", err)
}
