#!/bin/sh
# trqsh installer — detects your OS/arch, downloads the signed release binary,
# verifies its checksum, and installs it. Serve this at https://trqsh.uz/install.sh:
#
#   curl -fsSL https://trqsh.uz/install.sh | sh
#
# Overrides: TRQSH_VERSION, TRQSH_REPO, TRQSH_BINDIR.
set -eu

# CLI releases publish to their own dedicated repo (source stays in
# uzcreator/trqsh; see .goreleaser.yaml).
REPO="${TRQSH_REPO:-uzcreator/trqshcli}"
BINDIR="${TRQSH_BINDIR:-}"
# Fallback only if the version can't be resolved dynamically below (offline,
# API rate-limited, etc.) — bump occasionally, but going stale here just
# degrades the fallback, since the common path always resolves the real
# latest release.
FALLBACK_VERSION="0.1.5"

say() { printf 'trqsh: %s\n' "$1" >&2; }
die() { printf 'trqsh: error: %s\n' "$1" >&2; exit 1; }

resolve_latest_version() {
  # CLI (v*) and desktop GUI (desktop-v*) releases share this repo and
  # interleave by date, so GitHub's /releases/latest can't be used directly —
  # it can return a desktop build. List releases and take the newest v* tag
  # instead. Drafts are already excluded by the public API; prereleases
  # aren't filtered here (this project doesn't currently tag any) since a
  # full JSON parse without jq isn't worth it for a shell installer.
  url="https://api.github.com/repos/${REPO}/releases?per_page=30"
  if command -v curl >/dev/null 2>&1; then
    body=$(curl -fsSL "$url" 2>/dev/null) || return 0
  elif command -v wget >/dev/null 2>&1; then
    body=$(wget -qO- "$url" 2>/dev/null) || return 0
  else
    return 0
  fi
  printf '%s' "$body" \
    | grep -o '"tag_name": *"v[0-9][^"]*"' \
    | sed -E 's/.*"v([0-9][^"]*)"/\1/' \
    | head -n1
}

VERSION="${TRQSH_VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(resolve_latest_version)
  if [ -z "$VERSION" ]; then
    say "could not resolve the latest version (offline or API rate-limited?) — using ${FALLBACK_VERSION}"
    VERSION="$FALLBACK_VERSION"
  fi
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux) goos=linux ;;
  darwin) goos=darwin ;;
  *) die "unsupported OS '$os' — see https://github.com/$REPO/releases" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) goarch=amd64 ;;
  arm64 | aarch64) goarch=arm64 ;;
  *) die "unsupported architecture '$arch'" ;;
esac

archive="trqsh_${VERSION}_${goos}_${goarch}.tar.gz"
base="https://github.com/${REPO}/releases/download/v${VERSION}"

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t trqsh)
trap 'rm -rf "$tmp"' EXIT INT TERM

download() {
  # download <url> <dest>
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    die "need curl or wget to download"
  fi
}

say "downloading ${archive} (v${VERSION})..."
download "${base}/${archive}" "${tmp}/${archive}"

# Best-effort checksum verification.
if [ "${TRQSH_SKIP_CHECKSUM:-0}" != "1" ] && command -v sha256sum >/dev/null 2>&1; then
  if download "${base}/checksums.txt" "${tmp}/checksums.txt" 2>/dev/null; then
    want=$(grep " ${archive}\$" "${tmp}/checksums.txt" 2>/dev/null | awk '{print $1}' || true)
    if [ -n "${want:-}" ]; then
      got=$(sha256sum "${tmp}/${archive}" | awk '{print $1}')
      [ "$want" = "$got" ] || die "checksum mismatch for ${archive}"
      say "checksum verified"
    fi
  fi
fi

tar -xzf "${tmp}/${archive}" -C "${tmp}"
[ -f "${tmp}/trqsh" ] || die "trqsh binary missing from archive"

# Choose an install dir on PATH that we can write to.
if [ -z "$BINDIR" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then
    BINDIR=/usr/local/bin
  else
    BINDIR="${HOME}/.local/bin"
  fi
fi
mkdir -p "$BINDIR"

if install -m 0755 "${tmp}/trqsh" "${BINDIR}/trqsh" 2>/dev/null; then :; else
  cp "${tmp}/trqsh" "${BINDIR}/trqsh"
  chmod 0755 "${BINDIR}/trqsh"
fi

say "installed to ${BINDIR}/trqsh"
case ":${PATH}:" in
  *":${BINDIR}:"*) ;;
  *) say "add ${BINDIR} to your PATH to run 'trqsh' from anywhere" ;;
esac
say "run 'trqsh http 3000' to expose localhost:3000"
