package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
)

// Product URLs (deep links into the hosted dashboard/docs). Overridable at
// runtime for self-hosted deployments so the GUI never hardcodes a single SaaS
// origin — the frontend reads these via Env() instead of embedding literals.
var (
	dashboardURL = envOr("TRQSH_DASHBOARD_URL", "https://dashboard.trqsh.uz")
	docsURL      = envOr("TRQSH_DOCS_URL", "https://trqsh.uz/docs")
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// HostInfo is the environment snapshot the frontend fetches once at boot to
// adapt chrome to the OS (native traffic lights vs. custom window buttons) and
// to build deep links without hardcoding them (Env in frontend/src/lib/agent.ts).
type HostInfo struct {
	OS           string `json:"os"`   // runtime.GOOS: darwin | windows | linux
	Arch         string `json:"arch"` // runtime.GOARCH
	Version      string `json:"version"`
	DashboardURL string `json:"dashboard_url"`
	KeysURL      string `json:"keys_url"`
	BillingURL   string `json:"billing_url"`
	DocsURL      string `json:"docs_url"`
}

// AgentService is the Wails-bound surface the frontend calls as
// "AgentService.<Method>" (see frontend/src/lib/agent.ts). It is a thin adapter
// over the frozen agent.Core: it adds no tunnelling logic of its own, only the
// GUI concerns of auth state, event bridging, settings, updates, and URLs.
type AgentService struct {
	core    *agent.Agent
	log     *slog.Logger
	version string

	mu      sync.Mutex
	app     *application.App
	win     *application.WebviewWindow
	authed  bool
	onState func() // notified on status/tunnel changes (tray refresh)
}

// NewAgentService wraps an already-constructed agent core.
func NewAgentService(core *agent.Agent, log *slog.Logger, version string) *AgentService {
	return &AgentService{
		core:    core,
		log:     log,
		version: version,
		authed:  core.Status().AccountID != "" || hasStoredKey(),
	}
}

// Attach records the running app + window and starts pumping core events to the
// frontend on the single "agent:event" channel. Called once from main.
func (s *AgentService) Attach(app *application.App, win *application.WebviewWindow) {
	s.mu.Lock()
	s.app = app
	s.win = win
	s.mu.Unlock()
	go s.pumpEvents()
}

// OnState registers a callback invoked whenever connection state or the tunnel
// set changes — used to keep the system tray in sync with live state.
func (s *AgentService) OnState(fn func()) {
	s.mu.Lock()
	s.onState = fn
	s.mu.Unlock()
}

func (s *AgentService) pumpEvents() {
	for ev := range s.core.Events() {
		// Keep the GUI "connected" (workspace visible) for as long as the user
		// is authenticated, so transient transport reconnects don't bounce them
		// back to the login screen. Real sign-out flips authed and re-emits.
		if ev.Type == "status" && ev.Status != nil {
			s.mu.Lock()
			authed := s.authed
			s.mu.Unlock()
			if authed {
				ev.Status.Connected = true
			}
		}
		s.emit(ev)
		// Refresh dependent surfaces (tray) only on state-shaped events, never
		// on the high-frequency request stream.
		if ev.Type == "status" || ev.Type == "tunnel" {
			s.notifyState()
		}
	}
}

func (s *AgentService) notifyState() {
	s.mu.Lock()
	fn := s.onState
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Env returns the host environment + product deep links for the frontend.
func (s *AgentService) Env() HostInfo {
	return HostInfo{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Version:      s.version,
		DashboardURL: dashboardURL,
		KeysURL:      dashboardURL + "/keys",
		BillingURL:   dashboardURL + "/billing",
		DocsURL:      docsURL,
	}
}

func (s *AgentService) emit(ev agent.Event) {
	s.mu.Lock()
	app := s.app
	s.mu.Unlock()
	if app != nil {
		app.EmitEvent("agent:event", ev)
	}
}

// ---- Bound methods (frontend/src/lib/agent.ts) ----

// Login stores the API key (persisted by the core) and marks the session
// authenticated. The transport connects lazily on the first StartTunnel.
func (s *AgentService) Login(token string) (agent.Status, error) {
	if err := s.core.Login(context.Background(), token); err != nil {
		return agent.Status{}, err
	}
	s.mu.Lock()
	s.authed = true
	s.mu.Unlock()
	st := s.Status()
	s.emit(agent.Event{Type: "status", Status: &st})
	return st, nil
}

// Logout shuts down active tunnels, clears the stored key, and returns the GUI
// to the login screen.
func (s *AgentService) Logout() error {
	s.mu.Lock()
	s.authed = false
	s.mu.Unlock()
	err := s.core.Shutdown(context.Background())
	if cerr := clearStoredKey(); cerr != nil {
		s.log.Warn("clear stored key", "err", cerr)
	}
	st := agent.Status{Plan: "free"}
	s.emit(agent.Event{Type: "status", Status: &st})
	return err
}

// Status reports the connection snapshot, with Connected reflecting GUI auth
// state (see pumpEvents for the rationale).
func (s *AgentService) Status() agent.Status {
	st := s.core.Status()
	s.mu.Lock()
	authed := s.authed
	s.mu.Unlock()
	if authed {
		st.Connected = true
	}
	return st
}

// StartTunnel opens a tunnel (connecting the transport if needed).
func (s *AgentService) StartTunnel(spec agent.TunnelSpec) (agent.Tunnel, error) {
	return s.core.StartTunnel(context.Background(), spec)
}

// StopTunnel tears down a tunnel by id.
func (s *AgentService) StopTunnel(id string) error {
	return s.core.StopTunnel(context.Background(), id)
}

// List returns the current tunnels.
func (s *AgentService) List() []agent.Tunnel {
	return s.core.List()
}

// webTunnels returns only the browser-openable (http/https/tls) tunnels, used by
// the tray for its quick "open in browser" actions.
func (s *AgentService) webTunnels() []agent.Tunnel {
	var out []agent.Tunnel
	for _, t := range s.core.List() {
		switch t.Proto {
		case "http", "https", "tls":
			out = append(out, t)
		}
	}
	return out
}

// Recent returns the inspector's captured HTTP exchanges. Recorder.List already
// returns them newest-first, which is exactly the order the GUI renders.
func (s *AgentService) Recent() []inspect.CapturedRequest {
	return s.core.Inspector().List()
}

// Replay re-issues a captured request against the local target service.
func (s *AgentService) Replay(id string) error {
	req, ok := s.core.Inspector().Get(id)
	if !ok {
		return fmt.Errorf("%s: request %q not found", "ERR_INTERNAL", id)
	}
	return replayLocal(req)
}

// OpenURL opens a link in the user's default browser.
func (s *AgentService) OpenURL(url string) error {
	return openBrowser(url)
}
