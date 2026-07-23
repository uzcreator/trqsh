# trqsh (npm)

Install the [trqsh](https://trqsh.uz) CLI through npm. This package downloads the
signed, prebuilt binary for your platform from the GitHub release on install.

```bash
npm install -g trqsh
# or run without installing
npx trqsh http 3000
```

Then:

```bash
trqsh http 3000        # expose localhost:3000 over a public HTTPS URL
```

## How it works

- `postinstall` runs `lib/install.js`, which detects your platform (`darwin`,
  `linux`, `win32`) and CPU (`x64`, `arm64`), downloads
  `trqsh_<version>_<os>_<arch>.<ext>` from
  `https://github.com/trqsh/trqsh/releases`, verifies its SHA-256 against
  `checksums.txt`, and unpacks the binary into `vendor/`.
- `bin/trqsh.js` execs that binary, forwarding arguments, stdio, and the exit
  code. If install scripts were skipped, it downloads on first run.

## Environment overrides

| Variable | Purpose |
|---|---|
| `TRQSH_VERSION` | Pin a specific release (defaults to the package version). |
| `TRQSH_REPO` | Alternate `owner/repo` for the release download. |
| `TRQSH_SKIP_CHECKSUM=1` | Skip SHA-256 verification (not recommended). |

No third-party dependencies; extraction uses the system `tar` (bsdtar on
Windows 10+). Licensed Apache-2.0.
