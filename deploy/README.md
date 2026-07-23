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
│                            wildcard DNS + Spaces
├── observability/           Prometheus + Grafana + OTel collector + alerts
├── release/                 install.sh, update-feed generator, scoop manifest
└── secrets/                 SOPS-encrypted secret workflow + example
```

CI/CD lives in [`../.github/workflows/`](../.github/workflows/) and
[`../.goreleaser.yaml`](../.goreleaser.yaml).

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
| Dockerfiles | syntax review (daemon offline) | ✅ build + push |
| Helm chart | ✅ YAML + template balance | ✅ `helm lint` + `helm template` |
| Terraform | ✅ HCL structure | ✅ `fmt -check` + `validate` |
| Observability YAML/JSON | ✅ PyYAML + json | ✅ |
| Release scripts | ✅ `sh -n` + smoke | ✅ |
