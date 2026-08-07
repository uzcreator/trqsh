// Package store is the persistence layer for the control plane. It defines the
// domain types and a Store interface with two implementations: an in-memory
// store (used in dev and tests) and a Postgres store (production).
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// Sentinel errors.
var (
	ErrNotFound = errors.New("store: not found")
	ErrConflict = errors.New("store: conflict")
)

// User is an authenticated person.
type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	AvatarURL     string    `json:"avatar_url,omitempty"`
	OAuthProvider string    `json:"oauth_provider,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Org is a billing/ownership boundary (personal or team).
type Org struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Plan             string     `json:"plan"`
	StripeCustomerID string     `json:"stripe_customer_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	// PlanExpiresAt, when set, is when a manually-granted plan reverts to Free.
	// nil means the plan never expires (the Stripe/webhook path and the default).
	PlanExpiresAt *time.Time `json:"plan_expires_at,omitempty"`
}

// OrgMember links a user to an org with a role (owner|admin|member).
type OrgMember struct {
	OrgID  string `json:"org_id"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// APIKey is a hashed agent credential. The plaintext is shown only once.
type APIKey struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Hash       string     `json:"-"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ReservedSubdomain is a globally-unique subdomain claimed by an org.
type ReservedSubdomain struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Subdomain string    `json:"subdomain"`
	CreatedAt time.Time `json:"created_at"`
}

// CustomDomain is an org-owned domain, verified via DNS.
type CustomDomain struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	Domain      string     `json:"domain"`
	VerifyToken string     `json:"verify_token"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`
	CertStatus  string     `json:"cert_status"`
	CreatedAt   time.Time  `json:"created_at"`
}

// UsageRecord is a metered slice of traffic for an org/tunnel window.
type UsageRecord struct {
	OrgID       string    `json:"org_id"`
	TunnelID    string    `json:"tunnel_id"`
	BytesIn     int64     `json:"bytes_in"`
	BytesOut    int64     `json:"bytes_out"`
	Requests    int64     `json:"requests"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

// TunnelSession is the history record for one bound tunnel instance: it is opened
// when the edge binds a tunnel and closed when the tunnel is released (or its
// agent session dies). It captures where the tunnel lived (edge/region), where the
// agent connected from (client IP → country/city), and its final traffic totals —
// the raw material for per-org history views and the admin fleet dashboard. The
// triple (EdgeID, SessionID, TunnelID) uniquely identifies a live instance and is
// the key used to close it.
type TunnelSession struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"org_id"`
	EdgeID    string     `json:"edge_id"`
	SessionID string     `json:"session_id"`
	TunnelID  string     `json:"tunnel_id"` // agent-chosen client_tunnel_id
	Type      string     `json:"type"`      // http|https|tls|tcp|udp
	PublicURL string     `json:"public_url"`
	Host      string     `json:"host,omitempty"`
	Port      int        `json:"port,omitempty"`
	Region    string     `json:"region,omitempty"`
	ClientIP  string     `json:"client_ip,omitempty"` // the agent's public IP
	Country   string     `json:"country,omitempty"`   // ISO alpha-2, resolved from ClientIP
	City      string     `json:"city,omitempty"`
	BytesIn   int64      `json:"bytes_in"`
	BytesOut  int64      `json:"bytes_out"`
	Requests  int64      `json:"requests"`
	Status    string     `json:"status"` // "active" | "closed"
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// TunnelSessionFilter narrows a history listing. A zero OrgID lists across all
// orgs (admin only); Limit<=0 falls back to a sane default in the store.
type TunnelSessionFilter struct {
	OrgID      string
	ActiveOnly bool
	Limit      int
	Offset     int
}

// UsageBucket is one point in a usage time series (for dashboard graphs).
type UsageBucket struct {
	Start    time.Time `json:"start"`
	BytesIn  int64     `json:"bytes_in"`
	BytesOut int64     `json:"bytes_out"`
	Requests int64     `json:"requests"`
}

// Subscription mirrors a Stripe subscription (Part 07). Webhooks are the source
// of truth: each update rewrites this row and the derived orgs.plan.
type Subscription struct {
	ID                string    `json:"id"`
	OrgID             string    `json:"org_id"`
	StripeSubID       string    `json:"stripe_subscription_id"`
	StripeCustomerID  string    `json:"stripe_customer_id"`
	Plan              string    `json:"plan"`   // plan code this subscription grants
	Status            string    `json:"status"` // active|trialing|past_due|canceled|incomplete
	CurrentPeriodEnd  time.Time `json:"current_period_end"`
	CancelAtPeriodEnd bool      `json:"cancel_at_period_end"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// MeteredUsage is one pending/settled usage push to Stripe for a metered plan.
// Unique per (org, metric, window_start) so collection is idempotent.
type MeteredUsage struct {
	ID          int64      `json:"id"`
	OrgID       string     `json:"org_id"`
	Metric      string     `json:"metric"` // "bandwidth_bytes" | "requests"
	Quantity    int64      `json:"quantity"`
	WindowStart time.Time  `json:"window_start"`
	WindowEnd   time.Time  `json:"window_end"`
	ReportedAt  *time.Time `json:"reported_at,omitempty"` // nil = not yet pushed to Stripe
	CreatedAt   time.Time  `json:"created_at"`
}

// Store is the persistence contract for the control plane.
type Store interface {
	// Users.
	CreateUser(ctx context.Context, u User) (User, error)
	GetUser(ctx context.Context, id string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	UpdateUserProfile(ctx context.Context, id, name, avatarURL string) error

	// Orgs & membership.
	CreateOrg(ctx context.Context, o Org) (Org, error)
	GetOrg(ctx context.Context, id string) (Org, error)
	OrgsByPlan(ctx context.Context, plan string) ([]Org, error)
	UpdateOrgPlan(ctx context.Context, orgID, plan string) error
	// SetOrgPlan sets an org's plan together with an expiry (nil = never expires).
	// Used by the admin grant flow; UpdateOrgPlan (Stripe) leaves expiry untouched.
	SetOrgPlan(ctx context.Context, orgID, plan string, expiresAt *time.Time) error
	SetOrgStripeCustomer(ctx context.Context, orgID, customerID string) error
	AddOrgMember(ctx context.Context, m OrgMember) error
	ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error)
	OrgsForUser(ctx context.Context, userID string) ([]Org, error)

	// API keys.
	CreateAPIKey(ctx context.Context, k APIKey) (APIKey, error)
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (APIKey, error)
	ListAPIKeys(ctx context.Context, orgID string) ([]APIKey, error)
	TouchAPIKey(ctx context.Context, id string, at time.Time) error
	RevokeAPIKey(ctx context.Context, id, orgID string, at time.Time) error

	// Reserved subdomains.
	ReserveSubdomain(ctx context.Context, s ReservedSubdomain) (ReservedSubdomain, error)
	ListSubdomains(ctx context.Context, orgID string) ([]ReservedSubdomain, error)
	ReleaseSubdomain(ctx context.Context, id, orgID string) error
	IsSubdomainReserved(ctx context.Context, orgID, subdomain string) (bool, error)
	CountSubdomains(ctx context.Context, orgID string) (int, error)

	// Custom domains.
	AddCustomDomain(ctx context.Context, d CustomDomain) (CustomDomain, error)
	GetCustomDomain(ctx context.Context, id, orgID string) (CustomDomain, error)
	GetCustomDomainByName(ctx context.Context, domain string) (CustomDomain, error)
	ListCustomDomains(ctx context.Context, orgID string) ([]CustomDomain, error)
	VerifyCustomDomain(ctx context.Context, id, orgID string, at time.Time) error
	CountCustomDomains(ctx context.Context, orgID string) (int, error)

	// Usage.
	UpsertUsage(ctx context.Context, u UsageRecord) error
	UsageForOrg(ctx context.Context, orgID string, since time.Time) (UsageRecord, error)
	// UsageSeriesForOrg returns usage bucketed by "hour" or "day" since a time,
	// for dashboard graphs. Buckets with no traffic are omitted.
	UsageSeriesForOrg(ctx context.Context, orgID string, since time.Time, bucket string) ([]UsageBucket, error)

	// Tunnel history. RecordTunnelOpen upserts an "active" session keyed by
	// (edge_id, session_id, tunnel_id); CloseTunnelSession stamps ended_at + final
	// counters for that key. ListTunnelSessions/counts drive per-org history and the
	// admin fleet view (empty filter.OrgID = across all orgs). TunnelCountryBreakdown
	// aggregates sessions by resolved country for the geo dashboard.
	RecordTunnelOpen(ctx context.Context, s TunnelSession) (TunnelSession, error)
	CloseTunnelSession(ctx context.Context, edgeID, sessionID, tunnelID string, endedAt time.Time, bytesIn, bytesOut, requests int64) error
	ListTunnelSessions(ctx context.Context, f TunnelSessionFilter) ([]TunnelSession, error)
	CountTunnelSessions(ctx context.Context, orgID string, activeOnly bool) (int, error)
	TunnelCountryBreakdown(ctx context.Context, orgID string, activeOnly bool) (map[string]int, error)

	// Billing: subscriptions (Part 07). GetOrgByStripeCustomer maps a Stripe
	// customer back to an org during webhook handling.
	UpsertSubscription(ctx context.Context, s Subscription) (Subscription, error)
	GetSubscriptionForOrg(ctx context.Context, orgID string) (Subscription, error)
	DeleteSubscription(ctx context.Context, stripeSubID string) error
	GetOrgByStripeCustomer(ctx context.Context, customerID string) (Org, error)

	// Billing: idempotent webhook processing. MarkEventProcessed returns true
	// the first time an event ID is seen, false if it was already handled.
	MarkEventProcessed(ctx context.Context, eventID, eventType string) (bool, error)

	// Billing: metered-usage push queue. InsertMeteredUsage returns ErrConflict
	// when (org, metric, window_start) already exists, making collection idempotent.
	InsertMeteredUsage(ctx context.Context, u MeteredUsage) (MeteredUsage, error)
	PendingMeteredUsage(ctx context.Context, limit int) ([]MeteredUsage, error)
	MarkMeteredUsageReported(ctx context.Context, ids []int64, at time.Time) error

	Close() error
}

// NewID returns a random identifier with the given prefix, e.g. "org_1a2b…".
func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
