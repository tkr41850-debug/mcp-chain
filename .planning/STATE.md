# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-23)

**Core value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).
**Current focus:** Phase 7 — CLI Formatters (list, purge, resolve)

## Current Position

Phase: 6 of 10 (CLI Dispatch & Status Subcommand) — COMPLETE
Plan: 1 of 1 in current phase (COMPLETE)
Status: Ready to advance to Phase 7
Last activity: 2026-04-24 — Phase 6 Plan 01 complete: runStatus exit-code decision tree (0/2/1) + kong.Writers stderr routing + manual --version pre-parse; 12 tests added (5 unit + 7 integration); CORE-01 closed

Progress: [██████░░░░] 60%

## Performance Metrics

**Velocity:**
- Total plans completed: 5
- Average duration: ~12 min
- Total execution time: ~61 min

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 02 | 1 | ~5 min | ~5 min |
| 03 | 1 | ~4 min | ~4 min |
| 04 | 1 | ~25 min | ~25 min |
| 05 | 1 | (unrecorded) | — |
| 06 | 1 | ~20 min | ~20 min |

**Recent Trend:**
- Last 5 plans: 02-01 (5 min), 03-01 (4 min), 04-01 (25 min), 05-01 (—), 06-01 (20 min)
- Trend: ↑ (larger phases with integration suites)

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work (from research synthesis 2026-04-23):

- Stack pinned: `modelcontextprotocol/go-sdk` v1.5+, `alecthomas/kong` v1.15+, `gofrs/flock` **v0.12.1** (v0.13 requires Go 1.24), `google/renameio/v2` **v2.0.1** (v2.0.2 requires Go 1.25), GoReleaser v2.15+ — 5 direct pure-Go deps
- store: sibling `state.json.lock` on BOTH platforms (not windows-only) — uniform semantics, avoids POSIX rename-changes-inode race
- store: crypto/subtle.ConstantTimeCompare for OwnerToken (32-char hex; timing-attack resistance)
- store: saveStateFn function-pointer injection via export_test.go (no env-var hooks, no production footprint)
- Hexagonal split: `internal/store` (core) + `internal/mcpserver` + `internal/cli` adapters; MCP SDK choice localized to Phase 5
- Session-link resolved: per-process 128-bit `OwnerToken` at `serve` startup, stamped on register, checked on resolve; CLI `--force` bypass as escape hatch (MCP stdio has no SessionID)
- State schema: versioned JSON, monotonic counter never decremented on purge, `LOCK_SH` for reads, `LOCK_EX` for mutations, separate `state.json.lock` on Windows
- License deferred — no phase allocated
- statepath: fallback is `~/.mcp-chain/` (dot-dir) not strict-XDG `~/.local/state/` — aligned with PROJECT.md/REQUIREMENTS.md CORE-06; XDG-aware users set `$XDG_STATE_HOME`
- statepath: os.Getenv("HOME") directly (not os.UserHomeDir) — project policy for NSS-free startup path; no os/user in dep graph confirmed via go list -deps
- cli/status (Phase 6): exit-code contract locked 0/2/1 (NOT 0/1/2); `runStatus(out, errW io.Writer, path, id string) int` pure over writers + path (no env reads); `StatusCmd.Run` uses `os.Exit(code)` not kong `ExitCoder` so pending row keeps stderr empty
- cli/main (Phase 6): `kong.Writers(os.Stderr, os.Stderr)` + manual `--version` pre-parse replaces `kong.VersionFlag` (which hard-codes `app.Stdout`, incompatible with the Writers redirect); `--version` remains the ONE sanctioned stdout write in the CLI surface
- cli/status (Phase 6, LD-6): `ErrSchemaVersion` / `ErrCorruptJSON` fold into generic exit 1 — no distinct exit code for schema mismatch

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-04-24
Stopped at: Phase 6 Plan 01 complete — runStatus exit-code decision tree (0/2/1) wired under kong; 5 unit tests + 7 integration tests green under -race; 10-concurrent timing observed 525ms (<1s LD-7 threshold); CORE-01 closed; stripped binary 7.4 MB
Resume file: None — ready for Phase 7 (list/purge/resolve CLI formatters)
