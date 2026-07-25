package server_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/trqsh-uz/trqsh/internal/server"
	"github.com/trqsh-uz/trqsh/pkg/proto"
	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startEdge boots a Server on ephemeral ports and returns it once ready.
func startEdge(t *testing.T, portMin, portMax int) *server.Server {
	t.Helper()
	cfg := server.DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.QUICAddr = "127.0.0.1:0"
	cfg.TCPAddr = "127.0.0.1:0"
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.HTTPSAddr = "127.0.0.1:0"
	cfg.MetricsAddr = "127.0.0.1:0"
	cfg.HeartbeatInterval = 0
	if portMin > 0 {
		cfg.PortMin, cfg.PortMax = portMin, portMax
	}

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

// fakeAgent is a minimal Part 03 stand-in built on the Part 01 transport.
type fakeAgent struct {
	sess tunnel.Session
	ctrl tunnel.Stream
}

func dialAgent(t *testing.T, addr string) *fakeAgent {
	t.Helper()
	d := &tunnel.Dialer{
		TLSConfig:   &tls.Config{InsecureSkipVerify: true, NextProtos: []string{tunnel.ALPNProto}},
		ForceKind:   tunnel.KindQUIC,
		DialTimeout: 5 * time.Second,
	}
	sess, err := d.Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("agent dial: %v", err)
	}
	t.Cleanup(func() { sess.CloseWithError(0, "test done") })

	ctrl, err := sess.OpenStream(context.Background())
	if err != nil {
		t.Fatalf("open control stream: %v", err)
	}
	fa := &fakeAgent{sess: sess, ctrl: ctrl}
	return fa
}

func (fa *fakeAgent) hello(t *testing.T, apiKey string) {
	t.Helper()
	err := proto.WriteMsg(fa.ctrl, &proto.Envelope{Msg: &proto.Envelope_Hello{Hello: &proto.Hello{
		ProtocolVersion: "1",
		AgentVersion:    "test",
		ApiKey:          apiKey,
	}}})
	if err != nil {
		t.Fatalf("write hello: %v", err)
	}
	env, err := proto.ReadMsg(fa.ctrl)
	if err != nil {
		t.Fatalf("read auth resp: %v", err)
	}
	ar := env.GetAuthResp()
	if ar == nil || !ar.Ok {
		t.Fatalf("auth failed: %v", env)
	}
}

func (fa *fakeAgent) bind(t *testing.T, b *proto.BindTunnel) *proto.BindResp {
	t.Helper()
	if err := proto.WriteMsg(fa.ctrl, &proto.Envelope{Msg: &proto.Envelope_Bind{Bind: b}}); err != nil {
		t.Fatalf("write bind: %v", err)
	}
	env, err := proto.ReadMsg(fa.ctrl)
	if err != nil {
		t.Fatalf("read bind resp: %v", err)
	}
	br := env.GetBindResp()
	if br == nil {
		t.Fatalf("expected BindResp, got %v", env)
	}
	return br
}

// acceptStreams runs handler for every inbound data stream until the session dies.
func (fa *fakeAgent) acceptStreams(handler func(tunnel.Stream)) {
	go func() {
		for {
			st, err := fa.sess.AcceptStream(context.Background())
			if err != nil {
				return
			}
			go handler(st)
		}
	}()
}

func TestEdgeHTTPWeld(t *testing.T) {
	srv := startEdge(t, 0, 0)
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
	if resp.AssignedHost != "demo.lvh.me" {
		t.Fatalf("assigned host = %q, want demo.lvh.me", resp.AssignedHost)
	}

	// Agent serves each request stream with a canned response.
	fa.acceptStreams(func(st tunnel.Stream) {
		defer st.Close()
		if _, err := proto.ReadStreamInit(st); err != nil {
			return
		}
		req, err := http.ReadRequest(bufio.NewReader(st))
		if err != nil {
			return
		}
		body := "hello from agent path=" + req.URL.Path
		r := &http.Response{
			StatusCode: 200, ProtoMajor: 1, ProtoMinor: 1,
			Header:        http.Header{"Content-Type": {"text/plain"}},
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       req,
		}
		_ = r.Write(st)
	})

	// Public request routed by Host header.
	httpAddr := srv.HTTPAddr().String()
	req, _ := http.NewRequest("GET", "http://"+httpAddr+"/hi", nil)
	req.Host = "demo.lvh.me"
	client := &http.Client{Timeout: 5 * time.Second}

	got := doWithRetry(t, client, req)
	if !strings.Contains(got, "hello from agent path=/hi") {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestEdgeUnknownHost404(t *testing.T) {
	srv := startEdge(t, 0, 0)
	httpAddr := srv.HTTPAddr().String()
	req, _ := http.NewRequest("GET", "http://"+httpAddr+"/", nil)
	req.Host = "nope.lvh.me"
	client := &http.Client{Timeout: 5 * time.Second}
	r, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", r.StatusCode)
	}
	body, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(body), "offline") {
		t.Fatalf("expected branded 404, got %q", string(body))
	}
}

// A reserved control-plane host (apex/www/app/api) served over plain HTTP must
// 301 to HTTPS so browsers never stay on an insecure connection.
func TestReservedHostHTTPSRedirect(t *testing.T) {
	cfg := server.DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.SiteUpstream = "http://127.0.0.1:1" // marks apex reserved; never dialed (redirect wins)
	cfg.QUICAddr = "127.0.0.1:0"
	cfg.TCPAddr = "127.0.0.1:0"
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.HTTPSAddr = "127.0.0.1:0"
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

	req, _ := http.NewRequest("GET", "http://"+srv.HTTPAddr().String()+"/pricing?x=1", nil)
	req.Host = "lvh.me"
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	var r *http.Response
	for i := 0; i < 20; i++ {
		if r, err = client.Do(req); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", r.StatusCode)
	}
	if loc := r.Header.Get("Location"); loc != "https://lvh.me/pricing?x=1" {
		t.Fatalf("Location = %q, want https://lvh.me/pricing?x=1", loc)
	}
}

func TestEdgeTCPWeld(t *testing.T) {
	port := freePort(t)
	srv := startEdge(t, port, port)
	fa := dialAgent(t, srv.AgentAddr().String())
	fa.hello(t, "tq_test")

	resp := fa.bind(t, &proto.BindTunnel{
		ClientTunnelId: "tcp1",
		Type:           proto.TunnelType_TCP,
		RemotePort:     uint32(port),
	})
	if !resp.Ok {
		t.Fatalf("bind not ok: %v", resp.Error)
	}
	if int(resp.AssignedPort) != port {
		t.Fatalf("assigned port = %d, want %d", resp.AssignedPort, port)
	}

	// Agent echoes raw bytes on each stream.
	fa.acceptStreams(func(st tunnel.Stream) {
		defer st.Close()
		if _, err := proto.ReadStreamInit(st); err != nil {
			return
		}
		io.Copy(st, st)
	})

	// Public TCP client through the assigned port.
	var conn net.Conn
	var err error
	for i := 0; i < 20; i++ {
		conn, err = net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial tcp tunnel: %v", err)
	}
	defer conn.Close()

	msg := []byte("ping-through-tunnel")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, len(msg))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != string(msg) {
		t.Fatalf("echo mismatch: got %q want %q", buf, msg)
	}
}

// doWithRetry retries a GET briefly to absorb startup races, returning the body.
func doWithRetry(t *testing.T, client *http.Client, req *http.Request) string {
	t.Helper()
	var lastErr error
	for i := 0; i < 20; i++ {
		r, err := client.Do(req)
		if err == nil {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			return string(body)
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("request failed: %v", lastErr)
	return ""
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
