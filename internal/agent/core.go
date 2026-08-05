package agent

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
	"github.com/trqsh-uz/trqsh/pkg/proto"
	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

// ---- Frozen agent-core API (consumed by the CLI and the GUI, Part 04) ----

// Status is a snapshot of the agent's connection to the edge.
type Status struct {
	Connected bool   `json:"connected"`
	AccountID string `json:"account_id"`
	Plan      string `json:"plan"`
	Edge      string `json:"edge"`
	Kind      string `json:"kind"` // "quic" | "tcp"
}

// TunnelSpec describes a tunnel to open.
type TunnelSpec struct {
	Name         string `json:"name"`
	Proto        string `json:"proto"` // http|https|tls|tcp|udp
	Addr         string `json:"addr"`  // local target, e.g. "localhost:3000"
	Subdomain    string `json:"subdomain,omitempty"`
	CustomDomain string `json:"custom_domain,omitempty"`
	BasicAuth    string `json:"basic_auth,omitempty"`
	HostHeader   string `json:"host_header,omitempty"`
	RemotePort   int    `json:"remote_port,omitempty"`
}

// TunnelMetrics is a snapshot of a tunnel's traffic counters.
type TunnelMetrics struct {
	Connections int64 `json:"connections"`
	Requests    int64 `json:"requests"`
	BytesIn     int64 `json:"bytes_in"`
	BytesOut    int64 `json:"bytes_out"`
}

// Tunnel is an active tunnel's public state.
type Tunnel struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Proto     string        `json:"proto"`
	LocalAddr string        `json:"local_addr"`
	PublicURL string        `json:"public_url"`
	Status    string        `json:"status"` // connecting|online|error
	Metrics   TunnelMetrics `json:"metrics"`
	CreatedAt time.Time     `json:"created_at"` // when the tunnel came online (server-side truth for uptime)
}

// Event is a state change, captured request, or error emitted on Events().
type Event struct {
	Type    string                   `json:"type"` // status|tunnel|request|error
	Status  *Status                  `json:"status,omitempty"`
	Tunnel  *Tunnel                  `json:"tunnel,omitempty"`
	Request *inspect.CapturedRequest `json:"request,omitempty"`
	Err     string                   `json:"err,omitempty"`
}

// Core is the interface the CLI and GUI drive. Frozen for Part 04.
type Core interface {
	Login(ctx context.Context, token string) error
	Status() Status
	StartTunnel(ctx context.Context, spec TunnelSpec) (Tunnel, error)
	StopTunnel(ctx context.Context, id string) error
	List() []Tunnel
	Events() <-chan Event
	Shutdown(ctx context.Context) error
}

// ---- Implementation ----

type tunnelMetrics struct {
	connections atomic.Int64
	requests    atomic.Int64
	bytesIn     atomic.Int64
	bytesOut    atomic.Int64
}

func (m *tunnelMetrics) snapshot() TunnelMetrics {
	return TunnelMetrics{
		Connections: m.connections.Load(),
		Requests:    m.requests.Load(),
		BytesIn:     m.bytesIn.Load(),
		BytesOut:    m.bytesOut.Load(),
	}
}

type activeTunnel struct {
	spec           TunnelSpec
	clientTunnelID string
	publicURL      string
	assignedHost   string
	assignedPort   int
	status         string
	createdAt      time.Time
	metrics        tunnelMetrics
}

func (t *activeTunnel) view() Tunnel {
	return Tunnel{
		ID:        t.clientTunnelID,
		Name:      t.spec.Name,
		Proto:     t.spec.Proto,
		LocalAddr: t.spec.Addr,
		PublicURL: t.publicURL,
		Status:    t.status,
		Metrics:   t.metrics.snapshot(),
		CreatedAt: t.createdAt,
	}
}

// Agent is the concrete Core implementation.
type Agent struct {
	cfg    Config
	log    *slog.Logger
	dialer *tunnel.Dialer
	insp   *inspect.Recorder

	localRT   http.RoundTripper // pooled keep-alive transport to local services
	localOnce sync.Once

	mu        sync.Mutex
	sess      tunnel.Session
	control   tunnel.Stream
	writeMu   sync.Mutex
	connected bool
	accountID string
	plan      string
	kind      string
	tunnels   map[string]*activeTunnel

	pendMu       sync.Mutex
	pendingBinds map[string]chan *proto.BindResp

	events chan Event

	ctx     context.Context
	cancel  context.CancelFunc
	closed  bool
	sessGen atomic.Uint64
}

var _ Core = (*Agent)(nil)

// New creates an Agent from cfg. It does not connect until StartTunnel.
func New(cfg Config, log *slog.Logger) *Agent {
	if log == nil {
		log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Agent{
		cfg:          cfg,
		log:          log,
		insp:         inspect.NewRecorder(256),
		tunnels:      make(map[string]*activeTunnel),
		pendingBinds: make(map[string]chan *proto.BindResp),
		events:       make(chan Event, 256),
		ctx:          ctx,
		cancel:       cancel,
	}
	a.dialer = &tunnel.Dialer{
		TLSConfig:   a.clientTLS(),
		DialTimeout: 10 * time.Second,
		KeepAlive:   15 * time.Second,
	}
	switch cfg.forcedTransport() {
	case "quic":
		a.dialer.ForceKind = tunnel.KindQUIC
	case "tcp":
		a.dialer.ForceKind = tunnel.KindTCP
	}
	return a
}

// Inspector exposes the recorder so the CLI/GUI can serve the inspector UI.
func (a *Agent) Inspector() *inspect.Recorder { return a.insp }

// localTransport returns the pooled HTTP transport used to forward requests to
// local services. Connections are kept alive and reused across requests instead
// of dialing localhost afresh for every request — the main win for pages that
// fire dozens of resource requests, and it avoids churning ephemeral sockets on
// Windows. Built lazily so an agent that only serves TCP/UDP never allocates it.
func (a *Agent) localTransport() http.RoundTripper {
	a.localOnce.Do(func() {
		a.localRT = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   localDialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     false, // speak HTTP/1.1 to local services
			MaxIdleConns:          512,
			MaxIdleConnsPerHost:   128,
			IdleConnTimeout:       90 * time.Second,
			DisableCompression:    true, // pass the upstream's encoding through untouched
			ExpectContinueTimeout: time.Second,
		}
	})
	return a.localRT
}

func (a *Agent) clientTLS() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: a.cfg.Insecure, // #nosec G402 -- dev opt-in via TRQSH_INSECURE, off by default
		NextProtos:         []string{tunnel.ALPNProto},
	}
}

// Login stores an API key (in memory and persisted to the config file).
func (a *Agent) Login(_ context.Context, token string) error {
	a.mu.Lock()
	a.cfg.APIKey = token
	cfg := a.cfg
	a.mu.Unlock()
	return cfg.Save(DefaultConfigPath())
}

// Status returns the current connection snapshot.
func (a *Agent) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Status{
		Connected: a.connected,
		AccountID: a.accountID,
		Plan:      a.plan,
		Edge:      a.cfg.Server,
		Kind:      a.kind,
	}
}

// List returns snapshots of all active tunnels.
func (a *Agent) List() []Tunnel {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Tunnel, 0, len(a.tunnels))
	for _, t := range a.tunnels {
		out = append(out, t.view())
	}
	return out
}

// Events returns the event stream (state changes, requests, errors).
func (a *Agent) Events() <-chan Event { return a.events }

// StartTunnel connects if needed, binds the tunnel, and returns its public URL.
func (a *Agent) StartTunnel(ctx context.Context, spec TunnelSpec) (Tunnel, error) {
	if spec.Proto == "" {
		spec.Proto = "http"
	}
	if spec.Name == "" {
		spec.Name = spec.Proto + "-" + strings.TrimPrefix(spec.Addr, "localhost:")
	}
	if _, ok := tunnelType(spec.Proto); !ok {
		return Tunnel{}, &Error{Code: proto.CodeProtoUnsupported, Message: "unknown proto " + spec.Proto}
	}
	if err := a.ensureConnected(ctx); err != nil {
		return Tunnel{}, err
	}
	return a.bind(ctx, spec)
}

// StopTunnel unbinds a tunnel by id.
func (a *Agent) StopTunnel(ctx context.Context, id string) error {
	a.mu.Lock()
	t := a.tunnels[id]
	control := a.control
	a.mu.Unlock()
	if t == nil {
		return &Error{Code: proto.CodeInternal, Message: "no such tunnel " + id}
	}
	if control != nil {
		_ = a.writeControl(&proto.Envelope{Msg: &proto.Envelope_Unbind{Unbind: &proto.Unbind{
			ClientTunnelId: id,
		}}})
	}
	a.mu.Lock()
	delete(a.tunnels, id)
	a.mu.Unlock()
	a.emit(Event{Type: "tunnel", Tunnel: ptr(t.view())})
	return nil
}

// Shutdown closes all tunnels and the session.
func (a *Agent) Shutdown(_ context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	sess := a.sess
	a.mu.Unlock()

	a.cancel()
	if sess != nil {
		_ = sess.CloseWithError(0, "agent shutdown")
	}
	return nil
}

// emit delivers an event without blocking (drops if the buffer is full).
func (a *Agent) emit(ev Event) {
	select {
	case a.events <- ev:
	default:
	}
}

func (a *Agent) writeControl(env *proto.Envelope) error {
	a.mu.Lock()
	control := a.control
	a.mu.Unlock()
	if control == nil {
		return &Error{Code: proto.CodeInternal, Message: "not connected"}
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return proto.WriteMsg(control, env)
}

func tunnelType(p string) (proto.TunnelType, bool) {
	switch strings.ToLower(p) {
	case "http":
		return proto.TunnelType_HTTP, true
	case "https":
		return proto.TunnelType_HTTPS, true
	case "tls":
		return proto.TunnelType_TLS, true
	case "tcp":
		return proto.TunnelType_TCP, true
	case "udp":
		return proto.TunnelType_UDP, true
	}
	return proto.TunnelType_HTTP, false
}

func ptr[T any](v T) *T { return &v }
