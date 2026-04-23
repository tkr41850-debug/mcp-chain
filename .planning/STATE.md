# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-23)

**Core value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).
**Current focus:** Phase 2 — Wordlist & ID Allocator

## Current Position

Phase: 2 of 10 (Wordlist & ID Allocator)
Plan: 1 of 1 in current phase (COMPLETE)
Status: Executing
Last activity: 2026-04-23 — Phase 2 Plan 01 complete: internal/idgen with Allocate + 4 tests all green

Progress: [██░░░░░░░░] 20%

## Performance Metrics

**Velocity:**
- Total plans completed: 1
- Average duration: ~5 min
- Total execution time: ~5 min

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 02 | 1 | ~5 min | ~5 min |

**Recent Trend:**
- Last 5 plans: 02-01 (5 min)
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
Stopped at: Phase 2 Plan 01 complete — internal/idgen Allocate pure function, 4 tests green, sha256-pinned EFF wordlist embedded
Resume file: None
