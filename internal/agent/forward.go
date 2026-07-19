package agent

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rift/rift/internal/agent/inspect"
	"github.com/rift/rift/pkg/proto"
	"github.com/rift/rift/pkg/tunnel"
)

const localDialTimeout = 10 * time.Second

// handleDataStream services one edge-initiated data stream: read its StreamInit,
// find the tunnel, and forward to the local service (teeing HTTP to the inspector).
func (a *Agent) handleDataStream(st tunnel.Stream) {
	defer st.Close()
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
	fromPub, toPub := joinStreams(local, st)
	at.metrics.bytesIn.Add(fromPub)
	at.metrics.bytesOut.Add(toPub)
}

func (a *Agent) forwardHTTP(st tunnel.Stream, at *activeTunnel, si *proto.StreamInit) {
	req, err := http.ReadRequest(bufio.NewReader(st))
	if err != nil {
		return
	}
	start := time.Now()

	local, err := net.DialTimeout("tcp", normalizeLocalAddr(at.spec.Addr), localDialTimeout)
	if err != nil {
		writeHTTPError(st, http.StatusBadGateway, "local service unreachable")
		a.emit(Event{Type: "error", Err: "dial local " + at.spec.Addr + ": " + err.Error()})
		return
	}
	defer local.Close()

	if isUpgradeReq(req) {
		if err := req.Write(local); err != nil {
			return
		}
		fromPub, toPub := joinStreams(local, st)
		at.metrics.bytesIn.Add(fromPub)
		at.metrics.bytesOut.Add(toPub)
		return
	}

	reqBody, reqRest := capAndRest(req.Body)
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(reqBody), reqRest))
	reqHeaders := hdrMap(req.Header)
	reqPath := req.URL.RequestURI()
	reqMethod, reqHost := req.Method, req.Host

	if err := req.Write(local); err != nil {
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(local), req)
	if err != nil {
		writeHTTPError(st, http.StatusBadGateway, "bad local response")
		return
	}
	respBody, respRest := capAndRest(resp.Body)
	resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(respBody), respRest))
	status := resp.StatusCode
	respHeaders := hdrMap(resp.Header)
	if err := resp.Write(st); err != nil {
		resp.Body.Close()
		return
	}
	resp.Body.Close()

	at.metrics.requests.Add(1)
	at.metrics.bytesIn.Add(int64(len(reqBody)))
	at.metrics.bytesOut.Add(int64(len(respBody)))

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
		ReqBody:     reqBody,
		RespBody:    respBody,
		LocalAddr:   normalizeLocalAddr(at.spec.Addr),
	})
	a.emit(Event{Type: "request", Request: &captured})
}

// forwardUDP translates the uint16-length-framed datagram stream (matching the
// edge's ingress_udp framing) to and from a local UDP socket.
func (a *Agent) forwardUDP(st tunnel.Stream, at *activeTunnel) {
	local, err := net.Dial("udp", normalizeLocalAddr(at.spec.Addr))
	if err != nil {
		a.emit(Event{Type: "error", Err: "dial local udp " + at.spec.Addr + ": " + err.Error()})
		return
	}
	defer local.Close()

	// stream -> local
	go func() {
		var hdr [2]byte
		for {
			if _, err := io.ReadFull(st, hdr[:]); err != nil {
				local.Close()
				return
			}
			n := binary.BigEndian.Uint16(hdr[:])
			payload := make([]byte, n)
			if _, err := io.ReadFull(st, payload); err != nil {
				local.Close()
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
		binary.BigEndian.PutUint16(hdr[:], uint16(n))
		if _, err := st.Write(hdr[:]); err != nil {
			return
		}
		if _, err := st.Write(buf[:n]); err != nil {
			return
		}
		at.metrics.bytesOut.Add(int64(n))
	}
}

// joinStreams welds a local conn to an edge stream, returning bytes from the
// public side (into local) and to the public side (out of local).
func joinStreams(local net.Conn, st tunnel.Stream) (fromPublic, toPublic int64) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		fromPublic, _ = io.Copy(local, st)
		local.Close()
		st.Close()
	}()
	go func() {
		defer wg.Done()
		toPublic, _ = io.Copy(st, local)
		st.Close()
		local.Close()
	}()
	wg.Wait()
	return fromPublic, toPublic
}

// capAndRest reads up to MaxBodyCapture bytes for the inspector and returns a
// reader for whatever remains, so the full body can still be forwarded.
func capAndRest(r io.Reader) ([]byte, io.Reader) {
	if r == nil {
		return nil, bytes.NewReader(nil)
	}
	buf := make([]byte, inspect.MaxBodyCapture)
	n, _ := io.ReadFull(r, buf)
	return buf[:n], r
}

func writeHTTPError(w io.Writer, status int, msg string) {
	fmt.Fprintf(w, "HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
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
