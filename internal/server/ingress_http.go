package server

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
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
	defer func() { _ = conn.Close() }()
	br := bufio.NewReader(conn)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(httpIdleTimeout))
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Time{})

		host := hostOnly(req.Host)

		// Force HTTPS for our own control-plane hosts (marketing site, dashboard,
		// API) so a fresh device that typed "trqsh.uz" never stays on an insecure
		// http:// connection. Tunnels keep serving both schemes as before. Certs
		// use ACME DNS-01, so nothing needs answering over plain HTTP.
		if scheme == "http" && s.reservedUpstream(host) != "" {
			redirectToHTTPS(conn, host, req)
			return
		}

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
			// Cross-edge fallback: the tunnel may be homed on another edge. Hand the
			// connection to the owning edge instead of 404ing. A connection that
			// itself arrived over the inter-edge hop is never forwarded again — the
			// single-hop guard (a *forwardedConn) 404s locally instead of looping.
			if s.forwardingEnabled() {
				if _, isFwd := conn.(*forwardedConn); !isFwd {
					if s.forwardHTTP(conn, br, req, host, scheme) {
						return
					}
				}
			}
			s.metrics.Errors.WithLabelValues("no_route").Inc()
			writeHTTPResponse(conn, http.StatusNotFound, "Not Found", "text/html; charset=utf-8", branded404(host))
			return
		}
		if !checkBasicAuth(req, bt.options) {
			writeUnauthorized(conn)
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
				_ = st.Close()
				return
			}
			up, down := rawJoin(conn, br, st)
			// A forwardedConn's bytes were already counted once by the relaying edge
			// as they crossed its hop (forward.go's forwardHTTP, after its own
			// rawJoin) — don't double-count the same bytes again here. Usage/billing
			// is unaffected: only the edge that actually owns bt (this one, on the
			// receiving side of a hop) has an accountID to attribute it to, so
			// usage.record is correctly called exactly once regardless.
			if _, isFwd := conn.(*forwardedConn); !isFwd {
				s.metrics.countBytes(up, down)
			}
			s.usage.record(bt.accountID, bt.clientTunnelID, up, down, 1)
			return
		}

		if err := req.Write(st); err != nil {
			_ = st.Close()
			return
		}
		resp, err := http.ReadResponse(bufio.NewReader(st), req)
		if err != nil {
			_ = st.Close()
			s.metrics.Errors.WithLabelValues("bad_response").Inc()
			writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 bad upstream response\n")
			return
		}
		clientClose := req.Close || resp.Close
		// Advertise HTTP/3 so the browser upgrades subsequent requests off this
		// HOL-blocking HTTP/1.1 connection (no-op when h3 is disabled).
		s.setAltSvc(resp.Header)
		if err := resp.Write(conn); err != nil {
			_ = resp.Body.Close()
			_ = st.Close()
			return
		}
		_ = resp.Body.Close()
		_ = st.Close()

		// Both lengths are -1 when unknown (e.g. a chunked request or response with
		// no Content-Length); clamp to 0 so an unknown length never records negative
		// bytes into usage/billing. resp was already guarded; req had been missed.
		s.usage.record(bt.accountID, bt.clientTunnelID, max64(req.ContentLength, 0), max64(resp.ContentLength, 0), 1)
		if clientClose {
			return
		}
	}
}

// redirectToHTTPS writes a 301 pointing at the same host+path over HTTPS.
func redirectToHTTPS(conn net.Conn, host string, req *http.Request) {
	target := "https://" + host + req.URL.RequestURI()
	body := "Redirecting to " + target + "\n"
	_, _ = fmt.Fprintf(conn, "HTTP/1.1 301 Moved Permanently\r\n"+
		"Location: %s\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n\r\n%s", target, len(body), body)
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
// host (apex/www → site, app → dashboard, api + approve → API), or "" if the host
// is not reserved or its upstream isn't configured.
func (s *Server) reservedUpstream(host string) string {
	base := strings.ToLower(s.cfg.BaseDomain)
	switch host {
	case base, "www." + base:
		return s.cfg.SiteUpstream
	case "app." + base:
		return s.cfg.AppUpstream
	case "api." + base, "approve." + base:
		// The admin console (approve.<base>) is served by the API itself, on the
		// same origin as its /v1/admin/* endpoints.
		return s.cfg.APIUpstream
	}
	return ""
}

// newReverseProxyClient builds the shared client for the reserved-host reverse
// proxy. This is 1:1 edge→internal-service traffic (site/dashboard/api), so the
// pool raises MaxIdleConnsPerHost well above Go's default of 2 to keep upstream
// connections warm instead of dialing fresh per request. Compression is disabled
// so the edge forwards the client's Accept-Encoding and the upstream's
// Content-Encoding transparently (no auto-gzip/decompress). Redirects are handled
// by the browser, not followed here.
func newReverseProxyClient() *http.Client {
	t := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true,
	}
	return &http.Client{
		Transport:     t,
		Timeout:       httpIdleTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// hopByHopHeaders are stripped from the outbound request before it is handed to
// the pooled Transport. Leaving them in place (Connection especially) would fight
// http.Transport's own keep-alive/pooling — e.g. a forwarded "Connection: close"
// pins one upstream connection per request, defeating the pool.
var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func removeHopByHopHeaders(h http.Header) {
	// Anything named by a Connection header is itself hop-by-hop; drop those first.
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = strings.TrimSpace(sf); sf != "" {
				h.Del(sf)
			}
		}
	}
	for _, k := range hopByHopHeaders {
		h.Del(k)
	}
}

// proxyToUpstream reverse-proxies one request to an internal upstream and writes
// the response back to conn. It returns true if the client (public-side)
// connection may be reused (HTTP keep-alive) — a separate concern from the pooled
// upstream connection, which s.proxyClient reuses across requests.
func (s *Server) proxyToUpstream(conn net.Conn, req *http.Request, scheme, upstream string) bool {
	u, err := url.Parse(upstream)
	if err != nil || u.Host == "" {
		writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", "502 upstream misconfigured\n")
		return false
	}

	// Build the outbound request. The inbound req came from http.ReadRequest in
	// origin form, so its URL carries only Path/RawQuery and RequestURI is set —
	// both must be fixed for a client request. Cloning preserves req.Host (sent to
	// the upstream) and the header set; hop-by-hop headers are then stripped so the
	// pool can reuse connections.
	outReq := req.Clone(context.Background())
	outReq.RequestURI = ""
	outReq.URL.Scheme = u.Scheme
	outReq.URL.Host = u.Host
	removeHopByHopHeaders(outReq.Header)

	outReq.Header.Set("X-Forwarded-Proto", scheme)
	outReq.Header.Set("X-Forwarded-Host", req.Host)
	// Overwrite (not append) XFF with the true peer IP: the edge is the first
	// public hop, so any inbound XFF is client-supplied and untrusted. Downstream
	// clientIP() (TRQSH_TRUST_PROXY) reads the first element, so without this every
	// client collapses to the Docker-bridge IP and shares one rate-limit bucket.
	if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
		outReq.Header.Set("X-Forwarded-For", ip)
	} else {
		outReq.Header.Set("X-Forwarded-For", conn.RemoteAddr().String())
	}

	resp, err := s.proxyClient.Do(outReq)
	if err != nil {
		// A dial failure never reached the upstream; anything later (bad/truncated
		// response, timeout after connect) is a response error. Preserve the labels
		// the manual dial+read path used.
		label, msg := "upstream_bad_response", "502 upstream bad response\n"
		var netErr *net.OpError
		if errors.As(err, &netErr) && netErr.Op == "dial" {
			label, msg = "upstream_unreachable", "502 upstream unreachable\n"
		}
		s.metrics.Errors.WithLabelValues(label).Inc()
		writeHTTPResponse(conn, http.StatusBadGateway, "Bad Gateway", "text/plain", msg)
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if scheme == "https" {
		// Reserved control-plane hosts (site/dashboard/api) are always served
		// over TLS; tell browsers to remember that for a year.
		resp.Header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	keepAlive := !req.Close && !resp.Close
	s.setAltSvc(resp.Header)
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

// writeUnauthorized sends the 401 challenge for a --basic-auth protected
// tunnel. Unlike writeHTTPResponse, it sets WWW-Authenticate so browsers
// actually show their native credential prompt (without it, a 401 with no
// challenge header is just an error page — the user has no way to enter
// credentials), and it keeps the connection alive so the caller's retry loop
// can read the follow-up request with credentials on the same connection
// instead of forcing a fresh TCP/TLS handshake.
func writeUnauthorized(conn net.Conn) {
	const body = "401 unauthorized\n"
	_, _ = fmt.Fprintf(conn, "HTTP/1.1 401 Unauthorized\r\n"+
		"WWW-Authenticate: Basic realm=\"trqsh\"\r\n"+
		"Content-Type: text/plain\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: keep-alive\r\n\r\n%s", len(body), body)
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
