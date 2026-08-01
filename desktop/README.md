# desktop — trqsh desktop app

A native desktop client (**Tauri v2**: Rust shell + React/TypeScript UI) that drives the
trqsh agent from a GUI: sign in, start/stop tunnels, watch the live request inspector, and
manage keys and domains. macOS / Windows / Linux.

Part of the [trqsh monorepo](../README.md). The agent it wraps is the same Go binary built
from [`cmd/trqsh`](../cmd/trqsh); the UI shares design tokens with
[`web/dashboard`](../web/dashboard) and [`web/site`](../web/site).

## Architecture — a thin client over the local agent

The UI never touches the tunnel data path directly:

1. The app bundles the **Go agent** (`trqsh`) as a Tauri **sidecar**
   (`src-tauri/binaries/trqsh-<target-triple>`).
2. On launch, the Rust shell spawns `trqsh daemon`, which serves a token-authenticated
   loopback control API on `127.0.0.1:4041` (token in `~/.trqsh/control.token`).
3. The React UI is a plain `fetch` / `EventSource` client over that local API.

So the UI toolkit has zero effect on tunnel speed or server scale — those live in the agent
and the edge. Config and the API key live in `~/.trqsh/`, separate from the install dir, so
updates and uninstalls never touch user data.

## The agent sidecar

Tauri expects the agent binary at `src-tauri/binaries/trqsh-<host-triple>` (`.exe` on
Windows). Two ways to get it:

- **Local dev** — build it from this repo, from the repo root:

  ```bash
  go build -o "desktop/src-tauri/binaries/trqsh-$(rustc -vV | sed -n 's/^host: //p')" ./cmd/trqsh
  # on Windows, add a .exe suffix to the output name
  ```

- **CI** — [`.github/workflows/desktop-build.yml`](../.github/workflows/desktop-build.yml)
  downloads the released `trqsh` CLI for each platform from the public
  [trqsh-uz/cli](https://github.com/trqsh-uz/cli) releases, verifies its checksum, and drops
  it in under the host-triple name. A packaged build therefore bundles the **most recently
  released** CLI, not backend tip-of-main — deliberate, so installers always ship a stable
  agent.

## Run (local dev)

```bash
pnpm install
pnpm tauri dev        # native window; needs Rust + the platform's WebView

# UI-only in a browser (no native shell), against a running local agent:
pnpm dev              # http://localhost:1420
```

Requires **Rust** (stable) and, on Linux, the WebKitGTK / AppIndicator dev packages listed
in the build workflow. On Windows the UI renders in WebView2.

## Releases

CI builds installers for Windows / macOS / Linux and publishes them to the public
**[trqsh-uz/gui](https://github.com/trqsh-uz/gui)** releases repo (the `gui` name predates
the Wails→Tauri rewrite and is kept so existing download URLs keep working). Trigger via a
`v*` tag or the workflow's manual dispatch with `publish: true`.
