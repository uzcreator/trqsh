package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

// newUninstallCmd removes trqsh's local footprint — the config/credential dir and
// the downloaded-binary cache — and stops any background daemon, then prints how
// to remove the program itself. It's the convenient counterpart to `npm rm -g
// @trqsh-uz/trqsh` / `pip uninstall trqsh`, which drop the wrapper but leave the
// API key, control token, logs, and cached binary behind. A running binary can't
// delete its own package, so package removal stays a printed instruction.
func newUninstallCmd(g *globalFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove trqsh's local data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _ := g.loadConfig(cmd)
			cfgDir := filepath.Dir(agent.DefaultConfigPath())
			cacheDir := pypiCacheDir()

			fmt.Println("This removes trqsh's local data:")
			fmt.Printf("  • config & credentials  %s\n", cfgDir)
			if cacheDir != "" {
				fmt.Printf("  • downloaded binaries   %s\n", cacheDir)
			}
			fmt.Println("  • any running background tunnels")
			if !yes && !confirmPrompt("Continue?") {
				fmt.Println("canceled")
				return nil
			}

			// Stop the background daemon before deleting the token it authenticates with.
			addr := controlAddr(cfg)
			if daemonAlive(addr) {
				_ = controlPOST(addr, "/shutdown", nil, nil)
				fmt.Println("stopped background daemon")
			}

			removed := removePath(cfgDir)
			if removeBinaryCache(cacheDir) {
				removed = true
			}
			if removed {
				fmt.Println("removed local trqsh data ✓")
			} else {
				fmt.Println("no local trqsh data found")
			}
			fmt.Print(packageRemovalHelp())
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

// pypiCacheDir mirrors the PyPI wrapper's binary cache location (XDG_CACHE_HOME or
// ~/.cache, then /trqsh), so uninstall can clear the binaries it downloaded.
func pypiCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "trqsh")
}

// removePath deletes dir (recursively) if it exists, returning whether it was there.
func removePath(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(dir); err != nil {
		return false
	}
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not fully remove %s: %v\n", dir, err)
	}
	return true
}

// removeBinaryCache clears the downloaded-binary cache. The directory holding
// the binary currently running this command (pypi's ~/.cache/trqsh/<version>/)
// can't be removed synchronously — see removeSelfDir — so it's scheduled for
// removal after this process exits instead of being left behind for good.
func removeBinaryCache(cacheDir string) bool {
	self, _ := os.Executable()
	return removeBinaryCacheSelf(cacheDir, self)
}

// removeBinaryCacheSelf is removeBinaryCache with the running executable's path
// injected, so tests can simulate "we're running from inside this cache" without
// needing an actual binary planted under a temp dir.
func removeBinaryCacheSelf(cacheDir, selfPath string) bool {
	if cacheDir == "" {
		return false
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}
	self := strings.ToLower(filepath.Clean(selfPath))
	removedAny, selfDir := false, ""
	for _, e := range entries {
		p := filepath.Join(cacheDir, e.Name())
		if self != "" && strings.HasPrefix(self, strings.ToLower(filepath.Clean(p))+string(os.PathSeparator)) {
			selfDir = p // we're running from inside this version dir
			continue
		}
		if err := os.RemoveAll(p); err == nil {
			removedAny = true
		}
	}
	if selfDir != "" {
		removeSelfDir(selfDir, cacheDir)
		removedAny = true
	} else {
		_ = os.Remove(cacheDir) // drop the now-empty parent
	}
	return removedAny
}

// removeSelfDir removes dir — which holds the binary currently executing this
// process — plus parent once dir is gone. Unix allows unlinking a running
// binary immediately (the inode survives until the process exits), but
// Windows keeps an open exe locked, so a detached helper waits for this PID
// to exit before deleting; otherwise every `trqsh uninstall` would leave its
// own version behind forever, since it can never delete itself.
//
// The path values travel through environment variables rather than being
// interpolated into the -Command string, the same reasoning as the npm
// installer's PowerShell quoting fix: a path can legitimately contain quote
// characters that make naive string-building unsafe.
func removeSelfDir(dir, parent string) {
	if runtime.GOOS != "windows" {
		_ = os.RemoveAll(dir)
		_ = os.Remove(parent)
		return
	}
	// Both Remove-Item calls pass -Recurse, even the parent (expected to be an
	// empty dir by the time it's reached): without it, -NonInteractive mode
	// turns Remove-Item's "not empty, are you sure" confirmation into a hard
	// PSInvalidOperationException ("Read and Prompt functionality is not
	// available") that -ErrorAction SilentlyContinue does NOT swallow, since
	// it comes from the host-prompt subsystem rather than a normal error record.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command",
		`$p = Get-Process -Id ([int]$env:TRQSH_WAIT_PID) -ErrorAction SilentlyContinue; `+
			`if ($p) { $p.WaitForExit() }; Start-Sleep -Milliseconds 300; `+
			`Remove-Item -LiteralPath $env:TRQSH_RM_DIR -Recurse -Force -ErrorAction SilentlyContinue; `+
			`Remove-Item -LiteralPath $env:TRQSH_RM_PARENT -Recurse -Force -ErrorAction SilentlyContinue`)
	cmd.Env = append(os.Environ(),
		"TRQSH_WAIT_PID="+strconv.Itoa(os.Getpid()),
		"TRQSH_RM_DIR="+dir,
		"TRQSH_RM_PARENT="+parent,
	)
	_ = cmd.Start() // detached best-effort; nothing to do if it fails to launch
}

func packageRemovalHelp() string {
	return "\nTo remove the trqsh program itself, run whichever you installed with:\n" +
		"  npm:    npm rm -g @trqsh-uz/trqsh\n" +
		"  pip:    pip uninstall trqsh\n" +
		"  scoop:  scoop uninstall trqsh\n" +
		"Otherwise delete the trqsh binary from your PATH.\n"
}

// confirmPrompt asks a yes/no question, defaulting to no (also when stdin isn't a
// terminal), so `uninstall` never deletes data without an explicit yes or -y.
func confirmPrompt(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(sc.Text()))
	return ans == "y" || ans == "yes"
}
