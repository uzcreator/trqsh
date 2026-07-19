# 04 — Desktop GUI (macOS / Linux / Windows, Wails v3)

**Owns:** `gui/`
**Depends on:** Part 03 (`internal/agent` core API — `Core`, `TunnelSpec`, `Tunnel`, `Event`).
**Blocks:** nothing (installers/signing are finished with Part 08).

> Read `00-ARCHITECTURE.md` and `03-agent-cli.md` (the agent-core API section) first. The GUI is a
> thin, beautiful shell over the Part 03 core — a first-class GUI is a core differentiator vs ngrok.

## Goal

A polished cross-platform desktop app that lets a developer log in, start/stop tunnels with one
click, see live traffic, and manage settings — without the terminal. Ship signed builds for the
three OSes with auto-update.

## Stack
- **Wails v3** (Go backend + web frontend). Frontend: **React + TypeScript + Tailwind + shadcn/ui**
  (shared design tokens with the `web/dashboard` in Part 06 for a consistent brand).
- Reuse `internal/agent` **in-process** (import `Core` directly) — no separate daemon needed; the
  Part 03 local control API remains available for headless/CLI parity.

## Scope / task breakdown

### T1 — Wails scaffold & core binding (`gui/`, `gui/app.go`)
- `wails3 init` React+TS template. In `app.go`, construct the Part 03 `agent.Core` and expose bound
  methods to the frontend: `Login`, `Status`, `StartTunnel`, `StopTunnel`, `List`, `Shutdown`.
- Bridge `Core.Events()` to the frontend via Wails events (`EventsEmit`) for live updates.

### T2 — Screens (`gui/frontend/src/`)
- **Auth / Login** — API key or OAuth device flow (open browser → Part 05 → paste/redirect token).
  Persist securely (OS keychain: macOS Keychain, Windows Credential Manager, libsecret) via a small
  Go helper; never store the key in plaintext.
- **Dashboard / Tunnels** — list active tunnels with proto, local addr, **public URL + copy button**,
  status dot, live request counter; **Start tunnel** dialog (proto, port/addr, optional subdomain/
  custom domain/basic-auth). One-click stop.
- **Inspector** — live request feed (from `Event`/the Part 03 inspector), request detail, **replay**.
- **Settings** — default server/region, transport (auto/quic/tcp), launch-at-login, theme,
  update channel.
- **Account/Upgrade** — plan badge; when an action hits `ERR_PLAN_FORBIDS`/quota, show an upgrade CTA
  deep-linking to the dashboard/billing (Part 06/07).

### T3 — System tray & lifecycle (`gui/tray.go`)
- Tray icon with quick start/stop, active-tunnel list, open-window, quit. Minimize-to-tray;
  optional launch-at-login. Reflect connection state (connected/reconnecting) in the tray icon.

### T4 — Auto-update (`gui/update.go`)
- Check the release feed (Part 08) on launch + periodically; download, verify signature, prompt,
  and relaunch. Respect the update channel from Settings.

### T5 — Cross-platform build (`gui/wails.json`, `gui/build/`)
- Configure Wails build for darwin (arm64+amd64, `.app`/`.dmg`), windows (`.exe`/NSIS or MSI),
  linux (AppImage + `.deb`). App icons, metadata, entitlements (mac). **Code-signing + notarization**
  is wired in Part 08's CI; this part provides the config + local unsigned dev builds.

## Interfaces honored (do not modify)
- The Part 03 agent-core API (`Core`, `TunnelSpec`, `Tunnel`, `Event`). If the GUI needs a new
  capability, add it to the Part 03 core (and its spec), don't fork logic into the GUI.

## Done criteria
- `wails3 dev` runs; login works; starting a tunnel shows a live public URL with a working copy
  button; stopping works; the inspector shows live requests and can replay one.
- Tray controls work; reconnection state is reflected in the UI.
- `wails3 build` produces a runnable (unsigned) bundle on each OS (CI matrix in Part 08 signs them).

## Run / verify
```bash
cd gui
wails3 dev                      # against a running edge (Part 02) + agent core
# smoke test: login → start http tunnel to a local :3000 → copy URL → curl it → see it in Inspector
wails3 build                    # produces platform bundle under gui/build/bin
```
