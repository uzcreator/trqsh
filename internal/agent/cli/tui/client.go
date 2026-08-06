package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/trqsh-uz/trqsh/internal/agent"
)

// client is the TUI's view of a running trqsh daemon: a thin wrapper over the
// loopback control API (internal/agent/localapi.go). It lives here — rather than
// reusing package cli's control helpers — so the tui package stays independent of
// cli, letting cli import tui to launch it without an import cycle.
type client struct {
	addr  string
	token string
	hc    *http.Client
}

func newClient(addr, token string) *client {
	return &client{addr: addr, token: token, hc: &http.Client{Timeout: 10 * time.Second}}
}

func (c *client) url(path string) string { return "http://" + c.addr + path }

// do issues a JSON control-API request, attaching the loopback token, and
// decodes a 2xx body into out (when non-nil).
func (c *client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("control API %s %s: status %d", method, path, resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *client) status(ctx context.Context) (agent.Status, error) {
	var s agent.Status
	err := c.do(ctx, http.MethodGet, "/status", nil, &s)
	return s, err
}

func (c *client) tunnels(ctx context.Context) ([]agent.Tunnel, error) {
	var t []agent.Tunnel
	err := c.do(ctx, http.MethodGet, "/tunnels", nil, &t)
	return t, err
}

func (c *client) startTunnel(ctx context.Context, spec agent.TunnelSpec) (agent.Tunnel, error) {
	var t agent.Tunnel
	err := c.do(ctx, http.MethodPost, "/tunnels", spec, &t)
	return t, err
}

func (c *client) stopTunnel(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/tunnels/"+id, nil, nil)
}

func (c *client) login(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodPost, "/login", map[string]string{"token": key}, nil)
}

func (c *client) logout(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/logout", nil, nil)
}

// shutdown stops the background daemon (POST /shutdown).
func (c *client) shutdown(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/shutdown", nil, nil)
}

// --- cloud resources, proxied through the daemon (/cloud/*) ---

type subdomain struct {
	ID        string `json:"id"`
	Subdomain string `json:"subdomain"`
}

func (c *client) listSubdomains(ctx context.Context) ([]subdomain, error) {
	var out []subdomain
	err := c.do(ctx, http.MethodGet, "/cloud/subdomains", nil, &out)
	return out, err
}

func (c *client) reserveSubdomain(ctx context.Context, name string) (*subdomain, error) {
	var out subdomain
	err := c.do(ctx, http.MethodPost, "/cloud/subdomains", map[string]string{"subdomain": name}, &out)
	return &out, err
}

func (c *client) releaseSubdomain(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/cloud/subdomains/"+id, nil, nil)
}

type domain struct {
	ID         string    `json:"id"`
	Domain     string    `json:"domain"`
	VerifiedAt *struct{} `json:"verified_at"` // presence ⇒ verified; body unused here
	CertStatus string    `json:"cert_status"`
}

func (c *client) listDomains(ctx context.Context) ([]domain, error) {
	var out []domain
	err := c.do(ctx, http.MethodGet, "/cloud/domains", nil, &out)
	return out, err
}

// addDomainResp mirrors the create-domain response: the record plus the DNS
// records the user must set to verify ownership.
type addDomainResp struct {
	Domain          domain            `json:"domain"`
	DNSInstructions map[string]string `json:"dns_instructions"`
}

func (c *client) addDomain(ctx context.Context, name string) (*addDomainResp, error) {
	var out addDomainResp
	err := c.do(ctx, http.MethodPost, "/cloud/domains", map[string]string{"domain": name}, &out)
	return &out, err
}

func (c *client) verifyDomain(ctx context.Context, id string) (*domain, error) {
	var out domain
	err := c.do(ctx, http.MethodPost, "/cloud/domains/"+id+"/verify", nil, &out)
	return &out, err
}

// account is the slice of GET /cloud/me the header and /whoami care about.
type account struct {
	User struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Org struct {
		Name string `json:"name"`
		Plan string `json:"plan"`
	} `json:"org"`
	Plan  string `json:"plan"`
	Usage struct {
		BytesIn  int64 `json:"bytes_in"`
		BytesOut int64 `json:"bytes_out"`
		Requests int64 `json:"requests"`
	} `json:"usage"`
}

func (c *client) me(ctx context.Context) (*account, error) {
	var a account
	if err := c.do(ctx, http.MethodGet, "/cloud/me", nil, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// latestVersion asks the daemon (GET /update) for the newest published CLI
// release tag (no leading "v") and the installer URL for this platform.
func (c *client) latestVersion(ctx context.Context) (version, url string, err error) {
	var out struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}
	err = c.do(ctx, http.MethodGet, "/update", nil, &out)
	return out.Version, out.URL, err
}

// --- device-flow sign-in, proxied through the daemon (POST /login/oauth/*) ---

type deviceStart struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}

func (c *client) oauthStart(ctx context.Context) (*deviceStart, error) {
	var d deviceStart
	if err := c.do(ctx, http.MethodPost, "/login/oauth/start", nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// oauthPoll reports authorized=true once the browser approval completes (the
// daemon stores the minted key itself), pending=true while waiting, or an error.
func (c *client) oauthPoll(ctx context.Context, deviceCode string) (authorized, pending bool, err error) {
	var out struct {
		Authed bool   `json:"authed"`
		Error  string `json:"error"`
	}
	e := c.do(ctx, http.MethodPost, "/login/oauth/poll", map[string]string{"device_code": deviceCode}, &out)
	if e != nil {
		// The poll endpoint returns pending/expired as an {error} body, surfaced
		// here as a normal error; classify the ones worth retrying.
		switch e.Error() {
		case "authorization_pending", "slow_down":
			return false, true, nil
		}
		return false, false, e
	}
	return out.Authed, false, nil
}

// streamEvents connects to the control API's SSE feed and calls onEvent for each
// event until ctx is canceled, reconnecting with a short backoff so a transient
// daemon hiccup doesn't permanently kill the live traffic feed.
func (c *client) streamEvents(ctx context.Context, onEvent func(agent.Event)) {
	for ctx.Err() == nil {
		if err := c.streamOnce(ctx, onEvent); err != nil && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
}

func (c *client) streamOnce(ctx context.Context, onEvent func(agent.Event)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("/events"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	// No per-request timeout: the SSE stream is long-lived; ctx cancellation ends it.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("events: status %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue // SSE comments (": keepalive") and blank separators
		}
		var ev agent.Event
		if json.Unmarshal([]byte(data), &ev) == nil {
			onEvent(ev)
		}
	}
	return sc.Err()
}
