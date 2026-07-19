package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"time"

	"github.com/rift/rift/pkg/proto"
	"github.com/rift/rift/pkg/tunnel"
)

const protocolVersion = "1"

// serveSession runs the full lifecycle of one agent session: control-stream
// handshake, authentication, then the control loop until the session dies.
func (s *Server) serveSession(ctx context.Context, sess tunnel.Session) {
	defer sess.CloseWithError(0, "session closed")

	// The agent opens the control stream first.
	hsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	control, err := sess.AcceptStream(hsCtx)
	cancel()
	if err != nil {
		s.log.Warn("no control stream", "err", err)
		return
	}

	hello, err := readHello(control)
	if err != nil {
		s.metrics.Errors.WithLabelValues("hello").Inc()
		return
	}
	if hello.ProtocolVersion != "" && hello.ProtocolVersion != protocolVersion {
		writeAuthErr(control, proto.CodeVersionUnsupported, "unsupported protocol version")
		return
	}

	accountID, plan, err := s.ent.Authenticate(ctx, hello.ApiKey)
	if err != nil {
		code := proto.CodeAuthInvalid
		if hello.ApiKey == "" {
			code = proto.CodeAuthRequired
		}
		writeAuthErr(control, code, "authentication failed")
		return
	}

	a := &agentSession{
		sess:      sess,
		control:   control,
		sessionID: newSessionID(),
		accountID: accountID,
		plan:      plan,
		apiKey:    hello.ApiKey,
		tunnels:   make(map[string]*boundTunnel),
	}

	if err := a.writeControl(&proto.Envelope{Msg: &proto.Envelope_AuthResp{AuthResp: &proto.AuthResp{
		Ok:        true,
		AccountId: accountID,
		Plan:      plan,
		SessionId: a.sessionID,
	}}}); err != nil {
		return
	}

	s.hub.addSession(a)
	s.metrics.SessionsActive.Inc()
	s.log.Info("agent session up", "session", a.sessionID, "account", accountID, "plan", plan, "kind", sess.Kind())

	defer func() {
		s.teardownSession(ctx, a)
		s.metrics.SessionsActive.Dec()
		s.log.Info("agent session down", "session", a.sessionID)
	}()

	// Heartbeat: periodically ping the agent; a write failure means it's gone.
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go s.heartbeat(hbCtx, a)

	s.controlLoop(ctx, a)
}

func (s *Server) controlLoop(ctx context.Context, a *agentSession) {
	for {
		if s.cfg.SessionIdle > 0 {
			_ = a.control.SetReadDeadline(time.Now().Add(s.cfg.SessionIdle))
		}
		env, err := proto.ReadMsg(a.control)
		if err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				s.log.Debug("control read ended", "session", a.sessionID, "err", err)
			}
			return
		}
		switch env.GetMsg().(type) {
		case *proto.Envelope_Bind:
			resp := s.handleBind(ctx, a, env.GetBind())
			if err := a.writeControl(&proto.Envelope{Msg: &proto.Envelope_BindResp{BindResp: resp}}); err != nil {
				return
			}
			if resp.Ok {
				s.log.Info("tunnel bound", "session", a.sessionID, "url", resp.PublicUrl)
			}
		case *proto.Envelope_Unbind:
			s.handleUnbind(ctx, a, env.GetUnbind().ClientTunnelId)
		case *proto.Envelope_Ping:
			_ = a.writeControl(&proto.Envelope{Msg: &proto.Envelope_Pong{Pong: &proto.Pong{
				TsUnixMs: env.GetPing().TsUnixMs,
			}}})
		case *proto.Envelope_Pong:
			// liveness ack — deadline already refreshed by the read
		default:
			// ignore unexpected control messages
		}
	}
}

func (s *Server) heartbeat(ctx context.Context, a *agentSession) {
	if s.cfg.HeartbeatInterval <= 0 {
		return
	}
	t := time.NewTicker(s.cfg.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := a.writeControl(&proto.Envelope{Msg: &proto.Envelope_Ping{Ping: &proto.Ping{
				TsUnixMs: time.Now().UnixMilli(),
			}}}); err != nil {
				_ = a.sess.CloseWithError(0, "heartbeat failed")
				return
			}
		}
	}
}

// teardownSession removes a dead session and releases all its tunnels.
func (s *Server) teardownSession(ctx context.Context, a *agentSession) {
	a.mu.Lock()
	tunnels := make([]*boundTunnel, 0, len(a.tunnels))
	for _, bt := range a.tunnels {
		tunnels = append(tunnels, bt)
	}
	a.tunnels = make(map[string]*boundTunnel)
	a.mu.Unlock()

	for _, bt := range tunnels {
		s.releaseTunnel(ctx, bt)
	}
	s.hub.removeSession(a)
}

func readHello(r tunnel.Stream) (*proto.Hello, error) {
	_ = r.SetReadDeadline(time.Now().Add(10 * time.Second))
	env, err := proto.ReadMsg(r)
	_ = r.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, err
	}
	hello := env.GetHello()
	if hello == nil {
		return nil, errors.New("server: first control message was not Hello")
	}
	return hello, nil
}

func writeAuthErr(w io.Writer, code, msg string) {
	_ = proto.WriteMsg(w, &proto.Envelope{Msg: &proto.Envelope_AuthResp{AuthResp: &proto.AuthResp{
		Ok:    false,
		Error: proto.NewError(code, msg),
	}}})
}

func newSessionID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "sess_" + hex.EncodeToString(buf)
}
