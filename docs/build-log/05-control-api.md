# Step 5 — Part 05: Control Plane API & Auth (`trqshapi`)

- **Date:** 2026-07-17
- **Step:** 5 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/05-control-api.md`](../../plan/05-control-api.md)
- **Milestone:** M2 — Monetizable product (foundation)
- **Status:** ✅ Complete — build/vet/test green; **real binaries prove edge → real entitlements enforcement** (Qadam 7 gate demonstrated early).

> **TL;DR (Uz):** Control plane (`trqshapi`) yozildi — akkauntlar, API kalitlar (argon2id), reserved
> subdomainlar, custom domenlar, usage, planlar, JWT sessiyalar, OAuth (GitHub/Google) + device flow.
> Haqiqiy **`authz.Entitlements`** implementatsiyasi edge'ga internal RPC orqali ochilgan. Isbot:
> haqiqiy binarlar bilan edge `TRQSH_ENTITLEMENTS=api` rejimida ishlaydi — Free plan UDP'ni va band
> qilinmagan subdomainni **rad etadi**, oddiy HTTP'ga **ruxsat beradi**. Keyingi qadam: **Part 07 — Billing**.

## What was built

`trqshapi` — the control plane and system of record. Code in `internal/api` (+ `store/`, `auth/`),
the shared edge RPC in `internal/entitlerpc`, and `cmd/trqshapi`.

```
internal/api/plans.go              §11 plan/quota catalog → authz.Limits (source of truth)
internal/api/config.go             env config; DevAuth for password-less local login
internal/api/server.go             chi router, middleware, health, JSON helpers
internal/api/entitlements.go       real authz.Entitlements (Authenticate/CheckBind/ReportUsage)
internal/api/rpc.go                token-guarded internal entitlements endpoints (edge calls these)
internal/api/handlers_auth.go      signup/login/refresh, OAuth start+callback, device flow, account
internal/api/handlers_resources.go api-keys, subdomains, domains(+verify), usage, orgs, tunnels, plans
internal/api/store/store.go        Store interface + domain types
internal/api/store/mem.go          in-memory store (tested path)
internal/api/store/postgres.go     Postgres store (database/sql + pgx; matches the migrations)
internal/api/db/migrations/*.sql   goose schema (users, orgs, api_keys, subdomains, domains, usage, plans)
internal/api/auth/auth.go          JWT issue/verify/refresh; API-key authentication
internal/api/auth/apikey.go        key generation + argon2id hash/verify (store hash only)
internal/api/auth/oauth.go         GitHub/Google provider (auth URL + code→token→userinfo)
internal/api/auth/device.go        RFC 8628-style device flow store
internal/api/auth/middleware.go    Principal + Bearer (JWT or API key) authentication
internal/entitlerpc/entitlerpc.go  edge-side Client (authz.Entitlements) + wire types + short-TTL cache
cmd/trqshapi/main.go                entrypoint
docs/openapi.yaml                  OpenAPI 3.1 surface (consumed by Parts 06/09)
```

## How it works

- **Store abstraction** (same pattern as the edge registry): `MemStore` when `TRQSH_DATABASE_URL` is
  unset (dev + tests), `PostgresStore` otherwise. The goose migrations define the production schema.
- **Auth**: JWT access/refresh (HS256) for the dashboard; **API keys** `tq_live_<prefix>_<secret>`
  with the **argon2id hash stored** (plaintext shown once) and prefix-based lookup. Middleware accepts
  either a Bearer JWT or an API key. **OAuth** (GitHub/Google) is wired (auth URL + code exchange +
  userinfo → user upsert); a password-less email flow (`DevAuth`) backs local dev + tests. A
  **device flow** powers `trqsh login` (code → browser approve → CLI polls → API key).
- **Entitlements** (`authz.Entitlements`, §9) over the store + plan catalog:
  - `Authenticate` validates the key hash → `(orgID, plan)`.
  - `CheckBind` enforces protocol entitlements (UDP/TLS/TCP per plan), custom-domain ownership +
    verification, reserved-subdomain ownership (else assigns a random subdomain), and returns §8 codes.
  - `ReportUsage` upserts usage windows.
- **Internal RPC** (`internal/entitlerpc`): the edge imports a `Client` (an `authz.Entitlements` impl
  with a 30 s auth cache) that POSTs to the API's token-guarded `/internal/entitlements/*` endpoints.
  **Part 02's `TRQSH_ENTITLEMENTS=api` now returns this client** — the stub-to-real swap is done.

## Verification

| Check | Result |
|------|--------|
| `go build ./...` / `go vet ./...` | ✅ |
| `go test ./internal/api/...` (12 tests) | ✅ ok |
| Entitlements logic (Free denies UDP + unreserved subdomain; Pro allows; custom-domain verify) | ✅ |
| API-key create/list/revoke; reserved-subdomain limit; device flow | ✅ |
| Internal RPC: edge client `Authenticate` + `CheckBind` over the wire; bad token rejected | ✅ |
| **Real binaries: edge (`api` mode) → `trqshapi` enforcement** | ✅ (below) |

**End-to-end enforcement (the Qadam 7 gate, shown now)** — `trqshapi` + `trqshd TRQSH_ENTITLEMENTS=api`
+ `trqsh` with a real key:
- `trqsh http 3000` (Free, no subdomain) → **allowed**, random `du0ixy1k.lvh.me`; `curl` returned the
  local service. The real API key authenticated via the internal RPC (edge logs `plan=free`).
- `trqsh udp 5353` (Free) → **denied**: `ERR_PLAN_FORBIDS` — "UDP tunnels require a paid plan".
- `trqsh http 3000 --subdomain demo` (not reserved) → **denied**: `ERR_SUBDOMAIN_FORBIDDEN`.

`-race` still can't run here (no C compiler — see `02-protocol-transport.md`).

### Run locally
```powershell
# control API (in-memory store, dev auth)
$env:TRQSH_JWT_SECRET="devsecret"; $env:TRQSH_INTERNAL_TOKEN="dev-internal-token"; $env:TRQSH_BASE_DOMAIN="lvh.me"
go run ./cmd/trqshapi
# create an account + key
curl -s -X POST localhost:8080/v1/auth/signup -d '{"email":"you@example.com","name":"You"}'   # -> tokens
curl -s -X POST localhost:8080/v1/api-keys -H "authorization: Bearer <access>" -d '{"name":"cli"}'  # -> tq_live_...
# edge in real-entitlements mode
$env:TRQSH_ENTITLEMENTS="api"; $env:TRQSH_API_URL="http://127.0.0.1:8080"; $env:TRQSH_INTERNAL_TOKEN="dev-internal-token"
go run ./cmd/trqshd
# With Postgres instead of memory:
#   docker compose -f deploy/docker-compose.dev.yml up -d postgres
#   goose -dir internal/api/db/migrations postgres "$TRQSH_DATABASE_URL" up
#   set TRQSH_DATABASE_URL and restart trqshapi
```

## Key decisions

- **Store interface + MemStore/PostgresStore** (not sqlc) so the whole suite is green without a live
  database, mirroring the edge's in-mem/Redis split. The Postgres queries match the goose migrations
  and are validated by compilation + schema review; point `TRQSH_DATABASE_URL` at Postgres to exercise.
- **Reserved-subdomain semantics** reconcile §11 (Free gets 1) with the Qadam 7 gate (Free denied a
  reserved subdomain): a *specific* requested subdomain must be **pre-reserved by the org** (via
  `/subdomains`, capped by plan) — otherwise `CheckBind` returns `ERR_SUBDOMAIN_FORBIDDEN`. No
  requested subdomain → a random one is assigned.
- **Argon2id, hash-only** key storage; plaintext returned exactly once on create.
- **Internal RPC over authenticated HTTP** (shared `X-trqsh-Internal-Token`) rather than gRPC, to keep
  deps light; the edge caches `Authenticate` for 30 s to avoid a round-trip per stream.
- **DevAuth** enables password-less signup/login and a `?force=true` domain-verify shortcut for local
  dev/tests; both are gated off in production (`TRQSH_DEV_AUTH=false`).

## Known gaps / notes (for later parts)

- **OAuth** is implemented but untestable here (needs real GitHub/Google app credentials); the email/
  device flows cover local + CI. Set `TRQSH_GITHUB_CLIENT_ID/SECRET`, `TRQSH_GOOGLE_*` to enable.
- **`/tunnels`** returns `[]` until the edge publishes an org-scoped active-tunnel index in Redis
  (Part 06 refinement); the live list can also be read directly from the edge registry.
- **On-demand TLS allowlist**: verified custom domains should be pushed to the edge's cert allowlist
  (Redis) — wire this alongside the CertMagic manager in Part 02/08.
- **Postgres path** is compiled + schema-reviewed but not CI-exercised here (no DB in this env).
- **Plan seeding**: the `plans` table exists for Part 07/reference; the app reads the code `Catalog`.

## What's next

**Part 07 — Billing & Monetization** (`plan/07-billing-monetization.md`): Stripe Checkout + Customer
Portal + webhooks that flip `orgs.plan`, metered usage ingestion, and quota enforcement wired through
this `CheckBind`. Then the dashboard (Part 06) and GUI (Part 04) consume this API + OpenAPI. The edge↔
real-entitlements wiring (Qadam 7) is already proven above.
