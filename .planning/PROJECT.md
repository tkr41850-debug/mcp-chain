# mcp-chain

## What This Is

A lightweight MCP server (distributed as a Claude Code plugin) that lets you chain Claude Code sessions together at arbitrary fan-in/fan-out. Any session can register a lock with a resolution condition and get back a short word-ID; any number of other sessions (in different conversations) can wait on that ID until the registering session resolves it. Primary user: the author, for orchestrating multi-session Claude Code workflows at scale.

## Core Value

N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with **minimal overhead** (fast startup, low memory, terse tool prompts, small binary).

## Requirements

### Validated

(None yet — ship to validate)

### Active

**Core MCP + CLI**

- [ ] **CORE-01**: Single Go binary with subcommands: `serve` (MCP stdio server), `status <id>` (CLI poll — exit 0 resolved, 2 pending, 1 unknown), and any additional subcommands research/planning reveals as necessary
- [ ] **CORE-02**: MCP tool to register a session with a natural-language resolution condition; returns a short word-ID
- [ ] **CORE-03**: MCP tool for the registering session to mark an ID resolved; double-resolve returns a clear error ("already resolved")
- [ ] **CORE-04**: State persisted to a shared JSON file with OS file-lock (`flock`) for cross-process safety
- [ ] **CORE-05**: State file path defaults to `~/.mcp-chain/state.json`; respects `$XDG_STATE_HOME` when set (→ `$XDG_STATE_HOME/mcp-chain/state.json`)
- [ ] **CORE-06**: Word-ID generator uses the EFF short wordlist (1296 words, embedded in binary); falls back to a hex counter once exhausted
- [ ] **CORE-07**: All MCP tool descriptions and model-facing text are minimal (bare-essentials wording, no multi-sentence explanations)

**Slash commands (Claude Code plugin)**

- [ ] **CMD-01**: `/chain-reg [condition]` — registers the current session; prints the word-ID. If `[condition]` is omitted, the command prompts the user in-conversation for the condition before registering
- [ ] **CMD-02**: `/chain-wait [id] [--timeout DURATION]` — runs a bash monitor that polls `mcp-chain status <id>` every 1s and prints `continue` on resolve. `--timeout` accepts standard duration strings (`30s`, `1m`, `1h`, `1d`, `1w`); when provided, the timeout value is echoed on invocation so the user sees it in the monitor output. Without `--timeout`, wait is unbounded. Errors immediately if the ID doesn't exist
- [ ] **CMD-03**: `/chain-list` — prints a human-readable table of all sessions (ID, status, condition, timestamps)
- [ ] **CMD-04**: `/chain-purge [id | --all | --resolved]` — explicit cleanup (nothing auto-expires)

**Bash monitor helper**

- [ ] **HELPER-01**: Repo ships a small bash script that wraps `mcp-chain status <id>` in a 1-second poll loop and prints `continue` on resolve. `/chain-wait` tells Claude to run it via the monitor tool

**Quality & Testing**

- [ ] **QA-01**: Unit tests for all core logic (wordlist allocation, state file read/write under lock, ID lookup, resolve state transitions, timeout parsing)
- [ ] **QA-02**: Integration tests exercising full flows — register → status pending → resolve → status resolved; concurrent waiters on the same ID; double-resolve error; unknown-ID error; cross-process flock safety (spawn two processes, verify no state corruption)
- [ ] **QA-03**: CI runs `go test ./...` with race detector (`-race`) on every push/PR; failing tests block merges and releases

**Distribution & DX**

- [ ] **DIST-01**: Packaged as a Claude Code plugin (marketplace/plugin manifest, slash-command files) installable from the GitHub repo
- [ ] **DIST-02**: GitHub Actions CI — runs tests and `go build` on push/PR; on tag, cross-compiles release binaries (linux/macos/windows × amd64/arm64) and attaches them to the GitHub release
- [ ] **DIST-03**: Brief `README.md` covering why it exists, install steps, and usage examples

### Out of Scope

- **Auto-expiration of resolved IDs** — explicit `/chain-purge` only; keeping simple and predictable
- **Networked / multi-machine coordination** — local-filesystem only; all participating sessions must share the state file
- **Daemon / socket server** — rejected in favor of shared JSON file for simpler lifecycle (no background process to manage)
- **SQLite / embedded DB** — rejected; JSON + flock is sufficient for expected entry counts
- **Programmatic conditions** (shell expressions polled by the server) — rejected; natural-language, Claude-judged resolution is simpler and more flexible
- **License selection** — deferred; add before first release
- **Separate polling binary** — same binary exposes `status` subcommand instead
- **Granular auth/ACLs beyond the session-link question** — not needed for a single-user local tool

## Context

- **Target user:** solo developer (the author) orchestrating multiple parallel Claude Code conversations. N Claude Code processes can run simultaneously (different terminals / different working dirs) and need a shared coordination primitive — usage scales to arbitrary fan-in/fan-out (one registrant + many waiters, or many registrants each with their own waiters)
- **Why MCP:** Claude Code natively consumes MCP servers; shipping as a Claude Code plugin gives zero-config install for this audience
- **Why Go:** best fit for stated constraints — single static binary (~10MB), fast startup (<50ms), low runtime memory, cross-compiles trivially for CI release artifacts. Pure-Go deps only (no cgo) to keep cross-compile simple
- **Usage shape:** `/chain-reg` in a session returns a word-ID. The user tells any number of other sessions to run `/chain-wait <word>`, each spinning up its own bash monitor. When the registering session satisfies its condition, Claude calls the `resolve` MCP tool with the word once; every waiting monitor sees resolved state on its next poll and prints `continue`, unblocking each waiting session. The same pattern composes in any fan-in/fan-out shape
- **Open question for research/planning:** whether the MCP server can reliably distinguish "the session that registered an ID" from "some other session" via MCP per-connection identity. Preferred outcome: enforce that only the registering session can resolve. Needs investigation into MCP stdio identity guarantees before committing to a design. User is OK with the default of "any connection can resolve once" if session-link isn't reliably possible — flag this decision point during research

## Constraints

- **Tech stack:** Go (pure-Go deps only — no cgo)
- **Performance:** Startup ≤100ms; resident memory ≤20MB during normal use; binary size ≤15MB
- **Token budget:** MCP tool descriptions and any model-facing text (slash-command prompts, instruction strings) kept as terse as possible — this is a product principle, not a style note. Every tool description should be reviewable for "can this be shorter?"
- **Platform:** Linux + macOS primary; Windows supported via CI cross-compile
- **Dependencies:** Minimize. Stdlib-first. Only add a dep if it saves meaningful code
- **Distribution:** Must install via Claude Code plugin mechanism from a public GitHub repo

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over Rust/Bun/Python | Best balance of static-binary size, startup, memory, and build simplicity for stated constraints | — Pending |
| JSON + flock over SQLite/bbolt | Zero deps, human-readable, tiny; expected entry count is low | — Pending |
| Single binary with subcommands | One artifact to install; `serve` for MCP, `status` for the poll loop | — Pending |
| Natural-language condition (Claude-judged) | Flexible, matches the way the feature is used; avoids coupling server to shell semantics | — Pending |
| Keep-until-purge lifecycle | Predictable, no surprising GC; user retains control | — Pending |
| EFF short wordlist + hex fallback | Memorable short IDs, collision-free via deterministic allocation, embedded (zero runtime cost) | — Pending |
| Bash monitor polls `status` subcommand | Decouples bash from storage format; keeps monitor script trivial | — Pending |
| Session-link for resolve | TBD — investigate MCP per-connection identity during research | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-23 after initialization*
