# Step 2 — Part 01: Protocol & Transport

- **Date:** 2026-07-17
- **Step:** 2 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/01-protocol-transport.md`](../../plan/01-protocol-transport.md)
- **Milestone:** M0 — Foundation
- **Status:** ✅ Complete — `go build`, `go vet`, `go test ./pkg/...` all green (race gate: see note)

> **TL;DR (Uz):** Loyihaning **poydevori** yozildi — hamma qismlar import qiladigan **wire protokol**
> (`pkg/proto`), **multiplex transport** (`pkg/tunnel`, QUIC-birinchi + TCP/yamux zaxira), va
> **entitlements** tiplari (`pkg/authz`). Protobuf `buf` bilan generatsiya qilindi, length-prefixed
> codec, QUIC↔TCP avtomatik fallback, N ta parallel oqim echo testi, ping/pong — barchasi **yashil**.
> Yagona cheklov: `-race` uchun C kompilyatori kerak (bu mashinada yo'q). Keyingi qadam:
> **Part 02 — Edge server** va **Part 03 — Agent/CLI** (parallel boshlash mumkin).

## What was built

Part 01 delivers the three **frozen shared packages** every other part imports. Contracts follow
`plan/00-ARCHITECTURE.md` §6 (wire protocol), §7 (transport API), §8 (error taxonomy), §9 (entitlements).

```
proto/rift.proto            protobuf schema — package rift.v1, go_package .../pkg/proto
buf.yaml / buf.gen.yaml     buf v2 config (local protoc-gen-go, paths=source_relative)

pkg/proto/trqsh.pb.go        generated (buf) — Envelope oneof + all messages
pkg/proto/codec.go          length-prefixed framing: WriteMsg/ReadMsg, WriteStreamInit/ReadStreamInit
pkg/proto/errors.go         frozen ERR_* taxonomy constants + NewError()
pkg/proto/codec_test.go     round-trip, multi-frame, oversize (write+read), truncation

pkg/tunnel/tunnel.go        Kind, ALPNProto, Session/Stream/Listener interfaces, defaults
pkg/tunnel/quic.go          quic-go adapter: quicSession + quicStream (addr delegated to conn)
pkg/tunnel/tcp.go           yamux adapter: yamuxSession (ctx via CloseChan) + yamuxStream; yamuxConfig
pkg/tunnel/dial.go          Dialer — QUIC-first with automatic TCP+yamux fallback
pkg/tunnel/listen.go        Listen() — edge accepts QUIC + TCP on one Listener (fan-in)
pkg/tunnel/transport_test.go  echo over QUIC & TCP (20 concurrent streams), real fallback, ping/pong

pkg/authz/authz.go          Limits, BindRequest, Decision, Usage + Entitlements interface
pkg/authz/stub.go           StubEntitlements — allow-all for edge until Part 05 lands
```

## API surface delivered (the frozen glue)

- **Wire protocol** — `Envelope` with a `oneof` payload (`hello`, `auth_resp`, `bind`, `bind_resp`,
  `unbind`, `ping`, `pong`, `error`, `shutdown`) plus a separate `StreamInit` header that prefixes
  every data stream. Framing is a **uint32 big-endian length prefix + protobuf bytes**, bounded by
  `MaxFrameSize = 1 MiB` (`ErrFrameTooLarge` on either side).
- **Transport** — `Session { OpenStream, AcceptStream, Kind, RemoteAddr, CloseWithError, Context }`
  and `Stream { net.Conn; ID() uint64 }`. Two implementations behind one interface:
  - **QUIC** (`quic-go v0.60.0`): keepalive + `MaxIdleTimeout` + datagrams enabled; connection
    migration comes for free from QUIC.
  - **TCP + yamux** (`hashicorp/yamux v0.1.2`) over TLS: fallback when UDP/QUIC is blocked.
- **Dialer** — tries **QUIC first**, transparently falls back to **TCP+yamux** on the same address.
  `ForceKind` pins one transport (used by tests/diagnostics); `Prefer` chooses the primary.
- **Listener** — `Listen(quicAddr, tcpAddr, tlsConf)` fronts both acceptors and fans sessions into a
  single `Accept(ctx)`. Either address may be empty to run a single transport.
- **ALPN** — `"trqsh/1"` forced onto every QUIC/TLS handshake so intermediaries can version the wire.
- **Entitlements** — the `authz.Entitlements` seam (`Authenticate`, `CheckBind`, `ReportUsage`) with
  an allow-all `StubEntitlements`, so Part 02's edge runs today and swaps to real auth in Part 05.

## Key decisions

- **`quic.Stream` has no `LocalAddr`/`RemoteAddr`** → `quicStream` wraps it and delegates both to the
  owning `*quic.Conn` so it satisfies `net.Conn`. `ID()` comes from `StreamID()`.
- **yamux has no session `Context()`** → `yamuxSession` derives one from `CloseChan()` (a goroutine
  cancels it when the mux dies). It also has **no application error codes**, so `CloseWithError`
  degrades to a plain `Close()` (the code/reason are best-effort only) — documented in code.
- **yamux logs silenced** (`LogOutput = io.Discard`); errors surface through returned `error`s instead.
- **Fallback is address-shared:** QUIC (UDP) and TCP live on the **same port number** in production,
  so `Dialer.Dial(addr)` reuses one address for both attempts. The fallback test reserves a port and
  starts a TCP-only listener; the QUIC attempt times out (short `DialTimeout`) and TCP wins.
- **`buf.gen.yaml` must not use `clean: true`** — it deletes hand-written files that share `pkg/proto`
  (codec.go/errors.go/doc.go). Left off deliberately; noted here so regeneration stays safe.

## Verification

| Gate | Result |
|------|--------|
| `go mod tidy` | ✅ clean (direct deps: quic-go, yamux, protobuf) |
| `go build ./...` | ✅ success |
| `go vet ./...` | ✅ clean |
| `go test ./pkg/...` | ✅ **ok** — proto + tunnel pass |
| Concurrent-stream stress (`-count=8`) | ✅ pass (echo/fallback/pingpong ×8) |
| `go test ./pkg/... -race` | ⚠️ **not run here** — needs a C compiler (none installed; winget fetch failed on network) |

Tests covered: codec round-trip / multi-frame / oversize-write / oversize-read / truncation;
QUIC echo with 20 concurrent streams; TCP+yamux echo with 20 concurrent streams; **real QUIC→TCP
fallback**; ping/pong via the proto codec over a live stream.

Toolchain used: **Go 1.26.5** (`%LOCALAPPDATA%\trqsh-tools\go`), buf 1.71.0, protoc-gen-go.

### To run the race gate locally (recommended before Part 02/03 merge)
```powershell
winget install BrechtSanders.WinLibs.POSIX.UCRT   # or any mingw-w64 gcc on PATH
$env:CGO_ENABLED = "1"
go test ./pkg/... -race -count=1                    # expect: ok
```

## Known gaps / notes

- **Race detector pending** — code is written to be race-clean (per-stream goroutines, context-based
  session teardown, channel fan-in) and survived an ×8 concurrent stress run, but `-race` itself
  needs a C toolchain this machine lacks. Run the command above once a compiler is available.
- **Heartbeat policy is not in the transport** — `Ping`/`Pong` messages exist and are proven over a
  stream, but *who* sends them and how often (control-stream heartbeat loop) belongs to the agent/edge
  (Parts 02/03). QUIC/yamux keepalive already covers the socket layer.
- **`TRQSH_INSECURE=1`** dev skip-verify is intentionally **not** baked into `pkg/tunnel` (kept pure);
  it belongs to agent config (Part 03), which builds the client `*tls.Config`.
- **`make proto`** now works end-to-end (`buf generate`); regenerate after any `proto/rift.proto` edit.

## What's next

**Part 01 is the M0 gate and it is green.** Parts **02 (edge/`trqshd`)**, **03 (agent/CLI)**, and
**05 (control API)** may now start in parallel — 02 and 03 import `pkg/tunnel` + `pkg/proto` directly
and run against `authz.StubEntitlements` until Part 05 is ready. Use the ready-to-paste prompts for
Steps 3–5 in `plan/EXECUTION-ORDER.md`.
