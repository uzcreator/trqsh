package api

import "github.com/trqsh-uz/trqsh/internal/billing"

// The plan/quota catalog (§11) is owned by internal/billing (Part 07) so there
// is a single source of truth for limits, display pricing, and Stripe price IDs.
// The control plane re-exports it under the api package for the handlers and
// entitlements that referenced it before Part 07 existed.

// Plan is the billing plan/quota tier.
type Plan = billing.Plan

// Byte-size helpers.
const (
	GB = billing.GB
	TB = billing.TB
)

// Plan codes.
const (
	PlanFree = billing.PlanFree
	PlanPro  = billing.PlanPro
	PlanTeam = billing.PlanTeam
	PlanPAYG = billing.PlanPAYG
)

// Catalog is the frozen plan/quota catalog (§11), owned by internal/billing.
var Catalog = billing.Catalog

// PlanFor returns the plan for a code, defaulting to Free for unknown codes.
func PlanFor(code string) Plan { return billing.PlanFor(code) }
