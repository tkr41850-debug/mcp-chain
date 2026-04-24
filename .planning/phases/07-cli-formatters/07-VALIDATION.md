---
phase: 7
slug: cli-formatters
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-24
---

# Phase 7 — Validation Strategy

> Per-phase validation contract. Content derived from `07-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| **Config file** | none |
| **Quick run command** | `go test -race ./internal/cli/... ./internal/cli/format/... -count=1 -timeout 30s` |
| **Full suite command** | `go test -race -count=1 -timeout 60s ./...` |
| **Integration command** | `go test -race -count=1 -tags=integration -timeout 180s ./internal/cli/...` |
| **Smoke gates** | counter-non-decrement JSON inspection; stdout purity |
| **Estimated runtime** | ~2–4 s unit; ~30 s integration |

---

## Sampling Rate

- **After every task commit:** `go test -race ./internal/cli/ ./internal/cli/format/ -count=1 -timeout 30s` (~2–4 s)
- **After every plan wave:** `go test -race -tags=integration -count=1 -timeout 180s ./internal/cli/...`
- **Before `/gsd-verify-work`:** Full race suite + integration + `go vet`
- **Max feedback latency:** <5 s per-commit

---

## Per-Task Verification Map

| Task ID | Wave | Requirement | SC | Test / Gate | Type | Automated Command | Status |
|---------|------|-------------|----|-------------|------|-------------------|--------|
| 7-0-format | 0 | SC #1 | 1 | `TestWriteTable_EmptyIn_EmptyOut` — nil/empty records produces no output | unit | `go test -race ./internal/cli/format/ -run TestWriteTable_EmptyIn_EmptyOut -count=1` | ⬜ |
| 7-0-format | 0 | SC #1 | 1 | `TestWriteTable_NilResolvedAtRendersDash` — pending records render `-` in Resolved column | unit | `go test -race ./internal/cli/format/ -run TestWriteTable_NilResolvedAtRendersDash -count=1` | ⬜ |
| 7-0-format | 0 | SC #1 | 1 | `TestWriteTable_TruncatesLongConditionWithEllipsis` — condition > 48 chars truncates with `...` | unit | `go test -race ./internal/cli/format/ -run TestWriteTable_TruncatesLongConditionWithEllipsis -count=1` | ⬜ |
| 7-0-format | 0 | SC #1 | 1 | `TestWriteTable_SortsByCreatedAtThenID` — stable sort by CreatedAt ASC then ID ASC | unit | `go test -race ./internal/cli/format/ -run TestWriteTable_SortsByCreatedAtThenID -count=1` | ⬜ |
| 7-1-list | 1 | CMD-03 | 1 | `TestRunList_Empty_Exit0_HintToStderr` — empty store: stderr hint + exit 0 + empty stdout | unit | `go test -race ./internal/cli/ -run TestRunList_Empty_Exit0_HintToStderr -count=1` | ⬜ |
| 7-1-list | 1 | CMD-03 | 1 | `TestRunList_NEntries_SortedTable` — N seeded entries render to stdout in sorted order | unit | `go test -race ./internal/cli/ -run TestRunList_NEntries_SortedTable -count=1` | ⬜ |
| 7-1-list | 1 | CMD-03 | 1 | `TestRunList_OtherError_Exit1` — corrupt state.json: stderr + exit 1 | unit | `go test -race ./internal/cli/ -run TestRunList_OtherError_Exit1 -count=1` | ⬜ |
| 7-2-purge | 1 | CMD-04 | 2 | `TestRunPurge_ByID_Success_Exit0` — single id removed silently, exit 0 | unit | `go test -race ./internal/cli/ -run TestRunPurge_ByID_Success_Exit0 -count=1` | ⬜ |
| 7-2-purge | 1 | CMD-04 | 2 | `TestRunPurge_All_Success_Exit0` — `--all` clears records | unit | `go test -race ./internal/cli/ -run TestRunPurge_All_Success_Exit0 -count=1` | ⬜ |
| 7-2-purge | 1 | CMD-04 | 2 | `TestRunPurge_Resolved_OnlyResolvedGone_Exit0` — `--resolved` leaves pending records | unit | `go test -race ./internal/cli/ -run TestRunPurge_Resolved_OnlyResolvedGone_Exit0 -count=1` | ⬜ |
| 7-2-purge | 1 | CMD-04 | 2 | `TestRunPurge_NoArgs_Exit1` — bare `purge`: stderr + exit 1 (ErrPurgeArgRequired wired) | unit | `go test -race ./internal/cli/ -run TestRunPurge_NoArgs_Exit1 -count=1` | ⬜ |
| 7-2-purge | 1 | CMD-04 | 2 | `TestRunPurge_UnknownID_Exit1` — unknown id: stderr `unknown id: <id>` + exit 1 | unit | `go test -race ./internal/cli/ -run TestRunPurge_UnknownID_Exit1 -count=1` | ⬜ |
| 7-3-resolve | 1 | — | 3 | `TestRunResolve_Force_Success_Exit0` — `--force` bypasses OwnerToken, record becomes resolved | unit | `go test -race ./internal/cli/ -run TestRunResolve_Force_Success_Exit0 -count=1` | ⬜ |
| 7-3-resolve | 1 | — | 3 | `TestRunResolve_NoForce_NotOwner_Exit1` — no `--force` against owner-stamped record: `not owner (use --force to override)` | unit | `go test -race ./internal/cli/ -run TestRunResolve_NoForce_NotOwner_Exit1 -count=1` | ⬜ |
| 7-3-resolve | 1 | — | 3 | `TestRunResolve_UnknownID_Exit1` — unknown id: stderr + exit 1 | unit | `go test -race ./internal/cli/ -run TestRunResolve_UnknownID_Exit1 -count=1` | ⬜ |
| 7-3-resolve | 1 | — | 3 | `TestRunResolve_AlreadyResolved_Exit1` — second resolve fails with `already resolved` | unit | `go test -race ./internal/cli/ -run TestRunResolve_AlreadyResolved_Exit1 -count=1` | ⬜ |
| 7-4-integration | 2 | CMD-03/04 | 1, 2, 3 | `TestList_IntegrationExitCodes` — compiled binary list works end-to-end | integration | `go test -race -tags=integration ./internal/cli/ -run TestList_IntegrationExitCodes -count=1` | ⬜ |
| 7-4-integration | 2 | CMD-04 | 2 | `TestPurge_IntegrationExitCodes` — compiled binary purge dispatch | integration | `go test -race -tags=integration ./internal/cli/ -run TestPurge_IntegrationExitCodes -count=1` | ⬜ |
| 7-4-integration | 2 | — | 3 | `TestResolve_IntegrationExitCodes` — compiled binary resolve dispatch | integration | `go test -race -tags=integration ./internal/cli/ -run TestResolve_IntegrationExitCodes -count=1` | ⬜ |
| 7-4-integration | 2 | regression | 2 | `TestPurge_CounterNotDecremented` — read state.json before/after `purge --all`; counter unchanged | integration | `go test -race -tags=integration ./internal/cli/ -run TestPurge_CounterNotDecremented -count=1` | ⬜ |
| 7-5-stubs | 3 | cleanup | — | `stubs_test.go` rows for list/purge/resolve removed; `TestStubsExitCodes` deleted (empty); `TestVersionFlagWritesToStdout` retained | unit | `go test -race ./internal/cli/ -run TestVersionFlagWritesToStdout -count=1` | ⬜ |
| 7-5-stubs | 3 | smoke | — | `stubs.go` no longer declares `ListCmd` / `PurgeCmd` / `ResolveCmd`; each lives in its own file | smoke | `test -f internal/cli/list.go && test -f internal/cli/purge.go && test -f internal/cli/resolve.go && ! grep -qE 'type (List\|Purge\|Resolve)Cmd' internal/cli/stubs.go` | ⬜ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cli/format/table.go` — `WriteTable(w io.Writer, records []store.Record) error` using `text/tabwriter`; RFC3339 UTC timestamps; 48-char condition truncation with `...`; stable sort by CreatedAt ASC then ID ASC
- [ ] `internal/cli/format/table_test.go` — 4 renderer unit tests
- [ ] `internal/cli/list.go` — `ListCmd` + `runList(out, errW io.Writer, path string) int`; empty → stderr hint + exit 0
- [ ] `internal/cli/list_test.go` — 3 unit tests
- [ ] `internal/cli/purge.go` — `PurgeCmd` (`xor:"target"` on --all/--resolved, positional id) + `runPurge(out, errW io.Writer, path, id string, all, resolvedOnly bool) int`; bare → `ErrPurgeArgRequired` via store's validation
- [ ] `internal/cli/purge_test.go` — 5 unit tests
- [ ] `internal/cli/resolve.go` — `ResolveCmd` (`<id>` arg + `--force` flag) + `runResolve(out, errW io.Writer, path, id string, force bool) int`
- [ ] `internal/cli/resolve_test.go` — 4 unit tests
- [ ] `internal/cli/export_test.go` — append `RunList`, `RunPurge`, `RunResolve`
- [ ] `internal/cli/integration_test.go` — append 4 integration rows (list, purge, resolve exit codes + counter-non-decrement)
- [ ] `internal/cli/stubs.go` — remove `ListCmd` / `PurgeCmd` / `ResolveCmd` declarations (keep `ServeCmd` wiring if still applicable — Phase 5 already handled that)
- [ ] `internal/cli/stubs_test.go` — drop list/purge/resolve rows from `TestStubsExitCodes`; since no rows remain, delete `TestStubsExitCodes`; keep `TestVersionFlagWritesToStdout` + `buildBinary` helper

**Framework install:** None. Zero new `go.mod` deps.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Visual alignment of the list table in a real terminal | CMD-03 | `tabwriter` output can look ragged with multi-byte condition strings; hard to unit-test "looks right" | Phase 10 dogfooding will confirm table reads cleanly; unit tests verify byte-level column boundaries |
| Operator usability of `resolve <id>` not-owner hint | SC #3 | Subjective — "does the --force hint nudge the user correctly?" | Phase 10 dogfooding / README screenshots |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have automated verify or Wave 0 dependencies
- [x] Sampling continuity: every test maps to a commit-level verify (<5 s unit)
- [x] Wave 0 self-contained (depends only on Phase 4 store + Phase 6 kong wiring)
- [x] No watch-mode flags
- [x] Feedback latency < 60 s (<5 s unit; wave-end integration ~30 s)
- [x] `nyquist_compliant: true` set
- [x] All 3 SC mapped to ≥1 test (SC #1 → list×3 + format×4; SC #2 → purge×5 + counter-regression integration; SC #3 → resolve×4)

**Approval:** approved 2026-04-24 (autonomous mode — no open questions, defaults applied)
