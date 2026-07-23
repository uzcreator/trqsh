# Secrets & security

## What is secret

| Secret | Consumed by |
|---|---|
| `TRQSH_JWT_SECRET` | API — session signing |
| `TRQSH_INTERNAL_TOKEN` | API + edge — internal entitlements RPC auth |
| `TRQSH_DATABASE_URL` | API + migrate job |
| `TRQSH_REDIS_URL` | API + edge — routing registry |
| `TRQSH_STRIPE_SECRET_KEY` / `TRQSH_STRIPE_WEBHOOK_SECRET` | API — billing |
| DNS-provider token | edge — CertMagic DNS-01 wildcard issuance |
| Spaces keys | remote state + cert cache + release uploads |

## How secrets are managed

- **Git**: encrypted at rest with **SOPS** (`.sops.yaml`). Only `data`/`stringData`
  values are encrypted; commit `*.enc.yaml`, never plaintext. Decrypt in CI with the
  age/KMS key to create the Kubernetes Secret.
- **Cluster**: the chart references a pre-created Secret (`secrets.existingSecret`).
  `secrets.create=true` (render from values) is for **throwaway/dev clusters only**.
  Sealed-secrets is an alternative if you prefer committing SealedSecrets.
- **Cloud**: Terraform reads tokens from `TF_VAR_*` env (never committed) and emits
  DB/Redis URLs as **sensitive** outputs wired into the k8s Secret.

## Security posture (00-ARCHITECTURE.md §15)

- **TLS everywhere**: public ingress via cert-manager; edge wildcard/per-domain via
  CertMagic DNS-01.
- **Network isolation**: a `NetworkPolicy` (chart) restricts the control API (which
  serves the internal entitlements RPC) to the edge, dashboard, and ingress only.
- **Least privilege**: distroless `nonroot` images; `runAsNonRoot`; edge droplet
  firewall opens only :80/:443/:4443 (+ :22 to a bastion in prod).
- **Rotation**: rotate `TRQSH_INTERNAL_TOKEN` and `TRQSH_JWT_SECRET` by updating the
  Secret and rolling the deployments; the edge picks up the new token on restart.
- **Abuse/phishing** screening for public hostnames is a control-plane hook (see
  §15) enforced at bind time.
