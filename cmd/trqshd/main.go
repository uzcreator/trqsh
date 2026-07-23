// Command trqshd is the trqsh edge server (public data plane).
//
// It accepts multiplexed agent sessions and public traffic and routes between
// them. See plan/02-edge-server.md.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/trqsh-uz/trqsh/internal/server"
)

func main() {
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
