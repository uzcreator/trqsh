package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression test for the bug where `trqsh uninstall`, run from inside the
// pypi wrapper's binary cache, would skip its own version directory forever:
// nothing else ever revisits it, so every uninstalled version accumulated as
// dead weight under ~/.cache/trqsh. removeBinaryCacheSelf must still remove
// every *other* version synchronously, and must schedule (not silently drop)
// removal of its own.
func TestRemoveBinaryCacheSelf(t *testing.T) {
	cacheDir := t.TempDir()

	for _, v := range []string{"0.1.2", "0.1.6", "0.1.7"} {
		dir := filepath.Join(cacheDir, v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "trqsh.exe"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	selfExe := filepath.Join(cacheDir, "0.1.7", "trqsh.exe")

	if removed := removeBinaryCacheSelf(cacheDir, selfExe); !removed {
		t.Fatal("removeBinaryCacheSelf returned false, want true (other versions were present)")
	}

	for _, v := range []string{"0.1.2", "0.1.6"} {
		if _, err := os.Stat(filepath.Join(cacheDir, v)); !os.IsNotExist(err) {
			t.Errorf("version dir %s should have been removed synchronously, stat err = %v", v, err)
		}
	}

	if _, err := os.Stat(selfExe); err != nil {
		t.Errorf("self dir must still exist immediately after the call (removal is deferred), stat err = %v", err)
	}
}

// With no self-directory in play, every version should go and the now-empty
// parent should go with them (ordinary uninstall from a normal install, e.g.
// invoked via `go run`/a non-cached binary, or a stale cache with nothing
// currently running from it).
func TestRemoveBinaryCacheSelf_NoSelfMatch(t *testing.T) {
	cacheDir := t.TempDir()
	dir := filepath.Join(cacheDir, "0.1.7")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trqsh.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if removed := removeBinaryCacheSelf(cacheDir, filepath.Join(t.TempDir(), "trqsh.exe")); !removed {
		t.Fatal("removeBinaryCacheSelf returned false, want true")
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("cache dir should have been removed entirely, stat err = %v", err)
	}
}

func TestRemoveBinaryCacheSelf_MissingDir(t *testing.T) {
	if removed := removeBinaryCacheSelf(filepath.Join(t.TempDir(), "does-not-exist"), ""); removed {
		t.Fatal("removeBinaryCacheSelf returned true for a nonexistent cache dir")
	}
}
