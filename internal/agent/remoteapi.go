package agent

// This file is the daemon's half of /qr remote-control pairing: it creates a
// session on the control plane, holds a long-lived SSE connection to receive
// commands typed on a paired phone (re-broadcast as Events, same channel the
// TUI already consumes for everything else — see localapi.go's /events), and
// forwards outbound transcript/state publishes from the TUI. See
// internal/api/remote.go for the relay hub this talks to, and package tui's
// cmdQR/handleEvent for how the console drives it.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// remotePairing tracks the daemon's one active /qr session.
type remotePairing struct {
	code   string
	cancel context.CancelFunc
}

// remoteWireEvent decodes an SSE event from the control plane's agent stream.
// Deliberately a small local type (not shared with internal/api) — the agent
// core is the open-source half of trqsh and talks to the control plane over
// plain HTTP/JSON, never by importing it, the same way cloudapi.go's oauth
// helpers define their own response structs.
type remoteWireEvent struct {
	Type      string `json:"type"` // command|presence|ended
	Command   string `json:"command,omitempty"`
	Connected bool   `json:"connected,omitempty"`
}

var errRemoteEnded = errors.New("remote session ended")

// swapRemote atomically replaces the active pairing (nil to clear) and
// returns whatever was previously active, if anything.
func (l *LocalAPI) swapRemote(p *remotePairing) *remotePairing {
	l.remoteMu.Lock()
	defer l.remoteMu.Unlock()
	old := l.remote
	l.remote = p
	return old
}

func (l *LocalAPI) activeRemote() *remotePairing {
	l.remoteMu.Lock()
	defer l.remoteMu.Unlock()
	return l.remote
}

// remoteStart begins a new pairing: create a session on the control plane,
// replace (and end) any previous one, and start the goroutine that pumps
// inbound commands from it into Events().
func (l *LocalAPI) remoteStart(w http.ResponseWriter, r *http.Request) {
	key := storedAPIKey()
	if key == "" {
		apiWriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "not signed in"})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, cloudBase()+"/v1/remote/sessions", nil)
	if err != nil {
		apiWriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := cloudHTTP.Do(req)
	if err != nil {
		apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "cloud unreachable: " + err.Error()})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": remoteErrorFrom(resp)})
		return
	}
	var out struct {
		Code      string `json:"code"`
		URL       string `json:"url"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil || out.Code == "" {
		apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "bad response from cloud"})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	old := l.swapRemote(&remotePairing{code: out.Code, cancel: cancel})
	if old != nil {
		old.cancel()
		go endRemoteSession(key, old.code)
	}
	go l.pumpRemote(ctx, key, out.Code)

	apiWriteJSON(w, http.StatusOK, out)
}

// remoteStop ends the active pairing, if any. Idempotent: stopping with
// nothing active is a no-op, not an error.
func (l *LocalAPI) remoteStop(w http.ResponseWriter, r *http.Request) {
	old := l.swapRemote(nil)
	if old != nil {
		old.cancel()
		endRemoteSession(storedAPIKey(), old.code)
	}
	w.WriteHeader(http.StatusNoContent)
}

// remotePublish forwards the console's transcript lines / state snapshot to
// the active pairing's viewers. A no-op (still 204) when nothing is active or
// signed in — the TUI only calls this when it believes a session is live, and
// a lost race here shouldn't surface as an error in the local console.
func (l *LocalAPI) remotePublish(w http.ResponseWriter, r *http.Request) {
	p := l.activeRemote()
	key := storedAPIKey()
	if p == nil || key == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		cloudBase()+"/v1/remote/sessions/"+p.code+"/publish", bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := cloudHTTP.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	w.WriteHeader(http.StatusNoContent)
}

// pumpRemote holds the daemon's inbound leg of the pairing: a long-lived SSE
// connection to the control plane's agent stream, reconnected with backoff on
// a transient drop (mirrors package tui's own client.streamEvents), until ctx
// is canceled (explicit stop, or superseded by a fresh /qr) or the session
// itself ends.
func (l *LocalAPI) pumpRemote(ctx context.Context, key, code string) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := l.pumpRemoteOnce(ctx, key, code)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errRemoteEnded) {
			if old := l.swapRemote(nil); old != nil && old.code != code {
				// A newer pairing has already taken over; don't clobber it or
				// tell the console its (different, still-live) session ended.
				l.swapRemote(old)
				return
			}
			l.broadcast(Event{Type: "remote", Remote: &RemoteEvent{Kind: "ended"}})
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func (l *LocalAPI) pumpRemoteOnce(ctx context.Context, key, code string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cloudBase()+"/v1/remote/sessions/"+code+"/agent", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "text/event-stream")
	// No client timeout: this is a deliberately long-lived stream; ctx (canceled
	// on explicit stop or supersession) is what bounds it.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return errRemoteEnded // the session expired or was ended from elsewhere
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote agent stream: status %d", resp.StatusCode)
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue
		}
		var ev remoteWireEvent
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "command":
			l.broadcast(Event{Type: "remote", Remote: &RemoteEvent{Kind: "command", Command: ev.Command}})
		case "presence":
			l.broadcast(Event{Type: "remote", Remote: &RemoteEvent{Kind: "presence", Connected: ev.Connected}})
		case "ended":
			return errRemoteEnded
		}
	}
	return sc.Err()
}

// endRemoteSession best-effort tells the control plane a pairing is over, so
// its viewer(s) see "ended" immediately instead of waiting out the TTL. Fire
// tolerant of failure: the TTL sweep is the backstop.
func endRemoteSession(key, code string) {
	if key == "" || code == "" {
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		cloudBase()+"/v1/remote/sessions/"+code+"/end", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := cloudHTTP.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func remoteErrorFrom(resp *http.Response) string {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
	if e.Error != "" {
		return e.Error
	}
	return fmt.Sprintf("status %d", resp.StatusCode)
}
