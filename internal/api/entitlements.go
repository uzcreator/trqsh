package api

import (
	"context"
	"crypto/rand"

	"github.com/trqsh-uz/trqsh/internal/api/auth"
	"github.com/trqsh-uz/trqsh/internal/api/store"
	"github.com/trqsh-uz/trqsh/pkg/authz"
	"github.com/trqsh-uz/trqsh/pkg/proto"
)

// QuotaChecker consults billing (Part 07) for current-period metered quota. It
// returns a §8 error code + message when the org is over quota, or ("","") when
// within quota. It mirrors the authz.Entitlements seam: Part 05 depends only on
// this interface, and internal/billing.Service satisfies it.
type QuotaChecker interface {
	CheckQuota(ctx context.Context, orgID, plan string) (code, message string)
}

// Entitlements is the real authz.Entitlements implementation (§9), backed by the
// store and the plan catalog. It is what the edge reaches via entitlerpc.
type Entitlements struct {
	auth       *auth.Auth
	store      store.Store
	baseDomain string
	quota      QuotaChecker // optional; nil = no metered-quota enforcement
}

var _ authz.Entitlements = (*Entitlements)(nil)

// NewEntitlements builds the entitlements service.
func NewEntitlements(a *auth.Auth, st store.Store, baseDomain string) *Entitlements {
	return &Entitlements{auth: a, store: st, baseDomain: baseDomain}
}

// SetQuota wires in a billing quota checker so CheckBind enforces metered limits.
func (e *Entitlements) SetQuota(q QuotaChecker) { e.quota = q }

// Authenticate validates an API key and returns (orgID, plan).
func (e *Entitlements) Authenticate(ctx context.Context, apiKey string) (string, string, error) {
	k, err := e.auth.AuthenticateAPIKey(ctx, apiKey)
	if err != nil {
		return "", "", err
	}
	org, err := e.store.GetOrg(ctx, k.OrgID)
	if err != nil {
		return "", "", err
	}
	return org.ID, org.Plan, nil
}

// CheckBind authorizes a bind against the org's plan and reservations.
func (e *Entitlements) CheckBind(ctx context.Context, req authz.BindRequest) (authz.Decision, error) {
	accountID, planCode, err := e.Authenticate(ctx, req.APIKey)
	if err != nil {
		return deny(proto.CodeAuthInvalid, "invalid api key"), nil
	}
	plan := PlanFor(planCode)
	dec := authz.Decision{AccountID: accountID, Plan: planCode, Limits: plan.Limits()}

	// Protocol entitlements.
	switch req.Type {
	case "udp":
		if !plan.AllowUDP {
			return denyWith(dec, proto.CodePlanForbids, "UDP tunnels require a paid plan"), nil
		}
	case "tls":
		if !plan.AllowTLS {
			return denyWith(dec, proto.CodePlanForbids, "TLS tunnels require a paid plan"), nil
		}
	case "tcp":
		if !plan.AllowTCP {
			return denyWith(dec, proto.CodePlanForbids, "TCP tunnels are not allowed on this plan"), nil
		}
	}

	// Custom domain must be owned + verified.
	if req.CustomHost != "" {
		if !plan.AllowCustomDomains {
			return denyWith(dec, proto.CodePlanForbids, "custom domains require a paid plan"), nil
		}
		d, err := e.store.GetCustomDomainByName(ctx, req.CustomHost)
		if err != nil || d.OrgID != accountID || d.VerifiedAt == nil {
			return denyWith(dec, proto.CodeDomainUnverified, "custom domain not verified"), nil
		}
	}

	// Specific subdomain must be reserved by this org; otherwise assign a random one.
	if req.Subdomain != "" {
		reserved, err := e.store.IsSubdomainReserved(ctx, accountID, req.Subdomain)
		if err != nil {
			return deny(proto.CodeInternal, "subdomain lookup failed"), nil
		}
		if !reserved {
			return denyWith(dec, proto.CodeSubdomainForbidden, "subdomain is not reserved by your account"), nil
		}
	} else if req.CustomHost == "" {
		dec.AssignedSubdomain = randSubdomain(8)
	}

	// Metered quota (bandwidth/requests) enforcement via billing (Part 07).
	if e.quota != nil {
		if code, msg := e.quota.CheckQuota(ctx, accountID, planCode); code != "" {
			return denyWith(dec, code, msg), nil
		}
	}

	dec.Allow = true
	return dec, nil
}

// ReportUsage persists a usage window for billing/metering.
func (e *Entitlements) ReportUsage(ctx context.Context, u authz.Usage) error {
	return e.store.UpsertUsage(ctx, store.UsageRecord{
		OrgID:       u.AccountID,
		TunnelID:    u.TunnelID,
		BytesIn:     u.BytesIn,
		BytesOut:    u.BytesOut,
		Requests:    u.Requests,
		WindowStart: u.WindowStart,
		WindowEnd:   u.WindowEnd,
	})
}

func deny(code, msg string) authz.Decision {
	return authz.Decision{Allow: false, ErrorCode: code, ErrorMessage: msg}
}

func denyWith(dec authz.Decision, code, msg string) authz.Decision {
	dec.Allow = false
	dec.ErrorCode = code
	dec.ErrorMessage = msg
	return dec
}

const subAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func randSubdomain(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = subAlphabet[int(b[i])%len(subAlphabet)]
	}
	if b[0] >= '0' && b[0] <= '9' {
		b[0] = 'r'
	}
	return string(b)
}
