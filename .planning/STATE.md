# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-23)

**Core value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).
**Current focus:** Phase 9 — CI Release, Cross-compile & Test Gates

## Current Position

Phase: 10 of 10 (Docs & Dogfooding Polish) — Plan 01 COMPLETE (pending human v0.1.0 tag push)
Plan: 10-01-PLAN.md complete 2026-04-24 (5 atomic commits); Task 7 checkpoint = human DOGFOOD.md walk + git tag v0.1.0
Status: All 10 phases code-complete; v0.1.0 release gated on human manual dogfood + tag push
Last activity: 2026-04-24 — Phase 10 Plan 01 complete: README.md at repo root (DIST-04, 181 lines, 7 /mcp-chain: command refs, 11 D-01 sections in order) + scripts/dogfood.sh CLI-only end-to-end smoke (exit 0 in ~2s, build→unknown(1)→seed→pending(2)→resolve --force→resolved(0)→list→purge --all→list empty) + .planning/phases/10-docs-dogfooding/DOGFOOD.md 15-step 2-session manual checklist (Linux+macOS cols, step 7 exercises MCP resolve not --force, macOS deferral escape hatch per D-10) + scripts/smoke-chain-wait.sh PATH guard (exit 127 with readable stderr on missing go) + REQUIREMENTS.md prose swap (/chain-*→/mcp-chain:* across CMD-01..04/DIST-04 + HELPER-01 path scripts/chain-wait.sh→plugin/scripts/chain-wait.sh + MCP-01 net/http note preserved); 5 atomic commits (3620728 → c1de99a); make ci-local green (lint 0 issues, size 7.41 MB, startup P95 40.4 ms, stdout 0 bytes, 8-pkg -race test pass); 3 deviations auto-fixed/documented (sed-ordering bug in HELPER-01 path restored manually; 2 plan-verify over-broad regexes for filename-vs-command and POSIX bracket vs [[ test construct — documented, no code change needed)

Progress: [██████████] 100% (code-complete; v0.1.0 tag is human gate)

## Performance Metrics

**Velocity:**
- Total plans completed: 9
- Average duration: ~25 min (executor time only)
- Total execution time: ~250 min (Phase 9 skewed by goreleaser install + windows/arm64 TLS compile)

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
| 10 | 1 | ~25 min | ~25 min |

**Recent Trend:**
- Last 9 plans: 02-01 (5 min), 03-01 (4 min), 04-01 (25 min), 05-01 (—), 06-01 (20 min), 07-01 (30 min), 08-01 (40 min + inline resume), 09-01 (95 min), 10-01 (25 min)
- Trend: ↓ (back to docs-phase baseline after Phase 9 release-infra spike)

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
- Phase 10 D-01..D-15: 15 locked decisions applied (README section order, plugin-first install, EFF word `otter` for demos, CLI-only dogfood.sh no serve race, macOS deferral escape hatch, guarded `if ! command -v` form for PATH guards, 3-bullet security notes verbatim, MCP-01 net/http not re-opened, v0.1.0 tag push = human gate). Plan 01 commits: 3620728 (REQUIREMENTS prose swap + HELPER-01 path fix) → 7da601c (smoke-chain-wait PATH guard) → 53cc6a0 (dogfood.sh) → ac1c4d2 (DOGFOOD.md) → c1de99a (README.md).
- Phase 10 auto-fixed deviation: HELPER-01 path `plugin/scripts/chain-wait.sh` got corrupted to `plugin/scripts/mcp-chain:wait.sh` by the plan's sed sequence (the `/chain-wait→/mcp-chain:wait` swap also matched inside the newly-added path prefix). Manually restored. Plan's verify regex `grep -E '/chain-(reg|wait|list|purge)'` over-matches on the file path itself — semantic intent (no legacy *command* refs) is met; documented in Plan 01 SUMMARY.
- Phase 10 auto-fixed deviation: dogfood.sh plan verify `grep -nE '\[\['` flags POSIX bracket expression `[[:space:]]` inside a grep regex as a bash `[[ ... ]]` test construct. They are different; the script is bash 3.2–safe. Verified via stricter regex targeting only the test-construct form. No file change.

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
Stopped at: Phase 10 Plan 01 complete (5 atomic commits: 3620728 REQUIREMENTS prose swap → 7da601c smoke PATH guard → 53cc6a0 dogfood.sh → ac1c4d2 DOGFOOD.md → c1de99a README). Task 6 verification green (ci-local + both smokes). Task 7 = human gate: (1) walk DOGFOOD.md Linux column on a real machine, (2) decide macOS green-or-deferred per D-10, (3) `git tag v0.1.0 && git push origin v0.1.0` — executor intentionally did NOT push.
Resume file: .planning/phases/10-docs-dogfooding/10-01-SUMMARY.md + .planning/phases/10-docs-dogfooding/DOGFOOD.md (walk this before tagging)
