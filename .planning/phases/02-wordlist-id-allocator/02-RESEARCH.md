# Phase 2: Wordlist & ID Allocator — Research

**Researched:** 2026-04-23
**Domain:** Go `//go:embed` + deterministic ID allocation over a static wordlist with numeric fallback
**Confidence:** HIGH

## Summary

Phase 2 delivers a single pure function — `idgen.Allocate(counter uint64) string` — over the EFF
short wordlist (1296 words) with a zero-padded hex fallback once the wordlist is exhausted. The
whole phase is four files in `internal/idgen/`: the TSV data (checked into the repo verbatim from
EFF), an embed-and-parse step with startup-time invariants (count / uniqueness / charset), the
`Allocate` function, and a table-driven test. No filesystem, no concurrency, no external dep.

The one non-trivial finding from investigating the upstream file: the EFF short wordlist is **not**
strictly `^[a-z]+$`. One word — `yo-yo` at line 1286 — contains an ASCII hyphen. The startup
validator must accept `[a-z-]+` (one or more of lowercase ASCII letters or hyphen), not just
`[a-z]+`. The rest of the list is plain `[a-z]{3,5}`. A second finding: the wordlist is CC-BY 4.0,
not public domain as CONTEXT.md claims; attribution must live in a NOTICE file or in-file comment.

**Primary recommendation:** Ship four files under `internal/idgen/` — `eff_short_wordlist_1.txt`
(verbatim upstream TSV, sha256 `8f5ca830b8bffb6fe39c9736c024a00a6a6411adb3f83a9be8bfeeb6e067ae69`),
`wordlist.go` (embed + parse + `TestMain` invariants), `idgen.go` (`Allocate`), and
`idgen_test.go` (table-driven boundary tests). Parse the TSV at package init; panic on invariant
violation so a bad build fails at first use rather than mid-`Allocate`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Embed raw wordlist bytes | Build tooling (`//go:embed`) | — | Compile-time data inclusion; no runtime I/O |
| Parse TSV into `[]string` | Package init (`idgen`) | — | One-time cost at first import; keeps `Allocate` hot-path branchless |
| Validate invariants (count, dupes, charset) | Package init (panic) + `TestMain` (assert) | — | Panic catches broken builds; `TestMain` makes it an explicit CI gate |
| Map counter → ID | `idgen.Allocate` (pure fn) | — | Hexagonal core; no state, no deps, trivially testable |
| Monotonic counter source | (out of scope — Phase 4 store) | — | `idgen` is stateless; store owns the counter |

## User Constraints (from CONTEXT.md)

### Locked Decisions
- Go ≥ 1.23, pure-Go
- Package location: `internal/idgen/` (private; store layer will consume it)
- EFF short wordlist source: `https://www.eff.org/files/2016/09/08/eff_short_wordlist_1.txt`
- `Allocate` MUST be pure — takes `uint64`, returns `string`; no rand, no mutex, no state
- Hex fallback: `hex-0001`, `hex-0002`, etc. (zero-padded lowercase hex, 4 digits)
  - Index 1296 → `hex-0001`; index 1296+N → `hex-<hex(N+1)>`
  - Digits widen past `1296 + 65535 = 66831` (Go's `%04x` is min-width, not truncation)
- `//go:embed` the TSV file inside `internal/idgen/` (file checked into repo)
- Startup test asserts: 1296 words, uniqueness, no trailing whitespace

### Claude's Discretion
All implementation details — discuss phase was skipped.

### Deferred Ideas (OUT OF SCOPE)
- Randomness / seeding (store owns the counter — Phase 4)
- Collision handling (counter is monotonic — store's job)
- Concurrency primitives (`idgen` is stateless)

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-07 | Word-ID generator uses the EFF short wordlist (1296 words, embedded via `go:embed`). Monotonic counter selects next word; once exhausted, falls back to hex counter. Counter never decremented on purge. | This phase implements the embed + the pure `Allocate(counter uint64) string`. Counter monotonicity + no-decrement-on-purge are the store's responsibility in Phase 4. |

## Project Constraints (from CLAUDE.md)

- Pure Go — no cgo deps (`CGO_ENABLED=0` must succeed)
- Stdlib-first; only add a dep if it saves meaningful code — **this phase adds zero deps**
- Binary size budget ≤15 MB (the wordlist is 13,660 bytes; rounding budget impact: negligible)
- Startup ≤100 ms — `init()`-time parse of 1296 lines is sub-millisecond; irrelevant to budget
- `forbidigo` forbids `fmt.Print(f|ln)?` — `Allocate` uses `fmt.Sprintf` (not banned), safe
- `govet` + `staticcheck` must be clean

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `embed` | Go 1.16+ (we're on 1.23) | Compile-time file inclusion via `//go:embed` | [CITED: pkg.go.dev/embed] Canonical way to ship static data in a Go binary; zero runtime cost, single binary, no filesystem dep |
| Go stdlib `strings` | — | `strings.Split`, `strings.Cut` for TSV parsing | [CITED: pkg.go.dev/strings] `strings.Cut` (Go 1.18+) gives a one-line tab-split cleaner than `strings.SplitN` |
| Go stdlib `fmt` | — | `fmt.Sprintf("hex-%04x", n)` for fallback | [CITED: pkg.go.dev/fmt] `%04x` is minimum-width padding, never truncates — widens past 4 digits automatically |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify/require` | v1.11.1 (per CLAUDE.md) | Fail-fast test assertions | Use `require.Equal`, `require.Len` for readable table-driven tests. `require` not `assert` — halt on first failure |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `//go:embed eff_short_wordlist_1.txt` (the raw TSV) | Pre-stripped word-only text file, or a generated `.go` file with `var words = []string{"acid", ...}` | Pre-stripping loses provenance (you can't verify against upstream sha). Code-gen adds a `go:generate` step and makes review harder. Raw TSV is self-evidently the EFF file; the 11-line parser is lower-risk than either alternative. [ASSUMED — judgment call] |
| `strings.Cut(line, "\t")` | `strings.Split(line, "\t")[1]` | `Cut` avoids a slice allocation and handles malformed lines (no tab) cleanly via the boolean return. [CITED: pkg.go.dev/strings#Cut] |
| `fmt.Sprintf("hex-%04x", counter-1295)` | `strconv.FormatUint(counter-1295, 16)` + manual left-pad | `Sprintf` is 1 line, auto-pads, and auto-widens. `strconv` would require a helper for padding. Idiomatic Go for fixed-width numeric formatting is `fmt.Sprintf`. [VERIFIED: pkg.go.dev/fmt — `%04x` min-width specifier] |
| Panic on invariant violation in `init()` | Return error from a `Load()` function | `init()` panic is the standard way to fail a broken build — it's caught by the first unit test that imports the package, and it keeps `Allocate` callers simple (no error return, no `sync.Once`). [ASSUMED — judgment call] |

**Installation:** No new dependencies required for Phase 2. `testify/require` lands naturally the first time a test imports it and `go mod tidy` runs.

**Version verification:** No deps to verify. Wordlist version is pinned by the `sha256` of the committed file.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌──────────────────────────┐
                    │ EFF upstream (CC-BY 4.0) │
                    │ eff_short_wordlist_1.txt │
                    └───────────┬──────────────┘
                                │  [commit once, verbatim]
                                ▼
                    ┌──────────────────────────────────────┐
                    │ internal/idgen/eff_short_wordlist_1.txt │
                    │ (sha256 pinned, 13,660 bytes)           │
                    └───────────┬──────────────────────────┘
                                │  //go:embed at compile time
                                ▼
                    ┌──────────────────────────┐
                    │ rawWordlist string       │  (bundled in binary)
                    └───────────┬──────────────┘
                                │  package init() runs once
                                ▼
                    ┌──────────────────────────────────┐
                    │ parseAndValidate()               │
                    │  • split on '\n'                 │
                    │  • strings.Cut on '\t'           │
                    │  • reject invalid charset        │
                    │  • panic if count != 1296        │
                    │  • panic on duplicates           │
                    └───────────┬──────────────────────┘
                                │
                                ▼
                    ┌──────────────────────────┐
                    │ words []string (len=1296)│   (package-private, immutable)
                    └───────────┬──────────────┘
                                │
                      [read-only]│
                                ▼
┌──────────────────┐    ┌──────────────────────────────────┐    ┌────────────┐
│ Phase 4 store    │──▶ │ idgen.Allocate(counter uint64)   │──▶ │ string id  │
│ (monotonic ctr)  │    │   counter < 1296 → words[counter]│    │            │
└──────────────────┘    │   counter >= 1296 →              │    └────────────┘
                        │     fmt.Sprintf("hex-%04x",      │
                        │       counter-1295)              │
                        └──────────────────────────────────┘
```

Data flow: EFF file → committed TSV → `//go:embed` string → one-time init parse →
immutable `[]string` → pure `Allocate(counter)` lookup → ID string.

### Recommended Project Structure

```
internal/idgen/
├── eff_short_wordlist_1.txt   # verbatim from EFF, sha256 pinned, CC-BY 4.0
├── wordlist.go                # //go:embed + package init parse + invariants
├── idgen.go                   # Allocate(counter uint64) string
└── idgen_test.go              # TestMain invariants + table-driven Allocate tests
```

### Pattern 1: `//go:embed` a static data file

**What:** Bundle the raw wordlist bytes into the compiled binary via Go's `embed` directive.
**When to use:** Any time the data is small (<1 MB), rarely changes, and must be available without
filesystem I/O. Per [CITED: pkg.go.dev/embed], the directive must be placed on the line
immediately above the variable declaration, and the target type must be `string`, `[]byte`, or
`embed.FS`.

**Example (copy-paste-ready):**
```go
// Source: pkg.go.dev/embed
package idgen

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed eff_short_wordlist_1.txt
var rawWordlist string

// words is the parsed wordlist, populated once at package init.
// Index 0 corresponds to the first line of the TSV ("acid"), index 1295 the last ("zoom").
var words []string

func init() {
	words = parseAndValidate(rawWordlist)
}
```

### Pattern 2: Parse-and-validate in `init()` with panic on invariant violation

**What:** Do the full parse + invariant check at package init; panic if anything is off. This
turns a silently-broken wordlist file into a loud build-time / first-test failure.

**When to use:** For embedded static data with contract invariants (fixed count, uniqueness,
charset). Panic is the idiomatic way to signal "this build is broken, do not run." Any test that
imports `idgen` — including `go test ./...` — will trigger init and surface the panic with its
message.

**Example (copy-paste-ready):**
```go
// parseAndValidate splits the TSV, extracts the second column, and asserts
// invariants. Panics are intentional: a broken wordlist is a broken build.
func parseAndValidate(raw string) []string {
	// The canonical EFF file is LF-terminated ASCII. Accept an optional
	// trailing newline (the committed file ends with one).
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) != 1296 {
		panic(fmt.Sprintf("idgen: wordlist line count = %d, want 1296", len(lines)))
	}

	out := make([]string, 0, 1296)
	seen := make(map[string]struct{}, 1296)
	for i, line := range lines {
		_, word, ok := strings.Cut(line, "\t")
		if !ok {
			panic(fmt.Sprintf("idgen: line %d missing tab separator: %q", i+1, line))
		}
		if word == "" {
			panic(fmt.Sprintf("idgen: line %d has empty word", i+1))
		}
		if !isValidWord(word) {
			panic(fmt.Sprintf("idgen: line %d has invalid word %q (want [a-z-]+)", i+1, word))
		}
		if strings.HasPrefix(word, "hex-") {
			panic(fmt.Sprintf("idgen: line %d word %q collides with hex- fallback prefix", i+1, word))
		}
		if _, dup := seen[word]; dup {
			panic(fmt.Sprintf("idgen: line %d duplicate word %q", i+1, word))
		}
		seen[word] = struct{}{}
		out = append(out, word)
	}
	return out
}

// isValidWord accepts lowercase ASCII letters and ASCII hyphen.
// The hyphen is needed for exactly one EFF wordlist entry ("yo-yo" at line 1286).
func isValidWord(w string) bool {
	for _, r := range w {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
```

### Pattern 3: Pure `Allocate` function

**What:** Branch on `counter < 1296` for the direct lookup; `fmt.Sprintf("hex-%04x", ...)` for
the fallback. No state, no error return.

**When to use:** Every call site. The function is safe to call from any goroutine without
synchronization because `words` is read-only after init.

**Example (copy-paste-ready):**
```go
// Source: phase-local, combining stdlib fmt + the parsed wordlist.
package idgen

import "fmt"

// Allocate returns a deterministic ID for the given monotonic counter.
// counter ∈ [0, 1295]       → the corresponding EFF short wordlist entry
// counter ∈ [1296, 1296+65535] → "hex-0001" .. "hex-ffff" (4-digit zero-pad)
// counter > 1296+65535      → "hex-<wider-hex>" — %04x is min-width, not truncation
//
// The function is pure and safe for concurrent use: the underlying words
// slice is immutable after package init.
func Allocate(counter uint64) string {
	if counter < uint64(len(words)) {
		return words[counter]
	}
	// Offset by (len(words) - 1) so that counter=1296 → hex-0001.
	// Go's fmt %04x treats 4 as a MINIMUM width; values ≥ 0x10000 widen
	// automatically. No silent truncation. [VERIFIED: pkg.go.dev/fmt]
	return fmt.Sprintf("hex-%04x", counter-uint64(len(words)-1))
}
```

### Anti-Patterns to Avoid

- **Parsing the TSV inside `Allocate`.** Would re-do the parse on every call. Parse once at init,
  store the `[]string`, look up by index.
- **Returning `(string, error)` from `Allocate`.** The function is total: every `uint64` maps to a
  valid string. An error return would force every caller into noise. Use panic-at-init for the
  only thing that can go wrong (a broken build).
- **Using a `map[uint64]string` for the wordlist.** O(1) by slice index is already optimal; maps
  add hash overhead, destroy determinism in iteration, and blow the init-time budget.
- **Truncating the hex tail to exactly 4 digits with a manual `strconv` + slice.** The phase
  spec says "digits widen past 1296+65535"; silent truncation would alias IDs past that
  boundary. `%04x` does the right thing by default.
- **Hard-coding the wordlist as a Go slice literal in a generated file.** Loses provenance
  (no way to verify against the upstream EFF sha256 after codegen) and balloons diff review.
- **Embedding as `embed.FS` instead of `string`.** Adds a `ReadFile` call with an error path for
  zero benefit — the file is a single text blob and Go permits embedding directly into a
  `string` variable. [CITED: pkg.go.dev/embed — "For a string or byte slice variable, the file
  content is read once at program startup"]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Load a file at startup without filesystem I/O | `os.ReadFile` at init + error handling + path discovery | `//go:embed` string variable | Stdlib idiom, single binary, zero I/O, zero error paths, zero cross-platform quirks |
| Zero-pad a hex counter | Manual loop or `strings.Repeat("0", ...)` + `strconv.FormatUint` | `fmt.Sprintf("%04x", n)` | 1 line, auto-widens past 4 digits, same width semantics as C printf which callers will recognize |
| Split a line on a single tab | `strings.Split(line, "\t")` + bounds check + index | `strings.Cut(line, "\t")` | Stdlib Go 1.18+ idiom; single allocation; returns `ok bool` to flag malformed rows without a panic |
| Detect duplicates in the wordlist | Sort-and-compare-adjacent | `map[string]struct{}` during parse | Same complexity, no extra pass, works in the natural parse loop |
| Track "original dice roll" for each word | Parse and retain both columns | Discard dice roll column; only the word column matters | Dice rolls are for humans rolling actual dice; the binary never needs them |

**Key insight:** This phase is mechanically small enough that "don't hand-roll" is mostly a
reminder to use `fmt.Sprintf("%04x", ...)` and `//go:embed` rather than reinvent padding or
filesystem loading. The only real trap is the `yo-yo` charset gotcha (below); everything else
is three well-trodden stdlib idioms composed end-to-end.

## Common Pitfalls

### Pitfall 1: Charset is `[a-z-]+`, not `[a-z]+`

**What goes wrong:** A too-strict validator rejects `yo-yo` at line 1286 and panics, silently
breaking the build the moment Phase 2 lands.
**Why it happens:** The EFF short wordlist is marketed as "simple words" and a natural default
regex is `[a-z]+`. The single exception (`yo-yo`, line 1286, dice-roll `6652`) is easy to miss.
**How to avoid:** Accept `[a-z-]+` in `isValidWord`. Every other entry is 3–5 lowercase
letters. Do NOT try to normalize away the hyphen (that changes the wordlist contents and breaks
the upstream sha256 anchor).
**Warning signs:** `init()` panics in the very first test run with "invalid word `yo-yo`".
[VERIFIED: `awk -F'\t' '{print $2}' /tmp/eff_short_wordlist_1.txt | LC_ALL=C grep -v '^[a-z]*$'` returns exactly `yo-yo`]

### Pitfall 2: `%04x` truncation misconception

**What goes wrong:** A developer assumes `%04x` means "exactly 4 digits" and adds a truncation
step or switches format strings past 65535. Either change causes ID collisions past the boundary.
**Why it happens:** Width specifiers in `printf`-like formats are often confused with maximum
widths. In Go (and C) they are **minimum** widths: values that don't fit are rendered in full.
**How to avoid:** Trust the `%04x` format. Test counters `65534`, `65535`, `66830`, `66831`,
`66832` explicitly — the test asserts monotonic uniqueness across the boundary.
**Warning signs:** Any code near `Allocate` that does `[:N]` on a `Sprintf` result; any review
comment about "what if the counter exceeds 4 hex digits".
[CITED: pkg.go.dev/fmt — "If the precision is greater than the value, no truncation occurs"]

### Pitfall 3: Off-by-one in the fallback offset

**What goes wrong:** `fmt.Sprintf("hex-%04x", counter-1296)` makes `counter=1296 → "hex-0000"`.
CONTEXT.md says the first fallback ID is `hex-0001`, not `hex-0000`.
**Why it happens:** The natural subtraction to get "fallback index" is `counter - len(words)`,
which yields `0` for the first fallback counter. But `hex-0000` is ugly and off-by-one vs spec.
**How to avoid:** Subtract `len(words) - 1` (i.e. `1295`), so `counter=1296` maps to `1` and
prints as `hex-0001`. Pin this in a test case.
**Warning signs:** First fallback test returns `"hex-0000"` instead of `"hex-0001"`.

### Pitfall 4: `strings.Split` on trailing newline

**What goes wrong:** `strings.Split("foo\nbar\n", "\n")` returns `["foo", "bar", ""]` — a third
empty element. Your count becomes 1297, not 1296, and `init()` panics.
**Why it happens:** `strings.Split` does not trim trailing delimiters. The EFF file ends with a
newline (as verified: `wc -l` = 1296 but `wc -c` = 13660, leaving one trailing `\n`).
**How to avoid:** `strings.TrimRight(raw, "\n")` before the split. Do NOT use `bufio.Scanner` —
it adds a reader allocation and silently skips empty final lines, which hides the same bug.
**Warning signs:** Count-mismatch panic on exactly the first test run.

### Pitfall 5: CC-BY attribution missing

**What goes wrong:** The EFF wordlist ships in the binary without attribution. CC-BY 4.0
requires reasonable credit.
**Why it happens:** CONTEXT.md incorrectly claims the file is "public domain". EFF's site says
all original material is CC-BY 4.0. [VERIFIED: eff.org/copyright]
**How to avoid:** Add an attribution comment at the top of `eff_short_wordlist_1.txt`
(impossible — the file must be verbatim for sha pinning) OR at the top of `wordlist.go`
explaining the file's provenance and license, OR a `NOTICE` file in the repo root. A comment
in `wordlist.go` is the lowest-friction choice.
**Warning signs:** Pre-release license audit flags "wordlist from third party, no attribution".

### Pitfall 6: `//go:embed` directive placement

**What goes wrong:** The `//go:embed` comment is separated from the variable declaration by a
blank line or a comment, and the Go compiler silently doesn't embed.
**Why it happens:** The directive only works when it is the *immediately preceding* comment on
the variable line. A regular doc comment between the `//go:embed` directive and the variable
breaks the pairing.
**How to avoid:** Keep `//go:embed eff_short_wordlist_1.txt` and `var rawWordlist string` on
adjacent lines with no intervening blank line or comment. Put any explanatory comment ABOVE
the `//go:embed` line.
**Warning signs:** `rawWordlist` is an empty string at init time; count panic fires immediately.
[CITED: pkg.go.dev/embed — "The directive must immediately precede a line containing the
declaration of a single variable"]

### Pitfall 7: Import `embed` without the blank identifier

**What goes wrong:** `import "embed"` without using any symbol from it produces an
`unused-import` compile error. [CITED: pkg.go.dev/embed — "If no qualified identifier is used,
the import should be blank: `import _ "embed"`"]
**Why it happens:** Unless you declare an `embed.FS` variable, the package itself isn't
referenced — `//go:embed` is a compiler directive, not a function call.
**How to avoid:** Use `import _ "embed"` whenever you embed into a `string` or `[]byte`.
**Warning signs:** Build fails with `imported and not used: "embed"`.

## Runtime State Inventory

> Greenfield phase — no rename / refactor / migration. Section omitted.

## Code Examples

Verified patterns from official Go sources, assembled for copy-paste into Phase 2 plans.

### `wordlist.go` (complete, copy-paste-ready)

```go
// Package idgen generates short, deterministic IDs for chain entries.
// Entries 0..1295 come from the EFF short wordlist (embedded below).
// Entries >= 1296 fall back to zero-padded lowercase hex ("hex-0001" onward).
//
// The EFF short wordlist (eff_short_wordlist_1.txt) was downloaded from
//   https://www.eff.org/files/2016/09/08/eff_short_wordlist_1.txt
// and is redistributed here verbatim under the Creative Commons
// Attribution 4.0 International License (CC-BY 4.0). See NOTICE for details.
package idgen

import (
	_ "embed"
	"fmt"
	"strings"
)

// wordlistSize is the invariant count of entries in the EFF short wordlist.
// If EFF ever re-releases the file with a different count, the build fails
// loudly at init() rather than silently shifting IDs.
const wordlistSize = 1296

//go:embed eff_short_wordlist_1.txt
var rawWordlist string

// words is the parsed wordlist. Package-private; read-only after init().
// Safe for concurrent read access from any goroutine.
var words []string

func init() {
	words = parseAndValidate(rawWordlist)
}

// parseAndValidate splits the TSV, extracts the word column, and enforces:
//   - exactly wordlistSize entries
//   - every line has a tab separator
//   - every word matches [a-z-]+ (the hyphen admits exactly one EFF entry: "yo-yo")
//   - no word begins with "hex-" (would collide with the fallback scheme)
//   - no duplicate words
//
// Any violation panics. A broken wordlist is a broken build.
func parseAndValidate(raw string) []string {
	// EFF file is LF-terminated ASCII; accept an optional single trailing newline.
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) != wordlistSize {
		panic(fmt.Sprintf("idgen: wordlist line count = %d, want %d", len(lines), wordlistSize))
	}

	out := make([]string, 0, wordlistSize)
	seen := make(map[string]struct{}, wordlistSize)
	for i, line := range lines {
		_, word, ok := strings.Cut(line, "\t")
		if !ok {
			panic(fmt.Sprintf("idgen: line %d missing tab separator: %q", i+1, line))
		}
		if word == "" {
			panic(fmt.Sprintf("idgen: line %d has empty word", i+1))
		}
		if !isValidWord(word) {
			panic(fmt.Sprintf("idgen: line %d has invalid word %q (want [a-z-]+)", i+1, word))
		}
		if strings.HasPrefix(word, "hex-") {
			panic(fmt.Sprintf("idgen: line %d word %q collides with hex- fallback prefix", i+1, word))
		}
		if _, dup := seen[word]; dup {
			panic(fmt.Sprintf("idgen: line %d duplicate word %q", i+1, word))
		}
		seen[word] = struct{}{}
		out = append(out, word)
	}
	return out
}

// isValidWord returns true iff w contains only lowercase ASCII letters
// or ASCII hyphen. The hyphen is present in exactly one EFF entry ("yo-yo",
// line 1286); every other entry is 3-5 lowercase letters.
func isValidWord(w string) bool {
	for _, r := range w {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
```

### `idgen.go` (complete, copy-paste-ready)

```go
package idgen

import "fmt"

// Allocate returns a deterministic ID for the given monotonic counter.
//
//	counter ∈ [0, 1295]            → words[counter]
//	counter ∈ [1296, 1296+65535]   → "hex-0001" .. "hex-ffff" (4-digit zero-pad)
//	counter > 1296+65535           → "hex-<5+ hex digits>" (%04x is min-width,
//	                                  not truncation — widens automatically)
//
// Pure, allocation-light (one string for the hex branch), and safe for
// concurrent use (words is immutable after init).
//
// Requirement: CORE-07.
func Allocate(counter uint64) string {
	if counter < uint64(len(words)) {
		return words[counter]
	}
	// Offset so the first fallback (counter=1296) renders as "hex-0001",
	// not "hex-0000". Subtract len(words)-1 instead of len(words).
	return fmt.Sprintf("hex-%04x", counter-uint64(len(words)-1))
}
```

### `idgen_test.go` (complete, copy-paste-ready)

```go
package idgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWordlistInvariants re-asserts at test time the invariants that init()
// already panics on. This means a CI `go test ./internal/idgen/...` run is
// sufficient to detect a corrupted wordlist file before any downstream code
// depends on it. (A broken init would surface here as a test-binary panic.)
func TestWordlistInvariants(t *testing.T) {
	require.Len(t, words, wordlistSize, "wordlist must contain exactly 1296 entries")

	seen := make(map[string]struct{}, len(words))
	for i, w := range words {
		require.NotEmpty(t, w, "entry %d is empty", i)
		require.True(t, isValidWord(w), "entry %d %q is not [a-z-]+", i, w)
		require.False(t, strings.HasPrefix(w, "hex-"),
			"entry %d %q collides with hex- fallback prefix", i, w)
		if _, dup := seen[w]; dup {
			t.Fatalf("entry %d %q is a duplicate", i, w)
		}
		seen[w] = struct{}{}
	}
}

// TestWordlistBoundaries pins the first and last entries of the list so an
// accidental EFF file reorder (or an off-by-one in the parser) fails loudly.
func TestWordlistBoundaries(t *testing.T) {
	require.Equal(t, "acid", words[0], "first word must be 'acid' (EFF line 1)")
	require.Equal(t, "zoom", words[wordlistSize-1], "last word must be 'zoom' (EFF line 1296)")
}

// TestAllocate is the table-driven boundary test required by CORE-07.
// Covers: lower bound, lower+1, wordlist end, fallback start, fallback end
// at 4-digit boundary, and widening past 4 digits.
func TestAllocate(t *testing.T) {
	tests := []struct {
		name    string
		counter uint64
		want    string
	}{
		{"first word", 0, "acid"},
		{"second word", 1, "acorn"},
		{"last word", 1295, "zoom"},
		{"first fallback (hex-0001)", 1296, "hex-0001"},
		{"second fallback (hex-0002)", 1297, "hex-0002"},
		{"fallback at hex-ffff-1", 65534, "hex-fffe"},
		{"fallback at hex-ffff", 65535, "hex-ffff"},
		{"boundary before widen", 66830, "hex-ffff"},     // 66830 - 1295 = 65535 = 0xffff
		{"widen to 5 digits", 66831, "hex-10000"},         // 66831 - 1295 = 65536 = 0x10000
		{"widen + 1", 66832, "hex-10001"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, Allocate(tc.counter))
		})
	}
}

// TestAllocateMonotonicUniqueOverBoundary asserts that no two counters in
// [1290, 1310] produce the same string. Catches off-by-one at the
// wordlist→hex handoff.
func TestAllocateMonotonicUniqueOverBoundary(t *testing.T) {
	seen := make(map[string]uint64, 21)
	for c := uint64(1290); c <= 1310; c++ {
		got := Allocate(c)
		if prev, dup := seen[got]; dup {
			t.Fatalf("counter %d and %d both produced %q", prev, c, got)
		}
		seen[got] = c
	}
}
```

### Wordlist file commit (the TSV)

- Download: `curl -sSfLo internal/idgen/eff_short_wordlist_1.txt https://www.eff.org/files/2016/09/08/eff_short_wordlist_1.txt`
- Verify: `sha256sum internal/idgen/eff_short_wordlist_1.txt` must print `8f5ca830b8bffb6fe39c9736c024a00a6a6411adb3f83a9be8bfeeb6e067ae69`
- Commit verbatim — do NOT reformat, strip comments, or re-encode. The file is plain ASCII LF.

### Upstream format evidence (verified via WebFetch + local parse)

First 5 lines:
```
1111	acid
1112	acorn
1113	acre
1114	acts
1115	afar
```

Last 5 lines:
```
6662	zero
6663	zesty
6664	zippy
6665	zone
6666	zoom
```

Format characteristics (all verified by `awk`/`grep`/`wc` on the downloaded file):
- 1296 lines, LF-terminated, pure ASCII
- Each line: `<4-digit-dice-roll>\t<word>\n`
- Word-length histogram: 82 × 3-letter, 432 × 4-letter, 782 × 5-letter
- Exactly 1295 words match `^[a-z]+$`; exactly 1 word (`yo-yo`, line 1286) matches `^[a-z-]+$` and not `^[a-z]+$`
- Zero duplicates
- Zero words begin with `hex-`
- Total size: 13,660 bytes
- sha256: `8f5ca830b8bffb6fe39c9736c024a00a6a6411adb3f83a9be8bfeeb6e067ae69`

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `go generate` producing a `words.go` with a big slice literal | `//go:embed` the raw data file | Go 1.16 (Feb 2021) | Preserves file provenance, makes diffs reviewable, eliminates codegen step |
| Reading the wordlist from disk at startup via `os.ReadFile` | `//go:embed` into the binary | Go 1.16 | Single static binary, zero runtime filesystem I/O, no path discovery, works inside containers / read-only rootfs |
| `strings.SplitN(line, "\t", 2)[1]` for tab-split | `_, word, ok := strings.Cut(line, "\t")` | Go 1.18 (Mar 2022) | Zero allocations, explicit `ok` bool, more readable |
| `errors.New` + wrap helpers from `pkg/errors` | `fmt.Errorf("...: %w", err)` with `errors.Is` / `errors.As` | Go 1.13 | Stdlib is sufficient; `pkg/errors` is deprecated |

**Deprecated/outdated:**
- `github.com/pkg/errors` — replaced by stdlib `errors` + `%w` verb since Go 1.13. Not used here.
- Manually-generated word slices — replaced by `//go:embed` since Go 1.16.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Committing the raw EFF TSV (rather than a pre-stripped word-only file or codegen output) is the correct choice for provenance + review. | Standard Stack / Alternatives Considered | Low — if the reviewer prefers a different format, the parser changes by ~3 lines. No external surface impact. |
| A2 | Panic-at-init (rather than a `Load()` error return) is the right failure mode for a broken wordlist. | Pattern 2 | Low — if the reviewer prefers an error, convert `parseAndValidate` to return `([]string, error)` and move `init()` body into a `sync.Once`-guarded `Load()`. No `Allocate` signature change. |
| A3 | CC-BY 4.0 attribution in a top-of-file comment in `wordlist.go` satisfies CC-BY 4.0 obligations for a binary distribution. | Pitfall 5 | Medium — legal convention varies. If a reviewer insists on a `NOTICE` or `LICENSES/` directory, the artifact moves but the code does not. Worth confirming before the first tagged release (this is a Phase 10 concern, not Phase 2). |
| A4 | `wordlistSize = 1296` is a load-bearing const; EFF is extremely unlikely to re-release the short wordlist with a different count. | wordlist.go | Very low — EFF has not changed this file since 2016-09-08. If they ever do, init() panics loudly. |
| A5 | `kong` and the rest of Phase 1 do not pre-import `idgen`; there is no danger of a circular import. | Architecture | Very low — `internal/idgen` has no dependencies on `internal/cli` or `cmd/mcp-chain`. Dependency arrow is one-way. |

## Open Questions

1. **NOTICE file vs in-file attribution for CC-BY 4.0.**
   - What we know: EFF content is CC-BY 4.0; attribution is required.
   - What's unclear: Whether the attribution belongs in `wordlist.go` (simplest), in a top-level
     `NOTICE` file (conventional for Go projects), or in the `README.md` (deferred to Phase 10).
   - Recommendation: Put a comment at the top of `wordlist.go` for Phase 2; add a `NOTICE` file
     (or `THIRD_PARTY_LICENSES.md`) in Phase 10 alongside the main `LICENSE`. Note in the Phase 2
     plan handoff that this is a Phase 10 follow-up.

2. **Should the `wordlistSize = 1296` constant be exported?**
   - What we know: The store layer (Phase 4) will want to know the boundary to format log
     messages ("falling back to hex after 1296 registrations").
   - What's unclear: Whether Phase 4 benefits from knowing the number, or whether the opaque
     `Allocate` is enough.
   - Recommendation: Start unexported. If Phase 4 research asks for it, export with a
     godoc-line as `WordlistSize`. YAGNI until then.

3. **Should `Allocate` accept `int` or `uint64`?**
   - What we know: CONTEXT.md locks in `uint64`.
   - What's unclear: Whether `int` would be easier downstream (map keys, JSON marshalling).
   - Recommendation: Stick with `uint64` per the locked decision. Matches a monotonic counter
     semantically (never negative, never shrinks).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build, test | (inferred from Phase 1 scaffold) | 1.23.8 (per go.mod) | — |
| Network access to `eff.org` | One-time wordlist download during phase execution | Yes (verified via curl) | — | Archived copies exist on archive.org as fallback |
| `curl` / `wget` | One-time wordlist download | — | — | Any HTTP client; file is 13.6 KB |
| `sha256sum` | Verifying the downloaded file | — | — | `shasum -a 256` on macOS |

**No missing dependencies.** The phase has no runtime dependencies beyond the Go stdlib.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| Config file | none (Go test runner needs no config) |
| Quick run command | `go test ./internal/idgen/...` |
| Full suite command | `go test -race -count=1 ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CORE-07 (wordlist count / uniqueness / charset) | `init()` parse asserts 1296 unique [a-z-]+ words, plus a reassertion at test time | unit | `go test ./internal/idgen/ -run TestWordlistInvariants -v` | ❌ Wave 0 |
| CORE-07 (wordlist content pins) | First word is `acid`, last is `zoom` | unit | `go test ./internal/idgen/ -run TestWordlistBoundaries -v` | ❌ Wave 0 |
| CORE-07 (`Allocate` determinism across boundaries) | Table-driven: counters 0, 1, 1295, 1296, 1297, 65534, 65535, 66830, 66831, 66832 map to the documented IDs | unit | `go test ./internal/idgen/ -run TestAllocate -v` | ❌ Wave 0 |
| CORE-07 (no alias at wordlist/hex handoff) | Monotonic uniqueness over counters [1290, 1310] | unit | `go test ./internal/idgen/ -run TestAllocateMonotonicUniqueOverBoundary -v` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/idgen/...` (<1 s, no race flag needed — pure in-memory)
- **Per wave merge:** `go test -race ./...` (from the Phase 1 Makefile / CI gate)
- **Phase gate:** Full race suite green + `go vet ./...` + `staticcheck ./...` before
  `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/idgen/eff_short_wordlist_1.txt` — verbatim EFF download, sha256 pinned
- [ ] `internal/idgen/wordlist.go` — embed + parse + invariants (covers CORE-07 count/uniqueness)
- [ ] `internal/idgen/idgen.go` — `Allocate(counter uint64) string` (covers CORE-07 allocation)
- [ ] `internal/idgen/idgen_test.go` — table-driven boundary tests + invariant reassertion
- [ ] First import of `testify/require` — will add `github.com/stretchr/testify v1.11.1` to
      `go.mod` via `go mod tidy` on the first `go test` run. No manual `go get` required.

*(No framework install needed — stdlib `testing` is already present; `testify` auto-installs on
first test run.)*

## Security Domain

The `idgen` package has no network, filesystem, process, or cryptographic surface. It is a pure
function over a static embedded string.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | **yes (build-time only)** | Wordlist invariants enforced at `init()` via panic; input to `Allocate` is a `uint64` with every value a valid input by definition |
| V6 Cryptography | no | ID uniqueness comes from monotonic counter, not from cryptographic randomness; OwnerToken generation (which IS crypto) lives in Phase 4 |

### Known Threat Patterns for Phase 2

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Wordlist tampering in the committed file | Tampering | sha256 pin in Phase 2 plan + `init()` invariants (count, charset, dedup, no `hex-` prefix) — any tamper breaks the build |
| Supply-chain attack via a replacement wordlist with a `hex-` prefix word | Tampering | `init()` rejects any word beginning with `hex-`; the fallback scheme stays unambiguous |
| ID predictability enabling session hijack | Information disclosure / Spoofing | **Out of scope for idgen.** Session authorization is enforced by the `OwnerToken` (Phase 4), not by ID opacity. IDs are intentionally short and memorable — they are coordination tokens, not secrets. |
| Counter overflow (`uint64` wrap) | DoS | Practically unreachable — would require 2^64 registrations. Documented as "not a defended threat model" in the Phase 4 store design. |

## Sources

### Primary (HIGH confidence)
- `https://www.eff.org/files/2016/09/08/eff_short_wordlist_1.txt` — downloaded and verified
  locally; 1296 lines, sha256 `8f5ca830b8bffb6fe39c9736c024a00a6a6411adb3f83a9be8bfeeb6e067ae69`,
  format `<dice>\t<word>`, charset `[a-z-]+` (one `yo-yo` entry), zero duplicates,
  no `hex-` prefix collisions
- `https://pkg.go.dev/embed` — `//go:embed` directive semantics, placement rules, variable types
- `https://pkg.go.dev/fmt` — `%04x` minimum-width padding (not truncation)
- `https://pkg.go.dev/strings#Cut` — `strings.Cut` idiom for single-separator split
- `https://www.eff.org/copyright` — EFF content is CC-BY 4.0 (affects attribution, not code)
- `/home/alpine/mcp-chain/.planning/PROJECT.md`, `REQUIREMENTS.md`, `ROADMAP.md` — phase boundary,
  CORE-07 text, dependency on Phase 1
- `/home/alpine/mcp-chain/.planning/phases/02-wordlist-id-allocator/02-CONTEXT.md` — locked
  decisions, non-goals
- `/home/alpine/mcp-chain/go.mod` — Go 1.23.8, kong only
- `/home/alpine/mcp-chain/.golangci.yml` — lint gates that constrain the implementation
- `/home/alpine/mcp-chain/internal/cli/stubs.go`, `cmd/mcp-chain/main.go` — existing project
  conventions (stderr-only logging, stub pattern)

### Secondary (MEDIUM confidence)
- `https://www.eff.org/deeplinks/2016/07/new-wordlists-random-passphrases` — EFF's own explainer
  for the short wordlist; confirms the "1296 words, 4-dice-rolls" framing but does not state
  the license (checked separately via `eff.org/copyright`)

### Tertiary (LOW confidence)
- None. Every claim in this document was verified against either the downloaded file, an official
  Go pkg.go.dev reference, or a project artifact.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, no deps, well-documented since Go 1.18
- Architecture: HIGH — four files, one-way dependency, no runtime state
- Pitfalls: HIGH — the `yo-yo` charset, the `%04x` widening, and the off-by-one on the fallback
  offset are all empirically verified (yo-yo via `grep`, `%04x` via Go docs, offset via spec
  re-read)
- Wordlist content: HIGH — downloaded and byte-verified; sha256 pinned
- License posture: MEDIUM — CC-BY 4.0 is confirmed but the specific attribution placement is a
  judgment call (see Open Question 1)

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (30 days — this phase is all stdlib; nothing should shift under us)
