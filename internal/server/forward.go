package server

import (
	"context"
	"errors"
	"net"
)

// errNoForward is returned by the MVP forwarder: cross-edge routing is designed
// but not yet implemented.
var errNoForward = errors.New("server: cross-edge forwarding not implemented")

// Forwarder is the seam for multi-edge routing. When a public connection lands
// on edge B for a tunnel homed on edge A, edge B forwards it to A over an
// internal hop. The MVP runs a single edge, so this is a no-op stub that keeps
// the ingress code path honest.
//
// TODO(part-08): implement an internal QUIC hop between edges keyed by EdgeID,
// resolving peer addresses from the registry/service discovery, and weld the
// public conn to a stream on the peer edge. Until then, ingress only serves
// locally-homed tunnels (Hub) and returns errNoForward for remote bindings.
type Forwarder interface {
	// Forward welds a public conn to the tunnel homed on the named edge.
	Forward(ctx context.Context, edgeID string, pub net.Conn, route Route) error
}

type noopForwarder struct{}

// NewForwarder returns the MVP no-op forwarder.
func NewForwarder() Forwarder { return noopForwarder{} }

func (noopForwarder) Forward(_ context.Context, _ string, _ net.Conn, _ Route) error {
	return errNoForward
}
