# Execution Order & Ready-to-Paste Prompts

Bu — loyihani qurish **ketma-ketligi**. Har bir qadam = alohida Claude Code sessiyasi. Har bir
qadamda: yangi sessiya oching (repo root: `system/`), pastdagi **promptni copy-paste qiling**,
tugagach **"Gate" (tekshiruv)** ni bajaring, keyin keyingi qadamga o'ting.

> **Muhim:** Har bir sessiya avval `plan/00-ARCHITECTURE.md` ni o'qiydi (frozen kontraktlar).
> Qadamlar orasidagi **Gate** o'tmasa, keyingisiga o'tmang.

## Qisqacha tartib (solo — ketma-ket)

| Qadam | Sessiya | Milestone | Oldin tugashi shart | Parallel mumkin |
|------:|---------|-----------|---------------------|-----------------|
| 1 | Bootstrap scaffold | M0 | — | — |
| 2 | Part 01 protocol/transport | M0 | 1 | — (hamma shunga bog'liq) |
| 3 | Part 02 edge server | M1 | 2 | 4, 5 |
| 4 | Part 03 agent + CLI | M1 | 2 | 3, 5 |
| — | **✅ Gate: MVP verify** | M1 | 3, 4 | — |
| 5 | Part 05 control API | M2 | 2 | 3, 4 (01 dan keyin istalgan vaqt) |
| 6 | Part 07 billing | M2 | 5 | 8, 9 |
| 7 | **Integration: real entitlements** | M2 | 5 (6 ni ham) | — |
| 8 | Part 06 dashboard | M2 | 5, 6 | 6, 9 |
| 9 | Part 04 GUI | M2 | 4, 5 | 6, 8 |
| 10 | Part 08 infra (full) | M3 | 3, 4, 5 | 11 |
| 11 | Part 09 site + docs | M3 | 5, 6 | 10 |

**Parallel qilmoqchi bo'lsangiz:** hamma qism bitta Go modulida bo'lgani uchun, har bir parallel
qismni alohida **git branch / worktree** da qiling va keyin merge qiling (aks holda `go.mod`
konflikti bo'ladi). Solo ketma-ket ishlasangiz, buning hojati yo'q.

---

## Qadam 1 — Bootstrap scaffold (M0)

Repo skeletini yaratadi: `go.work`, papka tuzilmasi, minimal dev muhiti.

```text
Read plan/00-ARCHITECTURE.md in full (especially sections 4 and 5). Then bootstrap the repository
skeleton for the Rift project at the repo root:
- Root go.mod (module github.com/rift/rift — pick this org name and keep it forever) and go.work.
- The full directory tree from section 4 (proto/, cmd/riftd, cmd/rift, pkg/{proto,tunnel,authz},
  internal/{server,agent,api,billing}, gui/, web/{dashboard,site}, deploy/, docs/) with a .gitkeep
  or a tiny doc.go in each Go package dir so it compiles.
- deploy/docker-compose.dev.yml with just Postgres 16 and Redis 7 (so Parts 02/05 can run early).
- A Makefile with targets: proto, build, test, lint, run-edge, run-agent, dev.
- docs/DEVELOPMENT.md pinning tool versions (protoc, protoc-gen-go, sqlc, goose, golangci-lint,
  Node 20 + pnpm, Wails v3 CLI) and describing `make dev`.
- .gitignore, LICENSE (Apache-2.0 for the open-source agent), and a top-level README stub.
Do not implement any part's logic yet — only the scaffold. Verify `go build ./...` succeeds.
```

**✅ Gate:** `go build ./...` xatosiz; `docker compose -f deploy/docker-compose.dev.yml up -d` Postgres+Redis ko'taradi.

---

## Qadam 2 — Part 01: Protocol & Transport (M0, POYDEVOR)

Hamma narsa shunga tayanadi. Buni tugatmasdan keyingilarga o'tmang.

```text
Read plan/00-ARCHITECTURE.md in full, then plan/01-protocol-transport.md. Implement Part 01 exactly
as specified: the Protobuf wire protocol (proto/rift.proto -> pkg/proto), the length-prefixed codec,
the pkg/tunnel multiplexed transport with a QUIC-first Dialer and TCP+yamux fallback, an edge-side
Listener that accepts both QUIC and TCP, and pkg/authz (Limits/BindRequest/Decision/Usage/
Entitlements types + an allow-all StubEntitlements). Honor the frozen contracts in sections 6-9 of
00-ARCHITECTURE.md — do NOT change any signatures. When done, run `make proto && go test ./pkg/...
-race` and ensure: codec round-trip passes, the echo test passes for BOTH QUIC and forced-TCP, and
the fallback test lands on TCP when QUIC is blocked. Implement ONLY Part 01.
```

**✅ Gate:** `go test ./pkg/... -race` yashil (QUIC + TCP + fallback testlari). Endi §6–§9 kontraktlari **muzlatildi**.

---

## Qadam 3 — Part 02: Edge Server `riftd` (M1)

```text
Read plan/00-ARCHITECTURE.md, then plan/02-edge-server.md. Implement Part 02 (internal/server +
cmd/riftd) exactly as specified: agent session handler (accept sessions via pkg/tunnel, read Hello,
authenticate via authz.Entitlements), the Redis-backed tunnel registry, HTTP/HTTPS ingress with
vhost/SNI routing, raw TCP + UDP port tunnels, wildcard TLS via CertMagic DNS-01, the data-stream
weld (open stream -> WriteStreamInit -> io.Copy both ways), heartbeats, graceful drain, and
Prometheus metrics. Depend on the authz.Entitlements INTERFACE and default to StubEntitlements
(RIFT_ENTITLEMENTS=stub) so this runs before the control plane exists. Do NOT modify pkg/* contracts.
Verify per the spec's Run/verify section using local Redis and RIFT_BASE_DOMAIN=lvh.me. Implement
ONLY Part 02.
```

**✅ Gate:** `go run ./cmd/riftd` ishga tushadi, `/healthz` javob beradi (to'liq test Qadam 4 dan keyin).

---

## Qadam 4 — Part 03: Agent + CLI `rift` (M1)

```text
Read plan/00-ARCHITECTURE.md, then plan/03-agent-cli.md. Implement Part 03 (internal/agent +
cmd/rift) exactly as specified: config loader (~/.rift/rift.yml per section 10), the agent Core
(dial the edge via pkg/tunnel, open the control stream, send Hello, BindTunnel, accept incoming data
streams and forward to the local service), auto-reconnect with backoff + re-bind, the local
inspector at 127.0.0.1:4040 with replay, the local control API (unix socket / loopback) for the GUI,
and the Cobra CLI (rift http/tcp/udp/start/status/login/config/version). Freeze the agent-core API
(Core, TunnelSpec, Tunnel, Event) — Part 04 depends on it. Do NOT modify pkg/* contracts. Verify
end-to-end against the Part 02 edge (RIFT_ENTITLEMENTS=stub) per the spec. Implement ONLY Part 03.
```

**✅ Gate (MVP verify):** Quyidagi ishlashi kerak —
```text
# 1-terminal: local xizmat
python3 -m http.server 3000
# 2-terminal: edge (stub) + redis ko'tarilgan holda
RIFT_ENTITLEMENTS=stub RIFT_BASE_DOMAIN=lvh.me go run ./cmd/riftd
# 3-terminal:
go run ./cmd/rift http 3000 --server localhost:443
curl -k https://<chiqarilgan-subdomain>.lvh.me     # python listing qaytishi kerak
```
Tarmoqni uzib-ulasangiz avtomatik qayta ulanadi; `rift tcp 22` + ssh ishlaydi. **Bu M1 MVP — mahsulot tirik!**

---

## Qadam 5 — Part 05: Control API & Auth (M2)

```text
Read plan/00-ARCHITECTURE.md (sections 9, 11, 12), then plan/05-control-api.md. Implement Part 05
(internal/api + Postgres schema/migrations) exactly as specified: the data model, OAuth (GitHub/
Google) + device flow + JWT sessions, API keys (argon2id hash, shown once, revocable), the REST API
(/account, /orgs, /subdomains, /domains + verify, /tunnels from Redis, /usage) documented with
OpenAPI, and the REAL authz.Entitlements implementation exposed to the edge over an internal RPC with
a short-TTL cache. Seed the plans table from section 11. Do NOT modify pkg/* contracts. Verify per
the spec (goose up, create an API key, point the edge at the real entitlements client). Implement
ONLY Part 05.
```

**✅ Gate:** OAuth login JWT beradi; device flow CLI ga token qaytaradi; API-key create/revoke ishlaydi; edge real entitlements bilan taqiqlangan bind'ni rad etadi.

---

## Qadam 6 — Part 07: Billing & Monetization (M2, DAROMAD)

```text
Read plan/00-ARCHITECTURE.md (sections 9, 11), then plan/07-billing-monetization.md. Implement Part
07 (internal/billing) exactly as specified: the plan catalog (section 11) as the single source of
truth with Stripe price IDs, Stripe Checkout + Customer Portal, signature-verified webhooks that
flip orgs.plan, metered usage ingestion from authz.Usage, and LimitsForPlan + current-usage lookups
wired into Part 05's CheckBind so the edge returns ERR_QUOTA_*/ERR_PLAN_FORBIDS correctly. Make
enforcement fail-safe (never fall back to unlimited). Reuse Part 05's DB layer. Verify in Stripe test
mode per the spec. Implement ONLY Part 07.
```

**✅ Gate:** Stripe test mode'da Free→Pro upgrade webhook orqali planni o'zgartiradi; Pro-only bind (UDP/custom domain) endi ruxsat etiladi.

---

## Qadam 7 — Integration: edge'ni real entitlements'ga ulash (M2)

Kichik ulash qadami — edge stub'dan real control plane'ga o'tadi.

```text
Wire the edge (Part 02) to use the REAL authz.Entitlements from Part 05 instead of StubEntitlements:
set RIFT_ENTITLEMENTS=api and RIFT_API_URL to the control API, implement/enable the edge-side
entitlements client with its short-TTL cache, and confirm the internal RPC auth. Then run the full
stack (edge + control API + Postgres + Redis + agent) and verify: (1) a valid API key from Part 05
authenticates and binds; (2) a Free-plan account is denied a reserved subdomain (ERR_SUBDOMAIN_
FORBIDDEN) and UDP (ERR_PLAN_FORBIDS); (3) after a Stripe test-mode upgrade to Pro, the same binds
succeed. Do not change frozen contracts.
```

**✅ Gate:** Haqiqiy akkaunt + kalit bilan tunnel ochiladi; tarif cheklovlari edge darajasida ishlaydi.

---

## Qadam 8 — Part 06: Web Dashboard & Inspector (M2)

```text
Read plan/00-ARCHITECTURE.md (sections 11, 12), then plan/06-web-dashboard.md. Implement Part 06
(web/dashboard, Next.js + TS + Tailwind + shadcn/ui) exactly as specified: auth/session against Part
05, live tunnels list, reserved subdomains + custom domains (with DNS records + verify), API keys,
usage charts (follow the dataviz skill), the cloud request inspector/replay, and team + billing
screens embedding Part 07 (Stripe Checkout/Portal). Use the generated TS client from Part 05's
OpenAPI — never touch Postgres/Redis directly. Share Tailwind design tokens with Parts 04 and 09.
Verify per the spec. Implement ONLY Part 06.
```

**✅ Gate:** Login → jonli tunnellar; API-key yaratish; domen qo'shish/verify; grafiklar; inspector replay; test-mode upgrade.

---

## Qadam 9 — Part 04: Desktop GUI (M2)

```text
Read plan/00-ARCHITECTURE.md, then plan/04-gui-desktop.md AND the agent-core API section of
plan/03-agent-cli.md. Implement Part 04 (gui/, Wails v3 + React + TS + Tailwind + shadcn/ui) exactly
as specified: bind the Part 03 agent.Core in-process, screens (login with OS-keychain storage,
tunnels with copy-URL, inspector with replay, settings, account/upgrade CTA), system tray, and
auto-update against the Part 08 release feed. Configure cross-platform builds (mac .dmg, win .exe/MSI,
linux AppImage/.deb) — signing/notarization is wired in Part 08. Do not fork logic into the GUI; add
capabilities to the Part 03 core if needed. Verify with `wails3 dev` per the spec. Implement ONLY Part 04.
```

**✅ Gate:** `wails3 dev` — login, tunnel start, copy URL, curl, inspector; tray ishlaydi; `wails3 build` bundle beradi.

---

## Qadam 10 — Part 08: Infrastructure & Deploy (M3)

```text
Read plan/00-ARCHITECTURE.md (sections 3, 4, 5, 15), then plan/08-infra-deploy.md. Implement Part 08
(deploy/ + .github/workflows) exactly as specified: multi-stage Dockerfiles for riftd + api, the full
docker-compose.dev.yml (Postgres, Redis, edge, api, mailhog), Helm charts (edge with host networking
for :443 UDP+TCP, api, HPA, PDB, drain hooks), Terraform (cluster, managed Postgres/Redis, object
storage, wildcard *.rift.sh DNS + DNS-01 creds, secrets, per-region edge pools + GeoDNS/anycast),
CI (ci.yml) and release.yml (cross-compile rift/riftd for all OS/arch, build + SIGN/NOTARIZE the GUI
installers, publish GitHub Releases + Homebrew tap + winget/scoop + curl|sh + auto-update feed), and
observability (OTel + Prometheus + Grafana dashboards, alerts) + secrets management. Verify per the
spec (make dev, helm install to staging with Let's Encrypt staging, a test release tag). Implement ONLY Part 08.
```

**✅ Gate:** `make dev` to'liq stack; staging'ga `helm install`; staging wildcard cert; Grafana metrikalar; imzolangan installerlar.

---

## Qadam 11 — Part 09: Marketing Site, Docs & Onboarding (M3)

```text
Read plan/00-ARCHITECTURE.md (sections 1, 11), then plan/09-website-docs.md. Implement Part 09
(web/site + docs/, Next.js + Tailwind + shadcn/ui + MDX) exactly as specified: landing page selling
each differentiator (QUIC speed with a dataviz-compliant benchmark visual, generous free tier, GUI,
UDP, custom domains/teams, open-source agent), a pricing page rendering the Part 07 plan catalog
(never hardcoded), a download page pulling artifacts from the Part 08 release feed with install
snippets, full docs + an API reference generated from Part 05's OpenAPI (each section-8 error code
gets a docs anchor), and the signup->dashboard onboarding funnel with lifecycle emails. Share brand
tokens with Parts 04/06. Verify per the spec (Lighthouse budget, pricing matches Part 07). Implement
ONLY Part 09.
```

**✅ Gate:** Landing/pricing/download/docs tez yuklanadi; narxlar Part 07 ga mos; quickstart ~1 daqiqada tunnel beradi.

---

## Yakuniy holat

Barcha 11 qadam tugagach: tirik SaaS — CLI + GUI (3 OS), edge, control plane, billing, dashboard,
marketing sayt. Keyin: domenni sozlash, Stripe'ni live rejimga o'tkazish, prod deploy, va launch. 🚀
```
