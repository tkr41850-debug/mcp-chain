---
phase: 03-xdg-path-resolution
plan: 01
subsystem: infra
tags: [go, xdg, statepath, filesystem, os-getenv]

requires:
  - phase: 02-wordlist-id-allocator
    provides: testify/require v1.11.1 already in go.mod

provides:
  - internal/statepath.Resolve() (string, error) — XDG+HOME path resolver with 0700 MkdirAll
  - internal/statepath.ErrHomeUnset — wrap-compatible sentinel for missing HOME+XDG
  - Parent directory guaranteed at mode 0700 before returning path

affects:
  - phase-04-store (will call statepath.Resolve() once at Open() time)
  - phase-06-cli (--help should print resolved state path to close CORE-06 second half)
  - phase-09-ci (Windows statepath_windows.go build tag variant)
  - phase-10-docs (README must document XDG_STATE_HOME override and dot-dir fallback)

tech-stack:
  added: []
  patterns:
    - "env-only startup path: os.Getenv(HOME/XDG_STATE_HOME) directly — no os/user, no NSS"
    - "idempotent directory creation: os.MkdirAll(parent, 0o700) — never chmods existing dirs"
    - "XDG spec empty-string rule: os.Getenv + != '' check (not os.LookupEnv) treats empty as unset"

key-files:
  created:
    - internal/statepath/resolve.go
    - internal/statepath/resolve_test.go
  modified: []

key-decisions:
  - "Use os.Getenv(HOME) directly instead of os.UserHomeDir — project policy for NSS-free startup path"
  - "Fallback is ~/.mcp-chain/ (dot-dir) not strict-XDG ~/.local/state/mcp-chain/ — aligned with PROJECT.md and REQUIREMENTS.md CORE-06"
  - "Single Resolve() function (not split StatePath() + EnsureDir()) — YAGNI per RESEARCH.md Q3"
  - "No writability probe in Resolve() — Phase 4's store write surfaces permission errors cleanly"

patterns-established:
  - "Package doc comment cites requirement ID (CORE-06) — audit trail pattern"
  - "t.Setenv + t.TempDir for env-isolated tests — no defer os.Unsetenv needed"

requirements-completed: [CORE-06]

duration: 4min
completed: 2026-04-23
---

# Phase 3 Plan 01: XDG Path Resolution Summary

**stdlib-only `statepath.Resolve()` with XDG_STATE_HOME-first, HOME/.mcp-chain fallback, and idempotent 0700 MkdirAll — NSS-free startup path confirmed**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-04-23T15:41:27Z
- **Completed:** 2026-04-23T15:44:47Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `internal/statepath/resolve.go`: 73-line production file with `Resolve() (string, error)`, `ErrHomeUnset` sentinel, three package-private constants (`stateFileName`, `dirName`, `homeSubdir`), and package doc citing CORE-06
- `internal/statepath/resolve_test.go`: 127-line test file with 6 env-isolated tests covering all VALIDATION.md rows 41–46
- Zero new dependencies: stdlib only; `testify/require` reused from Phase 2
- `go list -deps ./internal/statepath/ | grep '^os/user$'` output is empty — NSS-free startup path confirmed

## Task Commits

1. **Task 1: statepath.Resolve() with XDG + HOME fallback** - `9468473` (feat)
2. **Task 2: env-isolated tests for Resolve() (CORE-06)** - `35ef15e` (feat)

**Plan metadata:** (final docs commit)

## Files Created/Modified

- `internal/statepath/resolve.go` — `Resolve() (string, error)` + `ErrHomeUnset` + XDG/HOME resolution with `os.MkdirAll(parent, 0o700)`
- `internal/statepath/resolve_test.go` — 6 tests: XDGSet, HOMEFallback, NeitherSet, EmptyXDG, ParentAlreadyExists, Idempotent

## CORE-06 Coverage Status

**PARTIAL-MET** — The resolution function half of CORE-06 is complete. `--help` and README documentation halves are scoped to Phase 6/7 (CLI) and Phase 10 (docs) respectively per REQUIREMENTS.md traceability and RESEARCH.md Phase Requirements note.

## Six Test Summary

| Test | Behavior Pinned |
|------|----------------|
| `TestResolve_XDGSet` | XDG_STATE_HOME set → `$X/mcp-chain/state.json`, parent mode 0700, state file NOT created |
| `TestResolve_HOMEFallback` | XDG unset/empty, HOME set → `$HOME/.mcp-chain/state.json`, parent mode 0700 |
| `TestResolve_NeitherSet` | Both unset/empty → `errors.Is(err, ErrHomeUnset)` true, path empty string |
| `TestResolve_EmptyXDG` | XDG_STATE_HOME="" (explicit empty) falls through to HOME per XDG spec §3 |
| `TestResolve_ParentAlreadyExists` | Pre-existing parent with 0755 preserved — MkdirAll does NOT chmod down |
| `TestResolve_Idempotent` | Two Resolve() calls in same env both succeed with equal paths |

## Smoke Invariant (Provenance Record)

Command: `go list -deps ./internal/statepath/ | grep '^os/user$'`
Output: (empty)
Result: PASS — `os/user` is NOT in the dep graph

This invariant must not regress. Any future change that adds `os/user` to the transitive deps of `internal/statepath` (e.g., importing `os.UserHomeDir` or `os/user.Current`) will break this check and violates CONTEXT.md line 32 project policy.

## Verification Results

| Command | Result |
|---------|--------|
| `go build ./internal/statepath/...` | PASS |
| `go vet ./internal/statepath/...` | PASS |
| `go test -race -count=1 ./internal/statepath/... -v` (6 tests) | PASS |
| `go test -race -count=1 ./...` (full suite) | PASS |
| `go list -deps ./internal/statepath/ \| grep -c '^os/user$'` | 0 (PASS) |
| `go build ./...` | PASS |

## Decisions Made

- `os.Getenv("HOME")` directly instead of `os.UserHomeDir` — project policy (NSS-free startup path)
- Fallback path is `~/.mcp-chain/` (dot-dir) not strict-XDG `~/.local/state/mcp-chain/` — both PROJECT.md and REQUIREMENTS.md CORE-06 independently specify this; intentional project choice
- No writability probe in `Resolve()` — Phase 4's store write surfaces permission errors cleanly (RESEARCH.md Q2 closed)
- Single `Resolve()` function, not split `StatePath()` + `EnsureDir()` — YAGNI (RESEARCH.md Q3 closed)

## Deviations from Plan

None — plan executed exactly as written. Code copied verbatim from RESEARCH.md copy-paste blocks.

## Issues Encountered

None.

## Follow-up Flags (from Plan output spec)

- **Phase 6/7:** CLI `--help` output must print the resolved state path (CORE-06 second half). Wire `statepath.Resolve()` into `--help` and/or a `status` pre-flight.
- **Phase 10:** README must document `$XDG_STATE_HOME` override behavior and the dot-dir fallback (`~/.mcp-chain/`), plus the strict-XDG override hint (`export XDG_STATE_HOME=~/.local/state`).
- **Phase 9:** Windows path resolution (`%LOCALAPPDATA%\mcp-chain\state.json`) deferred to CI cross-compile validation. Will ship as `statepath_windows.go` with `//go:build windows` build tag.

## Next Phase Readiness

Phase 4 (store) can import `github.com/tkr41850-debug/mcp-chain/internal/statepath` immediately.
- Call `statepath.Resolve()` once at `store.Open()` time — the returned path has a guaranteed-existing parent directory at mode 0700
- Store does NOT need to re-check parent-dir existence
- `errors.Is(err, statepath.ErrHomeUnset)` for exit-code differentiation if needed
- `go.mod` / `go.sum` unchanged — zero new dependencies introduced

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundary surfaces introduced. Only local filesystem access via `os.MkdirAll` (user-controlled paths, single-user local tool threat model, documented in plan threat register).

---
*Phase: 03-xdg-path-resolution*
*Completed: 2026-04-23*
