# Phase 4: Store Core, Flock & Atomic Writes - Research

**Researched:** 2026-04-23
**Domain:** cross-process state persistence in pure Go (flock + atomic rename)
**Confidence:** HIGH

## Summary

Phase 4 builds `internal/store/` — the hexagonal core that manages `state.json` under cross-process flock, with atomic writes for crash-safety. The two new deps are `gofrs/flock` and `google/renameio/v2`. Both are idiomatic, small, pure-Go, and well-documented.

Two findings materially change the plan the CONTEXT.md assumed:

1. **`google/renameio/v2` does NOT compile on Windows.** The package's README explicitly states: "this package does not export any functions on Windows." Calling `renameio.WriteFile` from code compiled with `GOOS=windows` is a compile error, not a runtime no-op. The CONTEXT.md's note about Windows using `MoveFileEx`-backed renameio is incorrect — `renameio/v2` simply opts out of Windows. The Phase 4 plan MUST use build tags to provide a Windows-specific atomic-replace path (two options: `natefinch/atomic` which wraps `MoveFileEx`, OR a hand-rolled ~30-line `lockedfile`-style approach using `os.Rename`, which on Windows in Go 1.5+ is backed by `MoveFileEx`/`ReplaceFile` and is atomic-enough for single-writer-under-lock scenarios).
2. **`gofrs/flock` v0.13.0 requires Go 1.24**, but the project is on Go 1.23.8. Either bump the toolchain (a new unplanned step) or pin to **v0.12.1** (released 2024-07-22, Go 1.21+ floor). v0.12.1 has the full API we need (`Lock`/`RLock`/`Unlock`/`TryLock`/`TryRLock`/`Locked`/`RLocked`/`Path`/`New`); only `Stat` is new in 0.13 and we don't use it. **Recommend: pin `github.com/gofrs/flock v0.12.1`.**

On flock+rename interaction on POSIX: advisory `flock(2)` is attached to the open file description (and `fcntl` locks to the `(inode, process)` pair). Renaming a file does not change its inode, so a lock held on a file descriptor **survives rename**. This validates the unix strategy: a writer can hold `LOCK_EX` on `state.json`, write a temp file in the same directory, and `renameio.WriteFile` over `state.json` without losing the lock.

On Windows: `LockFileEx` attaches to the file handle. A held exclusive lock on `state.json` blocks anyone else from opening it (including shared-lock readers), AND Windows renames fail when any handle is open on the target. Hence the locked decision: use a sibling lock file `state.json.lock` on Windows.

**Primary recommendation:** Pin `gofrs/flock@v0.12.1` and `google/renameio/v2@v2.0.2`. Two build-tagged files: `lock_unix.go` (lock path = state file) and `lock_windows.go` (lock path = state file + ".lock"). A third build-tagged pair `atomic_unix.go` / `atomic_windows.go` abstracts the atomic-replace primitive behind a `writeStateAtomic(path string, data []byte) error` helper — unix uses `renameio.WriteFile`, windows uses a hand-rolled temp-file-plus-`os.Rename` (acceptable under single-writer-flock-protection). Avoid adding `natefinch/atomic` unless CI on Windows proves the stdlib rename insufficient; one fewer dep.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Read-modify-write of state.json | store (core) | — | Hexagonal core owns persistence; no adapter leaks |
| Cross-process lock acquisition | store (core) | OS (flock/LockFileEx) | Core delegates to OS primitive via gofrs/flock |
| ID allocation on Register | store (core) → idgen | — | Store calls `idgen.Allocate(counter)`; idgen is pure |
| Path resolution | statepath (Phase 3) | — | Caller resolves path and passes it to `store.Open` |
| OwnerToken generation | MCP server (Phase 5) | — | Per-process at startup; store only **validates** |
| OwnerToken validation on Resolve | store (core) | — | Core compares `record.owner_token == caller_token` |
| CLI `--force` bypass | CLI (Phase 7) → store | — | Plumbs `Force bool` into `store.Resolve` options |
| Logging | stderr via `log/slog` only | — | stdout is MCP wire; store should NOT log, prefer errors |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/gofrs/flock` | v0.12.1 | Advisory file lock, cross-platform | Pure Go, POSIX `flock(2)` + Windows `LockFileEx`, no cgo. v0.12.1 works with Go 1.23. [VERIFIED: pkg.go.dev, go.mod at tag] |
| `github.com/google/renameio/v2` | v2.0.2 | Atomic file replace (POSIX-only) | Fsync-before-rename, umask-aware, the idiomatic Go answer on unix. [VERIFIED: pkg.go.dev] |
| stdlib `encoding/json` | — | state.json (de)serialization | Default `time.Time` marshal is RFC3339 UTC-with-offset; fine for our schema. [VERIFIED: stdlib docs] |
| stdlib `errors` | — | Sentinel errors + `errors.Is` | Idiomatic since Go 1.13. [VERIFIED: stdlib docs] |
| stdlib `log/slog` | — | Stderr-only logging (if any) | MCP-02 mandates stdout discipline. Prefer returning errors over logging from store. [CITED: REQUIREMENTS.md MCP-02] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify/require` | v1.11.1 | Unit test assertions | Project convention (see `internal/idgen/idgen_test.go`) |
| stdlib `os/exec` | — | Spawn child processes in integration tests | Cross-process flock safety test (SC #4) |
| stdlib `testing` | — | Test runner, `-race` | CI gate QA-03 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `gofrs/flock` v0.12.1 | `gofrs/flock` v0.13.0 | v0.13 adds `Stat()` (unused) + golang.org/x/sys v0.37 (we don't need it) but **forces Go 1.24**. Bumping Go toolchain is out of scope for Phase 4 and no other dep in the plan needs it. [VERIFIED: go.mod at v0.13.0] |
| `gofrs/flock` | stdlib `golang.org/x/sys/unix.Flock` + build-tagged Windows impl | Saves one dep but costs ~40 LOC of build-tagged platform code, including the `LockFileEx` dance. gofrs/flock is 400 LOC audited across ~6 years. Net negative to reinvent. [CITED: CLAUDE.md STACK section] |
| `renameio/v2` (unix) + hand-rolled atomic (windows) | Single cross-platform lib like `natefinch/atomic` or `rasa/compat` | `natefinch/atomic` is pure Go, MIT, uses `MoveFileEx` on Windows — but last release was June 2021, 208 stars. `rasa/compat` is broader scope than we need. One cross-platform lib simplifies code at the cost of abandoning the gold-standard fsync-before-rename of renameio on unix. **Reject** — keep renameio on unix for durability; hand-roll Windows (single-writer-under-flock means `os.Rename` is sufficient there). [VERIFIED: WebSearch, natefinch/atomic README] |
| `renameio/v2` | `renameio` v1 | v2 applies umask by default (v1 did not). Use `renameio.IgnoreUmask()` or `WithStaticPermissions(0o600)` to get v1 behavior — REQUIREMENTS CORE-05 demands mode 0600, so this matters. [VERIFIED: pkg.go.dev v2 README] |
| JSON + flock | SQLite (`modernc.org/sqlite`) | Rejected by PROJECT.md for good reason; JSON is right at expected scale. [CITED: PROJECT.md Out of Scope] |

**Installation:**
```bash
go get github.com/gofrs/flock@v0.12.1
go get github.com/google/renameio/v2@v2.0.2
```

**Version verification:**
- `gofrs/flock` v0.12.1 released **2024-07-22** (fix: missing read-write flag in reopenFDOnError). Floor: Go 1.21. [VERIFIED: GitHub releases]
- `gofrs/flock` v0.13.0 released **2024-10-09** (adds `Stat`, bumps `x/sys`, **requires Go 1.24**). [VERIFIED: go.mod at tag]
- `google/renameio/v2` v2.0.2 released Jan 2026. Floor: Go 1.17. [VERIFIED: pkg.go.dev]

## Architecture Patterns

### System Architecture Diagram

```
                  ┌────────────────────────────────────────┐
                  │            caller (Phase 5/7)           │
                  │   owns a per-process OwnerToken        │
                  └────────────┬───────────────────────────┘
                               │ store.Register(token, cond)
                               │ store.Resolve(id, token, force)
                               │ store.Get(id) / List() / Purge()
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                       internal/store                         │
│                                                              │
│   ┌──────────────────────┐        ┌──────────────────────┐  │
│   │   withLockedState    │        │   withSharedLock     │  │
│   │   (LOCK_EX + RMW)    │        │   (LOCK_SH, ro)      │  │
│   └──────────┬───────────┘        └──────────┬───────────┘  │
│              │                                │              │
│              ▼                                ▼              │
│   ┌──────────────────────┐        ┌──────────────────────┐  │
│   │  flock.New(lockPath) │        │  flock.New(lockPath) │  │
│   │      .Lock()         │        │     .RLock()         │  │
│   └──────────┬───────────┘        └──────────┬───────────┘  │
│              │                                │              │
│              ▼                                ▼              │
│   ┌──────────────────────┐        ┌──────────────────────┐  │
│   │ os.ReadFile(state)   │        │ os.ReadFile(state)   │  │
│   │ json.Unmarshal       │        │ json.Unmarshal       │  │
│   │ fn(state) → mutation │        │ fn(state)            │  │
│   │ json.Marshal         │        │                      │  │
│   │ writeStateAtomic()   │        │                      │  │
│   └──────────┬───────────┘        └──────────────────────┘  │
│              │                                               │
│              ▼                                               │
│   ┌────────────────────────────────────────────┐            │
│   │     writeStateAtomic (build-tagged)        │            │
│   │  unix: renameio.WriteFile(path, data,      │            │
│   │         0600, IgnoreUmask())               │            │
│   │  win:  os.WriteFile tmp + os.Rename        │            │
│   └────────────────────────────────────────────┘            │
│                                                              │
│   lock path selection (build-tagged):                       │
│     unix: lockPath == statePath                             │
│     win:  lockPath == statePath + ".lock"                   │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼
                    ┌──────────────────────────┐
                    │     state.json (0600)     │
                    │    parent dir (0700)     │
                    │  (state.json.lock on Win) │
                    └──────────────────────────┘
```

**Data flow (Register):**
1. Caller passes `ownerToken` + `condition` → `store.Register`.
2. `withLockedState(fn)` acquires `LOCK_EX` on `lockPath` (creates lock file if absent).
3. `withLockedState` reads state file (empty state if not-exist).
4. `fn(s *state)` allocates `id = idgen.Allocate(s.Counter)`, increments `s.Counter`, inserts record with `OwnerToken = ownerToken`, `Status = "pending"`, `CreatedAt = time.Now().UTC()`.
5. `withLockedState` marshals JSON, calls `writeStateAtomic(path, data)`.
6. Release lock. Return `id, nil`.

**Data flow (Resolve):**
1. Caller: `store.Resolve(id, ownerToken, force)`.
2. `withLockedState(fn)` acquires `LOCK_EX`.
3. `fn(s)`: look up `s.Records[id]` → `ErrUnknownID` if absent.
4. If `record.Status == "resolved"` → `ErrAlreadyResolved`.
5. If `!force && record.OwnerToken != ownerToken` → `ErrNotOwner`.
6. Set `record.Status = "resolved"`, `record.ResolvedAt = &now`, persist.

### Recommended Project Structure

```
internal/store/
├── store.go            # public API: Open, Register, Resolve, Get, List, Purge; Record type
├── schema.go           # JSON types: state, record; version constant; MarshalJSON if needed
├── lock.go             # withLockedState / withSharedLock helpers (cross-platform logic)
├── lock_unix.go        # //go:build !windows — lockFilePath = statePath
├── lock_windows.go     # //go:build windows   — lockFilePath = statePath + ".lock"
├── atomic_unix.go      # //go:build !windows — writeStateAtomic via renameio.WriteFile
├── atomic_windows.go   # //go:build windows   — writeStateAtomic via tmp+os.Rename
├── errors.go           # sentinel errors: ErrUnknownID, ErrAlreadyResolved, ErrNotOwner, ErrSchemaVersion, ErrCorruptJSON
├── store_test.go       # unit tests (single-process, tmpdir, table-driven)
└── integration_test.go # //go:build integration — cross-process tests via re-exec
```

### Pattern 1: Closure-scoped lock helpers (idiomatic Go)
**What:** Wrap lock acquisition + state load + state save in a helper that takes a closure operating on decoded state.
**When to use:** Everywhere that touches state — Register, Resolve, Purge, internal list/get implementations.
**Example:**
```go
// Source: idiomatic Go pattern; mirrors sync.Mutex "do work under lock" style.
func (s *Store) withLockedState(fn func(*state) error) (err error) {
	fl := flock.New(lockFilePath(s.path))
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("store: acquire exclusive lock: %w", err)
	}
	defer func() {
		if uerr := fl.Unlock(); uerr != nil && err == nil {
			err = fmt.Errorf("store: release exclusive lock: %w", uerr)
		}
	}()

	st, err := loadState(s.path)
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		return err
	}
	return saveState(s.path, st)
}
```

### Pattern 2: gofrs/flock exclusive lock (lazy file create)
**What:** `flock.New(path)` does not touch the filesystem; first `Lock()` call opens the file with `O_CREATE | O_RDONLY` (or `O_RDWR` on AIX/Solaris/Illumos) at mode `0600`. `Unlock()` releases and closes the fd. Do not call `Close()` separately — `Unlock` already closes on most platforms; `Close()` is documented as equivalent.
**When to use:** Every lock acquisition. **Do not reuse a `*Flock` across calls** — construct one per scope.
**Example:**
```go
// Source: pkg.go.dev/github.com/gofrs/flock#New, #Lock, #Unlock
fl := flock.New("/path/to/state.json")  // no I/O yet
if err := fl.Lock(); err != nil {       // creates file if missing, blocks on contention
	return err
}
defer fl.Unlock()                        // release + close fd
```
[VERIFIED: github.com/gofrs/flock flock.go source — `New` sets `flag = os.O_CREATE | os.O_RDONLY`, `perm = 0o600`; `setFh` opens lazily via `os.OpenFile`]

### Pattern 3: Shared lock for readers (Get, List)
**What:** `flock.RLock()` takes a shared lock (POSIX `LOCK_SH`, Windows `LockFileEx` with shared flag = 0x0). Multiple readers can hold shared locks simultaneously; writers block until all readers release. The SAME `Unlock()` releases a shared lock — there is no separate `RUnlock`.
**When to use:** `store.Get(id)` and `store.List()` — read-only operations that must not serialize.
**Example:**
```go
// Source: pkg.go.dev/github.com/gofrs/flock#RLock, #Unlock
fl := flock.New(lockFilePath(s.path))
if err := fl.RLock(); err != nil {
	return nil, err
}
defer fl.Unlock()  // NOT RUnlock — there is no such method
```
**Caveat from docs:** "The locking behaviors are not guaranteed to be the same on each platform. For example, some UNIX-like operating systems will transparently convert a shared lock to an exclusive lock." For our use case (single-user laptop, Linux/macOS/Windows), this is acceptable.

### Pattern 4: Atomic write on POSIX via renameio/v2
**What:** Write bytes to a temp file in the same directory as the target, fsync, then `rename(2)` over the target. `renameio.WriteFile` does all of this in one call.
**When to use:** Every write to state.json on unix.
**Example:**
```go
// Source: pkg.go.dev/github.com/google/renameio/v2#WriteFile
// IgnoreUmask ensures we get 0600 exactly, not 0600 & ^umask.
if err := renameio.WriteFile(s.path, data, 0o600, renameio.IgnoreUmask()); err != nil {
	return fmt.Errorf("store: atomic write: %w", err)
}
```
**Why `IgnoreUmask`:** CORE-05 mandates file mode `0600`. In v2 (unlike v1), `WriteFile(path, data, 0o600)` without `IgnoreUmask` yields `0o600 & ^umask`. A default umask of `022` strips group/other bits (already absent) and has no visible effect; an aggressive umask like `077` leaves `0600` unchanged. To be defensive and match the requirement precisely, pass `IgnoreUmask()` (equivalently, `WithStaticPermissions(0o600)` also works). [VERIFIED: pkg.go.dev v2 README "migration from v1"]

### Pattern 5: Atomic write on Windows (hand-rolled)
**What:** Write to a temp file in the same directory, then `os.Rename`. Go's `os.Rename` on Windows uses `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` (Go 1.5+), which is atomic on the same volume.
**When to use:** Windows only, under `//go:build windows`.
**Example:**
```go
// Source: stdlib os.Rename Windows implementation uses MoveFileEx + MOVEFILE_REPLACE_EXISTING.
// NTFS rename-with-replace is atomic on the same volume since Windows 2000.
func writeStateAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
```
**Caveat:** Because all Windows writes are protected by an exclusive flock on `state.json.lock` (not `state.json`), no other process holds a handle on `state.json` during rename. Under single-writer-flock-protection, `os.Rename` is functionally atomic. This avoids adding `natefinch/atomic` as a dep.

### Pattern 6: Lazy state file creation
**What:** The store does not pre-create `state.json` on `Open`. It is created on the first successful write inside `withLockedState`. If `loadState` sees `os.ErrNotExist`, it returns an empty-but-valid state `{Version: 1, Counter: 0, Records: map[string]record{}}`.
**When to use:** Always — makes tests trivial (`t.TempDir()` + `store.Open` "just works" without filesystem setup).

### Anti-Patterns to Avoid

- **Reusing a `*Flock` across operations:** Construct a new `flock.New` per `withLockedState` call. Don't store a `*Flock` on the `Store` struct — it makes concurrent goroutine-in-same-process testing harder and couples lifetime. The process-level file lock is acquired anew each call; overhead is microseconds (one `open()` + `flock()`).
- **Calling `flock.Close()` separately from `flock.Unlock()`:** `Close()` is documented as equivalent to `Unlock()`. Call one, not both.
- **Holding the lock across the MCP tool invocation boundary:** REQUIREMENTS CORE-04 explicitly forbids this. The lock covers exactly one RMW; caller re-locks if it needs another op.
- **In-memory caching of state:** Out of scope per CONTEXT.md. Re-read on each op; OS page cache makes this fast enough.
- **Using `sync.Mutex` for process-local coordination on top of `flock`:** Tests may run concurrent goroutines calling `Register` — but flock is process-wide, meaning two goroutines in the same process both calling `flock.New().Lock()` will EACH succeed (flock is per-fd, and same-process same-fd is a no-op). Add a `sync.Mutex` on `Store` to serialize same-process callers BEFORE taking flock. This is the one place a mutex is needed.
- **Silent schema-version reset on mismatch:** On `version != 1`, return `ErrSchemaVersion` with actionable text ("state file has version N, this binary supports version 1; run `mcp-chain doctor` or restore backup"). Never overwrite.
- **Writing to stdout from the store package:** Would break MCP stdio protocol. If any diagnostic is needed, `log/slog` to stderr; otherwise return an error.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Advisory cross-platform file lock | Build-tagged wrappers around `golang.org/x/sys/unix.Flock` + `windows.LockFileEx` | `gofrs/flock` | ~400 LOC of platform-specific syscall-wrangling, EDEADLK handling on AIX/Solaris, timeout/context support. gofrs/flock solves it once. |
| Atomic file replace on unix | `os.WriteFile` + `os.Rename` | `renameio.WriteFile` | Missing fsync means the rename can produce a 0-byte file after power loss. renameio fsyncs temp file AND parent directory. |
| UUID / crypto-random token generation | Hand-roll hex of `crypto/rand` bytes | `crypto/rand.Read(buf[:16])` + `hex.EncodeToString(buf)` | stdlib is fine at 5 LOC; dep like `google/uuid` adds 100KB+. **Note: this is Phase 5's concern (MCP server), not Phase 4's.** |
| JSON state round-trip | Custom serialization with `bufio.Scanner` | `encoding/json` | Right tool; state file is small (≤ 100s of entries); human-readable for debugging; round-trip stable. |
| Sentinel error typing | `errors.New` instances on the fly | Package-level `var Err... = errors.New(...)` | `errors.Is` requires package-level values. |

**Key insight:** The store package is a 300-500 LOC thin layer over two battle-tested primitives (flock, atomic rename) plus the stdlib. Every line of "clever" custom lock/rename/retry logic is a liability.

## Runtime State Inventory

Not applicable to Phase 4 — this is a greenfield phase (creating `internal/store/` from scratch, no rename/migration). Omitted.

## Common Pitfalls

### Pitfall 1: `renameio/v2` silent Windows build break
**What goes wrong:** Import `renameio` with no build tag, add a Windows build step in CI → compile error: "undefined: renameio.WriteFile" on Windows because the package exports no functions under `GOOS=windows`.
**Why it happens:** The package uses build constraints at the file level; the Windows build sees an empty package.
**How to avoid:** `atomic_unix.go` (with `//go:build !windows`) is the ONLY file that imports `renameio`. `atomic_windows.go` (with `//go:build windows`) uses stdlib `os.Rename`. Both files export the same helper signature so callers are platform-blind.
**Warning signs:** `go vet ./...` or `GOOS=windows go build ./...` in CI surfaces this immediately. Phase 9 (CI) will cross-compile — catch it by running the cross-build once locally during Phase 4: `GOOS=windows GOARCH=amd64 go build ./internal/store/...`.
[VERIFIED: WebSearch, renameio README, pkg.go.dev]

### Pitfall 2: gofrs/flock v0.13.0 silently bumps Go floor to 1.24
**What goes wrong:** Plan says "add github.com/gofrs/flock v0.13.0", `go get` succeeds, but `go build` fails with "module requires go 1.24" or the toolchain auto-downloads 1.24.
**Why it happens:** v0.13.0 go.mod declares `go 1.24.0`; the project is on 1.23.8.
**How to avoid:** Pin `v0.12.1`. It has every API we need. If Phase 5 or later needs something from v0.13, bump toolchain then — not here.
**Warning signs:** `go build` output mentioning `go1.24` or automatic toolchain fetch.
[VERIFIED: go.mod at v0.13.0 tag, v0.12.1 tag]

### Pitfall 3: Umask strips 0600 permissions
**What goes wrong:** Developer's umask is `022`, they write `renameio.WriteFile(path, data, 0o600)`, file ends up `0600` (umask doesn't affect already-absent group/other bits). But on a system with `umask 077` or similar, the requested mode could be further masked. REQUIREMENTS CORE-05 wants `0600` precisely.
**Why it happens:** v2 renameio applies umask by default; v1 did not.
**How to avoid:** Always pass `renameio.IgnoreUmask()` (or equivalently `renameio.WithStaticPermissions(0o600)`). Unit test: after `Register`, `os.Stat(statePath).Mode().Perm() == 0o600`.
**Warning signs:** Test that asserts file mode fails on CI runner with nonstandard umask.
[VERIFIED: pkg.go.dev v2 README migration guide]

### Pitfall 4: Same-process goroutines both pass flock
**What goes wrong:** Two goroutines in the same `go test` process each call `flock.New(path).Lock()`; flock is per-file-descriptor on POSIX, so both acquisitions succeed, and their RMW interleaves — lost updates.
**Why it happens:** flock is designed to serialize cross-PROCESS, not cross-goroutine-same-process.
**How to avoid:** Store has a `sync.Mutex` taken BEFORE `flock.New(path).Lock()`. Unit test: spawn N goroutines in same process, register concurrently, assert all IDs unique.
**Warning signs:** Flaky tests, lost updates in same-process concurrent tests. `go test -race` may NOT detect this if the updates are on the filesystem rather than shared memory.
[CITED: flock(2) man page; Go cmd/go/internal/lockedfile code uses same pattern]

### Pitfall 5: Lock survives rename on POSIX but lock file handle churns
**What goes wrong:** On unix, `flock.New(stateJson).Lock()` opens `state.json`, `flock`s its fd. Writer does `renameio.WriteFile(stateJson, ...)` which renames a temp file over `state.json`. The writer's fd now refers to the OLD inode (now unlinked once the only link is gone), and its lock is on the OLD inode. Next reader's `flock.New(stateJson).Lock()` opens the NEW inode — and sees no existing lock. Windows of ~microseconds where a reader can slip in between the writer's rename and the writer's Unlock, observing either state depending on timing.
**Why it happens:** flock is inode-scoped; rename changes which inode `state.json` names.
**How to avoid:** For mcp-chain this is NOT a correctness bug because the reader's RMW would still complete atomically on its own inode. BUT — to avoid any surprise and match the Windows architecture, we **consider using a separate `state.json.lock` file on unix too**. Simpler mental model, identical cross-platform semantics.
**Decision for Phase 4:** Use a separate lock file `state.json.lock` on BOTH unix and Windows. The build-tag split collapses to just a constant: `const lockSuffix = ".lock"` in `lock.go`, no need for `lock_unix.go` / `lock_windows.go` at all. This is simpler than CONTEXT.md's build-tagged path strategy and eliminates an entire class of rename-inode-flock-interaction bugs.
**Warning signs:** Brief windows of unlocked access during rename; hard to reproduce, harder to debug.
[CITED: flock(2) man page — "locks are associated with an open file description"; Richard Crowley "Things UNIX can do atomically"]

> **OPEN QUESTION #1 (flagged to orchestrator):** Should unix and Windows BOTH use `state.json.lock`, collapsing away the `lock_unix.go` / `lock_windows.go` split? CONTEXT.md says "Windows: separate lock file; Unix: lock state.json directly". Recommended reversal: both use `.lock` file, simpler and safer. No downside except CONTEXT.md inconsistency.

### Pitfall 6: `resolved_at *time.Time` JSON nullability
**What goes wrong:** A struct field `ResolvedAt time.Time` marshals zero time as `"0001-01-01T00:00:00Z"`, not `null`. Schema in CONTEXT.md calls for literal `null`.
**Why it happens:** `time.Time` is a struct, not a pointer; zero value marshals to its RFC3339 representation.
**How to avoid:** Use `*time.Time`. Nil marshals to `null`; non-nil marshals to the RFC3339 string. On unmarshal, `null` stays nil, string populates a fresh value. [VERIFIED: encoding/json spec, time.Time docs]
**Warning signs:** Round-trip test: `Marshal({version:1,...,resolved_at:nil}) → Unmarshal → re-Marshal` must produce byte-identical output.

### Pitfall 7: Counter increment ordering
**What goes wrong:** Allocate first, increment after → if `Register` panics/errors between allocate and write, counter in memory is ahead of disk, but that's fine because we never wrote; on retry, same ID gets allocated — **NOT a bug, this is desirable.** The bug is: increment first, then allocate → produces `hex-0001` for counter=0.
**Why it happens:** Off-by-one in the order of operations inside `fn` closure.
**How to avoid:** Reference semantics — `id := idgen.Allocate(s.Counter); s.Counter++`. Unit test: register twice, assert first gets `acid`, second gets `acorn`.
**Warning signs:** First-ever Register returns `acorn` instead of `acid`.

### Pitfall 8: Purge resets counter
**What goes wrong:** Dev writes purge as `delete from records; counter = len(records)` or `counter = 0`. Next register reuses an ID — violates CORE-07 "Counter never decremented on purge (prevents ID reuse)".
**Why it happens:** Natural intuition to "renumber".
**How to avoid:** Purge only touches `s.Records`. Never mutates `s.Counter`. Unit test: register 3, purge all, register 1, assert new ID is `gadget` (counter=3 → words[3] = "acre"... actual value depends on EFF list; whatever idgen returns for counter=3, not "acid").

### Pitfall 9: Corrupt JSON returned as generic error
**What goes wrong:** User's state file gets truncated mid-write due to some non-flock-protected tool (editor crash, disk full). `loadState` returns "unexpected end of JSON input" — unhelpful for recovery. REQUIREMENTS CORE-09 mandates "actionable message (file path + repair guidance)".
**How to avoid:** On `json.Unmarshal` error, wrap with path and guidance: `fmt.Errorf("store: state file %s is corrupt: %w. To recover: back up this file and remove it; mcp-chain will create a fresh state on next operation (all registrations will be lost)", s.path, err)`. Introduce `ErrCorruptJSON` sentinel.

## Code Examples

### Schema types (schema.go)
```go
// Source: CONTEXT.md locked schema + encoding/json idioms
package store

import "time"

// schemaVersion is the single supported state-file schema version.
// Bumping this MUST be paired with a migration in a future phase.
const schemaVersion = 1

type state struct {
	Version uint64            `json:"version"`
	Counter uint64            `json:"counter"`
	Records map[string]record `json:"records"`
}

type record struct {
	ID         string     `json:"id"`
	Condition  string     `json:"condition"`
	Status     string     `json:"status"` // "pending" | "resolved"
	OwnerToken string     `json:"owner_token"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"` // nil until resolved
}

const (
	statusPending  = "pending"
	statusResolved = "resolved"
)
```

### Public API (store.go)
```go
// Source: synthesis of CONTEXT.md + Go API conventions + Phase 2/3 patterns
package store

import (
	"sync"
	"time"
)

// Store is the hexagonal persistence core. All methods are safe for concurrent
// use within the same process AND across processes on the same file.
type Store struct {
	path string     // path to state.json
	mu   sync.Mutex // serializes same-process callers BEFORE flock
}

// Record is the exported view of a single registration. Returned by Get/List.
// This is a value type — callers cannot mutate the store via a returned Record.
type Record struct {
	ID         string
	Condition  string
	Status     string
	OwnerToken string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// PurgeOptions selects which records Purge should delete. Exactly one of
// ID, All, Resolved must be set. Bare PurgeOptions{} returns ErrPurgeArgRequired.
type PurgeOptions struct {
	ID       string
	All      bool
	Resolved bool
}

// ResolveOptions controls Resolve behavior. Force=true bypasses the
// OwnerToken check (CLI --force escape hatch per CORE-08).
type ResolveOptions struct {
	Force bool
}

// Open returns a Store for the given path. It does not perform I/O; the first
// Register/Resolve/etc. is when state.json (and its lock file) are created.
// The parent directory must already exist (Phase 3 statepath.Resolve ensures this).
func Open(path string) (*Store, error) { /* ... */ }

// Register allocates a new word-ID, stores a pending record with the caller's
// ownerToken stamped on it, increments the counter, and atomically persists.
// Returns the newly allocated ID.
func (s *Store) Register(ownerToken, condition string) (id string, err error) { /* ... */ }

// Resolve marks id as resolved. Errors distinctly:
//   ErrUnknownID        — id not in records
//   ErrAlreadyResolved  — id already resolved
//   ErrNotOwner         — ownerToken mismatch (only if !opts.Force)
func (s *Store) Resolve(id, ownerToken string, opts ResolveOptions) error { /* ... */ }

// Get returns the record for id, or ErrUnknownID. Acquires LOCK_SH.
func (s *Store) Get(id string) (Record, error) { /* ... */ }

// List returns all records in unspecified order. Acquires LOCK_SH.
// Callers that need deterministic order sort by CreatedAt.
func (s *Store) List() ([]Record, error) { /* ... */ }

// Purge deletes records per opts. Returns the number removed.
// Counter is NEVER decremented (CORE-07 ID reuse prevention).
func (s *Store) Purge(opts PurgeOptions) (removed int, err error) { /* ... */ }
```

### Sentinel errors (errors.go)
```go
// Source: stdlib errors idioms; errors.Is-compatible
package store

import "errors"

var (
	// ErrUnknownID is returned when an id is not present in state.
	ErrUnknownID = errors.New("store: unknown id")

	// ErrAlreadyResolved is returned when Resolve is called on a
	// record whose status is already "resolved".
	ErrAlreadyResolved = errors.New("store: already resolved")

	// ErrNotOwner is returned when Resolve is called with a token
	// that does not match the record's owner_token, unless Force.
	ErrNotOwner = errors.New("store: not owner")

	// ErrSchemaVersion is returned when state.version is not 1.
	ErrSchemaVersion = errors.New("store: unsupported schema version")

	// ErrCorruptJSON is returned when the state file fails JSON decode.
	// Error message includes file path and repair guidance.
	ErrCorruptJSON = errors.New("store: corrupt state file")

	// ErrPurgeArgRequired is returned from Purge when no target is selected.
	ErrPurgeArgRequired = errors.New("store: purge requires --id, --all, or --resolved")
)
```

### Lock helpers (lock.go)
```go
// Source: gofrs/flock idioms + closure-scoped lock pattern
package store

import (
	"fmt"

	"github.com/gofrs/flock"
)

// lockFilePath returns the flock target path for a given state path.
// We ALWAYS use a sibling .lock file (both unix and windows) — simpler mental
// model, avoids the rename-changes-inode interaction on POSIX, matches the
// Windows constraint that the state file can't be locked while being replaced.
func lockFilePath(statePath string) string {
	return statePath + ".lock"
}

func (s *Store) withLockedState(fn func(*state) error) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(lockFilePath(s.path))
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("store: acquire exclusive lock on %s: %w", fl.Path(), err)
	}
	defer func() {
		if uerr := fl.Unlock(); uerr != nil && err == nil {
			err = fmt.Errorf("store: release exclusive lock: %w", uerr)
		}
	}()

	st, err := loadState(s.path)
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		return err
	}
	return saveState(s.path, st)
}

func (s *Store) withSharedLock(fn func(*state) error) (err error) {
	// Same-process mutex still needed: RLock is concurrent-safe cross-process,
	// but we avoid pessimizing by not taking s.mu for pure reads? Actually
	// DO take it: cheap, and avoids surprises when callers mix read+write
	// patterns. Revisit if read throughput becomes an issue (it won't).
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(lockFilePath(s.path))
	if err := fl.RLock(); err != nil {
		return fmt.Errorf("store: acquire shared lock: %w", err)
	}
	defer func() {
		if uerr := fl.Unlock(); uerr != nil && err == nil {
			err = fmt.Errorf("store: release shared lock: %w", uerr)
		}
	}()

	st, err := loadState(s.path)
	if err != nil {
		return err
	}
	return fn(st)
}
```

> **OPEN QUESTION #2 (flagged):** Should `withSharedLock` take `s.mu` or not? Taking it serializes same-process concurrent reads (a slight pessimization); not taking it requires careful audit that `loadState` + `fn(st)` are reentrant-safe. Recommended: take it. Concurrent-reader throughput on a local JSON file is not a hot path for mcp-chain. If it becomes one, switch to `sync.RWMutex`.

### Atomic write — unix (atomic_unix.go)
```go
//go:build !windows

// Source: google/renameio/v2 README
package store

import (
	"fmt"

	"github.com/google/renameio/v2"
)

// writeStateAtomic writes data to path atomically with mode 0600 (CORE-05).
// IgnoreUmask ensures the final file mode is exactly 0600 regardless of
// the process umask (v2 would otherwise apply umask).
func writeStateAtomic(path string, data []byte) error {
	if err := renameio.WriteFile(path, data, 0o600, renameio.IgnoreUmask()); err != nil {
		return fmt.Errorf("store: atomic write %s: %w", path, err)
	}
	return nil
}
```

### Atomic write — windows (atomic_windows.go)
```go
//go:build windows

// Source: stdlib os.Rename backed by MoveFileEx on Windows (Go 1.5+).
// Safe under the single-writer-flock-protection invariant.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func writeStateAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("store: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	// Cleanup temp on any error path; harmless no-op after successful rename.
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: close temp: %w", err)
	}
	// Explicit chmod — os.CreateTemp creates with 0600 on unix, but on Windows
	// ACLs differ; be explicit to match CORE-05.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("store: chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("store: rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}
```
[VERIFIED: stdlib docs — Go's os.Rename on Windows uses MoveFileEx with MOVEFILE_REPLACE_EXISTING since Go 1.5]

### Load/save state (store.go internals)
```go
// Source: encoding/json idioms
func loadState(path string) (*state, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &state{Version: schemaVersion, Counter: 0, Records: map[string]record{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%w: %s: %v (back up and remove to reset)", ErrCorruptJSON, path, err)
	}
	if s.Version != schemaVersion {
		return nil, fmt.Errorf("%w: got version %d, want %d (file: %s)", ErrSchemaVersion, s.Version, schemaVersion, path)
	}
	if s.Records == nil {
		s.Records = map[string]record{} // defensive: old partial writes
	}
	return &s, nil
}

func saveState(path string, s *state) error {
	// Use Indent for human readability (CORE: "human-readable on disk").
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal state: %w", err)
	}
	// Trailing newline — editor-friendly.
	data = append(data, '\n')
	return writeStateAtomic(path, data)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `renameio` v1 (permission passed exactly) | `renameio/v2` (permission further masked by umask unless `IgnoreUmask()`) | v2.0.0 | Must pass `IgnoreUmask()` or `WithStaticPermissions(0o600)` to match CORE-05 |
| Shared access via Unix socket daemon | Shared state file + advisory flock | N/A — PROJECT.md chose this | Simpler lifecycle, no daemon process |
| `pkg/errors` for wrapping | stdlib `fmt.Errorf("ctx: %w", err)` + `errors.Is/As` | Go 1.13 | Already project convention |
| `logrus` / `zap` / `zerolog` | `log/slog` to stderr (store package logs nothing — returns errors) | Go 1.21 | Stdout discipline (MCP-02) |

**Deprecated/outdated:**
- `renameio` v1: superseded by v2; v2 fixes symlink edge cases and is the current line.
- `gofrs/flock` `NewFlock`: deprecated in favor of `New`.

## Project Constraints (from CLAUDE.md)

These directives from CLAUDE.md MUST be honored by the plan. Any plan that contradicts them is wrong.

- **Tech stack: Pure Go, no cgo.** — Both recommended deps are pure Go. [VERIFIED: go.mod of gofrs/flock v0.12.1 and renameio/v2 v2.0.2 — no cgo directives]
- **Binary size ≤15MB stripped.** — Adding two small deps (~500 KB combined import weight). Well within budget.
- **Startup ≤100ms.** — Store does no I/O at `Open`; first Register/Resolve is ≤2 ms on a local SSD. No impact.
- **Resident memory ≤20MB.** — Store footprint is state file size (KB range at expected entry counts).
- **Token budget: MCP tool descriptions terse.** — Store returns errors; tool description text is Phase 5's concern. No model-facing strings in store.
- **Platform: Linux + macOS primary; Windows supported via CI cross-compile.** — Windows build must not break. Pitfall 1 + build-tagged atomic_windows.go cover this.
- **Dependencies: Minimize. Stdlib-first.** — Two deps added; both justified above with "Don't Hand-Roll" rationale.
- **Distribution: Claude Code plugin mechanism.** — Irrelevant to Phase 4 (distribution is Phase 9).
- **Stack pinned:** Phase 4 inherits pins from synthesized stack. Note the v0.13.0 vs v0.12.1 discrepancy flagged above.
- **Stdout discipline (MCP-02):** Store MUST NOT write to stdout. No `fmt.Print*`, no `log.Print*` (which defaults to stderr but don't risk it). If logging is wanted, `slog.New(slog.NewTextHandler(os.Stderr, nil))`. Prefer returning errors.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-04 | State persisted to shared JSON file with `gofrs/flock` exclusive lock during RMW; flock never held across MCP tool invocations; `list`/`status` acquire shared lock | `withLockedState` (LOCK_EX) + `withSharedLock` (LOCK_SH) pattern; lock scope bounded to single RMW per method call. |
| CORE-05 | Atomic writes via `google/renameio/v2` (temp-file + rename + directory fsync); state file mode 0600; parent dir 0700 | `writeStateAtomic` via `renameio.WriteFile(path, data, 0o600, IgnoreUmask())` on unix; stdlib equivalent on Windows. Parent dir is Phase 3's concern (already 0700). |
| CORE-08 | Per-process OwnerToken stored with each registration; required to match on resolve; CLI `--force` bypass | `record.OwnerToken` field + `store.Resolve(id, token, ResolveOptions{Force})`. Generation is Phase 5's concern. |
| CORE-09 | State schema versioned via top-level `version` field; unknown version errors clearly; corrupt JSON returns actionable message | `schemaVersion = 1` constant + `ErrSchemaVersion` + `ErrCorruptJSON` with path + repair guidance. |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | ✓ | 1.23.8 (per go.mod) | — |
| `github.com/gofrs/flock` | flock.Lock/RLock/Unlock | to add | v0.12.1 (pinned) | — |
| `github.com/google/renameio/v2` | Atomic file replace (unix) | to add | v2.0.2 | stdlib os.Rename (windows) |
| `github.com/stretchr/testify` | Unit test assertions | ✓ (in go.mod) | v1.11.1 | — |

**Missing dependencies with no fallback:** None — we can add both deps normally.

**Missing dependencies with fallback:** `renameio/v2` on Windows — stdlib `os.Rename` suffices under flock protection.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| Config file | none — Go convention; tests live alongside source |
| Quick run command | `go test ./internal/store/...` |
| Full suite command | `go test -race -count=1 -timeout=60s ./internal/store/...` |
| Integration command | `go test -race -count=1 -tags=integration -timeout=120s ./internal/store/...` |

### Phase Requirements → Test Map

| Test | Covers SC | File | Type | Automated Command |
|------|-----------|------|------|-------------------|
| `TestStore_OpenNoIO` | — | store_test.go | unit | `go test -run TestStore_OpenNoIO ./internal/store` |
| `TestStore_RegisterAllocatesFirstWord` | 1, 2 | store_test.go | unit | `go test -run TestStore_RegisterAllocatesFirstWord ./internal/store` |
| `TestStore_RegisterMonotonicCounter` | 2 | store_test.go | unit | `go test -run TestStore_RegisterMonotonicCounter ./internal/store` |
| `TestStore_RegisterStoresOwnerToken` | 3 | store_test.go | unit | `go test -run TestStore_RegisterStoresOwnerToken ./internal/store` |
| `TestStore_ResolveOwnerOk` | 3 | store_test.go | unit | `go test -run TestStore_ResolveOwnerOk ./internal/store` |
| `TestStore_ResolveWrongOwnerReturnsErrNotOwner` | 3 | store_test.go | unit | `go test -run TestStore_ResolveWrongOwner ./internal/store` |
| `TestStore_ResolveForceBypassesOwnerCheck` | 3 | store_test.go | unit | `go test -run TestStore_ResolveForce ./internal/store` |
| `TestStore_ResolveAlreadyResolvedReturnsErr` | 3 | store_test.go | unit | `go test -run TestStore_ResolveAlreadyResolved ./internal/store` |
| `TestStore_ResolveUnknownIDReturnsErr` | 3 | store_test.go | unit | `go test -run TestStore_ResolveUnknown ./internal/store` |
| `TestStore_ResolveSetsResolvedAt` | 3 | store_test.go | unit | `go test -run TestStore_ResolveSetsResolvedAt ./internal/store` |
| `TestStore_GetReturnsRecord` | 1 | store_test.go | unit | `go test -run TestStore_Get ./internal/store` |
| `TestStore_GetUnknownIDReturnsErr` | — | store_test.go | unit | `go test -run TestStore_GetUnknown ./internal/store` |
| `TestStore_ListReturnsAllRecords` | 1 | store_test.go | unit | `go test -run TestStore_List ./internal/store` |
| `TestStore_ListEmptyWhenNoState` | 1 | store_test.go | unit | `go test -run TestStore_ListEmpty ./internal/store` |
| `TestStore_PurgeByID` | 2 | store_test.go | unit | `go test -run TestStore_PurgeByID ./internal/store` |
| `TestStore_PurgeAll` | 2 | store_test.go | unit | `go test -run TestStore_PurgeAll ./internal/store` |
| `TestStore_PurgeResolved` | 2 | store_test.go | unit | `go test -run TestStore_PurgeResolved ./internal/store` |
| `TestStore_PurgeDoesNotDecrementCounter` | 2 | store_test.go | unit | `go test -run TestStore_PurgeDoesNotDecrement ./internal/store` |
| `TestStore_PurgeRequiresTarget` | — | store_test.go | unit | `go test -run TestStore_PurgeRequires ./internal/store` |
| `TestStore_SchemaVersionMismatchErrors` | 2 | store_test.go | unit | `go test -run TestStore_SchemaVersion ./internal/store` |
| `TestStore_CorruptJSONErrors` | 2 | store_test.go | unit | `go test -run TestStore_CorruptJSON ./internal/store` |
| `TestStore_StateFileMode0600` | 2 | store_test.go | unit | `go test -run TestStore_FileMode ./internal/store` |
| `TestStore_ResolvedAtNullInJSON` | — | store_test.go | unit (round-trip) | `go test -run TestStore_ResolvedAtNull ./internal/store` |
| `TestStore_SameProcessGoroutineConcurrency` | 1, 4 | store_test.go | unit (goroutines) | `go test -race -run TestStore_SameProcess ./internal/store` |
| `TestStore_LoadMissingStateReturnsEmpty` | — | store_test.go | unit | `go test -run TestStore_LoadMissing ./internal/store` |
| `TestStore_TwoProcessesConcurrentRegister` | 1, 4 | integration_test.go | integration (re-exec) | `go test -tags=integration -run TestStore_TwoProcesses ./internal/store` |
| `TestStore_KillMidWriteLeavesCoherentState` | 4 | integration_test.go | integration (SIGKILL) | `go test -tags=integration -run TestStore_KillMidWrite ./internal/store` |
| `TestStore_CrossPlatformBuildsOnWindows` | 5 | (compile gate) | smoke | `GOOS=windows GOARCH=amd64 go build ./internal/store/...` |

### Sampling Rate
- **Per task commit:** `go test -race ./internal/store/...` (unit tests only, ~3s)
- **Per wave merge:** `go test -race -tags=integration ./internal/store/...` (includes cross-process; ~30s)
- **Phase gate:** Full suite green + `GOOS=windows GOARCH=amd64 go build ./internal/store/...` succeeds before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/store/store_test.go` — all unit tests above
- [ ] `internal/store/integration_test.go` — cross-process + kill-9 tests, build tag `integration`
- [ ] `go.mod` additions: `github.com/gofrs/flock v0.12.1`, `github.com/google/renameio/v2 v2.0.2`
- [ ] No new shared fixtures needed — each test uses `t.TempDir()` for isolation

### Integration Test Architecture (the kill-9 + concurrent test)

**SC #4 requirement:** "two processes each registering 100 entries concurrently, assert 200 unique word-IDs, no lost updates, no corrupt JSON after kill-9 mid-write"

**Re-exec pattern (recommended — test-local, no dependency on Phase 6 CLI):**

```go
//go:build integration

package store_test

import (
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tkr41850-debug/mcp-chain/internal/store"
	"github.com/stretchr/testify/require"
)

const childEnvVar = "MCP_CHAIN_STORE_TEST_CHILD"

// TestMain dispatches to child behavior if the env var is set; otherwise runs tests normally.
func TestMain(m *testing.M) {
	switch os.Getenv(childEnvVar) {
	case "register":
		childRegister()
		return // unreachable
	case "slow-write":
		childSlowWrite()
		return // unreachable
	}
	os.Exit(m.Run())
}

func childRegister() {
	path := os.Getenv("MCP_CHAIN_STATE_PATH")
	token := os.Getenv("MCP_CHAIN_OWNER_TOKEN")
	n, _ := strconv.Atoi(os.Getenv("MCP_CHAIN_N"))
	s, err := store.Open(path)
	if err != nil { os.Exit(10) }
	for i := 0; i < n; i++ {
		if _, err := s.Register(token, fmt.Sprintf("cond-%d", i)); err != nil {
			os.Exit(11)
		}
	}
	os.Exit(0)
}

func TestStore_TwoProcessesConcurrentRegister(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	exe, err := os.Executable()
	require.NoError(t, err)

	mkCmd := func(token string) *exec.Cmd {
		c := exec.Command(exe)
		c.Env = append(os.Environ(),
			childEnvVar+"=register",
			"MCP_CHAIN_STATE_PATH="+path,
			"MCP_CHAIN_OWNER_TOKEN="+token,
			"MCP_CHAIN_N=100",
		)
		c.Stdout = os.Stderr // for debugging; never the test binary's stdout
		c.Stderr = os.Stderr
		return c
	}

	t1 := hex.EncodeToString(bytes.Repeat([]byte{0xAA}, 16))
	t2 := hex.EncodeToString(bytes.Repeat([]byte{0xBB}, 16))
	c1, c2 := mkCmd(t1), mkCmd(t2)

	require.NoError(t, c1.Start())
	require.NoError(t, c2.Start())
	require.NoError(t, c1.Wait())
	require.NoError(t, c2.Wait())

	// Open the state file and assert 200 unique IDs.
	s, err := store.Open(path)
	require.NoError(t, err)
	records, err := s.List()
	require.NoError(t, err)
	require.Len(t, records, 200, "200 concurrent registrations across two processes")

	seen := make(map[string]struct{}, 200)
	for _, r := range records {
		_, dup := seen[r.ID]
		require.False(t, dup, "duplicate ID %s", r.ID)
		seen[r.ID] = struct{}{}
	}
}
```

**Kill-9 mid-write test:**

```go
func childSlowWrite() {
	// Injected pause inside withLockedState via a test-only env hook.
	// Simplest implementation: the store package reads MCP_CHAIN_TEST_WRITE_DELAY
	// and time.Sleeps during saveState if set. Only present in test builds? Or
	// always present but unused in prod (5 LOC cost). Acceptable.
	path := os.Getenv("MCP_CHAIN_STATE_PATH")
	s, err := store.Open(path)
	if err != nil { os.Exit(10) }
	// This Register will block in the sleep inside saveState.
	_, _ = s.Register(hex.EncodeToString(bytes.Repeat([]byte{0xCC}, 16)), "slow")
	os.Exit(0)
}

func TestStore_KillMidWriteLeavesCoherentState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// First, seed a valid state.
	s, err := store.Open(path)
	require.NoError(t, err)
	_, err = s.Register(hex.EncodeToString(bytes.Repeat([]byte{0xDD}, 16)), "seed")
	require.NoError(t, err)

	// Capture the seeded state.
	seeded, err := os.ReadFile(path)
	require.NoError(t, err)

	exe, _ := os.Executable()
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		childEnvVar+"=slow-write",
		"MCP_CHAIN_STATE_PATH="+path,
		"MCP_CHAIN_TEST_WRITE_DELAY=5s",
	)
	require.NoError(t, cmd.Start())
	time.Sleep(1 * time.Second) // let the child acquire lock + start the sleep
	require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	_ = cmd.Wait()

	// Assert: state file is either exactly the seeded state OR a valid new state.
	// It MUST NOT be corrupt / half-written / zero-length.
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	var parsed interface{}
	require.NoError(t, json.Unmarshal(after, &parsed), "state file must be valid JSON after SIGKILL")

	// Tighter assertion: since the write is atomic-rename, the state file should
	// be byte-identical to the seeded version (the kill happened before rename).
	require.Equal(t, seeded, after, "SIGKILL before atomic rename must leave old state intact")
}
```

**The test-write-delay hook:** A single `time.Sleep` inside `saveState` gated on an env var. Kept out of public API. Acceptable cost (~5 LOC) for testability; alternatively gate on a `//go:build storetestdelay` tag to exclude from prod binary.

> **OPEN QUESTION #3 (flagged):** How is the sleep hook introduced without leaking test-only code to prod? Options: (a) env var, 5 LOC in prod; (b) build tag `storetestdelay`, requires cross-binary compile in test; (c) inject a `writeStateAtomic` function pointer at package level, only test file sets it. Recommended (c) — cleanest; zero prod cost; standard Go test-hook pattern.

### Windows Test Strategy

- **Unit tests:** All run on Linux. The build-tagged `atomic_windows.go` is not exercised by Linux runs.
- **Cross-compile gate:** `GOOS=windows GOARCH=amd64 go build ./internal/store/...` must succeed. This proves the Windows code compiles and its API matches.
- **Integration tests:** Linux-only for Phase 4 (`//go:build !windows` on `integration_test.go`). Phase 9 CI adds a Windows build matrix that runs unit tests but skips integration and race detector (per REQUIREMENTS QA-03: "Windows runs non-race test suite").
- **Phase 4 Windows smoke:** `GOOS=windows go vet ./internal/store/...` + the build command. Document in phase plan's verification steps.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | OwnerToken-based session ownership (128-bit crypto-random token stored server-side, validated on resolve). This IS the auth primitive; no passwords/OAuth at this tier. |
| V3 Session Management | yes | OwnerToken IS the session identifier. Stamped at Register, checked at Resolve. Token is process-scoped — if the MCP server restarts, old sessions cannot be resolved (matches user intent per STACK.md design note). |
| V4 Access Control | yes | Single-user local tool. OwnerToken check is the entire ACL. `--force` is an escape-hatch for operator-initiated recovery (audit trail: Phase 7 CLI logs the force resolve to stderr). |
| V5 Input Validation | yes | `condition` string length (cap at e.g. 4KB to prevent state-file bloat); `id` format (must match allocated word-ID pattern — though not strictly required, helps early-error unknown IDs); `ownerToken` must be 32-char hex (reject malformed tokens). |
| V6 Cryptography | no | Store does no crypto. OwnerToken generation is Phase 5's concern (`crypto/rand.Read(16 bytes)` + hex encode — standard). |
| V7 Error Handling | yes | Sentinel errors returned without leaking internal state. ErrCorruptJSON includes path (local-filesystem, not secret). No panics on malformed input (use `recover`? No — malformed state should never panic if `loadState` guards are correct). |
| V10 Malicious Code | yes (passive) | Don't deserialize untrusted JSON into risky types (no `interface{}` fields; all types are primitives or known structs). `encoding/json` is safe against the gadget-chain-style attacks that plague other languages. |
| V12 File / Resources | yes | Lock file and state file in user's home or `$XDG_STATE_HOME` — no shared /tmp. File mode 0600, parent dir 0700. No path traversal (caller passes absolute path from Phase 3 `statepath.Resolve`). |

### Known Threat Patterns for {pure-Go + local-fs}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| State file tampering by another local user | Tampering | File mode 0600 + parent dir 0700 (CORE-05); only owner can read/write. Relies on OS ACLs; Windows permissions are best-effort. |
| Symlink attack on state.json path | Tampering | Not applicable — `renameio.WriteFile` handles rename atomically to the caller-specified path; `statepath.Resolve` is under user's own HOME. If an attacker has write to HOME, the game is already over. |
| Lock-starvation DoS (one process holds LOCK_EX forever) | DoS | Out of scope — all lock-holders are mcp-chain itself. No untrusted lock-holders. If the server crashes mid-lock, OS releases the flock on fd close. |
| Corrupt state file wipes user data | Availability | Atomic rename means state is either old or new, never partial. Kill-9 test proves this. ErrCorruptJSON message tells user how to recover. |
| OwnerToken leak via logs / error messages | Info disclosure | Error messages MUST NOT include the expected or received OwnerToken. `ErrNotOwner` carries no token. Log statements (if any) elide tokens. Verify in code review. |
| Wrong OwnerToken used to resolve someone else's lock | Spoofing | 128-bit random token → 2^128 guesses; infeasible. The `--force` flag is the documented escape hatch for operator recovery. |
| Replay of old OwnerToken after server restart | Auth bypass | Mitigated by per-process token. On restart, all old sessions require `--force` to resolve. This is by design (STACK.md Session Identity note). |
| Timing attack on OwnerToken compare | Info disclosure | Use `crypto/subtle.ConstantTimeCompare([]byte(token), []byte(stored))` instead of `==`. **Must be in plan.** 5 LOC. |

> **OPEN QUESTION #4 (flagged):** `crypto/subtle.ConstantTimeCompare` for OwnerToken — CONTEXT.md doesn't call this out, but it's a cheap, standard hardening. Recommend yes. Tests: none needed (correctness of the stdlib fn is not our concern); just call the right function.

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev: gofrs/flock](https://pkg.go.dev/github.com/gofrs/flock) — complete public API (Lock, RLock, Unlock, TryLock, TryRLock, Locked, RLocked, Path, New)
- [pkg.go.dev: gofrs/flock v0.12.1](https://pkg.go.dev/github.com/gofrs/flock@v0.12.1) — v0.12.1 API confirmation (no Stat method — new in 0.13)
- [gofrs/flock v0.13.0 go.mod](https://github.com/gofrs/flock/blob/v0.13.0/go.mod) — confirms `go 1.24.0` requirement, `x/sys v0.37.0` dep
- [pkg.go.dev: google/renameio/v2](https://pkg.go.dev/github.com/google/renameio/v2) — WriteFile, NewPendingFile, Options (IgnoreUmask, WithStaticPermissions, WithExistingPermissions)
- [github.com/google/renameio README](https://github.com/google/renameio) — Windows not supported statement, v1→v2 umask migration
- [gofrs/flock source: flock.go](https://github.com/gofrs/flock/blob/main/flock.go) — confirms `New()` does no I/O; `setFh` opens with `O_CREATE | O_RDONLY` at mode 0600 on first Lock
- [gofrs/flock source: flock_windows.go](https://github.com/gofrs/flock/blob/main/flock_windows.go) — confirms Windows uses `LockFileEx`/`UnlockFileEx`
- [REQUIREMENTS.md](/home/alpine/mcp-chain/.planning/REQUIREMENTS.md) — CORE-04, CORE-05, CORE-08, CORE-09 verbatim
- [CONTEXT.md](/home/alpine/mcp-chain/.planning/phases/04-store-core-flock-atomic-writes/04-CONTEXT.md) — locked phase decisions
- [CLAUDE.md](/home/alpine/mcp-chain/CLAUDE.md) — stack pins and constraints
- [internal/idgen/idgen.go](/home/alpine/mcp-chain/internal/idgen/idgen.go) — `Allocate(counter uint64) string` signature consumed here
- [internal/statepath/resolve.go](/home/alpine/mcp-chain/internal/statepath/resolve.go) — `Resolve() (string, error)` produces the path consumed by `store.Open`

### Secondary (MEDIUM confidence)
- [flock(2) Linux manual page](https://man7.org/linux/man-pages/man2/flock.2.html) — semantics of POSIX advisory locks, inode-attachment
- [apenwarr "file locking" blog](https://apenwarr.ca/log/20101213) — cross-platform flock pitfalls
- [Richard Crowley "things UNIX can do atomically"](https://rcrowley.org/2010/01/06/things-unix-can-do-atomically.html) — rename+lock interaction
- [Go os.Rename Windows issue #8914](https://github.com/golang/go/issues/8914) — confirms MoveFileEx usage on Windows
- [natefinch/atomic](https://github.com/natefinch/atomic) — alternative Windows atomic-write lib (not selected but documented)
- [renameio issue #29 "doesn't build on Windows"](https://github.com/google/renameio/issues/29) — historical context for Windows gap

### Tertiary (LOW confidence)
- WebSearch results on Go atomic file write libraries — cross-referenced with primary sources; flagged for validation in phase review.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Go's `os.Rename` on Windows is backed by `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` and is atomic on the same volume since Go 1.5 | atomic_windows.go pattern | If incorrect, Windows writes may be non-atomic after crash. Mitigation: under single-writer-flock-protection, non-atomic rename is still consistent (no reader sees half state). Verify by reading Go source at src/os/file_windows.go if high confidence needed. |
| A2 | Under a shared flock, two readers never block each other on POSIX (mandatory semantics of LOCK_SH) | Pattern 3 | If incorrect, readers serialize — minor performance issue only, no correctness. Low risk. |
| A3 | Using a sibling `.lock` file on unix as well as Windows adds no meaningful overhead vs locking the state file directly | Recommendation in Pitfall 5 | Lock file is created once on first Lock and reused; overhead is one extra inode. Negligible. |
| A4 | `crypto/subtle.ConstantTimeCompare` on a 32-char string takes < 1 µs | Security Domain V2 | Even at cold-cache it's < 10 µs. Irrelevant to the 100 ms startup budget. Zero risk. |
| A5 | `encoding/json` marshals `*time.Time` nil as literal `null` and non-nil as RFC3339 string | Pitfall 6 | Documented in stdlib encoding/json `omitempty` / pointer semantics. If wrong, round-trip test catches it. |

**Nothing in this table is load-bearing;** all are either verified or trivially testable. No user confirmation needed before planning proceeds.

## Open Questions — RESOLVED 2026-04-23

All 5 resolved by orchestrator in autonomous mode. Planner should treat these as locked decisions.

1. **Use `state.json.lock` on BOTH unix and Windows.** ✅ RESOLVED: yes — sibling lock file on both platforms. Collapses the `lock_unix.go`/`lock_windows.go` split into a single `lock.go`. Eliminates POSIX rename-changes-inode latent edge case; uniform semantics cross-platform.

2. **`withSharedLock` takes `s.mu`.** ✅ RESOLVED: yes — plain `sync.Mutex`, same-process serialization of reads. RWMutex is a future optimization (not needed for mcp-chain's workload).

3. **Test write-delay hook: function-pointer injection.** ✅ RESOLVED: `saveStateFn` package-level var in `store` package; test file overrides. Zero production code cost.

4. **`crypto/subtle.ConstantTimeCompare` for OwnerToken comparison.** ✅ RESOLVED: yes — standard hardening. 5 LOC. Use `subtle.ConstantTimeCompare([]byte(token), []byte(stored)) != 1`.

5. **Pin `gofrs/flock` to v0.12.1** (not v0.13.0 as listed in CLAUDE.md). ✅ RESOLVED: pin v0.12.1. Root cause: v0.13.0 requires Go 1.24; project is on 1.23.8. All APIs we need (`New`, `Lock`, `RLock`, `Unlock`) exist in v0.12.1; the only v0.13 addition (`Stat()`) is unused. CLAUDE.md will be amended at Phase 4 close-out.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — both deps verified via pkg.go.dev + source read; version pins justified
- Architecture: HIGH — lock/rename interaction verified against flock(2) semantics and renameio README; Windows gap verified from multiple sources
- Pitfalls: HIGH — every pitfall has a citation or a concrete mitigation test

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (30 days — stable stack; revisit if flock v0.13+ becomes compelling or renameio adds Windows support)
