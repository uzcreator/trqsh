# 06 — Web Dashboard & Cloud Request Inspector

**Owns:** `web/dashboard`
**Depends on:** Part 05 (REST/OpenAPI), Part 07 (billing UI embeds).
**Blocks:** nothing.

> Read `00-ARCHITECTURE.md` (§11, §12) and `05-control-api.md` first. This is the authenticated web
> control surface where users manage everything and where **revenue features** (upgrade, domains,
> teams) are exposed.

## Goal

A fast, clean dashboard for account/team management, tunnels, reserved subdomains, custom domains,
API keys, usage graphs, billing, and a **cloud request inspector/replay** — using the Part 05 API.

## Stack
- **Next.js 14+ (App Router) + TypeScript + Tailwind + shadcn/ui** (shared tokens with the GUI,
  Part 04, and the marketing site, Part 09). Data via the generated **TS client** from Part 05's
  OpenAPI. Auth via the Part 05 JWT session (httpOnly cookie).

## Scope / task breakdown

### T1 — App shell & auth (`web/dashboard/app/`)
- Login (redirect to Part 05 OAuth), session handling, protected layout, org switcher, nav.
- Global error mapping from §8 codes → friendly messages + upgrade CTAs.

### T2 — Tunnels (`app/tunnels/`)
- Live list of active tunnels (from `/tunnels`, backed by Redis): proto, public URL, local target,
  connected agent, region, live request/byte counters. Copy URL; link into the inspector.

### T3 — Subdomains & custom domains (`app/domains/`)
- Reserve/release subdomains (respecting plan limits). Add custom domain → show the exact DNS records
  to set (TXT/CNAME) → **Verify** button → status (pending/verified/cert-issued). Explain propagation.

### T4 — API keys (`app/keys/`)
- Create (show once, copy), list (name, prefix, last used), revoke. Usage instructions for CLI/GUI.

### T5 — Usage & analytics (`app/usage/`)
- Charts for bandwidth, requests, active tunnels over time (from `/usage`). Show plan limits and how
  close the org is (progress bars); warn near limits with an upgrade CTA. **Follow the `dataviz`
  design skill** for all charts (consistent, accessible, light/dark).

### T6 — Cloud request inspector / replay (`app/inspect/`)
- Consume a **capture stream** surfaced by the agent/edge (via Part 05: either a websocket relay or
  stored recent captures per plan retention, §11). Request list + detail (headers, timing, body) +
  **replay** to the local service (through the agent). This is the cloud twin of the local `:4040`
  inspector; retention follows the plan (1 h Free → 30 days Pro/Team).

### T7 — Team & billing (`app/team/`, `app/billing/`)
- Team: members, roles, invites, SSO settings (Team plan). Billing: **embed Part 07** — current plan,
  usage-based charges, upgrade/downgrade (Stripe Checkout), invoices (Stripe Customer Portal).

## Interfaces honored (do not modify)
- Part 05 REST/OpenAPI (§12) — treat as the only backend; don't reach into Postgres/Redis directly.
- Part 07 billing endpoints/components for the billing screens.
- Shared design tokens (Tailwind config) with Parts 04 and 09.

## Done criteria
- Login → see live tunnels; create/revoke an API key; reserve a subdomain; add + verify a custom
  domain (against a test DNS); usage charts render; inspector shows a captured request and replays it.
- Billing screens drive a Stripe **test-mode** upgrade and reflect the new plan/limits.
- Lighthouse: no major a11y/perf regressions; works in light + dark.

## Run / verify
```bash
cd web/dashboard
pnpm install && pnpm dev            # against a running Part 05 API (localhost:8080)
# smoke: login → create API key → use it with `trqsh` → tunnel appears live in the dashboard →
#        open Inspector → replay a request → open Billing → test-mode upgrade → limits change
```
