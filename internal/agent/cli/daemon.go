package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
)

// newDaemonCmd runs the agent headless, serving only the local control API so an
// out-of-process UI (the desktop app) can drive it: login, start/stop tunnels,
// inspect traffic. Unlike `trqsh http`, it starts with zero tunnels and stays up
// until signaled, letting the UI open and close tunnels on demand.
func newDaemonCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:    "daemon",
		Short:  "Run the background agent (control API for the desktop app)",
		Hidden: true, // implementation detail: spawned by -d and the desktop app, not run by hand
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			if cfg.ControlAddr == "" {
				cfg.ControlAddr = "127.0.0.1:4041"
			}
			log := g.logger()
			core := agent.New(cfg, log)

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// Inspector UI (optional; parity with `trqsh http`).
			if cfg.Inspector.Enabled && cfg.Inspector.Addr != "" {
				inspSrv := inspect.NewServer(core.Inspector())
				go func() {
					if err := inspSrv.Serve(ctx, cfg.Inspector.Addr); err != nil {
						log.Warn("inspector stopped", "err", err)
					}
				}()
			}

			// Loopback control-API token (disable for local browser dev only).
			token := ""
			if os.Getenv("TRQSH_CONTROL_NO_AUTH") != "1" {
				token, err = agent.LoadOrCreateControlToken()
				if err != nil {
					return fmt.Errorf("control token: %w", err)
				}
			}
			// OnShutdown lets `trqsh down` stop this daemon over the control API:
			// canceling ctx unblocks the run loop below and shuts the core down.
			api := agent.NewLocalAPI(core).SetToken(token).OnShutdown(stop)

			fmt.Printf("trqsh daemon — control API on http://%s\n", cfg.ControlAddr)
			if token != "" {
				fmt.Printf("  token file: %s\n", agent.ControlTokenPath())
			} else {
				fmt.Printf("  token: DISABLED (TRQSH_CONTROL_NO_AUTH=1 — dev only)\n")
			}
			if cfg.Inspector.Enabled {
				fmt.Printf("  inspector:  http://%s\n", cfg.Inspector.Addr)
			}
			fmt.Printf("  ready — press Ctrl+C to stop.\n")

			errCh := make(chan error, 1)
			go func() { errCh <- api.Serve(ctx, cfg.ControlAddr) }()

			select {
			case <-ctx.Done():
			case err := <-errCh:
				if err != nil {
					return fmt.Errorf("control api: %w", err)
				}
			}

			fmt.Println("\nshutting down…")
			sc, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			return core.Shutdown(sc)
		},
	}
}
