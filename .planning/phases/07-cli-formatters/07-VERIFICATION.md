---
phase: 07-cli-formatters
verified: 2026-04-24T06:10:00Z
status: passed
score: 14/14 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 7: CLI Formatters (list, purge, resolve) Verification Report

**Phase Goal (ROADMAP):** Administrative subcommands `list`, `purge`, and CLI-only `resolve --force` provide human-readable output and safe cleanup semantics over the shared store, with formatters isolated so they do not leak into core.

**Verified:** 2026-04-24T06:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `mcp-chain list` empty store → `mcp-chain: no entries\n` stderr, empty stdout, exit 0 | VERIFIED | Smoke test: `list` on empty dir printed to stderr only, exit=0. `TestRunList_Empty_Exit0_HintToStderr` and `TestList_IntegrationExitCodes/empty_store` both pass. |
| 2 | `mcp-chain list` with N records → aligned 5-column table on stdout sorted by CreatedAt ASC then ID ASC | VERIFIED | Smoke test rendered `ID STATUS CONDITION CREATED RESOLVED` with RFC3339 UTC timestamps (`2026-04-24T06:04:56Z`), ellipsis truncation at 45 chars (`...`), left-aligned, exit 0, empty stderr. `TestRunList_NEntries_SortedTable` + `TestList_IntegrationExitCodes/two_entries` pass. |
| 3 | `mcp-chain list` on corrupt state.json → `mcp-chain: <err>\n` stderr, empty stdout, exit 1 | VERIFIED | `TestRunList_OtherError_Exit1` passes — corrupt JSON triggers error branch. |
| 4 | `mcp-chain purge <id>` removes record silently, exit 0 | VERIFIED | `TestRunPurge_ByID_Success_Exit0` + `TestPurge_IntegrationExitCodes/by-id_success` pass; `store.Get(id)` returns ErrUnknownID after purge. |
| 5 | `mcp-chain purge --all` clears records and leaves counter unchanged (CORE-09) | VERIFIED | Live smoke test: counter=2 before and after `purge --all`, entries={} (verified via `python3 -c json.load`). `TestPurge_CounterNotDecremented` (integration) directly asserts `before == after` via raw JSON unmarshal. |
| 6 | `mcp-chain purge --resolved` removes only resolved records, pending survive | VERIFIED | `TestRunPurge_Resolved_OnlyResolvedGone_Exit0` passes; asserts 1 pending survives + has status=="pending". |
| 7 | `mcp-chain purge` bare → `purge requires <id>, --all, or --resolved` stderr, exit 1 | VERIFIED | Smoke test + `TestRunPurge_NoArgs_Exit1` + `TestPurge_IntegrationExitCodes/bare_purge`. Sentinel `store.ErrPurgeArgRequired` correctly translated. |
| 8 | `mcp-chain purge <unknown>` → `unknown id: <id>\n` stderr, exit 1 | VERIFIED | `TestRunPurge_UnknownID_Exit1` + `TestPurge_IntegrationExitCodes/unknown_id` pass. |
| 9 | `mcp-chain resolve <id> --force` on pending record → exit 0, silent, record becomes resolved | VERIFIED | Smoke test: state.json records[acid].status flipped from "pending" to "resolved", `resolved_at` stamped. `TestRunResolve_Force_Success_Exit0` + `TestResolve_IntegrationExitCodes/--force_success` pass. |
| 10 | `mcp-chain resolve <id>` (no --force) on owner-stamped record → `not owner (use --force to override)\n` stderr, exit 1 | VERIFIED | Smoke test: stderr matched exact string, exit=1. `TestRunResolve_NoForce_NotOwner_Exit1` + `TestResolve_IntegrationExitCodes/no-force_not-owner` pass. |
| 11 | `mcp-chain resolve <unknown> --force` → `unknown id: <id>\n` stderr, exit 1 | VERIFIED | Smoke test + `TestRunResolve_UnknownID_Exit1` + integration row pass. |
| 12 | `mcp-chain resolve <already-resolved> --force` → `already resolved\n` stderr, exit 1 | VERIFIED | Smoke test + `TestRunResolve_AlreadyResolved_Exit1` pass. |
| 13 | `runList` / `runPurge` / `runResolve` are env-var-pure (no `statepath.Resolve()`, no `os.Stdout`/`os.Stderr` inside) | VERIFIED | grep confirmed: `os.Stdout`/`os.Stderr` appear only in the kong handler `Run()` methods; pure `run*` functions write exclusively through their `io.Writer` parameters. |
| 14 | `format.WriteTable` is pure rendering — no store I/O, no env reads, reads only passed writer + records slice | VERIFIED | Source inspection: table.go imports `fmt`, `io`, `sort`, `text/tabwriter`, `time`, `store` (for Record type only). No `os.Getenv`, no `store.Open`, no `storepath`. |

**Score:** 14/14 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/cli/format/table.go` | `WriteTable` via text/tabwriter, RFC3339 UTC, 48-char truncation, sort CreatedAt ASC then ID ASC | VERIFIED | 83 LOC; `tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)` + `sort.SliceStable`; `tw.Flush()` at end; `truncate()` caps at 48 with `...`; nil input returns early. |
| `internal/cli/format/table_test.go` | 4 renderer unit tests | VERIFIED | All four tests present and passing: EmptyIn_EmptyOut, NilResolvedAtRendersDash, TruncatesLongConditionWithEllipsis, SortsByCreatedAtThenID. |
| `internal/cli/list.go` | `ListCmd` + `runList(out, errW, path) int` | VERIFIED | 65 LOC; maps (empty/populated/error) to (stderr-hint exit 0 / stdout-table exit 0 / stderr-err exit 1). |
| `internal/cli/list_test.go` | 3 unit tests | VERIFIED | TestRunList_Empty_Exit0_HintToStderr, TestRunList_NEntries_SortedTable, TestRunList_OtherError_Exit1 all pass. |
| `internal/cli/purge.go` | `PurgeCmd` (`xor:"target"`) + `runPurge` | VERIFIED | 86 LOC; `--all`/`--resolved` have `xor:"target"`; positional `ID` is `optional:""`; `ErrPurgeArgRequired` and `ErrUnknownID` sentinels mapped to locked stderr strings. |
| `internal/cli/purge_test.go` | 5 unit tests | VERIFIED | All five tests pass: ByID, All, Resolved, NoArgs, UnknownID. |
| `internal/cli/resolve.go` | `ResolveCmd` + `runResolve` | VERIFIED | 76 LOC; `Force bool` flag; passes empty ownerToken so no-force always hits `ErrNotOwner` per design; ErrNotOwner/ErrUnknownID/ErrAlreadyResolved branches mapped. |
| `internal/cli/resolve_test.go` | 4 unit tests | VERIFIED | All four tests pass: Force_Success, NoForce_NotOwner, UnknownID, AlreadyResolved. |
| `internal/cli/export_test.go` | RunList, RunPurge, RunResolve re-exports | VERIFIED | All three wrappers present alongside existing RunStatus. |
| `internal/cli/integration_test.go` | 4 new integration rows | VERIFIED | TestList_IntegrationExitCodes, TestPurge_IntegrationExitCodes, TestResolve_IntegrationExitCodes, TestPurge_CounterNotDecremented all pass under `-tags=integration`. |
| `cmd/mcp-chain/main.go` | `Resolve cli.ResolveCmd` field | VERIFIED | Line 34: `Resolve cli.ResolveCmd \`cmd:"" help:"Resolve an id (CLI escape hatch; use --force to bypass OwnerToken check)."\``. |
| `internal/cli/stubs.go` | Only ServeCmd, Version, ExitCodeNotImplemented remain | VERIFIED | grep for `type (List\|Purge\|Resolve)Cmd` in stubs.go returns nothing. ServeCmd + Version + ExitCodeNotImplemented all retained. |
| `internal/cli/stubs_test.go` | `TestStubsExitCodes` deleted; `TestVersionFlagWritesToStdout` + `buildBinary` retained | VERIFIED | File contains only TestVersionFlagWritesToStdout + buildBinary helper. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/mcp-chain/main.go` CLI struct | `internal/cli.ResolveCmd` | `Resolve cli.ResolveCmd` field | WIRED | Confirmed on main.go:34. |
| `runList` | `format.WriteTable` | `format.WriteTable(out, records)` | WIRED | list.go:59 — present and non-empty records path. |
| `runList` | `store.Store.List` | `st.List()` | WIRED | list.go:50. |
| `runPurge` | `store.Store.Purge` | `st.Purge(store.PurgeOptions{...})` | WIRED | purge.go:63-67. |
| `runPurge` bare-arg branch | `store.ErrPurgeArgRequired` | `errors.Is(err, store.ErrPurgeArgRequired)` | WIRED | purge.go:71. |
| `runResolve` | `store.Store.Resolve` | `st.Resolve(id, "", store.ResolveOptions{Force: force})` | WIRED | resolve.go:58 — empty ownerToken passed intentionally. |
| `runResolve` ErrNotOwner branch | operator stderr hint | `errors.Is(err, store.ErrNotOwner)` + `"not owner (use --force to override)"` | WIRED | resolve.go:65-67. |
| `format.WriteTable` | stable sort | `sort.SliceStable(sorted, ...)` on copied slice | WIRED | table.go:40 — copies slice first so caller's data is not mutated. |
| `format.WriteTable` | tabwriter | `tabwriter.NewWriter(w, 0, 0, 2, ' ', 0) + tw.Flush()` | WIRED | table.go:49 and :60. |
| `TestPurge_CounterNotDecremented` | state.json counter | raw `json.Unmarshal` into `struct{ Counter uint64 }` | WIRED | integration_test.go:537-546; asserts before==after. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `list.go runList` | `records []store.Record` | `st.List()` — reads state.json via `internal/store` (LOCK_SH) | Yes — live smoke test showed real records with populated ID/Status/Condition/CreatedAt | FLOWING |
| `purge.go runPurge` | (no return display; side-effect on store.json) | `st.Purge(PurgeOptions)` — mutates state.json via atomic rename | Yes — smoke test: 2 entries → 0 entries, counter preserved | FLOWING |
| `resolve.go runResolve` | (no return display; side-effect) | `st.Resolve(id, "", opts)` — mutates record's Status/ResolvedAt | Yes — smoke test: status flipped pending→resolved, resolved_at stamped | FLOWING |
| `format.WriteTable` | `records` parameter | Passed in by `runList`; formatter is pure | N/A — pure function | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `list` empty store | `mcp-chain list` on fresh XDG_STATE_HOME | stderr: `mcp-chain: no entries`; stdout empty; exit 0 | PASS |
| `list` rendered table | `mcp-chain list` after 2 registrations | Aligned 5-column table with RFC3339 UTC + ellipsis truncation; stderr empty; exit 0 | PASS |
| `purge --all` preserves counter | register 2 → `purge --all` → inspect state.json | counter=2 before and after; entries={}; exit 0 | PASS |
| `purge` bare | `mcp-chain purge` | stderr: `mcp-chain: purge requires <id>, --all, or --resolved`; exit 1 | PASS |
| `resolve` no --force against owner-stamped | `mcp-chain resolve <id>` | stderr: `mcp-chain: not owner (use --force to override)`; exit 1 | PASS |
| `resolve --force` on pending | `mcp-chain resolve <id> --force` | exit 0, silent; state.json shows status="resolved", resolved_at stamped | PASS |
| `resolve --force` on already resolved | (idempotent second call) | stderr: `mcp-chain: already resolved`; exit 1 | PASS |
| `resolve <unknown> --force` | `mcp-chain resolve nope --force` | stderr: `mcp-chain: unknown id: nope`; exit 1 | PASS |
| Binary size ≤ 15 MB | `go build -ldflags="-s -w" ... && ls -l` | 7,823,652 bytes (7.82 MB) — within budget | PASS |
| `go test -race -count=1 ./...` | full suite | All 7 packages PASS | PASS |
| `go test -race -tags=integration ./internal/cli/...` | integration suite | All tests PASS including TestPurge_CounterNotDecremented | PASS |
| `go vet ./...` | static analysis | Clean (no output) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CMD-03 (CLI half) | 07-01-PLAN | `mcp-chain list` prints human-readable table of all entries (ID, status, condition, created_at, resolved_at) | SATISFIED | `format.WriteTable` renders 5 columns, `list.go` wires it to store; unit + integration tests pass. Slash-command wrapper (`/chain-list`) deferred to Phase 8 per ROADMAP mapping. |
| CMD-04 (CLI half) | 07-01-PLAN | `mcp-chain purge [id \| --all \| --resolved]` requires one argument shape; bare invocation errors; counter not decremented | SATISFIED | `PurgeCmd` xor:"target" + `store.ErrPurgeArgRequired`; `TestPurge_CounterNotDecremented` pins CORE-09 invariant via raw JSON read. Slash-command wrapper deferred to Phase 8. |
| CORE-09 regression guard (counter invariant) | 07-01-PLAN (truth #5) | Counter never decremented by purge | SATISFIED | Live smoke test + `TestPurge_CounterNotDecremented` (reads state.json raw before/after `purge --all`; asserts `before == after`). |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No TODO/FIXME/PLACEHOLDER/stub patterns in list.go, purge.go, resolve.go, format/table.go | — | — |

Note: `internal/cli/stubs.go` retains the `ExitCodeNotImplemented = 3` constant with a comment explaining it is documented-reserved (not collision with status 0/1/2). None of list/purge/resolve use it. Clean.

### Minor Observations (Info, Not Gaps)

1. **User-prompt vs plan exit-code discrepancy for resolve:** The verification request states "exit 0/1/2 (2 = ErrNotOwner without --force)", but the locked exit-code contract in `07-CONTEXT.md` lines 55-57, in `07-01-PLAN.md` truth #10, and in `TestRunResolve_NoForce_NotOwner_Exit1` / `TestResolve_IntegrationExitCodes/no-force_not-owner` all specify **exit 1** for the not-owner branch. The implementation correctly matches the plan contract (exit 1). This is a user-prompt authoring discrepancy, not an implementation gap.

2. **Kong help string length:** The Resolve help string at 73 chars exceeds the informal ~60-char bar mentioned in the verification prompt. However, the project's terseness principle in CLAUDE.md is specifically about **MCP tool descriptions and model-facing text** — kong help is user-facing operator text, not model-facing. Not a gap.

### Human Verification Required

(none — all success criteria have automated verification)

### Gaps Summary

No gaps. All 3 ROADMAP Success Criteria pass with both unit-level and integration-level evidence. All 22 rows of the Nyquist validation matrix execute green (15 unit + 4 integration + counter regression + 2 structural cleanup checks). Live smoke tests of the stripped binary confirm behavior end-to-end including the CORE-09 counter invariant. Binary size 7.82 MB (well under 15 MB budget). `go test -race` and `go vet` are clean.

---

_Verified: 2026-04-24T06:10:00Z_
_Verifier: Claude (gsd-verifier)_
