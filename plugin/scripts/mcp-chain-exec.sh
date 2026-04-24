#!/usr/bin/env bash
# mcp-chain-exec.sh - Wrapper that ensures the release binary is installed
# into ${CLAUDE_PLUGIN_DATA}/bin before exec'ing it with the caller's args.
#
# Used by plugin/.mcp.json and every slash command so the plugin is
# self-healing: no separate SessionStart hook, no two-restart install UX.

set -eu

# Self-derive so the wrapper works whether Claude Code set the envs
# (MCP spawn / hooks) or not (LLM-invoked via Bash tool for a slash command).
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

PLUGIN_DATA="${CLAUDE_PLUGIN_DATA:-}"
if [ -z "$PLUGIN_DATA" ]; then
  case "$PLUGIN_ROOT" in
    */plugins/cache/*/*/*)
      D1="${PLUGIN_ROOT%/*}"; D2="${D1%/*}"; D3="${D2%/*}"; D4="${D3%/*}"
      PLUGIN_DATA="$D4/data/${D1##*/}-${D2##*/}"
      ;;
    *) PLUGIN_DATA="$PLUGIN_ROOT" ;;
  esac
fi

CLAUDE_PLUGIN_ROOT="$PLUGIN_ROOT" CLAUDE_PLUGIN_DATA="$PLUGIN_DATA" \
  "$PLUGIN_ROOT/scripts/install-binary.sh"

exec "$PLUGIN_DATA/bin/mcp-chain" "$@"
