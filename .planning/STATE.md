# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-23)

**Core value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).
**Current focus:** Phase 9 — CI Release, Cross-compile & Test Gates

## Current Position

Phase: 9 of 10 (CI Release, Cross-compile & Test Gates) — IN PROGRESS
Plan: CONTEXT captured; planning next
Status: Phase 9 context gathered (--auto); advancing to plan-phase
Last activity: 2026-04-24 — Phase 8 Plan 01 complete: plugin manifests (plugin.json, marketplace.json, .mcp.json) + 4 slash-command prompts (reg/wait/list/purge, ≤30 words each) + chain-wait.sh bash 3.2 monitor + 5 shell gates + E2E smoke harness; WR-01 OWNER placeholder fixed; review 0 HI/1 ME (WR-02 $ARGUMENTS norm, deferred to Phase 10 docs); verifier 4/4 SC PASS; go.mod/go.sum byte-identical; stripped binary 7.82 MB

Progress: [████████░░] 80%

## Performance Metrics

**Velocity:**
- Total plans completed: 7
- Average duration: ~16 min (executor time only)
- Total execution time: ~130 min (plus retry overhead on phase 8 after 429 mid-execution)

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 02 | 1 | ~5 min | ~5 min |
| 03 | 1 | ~4 min | ~4 min |
| 04 | 1 | ~25 min | ~25 min |
| 05 | 1 | (unrecorded) | — |
| 06 | 1 | ~20 min | ~20 min |
| 07 | 1 | ~30 min | ~30 min |
| 08 | 1 | ~40 min + inline resume | ~40 min |

**Recent Trend:**
- Last 7 plans: 02-01 (5 min), 03-01 (4 min), 04-01 (25 min), 05-01 (—), 06-01 (20 min), 07-01 (30 min), 08-01 (40 min + inline resume after 429)
- Trend: ↑ (multi-artifact phases — plugin tree + shell gates + Go tests + E2E smoke)

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
- plugin packaging (Phase 8, LD-13): `.claude-plugin/marketplace.json` lives at REPO ROOT (not inside plugin/); `/plugin install <repo>` requires a marketplace manifest at the repo root pointing to the plugin subdir via `source: "./plugin"`
- plugin commands (Phase 8, LD-14, user-approved 2026-04-24): slash commands named `reg`/`wait`/`list`/`purge` (not `chain-reg`/etc.) so Claude Code's mandatory `<plugin>:<command>` namespace yields clean `/mcp-chain:reg` forms; REQUIREMENTS.md CMD-01..04 prose to be updated in Phase 10 docs
- chain-wait.sh (Phase 8, LD-5/6/7): POSIX bash 3.2-safe; translates Phase 6 exit codes 0→`echo continue && exit 0`, 2→`sleep 1` loop, 1|127|*→stderr+exit 1; `--timeout DURATION` accepts exactly `{N}s|{N}m|{N}h` with 604800s (168h) clamp; exit 124 on timeout per `timeout(1)` convention
- chain-wait.sh (Phase 8, LD-9): binary path resolved via `${MCP_CHAIN_BIN:-mcp-chain}` — env override enables test isolation without modifying PATH
- Phase 8 known unfixed: WR-02 `$ARGUMENTS` shell-injection surface in wait.md/purge.md prompt templates — documented as Claude Code ecosystem norm (trusted slash-command invocation); Phase 10 README to note

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
