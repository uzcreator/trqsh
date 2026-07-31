// Command tunnelload is a data-plane load generator for trqsh: it opens N real
// synthetic agent sessions against a target edge and drives each to bind a tunnel,
// reporting connect success rate and connect-latency percentiles.
//
// A generic HTTP load tool cannot do this — an agent session speaks the trqsh
// agent protocol (QUIC/TCP + pkg/tunnel framing), not plain HTTP — so this harness
// reuses the real internal/agent Core, exactly as internal/agent/e2e_test.go does
// for a single agent, scaled to N. Each synthetic agent forwards to a small local
// HTTP server built into this process, so the resulting public URLs actually serve
// (feed them to the httpload/ step to push request traffic THROUGH the tunnels).
//
// This is a connect/capacity harness, not a full traffic model. It answers "how
// many concurrent agent sessions + tunnels can this edge accept, and how fast?".
// It is intentionally NOT run against anything live from CI — point it at a staging
// or prod edge yourself.
//
// Example:
//
//	go run ./deploy/loadtest/tunnelload \
//	  -server staging.trqsh.uz:4443 -apikey tq_xxx -n 500 -duration 5m \
//	  -insecure -urls-out /tmp/urls.txt
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trqsh-uz/trqsh/internal/agent"
)

func main() {
	var (
		server      = flag.String("server", "", "edge agent address host:port (e.g. staging.trqsh.uz:4443) [required]")
		apiKey      = flag.String("apikey", "", "trqsh API key (real edges require it; stub edges accept any)")
		n           = flag.Int("n", 100, "number of concurrent agent sessions / tunnels to open")
		proto       = flag.String("proto", "http", "tunnel protocol to bind (http|https|tls|tcp|udp)")
		transport   = flag.String("transport", "auto", "agent transport: auto|quic|tcp")
		insecure    = flag.Bool("insecure", false, "skip TLS verification (use for Let's Encrypt STAGING certs)")
		concurrency = flag.Int("concurrency", 50, "max simultaneous in-flight connect attempts")
		rampup      = flag.Duration("rampup", 10*time.Second, "spread connection starts over this window")
		duration    = flag.Duration("duration", 60*time.Second, "hold tunnels open this long after connecting (Ctrl+C to stop early)")
		connectTO   = flag.Duration("connect-timeout", 30*time.Second, "per-tunnel connect+bind timeout")
		localAddr   = flag.String("local", "", "forward tunnels to this local addr; empty = start a built-in dummy server")
		urlsOut     = flag.String("urls-out", "", "write the resulting public URLs (one per line) to this file")
	)
	flag.Parse()

	if *server == "" {
		fmt.Fprintln(os.Stderr, "error: -server is required")
		flag.Usage()
		os.Exit(2)
	}
	if *n < 1 {
		fmt.Fprintln(os.Stderr, "error: -n must be >= 1")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Built-in local target so tunnels actually serve without an external service.
	target := *localAddr
	var localSrv *http.Server
	if target == "" {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: start local server: %v\n", err)
			os.Exit(1)
		}
		localSrv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "trqsh-loadtest-ok\n")
		})}
		go func() { _ = localSrv.Serve(ln) }()
		target = ln.Addr().String()
	}

	discard := slog.New(slog.NewTextHandler(io.Discard, nil))

	type result struct {
		idx     int
		connect time.Duration
		url     string
		err     error
	}
	results := make([]result, *n)
	agents := make([]*agent.Agent, *n)

	var (
		wg       sync.WaitGroup
		sem      = make(chan struct{}, *concurrency)
		okCount  atomic.Int64
		errCount atomic.Int64
	)
	perStart := time.Duration(0)
	if *n > 1 {
		perStart = *rampup / time.Duration(*n-1)
	}

	fmt.Printf("tunnelload: opening %d %s tunnels against %s (transport=%s, rampup=%s)\n",
		*n, *proto, *server, *transport, *rampup)
	wallStart := time.Now()

	for i := 0; i < *n; i++ {
		select {
		case <-ctx.Done():
			// Interrupted mid-ramp: leave the rest unstarted.
			results[i] = result{idx: i, err: ctx.Err()}
			errCount.Add(1)
			continue
		case <-time.After(time.Duration(i) * perStart):
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			cfg := agent.DefaultConfig()
			cfg.Server = *server
			cfg.APIKey = *apiKey
			cfg.Transport = *transport
			cfg.Insecure = *insecure

			core := agent.New(cfg, discard)
			agents[i] = core

			bindCtx, cancel := context.WithTimeout(ctx, *connectTO)
			defer cancel()

			t0 := time.Now()
			tn, err := core.StartTunnel(bindCtx, agent.TunnelSpec{
				Proto: *proto,
				Addr:  target,
			})
			d := time.Since(t0)
			if err != nil {
				results[i] = result{idx: i, connect: d, err: err}
				errCount.Add(1)
				return
			}
			results[i] = result{idx: i, connect: d, url: tn.PublicURL}
			okCount.Add(1)
		}(i)
	}
	wg.Wait()
	connectWall := time.Since(wallStart)

	// --- report ---
	var durs []time.Duration
	errs := map[string]int{}
	var urls []string
	for _, r := range results {
		if r.err != nil {
			errs[classify(r.err)]++
			continue
		}
		durs = append(durs, r.connect)
		if r.url != "" {
			urls = append(urls, r.url)
		}
	}
	sort.Slice(durs, func(a, b int) bool { return durs[a] < durs[b] })

	ok := okCount.Load()
	fmt.Printf("\n=== connect results ===\n")
	fmt.Printf("requested:      %d\n", *n)
	fmt.Printf("connected:      %d (%.1f%%)\n", ok, 100*float64(ok)/float64(*n))
	fmt.Printf("failed:         %d\n", errCount.Load())
	fmt.Printf("wall time:      %s\n", connectWall.Round(time.Millisecond))
	if len(durs) > 0 {
		fmt.Printf("connect min:    %s\n", durs[0].Round(time.Millisecond))
		fmt.Printf("connect p50:    %s\n", pct(durs, 50).Round(time.Millisecond))
		fmt.Printf("connect p95:    %s\n", pct(durs, 95).Round(time.Millisecond))
		fmt.Printf("connect p99:    %s\n", pct(durs, 99).Round(time.Millisecond))
		fmt.Printf("connect max:    %s\n", durs[len(durs)-1].Round(time.Millisecond))
	}
	if len(errs) > 0 {
		fmt.Printf("\n=== failures by kind ===\n")
		for k, c := range errs {
			fmt.Printf("%6d  %s\n", c, k)
		}
	}

	if *urlsOut != "" && len(urls) > 0 {
		if err := os.WriteFile(*urlsOut, []byte(joinLines(urls)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "warn: write -urls-out: %v\n", err)
		} else {
			fmt.Printf("\nwrote %d public URLs to %s\n", len(urls), *urlsOut)
		}
	}

	// Hold the tunnels open so a separate HTTP load step can drive traffic through
	// them, then tear everything down.
	if ok > 0 {
		fmt.Printf("\nholding %d tunnels open for %s (Ctrl+C to stop early)...\n", ok, *duration)
		select {
		case <-ctx.Done():
		case <-time.After(*duration):
		}
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var closed int
	for _, a := range agents {
		if a != nil {
			_ = a.Shutdown(shutCtx)
			closed++
		}
	}
	if localSrv != nil {
		_ = localSrv.Shutdown(shutCtx)
	}
	fmt.Printf("closed %d agent sessions. done.\n", closed)
}

// classify collapses an error to a short, groupable label for the summary.
func classify(err error) string {
	if err == nil {
		return "ok"
	}
	msg := err.Error()
	if len(msg) > 60 {
		msg = msg[:60] + "…"
	}
	return msg
}

func pct(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func joinLines(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s + "\n"
	}
	return out
}
