package server_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/trqsh-uz/trqsh/internal/server"
	"github.com/trqsh-uz/trqsh/pkg/proto"
	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

// startForwardEdge boots an edge that is part of the forwarding mesh: it shares the
// given (mini)redis registry with its peers and runs an internal forwarding
// listener authenticated by the shared token. HeartbeatInterval==0 makes registry
// TTLs infinite, so state persists deterministically without the presence
// refresher. Returns the server and its cancel (to drain it mid-test).
func startForwardEdge(t *testing.T, edgeID, redisURL, token string) (*server.Server, context.CancelFunc) {
	t.Helper()
	cfg := server.DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.EdgeID = edgeID
	cfg.RedisURL = redisURL
	cfg.InternalToken = token
	cfg.ForwardAddr = "127.0.0.1:0" // ephemeral internal listener; advertise resolved addr
	cfg.QUICAddr = "127.0.0.1:0"
	cfg.TCPAddr = "127.0.0.1:0"
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.HTTPSAddr = "127.0.0.1:0"
	cfg.MetricsAddr = "127.0.0.1:0"
	cfg.HeartbeatInterval = 0
	// Cleanup waits for a full drain (see below) so a test HTTP client's idle
	// keep-alive connection — never told to close, since the test just returns —
	// would otherwise pin serveHTTPConn's goroutine open for the full
	// httpIdleTimeout (75s) and make drain()'s wg.Wait() ride its DrainTimeout out.
	// A short DrainTimeout bounds that to a fraction of a second per edge without
	// affecting what's actually under test (drain's forwarding-aware behavior runs
	// synchronously before the wg.Wait()/DrainTimeout wait this shortens).
	cfg.DrainTimeout = 2 * time.Second

	srv, err := server.New(cfg, testLogger())
	if err != nil {
		t.Fatalf("New(%s): %v", edgeID, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Run(ctx); close(done) }()
	// Wait for drain to finish before returning from cleanup so the shared miniredis
	// (closed by a later cleanup) outlives every edge's registry client — otherwise
	// a still-draining edge spams dial retries against a closed miniredis.
	t.Cleanup(func() { cancel(); <-done })
	select {
	case <-srv.Ready():
	case <-time.After(5 * time.Second):
		t.Fatalf("edge %s did not become ready", edgeID)
	}
	return srv, cancel
}

// cannedAgent serves each inbound data stream with a fixed 200 body echoing the
// request path — the same stand-in the local weld test uses, so a passing forward
// test proves the request reached a REAL agent on the owning edge.
func cannedAgent(fa *fakeAgent, body string) {
	fa.acceptStreams(func(st tunnel.Stream) {
		defer func() { _ = st.Close() }()
		if _, err := proto.ReadStreamInit(st); err != nil {
			return
		}
		req, err := http.ReadRequest(bufio.NewReader(st))
		if err != nil {
			return
		}
		b := body + " path=" + req.URL.Path
		r := &http.Response{
			StatusCode: 200, ProtoMajor: 1, ProtoMinor: 1,
			Header:        http.Header{"Content-Type": {"text/plain"}},
			Body:          io.NopCloser(strings.NewReader(b)),
			ContentLength: int64(len(b)),
			Request:       req,
		}
		_ = r.Write(st)
	})
}

func edgeReq(t *testing.T, srv *server.Server, host, path string) *http.Request {
	t.Helper()
	req, err := http.NewRequest("GET", "http://"+srv.HTTPAddr().String()+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Host = host
	return req
}

// TestCrossEdgeHTTPForward is the core proof of this stage: a tunnel bound only on
// edge-A is reachable through a public request that lands on edge-B. The two edges
// are distinct *server.Server instances that share state ONLY through miniredis's
// real TCP listener (not Go-level shared memory), so a pass demonstrates genuine
// cross-process forwarding — edge-B looks the route up in the shared registry,
// dials edge-A's forwarding listener, and edge-A welds to its real local agent.
func TestCrossEdgeHTTPForward(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	redisURL := "redis://" + mr.Addr()
	const token = "test-internal-token"

	edgeA, _ := startForwardEdge(t, "edge-a", redisURL, token)
	edgeB, _ := startForwardEdge(t, "edge-b", redisURL, token)

	// Bind demo.lvh.me on edge-A ONLY, backed by a real agent session.
	fa := dialAgent(t, edgeA.AgentAddr().String())
	fa.hello(t, "tq_test")
	resp := fa.bind(t, &proto.BindTunnel{
		ClientTunnelId: "t1",
		Type:           proto.TunnelType_HTTP,
		Subdomain:      "demo",
	})
	if !resp.Ok {
		t.Fatalf("bind not ok: %v", resp.Error)
	}
	if resp.AssignedHost != "demo.lvh.me" {
		t.Fatalf("assigned host = %q, want demo.lvh.me", resp.AssignedHost)
	}
	cannedAgent(fa, "hello from edge-a agent")

	// edge-B has no agent session for demo.lvh.me, so any successful response it
	// returns can ONLY have been forwarded to edge-A and welded to edge-A's agent.
	// CloseIdleConnections in cleanup (LIFO: runs before the edges' own cleanup)
	// drops the kept-alive public+forwarded conns instead of leaving them for
	// drain()'s wg.Wait() to sit out via DrainTimeout.
	client := &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	got := doWithRetry(t, client, edgeReq(t, edgeB, "demo.lvh.me", "/hi"))
	if !strings.Contains(got, "hello from edge-a agent path=/hi") {
		t.Fatalf("forwarded body = %q, want it served by edge-a's agent", got)
	}

	// The request that landed on edge-B must have been served by edge-A (against a
	// tunnel edge-B has no session for), so it can ONLY have been forwarded.
	got2 := doWithRetry(t, client, edgeReq(t, edgeB, "demo.lvh.me", "/second"))
	if !strings.Contains(got2, "path=/second") {
		t.Fatalf("second forwarded body = %q", got2)
	}
}

// TestCrossEdgeUnknownHost404 guards against regressions: a truly unknown host still
// gets the branded 404 on BOTH edges (the registry fallback finds nothing to
// forward and falls through to today's behavior).
func TestCrossEdgeUnknownHost404(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	redisURL := "redis://" + mr.Addr()
	const token = "test-internal-token"

	edgeA, _ := startForwardEdge(t, "edge-a", redisURL, token)
	edgeB, _ := startForwardEdge(t, "edge-b", redisURL, token)

	client := &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	for _, e := range []struct {
		name string
		srv  *server.Server
	}{{"edge-a", edgeA}, {"edge-b", edgeB}} {
		r, err := client.Do(edgeReq(t, e.srv, "nope.lvh.me", "/"))
		if err != nil {
			t.Fatalf("%s request: %v", e.name, err)
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if r.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", e.name, r.StatusCode)
		}
		if !strings.Contains(string(body), "offline") {
			t.Fatalf("%s expected branded 404, got %q", e.name, string(body))
		}
	}
}

// TestCrossEdgeDrainStopsForwarding exercises the drain-awareness mechanism: once
// edge-A drains it removes its advertised forwarding address, so edge-B's next
// forward attempt must fail cleanly (a fast 5xx/4xx) rather than hang against a
// shutting-down peer.
func TestCrossEdgeDrainStopsForwarding(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	redisURL := "redis://" + mr.Addr()
	const token = "test-internal-token"

	edgeA, cancelA := startForwardEdge(t, "edge-a", redisURL, token)
	edgeB, _ := startForwardEdge(t, "edge-b", redisURL, token)

	fa := dialAgent(t, edgeA.AgentAddr().String())
	fa.hello(t, "tq_test")
	resp := fa.bind(t, &proto.BindTunnel{ClientTunnelId: "t1", Type: proto.TunnelType_HTTP, Subdomain: "demo"})
	if !resp.Ok {
		t.Fatalf("bind not ok: %v", resp.Error)
	}
	cannedAgent(fa, "hello from edge-a agent")

	// DisableKeepAlives so every request dials a fresh public conn into edge-B and
	// thus makes a FRESH forward decision. Without this, http.Client would reuse the
	// TCP connection already welded through to edge-A end-to-end for the pre-drain
	// request — that weld legitimately keeps working after drain starts (an
	// in-flight connection someone already committed to is not torn down mid-flight;
	// see server.go's drain(), which only stops NEW routing, not existing welds) and
	// the test would wrongly observe "still 200" forever, never exercising the
	// post-drain lookup path this test is actually about.
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	if got := doWithRetry(t, client, edgeReq(t, edgeB, "demo.lvh.me", "/hi")); !strings.Contains(got, "hello from edge-a agent") {
		t.Fatalf("pre-drain forward failed: %q", got)
	}

	// Drain edge-A. UnregisterEdge runs early in drain(), removing edge-A's address
	// from the shared registry; edge-B must converge to a clean failure quickly on
	// the next FRESH request (each iteration below opens a brand-new connection).
	cancelA()

	deadline := time.Now().Add(5 * time.Second)
	for {
		r, err := client.Do(edgeReq(t, edgeB, "demo.lvh.me", "/hi"))
		if err != nil {
			// A clean transport error is acceptable post-drain, but a hang is not:
			// the 3s client timeout bounds each attempt, so we simply keep polling
			// until the deadline and fail if we never see a clean response.
			if time.Now().After(deadline) {
				t.Fatalf("edge-B forward hung/errored after edge-A drained: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		status := r.StatusCode
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		if status >= 400 {
			return // clean failure (502 unreachable, or 404 once the route lapses)
		}
		if time.Now().After(deadline) {
			t.Fatalf("edge-B still returned %d after edge-A drained; want a clean >=400", status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestCrossEdgeWebSocketUpgradeForward proves the OTHER branch of serveHTTPConn —
// isUpgrade — also welds correctly across a forwarded hop, not just the plain
// request/response path TestCrossEdgeHTTPForward covers. This path replays the
// request head TWICE (edge-B -> edge-A over the forward hop, then edge-A -> its own
// agent) and switches both edges independently into a raw byte weld once each
// parses the Upgrade header off its own copy of the replayed request — worth
// covering on its own because it is where a real double-count bug was found and
// fixed during this stage (a forwardedConn's bytes were being counted once by the
// relaying edge and again by the owning edge; see the forwardedConn guard around
// s.metrics.countBytes in ingress_http.go's isUpgrade branch).
func TestCrossEdgeWebSocketUpgradeForward(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	redisURL := "redis://" + mr.Addr()
	const token = "test-internal-token"

	edgeA, _ := startForwardEdge(t, "edge-a", redisURL, token)
	edgeB, _ := startForwardEdge(t, "edge-b", redisURL, token)

	fa := dialAgent(t, edgeA.AgentAddr().String())
	fa.hello(t, "tq_test")
	resp := fa.bind(t, &proto.BindTunnel{ClientTunnelId: "ws1", Type: proto.TunnelType_HTTP, Subdomain: "ws"})
	if !resp.Ok {
		t.Fatalf("bind not ok: %v", resp.Error)
	}

	// The agent parses the (twice-replayed) request head, answers 101 by hand (full
	// control over the exact bytes, avoiding any http.Response.Write opinions about
	// a 101's framing), then echoes raw bytes for the life of the stream — the same
	// "parse head, then go raw" shape serveHTTPConn's isUpgrade branch expects on
	// both hops.
	fa.acceptStreams(func(st tunnel.Stream) {
		defer func() { _ = st.Close() }()
		if _, err := proto.ReadStreamInit(st); err != nil {
			return
		}
		br := bufio.NewReader(st)
		if _, err := http.ReadRequest(br); err != nil {
			return
		}
		if _, err := io.WriteString(st, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"); err != nil {
			return
		}
		_, _ = io.Copy(st, br)
	})

	conn, err := net.Dial("tcp", edgeB.HTTPAddr().String())
	if err != nil {
		t.Fatalf("dial edge-b: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	const reqLine = "GET /socket HTTP/1.1\r\n" +
		"Host: ws.lvh.me\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: websocket\r\n\r\n"
	if _, err := io.WriteString(conn, reqLine); err != nil {
		t.Fatalf("write upgrade request: %v", err)
	}

	br := bufio.NewReader(conn)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status line: %v", err)
	}
	if !strings.HasPrefix(statusLine, "HTTP/1.1 101") {
		t.Fatalf("status line = %q, want 101 Switching Protocols (forwarded upgrade did not reach the agent)", statusLine)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read headers: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	const payload = "ping-through-forwarded-ws"
	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(br, got); err != nil {
		t.Fatalf("read echoed payload: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("echoed payload = %q, want %q", got, payload)
	}
}
