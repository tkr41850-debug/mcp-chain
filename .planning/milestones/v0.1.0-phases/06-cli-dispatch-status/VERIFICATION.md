---
phase: 06-cli-dispatch-status
verified: 2026-04-24T04:45:00Z
status: passed
score: 3/3 success criteria verified
overrides_applied: 0
---

# Phase 6: CLI Dispatch & Status Verification Report

**Phase Goal:** Wire `mcp-chain status <id>` through kong with locked exit-code contract (resolved=0, pending=2, unknown=1, other=1) and stdout/stderr discipline.
**Verified:** 2026-04-24
**Status:** PASSED

## Source-Level Contract (goal-backward)

| Assertion | Evidence | Status |
|-----------|----------|--------|
| `kong.Writers(os.Stderr, os.Stderr)` present | main.go:63 | PASS |
| `kong.VersionFlag` removed | 0 matches in main.go | PASS |
| Manual `--version` pre-parse → `os.Stdout` + `os.Exit(0)` | main.go:49-54 | PASS |
| `runStatus(out, errW io.Writer, path, id string) int` | status.go:57 | PASS |
| Decision tree covers resolved/pending/unknown/other | status.go:64-77, all 4 arms present | PASS |
| `os.Exit(code)` called from `Run`, not error return | status.go:42,46 | PASS |
| `StatusCmd` removed from stubs.go | 0 matches | PASS |
| `StatusCmd` single declaration in status.go | 1 match | PASS |

## Success Criteria

### SC #1 — Exit codes 0/2/1 (resolved/pending/unknown)
**PASS.** Integration test `TestStatus_IntegrationExitCodes` (table-driven, 3 sub-cases) execs compiled binary and asserts `exitErr.ExitCode()` + stdout + stderr for each row.

```
--- PASS: TestStatus_IntegrationExitCodes (7.73s)
    --- PASS: TestStatus_IntegrationExitCodes/resolved (0.41s)
    --- PASS: TestStatus_IntegrationExitCodes/pending (0.06s)
    --- PASS: TestStatus_IntegrationExitCodes/unknown (0.05s)
```

Test asserts stdout=`"resolved\n"`/empty and stderr substring `"unknown id"` on the unknown row (integration_test.go:82-107).

### SC #2 — 10 concurrent status < 1s (LOCK_SH non-serializing)
**PASS.** `TestStatus_Concurrent10WithinOneSecond` launches 10 processes via parallel `cmd.Start`, waits all, asserts elapsed < 1s.

```
integration_test.go:199: 10 concurrent status probes: elapsed=591.070261ms
--- PASS: TestStatus_Concurrent10WithinOneSecond (8.41s)
```

Observed 591ms (also 715ms on a second run) — comfortably under the 1s threshold; proves `store.Get` shared lock does not serialize readers.

### SC #3 — stdout/stderr discipline
**PASS.** Manual binary invocation on fresh build `/tmp/mcp6`:

| Command | stdout bytes | stderr bytes | exit |
|---------|-------------:|-------------:|-----:|
| `--help` | 0 | 468 | 0 |
| `nosuchcmd` | 0 | 517 | 80 |
| `status` (missing arg) | 0 | 216 | 80 |
| `--version` | 14 (`mcp-chain dev\n`) | 0 | 0 |

Additionally covered by integration tests `TestHelpGoesToStderrNotStdout`, `TestBadArgsGoesToStderr`, `TestUnknownCommandGoesToStderr`, `TestVersion_StdoutExit0`, `TestStatus_StdoutOnlyStatus` — all PASS.

## Test Suite Gate

```
go test -race -count=1 -timeout 60s ./...
  cmd/mcp-chain       ok  49.380s
  internal/cli        ok  48.175s
  internal/idgen      ok   3.574s
  internal/mcpserver  ok   6.590s
  internal/statepath  ok   2.685s
  internal/store      ok   9.104s

go test -race -count=1 -tags=integration -timeout 180s ./internal/cli/...
  internal/cli        ok  87.096s  (all 14 integration tests PASS)

go vet ./...   → clean
```

## Binary Size Constraint

Stripped `go build -ldflags="-s -w"` → **7,794,980 bytes (7.4 MB)** — well under 15 MB constraint.

## Gaps & Deferred

None. `list`/`purge`/`resolve` CLI stubs remain for Phase 7 per plan.

---

_Verified by Claude (gsd-verifier). Relevant files:_
- `/home/alpine/mcp-chain/cmd/mcp-chain/main.go`
- `/home/alpine/mcp-chain/internal/cli/status.go`
- `/home/alpine/mcp-chain/internal/cli/stubs.go`
- `/home/alpine/mcp-chain/internal/cli/integration_test.go`
- `/home/alpine/mcp-chain/internal/cli/status_test.go`
- `/home/alpine/mcp-chain/internal/cli/export_test.go`
