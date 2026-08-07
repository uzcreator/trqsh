package api_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// sseClient is a minimal SSE reader for the /remote endpoints' event stream,
// used the same way the TUI's own client.streamEvents consumes the daemon's
// /events: scan for "data: " lines, ignore comments/keepalives.
type sseClient struct {
	t    *testing.T
	resp *http.Response
	sc   *bufio.Scanner
}

func connectSSE(t *testing.T, url, bearer string) *sseClient {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connect SSE %s: status %d", url, resp.StatusCode)
	}
	c := &sseClient{t: t, resp: resp, sc: bufio.NewScanner(resp.Body)}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return c
}

// next reads events until one decodes successfully, or fails the test after
// within timeout with nothing but comments/keepalives.
func (c *sseClient) next(timeout time.Duration) map[string]any {
	c.t.Helper()
	done := make(chan map[string]any, 1)
	go func() {
		for c.sc.Scan() {
			line := c.sc.Text()
			data, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}
			var ev map[string]any
			if json.Unmarshal([]byte(data), &ev) == nil {
				done <- ev
				return
			}
		}
	}()
	select {
	case ev := <-done:
		return ev
	case <-time.After(timeout):
		c.t.Fatalf("timed out waiting for an SSE event")
		return nil
	}
}

func TestRemotePairingRoundTrip(t *testing.T) {
	ts := testServer(t)
	_, apiKey := signupAndKey(t, ts, "remote-a@example.com")

	var created struct {
		Code      string `json:"code"`
		URL       string `json:"url"`
		ExpiresAt string `json:"expires_at"`
	}
	resp := postJSON(t, ts.URL+"/v1/remote/sessions", apiKey, nil, &created)
	if resp.StatusCode != http.StatusCreated || created.Code == "" {
		t.Fatalf("create session: status %d code %q", resp.StatusCode, created.Code)
	}
	if !strings.Contains(created.URL, created.Code) {
		t.Fatalf("url %q should contain the code %q", created.URL, created.Code)
	}

	agent := connectSSE(t, ts.URL+"/v1/remote/sessions/"+created.Code+"/agent", apiKey)
	viewer := connectSSE(t, ts.URL+"/v1/remote/sessions/"+created.Code+"/viewer", "")

	// The viewer immediately sees agent presence (attachAgent broadcasts before
	// the create call's caller can race ahead — connectSSE already waited for
	// headers, so by the time we ask the viewer for its next event both legs
	// are live).
	if ev := viewer.next(3 * time.Second); ev["type"] != "presence" || ev["connected"] != true {
		t.Fatalf("expected connected presence, got %#v", ev)
	}

	// Agent (daemon) publishes transcript lines; the viewer (phone) sees them.
	pub := postJSON(t, ts.URL+"/v1/remote/sessions/"+created.Code+"/publish", apiKey,
		map[string]any{"type": "lines", "lines": []string{"❯ /ls", "no tunnels"}}, nil)
	if pub.StatusCode != http.StatusNoContent {
		t.Fatalf("publish: status %d", pub.StatusCode)
	}
	ev := viewer.next(3 * time.Second)
	if ev["type"] != "lines" {
		t.Fatalf("expected a lines event, got %#v", ev)
	}
	lines, _ := ev["lines"].([]any)
	if len(lines) != 2 || lines[0] != "❯ /ls" {
		t.Fatalf("unexpected lines payload: %#v", ev["lines"])
	}

	// Viewer (phone) sends a command; the agent (daemon) receives it.
	cmd := postJSON(t, ts.URL+"/v1/remote/sessions/"+created.Code+"/command", "",
		map[string]string{"text": "/status"}, nil)
	if cmd.StatusCode != http.StatusNoContent {
		t.Fatalf("command: status %d", cmd.StatusCode)
	}
	ev = agent.next(3 * time.Second)
	if ev["type"] != "command" || ev["command"] != "/status" {
		t.Fatalf("expected the command to reach the agent, got %#v", ev)
	}

	// A late-joining viewer replays the backlog instead of starting blank.
	late := connectSSE(t, ts.URL+"/v1/remote/sessions/"+created.Code+"/viewer", "")
	ev = late.next(3 * time.Second)
	if ev["type"] != "lines" {
		t.Fatalf("expected the late viewer to be replayed the backlog, got %#v", ev)
	}

	// Ending the session notifies both legs and a fresh viewer gets 404.
	end := postJSON(t, ts.URL+"/v1/remote/sessions/"+created.Code+"/end", apiKey, nil, nil)
	if end.StatusCode != http.StatusNoContent {
		t.Fatalf("end: status %d", end.StatusCode)
	}
	if ev := viewer.next(3 * time.Second); ev["type"] != "ended" {
		t.Fatalf("expected the viewer to see ended, got %#v", ev)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/remote/sessions/"+created.Code+"/viewer", nil)
	r2, _ := http.DefaultClient.Do(req)
	if r2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for an ended session, got %d", r2.StatusCode)
	}
	_ = r2.Body.Close()
}

func TestRemoteSessionIsolationAndUnknownCode(t *testing.T) {
	ts := testServer(t)
	_, keyA := signupAndKey(t, ts, "remote-b@example.com")
	_, keyB := signupAndKey(t, ts, "remote-c@example.com")

	var created struct {
		Code string `json:"code"`
	}
	postJSON(t, ts.URL+"/v1/remote/sessions", keyA, nil, &created)

	// A different org's key can't attach as the agent to org A's session.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/remote/sessions/"+created.Code+"/agent", nil)
	req.Header.Set("Authorization", "Bearer "+keyB)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org agent attach should 404, got %d", resp.StatusCode)
	}

	// An unauthenticated create is rejected outright.
	r2 := postJSON(t, ts.URL+"/v1/remote/sessions", "", nil, nil)
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 creating without auth, got %d", r2.StatusCode)
	}

	// A made-up viewer code 404s rather than hanging or panicking.
	r3, _ := http.Get(ts.URL + "/v1/remote/sessions/NOPE-NOPE-NOPE/viewer")
	if r3.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown code should 404, got %d", r3.StatusCode)
	}
	_ = r3.Body.Close()

	// Sending a command with nobody attached reports 409, not a silent drop.
	r4 := postJSON(t, ts.URL+"/v1/remote/sessions/"+created.Code+"/command", "",
		map[string]string{"text": "/ls"}, nil)
	if r4.StatusCode != http.StatusConflict {
		t.Fatalf("command with no attached agent should 409, got %d", r4.StatusCode)
	}
}
