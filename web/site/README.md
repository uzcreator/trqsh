# web/site — Marketing site & docs (Part 09)

The acquisition + activation funnel: landing, pricing, download, and full docs
including an API reference. Next.js (App Router) + Tailwind + shadcn-style UI,
sharing the brand tokens in `app/globals.css` with Parts 04 (GUI) and 06 (dashboard).

## Run

```bash
pnpm install
pnpm dev        # http://localhost:3002
pnpm build      # static production build
pnpm typecheck
```

Runs on **:3002** so it can coexist with the dashboard (:3000) locally.

## Structure

```
app/
  (marketing)/        landing, pricing, download, terms, privacy
  docs/               docs shell, [slug] (Markdown), api (OpenAPI), errors
  sitemap.ts robots.ts icon.svg
components/            header, footer, pricing table, comparison, latency chart, …
lib/
  catalog.generated.ts   GENERATED from internal/billing.Catalog (do not edit)
  plans.ts               pricing shaped from the generated catalog
  errors.ts              §8 error taxonomy → /docs/errors anchors
  docs.ts openapi.ts     build-time Markdown + OpenAPI pipeline
scripts/genplans/        Go generator for catalog.generated.ts
../../docs/content/      Markdown source for the docs pages
```

## Content sources (never hardcoded)

- **Pricing** comes from `internal/billing.Catalog` via a generated snapshot, so
  prices/limits can't drift from what the edge enforces. Regenerate after changing
  the catalog:

  ```bash
  make site-plans      # or: go run ./web/site/scripts/genplans
  ```

- **API reference** renders from the canonical `docs/openapi.yaml` (Part 05) at
  build time.
- **Download links** resolve to the real Part 08 release artifacts; version is set
  by `TRQSH_LATEST_VERSION`.

## Deploy-time env

| Var | Default | Purpose |
|---|---|---|
| `TRQSH_DASHBOARD_URL` | `http://localhost:3000` | Sign-up / login CTAs |
| `TRQSH_API_URL` | `http://localhost:8080` | OAuth deep-links |
| `TRQSH_GITHUB_REPO` | `trqsh-uz/trqsh` | Release/download links |
| `TRQSH_SITE_URL` | `https://trqsh.uz` | Canonical origin, sitemap, OG |
| `TRQSH_LATEST_VERSION` | `0.1.0` | Download artifact filenames |
