# trqsh — Scheme (where everything lives)

_Snapshot: 2026-07-31. Short map, not full docs — see `plan/00-ARCHITECTURE.md` for real architecture._

## ⚠️ Important: this local folder's git remote is stale

On **2026-07-31** the old monorepo (this folder) was split into 4 separate repos.
This local checkout's `origin` still points at **`archive-trqsh`** — the frozen,
pre-split snapshot — not at the active repos. Local `HEAD` (`a15cc35`) matches
`archive-trqsh` HEAD exactly and the working tree is clean, so nothing local is lost,
but **pushing here no longer reaches the live repos**. Treat this folder as the
historical/combined view; do real work against the split repos below (or repoint
`origin` per-subfolder if you keep developing from here).

## GitHub — `trqsh-uz` organization

| Repo | Visibility | Contains | Local folder |
|---|---|---|---|
| **trqsh** | private (Go) | backend: edge, control API, CLI/agent, infra | repo root (`cmd/`,`internal/`,`pkg/`,`proto/`,`deploy/`,`docs/`,`plan/`,`packaging/`) |
| **site** | private (Next.js) | marketing site (trqsh.uz) | `web/site/` |
| **dashboard** | private (Next.js) | user/customer dashboard | `web/dashboard/` |
| **desktop** | private (Tauri: Rust+TS) | desktop GUI app | `desktop/` |
| **archive-trqsh** | private (Go) | frozen full pre-split monorepo history | ← this checkout's `origin` today |
| **cli** | public | published CLI release binaries only, no source | — |
| **gui** | public | published desktop installer downloads only, no source | — |
| **scoop-bucket** | private | Windows Scoop manifest for the CLI | — |
| **homebrew-tap** | private | Homebrew formula for the CLI | — |

## Local vs GitHub sync check

- `git status`: clean, `main` up to date with (stale) `origin` = `archive-trqsh`.
- **trqsh, site, desktop**: each has exactly 1 commit — a straight export of local
  commit `a15cc35`. Content matches local 1:1.
- **dashboard**: 1 commit **ahead** of local — added Dockerfile/CI workflows/dependabot/README
  (`fc5a037`) that don't exist in local `web/dashboard/`.
- Conclusion: local code **is** on GitHub (fully, across the 4 split repos); only
  `dashboard` has drifted slightly ahead upstream.

## Backend (→ `trqsh-uz/trqsh`) — Go

- `cmd/trqsh` — CLI/agent, the open-source client (`plan/03-agent-cli.md`)
- `cmd/trqshapi` — control-plane API: accounts, keys, domains, quotas, entitlements (`plan/05-control-api.md`)
- `cmd/trqshd` — edge server, public data plane / tunnel routing (`plan/02-edge-server.md`)
- `internal/` — `agent`, `api`, `billing`, `entitlerpc`, `server`
- `pkg/` — `authz`, `proto`, `tunnel` (shared packages)
- `proto/` — protobuf definitions
- `deploy/` — docker, helm, terraform, observability, apilb, loadtest
- `plan/` — architecture specs, `00-ARCHITECTURE.md` first
- `docs/`, `packaging/` (install.sh, npm, pypi, scoop, winget)

## Frontend / Desktop

- `web/site/` — marketing site, Next.js → **trqsh-uz/site**
- `web/dashboard/` — dashboard, Next.js → **trqsh-uz/dashboard**
- `desktop/` — desktop app, Tauri (Rust + TS, `src-tauri/`) → **trqsh-uz/desktop**

## Production

- Live on Kamatera VPS `185.227.108.173` (Ubuntu 24.04), `deploy/docker-compose.prod.yml`.
- `trqsh.uz` on Cloudflare (DNS-only), real Let's Encrypt TLS.
