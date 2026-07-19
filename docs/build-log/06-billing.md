# Step 6 — Part 07: Billing & Monetization (`internal/billing`)

- **Date:** 2026-07-18
- **Step:** 6 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/07-billing-monetization.md`](../../plan/07-billing-monetization.md)
- **Milestone:** M2 — Monetizable product
- **Status:** ✅ Complete — build/vet/test green; **real binaries prove the Qadam 6 gate**: a signed
  Stripe webhook flips a Free org to Pro and the edge-facing entitlements RPC then allows a UDP bind
  it previously denied.

> **TL;DR (Uz):** Billing (`internal/billing`) yozildi — plan katalogi endi shu yerda (yagona manba,
> Stripe price ID'lari bilan), Stripe Checkout + Customer Portal, **imzo tekshiriladigan webhooklar**
> `orgs.plan` ni o'zgartiradi, metered usage yig'ish + Stripe'ga push, va **kvota majburlash**
> (`ERR_QUOTA_BANDWIDTH`) Part 05'ning `CheckBind`iga ulandi. Isbot: haqiqiy `riftapi` — Free UDP'ni
> rad etadi → imzolangan webhook Pro'ga o'tkazadi → UDP endi ruxsat; soxta imzo 400. Keyingi: Part 06 dashboard.

## What was built

`internal/billing` — monetization: the canonical plan catalog, Stripe integration, webhooks, metering,
and quota enforcement. It reuses Part 05's store and never imports `internal/api` (that would cycle).

```
internal/billing/plans.go          canonical §11 plan/quota catalog (Limits + display + Stripe price IDs)
internal/billing/config.go         env config: Stripe keys, price IDs, meter names, trial, dunning, metering
internal/billing/billing.go        Service: customer resolution, plan<-subscription mapping, metric consts
internal/billing/checkout.go       POST /billing/checkout, /billing/portal; GET /billing/subscription
internal/billing/webhooks.go       POST /billing/webhooks: sig-verify + idempotent event->plan state machine
internal/billing/metering.go       collect usage_records -> metered_usage (idempotent) -> push to Stripe
internal/billing/entitlements.go   CheckQuota(orgID, plan) -> §8 ERR_QUOTA_* (the Part 05 seam)
internal/billing/handlers.go       small JSON helpers + principal accessor (via internal/api/auth)
internal/billing/stripe/stripe.go  thin, dep-free Stripe client + API interface + webhook sig verify
internal/api/db/migrations/00002_billing.sql   subscriptions, billing_events, metered_usage (+orgs index)
```

Store additions (Part 05's `internal/api/store`, sanctioned by the spec — "reuse Part 05's DB layer"):
`Subscription`, `MeteredUsage` types; `UpsertSubscription`, `GetSubscriptionForOrg`, `DeleteSubscription`,
`GetOrgByStripeCustomer`, `OrgsByPlan`, `MarkEventProcessed`, `InsertMeteredUsage`, `PendingMeteredUsage`,
`MarkMeteredUsageReported` — implemented in both `MemStore` (tested) and `PostgresStore`.

## How it works

- **Single source of truth (T1):** the plan catalog moved to `internal/billing/plans.go`. `internal/api/plans.go`
  is now a thin alias (`type Plan = billing.Plan`, `Catalog = billing.Catalog`, `PlanFor`, `PlanFree`…),
  so all Part 05 handlers/tests keep resolving `api.Plan*` unchanged. `Plan` carries the frozen limits,
  display pricing (for Part 09), and a `StripePrices` field (populated from config at deploy time).
- **Checkout & Portal (T2):** `POST /v1/billing/checkout {plan,cadence}` resolves/creates the org's Stripe
  customer, looks up the configured price ID, and returns a Checkout Session URL (Pro gets a 14-day trial,
  promo codes allowed). `POST /v1/billing/portal` returns a Customer-Portal URL.
- **Webhooks (T3):** `POST /v1/billing/webhooks` is public but **HMAC-SHA256 signature-verified** against
  `RIFT_STRIPE_WEBHOOK_SECRET` (Stripe's `t=…,v1=…` scheme, constant-time compare, timestamp tolerance),
  **idempotent** (dedupe by event id in `billing_events`), and drives the state machine:
  `checkout.session.completed` links customer↔org; `customer.subscription.created/updated` upserts the
  subscription and sets `orgs.plan`; `deleted` → Free; `invoice.payment_failed` → `past_due` (plan retained
  during Stripe's Smart-Retry grace); `invoice.paid` → recover to active.
- **Metering (T4):** `CollectMeteredUsage` rolls `usage_records` up into `metered_usage` push rows for
  Pay-as-you-go orgs (idempotent per org+metric+window); `FlushMeteredUsage` pushes pending rows via the
  **Billing Meters API** (customer-keyed, the row id as the idempotency key) and marks them reported. An
  opt-in `RunMeteringLoop` (env `RIFT_BILLING_METERING_INTERVAL`) runs both on a ticker.
- **Quota enforcement (T5):** `Service.CheckQuota` compares current-month usage to plan limits and returns
  `ERR_QUOTA_BANDWIDTH`/`ERR_QUOTA_REQUESTS`. It is injected into Part 05's `Entitlements` via a new
  `QuotaChecker` seam (`ent.SetQuota`), so the edge's `CheckBind` now denies over-quota binds. **Fail-safe:**
  the plan is read from our own `orgs.plan` via the catalog (a Stripe/billing outage can never widen limits
  to unlimited), and a usage-read error fails *open* (never severs tunnels on a metering hiccup).

## Verification

| Check | Result |
|------|--------|
| `go build ./...` / `go vet ./...` | ✅ |
| `go test ./...` (all packages) | ✅ |
| `go test ./internal/billing/...` (stripe sig, catalog, quota+fail-safe, webhook lifecycle, metering) | ✅ |
| Stress `go test -count=20 ./internal/billing/...` | ✅ |
| Part 05's 12 api tests (after catalog alias + quota wiring) | ✅ still green |
| **Real binaries: `riftapi` — signed webhook flips plan; edge RPC enforces it** | ✅ (below) |

**Real-binary smoke (the Qadam 6 gate).** `riftapi` (billing enabled: `sk_test_smoke`, `whsec_smoke`,
`RIFT_STRIPE_PRICE_PRO_MONTHLY=price_pro_m`), driven over HTTP:

```
org: org_… plan: free
free udp  -> allow=false  ERR_PLAN_FORBIDS        # edge-facing /internal/entitlements/check-bind
webhook   -> 200 {"status":"ok"}                  # signed customer.subscription.created (Pro)
plan after webhook: pro
pro  udp  -> allow=true                           # same bind now permitted
pro  http -> allow=true (assigned subdomain)
bad-signature webhook -> 400                      # forged signature rejected, no state change
```

`-race` still cannot run here (no C compiler — see `02-protocol-transport.md`); `-count=20` stress used instead.

### Run locally (Stripe test mode)
```bash
export RIFT_STRIPE_SECRET_KEY=sk_test_…  RIFT_STRIPE_WEBHOOK_SECRET=whsec_…
export RIFT_STRIPE_PRICE_PRO_MONTHLY=price_…  RIFT_STRIPE_PRICE_PRO_ANNUAL=price_…
export RIFT_STRIPE_PRICE_TEAM_MONTHLY=price_…  RIFT_STRIPE_PRICE_PAYG_METERED=price_…
export RIFT_STRIPE_METER_BANDWIDTH=rift_bandwidth  RIFT_STRIPE_METER_REQUESTS=rift_requests
go run ./cmd/riftapi
stripe listen --forward-to localhost:8080/v1/billing/webhooks   # Stripe CLI test mode
# subscribe Free->Pro with test card 4242…; webhook flips orgs.plan; the edge then allows UDP/custom-domain.
```

## Key decisions

- **Thin Stripe client, not `stripe-go`.** The correctness-critical pieces (webhook signature verification,
  the event→plan state machine, idempotency, quota) are small and had to be **unit-testable offline** — the
  live API calls can't be exercised without a Stripe account regardless. A dependency-free client keeps the
  module lean and auditable; the `stripe.API` interface makes swapping in the official SDK a drop-in change.
  This is an internal implementation choice, not a frozen contract (§14).
- **Catalog owned by billing, aliased by api** — one source of truth for limits + pricing + price IDs, with
  zero churn to Part 05 call sites. No import cycle: `api → billing → api/store` (leaf); billing also imports
  `api/auth` (leaf) for the request principal, never `api`.
- **Billing Meters API for usage** (customer-keyed) avoids tracking Stripe subscription-item ids for the
  metered push.
- **Quota is always enforced**, even with Stripe disabled — the service is constructed unconditionally and
  `CheckQuota` reads the local plan/catalog. Stripe only gates Checkout/Portal/webhooks/meter-push.
- **Dunning grace = Stripe Smart Retries.** `invoice.payment_failed` records `past_due` and retains the plan;
  the eventual `customer.subscription.deleted` downgrades to Free. `RIFT_BILLING_DUNNING_GRACE` is reserved
  for an optional operator-run local reconciliation sweep.

## Known gaps / notes (for later parts)

- **Live Stripe calls** (Checkout/Portal session creation, meter-event push) are compiled + interface-tested
  with a fake, but not exercised against real Stripe here — needs test-mode keys + the Stripe CLI (Part 08 CI).
- **Postgres billing path** (migration `00002`, the new store methods incl. `= ANY($1)` array update) is
  compiled + schema-reviewed but not CI-exercised (no DB in this env); `MemStore` covers the tests.
- **Concurrent-tunnel quota** (`MaxConcurrentTunnels`) is passed to the edge in the Decision and enforced
  there against the Redis registry; billing enforces the *monthly metered* quotas (bandwidth/requests).
- **Team seats / proration / annual-vs-monthly upgrade math** rely on Stripe's proration; no custom logic added.
- **`orgs.plan` seeding for `plans` table**: the app reads the code catalog; the `plans` table remains reference.

## What's next

**Step 7 — Integration** is effectively already proven (edge → real entitlements, and now billing quota +
webhook plan-flips, demonstrated with real binaries). Next substantive part is **Step 8 — Part 06 Web
Dashboard** (`plan/06-web-dashboard.md`): it consumes this API + OpenAPI (billing screens embed Checkout/
Portal, usage charts, plan/subscription state).
