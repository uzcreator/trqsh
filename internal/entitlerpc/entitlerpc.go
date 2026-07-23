// Package entitlerpc is the internal RPC contract between the edge (Part 02) and
// the control plane's entitlements service (Part 05). The edge imports the
// Client (an authz.Entitlements implementation with a short-TTL auth cache); the
// API server implements the matching HTTP endpoints.
package entitlerpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/trqsh-uz/trqsh/pkg/authz"
)

// Internal auth header carrying the shared token between edge and API.
const HeaderToken = "X-trqsh-Internal-Token"

// Route paths for the internal entitlements endpoints.
const (
	PathAuthenticate = "/internal/entitlements/authenticate"
	PathCheckBind    = "/internal/entitlements/check-bind"
	PathReportUsage  = "/internal/entitlements/report-usage"
)

// AuthRequest / AuthResponse cover Entitlements.Authenticate.
type AuthRequest struct {
	APIKey string `json:"api_key"`
}
type AuthResponse struct {
	AccountID string `json:"account_id"`
	Plan      string `json:"plan"`
	Error     string `json:"error,omitempty"`
}

// CheckBindRequest / CheckBindResponse cover Entitlements.CheckBind.
type CheckBindRequest struct {
	APIKey     string `json:"api_key"`
	Type       string `json:"type"`
	Subdomain  string `json:"subdomain"`
	CustomHost string `json:"custom_host"`
	RemotePort int    `json:"remote_port"`
	Region     string `json:"region"`
}
type CheckBindResponse struct {
	Allow             bool         `json:"allow"`
	AccountID         string       `json:"account_id"`
	Plan              string       `json:"plan"`
	Limits            authz.Limits `json:"limits"`
	AssignedSubdomain string       `json:"assigned_subdomain"`
	ErrorCode         string       `json:"error_code"`
	ErrorMessage      string       `json:"error_message"`
}

// UsageRequest covers Entitlements.ReportUsage.
type UsageRequest struct {
	AccountID   string    `json:"account_id"`
	TunnelID    string    `json:"tunnel_id"`
	BytesIn     int64     `json:"bytes_in"`
	BytesOut    int64     `json:"bytes_out"`
	Requests    int64     `json:"requests"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

// Client is the edge-side authz.Entitlements implementation backed by the API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client

	cacheTTL  time.Duration
	mu        sync.Mutex
	authCache map[string]authCacheEntry
}

type authCacheEntry struct {
	accountID string
	plan      string
	expiry    time.Time
}

var _ authz.Entitlements = (*Client)(nil)

// NewClient builds an entitlements client for the given API base URL + token.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:   baseURL,
		token:     token,
		http:      &http.Client{Timeout: 5 * time.Second},
		cacheTTL:  30 * time.Second,
		authCache: make(map[string]authCacheEntry),
	}
}

// Authenticate validates an API key, caching positive results briefly.
func (c *Client) Authenticate(ctx context.Context, apiKey string) (string, string, error) {
	c.mu.Lock()
	if e, ok := c.authCache[apiKey]; ok && time.Now().Before(e.expiry) {
		c.mu.Unlock()
		return e.accountID, e.plan, nil
	}
	c.mu.Unlock()

	var resp AuthResponse
	if err := c.post(ctx, PathAuthenticate, AuthRequest{APIKey: apiKey}, &resp); err != nil {
		return "", "", err
	}
	if resp.Error != "" {
		return "", "", fmt.Errorf("entitlements: %s", resp.Error)
	}
	c.mu.Lock()
	c.authCache[apiKey] = authCacheEntry{resp.AccountID, resp.Plan, time.Now().Add(c.cacheTTL)}
	c.mu.Unlock()
	return resp.AccountID, resp.Plan, nil
}

// CheckBind authorizes a bind request (never cached).
func (c *Client) CheckBind(ctx context.Context, req authz.BindRequest) (authz.Decision, error) {
	var resp CheckBindResponse
	body := CheckBindRequest{
		APIKey: req.APIKey, Type: req.Type, Subdomain: req.Subdomain,
		CustomHost: req.CustomHost, RemotePort: req.RemotePort, Region: req.Region,
	}
	if err := c.post(ctx, PathCheckBind, body, &resp); err != nil {
		return authz.Decision{}, err
	}
	return authz.Decision{
		Allow:             resp.Allow,
		AccountID:         resp.AccountID,
		Plan:              resp.Plan,
		Limits:            resp.Limits,
		AssignedSubdomain: resp.AssignedSubdomain,
		ErrorCode:         resp.ErrorCode,
		ErrorMessage:      resp.ErrorMessage,
	}, nil
}

// ReportUsage forwards a usage window to the control plane.
func (c *Client) ReportUsage(ctx context.Context, u authz.Usage) error {
	return c.post(ctx, PathReportUsage, UsageRequest{
		AccountID: u.AccountID, TunnelID: u.TunnelID, BytesIn: u.BytesIn, BytesOut: u.BytesOut,
		Requests: u.Requests, WindowStart: u.WindowStart, WindowEnd: u.WindowEnd,
	}, nil)
}

func (c *Client) post(ctx context.Context, path string, in, out any) error {
	buf, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderToken, c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("entitlements: %s: status %d: %s", path, resp.StatusCode, b)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
