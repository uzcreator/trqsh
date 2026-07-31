# deploy/ — Infrastructure, deploy & release (Part 08)

Everything needed to run trqsh **locally**, ship it to the **cloud**, **release** it
to users, and **observe** it.

```
deploy/
├── docker-compose.dev.yml   full local M1 stack (+ web / observability profiles)
├── .env.example             compose overrides (secrets for localhost)
├── docker/                  multi-stage Dockerfiles: edge, api, migrate, dashboard
├── helm/trqsh/               Kubernetes chart: edge (DaemonSet), api, dashboard,
│                            ingress, HPA/PDB, migrate hook, NetworkPolicy, SM
├── terraform/               DigitalOcean: DOKS + Postgres + Redis + edge droplets +
│                            wildcard DNS (+ optional Cloudflare LB) + Spaces
├── apilb/                   opt-in Caddy internal API load balancer (compose profile)
├── loadtest/               capacity harness: tunnelload (Go data plane) + vegeta (HTTP)
├── observability/           Prometheus + Grafana + OTel collector + alerts
├── release/                 install.sh, update-feed generator, scoop manifest
└── secrets/                 SOPS-encrypted secret workflow + example
```

Single-host production is `docker-compose.prod.yml` + `.env.prod.example` (see
[`PRODUCTION.md`](PRODUCTION.md)); `.env.staging.example` is the same stack on a
separate staging host for rehearsing risky changes first.

CI/CD lives in [`../.github/workflows/`](../.github/workflows/) and
[`../.goreleaser.yaml`](../.goreleaser.yaml).

> **`deploy.yml` (Helm) is manual-only.** Its automatic push/tag triggers were
> removed on purpose: it runs `helm upgrade` against `KUBECONFIG_STAGING` /
> `KUBECONFIG_PROD`, but production currently runs on a plain docker-compose VPS
> (see [`PRODUCTION.md`](PRODUCTION.md)), not Kubernetes, and those kubeconfig
> secrets are unverified. Re-enable the triggers only after confirming the
> `staging` / `production` GitHub Environment secrets point at a real cluster.

## Local (M1 in one command)

```bash
make dev            # pg + redis + migrate + api + edge + mailhog  (builds images)
# in another shell — a public URL to your local server:
go run ./cmd/trqsh http 3000 --server 127.0.0.1:4443 --insecure
curl -H 'Host: <sub>.lvh.me' http://127.0.0.1

make dev-web        # + dashboard on :3000
make observability  # + Grafana :3001 / Prometheus :9091
make dev-down       # tear everything down
```

Ports: edge `:80/:443` + agent `:4443` (tcp+udp) + metrics `:9090`; API `:8080`;
dashboard `:3000`; mailhog `:8025`; Grafana `:3001`.

## Cloud

1. `deploy/terraform` → DOKS control cluster + managed Postgres/Redis + per-region
   edge droplets + wildcard DNS + Spaces. See [terraform/README.md](terraform/README.md).
2. `deploy/helm/trqsh` → the control plane (`edge.enabled=false` in the DO topology;
   the edge DaemonSet is for single-region / self-hosted clusters). Staging vs prod
   via `values.staging.yaml` / `values.prod.yaml`.

## Release

Tag `vX.Y.Z` → `release.yml`: goreleaser (CLI/edge archives, Homebrew, deb/rpm),
signed GUI bundles (macOS notarized, Windows Authenticode), and the auto-update
feed. `images.yml` publishes containers to GHCR on every main + tag.

## What is verified here vs. in CI

| Artifact | Verified locally | Verified in CI |
|---|---|---|
| `docker-compose.dev.yml` | ✅ `docker compose config` | ✅ |
| `docker-compose.prod.yml` | ✅ `docker compose config` (base + `apilb`/`observability` profiles) | ✅ |
| Dockerfiles | syntax review (daemon offline) | ✅ build + push |
| `apilb/Caddyfile` | manual review (caddy offline) | — |
| Helm chart | ✅ YAML + template balance | ✅ `helm lint` + `helm template` |
| Terraform | ✅ HCL structure | ✅ `fmt -check` + `validate` |
| Load-test harness | ✅ `go build`/`vet` (tunnelload) + `sh -n` (vegeta scripts) | ✅ |
| Observability YAML/JSON | ✅ PyYAML + json | ✅ |
| Release scripts | ✅ `sh -n` + smoke | ✅ |
