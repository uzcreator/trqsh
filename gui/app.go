package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/rift/rift/internal/agent"
	"github.com/rift/rift/internal/agent/inspect"
)

// AgentService is the Wails-bound surface the frontend calls as
// "AgentService.<Method>" (see frontend/src/lib/agent.ts). It is a thin adapter
// over the frozen agent.Core: it adds no tunnelling logic of its own, only the
// GUI concerns of auth state, event bridging, settings, updates, and URLs.
type AgentService struct {
	core *agent.Agent
	log  *slog.Logger

	mu     sync.Mutex
	app    *application.App
	authed bool
}

// NewAgentService wraps an already-constructed agent core.
func NewAgentService(core *agent.Agent, log *slog.Logger) *AgentService {
	return &AgentService{
		core:   core,
		log:    log,
		authed: core.Status().AccountID != "" || hasStoredKey(),
	}
}

// Attach records the running app and starts pumping core events to the frontend
// on the single "agent:event" channel. Called once from main after app.New.
func (s *AgentService) Attach(app *application.App) {
	s.mu.Lock()
	s.app = app
	s.mu.Unlock()
	go s.pumpEvents()
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
