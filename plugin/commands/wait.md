---
description: Wait for a chain-ID to resolve (polls every 1s)
argument-hint: <id> [--timeout DURATION]
---

Use Monitor with `persistent: true` to run `MCP_CHAIN_BIN="${CLAUDE_PLUGIN_ROOT}/scripts/mcp-chain-exec.sh" bash "${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh" $ARGUMENTS`, description `chain $ARGUMENTS`. Hard gate: do NOT begin any subsequent work in this conversation until `continue` arrives.
