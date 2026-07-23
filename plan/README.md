# trqsh — Build Plan (developer tunneling SaaS)

**trqsh** is a developer tunnel service (expose `localhost` to the public internet), in the same
category as **ngrok** and **Cloudflare Tunnel**, but positioned to be **faster** (QUIC-first
transport), with a **more generous free tier**, a **first-class cross-platform desktop GUI**, and
extras ngrok lacks (**UDP tunnels**, built-in inspector, open-source agent). It is a hosted **SaaS**
with accounts + billing.

> **Codename `trqsh`** is a placeholder. Rename with a global find/replace before launch (run a
> trademark check first). Binaries: `trqsh` (agent/CLI), `trqshd` (edge server).

This folder (`plan/`) contains the **build specifications**. The product is split into **10
independent parts**, each written to be executed by a **separate Claude Code session**. Sessions
coordinate only through the **frozen shared contracts** defined in [`00-ARCHITECTURE.md`](./00-ARCHITECTURE.md).

> **In what order do I run these?** See **[`EXECUTION-ORDER.md`](./EXECUTION-ORDER.md)** — it lists
> the exact sequence and gives a ready-to-paste prompt for each session, with a verification gate
> between steps.

## How to use these specs (read this first)

1. **Every session reads [`00-ARCHITECTURE.md`](./00-ARCHITECTURE.md) first.** It is the single
   source of truth: tech stack, monorepo layout, dev bootstrap, and the frozen contracts (wire
   protocol, transport API, entitlements interface, config schema, error taxonomy, plan catalog).
2. Then open **your part's spec** (below) and implement only what it owns.
3. **Do not change a frozen shared contract** unless you update `00-ARCHITECTURE.md` in the same
   change and note it in your part's spec. Contracts are what let parts be built in parallel.
4. **Repository root** = the directory that contains this `plan/` folder. All code paths in the
   specs (e.g. `pkg/proto`, `internal/server`) are **relative to that repo root**, not to `plan/`.

## The 10 parts

| # | Spec | Owns (dirs) | Depends on |
|---|------|-------------|------------|
| 00 | [Master Architecture & Contracts](./00-ARCHITECTURE.md) | *(shared)* | — |
| 01 | [Protocol & Transport core](./01-protocol-transport.md) | `pkg/proto`, `pkg/tunnel`, `pkg/authz` | 00 |
| 02 | [Edge server / data plane (`trqshd`)](./02-edge-server.md) | `internal/server`, `cmd/trqshd` | 01, (05 stubbed) |
| 03 | [Agent core + CLI (`trqsh`)](./03-agent-cli.md) | `internal/agent`, `cmd/trqsh` | 01, (05 stubbed) |
| 04 | [Desktop GUI (Wails v3)](./04-gui-desktop.md) | `gui/` | 03 |
| 05 | [Control plane API & auth](./05-control-api.md) | `internal/api`, DB | 01 |
| 06 | [Web dashboard & inspector](./06-web-dashboard.md) | `web/dashboard` | 05, 07 |
| 07 | [Billing & monetization](./07-billing-monetization.md) | `internal/billing` | 05 |
| 08 | [Infrastructure & deploy](./08-infra-deploy.md) | `deploy/`, CI/CD | 02, 03, 05 (stubs ok) |
| 09 | [Marketing site, docs & onboarding](./09-website-docs.md) | `web/site`, `docs/` | 05, 07 |

## Build order & milestones

```
00 ──▶ 01 ──┬──▶ 02 ─┐
            ├──▶ 03 ─┼─▶ (M1 MVP: live public URL) ──▶ 04 (GUI)
            └──▶ 05 ─┴─▶ 06 (dashboard)
                     └─▶ 07 (billing) ─▶ enforcement wired into 02
08 (infra) and 09 (site) run in parallel throughout.
```

- **M0 — Foundation:** `00`, `01`. Frozen contracts + green transport tests.
- **M1 — MVP:** `02`, `03`, minimal `05` (API keys), minimal `08` (one edge via docker-compose).
  Success: `trqsh http 3000` → live public HTTPS URL proxying to localhost, survives reconnect,
  TCP/UDP tunnels work.
- **M2 — Monetizable:** `04` GUI, `06` dashboard, `07` billing wired into `05` enforcement.
- **M3 — Launch/scale:** full `08` (multi-region, signed installers, observability), `09` site+docs.

## Recommended session order
`00` → `01` → then `02` + `03` + `05` in parallel (02 & 03 run against the `05` stub) → then
`04` + `06` + `07` in parallel → `08` and `09` may start at M1 and finish at M3.

## Coordination rules (avoiding collisions between parallel sessions)

- **Directory ownership is exclusive.** Only the owning part edits its directories (see table).
  Shared `pkg/*` is owned by Part 01; others import it read-only.
- **The seams are interfaces, not implementations.** Part 02 depends on `authz.Entitlements`
  (an interface from Part 01) and ships a stub; Part 05/07 provide the real implementation later.
  This lets 02/03 reach a working MVP before 05 exists.
- **The wire protocol and transport API are frozen at the end of Part 01.** Everything else builds
  on top without modifying them.
- **Each spec ends with a "Done criteria" and "Run/verify" section** — a session is finished only
  when those pass.
