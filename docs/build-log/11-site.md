# Step 11 (LAST) — Part 09: Marketing Site, Docs & Onboarding (`web/site`, `docs/`)

- **Date:** 2026-07-19
- **Step:** 11 of 11 — final (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/09-website-docs.md`](../../plan/09-website-docs.md)
- **Milestone:** M3 — Launch
- **Status:** ✅ Complete. Site builds to **27 static/SSG pages** (`pnpm build` green), every
  done-criterion verified against the prerendered HTML, **and the whole product was run
  end-to-end** — a real QUIC tunnel plus the control API serving the same pricing catalog the site
  renders. All 11 steps are now done.

> **TL;DR (Uz):** Marketing sayt + docs tayyor va **butun mahsulot ishga tushirildi**. Next.js
> (App Router) sayt: landing (differensiatorlar + QUIC dataviz grafik), pricing (Part 07 katalogidan
> **generatsiya** qilingan — hardcode yo'q), download (Part 08 real релиз artefaktlari), va to'liq
> docs + OpenAPI'dan API reference (har bir §8 xato kodi anchor bilan). Brend tokenlari Parts 04/06
> bilan bir xil. Keyin **hammasi Docker'siz, sof Go bilan ishga tushdi**: edge + agent QUIC ustidan
> jonli tunnel (`https://demo.lvh.me → localhost:3000`, HTTP 200), real Prometheus metrikalari, va
> control API `/v1/plans` — saytdagi narxlar bilan **aynan mos**. trqsh endi to'liq launch-ready. 🚀

## What was built

`web/site/` (Next.js 15 App Router, Tailwind, shadcn-style UI) + `docs/content/` (13 Markdown guides).

```
web/site/
├── app/(marketing)/     landing, pricing, download, terms, privacy
├── app/docs/            shell + [slug] (Markdown), api (OpenAPI), errors (generated)
├── app/                 layout (static, theme pre-paint), sitemap, robots, icon, 404
├── components/          header, footer, pricing table, comparison, latency chart, downloads, docs nav
├── lib/
│   ├── catalog.generated.ts   GENERATED from internal/billing.Catalog (checked in)
│   ├── plans.ts / errors.ts / downloads.ts / site.ts
│   └── docs.ts + docs-nav.ts + openapi.ts   build-time content pipeline
└── scripts/genplans/    Go generator (billing.Catalog → catalog.generated.ts)
docs/content/*.md         quickstart, install, auth, http/tcp-udp, subdomains, custom-domains,
                          inspector, gui, configuration, webhooks-ci, self-hosting, security
```

## How it works (per track)

- **T1 Landing.** Hero (one-liner + CTAs + auto-detect install snippet + terminal mock), six
  differentiator cards (§1), a **QUIC speed** section with a pure-SVG latency chart following the
  dataviz reference palette (fixed series slots, `--grid`/`--baseline`, honestly labeled an
  *illustrative model of head-of-line blocking, not a measured benchmark*), an honest three-way
  comparison vs ngrok/Cloudflare Tunnel (competitors credited where they're genuinely strong), and a
  pricing teaser drawn from the catalog.
- **T2 Pricing.** Renders the **Part 07 catalog, never hardcoded** — see the key decision below.
  Monthly/annual toggle, three flat-tier cards + a metered PAYG strip, a full comparison matrix, and
  a FAQ. Prices/limits are exactly what the edge enforces.
- **T3 Download.** OS auto-detect; desktop-app bundles + CLI package-manager snippets (`brew`,
  `scoop`, `curl | sh`) + raw archive links. URLs resolve to the **real Part 08 release artifacts**
  (`trqsh_<version>_<os>_<arch>` archives, `Trqsh.app.zip`, `trqsh-gui.exe`) with a `checksums.txt`
  link and verify instructions.
- **T4 Docs.** 13 Markdown guides rendered at build (h2/h3 get stable slug ids + a TOC via a
  version-independent post-process). An **API reference generated from `docs/openapi.yaml`** at build
  (grouped by resource, auth vs public marked, params/responses). A generated **error reference**
  where **every §8 code has a stable anchor** (`/docs/errors#err_…`) the CLI/GUI/dashboard deep-link
  to. Plus a security/abuse policy.
- **T5 Onboarding.** "Start free" / "Log in" CTAs deep-link to the Part 06 dashboard (email + OAuth
  via Part 05); OAuth links target the control API directly. Lifecycle-email copy lives in the docs;
  wiring to a provider is a Part 05/07 hook (noted as a gap).

## Key decisions

- **Pricing is generated from the Go catalog, not fetched or hardcoded.** `GET /v1/plans` is behind
  auth (a public marketing page can't call it), and editing Part 05/07 to expose it would cross
  directory ownership. Since `internal/billing.Catalog` is documented as "the single source of truth
  consumed by Part 09 pricing," a tiny Go generator (`web/site/scripts/genplans`) marshals it to a
  checked-in `catalog.generated.ts`. This can't drift (CI regenerates and fails on any diff), needs
  no running API, and renders statically for speed. **Verified at runtime**: the live API's
  `/v1/plans` returns byte-identical numbers.
- **Static-first, no cookies.** Theme is applied by a pre-paint script from `localStorage` +
  `prefers-color-scheme` (not a server cookie), so every page stays statically renderable — good for
  Lighthouse and cacheability. Runs on **:3002** to coexist with the dashboard (:3000).
- **Docs pipeline is dependency-light** (`marked` + a hand-rolled slugger; `js-yaml` for OpenAPI) —
  no heavy MDX/content framework, keeping First Load JS ~105–120 kB.
- **Client/server split** (`lib/docs-nav.ts` vs `lib/docs.ts`) after a real build error: the sidebar
  is a client component and must not pull `node:fs` into the browser bundle.

## Contracts honored (no drift)

- **Pricing** = `internal/billing.Catalog` (§11), verified equal to live `/v1/plans`.
- **Error anchors** = the frozen §8 taxonomy — all 15 codes present.
- **API reference** = the canonical `docs/openapi.yaml` (Part 05), read at build.
- **Download links** = real Part 08 goreleaser/`release.yml` artifact names.
- **Brand tokens** = the same RGB-channel CSS vars as Parts 04/06.

## Verification

| Check | Result |
|------|--------|
| `pnpm build` | ✅ 27 pages, all ○ Static / ● SSG; First Load JS ~105–120 kB |
| Pricing matches Part 07 | ✅ prerendered `/pricing` shows `$8`, `$20`, `200 GB`, "Most popular" |
| Every §8 error code has an anchor | ✅ 15 `id="err_…"` anchors in `/docs/errors` |
| API reference renders from OpenAPI | ✅ `/docs/api` contains `/api-keys`, `/subdomains`, `/billing/checkout`, title |
| Site serves in production (`next start`) | ✅ `/`, `/pricing`, `/download`, `/docs/*`, `/sitemap.xml`, `/robots.txt` → 200 |
| Root `go build ./...` (generator added to module) | ✅ green; `go vet ./web/site/scripts/genplans` clean |

### End-to-end runtime ("run everything") — pure Go, no Docker/Postgres/Redis

Both the edge (in-memory registry when `TRQSH_REDIS_URL` is empty) and the API (in-memory store when
`TRQSH_DATABASE_URL` is empty) run standalone, so the whole product was exercised live:

| Component | Result |
|---|---|
| Edge `trqshd` (stub entitlements, lvh.me) | ✅ agent listener :4443 (QUIC+TCP), ingress :8088/:8443, `/healthz`+`/readyz` 200 |
| Agent `trqsh http 3000 --insecure` | ✅ connected over **QUIC**, `tunnel online https://demo.lvh.me` |
| **Public request through the edge** | ✅ `curl -H 'Host: demo.lvh.me' http://127.0.0.1:8088/…` → **HTTP 200**, local body returned |
| Local inspector (:4040) | ✅ serving |
| Edge Prometheus metrics (:9099) | ✅ `trqsh_agent_handshakes_total{kind="quic"}`, `trqsh_sessions_active 1`, `trqsh_tunnels_active 1`, `trqsh_http_requests_total 4` |
| Control API `trqshapi` (in-memory) | ✅ `store=memory`, `/healthz` 200, signup → JWT |
| `GET /v1/plans` vs site pricing | ✅ **identical** (free 10 GB $0, pro 200 GB $8/$80, team 1 TB $20/$200) |

## Cross-part touches (additive, documented)

- `.github/workflows/ci.yml`: added `web/site` to the **frontends** build matrix, and a step in the
  **go** job that regenerates `catalog.generated.ts` and fails on drift.
- `Makefile`: `site`, `site-build`, `site-plans` targets.

These are the minimal integration needed to verify Part 09 the same way the other frontends are
verified; they add to existing lists without changing other parts' behavior.

## Known gaps / notes

- **Lighthouse not run here** (no headless Chrome in this box). The build is static-first with tiny
  JS and no blocking fonts/images, which is what the SEO/perf/a11y budget targets; run
  `npx lighthouse http://localhost:3002` in CI/locally to score it.
- **Lifecycle emails** (welcome/activation/near-limit) are copy + integration points; sending routes
  through the Part 05/07 email hook, not yet wired to a provider.
- **Not runnable in this box** (unchanged from Part 08): Docker Compose full stack, Helm, Terraform,
  and the native Wails GUI build — all exercised in CI.

## What's next

Nothing in the build plan — **all 11 steps are complete**. trqsh is a launch-ready SaaS: CLI + GUI
(3 OSes), edge, control plane, billing, dashboard, and now the marketing site + docs. Go-live is
operational: point DNS at the edge, set the deploy-time env (`TRQSH_SITE_URL`, `TRQSH_DASHBOARD_URL`,
`TRQSH_LATEST_VERSION`, …), flip Stripe to live mode, and ship the first tagged release.
