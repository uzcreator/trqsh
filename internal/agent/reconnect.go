package agent

import (
	"math/rand"
	"strings"
	"time"
)

const (
	reconnectBase = 500 * time.Millisecond
	reconnectMax  = 30 * time.Second
)

// handleSessionDown marks the session dead (once per generation) and kicks off
// reconnection, unless the agent is shutting down.
func (a *Agent) handleSessionDown(gen uint64, cause error) {
	a.mu.Lock()
	if gen != a.sessGen.Load() || !a.connected {
		a.mu.Unlock()
		return
	}
	a.connected = false
	a.control = nil
	closed := a.closed
	prev := make([]*activeTunnel, 0, len(a.tunnels))
	for _, t := range a.tunnels {
		prev = append(prev, t)
	}
	a.mu.Unlock()

	a.log.Warn("session down", "cause", cause)
	st := a.Status()
	a.emit(Event{Type: "status", Status: &st})

	if closed {
		return
	}
	go a.reconnect(prev)
}

// reconnect re-dials with exponential backoff + jitter, then re-binds every
// previously active tunnel (restoring reserved subdomains / ports).
func (a *Agent) reconnect(prev []*activeTunnel) {
	backoff := reconnectBase
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}
		a.mu.Lock()
		closed := a.closed
		a.mu.Unlock()
		if closed {
			return
		}

		if err := a.connect(a.ctx); err == nil {
			a.rebindAll(prev)
			a.log.Info("reconnected", "edge", a.cfg.Server)
			return
		}

		sleep := backoff + time.Duration(rand.Int63n(int64(backoff/2+1)))
		if sleep > reconnectMax {
			sleep = reconnectMax
		}
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(sleep):
		}
		if backoff < reconnectMax {
			backoff *= 2
		}
	}
}

func (a *Agent) rebindAll(prev []*activeTunnel) {
	for _, old := range prev {
		spec := old.spec
		// Restore the same public endpoint where possible.
		if spec.Subdomain == "" && old.assignedHost != "" {
			spec.Subdomain = firstLabel(old.assignedHost)
		}
		if spec.RemotePort == 0 && old.assignedPort != 0 {
			spec.RemotePort = old.assignedPort
		}
		a.mu.Lock()
		delete(a.tunnels, old.clientTunnelID)
		a.mu.Unlock()

		if _, err := a.bind(a.ctx, spec); err != nil {
			a.emit(Event{Type: "error", Err: "re-bind " + spec.Name + ": " + err.Error()})
		}
	}
}

func firstLabel(host string) string {
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return host
}
