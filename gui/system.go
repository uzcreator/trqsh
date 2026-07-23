package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
	"github.com/trqsh-uz/trqsh/pkg/proto"
)

// safeExternalURL validates a link before it is handed to the OS launcher.
// The GUI only ever opens web links (tunnel URLs, dashboard/docs deep links),
// so we hard-restrict the scheme to http/https. This closes the injection
// surface where a crafted string (e.g. "file://…", "javascript:…", or a value
// beginning with "-") could make the platform launcher run an unintended
// handler or be parsed as a flag.
func safeExternalURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "-") {
		return "", fmt.Errorf("%s: refusing to open %q", proto.CodeInternal, raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%s: parse url: %w", proto.CodeInternal, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return "", fmt.Errorf("%s: url has no host: %q", proto.CodeInternal, raw)
		}
		return u.String(), nil
	default:
		return "", fmt.Errorf("%s: unsupported url scheme %q", proto.CodeInternal, u.Scheme)
	}
}

// openBrowser opens a validated http/https url in the user's default browser.
// Using the OS launcher keeps this independent of any specific Wails runtime API.
func openBrowser(raw string) error {
	safe, err := safeExternalURL(raw)
	if err != nil {
		return err
	}
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		// "--" stops rundll from treating a "-"-leading value as an option;
		// safeExternalURL already rejects those, this is defense in depth.
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", safe}
	case "darwin":
		name, args = "open", []string{safe}
	default:
		name, args = "xdg-open", []string{safe}
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
