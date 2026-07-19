package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"runtime"
	"time"

	"github.com/rift/rift/pkg/proto"
	"github.com/rift/rift/pkg/tunnel"
)

var errEdgeDraining = errors.New("edge draining")

// ensureConnected dials and authenticates if not already connected.
func (a *Agent) ensureConnected(ctx context.Context) error {
	a.mu.Lock()
	connected := a.connected
	a.mu.Unlock()
	if connected {
		return nil
	}
	return a.connect(ctx)
}

// connect dials the edge, opens the control stream, and authenticates.
func (a *Agent) connect(ctx context.Context) error {
	sess, err := a.dialer.Dial(ctx, a.cfg.Server)
	if err != nil {
		return &Error{Code: proto.CodeUpstreamUnreachable, Message: "cannot reach edge: " + err.Error()}
	}
	control, err := sess.OpenStream(ctx)
	if err != nil {
		_ = sess.CloseWithError(0, "no control stream")
		return &Error{Code: proto.CodeInternal, Message: err.Error()}
	}

	apiKey := a.cfg.APIKey
	if apiKey == "" {
		apiKey = "rk_dev_local" // dev fallback; a real edge requires a valid key
	}
	hello := &proto.Hello{
		ProtocolVersion: "1",
		AgentVersion:    Version,
		Os:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		ApiKey:          apiKey,
		RegionHint:      a.cfg.Region,
		Features:        []string{"quic", "tcp", "udp"},
	}
	if err := proto.WriteMsg(control, &proto.Envelope{Msg: &proto.Envelope_Hello{Hello: hello}}); err != nil {
		_ = sess.CloseWithError(0, "hello failed")
		return &Error{Code: proto.CodeInternal, Message: err.Error()}
	}
	_ = control.SetReadDeadline(time.Now().Add(15 * time.Second))
	env, err := proto.ReadMsg(control)
	_ = control.SetReadDeadline(time.Time{})
	if err != nil {
		_ = sess.CloseWithError(0, "no auth resp")
		return &Error{Code: proto.CodeInternal, Message: err.Error()}
	}
	ar := env.GetAuthResp()
	if ar == nil {
		_ = sess.CloseWithError(0, "bad auth resp")
		return &Error{Code: proto.CodeInternal, Message: "expected AuthResp"}
	}
	if !ar.Ok {
		_ = sess.CloseWithError(0, "auth denied")
		return errorFromResp(ar.Error)
	}

	a.mu.Lock()
	a.sess = sess
	a.control = control
	a.connected = true
	a.accountID = ar.AccountId
	a.plan = ar.Plan
	a.kind = string(sess.Kind())
	gen := a.sessGen.Add(1)
	a.mu.Unlock()

	a.log.Info("connected to edge", "edge", a.cfg.Server, "kind", sess.Kind(), "plan", ar.Plan)
	st := a.Status()
	a.emit(Event{Type: "status", Status: &st})

	go a.controlLoop(control, gen)
	go a.acceptLoop(sess, gen)
	return nil
}

func (a *Agent) controlLoop(control tunnel.Stream, gen uint64) {
	for {
		env, err := proto.ReadMsg(control)
		if err != nil {
			a.handleSessionDown(gen, err)
			return
		}
		switch env.GetMsg().(type) {
		case *proto.Envelope_BindResp:
			a.deliverBind(env.GetBindResp())
		case *proto.Envelope_Ping:
			_ = a.writeControl(&proto.Envelope{Msg: &proto.Envelope_Pong{Pong: &proto.Pong{
				TsUnixMs: env.GetPing().TsUnixMs,
			}}})
		case *proto.Envelope_Pong:
			// liveness ack
		case *proto.Envelope_Shutdown:
			a.log.Warn("edge draining", "reason", env.GetShutdown().Reason)
			a.emit(Event{Type: "error", Err: "edge draining; reconnecting"})
			a.handleSessionDown(gen, errEdgeDraining)
			return
		case *proto.Envelope_Error:
			a.emit(Event{Type: "error", Err: env.GetError().Message})
		}
	}
}

func (a *Agent) acceptLoop(sess tunnel.Session, gen uint64) {
	for {
		st, err := sess.AcceptStream(a.ctx)
		if err != nil {
			a.handleSessionDown(gen, err)
			return
		}
		go a.handleDataStream(st)
	}
}

// bind sends a BindTunnel and waits for the matching BindResp.
func (a *Agent) bind(ctx context.Context, spec TunnelSpec) (Tunnel, error) {
	clientTunnelID := "ct_" + randHex(6)
	ttype, _ := tunnelType(spec.Proto)

	options := map[string]string{}
	if spec.BasicAuth != "" {
		options["basic_auth"] = spec.BasicAuth
	}
	if spec.HostHeader != "" {
		options["host_header"] = spec.HostHeader
	}

	ch := make(chan *proto.BindResp, 1)
	a.pendMu.Lock()
	a.pendingBinds[clientTunnelID] = ch
	a.pendMu.Unlock()
	defer func() {
		a.pendMu.Lock()
		delete(a.pendingBinds, clientTunnelID)
		a.pendMu.Unlock()
	}()

	bind := &proto.BindTunnel{
		ClientTunnelId: clientTunnelID,
		Type:           ttype,
		Subdomain:      spec.Subdomain,
		CustomDomain:   spec.CustomDomain,
		RemotePort:     uint32(spec.RemotePort),
		Options:        options,
	}
	if err := a.writeControl(&proto.Envelope{Msg: &proto.Envelope_Bind{Bind: bind}}); err != nil {
		return Tunnel{}, err
	}

	select {
	case br := <-ch:
		if !br.Ok {
			return Tunnel{}, errorFromResp(br.Error)
		}
		at := &activeTunnel{
			spec:           spec,
			clientTunnelID: clientTunnelID,
			publicURL:      br.PublicUrl,
			assignedHost:   br.AssignedHost,
			assignedPort:   int(br.AssignedPort),
			status:         "online",
		}
		a.mu.Lock()
		a.tunnels[clientTunnelID] = at
		a.mu.Unlock()
		a.log.Info("tunnel online", "name", spec.Name, "url", br.PublicUrl)
		a.emit(Event{Type: "tunnel", Tunnel: ptr(at.view())})
		return at.view(), nil
	case <-time.After(20 * time.Second):
		return Tunnel{}, &Error{Code: proto.CodeInternal, Message: "bind timed out"}
	case <-ctx.Done():
		return Tunnel{}, ctx.Err()
	}
}

func (a *Agent) deliverBind(br *proto.BindResp) {
	if br == nil {
		return
	}
	a.pendMu.Lock()
	ch := a.pendingBinds[br.ClientTunnelId]
	a.pendMu.Unlock()
	if ch != nil {
		select {
		case ch <- br:
		default:
		}
	}
}

// activeTunnel lookup by client tunnel id.
func (a *Agent) lookupTunnel(id string) *activeTunnel {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tunnels[id]
}

func randHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
