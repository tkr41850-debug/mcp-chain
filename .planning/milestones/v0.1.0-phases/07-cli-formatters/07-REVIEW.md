---
phase: 07-cli-formatters
reviewed: 2026-04-24T00:00:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - internal/cli/format/table.go
  - internal/cli/format/table_test.go
  - internal/cli/list.go
  - internal/cli/list_test.go
  - internal/cli/purge.go
  - internal/cli/purge_test.go
  - internal/cli/resolve.go
  - internal/cli/resolve_test.go
  - internal/cli/integration_test.go
  - internal/cli/stubs.go
  - internal/cli/export_test.go
findings:
  critical: 0
  warning: 0
  info: 5
  total: 5
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-04-24
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found (Info-only; no blockers)

## Summary

Phase 7 lands clean. The locked exit-code contract from 07-CONTEXT.md is honored row-for-row (list 0/1, purge 0/1, resolve 0/1 — note the user prompt's "exit 2 for ErrNotOwner" is a prompt-side typo; the locked contract is exit 1 for all resolve errors, and the code matches the spec). Writer routing is disciplined: `runList`/`runPurge`/`runResolve` take `(out, errW io.Writer, ...)` and have zero direct `os.Stdout/Stderr` references; only the outer `Run()` methods touch globals. Sentinel error handling uses `errors.Is` correctly for `ErrUnknownID` / `ErrNotOwner` / `ErrAlreadyResolved` / `ErrPurgeArgRequired`. OwnerToken never leaks into list output (the formatter pointedly does not render it). Counter-non-decrement is preserved at the store layer and explicitly regression-tested end-to-end. Kong tags are correct for the "flag-only xor" constraint documented in 07-RESEARCH.md — `xor:"target"` lives on `--all`/`--resolved`, with `<id>` handled positionally and the tri-state "exactly one target" check delegated to `store.ErrPurgeArgRequired`. The findings below are all Info: dead constant, byte-vs-rune truncation, stale doc comment, and a couple of minor terseness/defense-in-depth observations. None warrant a fix-now; they are noted for the next sweep.

Commit atomicity: the six commits are correctly ordered — `7c11b88` (format package) is self-contained; `32f142a`/`be3ea4d`/`1bc5dc7` each add one command + its tests + an export alias in `export_test.go`; `6a9735f` adds integration rows; `bc8b0b5` removes the now-empty `TestStubsExitCodes`. Each commit should build and test green in isolation.

## Info

### IN-01: `ExitCodeNotImplemented` constant is now dead code

**File:** `internal/cli/stubs.go:26`
**Issue:** After Phase 7 the constant has zero callers in the repo (verified via grep; only the comment inside `stubs.go` itself references it). The comment claims it is "retained as a documented reserved value (do not collide with status 0/1/2 in CORE-01)" but nothing enforces that reservation — a future author assigning `3` to a new exit code would not trip any linter. Keeping it is harmless (exported, zero-cost), but the justification is weaker than the comment implies.
**Fix:** Either (a) delete the constant and update the package docstring, or (b) tighten the docstring to say explicitly that this is a convention-only reservation:
```go
// ExitCodeNotImplemented (3) is reserved by convention for future
// "stub" behaviour; no current subcommand returns it. Kept to avoid
// collision with status 0/1/2 in CORE-01.
const ExitCodeNotImplemented = 3
```
Recommend (a) — dead code is a liability, and the "reservation" is not load-bearing since the CORE-01 exit codes are enumerated in `status.go`'s doc comment.

### IN-02: `truncate` is byte-based, not rune-based

**File:** `internal/cli/format/table.go:65-73`
**Issue:** `truncate(s string, max int)` indexes `s[:max-3]` by bytes. A multi-byte UTF-8 condition (emoji, CJK) crossing the 45-byte boundary will produce invalid UTF-8 in the rendered cell. The code comment acknowledges the "ASCII assumption" at `conditionMaxWidth`'s declaration (line 23), and 07-CONTEXT.md does not demand UTF-8 correctness for this column, so this is strictly an observation, not a bug against the locked contract. But:
  - The `format` package is a natural future target for `--format=json` (07-CONTEXT.md deferred ideas), where a mid-rune cut would produce invalid JSON.
  - Operators occasionally describe conditions with characters outside ASCII (unit symbols, curly quotes from paste).

**Fix:** Convert to runes when truncating, still counting display-width by rune (acceptable since we're not trying to handle East-Asian double-width at this phase):
```go
func truncate(s string, max int) string {
    r := []rune(s)
    if len(r) <= max {
        return s
    }
    if max < 3 {
        return string(r[:max])
    }
    return string(r[:max-3]) + "..."
}
```
This is defensive and costs ~nothing at expected lengths. Defer if Phase 7's locked contract explicitly forbids scope creep; flag it for Phase 8/v2.

### IN-03: Stale doc comment in `stubs.go` package docstring

**File:** `internal/cli/stubs.go:1-7`
**Issue:** The package comment says "Phase 5 wired serve, Phase 6 wired status (see status.go), Phase 7 wired list/purge/resolve (see list.go, purge.go, resolve.go); only ServeCmd remains in this file." That is accurate, but the next sentence — "The ExitCodeNotImplemented constant is retained as a documented reserved value (do not collide with status 0/1/2 in CORE-01)" — is a promise without enforcement (see IN-01). The file no longer contains any "stubs" in the Phase-1 sense; `ServeCmd` is a fully-wired command. The filename `stubs.go` is itself misleading now.
**Fix:** Either rename `stubs.go` → `serve.go` (matches the one-command-per-file pattern established in Phase 7), or add a one-line note at the top of `stubs.go` acknowledging the filename is historical. Renaming is cleaner:
- Move `ServeCmd` + `Version` + (optionally) `ExitCodeNotImplemented` to `serve.go`
- Delete `stubs.go`
Also update `stubs_test.go` → either `serve_test.go` or just `version_test.go` (it only holds `TestVersionFlagWritesToStdout` + `buildBinary`). `buildBinary` is referenced by `integration_test.go` so the helper must move to a file visible to the xtest package — `version_test.go` works fine since it's in `package cli_test`.

### IN-04: `runPurge` / `runResolve` discard the `out` writer via `_ = out`

**File:** `internal/cli/purge.go:57`, `internal/cli/resolve.go:52`
**Issue:** Both functions take `out io.Writer` in their signature to match the Phase 6 `runXxx(out, errW, ...)` template, then silently discard it with `_ = out // … stdout empty`. This is correct behavior per the contract (LD-1: purge/resolve success rows emit nothing to stdout) and the comments explain the choice. However:
  - If a future contract change ever wants a success confirmation on stdout (e.g. `purge: removed 3 entries`), a reviewer touching only the signature would miss that the parameter is unused.
  - `go vet` will not flag this because the `_ =` discharge is explicit.

Minor hygiene suggestion: a `//nolint:unused` comment is overkill; a slightly sharper comment wording tied to the contract is enough. Current comment: `_ = out // purge emits nothing to stdout (LD-1 success rows all empty)` — this is actually fine. I would leave as-is unless the project has a convention to drop the unused parameter entirely:
```go
// Alternative: drop the `out` parameter. Downside: signature drifts
// from runStatus/runList — the uniform shape is a Phase 6 asset.
// Keep the current form.
```
**Fix:** None needed. Flagged for awareness only — the `_ = out` pattern is defensible and keeps signature uniformity.

### IN-05: Integration test helper `seedStateForChild` assumes XDG layout

**File:** `internal/cli/integration_test.go:23-30`
**Issue:** The helper constructs the state path as `filepath.Join(dir, "mcp-chain", "state.json")` — this mirrors `statepath.Resolve()`'s XDG-based layout when `XDG_STATE_HOME=dir`, but it is a duplicated assumption. If `statepath.Resolve` ever changes its layout (e.g., inserts a version directory, or adds an `mcp-chain/v1/state.json` segment), these integration tests will silently diverge from production without the unit tests catching it.
**Fix:** Consider exposing a `statepath.ResolveFor(xdgStateHome string) (string, error)` variant (or use the existing `Resolve()` with env-var setup) inside the helper, rather than hand-constructing the path:
```go
func seedStateForChild(t *testing.T, dir string, setup func(...) string) string {
    t.Helper()
    t.Setenv("XDG_STATE_HOME", dir) // rely on statepath.Resolve
    statePath, err := statepath.Resolve()
    require.NoError(t, err)
    require.NoError(t, os.MkdirAll(filepath.Dir(statePath), 0o700))
    st, err := store.Open(statePath)
    require.NoError(t, err)
    return setup(t, st)
}
```
Caveat: `t.Setenv` is process-global; the current approach (child process inherits `XDG_STATE_HOME=dir`) sidesteps that by never relying on the parent's env. The duplication is therefore intentional and defensible. Flagged for awareness — if Phase 3's statepath layout changes, remember to update this helper.

---

## Checks Run (all passing)

- [x] Kong tags: `xor:"target"` on `PurgeCmd.All` / `PurgeCmd.Resolved`; `<id>` positional with `optional:""`; `ResolveCmd.ID` required arg; `ResolveCmd.Force` plain flag. Matches 07-RESEARCH.md finding #2 re: kong v1.15 flag-only xor.
- [x] Writer routing: zero `os.Stdout`/`os.Stderr` references inside `runList`/`runPurge`/`runResolve`; all writes go through the injected `out`/`errW` writers. Outer `Run()` methods correctly pass `os.Stdout, os.Stderr`.
- [x] Sentinel error handling: `errors.Is(err, store.ErrNotOwner)` mapped to the locked "not owner (use --force to override)" line; `store.ErrUnknownID` mapped to "unknown id: <id>"; `ErrAlreadyResolved` mapped to "already resolved"; `ErrPurgeArgRequired` mapped to "purge requires <id>, --all, or --resolved".
- [x] Counter non-decrement: preserved at `store.Purge` (see store.go:188 comment) AND regression-tested end-to-end by `TestPurge_CounterNotDecremented` which reads raw state.json.
- [x] RFC3339 UTC: `tsFormat = "2006-01-02T15:04:05Z07:00"` applied via `t.UTC().Format(tsFormat)` in both the created and resolved columns. Empty `ResolvedAt` renders `-`.
- [x] 48-char truncation: `conditionMaxWidth = 48` with `...` suffix; boundary test asserts exactly 45 x's + "...".
- [x] `text/tabwriter` config: `NewWriter(w, 0, 0, 2, ' ', 0)` — minwidth=0, tabwidth=0, padding=2, padchar=space, flags=0. Matches the "two-space minimum separator, left-aligned, no ANSI" spec.
- [x] `tw.Flush()` call present (Pitfall 8 guarded).
- [x] No OwnerToken leakage in list output: `WriteTable` emits 5 columns — ID, STATUS, CONDITION, CREATED, RESOLVED. OwnerToken is on `store.Record` but deliberately not rendered. Verified by reading the format loop at table.go:51-59.
- [x] Race-safe: no goroutines spawned from the CLI paths; shared state (the store file) is serialized by flock + sync.Mutex inside the store. Executor's `go test -race` report applies.
- [x] Comments / docstrings: terse, referencing LD-N/Pitfall-N IDs from the plan. No multi-paragraph essays. Per-function comments explain *why* the shape (testability pattern) rather than restating *what*.
- [x] Commit atomicity: 6 commits, each with a coherent unit (format pkg; list wiring; purge wiring; resolve wiring; integration suite; cleanup). Every `feat` commit carries matching tests.
- [x] Sort stability: `sort.SliceStable` with `CreatedAt.Equal` → `Before` → ID lexicographic tiebreaker. Tested by `TestWriteTable_SortsByCreatedAtThenID`.
- [x] Empty-input handling: `WriteTable([]) ` and `WriteTable(nil)` return nil with zero output; caller (runList) owns the "no entries" hint on stderr. Separation of concerns per LD-11.
- [x] No new `go.mod` dependencies (verified: all imports are stdlib, internal packages, or already-present testify/require).
- [x] Exit-code contract row coverage:
  - list: empty (0), N entries (0), other error (1) — all three rows have unit tests
  - purge: by-id, --all, --resolved, bare, unknown id — five rows, all tested
  - resolve: --force success, no-force not-owner, unknown id, already-resolved — four rows, all tested
  - Each locked stderr string is asserted verbatim (not just a substring) in at least one test, preventing wording drift.
- [x] `export_test.go` aliases: `RunList` / `RunPurge` / `RunResolve` mirror the Phase 6 `RunStatus` pattern exactly.
- [x] Kong help strings terse per token-budget principle: "Run MCP stdio server.", "List all registered ids.", "Purge entries. Requires one of <id>, --all, or --resolved.", "Resolve an id (CLI escape hatch; use --force to bypass OwnerToken check).". None exceed a line; none repeat information already obvious from the subcommand name.

---

_Reviewed: 2026-04-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
