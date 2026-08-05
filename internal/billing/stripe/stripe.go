// Package stripe is a small, dependency-free Stripe client covering exactly the
// surface trqsh needs: create customers, Checkout and Customer-Portal sessions,
// push metered usage via the Billing Meters API, and verify webhook signatures.
//
// We deliberately avoid the full github.com/stripe/stripe-go SDK: the pieces
// that matter for correctness here — webhook signature verification and the
// event->plan state machine — are small and must be unit-testable offline (the
// live API calls cannot be exercised without a Stripe account regardless), and
// a thin client keeps the module lean and its dependency surface auditable. The
// API interface makes swapping in the official SDK a drop-in change if desired.
package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// API is the Stripe surface the billing service depends on. The real Client
// implements it; tests inject a fake.
type API interface {
	CreateCustomer(ctx context.Context, p CustomerParams) (string, error)
	CreateCheckoutSession(ctx context.Context, p CheckoutParams) (Session, error)
	CreatePortalSession(ctx context.Context, p PortalParams) (Session, error)
	CreateMeterEvent(ctx context.Context, p MeterEventParams) error
}

// CustomerParams creates a Stripe customer bound to an org.
type CustomerParams struct {
	Email string
	Name  string
	OrgID string // stored in metadata for traceability
}

// CheckoutParams creates a subscription Checkout Session.
type CheckoutParams struct {
	CustomerID      string
	PriceID         string
	SuccessURL      string
	CancelURL       string
	OrgID           string // -> client_reference_id
	TrialDays       int
	AllowPromoCodes bool
	MeteredPrice    bool // metered prices are added without a quantity
}

// PortalParams creates a Customer-Portal Session.
type PortalParams struct {
	CustomerID string
	ReturnURL  string
}

// Session is a Checkout or Portal session (we only need id + redirect URL).
type Session struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// MeterEventParams reports one usage value for a metered price via the Billing
// Meters API (keyed by customer, so no subscription-item bookkeeping is needed).
type MeterEventParams struct {
	EventName  string // configured meter event_name, e.g. "trqsh_bandwidth"
	CustomerID string
	Value      int64
	Identifier string // idempotency: unique per usage window
	Timestamp  time.Time
}

// Client is the real HTTP Stripe client.
type Client struct {
	secretKey string
	baseURL   string
	http      *http.Client
}

var _ API = (*Client)(nil)

// New builds a Stripe client for the given secret key (sk_test_… / sk_live_…).
func New(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
		baseURL:   "https://api.stripe.com",
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

// WithBaseURL overrides the API base (used by tests to point at a stub server).
func (c *Client) WithBaseURL(u string) *Client { c.baseURL = strings.TrimRight(u, "/"); return c }

func (c *Client) CreateCustomer(ctx context.Context, p CustomerParams) (string, error) {
	form := url.Values{}
	if p.Email != "" {
		form.Set("email", p.Email)
	}
	if p.Name != "" {
		form.Set("name", p.Name)
	}
	if p.OrgID != "" {
		form.Set("metadata[org_id]", p.OrgID)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, "/v1/customers", form, "", &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *Client) CreateCheckoutSession(ctx context.Context, p CheckoutParams) (Session, error) {
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("customer", p.CustomerID)
	form.Set("line_items[0][price]", p.PriceID)
	if !p.MeteredPrice {
		form.Set("line_items[0][quantity]", "1")
	}
	form.Set("success_url", p.SuccessURL)
	form.Set("cancel_url", p.CancelURL)
	if p.OrgID != "" {
		form.Set("client_reference_id", p.OrgID)
		form.Set("subscription_data[metadata][org_id]", p.OrgID)
	}
	if p.TrialDays > 0 {
		form.Set("subscription_data[trial_period_days]", strconv.Itoa(p.TrialDays))
	}
	if p.AllowPromoCodes {
		form.Set("allow_promotion_codes", "true")
	}
	var s Session
	err := c.post(ctx, "/v1/checkout/sessions", form, "", &s)
	return s, err
}

func (c *Client) CreatePortalSession(ctx context.Context, p PortalParams) (Session, error) {
	form := url.Values{}
	form.Set("customer", p.CustomerID)
	form.Set("return_url", p.ReturnURL)
	var s Session
	err := c.post(ctx, "/v1/billing_portal/sessions", form, "", &s)
	return s, err
}

func (c *Client) CreateMeterEvent(ctx context.Context, p MeterEventParams) error {
	form := url.Values{}
	form.Set("event_name", p.EventName)
	form.Set("payload[stripe_customer_id]", p.CustomerID)
	form.Set("payload[value]", strconv.FormatInt(p.Value, 10))
	if !p.Timestamp.IsZero() {
		form.Set("timestamp", strconv.FormatInt(p.Timestamp.Unix(), 10))
	}
	if p.Identifier != "" {
		form.Set("identifier", p.Identifier)
	}
	// The identifier doubles as the idempotency key so retries never double-count.
	return c.post(ctx, "/v1/billing/meter_events", form, p.Identifier, nil)
}

// post sends a form-encoded request, optionally with an idempotency key, and
// decodes the JSON response into out (out may be nil).
func (c *Client) post(ctx context.Context, path string, form url.Values, idempotencyKey string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Status: resp.StatusCode, Body: string(body)}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// Error is a non-2xx Stripe API response.
type Error struct {
	Status int
	Body   string
}

func (e *Error) Error() string { return fmt.Sprintf("stripe: status %d: %s", e.Status, e.Body) }

// --- Webhook signature verification ---

// ErrInvalidSignature is returned when a webhook signature fails verification.
var ErrInvalidSignature = fmt.Errorf("stripe: webhook signature verification failed")

// VerifySignature checks a Stripe-Signature header against the raw request body
// using the webhook signing secret. It implements Stripe's scheme: the signed
// payload is "t.<body>", HMAC-SHA256 with the secret, compared (constant time)
// against each v1 signature, and the timestamp must be within tolerance.
func VerifySignature(payload []byte, sigHeader, secret string, tolerance time.Duration) error {
	if secret == "" {
		return fmt.Errorf("stripe: empty webhook secret")
	}
	var tsStr string
	var v1s []string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			tsStr = kv[1]
		case "v1":
			v1s = append(v1s, kv[1])
		}
	}
	if tsStr == "" || len(v1s) == 0 {
		return ErrInvalidSignature
	}
	tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}
	if tolerance > 0 {
		if age := time.Since(time.Unix(tsUnix, 0)); age > tolerance || age < -tolerance {
			return fmt.Errorf("stripe: webhook timestamp outside tolerance")
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsStr))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, got := range v1s {
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1 {
			return nil
		}
	}
	return ErrInvalidSignature
}

// SignPayload produces a valid Stripe-Signature header for a payload. It exists
// so tests (and local webhook stubs) can forge correctly-signed events; the real
// signing happens inside Stripe.
func SignPayload(payload []byte, secret string, ts time.Time) string {
	tsStr := strconv.FormatInt(ts.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsStr))
	mac.Write([]byte("."))
	mac.Write(payload)
	return "t=" + tsStr + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// --- Event wire types ---

// Event is the envelope of a Stripe webhook event.
type Event struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

// ParseEvent unmarshals a webhook body into an Event.
func ParseEvent(payload []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(payload, &e)
	return e, err
}

// Decode unmarshals the event's data.object into a typed struct.
func (e Event) Decode(v any) error { return json.Unmarshal(e.Data.Object, v) }

// SubscriptionObject is the subset of a Stripe subscription we consume.
type SubscriptionObject struct {
	ID                string `json:"id"`
	Customer          string `json:"customer"`
	Status            string `json:"status"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	CurrentPeriodEnd  int64  `json:"current_period_end"`
	Items             struct {
		Data []struct {
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

// PriceID returns the first line item's price ID (the plan-granting price).
func (s SubscriptionObject) PriceID() string {
	if len(s.Items.Data) > 0 {
		return s.Items.Data[0].Price.ID
	}
	return ""
}

// PeriodEnd returns current_period_end as a time (zero if unset).
func (s SubscriptionObject) PeriodEnd() time.Time {
	if s.CurrentPeriodEnd == 0 {
		return time.Time{}
	}
	return time.Unix(s.CurrentPeriodEnd, 0)
}

// CheckoutSessionObject is the subset of a completed Checkout Session we consume.
type CheckoutSessionObject struct {
	ID                string            `json:"id"`
	Customer          string            `json:"customer"`
	Subscription      string            `json:"subscription"`
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
}

// InvoiceObject is the subset of a Stripe invoice we consume (dunning).
type InvoiceObject struct {
	ID           string `json:"id"`
	Customer     string `json:"customer"`
	Subscription string `json:"subscription"`
	Status       string `json:"status"`
}
