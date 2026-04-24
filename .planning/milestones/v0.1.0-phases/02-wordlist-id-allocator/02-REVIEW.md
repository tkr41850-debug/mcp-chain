---
phase: 02-wordlist-id-allocator
reviewed: 2026-04-23T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - internal/idgen/wordlist.go
  - internal/idgen/idgen.go
  - internal/idgen/idgen_test.go
  - internal/idgen/eff_short_wordlist_1.txt
  - go.mod
findings:
  blocker: 0
  major: 0
  minor: 2
  info: 3
  total: 5
status: clean
---

# Phase 2: Code Review Report

**Reviewed:** 2026-04-23
**Depth:** standard
**Files Reviewed:** 5
**Status:** clean (no blockers or majors; two minors + three info items, none of which gate release)

## Summary

Phase 2 delivers a pure, deterministic ID allocator over an embedded EFF short wordlist. The code is small (~90 LOC of Go + ~60 LOC of test), tight, and aligns with CORE-07 and the Phase 2 PLAN end-to-end. All boundary arithmetic was verified by hand:

- `counter=0` -> `words[0]` = `"acid"` (EFF line 1, roll 1111)
- `counter=1295` -> `words[1295]` = `"zoom"` (EFF line 1296, roll 6666)
- `counter=1296` -> `fmt.Sprintf("hex-%04x", 1296 - 1295)` = `"hex-0001"` (offset `len(words)-1` is correct; `1295` not `1296`)
- `counter=66830` -> offset `65535` = `0xffff` -> `"hex-ffff"`
- `counter=66831` -> offset `65536` = `0x10000` -> `"hex-10000"` (`%04x` is min-width; widens as documented)

The wordlist file has been verified at sha256 `8f5ca830...067ae69` matching the pin, 1296 LF-terminated lines, 13,660 bytes, no duplicates, no non-ASCII, and the single legal hyphenated entry (`yo-yo` on line 1286) is admitted by `isValidWord`. No source file has a leading or trailing hyphen, so `[a-z-]+` as a regex spec is accurate for the data.

Invariant enforcement is properly two-layered: `init()` panics (fail-build) and `TestWordlistInvariants` re-asserts at test time (fail-CI). Purity of `Allocate` is confirmed — only `fmt` is imported, no mutable package state after `init`, no error path.

No blockers, no majors. Two minors and three infos are optional follow-ups; none block moving to Phase 3.

## Blockers

None.

## Majors

None.

## Minors

### MN-01: Panic messages for invariant violations include line content that may be logged downstream

**File:** `internal/idgen/wordlist.go:47, 55, 58, 61, 64, 67`
**Issue:** Panic strings interpolate the offending line or word (`%q line`). In the current design this is desirable — it makes a corrupt wordlist trivially debuggable from `go test` output. However, if the wordlist were ever loaded from an untrusted source (not the case today — file is sha256-pinned at build time), the panic messages would become an information-disclosure channel by echoing caller-supplied bytes to stderr/logs. Worth documenting that these messages are intentional and only safe because the embedded file is trusted.
**Fix:** Add a one-line comment above `parseAndValidate` noting "messages include raw line content intentionally — embedded file is sha256-pinned and trusted." No code change.

### MN-02: `tc := tc` loop-copy in test is a no-op under Go 1.22+

**File:** `internal/idgen/idgen_test.go:57`
**Issue:** `go.mod` declares `go 1.23.8`. Since Go 1.22 the loop variable is per-iteration by default, so the `tc := tc` shadow is redundant. Harmless but noise; may be flagged in future by a "redundant assignment" linter rule.
**Fix:** Remove line 57 (`tc := tc`). Purely cosmetic; does not affect correctness or lint today.

```go
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        require.Equal(t, tc.want, Allocate(tc.counter))
    })
}
```

## Info

### IN-01: `NOTICE` file referenced in package doc comment does not yet exist

**File:** `internal/idgen/wordlist.go:10`
**Issue:** The doc comment says "See NOTICE for details." but no `NOTICE` (or `THIRD_PARTY_LICENSES.md`) exists at the repo root today. This is explicitly a Phase 10 follow-up per `02-01-PLAN.md` line 311 and `02-01-SUMMARY.md`, so it is tracked — but a reader of `wordlist.go` alone would hit a dead reference. Not a blocker: the CC-BY 4.0 attribution text plus the upstream URL in the package doc itself is a compliant placement.
**Fix:** No action in Phase 2. Ensure Phase 10 backlog explicitly creates `NOTICE` at repo root citing the EFF wordlist under CC-BY 4.0 with the upstream URL and sha256 pin. (Already flagged in `02-01-PLAN.md` handoff notes.)

### IN-02: `TestAllocateMonotonicUniqueOverBoundary` asserts uniqueness but not monotonicity

**File:** `internal/idgen/idgen_test.go:67-76`
**Issue:** Test name says "Monotonic" but only checks uniqueness. Since Allocate switches from lexicographic words to `hex-...` midway through `[1290, 1310]`, the sequence is intentionally not lexicographically monotonic across the boundary, so a strict monotonicity check would be wrong. The uniqueness assertion is the right check here, but the name misleads slightly. Not a bug — the CORE-07 requirement is uniqueness (no two counters produce the same ID), which this pins correctly.
**Fix:** Optional rename to `TestAllocateUniqueOverWordlistHexBoundary` to remove the "monotonic" claim. Purely a readability nudge; do not block.

### IN-03: `TestAllocate` counter `66829` is present but not called out as the `hex-fffe` boundary-minus-one sentinel in the PLAN

**File:** `internal/idgen/idgen_test.go:51`
**Issue:** The plan lists 8 subtests; the test has 9 (`66829 -> hex-fffe`). This is actually stronger coverage than required — it pins `hex-fffe`, `hex-ffff`, `hex-10000` as three consecutive results, which is the tightest possible pin on the `%04x` widen behavior. Worth keeping; flagging only because the success criteria in the plan say "9 subtests covering counters 0, 1, 1295, 1296, 1297, 66829, 66830, 66831, 66832" — confirming they match.
**Fix:** None. Coverage exceeds plan.

---

## Review Notes by Focus Area

1. **Correctness**: PASS. Verified all documented boundary IDs by arithmetic. The `counter - uint64(len(words)-1)` subtraction (`idgen.go:22`) is the non-obvious right answer — using `len(words)` would produce `hex-0000` at `counter=1296`, an off-by-one the test `{"first fallback (hex-0001)", 1296, "hex-0001"}` would catch. The `%04x` min-width widen-past-4-digits is pinned by the 66830/66831/66832 trio.

2. **Embed safety**: PASS. `//go:embed eff_short_wordlist_1.txt` filename is exact match (case-sensitive), no blank line between the directive and `var rawWordlist string`. `strings.TrimRight(raw, "\n")` handles the EFF file's single trailing LF (verified by `tail | cat -A` — file ends with a single `zoom\n`). It also tolerates either no trailing newline or multiple trailing newlines, which is defensive without being permissive about interior structure.

3. **Invariant enforcement**: PASS. All six invariants (count, tab separator, non-empty, `[a-z-]+` charset, no `hex-` prefix, no duplicates) panic at `init()` with line-number-bearing messages. `TestWordlistInvariants` re-asserts count, non-emptiness, charset, no-hex-prefix, and uniqueness at test time — so even if Go ever relaxed `init()` panic visibility, CI would still catch wordlist corruption. The implicit panic channel (test binary fails to load) is the strongest fail-fast channel.

4. **Test completeness**: PASS. The widen boundary is explicitly pinned:
   - `66829 -> "hex-fffe"` (max-4-digit minus one)
   - `66830 -> "hex-ffff"` (max 4-digit)
   - `66831 -> "hex-10000"` (widen to 5 digits — this is the key assertion; a broken `%x` or a truncation would fail here)
   - `66832 -> "hex-10001"` (widen plus one, confirms stability)

   The monotonic-unique test over `[1290, 1310]` also catches any off-by-one that would cause `words[1295]` to alias with `hex-0000`.

5. **Purity**: PASS. `idgen.go` imports only `"fmt"`. `wordlist.go` imports `_ "embed"`, `"fmt"`, `"strings"`. No `math/rand`, no `crypto/rand`, no `sync`, no `time`, no `os`, no `io`. `Allocate` reads only `words` (immutable after init) — safe for concurrent read. No error return, no allocation on the word-path, one `fmt.Sprintf` allocation on the hex path.

6. **Licensing**: PASS with IN-01 tracked. Package doc at `wordlist.go:1-11` contains the CC-BY 4.0 license name, the EFF upstream URL, and a pointer to a `NOTICE` file (Phase 10 work). This placement is minimal-but-compliant for CC-BY 4.0; the NOTICE file promotion is already on the Phase 10 backlog.

7. **Lint readiness (static review, go tooling not runnable in sandbox)**:
   - `forbidigo`: the project config forbids bare `fmt.Print*` and allows `fmt.Sprintf` / `fmt.Errorf`. Code uses only `fmt.Sprintf` and `fmt.Sprintf`-inside-`panic()`. Clean.
   - `govet` (incl. `shadow`): no obvious variable shadowing. Loop var `tc` in test is per-iteration under Go 1.23; even the redundant `tc := tc` (MN-02) would not trigger shadow since the copy is in the enclosing scope before `t.Run`.
   - `staticcheck`: no deprecated APIs, no obvious SA rules hit. `_ = isValidWord` is not needed because `isValidWord` is referenced by `parseAndValidate` and the test.
   - `errcheck`: no error returns to check (`Allocate` returns `string`; `parseAndValidate` panics instead of returning an error by design — intentional per plan).
   - Expect `go vet ./internal/idgen/...` and `golangci-lint run ./internal/idgen/...` to exit 0.

---

_Reviewed: 2026-04-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
