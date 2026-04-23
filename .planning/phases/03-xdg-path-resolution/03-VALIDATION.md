---
phase: 3
slug: xdg-path-resolution
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-23
---

# Phase 3 — Validation Strategy

> Per-phase validation contract. Content derived from `03-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 (already in go.mod since Phase 2) |
| **Config file** | none |
| **Quick run command** | `go test ./internal/statepath/...` |
| **Full suite command** | `go test -race -count=1 ./...` |
| **Estimated runtime** | <1 s quick; ~10 s full |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/statepath/...` (<1 s)
- **After every plan wave:** `go test -race -count=1 ./...`
- **Before `/gsd-verify-work`:** Full race suite + `go vet` + `make lint` (CI-equivalent)
- **Max feedback latency:** <5 s per-commit

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 3-01-01 | 01 | 1 | CORE-06 | — | `Resolve()` returns `$XDG_STATE_HOME/mcp-chain/state.json` when set | unit | `go test ./internal/statepath/ -run TestResolve_XDGSet -v` | ❌ W0 | ⬜ pending |
| 3-01-01 | 01 | 1 | CORE-06 | — | Fallback to `$HOME/.mcp-chain/state.json` when XDG unset | unit | `go test ./internal/statepath/ -run TestResolve_HOMEFallback -v` | ❌ W0 | ⬜ pending |
| 3-01-01 | 01 | 1 | CORE-06 | — | Error when both XDG and HOME unset | unit | `go test ./internal/statepath/ -run TestResolve_NeitherSet -v` | ❌ W0 | ⬜ pending |
| 3-01-01 | 01 | 1 | CORE-06 | — | Empty-string XDG treated as unset (per spec) | unit | `go test ./internal/statepath/ -run TestResolve_EmptyXDG -v` | ❌ W0 | ⬜ pending |
| 3-01-01 | 01 | 1 | CORE-06 | — | Parent dir created with mode `0700`; pre-existing dir mode preserved | unit | `go test ./internal/statepath/ -run TestResolve_ParentAlreadyExists -v` | ❌ W0 | ⬜ pending |
| 3-01-01 | 01 | 1 | CORE-06 | — | Idempotent: second Resolve() call does not error if dir exists | unit | `go test ./internal/statepath/ -run TestResolve_Idempotent -v` | ❌ W0 | ⬜ pending |
| 3-01-01 | 01 | 1 | CORE-06 (constraint) | — | No `os/user` in dep graph (avoid NSS / cgo) | smoke | `go list -deps ./internal/statepath/ \| grep -q '^os/user$' && exit 1 \|\| exit 0` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/statepath/resolve.go` — `Resolve() (string, error)` + `MkdirAll(0o700)`
- [ ] `internal/statepath/resolve_test.go` — 6 tests above

**Framework install:** None.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows path resolution (`%LOCALAPPDATA%\mcp-chain\state.json`) | CORE-06 (cross-platform) | Deferred to Phase 9 (CI cross-compile) — Phase 3 lands linux/macOS only | Phase 9 will extend `internal/statepath` with `statepath_windows.go` build-tagged file |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have automated verify or Wave 0 dependencies
- [x] Sampling continuity: every test maps to a commit-level verify
- [x] Wave 0 self-contained
- [x] No watch-mode flags
- [x] Feedback latency < 60s (<5s actual)
- [x] `nyquist_compliant: true` set

**Approval:** approved 2026-04-23
