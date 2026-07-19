# Step 1 — Bootstrap scaffold

- **Date:** 2026-07-17
- **Step:** 1 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Milestone:** M0 — Foundation
- **Status:** ✅ Scaffold complete · ⏳ `go build` gate pending (Go not installed on this machine)

> **TL;DR (Uz):** Loyiha skeleti yaratildi — Go moduli, papka tuzilmasi, dev muhit (Postgres+Redis),
> Makefile, litsenziya va hujjatlar. Docker compose tekshirildi (yaroqli). `go build` ni ishga tushirish
> uchun **Go o'rnatilishi kerak** (hozir o'rnatilmagan). Keyingi qadam: **Part 01 — protocol & transport**.

## What was built

The empty repository was turned into a compiling Go monorepo skeleton matching the layout in
`plan/00-ARCHITECTURE.md` §4. No part logic was implemented — only scaffolding.

```
go.mod                      module github.com/rift/rift  (go 1.23)
go.work                     workspace (use .)
Makefile                    help, proto, build, test, lint, tidy, dev, run-edge, run-agent
.gitignore                  Go + Node + Wails + secrets
LICENSE                     Apache-2.0 (open-source client)
README.md                   repo overview + status + pointers to plan/
cmd/riftd/main.go           edge binary stub (empty main, TODO Part 02)
cmd/rift/main.go            agent/CLI binary stub (empty main, TODO Part 03)
pkg/proto/doc.go            package stub — FROZEN contract, Part 01
pkg/tunnel/doc.go           package stub — FROZEN contract, Part 01
pkg/authz/doc.go            package stub — FROZEN contract, Part 01
internal/server/doc.go      package stub, Part 02
internal/agent/doc.go       package stub, Part 03
internal/api/doc.go         package stub, Part 05
internal/billing/doc.go     package stub, Part 07
proto/.gitkeep              rift.proto lands here in Part 01
gui/.gitkeep                Wails app, Part 04
web/dashboard/.gitkeep      Next.js dashboard, Part 06
web/site/.gitkeep           Next.js marketing site, Part 09
deploy/docker-compose.dev.yml   Postgres 16 + Redis 7 (healthchecks, named volumes)
docs/DEVELOPMENT.md         pinned toolchain + common commands
docs/build-log/             this build log
```

## Key decisions

- **Go module path:** `github.com/rift/rift` — fixed now, do not change. Updated
  `plan/00-ARCHITECTURE.md` to replace the `<org>` placeholder with `rift` (paths + `go_package`).
- **Go version floor:** `go 1.23` in `go.mod` (matches the stack decision; newer toolchains are fine).
- **`go.work` present** even though there is a single module today, so the open-source client
  (`cmd/rift` + `pkg/*`) can later split into its own module without churn.
- **License:** Apache-2.0 for the client/shared libs; the hosted edge/control/billing stay proprietary.
- **Dev deps:** Postgres **16** + Redis **7** via compose, with healthchecks and named volumes so
  Parts 02/05 can run immediately. Part 08 will extend this file with edge/api/mailhog.
- **Binary stubs use an empty `func main()`** so `go build ./...` will pass as-is once Go is present.

## Verification

| Gate | Result |
|------|--------|
| Directory tree matches `00-ARCHITECTURE.md` §4 | ✅ (21 files created, confirmed) |
| `docker compose -f deploy/docker-compose.dev.yml config` | ✅ valid |
| `go build ./...` | ⏳ **pending** — Go is **not installed** on this machine |

Detected toolchain at bootstrap: **Go — not found**, Node v24.13.0, Docker 29.4.3, Git 2.53.0.

### To finish the Step 1 gate (install Go, then run)
```powershell
winget install GoLang.Go        # or download from https://go.dev/dl/ (need 1.23+)
# reopen the shell, then from the repo root:
go build ./...                  # expect: success (empty binaries + package stubs)
go vet ./...                    # expect: clean
```

## Known gaps / notes

- **Go must be installed** before Part 01 (protocol/transport). Nothing else blocks.
- **`make` on Windows** is not built in — install it (`scoop install make`) or run the `Makefile`
  commands directly. Documented in `docs/DEVELOPMENT.md`.
- **Git not initialized yet.** When ready: `git init && git add -A && git commit -m "chore: bootstrap scaffold"`.
  (Not done automatically — left to you.) Recommended before starting parallel parts, which use
  branches/worktrees per `plan/EXECUTION-ORDER.md`.
- `make proto` will fail until Part 01 adds `proto/rift.proto` — expected.

## What's next

**Step 2 — Part 01: Protocol & Transport** (`plan/01-protocol-transport.md`). It is the foundation
that everything else imports; do not start Parts 02/03/05 until its `go test ./pkg/...` gate is green.
Use the ready-to-paste prompt for Step 2 in `plan/EXECUTION-ORDER.md`.
