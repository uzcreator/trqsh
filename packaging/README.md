# Packaging trqsh

Distribution wrappers so people can install the **trqsh** CLI through whatever
package manager they already use. Every channel ultimately downloads the same
signed release artifact built by GoReleaser (see [`../.goreleaser.yaml`](../.goreleaser.yaml))
from the CLI's own dedicated release repo — source stays in this monorepo
(`uzcreator/trqsh`), but only CLI release artifacts publish to
`uzcreator/trqshcli`, so the release list there is CLI-only (no desktop GUI or
other release types mixed in):

```
https://github.com/uzcreator/trqshcli/releases/download/v<version>/trqsh_<version>_<os>_<arch>.<ext>
```

`<os>` ∈ `darwin|linux|windows`, `<arch>` ∈ `amd64|arm64`, `<ext>` = `zip` on
Windows else `tar.gz`. A `checksums.txt` (SHA-256) sits beside them and every
wrapper verifies against it.

| Channel | Path | Install command | Who builds it |
|---|---|---|---|
| **npm** | [`npm/`](./npm) | `npm i -g trqsh` · `npx trqsh` | wrapper here |
| **PyPI** | [`pypi/`](./pypi) | `pip install trqsh` · `pipx install trqsh` | wrapper here |
| **Shell** | [`install.sh`](./install.sh) | `curl -fsSL https://trqsh.uz/install.sh \| sh` | script here |
| **Scoop** (Win) | [`scoop/trqsh.json`](./scoop) | `scoop install trqsh` | manifest here |
| **winget** (Win) | [`winget/`](./winget) | `winget install trqsh.trqsh` | manifests here |
| **Homebrew** (Mac/Linux) | goreleaser `brews:` | `brew install uzcreator/tap/trqsh` | goreleaser → `uzcreator/homebrew-tap` |
| **apt / dnf** (`.deb`/`.rpm`) | goreleaser `nfpms:` | download from Releases | goreleaser |

The npm and PyPI packages carry **no third-party dependencies** — they detect the
platform, download the archive, verify its checksum, unpack the binary, and exec
it. `install.sh` is also copied to [`../web/site/public/install.sh`](../web/site/public/install.sh)
so the marketing site serves it at `trqsh.uz/install.sh`.

`checksums.txt` is itself keyless-signed at release time with
[cosign](https://github.com/sigstore/cosign) via GitHub Actions' OIDC token (no
key material to manage) — see the `signs:` block in
[`../.goreleaser.yaml`](../.goreleaser.yaml). To verify a download beyond the
checksum (i.e. confirm `checksums.txt` itself really came from this repo's CI,
not just that a binary matches whatever checksums.txt says):

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/uzcreator/trqsh/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

## Releasing (per version)

**Prerequisite (one-time):** the release workflow runs in `uzcreator/trqsh` but
publishes to `uzcreator/trqshcli`, so it needs a classic PAT with `repo` scope
on `trqshcli`, set as the `RELEASE_REPO_TOKEN` secret on the `trqsh` repo
(Settings → Secrets and variables → Actions). Without it, the release job fails
creating the GitHub release (everything else — npm/PyPI — is unaffected).

1. Tag `vX.Y.Z`; CI runs `goreleaser release` → publishes archives + `checksums.txt`
   + the Homebrew tap + `.deb`/`.rpm` to `uzcreator/trqshcli`.
2. **npm:** bump `version` in `npm/package.json`, then `cd npm && npm publish`.
3. **PyPI:** bump `version` in `pypi/pyproject.toml` + `src/trqsh/__init__.py`,
   then `cd pypi && python -m build && twine upload dist/*`.
4. **Scoop:** update `version`/`hash` in `scoop/trqsh.json` (hashes from
   `checksums.txt`) and push to the `uzcreator/scoop-bucket` repo.
5. **winget:** bump the version + `InstallerSha256` in `winget/*.yaml` and open a
   PR to `microsoft/winget-pkgs` (or use `wingetcreate update trqsh.trqsh`).

### Shared overrides

All wrappers honor `TRQSH_VERSION` and `TRQSH_REPO` (and `TRQSH_SKIP_CHECKSUM=1`)
so you can test against a pre-release or a fork before publishing.

## Local smoke test (before a real release exists)

```bash
# npm: platform detection + syntax
cd npm && node -e "console.log(require('./lib/common').target())"

# pypi: import + target mapping
cd pypi && python -c "import sys; sys.path.insert(0,'src'); from trqsh import _runtime; print(_runtime._target())"

# shell: syntax
sh -n install.sh
```
