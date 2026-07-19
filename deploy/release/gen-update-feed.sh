#!/usr/bin/env bash
# Emit the auto-update feed the GUI/CLI poll. Shape matches gui/update.go
# (releaseFeed: version, notes, url). Usage: gen-update-feed.sh v0.1.0
set -euo pipefail

tag="${1:?usage: gen-update-feed.sh <tag>}"
version="${tag#v}"
repo="${GITHUB_REPOSITORY:-rift/rift}"
url="https://github.com/${repo}/releases/tag/${tag}"
notes="Rift ${version}. See the release notes for changes."

cat <<JSON
{
  "version": "${version}",
  "notes": "${notes}",
  "url": "${url}"
}
JSON
