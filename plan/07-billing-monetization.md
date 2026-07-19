# 07 — Billing & Monetization

**Owns:** `internal/billing`
**Depends on:** Part 05 (accounts/orgs, entitlements). **Blocks:** nothing directly, but **gates revenue**
and provides the quota numbers the edge enforces.

> Read `00-ARCHITECTURE.md` (§9 entitlements, §11 plan catalog) and `05-control-api.md`. This part
> turns usage into money: Stripe subscriptions + metered usage, and it feeds real limits into the
> `authz.Entitlements` decisions so the edge (Part 02) actually enforces plans.

## Goal

1. **Stripe** integration: Checkout (subscribe/upgrade), Customer Portal (manage/cancel/invoices),
   webhooks (source of truth for subscription state).
2. **Plan catalog** (Free/Pro/Team/Pay-as-you-go) with concrete limits (§11) and Stripe price IDs.
3. **Metered usage**: ingest `authz.Usage` from edges → aggregate → report metered items to Stripe.
4. **Quota enforcement**: expose current entitlements so Part 05's `CheckBind` denies/limits correctly.

## Stack
- Go + `github.com/stripe/stripe-go`. Postgres tables live alongside Part 05 (`orgs.stripe_customer_id`,
  new `subscriptions`, `invoices_cache`, `metered_usage`). Reuse Part 05's DB layer.

## Scope / task breakdown

### T1 — Plan catalog (`internal/billing/plans.go`)
- Encode the §11 catalog as data: `plan code → { Limits (authz.Limits), StripePriceIDs, display }`.
- Single source of truth consumed by Part 05 entitlements, Part 06 billing UI, Part 09 pricing page.
- Support monthly/annual prices + usage (metered) price IDs for Pay-as-you-go.

### T2 — Checkout & Portal (`internal/billing/checkout.go`)
- `POST /billing/checkout` → create a Stripe Checkout Session for a target plan (upgrade/downgrade),
  bound to the org's `stripe_customer_id` (create customer on first use).
- `POST /billing/portal` → Customer Portal session (manage payment method, cancel, invoices).
- Trials (e.g. 14-day Pro trial), proration on plan changes.

### T3 — Webhooks (`internal/billing/webhooks.go`)
- Verify Stripe **webhook signatures**. Handle `checkout.session.completed`,
  `customer.subscription.created/updated/deleted`, `invoice.paid`, `invoice.payment_failed`.
- On each, update `subscriptions` + set `orgs.plan` → this is what flips a customer's entitlements.
- **Dunning**: on payment failure, grace period → downgrade to Free; notify (email hook).

### T4 — Metering (`internal/billing/metering.go`)
- Receive aggregated `authz.Usage` (bandwidth, requests) from Part 05's `ReportUsage` pipeline; store
  in `metered_usage`. Periodically push usage records to Stripe metered prices for Pay-as-you-go and
  for overage on metered plans. Idempotent (dedupe by window).

### T5 — Entitlement wiring (`internal/billing/entitlements.go`)
- Provide `func LimitsForPlan(plan string) authz.Limits` and current-period usage lookups so Part
  05's `CheckBind` can compare live usage against limits and return `ERR_QUOTA_*` (§8) when exceeded.
- Ensure enforcement is **fail-safe**: if billing is unreachable, fall back to the org's last-known
  plan (cached), never to unlimited.

## Interfaces honored / provided
- Consumes accounts/orgs from Part 05; writes to the shared DB. Provides `LimitsForPlan` + usage
  lookups to Part 05 entitlements. Provides billing HTTP endpoints/components to Part 06.
- Plan catalog (§11) is the canonical pricing referenced by Parts 05, 06, 09.

## Done criteria
- Stripe **test mode**: subscribe Free→Pro via Checkout; webhook flips `orgs.plan`; the edge now
  allows a Pro-only bind (e.g. custom domain / UDP) that Free denied.
- Downgrade/cancel via Portal reflects back (plan → Free, limits tighten at next bind).
- Metered usage for a Pay-as-you-go org appears as Stripe usage records; overage enforced.
- Payment-failure dunning path downgrades after the grace window.

## Run / verify
```bash
stripe listen --forward-to localhost:8080/v1/billing/webhooks   # Stripe CLI test mode
# create Pro checkout (test card 4242...), confirm webhook flips plan, then:
#   - bind a UDP/custom-domain tunnel with the agent → allowed (was ERR_PLAN_FORBIDS on Free)
#   - exceed a metered limit in a test window → ERR_QUOTA_* on next bind
```
