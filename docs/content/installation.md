# Installation

trqsh ships as a single CLI binary (`trqsh`) and a desktop app. Both are built from
the same open-source agent core.

## macOS

**Homebrew** (recommended):

```sh
brew install trqsh-uz/tap/trqsh
```

**One-line script:**

```sh
curl -fsSL https://trqsh.uz/install.sh | sh
```

**Desktop app:** download the notarized `.app` from the [download page](/download).

## Windows

**Scoop:**

```sh
scoop install trqsh
```

**Desktop app:** download the Authenticode-signed installer from the
[download page](/download).

## Linux

**One-line script** (installs to `/usr/local/bin`):

```sh
curl -fsSL https://trqsh.uz/install.sh | sh
```

**Packages:** `.deb` and `.rpm` builds are attached to every
[GitHub release](https://github.com/trqsh-uz/trqsh/releases). The desktop app is
available as an AppImage.

## Verify the download

Every release includes a `checksums.txt`. Verify an archive before running it:

```sh
shasum -a 256 -c checksums.txt --ignore-missing
```

## Confirm it works

```sh
trqsh version
```

## Updating

The CLI can update itself, or use your package manager (`brew upgrade trqsh`,
`scoop update trqsh`). The desktop app checks for updates automatically and applies
signed releases. Now continue to the [quickstart](/docs/quickstart).
