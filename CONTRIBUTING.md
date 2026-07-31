# Contributing to trqsh

Thanks for your interest in improving trqsh. The **agent** (`cmd/trqsh`, `internal/agent`) and the
shared libraries (`pkg/*`) are open source; the edge, control plane, and billing are also in this
repo for development but are proprietary. Contributions are welcome across all of it.

## Development setup

Prerequisites (pinned in [`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)):

- **Go 1.25+**
- **Node 24 + pnpm 10** (frontends)
- **Docker** (local stack, image builds)

```bash
make dev            # full local stack: postgres, redis, migrate, api, edge, mailhog
make build          # build all Go binaries
make test           # go test ./... -race
make lint           # golangci-lint
```

Run a public URL against a local server, no cloud needed:

```bash
# terminal 1 — edge (in-memory registry, stub entitlements)
TRQSH_ENTITLEMENTS=stub TRQSH_BASE_DOMAIN=lvh.me go run ./cmd/trqshd
# terminal 2 — agent
go run ./cmd/trqsh http 3000 --server 127.0.0.1:4443 --insecure
curl -H 'Host: <sub>.lvh.me' http://127.0.0.1
```

Frontends:

```bash
cd web/site      && pnpm install && pnpm dev        # :3002
cd web/dashboard && pnpm install && pnpm dev        # :3000
cd desktop       && pnpm install && pnpm tauri dev  # native desktop app (Tauri v2)
```

## Ground rules

- **Frozen contracts.** The shared contracts in [`plan/00-ARCHITECTURE.md`](./plan/00-ARCHITECTURE.md)
  (wire protocol, transport API, `authz.Entitlements`, config schema, error taxonomy, plan catalog)
  change only by editing that file first. Don't drift them.
- **Generated code stays in sync.** After editing the plan catalog run `make site-plans`; after
  editing the OpenAPI run `make openapi-sync`. CI fails on drift.
- **Match the surrounding style.** Go is `gofmt`/`goimports`-clean; TS mirrors Go JSON tags in
  **snake_case** (the Go APIs marshal with `encoding/json`, so the TS shapes track the Go field tags).

## Before you open a PR

```bash
gofmt -l .          # must print nothing
go vet ./...
go build ./... && go test ./...
```

- Keep changes focused; add tests for behavior changes.
- Never commit secrets. Real values come from the environment / SOPS, never source.
- Update docs and the relevant `docs/build-log/` entry when behavior changes.

## Reporting security issues

Do **not** file public issues for vulnerabilities — see [`SECURITY.md`](./SECURITY.md).

## Commit messages

Conventional, imperative present tense (`add edge drain timeout`, `fix quota off-by-one`). Group
related changes; explain the *why* in the body when non-obvious.
