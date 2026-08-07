package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/trqsh-uz/trqsh/internal/api/auth"
	"github.com/trqsh-uz/trqsh/internal/api/store"
	"github.com/trqsh-uz/trqsh/internal/billing"
	"github.com/trqsh-uz/trqsh/internal/billing/stripe"
	"github.com/trqsh-uz/trqsh/internal/geo"
)

// Server is the control-plane API.
type Server struct {
	cfg     Config
	log     *slog.Logger
	store   store.Store
	auth    *auth.Auth
	ent     *Entitlements
	billing *billing.Service
	geo     *geo.Service
	metrics *metrics

	authLimiter *rateLimiter
	apiLimiter  *rateLimiter

	providers map[string]auth.OAuthProvider

	oauthState oauthStateStore
	remote     *remoteHub
}

// New builds the API server, choosing Postgres or the in-memory store from config.
func New(cfg Config, log *slog.Logger) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	var st store.Store
	if cfg.DatabaseURL != "" {
		ps, err := store.NewPostgresStore(cfg.DatabaseURL, store.PoolConfig{
			MaxOpenConns: cfg.DBMaxOpenConns,
			MaxIdleConns: cfg.DBMaxIdleConns,
		})
		if err != nil {
			return nil, err
		}
		st = ps
	} else {
		st = store.NewMemStore()
	}

	a := auth.New(st, cfg.JWTSecret)
	s := &Server{
		cfg:       cfg,
		log:       log,
		store:     st,
		auth:      a,
		ent:       NewEntitlements(a, st, cfg.BaseDomain),
		metrics:   newMetrics(),
		providers: map[string]auth.OAuthProvider{},
	}

	// Billing (Part 07): always constructed so metered-quota enforcement runs
	// even when Stripe is disabled (fail-safe — never widens limits). Checkout/
	// Portal/webhooks are inert until Stripe keys are configured.
	bcfg := billing.LoadConfig()
	var sapi stripe.API
	if bcfg.Enabled() {
		sapi = stripe.New(bcfg.SecretKey)
	}
	s.billing = billing.New(st, sapi, bcfg, log)
	s.ent.SetQuota(s.billing)

	// Geo/location service: powers /v1/geo region routing and enriches tunnel
	// history + the admin dashboard with country data. A bad TRQSH_REGIONS value is
	// non-fatal — we log and fall back to the built-in PoP catalog.
	regions, err := geo.ParseRegions(cfg.Regions)
	if err != nil {
		log.Warn("invalid TRQSH_REGIONS, using built-in catalog", "err", err)
		regions = nil
	}
	s.geo = geo.New(geo.Config{Header: cfg.GeoIPHeader, APIURL: cfg.GeoIPAPI, Regions: regions})

	// /qr remote-control pairing relay (see remote.go) — in-process, no Redis
	// dependency today.
	s.remote = newRemoteHub()

	if cfg.GitHubClientID != "" {
		s.providers["github"] = auth.GitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret,
			cfg.PublicURL+"/v1/auth/oauth/github/callback")
	}
	if cfg.GoogleClientID != "" {
		s.providers["google"] = auth.GoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret,
			cfg.PublicURL+"/v1/auth/oauth/google/callback")
	}

	// Shared Redis client (optional): when TRQSH_REDIS_URL is set, the rate limiters
	// enforce one bucket per client across ALL API replicas rather than each replica
	// granting the full rate independently (N replicas => N× the intended limit). A
	// single client is shared by both limiters instead of opening two connections.
	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("api: parse TRQSH_REDIS_URL: %w", err)
		}
		rdb = redis.NewClient(opt)
	}

	// Abuse guards: a strict per-IP limit on auth endpoints (brute-force /
	// account-spam) and a broad flood limit on the rest of the public API.
	// The internal edge RPC is exempt (high-volume entitlement checks).
	s.authLimiter = newRateLimiter(rdb, "auth", 5, 10, cfg.TrustProxy, log)
	s.apiLimiter = newRateLimiter(rdb, "api", 50, 100, cfg.TrustProxy, log)

	// OAuth CSRF state and the CLI/GUI device-authorization flow both need to
	// survive a request landing on a different API replica than the one that
	// started the flow (browser callback vs. the replica that issued the state;
	// CLI poll vs. the replica the approval landed on) — same shared-Redis seam
	// as the rate limiters above, same in-process fallback for a single replica.
	if rdb != nil {
		s.oauthState = newRedisStateStore(rdb, log)
		a.SetDevices(newRedisDeviceStore(rdb, log))
	} else {
		s.oauthState = newMemStateStore()
	}
	return s, nil
}

// Entitlements exposes the entitlements service (for in-process edge tests).
func (s *Server) Entitlements() *Entitlements { return s.ent }

// Store exposes the backing store (for tests/seeding).
func (s *Server) Store() store.Store { return s.store }

// Billing exposes the billing service (for tests / scheduled jobs).
func (s *Server) Billing() *billing.Service { return s.billing }

// Router builds the HTTP handler.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.metrics.middleware)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.securityHeaders)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	// The API is not a browsable site: the root serves the admin console only on
	// approve.<base>; every other host (api.<base>) gets a bare 404.
	r.Get("/", s.handleRoot)

	// The raw OpenAPI spec is always public: it's plain documentation content (the
	// site's /docs/api reference page and, once split into its own repo, its
	// build-time fetch both depend on this — see web/site/scripts/gen-openapi.mjs),
	// and it's already effectively public via that rendered page. The interactive
	// Swagger UI ("try it out" against prod) and the verbose live status
	// (goroutine counts, uptime, store kind) are meaningfully more sensitive, so
	// those stay dev-only. Health checks use /healthz.
	r.Get("/openapi.yaml", s.handleOpenAPISpec)
	if !s.cfg.IsProduction() {
		r.Get("/status", s.handleStatus)
		r.Get("/docs", s.handleDocsUI)
	}

	// approve.<base> admin console: the page + login are served same-origin as the
	// /v1/admin/* API (below) so the admin session cookie flows without CORS.
	r.Get("/admin", s.handleAdminPage)
	r.Post("/admin/logout", s.handleAdminLogout)
	r.Group(func(r chi.Router) {
		r.Use(s.authLimiter.middleware) // brute-force guard on the credential check
		r.Post("/admin/login", s.handleAdminLogin)
	})

	r.Route("/v1", func(r chi.Router) {
		// Per-IP flood guard on the whole public API (internal RPC is exempt).
		r.Use(s.apiLimiter.middleware)

		// Unauthenticated, display-safe plan catalog (see handlers_plans_public.go),
		// for build-time consumption by the marketing site — which can't import
		// internal/billing across a module boundary once it's a separate repo.
		r.Get("/plans/public", s.handleListPlansPublic)

		// Location routing: the edge PoP catalog and the caller's detected country +
		// recommended nearest region. Public so the CLI/desktop/agent can pick a
		// region before authenticating (see internal/geo).
		r.Get("/regions", s.handleListRegions)
		r.Get("/geo", s.handleGeo)

		// Public auth endpoints — stricter per-IP limit (brute-force / account spam).
		r.Group(func(r chi.Router) {
			r.Use(s.authLimiter.middleware)
			r.Post("/auth/signup", s.handleSignup)
			r.Post("/auth/login", s.handleLogin)
			r.Post("/auth/refresh", s.handleRefresh)
			r.Get("/auth/oauth/{provider}", s.handleOAuthStart)
			r.Get("/auth/oauth/{provider}/callback", s.handleOAuthCallback)
			r.Post("/auth/logout", s.handleLogout)
			r.Post("/auth/device/code", s.handleDeviceCode)
			r.Post("/auth/device/token", s.handleDeviceToken)
		})

		// Stripe webhooks: public but signature-verified inside the handler.
		r.Post("/billing/webhooks", s.billing.HandleWebhook)

		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(s.auth.Middleware)
			r.Get("/account", s.handleAccount)
			r.Get("/me", s.handleMe)
			r.Post("/auth/device/approve", s.handleDeviceApprove)

			r.Get("/api-keys", s.handleListAPIKeys)
			r.Post("/api-keys", s.handleCreateAPIKey)
			r.Delete("/api-keys/{id}", s.handleRevokeAPIKey)

			r.Get("/subdomains", s.handleListSubdomains)
			r.Post("/subdomains", s.handleReserveSubdomain)
			r.Delete("/subdomains/{id}", s.handleReleaseSubdomain)

			r.Get("/domains", s.handleListDomains)
			r.Post("/domains", s.handleAddDomain)
			r.Post("/domains/{id}/verify", s.handleVerifyDomain)

			r.Get("/usage", s.handleUsage)
			r.Get("/orgs", s.handleListOrgs)
			r.Get("/tunnels", s.handleListTunnels)
			r.Get("/plans", s.handleListPlans)

			// Billing (Part 07): Stripe Checkout, Customer Portal, subscription.
			r.Post("/billing/checkout", s.billing.HandleCheckout)
			r.Post("/billing/portal", s.billing.HandlePortal)
			r.Get("/billing/subscription", s.billing.HandleSubscription)

			// /qr remote-control pairing (see remote.go) — agent (daemon) side.
			r.Post("/remote/sessions", s.handleRemoteCreate)
			r.Get("/remote/sessions/{code}/agent", s.handleRemoteAgentStream)
			r.Post("/remote/sessions/{code}/publish", s.handleRemotePublish)
			r.Post("/remote/sessions/{code}/end", s.handleRemoteEnd)
		})

		// /qr remote-control pairing (see remote.go) — viewer (phone/browser) side.
		// Deliberately outside the JWT/API-key group above: the session code
		// itself, embedded in the QR/link, is the capability — requiring a second
		// login on the phone would defeat the point of scanning a code. Still
		// covered by the flood guard (apiLimiter, r.Use'd on the whole /v1 group
		// at the top of this Route call) — the code's ~58 bits of entropy is what
		// actually resists guessing.
		r.Get("/remote/sessions/{code}/viewer", s.handleRemoteViewerStream)
		r.Post("/remote/sessions/{code}/command", s.handleRemoteCommand)

		// Admin (approve.<base>): subscription grants, gated by the admin session
		// cookie (not a user JWT). handlers refuse when admin creds are unset.
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/admin/session", s.handleAdminSession)
			r.Get("/admin/lookup", s.handleAdminLookup)
			r.Post("/admin/grant", s.handleAdminGrant)
			r.Post("/admin/revoke", s.handleAdminRevoke)
		})
	})

	// Internal entitlements RPC (token-guarded).
	s.mountInternal(r)
	return r
}

// opsHandler serves the internal-only metrics + health endpoints (mounted on
// s.cfg.MetricsAddr, not the public router). Ungated like the edge's ops server:
// the network boundary (no public host port; not reverse-proxied) is the guard.
func (s *Server) opsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.metrics.Registry(), promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

// Run starts the HTTP server until ctx is canceled.
func (s *Server) Run(ctx context.Context) error {
	// Opt-in background metering: collect + push metered usage to Stripe.
	if bc := s.billing.Config(); bc.Enabled() && bc.MeteringInterval > 0 {
		go s.billing.RunMeteringLoop(ctx, bc.MeteringInterval)
		s.log.Info("billing metering loop started", "interval", bc.MeteringInterval)
	}

	// Ops server (metrics + health) on a SEPARATE internal port. /metrics must not
	// ride on the public :8080 router, which the edge reverse-proxies to
	// api.<base> — that would expose internal metrics publicly. Prometheus scrapes
	// this port directly on the container network. A bind failure here is logged,
	// not fatal: the control API stays up even if the ops port is unavailable.
	if s.cfg.MetricsAddr != "" {
		ops := &http.Server{
			Addr:              s.cfg.MetricsAddr,
			Handler:           s.opsHandler(),
			ReadHeaderTimeout: 5 * time.Second, // slowloris guard (gosec G112)
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      20 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 16,
		}
		// #nosec G118 -- ctx is already Done() by the time this fires; a fresh context.Background() timeout is required for Shutdown's own grace period (standard graceful-shutdown pattern)
		go func() {
			<-ctx.Done()
			sc, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = ops.Shutdown(sc)
		}()
		go func() {
			s.log.Info("control API ops listening", "addr", s.cfg.MetricsAddr)
			if err := ops.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.log.Warn("ops server stopped", "err", err)
			}
		}()
	}

	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Router(),
		ReadHeaderTimeout: 10 * time.Second, // slowloris guard (gosec G112)
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
	// #nosec G118 -- ctx is already Done() by the time this fires; a fresh context.Background() timeout is required for Shutdown's own grace period (standard graceful-shutdown pattern)
	go func() {
		<-ctx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sc)
	}()
	s.log.Info("control API listening", "addr", s.cfg.Addr, "store", storeKind(s.cfg))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func storeKind(c Config) string {
	if c.DatabaseURL != "" {
		return "postgres"
	}
	return "memory"
}

// --- shared JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// logEscaper escapes newlines/carriage-returns in a value before it's written
// to a log line, so user-controlled input (an email, a plan name) can't forge
// fake log entries.
var logEscaper = strings.NewReplacer("\n", "\\n", "\r", "\\r")

func sanitizeLog(s string) string { return logEscaper.Replace(s) }

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
