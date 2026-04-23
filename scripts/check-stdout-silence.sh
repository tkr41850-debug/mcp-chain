#!/usr/bin/env bash
# scripts/check-stdout-silence.sh — assert `mcp-chain serve </dev/null` writes 0 bytes to stdout.
# This is the Phase 1 validator for MCP-02 (stdout discipline).
#
# Why this works even though `serve` isn't implemented yet:
#   - Phase 1 stub exits immediately with code 3 ("not implemented") after writing
#     ONLY to stderr (via fmt.Fprintln(os.Stderr, ...)).
#   - The test captures stdout into a tmp file and asserts the file is zero bytes.
#   - If someone in Phase 5 accidentally adds a fmt.Println, this gate catches the
#     regression — the gate carries forward unchanged.
set -euo pipefail

BIN="${1:?usage: check-stdout-silence.sh <binary>}"

if [[ ! -f "$BIN" ]]; then
  echo "ERROR: binary not found: $BIN" >&2
  exit 2
fi

STDOUT=$(mktemp)
STDERR=$(mktemp)
trap 'rm -f "$STDOUT" "$STDERR"' EXIT

# serve stub exits 3; we tolerate that exit code and check only stdout contents.
"$BIN" serve </dev/null >"$STDOUT" 2>"$STDERR" || true

STDOUT_BYTES=$(wc -c < "$STDOUT")

printf 'stdout bytes: %d (expect: 0)\n' "$STDOUT_BYTES"

if (( STDOUT_BYTES > 0 )); then
  echo "FAIL: serve wrote $STDOUT_BYTES bytes to stdout — MCP-02 violation." >&2
  echo "--- stdout contents ---" >&2
  cat "$STDOUT" >&2
  exit 1
fi

echo "OK: stdout is silent."
