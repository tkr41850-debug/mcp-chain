---
phase: 06-cli-dispatch-status
reviewed: 2026-04-24T00:00:00Z
depth: deep
files_reviewed: 7
files_reviewed_list:
  - cmd/mcp-chain/main.go
  - internal/cli/status.go
  - internal/cli/status_test.go
  - internal/cli/integration_test.go
  - internal/cli/export_test.go
  - internal/cli/stubs.go
  - internal/cli/stubs_test.go
findings:
  critical: 0
  high: 1
  medium: 2
  low: 3
  nit: 2
  total: 8
status: issues_found
---

# Phase 6: Code Review Report

**Depth:** deep
**Status:** issues_found (1 HIGH flake risk; rest advisory)

## Summary

Phase 6 meets the locked exit-code contract. `runStatus` correctly returns `int` (Pitfall 2 satisfied), `os.Exit` is called from `Run` only (avoiding kong's `error:` prefix), `--version` is a manual pre-parse ahead of `kong.Writers(stderr, stderr)` (Pitfall 5 satisfied), and stdout/stderr routing per SC #3 is belt-and-braces verified at unit + integration layers. `go vet` clean; `go test -race ./internal/cli/...` green at package scope; `go test -count=1 -tags=integration` green.

One genuine concern: the SC #2 concurrent test FAILED (1.26s > 1s) during a full `-race -tags=integration` run on this machine, but passed 3/3 in isolation under `-race`. This is a CI-flake risk, not a contract violation, but belongs in the record.

## High

### HI-01: SC #2 concurrent test is flake-prone under full `-race` suite load
**File:** `internal/cli/integration_test.go:149-205`
**Issue:** Observed elapsed=1.26s during full `-race -tags=integration` run (other suites co-resident, all under race instrumentation). Passes cleanly in isolation (3/3 at ~12s wall each) and without `-race`. Root cause is fork/exec × race-instrumentation overhead on a loaded box, not `LOCK_SH` serialization. The 1s budget is a bare-metal bound; CI under `-race` will occasionally brush it.
**Fix:** Keep the 1s assertion as the specification, but additionally log per-child durations and assert a stronger signal of non-serialization — e.g. `require.Less(t, elapsed, 4*time.Duration(N)*singleProbeDuration/10, ...)`. Alternatively, raise the budget to 2s and retain the inline comment explaining why; or skip under `testing.Short()`; or gate on a `-race` build tag with a relaxed bound. Current assertion will red-light CI intermittently.

## Medium

### ME-01: `statepath.Resolve` error branch in `StatusCmd.Run` is untested
**File:** `internal/cli/status.go:39-43`
**Issue:** The `if err := statepath.Resolve()` path writes to `os.Stderr` and calls `os.Exit(1)` — the correct contract behavior — but is exercised by neither the unit tests (which bypass `Run` entirely by calling `runStatus` directly) nor the integration tests (which always supply a writable `XDG_STATE_HOME`). A regression that swallowed the error or wrote to stdout would not be caught.
**Fix:** Add an integration test that sets `XDG_STATE_HOME=/dev/null/nope` (or a path under a non-writable parent) and asserts exit=1, empty stdout, `mcp-chain:` prefix on stderr. Alternatively, factor `Run` to take an `io.Writer` pair + a path-resolver func and unit-test the resolve-failure branch.

### ME-02: `export_test.go` uses a mutable `var` instead of a `func` alias
**File:** `internal/cli/export_test.go:9-11`
**Issue:** `var RunStatus = func(...) int { return runStatus(...) }` is mutable from xtests. Any future test that accidentally reassigns `RunStatus` (e.g. a parallel test wrapping it with a spy) silently bleeds into every other test in the same package run. Lower-risk form is `func RunStatus(...) int { return runStatus(...) }`.
**Fix:** `func RunStatus(out, errW io.Writer, path, id string) int { return runStatus(out, errW, path, id) }` — same API to xtests, immutable binding.

## Low

### LO-01: `runStatus` switch on `err == nil && r.Status == X` risks silent drift
**File:** `internal/cli/status.go:64-77`
**Issue:** The switch enumerates `"resolved"` and `"pending"` as literal strings. If `store.Record.Status` ever grows a third value (e.g. `"expired"`), the default branch will emit a generic error instead of a typed status — arguably desired, but the string literals duplicate `store`'s internal `statusPending`/`statusResolved` constants (store.go:77, 111).
**Fix:** Export `store.StatusResolved` / `store.StatusPending` (or expose typed constants) and reference them here. Low priority — current literals are correct.

### LO-02: `seedStateForChild` returns `""` for "no registration" setups
**File:** `internal/cli/integration_test.go:22-29, 71-79`
**Issue:** The unknown-case setup returns `""`, which the loop then discards via `idFn`. Works, but the `string` sentinel is implicit. A reader may assume returning `""` is meaningful.
**Fix:** Have the setup closure return `(string, error)` or pass an `idFn` that ignores the seeded value explicitly — current code already does the latter, so just a comment noting the sentinel would suffice.

### LO-03: `TestRunStatus_Unknown_Exit1` creates a `Store` and immediately discards it
**File:** `internal/cli/status_test.go:65-68`
**Issue:** `st, err := store.Open(path); require.NoError(t, err); _ = st` — `store.Open` does no I/O (store.go:52-61), so the open+discard is a no-op. The file does not, in fact, exist on disk when `runStatus` runs. The test still passes because `loadState` of a missing file yields an empty state (per store semantics), which then produces `ErrUnknownID`. Test is correct but the setup is misleading.
**Fix:** Drop the `store.Open` call entirely with a comment: "No state file exists → loadState returns empty state → Get returns ErrUnknownID." Or actually materialize an empty state file if exercising that code path matters.

## Nit

### NIT-01: `main.go` `--version` pre-parse accepts `--version` anywhere in argv
**File:** `cmd/mcp-chain/main.go:49-54`
**Issue:** `mcp-chain status myid --version` prints the version and exits 0, ignoring the `status` request. Matches common CLI behavior (git, docker both do this) but worth noting if a future test asserts strict positional rules.
**Fix:** None needed; document the choice in a comment if desired.

### NIT-02: Doc comment at `stubs.go:1-6` references "Phase 7 (list/purge/resolve)"
**File:** `internal/cli/stubs.go:5-6`
**Issue:** `resolve` is not a separate subcommand in the current grammar; resolution happens via the MCP tool. Minor doc inaccuracy.
**Fix:** Drop `/resolve` from the phase-7 list.

---

_Reviewed: 2026-04-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
