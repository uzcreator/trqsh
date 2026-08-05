// Command trqshapi is the trqsh control-plane API: accounts, API keys, domains,
// quotas, and the entitlements service the edge calls. See plan/05-control-api.md.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/trqsh-uz/trqsh/internal/api"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := api.LoadConfig()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	srv, err := api.New(cfg, log)
	if err != nil {
		log.Error("init", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		log.Error("run", "err", err)
		os.Exit(1)
	}
}

// healthcheck lets `trqshapi healthcheck` serve as the container's
// HEALTHCHECK CMD: the distroless runtime image has no shell/curl to run a
// normal probe command, so this gives Docker an exec-form check that hits the
// already-running server's own /healthz (reading the same TRQSH_API_ADDR it
// bound, not a hardcoded port). Returns a process exit code, not an error.
func healthcheck() int {
	addr := os.Getenv("TRQSH_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz") // #nosec G704 -- addr is our own TRQSH_API_ADDR env var (operator/deployment-controlled), not request input; this checks the process's own local port
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
