# Phase 7: CLI Formatters (list, purge, resolve) - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Administrative subcommands `list`, `purge`, and CLI-only `resolve --force` are wired through kong and operate on the shared store (Phase 4). Formatters live in a dedicated sub-package so tabular output logic never leaks into core. After this phase, the `mcp-chain` CLI surface is complete: `serve`, `status`, `list`, `purge`, `resolve` all function end-to-end; slash-command prompt wiring for `/chain-list` and `/chain-purge` is deferred to Phase 8.

**Success Criteria:**
1. `mcp-chain list` prints an aligned human-readable table (ID, status, condition, created_at, resolved_at) via a `LOCK_SH` read. Empty store → single informational line to stderr (not stdout), exit 0. Non-empty → table to stdout.
2. `mcp-chain purge` requires EXACTLY one of `<id>` / `--all` / `--resolved`; bare `mcp-chain purge` exits non-zero with usage text on stderr. The ID counter (`next` / `Counter` field per Phase 4 schema) is NEVER decremented — purge removes records but does not rewind allocation.
3. `mcp-chain resolve <id> [--force]` mirrors the MCP `resolve` tool for scripting; `--force` bypasses the OwnerToken check as the documented operator-driven recovery escape hatch. Exit 0 on success; non-zero with stderr message on unknown-id / not-owner / already-resolved.

**Requirements:** CMD-03 (CLI half), CMD-04 (CLI half)

**Depends on:** Phase 4 (`internal/store` — `List`, `Purge*`, `Resolve` with `ResolveOptions{Force}`), Phase 6 (kong dispatch wiring + stdout/stderr discipline)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices at Claude's discretion. Key constraints:

- **Package layout**: formatters live in a new `internal/cli/format` (or `internal/cli/tableview`) sub-package so the rendering logic is isolated from dispatch. Each command file (`list.go`, `purge.go`, `resolve.go`) calls into the formatter; the formatter does NOT read the store. This keeps Phase 4 (store) free of any presentation concerns and makes formatters unit-testable against fixture records.
- **File layout**:
  - `internal/cli/list.go` — `ListCmd.Run` body (replace stub)
  - `internal/cli/purge.go` — `PurgeCmd.Run` body (replace stub)
  - `internal/cli/resolve.go` — `ResolveCmd.Run` body (replace stub)
  - `internal/cli/format/table.go` — aligned-column rendering for `list`
  - `internal/cli/format/table_test.go` — deterministic fixture tests
  - `internal/cli/list_test.go` / `purge_test.go` / `resolve_test.go` — unit tests for each command's `runXxx(out, errW io.Writer, path string, ...) int` inner function (per Phase 6 testability pattern — LD-2)
  - `internal/cli/integration_test.go` — extend with new integration rows (end-to-end compiled-binary tests, build tag `integration`)
  - `internal/cli/stubs.go` — remove `ListCmd`, `PurgeCmd`, `ResolveCmd` stubs (they move to their own files)
  - `internal/cli/stubs_test.go` — drop all rows (file can be deleted or reduced to `TestVersionFlagWritesToStdout` only if that test lives there)

### Exit code contract (locked)

| Subcommand | Outcome | stdout | stderr | exit |
|------------|---------|--------|--------|------|
| `list` | empty store | — | `mcp-chain: no entries\n` | 0 |
| `list` | N entries | aligned table + trailing newline | — | 0 |
| `list` | other error | — | `mcp-chain: <err>\n` | 1 |
| `purge <id>` | id removed | — (no noise) | — | 0 |
| `purge --all` | N removed | — | — | 0 |
| `purge --resolved` | N removed | — | — | 0 |
| `purge` (no args/flags) | usage error | — | usage text + error | 1 |
| `purge <id>` | unknown id | — | `mcp-chain: unknown id: <id>\n` | 1 |
| `resolve <id>` | success | — | — | 0 |
| `resolve <id> --force` | success | — | — | 0 |
| `resolve <id>` | unknown id | — | `mcp-chain: unknown id: <id>\n` | 1 |
| `resolve <id>` | not owner (CLI is not the MCP session) | — | `mcp-chain: not owner (use --force to override)\n` | 1 |
| `resolve <id>` | already resolved | — | `mcp-chain: already resolved\n` | 1 |

**Rationale:** `list` empty-store → exit 0 (success, nothing to show) with stderr hint is the POSIX idiom (matches `ls` on empty dir). `purge` bare → exit 1 with kong's usage routing matches Phase 6's SC #3 stderr discipline. `resolve` not-owner error message includes the `--force` hint so an operator sees the escape hatch immediately. No distinct exit codes for each error — any error is exit 1 (CMD-03/04 don't demand otherwise, and callers rarely script on error *kind*).

### List formatting contract

- Columns: `ID`, `STATUS`, `CONDITION`, `CREATED`, `RESOLVED`
- Order: by `CreatedAt` ascending (stable), ties broken by ID
- Timestamp format: RFC3339 (`2006-01-02T15:04:05Z07:00`) — machine-readable by accident; more importantly, uniform-width in UTC. Empty `ResolvedAt` → `-` literal
- `CONDITION`: truncate to 48 chars with `...` suffix if longer; never wrap (keeps table aligned and terminal-friendly)
- Column separator: two spaces, no box characters (smallest-footprint ASCII rendering; friendly to narrow terminals and captured logs)
- Alignment: left for all columns (right-pad)
- Final newline: yes (Unix tool convention)

### Purge semantics (locked)

- `--all` and `--resolved` are mutually exclusive; combining them is a usage error
- `<id>` + either flag is a usage error
- Exactly one argument-shape must be provided:
  1. positional `<id>` (remove a single entry)
  2. `--all` (remove all entries)
  3. `--resolved` (remove only records where `Status == "resolved"`)
- The `Counter` field in state schema is NEVER modified by purge (IDs are not reused — the wordlist allocator's monotonic counter is a safety property for cross-session ID consumers)
- Purge must hold `LOCK_EX` for the read-modify-write (uses `store`'s exclusive-lock primitive)

### Resolve semantics (locked)

- `mcp-chain resolve <id>` without `--force` uses OwnerToken == "" (CLI has no session token) → Phase 4 `store.Resolve` returns `ErrNotOwner` unless the record was never owner-stamped (impossible after Phase 5). In practice the CLI path ALWAYS needs `--force` to succeed. This is by design: humans should go through the MCP tool; `mcp-chain resolve` is the recovery hatch.
- `--force` passes `store.ResolveOptions{Force: true}` which short-circuits the OwnerToken check. This is documented as the operator escape hatch.
- `resolve <id>` (no --force) printing `not owner (use --force to override)` is the discoverability affordance — operators who type it without thinking get told how to fix.

### Testability pattern

Mirror Phase 6's `runStatus(out, errW io.Writer, path, id string) int` shape:
- `runList(out, errW io.Writer, path string) int`
- `runPurge(out, errW io.Writer, path string, id string, all, resolvedOnly bool) int`
- `runResolve(out, errW io.Writer, path, id string, force bool) int`

Each `XxxCmd.Run` resolves the state path via `statepath.Resolve()`, invokes the `run` function, and calls `os.Exit(code)` if non-zero. Unit tests call the `run` function directly with `bytes.Buffer` for writers.

### Non-goals

- JSON output format for `list` — not v1 (can be added as `--format=json` later without breaking the table contract)
- Filtering on `list` (e.g., `--status=pending`) — not v1
- Interactive confirmation prompts on `purge --all` — not v1 (scripting-first tool; operators can add their own `read -p` wrappers)
- Bulk resolve (`resolve --all`) — not v1
- Colorized output — not v1 (would require TTY detection; one more thing to maintain)
- `mcp-chain list --watch` — not v1 (bash monitor in Phase 8 handles polling on single id)
- Slash-command prompts (`/chain-list`, `/chain-purge`) — Phase 8

</decisions>

<code_context>
## Existing Code Insights

- `internal/cli/stubs.go` — currently holds `ListCmd` (prints "not implemented", exits 3), `PurgeCmd`, `ResolveCmd`. After Phase 6, the `StatusCmd` has been extracted; follow that pattern for the remaining three.
- `internal/cli/stubs_test.go` — `TestStubsExitCodes` currently verifies list/purge/resolve exit 3. All three rows should be removed when each stub is replaced; the test table becomes empty and the test can be deleted (only `TestVersionFlagWritesToStdout` may remain, or move to a new `main_test.go`).
- `internal/cli/integration_test.go` (Phase 6) — has `buildBinary(t)` helper (via `stubs_test.go`) and `seedStateForChild(t, dir, setup)`. Reuse both. Build tag `integration` already established.
- `internal/store` (Phase 4) — provides `List()`, `PurgeID(id)`, `PurgeAll()`, `PurgeResolved()`, `Resolve(id, token, ResolveOptions{Force})`. Verify exact signatures during research. If any method is missing, plan a thin addition to `internal/store`.
- `internal/store.Record` — fields `ID`, `Status`, `Condition`, `OwnerToken`, `CreatedAt`, `ResolvedAt *time.Time` (confirm exact field names during research).
- `internal/statepath.Resolve()` — Phase 3 utility, used identically to Phase 6.
- `cmd/mcp-chain/main.go` — already has `kong.Writers(os.Stderr, os.Stderr)` from Phase 6; new subcommands inherit correct stderr routing for `--help` and bad flags.

</code_context>

<specifics>
## Specific Ideas

No specific user requirements — discuss phase skipped. Refer to success criteria + the exit-code contract table above.

</specifics>

<deferred>
## Deferred Ideas

- `--format=json` for `list` — deferred; table is the v1 contract
- `list --status=pending` filtering — deferred
- Interactive confirmation on `purge --all` — deferred
- Bulk `resolve --all` — deferred
- Colorized output / TTY detection — deferred
- `list --watch` — deferred to Phase 8's bash monitor (single-id polling)

</deferred>
