# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-23)

**Core value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).
**Current focus:** Phase 2 — Wordlist & ID Allocator

## Current Position

Phase: 3 of 10 (XDG Path Resolution)
Plan: 1 of 1 in current phase (COMPLETE)
Status: Executing
Last activity: 2026-04-23 — Phase 3 Plan 01 complete: internal/statepath with Resolve() + 6 tests all green

Progress: [███░░░░░░░] 30%

## Performance Metrics

**Velocity:**
- Total plans completed: 2
- Average duration: ~5 min
- Total execution time: ~9 min

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 02 | 1 | ~5 min | ~5 min |
| 03 | 1 | ~4 min | ~4 min |

**Recent Trend:**
- Last 5 plans: 02-01 (5 min), 03-01 (4 min)
- Trend: —

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work (from research synthesis 2026-04-23):

- Stack pinned: `modelcontextprotocol/go-sdk` v1.5+, `alecthomas/kong` v1.15+, `gofrs/flock` v0.13+, `google/renameio/v2` v2.0.2, GoReleaser v2.15+ — 5 direct pure-Go deps
- Hexagonal split: `internal/store` (core) + `internal/mcpserver` + `internal/cli` adapters; MCP SDK choice localized to Phase 5
- Session-link resolved: per-process 128-bit `OwnerToken` at `serve` startup, stamped on register, checked on resolve; CLI `--force` bypass as escape hatch (MCP stdio has no SessionID)
- State schema: versioned JSON, monotonic counter never decremented on purge, `LOCK_SH` for reads, `LOCK_EX` for mutations, separate `state.json.lock` on Windows
- License deferred — no phase allocated
- statepath: fallback is `~/.mcp-chain/` (dot-dir) not strict-XDG `~/.local/state/` — aligned with PROJECT.md/REQUIREMENTS.md CORE-06; XDG-aware users set `$XDG_STATE_HOME`
- statepath: os.Getenv("HOME") directly (not os.UserHomeDir) — project policy for NSS-free startup path; no os/user in dep graph confirmed via go list -deps

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-04-23
Stopped at: Phase 3 Plan 01 complete — internal/statepath Resolve() with XDG+HOME fallback, 6 tests green, os/user absent from dep graph
Resume file: None
