# Installation

Rift ships as a single CLI binary (`rift`) and a desktop app. Both are built from
the same open-source agent core.

## macOS

**Homebrew** (recommended):

```sh
brew install rift/tap/rift
```

**One-line script:**

```sh
curl -fsSL https://rift.dev/install.sh | sh
```

**Desktop app:** download the notarized `.app` from the [download page](/download).

## Windows

**Scoop:**

```sh
scoop install rift
```

**Desktop app:** download the Authenticode-signed installer from the
[download page](/download).

## Linux

**One-line script** (installs to `/usr/local/bin`):

```sh
curl -fsSL https://rift.dev/install.sh | sh
```

**Packages:** `.deb` and `.rpm` builds are attached to every
[GitHub release](https://github.com/rift/rift/releases). The desktop app is
available as an AppImage.

## Verify the download

Every release includes a `checksums.txt`. Verify an archive before running it:

```sh
shasum -a 256 -c checksums.txt --ignore-missing
```

## Confirm it works

```sh
rift version
```

## Updating

The CLI can update itself, or use your package manager (`brew upgrade rift`,
`scoop update rift`). The desktop app checks for updates automatically and applies
signed releases. Now continue to the [quickstart](/docs/quickstart).
