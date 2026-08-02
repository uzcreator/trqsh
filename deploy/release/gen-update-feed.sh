#!/usr/bin/env bash
# Emit a JSON version feed (version, notes, url). DORMANT: the old Wails GUI that
# polled this feed was removed; the Tauri desktop app self-updates via the bundled
# agent. Kept for potential future use. Usage: gen-update-feed.sh v0.1.0
set -euo pipefail

tag="${1:?usage: gen-update-feed.sh <tag>}"
version="${tag#v}"
repo="${GITHUB_REPOSITORY:-uzcreator/trqsh}"
url="https://github.com/${repo}/releases/tag/${tag}"
notes="trqsh ${version}. See the release notes for changes."

cat <<JSON
{
  "version": "${version}",
  "notes": "${notes}",
  "url": "${url}"
}
JSON
