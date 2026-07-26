package tunnel

import (
	"context"
	"net"
	"time"
)

// Kind identifies which underlying transport a session negotiated.
type Kind string

const (
	KindQUIC Kind = "quic"
	KindTCP  Kind = "tcp"
)

// ALPNProto is the ALPN token negotiated for both QUIC and TLS-TCP sessions,
// so intermediaries and servers can distinguish the protocol version.
const ALPNProto = "trqsh/1"

// Transport tuning applied when the corresponding Dialer/Listen field is zero.
const (
	defaultDialTimeout = 10 * time.Second
	defaultKeepAlive   = 15 * time.Second
	defaultMaxIdle     = 30 * time.Second
)

// QUIC flow-control and stream tuning, shared by the dialer and the listener.
// quic-go's zero-value defaults are deliberately conservative (512 KB initial
// receive windows, 100 concurrent streams). Because the edge opens one data
// stream per public request, a single busy page can need far more than 100
// concurrent streams, and the small windows throttle the throughput ramp on a
// real edge↔agent internet link. Raising them keeps one session fast under load.
const (
	quicInitialStreamWindow = 4 << 20   // 4 MB
	quicMaxStreamWindow     = 16 << 20  // 16 MB
	quicInitialConnWindow   = 8 << 20   // 8 MB
	quicMaxConnWindow       = 64 << 20  // 64 MB
	quicMaxIncomingStreams  = 4096      // concurrent data streams per session
)

// Stream is one multiplexed, bidirectional channel within a Session. It behaves
// as a net.Conn so callers can io.Copy raw bytes across it.
type Stream interface {
	net.Conn
	// ID returns the underlying mux stream id (for logs/metrics).
	ID() uint64
}

// Session is a multiplexed connection between the agent and the edge. The agent
// opens the control stream first (OpenStream); the edge opens data streams
// toward the agent (which AcceptStreams them). Callers never branch on Kind
// except for metrics — the two transports are behaviourally identical.
type Session interface {
	OpenStream(ctx context.Context) (Stream, error)
	AcceptStream(ctx context.Context) (Stream, error)
	Kind() Kind
	RemoteAddr() net.Addr
	CloseWithError(code uint32, msg string) error
	// Context is canceled when the session dies.
	Context() context.Context
}

// Listener accepts agent sessions arriving over QUIC and/or TCP on the edge.
type Listener interface {
	Accept(ctx context.Context) (Session, error)
	Addr() net.Addr
	Close() error
}
