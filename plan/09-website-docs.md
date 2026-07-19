# 09 — Marketing Site, Docs & Onboarding

**Owns:** `web/site`, `docs/`
**Depends on:** Part 05 (signup/API), Part 07 (pricing catalog). Can start early on branding.
**Blocks:** nothing; it is the **acquisition + activation funnel** (top of the revenue funnel).

> Read `00-ARCHITECTURE.md` (§1 differentiators, §11 plans). This part is how developers discover,
> understand, install, and start paying for Rift.

## Goal

A fast marketing site + docs that convert: landing (differentiators), pricing (mirrors Part 07),
download (per-OS installers), a 60-second quickstart, and complete docs incl. an API reference.

## Stack
- **Next.js (App Router) + Tailwind + shadcn/ui** (shared brand tokens with Parts 04/06). Docs via
  a Markdown/MDX content pipeline (e.g. Nextra/Contentlayer). Strong SEO + performance (static where
  possible). Analytics + consent.

## Scope / task breakdown

### T1 — Landing page (`web/site/app/(marketing)/`)
- Hero with the one-line promise + primary CTA (Start free). Sections for each differentiator (§1):
  **QUIC speed** (with a latency/benchmark visual — follow the `dataviz` skill), **generous free
  tier**, **desktop GUI**, **UDP + all protocols**, **custom domains/teams**, **open-source agent**.
- Social proof, comparison table vs ngrok/Cloudflare Tunnel (honest), footer, legal (ToS/Privacy).

### T2 — Pricing page (`web/site/app/pricing/`)
- Render the **Part 07 plan catalog** (§11) — never hardcode; import/fetch the canonical catalog so
  price/limits never drift from billing. Monthly/annual toggle, feature matrix, FAQ, CTA → signup.

### T3 — Download page (`web/site/app/download/`)
- Detect OS; show installers (macOS `.dmg`, Windows `.exe/.msi`, Linux AppImage/`.deb`) + CLI
  install snippets (`brew install rift`, `winget install rift`, `curl -fsSL https://rift.dev/install.sh | sh`).
  Pull artifact URLs from the Part 08 release feed. Show checksums/signatures.

### T4 — Docs (`docs/` + `web/site/app/docs/`)
- Quickstart (install → `rift login` → `rift http 3000` → live URL in under a minute).
- Guides: HTTP/TCP/UDP tunnels, reserved subdomains, custom domains (DNS setup), basic-auth, the
  inspector, the GUI, config file (§10), CI/webhook use cases, self-host notes (open-source agent).
- **API reference** generated from Part 05's OpenAPI. Troubleshooting mapped to §8 error codes
  (each code → a docs anchor the CLI/GUI/dashboard deep-link to). Security/abuse policy.

### T5 — Onboarding / signup funnel
- Signup → Part 05 OAuth → dashboard (Part 06) with a guided "create your first tunnel" flow (show
  the exact `rift http` command with the user's key). Instrument activation events for the funnel.
- Lifecycle email hooks (welcome, activation nudge, near-limit upgrade) — templates here, sending via
  the Part 05/07 email hook.

## Interfaces honored (do not modify)
- Part 07 plan catalog for pricing (single source of truth). Part 05 OpenAPI for the API reference
  and signup. Part 08 release feed for download links. Shared brand tokens with Parts 04/06.

## Done criteria
- Landing, pricing, download, and docs render fast (good Lighthouse SEO/perf/a11y), light + dark.
- Pricing exactly matches Part 07; changing a plan in billing reflects on the site without a code edit.
- Download links resolve to real (Part 08) artifacts with checksums; install snippets work per OS.
- Quickstart path verified end-to-end: a new visitor can sign up and get a live tunnel in ~1 minute.
- API reference matches the live Part 05 OpenAPI; every §8 error code has a docs anchor.

## Run / verify
```bash
cd web/site
pnpm install && pnpm dev
# checks: pricing matches Part 07 catalog; /download shows correct OS installer; docs quickstart
# works against a real signup; API reference renders from docs/openapi.yaml
pnpm build && npx lighthouse http://localhost:3000 --view    # SEO/perf/a11y budget
```
