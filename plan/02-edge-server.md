# 02 — Edge Server / Data Plane (`riftd`)

**Owns:** `internal/server`, `cmd/riftd`
**Depends on:** Part 01 (`pkg/proto`, `pkg/tunnel`, `pkg/authz`) — hard. Part 05 — **interface only**
(`authz.Entitlements`); ship against `authz.StubEntitlements` and swap to the real client later.
**Blocks:** the M1 MVP.

> Read `00-ARCHITECTURE.md` (§6–§9) and `01-protocol-transport.md` first. The edge is the public
> half: it accepts agent sessions and public traffic and welds them together.

## Goal

`riftd` — a horizontally scalable edge server that:
1. Accepts authenticated, multiplexed **agent sessions** (QUIC + TCP via `tunnel.Listener`).
2. Maintains a **tunnel registry** (`hostname/port → session`) in Redis (shared across edges).
3. Accepts **public traffic** — HTTP/HTTPS (vhost/SNI routing), raw **TCP**, and **UDP** — and
   forwards each connection over a fresh data stream to the owning agent.
4. Terminates TLS with **wildcard + custom-domain** certs (CertMagic, DNS-01).
5. Enforces entitlements at bind time and **reports usage** for billing.

## Scope / task breakdown

### T1 — Config & bootstrap (`internal/server/config.go`, `cmd/riftd/main.go`)
- Config via env/flags: `RIFT_QUIC_ADDR` (`:443/udp`), `RIFT_TCP_ADDR` (`:443`), `RIFT_HTTP_ADDR`
  (`:80` redirect), `RIFT_REDIS_URL`, `RIFT_BASE_DOMAIN` (`rift.sh`), `RIFT_REGION`, cert/DNS creds,
  entitlements mode (`stub` | `api` + `RIFT_API_URL`).
- `main.go` wires: entitlements client, Redis, cert manager, agent listener, ingress servers, metrics.
- Structured `slog` + OpenTelemetry init.

### T2 — Agent session handler (`internal/server/session.go`)
- `tunnel.Listen(...)` → for each accepted `Session`, `AcceptStream` the **control stream** (first).
- Read `Hello`; call `entitlements.Authenticate(apiKey)`. Reply `AuthResp` (ok/err per §8).
- Control loop: handle `BindTunnel` (→ T3), `Unbind`, `Ping`→`Pong`. On session close, unbind all
  its tunnels from the registry. Enforce heartbeats (drop dead sessions).

### T3 — Registry & bind (`internal/server/registry.go`)
- `Registry` abstraction over Redis:
  - `Bind(route Route, edgeID, sessionID)` where `Route` = `{host}` (HTTP) or `{proto,port}` (TCP/UDP).
  - `Lookup(route) → (edgeID, sessionID)`; `Unbind`; TTL + heartbeat refresh.
  - Pub/sub channel so edges learn of binds on other edges (for cross-edge routing, T7).
- On `BindTunnel`: build `authz.BindRequest`, call `entitlements.CheckBind`. If denied → `BindResp`
  with error code. If allowed → allocate hostname (`<sub|random>.<base_domain>`) or port, register a
  local **in-memory session map** `clientTunnelID → Session`, write the route to the registry, reply
  `BindResp` with `public_url`.
- Keep an in-process `map[route]*Session` for locally-owned tunnels (fast path).

### T4 — HTTP/HTTPS ingress (`internal/server/ingress_http.go`)
- Listeners on `:80` (redirect→https + ACME HTTP-01 fallback) and `:443` (TLS via T5).
- On each request: derive hostname from SNI/`Host`; `Registry.Lookup`. If local session: open a
  data stream to the agent, `WriteStreamInit{proto,remote_addr,sni}`, then **`io.Copy` both ways**
  (bidirectional weld). If remote edge: T7. If not found: 404 "tunnel offline" branded page.
- Support `basic_auth` option, `host_header` rewrite, websockets (hijack/stream), request-id header.
- Emit per-request bytes/latency to the usage aggregator (T6).

### T5 — TLS / certificates (`internal/server/tls.go`)
- **CertMagic** with a **DNS-01** solver (lego DNS provider from env) for `*.<base_domain>` wildcard.
- **On-demand TLS** for verified custom domains (check the control plane / a Redis allowlist before
  issuing, to prevent abuse). Cache certs in Redis/shared storage so all edges reuse them.
- Use Let's Encrypt **staging** in non-prod (`RIFT_ACME_STAGING=1`).

### T6 — TCP & UDP port tunnels + usage (`internal/server/ingress_tcp.go`, `ingress_udp.go`, `usage.go`)
- A pool of listenable ports (range from config) assigned on TCP/UDP binds. Accept a public conn →
  lookup by `{proto,port}` → data stream → `StreamInit` → weld. UDP: map datagrams to a stream with
  a session/flow table + idle timeout.
- `usage.go`: aggregate bytes in/out + request counts per account/tunnel in short windows; flush via
  `entitlements.ReportUsage`. Also enforce live bandwidth/rate limits from `Decision.Limits`.

### T7 — Multi-edge routing (design now, minimal impl) (`internal/server/forward.go`)
- MVP: single edge; `Lookup` returns local. Design the `Registry` + a `forward(edgeID, conn)` seam
  so a public connection arriving at edge B for a tunnel homed on edge A can be tunneled A↔B (an
  internal QUIC hop) later. Leave a clear TODO + interface; do not block MVP on it.

### T8 — Ops (`internal/server/metrics.go`, health, drain)
- Prometheus metrics: active sessions/tunnels, streams, bytes, handshake kind (quic/tcp), errors.
- `/healthz`, `/readyz`. Graceful drain: send `ShutdownMsg` to agents with a deadline, deregister
  from LB, finish in-flight, exit.

## Interfaces honored (do not modify)
- `tunnel.Listener/Session/Stream` (Part 01 §7), `proto.*` messages + codec (§6), error codes (§8),
  `authz.Entitlements` (§9). Inject the entitlements implementation; default to `StubEntitlements`.

## Done criteria
- `go build ./cmd/riftd` produces a running edge; `go test ./internal/server/...` passes.
- With `StubEntitlements` + local Redis: an agent (Part 03) can auth, bind an HTTP tunnel, and a
  `curl` to the public host returns the agent's local response; TCP + UDP tunnels weld correctly.
- Wildcard cert issues against Let's Encrypt **staging**; unknown host returns the branded 404.
- Metrics + `/healthz` live; graceful drain sends `ShutdownMsg`.

## Run / verify
```bash
docker compose -f deploy/docker-compose.dev.yml up -d redis   # from Part 08 (or minimal Part 00)
RIFT_ENTITLEMENTS=stub RIFT_BASE_DOMAIN=lvh.me RIFT_ACME_STAGING=1 go run ./cmd/riftd
# then, in another shell, use the Part 03 agent to bind a tunnel and curl the public URL
```
Note: `lvh.me` and `*.lvh.me` resolve to 127.0.0.1 — handy for local vhost testing without DNS.
