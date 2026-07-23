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

	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
	"github.com/spf13/cobra"
)

// Execute runs the root command.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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
		Use:          "trqsh",
		Short:        "trqsh — expose your localhost to the internet, fast.",
		Long:         "trqsh tunnels local services to a public URL over a QUIC-first, TCP-fallback transport.",
		SilenceUsage: true,
	}
	pf := root.PersistentFlags()
	pf.StringVar(&g.configPath, "config", "", "config file path (default ~/.trqsh-uz/trqsh.yml)")
	pf.StringVar(&g.server, "server", "", "edge server address (host:port)")
	pf.StringVar(&g.region, "region", "", "preferred region (auto|us|eu|ap)")
	pf.StringVar(&g.transport, "transport", "", "transport (auto|quic|tcp)")
	pf.BoolVar(&g.insecure, "insecure", false, "skip TLS verification (dev)")
	pf.StringVar(&g.logFormat, "log", "text", "log format (text|json)")
	pf.StringVar(&g.inspectorA, "inspector-addr", "", "inspector listen address")
	pf.StringVar(&g.controlA, "control-addr", "", "local control API listen address")

	root.AddCommand(
		newHTTPCmd(g),
		newTCPCmd(g),
		newUDPCmd(g),
		newStartCmd(g),
		newStatusCmd(g),
		newStopCmd(g),
		newLoginCmd(g),
		newConfigCmd(g),
		newVersionCmd(),
		newUpdateCmd(),
	)
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
			printErr(err)
			_ = core.Shutdown(context.Background())
			return err
		}
		fmt.Printf("\n  %s  →  %s\n", bold(t.PublicURL), t.LocalAddr)
	}
	if cfg.Inspector.Enabled {
		fmt.Printf("  inspector: http://%s\n", cfg.Inspector.Addr)
	}
	fmt.Printf("\n  Press Ctrl+C to stop.\n\n")

	go streamEvents(ctx, core)

	<-ctx.Done()
	fmt.Println("\nshutting down…")
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
					fmt.Printf("  %-6s %-40s %d  %dms\n",
						ev.Request.Method, truncate(ev.Request.Path, 40), ev.Request.Status, ev.Request.DurationMs)
				}
			case "status":
				if ev.Status != nil && !ev.Status.Connected {
					fmt.Println("  ⚠ disconnected — reconnecting…")
				}
			case "error":
				fmt.Printf("  ⚠ %s\n", ev.Err)
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func bold(s string) string { return "\033[1m" + s + "\033[0m" }

func printErr(err error) {
	var re *agent.Error
	if errors.As(err, &re) {
		fmt.Fprintf(os.Stderr, "\n  ✗ %s\n", re.Message)
		if h := re.Hint(); h != "" {
			fmt.Fprintf(os.Stderr, "    %s\n", h)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "\n  ✗ %v\n", err)
}
