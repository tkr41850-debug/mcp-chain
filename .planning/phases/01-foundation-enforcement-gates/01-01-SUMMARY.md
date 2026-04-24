---
phase: 01-foundation-enforcement-gates
plan: "01"
subsystem: cli-scaffold
tags:
  - go
  - kong
  - cli-scaffold
  - stdout-discipline
  - mcp-02
dependency_graph:
  requires: []
  provides:
    - go.mod (module github.com/tkr41850-debug/mcp-chain, kong v1.15.0)
    - cmd/mcp-chain/main.go (binary entrypoint, stdout discipline, kong dispatch)
    - internal/cli/stubs.go (ServeCmd/StatusCmd/ListCmd/PurgeCmd types, ExitCodeNotImplemented=3)
    - internal/cli/stubs_test.go (subprocess exit code + stdout silence tests)
    - .gitignore (binary artifacts excluded)
  affects: []
tech_stack:
  added:
    - "github.com/alecthomas/kong v1.15.0 (CLI dispatch)"
    - "Go 1.23.8 (module floor)"
  patterns:
    - "stdout discipline: log.SetOutput(os.Stderr) + slog.SetDefault in main() first lines"
    - "kong struct-tag subcommand dispatch"
    - "TDD: RED (stubs_test.go) -> GREEN (stubs.go + main.go)"
key_files:
  created:
    - go.mod
    - go.sum
    - .gitignore
    - cmd/mcp-chain/main.go
    - internal/cli/stubs.go
    - internal/cli/stubs_test.go
  modified: []
decisions:
  - "Module path github.com/tkr41850-debug/mcp-chain (placeholder - no git remote configured; Phase 10 Docs will verify against real repo)"
  - "Go installed via manual tarball download (go1.23.8.linux-amd64) - apk required doas/tty not available in agent context"
  - "kong v1.15.0 resolved correctly; go.sum has 8 lines (4 deps x h1+go.mod entries)"
  - "stubs use fmt.Fprintln(os.Stderr) not slog — intentional per RESEARCH.md: one-line operator messages, not structured logging"
metrics:
  duration: "~15 minutes"
  completed: "2026-04-23"
  tasks_completed: 2
  files_created: 6
base_commit: a04a53ec69f7333d2507cf959a06230f781a751a
head_commit: 3d69d4a0b00910aef1d32d1fa2478f8a8c2850b8
---

# Phase 01 Plan 01: Go Module Init + CLI Skeleton Summary

Go module initialized and CLI skeleton scaffolded with strict stdout discipline (MCP-02) — kong v1.15.0 with four subcommand stubs exiting code 3, all log output wired to stderr before any third-party code runs.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Initialize Go module and add kong dependency | f6fc3c7 | go.mod, go.sum, .gitignore |
| 2 | Write cmd/mcp-chain/main.go and internal/cli/stubs.go | 3d69d4a | cmd/mcp-chain/main.go, internal/cli/stubs.go, internal/cli/stubs_test.go |

## Module Path

**Chosen:** `github.com/tkr41850-debug/mcp-chain` (placeholder)

**Disposition:** No git remote configured (`git remote get-url origin` returned "no-remote"). Using the placeholder documented in RESEARCH.md. Phase 10 (Docs) will verify against the actual GitHub repo once created.

## Kong Version

**Resolved by go mod tidy:** `github.com/alecthomas/kong v1.15.0` (pinned; no drift from plan)

**Transitive deps in go.sum:** 8 lines (4 packages: alecthomas/assert, alecthomas/kong, alecthomas/repr, hexops/gotextdiff — all test-only transitive deps of kong itself)

## Dep Count

- `go list -deps ./...` total: 82 packages (all stdlib + kong)
- `net/http` in dep graph: NO (confirmed)

## Verify Results

| Command | Result |
|---------|--------|
| `go mod tidy && grep -q "^go 1.23" go.mod` | PASS |
| `grep -q "github.com/alecthomas/kong v1.15" go.mod` | PASS |
| `test -f go.sum && test -f .gitignore` | PASS |
| `go build ./...` | PASS |
| `grep -q 'log.SetOutput(os.Stderr)' cmd/mcp-chain/main.go` | PASS |
| `grep -q 'slog.SetDefault' cmd/mcp-chain/main.go` | PASS |
| `grep -q 'ExitCodeNotImplemented = 3' internal/cli/stubs.go` | PASS |
| `/tmp/mcp-chain-test --version \| grep -qE '^mcp-chain'` | PASS (outputs "mcp-chain dev") |
| `/tmp/mcp-chain-test serve </dev/null; exit code = 3` | PASS |
| `wc -c </tmp/stdout.check` = 0 (MCP-02 stdout silence) | PASS |
| `go test ./internal/cli/...` (5 subtests) | PASS |
| `! (go list -deps ./... \| grep -qx 'net/http')` | PASS |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Go not installed on system**
- **Found during:** Task 1 (first go command)
- **Issue:** `go` binary not found on PATH; `apk add go` requires doas/tty not available in agent context
- **Fix:** Downloaded `go1.23.8.linux-amd64.tar.gz` from go.dev and extracted to `/home/alpine/go-sdk/`; used `export PATH="/home/alpine/go-sdk/go/bin:$PATH"` for all go commands
- **Impact:** None on output — Go 1.23.8 satisfies the `go 1.23` floor requirement; installation is temporary to worktree session

**2. [Rule 1 - Process] go mod tidy cleared kong from go.mod before source files existed**
- **Found during:** Task 1
- **Issue:** `go mod tidy` removed kong require stanza because no source files imported it yet; go.sum was empty
- **Fix:** Proceeded to create source files (Task 2) before final tidy; after writing main.go and stubs.go, `go mod tidy` correctly resolved and retained kong v1.15.0 with go.sum populated
- **Impact:** None — tasks committed in correct order with valid go.mod/go.sum

## Known Stubs

All four subcommands are intentional stubs per plan design:

| Stub | File | Line | Planned Resolution |
|------|------|------|--------------------|
| `ServeCmd.Run()` | internal/cli/stubs.go | ~23 | Phase 5 (MCP stdio server) |
| `StatusCmd.Run()` | internal/cli/stubs.go | ~32 | Phase 6 (status exit codes) |
| `ListCmd.Run()` | internal/cli/stubs.go | ~41 | Phase 7 (list output) |
| `PurgeCmd.Run()` | internal/cli/stubs.go | ~50 | Phase 7 (purge logic) |

These stubs are the plan's goal — they establish the exit code convention (3 = not implemented) and the stdout discipline invariant before real logic lands in Phases 5–7.

## Threat Surface Scan

No new security-relevant surface beyond what the plan's threat model covers:
- T-01-01 mitigated: log.SetOutput(os.Stderr) + slog.SetDefault at top of main()
- T-01-03 mitigated: PurgeCmd uses `xor:"target"` tag

No new network endpoints, auth paths, file access, or schema changes introduced.

## Self-Check: PASSED

- go.mod exists: FOUND
- go.sum exists: FOUND (8 lines)
- .gitignore exists: FOUND
- cmd/mcp-chain/main.go exists: FOUND
- internal/cli/stubs.go exists: FOUND
- internal/cli/stubs_test.go exists: FOUND
- Commit f6fc3c7 exists: FOUND
- Commit 3d69d4a exists: FOUND
- All 12 verify checks: PASS
