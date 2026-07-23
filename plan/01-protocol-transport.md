# 01 — Protocol & Transport Core

**Owns:** `proto/`, `pkg/proto`, `pkg/tunnel`, `pkg/authz`
**Depends on:** [`00-ARCHITECTURE.md`](./00-ARCHITECTURE.md) (§6 wire protocol, §7 transport, §8 errors, §9 entitlements)
**Blocks:** Part 02 (edge), Part 03 (agent). **This is the foundation — build and freeze it first.**

> Read `00-ARCHITECTURE.md` fully first. This part turns §6–§9 of that document into working Go
> code. After this part is done, those contracts are frozen; 02 and 03 build on top without changing them.

## Goal

A reusable Go library that both the edge and the agent import:
1. The **wire protocol** (Protobuf messages + framed codec) from §6.
2. A **multiplexed transport** (`Session`/`Stream`) with a **QUIC-first dialer** and **TCP+yamux
   fallback**, plus an edge-side **listener** that accepts both.
3. The **entitlements types + interface** (§9) and an allow-all **stub** so the edge can run before
   the control plane exists.

## Scope / task breakdown

### T1 — Protobuf schema & codec (`proto/rift.proto`, `pkg/proto`)
- Author `proto/rift.proto` exactly as in §6 (package `rift.v1`, `go_package` → `pkg/proto`).
- Wire up generation: `protoc` + `protoc-gen-go` via a `make proto` target; output into `pkg/proto`.
- Hand-write the framing codec in `pkg/proto/codec.go`:
  - `WriteMsg(w io.Writer, m *Envelope) error` — `uint32` big-endian length prefix + `proto.Marshal`.
  - `ReadMsg(r io.Reader) (*Envelope, error)` — read length, guard against a `MaxFrameSize`
    (e.g. 1 MiB) to prevent abuse, read exactly N bytes, `proto.Unmarshal`.
  - `WriteStreamInit`/`ReadStreamInit` — same framing for the single data-stream header frame.
  - Export `const MaxFrameSize`, and error-code string constants from §8 in `pkg/proto/errors.go`
    (`CodeAuthInvalid = "ERR_AUTH_INVALID"`, …) plus a helper `func Error(code, msg string) *ErrorMsg`.

### T2 — Transport interfaces (`pkg/tunnel/tunnel.go`)
- Define `Kind`, `Session`, `Stream`, `Dialer`, `Listener` exactly as §7.
- `Stream` wraps the underlying mux stream and satisfies `net.Conn`; `ID()` returns the mux stream id.
- ALPN token `"trqsh/1"` for both QUIC and TLS-TCP so intermediaries can distinguish protocol versions.

### T3 — QUIC transport (`pkg/tunnel/quic.go`)
- Use `github.com/quic-go/quic-go`. A `quicSession` adapts `quic.Connection`:
  - `OpenStream` → `conn.OpenStreamSync`; `AcceptStream` → `conn.AcceptStream`.
  - Enable keepalive (`quic.Config.KeepAlivePeriod`), `MaxIdleTimeout`, and **connection migration**
    (quic-go supports address changes) — this is a headline feature; document it in code comments.
  - `Kind()` returns `KindQUIC`.

### T4 — TCP+yamux transport (`pkg/tunnel/tcp.go`)
- TLS over TCP (`crypto/tls`), then `github.com/hashicorp/yamux` for multiplexing.
- Agent = yamux **client**, edge = yamux **server** (keep initiator/responder consistent with QUIC).
- Configure yamux keepalive + accept backlog. `Kind()` returns `KindTCP`.

### T5 — Dialer with fallback (`pkg/tunnel/dial.go`)
- `Dialer.Dial(ctx, addr)`:
  1. If `ForceKind` set, use only that.
  2. Else if `Prefer==KindQUIC` (default): try QUIC with `DialTimeout`. On failure (UDP blocked,
     timeout, handshake error) **fall back** to TCP+yamux. Log which kind won (for metrics).
- Shared `*tls.Config` (with ALPN). Honor `TRQSH_INSECURE=1` (dev) to skip verify — never default-on.

### T6 — Listener accepting both (`pkg/tunnel/listen.go`)
- `Listen(quicAddr, tcpAddr, tlsConf)` starts a QUIC listener (UDP) **and** a TLS-TCP listener,
  fans accepted sessions into one `Accept(ctx)` channel. The edge (Part 02) calls this once.

### T7 — Entitlements types + stub (`pkg/authz`)
- `authz/authz.go`: `Limits`, `BindRequest`, `Decision`, `Usage`, `Entitlements` interface (copy §9).
- `authz/stub.go`: `StubEntitlements` — `Authenticate` accepts any non-empty key (returns
  `accountID="dev"`, `plan="pro"`); `CheckBind` returns `Allow=true` with generous `Limits`;
  `ReportUsage` no-ops. Used by Part 02 for the MVP and by tests.

### T8 — Tests (`pkg/tunnel/*_test.go`, `pkg/proto/*_test.go`)
- Codec round-trip (each Envelope variant + StreamInit; oversize frame rejected).
- Transport echo: start a `Listener`, `Dial`, open the control stream + N (e.g. 100) concurrent
  data streams, echo random payloads, assert integrity — run the suite **twice**: once `Prefer=QUIC`,
  once `ForceKind=TCP` (proves fallback path works identically).
- Fallback: point the dialer at a host with UDP "blocked" (QUIC dial fails fast) → assert it lands on TCP.
- Heartbeat: Ping/Pong round-trip over the control stream.

## Key implementation notes
- Keep `pkg/*` free of edge/agent business logic — only protocol + transport + entitlement types.
- The control stream is **always the first** stream opened by the agent; document this invariant.
- Data streams are opened by the **edge**; the agent only `AcceptStream`s them. Encode this in
  comments so Parts 02/03 stay consistent.
- No `panic` in library code; return wrapped errors. Use `context.Context` on every blocking call.

## Done criteria
- `make proto` regenerates cleanly; `go build ./pkg/...` and `go vet` pass.
- `go test ./pkg/...` green, including the QUIC run, the forced-TCP run, and the fallback test.
- `StubEntitlements` importable by Part 02 with zero extra deps.
- Public API matches §6–§9 exactly (no signature drift).

## Run / verify
```bash
make proto
go test ./pkg/... -race -count=1
```
Expected: transport echo passes for both QUIC and TCP; fallback test shows `Kind()==tcp` when QUIC
is blocked; codec rejects frames > MaxFrameSize.
