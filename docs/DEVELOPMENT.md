# Development setup

Toolchain and local workflow for trqsh. See [`../plan/`](../plan/) for the build specs and
[`build-log/`](./build-log/) for what each step delivered.

## Prerequisites (pinned)

| Tool | Version | Purpose | Install |
|------|---------|---------|---------|
| Go | **1.23+** | server, agent, CLI, control API | https://go.dev/dl/ or `winget install GoLang.Go` |
| Docker | 24+ | local Postgres/Redis, images | Docker Desktop |
| Node | **20+** (22/24 ok) | web dashboard + site, Wails frontend | `winget install OpenJS.NodeJS.LTS` |
| pnpm | 9+ | JS package manager | `npm i -g pnpm` |
| protoc | 25+ | compile `proto/*.proto` | `winget install protobuf` |
| protoc-gen-go | latest | Go protobuf plugin | `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` |
| sqlc | 1.25+ | typed SQL for the control plane (Part 05) | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |
| goose | 3+ | DB migrations (Part 05) | `go install github.com/pressly/goose/v3/cmd/goose@latest` |
| golangci-lint | 1.60+ | linting | https://golangci-lint.run/ |
| Wails | v3 | desktop GUI (Part 04) | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` |
| Stripe CLI | latest | billing webhooks (Part 07) | https://stripe.com/docs/stripe-cli |

> **Windows note:** `make` is not built in. Install it (`scoop install make` or `choco install make`),
> use Git Bash, or run the underlying commands from the `Makefile` directly.

## Common commands

```bash
make dev          # start Postgres + Redis (docker compose)
make build        # go build ./...
make test         # go test ./... -race
make lint         # golangci-lint
make proto        # regenerate protobuf (after Part 01 adds proto/rift.proto)
make run-edge     # run the edge with stub entitlements (after Part 02)
make run-agent ARGS="http 3000"   # run the agent (after Part 03)
```

## Module

Single Go module: **`github.com/trqsh-uz/trqsh`** (chosen at bootstrap — do not change). A `go.work` is
present so the open-source client (`cmd/trqsh` + `pkg/*`) can later split into its own module.

## Local TLS / DNS tips

- `lvh.me` and `*.lvh.me` resolve to `127.0.0.1` — use them for local vhost/subdomain testing
  without configuring DNS.
- For dev, the agent may trust a self-signed edge cert via `TRQSH_INSECURE=1`. **Never in production.**
