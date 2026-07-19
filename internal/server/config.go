package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the edge server configuration, sourced from environment variables
// (see plan/02-edge-server.md §T1). cmd/riftd may override fields from flags.
type Config struct {
	// Env selects the safety profile: "development" (default) | "production"
	// (RIFT_ENV=production — rejects stub entitlements and missing internal token).
	Env string // RIFT_ENV

	// Agent-facing listeners (Part 01 transport). NOTE: for the MVP these are a
	// dedicated port set, distinct from the public :443, because pkg/tunnel owns
	// its own socket. Sharing :443 via ALPN demux is a documented follow-up.
	QUICAddr string // RIFT_QUIC_ADDR, e.g. ":4443" (UDP)
	TCPAddr  string // RIFT_TCP_ADDR,  e.g. ":4443" (TCP fallback)

	// Public ingress.
	HTTPAddr  string // RIFT_HTTP_ADDR,  e.g. ":80"
	HTTPSAddr string // RIFT_HTTPS_ADDR, e.g. ":443"

	// Routing / identity.
	BaseDomain string // RIFT_BASE_DOMAIN, e.g. "lvh.me" (dev) / "rift.sh" (prod)
	Region     string // RIFT_REGION
	EdgeID     string // RIFT_EDGE_ID (defaults to hostname)

	// State.
	RedisURL string // RIFT_REDIS_URL; empty => in-memory registry (single edge)

	// Entitlements.
	EntitlementsMode string // RIFT_ENTITLEMENTS: "stub" (default) | "api"
	APIURL           string // RIFT_API_URL when mode=api
	InternalToken    string // RIFT_INTERNAL_TOKEN shared with the control API

	// TLS / ACME.
	ACMEStaging bool   // RIFT_ACME_STAGING=1 uses LE staging (or dev self-signed)
	ACMEEmail   string // RIFT_ACME_EMAIL

	// Public TCP/UDP tunnel port pool.
	PortMin int // RIFT_PORT_MIN
	PortMax int // RIFT_PORT_MAX

	// Ops.
	MetricsAddr string // RIFT_METRICS_ADDR, e.g. ":9090" (metrics + health)

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
	envStr(&c.Env, "RIFT_ENV")
	envStr(&c.QUICAddr, "RIFT_QUIC_ADDR")
	envStr(&c.TCPAddr, "RIFT_TCP_ADDR")
	envStr(&c.HTTPAddr, "RIFT_HTTP_ADDR")
	envStr(&c.HTTPSAddr, "RIFT_HTTPS_ADDR")
	envStr(&c.BaseDomain, "RIFT_BASE_DOMAIN")
	envStr(&c.Region, "RIFT_REGION")
	envStr(&c.EdgeID, "RIFT_EDGE_ID")
	envStr(&c.RedisURL, "RIFT_REDIS_URL")
	envStr(&c.EntitlementsMode, "RIFT_ENTITLEMENTS")
	envStr(&c.APIURL, "RIFT_API_URL")
	envStr(&c.InternalToken, "RIFT_INTERNAL_TOKEN")
	envStr(&c.ACMEEmail, "RIFT_ACME_EMAIL")
	envStr(&c.MetricsAddr, "RIFT_METRICS_ADDR")
	c.ACMEStaging = envBool("RIFT_ACME_STAGING", c.ACMEStaging)
	envInt(&c.PortMin, "RIFT_PORT_MIN")
	envInt(&c.PortMax, "RIFT_PORT_MAX")

	if c.BaseDomain == "" {
		return c, fmt.Errorf("server: RIFT_BASE_DOMAIN must not be empty")
	}
	if c.EntitlementsMode == "api" && c.APIURL == "" {
		return c, fmt.Errorf("server: RIFT_ENTITLEMENTS=api requires RIFT_API_URL")
	}
	if c.PortMin > c.PortMax {
		return c, fmt.Errorf("server: RIFT_PORT_MIN(%d) > RIFT_PORT_MAX(%d)", c.PortMin, c.PortMax)
	}

	// Production fails closed on dev-only defaults that would weaken the edge.
	if strings.EqualFold(c.Env, "production") {
		var problems []string
		if c.EntitlementsMode != "api" {
			problems = append(problems, "RIFT_ENTITLEMENTS must be 'api' in production — 'stub' allows every bind unauthenticated")
		}
		if c.InternalToken == "" || c.InternalToken == "dev-internal-token" {
			problems = append(problems, "RIFT_INTERNAL_TOKEN must be set to a strong value matching the control API")
		}
		if c.ACMEEmail == "" {
			problems = append(problems, "RIFT_ACME_EMAIL must be set for automatic TLS certificate issuance")
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
