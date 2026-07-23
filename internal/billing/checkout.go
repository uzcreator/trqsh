package billing

import (
	"net/http"

	"github.com/trqsh-uz/trqsh/internal/billing/stripe"
)

// HandleCheckout creates a Stripe Checkout Session for the caller's org to
// subscribe to (or upgrade/downgrade to) a target plan, and returns its URL.
//
//	POST /v1/billing/checkout  {"plan":"pro","cadence":"monthly"}
//	-> 200 {"url":"https://checkout.stripe.com/...","session_id":"cs_..."}
func (s *Service) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Enabled() {
		writeError(w, http.StatusServiceUnavailable, ErrBillingDisabled.Error())
		return
	}
	var req struct {
		Plan    string `json:"plan"`
		Cadence string `json:"cadence"` // "monthly" | "annual" | "metered"
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Cadence == "" {
		req.Cadence = "monthly"
	}
	plan, ok := Catalog[req.Plan]
	if !ok || plan.Code == PlanFree {
		writeError(w, http.StatusBadRequest, "unknown or non-billable plan")
		return
	}
	priceID := s.cfg.PriceID(req.Plan, req.Cadence)
	if priceID == "" {
		writeError(w, http.StatusBadRequest, "no Stripe price configured for this plan/cadence")
		return
	}

	orgID := orgOf(r)
	customerID, err := s.resolveCustomer(r.Context(), orgID)
	if err != nil {
		s.log.Error("checkout: resolve customer", "org", orgID, "err", err)
		writeError(w, http.StatusBadGateway, "could not create billing customer")
		return
	}

	trial := 0
	if plan.Code == PlanPro {
		trial = s.cfg.TrialDays
	}
	sess, err := s.stripe.CreateCheckoutSession(r.Context(), stripe.CheckoutParams{
		CustomerID:      customerID,
		PriceID:         priceID,
		SuccessURL:      s.cfg.SuccessURL,
		CancelURL:       s.cfg.CancelURL,
		OrgID:           orgID,
		TrialDays:       trial,
		AllowPromoCodes: true,
		MeteredPrice:    req.Cadence == "metered" || plan.Metered(),
	})
	if err != nil {
		s.log.Error("checkout: create session", "org", orgID, "err", err)
		writeError(w, http.StatusBadGateway, "could not create checkout session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": sess.URL, "session_id": sess.ID})
}

// HandlePortal creates a Stripe Customer-Portal session so the caller can manage
// their subscription (update card, cancel, download invoices).
//
//	POST /v1/billing/portal -> 200 {"url":"https://billing.stripe.com/..."}
func (s *Service) HandlePortal(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Enabled() {
		writeError(w, http.StatusServiceUnavailable, ErrBillingDisabled.Error())
		return
	}
	orgID := orgOf(r)
	org, err := s.store.GetOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "org lookup failed")
		return
	}
	if org.StripeCustomerID == "" {
		writeError(w, http.StatusBadRequest, "no subscription to manage yet")
		return
	}
	sess, err := s.stripe.CreatePortalSession(r.Context(), stripe.PortalParams{
		CustomerID: org.StripeCustomerID,
		ReturnURL:  s.cfg.PortalReturnURL,
	})
	if err != nil {
		s.log.Error("portal: create session", "org", orgID, "err", err)
		writeError(w, http.StatusBadGateway, "could not create portal session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": sess.URL})
}

// HandleSubscription returns the caller org's current subscription + plan, for
// the dashboard billing screen.
//
//	GET /v1/billing/subscription
func (s *Service) HandleSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := orgOf(r)
	org, err := s.store.GetOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "org lookup failed")
		return
	}
	out := map[string]any{"plan": org.Plan, "billing_enabled": s.cfg.Enabled()}
	if sub, err := s.store.GetSubscriptionForOrg(r.Context(), orgID); err == nil {
		out["subscription"] = sub
	}
	writeJSON(w, http.StatusOK, out)
}
