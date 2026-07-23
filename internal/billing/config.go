package billing

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the billing/Stripe configuration (env-sourced). Quota enforcement
// works with a zero Config (Stripe disabled); only Checkout/Portal/webhooks and
// the metered-usage push require the Stripe keys.
type Config struct {
	SecretKey     string // TRQSH_STRIPE_SECRET_KEY (sk_test_… / sk_live_…)
	WebhookSecret string // TRQSH_STRIPE_WEBHOOK_SECRET (whsec_…)

	// Prices maps plan code -> Stripe price IDs (overlaid onto the catalog).
	Prices map[string]StripePrices

	// Meter event names for the Billing Meters API (Pay-as-you-go / overage).
	MeterBandwidth string // TRQSH_STRIPE_METER_BANDWIDTH
	MeterRequests  string // TRQSH_STRIPE_METER_REQUESTS

	SuccessURL      string // TRQSH_BILLING_SUCCESS_URL
	CancelURL       string // TRQSH_BILLING_CANCEL_URL
	PortalReturnURL string // TRQSH_BILLING_PORTAL_RETURN_URL

	TrialDays        int           // TRQSH_BILLING_TRIAL_DAYS (Pro trial)
	DunningGrace     time.Duration // TRQSH_BILLING_DUNNING_GRACE (past_due -> Free)
	WebhookTolerance time.Duration // signature timestamp tolerance
	MeteringInterval time.Duration // TRQSH_BILLING_METERING_INTERVAL (0 = off)
}

// Enabled reports whether Stripe is configured (checkout/portal/meter push).
func (c Config) Enabled() bool { return c.SecretKey != "" }

// PriceID resolves a plan+cadence to a configured Stripe price ID.
// cadence is "monthly" | "annual" | "metered".
func (c Config) PriceID(plan, cadence string) string {
	p, ok := c.Prices[plan]
	if !ok {
		return ""
	}
	switch cadence {
	case "annual":
		return p.Annual
	case "metered":
		return p.Metered
	default:
		return p.Monthly
	}
}

// MeterEventName maps an internal metric to its configured Stripe meter event.
func (c Config) MeterEventName(metric string) string {
	switch metric {
	case MetricBandwidth:
		return c.MeterBandwidth
	case MetricRequests:
		return c.MeterRequests
	default:
		return ""
	}
}

// DefaultConfig returns dev-friendly defaults (Stripe disabled).
func DefaultConfig() Config {
	return Config{
		Prices:           map[string]StripePrices{},
		SuccessURL:       "http://localhost:3000/billing?status=success",
		CancelURL:        "http://localhost:3000/billing?status=cancel",
		PortalReturnURL:  "http://localhost:3000/billing",
		TrialDays:        14,
		DunningGrace:     72 * time.Hour,
		WebhookTolerance: 5 * time.Minute,
	}
}

// LoadConfig overlays environment variables onto the defaults.
func LoadConfig() Config {
	c := DefaultConfig()
	env(&c.SecretKey, "TRQSH_STRIPE_SECRET_KEY")
	env(&c.WebhookSecret, "TRQSH_STRIPE_WEBHOOK_SECRET")
	env(&c.MeterBandwidth, "TRQSH_STRIPE_METER_BANDWIDTH")
	env(&c.MeterRequests, "TRQSH_STRIPE_METER_REQUESTS")
	env(&c.SuccessURL, "TRQSH_BILLING_SUCCESS_URL")
	env(&c.CancelURL, "TRQSH_BILLING_CANCEL_URL")
	env(&c.PortalReturnURL, "TRQSH_BILLING_PORTAL_RETURN_URL")

	c.Prices = map[string]StripePrices{
		PlanPro: {
			Monthly: os.Getenv("TRQSH_STRIPE_PRICE_PRO_MONTHLY"),
			Annual:  os.Getenv("TRQSH_STRIPE_PRICE_PRO_ANNUAL"),
		},
		PlanTeam: {
			Monthly: os.Getenv("TRQSH_STRIPE_PRICE_TEAM_MONTHLY"),
			Annual:  os.Getenv("TRQSH_STRIPE_PRICE_TEAM_ANNUAL"),
		},
		PlanPAYG: {
			Metered: os.Getenv("TRQSH_STRIPE_PRICE_PAYG_METERED"),
		},
	}
	if v := os.Getenv("TRQSH_BILLING_TRIAL_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.TrialDays = n
		}
	}
	if v := os.Getenv("TRQSH_BILLING_DUNNING_GRACE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.DunningGrace = d
		}
	}
	if v := os.Getenv("TRQSH_BILLING_METERING_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.MeteringInterval = d
		}
	}
	return c
}

func env(dst *string, key string) {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		*dst = strings.TrimSpace(v)
	}
}
