#!/usr/bin/env sh
# Generic vegeta wrapper: attack a targets file and print a report + latency
# histogram. Used for the control-plane / web HTTP load (see *.targets.example).
#
#   ./run.sh <targets-file> [rate/s] [duration]
#   ./run.sh controlplane.targets 200 30s
set -eu

TARGETS="${1:?usage: run.sh <targets-file> [rate/s] [duration]}"
RATE="${2:-50}"
DURATION="${3:-30s}"

if ! command -v vegeta >/dev/null 2>&1; then
	echo "vegeta not found — install it: https://github.com/tsenart/vegeta" >&2
	exit 1
fi

BIN="$(mktemp)"
trap 'rm -f "$BIN"' EXIT

echo "attacking targets in '$TARGETS' at ${RATE}/s for ${DURATION}"
vegeta attack -targets="$TARGETS" -rate="$RATE" -duration="$DURATION" >"$BIN"
vegeta report "$BIN"
echo "--- latency histogram ---"
vegeta report -type='hist[0,10ms,25ms,50ms,100ms,250ms,500ms,1s]' "$BIN"
