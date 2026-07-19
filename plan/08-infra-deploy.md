# 08 — Infrastructure, Deploy & Multi-Region Edge

**Owns:** `deploy/`, CI/CD workflows (`.github/workflows/`)
**Depends on:** buildable binaries from Parts 02/03/05 (can start against stubs). **Blocks:** launch.

> Read `00-ARCHITECTURE.md` (§3 stack, §4 layout, §5 dev bootstrap, §15 security). This part makes
> everything runnable locally, deployable to the cloud, released to users, and observable.

## Goal

1. **Local dev**: one-command full stack (`docker compose`).
2. **Cloud**: containers + **Kubernetes/Helm** + **Terraform** (infra, DNS, Postgres, Redis, secrets).
3. **Release**: **GitHub Actions** that build/test, cross-compile + **sign/notarize** the GUI installers
   and CLI, and publish an **auto-update feed**.
4. **Multi-region edge** with latency-based routing + anycast.
5. **Observability**: OpenTelemetry + Prometheus + Grafana; centralized logs.

## Scope / task breakdown

### T1 — Containers (`deploy/docker/`)
- Multi-stage Dockerfiles for `riftd` (edge) and the control API (small, distroless/static).
- `deploy/docker-compose.dev.yml`: Postgres 16, Redis 7, edge, control API, mailhog (dev email),
  Stripe CLI note. A `make dev` brings the whole M1 stack up. (Part 00 may ship a minimal compose
  with just Postgres+Redis so Parts 02/05 run before this part lands.)

### T2 — Kubernetes + Helm (`deploy/helm/`)
- Charts for `edge` (DaemonSet/Deployment with host networking for :443 UDP+TCP), `api`, and shared
  config/secrets. HPA, PodDisruptionBudgets, readiness/liveness (`/healthz`,`/readyz`), graceful
  drain hooks (edge sends `ShutdownMsg`). Ingress/LB for the API + dashboard.

### T3 — Terraform (`deploy/terraform/`)
- Cloud infra: a Kubernetes cluster (or per-region VMs for edges), managed **Postgres** + **Redis**,
  object storage (cert cache, release artifacts), **DNS** incl. the **wildcard `*.rift.sh`** record,
  the DNS-provider credentials for CertMagic DNS-01, secrets manager, and per-region edge pools.
- Anycast IP / GeoDNS so agents and public users hit the **nearest edge** (latency = differentiator).

### T4 — CI/CD (`.github/workflows/`)
- `ci.yml`: `make proto`, `go build/test -race`, `golangci-lint`, frontend `pnpm build/test`.
- `release.yml`: on tag — cross-compile `rift`/`riftd` (darwin/linux/windows × amd64/arm64);
  build GUI bundles (Part 04); **macOS notarization** + **Windows Authenticode** signing; publish
  GitHub Releases + Homebrew tap + `winget`/scoop manifests + `curl | sh` script; generate the
  **auto-update feed** the GUI/CLI consume.
- Container image build + push; Helm deploy to staging on main, prod on tag (with approval).

### T5 — Observability (`deploy/observability/`)
- OpenTelemetry collector; Prometheus scrape configs; Grafana dashboards (active tunnels/sessions,
  bytes, p50/p95 latency, quic-vs-tcp ratio, cert issuance, error codes by §8); log aggregation;
  alerting (edge down, cert failures, webhook failures, quota anomalies).

### T6 — Secrets & security (`deploy/`)
- Secret management (sealed-secrets/SOPS/cloud KMS): DB, Redis, DNS creds, Stripe keys, JWT signing.
- TLS everywhere; network policies isolating the internal entitlements RPC; WAF/DDoS at the edge;
  abuse/phishing screening hooks for public hostnames (§15).

## Interfaces honored
- Consumes the binaries/images produced by 02/03/05 and the GUI bundles from 04. Provides the
  `docker-compose.dev.yml` and release/update feed that other parts reference in their verify steps.

## Done criteria
- `make dev` brings up the full M1 stack; `rift http 3000` works end-to-end locally.
- `helm install` to a **staging** cluster runs edge+api; Let's Encrypt **staging** wildcard issues;
  Grafana shows live metrics; drain works on rollout.
- `release.yml` on a test tag produces **signed** installers for all three OSes + working update feed;
  `brew install`/`winget`/`curl|sh` paths install the CLI.
- Terraform `plan` is clean and documents required cloud credentials.

## Run / verify
```bash
make dev                                   # full local stack
helm upgrade --install rift deploy/helm -f deploy/helm/values.staging.yaml   # staging
# tag a test release to exercise release.yml in a fork; verify signed artifacts + update feed
```
