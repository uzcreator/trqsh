# trqsh (npm)

Install the [trqsh](https://trqsh.uz) CLI through npm. This package downloads the
signed, prebuilt binary for your platform from the GitHub release on install.

```bash
npm install -g @trqsh-uz/trqsh
# or run without installing
npx @trqsh-uz/trqsh http 3000
```

## Commands

```bash
trqsh login                 # sign in through your browser (GitHub/Google)
trqsh whoami                # show your account, plan and usage
trqsh http 3000             # expose localhost:3000 over a public HTTPS URL
trqsh http 3000 -d          # …in the background; returns immediately
trqsh ls                    # list running background tunnels
trqsh open <id|name>        # open a tunnel's public URL in your browser
trqsh stop <id|all>         # stop one tunnel (or all of them)
trqsh down                  # stop the background daemon entirely
trqsh subdomains            # list reserved subdomains
trqsh domains               # list your custom domains
```

Also `trqsh tcp <port>` and `trqsh udp <port>` for raw TCP/UDP tunnels. Run
`trqsh --help` for the full list.

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

Run `trqsh uninstall` first: `npm rm` deletes the package but leaves your saved key,
control token, logs, and downloaded binary behind — `trqsh uninstall` clears those and
stops any background daemon. Add `-y` to skip the confirmation.

## Author

trqsh is created and maintained by **Otabek Hamroqulov** — GitHub
[@Hamroqulovv](https://github.com/Hamroqulovv). Licensed Apache-2.0.
