package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the edge server configuration, sourced from environment variables
// (see plan/02-edge-server.md §T1). cmd/trqshd may override fields from flags.
type Config struct {
	// Env selects the safety profile: "development" (default) | "production"
	// (TRQSH_ENV=production — rejects stub entitlements and missing internal token).
	Env string // TRQSH_ENV

	// Agent-facing listeners (Part 01 transport). NOTE: for the MVP these are a
	// dedicated port set, distinct from the public :443, because pkg/tunnel owns
	// its own socket. Sharing :443 via ALPN demux is a documented follow-up.
	QUICAddr string // TRQSH_QUIC_ADDR, e.g. ":4443" (UDP)
	TCPAddr  string // TRQSH_TCP_ADDR,  e.g. ":4443" (TCP fallback)

	// Public ingress.
	HTTPAddr  string // TRQSH_HTTP_ADDR,  e.g. ":80"
	HTTPSAddr string // TRQSH_HTTPS_ADDR, e.g. ":443"
	// Optional HTTP/3 (QUIC) public ingress. When set, the edge serves the same
	// routing over QUIC on this UDP address and advertises it via Alt-Svc, so
	// browsers can multiplex many concurrent requests on one connection without
	// HTTP/1.1 head-of-line blocking (the pain when tunneling dev servers that
	// fan out into hundreds of small requests). Empty => HTTP/3 disabled (the TCP
	// HTTP/1.1 ingress is unchanged). Set it to the same port as HTTPSAddr, e.g.
	// ":443", so the advertised h3 endpoint matches the TLS origin.
	H3Addr string // TRQSH_H3_ADDR, e.g. ":443" (UDP)
	// Public UDP port to advertise in the Alt-Svc header when it differs from the
	// port H3Addr binds. Needed behind an L4 port map / firewall redirect — e.g.
	// the distroless edge binds :8443 inside its container while Docker publishes
	// UDP 443, so browsers must be told 443, not 8443. 0 => advertise the bound
	// port (correct when the edge binds the public port directly).
	H3AdvertisePort int // TRQSH_H3_ADVERTISE_PORT, e.g. 443
	// Serve HTTP/2 (in addition to HTTP/1.1) on the TCP HTTPS ingress. h2
	// multiplexes many requests over one connection, removing the browser's ~6
	// connection-per-host HTTP/1.1 limit for clients that can't reach the UDP h3
	// port (blocked UDP, older browsers). WebSockets stay on HTTP/1.1 (h2 Extended
	// CONNECT is not advertised), so enabling this is the nginx-style h2+h1.1
	// pattern. Off by default (h2 is the primary path, not a fallback like h3);
	// validate WebSocket tunnels after enabling, and set back to 0 to revert.
	EnableH2 bool // TRQSH_ENABLE_H2

	// Routing / identity.
	BaseDomain string // TRQSH_BASE_DOMAIN, e.g. "lvh.me" (dev) / "trqsh.uz" (prod)
	Region     string // TRQSH_REGION
	EdgeID     string // TRQSH_EDGE_ID (defaults to hostname)

	// State. When set, also selects the shared TLS cert storage (RedisStorage) so
	// all edges share one cert cache + issuance lock instead of each hitting Let's
	// Encrypt independently (see tls_acme.go buildCertStorage).
	RedisURL string // TRQSH_REDIS_URL; empty => in-memory registry + on-disk cert storage (single edge)

	// Entitlements.
	EntitlementsMode string // TRQSH_ENTITLEMENTS: "stub" (default) | "api"
	APIURL           string // TRQSH_API_URL when mode=api
	InternalToken    string // TRQSH_INTERNAL_TOKEN shared with the control API

	// TLS / ACME.
	ACMEStaging bool   // TRQSH_ACME_STAGING=1 uses LE staging (or dev self-signed)
	ACMEEmail   string // TRQSH_ACME_EMAIL; when set, the edge issues real ACME certs
	// DNS-01 provider token for the wildcard cert (*.<base>). With Cloudflare set,
	// one cert covers every subdomain; without it, subdomains use on-demand
	// TLS-ALPN issuance and only custom domains strictly need per-name certs.
	CloudflareToken string // TRQSH_CLOUDFLARE_API_TOKEN (Zone.DNS:Edit)
	// Where issued certs/keys are cached so restarts don't re-issue. Empty => the
	// CertMagic default location.
	TLSStorageDir string // TRQSH_TLS_STORAGE_DIR

	// Public TCP/UDP tunnel port pool.
	PortMin int // TRQSH_PORT_MIN
	PortMax int // TRQSH_PORT_MAX

	// Ops.
	MetricsAddr string // TRQSH_METRICS_ADDR, e.g. ":9090" (metrics + health)

	// Inter-edge forwarding (multi-edge). When ForwardAddr is set AND InternalToken
	// is configured, the edge joins the forwarding mesh: it runs an internal,
	// token-authenticated forwarding listener and, when public traffic for a tunnel
	// homed on ANOTHER edge lands here, hands the connection off to the owning edge
	// instead of 404ing. Empty ForwardAddr => single-edge behavior (unchanged).
	// This is an INTERNAL-only port (analogous to MetricsAddr) — never expose it
	// publicly; peers authenticate with the shared InternalToken.
	ForwardAddr string // TRQSH_FORWARD_ADDR, e.g. ":4444" (internal bind address)
	// ForwardAdvertiseAddr is the address peers dial to reach THIS edge's forwarding
	// listener (e.g. the droplet's private IP + port). Empty => the listener's
	// resolved Addr() is advertised, which is correct in tests and when ForwardAddr
	// is a concrete host:port, but NOT when it binds a wildcard like ":4444" — a
	// multi-edge production deployment MUST set this to a routable address.
	ForwardAdvertiseAddr string // TRQSH_FORWARD_ADVERTISE_ADDR

	// Reserved control-plane hosts. When set, requests to these hostnames that
	// match no tunnel are reverse-proxied to the internal service instead of the
	// branded 404: apex (<base>) + www → SiteUpstream, app.<base> → AppUpstream,
	// api.<base> → APIUpstream.
	SiteUpstream string // TRQSH_SITE_UPSTREAM, e.g. "http://site:3000"
	AppUpstream  string // TRQSH_APP_UPSTREAM, e.g. "http://dashboard:3000"
	APIUpstream  string // TRQSH_API_UPSTREAM, e.g. "http://api:8080"

	// Tuning.
	HeartbeatInterval time.Duration
	SessionIdle       time.Duration
	DrainTimeout      time.Duration
}

// DefaultConfig returns the config with dev-friendly defaults applied.
func DefaultConfig() Config {
	host, _ := os.Hostname()
	if host == "" {
		host = "edge"
	}
	return Config{
		Env:               "development",
		QUICAddr:          ":4443",
		TCPAddr:           ":4443",
		HTTPAddr:          ":80",
		HTTPSAddr:         ":443",
		BaseDomain:        "lvh.me",
		Region:            "local",
		EdgeID:            host,
		EntitlementsMode:  "stub",
		PortMin:           20000,
		PortMax:           20099,
		MetricsAddr:       ":9090",
		HeartbeatInterval: 20 * time.Second,
		SessionIdle:       60 * time.Second,
		DrainTimeout:      25 * time.Second,
	}
}

// LoadConfig builds a Config from DefaultConfig overlaid with environment vars.
func LoadConfig() (Config, error) {
	c := DefaultConfig()
	envStr(&c.Env, "TRQSH_ENV")
	envStr(&c.QUICAddr, "TRQSH_QUIC_ADDR")
	envStr(&c.TCPAddr, "TRQSH_TCP_ADDR")
	envStr(&c.HTTPAddr, "TRQSH_HTTP_ADDR")
	envStr(&c.HTTPSAddr, "TRQSH_HTTPS_ADDR")
	envStr(&c.H3Addr, "TRQSH_H3_ADDR")
	envInt(&c.H3AdvertisePort, "TRQSH_H3_ADVERTISE_PORT")
	c.EnableH2 = envBool("TRQSH_ENABLE_H2", c.EnableH2)
	envStr(&c.BaseDomain, "TRQSH_BASE_DOMAIN")
	envStr(&c.Region, "TRQSH_REGION")
	envStr(&c.EdgeID, "TRQSH_EDGE_ID")
	envStr(&c.RedisURL, "TRQSH_REDIS_URL")
	envStr(&c.EntitlementsMode, "TRQSH_ENTITLEMENTS")
	envStr(&c.APIURL, "TRQSH_API_URL")
	envStr(&c.InternalToken, "TRQSH_INTERNAL_TOKEN")
	envStr(&c.ACMEEmail, "TRQSH_ACME_EMAIL")
	envStr(&c.CloudflareToken, "TRQSH_CLOUDFLARE_API_TOKEN")
	envStr(&c.TLSStorageDir, "TRQSH_TLS_STORAGE_DIR")
	envStr(&c.MetricsAddr, "TRQSH_METRICS_ADDR")
	envStr(&c.ForwardAddr, "TRQSH_FORWARD_ADDR")
	envStr(&c.ForwardAdvertiseAddr, "TRQSH_FORWARD_ADVERTISE_ADDR")
	envStr(&c.SiteUpstream, "TRQSH_SITE_UPSTREAM")
	envStr(&c.AppUpstream, "TRQSH_APP_UPSTREAM")
	envStr(&c.APIUpstream, "TRQSH_API_UPSTREAM")
	c.ACMEStaging = envBool("TRQSH_ACME_STAGING", c.ACMEStaging)
	envInt(&c.PortMin, "TRQSH_PORT_MIN")
	envInt(&c.PortMax, "TRQSH_PORT_MAX")

	if c.BaseDomain == "" {
		return c, fmt.Errorf("server: TRQSH_BASE_DOMAIN must not be empty")
	}
	if c.EntitlementsMode == "api" && c.APIURL == "" {
		return c, fmt.Errorf("server: TRQSH_ENTITLEMENTS=api requires TRQSH_API_URL")
	}
	if c.PortMin > c.PortMax {
		return c, fmt.Errorf("server: TRQSH_PORT_MIN(%d) > TRQSH_PORT_MAX(%d)", c.PortMin, c.PortMax)
	}

	// Production fails closed on dev-only defaults that would weaken the edge.
	if strings.EqualFold(c.Env, "production") {
		var problems []string
		if c.EntitlementsMode != "api" {
			problems = append(problems, "TRQSH_ENTITLEMENTS must be 'api' in production — 'stub' allows every bind unauthenticated")
		}
		if c.InternalToken == "" || c.InternalToken == "dev-internal-token" {
			problems = append(problems, "TRQSH_INTERNAL_TOKEN must be set to a strong value matching the control API")
		}
		if c.ACMEEmail == "" {
			problems = append(problems, "TRQSH_ACME_EMAIL must be set for automatic TLS certificate issuance")
		}
		if len(problems) > 0 {
			return c, fmt.Errorf("server: insecure production configuration:\n  - %s", strings.Join(problems, "\n  - "))
		}
	}
	return c, nil
}

func envStr(dst *string, key string) {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		*dst = strings.TrimSpace(v)
	}
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off", "":
		return false
	default:
		return def
	}
}

func envInt(dst *int, key string) {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			*dst = n
		}
	}
}
