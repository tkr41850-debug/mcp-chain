---
phase: 04
plan: 01
subsystem: internal/store
tags: [core, flock, atomic-write, state-persistence, owner-token]
dependency-graph:
  requires: [internal/idgen, internal/statepath]
  provides: [internal/store]
  affects: [Phase 5 MCP server, Phase 7 CLI]
tech-stack:
  added:
    - github.com/gofrs/flock v0.12.1
    - github.com/google/renameio/v2 v2.0.1  # DEVIATION: pinned 2.0.1 not 2.0.2
    - golang.org/x/sys v0.22.0 (indirect via flock)
  patterns:
    - closure-scoped lock helpers (withLockedState/withSharedLock)
    - build-tagged platform split (atomic_unix.go / atomic_windows.go)
    - constant-time token compare (crypto/subtle)
    - re-exec integration tests (TestMain dispatch via env var)
    - function-pointer test injection (saveStateFn + export_test.go)
key-files:
  created:
    - internal/store/schema.go
    - internal/store/errors.go
    - internal/store/store.go
    - internal/store/lock.go
    - internal/store/atomic_unix.go
    - internal/store/atomic_windows.go
    - internal/store/export_test.go
    - internal/store/store_test.go
    - internal/store/integration_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - Pin gofrs/flock v0.12.1 (v0.13 requires Go 1.24; project on 1.23.8)
  - DEVIATION — pin renameio/v2 v2.0.1 (v2.0.2 requires Go 1.25; v2.0.1 is the newest tag compatible with Go 1.23.8)
  - Sibling state.json.lock on BOTH platforms (uniform semantics)
  - sync.Mutex taken on BOTH read and write paths (same-process serialisation)
  - crypto/subtle.ConstantTimeCompare for OwnerToken equality
  - saveStateFn function-pointer injection via export_test.go (no env vars)
metrics:
  duration: ~25 min
  completed: 2026-04-23
---

# Phase 4 Plan 01: Store Core, Flock & Atomic Writes Summary

## Executive Summary

`internal/store` — the SDK-agnostic hexagonal core — persists state.json
under cross-process `gofrs/flock` with crash-safe atomic writes
(`renameio/v2` on POSIX, stdlib rename under lock on Windows), exposes
`Register`/`Resolve`/`Get`/`List`/`Purge` with sentinel errors, and
enforces OwnerToken-based ownership with a `--force` bypass; 28 tests
green under `-race` on Linux + Windows cross-compile.

## Files Created (9)

```
internal/store/
├── schema.go            (45 LOC)  state/record types + schemaVersion=1 + status enum
├── errors.go            (36 LOC)  six sentinel errors (errors.Is compatible)
├── store.go            (260 LOC)  public API + load/save + recordToRecord
├── lock.go              (82 LOC)  withLockedState + withSharedLock + .lock helper
├── atomic_unix.go       (25 LOC)  //go:build !windows — renameio.WriteFile
├── atomic_windows.go    (48 LOC)  //go:build windows — tmp + fsync + chmod + os.Rename
├── export_test.go       (21 LOC)  SetWriteDelayForTest hook (confined to test binary)
├── store_test.go       (437 LOC)  26 unit tests (CORE-04/05/08/09 + same-process concurrency)
└── integration_test.go (213 LOC)  //go:build integration && !windows — 2 cross-process tests
```

Total: 1167 LOC (~545 production, ~622 test).

## go.mod diff

```diff
 require (
   github.com/alecthomas/kong v1.15.0
   github.com/stretchr/testify v1.11.1
 )

 require (
   github.com/davecgh/go-spew v1.1.1 // indirect
+  github.com/gofrs/flock v0.12.1 // indirect
+  github.com/google/renameio/v2 v2.0.1 // indirect
   github.com/kr/pretty v0.3.1 // indirect
   github.com/pmezard/go-difflib v1.0.0 // indirect
+  golang.org/x/sys v0.22.0 // indirect  (transitive via flock)
   gopkg.in/check.v1 v1.0.0-... // indirect
   gopkg.in/yaml.v3 v3.0.1 // indirect
 )
```

`go` directive unchanged at `1.23.8`.

## Test Results

### Unit + integration + race (plan verify gate)

```
$ go test -race -count=1 -tags=integration -timeout=120s ./internal/store/...
ok  github.com/anthropics/mcp-chain/internal/store  49.398s
```

- **Unit tests (26)** — all rows 4-01…4-04 plus extra Purge-unknown-id
  and the Task 2.1 same-process goroutine concurrency test. All green
  under `-race`.
- **Integration tests (2)** — `TestStore_TwoProcessesConcurrentRegister`
  (41.86s: 2 processes × 100 Registers → 200 unique IDs) and
  `TestStore_KillMidWriteLeavesCoherentState` (1.12s: SIGKILL mid-write
  → state.json byte-identical to seed, still valid JSON).

### Windows cross-compile

```
$ GOOS=windows GOARCH=amd64 go build ./internal/store/...
(empty = success)

$ GOOS=windows GOARCH=amd64 go vet ./internal/store/...
(empty = success)
```

### Full repo race suite

```
$ go test -race -count=1 -timeout=60s ./...
ok  github.com/anthropics/mcp-chain/cmd/mcp-chain      14.374s
ok  github.com/anthropics/mcp-chain/internal/cli       14.011s
ok  github.com/anthropics/mcp-chain/internal/idgen      1.872s
ok  github.com/anthropics/mcp-chain/internal/statepath  1.845s
ok  github.com/anthropics/mcp-chain/internal/store      4.810s
```

### Smoke checks (all green)

| Check | Command | Result |
|-------|---------|--------|
| ConstantTimeCompare used | `grep subtle.ConstantTimeCompare internal/store/store.go` | OK |
| Sibling `.lock` | `grep '.lock' internal/store/lock.go` | OK |
| renameio quarantine | `grep -l renameio internal/store/*_windows.go` | empty (OK) |
| flock v0.12 pin | `grep 'github.com/gofrs/flock v0.12' go.mod` | OK |
| stdout discipline | `! grep -E 'fmt.Print\|log.Print' internal/store/*.go` | OK |
| gofmt clean | `gofmt -l ./internal/store/` | empty (OK) |
| `go vet ./...` | | OK |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Pinned `renameio/v2` to v2.0.1 instead of v2.0.2**

- **Found during:** Task 0.1 (`go get github.com/google/renameio/v2@v2.0.2`)
- **Issue:** v2.0.2's `go.mod` declares `go 1.25`, which forced the
  toolchain to auto-upgrade and bumped our project's `go 1.23.8` to
  `go 1.25` on first `go get`. RESEARCH.md claimed v2.0.2 had a Go
  1.17 floor — that was wrong; the tag actually bumped to 1.25.
- **Fix:** Pinned to `v2.0.1` instead (`go.mod` declares `go 1.13`;
  full API compatibility). Restored `go 1.23.8` directive.
- **Rationale:** CLAUDE.md pins the stack to Go 1.23.8; the plan's own
  Task 0.1 deviation handling says "DO NOT bump go.mod's `go 1.23.8`
  directive". v2.0.1 has identical public API (`WriteFile`,
  `IgnoreUmask()`), so no code changes required.
- **Files affected:** `go.mod`, `go.sum`
- **Commit:** `30b4e13`

### Other deviations

**2. [Rule 3 — Blocking] Wave 3 `make lint` unavailable**

- **Found during:** Wave 3 gate (`make lint` failed with
  `golangci-lint: command not found`).
- **Issue:** Environment does not have `golangci-lint` binary; install
  from source requires ~5+ min of compilation time and network access
  to goproxy, which while available was too slow to include in the
  plan execution window. Attempts to install in the background did not
  finish within the available execution window.
- **Fix:** Applied the plan's documented fallback
  (`Task 3.1 Deviation handling`): "If `make lint` is not present,
  skip and run `go vet ./...` + `gofmt -l ./internal/store/`". Both
  green.
- **No code change required.** Final lint run should be part of CI
  (already wired via `.golangci.yml` + GitHub Actions in Phase 1).

**3. Task collapse: 1.2 → 1.3 → 1.4 + 1.5 → 2.1**

- **Found during:** Task 1.2 write.
- **Issue:** Task 1.2's `store.go` stubs reference `writeStateAtomic`
  (from Task 1.3) in `saveState`; without 1.3 landed, `go build` fails
  on Task 1.2's verify.
- **Fix:** Followed the plan's own alternative — "Ship these two files
  together with Task 1.2 to avoid the unused-type warning". Each task
  still got its own commit in proper order (f33cb63 / f931c6c /
  da16796 / 3d57fc1), but Task 1.2's commit intentionally contained
  `not implemented` stubs so it could build alone if reverted. Task
  2.1's `TestStore_SameProcessGoroutineConcurrency` was combined into
  the Task 1.5 commit (same test file, one logical unit of test-suite
  state).
- **No plan-contract impact.** VALIDATION.md row 4-05 mapping
  unchanged.

## Authentication Gates

None — pure library phase.

## Test Count by Requirement

| Requirement | Tests | Gate |
|-------------|-------|------|
| CORE-04 (Register/Resolve/Get/List) | 14 unit + 1 same-process + 1 two-process integration | ✅ |
| CORE-05 (schema v1 + 0600 mode + RFC3339 + counter monotonic) | 6 unit | ✅ |
| CORE-08 (OwnerToken semantics + Force bypass) | 7 unit | ✅ |
| CORE-09 (Purge variants + counter invariant) | 6 unit | ✅ |
| SC #4 (no corrupt state under kill-9) | 1 integration | ✅ |
| SC #5 (Windows cross-compile) | 1 smoke | ✅ |

## Commits (7 per-task + this summary)

```
30b4e13 chore(04-01): add gofrs/flock v0.12.1 + renameio/v2 v2.0.1
593d6fc feat(04-01): add store schema types + sentinel errors
f33cb63 feat(04-01): add store.go skeleton (Open + load/save + stubs)
f931c6c feat(04-01): add lock.go + build-tagged atomic writers
da16796 feat(04-01): wire Register/Resolve/Get/List/Purge
3d57fc1 test(04-01): add store unit tests (CORE-04/05/08/09)
4d1d9d7 test(04-01): add cross-process integration tests (SC #4)
```

## Pointer for Phase 5 (MCP server)

The MCP server will generate a per-process OwnerToken at startup via:

```go
var buf [16]byte
if _, err := rand.Read(buf[:]); err != nil {
    // handle
}
token := hex.EncodeToString(buf[:])  // 32-char lowercase hex
```

This token is stamped on every `store.Register` call and required by
every `store.Resolve(id, token, ResolveOptions{})` call. On server
restart, the token is regenerated — so a restarted server CANNOT
resolve sessions the previous instance registered (by design: matches
user intent per STACK.md §MCP Session Identity). Users who need to
resolve from a different process use the CLI with `--force` (Phase 7).

Key API the MCP adapter will call:

```go
path, err := statepath.Resolve()
s, err := store.Open(path)
id, err := s.Register(ownerToken, condition)
err := s.Resolve(id, ownerToken, store.ResolveOptions{Force: false})
rec, err := s.Get(id)
recs, err := s.List()
n, err := s.Purge(store.PurgeOptions{Resolved: true})
```

## Known Stubs

None.

## Self-Check: PASSED
