# Step 3 — Part 02: Edge Server (`riftd`)

- **Date:** 2026-07-17
- **Step:** 3 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/02-edge-server.md`](../../plan/02-edge-server.md)
- **Milestone:** M1 — MVP (edge half)
- **Status:** ✅ Complete — `go build ./cmd/riftd`, `go vet`, `go test ./internal/server/...` green; `riftd` boots and serves.

> **TL;DR (Uz):** Edge server `riftd` yozildi — agent sessiyalarini qabul qiladi (Part 01 transport),
> tunnellarni registrga yozadi, va **public traffic'ni** (HTTP/HTTPS/TCP/UDP) agentga **weld** qiladi.
> Entitlements `StubEntitlements` orqali (Part 05 gacha). Integration test: **soxta agent → edge →
> public HTTP so'rov** to'liq ishlaydi; TCP echo ham. `riftd` ishga tushadi: `/healthz`, `/readyz`,
> `/metrics`, va noma'lum host uchun brendlangan 404. Keyingi qadam: **Part 03 — Agent + CLI** (haqiqiy agent).

## What was built

`riftd` — the public edge/data plane. All code in `internal/server` + `cmd/riftd`.

```
cmd/riftd/main.go            flags/env → Config → Server.Run with signal drain
internal/server/config.go    Config + LoadConfig (env, §T1)
internal/server/server.go    Server lifecycle, bind authorization, /healthz /readyz /metrics, drain
internal/server/session.go   agent handshake (Hello→Auth→AuthResp) + control loop + heartbeat
internal/server/hub.go       in-process routing: agentSession, boundTunnel, Hub (host/port maps)
internal/server/registry.go  Registry iface + InMemoryRegistry + RedisRegistry (TTL + pub/sub)
internal/server/weld.go      rawJoin bidirectional weld, branded 404, raw HTTP responses
internal/server/ingress_http.go   :80 + :443 HTTP proxy (Host route, keep-alive, websockets)
internal/server/ingress_tcp.go    port pool (portManager) + TCP tunnel listeners + weld
internal/server/ingress_udp.go    UDP tunnels (uint16-framed datagrams over a stream, idle GC)
internal/server/tls.go       CertManager iface + devCertManager (self-signed) + ACME seam
internal/server/usage.go     per-account/tunnel byte+request aggregation → ReportUsage
internal/server/metrics.go   Prometheus collectors (sessions, tunnels, streams, bytes, ...)
internal/server/forward.go   multi-edge Forwarder seam (no-op stub + TODO)
internal/server/*_test.go    registry unit tests + full weld integration tests (fake agent)
```

## How it works (data path)

1. **Agent sessions** arrive via `tunnel.Listen(QUICAddr, TCPAddr, tls)` (Part 01). The first stream
   (agent-opened) is the **control stream**: read `Hello` → `entitlements.Authenticate` → `AuthResp`.
2. The **control loop** handles `BindTunnel` (→ `CheckBind`, allocate host/port, register in Hub +
   Registry, reply `BindResp{public_url}`), `Unbind`, and `Ping`↔`Pong`. A heartbeat pings the agent;
   session death unbinds all its tunnels.
3. **Public traffic** hits ingress; the edge looks up the route in the **Hub**, opens a **fresh data
   stream** to the owning agent, writes `StreamInit`, and **welds** bytes both ways:
   - HTTP/HTTPS: parse request, route by `Host`, forward request/response; upgrades (websockets)
     switch to a raw byte weld. TLS terminates with a dev self-signed cert (SNI-based).
   - TCP: a pooled public port per tunnel; each accepted conn welds raw to a data stream.
   - UDP: datagrams framed with a uint16 length prefix over a per-flow stream, with an idle sweeper.
4. **Usage** is aggregated per account/tunnel and flushed via `entitlements.ReportUsage`.
5. **Ops**: Prometheus `/metrics`, `/healthz`, `/readyz`; graceful **drain** sends `ShutdownMsg`.

## Key decisions

- **Entitlements is injected** (`authz.Entitlements`); default `StubEntitlements` (allow-all).
  `RIFT_ENTITLEMENTS=api` currently also falls back to the stub with a `TODO(part-05)` seam — the real
  edge-side client (internal RPC + short-TTL cache) lands when Part 05 exists. Frozen §9 unchanged.
- **Registry is pluggable**: `InMemoryRegistry` when `RIFT_REDIS_URL` is empty (single edge, tests);
  `RedisRegistry` (go-redis) otherwise, with TTL refresh + a bind/unbind pub/sub channel for peers.
  The in-process **Hub** is the fast path; the Registry is the shared/cross-edge view.
- **TLS = dev self-signed** (`devCertManager`, cached per SNI) so `curl -k` works locally. A clear
  `TODO(prod)` documents the CertMagic DNS-01 wildcard + on-demand custom-domain path. CertMagic was
  intentionally **not** pulled in yet (heavy deps + needs real DNS creds that can't be exercised here).
- **Agent port vs public port:** `pkg/tunnel.Listen` owns its own socket and the frozen §7 API has no
  "wrap an accepted conn" helper, so agent sessions listen on a **dedicated port** (`RIFT_QUIC_ADDR`/
  `RIFT_TCP_ADDR`, default `:4443`) distinct from public `:80`/`:443`. Sharing `:443` via ALPN demux is
  a documented follow-up (would require an additive Part 01 contract change, announced via 00-ARCH).
  → **Part 03 note:** the agent's default `--server` must target `:4443` (not `:443`); the EXECUTION-ORDER
  MVP snippet will be reconciled when Part 03 lands.
- **HTTP proxying parses & re-emits** (not blind byte-copy) so `Host` routing, `basic_auth`,
  `host_header` rewrite, `X-Forwarded-*`, and `X-Request-Id` work; websocket upgrades fall back to raw weld.
- **UDP-over-stream framing**: uint16-BE length prefix per datagram (both directions) — the Part 03
  agent must mirror this exactly. Documented at the top of `ingress_udp.go`.

## Verification

| Gate | Result |
|------|--------|
| `go build ./...` / `go vet ./...` | ✅ |
| `go test ./internal/server/...` | ✅ ok (6 tests) |
| Stress `-count=6 -run Edge` | ✅ stable |
| `riftd` boots, `/healthz` + `/readyz` | ✅ HTTP 200 (`ok` / `ready`) |
| `/metrics` exposes collectors | ✅ `rift_sessions_active`, `rift_tunnels_active`, … |
| Unknown host | ✅ branded **404** |

Integration tests prove the edge half of the MVP against a **fake agent** built on the Part 01 transport:
- `TestEdgeHTTPWeld` — agent binds `demo` → public `GET http://.../hi` with `Host: demo.lvh.me`
  returns the agent's response (`hello from agent path=/hi`). Full handshake→bind→weld.
- `TestEdgeTCPWeld` — agent binds a TCP tunnel on an assigned port; a raw TCP client echoes through it.
- `TestEdgeUnknownHost404` — unknown host → branded 404.
- Registry: route-key formatting, bind/lookup/unbind, TTL expiry.

`-race` still can't run here (no C compiler — see `02-protocol-transport.md`); concurrency was stress-run instead.

### Run locally
```powershell
# minimal: no Redis needed (in-memory registry), high ports avoid admin
$env:RIFT_ENTITLEMENTS="stub"; $env:RIFT_BASE_DOMAIN="lvh.me"
$env:RIFT_HTTP_ADDR="127.0.0.1:8080"; $env:RIFT_HTTPS_ADDR="127.0.0.1:8443"
$env:RIFT_QUIC_ADDR="127.0.0.1:4443"; $env:RIFT_TCP_ADDR="127.0.0.1:4443"
$env:RIFT_METRICS_ADDR="127.0.0.1:9099"
go run ./cmd/riftd
# elsewhere: curl http://127.0.0.1:9099/healthz  → ok
```

## Known gaps / notes (intentional, for later parts)

- **Custom domains**: `AllowCustomDomains` is honored at bind, but on-demand cert issuance for custom
  domains needs the real CertManager (see `tls.go` TODO) and a control-plane allowlist (Part 05).
- **Multi-edge forwarding** is a designed seam only (`forward.go`) — MVP is single-edge; the Registry
  already stores `EdgeID` so a QUIC inter-edge hop can be added in Part 08 without contract churn.
- **`api` entitlements mode** returns the stub until Part 05; wiring happens in Qadam 7 (integration).
- **Usage/limits** enforcement is bind-time + best-effort byte counting; live bandwidth/rate limiting
  from `Decision.Limits.RateLimitRPS` is a Part 07 concern.

## What's next

**Part 03 — Agent + CLI (`rift`)** (`plan/03-agent-cli.md`). It dials this edge over `pkg/tunnel`,
opens the control stream, sends `Hello`/`BindTunnel`, accepts data streams, and forwards to the local
service — completing the **M1 MVP** (`rift http 3000` → live public URL). Mirror the UDP framing and
target `:4443` for the default server address.
