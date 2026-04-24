---
phase: 07-cli-formatters
plan: 07-01
status: complete
completed: 2026-04-24
---

# Phase 7 Plan 01 — Summary

## Goal Delivered

CLI administrative subcommands `list`, `purge`, and `resolve --force` land on top of the shared store with:

- `internal/cli/format/table.go` — reusable `WriteTable` helper (text/tabwriter, RFC3339 UTC, 48-char condition truncation, CreatedAt-then-ID sort, `-` for nil ResolvedAt).
- `internal/cli/list.go` — `runList` decision tree (exit 0 with table OR 0 with stderr hint / 1 on store error).
- `internal/cli/purge.go` — `runPurge` with kong `xor:"target"` flag group (`--all` / `--resolved`) and positional `<id>`; store-side `ErrPurgeArgRequired` enforces exactly-one-target; counter never decremented.
- `internal/cli/resolve.go` — `runResolve` with `--force` escape hatch bypassing OwnerToken; exit 0 (success) / 1 (any error — `ErrUnknownID`, `ErrNotOwner`, `ErrAlreadyResolved`).
- `cmd/mcp-chain/main.go` — `Resolve cli.ResolveCmd` field wired under kong.

## Commits (6, linear)

| SHA       | Subject                                                                           |
| --------- | --------------------------------------------------------------------------------- |
| `7c11b88` | feat(07-01): add internal/cli/format with WriteTable                              |
| `32f142a` | feat(07-01): wire list command                                                    |
| `be3ea4d` | feat(07-01): wire purge command                                                   |
| `1bc5dc7` | feat(07-01): wire resolve command                                                 |
| `6a9735f` | test(07-01): integration suite — exit codes + counter-non-decrement regression    |
| `bc8b0b5` | chore(07-01): delete empty TestStubsExitCodes; all stubs migrated                 |

## Tests Added

- **16 unit tests**: 4 format (`TestWriteTable_*`), 3 list, 5 purge, 4 resolve.
- **4 integration test functions / 10 sub-cases**: `TestList_IntegrationExitCodes` (2), `TestPurge_IntegrationExitCodes` (4), `TestResolve_IntegrationExitCodes` (3), `TestPurge_CounterNotDecremented` (CORE-09 regression).
- **1 unit test removed**: empty `TestStubsExitCodes` after stub migration.

All green under `go test -race -count=1 ./...` and `go test -race -count=1 -tags=integration ./...`.

## Quality Gates

- Code review (`07-REVIEW.md`): 0 HI / 0 ME / 5 LO (info-only, non-blocking).
- Verifier (`07-VERIFICATION.md`): 14/14 must-haves PASS; all 3 SCs covered.
- `go vet ./...`: clean.
- Stripped binary: **7.82 MB** (limit 15 MB).
- Zero new dependencies — stdlib only (`text/tabwriter`, `sort`, `encoding/json`, `io`, `os`, `time`, `errors`, `fmt`).

## Locked Decisions Honored

- LD-1: exit-code contracts list=0/1, purge=0/1, resolve=0/1 (no 2 for resolve per locked contract).
- LD-2: `text/tabwriter` (minwidth=0, tabwidth=0, padding=2, padchar=' ', flags=0).
- LD-3: `xor:"target"` kong tag — flag-only per kong `model.go:408`; positional `<id>` handled separately.
- LD-4–14: RFC3339 UTC, 48-char truncation, counter non-decrement, pure `runX(out, errW, path, ...) int` mirroring Phase 6, etc.

## Prompt-Authoring Note

Both review and verify agents noted the orchestrator's "exit 2 = ErrNotOwner without --force" phrasing for resolve was a prompt typo — the locked contract (07-CONTEXT.md + plan) specifies exit 1 for all resolve errors, which is what the code does. No drift.

## Next

Phase 8 — Plugin Packaging & Bash Monitor.
