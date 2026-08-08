package agent_test

// TestRemotePairingRealEndToEnd is the one test that exercises BOTH real
// implementations of the /qr relay against each other — internal/api/remote.go
// (control plane) and internal/agent/remoteapi.go (daemon) — with nothing
// faked on either side. internal/api's own remote_test.go and
// internal/agent's remoteapi_test.go each already cover their half in
// isolation (against a fake peer); this is the seam between them, driven the
// same way the real system is: a daemon starts the pairing, and a "phone"
// (a plain http.Client hitting the control plane directly, standing in for
// what web/dashboard's SSE proxy relays) drives the other side.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/api"
)

// startRealControlPlane boots a real internal/api.Server (in-memory store,
// password-less dev auth) on an httptest server.
func startRealControlPlane(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.DevAuth = true
	srv, err := api.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

// signupForAPIKey creates a real account on the real control plane and
// returns a real API key for it, the same way the CLI's device-flow login
// would end up with one.
func signupForAPIKey(t *testing.T, apiURL string) string {
	t.Helper()
	signupBody, _ := json.Marshal(map[string]string{"email": "qr-e2e@example.com", "name": "E2E"})
	resp, err := http.Post(apiURL+"/v1/auth/signup", "application/json", bytes.NewReader(signupBody))
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	var signup struct {
		Tokens struct {
			Access string `json:"access_token"`
		} `json:"tokens"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&signup)
	_ = resp.Body.Close()
	if signup.Tokens.Access == "" {
		t.Fatal("signup did not return an access token")
	}

	keyBody, _ := json.Marshal(map[string]string{"name": "e2e"})
	req, _ := http.NewRequest(http.MethodPost, apiURL+"/v1/api-keys", bytes.NewReader(keyBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signup.Tokens.Access)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	var key struct {
		APIKey string `json:"api_key"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&key)
	if !strings.HasPrefix(key.APIKey, "tq_live_") {
		t.Fatalf("expected a real api key, got %q (status %d)", key.APIKey, resp2.StatusCode)
	}
	return key.APIKey
}

func TestRemotePairingRealEndToEnd(t *testing.T) {
	cp := startRealControlPlane(t)
	apiKey := signupForAPIKey(t, cp.URL)

	t.Setenv("TRQSH_CONFIG", t.TempDir()+"/trqsh.yml")
	t.Setenv("TRQSH_API_KEY", apiKey)
	t.Setenv("TRQSH_API_URL", cp.URL)

	// The daemon (LocalAPI), exactly as the TUI would drive it.
	l := agent.NewLocalAPI(nil)
	daemon := httptest.NewServer(l.Handler())
	t.Cleanup(daemon.Close)

	eventsReq, _ := http.NewRequest(http.MethodGet, daemon.URL+"/events", nil)
	eventsResp, err := http.DefaultClient.Do(eventsReq)
	if err != nil {
		t.Fatalf("connect daemon /events: %v", err)
	}
	t.Cleanup(func() { _ = eventsResp.Body.Close() })
	daemonEvents := bufio.NewScanner(eventsResp.Body)

	nextDaemonEvent := func(timeout time.Duration) agent.Event {
		t.Helper()
		done := make(chan agent.Event, 1)
		go func() {
			for daemonEvents.Scan() {
				data, ok := strings.CutPrefix(daemonEvents.Text(), "data: ")
				if !ok {
					continue
				}
				var ev agent.Event
				if json.Unmarshal([]byte(data), &ev) == nil {
					done <- ev
					return
				}
			}
			if err := daemonEvents.Err(); err != nil {
				t.Logf("daemon /events scan ended: %v", err)
			}
		}()
		select {
		case ev := <-done:
			return ev
		case <-time.After(timeout):
			t.Fatal("timed out waiting for a daemon /events event")
			return agent.Event{}
		}
	}

	// 1. The console starts a pairing.
	startResp, err := http.Post(daemon.URL+"/remote/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /remote/start: %v", err)
	}
	var start struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	_ = json.NewDecoder(startResp.Body).Decode(&start)
	_ = startResp.Body.Close()
	if startResp.StatusCode != http.StatusOK || start.Code == "" {
		t.Fatalf("start: status %d code %q", startResp.StatusCode, start.Code)
	}
	if !strings.Contains(start.URL, start.Code) {
		t.Fatalf("returned url %q should contain the code %q", start.URL, start.Code)
	}

	// 2. The "phone" — a plain client hitting the control plane directly,
	// standing in for what web/dashboard's SSE proxy relays — connects to the
	// viewer stream for that exact code.
	viewerReq, _ := http.NewRequest(http.MethodGet, cp.URL+"/v1/remote/sessions/"+start.Code+"/viewer", nil)
	viewerResp, err := http.DefaultClient.Do(viewerReq)
	if err != nil {
		t.Fatalf("connect viewer stream: %v", err)
	}
	t.Cleanup(func() { _ = viewerResp.Body.Close() })
	if viewerResp.StatusCode != http.StatusOK {
		t.Fatalf("viewer stream: status %d", viewerResp.StatusCode)
	}
	viewerEvents := bufio.NewScanner(viewerResp.Body)
	nextViewerEvent := func(timeout time.Duration) map[string]any {
		t.Helper()
		done := make(chan map[string]any, 1)
		go func() {
			for viewerEvents.Scan() {
				data, ok := strings.CutPrefix(viewerEvents.Text(), "data: ")
				if !ok {
					continue
				}
				var ev map[string]any
				if json.Unmarshal([]byte(data), &ev) == nil {
					done <- ev
					return
				}
			}
			if err := viewerEvents.Err(); err != nil {
				t.Logf("viewer stream scan ended: %v", err)
			}
		}()
		select {
		case ev := <-done:
			return ev
		case <-time.After(timeout):
			t.Fatal("timed out waiting for a viewer event")
			return nil
		}
	}

	// The phone sees the console as connected almost immediately.
	if ev := nextViewerEvent(3 * time.Second); ev["type"] != "presence" || ev["connected"] != true {
		t.Fatalf("expected connected presence on the viewer stream, got %#v", ev)
	}

	// 3. The console publishes a line + state snapshot; the real control
	// plane relays it to the real phone.
	pubBody, _ := json.Marshal(map[string]any{"type": "lines", "lines": []string{"❯ /http 3000", "✓ https://abc.trqsh.uz"}})
	pubResp, err := http.Post(daemon.URL+"/remote/publish", "application/json", bytes.NewReader(pubBody))
	if err != nil {
		t.Fatalf("POST /remote/publish: %v", err)
	}
	_ = pubResp.Body.Close()
	if ev := nextViewerEvent(3 * time.Second); ev["type"] != "lines" {
		t.Fatalf("expected the published lines to reach the real phone, got %#v", ev)
	}

	// 4. The phone sends a command through the real control plane; it
	// reaches the real daemon as a "remote" Event, the same channel the TUI
	// already listens on for everything else.
	cmdBody, _ := json.Marshal(map[string]string{"text": "/status"})
	cmdResp, err := http.Post(cp.URL+"/v1/remote/sessions/"+start.Code+"/command", "application/json", bytes.NewReader(cmdBody))
	if err != nil {
		t.Fatalf("POST command: %v", err)
	}
	_ = cmdResp.Body.Close()
	if cmdResp.StatusCode != http.StatusNoContent {
		t.Fatalf("command: status %d", cmdResp.StatusCode)
	}
	ev := nextDaemonEvent(3 * time.Second)
	if ev.Type != "remote" || ev.Remote == nil || ev.Remote.Kind != "command" || ev.Remote.Command != "/status" {
		t.Fatalf("expected the command to reach the real daemon as a remote Event, got %#v", ev)
	}

	// 5. Ending the pairing from the console notifies the real phone.
	stopResp, err := http.Post(daemon.URL+"/remote/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /remote/stop: %v", err)
	}
	_ = stopResp.Body.Close()
	if ev := nextViewerEvent(3 * time.Second); ev["type"] != "ended" {
		t.Fatalf("expected the real phone to see ended, got %#v", ev)
	}
}
