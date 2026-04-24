---
phase: 02-wordlist-id-allocator
verified: 2026-04-23T18:25:42Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
sc_results:
  sc_1: PASS  # EFF short wordlist embedded with count+uniqueness test
  sc_2: PASS  # Allocate(0..1295) deterministic words; Allocate(1296+) deterministic hex
  sc_3: PASS  # Table-driven boundary test, no filesystem/concurrency deps
req_results:
  CORE-07: PASS
verdict: PHASE_COMPLETE
---

# Phase 2: Wordlist & ID Allocator — Verification Report

**Phase Goal:** A pure, deterministic `idgen.Allocate(counter uint64) string` is available over the embedded EFF short wordlist with a clean hex fallback past 1296 — fully tested in isolation, zero dependency on store or filesystem.

**Verified:** 2026-04-23T18:25:42Z
**Status:** passed
**Re-verification:** No — initial verification
**Verdict:** PHASE_COMPLETE

---

## Goal Achievement

### Observable Truths (PLAN must_haves)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `go test ./internal/idgen/...` produces 4 passing tests | PASS | `go test -race -count=1 ./internal/idgen/... -v` shows TestWordlistInvariants, TestWordlistBoundaries, TestAllocate (9 subtests), TestAllocateMonotonicUniqueOverBoundary all PASS; total 2.038s |
| 2 | `Allocate(0)`→`"acid"`, `Allocate(1295)`→`"zoom"`, `Allocate(1296)`→`"hex-0001"` deterministic | PASS | TestAllocate cases `first_word`, `last_word`, `first_fallback_(hex-0001)` all PASS; code in `idgen.go:16-23` is pure (no rand/mutex) |
| 3 | `Allocate(66831)`→`"hex-10000"` (5-digit widen, no collision) | PASS | TestAllocate case `widen_to_5_digits` PASS; TestAllocateMonotonicUniqueOverBoundary verifies no duplicates in [1290,1310]; `%04x` is min-width per `fmt` docs |
| 4 | `eff_short_wordlist_1.txt` has pinned sha256 | PASS | `sha256sum` returns `8f5ca830b8bffb6fe39c9736c024a00a6a6411adb3f83a9be8bfeeb6e067ae69` — exact match to pin |
| 5 | `init()` panics on wordlist invariant violations | PASS | `wordlist.go:31-33` calls `parseAndValidate`; `parseAndValidate` panics on count≠1296, missing tab, empty word, invalid charset, hex- prefix, or duplicate (lines 46-69) |

**Score:** 5/5 truths verified

### ROADMAP Success Criteria

| # | Success Criterion | Status | Evidence |
|---|------------------|--------|----------|
| 1 | EFF short wordlist (1296 unique lowercase words) embedded via `//go:embed` with startup-time test asserting count and uniqueness | **PASS** | `wordlist.go:24` has `//go:embed eff_short_wordlist_1.txt`; `wc -l` = 1296; TestWordlistInvariants asserts `len(words)==1296` and uniqueness via `seen map`; `init()` also enforces both and panics otherwise |
| 2 | `Allocate(0..1295)` returns `words[i]` deterministically; `Allocate(1296+)` returns deterministic hex-suffix ID | **PASS** | `idgen.go:17-22` — pure function: `if counter < uint64(len(words)) { return words[counter] }` else `fmt.Sprintf("hex-%04x", counter-uint64(len(words)-1))`. 9 TestAllocate subtests pass including 0→acid, 1295→zoom, 1296→hex-0001, 1297→hex-0002 |
| 3 | Table-driven unit test covers boundary indices (0, 1, 1295, 1296, large values) with no filesystem/concurrency dependency | **PASS** | `idgen_test.go:40-62` — table-driven `TestAllocate` with counters {0, 1, 1295, 1296, 1297, 66829, 66830, 66831, 66832}; no `t.TempDir()`, no goroutines, no `os` calls. Test imports: only `strings`, `testing`, `testify/require` |

**SC score:** 3/3

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/idgen/eff_short_wordlist_1.txt` | 1296 lines, sha256 pinned | PASS | 1296 lines; sha256 `8f5ca830…67ae69` matches pin exactly; first line `1111\tacid`, last `6666\tzoom`, line 1286 `6652\tyo-yo` (hyphen entry preserved) |
| `internal/idgen/wordlist.go` | `//go:embed` + `parseAndValidate` + `init()` panic + CC-BY 4.0 attribution; ≥60 lines | PASS | 89 lines; CC-BY 4.0 + upstream URL in package doc (lines 1-11); `//go:embed eff_short_wordlist_1.txt` on line 24 directly above `var rawWordlist string`; `parseAndValidate` enforces count/charset/dedup/hex- prefix (lines 43-73); `isValidWord` allows `[a-z-]+` including the `yo-yo` hyphen |
| `internal/idgen/idgen.go` | `Allocate(counter uint64) string`; ≥15 lines; only stdlib `fmt` | PASS | 23 lines; exports `Allocate`; godoc notes `Requirement: CORE-07`; imports only `fmt`; pure & concurrent-safe |
| `internal/idgen/idgen_test.go` | 4 tests including `TestAllocate`; ≥60 lines | PASS | 76 lines; contains TestWordlistInvariants, TestWordlistBoundaries, TestAllocate (9 subtests), TestAllocateMonotonicUniqueOverBoundary |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `wordlist.go` | `eff_short_wordlist_1.txt` | `//go:embed eff_short_wordlist_1.txt` directive | WIRED | Line 24 has directive directly above `var rawWordlist string` on line 25 (no blank line). Build succeeds; test binary `strings` shows `acid`/`zoom`/`yo-yo` embedded (3/1/3 occurrences in `/tmp/idgen-test`) |
| `wordlist.go init()` | `parseAndValidate` (enforces invariants) | `words = parseAndValidate(rawWordlist)` | WIRED | Line 32: `words = parseAndValidate(rawWordlist)`; parseAndValidate panics on violations (lines 46-69) |
| `idgen.go Allocate` | `wordlist.go words` slice | `words[counter]` index lookup | WIRED | Line 18: `return words[counter]` (same package, no import needed) |
| `idgen_test.go` | `testify/require` v1.11.1 | Import + use in assertions | WIRED | Import on line 7; `go.mod` line 7 declares `github.com/stretchr/testify v1.11.1` |

### Data-Flow Trace (Level 4)

N/A — `idgen` is a pure function package with no data-fetching, no dynamic rendering. The embedded wordlist is static at compile time; `Allocate` is pure (`uint64`→`string`, no side effects). No hollow-prop risk.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All 4 tests pass with race detector | `go test -race -count=1 ./internal/idgen/... -v` | TestWordlistInvariants PASS, TestWordlistBoundaries PASS, TestAllocate (9 subtests) PASS, TestAllocateMonotonicUniqueOverBoundary PASS — `ok 2.038s` | PASS |
| Full repo suite green under race | `go test -race -count=1 ./...` | 3 packages OK: cmd/mcp-chain, internal/cli, internal/idgen | PASS |
| Package imports only stdlib | `go list -f '{{.Imports}}' ./internal/idgen/` | `[embed fmt strings]` — all stdlib | PASS |
| Test imports are stdlib + testify only | `go list -f '{{.TestImports}}'` | `[github.com/stretchr/testify/require strings testing]` | PASS |
| `go vet` clean | `go vet ./...` | exit 0, no output | PASS |
| `go build` clean | `go build ./...` | exit 0 | PASS |
| Embed really works (bytes in binary) | `go test -c -o /tmp/idgen-test ./internal/idgen && strings /tmp/idgen-test \| grep -c acid` | `3` (acid appears); `zoom` = 3, `yo-yo` = 1 | PASS |
| Wordlist sha256 matches pin | `sha256sum internal/idgen/eff_short_wordlist_1.txt` | `8f5ca830b8bffb6fe39c9736c024a00a6a6411adb3f83a9be8bfeeb6e067ae69` — exact pin match | PASS |
| Line count = 1296 | `wc -l` | 1296 | PASS |
| Binary size gate preserved | `go build -ldflags="-s -w" ./cmd/mcp-chain && scripts/check-size.sh` | 3.31 MB / 15 MB — within budget | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CORE-07 | 02-01-PLAN.md | EFF short wordlist embedded via `go:embed`; monotonic `counter` selects next word; hex fallback once exhausted; counter never decremented on purge | **PASS (for idgen's share)** | `wordlist.go:24` embeds wordlist; `idgen.go:16-23` provides `Allocate` with hex fallback; all boundary tests green. Note: "counter never decremented on purge" is store-layer behavior (Phase 4) per plan's REQUIREMENTS traceability; `idgen` does not own counter state by design (Allocate is pure). That portion is explicitly deferred to Phase 4 and is NOT an idgen-layer gap. |

**Orphaned requirements check:** REQUIREMENTS.md traceability maps CORE-07 to Phase 2 — plan's `requirements: [CORE-07]` matches. No orphans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | None | — | grep for TODO/FIXME/XXX/HACK/PLACEHOLDER returned 0 hits across `internal/idgen/*.go`. No stub returns, no empty handlers, no `return null`, no hardcoded test data leaking into prod code. |

### Human Verification Required

None. This phase is a pure-function library with deterministic boundary tests; nothing here needs human eyes. All observable behavior is tested programmatically.

### Deferred Items

None. All phase-2 scope is achieved. The plan explicitly documents that:

- "Counter never decremented on purge" (part of CORE-07 wording) belongs to Phase 4 (store) — this is already the REQUIREMENTS.md intent and is not an idgen gap.
- Top-level `NOTICE` / `THIRD_PARTY_LICENSES.md` for EFF CC-BY 4.0 is deferred to Phase 10 per Open Question 1 resolution (attribution currently lives in the `wordlist.go` package doc comment — minimal-compliant).
- First downstream consumer of `idgen.Allocate` is Phase 4 (store); no non-test importer exists today, which matches the plan's "no upstream callers in Phase 2" note.

### Gaps Summary

None. Phase 2's deliverable — a pure `idgen.Allocate(counter uint64) string` with embedded EFF wordlist, hex fallback, and full boundary coverage — is present, correct, and test-pinned. sha256 on the wordlist matches the documented pin exactly. Test binary verifiably embeds the words (`strings` hit count ≥1 for `acid`/`zoom`/`yo-yo`). The package is concurrent-safe-by-default (immutable after init) and has zero runtime dependencies. `go test -race` passes across the full repo.

---

## One-line verdict

Phase 2 verification: PHASE_COMPLETE. SC: 3/3. REQ: 1/1.

---

_Verified: 2026-04-23T18:25:42Z_
_Verifier: Claude (gsd-verifier)_
