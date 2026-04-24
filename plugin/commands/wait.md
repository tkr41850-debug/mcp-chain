---
description: Wait for a chain-ID to resolve (polls every 1s)
argument-hint: <id> [--timeout DURATION]
---

Run: `MCP_CHAIN_BIN="${CLAUDE_PLUGIN_ROOT}/scripts/mcp-chain-exec.sh" bash "${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh" $ARGUMENTS` and echo the output verbatim. On `continue`, treat resolved. For long waits, use the Monitor tool (Bash caps at 10m).
