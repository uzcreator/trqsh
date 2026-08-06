// Package cli implements the `trqsh` command line: a thin launcher that opens
// the interactive console (package tui) and the hidden `daemon` the console
// spawns for itself. All user-facing actions live in the console as slash
// commands; this package only wires flags, spawns/【talks to the daemon, and
// bridges the two filesystem operations the console can't do over the API
// (self-update and local-data removal). See dashboard.go and control.go.
package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/ui"
)

// Execute runs the root command. Error formatting is centralized here
// (SilenceErrors on the root command stops cobra from also printing its own
// "Error: ..." line) so every failure is reported exactly once, via printErr.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		printErr(err)
		os.Exit(1)
	}
}

type globalFlags struct {
	configPath string
	server     string
	region     string
	transport  string
	insecure   bool
	logFormat  string
	inspectorA string
	controlA   string
}

func newRootCmd() *cobra.Command {
	g := &globalFlags{}
	root := &cobra.Command{
		Use:           "trqsh",
		Short:         "trqsh — expose your localhost to the internet, fast.",
		Long:          "trqsh tunnels local services to a public URL over a QUIC-first, TCP-fallback transport.",
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("trqsh {{.Version}}\n")
	pf := root.PersistentFlags()
	pf.StringVar(&g.configPath, "config", "", "config file path (default ~/.trqsh/trqsh.yml)")
	pf.StringVar(&g.server, "server", "", "edge server address (host:port)")
	pf.StringVar(&g.region, "region", "", "preferred region (auto|us|eu|ap)")
	pf.StringVar(&g.transport, "transport", "", "transport (auto|quic|tcp)")
	pf.BoolVar(&g.insecure, "insecure", false, "skip TLS verification (dev)")
	pf.StringVar(&g.logFormat, "log", "text", "log format (text|json)")
	pf.StringVar(&g.inspectorA, "inspector-addr", "", "inspector listen address")
	pf.StringVar(&g.controlA, "control-addr", "", "local control API listen address")

	// TUI-only: the interactive console is the whole interface, so no user-facing
	// subcommands are registered. Only `daemon` stays — it's the headless agent
	// the console spawns for itself (see ensureDaemon), never run by hand.
	root.AddCommand(newDaemonCmd(g))

	// Every invocation opens the console. Bare `trqsh` opens it empty; `trqsh
	// http 3000` opens it and runs that as the first slash command. Flags after
	// the first positional pass through verbatim (SetInterspersed(false)) so
	// per-command flags like `--subdomain` reach the console instead of erroring
	// as unknown root flags.
	root.Args = cobra.ArbitraryArgs
	root.Flags().SetInterspersed(false)
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return runTUI(cmd, g, strings.Join(args, " "))
	}
	applyBranding(root)
	return root
}

// loadConfig merges file+env (via agent.Load) then applies changed flags.
func (g *globalFlags) loadConfig(cmd *cobra.Command) (agent.Config, error) {
	cfg, err := agent.Load(g.configPath)
	if err != nil {
		return cfg, err
	}
	fl := cmd.Flags()
	if fl.Changed("server") {
		cfg.Server = g.server
	}
	if fl.Changed("region") {
		cfg.Region = g.region
	}
	if fl.Changed("transport") {
		cfg.Transport = g.transport
	}
	if fl.Changed("insecure") {
		cfg.Insecure = g.insecure
	}
	if fl.Changed("inspector-addr") {
		cfg.Inspector.Addr = g.inspectorA
	}
	if fl.Changed("control-addr") {
		cfg.ControlAddr = g.controlA
	}
	return cfg, nil
}

func (g *globalFlags) logger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if strings.EqualFold(g.logFormat, "json") {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

// printErr renders a failure through the ui layer: a red ✗ line plus, for a
// structured agent error, its actionable hint.
func printErr(err error) {
	fmt.Fprintln(ui.Stderr)
	var re *agent.Error
	if errors.As(err, &re) {
		ui.Fail("%s", re.Message)
		if h := re.Hint(); h != "" {
			fmt.Fprintf(ui.Stderr, "    %s\n", ui.Gray(h))
		}
		return
	}
	ui.Fail("%v", err)
}
