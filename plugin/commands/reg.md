---
description: Register a new chain and print the word-ID
argument-hint: [condition]
---

Call `mcp-chain__register` with `condition="$ARGUMENTS"` (if empty, ask first). After it returns the id, TaskCreate: subject `Resolve chain {id}`, description `When [condition] holds, call mcp-chain__resolve(id={id}).` This task is the only resolve reminder — without it, the obligation is forgotten.
