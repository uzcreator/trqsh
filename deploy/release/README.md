# Release & distribution

Artifacts and package-manager manifests produced by `.github/workflows/release.yml`
(triggered on a `v*` tag).

```
release/
├── install.sh            curl | sh installer for the CLI (Linux/macOS)
├── gen-update-feed.sh    dormant version-feed generator (latest.json); no live consumer
└── scoop/trqsh.json       Scoop (Windows) manifest template
```

## Channels

| Target | Mechanism |
|---|---|
| CLI (Linux/macOS) | `curl -fsSL https://trqsh.uz/install.sh \| sh` → GitHub Releases |
| CLI (macOS) | Homebrew tap `trqsh/tap` (goreleaser `brews:`) |
| CLI (Windows) | Scoop (`scoop/trqsh.json`) + winget manifest PR |
| CLI (Linux) | `.deb` / `.rpm` (goreleaser `nfpms:`) |
| Edge (`trqshd`) | container image `ghcr.io/trqsh/edge` + archives |
| Desktop app | Tauri installers per OS via `desktop-build.yml` → `trqsh-uz/gui` releases |

## Auto-update feed (dormant)

`gen-update-feed.sh <tag>` emits a `latest.json` version feed
(`{version, notes, url}`). It currently has **no consumer**: the old Wails GUI
that polled it was removed, and the Tauri desktop app checks for updates through
the bundled agent, which queries the `trqsh-uz/gui` releases API directly (see
`internal/agent/update.go`). The script is kept for potential future use.

```bash
deploy/release/gen-update-feed.sh v0.1.0
```

## Required secrets (release environment)

`RELEASE_REPO_TOKEN` (publish to the public `trqsh-uz/cli` + `trqsh-uz/gui`
releases), `HOMEBREW_TAP_TOKEN` (Homebrew tap), and `NPM_TOKEN` / `PYPI_TOKEN`
(language wrappers). Desktop builds are not code-signed yet — `desktop-build.yml`
ships unsigned preview builds.
