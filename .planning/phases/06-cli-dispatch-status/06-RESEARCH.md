# Phase 6: CLI Dispatch & Status Subcommand - Research

**Researched:** 2026-04-23
**Domain:** kong subcommand wiring + stdio discipline + cross-process timing validation
**Confidence:** HIGH

## Summary

Phase 6 replaces the `StatusCmd.Run()` stub in `internal/cli/stubs.go` with a real implementation that resolves the state path, opens the store, calls `store.Get(id)`, and emits exit codes 0/1/2 per the locked contract. Research confirms three concrete findings that drive the plan:

1. **kong's default help/usage routing writes to stdout** — empirically verified by building the current Phase-1 binary and running `mcp-chain --help`, `mcp-chain nosuchcmd`, and `mcp-chain status` (no id). All three put the Usage block on stdout. Phase 1's `kong.UsageOnError()` is **not sufficient** to satisfy SC #3; the plan must add `kong.Writers(os.Stderr, os.Stderr)` to `kong.Parse` in `cmd/mcp-chain/main.go`.

2. **Adding `kong.Writers(os.Stderr, os.Stderr)` redirects `VersionFlag` output to stderr too**, which breaks the existing `TestVersionFlagWritesToStdout` test in `internal/cli/stubs_test.go`. The fix is to handle `--version` manually in `main` (pre-parse `os.Args` for `--version`, print to `os.Stdout`, `os.Exit(0)`) and drop the `kong.VersionFlag` field from the CLI grammar. This preserves SC #3 without losing the sanctioned stdout write.

3. **The testable shape `runStatus(out, err io.Writer, path, id string) int` is the correct design.** kong's `ExitCoder` interface can return custom exit codes, but `FatalIfErrorf` also writes `err.Error()` to stderr via `k.Errorf(...)` — so returning an `ExitCoder` error for the pending case would pollute stderr (violating the "nothing on stderr" row of the exit-code contract). Pattern: unit tests call `runStatus` directly with `bytes.Buffer` pairs; `StatusCmd.Run` calls `os.Exit(runStatus(os.Stdout, os.Stderr, path, c.ID))` or returns nil on exit 0.

**Primary recommendation:** Write `internal/cli/status.go` exporting `runStatus(out, errW io.Writer, path, id string) int` with the full exit-code decision tree; `StatusCmd.Run()` wires it via `os.Exit`. Unit-test the decision tree in `status_test.go` with captured writers. Patch `cmd/mcp-chain/main.go` to (a) handle `--version` manually, (b) drop `kong.VersionFlag`, (c) add `kong.Writers(os.Stderr, os.Stderr)`. Add build-tagged `internal/cli/integration_test.go` for end-to-end exit codes, 10-concurrent timing, and help-to-stderr assertions. Reuse the `buildBinary` helper from `stubs_test.go`.

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Exit code contract (locked — Phase 8 bash monitor depends on this):**

| Store result | stdout | stderr | exit |
|--------------|--------|--------|------|
| `record.Status == "resolved"` | `resolved\n` | — | 0 |
| `record.Status == "pending"` | `pending\n` | — | 2 |
| `store.ErrUnknownID` | — | `mcp-chain: unknown id: <id>\n` | 1 |
| other error | — | `mcp-chain: <err>\n` | 1 (generic) |

- **No flags** on `status` in v1. No `--format json`, no `--timeout`, no `--wait`.
- **File layout**: `internal/cli/status.go` (new), `internal/cli/stubs.go` keeps `ListCmd`/`PurgeCmd` stubs, `internal/cli/status_test.go` (unit), `internal/cli/integration_test.go` (integration build tag, spawns binary).
- **Store access path**: `statepath.Resolve()` → `store.Open(path)` → `store.Get(id)` → exit per status.
- **Stdout discipline**: `status` writes a single line to stdout on the 0/2 cases, nothing otherwise. `--help` / bad flags / unknown commands all go to stderr.

### Claude's Discretion

All implementation choices (testability pattern, integration-test structure, how to wire `kong.Writers`). Context.md explicitly recommends the `runStatus(out, err io.Writer, ...) int` form over the simpler `resolveStatus() int` form.

### Deferred Ideas (OUT OF SCOPE)

- `--wait` / `--timeout` flags → Phase 8 bash monitor
- JSON output format → Phase 7 formatters or later
- Distinct exit code for schema-version error → folds into generic 1 for v1
- `list`, `purge`, `resolve` CLI bodies → Phase 7

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-01 | Single Go binary with kong dispatch: `serve`, `status <id>` (exit 0/2/1), `list`, `purge`, `--version`. Phase 1 delivered skeleton; **Phase 6 completes `status`**; Phase 7 completes `list`/`purge`. | Standard Stack (kong v1.15), Architecture Patterns (kong.Writers + manual --version), Exit-code contract decision tree, Integration-test pattern (spawn binary + assert exit code) |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| argv parsing + subcommand dispatch | CLI adapter (`internal/cli`) | entry point (`cmd/mcp-chain/main.go`) | main.go owns `kong.Parse` config; `internal/cli` owns command types and Run bodies |
| state path resolution | `internal/statepath` | — | Phase 3 owns XDG/HOME fallback (CORE-06) |
| state file read under LOCK_SH | `internal/store` | — | Phase 4 owns `Get` and `withSharedLock` |
| exit-code decision (resolved→0, pending→2, unknown/err→1) | `internal/cli` (status.go) | — | Hexagonal: domain boundary is `store.Get`; `runStatus` maps the domain result to an OS-level exit code |
| stdout/stderr routing | `cmd/mcp-chain/main.go` + `internal/cli` | — | main.go configures kong writers; individual commands write explicit payloads via `os.Stdout` / `os.Stderr` |

## Standard Stack

### Core (already declared in CLAUDE.md; Phase 6 adds no new deps)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/alecthomas/kong` | v1.15.0 | subcommand dispatch | declarative struct tags; `Writers`, `UsageOnError`, `ExitCoder` hooks documented in source [VERIFIED: `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/`] |
| stdlib `os/exec` | — | integration tests spawn the compiled binary | standard pattern; already used by `internal/cli/stubs_test.go::buildBinary` and `internal/mcpserver/integration_test.go` |
| `github.com/stretchr/testify/require` | v1.11.1 | fail-fast assertions | project standard per CLAUDE.md [VERIFIED] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `io` | — | `io.Writer` pair on `runStatus` | enables unit tests via `bytes.Buffer`, production uses `os.Stdout`/`os.Stderr` |
| stdlib `sync` | — | `sync.WaitGroup` in timing test | coordinate 10 concurrent child processes |
| stdlib `time` | — | `time.Now()` / `time.Since` for timing assertion | SC #2 (<1s wall) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.Exit(code)` inside `Run()` | return an `ExitCoder` error from `Run()` | `ExitCoder` is sanctioned by kong (see `exit.go` lines 14–18, 23–32), BUT `FatalIfErrorf` calls `k.Errorf("%s", err.Error())` before `Exit`, meaning the pending-case "nothing on stderr" row is violated. **Stay with `os.Exit`.** [VERIFIED: kong.go:462–483] |
| `runStatus(out, err io.Writer, ...) int` | `(c *StatusCmd) resolveStatus() int` method | Method form forces tests to construct a `StatusCmd{ID: ...}` and capture `os.Stdout`/`os.Stderr` globally (flaky in parallel tests). Function with explicit writers is trivially parallel-safe. [CITED: CONTEXT.md §Testability pattern] |
| `kong.Writers(os.Stderr, os.Stderr)` + drop `VersionFlag` | patch `kong.VersionFlag` at runtime | `VersionFlag` is `bool`-typed with a `BeforeReset` hook that hard-codes `app.Stdout` [VERIFIED: util.go:34]. No override point; monkey-patching `k.Stdout` post-parse is too late. Manual `--version` pre-parse is the clean answer. |
| re-exec test binary via env var | build `mcp-chain` once in TestMain, spawn that binary | Re-exec (`internal/mcpserver/integration_test.go` pattern) couples tests to internal APIs. Building the real binary exercises kong dispatch end-to-end. `internal/cli/stubs_test.go::buildBinary` already exists. **Reuse it.** |

**Installation:** No new deps; all imports already in `go.mod`.

**Version verification:** Not needed — stack unchanged from CLAUDE.md / Phase 5. Kong v1.15.0 source is present at `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/`.

## Architecture Patterns

### System Architecture Diagram

```
                                argv
                                  │
                                  ▼
                    ┌───────────────────────────┐
                    │ cmd/mcp-chain/main.go     │
                    │                           │
                    │  1. log→stderr (MCP-02)   │
                    │  2. pre-parse --version → │
                    │     os.Stdout, Exit(0)    │
                    │  3. kong.Parse(           │
                    │       Writers(            │
                    │         os.Stderr,        │
                    │         os.Stderr),       │
                    │       UsageOnError())     │
                    │  4. kctx.Run()            │
                    └────────────┬──────────────┘
                                 │ dispatch to StatusCmd
                                 ▼
                    ┌───────────────────────────┐
                    │ internal/cli/status.go    │
                    │                           │
                    │  StatusCmd.Run():         │
                    │   code := runStatus(      │
                    │     os.Stdout,            │
                    │     os.Stderr,            │
                    │     statepath.Resolve(),  │
                    │     c.ID)                 │
                    │   if code != 0 {          │
                    │     os.Exit(code) }       │
                    │   return nil              │
                    └────────────┬──────────────┘
                                 │
                                 ▼
                    ┌───────────────────────────┐
                    │ runStatus(out,errW,p,id)  │
                    │                           │
                    │  st, err := store.Open(p) │
                    │  if err: errW + "1"       │
                    │  r, err := st.Get(id)     │
                    │  switch {                 │
                    │   case err == nil &&      │
                    │        r.Status=="resolved":│
                    │     out.Write("resolved\n")│
                    │     return 0              │
                    │   case err == nil &&      │
                    │        r.Status=="pending":│
                    │     out.Write("pending\n")│
                    │     return 2              │
                    │   case Is(err,ErrUnknownID):│
                    │     errW.Write(            │
                    │       "mcp-chain: unknown  │
                    │        id: <id>\n")        │
                    │     return 1              │
                    │   default:                 │
                    │     errW.Write(            │
                    │       "mcp-chain: <err>\n")│
                    │     return 1 }            │
                    └────────────┬──────────────┘
                                 │ LOCK_SH
                                 ▼
                    ┌───────────────────────────┐
                    │ internal/store.Get(id)    │
                    │  Phase 4 delivery         │
                    └───────────────────────────┘
```

Reader can trace: (1) argv enters `main`, (2) main either handles `--version` directly or hands off to kong, (3) kong dispatches `StatusCmd.Run`, (4) Run calls `runStatus` which owns the decision tree, (5) `store.Get` under LOCK_SH returns the record or `ErrUnknownID`.

### Recommended File Layout

```
cmd/mcp-chain/
├── main.go              # PATCH: add --version pre-parse, add kong.Writers, drop VersionFlag
└── main_test.go         # existing; no change expected

internal/cli/
├── stubs.go             # REMOVE StatusCmd body+struct (move to status.go); keep ServeCmd/ListCmd/PurgeCmd
├── stubs_test.go        # UPDATE: TestStubsExitCodes drops "status" row (it's no longer a stub);
│                        #   TestVersionFlagWritesToStdout stays green (manual --version still stdout)
├── status.go            # NEW: StatusCmd type, StatusCmd.Run, runStatus(out, errW, path, id) int
├── status_test.go       # NEW: unit tests for runStatus decision tree via bytes.Buffer writers
└── integration_test.go  # NEW: //go:build integration; spawns binary; table-driven exit codes;
                         #   10-concurrent timing; --help/bad-args → stderr
```

### Pattern 1: The `runStatus` function

**What:** A pure-in/pure-out function that takes writers + path + id, returns the exit code. No package-level globals touched.

**When to use:** This is the canonical shape for exit-code-driven CLI commands in this codebase going forward. Phase 7's `list` / `purge` / `resolve` bodies should mirror it (`runList`, `runPurge`, `runResolve`).

**Example:**
```go
// Source: /home/alpine/mcp-chain/internal/cli/status.go (new in Phase 6)
package cli

import (
    "errors"
    "fmt"
    "io"
    "os"

    "github.com/anthropics/mcp-chain/internal/statepath"
    "github.com/anthropics/mcp-chain/internal/store"
)

// StatusCmd reports the status of a registered id.
//
// Exit codes (locked contract, bash monitor depends on these):
//   0  id resolved
//   2  id pending (intentional: shell `if mcp-chain status $id; then ...`
//                  gets intuitive semantics — only resolved triggers the
//                  `then` branch)
//   1  unknown id, or any other error
type StatusCmd struct {
    ID string `arg:"" help:"Id to check."`
}

func (c *StatusCmd) Run() error {
    path, err := statepath.Resolve()
    if err != nil {
        fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
        os.Exit(1)
    }
    code := runStatus(os.Stdout, os.Stderr, path, c.ID)
    if code != 0 {
        os.Exit(code)
    }
    return nil
}

// runStatus is the exit-code decision tree, split from Run for
// unit-testability with captured io.Writer pairs.
// Pre-condition: path is a resolved state-file path (parent dir exists).
func runStatus(out, errW io.Writer, path, id string) int {
    st, err := store.Open(path)
    if err != nil {
        fmt.Fprintf(errW, "mcp-chain: %v\n", err)
        return 1
    }
    r, err := st.Get(id)
    switch {
    case err == nil && r.Status == "resolved":
        fmt.Fprintln(out, "resolved")
        return 0
    case err == nil && r.Status == "pending":
        fmt.Fprintln(out, "pending")
        return 2
    case errors.Is(err, store.ErrUnknownID):
        fmt.Fprintf(errW, "mcp-chain: unknown id: %s\n", id)
        return 1
    default:
        fmt.Fprintf(errW, "mcp-chain: %v\n", err)
        return 1
    }
}
```

Note: `statepath.Resolve()` is called from `Run`, not from `runStatus`. This keeps `runStatus` pure (no env-var dependency) so tests can pass a `t.TempDir()`-local path directly.

### Pattern 2: Manual `--version` pre-parse in main

**What:** Scan `os.Args[1:]` for `--version` / `-v` before calling `kong.Parse`. Emit the version string to `os.Stdout`, `os.Exit(0)`. Drop `kong.VersionFlag` from the CLI struct.

**When to use:** Whenever the CLI must redirect kong's help/usage to stderr (SC #3) while preserving a single sanctioned stdout-writing command (the `--version` flag).

**Example:**
```go
// Source: /home/alpine/mcp-chain/cmd/mcp-chain/main.go (patched in Phase 6)
package main

import (
    "fmt"
    "log"
    "log/slog"
    "os"

    "github.com/alecthomas/kong"

    "github.com/anthropics/mcp-chain/internal/cli"
)

var version = "dev"

type CLI struct {
    // VersionFlag removed — handled manually in main to keep --version on stdout
    // while kong.Writers routes all other kong output to stderr.
    Serve  cli.ServeCmd  `cmd:"" help:"Run MCP stdio server."`
    Status cli.StatusCmd `cmd:"" help:"Check status of an id. Exits 0 resolved, 2 pending, 1 unknown."`
    List   cli.ListCmd   `cmd:"" help:"List all registered ids."`
    Purge  cli.PurgeCmd  `cmd:"" help:"Purge entries. Requires one of <id>, --all, or --resolved."`
}

func main() {
    // MCP-02 stdout discipline (see PITFALLS.md #1).
    log.SetOutput(os.Stderr)
    slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

    // Manual --version so it remains the single sanctioned stdout write.
    for _, a := range os.Args[1:] {
        if a == "--version" {
            fmt.Fprintln(os.Stdout, "mcp-chain "+version)
            os.Exit(0)
        }
    }

    var root CLI
    kctx := kong.Parse(&root,
        kong.Name("mcp-chain"),
        kong.Description("Chain Claude Code sessions via shared register/wait/resolve locks."),
        kong.Writers(os.Stderr, os.Stderr), // SC #3: help/usage/errors → stderr
        kong.UsageOnError(),
    )
    kctx.FatalIfErrorf(kctx.Run())
}
```

### Anti-Patterns to Avoid

- **Returning an `ExitCoder` error from `Run()` for the pending case.** kong's `FatalIfErrorf` writes `err.Error()` to stderr via `k.Errorf` before exiting. Result: `pending` case writes "mcp-chain: error: pending\n" to stderr, violating the "—" (empty) stderr column of the exit-code contract. [VERIFIED: kong.go:462–483]
- **Using `kong.Exit(func(code int) { ... })` override in production.** That's a test-only hook to make `Exit` mockable; production wants real process termination. The source comment says as much: "useful for testing or interactive use" (`options.go:50`).
- **Calling `statepath.Resolve()` inside `runStatus`.** Couples the unit-testable function to env vars (`$XDG_STATE_HOME`, `$HOME`). Resolve in `Run`, pass the path in.
- **Using `t.Setenv("XDG_STATE_HOME", ...)` without then rebuilding the binary per case.** Setting env var in the parent test only affects the parent; integration test children must receive it via `exec.Command.Env`.
- **Relying on `kong.UsageOnError()` alone for SC #3.** Empirically, with `UsageOnError()` alone, the Usage block still goes to stdout on parse error. Must also set `kong.Writers(os.Stderr, os.Stderr)`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| argv parsing | custom switch-on-`os.Args[1]` | kong (already wired in Phase 1) | Phase 1 delivered it; don't regress |
| state file path resolution | `os.UserHomeDir()` + XDG logic | `statepath.Resolve()` (Phase 3) | XDG/HOME edge cases already covered |
| shared-lock read | `flock` + `json.Unmarshal` | `store.Get(id)` (Phase 4) | already uses `withSharedLock`; Phase 6 just calls it |
| building the test binary | manual `go build` in each test | `buildBinary(t)` from `stubs_test.go` | already exists; reuse |
| cross-process timing test | `bash` script with `xargs -P10` | Go integration test with `sync.WaitGroup` + `exec.Command` | portable (Windows CI), diagnosable on failure, same testing package |

**Key insight:** Phase 6 is a wiring phase. Every building block is already in place (kong in Phase 1, statepath in Phase 3, store.Get in Phase 4, buildBinary helper in stubs_test.go). The only fresh code is (a) the `runStatus` decision tree, (b) the writer/version patch to main.go, (c) the integration tests.

## Runtime State Inventory

Not applicable — Phase 6 is a greenfield wiring phase (no rename / refactor / migration). No stored data, no live service config, no OS-registered state, no secrets, no build artifacts carry forward state that depends on this phase's changes.

**Stored data:** None — `state.json` schema and contents unchanged. Status is a read-only observer.
**Live service config:** None — no external services reference `status` by name.
**OS-registered state:** None — no registered OS-level hook names change.
**Secrets/env vars:** None — `$XDG_STATE_HOME` and `$HOME` are read via `statepath.Resolve()` which is unchanged; no new env var introduced.
**Build artifacts:** None — `go build` produces a fresh binary per CI run; no egg-info / cache / symlink to reinstall.

## Common Pitfalls

### Pitfall 1: kong's default help/usage output goes to stdout, not stderr

**What goes wrong:** `mcp-chain --help`, `mcp-chain zzz` (unknown subcommand), `mcp-chain status` (missing `<id>`) all print the Usage block to stdout. Any script wrapper piping `mcp-chain status $id | grep resolved` gets false positives (Usage contains the word "resolved" in `Exits 0 resolved...`).

**Why it happens:** `kong.FatalIfErrorf` calls `parseErr.Context.printHelp(...)` which routes through `ctx.Stdout`, and `ctx.Stdout` = `k.Stdout` = `os.Stdout` by default. [VERIFIED: kong.go:476, kong.go:85, help.go:120]

**How to avoid:** Add `kong.Writers(os.Stderr, os.Stderr)` in `main.go`'s `kong.Parse` call. Handle `--version` manually (pre-parse `os.Args`) to preserve the sanctioned stdout write.

**Warning signs:** `TestBadArgsGoesToStderr` fails in CI; `cat <(mcp-chain status) >/dev/null` produces output on the terminal (stdout).

### Pitfall 2: `ExitCoder` error from `Run()` pollutes stderr

**What goes wrong:** Using kong's `ExitCoder` interface to return a `{code: 2}` error for the pending case causes `FatalIfErrorf` to write `"mcp-chain: error: pending\n"` to stderr before exiting. The pending-case stderr column in the exit-code contract is `—` (empty).

**Why it happens:** `FatalIfErrorf` unconditionally calls `k.Errorf("%s", err.Error())` before `k.Exit(exitCodeFromError(err))`. [VERIFIED: kong.go:482–483]

**How to avoid:** Use the `runStatus(out, errW io.Writer, ...) int` pattern. `StatusCmd.Run` calls `os.Exit(code)` for the 1/2 cases, `return nil` for 0. Don't implement `ExitCoder`.

**Warning signs:** `TestRunStatus_Pending_Exit2` passes but integration test `TestStatus_IntegrationExitCodes` shows stderr contains "error: pending" for the pending row.

### Pitfall 3: `statepath.Resolve()` inside the tested function couples env vars to tests

**What goes wrong:** If `runStatus` calls `statepath.Resolve()` internally, unit tests must set `t.Setenv("XDG_STATE_HOME", ...)` per case, which is process-global and breaks parallelism.

**Why it happens:** `statepath.Resolve()` reads `os.Getenv("XDG_STATE_HOME")` and `os.Getenv("HOME")` at call time. [VERIFIED: resolve.go:52–62]

**How to avoid:** `runStatus(out, errW io.Writer, path, id string) int` takes the resolved `path` as a parameter. `Run` calls `statepath.Resolve()` and passes the result in. Unit tests provide `t.TempDir()+"/state.json"` directly.

**Warning signs:** Flaky test failures when running `go test -race -parallel 4 ./internal/cli/...`.

### Pitfall 4: Timing test flakiness from cold-start overhead

**What goes wrong:** The 10-concurrent timing test spawns `go build`-ed binary 10 times. First-spawn overhead (fork/exec + dynamic loader) on a cold CI runner can hit 50–200 ms per process. With 10 spawned serially from a WaitGroup, wall-clock can approach 1 s independent of file-lock contention, producing false positives.

**Why it happens:** The test is meant to prove LOCK_SH doesn't serialize, but measures fork/exec overhead + LOCK_SH + file read + stdout print + exit. If any of these components is slow on the runner, the test fails for reasons unrelated to the lock.

**How to avoid:**
- Launch all 10 children **in parallel** (goroutines calling `cmd.Start()` concurrently via `sync.WaitGroup`; then `cmd.Wait()` each). `cmd.Start` returns immediately once the child is forked.
- Use `require.Less(t, elapsed, 1*time.Second)` with `t.Logf("elapsed: %v", elapsed)` so CI failures include the actual duration.
- If CI runners are consistently slow, raise the threshold to 2s — SC #2 says "under one second" but the *point* is "no serialization", not a specific number. Document the threshold decision in the test file comment.

**Warning signs:** Timing test passes on dev laptop, fails on CI, succeeds on retry.

### Pitfall 5: Forgetting to drop `kong.VersionFlag` when adding `kong.Writers`

**What goes wrong:** Phase 1's main.go has `Version kong.VersionFlag` in the CLI struct. Adding `kong.Writers(os.Stderr, os.Stderr)` makes `--version` output go to stderr (because `VersionFlag.BeforeReset` writes to `app.Stdout` which is now `os.Stderr`). `TestVersionFlagWritesToStdout` fails with "expected version string on stdout; got empty".

**Why it happens:** `VersionFlag` is defined in kong at `util.go:30–37`; its `BeforeReset` hard-codes `fmt.Fprintln(app.Stdout, vars["version"])`. No override point exists. [VERIFIED: util.go:34]

**How to avoid:** In the same patch that adds `kong.Writers`, (a) drop `Version kong.VersionFlag` from the `CLI` struct, (b) drop `kong.Vars{"version": ...}`, (c) pre-parse `os.Args[1:]` for `--version` in `main`, write to `os.Stdout`, `os.Exit(0)`. Existing `TestVersionFlagWritesToStdout` stays green because it asserts substring match, not kong-specific format.

**Warning signs:** `TestVersionFlagWritesToStdout` red after main.go patch.

### Pitfall 6: Integration test leaks state file across cases

**What goes wrong:** Table-driven integration test cases share a single state file, so the resolved-in-case-1 record bleeds into case-2's "pending" expectation.

**Why it happens:** Phase 4 store persists to the path passed in; cases are additive unless isolated.

**How to avoid:** Each `t.Run(tt.name, ...)` closure calls `t.TempDir()` for its own dir, sets `XDG_STATE_HOME` on the child's `cmd.Env`, and seeds state via `store.Open(path) + st.Register(...) + st.Resolve(...)` from the parent test process. Do not share state across cases.

**Warning signs:** Tests pass individually (`go test -run One`), fail in the table run.

## Code Examples

### Example 1: Unit test for resolved-case exit 0

```go
// Source: /home/alpine/mcp-chain/internal/cli/status_test.go (new in Phase 6)
// Pattern: feed a pre-populated state file path, capture writers, assert exit code + wire text.
func TestRunStatus_Resolved_Exit0(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "state.json")
    st, err := store.Open(path)
    require.NoError(t, err)
    id, err := st.Register("owner-token", "cond")
    require.NoError(t, err)
    require.NoError(t, st.Resolve(id, "owner-token", store.ResolveOptions{}))

    var out, errW bytes.Buffer
    code := cli.RunStatus(&out, &errW, path, id) // RunStatus exported for tests via export_test.go

    require.Equal(t, 0, code)
    require.Equal(t, "resolved\n", out.String())
    require.Empty(t, errW.String())
}
```

### Example 2: Unit test for pending-case exit 2

```go
// Source: /home/alpine/mcp-chain/internal/cli/status_test.go (new in Phase 6)
func TestRunStatus_Pending_Exit2(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "state.json")
    st, _ := store.Open(path)
    id, _ := st.Register("owner-token", "cond") // leave pending

    var out, errW bytes.Buffer
    code := cli.RunStatus(&out, &errW, path, id)

    require.Equal(t, 2, code)
    require.Equal(t, "pending\n", out.String())
    require.Empty(t, errW.String(), "SC #3: pending case writes NOTHING to stderr")
}
```

### Example 3: Unit test for unknown-id exit 1

```go
// Source: /home/alpine/mcp-chain/internal/cli/status_test.go (new in Phase 6)
func TestRunStatus_Unknown_Exit1(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "state.json")

    var out, errW bytes.Buffer
    code := cli.RunStatus(&out, &errW, path, "nonexistent")

    require.Equal(t, 1, code)
    require.Empty(t, out.String(), "SC #3: unknown case writes NOTHING to stdout")
    require.Equal(t, "mcp-chain: unknown id: nonexistent\n", errW.String())
}
```

### Example 4: Export for test

```go
// Source: /home/alpine/mcp-chain/internal/cli/export_test.go (new in Phase 6)
// Exports runStatus under the name RunStatus to xtests without making it public.
package cli

import "io"

var RunStatus = func(out, errW io.Writer, path, id string) int {
    return runStatus(out, errW, path, id)
}
```

### Example 5: Integration test scaffold (binary spawn + exit-code table)

```go
// Source: /home/alpine/mcp-chain/internal/cli/integration_test.go (new in Phase 6)
//go:build integration

package cli_test

import (
    "bytes"
    "os/exec"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/anthropics/mcp-chain/internal/store"
)

func TestStatus_IntegrationExitCodes(t *testing.T) {
    binPath := buildBinary(t) // reused from stubs_test.go

    cases := []struct {
        name     string
        setup    func(t *testing.T, path string) string // returns id to query
        wantExit int
        wantOut  string
        wantErr  string // substring
    }{
        {"resolved", func(t *testing.T, p string) string {
            st, _ := store.Open(p)
            id, _ := st.Register("tok", "c")
            require.NoError(t, st.Resolve(id, "tok", store.ResolveOptions{}))
            return id
        }, 0, "resolved\n", ""},
        {"pending", func(t *testing.T, p string) string {
            st, _ := store.Open(p)
            id, _ := st.Register("tok", "c")
            return id
        }, 2, "pending\n", ""},
        {"unknown", func(t *testing.T, p string) string { return "nonexistent" },
            1, "", "unknown id"},
    }

    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            dir := t.TempDir()
            path := filepath.Join(dir, "mcp-chain", "state.json")
            // Ensure parent dir exists for direct store.Open before the CLI does MkdirAll
            _ = os.MkdirAll(filepath.Dir(path), 0o700)
            id := tt.setup(t, path)

            cmd := exec.Command(binPath, "status", id)
            cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
            var out, errW bytes.Buffer
            cmd.Stdout = &out
            cmd.Stderr = &errW
            err := cmd.Run()

            gotExit := 0
            if exitErr, ok := err.(*exec.ExitError); ok {
                gotExit = exitErr.ExitCode()
            }
            require.Equal(t, tt.wantExit, gotExit, "stderr=%q", errW.String())
            require.Equal(t, tt.wantOut, out.String())
            if tt.wantErr != "" {
                require.Contains(t, errW.String(), tt.wantErr)
            } else {
                require.Empty(t, errW.String())
            }
        })
    }
}
```

Note: `statepath.Resolve()` joins `$XDG_STATE_HOME/mcp-chain/state.json`, so setting `XDG_STATE_HOME=dir` puts the state file at `dir/mcp-chain/state.json`. The setup function must seed that exact path.

### Example 6: 10-concurrent timing test

```go
// Source: /home/alpine/mcp-chain/internal/cli/integration_test.go (new in Phase 6)
func TestStatus_Concurrent10WithinOneSecond(t *testing.T) {
    binPath := buildBinary(t)

    dir := t.TempDir()
    statePath := filepath.Join(dir, "mcp-chain", "state.json")
    require.NoError(t, os.MkdirAll(filepath.Dir(statePath), 0o700))
    st, err := store.Open(statePath)
    require.NoError(t, err)
    id, err := st.Register("tok", "c")
    require.NoError(t, err)
    require.NoError(t, st.Resolve(id, "tok", store.ResolveOptions{}))

    const N = 10
    var wg sync.WaitGroup
    errs := make([]error, N)
    start := time.Now()
    for i := 0; i < N; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            cmd := exec.Command(binPath, "status", id)
            cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
            errs[i] = cmd.Run() // should exit 0
        }(i)
    }
    wg.Wait()
    elapsed := time.Since(start)
    t.Logf("10 concurrent status probes: elapsed=%v", elapsed)
    for i, e := range errs {
        require.NoError(t, e, "child %d failed", i)
    }
    require.Less(t, elapsed, 1*time.Second,
        "SC #2: LOCK_SH must not serialize — 10 concurrent reads should complete in <1s")
}
```

### Example 7: Help/bad-args → stderr assertion

```go
// Source: /home/alpine/mcp-chain/internal/cli/integration_test.go (new in Phase 6)
func TestHelpGoesToStderrNotStdout(t *testing.T) {
    binPath := buildBinary(t)
    cmd := exec.Command(binPath, "--help")
    var out, errW bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &errW
    _ = cmd.Run() // kong exits 0 on --help
    require.Empty(t, out.String(), "SC #3: --help MUST NOT write to stdout")
    require.Contains(t, errW.String(), "Usage:", "--help must print usage on stderr")
}

func TestBadArgsGoesToStderr(t *testing.T) {
    binPath := buildBinary(t)
    cmd := exec.Command(binPath, "status") // missing <id>
    var out, errW bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &errW
    err := cmd.Run()
    exitErr, _ := err.(*exec.ExitError)
    require.NotNil(t, exitErr, "missing arg must exit non-zero")
    require.Empty(t, out.String(), "SC #3: bad args MUST NOT write to stdout")
    require.Contains(t, errW.String(), "expected \"<id>\"")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `kong.UsageOnError()` alone | `kong.UsageOnError()` + `kong.Writers(os.Stderr, os.Stderr)` | discovered empirically in Phase 6 research (2026-04-23) | help/usage text no longer leaks to stdout |
| `kong.VersionFlag` field | manual `--version` pre-parse in main | same | keeps `--version` on stdout (single sanctioned write) while kong output goes to stderr |
| `StatusCmd.Run()` stub with `os.Exit(3)` | `runStatus(out, errW, path, id) int` + `os.Exit(code)` in Run | Phase 6 | implements the 0/1/2 exit-code contract |

**Deprecated/outdated:** `ExitCodeNotImplemented = 3` in `stubs.go` — still used by `ListCmd`/`PurgeCmd` (Phase 7 targets). The constant stays; only `StatusCmd` migrates off it.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| — | none | — | — |

All claims in this research are either VERIFIED against the kong source at `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/` and the local mcp-chain sources, or empirically confirmed by building and running the Phase 1 binary. No user confirmation needed.

## Open Questions

All resolved 2026-04-24 (autonomous mode — defaults applied).

1. **Should `runStatus` differentiate `ErrSchemaVersion` / `ErrCorruptJSON` from other errors?** → **RESOLVED: NO.** Locked in CONTEXT.md §Non-goals. Default branch (`fmt.Fprintf(errW, "mcp-chain: %v\n", err)`) surfaces `store.loadState`'s wrapped recovery hint naturally.

2. **Should the timing test assert `< 1s` or `< 2s` on CI?** → **RESOLVED: `< 1s`.** Matches SC #2 literal wording. If CI flakes land, Phase 9 CI task can raise the threshold with a comment; not worth pre-relaxing without evidence.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `buildBinary` in tests; `go build` in CI | ✓ | 1.25.0 (at `/home/alpine/go-sdk/go/bin/go`) [VERIFIED] | — |
| `github.com/alecthomas/kong` | main.go + StatusCmd | ✓ | v1.15.0 [VERIFIED: `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/`] | — |
| `github.com/stretchr/testify/require` | all tests | ✓ | per go.mod [VERIFIED: imports in existing tests] | — |
| `internal/statepath.Resolve()` | StatusCmd.Run | ✓ | Phase 3 delivered | — |
| `internal/store.Open`, `store.Get`, `store.ErrUnknownID` | runStatus | ✓ | Phase 4 delivered | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `github.com/stretchr/testify/require` |
| Config file | none (Go stdlib testing is configless) |
| Quick run command | `go test ./internal/cli/... -count=1 -timeout 30s` |
| Full suite command | `go test -race -count=1 -timeout 120s ./...` |
| Integration suite | `go test -tags integration -race -count=1 -timeout 120s ./internal/cli/...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CORE-01 (status resolved→0) | `status <resolved-id>` exits 0 with `resolved\n` on stdout, nothing on stderr | unit | `go test ./internal/cli/ -run TestRunStatus_Resolved_Exit0 -count=1` | Wave 0 |
| CORE-01 (status pending→2) | `status <pending-id>` exits 2 with `pending\n` on stdout, nothing on stderr | unit | `go test ./internal/cli/ -run TestRunStatus_Pending_Exit2 -count=1` | Wave 0 |
| CORE-01 (status unknown→1) | `status <bogus>` exits 1 with `mcp-chain: unknown id: <id>\n` on stderr, nothing on stdout | unit | `go test ./internal/cli/ -run TestRunStatus_Unknown_Exit1 -count=1` | Wave 0 |
| SC #3 (stdout purity) | resolved-case stdout is `resolved\n` exactly; unknown-case stdout is empty | unit | `go test ./internal/cli/ -run TestRunStatus_StdoutIsJustStatus -count=1` | Wave 0 |
| SC #3 (stderr for error cases) | unknown-id stderr matches exact format | unit | `go test ./internal/cli/ -run TestRunStatus_UnknownWritesToStderr -count=1` | Wave 0 |
| CORE-01 (full CLI dispatch) | compiled binary dispatches `status <id>` end-to-end; exit codes propagate via `cmd.ProcessState.ExitCode()` | integration | `go test -tags integration ./internal/cli/ -run TestStatus_IntegrationExitCodes -count=1` | Wave 0 |
| SC #2 (LOCK_SH no serialization) | 10 parallel `status` processes complete in <1s wall-clock | integration | `go test -tags integration ./internal/cli/ -run TestStatus_Concurrent10WithinOneSecond -count=1` | Wave 0 |
| SC #3 (--help → stderr) | `mcp-chain --help` writes Usage to stderr, nothing to stdout | integration | `go test -tags integration ./internal/cli/ -run TestHelpGoesToStderrNotStdout -count=1` | Wave 0 |
| SC #3 (bad args → stderr) | `mcp-chain status` (no id) writes parse error and Usage to stderr, nothing to stdout | integration | `go test -tags integration ./internal/cli/ -run TestBadArgsGoesToStderr -count=1` | Wave 0 |
| Regression | `mcp-chain --version` still writes version to stdout after main.go patch | unit (existing) | `go test ./internal/cli/ -run TestVersionFlagWritesToStdout -count=1` | ✅ already green in Phase 1 |
| Regression | list/purge stubs still exit 3 after stubs.go edit | unit (existing, updated) | `go test ./internal/cli/ -run TestStubsExitCodes -count=1` | ✅ exists; row "status" removed |

### Sampling Rate

- **Per task commit:** `go test ./internal/cli/ -count=1 -timeout 30s` (unit tests only, ~1–2s)
- **Per wave merge:** `go test -race -count=1 ./...` (full unit suite, ~5–10s); plus `go test -tags integration -race -count=1 ./internal/cli/...` (integration, ~15–30s with 10-concurrent timing test)
- **Phase gate:** Full `go test -race -count=1 ./...` + `go test -tags integration -race -count=1 ./...` green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/cli/status.go` — new file, houses `StatusCmd` + `runStatus`
- [ ] `internal/cli/status_test.go` — new file, unit tests for the 3 exit-code branches + stdout/stderr purity
- [ ] `internal/cli/export_test.go` — new file, exports `runStatus` as `RunStatus` for xtests
- [ ] `internal/cli/integration_test.go` — new file with `//go:build integration`; table-driven exit codes, 10-concurrent timing, help-to-stderr, bad-args-to-stderr
- [ ] `internal/cli/stubs.go` — edit: remove `StatusCmd` struct + Run (now in status.go)
- [ ] `internal/cli/stubs_test.go` — edit: remove the `"status"` row from `TestStubsExitCodes` (no longer a stub)
- [ ] `cmd/mcp-chain/main.go` — edit: add `--version` pre-parse, drop `kong.VersionFlag` + `kong.Vars{"version": ...}`, add `kong.Writers(os.Stderr, os.Stderr)`

No framework install needed; all deps already in `go.mod`.

## Security Domain

**Applies:** No new security surface in Phase 6.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | status is a read-only observer; no token check (Phase 5 owns OwnerToken for `resolve`) |
| V3 Session Management | no | no session state introduced |
| V4 Access Control | no | status returns the record's status field regardless of caller; not a privileged op |
| V5 Input Validation | partial | `id` is a positional arg with no schema validation beyond kong string-typing. `store.Get` rejects unknown ids with `ErrUnknownID`. Adversarial input (empty string, path traversal, shell metachars) flows through to `store.Get` which looks up in a `map[string]record` — no injection surface. [VERIFIED: store.go:120–131] |
| V6 Cryptography | no | no crypto in status path |

### Known Threat Patterns for Go CLI + JSON state file

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Symlink attack on state.json | Tampering | `renameio/v2` atomic writes prevent partial files; `LOCK_SH` read is a simple `os.ReadFile` on the same path `Register/Resolve` writes. Phase 3 `statepath.Resolve` creates parent dir with 0700. |
| Disclosure of owner_token via stdout leak | Information disclosure | status does NOT print `OwnerToken` — it only prints `r.Status`. The `Record` struct contains `OwnerToken`, but `runStatus` never references that field. [VERIFIED: pattern 1 code example] |
| Denial of service via huge state.json | DoS | stdlib `encoding/json` + `os.ReadFile` read the whole file; at expected entry counts (≤100s per CLAUDE.md) this is a non-issue. Outside Phase 6 scope. |
| `id` argument command-injection into `store.Get` | Tampering | `store.Get(id)` does a map lookup, not a shell invocation. No shell involved. |

No new security controls needed in Phase 6.

## Project Constraints (from CLAUDE.md)

- **Tech stack:** Go, pure-Go deps only — Phase 6 adds zero deps.
- **Token budget:** MCP tool descriptions and model-facing text kept terse. Status help text is currently `"Check status of an id. Exits 0 resolved, 2 pending, 1 unknown."` — verified in main.go L27. No expansion needed.
- **Platform:** Linux + macOS primary; Windows supported via CI cross-compile. Integration test uses pure-Go primitives (`exec.Command`, `sync.WaitGroup`, `bytes.Buffer`) — works on all three. The 10-concurrent timing test tolerances must account for slower Windows fork/exec (flag as open question if it fails on Windows CI).
- **MCP-02 (stdout discipline):** `status` writes exactly one line (`resolved\n` or `pending\n`) on the success path, nothing otherwise. No log lines from status code. `main.go` already routes `log` and `slog` to stderr BEFORE any kong parsing.
- **GSD workflow:** This research was spawned under `/gsd-research-phase` per config; the planner will consume it next.

## Sources

### Primary (HIGH confidence)

- **kong v1.15.0 source (local):** `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/` — VERIFIED
  - `kong.go:85` (`Stdout: os.Stdout` default)
  - `kong.go:462–483` (`FatalIfErrorf` writes err.Error() to stderr, usage to stdout)
  - `options.go:214–221` (`Writers` option)
  - `options.go:390–397` (`UsageOnError` option)
  - `exit.go:14–18, 23–32` (`ExitCoder` interface + `exitCodeFromError`)
  - `help.go:87` (`HelpPrinter` type), `help.go:120, 135` (writes to `ctx.Stdout`)
  - `util.go:30–37` (`VersionFlag.BeforeReset` hard-codes `app.Stdout`)
- **Empirical behavior verification:** built `cmd/mcp-chain` with `go1.25.0` and ran `mcp-chain --help`, `mcp-chain nosuchcommand`, `mcp-chain status` — confirmed stdout leak for help/usage.
- **Empirical fix verification:** built a minimal `kt` binary with `kong.Writers(os.Stderr, os.Stderr)` + manual `--version` pre-parse — confirmed all help/usage goes to stderr, `--version` goes to stdout.
- **Local mcp-chain sources:** `cmd/mcp-chain/main.go`, `internal/cli/stubs.go`, `internal/cli/stubs_test.go`, `internal/store/store.go`, `internal/store/errors.go`, `internal/statepath/resolve.go`, `internal/mcpserver/integration_test.go` — VERIFIED all cited line numbers.
- **CLAUDE.md:** stack + constraints section; read in full.
- **CONTEXT.md:** exit-code contract, testability pattern, non-goals; read in full.
- **REQUIREMENTS.md:** CORE-01 scope split across Phase 1 / 6 / 7; read relevant lines.
- **PITFALLS.md:** `.planning/research/PITFALLS.md` Pitfall 1 ("Corrupting the MCP stdio wire with stdout output") — referenced in main.go patch rationale.

### Secondary (MEDIUM confidence)

None — all claims verified against primary sources.

### Tertiary (LOW confidence)

None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — kong v1.15 is already in go.mod; all behavior verified against vendored source and empirical binary build
- Architecture: HIGH — `runStatus(out, errW, path, id) int` is straightforward and matches CONTEXT.md's locked testability recommendation
- Pitfalls: HIGH — kong stdout-leak empirically reproduced; `ExitCoder` stderr pollution verified from source
- Validation: HIGH — all tests expressible as standard `go test` invocations with existing `buildBinary` helper

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (30 days — stack stable; kong v1.15.0 major-version change would invalidate the `Writers` + `FatalIfErrorf` claims, but no v2 is on the roadmap as of now)
