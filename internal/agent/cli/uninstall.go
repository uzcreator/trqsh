package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
		Short: "Remove trqsh's local data (config, token, cache) and show how to remove the package",
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

// removeBinaryCache clears the downloaded-binary cache but never the directory of
// the currently-running binary (a live .exe can't delete itself on Windows) —
// that one is reclaimed by `pip uninstall` / the OS.
func removeBinaryCache(cacheDir string) bool {
	if cacheDir == "" {
		return false
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}
	self, _ := os.Executable()
	self = strings.ToLower(filepath.Clean(self))
	removedAny, skipped := false, false
	for _, e := range entries {
		p := filepath.Join(cacheDir, e.Name())
		if self != "" && strings.HasPrefix(self, strings.ToLower(filepath.Clean(p))+string(os.PathSeparator)) {
			skipped = true // we're running from inside this version dir
			continue
		}
		if err := os.RemoveAll(p); err == nil {
			removedAny = true
		}
	}
	if !skipped {
		_ = os.Remove(cacheDir) // drop the now-empty parent
	}
	return removedAny
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
