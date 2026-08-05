package agent

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
	"github.com/trqsh-uz/trqsh/pkg/proto"
	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

const localDialTimeout = 10 * time.Second

// hopHeaders are per-connection headers that must not be forwarded across a
// proxy hop (RFC 7230 §6.1). They are stripped from the request handed to the
// pooled transport, which manages connection reuse itself.
var hopHeaders = []string{
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

// handleDataStream services one edge-initiated data stream: read its StreamInit,
// find the tunnel, and forward to the local service (teeing HTTP to the inspector).
func (a *Agent) handleDataStream(st tunnel.Stream) {
	defer func() { _ = st.Close() }()
	si, err := proto.ReadStreamInit(st)
	if err != nil {
		return
	}
	at := a.lookupTunnel(si.ClientTunnelId)
	if at == nil {
		return
	}
	at.metrics.connections.Add(1)

	switch strings.ToLower(si.Proto) {
	case "http", "https":
		a.forwardHTTP(st, at, si)
	case "udp":
		a.forwardUDP(st, at)
	default: // tcp, tls
		a.forwardRaw(st, at)
	}
}

func (a *Agent) forwardRaw(st tunnel.Stream, at *activeTunnel) {
	local, err := net.DialTimeout("tcp", normalizeLocalAddr(at.spec.Addr), localDialTimeout)
	if err != nil {
		a.emit(Event{Type: "error", Err: "dial local " + at.spec.Addr + ": " + err.Error()})
		return
	}
	fromPub, toPub := weld(local, st, st)
	at.metrics.bytesIn.Add(fromPub)
	at.metrics.bytesOut.Add(toPub)
}

// forwardHTTP forwards one HTTP exchange to the local service over a pooled,
// keep-alive connection and streams the response straight back to the edge,
// teeing a bounded copy of each body to the request inspector as it flows (so
// large or streaming responses are never buffered before delivery).
func (a *Agent) forwardHTTP(st tunnel.Stream, at *activeTunnel, si *proto.StreamInit) {
	br := bufio.NewReader(st)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	start := time.Now()

	// Websocket/other Upgrade requests can't use the pooled client; weld raw
	// bytes over a dedicated local connection instead.
	if isUpgradeReq(req) {
		a.forwardUpgrade(st, br, at, req)
		return
	}

	reqMethod, reqHost := req.Method, req.Host
	reqPath := req.URL.RequestURI()
	reqHeaders := hdrMap(req.Header)

	// Tee the request body (bounded) as the transport reads it, then reshape the
	// server-parsed request into a client request aimed at the local service.
	reqCap := &capWriter{max: inspect.MaxBodyCapture}
	if req.Body != nil {
		req.Body = &teeReadCloser{r: io.TeeReader(req.Body, reqCap), c: req.Body}
	}
	localAddr := normalizeLocalAddr(at.spec.Addr)
	outURL := *req.URL
	outURL.Scheme = "http"
	outURL.Host = localAddr
	req.URL = &outURL
	req.RequestURI = "" // must be empty for client requests sent via RoundTrip
	req.Close = false
	for _, h := range hopHeaders {
		req.Header.Del(h)
	}

	resp, err := a.localTransport().RoundTrip(req.WithContext(a.ctx))
	if err != nil {
		writeHTTPError(st, http.StatusBadGateway, "local service unreachable")
		a.emit(Event{Type: "error", Err: "forward to " + at.spec.Addr + ": " + err.Error()})
		return
	}

	// Stream the response to the edge, teeing a bounded copy for the inspector.
	respCap := &capWriter{max: inspect.MaxBodyCapture}
	body := resp.Body
	resp.Body = &teeReadCloser{r: io.TeeReader(body, respCap), c: body}
	status := resp.StatusCode
	respHeaders := hdrMap(resp.Header)

	writeErr := resp.Write(st)
	_ = resp.Body.Close()
	if writeErr != nil {
		return
	}

	at.metrics.requests.Add(1)
	at.metrics.bytesIn.Add(reqCap.total)
	at.metrics.bytesOut.Add(respCap.total)

	captured := a.insp.Add(inspect.CapturedRequest{
		TunnelID:    at.clientTunnelID,
		Proto:       si.Proto,
		Method:      reqMethod,
		Host:        reqHost,
		Path:        reqPath,
		Status:      status,
		StartedAt:   start,
		DurationMs:  time.Since(start).Milliseconds(),
		ReqHeaders:  reqHeaders,
		RespHeaders: respHeaders,
		ReqBody:     reqCap.buf,
		RespBody:    respCap.buf,
		BytesIn:     reqCap.total,
		BytesOut:    respCap.total,
		LocalAddr:   localAddr,
	})
	a.emit(Event{Type: "request", Request: &captured})
}

// forwardUpgrade handles websocket/other Upgrade requests by dialing a dedicated
// local connection and welding raw bytes both ways, including any bytes already
// buffered off the edge stream while reading the request head.
func (a *Agent) forwardUpgrade(st tunnel.Stream, clientRead io.Reader, at *activeTunnel, req *http.Request) {
	local, err := net.DialTimeout("tcp", normalizeLocalAddr(at.spec.Addr), localDialTimeout)
	if err != nil {
		writeHTTPError(st, http.StatusBadGateway, "local service unreachable")
		a.emit(Event{Type: "error", Err: "dial local " + at.spec.Addr + ": " + err.Error()})
		return
	}
	if err := req.Write(local); err != nil {
		_ = local.Close()
		return
	}
	fromPub, toPub := weld(local, st, clientRead)
	at.metrics.bytesIn.Add(fromPub)
	at.metrics.bytesOut.Add(toPub)
}

// forwardUDP translates the uint16-length-framed datagram stream (matching the
// edge's ingress_udp framing) to and from a local UDP socket.
func (a *Agent) forwardUDP(st tunnel.Stream, at *activeTunnel) {
	local, err := net.Dial("udp", normalizeLocalAddr(at.spec.Addr))
	if err != nil {
		a.emit(Event{Type: "error", Err: "dial local udp " + at.spec.Addr + ": " + err.Error()})
		return
	}
	defer func() { _ = local.Close() }()

	// stream -> local
	go func() {
		var hdr [2]byte
		for {
			if _, err := io.ReadFull(st, hdr[:]); err != nil {
				_ = local.Close()
				return
			}
			n := binary.BigEndian.Uint16(hdr[:])
			payload := make([]byte, n)
			if _, err := io.ReadFull(st, payload); err != nil {
				_ = local.Close()
				return
			}
			if _, err := local.Write(payload); err != nil {
				return
			}
			at.metrics.bytesIn.Add(int64(n))
		}
	}()

	// local -> stream
	buf := make([]byte, 65535)
	for {
		n, err := local.Read(buf)
		if err != nil {
			return
		}
		var hdr [2]byte
		binary.BigEndian.PutUint16(hdr[:], uint16(n)) // #nosec G115 -- n <= len(buf) == 65535 (io.Reader's own contract)
		if _, err := st.Write(hdr[:]); err != nil {
			return
		}
		if _, err := st.Write(buf[:n]); err != nil {
			return
		}
		at.metrics.bytesOut.Add(int64(n))
	}
}

// weld copies bytes both ways between a local conn and an edge stream until
// either side closes. fromEdge is the reader for the edge side (the stream
// itself, or a *bufio.Reader when bytes were already buffered off it). It
// returns bytes read from the public side (into local) and sent to the public
// side (out of local).
func weld(local net.Conn, st tunnel.Stream, fromEdge io.Reader) (fromPublic, toPublic int64) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		fromPublic, _ = io.Copy(local, fromEdge)
		_ = local.Close()
		_ = st.Close()
	}()
	go func() {
		defer wg.Done()
		toPublic, _ = io.Copy(st, local)
		_ = st.Close()
		_ = local.Close()
	}()
	wg.Wait()
	return fromPublic, toPublic
}

// capWriter counts every byte written and retains only the first max of them for
// the inspector, discarding the rest. Used as the sink of an io.TeeReader so a
// bounded copy is captured while the body streams through untouched.
type capWriter struct {
	max   int
	buf   []byte
	total int64
}

func (w *capWriter) Write(p []byte) (int, error) {
	w.total += int64(len(p))
	if room := w.max - len(w.buf); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf = append(w.buf, p[:room]...)
	}
	return len(p), nil
}

// teeReadCloser reads through r (typically an io.TeeReader) while closing c (the
// original body), so a body can be captured as it streams without buffering.
type teeReadCloser struct {
	r io.Reader
	c io.Closer
}

func (t *teeReadCloser) Read(p []byte) (int, error) { return t.r.Read(p) }
func (t *teeReadCloser) Close() error               { return t.c.Close() }

func writeHTTPError(w io.Writer, status int, msg string) {
	_, _ = fmt.Fprintf(w, "HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		status, http.StatusText(status), len(msg)+1, msg+"\n")
}

func isUpgradeReq(req *http.Request) bool {
	return strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade")
}

func hdrMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k, v := range h {
		m[k] = strings.Join(v, ", ")
	}
	return m
}

func normalizeLocalAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "localhost:80"
	}
	// Strip a scheme if the user passed one.
	if i := strings.Index(addr, "://"); i >= 0 {
		addr = addr[i+3:]
	}
	if !strings.Contains(addr, ":") {
		return "localhost:" + addr
	}
	return addr
}
