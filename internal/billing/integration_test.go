package billing_test

import (
	"context"
	"testing"
	"time"

	"github.com/trqsh-uz/trqsh/internal/api"
	"github.com/trqsh-uz/trqsh/internal/api/auth"
	"github.com/trqsh-uz/trqsh/internal/api/store"
	"github.com/trqsh-uz/trqsh/internal/billing"
	"github.com/trqsh-uz/trqsh/pkg/authz"
	"github.com/trqsh-uz/trqsh/pkg/proto"
)

// TestUpgradeLiftsEntitlements proves the Qadam 6 gate end to end through Part
// 05's CheckBind with Part 07 quota wired in: a Free org is denied UDP (plan
// gate) and denied once over its bandwidth quota (billing gate); after an
// upgrade to Pro both binds succeed.
func TestUpgradeLiftsEntitlements(t *testing.T) {
	st := store.NewMemStore()
	a := auth.New(st, "test-secret")
	ent := api.NewEntitlements(a, st, "lvh.me")
	ent.SetQuota(billing.New(st, nil, billing.DefaultConfig(), nil))

	ctx := context.Background()
	org, err := st.CreateOrg(ctx, store.Org{Name: "acme", Plan: billing.PlanFree})
	if err != nil {
		t.Fatal(err)
	}
	gen, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateAPIKey(ctx, store.APIKey{OrgID: org.ID, Prefix: gen.Prefix, Hash: gen.Hash}); err != nil {
		t.Fatal(err)
	}
	key := gen.Full

	// Free plan: UDP denied by the protocol gate.
	if dec, _ := ent.CheckBind(ctx, authz.BindRequest{APIKey: key, Type: "udp"}); dec.Allow || dec.ErrorCode != proto.CodePlanForbids {
		t.Fatalf("free UDP: allow=%v code=%s, want deny PLAN_FORBIDS", dec.Allow, dec.ErrorCode)
	}

	// Free plan under quota: HTTP allowed.
	if dec, _ := ent.CheckBind(ctx, authz.BindRequest{APIKey: key, Type: "http"}); !dec.Allow {
		t.Fatalf("free HTTP under quota should be allowed: %s", dec.ErrorCode)
	}

	// Push past the 10 GB Free cap -> HTTP now denied by the billing quota gate.
	_ = st.UpsertUsage(ctx, store.UsageRecord{
		OrgID: org.ID, BytesIn: 7 * billing.GB, BytesOut: 4 * billing.GB, WindowEnd: time.Now(),
	})
	if dec, _ := ent.CheckBind(ctx, authz.BindRequest{APIKey: key, Type: "http"}); dec.Allow || dec.ErrorCode != proto.CodeQuotaBandwidth {
		t.Fatalf("over-quota HTTP: allow=%v code=%s, want deny QUOTA_BANDWIDTH", dec.Allow, dec.ErrorCode)
	}

	// Upgrade to Pro (what a Stripe webhook does): UDP and the 11 GB of usage now fit.
	if err := st.UpdateOrgPlan(ctx, org.ID, billing.PlanPro); err != nil {
		t.Fatal(err)
	}
	if dec, _ := ent.CheckBind(ctx, authz.BindRequest{APIKey: key, Type: "udp"}); !dec.Allow {
		t.Fatalf("pro UDP should be allowed: %s", dec.ErrorCode)
	}
	if dec, _ := ent.CheckBind(ctx, authz.BindRequest{APIKey: key, Type: "http"}); !dec.Allow {
		t.Fatalf("pro HTTP (11 GB < 200 GB cap) should be allowed: %s", dec.ErrorCode)
	}
}
