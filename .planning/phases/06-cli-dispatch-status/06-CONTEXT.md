# Phase 6: CLI Dispatch & Status Subcommand - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The `mcp-chain status <id>` subcommand is wired through kong, reads the store under `LOCK_SH`, and returns scriptable exit codes (0/1/2) that the bash-monitor helper (Phase 8) depends on. After this phase, argv dispatch is complete for `serve` and `status`; `list`/`purge`/`resolve` remain stubbed for Phase 7.

**Success Criteria:**
1. `mcp-chain status <id>` exits **0** on resolved, **2** on pending, **1** on unknown ID — asserted by a table-driven integration test invoking the compiled binary
2. Status reads use `LOCK_SH`, so 10 concurrent `status` processes against the same state file complete in under one second wall-time with no serialization (timing test)
3. Unknown subcommands, bad arguments, and `--help` write to **stderr** (not stdout); `status` writes nothing to stdout beyond its single-line result

**Requirements:** CORE-01

**Depends on:** Phase 4 (`internal/store` — `Get` uses `LOCK_SH`)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices at Claude's discretion. Key constraints:

- **Package**: changes land in `internal/cli/` — specifically `stubs.go` (rename if sensible) and new `status.go`
- **Kong wiring**: already done in Phase 1. Just replace the `StatusCmd.Run()` stub body.
- **Store access path**: call `statepath.Resolve()` → `store.Open(path)` → `store.Get(id)` → exit per status
- **Stdout line**: single line to stdout for `status`, nothing else. Format: the exact status string (`resolved`, `pending`, or nothing — unknown case exits 1 with stderr-only message)

### Exit code contract (locked — the bash monitor in Phase 8 depends on this)
| Store result | Wire | stdout | stderr | exit |
|--------------|------|--------|--------|------|
| record.status == "resolved" | — | `resolved\n` | — | 0 |
| record.status == "pending" | — | `pending\n` | — | 2 |
| `store.ErrUnknownID` | — | — | `mcp-chain: unknown id: <id>\n` | 1 |
| other error | — | — | `mcp-chain: <err>\n` | 1 (generic) |

- **Rationale**: 0 = success for scripting (the common case after resolve); 2 = "try again later" (pending); 1 = hard error. `bash` scripts can `if mcp-chain status $id; then echo done; fi` and get the intuitive right semantics.
- **No flags** on `status` in v1. Not `--format json`, not `--timeout`, not `--wait`. Phase 8's monitor polls.

### Shared-lock semantics
- `store.Get` already uses `withSharedLock` (Phase 4 delivery). Phase 6 just calls it.
- The timing test spawns 10 concurrent `status <id>` processes against the same state file; all must complete under 1 second wall-time. This indirectly confirms no writer is blocking them.
- Measurement: bash `time`/`wait -n` parallel launch; assert wall-time.

### Stdout/stderr discipline
- `status` is the ONLY subcommand that writes to stdout in v1 (serve writes JSON-RPC; purge/list/resolve → Phase 7).
- `--help` / bad flags / unknown commands: kong writes to stderr by default (`kong.UsageOnError()` config), but we must verify / configure explicitly.
- Test: `cat <(mcp-chain status)` (missing arg) must produce NOTHING on stdout; all error text on stderr.

### File layout
- `internal/cli/status.go` — new file with `StatusCmd` body (moved out of `stubs.go` since it's no longer a stub)
- `internal/cli/stubs.go` — still holds `ListCmd`/`PurgeCmd`/`ResolveCmd` stubs for Phase 7
- `internal/cli/status_test.go` — unit tests for the exit-code mapping (possibly via a function that returns the exit code for easier unit testing — see "Testability" below)
- `internal/cli/integration_test.go` (build tag `integration`) — spawns compiled binary, asserts exit codes + stdout + stderr ALSO the 10-concurrent timing test

### Testability pattern (suggested)
To make exit codes unit-testable without `os.Exit`:
```go
// status.go
func (c *StatusCmd) Run() error {
    code := c.resolveStatus()
    if code != 0 {
        os.Exit(code) // exit non-zero propagates; 0 is implicit success
    }
    return nil
}

func (c *StatusCmd) resolveStatus() int {
    // returns the exit code; called directly from tests
}
```
Alternative: `Run` calls `runStatus(stdout, stderr io.Writer, path, id string) int` and the kong handler does `os.Exit(runStatus(os.Stdout, os.Stderr, ...))`. Second form is more testable; use it.

### Non-goals
- `list`, `purge`, `resolve` CLI — Phase 7
- JSON output format — not v1
- Custom exit code for schema-version error — folds into generic 1 for v1 (operator sees stderr)
- `--wait <timeout>` flag — monitor helper in Phase 8 handles polling
- Tab-completion — not v1

</decisions>

<code_context>
## Existing Code Insights

- `internal/cli/stubs.go` — contains `StatusCmd` stub at L58-L70; prints to stderr, `os.Exit(3)` via error return. The stub structure is: `type StatusCmd struct { ID string \`arg:"" help:"..."\`  }` with a `Run()` method.
- `internal/cli/cli.go` (from Phase 1) — kong wiring; `CLI struct { Status StatusCmd \`cmd\`  ...  }`. Let this stay; just the `Run` body changes.
- `cmd/mcp-chain/main.go` — entry point; `kong.Parse(&CLI{})` + `.Run()` propagates `err` → `os.Exit(1)` path already in place
- Phase 4 `store.Get(id) (Record, error)` returns `Record{ID, Status ("pending"|"resolved"), ResolvedAt *time.Time, ...}` and `ErrUnknownID` for missing
- Phase 3 `statepath.Resolve()` returns the path string
- Phase 5 `ServeCmd.Run` is now the reference for "how to wire a real subcommand" — follow that shape

</code_context>

<specifics>
## Specific Ideas

No specific user requirements — discuss phase skipped. Refer to success criteria + bash-monitor exit-code contract.

</specifics>

<deferred>
## Deferred Ideas

- `--wait` / `--timeout` for watch-mode status — deferred to Phase 8's bash monitor
- JSON output format — deferred to Phase 7 formatters or later
- Distinct exit code for schema-version error (not just generic 1) — deferred; folds into 1 for v1

</deferred>
