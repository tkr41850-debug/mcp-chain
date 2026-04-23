---
phase: 04-store-core-flock-atomic-writes
verified: 2026-04-23T00:00:00Z
status: passed
score: 5/5 success-criteria verified
overrides_applied: 0
---

# Phase 4: Store Core, Flock & Atomic Writes — Verification Report

**Phase Goal:** `internal/store` hexagonal core is complete, correct under concurrent cross-process load, exposes OwnerToken, and persists a versioned JSON file under flock + atomic rename.
**Verified:** 2026-04-23
**Status:** PASS
**Re-verification:** No — initial verification

## Success Criteria Verdicts

### SC1 — `withLockedState` under `LOCK_EX`; `Get`/`List` under `LOCK_SH`

**Verdict: PASS**

Evidence:
- `internal/store/lock.go:31-53` — `withLockedState(fn)`: `fl.Lock()` (LOCK_EX) → `loadState` → `fn(st)` → `saveStateFn` (atomic rename). Mutex taken first, flock second.
- `internal/store/lock.go:63-82` — `withSharedLock(fn)`: `fl.RLock()` (LOCK_SH) → `loadState` → `fn(st)`. Get/List call this (`store.go:122, 138`).
- `gofrs/flock` `RLock()` maps to `LOCK_SH` on Unix (POSIX `flock(2)`).
- Cross-process readers do NOT serialize (each process has its own `s.mu`; shared flock allows N holders). Same-process goroutine readers DO serialize on `s.mu` — documented in `lock.go:60-62` as an intentional decision (RESEARCH §Open Questions #2). This matches the plan's decision "sync.Mutex taken on BOTH read and write paths (same-process serialisation)".
- Test: `TestStore_Get` (store_test.go:237), `TestStore_List` (store_test.go:257) — pass under `-race`.

### SC2 — Schema `version:1` + monotonic `counter`; mode 0600; parent 0700; unknown version errors

**Verdict: PASS**

Evidence:
- Version constant: `schema.go:19` — `const schemaVersion = 1`.
- JSON shape: `schema.go:23-38` — `{version, counter, records{id → record}}` with `owner_token`, timestamps RFC3339 (`time.Time` default), `ResolvedAt *time.Time` so `null` round-trips (verified by `TestStore_ResolvedAtNullInJSON`).
- Counter monotonic: `store.go:72-73` — `Allocate(counter)` then `counter++` in Register. `Purge` at `store.go:152-195` touches Records only; the code comment at `:188` says "Counter is intentionally NOT touched (CORE-09)". Verified by `TestStore_PurgeDoesNotDecrementCounter` (store_test.go:360) — asserts counter=3 on disk after Purge-all of 3 and that the next Register returns `idgen.Allocate(3)`, not `"acid"`.
- File mode 0600: `atomic_unix.go:21` — `renameio.WriteFile(path, data, 0o600, renameio.IgnoreUmask())`. Verified by `TestStore_StateFileMode0600` (store_test.go:112) — `os.Stat(...).Mode().Perm() == 0o600`.
- Parent dir 0700: enforced in Phase 3 (`internal/statepath/resolve.go:68`) by `os.MkdirAll(parent, 0o700)`. Correct hand-off boundary per CONTEXT.
- Unknown version error: `store.go:217-219` — wraps `ErrSchemaVersion` with path, got-version, want-version. Verified by `TestStore_SchemaVersionMismatchErrors` (store_test.go:85) — asserts `errors.Is(err, ErrSchemaVersion)` + error contains path and "version 2".

### SC3 — `owner_token` on Register; distinct sentinel errors on Resolve; `--force` bypass

**Verdict: PASS**

Evidence:
- `owner_token` field: `schema.go:35` — `OwnerToken string \`json:"owner_token"\``, populated at `store.go:78`. Verified by `TestStore_RegisterStoresOwnerToken` (store_test.go:146) which reads the raw JSON.
- Sentinel errors: `errors.go:11, 16, 21, 26, 31, 36` — `ErrUnknownID`, `ErrAlreadyResolved`, `ErrNotOwner`, `ErrSchemaVersion`, `ErrCorruptJSON`, `ErrPurgeArgRequired`.
- Resolve branches `store.go:96-116`:
  - Unknown ID (`:99-101`) → `ErrUnknownID`. Test: `TestStore_ResolveUnknownIDReturnsErr:204`.
  - Already resolved (`:102-104`) → `ErrAlreadyResolved`. Test: `TestStore_ResolveAlreadyResolvedReturnsErr:193`.
  - Wrong owner (`:105-109`) → `ErrNotOwner`. Test: `TestStore_ResolveWrongOwnerReturnsErrNotOwner:171`.
  - Happy path sets `Status=resolved`, `ResolvedAt=now`. Tests: `TestStore_ResolveOwnerOk:159`, `TestStore_ResolveSetsResolvedAt:211`.
- `--force` bypass: `ResolveOptions{Force bool}` at `store.go:48-50`; gate at `store.go:105` — `if !opts.Force { ... }`. Verified by `TestStore_ResolveForceBypassesOwnerCheck:181` — wrong token succeeds when `Force: true`.
- Constant-time compare: `store.go:4` imports `crypto/subtle`; `store.go:106` uses `subtle.ConstantTimeCompare`. All 4 Resolve branches are test-covered.

### SC4 — Two-process integration test: 200 unique IDs + no corrupt JSON after SIGKILL

**Verdict: PASS**

Evidence (executed):

```
$ go test -race -count=1 -tags=integration -timeout=120s -v \
    -run 'TestStore_TwoProcesses|TestStore_KillMidWrite' ./internal/store/...
=== RUN   TestStore_TwoProcessesConcurrentRegister
--- PASS: TestStore_TwoProcessesConcurrentRegister (39.57s)
=== RUN   TestStore_KillMidWriteLeavesCoherentState
--- PASS: TestStore_KillMidWriteLeavesCoherentState (1.17s)
PASS
ok  github.com/anthropics/mcp-chain/internal/store  43.106s
```

- `integration_test.go:114-158` — two `os.Executable()` subprocesses each register 100 entries; asserts `len(records) == 200` and all IDs unique via map dedup.
- `integration_test.go:164-213` — seeds state.json, spawns slow-write child (hooks `saveStateFn` with 5s sleep), sleeps 1s, SIGKILLs child, asserts state.json is byte-identical to seed and still valid JSON.
- Re-exec via `TestMain` (integration_test.go:36) + `childEnvVar` dispatch, no Phase 6 CLI dependency — as planned.

### SC5 — Windows cross-compile succeeds; sibling lock file

**Verdict: PASS**

Evidence (executed):

```
$ GOOS=windows GOARCH=amd64 go build ./internal/store/...
(empty) → BUILD_EXIT=0
$ GOOS=windows GOARCH=amd64 go vet ./internal/store/...
(empty) → VET_EXIT=0
```

- `atomic_windows.go:1` — `//go:build windows`; uses stdlib `os.CreateTemp` + `tmp.Sync()` + `os.Chmod(0o600)` + `os.Rename` (MoveFileEx on Windows).
- `lock.go:17-19` — `lockFilePath` returns `statePath + ".lock"` (sibling `state.json.lock`). Used on BOTH platforms (uniform semantics per plan decision).
- `renameio` NOT imported in `atomic_windows.go` (grep of `_windows.go` files returns nothing). Imports in `atomic_unix.go:8` only.
- Runtime Windows behavior deferred to Phase 9 CI matrix (VALIDATION.md Manual-Only row 1).

## Extra-Gate Results

| Gate | Command | Result |
|------|---------|--------|
| Full race suite | `go test -race -count=1 ./...` | PASS: all 5 packages green (cmd/mcp-chain, internal/cli, internal/idgen, internal/statepath, internal/store) |
| `go vet ./...` | — | PASS (exit 0, empty output) |
| Stdout leak scan | `grep 'fmt\.Print\|log\.Print' internal/store/*.go` | PASS (exit 1, no matches — no test file matches either) |
| `crypto/subtle` used | `grep subtle.ConstantTimeCompare internal/store/*.go` | PASS — `store.go:106` uses `subtle.ConstantTimeCompare`; `store.go:4` imports it |
| `gofrs/flock` pin | `grep 'gofrs/flock v0.12' go.mod` | PASS — `go.mod:12` pins `v0.12.1` (not v0.13) |
| `renameio/v2` build-tag confinement | `grep -l renameio internal/store/*_windows.go` | PASS (no matches in `_windows.go`; only `atomic_unix.go` imports it — schema.go/store_test.go matches are doc comments/test strings, not imports) |

Noted deviation (already documented in 04-01-SUMMARY.md "Auto-fixed Issues"):
- `renameio/v2` pinned at **v2.0.1** (not v2.0.2 as RESEARCH prescribed) because v2.0.2's go.mod forces Go 1.25. v2.0.1 has identical public API. Go directive unchanged at 1.23.8. Acceptable.

## Requirements Coverage

| Requirement | Evidence | Status |
|-------------|----------|--------|
| CORE-04 (Register/Resolve/Get/List/Purge under flock) | 14 unit tests + same-process goroutine test + two-process integration test | SATISFIED |
| CORE-05 (schema v1, mode 0600, counter monotonic, RFC3339, unknown-version error) | 6 unit tests | SATISFIED |
| CORE-08 (OwnerToken stored + constant-time compare + sentinel errors + Force bypass) | 7 unit tests + `crypto/subtle` import/usage | SATISFIED |
| CORE-09 (Purge ID/All/Resolved + counter invariant + target-required) | 6 unit tests | SATISFIED |

No orphaned requirements (REQUIREMENTS.md Phase 4 row = {CORE-04, CORE-05, CORE-08, CORE-09}, all covered).

## Anti-Patterns Scan

No blockers, warnings, or info items found in `internal/store/*.go`. No TODO/FIXME/placeholder comments; no empty `return nil`/`return []`/`return {}` hollow paths in production code; no `console.log`-style debug residue; no stubs. Test files are self-contained and only override `saveStateFn` via the `export_test.go` hook.

## Human Verification Required

None. All phase behaviors are programmatically verified; Windows runtime (actual `LockFileEx` + `MoveFileEx`) is explicitly deferred to Phase 9 CI matrix per 04-VALIDATION.md Manual-Only row 1, which is acceptable.

## Overall Verdict

**PHASE COMPLETE**

All 5 success criteria PASS with direct code + test evidence. Integration suite green (`TestStore_TwoProcessesConcurrentRegister` 39.57s; `TestStore_KillMidWriteLeavesCoherentState` 1.17s). Full repo race suite green. Windows cross-compile + vet green. All extra gates pass. No blockers.

Single minor deviation (`renameio/v2` v2.0.1 vs v2.0.2) is documented, justified (Go 1.25 toolchain bump avoided), and API-compatible.

---

_Verified: 2026-04-23_
_Verifier: Claude (gsd-verifier)_
