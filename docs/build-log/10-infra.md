# Step 10 — Part 08: Infrastructure, Deploy & Multi-Region Edge (`deploy/`, CI/CD)

- **Date:** 2026-07-18
- **Step:** 10 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/08-infra-deploy.md`](../../plan/08-infra-deploy.md)
- **Milestone:** M3 — Launch/scale
- **Status:** ✅ Complete — all six tracks delivered (**62 files**). Everything validated with the tooling
  available here: `docker compose config` ✅, PyYAML on every pure-YAML file ✅, JSON parse ✅, Helm
  template brace/if/end balance ✅, Terraform HCL balance ✅, `sh -n` + smoke on release scripts ✅.
  Image builds / `helm lint` / `terraform validate` run in CI (Docker Linux engine, helm, terraform not
  available in this box — documented, exercised by `ci.yml`).

> **TL;DR (Uz):** Butun mahsulot endi ishga tushiriladigan, cloud'ga deploy qilinadigan, релиз qilinadigan
> va kuzatiladigan. `make dev` — to'liq M1 stack (pg+redis+migrate+api+edge+mailhog) bitta buyruqda; `web`
> va `observability` profillar ixtiyoriy. 4 ta Dockerfile (distroless), Helm chart (edge DaemonSet host-net,
> api HPA/PDB, dashboard, ingress, migrate-hook, NetworkPolicy), DigitalOcean Terraform (DOKS + managed
> PG/Redis + har-region edge droplet + wildcard DNS + Spaces), GitHub Actions (CI + images→GHCR +
> goreleaser релиз + GUI sign/notarize + update-feed + helm deploy staging/prod), Prometheus/Grafana/OTel
> (real edge metrikalari) va SOPS secrets. Keyingi (oxirgi): Part 09 — marketing sayti + docs.

## What was built

`deploy/` (55 files) + `.github/workflows/` (4) + root `.goreleaser.yaml` / `.sops.yaml` / `.dockerignore`.

```
deploy/
├── docker-compose.dev.yml + .env.example   full M1 stack; profiles: web, observability
├── docker/         Dockerfile.{edge,api,migrate,dashboard}  (multi-stage, distroless/static)
├── helm/trqsh/      Chart + values{,.staging,.prod} + 16 templates
├── terraform/      DigitalOcean: DOKS, PG, Redis, edge droplets, DNS, Spaces (11 files)
├── observability/  Prometheus (scrape+alerts) + Grafana (datasource+dashboard) + OTel collector
├── release/        install.sh + gen-update-feed.sh + scoop manifest
└── secrets/        SOPS workflow + example Secret
.github/workflows/  ci.yml, images.yml, release.yml, deploy.yml
```

## How it works (per track)

- **T1 Containers.** Multi-stage, CGO-free builds → `gcr.io/distroless/static:nonroot` for the edge and
  API; a goose-based `migrate` image; a Node image for the dashboard. `docker-compose.dev.yml` brings the
  **whole M1 stack** up with correct wiring (edge → `TRQSH_ENTITLEMENTS=api` → API; API → Postgres/Redis;
  `migrate` gates the API via `service_completed_successfully`). Dashboard and the monitoring stack are
  opt-in **profiles** so `make dev` stays fast.
- **T2 Helm.** One chart. **Edge** is a host-networked **DaemonSet** (owns :80/:443 TCP+UDP + QUIC on every
  edge node; `dnsPolicy: ClusterFirstWithHostNet` so it still resolves the API Service; SIGTERM →
  graceful drain via `terminationGracePeriodSeconds`). **API** is a Deployment + **HPA** + **PDB** +
  `/healthz` probes. **Dashboard** Deployment/Service. **Ingress** (api + app hosts, cert-manager TLS).
  **Migrations** run as a `pre-install,pre-upgrade` hook **Job** before the API rolls. **NetworkPolicy**
  restricts the control API (internal entitlements RPC) to edge + dashboard + ingress only.
- **T3 Terraform.** DigitalOcean single-provider story: **DOKS** control cluster + managed **Postgres 16**
  + **Redis 7** (VPC + firewalls), **per-region edge droplets** (cloud-init runs the edge container on the
  host network) each with a **reserved IP**, the apex domain + **wildcard `*.trqsh.uz`** round-robined
  across edge IPs, and **Spaces** buckets (CertMagic cert cache + release artifacts). Sensitive DB/Redis
  URLs surface as outputs wired into the k8s Secret.
- **T4 CI/CD.** `ci.yml` (go build/**test -race**/vet/golangci-lint, both frontends, **helm lint+template**,
  **terraform fmt+validate**, compose config). `images.yml` (buildx → **GHCR**, semver+sha tags). `release.yml`
  (**goreleaser** CLI/edge archives+Homebrew+deb/rpm; **Wails GUI** per-OS with **macOS notarization** +
  **Windows Authenticode**; **auto-update feed** matching `gui/update.go`). `deploy.yml` (helm → **staging**
  on main, **prod** on tag behind an environment approval).
- **T5 Observability.** Prometheus scrapes the edge `/metrics` (already exposed by `internal/server`);
  alerts + a Grafana dashboard are built on the **real** metric names (`trqsh_sessions_active`,
  `trqsh_bytes_total{dir}`, `trqsh_agent_handshakes_total{kind}` for QUIC-vs-TCP, `trqsh_errors_total{kind}`,
  …). OTel collector receives OTLP and re-exports to Prometheus.
- **T6 Secrets.** SOPS (`.sops.yaml`, only `data`/`stringData` encrypted) as the git-secret workflow; the
  chart consumes a pre-created Secret in staging/prod (`secrets.create` is dev-only). Security posture doc
  (TLS everywhere, NetworkPolicy isolation, non-root/distroless, token rotation).

## Contracts honored (no drift)

Every env var and port comes straight from the binaries' own config, verified against source:

| Component | Ports | Key env |
|---|---|---|
| edge `trqshd` | 80, 443, 4443 (tcp+udp), 9090 | `TRQSH_{HTTP,HTTPS,QUIC,TCP,METRICS}_ADDR`, `TRQSH_ENTITLEMENTS=api`, `TRQSH_API_URL`, `TRQSH_INTERNAL_TOKEN`, `TRQSH_REDIS_URL`, `TRQSH_ACME_*`, `TRQSH_PORT_MIN/MAX` |
| API `trqshapi` | 8080 (`/healthz`) | `TRQSH_API_ADDR`, `TRQSH_DATABASE_URL`, `TRQSH_REDIS_URL`, `TRQSH_JWT_SECRET`, `TRQSH_INTERNAL_TOKEN`, `TRQSH_STRIPE_*` |
| migrate | — | `GOOSE_DBSTRING` (goose over `internal/api/db/migrations`) |

## Verification

| Check | Result |
|------|--------|
| `docker compose -f deploy/docker-compose.dev.yml config` | ✅ valid; default services = pg, redis, migrate, api, edge, mailhog |
| PyYAML parse (compose, Helm values, all observability + workflow YAML) | ✅ all pass |
| JSON parse (Grafana dashboard, scoop manifest) | ✅ |
| Helm template `{{ }}` + if/range/with↔end balance (17 files) | ✅ balanced |
| Terraform HCL brace/bracket/paren balance (10 files) | ✅ balanced |
| `sh -n` / `bash -n` on release scripts + update-feed smoke | ✅ emits valid feed JSON |
| Root `go build`/`go test ./...` (unchanged; no Go touched) | ✅ still green |

## Key decisions

- **Edges as per-region droplets** (not DOKS node pools) in the Terraform reference topology — DOKS is
  single-region, and QUIC/UDP + host networking + anycast want stable per-region reserved IPs. The Helm
  edge DaemonSet stays valid for single-region / self-hosted clusters (`edge.enabled` toggles it). This is
  the one honest architectural fork, documented in both READMEs.
- **distroless/static + non-root** for Go services; **goose image** for migrations run as a Helm hook so
  schema is always applied before the API starts (the API does not self-migrate).
- **Real metrics only.** The Grafana dashboard/alerts use the metrics the edge actually emits; **p50/p95
  latency and control-API `/metrics` are called out as gaps**, not faked.
- **Profiles keep `make dev` lean** — core M1 stack always; dashboard + observability opt-in.
- **Secrets never rendered by the chart in prod** — pre-created + SOPS/KMS; `secrets.create` is dev-only.

## Known gaps / notes

- **Not executed in this environment:** container image builds (Docker Linux engine offline here), `helm
  lint`/`helm template`, `terraform validate` — all wired into `ci.yml` and run there. Structure/format
  validated locally with PyYAML/JSON/balance checks.
- **GUI signing** needs the Apple notarization + Windows Authenticode secrets and self-hosted/hosted
  runners; steps are in `release.yml` as the integration points.
- **GeoDNS/anycast:** DO DNS round-robins the wildcard; true nearest-edge routing uses a GeoDNS provider
  (NS1/Route 53) in prod — noted in `terraform/README.md`.
- **Latency histograms + API `/metrics`:** small follow-ups in `internal/server` / `internal/api` to fully
  light up the latency panel and webhook/quota alerts.

## What's next

One step remains: **Qadam 11 — Part 09 (Marketing site + docs)** — landing/pricing/download/quickstart +
docs from the OpenAPI, sharing the Parts 04/06 design tokens and consuming this release/update-feed +
install paths. After that: the full launch-ready SaaS.
