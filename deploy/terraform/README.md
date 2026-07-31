# Terraform — trqsh cloud infrastructure

Provisions the full platform on **DigitalOcean**: a DOKS control cluster
(api + dashboard), managed **Postgres** + **Redis**, per-region **edge droplets**
with reserved IPs, **DNS** (apex + wildcard `*.trqsh.uz`), and **Spaces** buckets
(CertMagic cert cache + release/update artifacts).

```
terraform/
├── versions.tf         providers + (commented) Spaces remote-state backend
├── variables.tf        tokens, domain, regions, sizes, forwarding + Cloudflare-LB knobs
├── providers.tf        DO + Cloudflare providers + project grouping
├── network.tf          VPC
├── k8s.tf              DOKS control cluster (control plane only)
├── data.tf             managed Postgres + Redis + firewalls
├── edge.tf             per-region edge droplets + reserved IPs + cloud-init + firewall
├── dns.tf              apex domain, wildcard → edge IPs, api/app records
├── dns_cloudflare.tf   OPTIONAL Cloudflare LB (DNS-only steering); inert by default
├── storage.tf          Spaces: certs + releases
└── outputs.tf          kubeconfig, DB/Redis URLs, edge IPs, buckets (sensitive)
```

## Topology

- **Control plane** runs in **DOKS** (`helm install` with `edge.enabled=false`).
- **Edges** run as **droplets, one pool per region** (host networking for QUIC/UDP
  + TCP on :443), each with a **reserved IP**. The wildcard `*.trqsh.uz` round-robins
  across them; use a **GeoDNS** provider in prod for nearest-edge routing.
- Edges reach the control API over the public `api.<domain>` (authenticated by the
  shared `internal_token`) and the managed Redis over its TLS URI (firewall-allowed).

## Cross-edge forwarding (Stage D)

Every edge joins a forwarding mesh: when public traffic for a tunnel homed on
edge B lands on edge A, edge A hands the connection to edge B instead of 404ing.
The cloud-init wires this automatically:

- `TRQSH_FORWARD_ADDR=:4444` opens the internal, token-authenticated forwarding
  listener; `TRQSH_FORWARD_ADVERTISE_ADDR` is the address peers dial, read from the
  droplet's DO metadata at boot (`var.edge_forward_iface`, default `public`).
- The `digitalocean_firewall.edge` rule for `4444` is scoped to **`source_tags =
  [<project>-edge]`** — only other edge droplets can reach it, never the public
  internet — which works across regions because DO tag firewalls match the source
  droplet's identity. (DO VPCs are single-region, so a VPC-CIDR rule could not; set
  `edge_forward_iface = "private"` only for a single-region deployment where you
  want the free VPC-internal hop.)
- Shared TLS (Stage C) needs no extra wiring here: because `TRQSH_REDIS_URL` is
  already passed to each edge, `buildCertStorage` selects the Redis-backed cert
  store automatically, so all edges share one ACME cache + issuance lock.

Port-based (TCP/UDP) tunnels are intentionally **not** forwarded cross-edge — a
public port is a per-edge physical resource with no globally-unique key (documented
in `internal/server/ingress_tcp.go` / `ingress_udp.go`).

## Cloudflare Load Balancing (optional — `dns_cloudflare.tf`)

By default the wildcard `*.<domain>` is a DigitalOcean round-robin across edge IPs.
`dns_cloudflare.tf` adds **health-checked DNS steering** (failover / latency) via
Cloudflare Load Balancing, in DNS-only mode (`proxied = false`, so it still returns
plain A answers and does not proxy — required to keep trqsh's own TLS/QUIC intact).
It is **inert until you opt in**. To activate (owner action; cannot be tested from
this repo):

1. Host the zone at Cloudflare (grey-cloud / DNS-only, as the repo already advises).
2. Ensure the Cloudflare plan includes **Load Balancing** (a paid add-on). A
   **wildcard** LB hostname needs an **Enterprise** zone; on lower tiers use
   per-hostname load balancers or keep the DO wildcard round-robin.
3. Set `enable_cloudflare_lb = true` plus `cloudflare_api_token`,
   `cloudflare_account_id`, `cloudflare_zone_id` (see `terraform.tfvars.example`).
4. `terraform apply`. This drops the DO wildcard records so the two never conflict.

The default health monitor is a **TCP check on :443** (no new firewall exposure).
To health-check the edge's application `/healthz` instead, set
`cloudflare_lb_monitor_type = "http"`, `cloudflare_lb_monitor_port = 9090`, and open
`9090` to Cloudflare's health-check ranges in `edge.tf` (note this also exposes
`/metrics` to those ranges).

## Required credentials

| Variable | What |
|---|---|
| `do_token` | DigitalOcean API token (write) |
| `spaces_access_id` / `spaces_secret_key` | Spaces (object storage + remote state) |
| `internal_token` | shared edge ↔ API token; must equal the API's `TRQSH_INTERNAL_TOKEN` |

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
kubectl create secret generic trqsh-secrets \
  --from-literal=TRQSH_DATABASE_URL="$(terraform output -raw postgres_url)" \
  --from-literal=TRQSH_REDIS_URL="$(terraform output -raw redis_url)" \
  --from-literal=TRQSH_INTERNAL_TOKEN="$TF_VAR_internal_token" \
  --from-literal=TRQSH_JWT_SECRET="$(openssl rand -hex 32)"
helm upgrade --install trqsh ../helm/trqsh -f ../helm/trqsh/values.prod.yaml \
  --set edge.enabled=false --set secrets.existingSecret=trqsh-secrets
```

> This module is **not applied from CI in this repo** (no cloud creds here). CI runs
> `terraform fmt -check` + `terraform validate`. Applies are gated behind an
> environment with credentials.
