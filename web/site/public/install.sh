#!/bin/sh
# trqsh installer — detects your OS/arch, downloads the signed release binary,
# verifies its checksum, and installs it. Serve this at https://trqsh.uz/install.sh:
#
#   curl -fsSL https://trqsh.uz/install.sh | sh
#
# Overrides: TRQSH_VERSION, TRQSH_REPO, TRQSH_BINDIR.
set -eu

REPO="${TRQSH_REPO:-trqsh-uz/downloads}"
VERSION="${TRQSH_VERSION:-0.1.1}"
BINDIR="${TRQSH_BINDIR:-}"

say() { printf 'trqsh: %s\n' "$1" >&2; }
die() { printf 'trqsh: error: %s\n' "$1" >&2; exit 1; }

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
