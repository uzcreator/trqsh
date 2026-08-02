package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/trqsh-uz/trqsh/internal/agent"
)

// errNotSignedIn is returned by cloud calls when no API key is stored.
var errNotSignedIn = errors.New("not signed in — run `trqsh login`")

// cloudClient talks to the trqsh control-plane API directly from the CLI, using
// the stored API key as the bearer credential. It backs the account/subdomain/
// domain commands and the browser (device-flow) sign-in. The desktop app reaches
// the same API through the agent's local proxy (see cloudapi.go); the CLI, being
// a first-party trusted process, calls it straight.
type cloudClient struct {
	base   string
	apiKey string
	hc     *http.Client
}

// newCloudClient builds a client from the loaded agent config.
func newCloudClient(cfg agent.Config) *cloudClient {
	base := strings.TrimRight(firstNonEmpty(cfg.APIBase, agent.DefaultAPIBase), "/")
	return &cloudClient{
		base:   base,
		apiKey: strings.TrimSpace(cfg.APIKey),
		hc:     &http.Client{Timeout: 20 * time.Second},
	}
}

// do issues a JSON request and decodes a 2xx body into out. When auth is set, the
// stored API key is required and sent as the bearer token.
func (c *cloudClient) do(ctx context.Context, method, path string, in, out any, auth bool) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		if c.apiKey == "" {
			return errNotSignedIn
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("cloud unreachable: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		if resp.StatusCode == http.StatusUnauthorized {
			return errNotSignedIn
		}
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("api error: status %d", resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// --- device-authorization (browser sign-in) ---

type deviceCodeResp struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}

// startDevice begins a device-authorization flow and returns the code pair plus
// the browser verification URL.
func (c *cloudClient) startDevice(ctx context.Context) (*deviceCodeResp, error) {
	var out deviceCodeResp
	if err := c.do(ctx, http.MethodPost, "/v1/auth/device/code", nil, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// pollDevice checks whether the browser approval completed. It returns the minted
// API key on success, pending=true while the user hasn't approved yet, or a
// terminal error (expired/invalid) that should stop polling. Unlike do(), it
// reads the body on the 400 the token endpoint uses for pending/expired states.
func (c *cloudClient) pollDevice(ctx context.Context, deviceCode string) (apiKey string, pending bool, err error) {
	b, _ := json.Marshal(map[string]string{"device_code": deviceCode})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/auth/device/token", bytes.NewReader(b))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("cloud unreachable: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		APIKey string `json:"api_key"`
		Error  string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out)
	if out.APIKey != "" {
		return out.APIKey, false, nil
	}
	switch out.Error {
	case "authorization_pending", "slow_down", "":
		return "", true, nil
	case "expired_token":
		return "", false, errors.New("this sign-in request expired — run `trqsh login` again")
	case "invalid_device_code":
		return "", false, errors.New("invalid sign-in session — run `trqsh login` again")
	default:
		return "", false, errors.New(out.Error)
	}
}

// --- account / resources ---

type meResp struct {
	User struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Org struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Plan string `json:"plan"`
	} `json:"org"`
	Plan          string     `json:"plan"`
	PlanExpiresAt *time.Time `json:"plan_expires_at"`
	ViaAPIKey     bool       `json:"via_api_key"`
	Usage         struct {
		BytesIn  int64 `json:"bytes_in"`
		BytesOut int64 `json:"bytes_out"`
		Requests int64 `json:"requests"`
	} `json:"usage"`
	Limits struct {
		BandwidthBytesMo      int64 `json:"bandwidth_bytes_mo"`
		RequestsMo            int64 `json:"requests_mo"`
		MaxConcurrentTunnels  int   `json:"max_concurrent_tunnels"`
		MaxReservedSubdomains int   `json:"max_reserved_subdomains"`
		MaxCustomDomains      int   `json:"max_custom_domains"`
	} `json:"limits"`
}

func (c *cloudClient) me(ctx context.Context) (*meResp, error) {
	var out meResp
	if err := c.do(ctx, http.MethodGet, "/v1/me", nil, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

type subdomain struct {
	ID        string    `json:"id"`
	Subdomain string    `json:"subdomain"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *cloudClient) listSubdomains(ctx context.Context) ([]subdomain, error) {
	var out []subdomain
	err := c.do(ctx, http.MethodGet, "/v1/subdomains", nil, &out, true)
	return out, err
}

func (c *cloudClient) reserveSubdomain(ctx context.Context, name string) (*subdomain, error) {
	var out subdomain
	err := c.do(ctx, http.MethodPost, "/v1/subdomains", map[string]string{"subdomain": name}, &out, true)
	return &out, err
}

func (c *cloudClient) releaseSubdomain(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/subdomains/"+id, nil, nil, true)
}

type domain struct {
	ID          string     `json:"id"`
	Domain      string     `json:"domain"`
	VerifyToken string     `json:"verify_token"`
	VerifiedAt  *time.Time `json:"verified_at"`
	CertStatus  string     `json:"cert_status"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (c *cloudClient) listDomains(ctx context.Context) ([]domain, error) {
	var out []domain
	err := c.do(ctx, http.MethodGet, "/v1/domains", nil, &out, true)
	return out, err
}

// addDomainResp mirrors the API's create-domain response: the stored record plus
// the DNS records the user must add to verify ownership.
type addDomainResp struct {
	Domain          domain            `json:"domain"`
	DNSInstructions map[string]string `json:"dns_instructions"`
}

func (c *cloudClient) addDomain(ctx context.Context, name string) (*addDomainResp, error) {
	var out addDomainResp
	err := c.do(ctx, http.MethodPost, "/v1/domains", map[string]string{"domain": name}, &out, true)
	return &out, err
}

func (c *cloudClient) verifyDomain(ctx context.Context, id string) (*domain, error) {
	var out domain
	err := c.do(ctx, http.MethodPost, "/v1/domains/"+id+"/verify", nil, &out, true)
	return &out, err
}

// --- shared display helpers ---

// newTab returns a tab writer that aligns the CLI's list tables.
func newTab() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

// baseDomainOf derives the public base domain (e.g. "trqsh.uz") from the edge
// server address, for rendering a reserved subdomain as a full host.
func baseDomainOf(cfg agent.Config) string {
	h := strings.TrimSpace(cfg.Server)
	if i := strings.LastIndex(h, ":"); i > 0 {
		h = h[:i]
	}
	if h == "" {
		return "trqsh.uz"
	}
	return h
}

// humanBytes renders a byte count as a compact human-readable size.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// humanInt renders an integer with thousands separators.
func humanInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
