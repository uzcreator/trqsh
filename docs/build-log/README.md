# Build log

One report per completed build step. When a session finishes a step from
[`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md), it adds a file here so the next
session can catch up in one read — without re-deriving context.

## Convention

- File name: `NN-<short-name>.md` matching the step/part number
  (`01-bootstrap.md`, `02-protocol-transport.md`, …).
- Each report contains: **what was built**, **key decisions**, **verification status**
  (what passed, what is still pending and why), **known gaps / TODOs**, and **what's next**.
- Keep it factual. If a gate did **not** pass, say so and explain — do not mark a step "done" if it isn't.

## Index

- [`01-bootstrap.md`](./01-bootstrap.md) — repo scaffold, dev deps, Makefile, license (Step 1).
- [`02-protocol-transport.md`](./02-protocol-transport.md) — wire protocol + QUIC/TCP mux transport, Part 01 (Step 2).
- [`03-edge-server.md`](./03-edge-server.md) — edge `trqshd`: sessions, registry, ingress, TLS, Part 02 (Step 3).
- [`04-agent-cli.md`](./04-agent-cli.md) — agent core + CLI `trqsh`, inspector, reconnect, Part 03 (Step 4). **M1 MVP.**
- [`05-control-api.md`](./05-control-api.md) — control plane `trqshapi`: accounts, auth, real entitlements, Part 05 (Step 5).
- [`06-billing.md`](./06-billing.md) — billing: catalog, Stripe Checkout/Portal/webhooks, metering, quota, Part 07 (Step 6).
- Step 7 (edge ↔ real entitlements integration) has no separate log — proven inline in Steps 5–6 with real binaries.
- [`08-dashboard.md`](./08-dashboard.md) — web dashboard: auth, tunnels, domains, keys, usage, billing, Part 06 (Step 8).
- [`09-gui.md`](./09-gui.md) — desktop GUI (Wails v3): login, tunnels, inspector/replay, account, settings, Part 04 (Step 9).
- [`10-infra.md`](./10-infra.md) — infra/deploy: Docker, compose, Helm, Terraform, CI/CD, observability, secrets, Part 08 (Step 10).
- [`11-site.md`](./11-site.md) — marketing site + docs (Next.js): landing, pricing, download, API reference, Part 09 (Step 11). **Final — launch-ready; whole stack run end-to-end.**
