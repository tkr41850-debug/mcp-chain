---
phase: 4
slug: store-core-flock-atomic-writes
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-23
---

# Phase 4 — Validation Strategy

> Per-phase validation contract. Content derived from `04-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| **Config file** | none |
| **Quick run command** | `go test -race ./internal/store/...` |
| **Full suite command** | `go test -race -count=1 -timeout=60s ./...` |
| **Integration command** | `go test -race -count=1 -tags=integration -timeout=120s ./internal/store/...` |
| **Windows cross-compile gate** | `GOOS=windows GOARCH=amd64 go build ./internal/store/...` |
| **Estimated runtime** | ~3 s unit; ~30 s integration; ~10 s full race suite |

---

## Sampling Rate

- **After every task commit:** `go test -race ./internal/store/...` (~3 s; unit only)
- **After every plan wave:** `go test -race -count=1 -tags=integration ./internal/store/...` (~30 s) + Windows cross-compile gate
- **Before `/gsd-verify-work`:** Full race suite (`go test -race -count=1 ./...`) + integration + Windows cross-compile + `go vet` + `make lint`
- **Max feedback latency:** <5 s per-commit (unit tests only; integration deferred to wave-end)

---

## Per-Task Verification Map

| Task ID | Wave | Requirement | SC | Test / Gate | Type | Automated Command | Status |
|---------|------|-------------|----|-------------|------|-------------------|--------|
| 4-01 | 1 | CORE-04 | 1 | `TestStore_OpenNoIO` — `Open()` performs no IO beyond stat | unit | `go test -race -run TestStore_OpenNoIO ./internal/store` | ⬜ |
| 4-01 | 1 | CORE-04 | 1 | `TestStore_RegisterAllocatesFirstWord` — first ID is `words[0]` | unit | `go test -race -run TestStore_RegisterAllocatesFirstWord ./internal/store` | ⬜ |
| 4-01 | 1 | CORE-05 | 2 | `TestStore_RegisterMonotonicCounter` — counter increments each Register | unit | `go test -race -run TestStore_RegisterMonotonicCounter ./internal/store` | ⬜ |
| 4-01 | 1 | CORE-05 | 2 | `TestStore_SchemaVersionMismatchErrors` — unknown version returns `ErrSchemaVersion` | unit | `go test -race -run TestStore_SchemaVersion ./internal/store` | ⬜ |
| 4-01 | 1 | CORE-05 | 2 | `TestStore_CorruptJSONErrors` — garbage file returns actionable error | unit | `go test -race -run TestStore_CorruptJSON ./internal/store` | ⬜ |
| 4-01 | 1 | CORE-05 | 2 | `TestStore_StateFileMode0600` — state.json mode is 0600 | unit | `go test -race -run TestStore_FileMode ./internal/store` | ⬜ |
| 4-01 | 1 | CORE-05 | 2 | `TestStore_ResolvedAtNullInJSON` — `resolved_at: null` round-trips | unit | `go test -race -run TestStore_ResolvedAtNull ./internal/store` | ⬜ |
| 4-01 | 1 | CORE-05 | — | `TestStore_LoadMissingStateReturnsEmpty` — missing file yields v1/counter 0 | unit | `go test -race -run TestStore_LoadMissing ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `TestStore_RegisterStoresOwnerToken` — token persisted on record | unit | `go test -race -run TestStore_RegisterStoresOwnerToken ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `TestStore_ResolveOwnerOk` — matching token resolves | unit | `go test -race -run TestStore_ResolveOwnerOk ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `TestStore_ResolveWrongOwnerReturnsErrNotOwner` — mismatch → `ErrNotOwner` | unit | `go test -race -run TestStore_ResolveWrongOwner ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `TestStore_ResolveForceBypassesOwnerCheck` — `Force: true` bypasses | unit | `go test -race -run TestStore_ResolveForce ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `TestStore_ResolveAlreadyResolvedReturnsErr` — idempotency guard | unit | `go test -race -run TestStore_ResolveAlreadyResolved ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `TestStore_ResolveUnknownIDReturnsErr` — distinct `ErrUnknownID` | unit | `go test -race -run TestStore_ResolveUnknown ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `TestStore_ResolveSetsResolvedAt` — sets timestamp and status | unit | `go test -race -run TestStore_ResolveSetsResolvedAt ./internal/store` | ⬜ |
| 4-02 | 1 | CORE-08 | 3 | `crypto/subtle.ConstantTimeCompare` used for token compare | smoke | `grep -q 'subtle.ConstantTimeCompare' internal/store/store.go` | ⬜ |
| 4-03 | 1 | CORE-04 | 1 | `TestStore_GetReturnsRecord` — Get uses LOCK_SH | unit | `go test -race -run TestStore_Get$ ./internal/store` | ⬜ |
| 4-03 | 1 | CORE-04 | 1 | `TestStore_GetUnknownIDReturnsErr` — returns `ErrUnknownID` | unit | `go test -race -run TestStore_GetUnknown ./internal/store` | ⬜ |
| 4-03 | 1 | CORE-04 | 1 | `TestStore_ListReturnsAllRecords` — List returns slice copy | unit | `go test -race -run TestStore_List$ ./internal/store` | ⬜ |
| 4-03 | 1 | CORE-04 | 1 | `TestStore_ListEmptyWhenNoState` — nil state yields empty slice | unit | `go test -race -run TestStore_ListEmpty ./internal/store` | ⬜ |
| 4-04 | 1 | CORE-09 | 2 | `TestStore_PurgeByID` — removes one record | unit | `go test -race -run TestStore_PurgeByID ./internal/store` | ⬜ |
| 4-04 | 1 | CORE-09 | 2 | `TestStore_PurgeAll` — removes all | unit | `go test -race -run TestStore_PurgeAll ./internal/store` | ⬜ |
| 4-04 | 1 | CORE-09 | 2 | `TestStore_PurgeResolved` — removes only resolved | unit | `go test -race -run TestStore_PurgeResolved ./internal/store` | ⬜ |
| 4-04 | 1 | CORE-09 | 2 | `TestStore_PurgeDoesNotDecrementCounter` — monotonic invariant | unit | `go test -race -run TestStore_PurgeDoesNotDecrement ./internal/store` | ⬜ |
| 4-04 | 1 | CORE-09 | — | `TestStore_PurgeRequiresTarget` — `PurgeOptions{}` zero → error | unit | `go test -race -run TestStore_PurgeRequires ./internal/store` | ⬜ |
| 4-05 | 2 | CORE-04 | 1, 4 | `TestStore_SameProcessGoroutineConcurrency` — N goroutines one process | unit (race) | `go test -race -run TestStore_SameProcess ./internal/store` | ⬜ |
| 4-05 | 2 | CORE-04 | 1, 4 | `TestStore_TwoProcessesConcurrentRegister` — 2×100 concurrent → 200 unique | integration | `go test -race -tags=integration -run TestStore_TwoProcesses ./internal/store` | ⬜ |
| 4-05 | 2 | CORE-04 | 4 | `TestStore_KillMidWriteLeavesCoherentState` — SIGKILL → valid JSON | integration | `go test -tags=integration -run TestStore_KillMidWrite ./internal/store` | ⬜ |
| 4-06 | 2 | CORE-04 | 5 | Windows cross-compile green | smoke | `GOOS=windows GOARCH=amd64 go build ./internal/store/...` | ⬜ |
| 4-06 | 2 | CORE-04 | 5 | Windows vet green | smoke | `GOOS=windows GOARCH=amd64 go vet ./internal/store/...` | ⬜ |
| 4-06 | 2 | CORE-04 | 5 | Sibling `state.json.lock` used (not state.json directly) | smoke | `grep -q 'state.json.lock\|.lock' internal/store/lock.go` | ⬜ |
| 4-06 | 2 | CORE-04 | 5 | `renameio/v2` only imported under `!windows` | smoke | `! grep -l 'renameio' internal/store/*_windows.go 2>/dev/null` | ⬜ |
| 4-06 | 2 | CORE-04 | 5 | `gofrs/flock` pinned v0.12.1 (not v0.13+) | smoke | `grep -q 'github.com/gofrs/flock v0.12' go.mod` | ⬜ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs (4-01 … 4-06) map to plan waves: exact plan IDs finalized by planner; this table provides the sampling contract per logical task group.*

---

## Wave 0 Requirements

- [ ] `go.mod` additions: `github.com/gofrs/flock v0.12.1`, `github.com/google/renameio/v2 v2.0.2`
- [ ] `internal/store/schema.go` — `state`, `record`, `version` constant
- [ ] `internal/store/errors.go` — `ErrUnknownID`, `ErrAlreadyResolved`, `ErrNotOwner`, `ErrSchemaVersion`
- [ ] `internal/store/store.go` — `Open`, `Register`, `Resolve(id, token string, force bool)`, `Get`, `List`, `Purge(PurgeOptions)`
- [ ] `internal/store/lock.go` — `withLockedState`, `withSharedLock`, sibling `.lock` file path helper
- [ ] `internal/store/atomic_unix.go` — `writeAtomic` via `renameio.WriteFile(…, IgnoreUmask())` (`//go:build !windows`)
- [ ] `internal/store/atomic_windows.go` — `writeAtomic` via stdlib `os.Rename` + `os.Chmod 0600` (`//go:build windows`)
- [ ] `internal/store/store_test.go` — all unit tests above
- [ ] `internal/store/integration_test.go` (build tag `integration`) — `TestMain` re-exec + two-process + kill-mid-write

**Framework install:** None beyond `go.mod` dep additions.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows runtime behavior (actual `LockFileEx` + `MoveFileEx`) | SC #5 | Requires Windows runner | Deferred to Phase 9 CI matrix (`GOOS=windows` runner runs non-race unit suite per QA-03) |
| NFS lock semantics | — | NFS not supported per PROJECT.md | Documented as unsupported in Phase 10 docs |
| Kill-9 with real system crash (not SIGKILL) | SC #4 | Cannot simulate power-loss in CI | SIGKILL mid-write approximates; atomic-rename guarantee covers power-loss semantics (fsync before rename in `renameio/v2`) |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have automated verify or Wave 0 dependencies
- [x] Sampling continuity: every test maps to a commit-level verify
- [x] Wave 0 self-contained (no cross-phase prerequisites beyond Phase 2 `idgen` + Phase 3 `statepath`)
- [x] No watch-mode flags
- [x] Feedback latency < 60 s (<5 s unit; wave-end integration acceptable at ~30 s)
- [x] `nyquist_compliant: true` set
- [x] All 5 SC mapped to ≥1 test (SC 1→unit+integration, SC 2→6 unit, SC 3→7 unit, SC 4→integration×2, SC 5→cross-compile+smoke)

**Approval:** approved 2026-04-23
