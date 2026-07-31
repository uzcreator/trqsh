#!/usr/bin/env sh
# Drive HTTP request traffic THROUGH live tunnels — point it at the URL list that
# `tunnelload -urls-out <file>` produced (while tunnelload is still holding those
# tunnels open). Measures the full public path: edge ingress -> agent -> local svc.
#
#   ./tunnels.sh <urls-file> [rate/s] [duration]
set -eu

URLS="${1:?usage: tunnels.sh <urls-file from tunnelload -urls-out> [rate/s] [duration]}"
RATE="${2:-100}"
DURATION="${3:-30s}"

if ! command -v vegeta >/dev/null 2>&1; then
	echo "vegeta not found — install it: https://github.com/tsenart/vegeta" >&2
	exit 1
fi

# One URL per line -> vegeta "GET <url>" targets. vegeta rotates through them.
TGT="$(mktemp)"
trap 'rm -f "$TGT"' EXIT
while IFS= read -r u; do
	[ -n "$u" ] && printf 'GET %s\n' "$u"
done <"$URLS" >"$TGT"

COUNT="$(grep -c . "$TGT" || true)"
echo "attacking $COUNT tunnel URLs at ${RATE}/s for ${DURATION}"
vegeta attack -targets="$TGT" -rate="$RATE" -duration="$DURATION" | vegeta report
