# trqsh Dashboard (Part 06)

The authenticated web control surface: tunnels, domains, API keys, usage, billing,
and the cloud request inspector. Next.js 15 (App Router) + TypeScript + Tailwind,
talking only to the Part 05 control API (`trqshapi`).

## Run

```bash
pnpm install
# point at a running control API (Part 05); defaults to http://localhost:8080
export TRQSH_API_URL=http://localhost:8080
export NEXT_PUBLIC_TRQSH_BASE_DOMAIN=lvh.me
pnpm dev            # http://localhost:3000
# production build:
pnpm build && pnpm start
```

## How it works

- **Auth** — the JWT session lives in **httpOnly cookies** set by a Server Action on
  login; `middleware.ts` guards every route. The token never reaches the browser: reads
  happen in Server Components via `lib/api.ts`, and writes go through Server Actions. A
  401 transparently refreshes and retries.
- **Data** — `lib/api.ts` is a typed client for the Part 05 endpoints
  (`docs/openapi.yaml`). No direct Postgres/Redis access.
- **Billing** — the billing screen embeds Part 07: plan cards open **Stripe Checkout**
  and "Manage billing" opens the **Customer Portal** (both via Server Actions). Plan
  changes land through the webhook, so the UI reflects them on the next load.
- **Design** — tokens in `app/globals.css` + `tailwind.config.ts` follow the dataviz
  reference palette (light/dark, RGB-channel tokens so opacity utilities work). This
  token set is the shared source of truth for Parts 04 (GUI) and 09 (site).

## Structure

```
app/(dashboard)/           protected app: overview, tunnels, domains, keys, usage,
                           inspect, billing, team, settings (each page + its actions)
app/login/                 public login (email dev flow + OAuth links)
components/                nav, topbar, tiles, charts, ui/ primitives (shadcn-style)
lib/                       api client, session cookies, error mapping, formatting
middleware.ts              route protection
```
