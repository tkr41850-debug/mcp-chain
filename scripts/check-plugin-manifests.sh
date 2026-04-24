#!/bin/sh
# check-plugin-manifests.sh — validate Phase 8 JSON manifests (SC-1 gate).
# Runs: jq empty on all three files + literal-substring sanity checks.
# Requires: jq (degrades gracefully if absent).
set -eu

FAIL=0

check_jq() {
  file="$1"
  if [ ! -f "$file" ]; then
    echo "FAIL: $file missing" >&2
    FAIL=1
    return
  fi
  if command -v jq >/dev/null 2>&1; then
    if ! jq empty < "$file" >/dev/null 2>&1; then
      echo "FAIL: $file is not valid JSON" >&2
      FAIL=1
      return
    fi
  fi
  echo "OK: $file"
}

check_jq plugin/.claude-plugin/plugin.json
check_jq plugin/.mcp.json
check_jq .claude-plugin/marketplace.json

# LD-2 reject-list on .mcp.json
for pattern in '/Users/' '/home/' 'C:\\' '"npm"' '"uvx"' '"node"' '"python"'; do
  if grep -q -F "$pattern" plugin/.mcp.json 2>/dev/null; then
    echo "FAIL: plugin/.mcp.json contains forbidden literal: $pattern" >&2
    FAIL=1
  fi
done

# Presence check on ${CLAUDE_PLUGIN_ROOT}
if ! grep -q -F '${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain' plugin/.mcp.json; then
  echo "FAIL: plugin/.mcp.json missing \${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain" >&2
  FAIL=1
fi

# WR-01 regression guard: reject OWNER placeholder in any manifest.
for f in plugin/.claude-plugin/plugin.json plugin/.mcp.json .claude-plugin/marketplace.json; do
  if grep -q -F 'OWNER' "$f" 2>/dev/null; then
    echo "FAIL: $f still contains OWNER placeholder" >&2
    FAIL=1
  fi
done

if [ "$FAIL" -eq 0 ]; then
  echo "All manifest checks passed."
fi
exit "$FAIL"
