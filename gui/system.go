package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/rift/rift/internal/agent/inspect"
	"github.com/rift/rift/pkg/proto"
)

// openBrowser opens url in the user's default browser. Using the OS launcher
// keeps this independent of any specific Wails runtime API.
func openBrowser(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		name, args = "open", []string{url}
	default:
		name, args = "xdg-open", []string{url}
	}
	return exec.Command(name, args...).Start()
}

// replayLocal re-issues a captured request directly against the local target
// service (the same host:port the tunnel forwards to). This mirrors ngrok's
// "replay" — it exercises the developer's app without re-crossing the edge.
func replayLocal(req inspect.CapturedRequest) error {
	target := "http://" + req.LocalAddr + req.Path
	var body io.Reader
	if len(req.ReqBody) > 0 {
		body = bytes.NewReader(req.ReqBody)
	}
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	r, err := http.NewRequest(method, target, body)
	if err != nil {
		return fmt.Errorf("%s: build replay request: %w", proto.CodeInternal, err)
	}
	for k, v := range req.ReqHeaders {
		r.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(r)
	if err != nil {
		return fmt.Errorf("%s: replay to %s: %w", proto.CodeUpstreamUnreachable, req.LocalAddr, err)
	}
	_ = resp.Body.Close()
	return nil
}
