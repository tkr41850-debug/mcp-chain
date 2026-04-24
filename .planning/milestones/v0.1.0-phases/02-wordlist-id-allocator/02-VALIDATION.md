---
phase: 2
slug: wordlist-id-allocator
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-23
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> Content lifted from `02-RESEARCH.md` §"Validation Architecture" (lines 707–736). Phase 2 is pure Go, no filesystem or concurrency — sampling is fast and unit-test-only.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 (first import lands this phase) |
| **Config file** | none — stdlib testing needs no config |
| **Quick run command** | `go test ./internal/idgen/...` |
| **Full suite command** | `go test -race -count=1 ./...` |
| **Estimated runtime** | <1 s quick; ~10 s full (warm cache; cold go build ~5 s extra) |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/idgen/...` (<1 s, no race flag — pure in-memory)
- **After every plan wave:** `go test -race -count=1 ./...` (Phase 1 Makefile target)
- **Before `/gsd-verify-work`:** Full race suite green + `go vet ./...` + `make lint` (CI-equivalent)
- **Max feedback latency:** <5 seconds for the per-commit loop

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 1 | CORE-07 | — | Wordlist file downloaded from canonical EFF URL, sha256 pinned | smoke | `sha256sum internal/idgen/eff_short_wordlist_1.txt` matches pinned value | ❌ W0 (built by task) | ⬜ pending |
| 2-01-02 | 01 | 1 | CORE-07 | — | `init()` parse asserts 1296 unique `[a-z-]+` words | unit | `go test ./internal/idgen/ -run TestWordlistInvariants -v` | ❌ W0 (built by task) | ⬜ pending |
| 2-01-02 | 01 | 1 | CORE-07 | — | First word is `acid`, last is `zoom` | unit | `go test ./internal/idgen/ -run TestWordlistBoundaries -v` | ❌ W0 (built by task) | ⬜ pending |
| 2-01-03 | 01 | 1 | CORE-07 | — | `Allocate(0..1295)` returns `words[i]`; `Allocate(1296+)` returns deterministic `hex-%04x` IDs | unit | `go test ./internal/idgen/ -run TestAllocate -v` | ❌ W0 (built by task) | ⬜ pending |
| 2-01-03 | 01 | 1 | CORE-07 | — | No alias across the wordlist/hex handoff (monotonic uniqueness [1290, 1310]) | unit | `go test ./internal/idgen/ -run TestAllocateMonotonicUniqueOverBoundary -v` | ❌ W0 (built by task) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Phase 2 is self-contained; Wave 0 files are built in-phase by Plan 02-01:

- [ ] `internal/idgen/eff_short_wordlist_1.txt` — verbatim EFF download, sha256 pinned in the plan
- [ ] `internal/idgen/wordlist.go` — `//go:embed` + parse + invariants (CC-BY 4.0 attribution comment at top)
- [ ] `internal/idgen/idgen.go` — `Allocate(counter uint64) string` (pure function, hex fallback)
- [ ] `internal/idgen/idgen_test.go` — table-driven boundary tests + invariant reassertion
- [ ] `go.mod` / `go.sum` — first import of `github.com/stretchr/testify` v1.11.1 (auto-added by `go mod tidy`)

**Framework install:** None. `testify` auto-installs on first `go test` run via `go mod tidy`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CC-BY 4.0 attribution visible to end users | Licensing (not a REQ — project norm) | Phase 2 adds the attribution only as a code comment; Phase 10 README/NOTICE consolidates attribution. Manual check at Phase 10 delivery | Phase 10 README task confirms `NOTICE` or `THIRD_PARTY_LICENSES.md` cites EFF CC-BY 4.0 |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: every task has an inline automated verify
- [x] Wave 0 covers all MISSING references (Phase 2 self-contained)
- [x] No watch-mode flags
- [x] Feedback latency < 60s (actually < 5s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-23
