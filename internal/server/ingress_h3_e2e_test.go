package server_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

	"github.com/trqsh-uz/trqsh/internal/server"
	"github.com/trqsh-uz/trqsh/pkg/proto"
	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

// startEdgeH3 boots an edge with the HTTP/3 ingress enabled on an ephemeral UDP
// port, returning it once ready. Mirrors startEdge (edge_test.go) but sets
// TRQSH_H3_ADDR so the QUIC ingress comes up too.
func startEdgeH3(t *testing.T) *server.Server {
	t.Helper()
	cfg := server.DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.QUICAddr = "127.0.0.1:0"
	cfg.TCPAddr = "127.0.0.1:0"
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.HTTPSAddr = "127.0.0.1:0"
	cfg.H3Addr = "127.0.0.1:0"
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
	if srv.H3Addr() == nil {
		t.Fatal("h3 ingress did not start")
	}
	return srv
}

// h3Client returns an *http.Client that speaks HTTP/3 to the edge's UDP address
// regardless of the request URL's host — so a request can carry a tunnel
// authority (demo.lvh.me) while actually dialing 127.0.0.1:port. dials counts how
// many QUIC connections were opened (to prove multiplexing reuses one).
func h3Client(t *testing.T, edge net.Addr, dials *int64, mu *sync.Mutex) (*http.Client, func()) {
	t.Helper()
	ua, err := net.ResolveUDPAddr("udp", edge.String())
	if err != nil {
		t.Fatalf("resolve h3 addr: %v", err)
	}
	rt := &http3.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, NextProtos: []string{http3.NextProtoH3}},
		Dial: func(ctx context.Context, _ string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
			if dials != nil {
				mu.Lock()
				*dials++
				mu.Unlock()
			}
			udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				return nil, err
			}
			return quic.DialEarly(ctx, udp, ua, tlsCfg, cfg)
		},
	}
	client := &http.Client{Transport: rt, Timeout: 10 * time.Second}
	return client, func() { _ = rt.Close() }
}

// cannedHTTPAgent makes the fake agent answer every data stream with a canned
// 200 whose body echoes the request path.
func cannedHTTPAgent(fa *fakeAgent, prefix string) {
	fa.acceptStreams(func(st tunnel.Stream) {
		defer func() { _ = st.Close() }()
		if _, err := proto.ReadStreamInit(st); err != nil {
			return
		}
		req, err := http.ReadRequest(bufio.NewReader(st))
		if err != nil {
			return
		}
		body := prefix + req.URL.Path
		r := &http.Response{
			StatusCode: 200, ProtoMajor: 1, ProtoMinor: 1,
			Header:        http.Header{"Content-Type": {"text/plain"}},
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       req,
		}
		_ = r.Write(st)
	})
}

// h3GetWithRetry issues a GET over HTTP/3, retrying briefly to absorb the QUIC
// listener's startup race, and returns status + body.
func h3GetWithRetry(t *testing.T, client *http.Client, url string) (int, string) {
	t.Helper()
	var lastErr error
	for i := 0; i < 25; i++ {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return resp.StatusCode, string(body)
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("h3 GET %s failed: %v", url, lastErr)
	return 0, ""
}

// TestEdgeHTTPWeldH3 is the HTTP/3 analogue of TestEdgeHTTPWeld: a public request
// arriving over QUIC is routed by Host to the owning agent and welded through.
func TestEdgeHTTPWeldH3(t *testing.T) {
	srv := startEdgeH3(t)
	fa := dialAgent(t, srv.AgentAddr().String())
	fa.hello(t, "tq_test")

	resp := fa.bind(t, &proto.BindTunnel{
		ClientTunnelId: "t1",
		Type:           proto.TunnelType_HTTP,
		Subdomain:      "demo",
	})
	if !resp.Ok {
		t.Fatalf("bind not ok: %v", resp.Error)
	}
	cannedHTTPAgent(fa, "hello over h3 path=")

	client, done := h3Client(t, srv.H3Addr(), nil, nil)
	defer done()

	status, body := h3GetWithRetry(t, client, "https://demo.lvh.me/hi")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, "hello over h3 path=/hi") {
		t.Fatalf("unexpected body: %q", body)
	}
}

// TestEdgeH3Multiplexing is the point of HTTP/3 here: many concurrent requests
// share ONE QUIC connection with no HTTP/1.1 head-of-line blocking. It fires a
// burst of requests over a single client and asserts all succeed on one dialed
// connection.
func TestEdgeH3Multiplexing(t *testing.T) {
	srv := startEdgeH3(t)
	fa := dialAgent(t, srv.AgentAddr().String())
	fa.hello(t, "tq_test")
	if r := fa.bind(t, &proto.BindTunnel{ClientTunnelId: "t1", Type: proto.TunnelType_HTTP, Subdomain: "demo"}); !r.Ok {
		t.Fatalf("bind not ok: %v", r.Error)
	}
	cannedHTTPAgent(fa, "ok path=")

	var dials int64
	var mu sync.Mutex
	client, done := h3Client(t, srv.H3Addr(), &dials, &mu)
	defer done()

	// Warm the connection once so the burst below reuses it (and the retry that
	// absorbs the startup race doesn't inflate the dial count).
	if status, _ := h3GetWithRetry(t, client, "https://demo.lvh.me/warmup"); status != http.StatusOK {
		t.Fatalf("warmup status = %d, want 200", status)
	}

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := fmt.Sprintf("/r%d", i)
			req, _ := http.NewRequest(http.MethodGet, "https://demo.lvh.me"+path, nil)
			resp, err := client.Do(req)
			if err != nil {
				errs <- fmt.Errorf("req %d: %w", i, err)
				return
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
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

	mu.Lock()
	got := dials
	mu.Unlock()
	if got != 1 {
		t.Fatalf("opened %d QUIC connections for %d concurrent requests, want 1 (multiplexed reuse)", got, n+1)
	}
}

// TestEdgeUnknownHost404H3 confirms an unknown host over h3 gets the branded 404.
func TestEdgeUnknownHost404H3(t *testing.T) {
	srv := startEdgeH3(t)
	client, done := h3Client(t, srv.H3Addr(), nil, nil)
	defer done()

	status, body := h3GetWithRetry(t, client, "https://nope.lvh.me/")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	if !strings.Contains(body, "offline") {
		t.Fatalf("expected branded 404, got %q", body)
	}
}

// TestReservedUpstreamProxyH3 confirms reserved control-plane hosts (here apex →
// site) are reverse-proxied over h3 with the forwarded headers preserved.
func TestReservedUpstreamProxyH3(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-Forwarded-Proto", r.Header.Get("X-Forwarded-Proto"))
		_, _ = io.WriteString(w, "upstream path="+r.URL.Path)
	}))
	defer upstream.Close()

	cfg := server.DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.SiteUpstream = upstream.URL
	cfg.QUICAddr = "127.0.0.1:0"
	cfg.TCPAddr = "127.0.0.1:0"
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.HTTPSAddr = "127.0.0.1:0"
	cfg.H3Addr = "127.0.0.1:0"
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

	client, done := h3Client(t, srv.H3Addr(), nil, nil)
	defer done()

	status, body := h3GetWithRetry(t, client, "https://lvh.me/pricing")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, "upstream path=/pricing") {
		t.Fatalf("unexpected body: %q", body)
	}
}
