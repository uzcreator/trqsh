package billing

import (
	"context"
	"time"

	"github.com/rift/rift/pkg/proto"
)

// CheckQuota compares an org's current-period usage against its plan limits and
// returns a §8 error code + message when a metered quota is exceeded, or ("","")
// when the bind is within quota. It satisfies the api.QuotaChecker seam that
// Part 05's CheckBind consults.
//
// Fail-safe: the plan is resolved from our own orgs.plan via the catalog, so a
// billing/Stripe outage can never widen limits to "unlimited". A usage-lookup
// error allows the bind (we will not sever a customer's tunnels because a
// metering read hiccupped) — the protocol/domain/subdomain gates still apply and
// overage is reconciled out-of-band.
func (s *Service) CheckQuota(ctx context.Context, orgID, planCode string) (code, message string) {
	plan := PlanFor(planCode)
	if plan.Metered() {
		return "", "" // Pay-as-you-go bills usage; no hard bind-time quota.
	}
	limits := plan.Limits()
	if limits.MaxBandwidthBytesMo <= 0 && limits.MaxRequestsMo <= 0 {
		return "", "" // nothing metered to enforce at bind time
	}

	usage, err := s.store.UsageForOrg(ctx, orgID, monthStart(time.Now()))
	if err != nil {
		s.log.Warn("quota: usage lookup failed; allowing bind (fail-safe)", "org", orgID, "err", err)
		return "", ""
	}
	if limits.MaxBandwidthBytesMo > 0 && usage.BytesIn+usage.BytesOut >= limits.MaxBandwidthBytesMo {
		return proto.CodeQuotaBandwidth, "monthly bandwidth quota exceeded for the " + plan.Name + " plan; upgrade to raise it"
	}
	if limits.MaxRequestsMo > 0 && usage.Requests >= limits.MaxRequestsMo {
		return proto.CodeQuotaRequests, "monthly request quota exceeded for the " + plan.Name + " plan; upgrade to raise it"
	}
	return "", ""
}

// monthStart returns the first instant of t's calendar month (UTC).
func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
