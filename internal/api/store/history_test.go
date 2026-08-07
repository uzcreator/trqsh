package store

import (
	"context"
	"testing"
	"time"
)

func TestTunnelSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()

	open := func(org, edge, sess, tun, country string) {
		if _, err := m.RecordTunnelOpen(ctx, TunnelSession{
			OrgID: org, EdgeID: edge, SessionID: sess, TunnelID: tun,
			Type: "http", PublicURL: "https://" + tun + ".trqsh.uz", Region: "eu", Country: country,
		}); err != nil {
			t.Fatalf("open: %v", err)
		}
	}

	open("org_a", "edge1", "s1", "t1", "US")
	open("org_a", "edge1", "s1", "t2", "US")
	open("org_b", "edge1", "s2", "t1", "DE")

	// Idempotent open: same instance twice stays one active row.
	open("org_a", "edge1", "s1", "t1", "US")

	active, err := m.CountTunnelSessions(ctx, "org_a", true)
	if err != nil {
		t.Fatal(err)
	}
	if active != 2 {
		t.Fatalf("org_a active = %d, want 2", active)
	}

	// Close one; it should leave the active set but remain in history.
	if err := m.CloseTunnelSession(ctx, "edge1", "s1", "t1", time.Now(), 100, 200, 5); err != nil {
		t.Fatalf("close: %v", err)
	}
	if active, _ := m.CountTunnelSessions(ctx, "org_a", true); active != 1 {
		t.Fatalf("org_a active after close = %d, want 1", active)
	}
	if total, _ := m.CountTunnelSessions(ctx, "org_a", false); total != 2 {
		t.Fatalf("org_a total = %d, want 2", total)
	}

	// Org scoping: org_a history excludes org_b.
	list, err := m.ListTunnelSessions(ctx, TunnelSessionFilter{OrgID: "org_a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("org_a list = %d, want 2", len(list))
	}
	// Newest first: t1 (closed, re-touched last via idempotent open then close).
	var closed *TunnelSession
	for i := range list {
		if list[i].TunnelID == "t1" {
			closed = &list[i]
		}
	}
	if closed == nil || closed.Status != "closed" || closed.EndedAt == nil || closed.BytesOut != 200 {
		t.Fatalf("closed session wrong: %+v", closed)
	}

	// Admin (all-orgs) view + geo breakdown.
	all, _ := m.ListTunnelSessions(ctx, TunnelSessionFilter{})
	if len(all) != 3 {
		t.Fatalf("all sessions = %d, want 3", len(all))
	}
	breakdown, _ := m.TunnelCountryBreakdown(ctx, "", false)
	if breakdown["US"] != 2 || breakdown["DE"] != 1 {
		t.Fatalf("breakdown = %+v", breakdown)
	}
}

func TestUsageSeries(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	// Two windows same day, one the next day.
	_ = m.UpsertUsage(ctx, UsageRecord{OrgID: "o", BytesIn: 10, Requests: 1, WindowEnd: base})
	_ = m.UpsertUsage(ctx, UsageRecord{OrgID: "o", BytesIn: 5, Requests: 2, WindowEnd: base.Add(2 * time.Hour)})
	_ = m.UpsertUsage(ctx, UsageRecord{OrgID: "o", BytesIn: 7, Requests: 1, WindowEnd: base.AddDate(0, 0, 1)})

	daily, err := m.UsageSeriesForOrg(ctx, "o", base.AddDate(0, 0, -1), "day")
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 2 {
		t.Fatalf("daily buckets = %d, want 2", len(daily))
	}
	if daily[0].BytesIn != 15 || daily[0].Requests != 3 {
		t.Fatalf("day 1 bucket = %+v, want in=15 req=3", daily[0])
	}
	hourly, _ := m.UsageSeriesForOrg(ctx, "o", base.AddDate(0, 0, -1), "hour")
	if len(hourly) != 3 {
		t.Fatalf("hourly buckets = %d, want 3", len(hourly))
	}
}
