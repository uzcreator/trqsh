package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/tui"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/ui"
)

// newUICmd launches the interactive console — the slash-command TUI. It's also
// what bare `trqsh` runs on a terminal (see newRootCmd). The console ensures a
// background daemon is up, then drives it over the loopback control API, so
// tunnels opened in the console behave exactly like ones started with
// `trqsh http` and persist after the console is closed.
func newUICmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "ui",
		Aliases: []string{"dashboard", "dash", "console"},
		Short:   "Open the interactive console",
		Example: "trqsh\ntrqsh ui",
		RunE:    func(cmd *cobra.Command, _ []string) error { return runTUI(cmd, g) },
	}
}

// runTUI boots the daemon (if needed) and hands off to the TUI. It refuses to
// run without a terminal, where a full-screen UI can't work.
func runTUI(cmd *cobra.Command, g *globalFlags) error {
	if !ui.IsInteractive() {
		return errors.New("the console needs an interactive terminal — run `trqsh --help` for the command list")
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
		Addr:       addr,
		Token:      agent.LoadControlToken(),
		APIKey:     strings.TrimSpace(cfg.APIKey),
		BaseDomain: baseDomainOf(cfg),
		Version:    agent.Version,
	})
}
