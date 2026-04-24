#!/usr/bin/env bash
# chain-wait.sh - Poll mcp-chain status until resolved, error, or timeout.
# Usage: chain-wait.sh <id> [--timeout DURATION]
# Exit codes:
#   0   resolved (printed "continue" to stdout)
#   1   unknown id / mid-wait error
#   124 timeout

set -eu

# -------- binary resolution (LD-9) ------------------------------------------
BIN="${MCP_CHAIN_BIN:-mcp-chain}"

# -------- argument parsing (POSIX, bash 3.2 safe) ---------------------------
ID=""
DURATION=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --timeout)
      shift
      if [ "$#" -eq 0 ]; then
        echo "mcp-chain: --timeout requires an argument" >&2
        exit 1
      fi
      DURATION="$1"
      shift
      ;;
    --timeout=*)
      DURATION="${1#--timeout=}"
      shift
      ;;
    --help|-h)
      echo "usage: chain-wait.sh <id> [--timeout DURATION]" >&2
      exit 0
      ;;
    --*)
      echo "mcp-chain: unknown flag: $1" >&2
      exit 1
      ;;
    *)
      if [ -z "$ID" ]; then
        ID="$1"
      else
        echo "mcp-chain: unexpected argument: $1" >&2
        exit 1
      fi
      shift
      ;;
  esac
done

if [ -z "$ID" ]; then
  echo "mcp-chain: chain-wait.sh requires an id" >&2
  exit 1
fi

# -------- duration parser (LD-6) --------------------------------------------
# Accepts exactly: 30s / 1m / 1h / 24h / 168h. Integer + single suffix.
# Rejects: 1h30m (combined), 1.5h (decimal), 1d (day), 1w (week).
BUDGET=0
if [ -n "$DURATION" ]; then
  case "$DURATION" in
    *s)
      n="${DURATION%s}"
      case "$n" in ''|*[!0-9]*) echo "mcp-chain: bad duration: $DURATION" >&2; exit 1 ;; esac
      BUDGET="$n"
      ;;
    *m)
      n="${DURATION%m}"
      case "$n" in ''|*[!0-9]*) echo "mcp-chain: bad duration: $DURATION" >&2; exit 1 ;; esac
      BUDGET=$(( n * 60 ))
      ;;
    *h)
      n="${DURATION%h}"
      case "$n" in ''|*[!0-9]*) echo "mcp-chain: bad duration: $DURATION" >&2; exit 1 ;; esac
      BUDGET=$(( n * 3600 ))
      ;;
    *)
      echo "mcp-chain: bad duration: $DURATION (use 30s|1m|1h|24h|168h)" >&2
      exit 1
      ;;
  esac
  if [ "$BUDGET" -gt 604800 ]; then
    echo "mcp-chain: duration exceeds max 168h" >&2
    exit 1
  fi
  if [ "$BUDGET" -le 0 ]; then
    echo "mcp-chain: duration must be positive: $DURATION" >&2
    exit 1
  fi
fi

# -------- invocation echo (LD-8) --------------------------------------------
if [ "$BUDGET" -gt 0 ]; then
  echo "mcp-chain: waiting for $ID (timeout: $DURATION)" >&2
else
  echo "mcp-chain: waiting for $ID" >&2
fi

# -------- poll loop (LD-5, LD-7) --------------------------------------------
START=$(date +%s)
while :; do
  set +e
  "$BIN" status "$ID" >/dev/null 2>&1
  CODE=$?
  set -e
  case "$CODE" in
    0)
      echo "continue"
      exit 0
      ;;
    2)
      # pending - keep polling
      ;;
    *)
      # LD-5: any other code (1, 127, 126, etc.) -> error
      # Re-run once to capture stderr for the user.
      set +e
      STDERR="$( "$BIN" status "$ID" 2>&1 1>/dev/null )"
      set -e
      if [ -n "$STDERR" ]; then
        echo "$STDERR" >&2
      else
        echo "mcp-chain: status error (exit $CODE)" >&2
      fi
      exit 1
      ;;
  esac
  if [ "$BUDGET" -gt 0 ]; then
    NOW=$(date +%s)
    ELAPSED=$(( NOW - START ))
    if [ "$ELAPSED" -ge "$BUDGET" ]; then
      echo "mcp-chain: timeout waiting for $ID after ${DURATION}" >&2
      exit 124
    fi
  fi
  sleep 1
done
