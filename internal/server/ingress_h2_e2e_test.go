package server_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/http2"

	"github.com/trqsh-uz/trqsh/internal/server"
	"github.com/trqsh-uz/trqsh/pkg/proto"
)

// startEdgeH2 boots an edge with the HTTP/2 TCP ingress enabled (TRQSH_ENABLE_H2)
// on an ephemeral port, returning it once ready.
func startEdgeH2(t *testing.T) *server.Server {
	t.Helper()
	cfg := server.DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.QUICAddr = "127.0.0.1:0"
	cfg.TCPAddr = "127.0.0.1:0"
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.HTTPSAddr = "127.0.0.1:0"
	cfg.EnableH2 = true
	cfg.MetricsAddr = "127.0.0.1:0"
	cfg.HeartbeatInterval = 0

	srv, err := server.New(cfg, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx) }()
	select {
	case <-srv.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("edge did not become ready")
	}
	return srv
}

// h2Client returns an *http.Client that speaks HTTP/2 to the edge's TCP HTTPS
// address, keeping the request host as SNI/authority so routing works while
// dialing 127.0.0.1:port.
func h2Client(t *testing.T, edge net.Addr) (*http.Client, func()) {
	t.Helper()
	tr := &http2.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2"}},
		DialTLSContext: func(ctx context.Context, _, addr string, cfg *tls.Config) (net.Conn, error) {
			host, _, _ := net.SplitHostPort(addr)
			c := cfg.Clone()
			c.ServerName = host
			return (&tls.Dialer{Config: c}).DialContext(ctx, "tcp", edge.String())
		},
	}
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}, tr.CloseIdleConnections
}

// h2GetWithRetry issues a GET over HTTP/2, retrying briefly to absorb the
// listener's startup race, and returns the response (caller closes the body).
func h2GetWithRetry(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	var lastErr error
	for i := 0; i < 25; i++ {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			return resp
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("h2 GET %s failed: %v", url, lastErr)
	return nil
}

// TestEdgeHTTPWeldH2 is the HTTP/2 analogue of TestEdgeHTTPWeld: a public request
// arriving over an h2 connection is routed by Host to the owning agent and welded
// through. resp.ProtoMajor==2 proves h2 was actually negotiated (not h1.1).
func TestEdgeHTTPWeldH2(t *testing.T) {
	srv := startEdgeH2(t)
	fa := dialAgent(t, srv.AgentAddr().String())
	fa.hello(t, "tq_test")
	if r := fa.bind(t, &proto.BindTunnel{ClientTunnelId: "t1", Type: proto.TunnelType_HTTP, Subdomain: "demo"}); !r.Ok {
		t.Fatalf("bind not ok: %v", r.Error)
	}
	cannedHTTPAgent(fa, "hello over h2 path=")

	client, done := h2Client(t, srv.HTTPSAddr())
	defer done()

	resp := h2GetWithRetry(t, client, "https://demo.lvh.me/hi")
	defer func() { _ = resp.Body.Close() }()
	if resp.ProtoMajor != 2 {
		t.Fatalf("response proto = HTTP/%d.%d, want HTTP/2", resp.ProtoMajor, resp.ProtoMinor)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "hello over h2 path=/hi") {
		t.Fatalf("unexpected body: %q", body)
	}
}

// TestEdgeH2Multiplexing fires a burst of concurrent requests over a single h2
// connection and asserts all succeed on HTTP/2 — the win that removes the
// browser's ~6-connection HTTP/1.1 limit for clients that can't reach h3's UDP.
func TestEdgeH2Multiplexing(t *testing.T) {
	srv := startEdgeH2(t)
	fa := dialAgent(t, srv.AgentAddr().String())
	fa.hello(t, "tq_test")
	if r := fa.bind(t, &proto.BindTunnel{ClientTunnelId: "t1", Type: proto.TunnelType_HTTP, Subdomain: "demo"}); !r.Ok {
		t.Fatalf("bind not ok: %v", r.Error)
	}
	cannedHTTPAgent(fa, "ok path=")

	client, done := h2Client(t, srv.HTTPSAddr())
	defer done()

	// Warm one connection so the burst reuses it.
	resp := h2GetWithRetry(t, client, "https://demo.lvh.me/warm")
	_ = resp.Body.Close()

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := fmt.Sprintf("/r%d", i)
			req, _ := http.NewRequest(http.MethodGet, "https://demo.lvh.me"+path, nil)
			r, err := client.Do(req)
			if err != nil {
				errs <- fmt.Errorf("req %d: %w", i, err)
				return
			}
			body, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if r.ProtoMajor != 2 {
				errs <- fmt.Errorf("req %d: proto HTTP/%d, want HTTP/2", i, r.ProtoMajor)
				return
			}
			if want := "ok path=" + path; !strings.Contains(string(body), want) {
				errs <- fmt.Errorf("req %d: body %q missing %q", i, body, want)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
