# trqsh Desktop GUI (Part 04)

A cross-platform desktop app (macOS / Linux / Windows) built with **Wails v3** —
a native window + WebView around the **same `internal/agent` core the CLI uses**,
so there is zero behavioural drift between the CLI and the GUI.

```
gui/
├── go.mod              # SEPARATE module (replace github.com/trqsh-uz/trqsh => ../)
├── main.go             # Wails app bootstrap: window, assets, lifecycle
├── app.go              # AgentService — the Wails-bound API + Env() host info
├── window.go           # WindowService — native min/max/close/hide/quit
├── storage.go          # settings (trqsh.yml + gui.json) and API-key seam
├── update.go           # release-feed update check
├── system.go           # hardened open-in-browser (scheme allow-list) + replay
├── tray.go             # live system tray (reflects connection + tunnels)
├── wails.json          # Wails project config
└── frontend/           # React + TypeScript + Tailwind (Vite)
    ├── src/lib/         # agent+host facade, wire mirrors, hooks, curl, format
    ├── src/components/  # UI kit + chrome (titlebar, sidebar, palette, toast…)
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
- **Native window shell.** The window is frameless; the React titlebar draws its
  own min/max/close (Windows/Linux — macOS uses native traffic lights) through
  `WindowService`. A live system tray reflects connection state, and `⌘K` opens a
  command palette (`⌘N` new tunnel, `⌘1–4` screens). The layout is responsive:
  the sidebar collapses and the inspector switches split/stacked by window width.
- **Hardened WebView.** A strict Content-Security-Policy is injected at build
  time (dev HMR unaffected); external links are restricted to `http`/`https` in
  `system.go`. Deep links come from `AgentService.Env()`, never hardcoded.
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
wails3 build            # → gui/bin/trqsh-gui
# or, during development, live-reload both Go and frontend:
wails3 dev
```

`go mod tidy` inside `gui/` (under the Wails toolchain) resolves the exact
`wails/v3` version; keep it aligned with `frontend/package.json`'s
`@wailsio/runtime` per the Wails release notes.
