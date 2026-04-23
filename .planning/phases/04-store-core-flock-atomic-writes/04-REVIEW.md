---
phase: 04-store-core-flock-atomic-writes
plan: 01
reviewed: 2026-04-23T00:00:00Z
depth: deep
reviewer: gsd-code-reviewer
files_reviewed: 10
files_reviewed_list:
  - internal/store/schema.go
  - internal/store/errors.go
  - internal/store/store.go
  - internal/store/lock.go
  - internal/store/atomic_unix.go
  - internal/store/atomic_windows.go
  - internal/store/store_test.go
  - internal/store/integration_test.go
  - internal/store/export_test.go
  - go.mod
findings:
  critical: 0
  warning: 0
  info: 3
  total: 3
verdict: APPROVED
status: issues_found
---

# Phase 4 Plan 01: Code Review Report

**Verdict:** APPROVED
**Reviewed:** 2026-04-23
**Depth:** deep (cross-file analysis, lock/rename semantics, build-tag hygiene, threat model cross-check)
**Files Reviewed:** 10
**Findings:** 0 critical, 0 warning, 3 info

## Summary

The Phase 4 Plan 01 deliverables for `internal/store/` are solid and ready to ship. The hexagonal core correctly serialises concurrent access across processes (sibling `state.json.lock` via `gofrs/flock`) and within a process (`sync.Mutex` taken before `flock`), writes atomically via `renameio/v2` on POSIX and tmp+fsync+chmod+rename on Windows under the flock invariant, and enforces OwnerToken semantics with `crypto/subtle.ConstantTimeCompare` plus a `Force` bypass. All six sentinel errors are package-level for `errors.Is` matching, schema version mismatch produces `ErrSchemaVersion` with actionable context naming the path and both versions, and the monotonic counter is correctly never decremented on any Purge variant.

The STRIDE threat model items called out in the plan are all addressed in code: constant-time token compare (Spoofing), file mode 0600 enforced via `renameio.IgnoreUmask()` (Information Disclosure), atomic rename + flock (Tampering), and sentinel error identity for caller dispatch (Denial-of-service surface reduction). Stdout discipline is clean — no `fmt.Print*` or `log.Print*` anywhere in the package. Forbidden imports (`net/http`, `os/user`, `os.UserHomeDir`) are absent. Windows build-tag hygiene is correct: `atomic_windows.go` does not import `renameio`, and `atomic_unix.go` carries `//go:build !windows`.

Only 3 minor/info-level items are worth recording. None block the phase; none require code change before merging.

## Per-SC Checklist

| # | Success Criterion | Evidence | Status |
|---|-------------------|----------|--------|
| 1 | `withLockedState(fn)` performs read→modify→atomic-rename under `LOCK_EX` via `gofrs/flock` + `renameio/v2`; `Get`/`List` acquire `LOCK_SH` | `lock.go:31-53` (Lock), `lock.go:63-82` (RLock); `saveStateFn` → `writeStateAtomic` (renameio on unix) | PASS |
| 2 | JSON state with `version: 1`, monotonic `counter` (never decremented on Purge); mode 0600; unknown `version` → actionable error | `schema.go:19` (`schemaVersion = 1`), `store.go:188` (Counter intentionally not touched), `store.go:217-219` (wrapped `ErrSchemaVersion`), `atomic_unix.go:21` (`0o600 + IgnoreUmask()`) | PASS |
| 3 | Records carry `owner_token`; `Resolve` returns distinct `ErrUnknownID`/`ErrAlreadyResolved`/`ErrNotOwner`; `--force` bypass present | `store.go:96-116` — three sentinels distinctly returned; `subtle.ConstantTimeCompare` used; `opts.Force` short-circuits | PASS |
| 4 | Cross-process test: two processes × 100 Registers → 200 unique IDs; SIGKILL mid-write never corrupts state.json | `integration_test.go:114-158` (`TestStore_TwoProcessesConcurrentRegister`) + `:164-213` (`TestStore_KillMidWriteLeavesCoherentState`) verified byte-identical post-kill | PASS |
| 5 | Windows build uses sibling `state.json.lock`; `MoveFileEx`-backed atomic replace via stdlib `os.Rename` under lock | `lock.go:17-19` (sibling `.lock` unified cross-platform), `atomic_windows.go:19-48` (tmp+fsync+chmod+rename); plan summary confirms `GOOS=windows GOARCH=amd64 go build ./internal/store/...` green | PASS |

## Critical Issues

_None._

## Warnings

_None._

## Info

### IN-01: go.mod pins `renameio/v2 v2.0.1`, not `v2.0.2` as specified in RESEARCH.md / plan

**File:** `go.mod:13`
**Issue:** The plan's frontmatter (`04-01-PLAN.md` line ~64) and STACK.md both specify `github.com/google/renameio/v2 v2.0.2`, but go.mod pins `v2.0.1`. This is NOT a bug — the deviation is explicitly documented in `04-01-SUMMARY.md` (Deviation #1): v2.0.2's go.mod declares `go 1.25`, which would force a toolchain upgrade past the project's pinned `go 1.23.8`. v2.0.1 has the identical public API (`WriteFile`, `IgnoreUmask()`), so no behavioural impact. Worth noting for future-you since CLAUDE.md still claims v2.0.2.
**Fix:** (optional) Either update CLAUDE.md's STACK.md section to reflect v2.0.1 at close-out, or add a one-line comment in go.mod next to the pin, e.g. `// v2.0.2 forces go 1.25; stay on 1.23.8`.

### IN-02: `export_test.go` unconditionally compiled into unit-test binary

**File:** `internal/store/export_test.go:1-21`
**Issue:** The file has no build tag, so `SetWriteDelayForTest` is compiled into every test binary including the plain (non-integration) unit suite. Currently only `integration_test.go` (gated by `//go:build integration && !windows`) calls it. Benign: zero production-binary cost (the `_test.go` suffix confines it to test binaries), and adding a `//go:build integration` tag here would force the same tag on any future unit test that wants the hook. Noted only so a future reader doesn't think the lack of tag is an oversight.
**Fix:** No change required. If desired, add a file-level comment: `// Note: intentionally untagged so future unit tests may also use this hook.`

### IN-03: Resolve accepts empty-string OwnerToken on a record registered with empty token

**File:** `internal/store/store.go:96-116`
**Issue:** If a caller calls `Register("", cond)` (empty token), the record stores `""`. A subsequent `Resolve(id, "", ResolveOptions{})` passes `ConstantTimeCompare("", "")` which returns 1 (match). This is correct constant-time behaviour and likely intentional (Phase 5 always provides a real 32-char hex token generated from `crypto/rand`), but it means the empty-token degenerate case bypasses ownership. Not exploitable given Phase 5's call pattern; flagging only so Phase 5 knows to validate `ownerToken != ""` at the MCP adapter boundary, not at the store boundary.
**Fix:** No change required in `internal/store`. Recommend Phase 5's MCP adapter reject `ownerToken == ""` before calling Register/Resolve — or, if preferred, add `if ownerToken == "" { return ErrNotOwner }` at `store.go:105` (before the subtle compare) with a note in the doc comment. Leave decision for Phase 5 planning.

## Findings Table

| # | Severity | File:Line | Summary | Fix |
|---|----------|-----------|---------|-----|
| IN-01 | Info | go.mod:13 | renameio/v2 pinned to v2.0.1 not v2.0.2 (documented deviation) | Update CLAUDE.md STACK or add inline comment |
| IN-02 | Info | export_test.go:1 | Test hook compiled into all test binaries, not just integration | No action; confined by _test.go suffix |
| IN-03 | Info | store.go:96-116 | Empty OwnerToken matches empty stored token | Validate at Phase 5 adapter boundary |

## Additional Verifications (all PASS)

- **Lock/unlock pairing:** both `withLockedState` and `withSharedLock` use `defer fl.Unlock()` wrappers that preserve the original error via the named-return `err` pattern. No paths can return without releasing the flock (barring SIGKILL, which the OS cleans up).
- **Lock ordering:** `s.mu` always acquired BEFORE `flock` in both helpers — prevents the "two same-process goroutines share a single file-descriptor flock" footgun (`lock.go:32-35`, `lock.go:64-67`).
- **Atomic write correctness (unix):** `renameio.WriteFile` creates temp in the same directory as the target (same-fs rename invariant satisfied), fsyncs, renames, fsyncs parent dir. `IgnoreUmask()` pins final mode to 0600.
- **Atomic write correctness (windows):** temp file created in `filepath.Dir(path)` (same-dir, same-volume → MoveFileEx atomic), written, fsynced, closed, chmodded 0600, renamed. `defer os.Remove(tmpPath)` handles error paths; after successful rename the source is gone so Remove is a no-op. Safe under the flock-protected single-writer invariant.
- **Lock file path:** `statePath + ".lock"` (sibling) — not `.tmp`, not in-place. Correct on both platforms (`lock.go:17-19`).
- **OwnerToken comparison:** `crypto/subtle.ConstantTimeCompare([]byte(caller), []byte(stored)) != 1` — correct idiom (`store.go:106`).
- **Counter monotonicity:** `Purge` touches only `st.Records`; counter is invariant across all three branches (ID / All / Resolved). Verified by `TestStore_PurgeDoesNotDecrementCounter` asserting on-disk counter and that post-purge Register yields `idgen.Allocate(3)`, not `idgen.Allocate(0)`.
- **Schema version:** `loadState` returns wrapped `ErrSchemaVersion` with both versions and file path. Verified by `TestStore_SchemaVersionMismatchErrors`.
- **`resolved_at` null marshalling:** `*time.Time` pointer; nil marshals as JSON `null`. Verified by `TestStore_ResolvedAtNullInJSON` asserting the literal `"resolved_at": null` substring.
- **Stdout discipline:** `grep -rn 'fmt.Print\|log.Print\|println\|fmt.Fprintln' internal/store/` returns nothing.
- **Forbidden imports:** no `net/http`, `os/user`, `os.UserHomeDir` anywhere in `internal/store/`.
- **Build-tag hygiene:** `atomic_windows.go` imports only `fmt`, `os`, `path/filepath` — NO `renameio`. `atomic_unix.go` has `//go:build !windows`. `integration_test.go` has `//go:build integration && !windows`.
- **TestMain dispatch:** re-exec pattern uses env var `MCP_CHAIN_STORE_TEST_CHILD`; children redirect stdout to parent's `os.Stderr` preserving MCP-02 stdout discipline (`integration_test.go:131-132`, `:189-190`).
- **Test isolation:** every test uses `t.TempDir()`; no shared state.
- **Error wrapping:** sentinels wrapped with `%w` throughout (`store.go:211`, `:215`, `:218`, `:232`, `:235`; `lock.go:37,41,69,73`; `atomic_unix.go:22`; `atomic_windows.go:23,32,36,39,42,45`).
- **Exported types have doc comments:** `Store`, `Record`, `PurgeOptions`, `ResolveOptions`, `Open`, `Register`, `Resolve`, `Get`, `List`, `Purge` — all have doc comments starting with the type/function name.
- **Sentinel error messages consistent:** all six prefix with `store: `.

---

_Reviewed: 2026-04-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
