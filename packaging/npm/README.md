<div align="center">

<img src="https://cdn.jsdelivr.net/gh/uzcreator/trqsh@main/packaging/assets/banner.svg" alt="trqsh — QUIC-first tunnels to localhost" width="100%" />

[![npm](https://img.shields.io/npm/v/@trqsh-uz/trqsh?color=2dd4bf&label=npm)](https://www.npmjs.com/package/@trqsh-uz/trqsh)
[![downloads](https://img.shields.io/npm/dm/@trqsh-uz/trqsh?color=2dd4bf)](https://www.npmjs.com/package/@trqsh-uz/trqsh)
[![release](https://img.shields.io/github/v/release/uzcreator/trqshcli?color=2dd4bf&label=release)](https://github.com/uzcreator/trqshcli/releases)
[![license](https://img.shields.io/badge/license-Apache--2.0-2dd4bf)](https://github.com/uzcreator/trqsh/blob/main/LICENSE)

</div>

**trqsh** puts a public HTTPS URL in front of anything on your machine — a dev
server, a webhook receiver, a game server, a raw TCP/UDP listener — in one
command. It's a QUIC-first tunnel (real HTTP/3, not a rebrand of TCP) with a
proper interactive console, live traffic inspection, real Let's Encrypt TLS,
and a fully open-source stack you can self-host end to end.

```bash
npm install -g @trqsh-uz/trqsh
# or run it once, no install:
npx @trqsh-uz/trqsh http 3000
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
| `/qr [id]` | Show a QR code for a tunnel URL — open it on your phone |
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

- `postinstall` runs `lib/install.js`, which detects your platform (`darwin`,
  `linux`, `win32`) and CPU (`x64`, `arm64`), downloads
  `trqsh_<version>_<os>_<arch>.<ext>` from
  `https://github.com/uzcreator/trqshcli/releases`, verifies its SHA-256 against
  `checksums.txt`, and unpacks the binary into `vendor/`.
- `bin/trqsh.js` execs that binary, forwarding arguments, stdio, and the exit
  code. If install scripts were skipped, it downloads on first run.

## Environment overrides

| Variable | Purpose |
|---|---|
| `TRQSH_VERSION` | Pin a specific release (defaults to the package version). |
| `TRQSH_REPO` | Alternate `owner/repo` for the release download. |
| `TRQSH_DOWNLOAD_BASE` | Full base URL of a mirror serving the release assets (for air-gapped/internal mirrors). |
| `TRQSH_SKIP_CHECKSUM=1` | Skip SHA-256 verification (not recommended). |

No third-party dependencies; unpacking uses PowerShell's `Expand-Archive` on
Windows and `tar` elsewhere.

## Uninstall

```bash
trqsh uninstall                 # remove local data (config, key, cache) + stop tunnels
npm rm -g @trqsh-uz/trqsh       # then remove the package itself
```

Run `trqsh uninstall` first: it stops any background daemon and clears your saved key,
control token, and logs, none of which live inside the npm package so `npm rm` never
touches them. (The downloaded binary itself lives in the package's own `vendor/` dir,
so `npm rm` removes that automatically.) Add `-y` to skip the confirmation.

## Links

[Website](https://trqsh.uz) · [Source](https://github.com/uzcreator/trqsh) ·
[CLI releases](https://github.com/uzcreator/trqshcli) ·
[Issues](https://github.com/uzcreator/trqsh/issues)

## Author

trqsh is created and maintained by **Otabek Hamroqulov** — GitHub
[@Hamroqulovv](https://github.com/Hamroqulovv). Licensed Apache-2.0.
