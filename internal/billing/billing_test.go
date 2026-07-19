package billing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rift/rift/internal/api/store"
	"github.com/rift/rift/internal/billing/stripe"
	"github.com/rift/rift/pkg/proto"
)

// --- fakes / helpers ---

type fakeStripe struct {
	customers   int
	meterEvents []stripe.MeterEventParams
}

func (f *fakeStripe) CreateCustomer(context.Context, stripe.CustomerParams) (string, error) {
	f.customers++
	return "cus_test", nil
}
func (f *fakeStripe) CreateCheckoutSession(context.Context, stripe.CheckoutParams) (stripe.Session, error) {
	return stripe.Session{ID: "cs_1", URL: "https://checkout.stripe.test/cs_1"}, nil
}
func (f *fakeStripe) CreatePortalSession(context.Context, stripe.PortalParams) (stripe.Session, error) {
	return stripe.Session{ID: "bps_1", URL: "https://billing.stripe.test/bps_1"}, nil
}
func (f *fakeStripe) CreateMeterEvent(_ context.Context, p stripe.MeterEventParams) error {
	f.meterEvents = append(f.meterEvents, p)
	return nil
}

func testService(t *testing.T) (*Service, store.Store, *fakeStripe) {
	t.Helper()
	st := store.NewMemStore()
	fs := &fakeStripe{}
	cfg := DefaultConfig()
	cfg.SecretKey = "sk_test"
	cfg.WebhookSecret = "whsec_test"
	cfg.Prices = map[string]StripePrices{
		PlanPro:  {Monthly: "price_pro_m", Annual: "price_pro_y"},
		PlanTeam: {Monthly: "price_team_m"},
		PlanPAYG: {Metered: "price_payg"},
	}
	cfg.MeterBandwidth = "rift_bandwidth"
	cfg.MeterRequests = "rift_requests"
	return New(st, fs, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))), st, fs
}

func newOrg(t *testing.T, st store.Store, plan string) store.Org {
	t.Helper()
	org, err := st.CreateOrg(context.Background(), store.Org{Name: "t", Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	return org
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func subEvent(eventID, typ, orgID, customer, price, status string) []byte {
	return mustJSON(map[string]any{
		"id": eventID, "type": typ,
		"data": map[string]any{"object": map[string]any{
			"id": "sub_1", "customer": customer, "status": status,
			"cancel_at_period_end": false,
			"current_period_end":   time.Now().Add(30 * 24 * time.Hour).Unix(),
			"items":                map[string]any{"data": []any{map[string]any{"price": map[string]any{"id": price}}}},
			"metadata":             map[string]any{"org_id": orgID},
		}},
	})
}

func invoiceEvent(eventID, typ, customer string) []byte {
	return mustJSON(map[string]any{
		"id": eventID, "type": typ,
		"data": map[string]any{"object": map[string]any{
			"id": "in_1", "customer": customer, "subscription": "sub_1", "status": "open",
		}},
	})
}

func postWebhook(t *testing.T, s *Service, secret string, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", stripe.SignPayload(payload, secret, time.Now()))
	rec := httptest.NewRecorder()
	s.HandleWebhook(rec, req)
	return rec
}

func orgPlan(t *testing.T, st store.Store, orgID string) string {
	t.Helper()
	o, err := st.GetOrg(context.Background(), orgID)
	if err != nil {
		t.Fatal(err)
	}
	return o.Plan
}

// --- catalog ---

func TestLimitsForPlan(t *testing.T) {
	if LimitsForPlan(PlanFree).AllowUDP {
		t.Fatal("free must not allow UDP")
	}
	if !LimitsForPlan(PlanPro).AllowUDP {
		t.Fatal("pro must allow UDP")
	}
	if LimitsForPlan(PlanFree).MaxBandwidthBytesMo != 10*GB {
		t.Fatalf("free bandwidth = %d", LimitsForPlan(PlanFree).MaxBandwidthBytesMo)
	}
	// Unknown plan collapses to Free (fail-safe: never unlimited).
	if LimitsForPlan("bogus").MaxBandwidthBytesMo != 10*GB {
		t.Fatal("unknown plan must collapse to Free limits, not unlimited")
	}
}

func TestPlanForPriceID(t *testing.T) {
	prices := map[string]StripePrices{
		PlanPro:  {Monthly: "price_pro_m", Annual: "price_pro_y"},
		PlanPAYG: {Metered: "price_payg"},
	}
	if code, ok := PlanForPriceID(prices, "price_pro_y"); !ok || code != PlanPro {
		t.Fatalf("annual price should map to pro, got %q ok=%v", code, ok)
	}
	if code, ok := PlanForPriceID(prices, "price_payg"); !ok || code != PlanPAYG {
		t.Fatalf("metered price should map to payg, got %q ok=%v", code, ok)
	}
	if _, ok := PlanForPriceID(prices, "price_unknown"); ok {
		t.Fatal("unknown price should not map")
	}
}

// --- quota enforcement ---

func TestCheckQuotaBandwidth(t *testing.T) {
	s, st, _ := testService(t)
	org := newOrg(t, st, PlanFree)
	ctx := context.Background()

	// Under quota -> allowed.
	if code, _ := s.CheckQuota(ctx, org.ID, PlanFree); code != "" {
		t.Fatalf("fresh org should be within quota, got %s", code)
	}

	// Push usage past the 10 GB Free cap.
	_ = st.UpsertUsage(ctx, store.UsageRecord{
		OrgID: org.ID, BytesIn: 6 * GB, BytesOut: 5 * GB, Requests: 10,
		WindowStart: time.Now().Add(-time.Hour), WindowEnd: time.Now(),
	})
	code, msg := s.CheckQuota(ctx, org.ID, PlanFree)
	if code != proto.CodeQuotaBandwidth {
		t.Fatalf("over-quota code = %q, want %s", code, proto.CodeQuotaBandwidth)
	}
	if msg == "" {
		t.Fatal("over-quota should carry an upgrade message")
	}
}

func TestCheckQuotaPAYGNeverBlocks(t *testing.T) {
	s, st, _ := testService(t)
	org := newOrg(t, st, PlanPAYG)
	_ = st.UpsertUsage(context.Background(), store.UsageRecord{
		OrgID: org.ID, BytesIn: 100 * TB, WindowEnd: time.Now(),
	})
	if code, _ := s.CheckQuota(context.Background(), org.ID, PlanPAYG); code != "" {
		t.Fatalf("pay-as-you-go must never block at bind time, got %s", code)
	}
}

// errUsageStore fails UsageForOrg to exercise the fail-safe path.
type errUsageStore struct {
	store.Store
}

func (errUsageStore) UsageForOrg(context.Context, string, time.Time) (store.UsageRecord, error) {
	return store.UsageRecord{}, errors.New("db down")
}

func TestCheckQuotaFailSafeAllowsOnError(t *testing.T) {
	_, st, fs := testService(t)
	s := New(errUsageStore{st}, fs, DefaultConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	org := newOrg(t, st, PlanFree)
	// A usage-read failure must not deny the bind (never sever tunnels on a hiccup).
	if code, _ := s.CheckQuota(context.Background(), org.ID, PlanFree); code != "" {
		t.Fatalf("usage error should fail open, got %s", code)
	}
}

// --- webhook lifecycle ---

func TestWebhookUpgradeFlipsPlan(t *testing.T) {
	s, st, _ := testService(t)
	org := newOrg(t, st, PlanFree)

	rec := postWebhook(t, s, "whsec_test",
		subEvent("evt_1", "customer.subscription.created", org.ID, "cus_1", "price_pro_m", "active"))
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook status = %d body=%s", rec.Code, rec.Body)
	}
	if p := orgPlan(t, st, org.ID); p != PlanPro {
		t.Fatalf("plan after upgrade = %q, want pro", p)
	}
	sub, err := st.GetSubscriptionForOrg(context.Background(), org.ID)
	if err != nil || sub.Status != "active" || sub.Plan != PlanPro {
		t.Fatalf("subscription not recorded: %+v err=%v", sub, err)
	}
}

func TestWebhookIdempotent(t *testing.T) {
	s, st, _ := testService(t)
	org := newOrg(t, st, PlanFree)
	payload := subEvent("evt_dup", "customer.subscription.created", org.ID, "cus_1", "price_pro_m", "active")

	first := postWebhook(t, s, "whsec_test", payload)
	if first.Code != http.StatusOK {
		t.Fatalf("first webhook: %d", first.Code)
	}
	second := postWebhook(t, s, "whsec_test", payload)
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), "duplicate_ignored") {
		t.Fatalf("redelivered event should be ignored, got %d %s", second.Code, second.Body)
	}
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	s, st, _ := testService(t)
	org := newOrg(t, st, PlanFree)
	payload := subEvent("evt_2", "customer.subscription.created", org.ID, "cus_1", "price_pro_m", "active")

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", stripe.SignPayload(payload, "whsec_WRONG", time.Now()))
	rec := httptest.NewRecorder()
	s.HandleWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad signature should 400, got %d", rec.Code)
	}
	if p := orgPlan(t, st, org.ID); p != PlanFree {
		t.Fatalf("unverified event must not change plan, got %q", p)
	}
}

func TestWebhookDunningLifecycle(t *testing.T) {
	s, st, _ := testService(t)
	org := newOrg(t, st, PlanFree)
	ctx := context.Background()

	// Upgrade to Pro.
	postWebhook(t, s, "whsec_test", subEvent("e1", "customer.subscription.created", org.ID, "cus_1", "price_pro_m", "active"))
	if orgPlan(t, st, org.ID) != PlanPro {
		t.Fatal("should be pro after create")
	}
	// The customer mapping was persisted so customer-keyed events resolve.
	if _, err := st.GetOrgByStripeCustomer(ctx, "cus_1"); err != nil {
		t.Fatalf("customer should map to org: %v", err)
	}

	// Payment fails -> subscription past_due, but plan retained during grace.
	postWebhook(t, s, "whsec_test", invoiceEvent("e2", "invoice.payment_failed", "cus_1"))
	if p := orgPlan(t, st, org.ID); p != PlanPro {
		t.Fatalf("plan should be retained during dunning grace, got %q", p)
	}
	sub, _ := st.GetSubscriptionForOrg(ctx, org.ID)
	if sub.Status != "past_due" {
		t.Fatalf("subscription status = %q, want past_due", sub.Status)
	}

	// Recovery: invoice paid -> back to active.
	postWebhook(t, s, "whsec_test", invoiceEvent("e3", "invoice.paid", "cus_1"))
	sub, _ = st.GetSubscriptionForOrg(ctx, org.ID)
	if sub.Status != "active" {
		t.Fatalf("status after recovery = %q, want active", sub.Status)
	}

	// Cancellation -> downgrade to Free, subscription removed.
	postWebhook(t, s, "whsec_test", subEvent("e4", "customer.subscription.deleted", org.ID, "cus_1", "price_pro_m", "canceled"))
	if p := orgPlan(t, st, org.ID); p != PlanFree {
		t.Fatalf("plan after cancel = %q, want free", p)
	}
	if _, err := st.GetSubscriptionForOrg(ctx, org.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("subscription should be removed, err=%v", err)
	}
}

func TestWebhookResolvesOrgByCustomerWithoutMetadata(t *testing.T) {
	s, st, _ := testService(t)
	org := newOrg(t, st, PlanFree)
	ctx := context.Background()
	// Pre-bind the customer to the org (as checkout.session.completed would).
	if err := st.SetOrgStripeCustomer(ctx, org.ID, "cus_x"); err != nil {
		t.Fatal(err)
	}
	// Event carries no org_id metadata; resolution must fall back to the customer.
	payload := mustJSON(map[string]any{
		"id": "evt_nom", "type": "customer.subscription.updated",
		"data": map[string]any{"object": map[string]any{
			"id": "sub_1", "customer": "cus_x", "status": "active",
			"current_period_end": time.Now().Add(24 * time.Hour).Unix(),
			"items":              map[string]any{"data": []any{map[string]any{"price": map[string]any{"id": "price_pro_m"}}}},
		}},
	})
	if rec := postWebhook(t, s, "whsec_test", payload); rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if p := orgPlan(t, st, org.ID); p != PlanPro {
		t.Fatalf("plan = %q, want pro (resolved via customer)", p)
	}
}

// --- metering ---

func TestMeteringCollectAndFlush(t *testing.T) {
	s, st, fs := testService(t)
	ctx := context.Background()
	org := newOrg(t, st, PlanPAYG)
	if err := st.SetOrgStripeCustomer(ctx, org.ID, "cus_payg"); err != nil {
		t.Fatal(err)
	}
	_ = st.UpsertUsage(ctx, store.UsageRecord{
		OrgID: org.ID, BytesIn: 3 * GB, BytesOut: 1 * GB, Requests: 5000, WindowEnd: time.Now(),
	})

	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()
	created, err := s.CollectMeteredUsage(ctx, start, end)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if created != 2 { // one bandwidth + one requests row
		t.Fatalf("created = %d, want 2", created)
	}

	// Re-running the same window is idempotent (no new rows).
	again, err := s.CollectMeteredUsage(ctx, start, end)
	if err != nil || again != 0 {
		t.Fatalf("re-collect should create 0 rows, got %d err=%v", again, err)
	}

	// Flush pushes both rows to Stripe and marks them reported.
	n, err := s.FlushMeteredUsage(ctx)
	if err != nil {
		t.Fatalf("flush: %v", err)
	}
	if n != 2 {
		t.Fatalf("flushed = %d, want 2", n)
	}
	if len(fs.meterEvents) != 2 {
		t.Fatalf("stripe meter events = %d, want 2", len(fs.meterEvents))
	}
	// Nothing left pending; a second flush is a no-op.
	pending, _ := st.PendingMeteredUsage(ctx, 100)
	if len(pending) != 0 {
		t.Fatalf("pending after flush = %d, want 0", len(pending))
	}
	n2, _ := s.FlushMeteredUsage(ctx)
	if n2 != 0 {
		t.Fatalf("second flush = %d, want 0", n2)
	}
}
