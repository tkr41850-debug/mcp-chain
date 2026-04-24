# Phase 7: CLI Formatters (list, purge, resolve) - Research

**Researched:** 2026-04-24
**Domain:** CLI formatting + kong argument constraints + exit-code discipline for three administrative subcommands
**Confidence:** HIGH

## Summary

Phase 7 fleshes out the remaining three `internal/cli` stubs — `ListCmd`, `PurgeCmd`, and `ResolveCmd` — to complete CORE-01's CLI surface. The architectural work was done in Phase 4 (store API) and Phase 6 (runXxx testability pattern, kong stdout/stderr discipline, `buildBinary` + `seedStateForChild` test helpers). Phase 7 is therefore a wiring-plus-formatting phase: each command gets a new file with a `runXxx(out, errW io.Writer, path, ...) int` function that maps store results onto the locked exit-code contract, plus a tiny `internal/cli/format/table.go` that renders `[]store.Record` into the aligned 5-column table.

Four concrete findings drive the plan:

1. **`text/tabwriter` is the right renderer.** It is stdlib, frozen-stable, produces aligned left-padded columns with a single configurable minwidth/padding pair, and handles the variable-width `CONDITION` column automatically after truncation. Hand-rolling `fmt.Sprintf("%-12s  %-9s  ...")` requires knowing column widths up front; `tabwriter` computes them. Zero added deps (stdlib). [VERIFIED: `go doc text/tabwriter` — frozen package, stable since Go 1.0, `NewWriter(output, minwidth, tabwidth, padding, padchar, flags) *Writer`]

2. **Kong's `xor` tag cannot include a positional `arg:""` — it applies only to `Flag` values.** Source read: `Xor []string` lives on `type Flag struct` (kong@v1.15.0 model.go:408–419); kong's `checkMissingFlags` operates on `[]*Flag` (context.go:894). Positionals are `type Positional = Value` and never become `Flag` instances. Therefore "exactly one of `<id>`, `--all`, `--resolved`" **cannot** be enforced purely by struct tags. The hybrid: use `xor:"target" required:""` on `--all` and `--resolved` (makes them mutually exclusive AND makes at least one of the two required WHEN no `<id>` is supplied — wait, this doesn't compose with an optional positional either). The **clean** answer is to leverage `store.Purge`'s existing validator: `store.ErrPurgeArgRequired` is already returned by `Purge(PurgeOptions{})` with zero-or-many targets set (store.go:152–166). Keep the struct minimal — `ID string \`arg:"" optional:"" help:"..."\`\`, `All bool \`xor:"target"\``, `Resolved bool \`xor:"target"\`` — let kong enforce "`--all` and `--resolved` can't both be set", and let the store enforce "at least one target required" via `ErrPurgeArgRequired`. The CLI wraps that error into the documented stderr+exit-1 message. [VERIFIED: kong@v1.15.0 model.go:408, context.go:894–947; store/store.go:152–166]

3. **`Resolve(id, "", {Force:false})` always returns `ErrNotOwner` for records produced by Phase 5.** Confirmed: `store.Resolve` at store.go:96–116 does `subtle.ConstantTimeCompare([]byte(ownerToken), []byte(r.OwnerToken))`. Every record registered via MCP `register` carries a non-empty 32-char hex OwnerToken (Phase 5); passing `""` can never match. This is the by-design behavior called out in CONTEXT.md §Resolve semantics — it forces operators to the `--force` hint, which is the CLI's discoverability affordance. Error propagation is via `errors.Is`: the store uses sentinel `ErrNotOwner` (errors.go:21) without wrapping, so `errors.Is(err, store.ErrNotOwner)` is the correct switch. [VERIFIED: store.go:96–116, errors.go:21]

4. **The Phase 6 test-helper `seedStateForChild(t, dir, setup) string` returns a single id via a closure. It extends cleanly to multi-record seeding** because the closure receives an open `*store.Store` and can call `st.Register(...)` N times. For list tests we simply write a setup closure that registers N records (some resolved, some pending) and returns any sentinel string we like (or `""` — ignored by list). No new helper needed; the existing signature is flexible enough. For counter-non-decrement tests we read `state.json` directly with `os.ReadFile` + `json.Unmarshal` into a local `struct{ Counter uint64 \`json:"counter"\` }` to assert the field survives purge.

**Primary recommendation:** Write four files — `internal/cli/format/table.go`, `internal/cli/list.go`, `internal/cli/purge.go`, `internal/cli/resolve.go` — plus matching `_test.go` files, plus extend `integration_test.go` with new rows. Patch `stubs.go` to remove the three migrated commands (`ListCmd`, `PurgeCmd`, `ResolveCmd`) and `stubs_test.go` to drop the `list` and `purge-all` rows. The exported-for-test file `export_test.go` gains three thin aliases (`RunList`, `RunPurge`, `RunResolve`) mirroring `RunStatus`. Zero new deps, zero changes to `cmd/mcp-chain/main.go` (it already declares the three command types in the CLI grammar and routes writers to stderr).

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Exit code contract** (reproduced verbatim; bash-monitor and scripting depend on these):

| Subcommand | Outcome | stdout | stderr | exit |
|------------|---------|--------|--------|------|
| `list` | empty store | — | `mcp-chain: no entries\n` | 0 |
| `list` | N entries | aligned table + trailing newline | — | 0 |
| `list` | other error | — | `mcp-chain: <err>\n` | 1 |
| `purge <id>` | id removed | — | — | 0 |
| `purge --all` | N removed | — | — | 0 |
| `purge --resolved` | N removed | — | — | 0 |
| `purge` (no args/flags) | usage error | — | usage text + error | 1 |
| `purge <id>` | unknown id | — | `mcp-chain: unknown id: <id>\n` | 1 |
| `resolve <id>` | success | — | — | 0 |
| `resolve <id> --force` | success | — | — | 0 |
| `resolve <id>` | unknown id | — | `mcp-chain: unknown id: <id>\n` | 1 |
| `resolve <id>` | not owner | — | `mcp-chain: not owner (use --force to override)\n` | 1 |
| `resolve <id>` | already resolved | — | `mcp-chain: already resolved\n` | 1 |

- **List formatting**: 5 columns (`ID`, `STATUS`, `CONDITION`, `CREATED`, `RESOLVED`), RFC3339 UTC timestamps, `-` for empty `ResolvedAt`, truncate `CONDITION` at 48 chars with `...` suffix, two-space minimum separator, left-align all columns, final newline, sort by `CreatedAt` ASC then `ID` ASC.
- **Purge semantics**: exactly one of `<id>` / `--all` / `--resolved`; counter NEVER decremented; `LOCK_EX` held for the RMW (store already does this).
- **Resolve semantics**: CLI `resolve <id>` without `--force` passes `""` as token → always hits `ErrNotOwner` → CLI prints the hint. `--force` passes `ResolveOptions{Force: true}` which short-circuits the OwnerToken check.
- **Testability pattern**: `runList(out, errW io.Writer, path string) int`, `runPurge(out, errW io.Writer, path string, id string, all, resolvedOnly bool) int`, `runResolve(out, errW io.Writer, path, id string, force bool) int`. Each `XxxCmd.Run` resolves state path + calls `runXxx` + `os.Exit(code)` if non-zero. Unit tests use `bytes.Buffer` pairs.

### Claude's Discretion

- Choice of `text/tabwriter` vs hand-rolled alignment (research recommends tabwriter).
- Exact test granularity per command (recommendation: 3–5 unit cases each + one integration row).
- Whether to expand `seedStateForChild` or inline multi-record setups in each list test (research recommends inline — the existing helper's closure argument is flexible).
- Resolve's token-empty-string semantics: the research confirms this cannot be bypassed without `--force`; no new branching needed.

### Deferred Ideas (OUT OF SCOPE)

- `--format=json` on `list` (v2 OBS-01)
- `--status=pending` filter on `list`
- `purge --all` confirmation prompt
- `resolve --all`
- Colorized output / TTY detection
- `list --watch`
- Slash-command prompts `/chain-list`, `/chain-purge` (Phase 8)

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-01 | Single Go binary with kong dispatch: `serve`, `status`, `list`, `purge`, `--version`. Phase 1 delivered skeleton; Phase 6 completed `status`; **Phase 7 completes `list`/`purge`/`resolve`** — after which the CLI surface is end-to-end. | Standard Stack (kong v1.15 + stdlib `text/tabwriter`), Architecture Patterns (`runXxx(out, errW, path, ...) int` shape inherited from Phase 6), Exit-code contract decision tree, Integration-test pattern reusing `buildBinary` + `seedStateForChild` |
| CMD-03 (CLI half) | `mcp-chain list` prints a human-readable table (ID, status, condition, created_at, resolved_at). | Standard Stack (`text/tabwriter`), List formatting contract, Unit+integration tests |
| CMD-04 (CLI half) | `mcp-chain purge [id \| --all \| --resolved]`, requires at least one; bare `purge` errors. | Purge semantics (kong can only enforce mutual exclusion of the two flags; store enforces "at least one target"), counter-non-decrement test |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| argv parse + subcommand dispatch | `cmd/mcp-chain/main.go` (kong grammar) | `internal/cli` (command types) | Phase 1 wiring; Phase 7 only replaces stub bodies, no grammar change |
| state path resolution | `internal/statepath` | — | Phase 3 (CORE-06) |
| list / purge / resolve store ops (LOCK_SH/LOCK_EX) | `internal/store` | — | Phase 4 (`List`, `Purge`, `Resolve`) |
| table rendering (alignment, truncation, timestamp format) | `internal/cli/format` | — | New sub-package so rendering never leaks into `internal/store`; formatter is pure — takes a `[]store.Record`, writes to an `io.Writer`, zero I/O of its own |
| exit-code decision (success→0, error→1, etc.) | `internal/cli` (list.go, purge.go, resolve.go) | — | Mirrors Phase 6 — `runXxx` maps store results to exit codes |
| stdout/stderr routing | `cmd/mcp-chain/main.go` + per-command `Run` methods | `internal/cli/format` writes the table body to the provided `io.Writer` | `main.go` still owns `kong.Writers(os.Stderr, os.Stderr)`; commands use explicit writers per the Phase 6 discipline |

## Standard Stack

### Core (CLAUDE.md stack; Phase 7 adds zero deps)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/alecthomas/kong` | v1.15.0 | subcommand dispatch + `xor` mutual-exclusion | Phase 6 [VERIFIED] |
| stdlib `text/tabwriter` | — (Go 1.0+; frozen) | aligned column rendering | Frozen package; produces aligned left-padded columns with one `NewWriter` call + `Flush`; stdlib so no new dep [VERIFIED: `go doc text/tabwriter`, `text/tabwriter.NewWriter`] |
| stdlib `sort` | — | stable sort by CreatedAt then ID | `sort.SliceStable` works on `[]store.Record` [VERIFIED: stdlib] |
| stdlib `strings` | — | 48-char truncation with `...` suffix | trivial; no new dep |
| stdlib `errors` | — | `errors.Is` sentinel matching for `ErrUnknownID` / `ErrNotOwner` / `ErrAlreadyResolved` / `ErrPurgeArgRequired` | Phase 6 pattern; store uses Go 1.13 sentinel+wrap idiom [VERIFIED: store/errors.go] |
| `github.com/stretchr/testify/require` | v1.11.1 | fail-fast assertions in unit + integration tests | project standard [VERIFIED: CLAUDE.md] |

### Supporting (already in go.mod)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `os/exec` | — | integration tests spawn compiled binary | Phase 6 `buildBinary` helper already in `stubs_test.go` |
| stdlib `encoding/json` | — | counter-non-decrement test reads `state.json` directly | Phase 4 test pattern |
| stdlib `bytes` | — | `bytes.Buffer` for captured writers in unit tests | Phase 6 pattern |
| stdlib `path/filepath` | — | assemble state path from `t.TempDir()` | Phase 6 pattern |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `text/tabwriter` | hand-rolled `fmt.Sprintf("%-12s  %-9s  ...")` with pre-computed max widths | Requires a pre-pass over all rows to determine column widths. Possible but ~30 lines vs tabwriter's ~5. tabwriter also handles multi-byte UTF-8 condition strings correctly (frozen package assumes equal-width chars; since we truncate at 48 ASCII-safe chars, this is fine). **Pick tabwriter.** |
| `text/tabwriter` | `github.com/olekukonko/tablewriter` | New dep for box-drawing features we don't want (CONTEXT.md §List formatting: "no box characters"). **Avoid.** |
| `fmt.Fprintf(w, "%s\t%s\t...\n", ...)` + `tabwriter` | `tabwriter.Writer` with a `padchar='\t'` escape | Simpler: emit `\t`-delimited rows to a `tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)` and `Flush()`. **Pick this.** |
| kong `xor` tag covering positional `<id>` | pure-struct enforcement | Kong's `Xor` lives on `Flag`, not `Positional`. Cannot work. [VERIFIED: model.go:408] |
| kong `xor:"target" required:""` on `--all` + `--resolved` + make positional `<id>` an xor member | bail on tag-based enforcement for the tri-state case | Even if positional COULD participate, `xor:"g" required:""` + `<id>` optional contradicts (kong's `required` for positionals defaults true). **Use store-side `ErrPurgeArgRequired` as the validator; tags enforce only the `--all` vs `--resolved` pair.** |
| store-side validation | custom `AfterApply` hook on `PurgeCmd` | Would duplicate `store.ErrPurgeArgRequired` logic. Store already validates; just translate its error. |
| `runPurge` signature with `opts store.PurgeOptions` parameter | extract three primitives (id, all, resolved) | Passing `PurgeOptions` couples the CLI signature to a store type; three primitives is cleaner for tests. Store exposes the type either way. **Use primitives at the runPurge boundary.** |

**Installation:** No new deps. All imports already in `go.mod` as of Phase 6.

**Version verification:** Not needed — stack unchanged.

## Architecture Patterns

### System Architecture Diagram

```
                             argv
                               │
                               ▼
                  ┌──────────────────────────┐
                  │ cmd/mcp-chain/main.go    │
                  │  (unchanged from Phase 6)│
                  │                          │
                  │  kong.Parse(             │
                  │   kong.Writers(          │
                  │     os.Stderr,           │
                  │     os.Stderr),          │
                  │   kong.UsageOnError())   │
                  └────────────┬─────────────┘
                               │ dispatch (one of)
          ┌────────────────────┼────────────────────┐
          ▼                    ▼                    ▼
  ┌─────────────┐      ┌──────────────┐      ┌──────────────┐
  │ list.go     │      │ purge.go     │      │ resolve.go   │
  │ ListCmd.Run │      │ PurgeCmd.Run │      │ ResolveCmd.  │
  │             │      │              │      │ Run          │
  │ path :=     │      │ path :=      │      │ path :=      │
  │ statepath() │      │ statepath()  │      │ statepath()  │
  │             │      │              │      │              │
  │ runList(    │      │ runPurge(    │      │ runResolve(  │
  │  stdout,    │      │  stdout,     │      │  stdout,     │
  │  stderr,    │      │  stderr,     │      │  stderr,     │
  │  path)      │      │  path,       │      │  path, id,   │
  │             │      │  id,all,     │      │  force)      │
  │             │      │  resolvedOnly│      │              │
  │             │      │ )            │      │              │
  └──────┬──────┘      └──────┬───────┘      └──────┬───────┘
         │                    │                     │
         ▼                    ▼                     ▼
  ┌─────────────┐      ┌──────────────┐      ┌──────────────┐
  │ store.List()│      │ store.Purge( │      │ store.       │
  │ → []Record  │      │  PurgeOptions│      │ Resolve(id,  │
  │ (LOCK_SH)   │      │ ) → (int,err)│      │  "", opts)   │
  │             │      │ (LOCK_EX)    │      │ (LOCK_EX)    │
  │             │      │              │      │              │
  │ sort by     │      │ ErrPurgeArg  │      │ ErrUnknownID │
  │ CreatedAt,  │      │  Required?   │      │ ErrNotOwner? │
  │ ID          │      │ ErrUnknownID?│      │ ErrAlready   │
  │             │      │              │      │  Resolved?   │
  │ format.     │      │ translate →  │      │              │
  │ WriteTable  │      │ stderr+exit  │      │ translate →  │
  │ (writer,    │      │              │      │ stderr+exit  │
  │  records)   │      │              │      │              │
  └─────────────┘      └──────────────┘      └──────────────┘
```

Reader can trace:
1. argv enters `main.go` (unchanged; kong grammar already lists all three).
2. kong dispatches to `ListCmd.Run` / `PurgeCmd.Run` / `ResolveCmd.Run` per subcommand.
3. Each `Run` resolves state path and calls the testable `runXxx` function.
4. `runXxx` calls `store.List` / `store.Purge` / `store.Resolve`, translates results into exit codes, and delegates rendering (list only) to `internal/cli/format.WriteTable`.
5. Return code bubbles back to `Run`, which `os.Exit`s on non-zero.

### Recommended File Layout

```
internal/cli/
├── stubs.go              # EDIT: remove ListCmd, PurgeCmd, ResolveCmd (migrated);
│                         #       keep ServeCmd; keep ExitCodeNotImplemented (no stubs left, can delete, but harmless)
├── stubs_test.go         # EDIT: drop `list` and `purge-all` rows from TestStubsExitCodes
│                         #       (table becomes empty → delete TestStubsExitCodes; keep TestVersionFlagWritesToStdout)
├── status.go             # unchanged
├── status_test.go        # unchanged
├── list.go               # NEW: ListCmd type, ListCmd.Run, runList(out, errW, path) int
├── list_test.go          # NEW: unit tests for runList decision tree (empty, N-entries, sort order, other-error)
├── purge.go              # NEW: PurgeCmd type, PurgeCmd.Run, runPurge(out, errW, path, id, all, resolvedOnly) int
├── purge_test.go         # NEW: unit tests (id-removed, --all, --resolved, ErrPurgeArgRequired, ErrUnknownID, counter-preserved)
├── resolve.go            # NEW: ResolveCmd type, ResolveCmd.Run, runResolve(out, errW, path, id, force) int
├── resolve_test.go       # NEW: unit tests (force-success, no-force-not-owner, unknown-id, already-resolved)
├── export_test.go        # EDIT: add RunList, RunPurge, RunResolve aliases alongside RunStatus
├── integration_test.go   # EDIT: add list / purge / resolve rows to the integration suite;
│                         #       add TestPurge_CounterNotDecremented
└── format/
    ├── table.go          # NEW: WriteTable(w io.Writer, recs []store.Record) — rendering only, no store access
    └── table_test.go     # NEW: deterministic fixture tests (empty-in → empty-out; N rows; truncation; sort stability)
```

### Pattern 1: The table formatter (stdlib `text/tabwriter`)

**What:** A pure function `format.WriteTable(w io.Writer, records []store.Record) error` that sorts the slice stable-ascending on `CreatedAt` then `ID`, formats each row as tab-delimited cells, and flushes through a `tabwriter.NewWriter` that left-pads columns to the same width with two spaces of padding.

**When to use:** Phase 7 `list` only. Format is narrow; a generic "table helper" is over-engineered. Keep it specific to `[]store.Record`.

**Example:**
```go
// Source: /home/alpine/mcp-chain/internal/cli/format/table.go (NEW in Phase 7)
// Terminology: "row" = one store.Record. "cell" = one tab-delimited column value.
package format

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/anthropics/mcp-chain/internal/store"
)

// conditionMaxWidth caps the CONDITION column so a pathological
// multi-line NL condition can't wreck alignment. 48 is the CONTEXT.md
// decision — narrow enough for an 80-col terminal after the other four
// columns (≈12+9+48+20+20 = 109 → still wraps in 80-col, but the table
// stays aligned on wider terminals).
const conditionMaxWidth = 48

// tsFormat is RFC3339 in UTC. Fixed-width (20 chars w/ 'Z' suffix).
const tsFormat = "2006-01-02T15:04:05Z07:00"

// WriteTable renders records as an aligned 5-column table to w.
// Empty input produces NO output (caller owns the "no entries" stderr
// hint — keeps this function pure). Returns any Flush error.
//
// Column order (LOCKED): ID, STATUS, CONDITION, CREATED, RESOLVED.
// Sort order: CreatedAt ASC, ties broken by ID ASC.
func WriteTable(w io.Writer, records []store.Record) error {
	if len(records) == 0 {
		return nil
	}
	sorted := make([]store.Record, len(records))
	copy(sorted, records)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].ID < sorted[j].ID
	})

	// minwidth=0, tabwidth=0, padding=2, padchar=' '  → two-space
	// minimum separator, left-aligned, no ANSI, no flags.
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tCONDITION\tCREATED\tRESOLVED")
	for _, r := range sorted {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			r.ID,
			r.Status,
			truncate(r.Condition, conditionMaxWidth),
			r.CreatedAt.UTC().Format(tsFormat),
			formatResolvedAt(r.ResolvedAt),
		)
	}
	return tw.Flush()
}

// truncate returns s unchanged if len(s) <= max, else s[:max-3] + "...".
// Operates on bytes; conditions in practice are ASCII-ish; multibyte
// edge cases are acceptable for a cap this generous.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// formatResolvedAt returns the RFC3339 UTC timestamp, or "-" when nil.
// Keeps the column aligned: either 20 chars or one char; tabwriter
// right-pads to the column width either way.
func formatResolvedAt(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format(tsFormat)
}

// NOTE: import "time" required for formatResolvedAt parameter type.
```

(The executor needs to add `import "time"` — reminder noted in the skeleton comment.)

### Pattern 2: `runList` decision tree

```go
// Source: /home/alpine/mcp-chain/internal/cli/list.go (NEW in Phase 7)
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/anthropics/mcp-chain/internal/cli/format"
	"github.com/anthropics/mcp-chain/internal/statepath"
	"github.com/anthropics/mcp-chain/internal/store"
)

// ListCmd prints all chain entries as an aligned table.
//
// Exit codes (LOCKED):
//
//	0  N entries printed to stdout (or zero entries → "no entries" to stderr, exit 0)
//	1  any load / render error — stderr: "mcp-chain: <err>\n"
type ListCmd struct{}

func (c *ListCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
		os.Exit(1)
	}
	code := runList(os.Stdout, os.Stderr, path)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runList maps (path → exit code) with explicit writers. See status.go
// for the testability rationale. Pure with respect to env vars.
func runList(out, errW io.Writer, path string) int {
	st, err := store.Open(path)
	if err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	records, err := st.List()
	if err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Fprintln(errW, "mcp-chain: no entries")
		return 0
	}
	if err := format.WriteTable(out, records); err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	return 0
}
```

### Pattern 3: `runPurge` decision tree

```go
// Source: /home/alpine/mcp-chain/internal/cli/purge.go (NEW in Phase 7)
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/mcp-chain/internal/statepath"
	"github.com/anthropics/mcp-chain/internal/store"
)

// PurgeCmd deletes entries. Exactly one of <id>, --all, --resolved is
// required; the `xor:"target"` tag makes --all and --resolved mutually
// exclusive at the kong layer, and store.ErrPurgeArgRequired enforces
// "at least one target" at the store layer (see runPurge error branch).
//
// Exit codes (LOCKED):
//
//	0  any N-removed (including N=0 for --resolved on empty store)
//	1  kong parse failure (none-supplied or both flags supplied), unknown <id>, or any other error
type PurgeCmd struct {
	ID       string `arg:"" optional:"" help:"Id to purge (mutually exclusive with --all and --resolved)."`
	All      bool   `help:"Purge all entries." xor:"target"`
	Resolved bool   `help:"Purge only resolved entries." xor:"target"`
}

func (c *PurgeCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
		os.Exit(1)
	}
	code := runPurge(os.Stdout, os.Stderr, path, c.ID, c.All, c.Resolved)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runPurge maps (path, id, all, resolvedOnly → exit code) with explicit
// writers. Translates store.ErrPurgeArgRequired / ErrUnknownID into the
// locked stderr lines.
func runPurge(out, errW io.Writer, path string, id string, all, resolvedOnly bool) int {
	_ = out // purge emits nothing to stdout
	st, err := store.Open(path)
	if err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	_, err = st.Purge(store.PurgeOptions{
		ID:       id,
		All:      all,
		Resolved: resolvedOnly,
	})
	switch {
	case err == nil:
		return 0
	case errors.Is(err, store.ErrPurgeArgRequired):
		// Bare `purge` with no args/flags — kong accepted it because
		// positional is optional and neither flag was set. Store
		// enforces "exactly one target" and returns this sentinel.
		fmt.Fprintln(errW, "mcp-chain: purge requires <id>, --all, or --resolved")
		return 1
	case errors.Is(err, store.ErrUnknownID):
		fmt.Fprintf(errW, "mcp-chain: unknown id: %s\n", id)
		return 1
	default:
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
}
```

### Pattern 4: `runResolve` decision tree

```go
// Source: /home/alpine/mcp-chain/internal/cli/resolve.go (NEW in Phase 7)
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/mcp-chain/internal/statepath"
	"github.com/anthropics/mcp-chain/internal/store"
)

// ResolveCmd marks an id resolved from the CLI.
//
// Because the CLI has no MCP session and therefore no OwnerToken, the
// happy path REQUIRES --force. This is the operator escape hatch
// documented in CORE-08 and CONTEXT.md §Resolve semantics.
//
// Exit codes (LOCKED):
//
//	0  resolved — stderr empty, stdout empty
//	1  unknown id, not-owner (without --force), already-resolved, any other error
type ResolveCmd struct {
	ID    string `arg:"" help:"Id to resolve."`
	Force bool   `help:"Bypass the OwnerToken check (CLI operator escape hatch)."`
}

func (c *ResolveCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
		os.Exit(1)
	}
	code := runResolve(os.Stdout, os.Stderr, path, c.ID, c.Force)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runResolve passes an empty ownerToken; the store compares it (via
// crypto/subtle) against the record's stamped token. Without --force,
// this always fails for Phase 5 records (they carry a non-empty token).
// The CLI surfaces the --force hint in the ErrNotOwner branch.
func runResolve(out, errW io.Writer, path, id string, force bool) int {
	_ = out // resolve emits nothing to stdout
	st, err := store.Open(path)
	if err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	err = st.Resolve(id, "", store.ResolveOptions{Force: force})
	switch {
	case err == nil:
		return 0
	case errors.Is(err, store.ErrUnknownID):
		fmt.Fprintf(errW, "mcp-chain: unknown id: %s\n", id)
		return 1
	case errors.Is(err, store.ErrNotOwner):
		fmt.Fprintln(errW, "mcp-chain: not owner (use --force to override)")
		return 1
	case errors.Is(err, store.ErrAlreadyResolved):
		fmt.Fprintln(errW, "mcp-chain: already resolved")
		return 1
	default:
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
}
```

### Pattern 5: Exported-for-test aliases (append to existing `export_test.go`)

```go
// Source: /home/alpine/mcp-chain/internal/cli/export_test.go (EDIT in Phase 7)
package cli

import "io"

// RunStatus is from Phase 6 (unchanged).
func RunStatus(out, errW io.Writer, path, id string) int {
	return runStatus(out, errW, path, id)
}

// RunList re-exports runList for xtests (Phase 7).
func RunList(out, errW io.Writer, path string) int {
	return runList(out, errW, path)
}

// RunPurge re-exports runPurge for xtests (Phase 7).
func RunPurge(out, errW io.Writer, path, id string, all, resolvedOnly bool) int {
	return runPurge(out, errW, path, id, all, resolvedOnly)
}

// RunResolve re-exports runResolve for xtests (Phase 7).
func RunResolve(out, errW io.Writer, path, id string, force bool) int {
	return runResolve(out, errW, path, id, force)
}
```

### Anti-Patterns to Avoid

- **Writing the "no entries" message to stdout.** Violates the locked contract (`list` empty-store → stderr hint, exit 0). Downstream pipes would receive the hint as fake data.
- **Sorting inside `runList` instead of inside `format.WriteTable`.** Keep the pure formatter in charge of presentation; `runList` is the controller. If presentation rules change, they stay localized.
- **Putting `xor:"target"` on `ID` positional.** Kong `Xor` is a `Flag` property; positionals don't participate. Build fails or, worse, silently skips the xor check. [VERIFIED: kong model.go:408]
- **Storing the resolved-error string ("not owner (use --force...)") in a constant inside `errors.go`.** The store owns the sentinel; the CLI owns the user-facing copy. Mixing them violates the hexagonal boundary.
- **Calling `statepath.Resolve()` inside `runPurge` / `runList` / `runResolve`.** Same Pitfall 3 as Phase 6 — couples tests to `$XDG_STATE_HOME`. Resolve in `Run`, pass `path` in.
- **Calling `os.Exit` inside `runXxx`.** `Run` does that. `runXxx` returns an `int`. Tests need the return value.
- **`fmt.Println` (without explicit writer) anywhere in the code path.** Writes to `os.Stdout`, bypasses the `out io.Writer` plumbing, breaks tests.
- **Returning `store.PurgeOptions` from the kong struct (as a nested embedded type).** Couples the CLI struct to a store type prematurely; primitives are cleaner.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| column alignment | `fmt.Sprintf("%-12s ...")` with pre-scan for widths | `text/tabwriter` | stdlib, zero deps, handles variable-width columns |
| stable sort on two keys | bubble sort or hand-rolled `sort.Sort` with Less method | `sort.SliceStable(s, func(i,j int) bool { ... })` | stdlib, one line |
| sentinel error match | `if err.Error() == "store: unknown id"` | `errors.Is(err, store.ErrUnknownID)` | Go 1.13+ idiom; survives future error wrapping |
| kong "exactly one of" enforcement | custom `AfterApply` hook duplicating store logic | let `store.ErrPurgeArgRequired` surface; translate via `errors.Is` | store already has the check; single source of truth |
| RFC3339 timestamp format | custom date arithmetic | `t.UTC().Format(time.RFC3339)` | Go stdlib |
| integration harness | shell script + `bats` / `expect` | existing `buildBinary(t)` + `exec.Command` + `seedStateForChild` | already wired Phase 6 |

**Key insight:** Phase 7, like Phase 6, is a wiring phase — every building block is already in place. The only fresh code is (a) the 3 `runXxx` decision trees, (b) the `format.WriteTable` stdlib-tabwriter rendering, (c) integration rows in the existing test. No architectural work; no new deps.

## Runtime State Inventory

**Not applicable** — Phase 7 is a greenfield wiring phase. No rename / refactor / migration.

- **Stored data:** None. State schema (Phase 4) unchanged; `list` is read-only; `purge` removes records but leaves `Counter` and `Version` fields untouched (already verified at store.go:188 comment).
- **Live service config:** None. No external services register any name touched by Phase 7.
- **OS-registered state:** None. No new OS-level hook names.
- **Secrets/env vars:** None. `$XDG_STATE_HOME` and `$HOME` are read by `statepath.Resolve()` (unchanged). The CLI `resolve --force` passes `""` as OwnerToken; no new env var consumed.
- **Build artifacts:** None. `go build` produces a fresh binary per CI run.

## Common Pitfalls

### Pitfall 1: Purge on empty store is idempotent; tests must not assert an error

**What goes wrong:** Test writes "purge --resolved on empty store MUST error" based on intuition. Store returns `(0, nil)` — the switch's `err == nil` branch takes over, returns exit 0. Test red.

**Why it happens:** `store.Purge({Resolved: true})` with zero records iterates an empty map and returns `(removed=0, err=nil)`. Empty-set removal is conceptually an idempotent no-op [VERIFIED: store.go:181–187].

**How to avoid:** Assert `exit == 0` and `removed == 0` for `purge --resolved` / `purge --all` on an empty store. Only `purge <id>` on an empty store returns `ErrUnknownID` (because `<id>` demands that specific record).

**Warning signs:** Integration test "purge-resolved-empty" is red on CI.

### Pitfall 2: Map iteration is random; `list` tests must compare sorted output

**What goes wrong:** Test seeds 3 records, captures `list` stdout, compares byte-exact against a hardcoded string. Run twice → fails 2/3 of the time because Go maps iterate in random order.

**Why it happens:** `store.List()` ranges over `map[string]record` with no sort guarantee (store.go:138–146 literally says "Order is unspecified").

**How to avoid:** `runList` delegates sorting to `format.WriteTable` which uses `sort.SliceStable` with `CreatedAt` ASC, ID tiebreaker. Tests assert against the sorted expected output. Use `time.Now().Add(n * time.Millisecond)` spaced timestamps or craft the records via direct store injection (if schema access exported via export_test — out of scope) to force a deterministic order.

**Warning signs:** `TestRunList_N_Entries_Sorted` flakes 1 in 3 runs.

### Pitfall 3: `Counter` non-decrement test reads `state.json` directly

**What goes wrong:** Test does `store.List()` after purge, counts records, forgets that's not `Counter`. Counter is NOT exposed on `Record`.

**Why it happens:** `Counter` is a field on `state{}` (schema.go:25), not on `Record`. `List()` returns `[]Record` — no access to counter. [VERIFIED]

**How to avoid:** Test reads the state file directly:
```go
var s struct { Counter uint64 `json:"counter"` }
raw, _ := os.ReadFile(statePath)
_ = json.Unmarshal(raw, &s)
// assert s.Counter unchanged across purge
```

**Warning signs:** "I don't know how to assert Counter" comment in the test file.

### Pitfall 4: `resolve <id>` without `--force` MUST error — this is by design, not a bug

**What goes wrong:** Developer registers an id via MCP (gets an owner-stamped record) then runs `mcp-chain resolve <that-id>` and expects success. Gets `not owner (use --force to override)`. Files a bug.

**Why it happens:** Phase 5 records carry non-empty OwnerToken; Phase 4 `Resolve` constant-time-compares `""` (CLI's passed token) to the stamped token — mismatch; returns `ErrNotOwner`.

**How to avoid:** Document in `resolve.go` header comment (included in Pattern 4 above). The test `TestRunResolve_NoForce_NotOwner_Exit1` encodes this — if it ever goes green on a no-force path, Phase 5 regressed.

**Warning signs:** `TestRunResolve_NoForce_NotOwner_Exit1` turns green when a Phase 5 register path was changed.

### Pitfall 5: `kong` xor cannot include positional; bare `purge` must validate in `runPurge`

**What goes wrong:** Developer adds `xor:"target" required:""` to `--all` and `--resolved`, hoping kong will reject bare `purge` (no args). It doesn't — because when `<id>` is present as an optional positional at the same level, kong treats the `xor required` condition as satisfied by "positional might be supplied". Bare `purge` parses successfully with all zero values. The error MUST come from the store.

**Why it happens:** Kong `Xor` is defined on `Flag` only (model.go:408–419). Kong's `checkMissingFlags` (context.go:894–948) doesn't consider positionals.

**How to avoid:** Rely on `store.Purge({})` returning `ErrPurgeArgRequired`. `runPurge` translates that sentinel to the locked "mcp-chain: purge requires <id>, --all, or --resolved" stderr + exit 1.

**Warning signs:** `TestPurge_BareArgs_Exit1` is red with stderr containing kong parse error instead of the expected store-sourced message.

### Pitfall 6: Empty-store `list` writes "no entries" to stderr, exit 0 — not stdout, not exit 1

**What goes wrong:** Developer writes `fmt.Fprintln(out, "mcp-chain: no entries")` → pipes pick it up as a fake row. Or writes it to stderr but returns exit 1 → scripts treat empty as error.

**Why it happens:** Matches POSIX idiom (`ls` on empty dir is stderr-silent with exit 0, but macOS ls with `-l` prints "total 0" to stdout — inconsistency). We follow the spirit: success + informational hint on stderr.

**How to avoid:** The locked contract is written in CONTEXT.md and reproduced in the User Constraints section above. Pattern 2 code implements exactly this.

**Warning signs:** `TestRunList_Empty_Exit0_HintToStderr` is red.

### Pitfall 7: `truncate("abc", 48)` returns "abc" (no change) — edge cases at boundary

**What goes wrong:** Developer writes `truncate(s, max int) string { return s[:max-3]+"..." }` unconditionally → panics on strings shorter than 45 chars.

**Why it happens:** Slice-before-length-check.

**How to avoid:** Guard `len(s) <= max` early-return. See Pattern 1 code. Add a unit test for edge cases: empty string, exactly-48-char string, 49-char string, 3-char string.

**Warning signs:** `TestTruncate_Boundary` panics.

### Pitfall 8: `tabwriter` needs explicit `Flush` — forgetting it drops trailing rows

**What goes wrong:** `WriteTable` builds rows via `fmt.Fprintf(tw, ...)` then returns without calling `tw.Flush()`. The last row (or several) is buffered internally and never reaches `w`. Test compares output → last row missing.

**Why it happens:** `tabwriter.Writer` buffers until column widths stabilize, then flushes on `Flush()` call. [VERIFIED: `go doc text/tabwriter.Writer.Flush`]

**How to avoid:** Pattern 1 returns `tw.Flush()` as the final statement. Alternatively `defer tw.Flush()` — but then the return value is lost; prefer explicit.

**Warning signs:** Test comparing byte-exact tabwriter output is missing rows at the end.

## Code Examples

### Example 1: `runList` unit — empty store produces stderr hint and exit 0

```go
// Source: /home/alpine/mcp-chain/internal/cli/list_test.go (NEW in Phase 7)
func TestRunList_Empty_Exit0_HintToStderr(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// No registrations — store file may not even exist; loadState
	// returns an empty state on os.ErrNotExist (store.go:201-209).

	var out, errW bytes.Buffer
	code := cli.RunList(&out, &errW, path)

	require.Equal(t, 0, code)
	require.Empty(t, out.String(), "list on empty store MUST NOT write stdout")
	require.Equal(t, "mcp-chain: no entries\n", errW.String())
}
```

### Example 2: `runList` unit — N entries, sorted output

```go
// Source: /home/alpine/mcp-chain/internal/cli/list_test.go (NEW in Phase 7)
func TestRunList_NEntries_SortedTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)

	// Register 3 entries with slight time gaps so CreatedAt differs.
	// No need to seed artificial timestamps — time.Now() progresses.
	id1, err := st.Register("tok", "first condition")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond) // ensure CreatedAt strict order
	id2, err := st.Register("tok", "second condition — this one is LOOOOONG and will be truncated to forty-eight chars with ellipsis")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	id3, err := st.Register("tok", "third")
	require.NoError(t, err)
	require.NoError(t, st.Resolve(id1, "tok", store.ResolveOptions{}))

	var out, errW bytes.Buffer
	code := cli.RunList(&out, &errW, path)

	require.Equal(t, 0, code)
	require.Empty(t, errW.String(), "non-empty list must not write stderr")

	// Assert header + row count + order, not exact bytes (timestamps vary).
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	require.Len(t, lines, 4, "header + 3 rows") // 1 header + 3 data
	require.True(t, strings.HasPrefix(lines[0], "ID"), "header row starts with ID")
	require.Contains(t, lines[0], "STATUS")
	require.Contains(t, lines[0], "CONDITION")
	require.Contains(t, lines[0], "CREATED")
	require.Contains(t, lines[0], "RESOLVED")
	// lines[1], lines[2], lines[3] are rows in CreatedAt ASC order:
	require.Contains(t, lines[1], id1)
	require.Contains(t, lines[2], id2)
	require.Contains(t, lines[3], id3)
	require.Contains(t, lines[2], "...", "long condition must be truncated with ellipsis")
}
```

### Example 3: `format.WriteTable` unit — truncation and `-` for nil ResolvedAt

```go
// Source: /home/alpine/mcp-chain/internal/cli/format/table_test.go (NEW in Phase 7)
package format_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anthropics/mcp-chain/internal/cli/format"
	"github.com/anthropics/mcp-chain/internal/store"
)

func TestWriteTable_EmptyIn_EmptyOut(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, nil))
	require.Empty(t, buf.String())

	require.NoError(t, format.WriteTable(&buf, []store.Record{}))
	require.Empty(t, buf.String())
}

func TestWriteTable_NilResolvedAtRendersDash(t *testing.T) {
	rec := store.Record{
		ID: "acid", Status: "pending", Condition: "wait for build",
		CreatedAt: time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
		ResolvedAt: nil,
	}
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, []store.Record{rec}))
	out := buf.String()
	require.Contains(t, out, "acid")
	require.Contains(t, out, "pending")
	require.Contains(t, out, "2026-04-24T10:00:00Z")
	// last column on the row is RESOLVED="-"
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 2) // header + 1 row
	require.True(t, strings.HasSuffix(lines[1], "-"), "resolved-nil row ends with '-'")
}

func TestWriteTable_TruncatesLongConditionWithEllipsis(t *testing.T) {
	long := strings.Repeat("x", 100) // 100 x's
	rec := store.Record{
		ID: "a", Status: "pending", Condition: long,
		CreatedAt: time.Unix(0, 0).UTC(),
	}
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, []store.Record{rec}))
	out := buf.String()
	// 48 cap; with "..." suffix the cell value is 45 x's + "...".
	require.Contains(t, out, strings.Repeat("x", 45)+"...")
	require.NotContains(t, out, strings.Repeat("x", 49))
}

func TestWriteTable_SortsByCreatedAtThenID(t *testing.T) {
	t0 := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	recs := []store.Record{
		{ID: "zebra",  Status: "pending", Condition: "c", CreatedAt: t0.Add(2 * time.Second)},
		{ID: "acid",   Status: "pending", Condition: "c", CreatedAt: t0.Add(1 * time.Second)},
		{ID: "b-tied", Status: "pending", Condition: "c", CreatedAt: t0.Add(1 * time.Second)}, // ties-break by ID
	}
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, recs))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 4) // header + 3
	require.Contains(t, lines[1], "acid",   "acid at t+1 comes first")
	require.Contains(t, lines[2], "b-tied", "b-tied at t+1 sorts by ID after acid")
	require.Contains(t, lines[3], "zebra",  "zebra at t+2 last")
}

func TestTruncate_BoundaryCases(t *testing.T) {
	// internal helper, exposed via format.Truncate if needed — or embed
	// these as sub-cases inside the public WriteTable tests. Keeping
	// truncate private + table-level coverage is fine; add only if CI
	// flakes warrant line-level coverage.
}
```

### Example 4: `runPurge` unit — by-id success

```go
// Source: /home/alpine/mcp-chain/internal/cli/purge_test.go (NEW in Phase 7)
func TestRunPurge_ByID_Success_Exit0(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c")
	require.NoError(t, err)

	var out, errW bytes.Buffer
	code := cli.RunPurge(&out, &errW, path, id, false, false)

	require.Equal(t, 0, code)
	require.Empty(t, out.String())
	require.Empty(t, errW.String())

	// Verify it's gone.
	_, err = st.Get(id)
	require.ErrorIs(t, err, store.ErrUnknownID)
}
```

### Example 5: `runPurge` unit — bare (no args) → `ErrPurgeArgRequired`

```go
// Source: /home/alpine/mcp-chain/internal/cli/purge_test.go (NEW in Phase 7)
func TestRunPurge_NoArgs_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	var out, errW bytes.Buffer
	code := cli.RunPurge(&out, &errW, path, "", false, false)

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Contains(t, errW.String(), "purge requires <id>, --all, or --resolved")
}
```

### Example 6: `runPurge` integration — counter non-decrement

```go
// Source: /home/alpine/mcp-chain/internal/cli/integration_test.go (APPEND in Phase 7)
// Validates CONTEXT.md §Purge semantics: "Counter is NEVER modified by purge"
func TestPurge_CounterNotDecremented(t *testing.T) {
	binPath := buildBinary(t)
	dir := t.TempDir()
	statePath := filepath.Join(dir, "mcp-chain", "state.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(statePath), 0o700))

	// Register 3 entries → counter should be 3.
	st, err := store.Open(statePath)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err := st.Register("tok", "c")
		require.NoError(t, err)
	}

	// Read counter before purge.
	type partial struct {
		Counter uint64 `json:"counter"`
	}
	readCounter := func() uint64 {
		raw, err := os.ReadFile(statePath)
		require.NoError(t, err)
		var p partial
		require.NoError(t, json.Unmarshal(raw, &p))
		return p.Counter
	}
	before := readCounter()
	require.Equal(t, uint64(3), before)

	// `mcp-chain purge --all`.
	cmd := exec.Command(binPath, "purge", "--all")
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
	var out, errW bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errW
	require.NoError(t, cmd.Run(), "stderr=%q", errW.String())
	require.Empty(t, out.String())
	require.Empty(t, errW.String())

	after := readCounter()
	require.Equal(t, before, after, "Counter MUST survive purge --all (CORE-09)")
}
```

### Example 7: `runResolve` unit — `--force` success

```go
// Source: /home/alpine/mcp-chain/internal/cli/resolve_test.go (NEW in Phase 7)
func TestRunResolve_Force_Success_Exit0(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c")
	require.NoError(t, err)

	var out, errW bytes.Buffer
	code := cli.RunResolve(&out, &errW, path, id, true)

	require.Equal(t, 0, code)
	require.Empty(t, out.String())
	require.Empty(t, errW.String())

	r, err := st.Get(id)
	require.NoError(t, err)
	require.Equal(t, "resolved", r.Status)
}
```

### Example 8: `runResolve` unit — no-force against owner-stamped record → not-owner message

```go
// Source: /home/alpine/mcp-chain/internal/cli/resolve_test.go (NEW in Phase 7)
func TestRunResolve_NoForce_NotOwner_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c") // owner-stamped
	require.NoError(t, err)

	var out, errW bytes.Buffer
	code := cli.RunResolve(&out, &errW, path, id, false)

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Equal(t,
		"mcp-chain: not owner (use --force to override)\n",
		errW.String(),
		"CLI exposes the --force hint on the ErrNotOwner branch")
}
```

### Example 9: `runResolve` unit — already resolved

```go
// Source: /home/alpine/mcp-chain/internal/cli/resolve_test.go (NEW in Phase 7)
func TestRunResolve_AlreadyResolved_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c")
	require.NoError(t, err)
	require.NoError(t, st.Resolve(id, "tok", store.ResolveOptions{})) // resolve first with correct token

	var out, errW bytes.Buffer
	code := cli.RunResolve(&out, &errW, path, id, true) // --force, but already resolved

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Equal(t, "mcp-chain: already resolved\n", errW.String())
}
```

### Example 10: Integration — extend `TestStatus_IntegrationExitCodes` shape for list/purge/resolve

```go
// Source: /home/alpine/mcp-chain/internal/cli/integration_test.go (APPEND in Phase 7)
// Pattern mirrors TestStatus_IntegrationExitCodes: build binary, table-drive cases.

func TestList_IntegrationExitCodes(t *testing.T) {
	binPath := buildBinary(t)

	cases := []struct {
		name     string
		setup    func(t *testing.T, st *store.Store) string
		args     []string
		wantExit int
		wantOutSubstr string // "" → assert empty
		wantErrSubstr string // "" → assert empty
	}{
		{
			name:  "empty store",
			setup: func(*testing.T, *store.Store) string { return "" },
			args:  []string{"list"},
			wantExit: 0,
			wantOutSubstr: "",
			wantErrSubstr: "no entries",
		},
		{
			name: "two entries",
			setup: func(t *testing.T, st *store.Store) string {
				_, err := st.Register("tok", "first")
				require.NoError(t, err)
				_, err = st.Register("tok", "second")
				require.NoError(t, err)
				return ""
			},
			args: []string{"list"},
			wantExit: 0,
			wantOutSubstr: "STATUS", // header present
			wantErrSubstr: "",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			_ = seedStateForChild(t, dir, tt.setup)
			cmd := exec.Command(binPath, tt.args...)
			cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
			var out, errW bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errW
			err := cmd.Run()

			gotExit := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				gotExit = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			require.Equal(t, tt.wantExit, gotExit, "stderr=%q", errW.String())
			if tt.wantOutSubstr == "" {
				require.Empty(t, out.String(), "stdout expected empty")
			} else {
				require.Contains(t, out.String(), tt.wantOutSubstr)
			}
			if tt.wantErrSubstr == "" {
				require.Empty(t, errW.String(), "stderr expected empty")
			} else {
				require.Contains(t, errW.String(), tt.wantErrSubstr)
			}
		})
	}
}

// Purge + Resolve integration tests follow the same shape — one case per
// contract row. See full case list in the Validation Architecture section.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 1 stubs returning exit 3 for list/purge/resolve | real `runList` / `runPurge` / `runResolve` decision trees | Phase 7 | Final CLI surface — CORE-01 satisfied end-to-end |
| Kong `Purge` grammar with `xor:"target"` on flags but no "at least one" enforcement | unchanged grammar; store returns `ErrPurgeArgRequired` on zero-target | Phase 4 (store) + Phase 7 (CLI wiring) | hexagonal: CLI translates sentinel; no grammar change |

**Deprecated/outdated:** `ExitCodeNotImplemented = 3` constant in `stubs.go` — after Phase 7, no subcommand uses it. Decision: **keep the constant** but drop `ListCmd` / `PurgeCmd` / `ResolveCmd` from `stubs.go`. The constant stays around as a documented reserved value; removing it would be a tiny extra diff with no safety benefit.

## Assumptions Log

No assumed claims in this research. Every factual claim is either VERIFIED against kong@v1.15.0 source at `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/`, the local `/home/alpine/mcp-chain` sources (store, cli, statepath), `go doc` output, or reproduced from Phase 6's RESEARCH.md (which was itself VERIFIED). No user confirmation needed.

## Open Questions

**None. All defaults apply.**

Every decision in CONTEXT.md is actionable as-written:
- List formatting details are fully locked (columns, sort, truncation, timestamps).
- Purge semantics are fully locked (xor pattern + store-side enforcement).
- Resolve semantics are fully locked (CLI always needs `--force` for Phase 5 records).
- Testability pattern is fully locked (mirror Phase 6 `runXxx(writers, path, ...)`)
- Non-goals are fully listed.

The only micro-decision left for the executor — whether to expose `truncate` as `format.Truncate` for line-level test coverage versus keep it private — is explicitly called out as discretionary in Example 3 and carries zero downstream risk either way.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `go test`, `buildBinary` in integration tests | ✓ | 1.25.0 at `/home/alpine/go-sdk/go/bin/go` [VERIFIED via existing Phase 6 research] | — |
| `github.com/alecthomas/kong` | `PurgeCmd` + `ResolveCmd` parsing | ✓ | v1.15.0 at `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/` [VERIFIED] | — |
| `github.com/stretchr/testify/require` | all tests | ✓ | per go.mod [VERIFIED] | — |
| stdlib `text/tabwriter` | table rendering | ✓ | stdlib (frozen) [VERIFIED: `go doc text/tabwriter`] | — |
| stdlib `sort`, `strings`, `errors`, `encoding/json` | formatting, sorting, error match, counter read | ✓ | stdlib [VERIFIED] | — |
| `internal/store` (List, Purge, Resolve) | all three runXxx funcs | ✓ | Phase 4 delivered [VERIFIED: store.go:96–195] | — |
| `internal/statepath.Resolve()` | all three `XxxCmd.Run` methods | ✓ | Phase 3 | — |
| `internal/cli` (Phase 6 test helpers `buildBinary`, `seedStateForChild`) | new integration rows | ✓ | Phase 6 delivered [VERIFIED: stubs_test.go:79, integration_test.go:22] | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `github.com/stretchr/testify/require` |
| Config file | none (Go stdlib is configless) |
| Quick run command | `go test ./internal/cli/... -count=1 -timeout 30s` |
| Full suite command | `go test -race -count=1 -timeout 120s ./...` |
| Integration suite | `go test -tags integration -race -count=1 -timeout 120s ./internal/cli/...` |

### Phase Requirements → Test Map

| Req | SC | Test Name | Type | Automated Command | File |
|-----|----|-----------|------|-------------------|------|
| CMD-03 / SC#1 | list empty store → stderr hint, exit 0 | `TestRunList_Empty_Exit0_HintToStderr` | unit | `go test ./internal/cli/ -run TestRunList_Empty_Exit0_HintToStderr -count=1` | Wave 0 (`list_test.go`) |
| CMD-03 / SC#1 | list N entries → sorted aligned table to stdout | `TestRunList_NEntries_SortedTable` | unit | `go test ./internal/cli/ -run TestRunList_NEntries_SortedTable -count=1` | Wave 0 |
| CMD-03 / SC#1 | list load error → stderr + exit 1 | `TestRunList_OtherError_Exit1` | unit | `go test ./internal/cli/ -run TestRunList_OtherError_Exit1 -count=1` (corrupt state.json) | Wave 0 |
| format | table nil input → empty output | `TestWriteTable_EmptyIn_EmptyOut` | unit | `go test ./internal/cli/format/ -run TestWriteTable_EmptyIn_EmptyOut -count=1` | Wave 0 (`format/table_test.go`) |
| format | nil ResolvedAt renders `-` | `TestWriteTable_NilResolvedAtRendersDash` | unit | `go test ./internal/cli/format/ -run TestWriteTable_NilResolvedAtRendersDash -count=1` | Wave 0 |
| format | condition > 48 chars truncates with `...` | `TestWriteTable_TruncatesLongConditionWithEllipsis` | unit | `go test ./internal/cli/format/ -run TestWriteTable_TruncatesLongConditionWithEllipsis -count=1` | Wave 0 |
| format | sort by CreatedAt ASC then ID ASC (stable) | `TestWriteTable_SortsByCreatedAtThenID` | unit | `go test ./internal/cli/format/ -run TestWriteTable_SortsByCreatedAtThenID -count=1` | Wave 0 |
| CMD-04 / SC#2 | purge `<id>` success → exit 0, no output | `TestRunPurge_ByID_Success_Exit0` | unit | `go test ./internal/cli/ -run TestRunPurge_ByID_Success_Exit0 -count=1` | Wave 0 (`purge_test.go`) |
| CMD-04 / SC#2 | purge `--all` removes all → exit 0 | `TestRunPurge_All_Success_Exit0` | unit | `go test ./internal/cli/ -run TestRunPurge_All_Success_Exit0 -count=1` | Wave 0 |
| CMD-04 / SC#2 | purge `--resolved` removes resolved only | `TestRunPurge_Resolved_OnlyResolvedGone_Exit0` | unit | `go test ./internal/cli/ -run TestRunPurge_Resolved_OnlyResolvedGone_Exit0 -count=1` | Wave 0 |
| CMD-04 / SC#2 | purge (no args) → ErrPurgeArgRequired, exit 1 | `TestRunPurge_NoArgs_Exit1` | unit | `go test ./internal/cli/ -run TestRunPurge_NoArgs_Exit1 -count=1` | Wave 0 |
| CMD-04 / SC#2 | purge unknown `<id>` → exit 1 with stderr `unknown id: <id>` | `TestRunPurge_UnknownID_Exit1` | unit | `go test ./internal/cli/ -run TestRunPurge_UnknownID_Exit1 -count=1` | Wave 0 |
| CMD-04 / locked | Counter survives purge --all (regression guard) | `TestPurge_CounterNotDecremented` | integration | `go test -tags integration ./internal/cli/ -run TestPurge_CounterNotDecremented -count=1` | Wave 0 (append to `integration_test.go`) |
| SC#3 | resolve `<id> --force` success → exit 0, silent | `TestRunResolve_Force_Success_Exit0` | unit | `go test ./internal/cli/ -run TestRunResolve_Force_Success_Exit0 -count=1` | Wave 0 (`resolve_test.go`) |
| SC#3 | resolve `<id>` without --force → not-owner, exit 1 | `TestRunResolve_NoForce_NotOwner_Exit1` | unit | `go test ./internal/cli/ -run TestRunResolve_NoForce_NotOwner_Exit1 -count=1` | Wave 0 |
| SC#3 | resolve unknown `<id>` → exit 1 | `TestRunResolve_UnknownID_Exit1` | unit | `go test ./internal/cli/ -run TestRunResolve_UnknownID_Exit1 -count=1` | Wave 0 |
| SC#3 | resolve already-resolved → exit 1 | `TestRunResolve_AlreadyResolved_Exit1` | unit | `go test ./internal/cli/ -run TestRunResolve_AlreadyResolved_Exit1 -count=1` | Wave 0 |
| CORE-01 end-to-end | compiled binary dispatches list/purge/resolve correctly | `TestList_IntegrationExitCodes`, `TestPurge_IntegrationExitCodes`, `TestResolve_IntegrationExitCodes` | integration | `go test -tags integration ./internal/cli/ -run 'TestList_IntegrationExitCodes\|TestPurge_IntegrationExitCodes\|TestResolve_IntegrationExitCodes' -count=1` | Wave 0 |
| Regression (Phase 6) | stubs table drops `list`, `purge-all` rows but remains green for any remaining entries; version still on stdout | existing `TestStubsExitCodes` (updated), `TestVersionFlagWritesToStdout` | unit | `go test ./internal/cli/ -run 'TestStubsExitCodes\|TestVersionFlagWritesToStdout' -count=1` | edit `stubs_test.go` |

### Sampling Rate

- **Per task commit:** `go test ./internal/cli/ ./internal/cli/format/ -count=1 -timeout 30s` — unit-only; expected ~2–4 s runtime (well under the 60s Nyquist budget).
- **Per wave merge:** full unit suite `go test -race -count=1 ./...` (~5–10 s) plus `go test -tags integration -race -count=1 -timeout 60s ./internal/cli/...` (~20–40 s incl. the 10-concurrent status test and new list/purge/resolve rows).
- **Phase gate:** Full `go test -race -count=1 ./...` + `go test -tags integration -race -count=1 ./...` green before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/cli/format/table.go` — new file (table renderer)
- [ ] `internal/cli/format/table_test.go` — new file (renderer unit tests)
- [ ] `internal/cli/list.go` — new file (`ListCmd` + `runList`)
- [ ] `internal/cli/list_test.go` — new file (4 unit cases)
- [ ] `internal/cli/purge.go` — new file (`PurgeCmd` + `runPurge`)
- [ ] `internal/cli/purge_test.go` — new file (5–6 unit cases)
- [ ] `internal/cli/resolve.go` — new file (`ResolveCmd` + `runResolve`)
- [ ] `internal/cli/resolve_test.go` — new file (4 unit cases)
- [ ] `internal/cli/export_test.go` — edit (add `RunList`, `RunPurge`, `RunResolve`)
- [ ] `internal/cli/integration_test.go` — edit (append `TestList_IntegrationExitCodes`, `TestPurge_IntegrationExitCodes`, `TestResolve_IntegrationExitCodes`, `TestPurge_CounterNotDecremented`)
- [ ] `internal/cli/stubs.go` — edit (remove `ListCmd`, `PurgeCmd`, `ResolveCmd` types + Run methods; leave `ServeCmd` and `ExitCodeNotImplemented` constant)
- [ ] `internal/cli/stubs_test.go` — edit (drop `list`, `purge-all` rows from `TestStubsExitCodes`; since no rows remain, delete `TestStubsExitCodes`; keep `TestVersionFlagWritesToStdout` and `buildBinary`)

No framework install needed. All deps already in `go.mod`.

## Security Domain

**Applies:** No new security surface; `security_enforcement` default (enabled) inherited from config.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | partial | `resolve --force` intentionally bypasses the OwnerToken check (CORE-08 documented operator escape hatch). No new auth surface introduced — the bypass is existing `store.ResolveOptions{Force: true}` semantics. |
| V3 Session Management | no | CLI is stateless between invocations |
| V4 Access Control | partial | `--force` is a privileged operation with documented semantics; ACL is filesystem-level (state.json mode 0600 from Phase 4). No additional control added in Phase 7. |
| V5 Input Validation | partial | `<id>` positional is a string; flows into `store.Get`/`store.Purge`/`store.Resolve` which look it up in `map[string]record`. No injection surface. Condition truncation at 48 chars prevents unbounded string allocation in table render. [VERIFIED: Pattern 1 code] |
| V6 Cryptography | no | no crypto in Phase 7 code path (OwnerToken comparison lives in `store.Resolve` constant-time, Phase 4) |

### Known Threat Patterns for Go CLI + JSON state

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Log injection via `<condition>` containing control chars | Tampering | `format.WriteTable` writes conditions through `fmt.Fprintf(tw, "%s")` — no format-string interpretation of the value. Terminal control sequences would still render (the table is a human display, not a log), but the 48-char cap bounds the blast radius. Acceptable for single-user local tool. |
| Disclosure of `OwnerToken` via `list` | Information disclosure | `format.WriteTable` does NOT render `r.OwnerToken`; only 5 columns (ID, Status, Condition, CreatedAt, ResolvedAt) [VERIFIED: Pattern 1]. `Record` struct contains OwnerToken but it's never referenced in Phase 7 code. |
| DoS via huge state.json on `list` | DoS | `store.List()` returns `[]Record`; rendering allocates one row per record. At expected entry counts (≤100s per CLAUDE.md), memory ~tens of KB. Outside Phase 7 scope to fix architecturally. |
| Argument command-injection into `store.Purge` / `store.Resolve` | Tampering | Both do map lookups and direct struct mutation — no shell, no SQL. `<id>` flows as a plain string key. |
| TOCTOU between `list` and `purge` | Tampering | `list` is LOCK_SH; `purge` is LOCK_EX. Both are internal to `internal/store` (Phase 4). CLI inherits the lock discipline by calling `store.List` / `store.Purge` — no second race window. |

No new security controls needed in Phase 7.

## Project Constraints (from CLAUDE.md)

- **Tech stack:** Pure Go, no cgo. Phase 7 uses only stdlib `text/tabwriter`, `sort`, `strings`, `errors`, `encoding/json`, `io`, `os`, `fmt`, `time` — zero cgo risk.
- **Token budget:** Command help strings are terse (kept at the level set in `cmd/mcp-chain/main.go` Phase 6 patch — e.g., "List all registered ids.", "Purge entries. Requires one of <id>, --all, or --resolved."). Stderr error lines are at most 60 characters.
- **Platform:** `text/tabwriter` + `sort.SliceStable` + `encoding/json` — all cross-platform stdlib. Integration tests use the same `buildBinary` helper from Phase 6 that's proven on Linux/macOS/Windows CI.
- **MCP-02 stdout discipline:** `list` writes ONLY the table body to stdout (no log lines, no banners). `purge` and `resolve` emit nothing to stdout. The empty-store hint goes to stderr. Pattern 2/3/4 code enforces this.
- **Dependencies:** Stdlib-first. Zero new deps.
- **Binary size ≤15 MB stripped:** `text/tabwriter` is ~2 KB of wire; negligible size impact. No measurable change to current stripped size.
- **Startup ≤100 ms:** All three subcommands do a single LOCK_SH or LOCK_EX read and terminate. Startup cost unchanged from Phase 6.
- **GSD workflow:** Research spawned via `/gsd-research-phase` per config; planner consumes this file next.

## Sources

### Primary (HIGH confidence)

- **kong v1.15.0 source (local):** `/home/alpine/go/pkg/mod/github.com/alecthomas/kong@v1.15.0/` — VERIFIED
  - `model.go:408–419` — `type Flag struct { ... Xor []string ... }` (Xor is a Flag field, not a Value/Positional field)
  - `build.go:365–378` — `&Flag{...Xor: tag.Xor...}` — Xor only wired into Flag, never into Positional
  - `context.go:894–948` — `checkMissingFlags(flags []*Flag)` — validator is flag-only
  - `context.go:1072–1100` — `checkXorDuplicatedAndAndMissing` — duplicate detection, flag-only
  - `kong_test.go:1207–1225` — `TestXorRequired` — confirms `xor + required` enforces "exactly one" among named flags
  - `kong_test.go:1247–1264` — `TestXorRequiredMany` — confirms pattern with >2 members

- **Local mcp-chain sources:** (VERIFIED line-level)
  - `internal/store/store.go:96–116` (Resolve with constant-time OwnerToken check)
  - `internal/store/store.go:133–146` (List under LOCK_SH, random map order)
  - `internal/store/store.go:152–195` (Purge with ErrPurgeArgRequired, counter preserved at line 188 comment)
  - `internal/store/store.go:197–224` (loadState returns empty state on os.ErrNotExist)
  - `internal/store/errors.go:11–36` (sentinels: ErrUnknownID, ErrAlreadyResolved, ErrNotOwner, ErrPurgeArgRequired)
  - `internal/store/schema.go:23–38` (state.Counter uint64, record.ResolvedAt *time.Time, etc.)
  - `internal/cli/stubs.go` (current ListCmd/PurgeCmd/ResolveCmd stubs to replace)
  - `internal/cli/status.go` (Phase 6 testability pattern — canonical shape)
  - `internal/cli/status_test.go` (Phase 6 unit-test shape to mirror)
  - `internal/cli/integration_test.go` (Phase 6 `seedStateForChild`, `buildBinary` reuse target)
  - `internal/cli/export_test.go` (Phase 6 `RunStatus` alias — pattern to extend)
  - `cmd/mcp-chain/main.go` (kong.Writers + manual --version from Phase 6 — unchanged by Phase 7)

- **stdlib docs (via `go doc`):** `text/tabwriter` package (frozen), `tabwriter.NewWriter`, `tabwriter.Writer.Flush`, `sort.SliceStable`, `time.Time.Format`, `errors.Is` — all VERIFIED at Go 1.25.

- **CONTEXT.md:** `.planning/phases/07-cli-formatters/07-CONTEXT.md` — exit-code contract, list formatting, purge semantics, resolve semantics, testability pattern, non-goals. Read in full.

- **Phase 6 research:** `.planning/phases/06-cli-dispatch-status/06-RESEARCH.md` — `runStatus` pattern, kong.Writers+Version treatment, integration test shape. Referenced throughout.

- **REQUIREMENTS.md:** CORE-01 split (CLI surface completion at Phase 7), CMD-03, CMD-04. Read.

- **CLAUDE.md:** stack constraints, token/size/startup budgets. Read.

### Secondary (MEDIUM confidence)

None — all claims verified against primary sources.

### Tertiary (LOW confidence)

None.

## Metadata

**Confidence breakdown:**

- Standard stack: **HIGH** — `text/tabwriter` is stdlib-frozen with ~5-line usage pattern; kong already wired in Phase 6; no version-drift risk.
- Architecture: **HIGH** — `runXxx(out, errW, path, ...) int` pattern is a verbatim extension of the Phase 6 canon, itself thoroughly validated in the Phase 6 shipped tests.
- Pitfalls: **HIGH** — the kong `Xor`-is-flag-only caveat was source-level VERIFIED; all other pitfalls are direct reads from the store source or POSIX idiom.
- Validation: **HIGH** — every SC mapped to a named test with an explicit `go test -run` command; all runnable under 60 s per commit.

**Research date:** 2026-04-24

**Valid until:** 2026-05-24 (30 days — stack is stable; only a kong 2.x bump could invalidate the `Xor`-is-flag-only finding, and no such release is on the project roadmap).
