# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-23)

**Core value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).
**Current focus:** Phase 8 — Plugin Packaging & Bash Monitor

## Current Position

Phase: 7 of 10 (CLI Formatters: list, purge, resolve) — COMPLETE
Plan: 1 of 1 in current phase (COMPLETE)
Status: Ready to advance to Phase 8
Last activity: 2026-04-24 — Phase 7 Plan 01 complete: format.WriteTable + list/purge/resolve wired; 16 unit tests + 4 integration tests added; CORE-09 counter regression pinned; CMD-03/CMD-04 closed; review 0 HI/0 ME, verifier 14/14 PASS; stripped binary 7.82 MB

Progress: [███████░░░] 70%

## Performance Metrics

**Velocity:**
- Total plans completed: 6
- Average duration: ~15 min
- Total execution time: ~91 min

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 02 | 1 | ~5 min | ~5 min |
| 03 | 1 | ~4 min | ~4 min |
| 04 | 1 | ~25 min | ~25 min |
| 05 | 1 | (unrecorded) | — |
| 06 | 1 | ~20 min | ~20 min |
| 07 | 1 | ~30 min | ~30 min |

**Recent Trend:**
- Last 6 plans: 02-01 (5 min), 03-01 (4 min), 04-01 (25 min), 05-01 (—), 06-01 (20 min), 07-01 (30 min)
- Trend: ↑ (larger phases with more artifacts + integration suites)

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
- cli/format (Phase 7, LD-2): stdlib `text/tabwriter` with `NewWriter(w, 0, 0, 2, ' ', 0)` for list rendering; RFC3339 UTC timestamps, 48-char condition truncation with ellipsis, `-` for nil ResolvedAt, CreatedAt-then-ID sort
- cli/purge (Phase 7, LD-3): kong `xor:"target"` is **flag-only** (verified in kong model.go:408) — positional `<id>` handled separately; store-side `ErrPurgeArgRequired` enforces exactly-one-target semantics
- cli/resolve (Phase 7, LD-1): exit-code contract is **0/1** (success / any error); ErrNotOwner folds into exit 1 along with ErrUnknownID and ErrAlreadyResolved — no distinct exit 2 for resolve (unlike status's 0/2/1)
- cli/* (Phase 7): all three `runList`/`runPurge`/`runResolve` functions pure over `(out, errW io.Writer, path string, ...)` mirroring Phase 6's `runStatus` pattern — kong `Run()` wrappers translate to `os.Exit(code)`

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
Stopped at: Phase 7 Plan 01 complete — `internal/cli/format/WriteTable` (text/tabwriter) + list/purge/resolve commands wired; 6 atomic commits (7c11b88 → bc8b0b5); 16 unit + 4 integration tests green under -race; `TestPurge_CounterNotDecremented` pins CORE-09 (counter never decremented on purge); review 0 HI / 0 ME / 5 LO; verifier 14/14 PASS; stripped binary 7.82 MB; CMD-03 + CMD-04 closed
Resume file: None — ready for Phase 8 (Plugin Packaging & Bash Monitor)
