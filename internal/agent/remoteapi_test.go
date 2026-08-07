package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeControlPlane stands in for internal/api's /v1/remote/sessions/* during
// these tests: a single session, an SSE agent stream the test can push
// events onto, and a record of what got published/ended so assertions don't
// need a second real server round-trip.
type fakeControlPlane struct {
	mu        sync.Mutex
	agentSSE  chan string // raw "data: {...}" lines pushed to whatever agent stream is open
	published []string    // raw bodies POSTed to /publish
	ended     bool
}

func newFakeControlPlane(t *testing.T) (*httptest.Server, *fakeControlPlane) {
	t.Helper()
	f := &fakeControlPlane{agentSSE: make(chan string, 8)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/remote/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tq_test_key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "TEST-CODE", "url": "https://qr.example.test/TEST-CODE", "expires_at": time.Now().Add(time.Minute),
		})
	})
	mux.HandleFunc("GET /v1/remote/sessions/TEST-CODE/agent", func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case line := <-f.agentSSE:
				_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			}
		}
	})
	mux.HandleFunc("POST /v1/remote/sessions/TEST-CODE/publish", func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		f.mu.Lock()
		f.published = append(f.published, string(body))
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /v1/remote/sessions/TEST-CODE/end", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.ended = true
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, f
}

// localAPITestSetup points storedAPIKey()/cloudBase() at a temp config +
// the fake control plane, and returns a running LocalAPI reachable over its
// own httptest server, plus an /events reader.
func localAPITestSetup(t *testing.T) (localURL string, events *sseLineReader, cp *fakeControlPlane) {
	t.Helper()
	t.Setenv("TRQSH_CONFIG", t.TempDir()+"/trqsh.yml")
	t.Setenv("TRQSH_API_KEY", "tq_test_key")
	cpSrv, f := newFakeControlPlane(t)
	t.Setenv("TRQSH_API_URL", cpSrv.URL)

	l := NewLocalAPI(nil)
	localSrv := httptest.NewServer(l.Handler())
	t.Cleanup(localSrv.Close)

	req, _ := http.NewRequest(http.MethodGet, localSrv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect /events: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return localSrv.URL, &sseLineReader{t: t, sc: bufio.NewScanner(resp.Body)}, f
}

type sseLineReader struct {
	t  *testing.T
	sc *bufio.Scanner
}

func (r *sseLineReader) next(timeout time.Duration) Event {
	r.t.Helper()
	done := make(chan Event, 1)
	go func() {
		for r.sc.Scan() {
			data, ok := strings.CutPrefix(r.sc.Text(), "data: ")
			if !ok {
				continue
			}
			var ev Event
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
		r.t.Fatalf("timed out waiting for a local /events event")
		return Event{}
	}
}

func TestRemoteStartCommandPublishStop(t *testing.T) {
	localURL, events, cp := localAPITestSetup(t)

	var start struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	resp, err := http.Post(localURL+"/remote/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /remote/start: %v", err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&start)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || start.Code != "TEST-CODE" {
		t.Fatalf("start: status %d code %q", resp.StatusCode, start.Code)
	}

	// A command arriving on the (fake) control plane's agent stream surfaces
	// as a "remote" Event over the daemon's own /events, exactly like a
	// request/status/error event — same channel the TUI already listens on.
	cp.agentSSE <- `{"type":"command","command":"/status"}`
	ev := events.next(3 * time.Second)
	if ev.Type != "remote" || ev.Remote == nil || ev.Remote.Kind != "command" || ev.Remote.Command != "/status" {
		t.Fatalf("expected a remote command event, got %#v", ev)
	}

	// Presence relays too.
	cp.agentSSE <- `{"type":"presence","connected":true}`
	ev = events.next(3 * time.Second)
	if ev.Type != "remote" || ev.Remote == nil || ev.Remote.Kind != "presence" || !ev.Remote.Connected {
		t.Fatalf("expected a remote presence event, got %#v", ev)
	}

	// The console publishing lines forwards to the control plane's /publish.
	pub, err := http.Post(localURL+"/remote/publish", "application/json",
		strings.NewReader(`{"type":"lines","lines":["hello"]}`))
	if err != nil {
		t.Fatalf("POST /remote/publish: %v", err)
	}
	_ = pub.Body.Close()
	if pub.StatusCode != http.StatusNoContent {
		t.Fatalf("publish: status %d", pub.StatusCode)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		cp.mu.Lock()
		n := len(cp.published)
		cp.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("publish never reached the control plane")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Stopping tells the control plane the session ended.
	stop, err := http.Post(localURL+"/remote/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /remote/stop: %v", err)
	}
	_ = stop.Body.Close()
	if stop.StatusCode != http.StatusNoContent {
		t.Fatalf("stop: status %d", stop.StatusCode)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		cp.mu.Lock()
		ended := cp.ended
		cp.mu.Unlock()
		if ended {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stop never told the control plane the session ended")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRemoteStartRequiresSignIn(t *testing.T) {
	t.Setenv("TRQSH_CONFIG", t.TempDir()+"/trqsh.yml")
	// No TRQSH_API_KEY set: storedAPIKey() is "".
	l := NewLocalAPI(nil)
	srv := httptest.NewServer(l.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/remote/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when not signed in, got %d", resp.StatusCode)
	}
}
