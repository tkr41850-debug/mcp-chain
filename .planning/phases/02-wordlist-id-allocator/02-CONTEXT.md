# Phase 2: Wordlist & ID Allocator - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

A pure, deterministic `idgen.Allocate(counter uint64) string` is available over the embedded EFF short wordlist with a clean hex fallback past 1296 — fully tested in isolation, zero dependency on store or filesystem.

**Success Criteria:**
1. The EFF short wordlist (1296 unique lowercase words) is embedded via `//go:embed` with a startup-time test asserting count and uniqueness
2. `idgen.Allocate(0..1295)` returns `words[i]` deterministically; `Allocate(1296+)` returns a deterministic hex-suffix ID (`hex-0001`, `hex-0002`, …)
3. A table-driven unit test covers boundary indices (0, 1, 1295, 1296, large values) with no filesystem or concurrency dependency

**Requirements:** CORE-07

**Depends on:** Phase 1 (module scaffold, lint gates)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting.

Key constraints from PROJECT.md / REQUIREMENTS.md / STACK.md:
- Go ≥ 1.23 (as set in Phase 1 go.mod); pure-Go
- Package location: `internal/idgen/` (private; store layer will consume it)
- EFF short wordlist source: https://www.eff.org/files/2016/09/08/eff_short_wordlist_1.txt (1296 words, public domain)
- Allocate MUST be pure: takes uint64 counter, returns string — no rand, no mutex, no state
- Hex fallback format: `hex-0001`, `hex-0002`, etc. — zero-padded lowercase hex, 4 digits
  - Index 1296 → `hex-0001`, index 1296+N → `hex-<hex(N+1)>`
  - At 4 digits, wraparound occurs at 1296 + 65535 = 66831; beyond that digits widen
- `//go:embed wordlist.txt` inside `internal/idgen/` — file checked into repo
- Startup test (or TestMain) asserts: 1296 words, all lowercase, all unique, no trailing whitespace

### Non-goals for this phase
- No randomness / seeding (that's per-process concern handled by store counter in Phase 4)
- No collision handling (counter is monotonic — store's job)
- No concurrency primitives

</decisions>

<code_context>
## Existing Code Insights

Phase 1 established:
- `cmd/mcp-chain/main.go` — kong CLI with stdout discipline
- `internal/cli/stubs.go` — subcommand stubs (exit 3)
- `.golangci.yml` — govet + staticcheck + forbidigo (bans fmt.Print* + net/http)
- CI with size/startup/stdout/net/http gates

No existing `internal/idgen` package. Module path: `github.com/tkr41850-debug/mcp-chain`.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
