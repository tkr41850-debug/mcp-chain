#!/usr/bin/env bash
# dogfood.sh — end-to-end smoke test for mcp-chain CLI flow.
# build → status unknown (1) → seed pending → status pending (2) → resolve → status resolved (0)
#      → list nonempty → purge --all → list empty
# Requires: bash 3.2+, go in PATH, mktemp.

set -eu

# -------- PATH guard (matches D-11 pattern) ---------------------------------
if ! command -v go >/dev/null 2>&1; then
  echo "dogfood: go not found on PATH" >&2
  exit 127
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
BIN="$WORK/mcp-chain"
export XDG_STATE_HOME="$WORK/state"
mkdir -p "$XDG_STATE_HOME"

cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

echo "dogfood: building binary" >&2
go build -o "$BIN" "$ROOT/cmd/mcp-chain"

# ----- step 1: unknown id → exit 1 ------------------------------------------
set +e
"$BIN" status no-such-id >/dev/null 2>&1
CODE=$?
set -e
[ "$CODE" -eq 1 ] || { echo "dogfood: step 1 FAIL unknown-id exit=$CODE want 1" >&2; exit 1; }
echo "dogfood: step 1 OK (unknown exit 1)" >&2

# ----- step 2: seed pending entry → status exit 2 ---------------------------
ID="$(go run "$ROOT/scripts/internal/seed_pending.go")"
[ -n "$ID" ] || { echo "dogfood: step 2 FAIL empty id from seed" >&2; exit 1; }
set +e
"$BIN" status "$ID" >/dev/null 2>&1
CODE=$?
set -e
[ "$CODE" -eq 2 ] || { echo "dogfood: step 2 FAIL pending exit=$CODE want 2" >&2; exit 1; }
echo "dogfood: step 2 OK (pending exit 2 for id=$ID)" >&2

# ----- step 3: resolve --force → status exit 0 ------------------------------
"$BIN" resolve "$ID" --force >/dev/null
set +e
"$BIN" status "$ID" >/dev/null 2>&1
CODE=$?
set -e
[ "$CODE" -eq 0 ] || { echo "dogfood: step 3 FAIL resolved exit=$CODE want 0" >&2; exit 1; }
echo "dogfood: step 3 OK (resolved exit 0)" >&2

# ----- step 4: list shows the entry -----------------------------------------
OUT="$("$BIN" list)"
echo "$OUT" | grep -q "$ID" || { echo "dogfood: step 4 FAIL list missing $ID" >&2; exit 1; }
echo "dogfood: step 4 OK (list shows entry)" >&2

# ----- step 5: purge --all → list empty -------------------------------------
"$BIN" purge --all >/dev/null
OUT="$("$BIN" list)"
# "empty" means list prints 0 data rows; accept either no output or header-only.
DATA_LINES="$(echo "$OUT" | grep -cE '^[a-z]+[[:space:]]+' || true)"
[ "$DATA_LINES" -eq 0 ] || { echo "dogfood: step 5 FAIL list nonempty after purge" >&2; exit 1; }
echo "dogfood: step 5 OK (list empty after purge)" >&2

echo "dogfood: all steps passed" >&2
