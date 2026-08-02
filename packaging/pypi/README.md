# trqsh (PyPI)

Install the [trqsh](https://trqsh.uz) CLI with pip. This package fetches the
signed, prebuilt binary for your platform from the GitHub release on first run.

```bash
pip install trqsh
# or, isolated:
pipx install trqsh
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

The console script `trqsh` calls `trqsh.ensure_binary()`, which detects your OS
(`darwin`/`linux`/`windows`) and CPU (`amd64`/`arm64`), downloads
`trqsh_<version>_<os>_<arch>.<ext>` from
`https://github.com/uzcreator/trqsh/releases`, verifies its SHA-256 against
`checksums.txt`, caches the binary under `~/.cache/trqsh/<version>/`, and execs
it. Pure standard library — no runtime dependencies.

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

## Author

trqsh is created and maintained by **Otabek Hamroqulov** — GitHub
[@Hamroqulovv](https://github.com/Hamroqulovv). Licensed Apache-2.0.
