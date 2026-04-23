#!/usr/bin/env bash
# scripts/check-size.sh — fail if stripped binary exceeds 15 MB.
# Usage: check-size.sh <path-to-binary>
set -euo pipefail

BIN="${1:?usage: check-size.sh <binary>}"
MAX_BYTES=15728640  # 15 * 1024 * 1024

if [[ ! -f "$BIN" ]]; then
  echo "ERROR: binary not found: $BIN" >&2
  exit 2
fi

SIZE=$(wc -c < "$BIN")

printf 'binary: %s\n' "$BIN"
printf 'size:   %d bytes (%.2f MB)\n' "$SIZE" "$(awk "BEGIN {print $SIZE/1048576}")"
printf 'limit:  %d bytes (15 MB)\n' "$MAX_BYTES"

if (( SIZE > MAX_BYTES )); then
  echo "FAIL: binary exceeds 15 MB budget (DIST-03)." >&2
  exit 1
fi

echo "OK: within budget."
