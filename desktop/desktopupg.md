# trqsh desktop — redesign + embedded terminal

**Qisqacha (uz):** `desktop/` (Tauri v2 + React) ilovasini butunlay qayta dizayn qilamiz — ranglar (yashil-qora "operator console" palitrasi) o'zgarmaydi, lekin joylashuv/bo'limlar butunlay boshqacha bo'ladi (generic SaaS-dashboard ko'rinishidan chiqib, VS Code'ga o'xshash "activity rail + status bar + console panels" ko'rinishiga). Ustiga, ilovaga VS Code'dagidek o'rnatilgan terminal qo'shiladi (Rust `portable-pty` backend + xterm.js, `Ctrl+backtick` bilan ochiladi/yopiladi), shunda foydalanuvchi boshqa terminal ochib turishi shart bo'lmaydi. Ish 5 bosqichga, 11 ta commit'ga bo'lingan — har bosqich alohida, ishlaydigan holatda commit qilinadi. `internal/agent/cli/tui/` (boshqa AI session hozir shu yerda ishlayapti), `web/site`, `web/dashboard`, `internal/agent/*` (Go tunnel engine) — bularga umuman tegilmaydi. Texnik tafsilotlar (Rust/Tauri API'lari) chuqur tekshirilgan — taxmin qilinmagan.

---

## 1. Why this is happening

The desktop app is the actual flagship product (web dashboard was originally "just a test"). It's functionally complete but visually generic — a competent-but-templated shadcn-style SaaS dashboard. The ask: make it feel like a deliberately designed, best-in-class tool, and close the biggest functional gap versus something like VS Code — no integrated terminal, forcing users out to a separate OS window.

---

## 2. Current-state findings (full file-by-file survey completed)

### 2.1 Architecture (unchanged by this work)
`desktop/` is a thin Tauri v2 client over a bundled Go agent binary. `src-tauri/src/main.rs` spawns `trqsh daemon` on launch (resolved next to `current_exe()`, not via Tauri's sidecar resolver — a past bug taught this project not to trust bundle-flattened sidecar paths), serving a token-authed loopback HTTP+SSE API on `127.0.0.1:4041`. `lib/agent.ts` is a plain `fetch`/`EventSource` client over it. **The UI is never in the tunnel data path** — nothing here touches throughput or the Go engine. Window is frameless with a custom titlebar + system tray (hide-to-tray on close, agent keeps running).

**Hard constraint:** Rust cannot be *linked* locally (no MSVC Build Tools) — confirmed `rustc`/`cargo` 1.97.1 are on PATH but no `cl.exe`/`link.exe`/`vswhere.exe`. Every prior Rust change in this project's history was written + carefully reviewed locally without a full build, then proven via GitHub Actions CI — this plan follows that same established pattern, not a new risk.

**Off-limits, respected throughout:** `internal/agent/cli/tui/*` (another AI session has uncommitted work there right now), `web/site`, `web/dashboard` (separate workstreams per standing rule), `internal/agent/*` generally (the Go tunnel engine — frozen/proven, and the terminal feature has zero reason to touch it since it's a Rust/OS-process concern).

### 2.2 Design tokens (`src/index.css` + `tailwind.config.ts`) — kept as-is
RGB-channel CSS vars, dark-primary "operator console" palette: near-black with a faint green cast, signal-green `--primary`, cobalt-blue `--wire` reserved for data values, semantic `--good/--warning/--serious/--critical`, chart `--series-1/--series-2`. **This palette does not change** — only structure does.

### 2.3 Current structural pattern (being replaced)
Frameless titlebar (brand + connection pill + search/theme/window buttons) → pill-list sidebar nav (`w-52`/`w-14` collapsed) → main area of stacked `rounded-lg border border-border bg-surface` cards. Generic, competent, templated — the thing changing.

### 2.4 Component inventory highlights (everything under `desktop/src/` surveyed)
- `status-dot.tsx` — **the app's declared signature motif**: `online` stacks two staggered custom-keyframe ping rings (`signal-ring`/`signal-ring-delayed`, not Tailwind's stock `animate-ping`) + a glow shadow. Extended, not replaced, into the new status bar and terminal live-indicator.
- `dialog.tsx` — hand-rolled focus trap; documented gotcha: `onClose` is kept in a ref so the focus-management effect depends only on `[open]` (an inline-arrow prop previously re-ran it every render and stole focus from typed fields). Any new similar surface must respect this.
- `input.tsx`/`select.tsx` use a **second, distinct token pair** (`border-border-strong`+`bg-page`) from the card convention (`border-border`+`bg-surface`) — deliberate, kept.
- `empty.tsx` used by `tunnels.tsx`/`inspector.tsx` but **`keys.tsx` hand-rolls its own instead** — inconsistency, fixed in Phase 3.
- Repeated inline error-banner markup duplicated verbatim 3+ times across `start-tunnel-dialog.tsx`/`login.tsx`, warning-toned sibling in `keys.tsx` — extracted to `<InlineAlert>` in Phase 3.
- Animation is **100% named-CSS-class-driven** (`dialog-in`, `toast-in`, `animate-boot`, `signal-ring`, `screen-in`), all in `index.css`. No JS animation library exists or should be introduced.
- `useHotkeys(keys: Hotkey[])` (`lib/hooks.ts`) — global keydown with an `allowInInput` escape hatch; `"mod"` token hardcodes to `metaKey` on Mac / `ctrlKey` elsewhere — **no way today to express "literal Ctrl on every platform"** (needed for true `Ctrl+backtick` parity, see Phase 5).
- Two stale Wails-era comments found (`window-controls.tsx` references `main.go`; `lib/types.ts` says "Wails marshals across the boundary") — leftovers from before the Tauri rewrite, fixed as drive-by cleanup in Phase 3.
- **Zero existing terminal/PTY/xterm/shell-exec code anywhere in `desktop/`** — confirmed via exhaustive grep. Clean slate.

---

## 3. Verified technical corrections (from deep implementation research — not assumptions)

A dedicated research pass fact-checked the trickiest parts against this actual repo's files, installed package type-defs, and docs.rs/npm — not against general training-knowledge recall. Several corrected the original brief:

1. **Tauri v2's capability/ACL system does not gate plain app commands in this app.** Empirically confirmed: `capabilities/default.json` has zero entries for the four existing custom commands (`get_agent_endpoint`, `get_host_info`, `open_url`, `quit`), and they work today anyway. The capabilities file only governs `core:*`/plugin permissions here. So the new PTY commands can't be "scoped via a capability entry" the way `core:window:allow-close` is — the real security boundary is the narrow 4-command surface (no generic "exec" command) plus argv-only spawning, not an ACL restriction. The capability file gets a documentation-only update explaining this, matching its existing self-documenting style.
2. **`cargo check`/`cargo clippy` likely work locally; `cargo build` almost certainly doesn't.** `rustc`/`cargo` 1.97.1 confirmed on PATH; no MSVC linker found. `check`/`clippy` never invoke the linker. One residual uncertainty: `tauri-build`'s `build.rs` runs a resource-compiler step for the Windows `.ico` on *every* invocation including `check` — unconfirmed whether that needs Windows SDK tooling beyond what's present. **First concrete action of Phase 4: run `cargo check` from `desktop/src-tauri` against the unmodified tree**, to establish ground truth for free before writing PTY code.
3. **`desktop-build.yml` (the only CI that compiles Rust) is `workflow_dispatch`-only** — it does not run on push/PR. `ci.yml`'s `frontends` matrix *does* run automatically on every push/PR and covers `desktop`'s `pnpm build` — so all TS/React phases (1, 2, 3, 5) get automatic CI coverage, but **Phase 4's Rust and Phase 5's Rust-adjacent wiring need a manual `gh workflow run desktop-build.yml` dispatch** after each to get real compile+link proof on all 3 OSes. Easy to forget — called out explicitly so it isn't.
4. **The monospace "gap" is real but subtler than assumed.** Tailwind 3.4 ships an *implicit default* `mono` stack even though only `sans` is overridden today — so `font-mono` (already used extensively) isn't rendering the browser's raw default, it's rendering Tailwind's undeclared default. Declaring it explicitly (with Cascadia Code/JetBrains Mono prioritized) in Phase 1 is a real, deliberate behavior change, not a no-op.
5. **`rounded-lg` and `rounded-xl` currently compute to the identical 12px** — this project's Tailwind override only touches `lg/md/sm`, leaving `xl` on Tailwind's own default (`0.75rem`), which now numerically equals the overridden `lg`. So `Dialog`/`CommandPalette` (`xl`) and `Card` (`lg`) render at the same radius today — the "docked panel vs. floating overlay" distinction doesn't visually exist yet. Phase 1 fixes this for real by explicitly setting `xl: 1rem` (16px).
6. **`@xterm/xterm` + `@xterm/addon-fit` are the correct current (scoped) package names** (the unscoped `xterm`/`xterm-addon-fit` are legacy/renamed). Exact latest patch versions to be confirmed via `pnpm add @xterm/xterm@latest @xterm/addon-fit@latest` at implementation time rather than hand-typed.
7. **`tauri::ipc::Channel<T>` is real; its exact shape was found in the installed `@tauri-apps/api@2.9.0` type defs** (`node_modules/@tauri-apps/api/core.d.ts`) — `Channel` exported from the same `@tauri-apps/api/core` module this codebase already imports `invoke` from, `new Channel<T>(onmessage?)`, settable `.onmessage`. Rust-side: `tauri::ipc::Channel<T>` as a command parameter with `.send(value)`. Command names stay snake_case in `invoke("pty_spawn", …)` — only argument keys auto-convert camelCase(JS)↔snake_case(Rust), confirmed against this codebase's existing `invoke("open_url", { url })`.
8. **`portable-pty` latest is `0.9.0`** (docs.rs) — confirmed shape: `native_pty_system() -> Box<dyn PtySystem + Send>`, `.openpty(PtySize{rows,cols,pixel_width,pixel_height}) -> Result<PtyPair>` where `PtyPair{master, slave}`, `MasterPty::resize(&self, PtySize)`/`.take_writer()`/`.try_clone_reader()`, `CommandBuilder::new(program)`, `SlavePty::spawn_command(CommandBuilder) -> Result<Box<dyn Child + Send + Sync>>`. High confidence in this overall shape (a well-known, API-stable wezterm-authored crate); exact arg-builder method name and `Child`'s kill signature to be spot-checked via `cargo doc -p portable-pty --open` (works without a linker) before writing the final code, rather than guessed.

---

## 4. Design tokens — concrete radius + monospace system

`desktop/tailwind.config.ts` (`theme.extend`):

```ts
borderRadius: {
  lg: "0.75rem",   // unchanged (12px) — docked/persistent panels: console-panel, channel strips, status bar
  xl: "1rem",      // NEW explicit override (16px, was accidentally == lg) — transient floating overlays: Dialog, CommandPalette, terminal panel
  md: "0.5rem",    // unchanged (8px) — interactive controls: buttons, inputs, selects, tabs
  sm: "0.3rem",    // unchanged — dense chrome (small buttons, kbd tags)
  // `full` stays Tailwind's default (9999px) — pills/indicators/switch/avatars; now documented as intentional.
},
fontFamily: {
  sans: [/* unchanged */],
  mono: ["ui-monospace", '"Cascadia Code"', '"SF Mono"', "Consolas", '"JetBrains Mono"', "monospace"],
},
```

`splash.tsx`'s one-off `rounded-2xl` logo badge becomes `rounded-xl` (still generously rounded; the splash is itself a transient full-window moment, so it reuses the "floating/transient" tier rather than needing a 4th tier).

---

## 5. Execution plan — 5 phases, 11 commits

### Phase 1 — Shell restructure (rail nav, status bar, console-panel, tokens)

**New:**
- `components/activity-rail.tsx` — replaces `sidebar.tsx`. Icon-only, fixed `w-14` (56px, reuses the exact width the old sidebar already used collapsed — no new arbitrary value), **no expand/collapse state** (VS Code's activity bar has one width, always — the whole `manualCollapse` mechanism is retired). Top-to-bottom: 5 primary nav icons (Tunnels/Inspector/Domains/Keys/Billing) → hairline divider → Account (small circular avatar/initials, reusing `sidebar.tsx`'s existing `Profile` fetch logic) + Settings → spacer. Active screen gets a left-edge accent bar (adapts the existing convention). **The sidebar's Log-out button + confirm dialog is dropped, not relocated** — it's a full duplicate of `settings.tsx`'s existing "Disconnect" button + confirm dialog (same callback, same copy); removing the sidebar naturally leaves Settings as the one home for this action, resolving an existing duplication for free.
- `components/status-bar.tsx` — new, docked at the very bottom of the *whole window* (sibling to the rail+content row, spans full width under the rail too — matches VS Code). `h-6`, `border-t border-border bg-surface-2`. Content: `StatusDot` (signal-ring motif) + connection text, transport kind, active tunnel count, app version — `text-[11px]`, numeric bits in `font-mono tabular`.

**Modified:**
- `tailwind.config.ts` — radius/font changes above.
- `components/splash.tsx` — `rounded-2xl` → `rounded-xl`.
- `components/ui/card.tsx` — evolve in place (same exported API, so screens inherit automatically): `CardHeader` gains `border-b border-border` (the concrete hairline divider); `CardTitle` becomes `text-xs font-semibold uppercase tracking-wide text-secondary` (systematizes the label pattern already used ad hoc by `Stat` and Inspector); `CardContent` drops the special-cased `pt-0`.
- `components/titlebar.tsx` — remove the connection-status pill (status bar becomes the single owner — see Decision 1 below); keep brand mark, palette trigger, theme toggle, window controls.
- `App.tsx` — swap `<Sidebar>` for `<ActivityRail>`; add `<StatusBar>`; delete `manualCollapse` state. Reserve (but don't populate) a `terminalOpen` slot — Phase 1 doesn't add a terminal rail icon at all, avoiding a dead button for 4 phases; Phase 5 adds it once the feature is real.

**Deleted:** `components/sidebar.tsx`.

**Commits:**
1. `style(desktop): declare deliberate radius + monospace token system` — `tailwind.config.ts`, `splash.tsx`.
2. `feat(desktop): replace sidebar+card shell with activity rail, status bar, console-panel` — `activity-rail.tsx`(new), `status-bar.tsx`(new), `card.tsx`, `titlebar.tsx`, `App.tsx`, delete `sidebar.tsx`.

### Phase 2 — Screen-by-screen redesign onto the console-panel language

- **`tunnels.tsx` (the hero treatment):** each `TunnelCard` gets a 2px left-edge accent bar (tone-colored, reuses existing status-tone mapping) — the "this channel is live" cue. Sparkline promoted from a small 30px band into a full-width ~48-56px band directly under the header (before stats), with the `{rate}/s` figure overlaid top-right in larger monospace. 4-stat footer gets `divide-x divide-border` so Requests/Conns/In/Out read as discrete gang-meter modules.
- **`inspector.tsx`:** lightest touch — it already doesn't use `Card` and is already the closest thing in the app to the target aesthetic; only minor alignment tweaks alongside the new rail/status-bar.
- **`domains.tsx`/`keys.tsx`/`billing.tsx`/`account.tsx`/`settings.tsx`:** inherit the Phase-1 `Card` evolution automatically; this phase tightens layout/spacing to match, and fixes `keys.tsx`'s hand-rolled key-row divs to match the systematized panel language. (Swapping `keys.tsx`'s hand-rolled *empty state* for `<Empty>` is deliberately deferred to Phase 3 — a consistency fix, not a redesign one.)

**Commits:**
3. `feat(desktop): channel-strip redesign for the Tunnels screen` — `tunnels.tsx`.
4. `style(desktop): align Inspector spacing to the console-panel system` — `inspector.tsx`.
5. `style(desktop): restyle Domains/Keys/Billing/Account/Settings onto console panels` — 5 screens.

### Phase 3 — Consistency / polish pass

**New:**
- `components/ui/inline-alert.tsx` — `InlineAlert({tone, icon?, children})`, tone union matches `Badge`'s existing tones. Replaces the verbatim-duplicated error-banner markup.
- `components/ui/skeleton.tsx` — one primitive (`animate-pulse bg-border/60 rounded-md`), composed per-screen rather than four bespoke skeletons.

**Modified:**
- `start-tunnel-dialog.tsx`, `login.tsx` (both spots), `settings.tsx` (load-error banner), `keys.tsx` (`NewKeyDialog` warning) → `<InlineAlert>`.
- `account.tsx`/`billing.tsx`/`domains.tsx`/`keys.tsx` → real skeleton states replacing bare spinners/blank-while-loading.
- `keys.tsx` → swap hand-rolled empty state for `<Empty icon={KeyRound} .../>`.
- Button-state audit — concrete, found-by-reading, not vague: `titlebar.tsx`'s raw search-trigger `<button>`, `login.tsx`'s two raw text-link buttons, `toast.tsx`'s raw dismiss button → converted to `<Button variant="ghost"|"link">` where semantics genuinely match. *Not* touched: command-palette result rows (listbox semantics), login's password-reveal icon toggle (inline toggle, not a CTA).
- `window-controls.tsx` and `lib/types.ts` — fix the two stale Wails-era comments.

**Commits:**
6. `refactor(desktop): extract InlineAlert, replace duplicated error-banner markup`
7. `feat(desktop): skeleton loading states for Account/Billing/Domains/Keys`
8. `chore(desktop): Empty-component consistency, button-state audit, stale Wails-era comment cleanup`

### Phase 4 — Terminal Rust backend

**New: `src-tauri/src/pty.rs`** — architecture (exact method-chain syntax to be finalized against `cargo doc -p portable-pty` at write-time, per correction #8 above, not guessed):
- `PtyState(Mutex<HashMap<String, PtySession>>)` managed state, same pattern as the existing `Sidecar` handle in `main.rs`. `PtySession` holds the `MasterPty`, a writer handle, and the `Child`.
- `default_shell()` — Windows: try `pwsh.exe`, fall back to `powershell.exe`; Unix: `$SHELL` env var, fall back to `/bin/bash`. **Never** derived from remote/agent-sourced data.
- `pty_spawn(app, state, cwd: Option<String>, rows, cols, on_data: Channel<Vec<u8>>) -> Result<String, String>` — opens the PTY (`native_pty_system().openpty(PtySize{...})`), resolves `cwd` via `app.path().home_dir()` (reuses the *same* call `main.rs` already makes for `~/.trqsh` — no new `dirs` crate), spawns the shell via `CommandBuilder`, spins a background thread reading the master and forwarding chunks through `on_data.send(...)`, stores the session under a new atomic-counter id (no `uuid` crate needed), returns the id.
- `pty_write(state, id, data: String)`, `pty_resize(state, id, rows, cols)`, `pty_kill(state, id)`.
- `pub fn stop_all(state: &PtyState)` — called from `main.rs`'s *existing* `RunEvent::Exit | ExitRequested` handler, alongside the current `stop_agent(...)` call, killing every live PTY child on app exit.

**Why `Channel<Vec<u8>>`, not `Channel<String>`:** PTY output can split a multi-byte UTF-8 sequence (or an ANSI escape) across two reads. Lossy-decoding each chunk independently would corrupt any character straddling a chunk boundary — a real, visible bug for non-ASCII output (emoji, box-drawing progress bars, non-English filenames). This is exactly why xterm.js has its own stateful streaming decoder and why VS Code's own terminal backend (`node-pty`) pipes raw bytes. `Vec<u8>` round-trips correctly via serde regardless of whether Tauri's `Channel` has a binary fast path.

**Modified:**
- `Cargo.toml` — add `portable-pty = "0.9"`. **No other new crates** — shell resolution is try-then-fallback (no `which`), IDs are an atomic counter (no `uuid`), cwd reuses `app.path().home_dir()` (no `dirs`) — deliberately zero incidental dependencies.
- `main.rs` — `mod pty;`, register the 4 commands in the existing single `generate_handler!`, `.manage(pty::PtyState::default())` alongside `.manage(Sidecar::default())`, extend the existing exit handler to call `pty::stop_all(...)`.
- `capabilities/default.json` — documentation-only update (per correction #1) explaining the 4 PTY commands are app-registered with no separate capability entry, and that the real boundary is the narrow command surface + argv-only spawning.

**No changes anywhere in `internal/agent/*`.**

**Already-covered for free:** `src-tauri/nsis-hooks.nsh` already runs `taskkill /F /T /IM trqsh-desktop.exe` on install/uninstall — `/T` tree-kills child processes, so any live PTY shell (a child of the app) is already reaped by this existing hook with zero changes needed.

**Before this commit:** run `cargo check` (and ideally `clippy`, `fmt --check`) from `desktop/src-tauri` — first against the unmodified tree per correction #2, then iteratively against the new code.

**Commit:**
9. `feat(desktop): PTY session backend via portable-pty` — `Cargo.toml`, `pty.rs`(new), `main.rs`, `capabilities/default.json`.

### Phase 5 — Terminal frontend + wiring

**New:**
- `lib/pty.ts` — the only other file besides `lib/agent.ts` importing `Channel`/`invoke` from `@tauri-apps/api/core`, dynamically imported the same way `lib/agent.ts` already does (so browser-only `pnpm dev` never crashes on import). `ptyClient.spawn({cwd?, rows, cols}, onData: (bytes: Uint8Array) => void) -> Promise<string>`, `.write(id, data)`, `.resize(id, rows, cols)`, `.kill(id)`. Gated by the existing `hasTauri()`.
- `components/terminal-panel.tsx` — docks *within the main content column*, below the routed screen and above the always-pinned status bar (not full-width under the rail — matches VS Code exactly, see Decision 2). Tab bar: new tab spawns a PTY + `xterm.Terminal`+`FitAddon` pair; closing kills that PTY and disposes the terminal. **All tab DOM nodes stay mounted simultaneously** (toggled via `hidden`, never conditionally unmounted) — unmounting an xterm instance on tab-switch would destroy scrollback and force a respawn, which no real terminal multiplexer does. Resize: `ResizeObserver` + `FitAddon.fit()` on panel-height drag or window resize, followed by `ptyClient.resize(id, term.rows, term.cols)` so the kernel-level PTY size matches (get this wrong and TUI programs like vim/htop render garbled). Height drag-handle: hand-rolled mouse tracking, matching this codebase's existing no-drag-library ethos. Theme mapping: reads the app's own RGB CSS vars via `getComputedStyle`, maps into an xterm `ITheme` (background→`--page`, cursor→`--primary`, ANSI red/green/yellow/blue→`--critical`/`--good`/`--warning`/`--wire`) so it matches the palette instead of a generic black box; recomputed when the panel becomes visible (simplest approach, not a live `MutationObserver`). Graceful no-Tauri fallback (disabled explanatory state) when running in browser-only dev mode.

**Modified:**
- `package.json` (+lockfile, generated via `pnpm add`, never hand-edited) — add `@xterm/xterm`, `@xterm/addon-fit`.
- `components/activity-rail.tsx` — add the terminal toggle (`SquareTerminal` from `lucide-react`, already a dependency) at the bottom of the rail. Shows a `StatusDot status="online"` overlay when 1+ PTY sessions are alive, regardless of panel open/closed — see Decision 5.
- `App.tsx` — mount `<TerminalPanel>` in the Phase-1-reserved slot, wire `terminalOpen` state.
- `lib/hooks.ts` — extend `useHotkeys`'s combo-matching with a literal `ctrl` token (matched directly against `e.ctrlKey`, independent of the platform-dependent `mod` resolution), then register `{combo: "ctrl+\`", allowInInput: true, handler: toggleTerminal}` — see Decision 3 for why this is a real code change, not just wiring.

**Commits:**
10. `feat(desktop): xterm-based terminal panel with multi-session tabs` — `package.json`+lock, `lib/pty.ts`(new), `terminal-panel.tsx`(new). Typechecks/builds standalone before being wired anywhere.
11. `feat(desktop): wire the terminal panel into the activity rail and Ctrl+backtick` — `activity-rail.tsx`, `App.tsx`, `hooks.ts`.

---

## 6. Dependencies summary

`desktop/package.json` (`dependencies`): `@xterm/xterm` (~5.5, confirm exact latest via `pnpm add ...@latest`), `@xterm/addon-fit` (~0.11, same).
`desktop/src-tauri/Cargo.toml` (`[dependencies]`): `portable-pty = "0.9"` — nothing else.

---

## 7. Commit table (full list — 11 commits, 5 phases)

| # | Phase | Commit | Files |
|---|-------|--------|-------|
| 1 | 1 | `style(desktop): declare deliberate radius + monospace token system` | `tailwind.config.ts`, `splash.tsx` |
| 2 | 1 | `feat(desktop): replace sidebar+card shell with activity rail, status bar, console-panel` | `activity-rail.tsx`★, `status-bar.tsx`★, `card.tsx`, `titlebar.tsx`, `App.tsx`, del `sidebar.tsx` |
| 3 | 2 | `feat(desktop): channel-strip redesign for the Tunnels screen` | `tunnels.tsx` |
| 4 | 2 | `style(desktop): align Inspector spacing to the console-panel system` | `inspector.tsx` |
| 5 | 2 | `style(desktop): restyle Domains/Keys/Billing/Account/Settings onto console panels` | 5 screens |
| 6 | 3 | `refactor(desktop): extract InlineAlert, replace duplicated error-banner markup` | `inline-alert.tsx`★ + 4 call sites |
| 7 | 3 | `feat(desktop): skeleton loading states for Account/Billing/Domains/Keys` | `skeleton.tsx`★ + 4 screens |
| 8 | 3 | `chore(desktop): Empty-component consistency, button-state audit, stale Wails-era comment cleanup` | `keys.tsx`, `titlebar.tsx`, `login.tsx`, `toast.tsx`, `window-controls.tsx`, `types.ts` |
| 9 | 4 | `feat(desktop): PTY session backend via portable-pty` | `Cargo.toml`, `pty.rs`★, `main.rs`, `capabilities/default.json` |
| 10 | 5 | `feat(desktop): xterm-based terminal panel with multi-session tabs` | `package.json`+lock, `lib/pty.ts`★, `terminal-panel.tsx`★ |
| 11 | 5 | `feat(desktop): wire the terminal panel into the activity rail and Ctrl+backtick` | `activity-rail.tsx`, `App.tsx`, `hooks.ts` |

★ = new file. Each commit leaves the tree typechecking/building; commit 9 leaves `cargo check` passing (pending the build-script caveat in correction #2) even though full link can't be verified locally.

---

## 8. Verification plan

**Fully local (phases 1, 2, 3, 5):** `pnpm typecheck`, `pnpm build`, and `pnpm dev` (no Tauri needed — real browser verification against a locally-run `trqsh daemon` with `TRQSH_CONTROL_NO_AUTH=1`) for every visual/structural change including the terminal panel's UI/layout/theming (only the actual PTY spawn/write round-trip needs a real Tauri window).

**Automatic CI:** `ci.yml`'s `frontends` matrix runs `pnpm build` for `desktop` on every push/PR — covers every phase's frontend change automatically (Phase 5's new deps require the lockfile to be committed alongside `package.json`, or `--frozen-lockfile` fails immediately).

**Rust (Phase 4):** `cargo fmt --check` always works; `cargo check`/`cargo clippy` likely work (confirm first against the unmodified tree); `cargo build`/`pnpm tauri dev`/`pnpm tauri build` not expected to work here (matches this repo's established pattern for every prior Rust change). **`desktop-build.yml` must be manually dispatched** (`gh workflow run desktop-build.yml`) after Phase 4 and again after Phase 5 — it's the only thing that actually compiles+links on Windows/macOS/Linux, and it does not run automatically.

**Manual smoke pass** once CI produces installers: activity rail navigation, status bar live values, each redesigned screen, and the terminal (spawn/type/resize/multiple tabs/close) at least on Windows.

---

## 9. Decisions made (resolving judgment calls raised during planning)

1. **Titlebar connection pill removed**, not duplicated — the new status bar becomes the single owner of "are we connected," avoiding two redundant vitals indicators.
2. **Terminal panel docks within the main content column only** (not under the rail); the status bar spans the full width under everything including the rail. This matches VS Code exactly — the explicit spec — rather than being left ambiguous.
3. **`Ctrl+backtick` implemented as a true, platform-invariant binding**, extending `useHotkeys` with a literal-ctrl token, rather than the simpler `mod+backtick` (which would be `Cmd+backtick` on Mac — already a macOS system shortcut for window-cycling, and would collide with it). This is a small real code change, chosen because exact VS Code muscle-memory parity was explicitly requested.
4. **PTY commands get a documentation-only capability update**, not a hardened opt-in ACL — verified the capability system doesn't gate plain app commands in this app's config today (finding #1), so the simpler path is equally secure in practice and avoids an unverifiable-locally moving part.
5. **The rail's terminal live-dot means genuinely-live activity** (a shell session actually running), not "currently selected screen" — consistent with how the signal-ring motif is used everywhere else in the app (always "live," never "selected").
6. **PTY output streams as `Vec<u8>`, not `String`**, end-to-end — avoids corrupting multi-byte UTF-8/ANSI sequences split across read chunks, the same reason VS Code's own terminal backend streams raw bytes rather than decoded strings.

---

## 10. Critical files

- `desktop/src/App.tsx`
- `desktop/tailwind.config.ts`
- `desktop/src/components/ui/card.tsx`
- `desktop/src-tauri/src/main.rs`
- `desktop/src-tauri/Cargo.toml`
- `desktop/src/lib/hooks.ts`
- `desktop/src/lib/agent.ts` (pattern reference for the new `lib/pty.ts`)

---

## 11. Scope boundaries (respected throughout, no exceptions)
- `internal/agent/cli/tui/*` — untouched (another AI session's in-flight, uncommitted work).
- `web/site`, `web/dashboard` — untouched (separate workstreams per standing project rule).
- `internal/agent/*` (the Go tunnel engine) — untouched; the terminal feature is entirely Rust/OS-process, with zero reason to import or modify any of it.

---

## 12. Progress log

- **Commits 1–8 (Phases 1–3) shipped.** Activity rail + status bar + console-panel shell, Tunnels channel-strip redesign, Inspector alignment, Domains/Keys/Billing/Account/Settings restyle, InlineAlert extraction, skeleton loading states, and the consistency cleanup pass (Empty-component reuse, focus-visible states on 6 previously-unstyled interactive elements, stale Wails-era comment fixes) — all verified visually via Playwright screenshots against a locally-run `trqsh daemon` in both light and dark mode, not just typecheck.
- **Commit 9 (PTY Rust backend) written, not yet locally compiled.** `src-tauri/src/pty.rs` (new), `Cargo.toml` (+`portable-pty = "0.9"`), `main.rs` wiring, `capabilities/default.json` doc update. Every `portable-pty`/`tauri::ipc::Channel` API signature used was fetched fresh from docs.rs/Tauri docs and cross-checked (not guessed) — see inline comments. **Local verification ceiling on this box turned out lower than hoped:** `cargo check` (not just `cargo build`) fails, because build-script/proc-macro compilation for foundational deps (serde, proc-macro2, etc.) requires the linker even under `check` — and this box's `link.exe` resolves to Git Bash's POSIX `link` utility, while the real MSVC linker is absent (VS Installer shell present, C++ Build Tools workload never installed). Did not install it mid-session — multi-GB, and C: had just been freed from 0 bytes by the user. `cargo fmt --check` **does** work (rustfmt component installed) and passes clean — used to catch real formatting issues. Everything past formatting defers to CI (`desktop-build.yml`, manual dispatch) per the plan's own established fallback.
- **First CI dispatch (run 31152415456) failed on all 3 OSes — fast (18-33s), before real Rust compilation started.** Root cause: `tauri (v2.11.5) : @tauri-apps/api (v2.9.0)` version mismatch — Tauri's own CLI hard-fails on that gap. Pre-existing latent issue (Cargo.toml's `tauri = "2"` had no committed lockfile ever pinning it before this session), not caused by the PTY work; my new Cargo.lock (committed in commit 9) was simply the first thing to surface it. Fixed in its own commit: bumped `@tauri-apps/api`/`@tauri-apps/cli` to 2.11.x to match.
- **Commits 10–11 (Phase 5) shipped.** `lib/pty.ts` (typed Channel-based client, `Vec<u8>`→`number[]` over IPC confirmed via the installed `@tauri-apps/api` type defs, converted to `Uint8Array` at the boundary since xterm's own `write()` accepts raw bytes directly — no manual UTF-8 decoding needed anywhere), `terminal-panel.tsx` (tabs stay mounted/CSS-hidden, ResizeObserver-driven PTY resize, palette derived from the app's CSS vars, `attachCustomKeyEventHandler` used to guarantee Ctrl+` always reaches the global toggle even when xterm has focus), `hooks.ts` extended with a literal `ctrl` token for true cross-platform Ctrl+` (VS Code parity, avoids the Mac Cmd+` collision), activity-rail + App.tsx wiring. **A real bug was found and fixed via visual verification, not just typecheck:** the "open with one session" effect ran regardless of the `hasTauri()` fallback branch (hooks can't follow an early return), so the rail's live-session dot lit up falsely in browser-only dev even though no PTY was ever attempted — gated the effect on `hasTauri()` too. Confirmed via Playwright: rail icon placement, panel docking (within the main content column, not under the rail, matching Decision 2), the no-Tauri fallback message, and Ctrl+` toggling the panel open/closed end-to-end.
- **Next:** dispatch CI again now that the version mismatch is fixed, to get the first real Windows/macOS/Linux compile+link proof of the PTY backend and the full terminal feature.
