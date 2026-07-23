# Security Hardening & Professionalization Report

**Date:** 2026-07-19
**Scope:** whole repository — ~13k lines of Go (5 modules of packages), 3 frontends, infra.
**Goal:** take the codebase from MVP to a production-grade, security-forward, GitHub-ready project.

---

## 1. Method

- Read the security-critical paths directly: the wire codec (`pkg/proto`), transport/TLS
  (`pkg/tunnel`, `internal/server/tls.go`), edge ingress (`internal/server`), control-plane auth
  (`internal/api/auth`), config (`internal/{api,server}/config.go`), and billing webhook verification.
- Mapped the whole tree with targeted greps for known risk patterns: unbounded reads, missing server
  timeouts, TLS config, rate limiting, CORS, SQL string-building, secret logging, constant-time
  comparisons.
- Validated findings against the current threat landscape (see §6).

## 2. What was already solid (kept as-is)

The code was **better than "beginner level"** — several controls were already correct:

- **Wire framing is bounded** — `proto.MaxFrameSize` (1 MiB) enforced on read *and* write; a
  malicious length prefix cannot exhaust memory.
- **JWT signing method is pinned** on parse (`*jwt.SigningMethodHMAC`) — no `alg=none` /
  algorithm-confusion; golang-jwt/v5 validates `exp`.
- **API keys** are argon2id-hashed (m=64 MB, t=1, p=4), verified in **constant time**
  (`crypto/subtle`), with a clear prefix for lookup only.
- **Constant-time comparisons** everywhere they matter: basic-auth, the edge↔API internal token,
  Stripe webhook signatures, API keys.
- **Bounded outbound reads** — `io.LimitReader` on OAuth, entitlements RPC, billing, Stripe,
  inspector; **request bodies** capped via `http.MaxBytesReader` (1 MiB).
- **No SQL string-building** and **no secret logging** found.

## 3. Vulnerabilities fixed

| # | Severity | Issue | Fix |
|---|---|---|---|
| 1 | **Critical** | `DefaultConfig()` shipped a hardcoded JWT secret, internal token, and `DevAuth=true`; `LoadConfig` only errored on an *empty* secret — so a prod deploy that forgot env vars silently ran with a **known JWT secret** (forgeable sessions) and **password-less auth**. | Added `TRQSH_ENV=production` profile. `internal/api/config.go` now **fails closed**: rejects the dev-default secret, secrets < 32 chars, the default internal token, `DevAuth` on, the in-memory store, and non-HTTPS public URL. |
| 2 | **High** | The **edge** defaulted to `TRQSH_ENTITLEMENTS=stub` (allow-all binds) with no internal token — dangerous if shipped to prod. | `internal/server/config.go` production profile requires `TRQSH_ENTITLEMENTS=api`, a non-default `TRQSH_INTERNAL_TOKEN`, and `TRQSH_ACME_EMAIL`. |
| 3 | **High** | Four `http.Server` instances (control API, edge ops, agent localapi, inspector) had **no timeouts** → Slowloris / gosec **G112**. | Added `ReadHeaderTimeout` everywhere; full read/write/idle/`MaxHeaderBytes` on the API + ops servers. The two SSE servers (localapi, inspector) get `ReadHeaderTimeout` only, so streams aren't cut. |
| 4 | **Medium-High** | **No rate limiting** — auth endpoints open to brute-force / account-spam, API open to flooding. | Dependency-free per-IP **token-bucket limiter** (`internal/api/middleware.go`): 5 rps/burst 10 on auth endpoints, 50 rps/burst 100 on the public API. Internal edge RPC is exempt (high-volume entitlement checks). `X-Forwarded-For` honored only when `TRQSH_TRUST_PROXY` is set (no spoofing). |
| 5 | **Medium** | No security response headers. | `securityHeaders` middleware: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, COOP, and HSTS in production. |

All timeout values align with published Go production guidance (see §6).

## 4. Professionalization

- **`SECURITY.md`** — vulnerability reporting, the full security posture, and an **operator
  hardening checklist** (the env vars `TRQSH_ENV=production` enforces).
- **`CONTRIBUTING.md`**, **`CODE_OF_CONDUCT.md`** (Contributor Covenant 2.1), **`.editorconfig`**.
- **`.gitattributes`** — enforce LF (prevents CRLF/gofmt drift on Windows); mark generated files.
- Issue templates (`bug_report`, `feature_request`, `config` steering security reports to advisories)
  and a **PR template** with a security-impact prompt.
- **Rewrote `README.md`** — was stale ("Step 1 complete"); now a proper project README with
  architecture, quickstart, security, and deploy sections.
- **`gofmt -w`** normalized the whole tree (a few pre-existing files + new code) — `gofmt -l` clean.
- Hardened root **`.gitignore`** (extra secret patterns; fixed a `secrets/` glob that would have
  dropped the committed example templates).

### Automated security scanning (CI)

- **`.github/workflows/security.yml`** — **gosec** (SAST → SARIF), **govulncheck** (Go vuln DB),
  **Trivy** (deps + IaC misconfig + secrets → SARIF), weekly + on PR.
- **`.github/workflows/codeql.yml`** — CodeQL for Go and JS/TS (`security-and-quality`).
- **`.github/dependabot.yml`** — weekly updates for gomod, npm (×3 frontends), github-actions, docker.
- Extended **`ci.yml`**'s Go job with a generated-catalog + embedded-OpenAPI drift guard (added in
  the docs work) — kept intact.

## 5. GitHub readiness

- `git init` on `main`; **initial commit `3dca5e4`** — 370 files, 34,375 insertions.
- **Leak check passed**: no `node_modules`, `.next`, `dist`, or binaries staged; a real-secret
  regex scan (`sk_live_`, private keys, AWS/Slack tokens) found **nothing**. Only clearly-labeled
  *dev* defaults and `*.example.*` templates are committed.
- Push is a manual step (needs your remote + credentials):
  ```bash
  gh repo create <you>/trqsh --private --source=. --remote=origin --push
  ```

## 6. Threat-landscape research

- **Abuse is the dominant real-world threat** for tunneling services — ngrok reported a ~700% surge
  in malware/phishing abuse (leading them to gate TCP endpoints behind payment verification), and in
  June 2025 attackers abused Cloudflare Tunnel to deliver Python RATs, using random subdomains to
  evade domain-reputation systems. **Recommendation (feature work, not just hardening):** screen
  public hostnames against phishing/malware feeds on bind, rate-limit new-subdomain creation, and
  keep the `abuse@` pathway (already in `SECURITY.md`).
- **Misconfiguration → exposed databases** (a classic: `ngrok tcp 5432` with no auth). trqsh already
  supports per-tunnel basic-auth; an **IP-allowlist** tunnel option is a strong future addition.
- **Go server timeouts**: confirmed `ReadHeaderTimeout` is the key Slowloris control; our values are
  within/stricter than the commonly recommended (20 s / 1 m / 2 m). Modern Go already prefers safe
  cipher ordering, so no `PreferServerCipherSuites` change is needed.

Sources: [ngrok security](https://ngrok.com/security) ·
[Trend Micro: abusing cloud tunneling](https://www.trendmicro.com/vinfo/us/security/news/cybercrime-and-digital-threats/how-cybercriminals-abuse-cloud-tunneling-services) ·
[Huntress: abusing ngrok](https://www.huntress.com/blog/abusing-ngrok-hackers-at-the-end-of-the-tunnel) ·
[Cloudflare: exposing Go on the Internet](https://blog.cloudflare.com/exposing-go-on-the-internet/) ·
[Resilient Go net/http servers](https://ieftimov.com/posts/make-resilient-golang-net-http-servers-using-timeouts-deadlines-context-cancellation/)

## 7. Verification

| Check | Result |
|---|---|
| `go build ./...` | ✅ |
| `go vet ./...` | ✅ |
| `go test ./...` (all packages) | ✅ |
| `gofmt -l internal pkg cmd` | ✅ empty |
| Secret scan (real-secret regexes) | ✅ none |
| Frontends (`web/site`, `web/dashboard`, `gui/frontend`) `pnpm build` | ✅ (unchanged since prior green build) |

## 8. Honest gaps / follow-ups

- **Wildcard TLS (CertMagic + DNS-01)** for tunnel subdomains is still a documented `TODO(prod)` in
  `internal/server/tls.go` (edge is self-signed today) — required before public HTTPS works.
- **gosec / govulncheck / CodeQL / Trivy / golangci-lint were not run locally** (not installed in
  this environment) — they are wired into CI and run there. Findings will populate the Security tab.
- **Abuse-detection pipeline** and **per-tunnel IP allowlist** are recommended feature work (§6).
- Rate-limit values are sane defaults; tune per real traffic and make them configurable if needed.
