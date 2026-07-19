# Terraform — Rift cloud infrastructure

Provisions the full platform on **DigitalOcean**: a DOKS control cluster
(api + dashboard), managed **Postgres** + **Redis**, per-region **edge droplets**
with reserved IPs, **DNS** (apex + wildcard `*.rift.sh`), and **Spaces** buckets
(CertMagic cert cache + release/update artifacts).

```
terraform/
├── versions.tf     providers + (commented) Spaces remote-state backend
├── variables.tf    tokens, domain, regions, sizes
├── providers.tf    DO provider + project grouping
├── network.tf      VPC
├── k8s.tf          DOKS control cluster (control plane only)
├── data.tf         managed Postgres + Redis + firewalls
├── edge.tf         per-region edge droplets + reserved IPs + cloud-init + firewall
├── dns.tf          apex domain, wildcard → edge IPs, api/app records
├── storage.tf      Spaces: certs + releases
└── outputs.tf      kubeconfig, DB/Redis URLs, edge IPs, buckets (sensitive)
```

## Topology

- **Control plane** runs in **DOKS** (`helm install` with `edge.enabled=false`).
- **Edges** run as **droplets, one pool per region** (host networking for QUIC/UDP
  + TCP on :443), each with a **reserved IP**. The wildcard `*.rift.sh` round-robins
  across them; use a **GeoDNS** provider in prod for nearest-edge routing.
- Edges reach the control API over the public `api.<domain>` (authenticated by the
  shared `internal_token`) and the managed Redis over its TLS URI (firewall-allowed).

## Required credentials

| Variable | What |
|---|---|
| `do_token` | DigitalOcean API token (write) |
| `spaces_access_id` / `spaces_secret_key` | Spaces (object storage + remote state) |
| `internal_token` | shared edge ↔ API token; must equal the API's `RIFT_INTERNAL_TOKEN` |

Pass them as `TF_VAR_*` env vars in CI (never commit `terraform.tfvars`).

## Usage

```bash
cd deploy/terraform
terraform init
terraform plan  -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars

# Wire the k8s Secret from outputs, then deploy the control plane:
terraform output -raw kubeconfig > kubeconfig.yaml
export KUBECONFIG=$PWD/kubeconfig.yaml
kubectl create secret generic rift-secrets \
  --from-literal=RIFT_DATABASE_URL="$(terraform output -raw postgres_url)" \
  --from-literal=RIFT_REDIS_URL="$(terraform output -raw redis_url)" \
  --from-literal=RIFT_INTERNAL_TOKEN="$TF_VAR_internal_token" \
  --from-literal=RIFT_JWT_SECRET="$(openssl rand -hex 32)"
helm upgrade --install rift ../helm/rift -f ../helm/rift/values.prod.yaml \
  --set edge.enabled=false --set secrets.existingSecret=rift-secrets
```

> This module is **not applied from CI in this repo** (no cloud creds here). CI runs
> `terraform fmt -check` + `terraform validate`. Applies are gated behind an
> environment with credentials.
