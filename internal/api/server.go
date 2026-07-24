package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/trqsh-uz/trqsh/internal/api/auth"
	"github.com/trqsh-uz/trqsh/internal/api/store"
	"github.com/trqsh-uz/trqsh/internal/billing"
	"github.com/trqsh-uz/trqsh/internal/billing/stripe"
)

// Server is the control-plane API.
type Server struct {
	cfg     Config
	log     *slog.Logger
	store   store.Store
	auth    *auth.Auth
	ent     *Entitlements
	billing *billing.Service

	authLimiter *rateLimiter
	apiLimiter  *rateLimiter

	providers map[string]auth.OAuthProvider

	stateMu sync.Mutex
	states  map[string]time.Time // oauth state -> expiry
}

// New builds the API server, choosing Postgres or the in-memory store from config.
func New(cfg Config, log *slog.Logger) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	var st store.Store
	if cfg.DatabaseURL != "" {
		ps, err := store.NewPostgresStore(cfg.DatabaseURL)
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
		providers: map[string]auth.OAuthProvider{},
		states:    map[string]time.Time{},
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

	if cfg.GitHubClientID != "" {
		s.providers["github"] = auth.GitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret,
			cfg.PublicURL+"/v1/auth/oauth/github/callback")
	}
	if cfg.GoogleClientID != "" {
		s.providers["google"] = auth.GoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret,
			cfg.PublicURL+"/v1/auth/oauth/google/callback")
	}

	// Abuse guards: a strict per-IP limit on auth endpoints (brute-force /
	// account-spam) and a broad flood limit on the rest of the public API.
	// The internal edge RPC is exempt (high-volume entitlement checks).
	s.authLimiter = newRateLimiter(5, 10, cfg.TrustProxy)
	s.apiLimiter = newRateLimiter(50, 100, cfg.TrustProxy)
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
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.securityHeaders)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	// Self-hosted, interactive API docs (Swagger UI) + live backend status.
	r.Get("/status", s.handleStatus)
	r.Get("/docs", s.handleDocsUI)
	r.Get("/openapi.yaml", s.handleOpenAPISpec)

	r.Route("/v1", func(r chi.Router) {
		// Per-IP flood guard on the whole public API (internal RPC is exempt).
		r.Use(s.apiLimiter.middleware)

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
		})
	})

	// Internal entitlements RPC (token-guarded).
	s.mountInternal(r)
	return r
}

// Run starts the HTTP server until ctx is canceled.
func (s *Server) Run(ctx context.Context) error {
	// Opt-in background metering: collect + push metered usage to Stripe.
	if bc := s.billing.Config(); bc.Enabled() && bc.MeteringInterval > 0 {
		go s.billing.RunMeteringLoop(ctx, bc.MeteringInterval)
		s.log.Info("billing metering loop started", "interval", bc.MeteringInterval)
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

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
