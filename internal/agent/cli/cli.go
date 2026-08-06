// Package cli implements the `trqsh` command-line interface over the agent core.
package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/ui"
	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
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

	root.AddCommand(
		// Interactive console (also what bare `trqsh` runs on a terminal).
		newUICmd(g),
		// Tunnels (foreground + background).
		newHTTPCmd(g),
		newTCPCmd(g),
		newUDPCmd(g),
		newStartCmd(g),
		newLsCmd(g),
		newOpenCmd(g),
		newStatusCmd(g),
		newStopCmd(g),
		newDownCmd(g),
		newDaemonCmd(g),
		// Auth + account.
		newLoginCmd(g),
		newLogoutCmd(g),
		newWhoamiCmd(g),
		// Account resources.
		newSubdomainsCmd(g),
		newDomainsCmd(g),
		// Misc.
		newConfigCmd(g),
		newVersionCmd(),
		newUpdateCmd(g),
		newUninstallCmd(g),
	)
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

// runTunnels connects, binds each spec, prints URLs, and streams events until
// interrupted.
func runTunnels(cmd *cobra.Command, g *globalFlags, specs []agent.TunnelSpec) error {
	cfg, err := g.loadConfig(cmd)
	if err != nil {
		return err
	}
	log := g.logger()
	core := agent.New(cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Inspector.
	if cfg.Inspector.Enabled && cfg.Inspector.Addr != "" {
		inspSrv := inspect.NewServer(core.Inspector())
		go func() {
			if err := inspSrv.Serve(ctx, cfg.Inspector.Addr); err != nil {
				log.Warn("inspector stopped", "err", err)
			}
		}()
	}
	// Local control API (for the GUI / `trqsh status`).
	if cfg.ControlAddr != "" {
		api := agent.NewLocalAPI(core)
		go func() {
			if err := api.Serve(ctx, cfg.ControlAddr); err != nil {
				log.Warn("control api stopped", "err", err)
			}
		}()
	}

	for _, spec := range specs {
		t, err := core.StartTunnel(ctx, spec)
		if err != nil {
			_ = core.Shutdown(context.Background())
			return err
		}
		fmt.Printf("\n  %s  %s  %s  %s\n",
			ui.Green(ui.IconOK), ui.AccentBold(t.PublicURL), ui.Gray("→"), t.LocalAddr)
	}
	if cfg.Inspector.Enabled {
		fmt.Printf("  %s http://%s\n", ui.Gray("inspector"), cfg.Inspector.Addr)
	}
	fmt.Printf("\n  %s\n\n", ui.Gray("Press Ctrl+C to stop."))

	go streamEvents(ctx, core)

	<-ctx.Done()
	fmt.Printf("\n  %s\n", ui.Gray("shutting down…"))
	sc, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return core.Shutdown(sc)
}

func streamEvents(ctx context.Context, core *agent.Agent) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-core.Events():
			switch ev.Type {
			case "request":
				if ev.Request != nil {
					fmt.Printf("  %s %-40s %s  %s\n",
						methodField(ev.Request.Method), truncate(ev.Request.Path, 40),
						statusField(ev.Request.Status), ui.Gray(fmt.Sprintf("%dms", ev.Request.DurationMs)))
				}
			case "status":
				if ev.Status != nil && !ev.Status.Connected {
					ui.Warn("disconnected — reconnecting…")
				}
			case "error":
				ui.Warn("%s", ev.Err)
			}
		}
	}
}

// methodField renders a request's HTTP method in a fixed-width, colored column
// (padding is applied before coloring so the ANSI codes don't skew alignment).
func methodField(method string) string {
	return ui.Cyan(fmt.Sprintf("%-6s", method))
}

// statusField colors an HTTP status code by class for the live request log.
func statusField(code int) string {
	s := fmt.Sprintf("%3d", code)
	switch {
	case code >= 500:
		return ui.Red(s)
	case code >= 400:
		return ui.Yellow(s)
	case code >= 300:
		return ui.Cyan(s)
	case code >= 200:
		return ui.Green(s)
	default:
		return ui.Gray(s)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func bold(s string) string { return ui.Bold(s) }

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
