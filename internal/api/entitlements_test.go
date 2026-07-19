package api

import (
	"context"
	"testing"
	"time"

	"github.com/rift/rift/internal/api/auth"
	"github.com/rift/rift/internal/api/store"
	"github.com/rift/rift/pkg/authz"
	"github.com/rift/rift/pkg/proto"
)

func setupOrgKey(t *testing.T, plan string) (*Entitlements, store.Store, string, string) {
	t.Helper()
	st := store.NewMemStore()
	a := auth.New(st, "test-secret")
	ent := NewEntitlements(a, st, "lvh.me")
	org, err := st.CreateOrg(context.Background(), store.Org{Name: "t", Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	gen, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateAPIKey(context.Background(), store.APIKey{OrgID: org.ID, Prefix: gen.Prefix, Hash: gen.Hash}); err != nil {
		t.Fatal(err)
	}
	return ent, st, gen.Full, org.ID
}

func TestAuthenticate(t *testing.T) {
	ent, _, key, orgID := setupOrgKey(t, PlanFree)
	acct, plan, err := ent.Authenticate(context.Background(), key)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if acct != orgID || plan != PlanFree {
		t.Fatalf("got (%s,%s), want (%s,free)", acct, plan, orgID)
	}
	if _, _, err := ent.Authenticate(context.Background(), "rk_live_bogus_key"); err == nil {
		t.Fatal("expected error for bogus key")
	}
}

func TestCheckBindFreeDeniesUDP(t *testing.T) {
	ent, _, key, _ := setupOrgKey(t, PlanFree)
	dec, _ := ent.CheckBind(context.Background(), authz.BindRequest{APIKey: key, Type: "udp"})
	if dec.Allow {
		t.Fatal("free plan should deny UDP")
	}
	if dec.ErrorCode != proto.CodePlanForbids {
		t.Fatalf("code = %s, want %s", dec.ErrorCode, proto.CodePlanForbids)
	}
}

func TestCheckBindFreeDeniesUnreservedSubdomain(t *testing.T) {
	ent, _, key, _ := setupOrgKey(t, PlanFree)
	dec, _ := ent.CheckBind(context.Background(), authz.BindRequest{APIKey: key, Type: "http", Subdomain: "demo"})
	if dec.Allow {
		t.Fatal("should deny an unreserved specific subdomain")
	}
	if dec.ErrorCode != proto.CodeSubdomainForbidden {
		t.Fatalf("code = %s, want %s", dec.ErrorCode, proto.CodeSubdomainForbidden)
	}
}

func TestCheckBindAllowsReservedSubdomain(t *testing.T) {
	ent, st, key, orgID := setupOrgKey(t, PlanFree)
	if _, err := st.ReserveSubdomain(context.Background(), store.ReservedSubdomain{OrgID: orgID, Subdomain: "demo"}); err != nil {
		t.Fatal(err)
	}
	dec, _ := ent.CheckBind(context.Background(), authz.BindRequest{APIKey: key, Type: "http", Subdomain: "demo"})
	if !dec.Allow {
		t.Fatalf("reserved subdomain should be allowed: %s", dec.ErrorCode)
	}
}

func TestCheckBindAssignsRandomSubdomain(t *testing.T) {
	ent, _, key, _ := setupOrgKey(t, PlanFree)
	dec, _ := ent.CheckBind(context.Background(), authz.BindRequest{APIKey: key, Type: "http"})
	if !dec.Allow || dec.AssignedSubdomain == "" {
		t.Fatalf("expected allow + assigned subdomain, got allow=%v sub=%q", dec.Allow, dec.AssignedSubdomain)
	}
}

func TestCheckBindProAllowsUDP(t *testing.T) {
	ent, _, key, _ := setupOrgKey(t, PlanPro)
	dec, _ := ent.CheckBind(context.Background(), authz.BindRequest{APIKey: key, Type: "udp"})
	if !dec.Allow {
		t.Fatalf("pro plan should allow UDP: %s", dec.ErrorCode)
	}
	if !dec.Limits.AllowUDP {
		t.Fatal("pro limits should allow UDP")
	}
}

func TestCheckBindCustomDomainVerification(t *testing.T) {
	ent, st, key, orgID := setupOrgKey(t, PlanPro)
	// Unverified → denied.
	if _, err := st.AddCustomDomain(context.Background(), store.CustomDomain{OrgID: orgID, Domain: "app.example.com", VerifyToken: "t"}); err != nil {
		t.Fatal(err)
	}
	dec, _ := ent.CheckBind(context.Background(), authz.BindRequest{APIKey: key, Type: "http", CustomHost: "app.example.com"})
	if dec.Allow {
		t.Fatal("unverified custom domain should be denied")
	}
	if dec.ErrorCode != proto.CodeDomainUnverified {
		t.Fatalf("code = %s, want %s", dec.ErrorCode, proto.CodeDomainUnverified)
	}
	// Verify → allowed.
	d, _ := st.GetCustomDomainByName(context.Background(), "app.example.com")
	if err := st.VerifyCustomDomain(context.Background(), d.ID, orgID, time.Now()); err != nil {
		t.Fatal(err)
	}
	dec2, _ := ent.CheckBind(context.Background(), authz.BindRequest{APIKey: key, Type: "http", CustomHost: "app.example.com"})
	if !dec2.Allow {
		t.Fatalf("verified custom domain should be allowed: %s", dec2.ErrorCode)
	}
}

func TestReportUsage(t *testing.T) {
	ent, st, _, orgID := setupOrgKey(t, PlanFree)
	err := ent.ReportUsage(context.Background(), authz.Usage{AccountID: orgID, TunnelID: "t1", BytesIn: 100, BytesOut: 200, Requests: 3})
	if err != nil {
		t.Fatalf("ReportUsage: %v", err)
	}
	u, _ := st.UsageForOrg(context.Background(), orgID, time.Time{})
	if u.BytesIn != 100 || u.BytesOut != 200 || u.Requests != 3 {
		t.Fatalf("usage not recorded: %+v", u)
	}
}
