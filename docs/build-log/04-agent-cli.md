# Step 4 — Part 03: Agent Core + CLI (`trqsh`)

- **Date:** 2026-07-17
- **Step:** 4 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/03-agent-cli.md`](../../plan/03-agent-cli.md)
- **Milestone:** **M1 — MVP reached** 🎉
- **Status:** ✅ Complete — build/vet/test green; **real binaries prove the full loop** (`trqsh http 3000` → live public URL).

> **TL;DR (Uz):** Agent yadrosi + `trqsh` CLI yozildi. Agent edge'ga `pkg/tunnel` orqali ulanadi,
> `Hello`/`BindTunnel` yuboradi, data streamlarni qabul qilib local xizmatga uzatadi (HTTP inspector
> orqali). Avtomatik qayta ulanish (backoff + re-bind), `127.0.0.1:4040` inspector (replay bilan),
> GUI uchun local control API (JSON+SSE), va cobra CLI (`http/tcp/udp/start/status/stop/login/config/
> version/update`). **MVP TIRIK:** haqiqiy binarlar bilan `trqsh http 3000` → `curl` local javobni
> qaytardi. Keyingi qadam: **Part 05 — Control API** (haqiqiy auth/billing).

## What was built

`trqsh` — the open-source client. All code in `internal/agent` (+ `inspect/`, `cli/`) and `cmd/trqsh`.

```
cmd/trqsh/main.go                    → cli.Execute()
internal/agent/config.go            §10 config (~/.trqsh-uz/trqsh.yml): Load (flags>env>file>defaults), Save
internal/agent/core.go              FROZEN agent-core API: Core, TunnelSpec, Tunnel, Event, Status + impl
internal/agent/session.go           dial → control stream → Hello/Auth → Bind; control loop; data accept
internal/agent/forward.go           local forwarding: HTTP (inspector tee), raw TCP, UDP (uint16 framing)
internal/agent/reconnect.go         backoff + jitter re-dial, re-bind (restores reserved subdomains/ports)
internal/agent/localapi.go          local control API: loopback HTTP JSON + SSE (fan-out) for the GUI
internal/agent/errors.go            *Error over §8 codes + user-facing Hint()
internal/agent/version.go           ldflags build vars
internal/agent/inspect/inspect.go   ring-buffer Recorder + CapturedRequest
internal/agent/inspect/server.go    inspector web UI at :4040 + JSON API + replay + SSE
internal/agent/cli/cli.go           cobra root, global flags, run loop, event stream
internal/agent/cli/commands.go      http/tcp/udp/start/status/stop/login/config/version/update
internal/agent/*_test.go            config precedence + E2E MVP + local control API integration
```

## The frozen agent-core API (defined here, consumed by Part 04 GUI)

```go
type Core interface {
    Login(ctx, token string) error
    Status() Status
    StartTunnel(ctx, spec TunnelSpec) (Tunnel, error)
    StopTunnel(ctx, id string) error
    List() []Tunnel
    Events() <-chan Event
    Shutdown(ctx) error
}
```
`TunnelSpec{Name,Proto,Addr,Subdomain,CustomDomain,BasicAuth,HostHeader,RemotePort}`,
`Tunnel{ID,Name,Proto,LocalAddr,PublicURL,Status,Metrics}`, `Event{Type,Status,Tunnel,Request,Err}`.
**Do not change these signatures** — Part 04 binds to them (in-process or via the local control API).

## How it works

- **Connect**: `tunnel.Dialer.Dial(server)` (QUIC-first, TCP fallback) → open the **control stream**
  (first stream) → send `Hello`(+api_key) → read `AuthResp`. A single reader loop demuxes the control
  stream (`BindResp` routing, `Ping`→`Pong`, `Shutdown`→reconnect, `Error`).
- **Bind**: `StartTunnel` sends `BindTunnel`, awaits `BindResp`, surfaces `PublicURL`.
- **Serve**: `AcceptStream` (edge-initiated data streams) → `ReadStreamInit` → forward to the local
  service. HTTP tunnels are **teed through the inspector**; TCP/TLS weld raw; UDP mirrors the edge's
  uint16-length datagram framing.
- **Resilience**: on session drop, exponential backoff + jitter re-dial and **re-bind every tunnel**,
  restoring reserved subdomains/ports. `ShutdownMsg` (edge drain) triggers the same path.
- **Inspector** (`:4040`): ring buffer of request/response captures with a web UI, live SSE, and
  **replay** (re-issue a captured request to the local service).
- **Local control API** (`cfg.ControlAddr`): JSON over loopback HTTP + an SSE event stream, so the GUI
  drives the same core out-of-process (`GET /status|/tunnels`, `POST /tunnels`, `DELETE /tunnels/{id}`,
  `GET /events`).
- **CLI**: `trqsh http 3000`, `trqsh tcp 22 --remote-port 2222`, `trqsh udp 5353`, `trqsh start` (all
  config tunnels), `trqsh status`/`trqsh stop` (talk to a running agent's control API), `trqsh login`,
  `trqsh config`, `trqsh version`, `trqsh update`. Errors map §8 codes → messages + upgrade hints.

## Verification — MVP proven with real binaries

The in-process integration test and a real-binary smoke both pass:

| Check | Result |
|------|--------|
| `go build ./...` / `go vet ./...` | ✅ |
| `go test ./internal/agent/...` (config + E2E + control API) | ✅ ok |
| Stress `-count=5` (MVP + ControlAPI) | ✅ stable |
| **`trqsh http 3000` → `curl` → local response** | ✅ `hello-from-local-service` |
| `trqsh status` via control API | ✅ connected, plan=pro, req counter |
| Inspector captured the request | ✅ full headers + timing |

`TestMVPEndToEnd` boots the **real edge** (Part 02) + agent core + a local `httptest` service, binds
`demo`, and a public `GET Host: demo.lvh.me` returns the local service's body — the complete
**agent ↔ edge ↔ public** loop. `TestLocalControlAPIDrivesCore` drives start/list/status over the
control API. Real-binary run (`trqshd` + `trqsh http 3000 --server 127.0.0.1:4443 --insecure`) confirmed
the same end to end, plus the inspector at `:4040`.

`-race` still can't run here (no C compiler — see `02-protocol-transport.md`); stress runs stand in.

### Run locally (spec Run/verify)
```powershell
# 1) local service
python -m http.server 3000
# 2) edge (dev ports; :443/:80 need admin, so use highs)
$env:TRQSH_ENTITLEMENTS="stub"; $env:TRQSH_BASE_DOMAIN="lvh.me"
$env:TRQSH_HTTP_ADDR="127.0.0.1:8080"; $env:TRQSH_QUIC_ADDR="127.0.0.1:4443"; $env:TRQSH_TCP_ADDR="127.0.0.1:4443"
go run ./cmd/trqshd
# 3) agent
go run ./cmd/trqsh http 3000 --server 127.0.0.1:4443 --insecure --subdomain demo
curl -H "Host: demo.lvh.me" http://127.0.0.1:8080/    # local listing
# inspector: http://127.0.0.1:4040
```

## Key decisions

- **Default server `localhost:4443`** and dev API-key fallback (`tq_dev_local` when unset) so
  `trqsh http 3000` works against the stub edge out of the box. A real edge requires `trqsh login`.
  (This reconciles the `:4443` agent-port divergence noted in `03-edge-server.md`.)
- **Local control API = loopback HTTP** (not a unix socket / named pipe) for cross-platform simplicity;
  the frozen `Core` interface is transport-agnostic, so Part 04 can also embed the core in-process.
- **HTTP forwarding parses & re-emits** (via `req.Write`/`http.ReadResponse`) to feed the inspector
  full request/response captures; websocket upgrades fall back to a raw weld. Bodies captured up to 64 KiB.
- **UDP framing mirrors the edge** exactly (uint16-BE length prefix per datagram) — the contract from
  `03-edge-server.md` is honored.
- **`Events()` is a single shared channel** (documented single-consumer); the local control API is that
  consumer and re-broadcasts to any number of GUI clients via its own fan-out.

## Known gaps / notes (for later parts)

- **`trqsh login`/real keys**: today any non-empty key passes the stub edge; real issuance + validation
  arrives with Part 05 (control API) and is wired at Qadam 7.
- **`trqsh update`** is a stub pointing at the download page; the signed release feed is Part 08.
- **No daemonization**: `trqsh status`/`trqsh stop` talk to a *running* foreground agent's control API.
  A background service/daemon mode can come with Part 04/08 if desired.
- **Reconnect** is implemented and emits events; it's covered by code + manual network-drop testing
  rather than an automated (flaky) port-recycle test.

## What's next

The MVP is complete: **`trqsh http 3000` yields a live public URL that proxies to localhost**, TCP/UDP
tunnels weld, and the inspector shows traffic. Next is **Part 05 — Control API & Auth**
(`plan/05-control-api.md`): real accounts, API keys, domains, quotas, and the real `authz.Entitlements`
that the edge swaps in for the stub at Qadam 7 (Integration).
