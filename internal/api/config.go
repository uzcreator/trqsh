package api

import (
	"fmt"
	"os"
	"strings"
)

// Dev-only sentinel secrets. In production these MUST be replaced; LoadConfig
// refuses to start if they leak into a production deployment (see validate).
const (
	devJWTSecret     = "dev-insecure-secret-change-me"
	devInternalToken = "dev-internal-token"
	minJWTSecretLen  = 32
)

// Config is the control-plane API configuration (env-sourced).
type Config struct {
	// Env selects the safety profile: "development" (default, permissive) or
	// "production" (TRQSH_ENV=production — strict secret + auth validation).
	Env string // TRQSH_ENV

	Addr        string // TRQSH_API_ADDR (":8080")
	DatabaseURL string // TRQSH_DATABASE_URL; empty => in-memory store (dev only)
	RedisURL    string // TRQSH_REDIS_URL (for live tunnels list; optional)
	BaseDomain  string // TRQSH_BASE_DOMAIN
	PublicURL   string // TRQSH_API_PUBLIC_URL (for OAuth redirects), e.g. https://api.example.com

	JWTSecret     string // TRQSH_JWT_SECRET
	InternalToken string // TRQSH_INTERNAL_TOKEN (edge <-> api)

	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string

	// DevAuth enables the password-less /v1/auth/signup + /v1/auth/login used in
	// local dev and tests. Forced off (and rejected) in production.
	DevAuth bool

	// TrustProxy trusts X-Forwarded-For for client-IP extraction (rate limiting).
	// Only enable behind a trusted reverse proxy / load balancer.
	TrustProxy bool // TRQSH_TRUST_PROXY
}

// IsProduction reports whether strict production validation applies.
func (c Config) IsProduction() bool { return strings.EqualFold(c.Env, "production") }

// DefaultConfig returns dev-friendly defaults. These are safe for local dev only;
// production is gated by validate().
func DefaultConfig() Config {
	return Config{
		Env:           "development",
		Addr:          ":8080",
		BaseDomain:    "lvh.me",
		PublicURL:     "http://localhost:8080",
		JWTSecret:     devJWTSecret,
		InternalToken: devInternalToken,
		DevAuth:       true,
	}
}

// LoadConfig overlays environment variables on the defaults and validates the
// result. In production it fails closed on any insecure default.
func LoadConfig() (Config, error) {
	c := DefaultConfig()
	env(&c.Env, "TRQSH_ENV")
	env(&c.Addr, "TRQSH_API_ADDR")
	env(&c.DatabaseURL, "TRQSH_DATABASE_URL")
	env(&c.RedisURL, "TRQSH_REDIS_URL")
	env(&c.BaseDomain, "TRQSH_BASE_DOMAIN")
	env(&c.PublicURL, "TRQSH_API_PUBLIC_URL")
	env(&c.JWTSecret, "TRQSH_JWT_SECRET")
	env(&c.InternalToken, "TRQSH_INTERNAL_TOKEN")
	env(&c.GitHubClientID, "TRQSH_GITHUB_CLIENT_ID")
	env(&c.GitHubClientSecret, "TRQSH_GITHUB_CLIENT_SECRET")
	env(&c.GoogleClientID, "TRQSH_GOOGLE_CLIENT_ID")
	env(&c.GoogleClientSecret, "TRQSH_GOOGLE_CLIENT_SECRET")

	// DevAuth defaults on in dev, off in production, unless explicitly set.
	if v, ok := os.LookupEnv("TRQSH_DEV_AUTH"); ok {
		c.DevAuth = truthy(v)
	} else if c.IsProduction() {
		c.DevAuth = false
	}
	if v, ok := os.LookupEnv("TRQSH_TRUST_PROXY"); ok {
		c.TrustProxy = truthy(v)
	}

	return c, c.validate()
}

// validate enforces the security invariants. Development is permissive; a
// production deployment must not run on any dev default or weak secret.
func (c Config) validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("api: TRQSH_JWT_SECRET must be set")
	}
	if !c.IsProduction() {
		return nil
	}

	var problems []string
	if c.JWTSecret == devJWTSecret {
		problems = append(problems, "TRQSH_JWT_SECRET is the insecure dev default — set a strong random value")
	}
	if len(c.JWTSecret) < minJWTSecretLen {
		problems = append(problems, fmt.Sprintf("TRQSH_JWT_SECRET must be at least %d characters", minJWTSecretLen))
	}
	if c.InternalToken == "" || c.InternalToken == devInternalToken {
		problems = append(problems, "TRQSH_INTERNAL_TOKEN must be set to a strong non-default value")
	}
	if c.DevAuth {
		problems = append(problems, "TRQSH_DEV_AUTH must be disabled in production (password-less auth)")
	}
	if c.DatabaseURL == "" {
		problems = append(problems, "TRQSH_DATABASE_URL must be set — the in-memory store is dev-only")
	}
	if !strings.HasPrefix(c.PublicURL, "https://") {
		problems = append(problems, "TRQSH_API_PUBLIC_URL must use https:// in production")
	}
	if len(problems) > 0 {
		return fmt.Errorf("api: insecure production configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func env(dst *string, key string) {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		*dst = strings.TrimSpace(v)
	}
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
