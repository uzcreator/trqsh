# 00 — Master Architecture & Frozen Contracts

> **Read this before touching any code.** Every other part depends on the decisions and contracts
> here. If you must change a contract, change it *here first* and announce it in your part's spec.

## 1. What we are building

trqsh is a hosted developer tunneling service. A local **agent** exposes a `localhost` service to
the public internet through trqsh's **edge** servers, without port-forwarding. Revenue comes from a
**SaaS subscription**; the **agent is open source**, the edge/control/billing are proprietary.

### Differentiators (design every feature to protect these)
1. **QUIC-first transport** → lower latency on lossy/mobile links + connection migration; automatic
   **TCP+yamux fallback** where UDP/QUIC is blocked.
2. **Generous free tier** (see §9 plan catalog) — a direct wedge against ngrok's 2026 cuts.
3. **First-class desktop GUI** (Wails) + built-in **request inspector/replay** (`localhost:4040`).
4. **UDP tunnels** (ngrok has none) alongside HTTP/HTTPS/TLS/TCP.
5. **Instant custom domains + reserved subdomains**, teams/orgs, simple predictable pricing.

## 2. Glossary

- **Agent** — `trqsh` binary running on the developer's machine (also embedded in the GUI).
- **Edge / `trqshd`** — public server accepting agent sessions and public traffic; routes between them.
- **Control plane** — the API + DB owning identity, keys, domains, quotas, billing.
- **Session** — one authenticated, multiplexed connection agent↔edge (QUIC or TCP+yamux).
- **Tunnel** — one bound route (e.g. `myapp.trqsh.uz` → `localhost:3000`).
- **Stream** — one multiplexed channel within a session (control stream, or one per public connection).
- **Registry** — edge's map of `hostname/port → session` (Redis-backed, shared across edges).

## 3. Tech stack (locked — do not substitute without updating this file)

| Layer | Choice |
|---|---|
| Core language | **Go 1.23+**, Go workspace (`go.work`) |
| Transport | **quic-go** (QUIC/HTTP-3) primary + **hashicorp/yamux** over TLS-TCP fallback |
| Wire codec | **Protobuf** (`google.golang.org/protobuf`), length-prefixed frames |
| vhost/SNI | `inconshreveable/go-vhost` or a custom SNI/Host peeker |
| TLS / certs | **CertMagic** (`github.com/caddyserver/certmagic`) with DNS-01 wildcard |
| Cache / routing state | **Redis** (`github.com/redis/go-redis`) |
| Control DB | **Postgres**; queries via **sqlc**; migrations via **goose** |
| HTTP API framework | **chi** (`github.com/go-chi/chi`) + `net/http` |
| Desktop GUI | **Wails v3** (Go + **React + TypeScript + Tailwind + shadcn/ui**) |
| Web (dashboard + site) | **Next.js 14+ (App Router) + TypeScript + Tailwind + shadcn/ui** |
| Billing | **Stripe** (`github.com/stripe/stripe-go`) |
| Logging | `log/slog` (JSON in prod) |
| Metrics/trace | **OpenTelemetry** + **Prometheus** |
| Infra | **Docker**, **Kubernetes + Helm**, **Terraform**, **GitHub Actions** |
| Auth | agent→edge via **API keys**; dashboard via **JWT** sessions + OAuth (GitHub/Google) |

## 4. Monorepo layout (repo root = the folder containing `plan/`)

```
<repo-root>/
├── go.work                      # Go workspace
├── go.mod (module github.com/trqsh-uz/trqsh)
├── plan/                        # these build specs (this folder)
├── proto/                       # .proto source files (Part 01 owns)
├── cmd/
│   ├── trqshd/                   # edge binary            (Part 02)
│   └── trqsh/                    # agent + CLI binary     (Part 03)
├── pkg/                         # SHARED — Part 01 owns; others import read-only
│   ├── proto/                   # generated + codec      (Part 01)
│   ├── tunnel/                  # QUIC+TCP mux transport (Part 01)
│   └── authz/                   # entitlement types+iface(Part 01; impl in 05/07)
├── internal/
│   ├── server/                  # edge data plane        (Part 02)
│   ├── agent/                   # agent core library     (Part 03)
│   ├── api/                     # control plane API + DB (Part 05)
│   └── billing/                 # Stripe, plans, metering(Part 07)
├── gui/                         # Wails v3 app           (Part 04)
├── web/
│   ├── dashboard/               # Next.js authed app     (Part 06)
│   └── site/                    # Next.js marketing+docs (Part 09)
├── deploy/                      # docker, helm, terraform, CI (Part 08)
└── docs/                        # engineering + API docs (cross-cutting; 09 curates)
```

**Module strategy:** single Go module `github.com/trqsh-uz/trqsh` at the root is simplest; a `go.work`
is included for future multi-module splits (e.g. open-sourcing `pkg/*` + `cmd/trqsh` separately).
The module path is fixed to `github.com/trqsh-uz/trqsh` (chosen at bootstrap); never change it.

## 5. Dev environment bootstrap (Part 00 / first session sets this up)

- `go.work` + root `go.mod` (`module github.com/trqsh-uz/trqsh`, `go 1.23`).
- `deploy/docker-compose.dev.yml` running **Postgres 16** and **Redis 7** for local dev (Part 08
  authors the full file; Part 00 may add a minimal version so Parts 02/05 can run immediately).
- `Makefile` (or `Taskfile.yml`) targets: `proto` (regenerate), `build`, `test`, `lint` (golangci-lint),
  `run-edge`, `run-agent`, `dev` (compose up + edge + api).
- Toolchain pins: `protoc` + `protoc-gen-go`, `sqlc`, `goose`, `golangci-lint`, Node 20 + pnpm,
  Wails v3 CLI. List exact versions in `docs/DEVELOPMENT.md`.
- Local TLS for dev: edge uses a self-signed CA (or mkcert); agent trusts it via `TRQSH_INSECURE=1`
  for dev only. Never in prod.

## 6. FROZEN CONTRACT — Wire protocol (`proto/rift.proto` → `pkg/proto`)

Two kinds of streams inside a session:
- **Control stream** — the first stream; carries length-prefixed `Envelope` protobuf messages both ways.
- **Data stream** — one per public connection; begins with a single `StreamInit` header frame, then raw bytes are copied verbatim.

```proto
syntax = "proto3";
package rift.v1;
option go_package = "github.com/trqsh-uz/trqsh/pkg/proto;proto";

// ---- Control-stream messages (length-prefixed: uint32 BE length + bytes) ----
message Envelope {
  oneof msg {
    Hello       hello        = 1;   // agent -> edge (first)
    AuthResp    auth_resp    = 2;   // edge  -> agent
    BindTunnel  bind         = 3;   // agent -> edge
    BindResp    bind_resp    = 4;   // edge  -> agent
    Unbind      unbind       = 5;   // agent -> edge
    Ping        ping         = 6;   // both
    Pong        pong         = 7;   // both
    ErrorMsg    error        = 8;   // both
    ShutdownMsg shutdown     = 9;   // edge  -> agent (drain)
  }
}

message Hello {
  string protocol_version = 1;      // "1"
  string agent_version    = 2;
  string os               = 3;      // "darwin|linux|windows"
  string arch             = 4;
  string api_key          = 5;      // tq_live_... (auth in the Hello)
  string region_hint      = 6;      // "auto" or region code
  repeated string features = 7;     // e.g. "udp","tcp","quic"
}

message AuthResp {
  bool   ok         = 1;
  string account_id = 2;
  string plan       = 3;            // "free|pro|team|payg"
  string session_id = 4;            // server-assigned, for logs/inspector
  ErrorMsg error    = 5;            // set when ok=false
}

enum TunnelType { HTTP = 0; HTTPS = 1; TLS = 2; TCP = 3; UDP = 4; }

message BindTunnel {
  string     client_tunnel_id = 1;  // agent-chosen local id (unique per session)
  TunnelType type             = 2;
  string     subdomain        = 3;  // requested; empty => random
  string     custom_domain    = 4;  // optional, must be verified in control plane
  uint32     remote_port      = 5;  // for TCP/UDP; 0 => ephemeral assigned
  map<string,string> options  = 6;  // e.g. "basic_auth","host_header","schemes"
}

message BindResp {
  string client_tunnel_id = 1;
  bool   ok               = 2;
  string public_url       = 3;      // https://abc.trqsh.uz  or  tcp://edge:2222
  string assigned_host    = 4;
  uint32 assigned_port    = 5;
  ErrorMsg error          = 6;      // set when ok=false (see §8 codes)
}

message Unbind   { string client_tunnel_id = 1; }
message Ping     { int64 ts_unix_ms = 1; }
message Pong     { int64 ts_unix_ms = 1; }
message ShutdownMsg { string reason = 1; int64 drain_deadline_unix_ms = 2; }

message ErrorMsg { string code = 1; string message = 2; } // code = §8 taxonomy

// ---- Data-stream header (first frame on every data stream, then raw bytes) ----
message StreamInit {
  string client_tunnel_id = 1;      // which tunnel this data stream serves
  string remote_addr      = 2;      // public client ip:port (for inspector/logs)
  string proto            = 3;      // "http|https|tcp|udp"
  map<string,string> meta = 4;      // e.g. sni, alpn
}
```

**Codec API (Part 01 provides, everyone uses):**
```go
package proto
func WriteMsg(w io.Writer, m *Envelope) error   // uint32 BE length prefix + marshal
func ReadMsg(r io.Reader) (*Envelope, error)
func WriteStreamInit(w io.Writer, s *StreamInit) error
func ReadStreamInit(r io.Reader) (*StreamInit, error)
```

## 7. FROZEN CONTRACT — Transport API (`pkg/tunnel`)

```go
package tunnel

type Kind string
const ( KindQUIC Kind = "quic"; KindTCP Kind = "tcp" )

// A multiplexed session between agent and edge.
type Session interface {
    OpenStream(ctx context.Context) (Stream, error)    // initiator side
    AcceptStream(ctx context.Context) (Stream, error)  // responder side
    Kind() Kind
    RemoteAddr() net.Addr
    CloseWithError(code uint32, msg string) error
    Context() context.Context   // canceled when the session dies
}

type Stream interface {
    net.Conn        // Read, Write, Close, SetDeadline...
    ID() uint64
}

// Agent side: dial with QUIC first, fall back to TCP+yamux.
type Dialer struct {
    TLSConfig   *tls.Config
    Prefer      Kind          // KindQUIC (default) | KindTCP
    ForceKind   Kind          // "" = allow fallback; set to pin one transport
    DialTimeout time.Duration
    KeepAlive   time.Duration
}
func (d *Dialer) Dial(ctx context.Context, addr string) (Session, error)

// Edge side: accept both QUIC and TCP sessions on the same port set.
type Listener interface {
    Accept(ctx context.Context) (Session, error)
    Addr() net.Addr
    Close() error
}
func Listen(quicAddr, tcpAddr string, tlsConf *tls.Config) (Listener, error)
```

**Rules:** the control stream is the **first stream** opened by the agent right after the session
is established. Data streams are opened by the **edge** toward the agent. Heartbeats use `Ping/Pong`
on the control stream; also rely on QUIC keepalive. On fallback, everything above `Session` is
identical — callers never branch on `Kind()` except for metrics.

## 8. FROZEN CONTRACT — Error taxonomy (`pkg/proto` constants)

String codes carried in `ErrorMsg.code` and surfaced by CLI/GUI/API:

```
ERR_AUTH_REQUIRED       ERR_AUTH_INVALID        ERR_VERSION_UNSUPPORTED
ERR_QUOTA_TUNNELS       ERR_QUOTA_BANDWIDTH     ERR_QUOTA_REQUESTS
ERR_SUBDOMAIN_TAKEN     ERR_SUBDOMAIN_FORBIDDEN ERR_DOMAIN_UNVERIFIED
ERR_PLAN_FORBIDS        ERR_PROTO_UNSUPPORTED   ERR_PORT_UNAVAILABLE
ERR_RATE_LIMITED        ERR_INTERNAL            ERR_UPSTREAM_UNREACHABLE
```

Each maps to a human message + a docs URL slug. `ERR_PLAN_FORBIDS` etc. must include the plan
needed so the GUI/dashboard can offer an upgrade CTA.

## 9. FROZEN CONTRACT — Entitlements interface (`pkg/authz`)

The **seam** between the edge (Part 02) and the control/billing plane (Parts 05/07). Part 02
compiles against this interface and ships a `StubEntitlements` (allow-all, for dev/MVP). Part 05
provides the real implementation backed by Postgres + Part 07 quotas.

```go
package authz

type Limits struct {
    MaxConcurrentTunnels   int
    MaxBandwidthBytesMo    int64  // 0 = unlimited
    MaxRequestsMo          int64  // 0 = unlimited
    AllowCustomDomains     bool
    AllowReservedSubdomain bool
    AllowTCP               bool
    AllowUDP               bool
    RateLimitRPS           int    // 0 = unlimited
}

type BindRequest struct {
    APIKey      string
    Type        string  // "http|https|tls|tcp|udp"
    Subdomain   string
    CustomHost  string
    RemotePort  int
    Region      string
}

type Decision struct {
    Allow             bool
    AccountID         string
    Plan              string
    Limits            Limits
    AssignedSubdomain string  // server may assign a random one
    ErrorCode         string  // §8 code when Allow=false
    ErrorMessage      string
}

type Usage struct {
    AccountID   string
    TunnelID    string
    BytesIn     int64
    BytesOut    int64
    Requests    int64
    WindowStart time.Time
    WindowEnd   time.Time
}

type Entitlements interface {
    Authenticate(ctx context.Context, apiKey string) (accountID, plan string, err error)
    CheckBind(ctx context.Context, req BindRequest) (Decision, error)
    ReportUsage(ctx context.Context, u Usage) error
}
```

## 10. FROZEN CONTRACT — Agent config schema (`~/.trqsh-uz/trqsh.yml`)

```yaml
version: 1
api_key: "tq_live_xxx"          # or env TRQSH_API_KEY, or `trqsh login`
server: "edge.trqsh.uz:443"     # default endpoint (region router resolves nearest)
region: "auto"                  # auto | us | eu | ap ...
transport: "auto"               # auto | quic | tcp
tunnels:
  web:
    proto: http                 # http|https|tls|tcp|udp
    addr: "localhost:3000"
    subdomain: "myapp"          # optional; reserved subdomains need Pro
    basic_auth: "user:pass"     # optional
  ssh:
    proto: tcp
    addr: "localhost:22"
    remote_port: 2222           # optional reserved port
inspector:
  enabled: true
  addr: "127.0.0.1:4040"
log_level: "info"               # debug|info|warn|error
```

Precedence: CLI flags > env (`TRQSH_*`) > config file > defaults.

## 11. FROZEN CONTRACT — Plan / quota catalog (numbers tunable in Part 07)

| Feature | Free | Pro (~$8/mo) | Team (~$20/user/mo) | Pay-as-you-go |
|---|---|---|---|---|
| Concurrent tunnels | 3 | 10 | 25/user | metered |
| Bandwidth / mo | 10 GB | 200 GB | 1 TB pooled | metered |
| Reserved subdomains | 1 | 10 | 50 | metered |
| Custom domains | ✗ | 5 | 50 | metered |
| Protocols | HTTP/S, TCP | + UDP, TLS | all | all |
| Request inspector history | 1 h | 30 days | 30 days | 30 days |
| Team seats / SSO | ✗ | ✗ | ✓ (SSO/SAML) | option |
| Support | community | email | priority | priority |

Free is deliberately more generous than ngrok's 2026 tier (a differentiator). Concrete Stripe
price IDs, metering, and enforcement live in [`07-billing-monetization.md`](./07-billing-monetization.md);
the enforcement path is the `authz.Entitlements` interface (§9).

## 12. Control REST API surface (defined fully in Part 05; listed here for planning)

Base `https://api.trqsh.uz/v1`. JWT (dashboard) or API key (programmatic). Key groups:
`/auth/*` (oauth, session), `/account`, `/api-keys`, `/tunnels` (active, from Redis), `/subdomains`
(reserve), `/domains` (add/verify custom), `/usage`, `/billing/*` (Part 07), `/orgs` + `/members`.
Full OpenAPI is authored in Part 05 and consumed by 02 (internal entitlement RPC), 03, 06, 07, 09.

## 13. Dependency graph & milestones (mirror of README)

```
00 ──▶ 01 ──┬──▶ 02 ─┐
            ├──▶ 03 ─┼─▶ M1 MVP (public URL) ──▶ 04 GUI
            └──▶ 05 ─┴─▶ 06 dashboard
                     └─▶ 07 billing ─▶ enforcement into 02
08 infra + 09 site: parallel throughout.
```
M0 = 00,01 · M1 = 02,03,min-05,min-08 · M2 = 04,06,07 · M3 = full-08,09.

## 14. Coordination & change rules

- **Exclusive directory ownership** per the layout in §4; never edit another part's dirs.
- **Contracts (§6–§11) are frozen.** Change them only by editing this file and flagging it in your
  part's spec header ("CONTRACT CHANGE").
- **Interfaces over implementations at every seam** (`authz.Entitlements`, agent-core API, control
  REST API) so parts stub their dependencies and progress independently.
- **Versioning:** `Hello.protocol_version="1"`. Any breaking wire change bumps it and the edge must
  support N and N-1 during rollout.

## 15. Security baseline (applies to all parts)

- TLS everywhere (agent↔edge, public↔edge, browser↔api). No plaintext in prod.
- API keys are high-entropy, hashed at rest (argon2id/bcrypt), shown once, revocable.
- Per-account rate limits + global DDoS protections at the edge.
- Tenant isolation: a session can only bind subdomains/domains/ports its account is entitled to.
- Secrets via env/secret manager (Part 08), never committed. Stripe webhook signatures verified.
- Abuse controls: phishing/malware domain screening on public hostnames; report/abuse pathway.

## 16. Verification per milestone

- **M0:** `make proto && go build ./... && go test ./pkg/...` green; transport test covers QUIC,
  forced-TCP fallback, and N concurrent echo streams.
- **M1:** `make dev` (compose + edge + agent); local `:3000` server; `trqsh http 3000` → `curl`
  the public URL returns the local body; drop/restore network → auto-reconnect; `trqsh tcp 22` via
  ssh; UDP echo.
- **M2:** GUI starts a tunnel end-to-end; dashboard shows it; Stripe test-mode upgrade lifts a quota.
- **M3:** Helm deploy to staging; Let's Encrypt **staging** wildcard issues; Grafana shows metrics;
  release workflow emits signed installers; site signup funnels into a real account.
