package api

// internal/api/remote.go implements the /qr "remote control" pairing relay: a
// signed-in CLI daemon opens an agent-side session here, the console renders
// its code as a QR pointing at qr.<base>/<code>, and a phone that scans it
// (viewer side — no login, the code itself is the capability) gets a live
// mirror of the console and can send commands back. Both legs use the same
// shape as the daemon's own loopback control API: SSE for the receiving
// direction, plain POSTs for the sending direction — so this hub is just two
// named pipes per session, fanned out to any number of viewers.
//
// In-process only today: fine for the single API replica this runs on now. A
// Redis pub/sub-backed hub is the natural upgrade if the control API ever
// scales past one replica — same seam already used for the device store and
// OAuth state (see device_redis.go, oauth_state.go) — but isn't built until
// that's actually needed.

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/trqsh-uz/trqsh/internal/api/auth"
)

// remoteSessionTTL bounds how long a pairing stays connectable after
// creation. The console requests a fresh code every time /qr runs (ending
// any prior session for that daemon), so this is a backstop for abandoned
// sessions, not the primary lifetime control.
const remoteSessionTTL = 30 * time.Minute

// remoteBacklogLimit caps how many recent transcript lines are replayed to a
// viewer that connects mid-session, so a phone joining late has context
// instead of a blank screen, without the backlog growing unbounded.
const remoteBacklogLimit = 200

// remoteEvent is the single envelope shape both legs exchange: the agent
// stream carries "command"/"presence"/"ended"; the viewer stream carries
// "lines"/"state"/"presence"/"ended". One reused type rather than two, the
// same way agent.Event is a single tagged struct on the daemon side.
type remoteEvent struct {
	Type      string       `json:"type"` // lines|state|command|presence|ended
	Lines     []string     `json:"lines,omitempty"`
	State     *remoteState `json:"state,omitempty"`
	Command   string       `json:"command,omitempty"`
	Connected bool         `json:"connected,omitempty"`
}

// remoteState is a snapshot of the console's tunnels/connection, for the
// viewer's header card. Deliberately not agent.Tunnel/agent.Status verbatim:
// this crosses a public, unauthenticated (code-guarded) endpoint, so it's a
// hand-picked, display-only subset.
type remoteState struct {
	Online  bool           `json:"online"`
	Edge    string         `json:"edge"`
	Tunnels []remoteTunnel `json:"tunnels"`
}

type remoteTunnel struct {
	ID        string `json:"id"`
	PublicURL string `json:"public_url"`
	LocalAddr string `json:"local_addr"`
	Status    string `json:"status"`
	Requests  int64  `json:"requests"`
}

// --- hub ---

type remoteHub struct {
	mu       sync.Mutex
	sessions map[string]*remoteSession
}

func newRemoteHub() *remoteHub {
	h := &remoteHub{sessions: make(map[string]*remoteSession)}
	go h.sweepLoop()
	return h
}

func (h *remoteHub) create(orgID string) *remoteSession {
	s := &remoteSession{
		code:      newRemoteCode(),
		orgID:     orgID,
		createdAt: time.Now(),
		expiresAt: time.Now().Add(remoteSessionTTL),
		viewers:   make(map[int]chan remoteEvent),
	}
	h.mu.Lock()
	h.sessions[s.code] = s
	h.mu.Unlock()
	return s
}

// get returns the session for code, or nil if it doesn't exist or has
// expired (lazily reaped here, same pattern as the device store's Poll).
func (h *remoteHub) get(code string) *remoteSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[code]
	if s == nil {
		return nil
	}
	if time.Now().After(s.expiresAt) {
		delete(h.sessions, code)
		return nil
	}
	return s
}

// end removes and tears down a session immediately (explicit /qr stop, or
// the creating daemon disconnecting for good).
func (h *remoteHub) end(code string) {
	h.mu.Lock()
	s := h.sessions[code]
	delete(h.sessions, code)
	h.mu.Unlock()
	if s != nil {
		s.endSession()
	}
}

// sweepLoop reaps expired sessions a caller never explicitly ended.
func (h *remoteHub) sweepLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		var expired []*remoteSession
		h.mu.Lock()
		for code, s := range h.sessions {
			if now.After(s.expiresAt) {
				expired = append(expired, s)
				delete(h.sessions, code)
			}
		}
		h.mu.Unlock()
		for _, s := range expired {
			s.endSession()
		}
	}
}

// --- session ---

type remoteSession struct {
	code      string
	orgID     string
	createdAt time.Time
	expiresAt time.Time

	mu             sync.Mutex
	agentCh        chan remoteEvent
	agentConnected bool
	viewers        map[int]chan remoteEvent
	nextViewer     int
	backlog        []string
	state          *remoteState
	ended          bool
}

// attachAgent registers the (single) daemon connection, replacing any prior
// one — an agent reconnecting after a network blip keeps the same session
// rather than orphaning it.
func (s *remoteSession) attachAgent() chan remoteEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan remoteEvent, 32)
	s.agentCh = ch
	s.agentConnected = true
	s.broadcastViewersLocked(remoteEvent{Type: "presence", Connected: true})
	return ch
}

// detachAgent unregisters ch if it's still the current agent connection (a
// stale detach from a since-superseded reconnect is a no-op).
func (s *remoteSession) detachAgent(ch chan remoteEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentCh != ch {
		return
	}
	s.agentCh = nil
	s.agentConnected = false
	s.broadcastViewersLocked(remoteEvent{Type: "presence", Connected: false})
}

// addViewer registers a new phone/browser connection and replays recent
// context (backlog + last known state + current presence) so it isn't
// staring at a blank page.
func (s *remoteSession) addViewer() (int, chan remoteEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextViewer
	s.nextViewer++
	ch := make(chan remoteEvent, 32)
	s.viewers[id] = ch
	if len(s.backlog) > 0 {
		ch <- remoteEvent{Type: "lines", Lines: append([]string(nil), s.backlog...)}
	}
	if s.state != nil {
		ch <- remoteEvent{Type: "state", State: s.state}
	}
	ch <- remoteEvent{Type: "presence", Connected: s.agentConnected}
	return id, ch
}

func (s *remoteSession) removeViewer(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.viewers[id]; ok {
		delete(s.viewers, id)
		close(ch)
	}
}

// publish is the agent side pushing transcript lines / a state snapshot: it
// updates the replay backlog and fans the event out to every viewer.
func (s *remoteSession) publish(ev remoteEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch ev.Type {
	case "lines":
		s.backlog = append(s.backlog, ev.Lines...)
		if n := len(s.backlog); n > remoteBacklogLimit {
			s.backlog = s.backlog[n-remoteBacklogLimit:]
		}
	case "state":
		s.state = ev.State
	}
	s.broadcastViewersLocked(ev)
}

func (s *remoteSession) broadcastViewersLocked(ev remoteEvent) {
	for _, ch := range s.viewers {
		select {
		case ch <- ev:
		default: // a stalled viewer drops a frame rather than blocking the fan-out
		}
	}
}

// command is the viewer side sending a typed/tapped command: forwarded to
// the attached agent if one is connected. Reports whether it was delivered.
func (s *remoteSession) command(text string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentCh == nil {
		return false
	}
	select {
	case s.agentCh <- remoteEvent{Type: "command", Command: text}:
		return true
	default:
		return false
	}
}

// endSession notifies both legs and closes every viewer channel. Safe to
// call more than once (the second call is a no-op).
func (s *remoteSession) endSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ended = true
	s.broadcastViewersLocked(remoteEvent{Type: "ended"})
	if s.agentCh != nil {
		select {
		case s.agentCh <- remoteEvent{Type: "ended"}:
		default:
		}
	}
	for id, ch := range s.viewers {
		delete(s.viewers, id)
		close(ch)
	}
}

// newRemoteCode returns a fresh pairing code: three dash-separated groups
// from a no-vowel, no-ambiguous-character alphabet (same alphabet as the
// device flow's user_code), ~58 bits of entropy — enough to resist online
// guessing over a 30-minute TTL even before the shared /v1 rate limiter,
// while still short enough to type by hand if the camera can't scan.
func newRemoteCode() string {
	const alphabet = "BCDFGHJKLMNPQRSTVWXZ0123456789"
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	out := make([]byte, 0, 14)
	for i, c := range b {
		if i > 0 && i%4 == 0 {
			out = append(out, '-')
		}
		out = append(out, alphabet[int(c)%len(alphabet)])
	}
	return string(out)
}

// --- handlers: agent side (authenticated — JWT or API key) ---

func (s *Server) handleRemoteCreate(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	sess := s.remote.create(p.OrgID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"code":       sess.code,
		"url":        s.qrBase() + "/" + sess.code,
		"expires_at": sess.expiresAt,
	})
}

// remoteSessionForAgent resolves {code} and checks it belongs to the caller's
// org, writing a response and returning nil if not.
func (s *Server) remoteSessionForAgent(w http.ResponseWriter, r *http.Request) *remoteSession {
	p, _ := auth.PrincipalFrom(r.Context())
	sess := s.remote.get(chi.URLParam(r, "code"))
	if sess == nil || sess.orgID != p.OrgID {
		writeError(w, http.StatusNotFound, "no such session")
		return nil
	}
	return sess
}

func (s *Server) handleRemoteAgentStream(w http.ResponseWriter, r *http.Request) {
	sess := s.remoteSessionForAgent(w, r)
	if sess == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	ch := sess.attachAgent()
	defer sess.detachAgent(ch)

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte("data: "))
			_ = enc.Encode(ev)
			_, _ = w.Write([]byte("\n"))
			flusher.Flush()
			if ev.Type == "ended" {
				return
			}
		}
	}
}

func (s *Server) handleRemotePublish(w http.ResponseWriter, r *http.Request) {
	sess := s.remoteSessionForAgent(w, r)
	if sess == nil {
		return
	}
	var ev remoteEvent
	if !decode(w, r, &ev) {
		return
	}
	if ev.Type != "lines" && ev.Type != "state" {
		writeError(w, http.StatusBadRequest, "type must be lines or state")
		return
	}
	sess.publish(ev)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoteEnd(w http.ResponseWriter, r *http.Request) {
	sess := s.remoteSessionForAgent(w, r)
	if sess == nil {
		return
	}
	s.remote.end(sess.code)
	w.WriteHeader(http.StatusNoContent)
}

// --- handlers: viewer side (public — the code itself is the capability) ---

func (s *Server) handleRemoteViewerStream(w http.ResponseWriter, r *http.Request) {
	sess := s.remote.get(chi.URLParam(r, "code"))
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found or expired")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	id, ch := sess.addViewer()
	defer sess.removeViewer(id)

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte("data: "))
			_ = enc.Encode(ev)
			_, _ = w.Write([]byte("\n"))
			flusher.Flush()
			if ev.Type == "ended" {
				return
			}
		}
	}
}

func (s *Server) handleRemoteCommand(w http.ResponseWriter, r *http.Request) {
	sess := s.remote.get(chi.URLParam(r, "code"))
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found or expired")
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Text) == 0 || len(req.Text) > 240 {
		writeError(w, http.StatusBadRequest, "text must be 1-240 characters")
		return
	}
	if !sess.command(req.Text) {
		writeError(w, http.StatusConflict, "no console currently attached to this session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// qrBase returns the public /qr pairing origin (no trailing slash).
func (s *Server) qrBase() string {
	if s.cfg.QRURL != "" {
		return strings.TrimRight(s.cfg.QRURL, "/")
	}
	return "https://qr." + s.cfg.BaseDomain
}
