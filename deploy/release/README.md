# Release & distribution

Artifacts and package-manager manifests produced by `.github/workflows/release.yml`
(triggered on a `v*` tag).

```
release/
├── install.sh            curl | sh installer for the CLI (Linux/macOS)
├── gen-update-feed.sh    emits the GUI/CLI auto-update feed (latest.json)
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
| Desktop GUI | signed bundles per OS (macOS notarized, Windows Authenticode) |

## Auto-update feed

`gen-update-feed.sh <tag>` emits `latest.json` matching the shape the GUI polls
(`gui/update.go` → `{version, notes, url}`), published to
`https://downloads.trqsh.uz/desktop/latest.json` (the Spaces `releases` bucket).

```bash
deploy/release/gen-update-feed.sh v0.1.0
```

## Required secrets (release environment)

`HOMEBREW_TAP_TOKEN`, `APPLE_ID` / `APPLE_TEAM_ID` / `APPLE_APP_PASSWORD`
(notarization), `WINDOWS_CERT_BASE64` / `WINDOWS_CERT_PASSWORD` (Authenticode),
`SPACES_ACCESS_ID` / `SPACES_SECRET_KEY` (feed upload).
