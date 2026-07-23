// Command genplans regenerates web/site/lib/catalog.generated.ts from the
// canonical plan catalog in internal/billing (§11 of 00-ARCHITECTURE.md).
//
// The marketing/pricing site (Part 09) must never hardcode prices or limits:
// billing.Catalog is "the single source of truth consumed by Part 09 pricing"
// (see internal/billing/plans.go). Because the site is TypeScript, it can't
// import the Go catalog directly, so we generate a typed snapshot from it and
// check the result in. CI re-runs this and fails on any diff, so the site can
// never drift from what the edge actually enforces.
//
// Regenerate:  go run ./web/site/scripts/genplans   (or: make site-plans)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/trqsh-uz/trqsh/internal/billing"
)

// outPlan mirrors the dashboard's TS `Plan` shape (web/dashboard/lib/api.ts) and
// the JSON emitted by GET /v1/plans, so the generated snapshot is byte-for-byte
// the same contract the authenticated API returns — minus deploy-specific Stripe
// price IDs, which are irrelevant to public display.
type outPlan struct {
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

// rank fixes catalog order (cheapest → most flexible) so output is deterministic
// regardless of Go's randomized map iteration — required for a stable CI diff.
var rank = map[string]int{
	billing.PlanFree: 0,
	billing.PlanPro:  1,
	billing.PlanTeam: 2,
	billing.PlanPAYG: 3,
}

func main() {
	plans := make([]outPlan, 0, len(billing.Catalog))
	for _, p := range billing.Catalog {
		plans = append(plans, outPlan{
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
	sort.Slice(plans, func(i, j int) bool { return rank[plans[i].Code] < rank[plans[j].Code] })

	body, err := json.MarshalIndent(plans, "", "  ")
	if err != nil {
		fatal(err)
	}

	out := fmt.Sprintf(`// Code generated from internal/billing.Catalog by web/site/scripts/genplans.
// DO NOT EDIT — run "make site-plans" (or "go run ./web/site/scripts/genplans")
// after changing the plan catalog in internal/billing/plans.go.
//
// This is how Part 09 consumes the single source of truth for pricing without
// hardcoding: prices/limits shown here can never drift from what the edge enforces.

export interface CatalogPlan {
  code: string;
  name: string;
  max_concurrent_tunnels: number;
  max_bandwidth_bytes_mo: number;
  max_requests_mo: number;
  max_reserved_subdomains: number;
  max_custom_domains: number;
  allow_custom_domains: boolean;
  allow_tcp: boolean;
  allow_tls: boolean;
  allow_udp: boolean;
  rate_limit_rps: number;
  /** Go time.Duration in nanoseconds (see formatRetention). */
  inspector_history: number;
  metered_seats: boolean;
  price_monthly_cents: number;
  price_annual_cents: number;
}

export const CATALOG: CatalogPlan[] = %s;
`, string(body))

	dest := outputPath()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(dest, []byte(out), 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (%d plans)\n", dest, len(plans))
}

// outputPath resolves web/site/lib/catalog.generated.ts relative to THIS source
// file, so the generator works no matter the current working directory.
func outputPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fatal(fmt.Errorf("cannot locate generator source"))
	}
	// file = <repo>/web/site/scripts/genplans/main.go
	return filepath.Join(filepath.Dir(file), "..", "..", "lib", "catalog.generated.ts")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "genplans:", err)
	os.Exit(1)
}
