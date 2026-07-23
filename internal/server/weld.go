package server

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/trqsh-uz/trqsh/pkg/tunnel"
)

// rawJoin welds a public connection to an agent data stream, copying bytes in
// both directions until either side closes. clientRead is the reader for the
// public side (a *bufio.Reader when bytes were already buffered, else the conn).
// It returns bytes sent to the agent (up) and received from the agent (down).
func rawJoin(clientConn net.Conn, clientRead io.Reader, agent tunnel.Stream) (up, down int64) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		up, _ = io.Copy(agent, clientRead) // public -> agent
		agent.Close()
		clientConn.Close()
	}()
	go func() {
		defer wg.Done()
		down, _ = io.Copy(clientConn, agent) // agent -> public
		clientConn.Close()
		agent.Close()
	}()
	wg.Wait()
	return up, down
}

// branded404 is the response body for an unknown host — a small, on-brand page.
func branded404(host string) string {
	return fmt.Sprintf(`<!doctype html>
<html><head><meta charset="utf-8"><title>Tunnel offline · trqsh</title>
<style>body{font-family:ui-sans-serif,system-ui,sans-serif;background:#0b0d12;color:#e6e8ee;
display:grid;place-items:center;height:100vh;margin:0}main{max-width:32rem;text-align:center;padding:2rem}
h1{font-size:1.5rem;margin:0 0 .5rem}code{background:#181b22;padding:.15rem .4rem;border-radius:.35rem}
p{color:#9aa3b2;line-height:1.5}a{color:#7aa2ff}</style></head>
<body><main><h1>This tunnel is offline</h1>
<p>No active tunnel is serving <code>%s</code>. If this is your tunnel, make sure the
trqsh agent is running and connected.</p>
<p><a href="https://trqsh.uz">trqsh.uz</a></p></main></body></html>`, host)
}

// writeHTTPResponse writes a minimal HTTP/1.1 response with a body to a raw conn.
func writeHTTPResponse(conn net.Conn, status int, reason, contentType, body string) {
	fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\n", status, reason)
	fmt.Fprintf(conn, "Content-Type: %s\r\n", contentType)
	fmt.Fprintf(conn, "Content-Length: %d\r\n", len(body))
	fmt.Fprintf(conn, "Connection: close\r\n\r\n")
	io.WriteString(conn, body)
}
