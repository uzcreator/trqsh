# trqsh (PyPI)

Install the [trqsh](https://trqsh.uz) CLI with pip. This package fetches the
signed, prebuilt binary for your platform from the GitHub release on first run.

```bash
pip install trqsh
# or, isolated:
pipx install trqsh

trqsh http 3000        # expose localhost:3000 over a public HTTPS URL
```

## How it works

The console script `trqsh` calls `trqsh.ensure_binary()`, which detects your OS
(`darwin`/`linux`/`windows`) and CPU (`amd64`/`arm64`), downloads
`trqsh_<version>_<os>_<arch>.<ext>` from
`https://github.com/trqsh/trqsh/releases`, verifies its SHA-256 against
`checksums.txt`, caches the binary under `~/.cache/trqsh/<version>/`, and execs
it. Pure standard library — no runtime dependencies.

## Environment overrides

| Variable | Purpose |
|---|---|
| `TRQSH_VERSION` | Pin a specific release (defaults to the package version). |
| `TRQSH_REPO` | Alternate `owner/repo` for the release download. |
| `TRQSH_SKIP_CHECKSUM=1` | Skip SHA-256 verification (not recommended). |

Licensed Apache-2.0.
