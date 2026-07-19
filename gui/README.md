# Rift Desktop GUI (Part 04)

A cross-platform desktop app (macOS / Linux / Windows) built with **Wails v3** —
a native window + WebView around the **same `internal/agent` core the CLI uses**,
so there is zero behavioural drift between the CLI and the GUI.

```
gui/
├── go.mod              # SEPARATE module (replace github.com/rift/rift => ../)
├── main.go             # Wails app bootstrap: window, assets, lifecycle
├── app.go              # AgentService — the Wails-bound API the frontend calls
├── storage.go          # settings (rift.yml + gui.json) and API-key seam
├── update.go           # release-feed update check
├── system.go           # open-in-browser + request replay
├── tray.go             # system tray
├── wails.json          # Wails project config
└── frontend/           # React + TypeScript + Tailwind (Vite)
    ├── src/lib/         # agent facade, wire-type mirrors, formatters
    ├── src/components/  # UI kit + app chrome
    └── src/screens/     # Login, Tunnels, Inspector, Account, Settings
```

## Architecture

- **One event channel.** The Go `AgentService` pumps `agent.Core.Events()`
  (`status` / `tunnel` / `request` / `error`) to the single Wails event
  `agent:event`. The frontend subscribes once (`agent.onEvent`) and reduces it
  into React state (`App.tsx`).
- **Frozen wire types.** `frontend/src/lib/types.ts` mirrors the Go structs by
  their **json tags** (snake_case) because Wails marshals with `encoding/json`.
- **Auth vs. transport.** `AgentService` keeps a sticky `authed` flag so a
  transient reconnect never bounces the user to the login screen; the frozen
  `agent.Core` is untouched.
- **Shared design tokens.** The dataviz reference palette (RGB-channel CSS
  variables) is identical to `web/dashboard` and `web/site`.

## Develop the frontend (no native toolchain needed)

The frontend runs standalone in a browser using an **in-memory mock** of the
agent (`src/lib/agent.ts`), so the whole UI is buildable and browsable without
compiling the Go shell:

```bash
cd frontend
pnpm install
pnpm dev        # http://localhost:9245  (mock agent: fake tunnels + live traffic)
pnpm build      # tsc --noEmit && vite build  → frontend/dist  (embedded by Go)
```

`isDesktop()` detects the real WebView (WebView2 / WKWebView) and switches from
the mock to live `Call.ByName("AgentService.*")` bindings automatically.

## Build the native app

> **Requires the Wails v3 toolchain, a C compiler (CGO), and a platform WebView**
> (WebView2 on Windows, WebKit on macOS/Linux). This is intentionally **not**
> part of the CGO-less root CI — `gui/` is a separate module excluded from
> `go.work`, so `go build ./...` at the repo root never touches it. Packaging,
> code-signing, and notarization are owned by **Part 08**.

```bash
# one-time
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

cd gui
wails3 build            # → gui/bin/rift-gui
# or, during development, live-reload both Go and frontend:
wails3 dev
```

`go mod tidy` inside `gui/` (under the Wails toolchain) resolves the exact
`wails/v3` version; keep it aligned with `frontend/package.json`'s
`@wailsio/runtime` per the Wails release notes.
