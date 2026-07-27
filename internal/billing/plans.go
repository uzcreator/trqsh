// Package billing owns monetization for trqsh: the canonical plan/quota catalog
// (§11 of 00-ARCHITECTURE.md), Stripe Checkout/Portal, signature-verified
// webhooks that flip an org's plan, metered-usage ingestion, and the quota
// enforcement surfaced through Part 05's authz.Entitlements. It reuses Part 05's
// store (internal/api/store) and never imports internal/api (that would cycle).
package billing

import (
	"time"

	"github.com/trqsh-uz/trqsh/pkg/authz"
)

// Byte-size helpers.
const (
	GB int64 = 1 << 30
	TB int64 = 1 << 40
)

// Plan codes (the string stored in orgs.plan and carried on the wire).
const (
	PlanFree = "free"
	PlanPro  = "pro"
	PlanTeam = "team"
	PlanPAYG = "payg"
)

// StripePrices holds the Stripe price IDs that grant a plan. They are
// deploy-specific and injected from config (see Config.PricesFor); the catalog
// keeps them zero so the frozen limits/display never depend on a live account.
type StripePrices struct {
	Monthly string `json:"monthly,omitempty"` // recurring price for the monthly cadence
	Annual  string `json:"annual,omitempty"`  // recurring price for the annual cadence
	Metered string `json:"metered,omitempty"` // usage price (Pay-as-you-go / overage)
}

// Plan is a plan/quota tier: the frozen §11 limits, display pricing for the
// marketing/pricing page (Part 09), and the Stripe price IDs that grant it.
// This is the single source of truth consumed by Part 05 entitlements, Part 06
// billing UI, and Part 09 pricing. internal/api aliases this type.
type Plan struct {
	Code string `json:"code"`
	Name string `json:"name"`

	// Quota limits (§11). 0 means unlimited/metered where noted.
	MaxConcurrentTunnels  int           `json:"max_concurrent_tunnels"`
	MaxBandwidthBytesMo   int64         `json:"max_bandwidth_bytes_mo"` // 0 = unlimited/metered
	MaxRequestsMo         int64         `json:"max_requests_mo"`        // 0 = unlimited/metered
	MaxReservedSubdomains int           `json:"max_reserved_subdomains"`
	MaxCustomDomains      int           `json:"max_custom_domains"`
	AllowCustomDomains    bool          `json:"allow_custom_domains"`
	AllowTCP              bool          `json:"allow_tcp"`
	AllowTLS              bool          `json:"allow_tls"`
	AllowUDP              bool          `json:"allow_udp"`
	RateLimitRPS          int           `json:"rate_limit_rps"`
	InspectorHistory      time.Duration `json:"inspector_history"`
	MeteredSeats          bool          `json:"metered_seats"`

	// Display pricing for the pricing page (cents, USD). Metered plans price by usage.
	PriceMonthlyCents int `json:"price_monthly_cents"`
	PriceAnnualCents  int `json:"price_annual_cents"`

	// StripePrices is populated at deploy time from Config; zero in the catalog.
	StripePrices StripePrices `json:"stripe_prices"`
}

// Limits maps a Plan to the frozen authz.Limits the edge enforces (§9).
func (p Plan) Limits() authz.Limits {
	return authz.Limits{
		MaxConcurrentTunnels:   p.MaxConcurrentTunnels,
		MaxBandwidthBytesMo:    p.MaxBandwidthBytesMo,
		MaxRequestsMo:          p.MaxRequestsMo,
		AllowCustomDomains:     p.AllowCustomDomains,
		AllowReservedSubdomain: p.MaxReservedSubdomains > 0,
		AllowTCP:               p.AllowTCP,
		AllowUDP:               p.AllowUDP,
		RateLimitRPS:           p.RateLimitRPS,
	}
}

// Metered reports whether the plan bills by usage rather than a flat quota.
func (p Plan) Metered() bool { return p.Code == PlanPAYG }

// Catalog is the frozen plan/quota catalog (§11). Numbers are tunable here.
var Catalog = map[string]Plan{
	PlanFree: {
		Code: PlanFree, Name: "Free",
		MaxConcurrentTunnels: 3, MaxBandwidthBytesMo: 10 * GB, MaxRequestsMo: 0,
		MaxReservedSubdomains: 1, MaxCustomDomains: 0,
		AllowCustomDomains: false, AllowTCP: true, AllowTLS: false, AllowUDP: false,
		RateLimitRPS: 60, InspectorHistory: time.Hour,
		PriceMonthlyCents: 0, PriceAnnualCents: 0,
	},
	PlanPro: {
		Code: PlanPro, Name: "Pro",
		MaxConcurrentTunnels: 10, MaxBandwidthBytesMo: 200 * GB, MaxRequestsMo: 0,
		MaxReservedSubdomains: 10, MaxCustomDomains: 5,
		AllowCustomDomains: true, AllowTCP: true, AllowTLS: true, AllowUDP: true,
		RateLimitRPS: 600, InspectorHistory: 30 * 24 * time.Hour,
		// $3/mo, or $2/mo billed annually ($24/yr). PriceAnnualCents is the yearly total.
		PriceMonthlyCents: 300, PriceAnnualCents: 2400,
	},
	PlanTeam: {
		Code: PlanTeam, Name: "Team",
		MaxConcurrentTunnels: 25, MaxBandwidthBytesMo: TB, MaxRequestsMo: 0,
		MaxReservedSubdomains: 50, MaxCustomDomains: 50,
		AllowCustomDomains: true, AllowTCP: true, AllowTLS: true, AllowUDP: true,
		RateLimitRPS: 2000, InspectorHistory: 30 * 24 * time.Hour,
		// $5/mo, or $3.70/mo billed annually ($44.40/yr). PriceAnnualCents is the yearly total.
		PriceMonthlyCents: 500, PriceAnnualCents: 4440,
	},
	PlanPAYG: {
		Code: PlanPAYG, Name: "Pay-as-you-go",
		MaxConcurrentTunnels: 0, MaxBandwidthBytesMo: 0, MaxRequestsMo: 0,
		MaxReservedSubdomains: 1000, MaxCustomDomains: 1000,
		AllowCustomDomains: true, AllowTCP: true, AllowTLS: true, AllowUDP: true,
		RateLimitRPS: 0, InspectorHistory: 30 * 24 * time.Hour, MeteredSeats: true,
	},
}

// PlanFor returns the plan for a code, defaulting to Free for unknown codes.
// This is the fail-safe path: an unrecognized plan never yields unlimited.
func PlanFor(code string) Plan {
	if p, ok := Catalog[code]; ok {
		return p
	}
	return Catalog[PlanFree]
}

// LimitsForPlan returns the authz.Limits for a plan code (Part 05's CheckBind
// consults this). Unknown codes collapse to Free — never unlimited.
func LimitsForPlan(code string) authz.Limits { return PlanFor(code).Limits() }

// PlanForPriceID returns the plan whose configured Stripe prices include the
// given price ID, and whether one matched. Used by webhooks to translate a
// Stripe subscription's price back into a plan code.
func PlanForPriceID(prices map[string]StripePrices, priceID string) (string, bool) {
	if priceID == "" {
		return "", false
	}
	for code, p := range prices {
		if p.Monthly == priceID || p.Annual == priceID || p.Metered == priceID {
			return code, true
		}
	}
	return "", false
}
