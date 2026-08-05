// Command trqshd is the trqsh edge server (public data plane).
//
// It accepts multiplexed agent sessions and public traffic and routes between
// them. See plan/02-edge-server.md.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/trqsh-uz/trqsh/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	var (
		logJSON    bool
		baseDomain string
		quicAddr   string
		tcpAddr    string
		httpAddr   string
		httpsAddr  string
	)
	flag.BoolVar(&logJSON, "log-json", false, "emit JSON logs")
	flag.StringVar(&baseDomain, "base-domain", "", "override TRQSH_BASE_DOMAIN")
	flag.StringVar(&quicAddr, "quic-addr", "", "override TRQSH_QUIC_ADDR")
	flag.StringVar(&tcpAddr, "tcp-addr", "", "override TRQSH_TCP_ADDR")
	flag.StringVar(&httpAddr, "http-addr", "", "override TRQSH_HTTP_ADDR")
	flag.StringVar(&httpsAddr, "https-addr", "", "override TRQSH_HTTPS_ADDR")
	flag.Parse()

	log := newLogger(logJSON)

	cfg, err := server.LoadConfig()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	// Flag overrides win over env.
	overrideStr(&cfg.BaseDomain, baseDomain)
	overrideStr(&cfg.QUICAddr, quicAddr)
	overrideStr(&cfg.TCPAddr, tcpAddr)
	overrideStr(&cfg.HTTPAddr, httpAddr)
	overrideStr(&cfg.HTTPSAddr, httpsAddr)

	srv, err := server.New(cfg, log)
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
	log.Info("trqshd stopped")
}

func newLogger(jsonOut bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if jsonOut {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func overrideStr(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}

// healthcheck lets `trqshd healthcheck` serve as the container's HEALTHCHECK
// CMD: the distroless runtime image has no shell/curl to run a normal probe
// command, so this gives Docker an exec-form check that hits the
// already-running server's own /healthz on the metrics port (reading the
// same TRQSH_METRICS_ADDR it bound, not a hardcoded port). Returns a process
// exit code, not an error.
func healthcheck() int {
	addr := os.Getenv("TRQSH_METRICS_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz") // #nosec G704 -- addr is our own TRQSH_METRICS_ADDR env var (operator/deployment-controlled), not request input; this checks the process's own local port
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
