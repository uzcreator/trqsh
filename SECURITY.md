# Security Policy

Security is a first-class concern for Rift: we route other people's traffic and hold their
credentials. This document describes how to report issues and the controls already in place.

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Email **security@rift.dev** with:

- a description of the issue and its impact,
- steps to reproduce (a proof-of-concept if possible),
- affected component (agent, edge, control API, dashboard, site) and version.

We aim to acknowledge within **48 hours**, provide an initial assessment within **5 business
days**, and coordinate a fix and disclosure timeline with you. We do not pursue legal action
against good-faith research that respects user privacy and data and does not degrade the service.

For abuse (phishing/malware hosted on a tunnel), email **abuse@rift.dev** with the hostname.

## Supported versions

Until 1.0, security fixes land on the latest `main` and the most recent tagged release.

## Security posture

The controls below are implemented in this repository.

### Transport & TLS
- TLS everywhere: agent↔edge, public↔edge, browser↔API. No plaintext in production.
- QUIC/HTTP-3 primary with an authenticated TLS-over-TCP fallback.
- Minimum TLS 1.2 on public listeners.
- Length-prefixed protocol frames are bounded (`proto.MaxFrameSize`, 1 MiB) on read **and**
  write, so a malicious length prefix cannot exhaust memory.

### Authentication & secrets
- API keys are high-entropy, **argon2id-hashed at rest**, shown once, and revocable. Lookups use a
  clear prefix; verification is **constant-time** (`crypto/subtle`).
- Dashboard sessions are HMAC (HS256) JWTs with the signing method **pinned** on parse (no
  `alg=none` / algorithm-confusion), short access TTL + refresh rotation.
- The edge↔API internal RPC token and Stripe webhook signatures are compared in **constant time**.
- **Fail-closed production config:** with `RIFT_ENV=production`, the API and edge refuse to start
  on any dev-default or weak secret — see the operator checklist below.
- Secrets come from the environment / a secret manager (SOPS in `deploy/secrets`), never committed.
  Webhook signatures are always verified before a payload is trusted.

### Abuse & DoS resistance
- HTTP servers set `ReadHeaderTimeout` (and read/write/idle timeouts where streaming allows),
  mitigating Slowloris-style attacks.
- Per-IP **rate limiting**: a strict limit on auth endpoints (brute-force / account spam) and a
  broad flood limit on the rest of the public API. Client IP is only taken from
  `X-Forwarded-For` when `RIFT_TRUST_PROXY` is set (no spoofing otherwise).
- Request bodies are size-limited (`http.MaxBytesReader`); outbound reads use `io.LimitReader`.
- Per-account quotas + protocol entitlements are enforced **at the edge on every bind**, not just
  in the UI, so limits cannot be bypassed by scripting the agent.
- Security response headers (`nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, COOP, HSTS in
  prod) on the control API.

### Tenant isolation & platform
- A session may only bind subdomains/domains/ports its account is entitled to.
- Containers run **non-root** on `distroless/static`; Kubernetes `NetworkPolicy` restricts the
  control API's internal RPC to the edge, dashboard, and ingress only.
- Public hostnames are subject to phishing/malware screening.

## Operator hardening checklist (production)

Set `RIFT_ENV=production` — start-up then **fails closed** unless all of these hold:

| Variable | Requirement |
|---|---|
| `RIFT_JWT_SECRET` | strong random, ≥ 32 chars, not the dev default |
| `RIFT_INTERNAL_TOKEN` | strong random, shared by edge + API, not the dev default |
| `RIFT_DEV_AUTH` | disabled (password-less auth off; use OAuth) |
| `RIFT_DATABASE_URL` | set (the in-memory store is dev-only) |
| `RIFT_API_PUBLIC_URL` | `https://…` |
| `RIFT_ENTITLEMENTS` (edge) | `api` (never `stub`, which allows all binds) |
| `RIFT_ACME_EMAIL` (edge) | set for automatic TLS |
| `RIFT_TRUST_PROXY` | only if behind a trusted proxy/LB |

## Automated scanning

CI runs **gosec** (SAST), **govulncheck** (Go vuln DB), **CodeQL** (Go + JS/TS), and **Trivy**
(dependencies, IaC misconfig, secrets). Dependencies are kept current via Dependabot. See
[`.github/workflows/security.yml`](.github/workflows/security.yml) and
[`codeql.yml`](.github/workflows/codeql.yml).
