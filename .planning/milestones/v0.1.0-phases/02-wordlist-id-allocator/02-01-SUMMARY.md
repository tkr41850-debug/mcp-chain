---
phase: "02"
plan: "01"
subsystem: internal/idgen
tags: [wordlist, idgen, embed, pure-function, core-07]
dependency_graph:
  requires: []
  provides: [internal/idgen.Allocate]
  affects: [phase-04-store]
tech_stack:
  added: [github.com/stretchr/testify v1.11.1]
  patterns: [go-embed, init-panic-invariants, table-driven-tests]
key_files:
  created:
    - internal/idgen/eff_short_wordlist_1.txt
    - internal/idgen/wordlist.go
    - internal/idgen/idgen.go
    - internal/idgen/idgen_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - "Used corrected boundary test counters (66829/66830) instead of RESEARCH.md's 65534/65535 which had an arithmetic bug (65534-1295=64239=0xfaef, not 0xfffe)"
  - "CC-BY 4.0 attribution placed as comment in wordlist.go header per research recommendation; NOTICE file deferred to Phase 10"
  - "wordlistSize const kept unexported per YAGNI guidance in RESEARCH.md"
metrics:
  duration: "~5 minutes"
  completed: "2026-04-23"
  tasks_completed: 3
  files_created: 4
  files_modified: 2
---

# Phase 02 Plan 01: Wordlist ID Allocator Summary

**One-liner:** Pure `Allocate(counter uint64) string` over the EFF short wordlist (1296 words, CC-BY 4.0, sha256-pinned) with zero-padded hex fallback starting at `hex-0001`.

## Tasks Completed

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Download + pin + commit EFF short wordlist (1296 lines, sha256 verified) | f778131 |
| 2 | `wordlist.go` with `//go:embed`, `parseAndValidate`, invariant tests | d719461 |
| 3 | `idgen.go` with `Allocate`, boundary tests, monotonicity test | 165d42c |

## Commits

- `f778131` feat(02-01): task 1: embed EFF short wordlist (1296 words, CC-BY 4.0)
- `d719461` feat(02-01): task 2: embed wordlist + invariant tests
- `165d42c` feat(02-01): task 3: Allocate + boundary + monotonicity tests

## Verification Results

| Command | Result |
|---------|--------|
| `sha256sum internal/idgen/eff_short_wordlist_1.txt` | PASS — `8f5ca830b8bffb6fe39c9736c024a00a6a6411adb3f83a9be8bfeeb6e067ae69` |
| `wc -l internal/idgen/eff_short_wordlist_1.txt` | PASS — 1296 |
| `go test ./internal/idgen/ -run TestWordlistInvariants -v` | PASS |
| `go test ./internal/idgen/ -run TestWordlistBoundaries -v` | PASS |
| `go test -race -count=1 ./internal/idgen/...` | PASS — all 4 tests |
| `go test -race -count=1 ./...` | PASS — all packages |
| `go vet ./...` | PASS — clean |
| `go build ./...` | PASS — no errors |
| `scripts/check-size.sh` (binary) | PASS — 4.92 MB / 15 MB limit |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected boundary test counter values in TestAllocate**
- **Found during:** Task 3
- **Issue:** RESEARCH.md lines 584-585 listed `counter=65534 → "hex-fffe"` and `counter=65535 → "hex-ffff"`, but with `Allocate` formula `counter - (len(words)-1)` = `counter - 1295`, counter 65534 yields `65534-1295=64239=0xfaef`, not `0xfffe`. The execution instructions explicitly noted these were old values with an arithmetic bug and the correct counters are 66829/66830.
- **Fix:** Used `66829 → "hex-fffe"` and `66830 → "hex-ffff"` (plus 66831/66832 from plan). All test cases verified by math: `66829-1295=65534=0xfffe`, `66830-1295=65535=0xffff`, `66831-1295=65536=0x10000`, `66832-1295=65537=0x10001`.
- **Files modified:** `internal/idgen/idgen_test.go`
- **Commit:** 165d42c

## Known Stubs

None.

## Threat Flags

None. `internal/idgen` is a pure function over embedded static data. No network, filesystem, process, auth, or cryptographic surface.

## Self-Check: PASSED

All 4 created files confirmed present. All 3 task commits confirmed in git log.
