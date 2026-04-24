# mcp-chain — Manual Dogfood Checklist (Phase 10 gate for v0.1.0)

Linux and macOS each need a green run (or a documented deferral) before tagging.

| # | Step | Done means | Linux | macOS |
|---|------|------------|-------|-------|
| 1 | Tag a snapshot tag locally: `git tag v0.1.0-rc1 && make release-snapshot` | `dist/` contains 6 archives + checksums.txt | ☐ | ☐ |
| 2 | Register marketplace + install: `/plugin marketplace add tkr41850-debug/mcp-chain` then `/plugin install mcp-chain@mcp-chain` (pins to latest pre-release after `v0.1.0-rc.1` is pushed) | Claude Code reports "installed"; `/mcp` list shows `mcp-chain` | ☐ | ☐ |
| 3 | Run a plugin command (e.g. `/mcp-chain:list`) once so the wrapper fetches the binary, then `${CLAUDE_PLUGIN_DATA}/mcp-chain-mcp-chain-marketplace/bin/mcp-chain --version` | Prints `mcp-chain 0.1.0-rc.1` (or whatever `plugin.json` pins) | ☐ | ☐ |
| 4 | Open Session A in Claude Code; run `/mcp-chain:reg build passes` | Response includes a single EFF-word ID (e.g., `otter`) | ☐ | ☐ |
| 5 | Run `/mcp-chain:list` in Session A | Table shows `otter`, status `pending`, condition `build passes` | ☐ | ☐ |
| 6 | Open Session B in a separate Claude Code conversation; run `/mcp-chain:wait otter` | Monitor starts; no output yet; stays running | ☐ | ☐ |
| 7 | In Session A, say "build passed, resolve otter" — let Claude call the MCP `resolve` tool (NOT the `--force` CLI escape hatch) | Session A reports resolved | ☐ | ☐ |
| 8 | Session B within 2 seconds prints `continue` and exits | Session B unblocked | ☐ | ☐ |
| 9 | `/mcp-chain:list` from either session | Shows `otter` status `resolved` with a `resolved_at` timestamp | ☐ | ☐ |
| 10 | `/mcp-chain:purge --resolved` | No output on success; subsequent `/mcp-chain:list` empty | ☐ | ☐ |
| 11 | `/mcp-chain:purge` with no args | Errors with `mcp-chain: purge requires <id>, --all, or --resolved` on stderr | ☐ | ☐ |
| 12 | `/mcp-chain:wait nonexistent` | Exits 1 immediately with "unknown id" on stderr | ☐ | ☐ |
| 13 | Inspect `~/.mcp-chain/state.json` OR `$XDG_STATE_HOME/mcp-chain/state.json` | File permissions `0600`, parent dir `0700`, JSON valid | ☐ | ☐ |
| 14 | Kill Claude Code mid-wait in Session B, reopen, `/mcp-chain:wait <id-still-pending>` | Waiter resumes cleanly | ☐ | ☐ |
| 15 | (Optional) Reload plugin: `/mcp` list → select `mcp-chain` → restart | Plugin reconnects without error | ☐ | ☐ |

**macOS deferral option:** If no macOS machine available, write "macOS: deferred pending runner access" under the macOS column header and annotate the phase verifier accordingly. Treats as `human_needed`, not a blocker per D-10.

**Pre-tag sign-off:** Both columns complete (or macOS explicitly deferred). Initial and date.
