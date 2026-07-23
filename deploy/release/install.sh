#!/bin/sh
# trqsh CLI installer.  curl -fsSL https://trqsh.uz/install.sh | sh
# Detects OS/arch, downloads the latest release archive, installs `trqsh`.
set -eu

REPO="${TRQSH_REPO:-trqsh-uz/trqsh}"
BIN="trqsh"
INSTALL_DIR="${TRQSH_INSTALL_DIR:-/usr/local/bin}"

err() { echo "install: $*" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) err "unsupported architecture: $arch" ;;
esac
case "$os" in
  linux|darwin) ;;
  *) err "unsupported OS: $os (use the Windows installer or 'scoop install trqsh')" ;;
esac

# Resolve the latest tag via the GitHub API (no jq dependency).
tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
[ -n "${tag:-}" ] || err "could not determine latest release"
version="${tag#v}"

asset="trqsh_${version}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading ${asset}…"
curl -fsSL "$url" -o "$tmp/$asset" || err "download failed: $url"
tar -xzf "$tmp/$asset" -C "$tmp"

if [ -w "$INSTALL_DIR" ]; then
  mv "$tmp/$BIN" "$INSTALL_DIR/$BIN"
else
  echo "Elevating to install into $INSTALL_DIR (sudo)…"
  sudo mv "$tmp/$BIN" "$INSTALL_DIR/$BIN"
fi
chmod +x "$INSTALL_DIR/$BIN"

echo "Installed $("$INSTALL_DIR/$BIN" version 2>/dev/null || echo "$BIN $version") → $INSTALL_DIR/$BIN"
echo "Get started:  trqsh http 3000"
