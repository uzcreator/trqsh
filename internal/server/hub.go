package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/rift/rift/pkg/proto"
	"github.com/rift/rift/pkg/tunnel"
)

// boundTunnel is a single registered tunnel homed on this edge, tying a route to
// the owning agent session.
type boundTunnel struct {
	clientTunnelID string
	accountID      string
	plan           string
	ttype          proto.TunnelType
	host           string // http/https/tls routing key (empty for tcp/udp)
	port           int    // tcp/udp assigned port (0 for http/https)
	options        map[string]string
	session        *agentSession
}

// agentSession wraps a live tunnel.Session plus the identity resolved at Hello
// time and the set of tunnels the agent has bound.
type agentSession struct {
	sess      tunnel.Session
	control   tunnel.Stream // the control stream (first stream opened by the agent)
	sessionID string
	accountID string
	plan      string
	apiKey    string

	mu      sync.Mutex
	tunnels map[string]*boundTunnel // by clientTunnelID

	writeMu sync.Mutex // serializes writes to the control stream
}

// controlStream returns the session's control stream (may be nil pre-handshake).
func (a *agentSession) controlStream() tunnel.Stream { return a.control }

// writeControl serializes an envelope write onto the control stream.
func (a *agentSession) writeControl(env *proto.Envelope) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return proto.WriteMsg(a.control, env)
}

// openDataStream opens a fresh data stream to the agent and writes the
// StreamInit header, returning the stream ready for welding.
func (a *agentSession) openDataStream(ctx context.Context, init *proto.StreamInit) (tunnel.Stream, error) {
	st, err := a.sess.OpenStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("open data stream: %w", err)
	}
	if err := proto.WriteStreamInit(st, init); err != nil {
		st.Close()
		return nil, fmt.Errorf("write stream init: %w", err)
	}
	return st, nil
}

// Hub is the in-process routing table for tunnels homed on this edge. It is the
// fast path; the shared Registry (Redis) mirrors routes for cross-edge lookup.
type Hub struct {
	mu       sync.RWMutex
	byHost   map[string]*boundTunnel // hostname -> tunnel
	byPort   map[string]*boundTunnel // "tcp:2222" / "udp:2222" -> tunnel
	sessions map[string]*agentSession
}

// NewHub creates an empty routing hub.
func NewHub() *Hub {
	return &Hub{
		byHost:   make(map[string]*boundTunnel),
		byPort:   make(map[string]*boundTunnel),
		sessions: make(map[string]*agentSession),
	}
}

func portKey(proto string, port int) string { return fmt.Sprintf("%s:%d", proto, port) }

func (h *Hub) addSession(a *agentSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[a.sessionID] = a
}

func (h *Hub) removeSession(a *agentSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, a.sessionID)
	for _, bt := range a.tunnels {
		if bt.host != "" {
			delete(h.byHost, bt.host)
		}
		if bt.port != 0 {
			delete(h.byPort, portKey(protoForType(bt.ttype), bt.port))
		}
	}
}

// bindHost registers an HTTP/HTTPS/TLS route. Returns an error if the host is
// already taken by another session.
func (h *Hub) bindHost(bt *boundTunnel) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.byHost[bt.host]; ok && existing.session != bt.session {
		return fmt.Errorf("host %q already bound", bt.host)
	}
	h.byHost[bt.host] = bt
	return nil
}

// bindPort registers a TCP/UDP route on an assigned port.
func (h *Hub) bindPort(bt *boundTunnel) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := portKey(protoForType(bt.ttype), bt.port)
	if existing, ok := h.byPort[key]; ok && existing.session != bt.session {
		return fmt.Errorf("port %s already bound", key)
	}
	h.byPort[key] = bt
	return nil
}

func (h *Hub) unbind(bt *boundTunnel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if bt.host != "" {
		delete(h.byHost, bt.host)
	}
	if bt.port != 0 {
		delete(h.byPort, portKey(protoForType(bt.ttype), bt.port))
	}
}

// LookupHost returns the tunnel bound to a hostname, or nil.
func (h *Hub) LookupHost(host string) *boundTunnel {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.byHost[host]
}

// LookupPort returns the tunnel bound to a proto+port, or nil.
func (h *Hub) LookupPort(proto string, port int) *boundTunnel {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.byPort[portKey(proto, port)]
}

// Stats reports current counts for metrics.
func (h *Hub) Stats() (sessions, tunnels int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions), len(h.byHost) + len(h.byPort)
}

// SnapshotSessions returns a copy of the current sessions (for drain/broadcast).
func (h *Hub) SnapshotSessions() []*agentSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*agentSession, 0, len(h.sessions))
	for _, a := range h.sessions {
		out = append(out, a)
	}
	return out
}

func protoForType(t proto.TunnelType) string {
	switch t {
	case proto.TunnelType_UDP:
		return "udp"
	case proto.TunnelType_TCP:
		return "tcp"
	case proto.TunnelType_TLS:
		return "tls"
	case proto.TunnelType_HTTPS:
		return "https"
	default:
		return "http"
	}
}
