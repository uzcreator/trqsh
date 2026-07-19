package agent_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rift/rift/internal/agent"
	"github.com/rift/rift/internal/server"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// startEdge boots a real edge on ephemeral ports with the stub entitlements.
func startEdge(t *testing.T) *server.Server {
	t.Helper()
	cfg := server.DefaultConfig()
	cfg.BaseDomain = "lvh.me"
	cfg.QUICAddr = "127.0.0.1:0"
	cfg.TCPAddr = "127.0.0.1:0"
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.HTTPSAddr = "127.0.0.1:0"
	cfg.MetricsAddr = "127.0.0.1:0"
	cfg.HeartbeatInterval = 0
	srv, err := server.New(cfg, testLogger())
	if err != nil {
		t.Fatalf("edge new: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx) }()
	select {
	case <-srv.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("edge not ready")
	}
	return srv
}

func newCore(t *testing.T, edgeAddr string) *agent.Agent {
	t.Helper()
	cfg := agent.DefaultConfig()
	cfg.Server = edgeAddr
	cfg.Insecure = true
	cfg.Transport = "quic" // dial the edge's QUIC listener (its advertised Addr)
	cfg.APIKey = "rk_test"
	core := agent.New(cfg, testLogger())
	t.Cleanup(func() { _ = core.Shutdown(context.Background()) })
	return core
}

func TestMVPEndToEnd(t *testing.T) {
	// Local service the agent forwards to.
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "local ok path="+r.URL.Path)
	}))
	defer local.Close()
	localAddr := strings.TrimPrefix(local.URL, "http://")

	edge := startEdge(t)
	core := newCore(t, edge.AgentAddr().String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tn, err := core.StartTunnel(ctx, agent.TunnelSpec{
		Proto:     "http",
		Addr:      localAddr,
		Subdomain: "demo",
	})
	if err != nil {
		t.Fatalf("StartTunnel: %v", err)
	}
	if tn.PublicURL != "https://demo.lvh.me" {
		t.Fatalf("public url = %q, want https://demo.lvh.me", tn.PublicURL)
	}
	if !core.Status().Connected {
		t.Fatal("agent should be connected")
	}

	// Public request through the edge → agent → local service.
	req, _ := http.NewRequest("GET", "http://"+edge.HTTPAddr().String()+"/hello", nil)
	req.Host = "demo.lvh.me"
	body := doGET(t, req)
	if !strings.Contains(body, "local ok path=/hello") {
		t.Fatalf("unexpected body: %q", body)
	}

	// The inspector should have captured the request.
	if got := len(core.Inspector().List()); got == 0 {
		t.Error("expected inspector to capture the request")
	}
}

func TestLocalControlAPIDrivesCore(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "svc")
	}))
	defer local.Close()
	localAddr := strings.TrimPrefix(local.URL, "http://")

	edge := startEdge(t)
	core := newCore(t, edge.AgentAddr().String())

	apiAddr := "127.0.0.1:" + strconv.Itoa(freePort(t))
	api := agent.NewLocalAPI(core)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = api.Serve(ctx, apiAddr) }()
	waitHTTP(t, "http://"+apiAddr+"/healthz")

	// Start a tunnel via the control API (POST /tunnels).
	spec := `{"proto":"http","addr":"` + localAddr + `","subdomain":"ctl"}`
	resp, err := http.Post("http://"+apiAddr+"/tunnels", "application/json", strings.NewReader(spec))
	if err != nil {
		t.Fatalf("POST /tunnels: %v", err)
	}
	tbody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /tunnels status %d: %s", resp.StatusCode, tbody)
	}
	if !strings.Contains(string(tbody), "https://ctl.lvh.me") {
		t.Fatalf("unexpected tunnel body: %s", tbody)
	}

	// List should show one tunnel.
	list := doGET(t, mustReq(t, "GET", "http://"+apiAddr+"/tunnels"))
	if !strings.Contains(list, "ctl.lvh.me") {
		t.Fatalf("GET /tunnels missing tunnel: %s", list)
	}

	// Status should report connected.
	st := doGET(t, mustReq(t, "GET", "http://"+apiAddr+"/status"))
	if !strings.Contains(st, `"connected":true`) {
		t.Fatalf("GET /status not connected: %s", st)
	}
}

// --- helpers ---

func doGET(t *testing.T, req *http.Request) string {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for i := 0; i < 20; i++ {
		r, err := client.Do(req.Clone(context.Background()))
		if err == nil {
			b, _ := io.ReadAll(r.Body)
			r.Body.Close()
			return string(b)
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("GET failed: %v", lastErr)
	return ""
}

func mustReq(t *testing.T, method, url string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func waitHTTP(t *testing.T, url string) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if r, err := http.Get(url); err == nil {
			r.Body.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("service at %s never came up", url)
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
