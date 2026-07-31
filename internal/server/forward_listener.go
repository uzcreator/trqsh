package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"time"

	"github.com/trqsh-uz/trqsh/pkg/proto"
)

// startForward brings up the internal, token-authenticated forwarding listener and
// advertises this edge's address so peers can hand it traffic for tunnels homed
// here. It is INTERNAL-ONLY: bind ForwardAddr on a private interface and gate it at
// the firewall to peer edges (see deploy/terraform/edge.tf). The shared
// InternalToken authenticates every hop, checked constant-time in serveForward.
func (s *Server) startForward(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.ForwardAddr)
	if err != nil {
		return fmt.Errorf("server: forward listen %s: %w", s.cfg.ForwardAddr, err)
	}
	s.forwardLn = ln

	advertise := s.cfg.ForwardAdvertiseAddr
	if advertise == "" {
		advertise = ln.Addr().String()
	}
	s.fwdAddr = advertise

	// Announce immediately so peers can resolve us without waiting a heartbeat.
	rctx, cancel := context.WithTimeout(ctx, forwardDialTimeout)
	if err := s.reg.RegisterEdge(rctx, s.cfg.EdgeID, advertise, s.edgeTTL()); err != nil {
		s.log.Warn("forward: initial edge register failed", "err", err)
	}
	cancel()

	s.log.Info("forward listener up", "addr", ln.Addr().String(), "advertise", advertise)
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.acceptForward(ctx, ln) }()
	return nil
}

func (s *Server) acceptForward(ctx context.Context, ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.serveForward(c) }()
	}
}

// serveForward handles one inbound inter-edge connection: read the forward header,
// verify the shared token (constant time, same as the edge↔API internal guard),
// then serve the announced route locally by presenting the peer conn to the normal
// serving path as the original public connection.
func (s *Server) serveForward(peer net.Conn) {
	_ = peer.SetReadDeadline(time.Now().Add(forwardHandshakeTimeout))
	hdr, err := proto.ReadStreamInit(peer)
	if err != nil {
		peer.Close()
		return
	}
	_ = peer.SetReadDeadline(time.Time{})

	token := ""
	if hdr.Meta != nil {
		token = hdr.Meta[forwardTokenMeta]
	}
	if s.cfg.InternalToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.InternalToken)) != 1 {
		s.metrics.Errors.WithLabelValues("forward_auth").Inc()
		peer.Close()
		return
	}

	fc := &forwardedConn{Conn: peer, remote: forwardAddr(hdr.RemoteAddr)}
	switch hdr.Proto {
	case "http", "https":
		// serveHTTPConn owns the conn (and closes it). It reads the replayed request
		// and serves it against the local hub; if the local hub misses (a stale
		// binding) it will NOT re-forward — the single-hop guard there refuses to
		// forward a forwardedConn, so it 404s instead of looping.
		s.metrics.Forwards.WithLabelValues("in").Inc()
		s.serveHTTPConn(fc, hdr.Proto)
	default:
		// Port (tcp/udp) forwarding is intentionally not offered yet — see the note
		// in ingress_tcp.go / ingress_udp.go. Reject anything unexpected.
		s.metrics.Errors.WithLabelValues("forward_proto").Inc()
		peer.Close()
	}
}

// runPresence renews this edge's forwarding address AND every locally-homed route
// binding on the heartbeat cadence, so neither lapses in the shared registry while
// the edge is live. It wires the registry's Refresh (which the single-edge MVP
// never needed to call) so multi-edge routes survive past their 2×heartbeat TTL.
// With HeartbeatInterval==0 (tests) TTLs are infinite, so no refresher is needed.
func (s *Server) runPresence(ctx context.Context) {
	if s.cfg.HeartbeatInterval <= 0 {
		return
	}
	ttl := s.edgeTTL()
	t := time.NewTicker(s.cfg.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rctx, cancel := context.WithTimeout(context.Background(), forwardDialTimeout)
			if err := s.reg.RegisterEdge(rctx, s.cfg.EdgeID, s.fwdAddr, ttl); err != nil {
				s.log.Warn("forward: edge address refresh failed", "err", err)
			}
			for _, r := range s.hub.SnapshotRoutes() {
				_ = s.reg.Refresh(rctx, r, ttl)
			}
			cancel()
		}
	}
}
