package billing

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/trqsh-uz/trqsh/internal/api/store"
	"github.com/trqsh-uz/trqsh/internal/billing/stripe"
)

// HandleWebhook is the Stripe webhook endpoint (public, but signature-verified).
// It is the source of truth for subscription state: each event updates the
// subscriptions row and the derived orgs.plan.
//
//	POST /v1/billing/webhooks
func (s *Service) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body")
		return
	}
	if s.cfg.WebhookSecret == "" {
		// Refuse to process unverifiable events rather than trusting them.
		writeError(w, http.StatusServiceUnavailable, "webhook secret not configured")
		return
	}
	if err := stripe.VerifySignature(body, r.Header.Get("Stripe-Signature"), s.cfg.WebhookSecret, s.cfg.WebhookTolerance); err != nil {
		s.log.Warn("webhook: signature verification failed", "err", err)
		writeError(w, http.StatusBadRequest, "invalid signature")
		return
	}
	event, err := stripe.ParseEvent(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event")
		return
	}

	// Idempotency: Stripe redelivers events; process each at most once.
	first, err := s.store.MarkEventProcessed(r.Context(), event.ID, event.Type)
	if err != nil {
		s.log.Error("webhook: dedupe", "event", sanitizeLog(event.ID), "err", err)
		writeError(w, http.StatusInternalServerError, "dedupe failed")
		return
	}
	if !first {
		writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate_ignored"})
		return
	}

	if err := s.dispatch(r.Context(), event); err != nil {
		// Return 500 so Stripe retries transient failures.
		s.log.Error("webhook: handler failed", "type", sanitizeLog(event.Type), "event", sanitizeLog(event.ID), "err", err)
		writeError(w, http.StatusInternalServerError, "handler error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// dispatch routes a verified event to its handler. Unknown event types are a
// successful no-op (Stripe sends many we don't subscribe to).
func (s *Service) dispatch(ctx context.Context, e stripe.Event) error {
	switch e.Type {
	case "checkout.session.completed":
		return s.onCheckoutCompleted(ctx, e)
	case "customer.subscription.created", "customer.subscription.updated":
		return s.onSubscriptionUpsert(ctx, e)
	case "customer.subscription.deleted":
		return s.onSubscriptionDeleted(ctx, e)
	case "invoice.paid", "invoice.payment_succeeded":
		return s.onInvoicePaid(ctx, e)
	case "invoice.payment_failed":
		return s.onInvoicePaymentFailed(ctx, e)
	default:
		s.log.Debug("webhook: ignoring event", "type", sanitizeLog(e.Type))
		return nil
	}
}

// (subscription/invoice objects are decoded via stripe.Event.Decode.)

// onCheckoutCompleted binds the Stripe customer to the org so later events (and
// the Customer Portal) can resolve it. The plan itself is applied by the
// subscription.created/updated event that follows.
func (s *Service) onCheckoutCompleted(ctx context.Context, e stripe.Event) error {
	var obj stripe.CheckoutSessionObject
	if err := e.Decode(&obj); err != nil {
		return err
	}
	orgID := obj.ClientReferenceID
	if orgID == "" {
		orgID = obj.Metadata["org_id"]
	}
	if orgID != "" && obj.Customer != "" {
		if err := s.store.SetOrgStripeCustomer(ctx, orgID, obj.Customer); err != nil {
			return err
		}
	}
	return nil
}

// onSubscriptionUpsert applies a created/updated subscription: it rewrites the
// subscription row and derives orgs.plan from the subscription status.
func (s *Service) onSubscriptionUpsert(ctx context.Context, e stripe.Event) error {
	var sub stripe.SubscriptionObject
	if err := e.Decode(&sub); err != nil {
		return err
	}
	orgID, err := s.resolveOrgID(ctx, sub.Metadata, sub.Customer)
	if err != nil {
		return err
	}
	// Keep the org<->customer link in sync so later customer-keyed events (e.g.
	// invoice.payment_failed) resolve the org even without org_id metadata.
	if sub.Customer != "" {
		if org, gerr := s.store.GetOrg(ctx, orgID); gerr == nil && org.StripeCustomerID != sub.Customer {
			if err := s.store.SetOrgStripeCustomer(ctx, orgID, sub.Customer); err != nil {
				return err
			}
		}
	}
	plan := s.planForSubscription(sub)
	if _, err := s.store.UpsertSubscription(ctx, store.Subscription{
		OrgID:             orgID,
		StripeSubID:       sub.ID,
		StripeCustomerID:  sub.Customer,
		Plan:              plan,
		Status:            sub.Status,
		CurrentPeriodEnd:  sub.PeriodEnd(),
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
	}); err != nil {
		return err
	}
	if target := derivePlan(sub.Status, plan); target != "" {
		return s.store.UpdateOrgPlan(ctx, orgID, target)
	}
	return nil
}

// onSubscriptionDeleted downgrades the org to Free and removes the subscription.
func (s *Service) onSubscriptionDeleted(ctx context.Context, e stripe.Event) error {
	var sub stripe.SubscriptionObject
	if err := e.Decode(&sub); err != nil {
		return err
	}
	orgID, err := s.resolveOrgID(ctx, sub.Metadata, sub.Customer)
	if err != nil {
		return err
	}
	if err := s.store.UpdateOrgPlan(ctx, orgID, PlanFree); err != nil {
		return err
	}
	if err := s.store.DeleteSubscription(ctx, sub.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return nil
}

// onInvoicePaid re-affirms the org's plan when a renewal succeeds (recovering
// from a prior past_due state).
func (s *Service) onInvoicePaid(ctx context.Context, e stripe.Event) error {
	var inv stripe.InvoiceObject
	if err := e.Decode(&inv); err != nil {
		return err
	}
	if inv.Customer == "" {
		return nil
	}
	org, err := s.store.GetOrgByStripeCustomer(ctx, inv.Customer)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	sub, err := s.store.GetSubscriptionForOrg(ctx, org.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if sub.Status == "past_due" || sub.Status == "unpaid" {
		sub.Status = "active"
		if _, err := s.store.UpsertSubscription(ctx, sub); err != nil {
			return err
		}
	}
	if sub.Plan != "" {
		return s.store.UpdateOrgPlan(ctx, org.ID, sub.Plan)
	}
	return nil
}

// onInvoicePaymentFailed marks the subscription past_due (dunning). The plan is
// retained during Stripe's Smart-Retry grace window; if retries are exhausted
// Stripe emits customer.subscription.deleted, which downgrades to Free.
func (s *Service) onInvoicePaymentFailed(ctx context.Context, e stripe.Event) error {
	var inv stripe.InvoiceObject
	if err := e.Decode(&inv); err != nil {
		return err
	}
	if inv.Customer == "" {
		return nil
	}
	org, err := s.store.GetOrgByStripeCustomer(ctx, inv.Customer)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	sub, err := s.store.GetSubscriptionForOrg(ctx, org.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	sub.Status = "past_due"
	_, err = s.store.UpsertSubscription(ctx, sub)
	return err
}

// resolveOrgID finds the org for an event, preferring the metadata org_id and
// falling back to the Stripe customer mapping.
func (s *Service) resolveOrgID(ctx context.Context, metadata map[string]string, customerID string) (string, error) {
	if id := metadata["org_id"]; id != "" {
		return id, nil
	}
	org, err := s.store.GetOrgByStripeCustomer(ctx, customerID)
	if err != nil {
		return "", err
	}
	return org.ID, nil
}

// derivePlan maps a Stripe subscription status + subscribed plan to the plan
// code that should be written to orgs.plan (empty = leave the plan unchanged).
func derivePlan(status, plan string) string {
	switch status {
	case "active", "trialing":
		return plan // "" when the price is unrecognized: leave plan untouched
	case "canceled", "incomplete_expired", "unpaid":
		return PlanFree
	case "past_due":
		return "" // keep plan during the dunning grace window
	default:
		return ""
	}
}
