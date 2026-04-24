---
phase: 1
slug: foundation-enforcement-gates
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-23
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> Content lifted from `01-RESEARCH.md` §"Validation Architecture" (lines 813–855). Phase 1 is predominantly infrastructure — most validation is via shell-script CI gates, not Go tests. The REQ-to-test map reflects this.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` (Phase 1 has no test files yet; framework is chosen but unexercised) |
| **Config file** | none — stdlib testing needs no config |
| **Quick run command** | `make build && make size-check` |
| **Full suite command** | `make all` (lint + build + test + size + startup + stdout gates) |
| **Estimated runtime** | ~60 seconds on dev box; ~2 min cold CI, ~45 s warm CI |

---

## Sampling Rate

- **After every task commit:** `make lint && make build && make size-check` (≤ 30 s on dev box)
- **After every plan wave:** `make all` (~1 min)
- **Before `/gsd-verify-work`:** Full CI workflow green (all jobs pass)
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | CORE-01 | — | N/A (scaffold) | smoke | `go build ./... && ./mcp-chain --version \| grep -qE '^mcp-chain'` | ❌ W0 (built by task) | ⬜ pending |
| 1-01-02 | 01 | 1 | CORE-01 | — | Exit code 3 on `serve\|status\|list\|purge` stubs | unit + smoke | `go test ./internal/cli/... && ./mcp-chain serve </dev/null; test $? -eq 3` | ❌ W0 (built by task) | ⬜ pending |
| 1-01-02 | 01 | 1 | MCP-02 | — | Stdout emits zero bytes on serve stub | smoke | `./scripts/check-stdout-silence.sh ./mcp-chain` (falls back to inline check until Plan 02) | ❌ W0 (built by task) | ⬜ pending |
| 1-02-01 | 02 | 2 | QA-04 | — | `govet`, `staticcheck`, `forbidigo` run and block on non-zero exit | unit | `golangci-lint run ./...` | ❌ W0 (built by Plan 02) | ⬜ pending |
| 1-02-02 | 02 | 2 | DIST-03 | — | Stripped binary ≤ 15 MB | smoke | `./scripts/check-size.sh ./mcp-chain` | ❌ W0 (built by Plan 02) | ⬜ pending |
| 1-02-02 | 02 | 2 | DIST-03 | — | `--version` startup ≤ 100 ms (P95 of 5) | smoke | `./scripts/check-startup.sh ./mcp-chain` | ❌ W0 (built by Plan 02) | ⬜ pending |
| 1-02-02 | 02 | 2 | MCP-02 | — | `scripts/check-stdout-silence.sh` rejects any stdout bytes from `serve </dev/null` | smoke | `./scripts/check-stdout-silence.sh ./mcp-chain` | ❌ W0 (built by Plan 02) | ⬜ pending |
| 1-03-01 | 03 | 3 | DIST-03 | — | CI workflow wires every gate (lint, build, test, size, startup, stdout, net/http ban) | integration | `.github/workflows/ci.yml` exists and references each gate script | ❌ W0 (built by Plan 03) | ⬜ pending |
| 1-03-02 | 03 | 3 | QA-04 | — | `main_test.go` smoke asserts `--version` exit 0 and stdout format | unit | `go test ./cmd/mcp-chain/...` | ❌ W0 (built by Plan 03) | ⬜ pending |
| 1-03-01 | 03 | 3 | MCP-01 (prep) | — | `net/http` not in dep graph (pre-emptive, formal gate lands in Phase 5) | smoke | `go list -f '{{ join .Deps "\n" }}' ./... \| grep -q '^net/http$' && exit 1` (inline in ci.yml) | ❌ W0 (built by Plan 03) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Phase 1 is itself the Wave 0 for the project — all validation infrastructure is built in this phase. Specifically:

- [ ] `cmd/mcp-chain/main.go` — built by Plan 01-01 (provides the binary under test)
- [ ] `internal/cli/stubs.go` + `internal/cli/stubs_test.go` — built by Plan 01-01
- [ ] `.golangci.yml` — built by Plan 01-02 (enables `govet`/`staticcheck`/`forbidigo` checks)
- [ ] `scripts/check-size.sh` — built by Plan 01-02 (DIST-03 size gate)
- [ ] `scripts/check-startup.sh` — built by Plan 01-02 (DIST-03 startup gate)
- [ ] `scripts/check-stdout-silence.sh` — built by Plan 01-02 (MCP-02 stdout gate)
- [ ] `Makefile` — built by Plan 01-02 (local developer orchestration)
- [ ] `.github/workflows/ci.yml` — built by Plan 01-03 (CI orchestration)
- [ ] `cmd/mcp-chain/main_test.go` — built by Plan 01-03 (smoke test)

**Framework install:** None. `go test` is built-in. Phase 1 has no `*_test.go` files until Plan 01-01 Task 2 lands `stubs_test.go` and Plan 01-03 Task 2 lands `main_test.go`. Before that, `go test ./...` is a trivial no-op that succeeds — perfectly valid CI plumbing for Phase 2 onward.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| First CI run on real GitHub Actions runner validates `$EPOCHREALTIME` is available in `check-startup.sh` | DIST-03 | Research assumption A3 — verified on bash 5.x / Ubuntu 22/24 via docs but not executed yet | After first push to the repo, check the Actions log for `scripts/check-startup.sh` success and note the reported `--version` duration |
| Module path confirmation against real GitHub owner | CORE-01 (skeleton) | Research assumption A1 — code uses placeholder `github.com/tkr41850-debug/mcp-chain` if no remote is set | Phase 10 README task re-verifies the module path matches the real repo URL before first release |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: every task has an inline automated verify (Plans 01-01, 01-02, 01-03 each verify their own outputs)
- [x] Wave 0 covers all MISSING references (Phase 1 itself is Wave 0 — everything built in-phase)
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-23
