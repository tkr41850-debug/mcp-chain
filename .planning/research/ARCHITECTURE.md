# Architecture Research

**Domain:** Go MCP server + CLI + Claude Code plugin (single-binary dual-mode, shared JSON-file state)
**Researched:** 2026-04-23
**Confidence:** HIGH for Go layout and flock semantics; HIGH for the stdio-session-identity finding; MEDIUM for the exact API surface of the chosen MCP SDK (pinned in STACK.md).

## Standard Architecture

### System Overview

```
 Claude Code session A              Claude Code session B …N
 (registrant)                       (waiters)
 ┌────────────────────────┐         ┌────────────────────────┐
 │ /chain-reg "CI passed" │         │ /chain-wait otter      │
 │   → MCP tool: register │         │   → bash monitor loop  │
 │   → MCP tool: resolve  │         │        │               │
 └───────────┬────────────┘         └────────┼───────────────┘
             │ stdio (JSON-RPC)              │ spawns every 1s
             │                               ▼
 ┌───────────▼────────────┐         ┌────────────────────────┐
 │ mcp-chain serve        │         │ mcp-chain status otter │
 │  (long-lived subproc   │         │  (short-lived CLI;     │
 │   of session A's CC)   │         │   exit 0/1/2)          │
 └───────────┬────────────┘         └────────┬───────────────┘
             │                               │
             │ read-modify-write             │ read-only
             │ with flock(LOCK_EX)           │ with flock(LOCK_SH)
             ▼                               ▼
  ┌─────────────────────────────────────────────────┐
  │  ~/.mcp-chain/state.json  (flock-protected)     │
  │  { version, counter, entries{ id → record } }   │
  └─────────────────────────────────────────────────┘
```

Key properties:
- **Two execution modes, one binary.** `serve` talks MCP on stdio and lives for the whole Claude Code session. `status` is a one-shot CLI that exits fast. `list`, `resolve`, `purge` round out the CLI surface but layer trivially on the same primitives.
- **All cross-process coordination happens through the state file.** No sockets, no daemons. The file is the source of truth; both modes are stateless clients of it.
- **Business logic lives below the MCP wire.** The MCP handler is a thin adapter: unmarshal params → call a plain-Go function on the `store` → marshal result. The same store functions back the CLI subcommands with no MCP types leaking in.

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| `cmd/mcp-chain/main.go` | Argv dispatch: `serve` → MCP server; `status|list|resolve|purge` → CLI. Should be ~50 lines. | stdlib `os.Args` switch or tiny `flag.FlagSet` per subcommand. No third-party CLI framework needed. |
| `internal/store` | Domain model and on-disk persistence. Owns the `State` struct, the file path resolution, atomic read-modify-write under flock, and the pure transition functions (`Register`, `Resolve`, `Get`, `List`, `Purge`). **No MCP or CLI types.** | Plain Go types + `encoding/json` + `golang.org/x/sys/unix.Flock` (and a Windows `LockFileEx` shim via build tags). |
| `internal/idgen` | Allocate the next short ID given a counter. Index into the embedded EFF wordlist for `counter < 1296`, then fall back to deterministic hex suffixes. Pure function: `Allocate(counter uint64) string`. | Single file; stateless. |
| `internal/wordlist` | Ship the EFF short wordlist as an embedded resource; expose as `[]string` or an indexed lookup. | `//go:embed eff_short_wordlist_1.txt` + a `var Words []string` initialized once. |
| `internal/mcpserver` | MCP protocol adapter. Wires the store's methods to MCP tools (`register`, `resolve`, optionally `list`/`status`), defines the tool schemas with terse descriptions, runs the stdio loop. | Thin handlers that call `store.*` and translate errors to MCP error results. |
| `internal/cli` | Subcommand implementations for `status`, `list`, `resolve`, `purge`. Each is a function `func(args []string) int` returning a process exit code. | stdlib only. Output formatters live here (table rendering for `list`). |
| `internal/xdg` | Resolve the state file path per spec (`$XDG_STATE_HOME/mcp-chain/state.json` else `~/.mcp-chain/state.json`) and guarantee the directory exists with 0700. | 20 lines of stdlib. |
| `plugin/` (repo root, not compiled in) | Claude Code plugin packaging: `.claude-plugin/plugin.json`, `commands/*.md`, `.mcp.json`, `scripts/chain-wait.sh`. | Static files shipped alongside the binary. |

**Why `internal/` and not top-level packages:** Go enforces at compile time that nothing outside this module can import `internal/*`. For a single-binary tool, this cleanly signals "these are implementation details" and prevents accidental coupling if the repo ever grows. This matches the overwhelming majority convention for small single-binary Go projects surveyed ([Go docs on module layout](https://go.dev/doc/modules/layout), [Standard Go Project Layout](https://github.com/golang-standards/project-layout)).

## Recommended Project Structure

```
mcp-chain/
├── cmd/
│   └── mcp-chain/
│       └── main.go                       # ~50 LoC: argv dispatch only
├── internal/
│   ├── store/
│   │   ├── store.go                      # Register/Resolve/Get/List/Purge
│   │   ├── state.go                      # State, Record, schema version
│   │   ├── file.go                       # load/save with flock + atomic rename
│   │   ├── flock_unix.go                 # build tag: !windows
│   │   ├── flock_windows.go              # build tag: windows (LockFileEx)
│   │   ├── store_test.go
│   │   └── file_test.go                  # includes cross-process flock test
│   ├── idgen/
│   │   ├── idgen.go                      # Allocate(counter) string
│   │   └── idgen_test.go
│   ├── wordlist/
│   │   ├── wordlist.go                   # //go:embed + exported Words []string
│   │   ├── eff_short_wordlist_1.txt      # 1296 words, copied from EFF
│   │   └── wordlist_test.go              # assert len == 1296, uniqueness
│   ├── mcpserver/
│   │   ├── server.go                     # Run(ctx) — stdio loop
│   │   ├── tools.go                      # register/resolve/list tool defs
│   │   ├── tools_test.go                 # table-driven over the store iface
│   │   └── session.go                    # per-serve-process ownership token
│   ├── cli/
│   │   ├── status.go                     # exit 0/1/2
│   │   ├── list.go                       # human-readable table
│   │   ├── resolve.go                    # mirrors MCP tool (for scripts)
│   │   ├── purge.go                      # id | --all | --resolved
│   │   ├── duration.go                   # shared timeout parser for tests
│   │   └── *_test.go
│   └── xdg/
│       ├── path.go
│       └── path_test.go
├── plugin/                               # Claude Code plugin package
│   ├── .claude-plugin/
│   │   └── plugin.json
│   ├── commands/
│   │   ├── chain-reg.md
│   │   ├── chain-wait.md
│   │   ├── chain-list.md
│   │   └── chain-purge.md
│   ├── scripts/
│   │   └── chain-wait.sh                 # the bash monitor helper
│   └── .mcp.json                         # points at the installed binary
├── .github/workflows/
│   ├── test.yml                          # go test -race on push/PR
│   └── release.yml                       # tag → cross-compile + attach
├── go.mod
├── go.sum
├── README.md
└── PROJECT.md                            # existing
```

### Structure Rationale

- **`cmd/mcp-chain/`:** single binary, so one subdirectory. Even with one binary, `cmd/<name>/` is still the idiomatic location for the entry point — it keeps `main.go` off the module root and makes `go build ./cmd/mcp-chain` unambiguous.
- **Split `internal/store` from `internal/mcpserver`:** this is the single most important boundary. `store` must stay MCP-agnostic so the CLI subcommands can call identical code paths. Tests against `store` don't need to mock an MCP transport.
- **`internal/idgen` and `internal/wordlist` are separate even though tiny:** `idgen` depends on `wordlist`, not the other way around, and `wordlist` is large static data — keeping the `go:embed` isolated means `idgen` tests don't pay for loading 1296 words.
- **`internal/cli` per-subcommand files:** each subcommand is small and easily tested in isolation; grouping them by file mirrors the argv dispatcher in `main.go`.
- **`plugin/` at the repo root, not under `internal/`:** it's shipped content, not compiled. Keeping it visible at root makes the install path obvious and lets users who cloned the repo inspect it directly.

## Architectural Patterns

### Pattern 1: Hexagonal (thin adapters, fat core)

**What:** `internal/store` is the hexagon — pure Go types, plain functions, no knowledge of MCP or CLI. `internal/mcpserver` and `internal/cli` are adapters that translate their respective inputs into store calls and the store's return values into their respective outputs.

**When to use:** any time a single piece of business logic is exposed through more than one surface. This project has exactly that situation (`register` reachable via MCP; `status`/`list`/`resolve`/`purge` reachable via CLI; future surfaces like a web UI would plug in the same way).

**Trade-offs:** one extra layer of function calls. For ~2000 LoC this costs nothing and buys testability: you can exercise 100% of `register → resolve → double-resolve error` without instantiating a JSON-RPC transport or parsing MCP envelopes.

**Interface signatures (the contract):**

```go
// internal/store/store.go — the entire surface area the adapters need.
package store

type Status string
const (
    StatusPending  Status = "pending"
    StatusResolved Status = "resolved"
)

type Record struct {
    ID          string    `json:"id"`
    Condition   string    `json:"condition"`
    Status      Status    `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
    ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
    OwnerToken  string    `json:"owner_token,omitempty"` // see session-link section
}

type Store interface {
    Register(cond string, ownerToken string) (Record, error)
    Resolve(id string, ownerToken string) (Record, error)
    Get(id string) (Record, bool, error)
    List() ([]Record, error)
    Purge(filter PurgeFilter) (n int, err error)
}

type PurgeFilter struct {
    ID        string // if set, purge just this one
    All       bool
    Resolved  bool   // only resolved entries
}

// Sentinel errors the adapters translate into their native error formats.
var (
    ErrUnknownID         = errors.New("unknown id")
    ErrAlreadyResolved   = errors.New("already resolved")
    ErrNotOwner          = errors.New("resolve refused: not the registering session")
)

func Open(path string) (Store, error) // returns a flock-using file-backed Store
```

The MCP handler and the CLI subcommands BOTH depend only on this interface. The `flock` + `encoding/json` + atomic-rename logic lives in the `Open` constructor's concrete type and is invisible to callers.

### Pattern 2: Read–Modify–Write under exclusive flock (the ONLY correct pattern here)

**What:** every mutation is a three-step transaction where the lock is held across all three steps and nothing else:

```go
func (s *fileStore) mutate(fn func(*State) error) error {
    f, err := os.OpenFile(s.lockPath, os.O_RDWR|os.O_CREATE, 0600)
    if err != nil { return err }
    defer f.Close()
    if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil { return err }
    defer unix.Flock(int(f.Fd()), unix.LOCK_UN)

    state, err := loadLocked(s.statePath)                       // read
    if err != nil { return err }
    if err := fn(state); err != nil { return err }              // modify
    return saveAtomicLocked(s.statePath, state)                 // write (temp+rename)
}
```

**When to use:** every `Register`, `Resolve`, `Purge`. For `Get`/`List`, use `LOCK_SH` (shared lock) so concurrent `status` polls don't serialize.

**Trade-offs:**
- Flock serializes writers, which is *correct* for our model and fine at our scale (human-driven register/resolve, maybe a few ops/second).
- The `defer unix.Flock(..., LOCK_UN)` on top of `defer f.Close()` is belt-and-suspenders; closing the fd releases the lock, but explicit release makes the critical section unambiguous.
- **Cross-platform caveat:** Windows uses `LockFileEx` via a syscall wrapper. Put them behind `flock_unix.go` / `flock_windows.go` with matching build tags and a shared interface `type locker interface { Lock() error; Unlock() error }`. The Go issue tracker confirms `os.Rename` is not atomic on Windows when the destination exists — use `golang.org/x/sys/windows.MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` ([issue 8914](https://github.com/golang/go/issues/8914)).

**The critical-section boundary — explicitly:**

> The flock is held from just before loading state to just after the atomic rename completes. It is **never** held across the outer MCP call. The MCP handler receives the request, enters `store.Register`, acquires the lock, does the RMW, releases the lock, and returns. If the handler also does logging or response formatting, that happens OUTSIDE the lock. A misbehaving or slow client cannot stall another process's `mcp-chain status` because the writer's lock hold time is bounded by "load + allocate ID + save".

### Pattern 3: Atomic write via temp-file + rename in the same directory

**What:** never write to `state.json` directly. Write to `state.json.tmp-<pid>-<nanos>` in the same directory, `fsync` it, then `os.Rename` it over `state.json`.

**When to use:** every save. A crash/power-loss between "truncate" and "write complete" on the real file would corrupt state; a crash during the temp-file write simply leaves a stray temp file that the next run can clean up.

**Trade-offs:** one extra file operation per save. At our update rate this is negligible. The temp file must be on the same filesystem as the target for `rename` to be atomic (hence "same directory").

```go
func saveAtomicLocked(path string, s *State) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, ".state-*.json.tmp")
    if err != nil { return err }
    enc := json.NewEncoder(tmp)
    enc.SetIndent("", "  ")
    if err := enc.Encode(s); err != nil {
        tmp.Close(); os.Remove(tmp.Name()); return err
    }
    if err := tmp.Sync(); err != nil { tmp.Close(); os.Remove(tmp.Name()); return err }
    if err := tmp.Close(); err != nil { os.Remove(tmp.Name()); return err }
    return os.Rename(tmp.Name(), path) // Windows: use MoveFileEx via build tag
}
```

### Pattern 4: Schema versioning from day one

**What:** the JSON state file has a top-level `"version": 1` field. Every loader checks the version and refuses unknown versions with a clear error. Migrations (when they eventually exist) bump the number.

**When to use:** always, for any on-disk format. Cheap to add today, impossible to retrofit cleanly.

**Trade-offs:** three extra lines of code. Zero runtime cost.

## Data Flow

### State file schema (proposal)

```json
{
  "version": 1,
  "counter": 42,
  "entries": {
    "otter": {
      "id": "otter",
      "condition": "CI passed on PR #123",
      "status": "pending",
      "created_at": "2026-04-23T14:12:01.123Z",
      "resolved_at": null,
      "owner_token": "srv-7f3a9c2e1b4d8f60"
    },
    "badge": {
      "id": "badge",
      "condition": "migration reviewed",
      "status": "resolved",
      "created_at": "2026-04-23T12:05:00.000Z",
      "resolved_at": "2026-04-23T13:30:15.450Z",
      "owner_token": "srv-1a2b3c4d5e6f7a8b"
    }
  }
}
```

Rationale:
- **Flat object `{id → record}`, not a list.** Lookup is O(1) and the JSON is self-documenting. A list forces every Get/Resolve to scan.
- **Counter is top-level, monotonically increasing, never reused.** Purging an entry does NOT decrement the counter. The counter is the index into the EFF wordlist; reusing counter values would reissue the same word, which breaks the expectation that an ID uniquely names a past or present lock.
- **`resolved_at` nullable; `owner_token` omitempty.** These keep the pending case compact.
- **ISO-8601 RFC 3339 timestamps** are what `time.Time.MarshalJSON` produces by default and are human-readable for debugging.

### Key Data Flows

**1. Register flow** — `/chain-reg "CI passed"` in session A

```
Claude Code A      mcp-chain serve A             state.json
     │                    │                           │
     │ MCP tool call      │                           │
     │─ register(cond) ──▶│                           │
     │                    │ acquire flock LOCK_EX     │
     │                    │──────────────────────────▶│
     │                    │ read state                │
     │                    │◀──────────────────────────│
     │                    │ ctr = state.counter       │
     │                    │ id  = idgen.Allocate(ctr) │
     │                    │ state.counter = ctr+1     │
     │                    │ state.entries[id] = {...} │
     │                    │ write temp + rename       │
     │                    │──────────────────────────▶│
     │                    │ release flock             │
     │ {"id":"otter"}     │                           │
     │◀───────────────────│                           │
```

Flock is held for steps 2–6 only. Typical duration: sub-millisecond at our data size.

**2. Status flow** — `mcp-chain status otter` (run once per second by the bash monitor)

```
bash monitor      mcp-chain status (short-lived)    state.json
     │                    │                               │
     │ exec ─────────────▶│                               │
     │                    │ acquire flock LOCK_SH         │
     │                    │──────────────────────────────▶│
     │                    │ read state                    │
     │                    │◀──────────────────────────────│
     │                    │ release flock                 │
     │                    │ lookup entries["otter"]       │
     │                    │ exit 0/1/2 by status          │
     │ exit code ◀────────│                               │
```

Multiple `status` processes can hold `LOCK_SH` concurrently — readers don't block readers. This is essential because N waiters each poll every second; serializing them would be silly.

**3. Resolve flow** — `resolve("otter")` from session A's MCP handler

Identical to Register, except the `fn` passed to `mutate` looks up the entry, returns `ErrUnknownID` if missing, returns `ErrAlreadyResolved` if status is already resolved, optionally returns `ErrNotOwner` if `owner_token` doesn't match (see next section), else flips status + sets `resolved_at`. Waiters see the new status on their next 1-second poll and print `continue`.

**4. Purge flow** — `mcp-chain purge --resolved`

Same RMW pattern. `fn` iterates entries and deletes those matching the filter. Counter is NOT decremented.

### Session-Link Design (the flagged open question)

**Finding from research:** the official `modelcontextprotocol/go-sdk` stdio transport explicitly returns `""` from `SessionID()` ([source: `mcp/transport.go`](https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/transport.go)). The session-ID mechanism in the MCP spec is defined for HTTP transports only ([MCP transports spec](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)). Mark3labs/mcp-go has the same constraint: per-session state exists in-process, but the stdio transport does not synthesize a stable cross-process session ID.

**However, the architecture still permits ownership enforcement**, because of the deployment shape:

> Each Claude Code session spawns its OWN `mcp-chain serve` subprocess (that's how Claude Code plugins register MCP servers). The subprocess's lifetime maps 1-to-1 onto the Claude Code session's lifetime. Therefore, "the session that called register" is uniquely identified by "the serve-process PID that called register" — we don't need any identity from the MCP layer.

**Primary design (recommended):**

1. At `mcp-chain serve` startup, generate a random 128-bit `OwnerToken` (hex-encoded, prefixed `srv-`). Hold it in process memory only.
2. On `register`, write this token into the record's `owner_token` field.
3. On `resolve`, compare the serve process's current `OwnerToken` against the record's `owner_token`. Mismatch → return `ErrNotOwner`.
4. If session A's `serve` crashes/restarts, its new token won't match the old records. The user then resolves the old IDs via the CLI path (`mcp-chain resolve <id>`), which MUST accept an `--override-owner` flag or simply not enforce ownership (CLI is out-of-band administrative access).

This design enforces the preferred invariant (only the registering session can resolve) WITHOUT requiring MCP-provided identity, and gracefully degrades on crash via the CLI escape hatch.

**Fallback design (if the primary is rejected as too fragile w.r.t. serve restarts):**

Store `owner_token = ""` for all records and accept "any MCP caller can resolve once" — this is the user-approved fallback per PROJECT.md. The `Resolve` interface stays the same; the enforcement check becomes a no-op.

**Recommendation:** ship the primary design. It's ~30 lines of code, adds real safety, and the CLI escape hatch covers the restart edge case. Log the `owner_token` check as a "Key Decision" in PROJECT.md once implemented.

## Build Order

Dependency-ordered bottom-up. Each step can be fully tested before the next begins.

| # | Component | Depends on | Why it's in this position |
|---|-----------|-----------|---------------------------|
| 1 | `internal/wordlist` + `internal/idgen` | nothing | Pure functions over static data. Table-test in isolation: assert 1296 words, all unique, all lowercase, no spaces; `idgen.Allocate(0..1295)` = words[i]; `idgen.Allocate(1296)` = deterministic hex fallback. |
| 2 | `internal/xdg` | nothing | Five minutes of code; tested with env-var manipulation in tests. |
| 3 | `internal/store` | idgen, xdg | The heart of the system. Implement `Open`, `Register`, `Resolve`, `Get`, `List`, `Purge` plus the flock/atomic-rename machinery. Unit tests with `t.TempDir()` for isolation. **Integration test: spawn two goroutines and/or two subprocesses hammering the same file, assert no corruption and no lost updates.** (QA-02 requirement.) |
| 4 | `internal/cli` | store | `status`, `list`, `resolve`, `purge` subcommands. Each is ~30 lines. Test by invoking the function with argv slices and a temp state dir. |
| 5 | `internal/mcpserver` | store | MCP tool definitions (terse descriptions per CORE-07), handler wiring. Test with a table of `(input → expected store call → expected tool result)`. Needs MCP SDK choice from STACK.md locked in first. |
| 6 | `cmd/mcp-chain/main.go` | cli, mcpserver | Argv dispatch. ~50 lines. |
| 7 | `plugin/` | cmd binary | Static files. `plugin.json`, four `commands/*.md`, `.mcp.json` pointing at the installed binary, `scripts/chain-wait.sh`. |
| 8 | `.github/workflows/` | all | `test.yml` (go test -race), `release.yml` (tag triggers cross-compile matrix + release attach). |

**Why this order:** every layer is independently verifiable. If step 3's integration tests catch a flock bug, you've found it with zero MCP machinery in the way. If step 5 breaks, you know the store is solid and the defect is in the wire-protocol adapter.

> **Callout where the MCP SDK choice (STACK.md) shapes the design:** steps 1–4 are SDK-independent. Step 5 (and only step 5) depends on which SDK you pick. If STACK.md locks in `modelcontextprotocol/go-sdk`, tool handlers look like `func(ctx, *CallToolRequest, any) (*CallToolResult, any, error)`. If it picks `mark3labs/mcp-go`, the signature is different but the adapter shape is the same: unmarshal → call `store.Register` → marshal. The hexagonal boundary means the SDK choice is localized to ~200 lines in `internal/mcpserver/` and is swappable.

## Scaling Considerations

This tool is explicitly a single-user local tool (PROJECT.md: "single user local tool"; "Networked / multi-machine coordination" is Out of Scope). The scaling axes that matter here are different from a typical service.

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 1–20 concurrent entries | None. Current design handles this trivially. |
| 20–200 concurrent entries | None. JSON file is ~50KB at 200 entries — still reads in microseconds. |
| 200+ concurrent entries | Consider `/chain-purge --resolved` more aggressively. No code change. If entries ever cross ~10K (not realistic for this use case), the flat-object layout may warrant moving to SQLite — which is explicitly Out of Scope. |
| 1296+ *lifetime* registrations (counter exhaustion) | `idgen` falls back to hex counters (`hex-0001`, `hex-0002`, …). Already in the design. |

### Scaling Priorities

1. **First bottleneck (write contention):** if the user ever saturates the exclusive flock, they'll notice as `register`/`resolve` latency. This is impossible in practice at human interaction rates but is worth knowing. Mitigation: the RMW critical section is sub-millisecond; we literally cannot produce this bottleneck at realistic usage.
2. **Second bottleneck (state file size):** the file is read and rewritten in full on every mutation. At thousands of entries this becomes measurable (tens of milliseconds). Mitigation: purge. If that ever stops being sufficient, the escape hatch is the documented Out-of-Scope: SQLite.

## Anti-Patterns

### Anti-Pattern 1: Holding the flock across the MCP tool call

**What people do:** acquire the flock when `serve` starts, hold it for the life of the process.
**Why it's wrong:** defeats the entire purpose. Other `mcp-chain` processes (and other `serve` instances, one per Claude Code session) cannot make progress. The system only works because the lock's hold time is microseconds.
**Do this instead:** the critical section is exactly "read + compute + write". Enter, transact, leave. Every mutation is independent.

### Anti-Pattern 2: Writing directly to `state.json` (no temp+rename)

**What people do:** `os.WriteFile("state.json", data, 0600)`.
**Why it's wrong:** a crash or OOM during the write leaves a truncated or half-written JSON file. The next read fails, the user's state is gone.
**Do this instead:** write to `state.json.tmp-<pid>-<nanos>` in the same directory, fsync, close, `os.Rename` over the target. On Windows use `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING`.

### Anti-Pattern 3: Leaking MCP types into the store

**What people do:** `store.Register` takes an `*mcp.CallToolRequest` or returns an `*mcp.CallToolResult`.
**Why it's wrong:** the CLI subcommands now can't call it without constructing fake MCP envelopes; tests need MCP scaffolding; swapping MCP SDKs becomes a major refactor instead of a ~200-line diff.
**Do this instead:** `store.Register(cond string, ownerToken string) (Record, error)`. The MCP handler translates; the CLI handler translates; neither depends on the other.

### Anti-Pattern 4: Reusing counter values after purge

**What people do:** `if len(entries) == 0 { counter = 0 }` after a `purge --all`.
**Why it's wrong:** reissues word-IDs that previously named different conditions. Users will see `otter` mean two different things at different times; the bash monitor script's prior output becomes confusing.
**Do this instead:** counter is monotonic forever. Purge deletes entries; it does not touch the counter. If hex fallback kicks in eventually, that's fine.

### Anti-Pattern 5: Relying on `SessionID()` from the MCP SDK for identity

**What people do:** thread `req.GetSession().ID()` into the store as `owner_token`.
**Why it's wrong:** for the official go-sdk stdio transport, `SessionID()` returns `""` — always. Authentication on that value is a no-op.
**Do this instead:** generate a random opaque token once at `serve` startup, hold it in process memory, write it into records on register, compare on resolve. Do not derive it from anything MCP provides on stdio.

### Anti-Pattern 6: Running the `status` CLI under exclusive lock

**What people do:** use `LOCK_EX` for every lock acquisition, including reads.
**Why it's wrong:** N waiters polling once per second each now serialize their reads. On a machine with 10 waiters, you've created a one-lock-per-second bottleneck that is entirely avoidable.
**Do this instead:** `LOCK_SH` for `Get`/`List`/`status`; `LOCK_EX` only for mutations.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Claude Code MCP host | stdio subprocess over JSON-RPC | Spawned by Claude Code per `.mcp.json`. Receives tool calls; returns tool results. No HTTP. |
| Claude Code slash commands | Markdown files under `commands/` | Each `.md` file defines a slash command; the body is the prompt/instruction Claude executes. |
| Claude Code bash monitor (`/chain-wait`) | Spawns `scripts/chain-wait.sh`, which `exec`s `mcp-chain status <id>` in a loop | The bash script is simple: poll, check exit code, sleep, repeat, with optional `--timeout`. Prints `continue` on exit 0. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `cli` ↔ `store` | Direct function calls | CLI translates argv → store calls → stdout + exit code. |
| `mcpserver` ↔ `store` | Direct function calls | MCP handler translates CallToolRequest → store calls → CallToolResult. |
| `store` ↔ filesystem | `os` + `encoding/json` + flock | The only I/O boundary in the system. Isolated behind `store.Open`. |
| `mcpserver` ↔ Claude Code | stdio JSON-RPC via chosen MCP SDK | The only network-protocol-like boundary. All wire-format code lives here. |
| `idgen` ↔ `wordlist` | `wordlist.Words[i]` | One-way; idgen depends on wordlist, not vice versa. |

## Sources

- [Organizing a Go module — go.dev](https://go.dev/doc/modules/layout) — the cmd/ + internal/ convention (HIGH confidence: official docs).
- [Standard Go Project Layout — golang-standards/project-layout](https://github.com/golang-standards/project-layout) — community reference for the same convention (MEDIUM, community).
- [modelcontextprotocol/go-sdk — mcp package](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — official Go MCP SDK API surface (HIGH).
- [modelcontextprotocol/go-sdk transport.go](https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/transport.go) — confirms stdio `SessionID()` returns `""` (HIGH, source code).
- [MCP Transports spec (2025-11-25)](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports) — confirms session-ID mechanism is HTTP-transport-only (HIGH, official spec).
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) — alternative Go SDK, per-session state model (MEDIUM, community).
- [gofrs/flock — thread-safe file locking](https://github.com/gofrs/flock) — cross-platform flock wrapper; also usable directly (MEDIUM).
- [golang.org/x/sys/unix flock example](https://pkg.go.dev/golang.org/x/sys/unix#Flock) — authoritative stdlib-adjacent flock API (HIGH).
- [golang/go issue 8914 — os.Rename atomicity on Windows](https://github.com/golang/go/issues/8914) — confirms Windows needs `MoveFileEx` for atomic replace (HIGH, Go tracker).
- [Claude Code plugins reference](https://code.claude.com/docs/en/plugins-reference) — plugin directory conventions (HIGH, official docs).
- [Go embed package docs](https://pkg.go.dev/embed) — `go:embed` for the EFF wordlist (HIGH, official).

---
*Architecture research for: Go MCP server + CLI + Claude Code plugin*
*Researched: 2026-04-23*
