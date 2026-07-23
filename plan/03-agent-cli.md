# 03 — Agent Core + CLI (`trqsh`)

**Owns:** `internal/agent`, `cmd/trqsh`
**Depends on:** Part 01 (`pkg/proto`, `pkg/tunnel`) — hard. Part 05 for `trqsh login` / API-key issuance
— stub-friendly (any non-empty key works against the edge's `StubEntitlements`).
**Blocks:** Part 04 (GUI embeds this core). **Defines:** the agent-core API the GUI consumes.

> Read `00-ARCHITECTURE.md` (§6, §7, §10) and `01-protocol-transport.md` first. This is the local
> engine: it dials the edge, binds tunnels, forwards traffic to local services, and exposes a clean
> API + local inspector. **This part is open source** — keep it dependency-light and readable.

## Goal

1. A reusable **agent core** (`internal/agent`) that the CLI and the GUI both drive.
2. A polished **CLI** (`trqsh http 3000`, `trqsh tcp 22`, `trqsh start`, `trqsh login`, …).
3. A **local inspector** at `127.0.0.1:4040` (capture/replay HTTP, like ngrok).
4. A **local control API** (JSON over a unix socket / loopback HTTP) so the GUI (Part 04) drives the
   same core out-of-process.

## Scope / task breakdown

### T1 — Config (`internal/agent/config.go`)
- Load `~/.trqsh-uz/trqsh.yml` per §10; merge flags > env (`TRQSH_*`) > file > defaults.
- `trqsh config` subcommands to view/edit; `trqsh login` writes `api_key`.

### T2 — Agent core & session (`internal/agent/core.go`, `session.go`)
- `type Core` with the **frozen agent-core API** (consumed by CLI + GUI):
  ```go
  type Core interface {
      Login(ctx, token string) error
      Status() Status                      // connected?, plan, edge, kind(quic/tcp)
      StartTunnel(ctx, spec TunnelSpec) (Tunnel, error)
      StopTunnel(ctx, id string) error
      List() []Tunnel
      Events() <-chan Event                // state changes, new requests, errors
      Shutdown(ctx) error
  }
  type TunnelSpec struct { Name, Proto, Addr, Subdomain, CustomDomain, BasicAuth string; RemotePort int }
  type Tunnel struct { ID, Name, Proto, LocalAddr, PublicURL, Status string; Metrics TunnelMetrics }
  type Event struct { Type string; Tunnel *Tunnel; Request *CapturedRequest; Err string }
  ```
- Session lifecycle: `tunnel.Dialer.Dial(server)` → open control stream → send `Hello`(+api_key) →
  read `AuthResp`. For each `TunnelSpec`, send `BindTunnel`, await `BindResp`, surface `PublicURL`.
- Handle **incoming data streams**: `AcceptStream` → `ReadStreamInit` → dial local `Addr` → `io.Copy`
  both ways. For HTTP protos, tee through the inspector (T5) before forwarding.

### T3 — Reconnect & resilience (`internal/agent/reconnect.go`)
- On session drop: exponential backoff + jitter, re-dial, re-`Hello`, re-`BindTunnel` every active
  tunnel, restore `PublicURL`s (reserved subdomains return the same host). Emit `Event`s throughout.
- Respect `ShutdownMsg` (drain) by reconnecting to another edge before the deadline.

### T4 — Local control API for the GUI (`internal/agent/localapi.go`)
- Serve the `Core` methods as JSON over a **unix socket** (Windows: named pipe) at a well-known path,
  plus optional loopback HTTP. Include an SSE/websocket stream mirroring `Events()`.
- This is the contract Part 04 (GUI) binds to when it runs the core out-of-process. (Wails can also
  embed the core in-process; expose both — see Part 04.)

### T5 — Inspector (`internal/agent/inspect/`)
- For HTTP tunnels, capture request/response (method, path, headers, timing, sizes, body up to a cap)
  into a ring buffer; serve a small web UI at `127.0.0.1:4040` with a request list, detail view, and
  **replay** (re-issue a captured request to the local service). Stream captures as `Event`s too, so
  the GUI/dashboard can show live traffic.

### T6 — CLI (`cmd/trqsh`, `internal/agent/cli/`)
- `github.com/spf13/cobra`. Commands:
  - `trqsh http <port|addr> [--subdomain] [--basic-auth] [--host-header]`
  - `trqsh tcp <port|addr> [--remote-port]`, `trqsh udp <port|addr>`
  - `trqsh start` (bind all tunnels from config), `trqsh status`, `trqsh stop`
  - `trqsh login [--token]`, `trqsh config`, `trqsh version`, `trqsh update`
- TTY UX: a live status panel (tunnel table + public URLs + request counter), copyable URLs, colored
  errors mapped from §8 codes (with upgrade hints for `ERR_PLAN_FORBIDS`). Support `--log json`.

### T7 — Packaging hooks
- Version/build info via `-ldflags`. Self-update check (`trqsh update`) against the release feed
  (Part 08). Cross-platform paths for config, socket, and inspector.

## Interfaces honored (do not modify)
- `pkg/tunnel` (§7), `pkg/proto` (§6), error codes (§8), config schema (§10). The **agent-core API**
  (`Core`, `TunnelSpec`, `Tunnel`, `Event`) is defined here and **frozen for Part 04** — keep it stable.

## Done criteria
- `go build ./cmd/trqsh`; `go test ./internal/agent/...` passes (unit + a loopback integration test
  against a fake edge or the real `trqshd` with `StubEntitlements`).
- `trqsh http 3000` against a running edge yields a working public URL; killing the network and
  restoring it auto-reconnects and restores the URL.
- Inspector at `:4040` shows requests and can replay one.
- Local control API drives start/stop/list/events (exercised by a small test client).

## Run / verify
```bash
# terminal 1: local service
python3 -m http.server 3000
# terminal 2: edge (Part 02) with stub entitlements + redis
TRQSH_ENTITLEMENTS=stub TRQSH_BASE_DOMAIN=lvh.me go run ./cmd/trqshd
# terminal 3: the agent
go run ./cmd/trqsh http 3000 --server localhost:443
curl -k https://<printed-subdomain>.lvh.me   # returns the python server's listing
open http://127.0.0.1:4040                    # inspector shows the request
```
