---
description: Block until chain resolves, then continue with the user's follow-up
argument-hint: <id> [--timeout DURATION]
---

Start Monitor (`persistent: true`) on `MCP_CHAIN_BIN="${CLAUDE_PLUGIN_ROOT}/scripts/mcp-chain-exec.sh" bash "${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh" $ARGUMENTS`, description `gate $ARGUMENTS`. TaskCreate: subject `Run gated work after chain $ARGUMENTS resolves`, description = the follow-up the user requested alongside this command. Hard gate — do nothing else in this conversation until `continue` arrives, then run the task.
