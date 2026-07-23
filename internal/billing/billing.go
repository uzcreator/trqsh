package billing

import (
	"context"
	"errors"
	"log/slog"

	"github.com/trqsh-uz/trqsh/internal/api/store"
	"github.com/trqsh-uz/trqsh/internal/billing/stripe"
)

// Metric names for metered usage (match the store's metered_usage.metric).
const (
	MetricBandwidth = "bandwidth_bytes"
	MetricRequests  = "requests"
)

// ErrBillingDisabled is returned by Checkout/Portal when Stripe is not configured.
var ErrBillingDisabled = errors.New("billing: Stripe is not configured")

// Service is the billing/monetization service: Stripe Checkout/Portal, webhooks
// that flip an org's plan, metered-usage collection, and quota enforcement. It
// reuses Part 05's store and is safe to construct even when Stripe is disabled
// (quota enforcement still runs — the fail-safe path never yields unlimited).
type Service struct {
	store  store.Store
	stripe stripe.API
	cfg    Config
	log    *slog.Logger
}

// New builds the billing service. api may be nil when Stripe is disabled; the
// quota and metered-collection paths do not touch it.
func New(st store.Store, api stripe.API, cfg Config, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{store: st, stripe: api, cfg: cfg, log: log}
}

// Config exposes the billing config (for wiring/tests).
func (s *Service) Config() Config { return s.cfg }

// resolveCustomer returns the org's Stripe customer ID, creating (and persisting)
// one on first use.
func (s *Service) resolveCustomer(ctx context.Context, orgID string) (string, error) {
	if !s.cfg.Enabled() || s.stripe == nil {
		return "", ErrBillingDisabled
	}
	org, err := s.store.GetOrg(ctx, orgID)
	if err != nil {
		return "", err
	}
	if org.StripeCustomerID != "" {
		return org.StripeCustomerID, nil
	}
	email, name := s.ownerContact(ctx, orgID)
	custID, err := s.stripe.CreateCustomer(ctx, stripe.CustomerParams{Email: email, Name: name, OrgID: orgID})
	if err != nil {
		return "", err
	}
	if err := s.store.SetOrgStripeCustomer(ctx, orgID, custID); err != nil {
		return "", err
	}
	return custID, nil
}

// ownerContact best-effort resolves an org owner's email + name for the Stripe
// customer record. Missing contact info is fine (Stripe accepts empty).
func (s *Service) ownerContact(ctx context.Context, orgID string) (email, name string) {
	members, err := s.store.ListOrgMembers(ctx, orgID)
	if err != nil {
		return "", ""
	}
	var userID string
	for _, m := range members {
		if m.Role == "owner" {
			userID = m.UserID
			break
		}
		if userID == "" {
			userID = m.UserID
		}
	}
	if userID == "" {
		return "", ""
	}
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return "", ""
	}
	return u.Email, u.Name
}

// planForSubscription maps a Stripe subscription to a trqsh plan code, preferring
// the configured price ID and falling back to the subscription metadata.
func (s *Service) planForSubscription(sub stripe.SubscriptionObject) string {
	if code, ok := PlanForPriceID(s.cfg.Prices, sub.PriceID()); ok {
		return code
	}
	if code := sub.Metadata["plan"]; code != "" {
		if _, ok := Catalog[code]; ok {
			return code
		}
	}
	return ""
}
