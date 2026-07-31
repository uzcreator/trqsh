package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/trqsh-uz/trqsh/pkg/proto"
)

// errNoForward is returned by the no-op forwarder used in single-edge mode
// (TRQSH_FORWARD_ADDR unset): there is no peer to hand a connection to.
var errNoForward = errors.New("server: cross-edge forwarding disabled")

// Forward header framing + timeouts. The inter-edge hop reuses pkg/proto's
// length-prefixed StreamInit frame (the same framing agent data streams use) as
// its handshake, so no third wire format is introduced: Proto carries the origin
// scheme, RemoteAddr the real public client, and Meta carries the shared-secret
// token. The target host is NOT duplicated into the header — the replayed HTTP
// request that follows already carries it (Host header), and serveHTTPConn parses
// that fresh on the receiving edge exactly as it would for a direct public
// connection, so there is nothing else for the header to authoritatively assert.
const (
	forwardTokenMeta        = "fwd_token"
	forwardDialTimeout      = 10 * time.Second
	forwardHandshakeTimeout = 10 * time.Second
)

// Forwarder hands a public connection off to the edge that actually owns a route.
// It is the cross-edge data path's dial side: resolve the owning edge's internal
// forwarding address, dial it, authenticate with the shared InternalToken, and
// announce the target route. The caller then welds its public conn to the returned
// connection (reusing rawJoin), so the owning edge serves the traffic against its
// local agent as if the connection had landed there directly.
type Forwarder interface {
	// Dial opens an authenticated hop to the edge owning route and returns the
	// connection to weld the public side to. scheme is the origin scheme
	// ("http"/"https"); clientAddr is the real public client address, carried
	// across the hop so the owning edge logs the true origin.
	Dial(ctx context.Context, edgeID string, route Route, scheme, clientAddr string) (net.Conn, error)
}

// dialForwarder is the real Forwarder: it resolves EdgeID -> address via the shared
// registry and dials the peer's forwarding listener.
type dialForwarder struct {
	reg     Registry
	token   string
	timeout time.Duration
}

type noForwarder struct{}

// newForwarder returns the real forwarder when this edge is part of the forwarding
// mesh (ForwardAddr set and InternalToken configured), else a no-op so ingress in
// single-edge mode behaves exactly as before.
func newForwarder(cfg Config, reg Registry) Forwarder {
	if cfg.ForwardAddr == "" || cfg.InternalToken == "" {
		return noForwarder{}
	}
	return &dialForwarder{reg: reg, token: cfg.InternalToken, timeout: forwardDialTimeout}
}

func (noForwarder) Dial(context.Context, string, Route, string, string) (net.Conn, error) {
	return nil, errNoForward
}

func (f *dialForwarder) Dial(ctx context.Context, edgeID string, route Route, scheme, clientAddr string) (net.Conn, error) {
	addr, ok, err := f.reg.LookupEdge(ctx, edgeID)
	if err != nil {
		return nil, fmt.Errorf("server: forward %s: resolve edge %q: %w", route.Key(), edgeID, err)
	}
	if !ok {
		// No advertised address: the edge is down or has drained (it removes its
		// address at drain start). Treat as unreachable so the caller fails fast.
		return nil, fmt.Errorf("server: forward %s: edge %q has no forwarding address", route.Key(), edgeID)
	}
	d := net.Dialer{Timeout: f.timeout}
	peer, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("server: forward %s: dial %s: %w", route.Key(), addr, err)
	}
	hdr := &proto.StreamInit{
		Proto:      scheme,
		RemoteAddr: clientAddr,
		Meta:       map[string]string{forwardTokenMeta: f.token},
	}
	_ = peer.SetWriteDeadline(time.Now().Add(f.timeout))
	if err := proto.WriteStreamInit(peer, hdr); err != nil {
		peer.Close()
		return nil, fmt.Errorf("server: forward %s: write header to %s: %w", route.Key(), addr, err)
	}
	_ = peer.SetWriteDeadline(time.Time{})
	return peer, nil
}

// forwardingEnabled reports whether this edge joins the forwarding mesh. An
// unauthenticated internal listener would be dangerous, so a token is required.
func (s *Server) forwardingEnabled() bool {
	return s.cfg.ForwardAddr != "" && s.cfg.InternalToken != ""
}

// edgeTTL is the lease for this edge's forwarding-address key, mirroring the route
// binding TTL (2× the heartbeat). Zero HeartbeatInterval (tests) => no expiry.
func (s *Server) edgeTTL() time.Duration {
	if s.cfg.HeartbeatInterval <= 0 {
		return 0
	}
	return 2 * s.cfg.HeartbeatInterval
}

// forwardHTTP hands an HTTP connection to the edge that owns host when it is homed
// elsewhere. It returns true when it has taken over the connection (forwarded, or
// failed after committing to a peer — in both cases the conn is spent and the
// caller must not fall through to its 404); false when host is bound on no peer, so
// the caller serves its existing branded 404.
//
// req has already been parsed off conn (into br). To hand off transparently the
// request is replayed onto the peer conn — the same req.Write the local serve path
// uses toward an agent — and then the remainder of the client conn is welded to the
// peer, which drives the full HTTP exchange (including keep-alive and upgrades)
// against its local agent.
func (s *Server) forwardHTTP(conn net.Conn, br *bufio.Reader, req *http.Request, host, scheme string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), forwardDialTimeout)
	defer cancel()

	b, ok, err := s.reg.Lookup(ctx, Route{Host: host})
	if err != nil || !ok || b.EdgeID == s.cfg.EdgeID {
		// Unknown, registry error, or a stale local-hub miss for our own edge —
		// nothing to forward to; let the caller 404.
		return false
	}

	peer, err := s.dialForwardWithRetry(ctx, b.EdgeID, host, scheme, conn.RemoteAddr().String())
	if err != nil {
		s.metrics.Errors.WithLabelValues("forward_dial").Inc()
		writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 tunnel unreachable\n")
		return true
	}
	if err := req.Write(peer); err != nil {
		peer.Close()
		s.metrics.Errors.WithLabelValues("forward_write").Inc()
		writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 tunnel unreachable\n")
		return true
	}
	s.metrics.Forwards.WithLabelValues("out").Inc()
	up, down := rawJoin(conn, br, peer)
	s.metrics.countBytes(up, down)
	return true
}

// dialForwardWithRetry resolves the owning edge and dials it, retrying ONCE after a
// fresh registry lookup if the first peer is gone (it drained and removed its
// address, or the route just moved to a different edge). This turns a peer drain
// into a clean reroute or a fast failure instead of a hang or an opaque error.
func (s *Server) dialForwardWithRetry(ctx context.Context, edgeID, host, scheme, clientAddr string) (net.Conn, error) {
	peer, err := s.fwd.Dial(ctx, edgeID, Route{Host: host}, scheme, clientAddr)
	if err == nil {
		return peer, nil
	}
	b, ok, lerr := s.reg.Lookup(ctx, Route{Host: host})
	if lerr != nil || !ok || b.EdgeID == s.cfg.EdgeID || b.EdgeID == edgeID {
		// No better target than the one that just failed — surface the dial error.
		return nil, err
	}
	return s.fwd.Dial(ctx, b.EdgeID, Route{Host: host}, scheme, clientAddr)
}

// forwardedConn presents a peer-edge connection to the local serving path as if it
// were the original public connection: RemoteAddr reports the real client (carried
// in the forward header) rather than the peer edge, so inspector/logs and the agent
// StreamInit see the true origin across the inter-edge hop. Reads/writes/deadlines
// pass through to the embedded peer conn.
type forwardedConn struct {
	net.Conn
	remote net.Addr
}

func (c *forwardedConn) RemoteAddr() net.Addr { return c.remote }

// forwardAddr is a net.Addr carrying the original client's address string across
// the hop. Only String() is consulted (StreamInit.RemoteAddr, logs).
type forwardAddr string

func (a forwardAddr) Network() string { return "tcp" }
func (a forwardAddr) String() string  { return string(a) }
