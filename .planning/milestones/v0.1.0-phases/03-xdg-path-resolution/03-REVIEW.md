---
phase: 03-xdg-path-resolution
reviewed: 2026-04-23T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - internal/statepath/resolve.go
  - internal/statepath/resolve_test.go
findings:
  critical: 0
  warning: 0
  info: 3
  total: 3
status: clean
---

# Phase 3: Code Review Report

**Reviewed:** 2026-04-23
**Depth:** standard
**Files Reviewed:** 2
**Status:** clean (only informational suggestions)

## Summary

Phase 3 delivers `internal/statepath.Resolve()` — a pure-stdlib XDG/HOME
path resolver with idempotent parent-directory creation at mode 0700.
The implementation is tight (73 lines of production code), well-documented,
and the test suite (127 lines, 6 tests) pins every VALIDATION.md requirement
behavior.

All CORE-06 correctness criteria are met:

- XDG_STATE_HOME (non-empty) routes to `$XDG_STATE_HOME/mcp-chain/state.json`.
- XDG_STATE_HOME=`""` correctly falls through to HOME per XDG spec §3
  (pinned by `TestResolve_EmptyXDG`).
- `ErrHomeUnset` is a distinct `errors.Is`-compatible sentinel for the
  both-unset case.
- `os.MkdirAll(parent, 0o700)` is used, which does NOT tighten a
  pre-existing parent's permissions (pinned by
  `TestResolve_ParentAlreadyExists`).
- The state file itself is not created — `Resolve` only ensures the parent
  (pinned in `TestResolve_XDGSet` lines 37-38).
- No `os/user` in the dependency graph — confirmed upstream by the smoke
  invariant recorded in `03-01-SUMMARY.md` line 99-101
  (`go list -deps ./internal/statepath/ | grep '^os/user$'` returns empty).
- All paths use `filepath.Join`; no hardcoded separators, so the logic is
  POSIX-correct today and will remain correct if lifted verbatim onto a
  build-tagged platform variant later.

No bugs, security issues, or warnings found. Three minor informational
notes below — all are suggestions, none block Phase 3 acceptance.

## Info

### IN-01: `TestResolve_HOMEFallback` does not assert state-file non-creation

**File:** `internal/statepath/resolve_test.go:43-59`
**Issue:** `TestResolve_XDGSet` correctly asserts that `Resolve()` does
not create the state file itself (lines 37-38,
`require.True(t, os.IsNotExist(err), ...)`). The symmetric HOME-fallback
test omits that assertion. Since both branches share the same MkdirAll
+ `filepath.Join` tail, the coverage gap is harmless, but symmetry
between the two primary-path tests makes future regressions cheaper
to catch.
**Fix:** Append the same three lines used in `TestResolve_XDGSet`:
```go
_, err = os.Stat(wantPath)
require.True(t, os.IsNotExist(err), "Resolve must not create the state file")
```

### IN-02: `t.Setenv("HOME", "")` vs. explicit unset

**File:** `internal/statepath/resolve_test.go:21, 64-65, 99, 120`
**Issue:** The tests use `t.Setenv("HOME", "")` where they want
"HOME is not honored". `t.Setenv` cannot unset (no `t.Unsetenv`
helper exists), and `os.Getenv` returns `""` for both unset and
empty-set variables, so the behavior being tested is exercised.
The comment on lines 18-21 documents this explicitly and justifies
the choice. Informational only — not actionable under current
stdlib APIs without reaching for `os.Unsetenv` + manual defer
(which would defeat `t.Setenv`'s cleanup guarantees).
**Fix:** None required. If Go ever adds `t.Unsetenv`, prefer it here
for semantic clarity. No change recommended for Phase 3.

### IN-03: No test for `MkdirAll` failure path

**File:** `internal/statepath/resolve.go:68-70`
**Issue:** The error-wrapping branch at lines 68-70
(`fmt.Errorf("statepath: create parent dir %q: %w", parent, err)`) is
not covered by a test. Exercising it portably is awkward — the
usual trick is making `$XDG_STATE_HOME` point at a path whose parent
is a regular file (so MkdirAll sees ENOTDIR), but that requires
setup that varies by OS. Given the single `%w` call is trivially
correct and the happy paths are thoroughly tested, this is not a
quality gap worth closing in Phase 3.
**Fix:** Optional — if desired, add a test like:
```go
func TestResolve_MkdirAllFails(t *testing.T) {
    tmp := t.TempDir()
    // Create a regular file where mcp-chain/ wants to live.
    blocker := filepath.Join(tmp, "mcp-chain")
    require.NoError(t, os.WriteFile(blocker, nil, 0o600))
    t.Setenv("XDG_STATE_HOME", tmp)
    t.Setenv("HOME", "")

    _, err := Resolve()
    require.Error(t, err)
    require.Contains(t, err.Error(), "statepath: create parent dir")
}
```
Skip if Phase 4's store-layer tests will exercise the same error surface.

---

_Reviewed: 2026-04-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
