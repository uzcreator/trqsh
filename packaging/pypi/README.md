<div align="center">

<img src="https://cdn.jsdelivr.net/gh/uzcreator/trqsh@main/packaging/assets/banner.svg" alt="trqsh — QUIC-first tunnels to localhost" width="100%" />

[![PyPI](https://img.shields.io/pypi/v/trqsh?color=2dd4bf)](https://pypi.org/project/trqsh/)
[![Downloads](https://img.shields.io/pypi/dm/trqsh?color=2dd4bf)](https://pypi.org/project/trqsh/)
[![Python](https://img.shields.io/pypi/pyversions/trqsh?color=2dd4bf)](https://pypi.org/project/trqsh/)
[![release](https://img.shields.io/github/v/release/uzcreator/trqshcli?color=2dd4bf&label=release)](https://github.com/uzcreator/trqshcli/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-2dd4bf)](https://github.com/uzcreator/trqsh/blob/main/LICENSE)

</div>

**trqsh** puts a public HTTPS URL in front of anything on your machine — a dev
server, a webhook receiver, a game server, a raw TCP/UDP listener — in one
command. It's a QUIC-first tunnel (real HTTP/3, not a rebrand of TCP) with a
proper interactive console, live traffic inspection, real Let's Encrypt TLS,
and a fully open-source stack you can self-host end to end. This package
installs the signed `trqsh` CLI binary for your platform: a thin wrapper
around a small Go binary, verified by SHA-256 on every install.

```bash
pip install trqsh
# or, isolated:
pipx install trqsh

trqsh login        # sign in through your browser
trqsh http 3000    # → a public HTTPS URL for localhost:3000
```

<img src="https://cdn.jsdelivr.net/gh/uzcreator/trqsh@main/packaging/assets/terminal-demo.svg" alt="trqsh http 3000 opening a live tunnel" width="100%" />

## Why trqsh

- **QUIC-first, TCP fallback** — built on HTTP/3's transport from the ground
  up, not HTTP/1.1 with a marketing label; falls back automatically where UDP
  is blocked.
- **Every protocol** — `http`, `tcp`, and `udp` tunnels, not just HTTP.
- **A real console, not a log spinner** — `/pin traffic`, `/pin status`, and
  friends keep live panels on screen while you keep working, with command
  history, autocomplete, and a scrolling transcript. See below.
- **Remote control from your phone** — `/qr` pairs the console to a scanned
  QR: watch the live transcript and tunnels, or type commands back, from
  anywhere.
- **Real TLS** — Let's Encrypt via CertMagic, not a self-signed cert with a
  browser warning.
- **Background tunnels** (`-d`) that outlive your terminal, reserved
  subdomains, custom domains, and a live request inspector.
- **Signed, verifiable releases** — every download is checksummed and the
  checksum file itself is cosign-signed in CI (see Security below).
- **Fully open source (Apache-2.0)** — read every line, self-host the whole
  edge + control plane instead of trusting a black box.

## The console

Bare `trqsh` (or `trqsh http 3000`) opens an interactive, slash-command
console — type `/` to browse everything it can do, pin live panels above the
prompt, and keep an eye on traffic without leaving the terminal:

<img src="https://cdn.jsdelivr.net/gh/uzcreator/trqsh@main/packaging/assets/console-demo.svg" alt="the trqsh console: welcome banner, an active tunnel, and pinned traffic + status panels" width="100%" />

## Commands

| Command | Does |
|---|---|
| `/http <port>` | Expose a local HTTP port |
| `/tcp <port>` / `/udp <port>` | Expose a local TCP/UDP port |
| `/start` | Start every tunnel from your config file at once |
| `/ls` | List running tunnels |
| `/open <id>` | Open a tunnel's public URL in your browser |
| `/qr [id\|stop]` | Pair this console to your phone (or show a tunnel's QR) |
| `/copy [id]` | Copy a tunnel URL to the clipboard |
| `/requests` | Show recently captured requests |
| `/pin <traffic\|tunnels\|status>` | Keep a live panel on screen |
| `/stop <id\|all>` / `/down` | Stop a tunnel, or the whole daemon |
| `/subdomains`, `/domains` | Reserve subdomains, manage custom domains |
| `/whoami`, `/login`, `/logout` | Account, plan, usage; sign in via browser |
| `/update`, `/version` | Self-update in place; show version |

Run `/help` inside the console for the full, current list.

## Security

Every release archive ships with a `checksums.txt` that this package verifies
before it ever execs the binary. The checksum file itself is keyless-signed
with [cosign](https://github.com/sigstore/cosign) via GitHub Actions' OIDC
token — no key to leak, no key to trust blindly:

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/uzcreator/trqsh/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

## How it works

The console script `trqsh` calls `trqsh.ensure_binary()`, which detects your OS
(`darwin`/`linux`/`windows`) and CPU (`amd64`/`arm64`), downloads
`trqsh_<version>_<os>_<arch>.<ext>` from
`https://github.com/uzcreator/trqshcli/releases`, verifies its SHA-256 against
`checksums.txt`, caches the binary under `~/.cache/trqsh/<version>/`, and execs
it. Pure standard library — no runtime dependencies.

## Troubleshooting: "'trqsh' is not recognized" on Windows

`pip install trqsh` without admin rights (the common case) installs the
`trqsh.exe` launcher into a per-user Scripts folder that isn't always on
PATH. Every `trqsh` invocation checks for this and fixes it automatically —
but pip has no post-install hook, so the very first invocation can't (if
PATH is broken, that first `trqsh` can't run at all to fix it). Run it once
via the Python launcher, which is always on PATH:

```bash
python -m trqsh version
# then open a new terminal — bare `trqsh` now works
```

## Environment overrides

| Variable | Purpose |
|---|---|
| `TRQSH_VERSION` | Pin a specific release (defaults to the package version). |
| `TRQSH_REPO` | Alternate `owner/repo` for the release download. |
| `TRQSH_DOWNLOAD_BASE` | Full base URL of a mirror serving the release assets (for air-gapped/internal mirrors). |
| `TRQSH_SKIP_CHECKSUM=1` | Skip SHA-256 verification (not recommended). |

## Uninstall

```bash
trqsh uninstall        # remove local data (config, key, cache) + stop tunnels
pip uninstall trqsh    # then remove the package itself
```

Run `trqsh uninstall` first: `pip uninstall` drops the package but leaves your saved
key, control token, logs, and the cached binary under `~/.cache/trqsh/` behind —
`trqsh uninstall` clears those and stops any background daemon. Add `-y` to skip the
confirmation.

## Links

[Website](https://trqsh.uz) · [Source](https://github.com/uzcreator/trqsh) ·
[CLI releases](https://github.com/uzcreator/trqshcli) ·
[Issues](https://github.com/uzcreator/trqsh/issues)

## Author

trqsh is created and maintained by **Otabek Hamroqulov** — GitHub
[@Hamroqulovv](https://github.com/Hamroqulovv). Licensed Apache-2.0.
