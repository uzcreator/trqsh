# Step 8 — Part 06: Web Dashboard & Cloud Inspector (`web/dashboard`)

- **Date:** 2026-07-18
- **Step:** 8 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/06-web-dashboard.md`](../../plan/06-web-dashboard.md)
- **Milestone:** M2 — Monetizable product
- **Status:** ✅ Complete — `pnpm build` green (all 13 routes type-check + compile); **the app runs and
  renders real data** end-to-end against a live `trqshapi` (auth guard + Server-Component reads + billing).

> **TL;DR (Uz):** Birinchi frontend qism — Next.js 15 (App Router) + TS + Tailwind dashboard. Login
> (httpOnly JWT cookie), himoyalangan sahifalar: Overview, Tunnels, Domains (subdomain+custom, DNS+verify),
> API Keys (bir marta ko'rsatiladi, revoke), Usage (dataviz meter/bar), **Billing (Stripe Checkout/Portal
> embed)**, Inspector (capture-stream kutmoqda — hujjatlangan gap), Team, Settings. Token brauzerga
> chiqmaydi: o'qish RSC'da, yozish Server Action'da. `pnpm build` yashil; haqiqiy `trqshapi` bilan ishlab,
> real ma'lumot render qildi. Keyingi: Part 04 GUI yoki Part 08 infra.

## What was built

`web/dashboard` — the authenticated control surface, talking only to the Part 05 API (`docs/openapi.yaml`).

```
app/layout.tsx                  root layout + theme (cookie-driven, no-flash), globals
app/login/                      public login: email dev flow (Server Action) + OAuth links
app/(dashboard)/layout.tsx      protected shell: sidebar nav + topbar (org/plan, theme, logout)
app/(dashboard)/page.tsx        Overview: KPI tiles, bandwidth meter, quick-start, quick links
app/(dashboard)/tunnels/        live tunnels table (from /tunnels) + empty state
app/(dashboard)/domains/        reserve/release subdomains; add/verify custom domains (+DNS records)
app/(dashboard)/keys/           create (shown once) / list / revoke API keys
app/(dashboard)/usage/          dataviz: bandwidth meter, in/out bar, plan-limits, near-limit CTA
app/(dashboard)/billing/        plan cards -> Stripe Checkout; "Manage billing" -> Customer Portal
app/(dashboard)/inspect/        cloud inspector shell (awaits the capture stream — documented gap)
app/(dashboard)/team/           org member (owner) + Team-plan upsell
app/(dashboard)/settings/       account + org details + logout
lib/api.ts                      typed control-API client (+ transparent 401 refresh)
lib/session.ts                  httpOnly cookie session; lib/errors.ts §8 code -> friendly copy
components/ui/*                  shadcn-style primitives (button, card, input, badge, table, …)
components/{nav,topbar,stat-tile,hbar,…}   app chrome + dataviz tiles/bar
middleware.ts                   route protection
```

**Stack:** Next.js 15.1 (App Router) + React 19 + TypeScript 5.7 + Tailwind 3.4. Lean deps only
(`clsx`, `tailwind-merge`, `class-variance-authority`, `lucide-react`) — no Radix/Recharts.

## How it works

- **Auth (T1):** login posts to Part 05 via a Server Action that stores the JWT access/refresh in
  **httpOnly cookies**; `middleware.ts` redirects unauthenticated requests to `/login` (and vice-versa).
  **The token never reaches browser JS** — reads run in Server Components through `lib/api.ts`, writes run
  in Server Actions. On a 401 the client refreshes and retries (persisting new cookies in actions;
  in-memory for the current render otherwise). §8 error codes map to friendly copy + upgrade CTAs.
- **Resources (T2–T4):** tunnels list (from `/tunnels`), reserve/release subdomains, add custom domains
  with the exact TXT/CNAME records + a Verify button, and API-key create (plaintext shown once) / revoke.
- **Usage (T5):** honest dataviz for the aggregate the API exposes — a bandwidth **meter** toward the plan
  cap (status colors at 80/100%), an in/out **categorical bar** (fixed-order series colors, direct-labeled,
  legend), and a plan-limits panel. Per-day time series is deferred to a windowed usage endpoint.
- **Billing (T7):** the billing screen **embeds Part 07** — plan cards (monthly/annual) open **Stripe
  Checkout** and "Manage billing" opens the **Customer Portal**, both via Server Actions that redirect to
  Stripe. It reads `/billing/subscription` for the live plan/status and disables checkout when Stripe is off.
- **Design:** tokens follow the **dataviz reference palette** (light + dark), stored as RGB-channel CSS
  variables so Tailwind opacity utilities work. This token set is the shared source of truth for Parts 04/09.

## Verification

| Check | Result |
|------|--------|
| `pnpm build` (13 routes: compile + type-check + lint) | ✅ green |
| Runtime: `next start` + live `trqshapi` | ✅ |
| `/login` unauthenticated | ✅ 200 |
| `/` without cookie → middleware redirect | ✅ 307 → `/login` |
| `/` with real JWT cookie → Overview renders (RSC fetched `/account`, usage, …) | ✅ 200, "Overview"+"Quick start" |
| `/keys` with cookie → renders keys manager | ✅ 200, "Create an API key" |
| `/billing` with cookie → renders plan cards from `/plans` + `/billing/subscription` | ✅ 200, "Pro" |

Smoke: signed up via `trqshapi`, set the `trqsh_access` cookie, and drove the pages over HTTP — the
Server Components authenticated to the control API and rendered real account/usage/plan data.

## Key decisions

- **Token stays server-side** (httpOnly cookies + RSC reads + Server-Action writes) — no bearer token in
  browser JS; middleware guards routes; a 401 self-heals via refresh. Simpler and safer than a client store.
- **Lean, hand-written UI kit** (shadcn-style, cva + Tailwind) instead of the shadcn CLI / Radix — keeps the
  dependency surface small and the build reliable, with the same look and full dark-mode support.
- **Typed hand-written API client** rather than an OpenAPI generator — the surface is small and known, and a
  readable client avoids a codegen step; it stays in lockstep with `docs/openapi.yaml`.
- **Honest dataviz** — with only an aggregate from `/usage`, the right forms are meter tiles + a direct-labeled
  2-series bar (per the dataviz skill: "sometimes the answer is not a time-series chart"), not fabricated trends.

## Known gaps / notes (for later parts)

- **Cloud inspector (T6):** the request-capture stream (websocket relay / stored captures per retention) is a
  follow-up on the edge + control API; the page ships the two-pane shell + retention and points at the local
  `:4040` inspector, which works today.
- **OAuth login:** buttons link to the API's `/auth/oauth/{provider}`; completing the dashboard cookie session
  needs the Part 05 callback to redirect back with tokens (today it returns JSON). Email dev flow works E2E.
- **Team management (T7):** invites/roles/SSO await control-API team endpoints (the data model already has
  org members + roles). The screen shows the owner + a Team-plan upsell.
- **Org switcher:** a token is scoped to one org at login; multi-org switching needs a Part 05 re-scope endpoint.
- **Per-day usage charts:** await a windowed `/usage` endpoint; today's aggregate is shown as meters/bars.

## What's next

Two M2 tracks remain: **Part 04 — Desktop GUI** (Wails v3, embeds the Part 03 agent core; shares these
design tokens) and **Part 08 — Infra/Deploy**. Per `EXECUTION-ORDER.md` the next step is **Qadam 9 — Part 04
(GUI)**; Part 08 (infra) and Part 09 (site) can run in parallel and also consume this token set + OpenAPI.
