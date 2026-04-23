#!/usr/bin/env bash
# scripts/check-startup.sh — fail if `binary --version` startup exceeds 100 ms P95 of 5 runs.
# Uses bash 5's $EPOCHREALTIME (microsecond-precision wall clock).
# Usage: check-startup.sh <path-to-binary>
#
# Measurement caveats (PITFALLS.md #4):
# - CI runners are warm-cache after first invocation. The P95 of 5 is a regression
#   gate, not a cold-cache simulation. A 10ms→110ms regression will still fail.
# - If flakes appear, swap to `hyperfine --warmup 1 --runs 20 --max-runs 20` and
#   parse its JSON output. For Phase 1, keep it dependency-free.
set -euo pipefail

BIN="${1:?usage: check-startup.sh <binary>}"
MAX_MS=100
RUNS=5

if [[ ! -f "$BIN" ]]; then
  echo "ERROR: binary not found: $BIN" >&2
  exit 2
fi

# bash 5 $EPOCHREALTIME produces "SECONDS.MICROSECONDS" e.g. "1714068000.123456"
if [[ -z "${EPOCHREALTIME:-}" ]]; then
  echo "ERROR: bash 5.0+ required (no \$EPOCHREALTIME). Install bash 5 or use hyperfine." >&2
  exit 2
fi

# Warm-up run — populate OS page cache so measurements aren't dominated by
# first-invocation I/O. Discarded from the sample.
"$BIN" --version >/dev/null 2>&1 || true

declare -a TIMES_MS
echo "measuring ${BIN} --version (${RUNS} runs, 1 warm-up discarded):"
for ((i=1; i<=RUNS; i++)); do
  START="$EPOCHREALTIME"
  "$BIN" --version >/dev/null 2>&1
  END="$EPOCHREALTIME"
  # Multiply (END - START) by 1000 to get ms. awk handles the float math.
  MS=$(awk "BEGIN { printf \"%.3f\", ($END - $START) * 1000 }")
  TIMES_MS+=("$MS")
  printf '  run %d: %s ms\n' "$i" "$MS"
done

# Compute max — with only 5 runs, max IS the P95. For larger N use a sort+index.
MAX=$(printf '%s\n' "${TIMES_MS[@]}" | sort -g | tail -1)

printf 'max (P95 of %d): %s ms\n' "$RUNS" "$MAX"
printf 'limit:           %d ms\n' "$MAX_MS"

# Integer comparison via awk (bash can't compare floats).
if awk "BEGIN { exit !($MAX > $MAX_MS) }"; then
  echo "FAIL: startup exceeds ${MAX_MS}ms budget (DIST-03)." >&2
  exit 1
fi

echo "OK: within budget."
