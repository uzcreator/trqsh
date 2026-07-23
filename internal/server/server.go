package server

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/trqsh-uz/trqsh/internal/entitlerpc"
	"github.com/trqsh-uz/trqsh/pkg/authz"
	"github.com/trqsh-uz/trqsh/pkg/proto"
	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

// Server is the trqsh edge (`trqshd`): it accepts agent sessions, tracks tunnels,
// and welds public traffic to the owning agent.
type Server struct {
	cfg     Config
	log     *slog.Logger
	ent     authz.Entitlements
	hub     *Hub
	reg     Registry
	certs   CertManager
	metrics *Metrics
	usage   *usageAgg
	fwd     Forwarder
	ports   *portManager

	agentLn tunnel.Listener
	httpLn  net.Listener
	httpsLn net.Listener
	opsSrv  *http.Server

	mu       sync.Mutex
	draining bool

	ready  chan struct{}
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New builds a Server from cfg. Dependencies (entitlements, registry, certs)
// are chosen from config but may be swapped in tests via the fields.
func New(cfg Config, log *slog.Logger) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	ent, err := buildEntitlements(cfg)
	if err != nil {
		return nil, err
	}
	reg, err := buildRegistry(cfg)
	if err != nil {
		return nil, err
	}
	certs, err := NewDevCertManager(cfg.BaseDomain)
	if err != nil {
		return nil, err
	}
	m := NewMetrics()
	s := &Server{
		cfg:     cfg,
		log:     log,
		ent:     ent,
		hub:     NewHub(),
		reg:     reg,
		certs:   certs,
		metrics: m,
		usage:   newUsageAgg(ent, 30*time.Second),
		fwd:     NewForwarder(),
		ports:   newPortManager(cfg.PortMin, cfg.PortMax),
		ready:   make(chan struct{}),
	}
	return s, nil
}

func buildEntitlements(cfg Config) (authz.Entitlements, error) {
	switch cfg.EntitlementsMode {
	case "", "stub":
		return authz.StubEntitlements{}, nil
	case "api":
		// Real edge-side entitlements client (internal RPC to the control API,
		// with a short-TTL auth cache). See internal/entitlerpc.
		return entitlerpc.NewClient(cfg.APIURL, cfg.InternalToken), nil
	default:
		return nil, fmt.Errorf("server: unknown TRQSH_ENTITLEMENTS=%q", cfg.EntitlementsMode)
	}
}

func buildRegistry(cfg Config) (Registry, error) {
	if cfg.RedisURL == "" {
		return NewInMemoryRegistry(), nil
	}
	return NewRedisRegistry(cfg.RedisURL)
}

// Run starts all listeners and blocks until ctx is canceled, then drains.
func (s *Server) Run(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	// Agent listener (QUIC + TCP) via Part 01.
	agentTLS, err := s.agentTLSConfig()
	if err != nil {
		return err
	}
	ln, err := tunnel.Listen(s.cfg.QUICAddr, s.cfg.TCPAddr, agentTLS)
	if err != nil {
		return fmt.Errorf("server: agent listen: %w", err)
	}
	s.agentLn = ln
	s.log.Info("agent listener up", "quic", s.cfg.QUICAddr, "tcp", s.cfg.TCPAddr)

	// Public ingress.
	if err := s.startHTTP(ctx); err != nil {
		return err
	}
	if err := s.startHTTPS(ctx); err != nil {
		return err
	}
	s.startOps(ctx)
	s.usage.run()

	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.acceptAgents(ctx) }()

	close(s.ready)
	s.log.Info("trqshd ready", "base_domain", s.cfg.BaseDomain, "region", s.cfg.Region, "edge", s.cfg.EdgeID)

	<-ctx.Done()
	return s.drain()
}

// Ready blocks until the server has started its listeners (used by tests).
func (s *Server) Ready() <-chan struct{} { return s.ready }

// AgentAddr is the address agents dial (QUIC). Valid after Ready fires.
func (s *Server) AgentAddr() net.Addr {
	if s.agentLn == nil {
		return nil
	}
	return s.agentLn.Addr()
}

// HTTPAddr is the public HTTP ingress address. Valid after Ready fires.
func (s *Server) HTTPAddr() net.Addr {
	if s.httpLn == nil {
		return nil
	}
	return s.httpLn.Addr()
}

func (s *Server) agentTLSConfig() (*tls.Config, error) {
	// Agents connect with the trqsh ALPN; reuse the dev self-signed cert.
	cert, err := s.certs.GetCertificate(&tls.ClientHelloInfo{ServerName: s.cfg.EdgeID})
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{*cert},
		NextProtos:   []string{tunnel.ALPNProto},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func (s *Server) acceptAgents(ctx context.Context) {
	for {
		sess, err := s.agentLn.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.metrics.Errors.WithLabelValues("accept").Inc()
			s.log.Warn("agent accept failed", "err", err)
			continue
		}
		s.metrics.Handshakes.WithLabelValues(string(sess.Kind())).Inc()
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.serveSession(ctx, sess) }()
	}
}

// handleBind authorizes and registers a tunnel, returning the BindResp to send.
func (s *Server) handleBind(ctx context.Context, a *agentSession, b *proto.BindTunnel) *proto.BindResp {
	typ := protoForType(b.Type)
	req := authz.BindRequest{
		APIKey:     a.apiKey,
		Type:       typ,
		Subdomain:  b.Subdomain,
		CustomHost: b.CustomDomain,
		RemotePort: int(b.RemotePort),
		Region:     s.cfg.Region,
	}
	dec, err := s.ent.CheckBind(ctx, req)
	if err != nil {
		return bindErr(b, proto.CodeInternal, "entitlement check failed")
	}
	if !dec.Allow {
		code := dec.ErrorCode
		if code == "" {
			code = proto.CodePlanForbids
		}
		return bindErr(b, code, dec.ErrorMessage)
	}
	// Defense in depth: honor the returned Limits even if Allow=true.
	if b.Type == proto.TunnelType_TCP && !dec.Limits.AllowTCP {
		return bindErr(b, proto.CodePlanForbids, "TCP tunnels not allowed on this plan")
	}
	if b.Type == proto.TunnelType_UDP && !dec.Limits.AllowUDP {
		return bindErr(b, proto.CodePlanForbids, "UDP tunnels not allowed on this plan")
	}

	bt := &boundTunnel{
		clientTunnelID: b.ClientTunnelId,
		accountID:      firstNonEmpty(dec.AccountID, a.accountID),
		plan:           firstNonEmpty(dec.Plan, a.plan),
		ttype:          b.Type,
		options:        b.Options,
		session:        a,
	}

	switch b.Type {
	case proto.TunnelType_TCP, proto.TunnelType_UDP:
		return s.bindPortTunnel(ctx, a, b, bt, typ)
	default:
		return s.bindHostTunnel(ctx, a, b, bt, dec)
	}
}

func (s *Server) bindHostTunnel(ctx context.Context, a *agentSession, b *proto.BindTunnel, bt *boundTunnel, dec authz.Decision) *proto.BindResp {
	host, code := s.allocateHost(b, dec)
	if code != "" {
		return bindErr(b, code, "could not allocate hostname")
	}
	bt.host = host
	if err := s.hub.bindHost(bt); err != nil {
		return bindErr(b, proto.CodeSubdomainTaken, err.Error())
	}
	s.trackTunnel(a, bt)
	_ = s.reg.Bind(ctx, Route{Host: host}, Binding{EdgeID: s.cfg.EdgeID, SessionID: a.sessionID}, 2*s.cfg.HeartbeatInterval)
	s.metrics.TunnelsActive.Inc()

	scheme := "https"
	return &proto.BindResp{
		ClientTunnelId: b.ClientTunnelId,
		Ok:             true,
		PublicUrl:      scheme + "://" + host,
		AssignedHost:   host,
	}
}

func (s *Server) bindPortTunnel(ctx context.Context, a *agentSession, b *proto.BindTunnel, bt *boundTunnel, typ string) *proto.BindResp {
	port, err := s.openPort(typ, int(b.RemotePort), bt)
	if err != nil {
		if errors.Is(err, errPortUnavailable) {
			return bindErr(b, proto.CodePortUnavailable, "no ports available")
		}
		return bindErr(b, proto.CodeInternal, err.Error())
	}
	bt.port = port
	if err := s.hub.bindPort(bt); err != nil {
		s.ports.release(typ, port)
		return bindErr(b, proto.CodePortUnavailable, err.Error())
	}
	s.trackTunnel(a, bt)
	_ = s.reg.Bind(ctx, Route{Proto: typ, Port: port}, Binding{EdgeID: s.cfg.EdgeID, SessionID: a.sessionID}, 2*s.cfg.HeartbeatInterval)
	s.metrics.TunnelsActive.Inc()

	return &proto.BindResp{
		ClientTunnelId: b.ClientTunnelId,
		Ok:             true,
		PublicUrl:      fmt.Sprintf("%s://%s:%d", typ, s.cfg.BaseDomain, port),
		AssignedHost:   s.cfg.BaseDomain,
		AssignedPort:   uint32(port),
	}
}

func (s *Server) trackTunnel(a *agentSession, bt *boundTunnel) {
	a.mu.Lock()
	a.tunnels[bt.clientTunnelID] = bt
	a.mu.Unlock()
}

// allocateHost resolves the public hostname for an HTTP-family tunnel.
func (s *Server) allocateHost(b *proto.BindTunnel, dec authz.Decision) (string, string) {
	if b.CustomDomain != "" {
		if !dec.Limits.AllowCustomDomains {
			return "", proto.CodePlanForbids
		}
		return strings.ToLower(b.CustomDomain), ""
	}
	sub := b.Subdomain
	if sub == "" && dec.AssignedSubdomain != "" {
		sub = dec.AssignedSubdomain
	}
	if sub != "" {
		if !dec.Limits.AllowReservedSubdomain {
			return "", proto.CodeSubdomainForbidden
		}
		host := strings.ToLower(sub) + "." + s.cfg.BaseDomain
		if s.hub.LookupHost(host) != nil {
			return "", proto.CodeSubdomainTaken
		}
		return host, ""
	}
	// Random subdomain, retry on collision.
	for i := 0; i < 8; i++ {
		host := randSubdomain(8) + "." + s.cfg.BaseDomain
		if s.hub.LookupHost(host) == nil {
			return host, ""
		}
	}
	return "", proto.CodeInternal
}

func (s *Server) handleUnbind(ctx context.Context, a *agentSession, clientTunnelID string) {
	a.mu.Lock()
	bt := a.tunnels[clientTunnelID]
	delete(a.tunnels, clientTunnelID)
	a.mu.Unlock()
	if bt == nil {
		return
	}
	s.releaseTunnel(ctx, bt)
}

func (s *Server) releaseTunnel(ctx context.Context, bt *boundTunnel) {
	s.hub.unbind(bt)
	if bt.host != "" {
		_ = s.reg.Unbind(ctx, Route{Host: bt.host})
	}
	if bt.port != 0 {
		typ := protoForType(bt.ttype)
		s.ports.release(typ, bt.port)
		_ = s.reg.Unbind(ctx, Route{Proto: typ, Port: bt.port})
	}
	s.metrics.TunnelsActive.Dec()
}

// startOps starts the metrics + health HTTP server.
func (s *Server) startOps(ctx context.Context) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.metrics.Registry(), promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		draining := s.draining
		s.mu.Unlock()
		if draining {
			http.Error(w, "draining", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	s.opsSrv = &http.Server{
		Addr:              s.cfg.MetricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // slowloris guard (gosec G112)
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.opsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Warn("ops server stopped", "err", err)
		}
	}()
}

// drain gracefully shuts the edge down: notify agents, close listeners, wait.
func (s *Server) drain() error {
	s.mu.Lock()
	s.draining = true
	s.mu.Unlock()
	s.log.Info("draining", "timeout", s.cfg.DrainTimeout)

	deadline := time.Now().Add(s.cfg.DrainTimeout)
	msg := &proto.Envelope{Msg: &proto.Envelope_Shutdown{Shutdown: &proto.ShutdownMsg{
		Reason:              "edge draining",
		DrainDeadlineUnixMs: deadline.UnixMilli(),
	}}}
	for _, a := range s.hub.SnapshotSessions() {
		if st := a.controlStream(); st != nil {
			_ = proto.WriteMsg(st, msg)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if s.opsSrv != nil {
		_ = s.opsSrv.Shutdown(ctx)
	}
	if s.httpLn != nil {
		_ = s.httpLn.Close()
	}
	if s.httpsLn != nil {
		_ = s.httpsLn.Close()
	}
	if s.agentLn != nil {
		_ = s.agentLn.Close()
	}
	s.ports.closeAll()
	s.usage.Stop()
	_ = s.reg.Close()

	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(s.cfg.DrainTimeout):
		s.log.Warn("drain timed out; forcing exit")
	}
	return nil
}

func bindErr(b *proto.BindTunnel, code, msg string) *proto.BindResp {
	return &proto.BindResp{
		ClientTunnelId: b.ClientTunnelId,
		Ok:             false,
		Error:          proto.NewError(code, msg),
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

const subdomainAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func randSubdomain(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	for i := range buf {
		buf[i] = subdomainAlphabet[int(buf[i])%len(subdomainAlphabet)]
	}
	// Ensure it starts with a letter.
	if buf[0] >= '0' && buf[0] <= '9' {
		buf[0] = 'r'
	}
	return string(buf)
}
