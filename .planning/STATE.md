# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-23)

**Core value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).
**Current focus:** Phase 9 — CI Release, Cross-compile & Test Gates

## Current Position

Phase: 10 of 10 (Docs & Dogfooding Polish) — IN PROGRESS
Plan: CONTEXT captured, planning next
Status: Phase 9 closed (WR-01/WR-02 fixed in 0fdddd5); Phase 10 CONTEXT auto-mode (7 areas resolved)
Last activity: 2026-04-24 — Phase 9 Plan 01 complete: GoReleaser v2 config (6-arch darwin/linux/windows × amd64/arm64) + tag-driven release workflow + 3-OS CI matrix with -race on ubuntu+macos and non-race on windows + release dry-run job + golangci-lint v2.11.4 pinned with errcheck/forbidigo/gofmt/govet/staticcheck + TestConcurrentWaiters_AllSeeResolve + TestStatus_PurgedMidPoll_Exit1 (QA-02 gap-fill, -race -count=5 green) + Makefile release-snapshot+ci-local targets; codebase-wide errcheck/shadow/S1016 cleanup (21 Fprint sites, 2 shadow, 4 ResolveIn); 6 atomic commits (a10508e → fddb006); snapshot build produced 4 tar.gz + 2 zip + checksums.txt; binary 7.41 MB / startup P95 38.4 ms / stdout 0 bytes; ldflags version injection verified (`mcp-chain 0.0.1-snapshot-none`); 3 deviations auto-fixed (N-1 go.mod 1.25 stays, N-2 gofmt→formatters block, N-3 sub-process env propagation) + 1 codebase cleanup batch (N-4)

Progress: [█████████░] 90%

## Performance Metrics

**Velocity:**
- Total plans completed: 8
- Average duration: ~26 min (executor time only)
- Total execution time: ~225 min (Phase 9 skewed by goreleaser install + windows/arm64 TLS compile)

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
| 09 | 1 | ~95 min (inc. 30-min goreleaser install + 32-min snapshot build) | ~95 min |

**Recent Trend:**
- Last 8 plans: 02-01 (5 min), 03-01 (4 min), 04-01 (25 min), 05-01 (—), 06-01 (20 min), 07-01 (30 min), 08-01 (40 min + inline resume), 09-01 (95 min)
- Trend: ↑ (release-infrastructure phase dominated by toolchain install + cross-compile time; task-scope work itself ~20 min)

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
- Phase 9 Go directive (N-1): go.mod stays at `go 1.25.0` — SDK v1.5.0 requires it; CI uses `go-version: "1.25"`, release workflow uses `go-version-file: go.mod` (single source of truth)
- Phase 9 lint config (N-2): golangci-lint v2 requires gofmt in top-level `formatters:` block (not `linters:`); v2.11.4 pinned in both .golangci.yml and golangci-lint-action@v8
- Phase 9 release pin: `goreleaser-action@v6` uses `~> v2` version constraint (allows 2.x minor bumps, blocks v3 surprise); `golangci-lint-action@v8` uses explicit `version: v2.11.4` (patch-pin because linter rules shift)
- Phase 9 CI matrix: fail-fast: false on 3-OS test matrix; -race enabled on ubuntu-latest + macos-latest (Go race detector unsupported on Windows → plain `-count=1` there)
- Phase 9 integration tests (N-3): sub-process helpers spawning `resolve --force` / `purge` from parent Go test MUST propagate `XDG_STATE_HOME` via explicit `cmd.Env = env` — inheriting from os.Environ is NOT enough because `t.Setenv` doesn't flow to spawned children
- Phase 9 known pre-existing (out of scope): SDK v1.5.0 unconditionally imports `net/http` in mcp/{event,streamable,sse,shared}.go — this is a regression from earlier stdio-only versions; candidate for Phase 10 SDK review if product principle "no net/http" is reasserted
- Phase 9 known pre-existing (out of scope): `scripts/smoke-chain-wait.sh` fails in minimal shells with `go: command not found` — script relies on PATH containing Go SDK; candidate for Phase 10 dev-env docs

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
Stopped at: Phase 10 CONTEXT captured (auto-mode) — 15 locked decisions across 7 gray areas; scope = README at repo root + CMD-01..04 namespace prose update + scripts/dogfood.sh + DOGFOOD.md 2-session checklist + smoke-chain-wait.sh PATH guard + README security notes (Phase 8 WR-02 inherit) + first tag v0.1.0; MCP-01 `net/http` regression NOT re-opened. Phase 9 close-out: WR-01 (integration-tagged tests now in CI matrix) + WR-02 (windows .exe suffix) fixed in 0fdddd5.
Resume file: .planning/phases/10-docs-dogfooding/10-CONTEXT.md
