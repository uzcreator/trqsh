package server

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// tcpConnPair returns a connected pair of real loopback TCP conns (client side,
// server side). Real TCP (vs net.Pipe) gives kernel buffering and a host:port
// RemoteAddr, so the XFF peer-IP path in proxyToUpstream is exercised too.
func tcpConnPair(t *testing.T) (client, server net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	type accepted struct {
		c   net.Conn
		err error
	}
	ch := make(chan accepted, 1)
	go func() {
		c, err := ln.Accept()
		ch <- accepted{c, err}
	}()
	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	a := <-ch
	if a.err != nil {
		t.Fatalf("accept: %v", a.err)
	}
	return client, a.c
}

// TestReservedUpstreamProxyPoolsConnections drives three sequential requests to a
// reserved host through serveHTTPConn/proxyToUpstream and asserts they reuse ONE
// upstream connection (proving the shared pooled Transport works), while also
// confirming the preserved behaviour: X-Forwarded-* forwarding, HSTS on https,
// and correct per-path round-tripping.
func TestReservedUpstreamProxyPoolsConnections(t *testing.T) {
	var newConns int64
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-Forwarded-Host", r.Header.Get("X-Forwarded-Host"))
		w.Header().Set("X-Seen-Forwarded-Proto", r.Header.Get("X-Forwarded-Proto"))
		w.Header().Set("X-Seen-Forwarded-For", r.Header.Get("X-Forwarded-For"))
		_, _ = io.WriteString(w, "upstream-ok path="+r.URL.Path)
	}))
	upstream.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			atomic.AddInt64(&newConns, 1)
		}
	}
	upstream.Start()
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.SiteUpstream = upstream.URL // apex (lvh.me) is reserved -> site upstream
	s, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	client, srvConn := tcpConnPair(t)
	defer client.Close()
	go s.serveHTTPConn(srvConn, "https")

	br := bufio.NewReader(client)
	for i := 0; i < 3; i++ {
		path := "/p" + strconv.Itoa(i)
		req, _ := http.NewRequest(http.MethodGet, "http://lvh.me"+path, nil)
		req.Host = "lvh.me"
		if err := req.Write(client); err != nil {
			t.Fatalf("write request %d: %v", i, err)
		}
		resp, err := http.ReadResponse(br, req)
		if err != nil {
			t.Fatalf("read response %d: %v", i, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200", i, resp.StatusCode)
		}
		if want := "path=" + path; !strings.Contains(string(body), want) {
			t.Fatalf("request %d: body %q missing %q", i, body, want)
		}
		if got := resp.Header.Get("X-Seen-Forwarded-Host"); got != "lvh.me" {
			t.Fatalf("request %d: upstream saw X-Forwarded-Host %q, want lvh.me", i, got)
		}
		if got := resp.Header.Get("X-Seen-Forwarded-Proto"); got != "https" {
			t.Fatalf("request %d: upstream saw X-Forwarded-Proto %q, want https", i, got)
		}
		if got := resp.Header.Get("X-Seen-Forwarded-For"); got != "127.0.0.1" {
			t.Fatalf("request %d: upstream saw X-Forwarded-For %q, want 127.0.0.1", i, got)
		}
		if got := resp.Header.Get("Strict-Transport-Security"); got == "" {
			t.Fatalf("request %d: missing HSTS header on https response", i)
		}
	}

	if got := atomic.LoadInt64(&newConns); got != 1 {
		t.Fatalf("upstream accepted %d connections across 3 requests, want 1 (pooled reuse)", got)
	}
}

// TestReservedUpstreamProxyPassesRedirectThrough guards the CheckRedirect setting:
// the pooled client must NOT follow upstream redirects (the browser does). A 302
// from the upstream must be relayed verbatim, Location intact.
func TestReservedUpstreamProxyPassesRedirectThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/signin", http.StatusFound) // 302
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.SiteUpstream = upstream.URL
	s, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	client, srvConn := tcpConnPair(t)
	defer client.Close()
	go s.serveHTTPConn(srvConn, "https")

	req, _ := http.NewRequest(http.MethodGet, "http://lvh.me/", nil)
	req.Host = "lvh.me"
	if err := req.Write(client); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(client), req)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status %d, want 302 (redirect must pass through, not be followed)", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/signin" {
		t.Fatalf("Location = %q, want /signin", got)
	}
}

// TestReservedUpstreamProxyUnreachable confirms a dial failure still yields a 502
// with the upstream_unreachable metric label (behaviour preserved from the
// pre-pool manual-dial path).
func TestReservedUpstreamProxyUnreachable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	s, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	client, srvConn := tcpConnPair(t)
	defer client.Close()
	// 127.0.0.1:1 refuses connections -> dial error -> upstream_unreachable.
	go func() {
		req, _ := http.NewRequest(http.MethodGet, "http://lvh.me/", nil)
		req.Host = "lvh.me"
		s.proxyToUpstream(srvConn, req, "https", "http://127.0.0.1:1")
		srvConn.Close()
	}()

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", resp.StatusCode)
	}
	if got := testutil.ToFloat64(s.metrics.Errors.WithLabelValues("upstream_unreachable")); got != 1 {
		t.Fatalf("upstream_unreachable counter = %v, want 1", got)
	}
}
