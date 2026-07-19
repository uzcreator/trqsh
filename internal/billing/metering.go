package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rift/rift/internal/api/store"
	"github.com/rift/rift/internal/billing/stripe"
)

// CollectMeteredUsage rolls usage_records up into metered_usage push rows for
// every Pay-as-you-go org over [start, end). It is idempotent: re-running over
// the same window inserts nothing new (the store enforces uniqueness on
// org+metric+window_start). Returns the number of push rows created.
//
// Run this once per billing period close (e.g. daily/at month end) from a
// scheduler; usage itself flows in continuously via ReportUsage.
func (s *Service) CollectMeteredUsage(ctx context.Context, start, end time.Time) (int, error) {
	orgs, err := s.store.OrgsByPlan(ctx, PlanPAYG)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, org := range orgs {
		n, err := s.collectForOrg(ctx, org.ID, start, end)
		if err != nil {
			s.log.Error("metering: collect", "org", org.ID, "err", err)
			continue
		}
		created += n
	}
	return created, nil
}

func (s *Service) collectForOrg(ctx context.Context, orgID string, start, end time.Time) (int, error) {
	usage, err := s.store.UsageForOrg(ctx, orgID, start)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, m := range []struct {
		metric string
		qty    int64
	}{
		{MetricBandwidth, usage.BytesIn + usage.BytesOut},
		{MetricRequests, usage.Requests},
	} {
		if m.qty <= 0 {
			continue
		}
		_, err := s.store.InsertMeteredUsage(ctx, store.MeteredUsage{
			OrgID: orgID, Metric: m.metric, Quantity: m.qty, WindowStart: start, WindowEnd: end,
		})
		if errors.Is(err, store.ErrConflict) {
			continue // already collected for this window
		}
		if err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// FlushMeteredUsage pushes pending metered_usage rows to Stripe via the Billing
// Meters API and marks them reported. The row's stable identifier doubles as the
// Stripe idempotency key, so a retry after a partial failure never double-counts.
// Returns the number of rows successfully reported.
func (s *Service) FlushMeteredUsage(ctx context.Context) (int, error) {
	if !s.cfg.Enabled() || s.stripe == nil {
		return 0, ErrBillingDisabled
	}
	pending, err := s.store.PendingMeteredUsage(ctx, 500)
	if err != nil {
		return 0, err
	}
	var reported []int64
	for _, u := range pending {
		eventName := s.cfg.MeterEventName(u.Metric)
		if eventName == "" {
			continue // metric not wired to a Stripe meter; leave pending
		}
		org, err := s.store.GetOrg(ctx, u.OrgID)
		if err != nil || org.StripeCustomerID == "" {
			continue
		}
		if err := s.stripe.CreateMeterEvent(ctx, stripe.MeterEventParams{
			EventName:  eventName,
			CustomerID: org.StripeCustomerID,
			Value:      u.Quantity,
			Identifier: meterIdentifier(u),
			Timestamp:  u.WindowEnd,
		}); err != nil {
			s.log.Error("metering: push", "org", u.OrgID, "metric", u.Metric, "err", err)
			continue
		}
		reported = append(reported, u.ID)
	}
	if len(reported) > 0 {
		if err := s.store.MarkMeteredUsageReported(ctx, reported, time.Now()); err != nil {
			return len(reported), err
		}
	}
	return len(reported), nil
}

// meterIdentifier is the stable per-window idempotency key for a usage push.
func meterIdentifier(u store.MeteredUsage) string {
	return fmt.Sprintf("mu_%s_%s_%d", u.OrgID, u.Metric, u.WindowStart.Unix())
}

// RunMeteringLoop periodically collects the current period's metered usage and
// flushes it to Stripe until ctx is canceled. Start it from the API server when
// billing is enabled and an interval is configured.
func (s *Service) RunMeteringLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if _, err := s.CollectMeteredUsage(ctx, monthStart(now), now); err != nil {
				s.log.Error("metering: periodic collect", "err", err)
			}
			if n, err := s.FlushMeteredUsage(ctx); err != nil && !errors.Is(err, ErrBillingDisabled) {
				s.log.Error("metering: periodic flush", "err", err)
			} else if n > 0 {
				s.log.Info("metering: flushed usage to Stripe", "rows", n)
			}
		}
	}
}
