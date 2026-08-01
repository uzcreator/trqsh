# web/dashboard — User dashboard

The authenticated web control surface: tunnels, domains, API keys, usage, billing, and the
cloud request inspector. **Next.js 15** (App Router) + **TypeScript** + **Tailwind**,
talking only to the control API (`trqshapi`, in [`cmd/trqshapi`](../../cmd/trqshapi) /
[`internal/api`](../../internal/api)). Served at **app.trqsh.uz**.

Part of the [trqsh monorepo](../../README.md); shares design tokens with
[`web/site`](../site) and [`desktop`](../../desktop).

## Run

```bash
pnpm install
# point at a running control API; defaults to http://localhost:8080
export TRQSH_API_URL=http://localhost:8080
export NEXT_PUBLIC_TRQSH_BASE_DOMAIN=lvh.me
pnpm dev            # http://localhost:3000
# production build:
pnpm build && pnpm start
```

Bring the API up with `make dev` from the repo root.

## How it works

- **Auth** — the JWT session lives in **httpOnly cookies** set by a Server Action on login;
  `middleware.ts` guards every route. The token never reaches the browser: reads happen in
  Server Components via `lib/api.ts`, writes go through Server Actions, and a 401
  transparently refreshes and retries.
- **Data** — `lib/api.ts` is a typed client for the control API (see
  [`docs/openapi.yaml`](../../docs/openapi.yaml)). No direct Postgres/Redis access.
- **Billing** — the billing screen embeds Stripe: plan cards open **Checkout** and
  "Manage billing" opens the **Customer Portal** (both via Server Actions). Plan changes
  land through the webhook, so the UI reflects them on the next load.
- **Design** — tokens in `app/globals.css` + `tailwind.config.ts` follow the dataviz
  reference palette (light/dark, RGB-channel tokens so opacity utilities work), shared with
  the site and desktop app.

## Structure

```
app/(dashboard)/   protected app: overview, tunnels, domains, keys, usage,
                   inspect, billing, team, settings (each page + its actions)
app/login/         public login (email dev flow + OAuth links)
components/         nav, topbar, tiles, charts, ui/ primitives (shadcn-style)
lib/               api client, session cookies, error mapping, formatting
middleware.ts      route protection
```

## Deploy

Built as a Docker image
([`deploy/docker/Dockerfile.dashboard`](../../deploy/docker/Dockerfile.dashboard)) and
published to `ghcr.io/trqsh-uz/dashboard` by
[`.github/workflows/images.yml`](../../.github/workflows/images.yml). `TRQSH_API_URL` and
`NEXT_PUBLIC_TRQSH_BASE_DOMAIN` are inlined at **build** time, so the published image bakes
the production values; see [`deploy/PRODUCTION.md`](../../deploy/PRODUCTION.md).
