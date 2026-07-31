package api

import (
	"net/http"
	"sort"

	"github.com/trqsh-uz/trqsh/internal/billing"
)

// publicPlan is the unauthenticated, display-safe projection of billing.Plan —
// deliberately excludes StripePrices (deploy-specific, irrelevant to public
// display) — mirroring what web/site/scripts/genplans.mjs generates into the
// marketing site's pricing page.
type publicPlan struct {
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	MaxConcurrentTunnels  int    `json:"max_concurrent_tunnels"`
	MaxBandwidthBytesMo   int64  `json:"max_bandwidth_bytes_mo"`
	MaxRequestsMo         int64  `json:"max_requests_mo"`
	MaxReservedSubdomains int    `json:"max_reserved_subdomains"`
	MaxCustomDomains      int    `json:"max_custom_domains"`
	AllowCustomDomains    bool   `json:"allow_custom_domains"`
	AllowTCP              bool   `json:"allow_tcp"`
	AllowTLS              bool   `json:"allow_tls"`
	AllowUDP              bool   `json:"allow_udp"`
	RateLimitRPS          int    `json:"rate_limit_rps"`
	InspectorHistory      int64  `json:"inspector_history"` // nanoseconds (Go time.Duration)
	MeteredSeats          bool   `json:"metered_seats"`
	PriceMonthlyCents     int    `json:"price_monthly_cents"`
	PriceAnnualCents      int    `json:"price_annual_cents"`
}

// planDisplayRank fixes catalog order (cheapest -> most flexible) for a stable
// public listing — Catalog is a map, and Go randomizes map iteration order.
var planDisplayRank = map[string]int{
	billing.PlanFree: 0,
	billing.PlanPro:  1,
	billing.PlanTeam: 2,
	billing.PlanPAYG: 3,
}

// handleListPlansPublic serves the plan catalog for unauthenticated, build-time
// consumption — no session or API key needed, the same public boundary as
// /openapi.yaml (handleOpenAPISpec). This exists specifically so the marketing
// site (a separate Go module/repo once split) can fetch pricing data over HTTP
// instead of importing internal/billing directly, which Go's internal/
// visibility rule would otherwise forbid across a module boundary.
func (s *Server) handleListPlansPublic(w http.ResponseWriter, _ *http.Request) {
	plans := make([]publicPlan, 0, len(billing.Catalog))
	for _, p := range billing.Catalog {
		plans = append(plans, publicPlan{
			Code:                  p.Code,
			Name:                  p.Name,
			MaxConcurrentTunnels:  p.MaxConcurrentTunnels,
			MaxBandwidthBytesMo:   p.MaxBandwidthBytesMo,
			MaxRequestsMo:         p.MaxRequestsMo,
			MaxReservedSubdomains: p.MaxReservedSubdomains,
			MaxCustomDomains:      p.MaxCustomDomains,
			AllowCustomDomains:    p.AllowCustomDomains,
			AllowTCP:              p.AllowTCP,
			AllowTLS:              p.AllowTLS,
			AllowUDP:              p.AllowUDP,
			RateLimitRPS:          p.RateLimitRPS,
			InspectorHistory:      int64(p.InspectorHistory),
			MeteredSeats:          p.MeteredSeats,
			PriceMonthlyCents:     p.PriceMonthlyCents,
			PriceAnnualCents:      p.PriceAnnualCents,
		})
	}
	sort.Slice(plans, func(i, j int) bool {
		return planDisplayRank[plans[i].Code] < planDisplayRank[plans[j].Code]
	})
	writeJSON(w, http.StatusOK, plans)
}
