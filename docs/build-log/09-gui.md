# Step 9 — Part 04: Desktop GUI (Wails v3, `gui/`)

- **Date:** 2026-07-18
- **Step:** 9 of 11 (see [`../../plan/EXECUTION-ORDER.md`](../../plan/EXECUTION-ORDER.md))
- **Spec:** [`../../plan/04-gui-desktop.md`](../../plan/04-gui-desktop.md) + the agent-core API section of
  [`../../plan/03-agent-cli.md`](../../plan/03-agent-cli.md)
- **Milestone:** M2 — Monetizable product
- **Status:** ✅ Complete — **frontend `pnpm build` green** (tsc + vite, 0 errors); Go shell written against
  the frozen `agent.Core` and **parses/formats clean**; **root `go build`/`go vet`/`go test ./...` still green**
  (the GUI is a separate module, isolated from CGO-less CI). Native `wails3 build` requires the Wails toolchain
  and is out of scope for this environment (documented gap, mirrors the Postgres/OAuth precedent).

> **TL;DR (Uz):** Ikkinchi frontend qism — Wails v3 desktop app: Go backend (native oyna + WebView) Part 03
> ning **aynan o'sha agent core**'ini o'raydi (drift yo'q). Ekranlar: Login (API key), Tunnels (bir klik
> start/stop, public URL + copy + brauzerda ochish, jonli metrikalar), Inspector (jonli HTTP req/resp +
> replay), Account (plan + upgrade CTA), Settings (edge, tema, tray, yangilanish). Bitta `agent:event`
> kanalidan hamma yangilanish keladi. Frontend brauzerda mock agent bilan to'liq ishlaydi — shuning uchun
> `pnpm build` yashil, Wails/CGO'siz ham. Go qobiq yozilgan, parse bo'ladi; root Go hamon yashil. Keyingi:
> Part 08 infra yoki Part 09 site.

## What was built

`gui/` — a Wails v3 desktop app. A **separate Go module** (`github.com/trqsh-uz/trqsh/gui`, `replace … => ../`)
so it stays out of the root `go.work` and never blocks the CGO-less root build/CI.

```
gui/
├── go.mod            separate module; replace github.com/trqsh-uz/trqsh => ../
├── main.go           Wails bootstrap: frameless window, embedded assets, lifecycle, tray wiring
├── app.go            AgentService — the Wails-bound API; core.Events() → "agent:event" pump
├── storage.go        Settings/SaveSettings (trqsh.yml + gui.json) + API-key storage seam
├── update.go         CheckUpdate against the release feed + semver compare
├── system.go         openBrowser (OS launcher) + replayLocal (re-issue a captured request)
├── tray.go           system tray (Open / Quit, click-to-toggle)
├── wails.json        Wails project config (name, version, frontend commands)
├── README.md         architecture + how to develop the frontend and build the native app
└── frontend/         React 19 + TypeScript 5.7 + Tailwind 3.4 (Vite 6)
    ├── src/lib/       agent.ts (Wails facade + browser mock), types.ts (wire mirrors),
    │                  errors.ts (§8 map), format.ts, theme.ts, utils.ts
    ├── src/components/ ui/* (button, card, input, dialog, select, switch, badge, status-dot, …),
    │                  titlebar, sidebar, copy-button, stat, empty, start-tunnel-dialog
    └── src/screens/   login, tunnels, inspector, account, settings   (App.tsx routes them)
```

**Stack:** Go + Wails v3 (`@wailsio/runtime@3.0.0-alpha.97`) backend; React 19 + TS 5.7 + Tailwind 3.4
frontend. Lean deps only (`clsx`, `tailwind-merge`, `class-variance-authority`, `lucide-react`) — same
UI-kit philosophy as `web/dashboard`, no Radix.

## How it works

- **One core, no drift.** `main.go` loads `~/.trqsh-uz/trqsh.yml` and builds the **same `internal/agent` core the
  CLI uses**, wraps it in `AgentService`, and binds it. The GUI adds *no* tunnelling logic — only presentation,
  auth state, event bridging, settings, updates, and URL opening.
- **One event channel.** `AgentService.pumpEvents` ranges over `agent.Core.Events()`
  (`status`/`tunnel`/`request`/`error`) and re-emits each on the single Wails event **`agent:event`**. The
  frontend subscribes once (`agent.onEvent` in `App.tsx`) and reduces events into React state: status →
  connection, tunnel → upsert-by-id, request → prepend to the inspector (capped at 200).
- **Frozen wire types.** `frontend/src/lib/types.ts` mirrors the Go structs by their **json tags**
  (snake_case: `public_url`, `bytes_in`, …) because Wails marshals with `encoding/json` — the TS shapes are
  exactly what crosses the boundary. Getting this right was a real correctness fix (see decisions).
- **Auth vs. transport.** The frozen `Status.Connected` means "transport session up," but a GUI shouldn't
  bounce you to login during a reconnect. `AgentService` keeps a **sticky `authed`** flag (set on Login,
  cleared on Logout) and OR-s it into forwarded `status` events + `Status()`, so the workspace stays visible
  while you hold a valid key. The frozen `Core` is untouched.
- **Browser-preview mock.** `isDesktop()` replicates Wails' own runtime probe (WebView2 / WKWebView /
  Android). Off-desktop, `agent.ts` swaps in an **in-memory mock** that fakes login, tunnels going
  connecting→online, and a live stream of captured requests. This is what makes the whole UI buildable and
  browsable **without** the Wails/CGO toolchain — and it flips to real `Call.ByName` bindings automatically
  inside the native shell.
- **Screens.** Login (paste API key), Tunnels (proto-aware start dialog, copy URL, open-in-browser, live
  4-metric footer, stop), Inspector (list + detail with headers/body, status/method colors, **Replay**),
  Account (plan, session usage, **Upgrade → dashboard billing**), Settings (edge server, insecure TLS, theme,
  start-at-login, minimize-to-tray, **Check for updates**, Disconnect).
- **Shared design tokens.** The dataviz reference palette (RGB-channel CSS variables, light + dark) is
  **identical** to `web/dashboard` — the brand stays consistent across surfaces.

## Verification

| Check | Result |
|------|--------|
| `pnpm build` (frontend: `tsc --noEmit && vite build`) | ✅ green — 1634 modules, 0 type errors |
| Frontend bundle | ✅ `dist/` (JS 276 KB / 86 KB gz, CSS 18 KB) — the Go embed target |
| Browser preview (`pnpm dev`, mock agent) | ✅ full flow: login → start tunnels → live inspector traffic → replay → settings |
| `gofmt -e gui/*.go` | ✅ all 6 Go files parse + already formatted |
| Root `go build ./...` | ✅ green (GUI module excluded from `./...`) |
| Root `go vet ./...` | ✅ green |
| Root `go test ./...` | ✅ green (agent, api, billing, server, proto, tunnel) |

The GUI module being separate means adding it changed **nothing** for the root module's build or tests —
verified by re-running all three after writing `gui/`.

## Key decisions

- **`gui/` as a separate Go module.** Wails v3 needs CGO + a platform WebView (WebView2 / WebKit) that the
  root CGO-less CI doesn't have. Isolating it (own `go.mod`, excluded from `go.work`) keeps `go build`/`test
  ./...` green everywhere while the GUI still consumes the real agent core via a relative `replace`. Same
  honest-gap pattern used for Postgres (compiled, not CI-exercised) and OAuth.
- **snake_case TS mirrors.** Initial mirrors used PascalCase; corrected to match the Go **json tags**, since
  Wails serializes with `encoding/json`. Left unfixed this would have been a silent runtime break (every
  `tunnel.PublicURL` undefined). The mock emits the same snake_case, so preview and production agree.
- **Single `agent:event` channel** instead of many named events — mirrors the frozen `Core.Events()` exactly,
  so the Go↔JS contract is one enum-tagged struct, trivial to extend and reason about.
- **Sticky auth flag** to reconcile "signed in" (GUI gate) with "transport connected" (frozen semantics)
  without touching the frozen `Core` or its `Status`.
- **In-memory mock as a first-class dev target**, not an afterthought — it's the mechanism that keeps Part 04
  fully verifiable (build + interactive UI) under the no-CGO constraint, exactly like the frontend verify path
  for `web/dashboard`.
- **OS-launcher `openBrowser` + hand-written semver** instead of leaning on uncertain alpha runtime APIs —
  fewer moving parts across Wails alpha churn.
- **Lean hand-written UI kit** (cva + Tailwind), consistent with `web/dashboard`; full dark mode.

## Known gaps / notes (for later parts)

- **Native `wails3 build` not run here.** Requires the Wails v3 toolchain + CGO + a platform WebView. The Go
  shell is written against the documented v3 alpha API (`application.New`, `NewWebviewWindowWithOptions`,
  `NewSystemTray`, `EmitEvent`, `Call.ByName`/`Events.On`); the exact `wails/v3` module version must be pinned
  to match `@wailsio/runtime` via `wails3 update` / `go mod tidy` under that toolchain. `gui/README.md` has the
  steps. **Packaging, code-signing, and notarization are Part 08.**
- **API key storage** currently reuses the agent's `~/.trqsh-uz/trqsh.yml` (shared source of truth with the CLI).
  `storage.go` marks the single seam to swap in the OS keychain (Keychain / Credential Manager / libsecret)
  for a hardened build.
- **Replay** re-issues the captured request against the **local** target (ngrok-style; exercises the dev's
  app). Replaying *through* the edge (so it re-appears in the capture stream) is a small follow-up.
- **Tray icon / app icons** are supplied per-platform by Part 08 packaging; the tray ships with a label + menu.
- **Auto-update** checks a release feed and links to the download; in-place binary replacement is wired by the
  Part 08 release channel + updater.

## What's next

M2 GUI is done. Remaining per `EXECUTION-ORDER.md`: **Qadam 10 — Part 08 (Infra/Deploy)** — Dockerfiles,
docker-compose full stack, Helm/K8s, Terraform (wildcard DNS + Postgres/Redis), GitHub Actions (build/test +
cross-compile + **sign/notarize the GUI installers** built here) and observability; and **Qadam 11 — Part 09
(Marketing site + docs)**, which shares this exact design-token set and the OpenAPI surface.

---

## Enhancement pass — "real desktop app" + responsive + security (2026-07-20)

A follow-up pass turned the shell into a first-class native app: strong desktop affordances, a fully
responsive layout, live feedback, and WebView hardening. Same rule as before — **the frozen `agent.Core`
is untouched**; every addition is GUI presentation, chrome, or a new Wails-bound host service.

**New native window controls (the biggest gap).** The window is frameless, but had no min/max/close.
Added `gui/window.go` — a `WindowService` bound to the frontend (`Minimize`/`ToggleMaximize`/`IsMaximized`/
`Hide`/`Show`/`Quit`) driven by `application.WebviewWindow` methods. `components/window-controls.tsx` draws
custom min/max/close on Windows/Linux and renders **nothing on macOS** (the OS draws traffic lights; the
titlebar reserves 76px for them). The close button honors the minimize-to-tray preference (hide vs. quit).

**Live system tray.** `tray.go` is now a `Tray` controller wired to `AgentService.OnState`; it rebuilds on
every status/tunnel change, reflects state in its label ("trqsh — N active"), and offers quick actions that
read the current tunnel set at click time (open each web tunnel in the browser, "New tunnel…" which emits
`ui:new-tunnel` to the frontend, quit). Request-stream events are deliberately excluded from refresh.

**Responsive everywhere** (`lib/hooks.ts` — `useWindowWidth`, `useNow`, `useHotkeys`): the sidebar collapses
to an icon rail (auto < 940px, manual toggle), the Inspector switches between a two-pane split (≥ 880px) and
a master–detail stack, and Tunnels flow into a 1/2-column grid.

**Keyboard-driven + command palette.** `⌘/Ctrl-K` opens a fuzzy command palette (`command-palette.tsx`);
`⌘N` new tunnel, `⌘1–4` switch screens. Global matcher ignores keystrokes inside inputs (except the palette
toggle).

**Feedback system.** A dependency-free toast provider (`ui/toast.tsx`) wired into copy actions, tunnel
start/stop, settings save, update checks, and errors.

**Richer screens.** Inspector: search + method/status filters, follow-newest, clear, request/response
**tabs**, JSON pretty-printing, per-body copy, and **copy-as-cURL** (`lib/curl.ts`). Tunnels: live traffic
**sparklines** (`components/sparkline.tsx`, sampled 1/s), uptime, req/s, copy-as-cURL, aggregate header
stats. Account: usage **meters** vs. plan allowance (`ui/meter.tsx`). Settings: transport (auto/quic/tcp)
control + **confirm-before-disconnect** dialog + host os/arch/version. Login: show/hide key toggle.
Deep links are no longer hardcoded — `AgentService.Env()` returns OS/arch/version + dashboard/keys/billing/
docs URLs (overridable via `TRQSH_DASHBOARD_URL`/`TRQSH_DOCS_URL` for self-host).

### Security hardening (the user's explicit ask)
- **Strict Content-Security-Policy** on the WebView, injected at **build time only** (so `vite dev` HMR is
  unaffected): `default-src 'self'`, `object-src 'none'`, `base-uri/frame-ancestors/form-action 'none'`,
  `script-src 'self'`, `connect-src 'self'`, `img/font 'self' data:`, `style-src 'self' 'unsafe-inline'`.
  The app is fully self-contained (embedded assets, native postMessage bridge — no remote origins), so this
  is a clean lock-down and defense-in-depth against injected content.
- **`openBrowser` scheme allow-list** (`system.go` → `safeExternalURL`): only `http`/`https` with a host are
  ever handed to the OS launcher. Closes the injection surface where a crafted string (`file://`,
  `javascript:`, or a `-`-leading value parsed as a flag) could launch an unintended handler. Applied to
  every deep link and the update URL.
- **API-key hygiene**: masked input with an explicit reveal toggle; still cleared on logout. (OS-keychain
  storage remains the documented seam in `storage.go`.)
- **Focus-trapped, escape-closable modals** with focus restore.

### Verification
| Check | Result |
|------|--------|
| `pnpm build` (`tsc --noEmit && vite build`, strict + noUnused*) | ✅ green — 1642 modules, 0 type errors |
| CSP present in `dist/index.html`, relative `./assets` paths (Wails-embeddable) | ✅ |
| `gofmt -l gui/*.go` | ✅ empty (all 7 Go files clean, incl. new `window.go`) |

**Process note:** the screen rewrites were fanned out to 3 parallel subagents; a session/rate limit killed
them mid-flight after they'd completed `account.tsx` + `settings.tsx`, so the remaining screens + the whole
app shell were finished inline. Native `wails3 build` still needs the Wails toolchain + CGO + WebView
(unchanged gap, Part 08) — so the Wails window-control / tray / CSP behavior is written against the
documented v3 API but verified natively only in Part 08.
