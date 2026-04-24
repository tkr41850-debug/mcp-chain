#!/usr/bin/env bash
# mcp-chain-exec.sh - Wrapper that ensures the release binary is installed
# into ${CLAUDE_PLUGIN_DATA}/bin before exec'ing it with the caller's args.
#
# Used by plugin/.mcp.json and every slash command so the plugin is
# self-healing: no separate SessionStart hook, no two-restart install UX.

set -eu

"${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh"
exec "${CLAUDE_PLUGIN_DATA}/bin/mcp-chain" "$@"
