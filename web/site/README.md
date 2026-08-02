# web/site — Marketing site & docs

The public face of trqsh: landing, pricing, download, and the full documentation
(including a generated API reference). **Next.js 15** (App Router) + **TypeScript** +
**Tailwind**, with a hand-written shadcn-style UI kit. Lives at
**[trqsh.uz](https://trqsh.uz)**.

Part of the [trqsh monorepo](../../README.md) — the backend it links to lives in
[`cmd/`](../../cmd) / [`internal/`](../../internal), and the brand/design tokens are
shared with [`web/dashboard`](../dashboard) and [`desktop`](../../desktop).

## Run

```bash
pnpm install
pnpm dev        # http://localhost:3002
pnpm build      # production build
pnpm typecheck
```

Runs on **:3002** so it can coexist with the dashboard (:3000) locally.

## Content is generated from the live API (never hardcoded)

Two pieces of content are generated from a running control API at build/regenerate time,
then **checked in** — so a normal `pnpm build` never needs the API to be up, only
regeneration does. This keeps the site's numbers from ever drifting from what the edge
actually enforces, and lets the site build with no Go toolchain:

- **Pricing** — `lib/catalog.generated.ts` is produced by `scripts/genplans.mjs`, which
  fetches `GET $TRQSH_API_URL/v1/plans/public` (the unauthenticated plan catalog).
- **API reference** — `lib/openapi.generated.yaml` is produced by `scripts/gen-openapi.mjs`,
  which fetches `$TRQSH_API_URL/openapi.yaml`; the `/docs/api` page renders from it.

Regenerate after the backend's plan catalog or OpenAPI changes:

```bash
node scripts/genplans.mjs
node scripts/gen-openapi.mjs
# or, from the repo root:  make site-plans site-openapi
```

CI re-runs both and fails on any diff, so the checked-in copies can't silently go stale.
Docs pages render from Markdown in [`docs/content/`](../../docs/content).

## Deploy-time env

| Var | Default | Purpose |
|---|---|---|
| `TRQSH_DASHBOARD_URL` | `https://app.trqsh.uz` | Sign-up / login CTAs |
| `TRQSH_API_URL` | `https://api.trqsh.uz` | OAuth deep-links + build-time content fetch |
| `TRQSH_GITHUB_REPO` | `uzcreator/trqsh` | Release / download links |
| `TRQSH_SITE_URL` | `https://trqsh.uz` | Canonical origin, sitemap, OG |
| `TRQSH_LATEST_VERSION` | see `lib/site.ts` | Download artifact filenames |

## Deploy

Built as a Docker image
([`deploy/docker/Dockerfile.site`](../../deploy/docker/Dockerfile.site) — a self-contained
Next.js standalone server) and published to `ghcr.io/uzcreator/site` by
[`.github/workflows/images.yml`](../../.github/workflows/images.yml). See
[`deploy/`](../../deploy) for how it runs behind the edge, which reverse-proxies the apex
domain to it.
