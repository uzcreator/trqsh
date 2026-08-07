package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/tui"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/ui"
)

// runTUI is the whole trqsh entry point now: it ensures a background daemon is
// up, then hands off to the interactive console (package tui), which drives that
// daemon over its loopback control API. `initial` is the command the user typed
// after `trqsh` (e.g. "http 3000"), run as the console's first slash command.
//
// The two filesystem/binary operations the console can't do over the daemon —
// self-update and local-data removal — are passed in as callbacks that reuse the
// logic in update.go / uninstall.go.
func runTUI(cmd *cobra.Command, g *globalFlags, initial string) error {
	if !ui.IsInteractive() {
		return errors.New("trqsh is an interactive console — run it in a terminal (not through a pipe or redirect)")
	}
	cfg, err := g.loadConfig(cmd)
	if err != nil {
		return err
	}
	addr := controlAddr(cfg)
	if err := ensureDaemon(cmd, g, addr); err != nil {
		return err
	}
	return tui.Run(tui.Options{
		Addr:           addr,
		Token:          agent.LoadControlToken(),
		APIKey:         strings.TrimSpace(cfg.APIKey),
		BaseDomain:     baseDomainOf(cfg),
		Version:        agent.Version,
		InitialCommand: initial,
		StartSpecs:     configSpecs(cfg),
		ConfigRows:     configRows(g, cfg),
		OnUpdate:       selfUpdate,
		OnCheckUpdate:  latestCLIVersion,
		OnUninstall:    func(_ context.Context) (string, error) { return removeLocalData(cfg) },
	})
}

// configSpecs turns the config file's tunnel definitions into specs for /start.
func configSpecs(cfg agent.Config) []agent.TunnelSpec {
	specs := make([]agent.TunnelSpec, 0, len(cfg.Tunnels))
	for name, t := range cfg.Tunnels {
		specs = append(specs, agent.TunnelSpec{
			Name: name, Proto: t.Proto, Addr: t.Addr, Subdomain: t.Subdomain,
			CustomDomain: t.CustomDomain, BasicAuth: t.BasicAuth, HostHeader: t.HostHeader, RemotePort: t.RemotePort,
		})
	}
	return specs
}

// configRows renders the key/value pairs shown by the console's /config.
func configRows(g *globalFlags, cfg agent.Config) [][2]string {
	signedIn := "no"
	if strings.TrimSpace(cfg.APIKey) != "" {
		signedIn = "yes"
	}
	return [][2]string{
		{"file", firstNonEmpty(g.configPath, agent.DefaultConfigPath())},
		{"server", cfg.Server},
		{"region", cfg.Region},
		{"transport", cfg.Transport},
		{"signed in", signedIn},
		{"control", cfg.ControlAddr},
		{"tunnels", fmt.Sprintf("%d defined", len(cfg.Tunnels))},
	}
}

// selfUpdate downloads and installs the latest release over the running binary,
// returning a one-line summary for the console transcript. It reuses the update
// machinery in update.go.
func selfUpdate(ctx context.Context) (string, error) {
	current := agent.Version
	if current == "" || current == "dev" {
		return "", errors.New("this is a dev build — install a release from https://trqsh.uz/download")
	}
	latest, err := latestCLIVersion(ctx)
	if err != nil {
		return "", err
	}
	if cmp, ok := compareVersions(latest, current); !ok || cmp <= 0 {
		return "trqsh " + current + " is already the latest", nil
	}
	target, err := runningBinaryPath()
	if err != nil {
		return "", err
	}
	if err := installUpdate(ctx, latest, target); err != nil {
		return "", err
	}
	return "updated trqsh " + current + " → " + latest, nil
}

// removeLocalData stops the daemon and deletes trqsh's config/credentials and
// cached binaries, returning a summary (plus how to remove the program itself).
// It reuses the helpers in uninstall.go.
func removeLocalData(cfg agent.Config) (string, error) {
	if addr := controlAddr(cfg); daemonAlive(addr) {
		_ = controlPOST(addr, "/shutdown", nil, nil)
		// Give it a moment to actually exit before touching its files: the
		// /shutdown handler acks before it stops, and Windows can't delete a
		// file (daemon.log) the still-exiting process has open.
		waitDaemonExit(addr, 5*time.Second)
	}
	removed := removePath(filepath.Dir(agent.DefaultConfigPath()))
	if removeBinaryCache(pypiCacheDir()) {
		removed = true
	}
	if !removed {
		return "no local trqsh data found", nil
	}
	return "removed local data — to remove the program: `npm rm -g @trqsh-uz/trqsh`, `pip uninstall trqsh`, or `scoop uninstall trqsh`", nil
}
