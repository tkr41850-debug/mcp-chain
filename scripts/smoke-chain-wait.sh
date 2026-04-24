#!/usr/bin/env bash
# smoke-chain-wait.sh — end-to-end smoke test for plugin/scripts/chain-wait.sh.
#
# Builds a throwaway mcp-chain binary, seeds an isolated XDG_STATE_HOME,
# exercises three paths of the monitor: resolve -> "continue", unknown id
# -> exit 1, timeout -> exit 124. Cleans up via trap.
#
# Requires: bash 3.2+, go in PATH, sed, date, mktemp.

set -eu

SMOKE_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MONITOR="$SMOKE_ROOT/plugin/scripts/chain-wait.sh"
SEED="$SMOKE_ROOT/scripts/internal/seed_pending.go"

WORK="$(mktemp -d)"
BIN="$WORK/mcp-chain"
STATE_ROOT="$WORK/state"
mkdir -p "$STATE_ROOT"

cleanup() {
  rm -rf "$WORK"
}
trap cleanup EXIT

export XDG_STATE_HOME="$STATE_ROOT"
export MCP_CHAIN_BIN="$BIN"

echo "smoke: building binary at $BIN" >&2
go build -o "$BIN" "$SMOKE_ROOT/cmd/mcp-chain"

# ----- case 1: resolve path -> exit 0 + stdout "continue" --------------------
echo "smoke: case 1 -- register, resolve, expect 'continue'" >&2
ID="$(go run "$SEED")"
if [ -z "$ID" ]; then
  echo "smoke: case 1 FAIL -- empty id from seed" >&2
  exit 1
fi

MONITOR_OUT="$WORK/monitor1.out"
MONITOR_ERR="$WORK/monitor1.err"
"$MONITOR" "$ID" --timeout 10s >"$MONITOR_OUT" 2>"$MONITOR_ERR" &
MONITOR_PID=$!

# Give the monitor one poll cycle, then resolve via CLI (--force bypasses OwnerToken).
sleep 2
"$BIN" resolve "$ID" --force >/dev/null

set +e
wait "$MONITOR_PID"
CODE=$?
set -e
if [ "$CODE" -ne 0 ]; then
  echo "smoke: case 1 FAIL -- monitor exit $CODE (expected 0)" >&2
  sed -e 's/^/  err: /' "$MONITOR_ERR" >&2
  exit 1
fi
if ! grep -q '^continue$' "$MONITOR_OUT"; then
  echo "smoke: case 1 FAIL -- stdout missing 'continue'" >&2
  sed -e 's/^/  out: /' "$MONITOR_OUT" >&2
  exit 1
fi
echo "smoke: case 1 OK" >&2

# Clean up so case 2/3 start from empty state.
"$BIN" purge --all >/dev/null

# ----- case 2: unknown id -> exit 1 -----------------------------------------
echo "smoke: case 2 -- unknown id, expect exit 1" >&2
set +e
"$MONITOR" no-such-id --timeout 5s >"$WORK/monitor2.out" 2>"$WORK/monitor2.err"
CODE=$?
set -e
if [ "$CODE" -ne 1 ]; then
  echo "smoke: case 2 FAIL -- monitor exit $CODE (expected 1)" >&2
  sed -e 's/^/  err: /' "$WORK/monitor2.err" >&2
  exit 1
fi
echo "smoke: case 2 OK" >&2

# ----- case 3: timeout -> exit 124 ------------------------------------------
echo "smoke: case 3 -- never resolve, expect exit 124 after 2s" >&2
ID="$(go run "$SEED")"
set +e
"$MONITOR" "$ID" --timeout 2s >"$WORK/monitor3.out" 2>"$WORK/monitor3.err"
CODE=$?
set -e
if [ "$CODE" -ne 124 ]; then
  echo "smoke: case 3 FAIL -- monitor exit $CODE (expected 124)" >&2
  sed -e 's/^/  err: /' "$WORK/monitor3.err" >&2
  exit 1
fi
echo "smoke: case 3 OK" >&2

echo "smoke: all cases passed" >&2
