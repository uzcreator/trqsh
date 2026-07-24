package server

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/trqsh-uz/trqsh/pkg/proto"
)

const httpIdleTimeout = 75 * time.Second

func (s *Server) startHTTP(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("server: http listen %s: %w", s.cfg.HTTPAddr, err)
	}
	s.httpLn = ln
	s.log.Info("http ingress up", "addr", s.cfg.HTTPAddr)
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.acceptHTTP(ctx, ln, "http") }()
	return nil
}

func (s *Server) startHTTPS(ctx context.Context) error {
	raw, err := net.Listen("tcp", s.cfg.HTTPSAddr)
	if err != nil {
		return fmt.Errorf("server: https listen %s: %w", s.cfg.HTTPSAddr, err)
	}
	s.httpsLn = raw
	tlsLn := tlsListener(raw, s.certs)
	s.log.Info("https ingress up", "addr", s.cfg.HTTPSAddr)
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.acceptHTTP(ctx, tlsLn, "https") }()
	return nil
}

func (s *Server) acceptHTTP(ctx context.Context, ln net.Listener, scheme string) {
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.serveHTTPConn(c, scheme) }()
	}
}

// serveHTTPConn proxies HTTP over a public connection: it parses each request,
// routes by Host, opens a fresh data stream to the owning agent, and forwards
// the request/response. Upgrades (websockets) switch to a raw bidirectional weld.
func (s *Server) serveHTTPConn(conn net.Conn, scheme string) {
	defer conn.Close()
	br := bufio.NewReader(conn)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(httpIdleTimeout))
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Time{})

		host := hostOnly(req.Host)
		bt := s.hub.LookupHost(host)
		if bt == nil {
			// Reserved control-plane hosts (apex/www → site, app → dashboard,
			// api → API) are reverse-proxied; anything else gets the branded 404.
			if up := s.reservedUpstream(host); up != "" {
				if s.proxyToUpstream(conn, req, scheme, up) {
					continue
				}
				return
			}
			s.metrics.Errors.WithLabelValues("no_route").Inc()
			writeHTTPResponse(conn, http.StatusNotFound, "Not Found", "text/html; charset=utf-8", branded404(host))
			return
		}
		if !checkBasicAuth(req, bt.options) {
			writeHTTPResponse(conn, http.StatusUnauthorized, "Unauthorized", "text/plain", "401 unauthorized\n")
			// keep the connection open for a retry with credentials
			continue
		}

		s.metrics.Requests.WithLabelValues(scheme).Inc()
		s.metrics.StreamsOpened.WithLabelValues(scheme).Inc()

		rid := newRequestID()
		if hh := bt.options["host_header"]; hh != "" {
			req.Host = hh
		}
		req.Header.Set("X-Forwarded-Proto", scheme)
		req.Header.Set("X-Forwarded-Host", host)
		req.Header.Set("X-Request-Id", rid)

		init := &proto.StreamInit{
			ClientTunnelId: bt.clientTunnelID,
			RemoteAddr:     conn.RemoteAddr().String(),
			Proto:          scheme,
			Meta:           map[string]string{"host": host, "request_id": rid},
		}
		st, err := bt.session.openDataStream(context.Background(), init)
		if err != nil {
			s.metrics.Errors.WithLabelValues("open_stream").Inc()
			writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 tunnel unreachable\n")
			return
		}

		if isUpgrade(req) {
			// Forward the request head, then weld raw bytes both ways.
			if err := req.Write(st); err != nil {
				st.Close()
				return
			}
			up, down := rawJoin(conn, br, st)
			s.metrics.countBytes(up, down)
			s.usage.record(bt.accountID, bt.clientTunnelID, up, down, 1)
			return
		}

		if err := req.Write(st); err != nil {
			st.Close()
			return
		}
		resp, err := http.ReadResponse(bufio.NewReader(st), req)
		if err != nil {
			st.Close()
			s.metrics.Errors.WithLabelValues("bad_response").Inc()
			writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 bad upstream response\n")
			return
		}
		clientClose := req.Close || resp.Close
		if err := resp.Write(conn); err != nil {
			resp.Body.Close()
			st.Close()
			return
		}
		resp.Body.Close()
		st.Close()

		s.usage.record(bt.accountID, bt.clientTunnelID, req.ContentLength, max64(resp.ContentLength, 0), 1)
		if clientClose {
			return
		}
	}
}

func hostOnly(hostport string) string {
	h := strings.ToLower(strings.TrimSpace(hostport))
	if strings.Contains(h, ":") {
		if host, _, err := net.SplitHostPort(h); err == nil {
			return host
		}
	}
	return h
}

// reservedUpstream returns the internal upstream for a reserved control-plane
// host (apex/www → site, app → dashboard, api → API), or "" if the host is not
// reserved or its upstream isn't configured.
func (s *Server) reservedUpstream(host string) string {
	base := strings.ToLower(s.cfg.BaseDomain)
	switch host {
	case base, "www." + base:
		return s.cfg.SiteUpstream
	case "app." + base:
		return s.cfg.AppUpstream
	case "api." + base:
		return s.cfg.APIUpstream
	}
	return ""
}

// proxyToUpstream reverse-proxies one request to an internal upstream and writes
// the response back to conn. It returns true if the client connection may be
// reused (HTTP keep-alive). A fresh upstream connection is used per request.
func (s *Server) proxyToUpstream(conn net.Conn, req *http.Request, scheme, upstream string) bool {
	u, err := url.Parse(upstream)
	if err != nil || u.Host == "" {
		writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 upstream misconfigured\n")
		return false
	}
	up, err := net.DialTimeout("tcp", u.Host, 10*time.Second)
	if err != nil {
		s.metrics.Errors.WithLabelValues("upstream_unreachable").Inc()
		writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 upstream unreachable\n")
		return false
	}
	defer up.Close()

	req.Header.Set("X-Forwarded-Proto", scheme)
	req.Header.Set("X-Forwarded-Host", req.Host)
	_ = up.SetDeadline(time.Now().Add(httpIdleTimeout))
	if err := req.Write(up); err != nil {
		return false
	}
	resp, err := http.ReadResponse(bufio.NewReader(up), req)
	if err != nil {
		s.metrics.Errors.WithLabelValues("upstream_bad_response").Inc()
		writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 upstream bad response\n")
		return false
	}
	defer resp.Body.Close()
	keepAlive := !req.Close && !resp.Close
	if err := resp.Write(conn); err != nil {
		return false
	}
	s.metrics.Requests.WithLabelValues(scheme).Inc()
	return keepAlive
}

func isUpgrade(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Connection"), "upgrade") ||
		strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade")
}

// checkBasicAuth enforces the "basic_auth" tunnel option ("user:pass"), if set.
func checkBasicAuth(req *http.Request, options map[string]string) bool {
	want := options["basic_auth"]
	if want == "" {
		return true
	}
	const prefix = "Basic "
	h := req.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	dec, err := base64.StdEncoding.DecodeString(h[len(prefix):])
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(dec, []byte(want)) == 1
}

func newRequestID() string {
	return "req_" + randSubdomain(12)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
