---
phase: 6
slug: cli-dispatch-status
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-24
---

# Phase 6 — Validation Strategy

> Per-phase validation contract. Content derived from `06-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| **Config file** | none |
| **Quick run command** | `go test -race ./internal/cli/... -count=1 -timeout 30s` |
| **Full suite command** | `go test -race -count=1 -timeout 60s ./...` |
| **Integration command** | `go test -race -count=1 -tags=integration -timeout 120s ./internal/cli/...` |
| **Smoke gates** | stdout purity (bash `cmp /dev/null`), binary dispatch end-to-end |
| **Estimated runtime** | ~2 s unit; ~15–30 s integration (dominated by 10-concurrent test) |

---

## Sampling Rate

- **After every task commit:** `go test -race ./internal/cli/... -count=1 -timeout 30s` (~1–2 s)
- **After every plan wave:** `go test -race -tags=integration -count=1 -timeout 120s ./internal/cli/...` (+ full unit suite)
- **Before `/gsd-verify-work`:** Full race suite (`go test -race -count=1 ./...`) + integration (`go test -race -tags=integration -count=1 ./...`) + `go vet`
- **Max feedback latency:** <5 s per-commit (unit); <30 s per-wave (integration)

---

## Per-Task Verification Map

| Task ID | Wave | Requirement | SC | Test / Gate | Type | Automated Command | Status |
|---------|------|-------------|----|-------------|------|-------------------|--------|
| 6-0-main | 0 | SC #3 | 3 | `cmd/mcp-chain/main.go` uses `kong.Writers(os.Stderr, os.Stderr)`; `VersionFlag` removed; manual `--version` pre-parse added | smoke | `grep -q 'kong.Writers' cmd/mcp-chain/main.go && ! grep -q 'kong.VersionFlag' cmd/mcp-chain/main.go` | ⬜ |
| 6-0-main | 0 | regression | — | `--version` still prints to stdout with exit 0 (integration assertion below also covers) | smoke | (covered by integration row `TestVersion_StdoutExit0`) | ⬜ |
| 6-1-status | 1 | CORE-01 | 1 | `TestRunStatus_Resolved_Exit0` — `resolved\n` on stdout, empty stderr, returns 0 | unit | `go test -race ./internal/cli/ -run TestRunStatus_Resolved_Exit0 -count=1` | ⬜ |
| 6-1-status | 1 | CORE-01 | 1 | `TestRunStatus_Pending_Exit2` — `pending\n` on stdout, empty stderr, returns 2 | unit | `go test -race ./internal/cli/ -run TestRunStatus_Pending_Exit2 -count=1` | ⬜ |
| 6-1-status | 1 | CORE-01 | 1 | `TestRunStatus_Unknown_Exit1` — empty stdout, `mcp-chain: unknown id: <id>\n` on stderr, returns 1 | unit | `go test -race ./internal/cli/ -run TestRunStatus_Unknown_Exit1 -count=1` | ⬜ |
| 6-1-status | 1 | CORE-01 | 3 | `TestRunStatus_StdoutIsJustStatus` — resolved/pending stdout is EXACTLY the status string + newline; no banner | unit | `go test -race ./internal/cli/ -run TestRunStatus_StdoutIsJustStatus -count=1` | ⬜ |
| 6-1-status | 1 | CORE-01 | 1 | `TestRunStatus_GenericError_Exit1` — schema/corrupt errors folded into exit 1 with `mcp-chain: <err>` stderr | unit | `go test -race ./internal/cli/ -run TestRunStatus_GenericError_Exit1 -count=1` | ⬜ |
| 6-2-dispatch | 2 | CORE-01 | 1 | `TestStatus_IntegrationExitCodes` — compiled binary: resolved→0, pending→2, unknown→1 via `cmd.ProcessState.ExitCode()` | integration | `go test -race -tags=integration ./internal/cli/ -run TestStatus_IntegrationExitCodes -count=1` | ⬜ |
| 6-2-dispatch | 2 | CORE-01 | 3 | `TestStatus_StdoutOnlyStatus` — compiled binary: stdout is `resolved\n` exactly for resolved; empty for unknown | integration | `go test -race -tags=integration ./internal/cli/ -run TestStatus_StdoutOnlyStatus -count=1` | ⬜ |
| 6-3-lockshared | 2 | CORE-01 | 2 | `TestStatus_Concurrent10WithinOneSecond` — 10 parallel `status <id>` processes complete in <1 s wall-clock (LOCK_SH path) | integration | `go test -race -tags=integration ./internal/cli/ -run TestStatus_Concurrent10WithinOneSecond -count=1` | ⬜ |
| 6-4-stderr | 2 | SC #3 | 3 | `TestHelpGoesToStderrNotStdout` — `mcp-chain --help`: stdout empty, stderr contains "Usage:" | integration | `go test -race -tags=integration ./internal/cli/ -run TestHelpGoesToStderrNotStdout -count=1` | ⬜ |
| 6-4-stderr | 2 | SC #3 | 3 | `TestBadArgsGoesToStderr` — `mcp-chain status` (no id): stdout empty, stderr contains "expected" / "Usage:" | integration | `go test -race -tags=integration ./internal/cli/ -run TestBadArgsGoesToStderr -count=1` | ⬜ |
| 6-4-stderr | 2 | SC #3 | 3 | `TestUnknownCommandGoesToStderr` — `mcp-chain nosuchcommand`: stdout empty, stderr contains error text | integration | `go test -race -tags=integration ./internal/cli/ -run TestUnknownCommandGoesToStderr -count=1` | ⬜ |
| 6-4-stderr | 2 | regression | — | `TestVersion_StdoutExit0` — `mcp-chain --version`: stdout non-empty, stderr empty, exit 0 | integration | `go test -race -tags=integration ./internal/cli/ -run TestVersion_StdoutExit0 -count=1` | ⬜ |
| 6-5-stubs | 3 | cleanup | — | `stubs_test.go` row for `status` removed (no longer a stub); list/purge/resolve rows still green | unit | `go test -race ./internal/cli/ -run TestStubsExitCodes -count=1` | ⬜ |
| 6-5-stubs | 3 | smoke | — | `internal/cli/status.go` exists; `internal/cli/stubs.go` no longer declares `StatusCmd` | smoke | `test -f internal/cli/status.go && ! grep -q 'type StatusCmd' internal/cli/stubs.go` | ⬜ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/mcp-chain/main.go` — add `kong.Writers(os.Stderr, os.Stderr)` to `kong.Parse`; drop `kong.VersionFlag` and `kong.Vars{"version": ...}`; add pre-parse `os.Args[1:]` check for `--version` / `-V` → `fmt.Println(version); os.Exit(0)`
- [ ] `internal/cli/status.go` — new file: `StatusCmd` struct (moved out of stubs.go), `Run()` method that calls `runStatus(os.Stdout, os.Stderr, path, id)` and `os.Exit(code)`; `runStatus(out, errW io.Writer, path, id string) int` is the testable core
- [ ] `internal/cli/export_test.go` — new file exposing `RunStatus = runStatus` for xtests
- [ ] `internal/cli/status_test.go` — new file: 5 unit tests (`TestRunStatus_Resolved_Exit0`, `_Pending_Exit2`, `_Unknown_Exit1`, `_StdoutIsJustStatus`, `_GenericError_Exit1`)
- [ ] `internal/cli/integration_test.go` — new file with `//go:build integration`: table-driven exit codes (`TestStatus_IntegrationExitCodes`), stdout purity (`TestStatus_StdoutOnlyStatus`), 10-concurrent timing (`TestStatus_Concurrent10WithinOneSecond`), help/bad-args/unknown-command stderr assertions, `--version` regression. Uses existing `buildBinary(t)` helper from `stubs_test.go`; pre-populates state via `store.Register` + `store.Resolve` from parent test; sets `XDG_STATE_HOME=t.TempDir()` per child `cmd.Env`
- [ ] `internal/cli/stubs.go` — edit: remove `StatusCmd` struct + `Run` (moved to status.go); retain `ListCmd`, `PurgeCmd`, `ResolveCmd`
- [ ] `internal/cli/stubs_test.go` — edit: remove the `"status"` row from `TestStubsExitCodes` (status is no longer a stub)

**Framework install:** None. Zero new `go.mod` deps.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Bash `if mcp-chain status $id; then ...` works in a real shell | SC #1 | The exit-code contract semantics ("0 = success for scripting") are validated by the integration test, but the ergonomic final check belongs to Phase 8's bash monitor | Deferred to Phase 8 (bash monitor consumes exit codes) |
| Windows CI timing tolerance | SC #2 | Windows fork/exec is slower than Linux; may need threshold relaxation | Phase 9 CI task will raise threshold if Windows runners flake, with comment citing SC #2 "no serialization" rationale |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have automated verify or Wave 0 dependencies
- [x] Sampling continuity: every test maps to a commit-level verify (<5 s unit)
- [x] Wave 0 self-contained (depends only on Phase 3 statepath + Phase 4 store)
- [x] No watch-mode flags
- [x] Feedback latency < 60 s (<5 s unit; wave-end integration ~30 s)
- [x] `nyquist_compliant: true` set
- [x] All 3 SC mapped to ≥1 test (SC #1 → unit×3+integration×1, SC #2 → concurrent timing integration, SC #3 → stdout purity unit+integration×3 stderr routing)

**Approval:** approved 2026-04-24 (autonomous mode — defaults applied, both OQs resolved in favor of simpler defaults)
