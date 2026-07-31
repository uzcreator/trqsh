<div align="center">

# trqsh

**Your localhost, live on the internet.** A developer tunneling service — like ngrok or
Cloudflare Tunnel, but **QUIC-first** (faster on lossy/mobile links), with a **genuinely generous
free tier**, **UDP tunnels**, a **first-class desktop app**, and an **open-source agent**.

[![CI](https://github.com/trqsh-uz/trqsh/actions/workflows/ci.yml/badge.svg)](https://github.com/trqsh-uz/trqsh/actions/workflows/ci.yml)
[![Security](https://github.com/trqsh-uz/trqsh/actions/workflows/security.yml/badge.svg)](https://github.com/trqsh-uz/trqsh/actions/workflows/security.yml)
[![CodeQL](https://github.com/trqsh-uz/trqsh/actions/workflows/codeql.yml/badge.svg)](https://github.com/trqsh-uz/trqsh/actions/workflows/codeql.yml)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8)](https://go.dev)
[![License](https://img.shields.io/badge/agent-Apache--2.0-blue)](./LICENSE)

</div>

> `trqsh` is a working codename — rename before launch (trademark check first).

```console
$ trqsh http 3000
● session online   transport quic   region us-east
Forwarding  https://tidy-otter-4f2a.trqsh.uz  →  http://localhost:3000
Inspect     http://localhost:4040
```

## Why trqsh

- ⚡ **QUIC / HTTP-3 transport** — lower latency on lossy/mobile networks, connection migration
  across Wi-Fi ↔ 5G, with automatic **TCP + yamux fallback** where UDP is blocked.
- 🎁 **A free tier that isn't a trap** — deliberately more generous than ngrok's 2026 cuts.
- 🖥️ **A real desktop app** (macOS/Windows/Linux) with one-click tunnels and a live request
  inspector + replay.
- 🔌 **Every protocol, incl. UDP** — HTTP/HTTPS/TLS/TCP **and UDP** (ngrok has none).
- 🌐 **Custom domains + reserved subdomains**, teams/orgs, simple predictable pricing.
- 🔓 **Open-source agent** — audit it, script it, self-host it.

## Architecture

```text
 Developer machine            trqsh Cloud                        Public
 ┌──────────────┐   QUIC/TCP  ┌───────────────────────────┐    ┌─────────┐
 │ trqsh agent   │────mux──────▶│ edge (trqshd)             │◀───│ browser │
 │ localhost:3k │             │  ingress + vhost/SNI router│    │ *.trqsh.uz│
 │ inspector    │             │  registry (Redis)          │    └─────────┘
 └──────────────┘             │  ┌──────────────────────┐  │
                              │  │ control API (trqshapi)│──┼──▶ Postgres
                              │  │ auth · quotas · domains│  │   Redis
                              │  └──────────┬───────────┘  │
                              │       Stripe │ billing      │
                              └──────────────┴─────────────┘
```

- **agent** (`cmd/trqsh`) opens one authenticated, multiplexed QUIC session to the nearest **edge**,
  registers tunnels, and forwards streams to local services.
- **edge** (`cmd/trqshd`) accepts public traffic, resolves subdomain/SNI → session via a Redis
  registry, and welds the two connections.
- **control plane** (`cmd/trqshapi`) owns identity, API keys, domains, quotas, and billing, and
  enforces entitlements at the edge on every bind.

## Quickstart

```bash
# macOS / Linux
curl -fsSL https://trqsh.uz/install.sh | sh
# Windows
scoop install trqsh

trqsh login
trqsh http 3000     # → a public HTTPS URL for localhost:3000
```

Full docs and an API reference live on the site (`web/site` → `/docs`), and the control API serves
its own interactive Swagger UI at **`/docs`**.

## Local development

```bash
make dev          # full local stack: postgres, redis, migrate, api, edge, mailhog
make build        # build all Go binaries
make test         # go test ./... -race
make lint         # golangci-lint
```

Run a public URL with **no cloud** (pure Go, in-memory registry + store):

```bash
TRQSH_ENTITLEMENTS=stub TRQSH_BASE_DOMAIN=lvh.me go run ./cmd/trqshd   # edge
go run ./cmd/trqsh http 3000 --server 127.0.0.1:4443 --insecure     # agent
curl -H 'Host: <sub>.lvh.me' http://127.0.0.1
```

Frontends: `make site` (:3002), `pnpm dev` in `web/dashboard` (:3000), and `pnpm tauri dev` in `desktop/` (the native Tauri app).

## Security

trqsh routes other people's traffic, so security is a first-class concern: TLS everywhere,
argon2id-hashed API keys, HMAC-pinned JWTs, constant-time comparisons, bounded protocol frames,
per-IP rate limiting, server timeouts, and a **fail-closed production config** (`TRQSH_ENV=production`
refuses to boot on dev-default secrets). CI runs gosec, govulncheck, CodeQL, and Trivy.

See **[SECURITY.md](./SECURITY.md)** for the full posture, the operator hardening checklist, and how
to report a vulnerability.

## Deployment

Everything to ship and scale lives in **[`deploy/`](./deploy/)**: multi-stage Dockerfiles,
`docker-compose` for the full local stack, a Helm chart (edge DaemonSet, API HPA/PDB, ingress,
migrate hook, NetworkPolicy), Terraform (DigitalOcean: DOKS + managed Postgres/Redis + per-region
edge droplets + wildcard DNS), and GitHub Actions for CI, images, releases, and deploys.

## Repository layout

```text
cmd/{trqshd,trqsh,trqshapi}        edge, agent/CLI, control-plane binaries
pkg/{proto,tunnel,authz}        shared frozen contracts (wire protocol, transport, entitlements)
internal/{server,agent,api,billing}   edge · agent core · control plane · billing
desktop/                        Tauri v2 desktop app (React UI over the bundled Go agent)
web/{dashboard,site}            Next.js dashboard + marketing site
deploy/                         docker, helm, terraform, CI/CD, observability, secrets
docs/                           engineering docs, API spec, build log
plan/                           architecture + build specs (frozen contracts)
```

## Contributing

See **[CONTRIBUTING.md](./CONTRIBUTING.md)** for setup, standards, and the PR checklist, and
**[CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)**.

## License

The client **agent** and shared libraries (`cmd/trqsh`, `internal/agent`, `pkg/*`) are licensed under
**[Apache-2.0](./LICENSE)**. The hosted edge, control plane, and billing are proprietary.
