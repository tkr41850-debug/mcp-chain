---
description: Register a new chain and print the word-ID
argument-hint: [condition]
---

Call `mcp-chain__register` with `condition="$ARGUMENTS"` (ask if empty). On the returned id, TaskCreate: subject `Resolve chain {id}`, description `When [condition] holds, call mcp-chain__resolve(id={id}).` Without this task you will forget.
