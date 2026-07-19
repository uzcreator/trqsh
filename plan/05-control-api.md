# 05 — Control Plane API & Auth

**Owns:** `internal/api`, Postgres schema + migrations, the real `authz.Entitlements` implementation.
**Depends on:** Part 01 (`pkg/authz` types). **Blocks:** Part 06 (dashboard), Part 07 (billing);
upgrades Parts 02/03 from the stub to real auth.

> Read `00-ARCHITECTURE.md` (§9 entitlements, §11 plans, §12 API surface). This is the system of
> record: identity, API keys, domains, quotas, teams — and it implements the entitlements interface
> the edge calls.

## Goal

1. A REST API (`https://api.rift.dev/v1`) for accounts, API keys, tunnels view, subdomains, custom
   domains, usage, teams — documented with **OpenAPI**.
2. Auth: OAuth (GitHub/Google) + email, **JWT** sessions for the dashboard, **API keys** for agents.
3. The real **`authz.Entitlements`** implementation (backed by Postgres + Part 07 quotas), exposed
   to the edge over an internal RPC.

## Stack
- Go + **chi** router + `net/http`; **Postgres** via **sqlc**; migrations via **goose**; validation
  + structured errors; OpenAPI generated (e.g. `oapi-codegen` or hand-authored `openapi.yaml`).

## Data model (Postgres — `internal/api/db/migrations/`)
- `users` (id, email, name, avatar, oauth_provider, created_at)
- `orgs` (id, name, plan, stripe_customer_id) + `org_members` (org_id, user_id, role)
- `api_keys` (id, org_id, name, hash, prefix, last_used_at, revoked_at) — store **hash only**
- `reserved_subdomains` (id, org_id, subdomain UNIQUE, created_at)
- `custom_domains` (id, org_id, domain UNIQUE, verify_token, verified_at, cert_status)
- `plans` (code, limits_json) — mirrors §11; source of truth referenced by Part 07
- `usage_records` (org_id, tunnel_id, bytes_in, bytes_out, requests, window_start, window_end)
- `tunnels_active` is **not** in Postgres — it lives in Redis (edge registry); the API reads Redis
  for the live list.

## Scope / task breakdown

### T1 — Auth (`internal/api/auth/`)
- OAuth GitHub + Google (authorization-code + a **device flow** for the CLI/GUI), email magic-link
  optional. Issue short-lived **JWT** access + refresh; `/auth/session` for the dashboard.
- **API keys**: `POST /api-keys` returns the key **once** (`rk_live_<random>`), store argon2id hash +
  a display prefix; `GET`/`DELETE` (revoke). Middleware authenticates either JWT (browser) or API key.

### T2 — Core resources (`internal/api/handlers/`)
- `/account`, `/orgs`, `/orgs/{id}/members` (roles: owner/admin/member; invites).
- `/subdomains` — reserve/list/release (enforce plan limits from §11).
- `/domains` — add custom domain → return DNS TXT/CNAME to set; `/domains/{id}/verify` checks DNS and
  flips `verified_at`; expose verified domains to the edge's on-demand-TLS allowlist (Redis).
- `/tunnels` — read live tunnels from Redis (per org).
- `/usage` — aggregate from `usage_records` for dashboard graphs + billing (Part 07).

### T3 — Entitlements implementation (`internal/api/entitlements/`)
- Implement `authz.Entitlements` (§9):
  - `Authenticate(apiKey)` → validate hash, return `(orgID, plan)`, update `last_used_at`.
  - `CheckBind(req)` → load plan limits + current usage/counts (Redis + Postgres), decide allow/deny,
    assign a random subdomain if none requested, enforce custom-domain verification + protocol/port
    entitlements. Return `Decision` with §8 error codes on denial.
  - `ReportUsage(u)` → upsert `usage_records`; forward to Part 07 metering.
- Expose it to the edge via an **internal RPC** (gRPC or authenticated HTTP on a private network),
  with a short-TTL **cache** in the edge to avoid a round-trip per bind. Define this internal
  endpoint here; Part 02 swaps its `StubEntitlements` for this client.

### T4 — OpenAPI + SDK (`docs/openapi.yaml`)
- Author/generate the OpenAPI spec; publish it for Part 06 (dashboard), Part 09 (docs/API reference),
  and a generated TS client for the frontend.

### T5 — Ops
- Rate limiting, request logging, `/healthz`, migrations on boot (goose), seed the `plans` table
  from §11, OpenTelemetry traces. Secrets via env (Part 08).

## Interfaces honored / provided
- Implements `authz.Entitlements` (§9) — **the contract Part 02 depends on**.
- Publishes the REST/OpenAPI surface (§12) consumed by 03 (`rift login` device flow), 06, 07, 09.

## Done criteria
- `goose up` builds the schema; `go test ./internal/api/...` passes (handlers + entitlements logic).
- OAuth login issues a JWT; device flow returns a token to the CLI; API-key create/list/revoke works.
- Edge (Part 02) pointed at the real entitlements client authenticates a key and enforces a denied
  bind (e.g. reserved subdomain on Free → `ERR_SUBDOMAIN_FORBIDDEN`).
- Subdomain reserve + custom-domain add/verify flows work end-to-end; `/usage` returns data.

## Run / verify
```bash
docker compose -f deploy/docker-compose.dev.yml up -d postgres redis
goose -dir internal/api/db/migrations up
go run ./cmd/riftapi          # or the api entrypoint defined here
# exercise:
curl -X POST localhost:8080/v1/api-keys -H "authorization: Bearer <jwt>"   # returns rk_live_...
# then run edge with RIFT_ENTITLEMENTS=api RIFT_API_URL=... and bind with that key via the agent
```
