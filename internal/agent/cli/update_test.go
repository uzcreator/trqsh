package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		want [3]int
	}{
		{"1.2.3", true, [3]int{1, 2, 3}},
		{"v1.2.3", true, [3]int{1, 2, 3}},
		{"0.1.5", true, [3]int{0, 1, 5}},
		{"1.2.3-dev", true, [3]int{1, 2, 3}},
		{"1.2.3+build5", true, [3]int{1, 2, 3}},
		{"dev", false, [3]int{}},
		{"1.2", false, [3]int{}},
		{"1.2.3.4", false, [3]int{}},
		{"a.b.c", false, [3]int{}},
		{"", false, [3]int{}},
	}
	for _, c := range cases {
		got, ok := parseSemver(c.in)
		if ok != c.ok {
			t.Errorf("parseSemver(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseSemver(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"0.1.6", "0.1.5", 1, true},
		{"0.1.5", "0.1.5", 0, true},
		{"0.1.5", "0.1.6", -1, true},
		{"1.0.0", "0.99.99", 1, true},
		{"0.2.0", "0.1.99", 1, true},
		{"0.1.5", "dev", 0, false},
	}
	for _, c := range cases {
		got, ok := compareVersions(c.a, c.b)
		if ok != c.ok {
			t.Errorf("compareVersions(%q,%q) ok = %v, want %v", c.a, c.b, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("fake archive bytes")
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])

	t.Run("two-space form matches", func(t *testing.T) {
		sums := hexSum + "  trqsh_0.1.6_linux_amd64.tar.gz\nabc123  other_file.tar.gz\n"
		if err := verifyChecksum(data, "trqsh_0.1.6_linux_amd64.tar.gz", sums); err != nil {
			t.Fatalf("verifyChecksum: %v", err)
		}
	})

	t.Run("asterisk binary-mode form matches", func(t *testing.T) {
		sums := hexSum + " *trqsh_0.1.6_linux_amd64.tar.gz\n"
		if err := verifyChecksum(data, "trqsh_0.1.6_linux_amd64.tar.gz", sums); err != nil {
			t.Fatalf("verifyChecksum: %v", err)
		}
	})

	t.Run("mismatch is rejected", func(t *testing.T) {
		sums := "0000000000000000000000000000000000000000000000000000000000000000  trqsh_0.1.6_linux_amd64.tar.gz\n"
		if err := verifyChecksum(data, "trqsh_0.1.6_linux_amd64.tar.gz", sums); err == nil {
			t.Fatal("expected checksum mismatch error, got nil")
		}
	})

	t.Run("missing entry is rejected", func(t *testing.T) {
		sums := hexSum + "  some_other_archive.tar.gz\n"
		if err := verifyChecksum(data, "trqsh_0.1.6_linux_amd64.tar.gz", sums); err == nil {
			t.Fatal("expected 'not listed' error, got nil")
		}
	})
}

func TestExtractBinaryFromArchiveZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile(t, zw, "decoy.txt", []byte("not the binary"))
	writeZipFile(t, zw, "trqsh_0.1.6_windows_amd64/trqsh.exe", []byte("fake windows binary"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := extractBinaryFromArchive(buf.Bytes(), "zip", "trqsh.exe")
	if err != nil {
		t.Fatalf("extractBinaryFromArchive: %v", err)
	}
	if string(got) != "fake windows binary" {
		t.Fatalf("got %q, want %q", got, "fake windows binary")
	}

	if _, err := extractBinaryFromArchive(buf.Bytes(), "zip", "nope.exe"); err == nil {
		t.Fatal("expected error for missing binary name, got nil")
	}
}

func TestExtractBinaryFromArchiveTarGz(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarFile(t, tw, "decoy.txt", []byte("not the binary"))
	writeTarFile(t, tw, "trqsh_0.1.6_linux_amd64/trqsh", []byte("fake linux binary"))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := extractBinaryFromArchive(buf.Bytes(), "tar.gz", "trqsh")
	if err != nil {
		t.Fatalf("extractBinaryFromArchive: %v", err)
	}
	if string(got) != "fake linux binary" {
		t.Fatalf("got %q, want %q", got, "fake linux binary")
	}
}

func TestReplaceBinary(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "trqsh")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	if err := os.WriteFile(target, []byte("old binary content"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(target, []byte("new binary content")); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after replace: %v", err)
	}
	if string(got) != "new binary content" {
		t.Fatalf("target content = %q, want %q", got, "new binary content")
	}

	// A second replace must also succeed (covers the Windows .old-cleanup path
	// on a second run, and a plain overwrite on Unix).
	if err := replaceBinary(target, []byte("newer binary content")); err != nil {
		t.Fatalf("second replaceBinary: %v", err)
	}
	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after second replace: %v", err)
	}
	if string(got) != "newer binary content" {
		t.Fatalf("target content = %q, want %q", got, "newer binary content")
	}
}

func TestLatestCLIVersionSkipsDesktopAndDrafts(t *testing.T) {
	const body = `[
		{"tag_name":"v0.1.6-rc1","draft":false,"prerelease":true},
		{"tag_name":"v0.1.5","draft":false,"prerelease":false},
		{"tag_name":"v0.1.4","draft":false,"prerelease":false},
		{"tag_name":"desktop-v0.3.1","draft":false,"prerelease":false},
		{"tag_name":"desktop-v0.3.0","draft":false,"prerelease":false},
		{"tag_name":"v0.1.9","draft":true,"prerelease":false}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	orig := updateReleasesAPIURL
	updateReleasesAPIURL = srv.URL
	defer func() { updateReleasesAPIURL = orig }()

	got, err := latestCLIVersion(context.Background())
	if err != nil {
		t.Fatalf("latestCLIVersion: %v", err)
	}
	if got != "0.1.5" {
		t.Fatalf("latestCLIVersion = %q, want %q (must skip the prerelease, desktop-v*, and draft entries)", got, "0.1.5")
	}
}

func TestFetchURLRateLimitMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := fetchURL(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("error = %q, want it to mention the rate limit", err.Error())
	}
}

// TestInstallUpdateEndToEnd drives the real download -> checksum verify ->
// extract -> replace pipeline against a fake release server, proving the
// pieces are wired together correctly (the other tests only cover each piece
// in isolation).
func TestInstallUpdateEndToEnd(t *testing.T) {
	const version = "9.9.9"
	binName := "trqsh"
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		binName = "trqsh.exe"
		ext = "zip"
	}
	archiveName := "trqsh_" + version + "_" + runtime.GOOS + "_" + runtime.GOARCH + "." + ext

	archiveBytes := buildArchive(t, ext, binName, "brand new binary")
	sum := sha256.Sum256(archiveBytes)
	checksums := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/v"+version+"/"+archiveName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archiveBytes)
	})
	mux.HandleFunc("/v"+version+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	origBase := updateDownloadBase
	updateDownloadBase = srv.URL
	defer func() { updateDownloadBase = origBase }()

	dir := t.TempDir()
	target := filepath.Join(dir, binName)
	if err := os.WriteFile(target, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := installUpdate(context.Background(), version, target); err != nil {
		t.Fatalf("installUpdate: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "brand new binary" {
		t.Fatalf("target content = %q, want %q", got, "brand new binary")
	}
}

func buildArchive(t *testing.T, ext, binName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if ext == "zip" {
		zw := zip.NewWriter(&buf)
		writeZipFile(t, zw, binName, []byte(content))
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarFile(t, tw, binName, []byte(content))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeZipFile(t *testing.T, zw *zip.Writer, name string, content []byte) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
}

func writeTarFile(t *testing.T, tw *tar.Writer, name string, content []byte) {
	t.Helper()
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
}
