---
phase: 01-foundation-enforcement-gates
reviewed: 2026-04-23T13:43:44Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - cmd/mcp-chain/main.go
  - cmd/mcp-chain/main_test.go
  - internal/cli/stubs.go
  - internal/cli/stubs_test.go
  - .golangci.yml
  - scripts/check-size.sh
  - scripts/check-startup.sh
  - scripts/check-stdout-silence.sh
  - Makefile
  - .github/workflows/ci.yml
findings:
  critical: 0
  warning: 2
  info: 6
  total: 8
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-04-23T13:43:44Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found (no blockers; 2 minor correctness/consistency issues, 6 info-level observations)

## Summary

Phase 1 delivers a clean, minimal foundation: kong-dispatched CLI skeleton with
four subcommand stubs, stdout-discipline bootstrap as the first executable
statements in `main()`, four enforcement gates (lint, size, startup,
stdout-silence) wired through a Makefile and GitHub Actions, and a pair of
smoke test files. The scaffolding matches the plan 1:1 and the specified
requirements (CORE-01, MCP-02, DIST-03, QA-04).

No security issues. No blockers. The two Warning findings are consistency
gaps: (a) the `purge` xor group does not enforce "exactly one required" as
the code comment claims, and (b) the CI `go test` invocation omits the
`-race -count=1` flags that the Makefile's `test` target uses. Both are
benign for a scaffold phase but worth noting so later phases do not inherit
the inconsistency.

Verified against kong v1.15.0 source: `VersionFlag.BeforeReset` writes to
`app.Stdout` which defaults to `os.Stdout`; `main.go` correctly does NOT
override `Stdout`, so `--version` is the single sanctioned stdout write as
documented.

## Warnings

### WR-01: Purge xor group does not enforce "exactly one required"

**File:** `internal/cli/stubs.go:50-56`

**Issue:** The code comment on line 51 states: "One of `<id>`, `--all`, or
`--resolved` is required (kong's xor tag enforces this) - bare `purge`
errors at parse time." This is inaccurate. Kong's `xor` tag enforces
mutual exclusion among group members, not "one-of-required". `ID` is
declared `optional:""` and is NOT tagged into any xor group, while `All`
and `Resolved` share `xor:"target"`. Consequences:

- Bare `purge` (no arg, no flags) passes kong-parse and enters the stub,
  exiting 3 via the stub body. It does NOT error at parse time.
- `purge --all --resolved` is correctly rejected (xor mutual exclusion).

The Phase-1 stub behavior (exit 3) is fine because the stub is a no-op,
but the Phase 7 implementation that replaces this stub will need explicit
validation (e.g., the kong `and`/`xor` combination with `required:""`, or
an explicit check in `Run()`). The misleading comment risks downstream
authors trusting kong to catch an error kong is not catching.

**Fix:** Update the comment to reflect reality, or (optionally) add
`required:""` onto the xor group if kong v1.15 supports that semantics
for this shape. Minimal change — just correct the comment:

```go
// PurgeCmd deletes entries. All/Resolved are mutually exclusive via xor.
// Phase 7 implementation will enforce "exactly one of <id>, --all,
// --resolved" via Run()-body validation (kong's xor alone only enforces
// mutual exclusion, not one-of-required).
```

### WR-02: CI `go test` step omits `-race -count=1`

**File:** `.github/workflows/ci.yml:60`

**Issue:** The CI workflow runs `go test ./...` (line 60) while the
Makefile `test` target runs `go test -race -count=1 ./...` (line 20).
The inconsistency means the CI test step will use build cache and does
NOT exercise the race detector. The research doc calls out `-race
-count=1` as the QA-03 gate and the CLAUDE.md stack guide lists it as
the CI gate. For Phase 1 there is no concurrent code, so this is
behaviorally inert today — but the CI step becomes the source of truth
for downstream phases that DO have goroutines (Phase 5 onward), and
letting it drift from the Makefile now is how regressions hide.

**Fix:** Align the CI step with the Makefile (prefer invoking `make test`
so the command lives in one place):

```yaml
- name: Unit tests (Phase 1 smoke; expands from Phase 2)
  run: make test
```

## Info

### IN-01: Test duplication between `main_test.go` and `stubs_test.go`

**File:** `cmd/mcp-chain/main_test.go:17-33`, `internal/cli/stubs_test.go:59-74`

**Issue:** `TestVersionOutput` (in `cmd/mcp-chain/main_test.go`) and
`TestVersionFlagWritesToStdout` (in `internal/cli/stubs_test.go`) assert
essentially the same invariant (`--version` writes to stdout, exits 0,
output starts with/contains `mcp-chain`). Each test also rebuilds the
binary from scratch via `go build`, which adds ~1-2 seconds of test
runtime per duplicate.

**Fix:** Consolidate into one `TestVersionOutput` in `main_test.go`
(closer to the entrypoint it validates) and remove the duplicate from
`stubs_test.go`. Or, if the stubs-package test is meant to be
package-local, leave it and delete the main_test.go version — either
resolution works; just pick one.

### IN-02: `stubs_test.go` builds the `cmd/mcp-chain` binary from an `internal/cli` test

**File:** `internal/cli/stubs_test.go:77-88`

**Issue:** The `buildBinary` helper in the `internal/cli` test package
shells out to `go build -o <tmp> ../../cmd/mcp-chain`. This mixes layers:
the stubs package is tested by invoking the top-level binary, not the
stubs package itself. It works, but the test is effectively an
integration test of the main binary that happens to live in the
`internal/cli` directory. A clearer structure would place all
binary-level tests in `cmd/mcp-chain/*_test.go` and leave
`internal/cli/stubs_test.go` for unit tests of the stubs' `Run()`
methods (these are hard to unit-test directly because they call
`os.Exit` — so the current integration-style approach is a reasonable
compromise for Phase 1).

**Fix:** Defer to Phase 5/6/7 refactor when real subcommand logic lands.
For now, add a one-line package-doc comment explaining the layering
choice:

```go
// Package cli_test is an integration-style test that builds the top-level
// binary. Subcommand Run() methods call os.Exit, which cannot be unit-
// tested in-process, so we drive them through the real entrypoint.
```

### IN-03: `printf '%.2f'` with awk output can format-break on tiny sizes

**File:** `scripts/check-size.sh:17`

**Issue:** The line
`printf 'size:   %d bytes (%.2f MB)\n' "$SIZE" "$(awk "BEGIN {print $SIZE/1048576}")"`
pipes `awk`'s default-format output (`%g`-style) into `printf %.2f`. For
any binary size in the realistic mcp-chain range (1-15 MB), awk emits
plain decimal (`7.234567`) and `%.2f` formats cleanly. At exotic sizes
(e.g., <1 KB), awk can emit scientific notation (`7.62939e-07`) which
breaks `%.2f` and prints `0.00`. Not a practical concern for an
executable binary, but worth a one-line change for robustness.

**Fix:** Format in awk directly and pass a pre-formatted string to
printf:

```bash
printf 'size:   %d bytes (%s MB)\n' "$SIZE" "$(awk "BEGIN {printf \"%.2f\", $SIZE/1048576}")"
```

### IN-04: Redundant linter listing in `.golangci.yml`

**File:** `.golangci.yml:13-17`

**Issue:** `default: standard` already enables `govet` and `staticcheck`
in golangci-lint v2. Explicitly listing them under `enable:` is
redundant. The file comments acknowledge this for `govet` ("listed for
visibility") — the redundancy is a stylistic choice, not a bug, but it
adds five lines of config for zero functional change. `forbidigo` is
the only genuinely new linter being enabled here.

**Fix:** Optional. Either keep the current form (explicit is nice) or
trim to:

```yaml
linters:
  default: standard
  enable:
    - forbidigo
```

### IN-05: `go list -f '{{ join .Deps "\n" }}' ./...` does not see test-only imports

**File:** `.github/workflows/ci.yml:52-57`

**Issue:** The inline net/http ban uses `go list -f ... ./...` without
`-test`. This correctly scans runtime deps but will NOT catch a test
that imports `net/http` (e.g., `httptest.NewServer`). For Phase 1 there
are no such tests — but the ban's stated intent is "pre-emptive MCP-01"
which applies even to tests if they pull `net/http` transitively into
the build graph via shared packages. Adding `-deps -test` makes the
check stricter.

**Fix:** Optional hardening. Change line 54 to:

```yaml
if go list -deps -test -f '{{ .ImportPath }}' ./... | grep -q '^net/http$'; then
```

(`-deps` walks full dep graph; `-test` includes test-only imports.)

### IN-06: `goroutine` / `errgroup` concurrency tests not exercised

**File:** `cmd/mcp-chain/main_test.go`, `internal/cli/stubs_test.go`

**Issue:** As noted in WR-02, even with `-race` the Phase 1 code has no
concurrency to race on. This is not an omission — it is a correct
description of the phase scope. Flagged here only so Phase 2/5
reviewers remember that the race gate becomes meaningful when
`go-sdk` stdio serving and file-lock code land.

**Fix:** No action. This is a tripwire for later phases.

---

_Reviewed: 2026-04-23T13:43:44Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
