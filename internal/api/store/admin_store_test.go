package store

import (
	"context"
	"testing"
	"time"
)

func TestAdminOverviewAndListings(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()

	// Two accounts: one pro org, one free org.
	ua, _ := m.CreateUser(ctx, User{Email: "alice@example.com", Name: "Alice"})
	ub, _ := m.CreateUser(ctx, User{Email: "bob@example.com", Name: "Bob"})
	oa, _ := m.CreateOrg(ctx, Org{Name: "Alice", Plan: "pro"})
	ob, _ := m.CreateOrg(ctx, Org{Name: "Bob", Plan: "free"})
	_ = m.AddOrgMember(ctx, OrgMember{OrgID: oa.ID, UserID: ua.ID, Role: "owner"})
	_ = m.AddOrgMember(ctx, OrgMember{OrgID: ob.ID, UserID: ub.ID, Role: "owner"})
	_, _ = m.CreateAPIKey(ctx, APIKey{OrgID: oa.ID, Name: "k", Prefix: "tq_p1", Hash: "h"})
	_, _ = m.ReserveSubdomain(ctx, ReservedSubdomain{OrgID: oa.ID, Subdomain: "alice"})
	_, _ = m.AddCustomDomain(ctx, CustomDomain{OrgID: oa.ID, Domain: "alice.dev", VerifyToken: "t"})
	_ = m.UpsertUsage(ctx, UsageRecord{OrgID: oa.ID, BytesIn: 100, BytesOut: 200, Requests: 3, WindowEnd: time.Now()})
	_, _ = m.RecordTunnelOpen(ctx, TunnelSession{OrgID: oa.ID, EdgeID: "e", SessionID: "s", TunnelID: "t", Type: "http", Country: "US"})

	st, err := m.AdminOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Users != 2 || st.Orgs != 2 {
		t.Fatalf("users/orgs = %d/%d, want 2/2", st.Users, st.Orgs)
	}
	if st.OrgsByPlan["pro"] != 1 || st.OrgsByPlan["free"] != 1 {
		t.Fatalf("orgs by plan = %+v", st.OrgsByPlan)
	}
	if st.APIKeys != 1 || st.ReservedSubdomains != 1 || st.CustomDomains != 1 {
		t.Fatalf("resource counts wrong: %+v", st)
	}
	if st.ActiveTunnels != 1 || st.TotalTunnels != 1 {
		t.Fatalf("tunnel counts = %d/%d", st.ActiveTunnels, st.TotalTunnels)
	}
	if st.BytesIn30d != 100 || st.Requests30d != 3 {
		t.Fatalf("usage = in %d req %d", st.BytesIn30d, st.Requests30d)
	}
	if st.NewUsers7d != 2 {
		t.Fatalf("new users 7d = %d, want 2", st.NewUsers7d)
	}

	// Directory listings: search + plan filter.
	users, _ := m.ListUsers(ctx, 10, 0, "alice")
	if len(users) != 1 || users[0].Email != "alice@example.com" {
		t.Fatalf("search users = %+v", users)
	}
	proOrgs, _ := m.ListOrgs(ctx, 10, 0, "pro")
	if len(proOrgs) != 1 || proOrgs[0].Plan != "pro" {
		t.Fatalf("plan-filtered orgs = %+v", proOrgs)
	}
	allOrgs, _ := m.ListOrgs(ctx, 10, 0, "")
	if len(allOrgs) != 2 {
		t.Fatalf("all orgs = %d, want 2", len(allOrgs))
	}
}
