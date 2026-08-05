package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/quic-go/quic-go/http3"

	"github.com/trqsh-uz/trqsh/pkg/proto"
)

// h3Ingress is the optional HTTP/3 (QUIC) public ingress. It serves the SAME
// routing as the HTTP/1.1 ingress (serveHTTPConn) — reserved-host reverse proxy,
// tunnel weld, branded 404 — but over QUIC, so a browser can multiplex many
// concurrent requests on one connection without HTTP/1.1 head-of-line blocking
// (the pain when tunneling dev servers like Vite that fan a single page out into
// hundreds of small module requests, all queued behind the browser's ~6
// per-host HTTP/1.1 connections). It is enabled only when TRQSH_H3_ADDR is set;
// the TCP HTTP/1.1 ingress is unchanged and remains the bootstrap over which
// browsers discover h3 via the Alt-Svc header.
//
// WebSockets/upgrades are intentionally NOT offered over h3 here: HTTP/3 carries
// them via Extended CONNECT (RFC 9220), which a server must opt into with
// SETTINGS_ENABLE_CONNECT_PROTOCOL. Leaving it off means browsers keep doing
// WebSockets over the TCP h1.1 path and use h3 only for ordinary
// request/response — exactly the traffic that suffers from HOL blocking.
// Cross-edge forwarding is likewise h1.1-only for now: an h3 request for a
// tunnel homed on another edge 404s locally rather than being forwarded.
type h3Ingress struct {
	srv    *http3.Server
	conn   net.PacketConn
	addr   net.Addr
	altSvc string // value advertised in the Alt-Svc header, e.g. `h3=":443"; ma=86400`
}

// close shuts the HTTP/3 server (stops accepting, closes active conns) and then
// its UDP socket.
func (h *h3Ingress) close() error {
	var err error
	if h.srv != nil {
		err = h.srv.Close()
	}
	if h.conn != nil {
		if cerr := h.conn.Close(); err == nil {
			err = cerr
		}
	}
	return err
}

// startH3 brings up the HTTP/3 ingress on TRQSH_H3_ADDR (UDP). The caller treats
// a returned error as non-fatal (logs it and keeps serving HTTP/1.1).
func (s *Server) startH3(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.cfg.H3Addr)
	if err != nil {
		return fmt.Errorf("server: resolve TRQSH_H3_ADDR %q: %w", s.cfg.H3Addr, err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("server: h3 udp listen %s: %w", s.cfg.H3Addr, err)
	}

	// HTTP/3 requires TLS 1.3; reuse the same per-SNI certificates as the TCP
	// ingress. ConfigureTLSConfig forces ALPN to "h3".
	tlsConf := http3.ConfigureTLSConfig(&tls.Config{
		GetCertificate: s.certs.GetCertificate,
		MinVersion:     tls.VersionTLS13,
	})
	// The port browsers must actually dial for h3 — the bound port, unless a
	// public port is configured because an L4 map (Docker/firewall) redirects a
	// different public UDP port onto the bound one.
	advPort := portOf(conn.LocalAddr())
	if s.cfg.H3AdvertisePort > 0 {
		advPort = s.cfg.H3AdvertisePort
	}
	h3 := &http3.Server{
		TLSConfig:      tlsConf,
		Handler:        http.HandlerFunc(s.serveH3),
		Port:           advPort,
		IdleTimeout:    httpIdleTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	// Alt-Svc tells browsers (on their initial TCP/HTTP-1.1 connection) that the
	// same origin is reachable over HTTP/3 on this UDP port, so they upgrade
	// subsequent requests. ma is the advertisement's lifetime in seconds.
	altSvc := fmt.Sprintf(`h3=":%d"; ma=86400`, advPort)
	s.h3 = &h3Ingress{srv: h3, conn: conn, addr: conn.LocalAddr(), altSvc: altSvc}

	s.log.Info("http/3 ingress up", "addr", conn.LocalAddr().String())
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Serve returns once the server is Closed during drain (ctx already
		// canceled); only surface errors that happen while we still expected to
		// be serving.
		if err := h3.Serve(conn); err != nil && ctx.Err() == nil &&
			!errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			s.log.Warn("http/3 ingress stopped", "err", err)
		}
	}()
	return nil
}

// serveH3 is the HTTP/3 request handler. It mirrors serveHTTPConn's routing but
// writes through an http.ResponseWriter instead of a raw net.Conn.
func (s *Server) serveH3(w http.ResponseWriter, r *http.Request) {
	const scheme = "https" // HTTP/3 is TLS-only
	host := hostOnly(r.Host)

	bt := s.hub.LookupHost(host)
	if bt == nil {
		// Reserved control-plane hosts (apex/www → site, app → dashboard, api →
		// API) are reverse-proxied; anything else gets the branded 404.
		if up := s.reservedUpstream(host); up != "" {
			s.proxyToUpstreamH3(w, r, scheme, up)
			return
		}
		s.metrics.Errors.WithLabelValues("no_route").Inc()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, branded404(host))
		return
	}
	if !checkBasicAuth(r, bt.options) {
		w.Header().Set("WWW-Authenticate", `Basic realm="trqsh"`)
		http.Error(w, "401 unauthorized", http.StatusUnauthorized)
		return
	}

	s.metrics.Requests.WithLabelValues(scheme).Inc()
	s.metrics.StreamsOpened.WithLabelValues(scheme).Inc()

	rid := newRequestID()
	if hh := bt.options["host_header"]; hh != "" {
		r.Host = hh
	}
	r.Header.Set("X-Forwarded-Proto", scheme)
	r.Header.Set("X-Forwarded-Host", host)
	r.Header.Set("X-Request-Id", rid)

	init := &proto.StreamInit{
		ClientTunnelId: bt.clientTunnelID,
		RemoteAddr:     r.RemoteAddr,
		Proto:          scheme,
		Meta:           map[string]string{"host": host, "request_id": rid},
	}
	st, err := bt.session.openDataStream(r.Context(), init)
	if err != nil {
		s.metrics.Errors.WithLabelValues("open_stream").Inc()
		http.Error(w, "502 tunnel unreachable", http.StatusBadGateway)
		return
	}
	defer func() { _ = st.Close() }()

	// Serialize the request to the agent as HTTP/1.1 (agents speak h1.x on the
	// data stream) and relay its response back over h3. r.Write streams the body,
	// so large uploads aren't buffered in memory.
	if err := r.Write(st); err != nil {
		http.Error(w, "502 tunnel write failed", http.StatusBadGateway)
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(st), r)
	if err != nil {
		s.metrics.Errors.WithLabelValues("bad_response").Inc()
		http.Error(w, "502 bad upstream response", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Connection/Transfer-Encoding et al. are HTTP/1.x hop-by-hop headers and are
	// invalid over HTTP/3; strip them before relaying the response.
	removeHopByHopHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)
	s.setAltSvc(w.Header())
	w.WriteHeader(resp.StatusCode)
	n, _ := io.Copy(w, resp.Body)

	s.usage.record(bt.accountID, bt.clientTunnelID, max64(r.ContentLength, 0), n, 1)
}

// proxyToUpstreamH3 reverse-proxies one h3 request to an internal reserved-host
// upstream (site/dashboard/api) and relays the response. It mirrors
// proxyToUpstream but writes through an http.ResponseWriter.
func (s *Server) proxyToUpstreamH3(w http.ResponseWriter, r *http.Request, scheme, upstream string) {
	u, err := url.Parse(upstream)
	if err != nil || u.Host == "" {
		http.Error(w, "502 upstream misconfigured", http.StatusBadGateway)
		return
	}

	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	outReq.URL.Scheme = u.Scheme
	outReq.URL.Host = u.Host
	removeHopByHopHeaders(outReq.Header)
	outReq.Header.Set("X-Forwarded-Proto", scheme)
	outReq.Header.Set("X-Forwarded-Host", r.Host)
	// Overwrite XFF with the true peer IP: the edge is the first public hop, so
	// any inbound XFF is client-supplied and untrusted (matches proxyToUpstream).
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		outReq.Header.Set("X-Forwarded-For", ip)
	} else if r.RemoteAddr != "" {
		outReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	}

	resp, err := s.proxyClient.Do(outReq)
	if err != nil {
		label, msg := "upstream_bad_response", "502 upstream bad response"
		var netErr *net.OpError
		if errors.As(err, &netErr) && netErr.Op == "dial" {
			label, msg = "upstream_unreachable", "502 upstream unreachable"
		}
		s.metrics.Errors.WithLabelValues(label).Inc()
		http.Error(w, msg, http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if scheme == "https" {
		resp.Header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	removeHopByHopHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)
	s.setAltSvc(w.Header())
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	s.metrics.Requests.WithLabelValues(scheme).Inc()
}

// setAltSvc advertises the HTTP/3 endpoint on a response header when the h3
// ingress is up; a no-op otherwise. Called on both h1.1 responses (so browsers
// discover h3) and h3 responses (so the advertisement stays fresh).
func (s *Server) setAltSvc(h http.Header) {
	if s.h3 != nil && s.h3.altSvc != "" {
		h.Set("Alt-Svc", s.h3.altSvc)
	}
}

// copyHeader appends every value of src into dst.
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// portOf extracts the numeric port from a net.Addr (0 if it can't be parsed).
func portOf(a net.Addr) int {
	if ua, ok := a.(*net.UDPAddr); ok {
		return ua.Port
	}
	if _, p, err := net.SplitHostPort(a.String()); err == nil {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	return 0
}
