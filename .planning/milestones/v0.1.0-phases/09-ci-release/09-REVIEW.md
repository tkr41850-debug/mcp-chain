---
phase: 09-ci-release
reviewed: 2026-04-24T20:30:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - .goreleaser.yaml
  - .github/workflows/release.yml
  - .github/workflows/ci.yml
  - .golangci.yml
  - Makefile
  - internal/cli/integration_test.go
  - cmd/mcp-chain/main.go
  - internal/cli/format/table.go
  - internal/cli/list.go
  - internal/cli/purge.go
  - internal/cli/resolve.go
  - internal/cli/status.go
  - internal/cli/stubs.go
  - internal/store/lock.go
  - internal/mcpserver/server_test.go
findings:
  critical: 0
  warning: 2
  info: 6
  total: 8
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-04-24T20:30:00Z
**Depth:** standard
**Files Reviewed:** 14 (+1 cross-ref: `internal/mcpserver/tools.go`)
**Status:** issues_found (2 warnings, 6 info — no critical issues)

## Summary

Phase 9 cleanly delivers the GoReleaser v2 release pipeline, the 3-OS CI matrix with per-OS race flag, the release dry-run job, the tightened `.golangci.yml` (v2 schema with formatters/linters split), the two QA-02 gap-fill tests, and the Makefile convenience targets. The v2 `.goreleaser.yaml` mirrors `Makefile` flags exactly (`-trimpath`, `-s -w -X main.version={{.Version}}`, `CGO_ENABLED=0`), yields the expected 6-archive × {tar.gz,zip} matrix + `checksums.txt`, and the snapshot-version assertion in `ci.yml::release-dry-run` correctly proves the ldflags pipeline end-to-end. The lint cleanups are all mechanically correct: `_, _ = fmt.Fprint*` on tabwriter / stderr sites is idiomatic (the `Flush` error path on `tabwriter.Writer` still surfaces real IO failures in `format.WriteTable`), the `internal/store/lock.go` `err`→`lerr` rename eliminates the govet shadow without changing control flow, and `ResolveIn(out)` in `server_test.go` is a valid named-conversion between identical single-field struct types (`RegisterOut{ID string}` → `ResolveIn{ID string}`). `TestConcurrentWaiters_AllSeeResolve` uses `context.WithTimeout` + unbuffered-style channels + `sync.WaitGroup` — no `time.Sleep`-based coordination (D-17 compliant). `TestStatus_PurgedMidPoll_Exit1` is similarly deterministic.

Two warnings concern (a) a missing explicit **test-runner flag alignment** between the CI matrix and the `-tags=integration` gate — new QA-02 tests have the `//go:build integration` tag, so the matrix's `go test -race -count=1 ./...` does NOT execute them on any of the three OSes; and (b) CI uses `go-version: "1.25"` literal while `release.yml` uses `go-version-file: go.mod` which resolves to the fully-specified `go 1.25.0` — the values are equivalent today but drift will surface as a silent skew. Info items flag minor polish (windows `.exe` handling in `buildBinary`, the `macos-latest` arm64-only posture being implicit, changelog filter accepting chore commits that include `(scope)`, and the release-dry-run using `tar` when `dist/*.tar.gz` glob could match multiple files).

## Warnings

### WR-01: New QA-02 integration tests are gated by `//go:build integration` but the CI test job omits `-tags=integration`

**File:** `internal/cli/integration_test.go:1`, `.github/workflows/ci.yml:90-92`
**Issue:** `internal/cli/integration_test.go` line 1 is `//go:build integration`. All tests in that file — including the two new QA-02 fills `TestConcurrentWaiters_AllSeeResolve` and `TestStatus_PurgedMidPoll_Exit1` — are compiled out unless the build tag is passed. The CI matrix test step runs:

```yaml
run: go test ${{ matrix.race }} -count=1 -timeout=120s ./...
```

with no `-tags=integration` flag. Local verification in `09-01-SUMMARY.md` T-09 confirms this: the passing run was explicitly `go test -tags=integration -race -count=1 -timeout=180s ./internal/cli/...`. The matrix in CI runs a DIFFERENT command and therefore does NOT exercise the two new tests on any OS — including the Linux+macOS race gate that QA-02 bullets 2+5 are supposed to land on. The summary claims "QA-02 PASS — `TestConcurrentWaiters_AllSeeResolve` … both pass -race -count=5" which is true locally but NOT enforced in CI.

This also means `TestStatus_Concurrent10WithinOneSecond`, `TestPurge_CounterNotDecremented`, and every other subprocess-spawning test in that file is invisible to CI today. The exceptions are the small set in `stubs_test.go` which has no build tag.

**Fix:** Add a second test invocation (or change the matrix step) to run the integration tag:
```yaml
- name: go test
  shell: bash
  run: |
    go test ${{ matrix.race }} -count=1 -timeout=120s ./...
    go test ${{ matrix.race }} -tags=integration -count=1 -timeout=300s ./...
```
On Windows, the same concern as WR-02 applies (no `-race`). A cleaner variant is to add `integration` to a `matrix.tags: ["", "integration"]` dimension, but a second explicit `go test` line is fine and more readable.

Severity is WARNING (not CRITICAL) because the tests do exist, do pass locally, and the gap is in the gate — not the code. But QA-02 / QA-03 both claim "race gate covers these"; today the claim is false.

### WR-02: `-tags=integration` on Windows will need `-race=""` branching

**File:** `.github/workflows/ci.yml:75-76, 90-92`
**Issue:** Follow-on from WR-01: when the integration tests are added to the Windows runner, their current form may fail because `buildBinary` (`internal/cli/stubs_test.go:30`) runs `go build -o tmp/mcp-chain ./cmd/mcp-chain` without `.exe` and then `exec.Command(binPath, ...)`. Actually, `go build -o path` on Windows does NOT auto-append `.exe` — Go only appends `.exe` when `-o` is a directory or when no `-o` is given. The current tests work on linux/darwin but will fail on Windows with `exec: "...\\mcp-chain": file does not exist`.

Existing `stubs_test.go` tests already run on Windows (no build tag) and their continued passing proves either (a) Go does auto-append on Windows after all (behavior is go-version dependent; Go 1.25 may differ from older docs) or (b) these tests were never run on Windows until this PR. Given that Phase 1 CI was Linux-only and this PR introduces the Windows matrix, option (b) is live.

**Fix:** Two-part:
1. Update `buildBinary` to append `.exe` on Windows:
   ```go
   binPath := tmp + "/mcp-chain"
   if runtime.GOOS == "windows" {
       binPath += ".exe"
   }
   ```
2. Once WR-01's integration tag is wired into CI, confirm the Windows runner passes. If it doesn't, consider excluding the integration suite from Windows via a second `//go:build` constraint (`//go:build integration && !windows`) — aligned with D-03 (linux-only gates).

Severity WARNING because the gap is observable on the first Windows CI run; not CRITICAL because the failure mode is "test fails to build" not "silently passes."

## Info

### IN-01: `ci.yml` uses literal `go-version: "1.25"` while `release.yml` uses `go-version-file: go.mod`

**File:** `.github/workflows/ci.yml:21,37,81,105`, `.github/workflows/release.yml:26`
**Issue:** `go.mod` declares `go 1.25.0`. The CI jobs pin the literal `"1.25"`; the release job reads from `go.mod` (which resolves to `1.25.x`, the latest matching minor). Today these produce the same toolchain. If a future contributor bumps `go.mod` to `go 1.26.0`, the release workflow picks up 1.26 automatically but CI stays on 1.25 — the release would build on a toolchain CI never tested. CLAUDE.md's stack doc prefers "single version across dev + CI + release."
**Fix:** Unify on one convention. Recommended: use `go-version-file: go.mod` on every job — single source of truth, matches what the release job does. A release bump that alters the toolchain then requires a go.mod edit (intentional, reviewable) instead of two YAML edits.

### IN-02: `.goreleaser.yaml` changelog `filters.exclude` drops `^chore:` but accepts `^chore(scope):`

**File:** `.goreleaser.yaml:66-73`
**Issue:** The exclude-list regexes (`^docs:`, `^test:`, `^chore:`, `^ci:`) match only the bare-colon form. Conventional commit scopes like `chore(09-01): bump dep` are NOT excluded — they slip through into the "Others" bucket in the release changelog. The project's own Phase-9 commits (per the summary) use scoped conventional commits: `feat(09-01):`, `test(09-01):`, `docs(09-01):`, `chore(09-01):`. The `feat:` and `fix:` group regexes do handle `(scope)` via `(\([[:word:]]+\))??` — the excludes should too.
**Fix:** Tighten the exclude regexes:
```yaml
filters:
  exclude:
    - '^docs(\([^)]+\))?:'
    - '^test(\([^)]+\))?:'
    - '^chore(\([^)]+\))?:'
    - '^ci(\([^)]+\))?:'
    - Merge pull request
    - Merge branch
```
Low severity because it only affects generated release notes cosmetics.

### IN-03: `macos-latest` arm64-only posture is implicit

**File:** `.github/workflows/ci.yml:69`
**Issue:** `matrix.os: [ubuntu-latest, macos-latest, windows-latest]` — `macos-latest` has been arm64 (Apple Silicon) since late 2024. The release artifact matrix produces both `darwin_amd64` and `darwin_arm64` archives, but CI only test-exercises arm64. If a Phase 10+ change subtly breaks darwin/amd64 (e.g., an `if runtime.GOARCH == "amd64"` branch on darwin), CI cannot catch it. RESEARCH §Known Landmines #1 flagged this as a planner-discretion item.
**Fix:** Optional — add `macos-13` (last amd64 runner label) to the matrix if darwin/amd64 coverage matters:
```yaml
matrix:
  os: [ubuntu-latest, macos-latest, macos-13, windows-latest]
  include:
    - os: ubuntu-latest
      race: "-race"
    - os: macos-latest
      race: "-race"
    - os: macos-13
      race: "-race"
    - os: windows-latest
      race: ""
```
Alternatively, document the arm64-only posture in a comment so the decision is explicit. Leaving as-is is also defensible (pure-Go, stdlib-only syscalls — arm64 is representative).

### IN-04: `Makefile::release-snapshot` uses shell globs without `set -eu` discipline

**File:** `Makefile:47-51`
**Issue:** The make recipe uses `ls dist/*.tar.gz 2>/dev/null | wc -l` which:
- silently succeeds with count `1` if `dist/*.tar.gz` does not glob-expand (ls receives the literal argument, errors → suppressed, but `wc -l` counts newlines from stderr-less output = 0) — actually this one's correct
- is rebuilt on every `make release-snapshot` call, which is fine
- but lacks the `set -euo pipefail` discipline of `ci.yml` release-dry-run.

If `goreleaser release --snapshot` partially succeeds (e.g., 3 tar.gz + 2 zip and then fails mid-run), the `test $(ls ...) -eq 4` line correctly reports "ERROR: expected 4 .tar.gz". But if `ls` itself fails (e.g., `dist/` doesn't exist because goreleaser never ran), the user sees a bare `ls: dist/*.tar.gz: No such file or directory` followed by the "ERROR" line with ambiguous causation.

**Fix:** Add an explicit existence check before the count tests:
```make
release-snapshot:
	goreleaser release --snapshot --clean --skip=publish
	@test -d dist || (echo "ERROR: dist/ not produced by goreleaser" && exit 1)
	@echo "--- dist/ contents:"
	@ls -la dist/
	@test $$(ls dist/*.tar.gz 2>/dev/null | wc -l) -eq 4 || ...
```
Defensive polish — not a real correctness issue because the count-check catches the final state.

### IN-05: `ci.yml::release-dry-run` tar glob could match multiple archives

**File:** `.github/workflows/ci.yml:142`
**Issue:** `tar -xzf dist/mcp-chain_*_linux_amd64.tar.gz -C extract` — the glob expands to a single file in practice (GoReleaser produces exactly one `*_linux_amd64.tar.gz` per snapshot). If `--clean` is ever removed or a previous partial run leaves stale artifacts under `dist/`, the glob could match multiple files, and `tar -xzf` with multiple files in one invocation behaves predictably (processes the first) — but a future GoReleaser `split_platforms` feature or a config change that produces both `amd64` and `amd64v3` variants would silently extract the wrong archive.
**Fix:** Make the extract more explicit. Either enumerate the expected file:
```bash
ARCHIVE=$(ls dist/mcp-chain_*_linux_amd64.tar.gz | head -1)
tar -xzf "$ARCHIVE" -C extract
```
or assert exactly one match:
```bash
MATCHES=$(ls dist/mcp-chain_*_linux_amd64.tar.gz 2>/dev/null | wc -l)
if [ "$MATCHES" -ne 1 ]; then
    echo "ERROR: expected exactly 1 linux_amd64 archive, got $MATCHES"
    exit 1
fi
```
Low severity — the happy path is correct, and `--clean` is set on the snapshot invocation.

### IN-06: `.golangci.yml` comments reference `gosimple` as a linter, but it's actually bundled in `staticcheck` in v2

**File:** `.golangci.yml:17`
**Issue:** The comment `# existing — v2 bundles gosimple + stylecheck` on the `staticcheck` entry is correct but the enabled list does not explicitly include `gosimple`, while `D-18` in CONTEXT.md spelled out `{govet, staticcheck, errcheck, gofmt, gosimple}`. The linter is effectively covered (staticcheck v2 runs the old gosimple checks under SA-series rule IDs), but a reader checking "is gosimple running?" may be confused.
**Fix:** Either (a) add `gosimple` explicitly under `enable:` — golangci-lint v2 will warn that it's redundant but the enabled set is documentation, not just a toggle; or (b) tighten the comment: `# staticcheck bundles former-gosimple (S1xxx) + former-stylecheck (ST1xxx); do NOT list gosimple separately in v2`. Option (b) is recommended — the current wording is accurate but reads as a reassurance, not a directive.

---

## Compliance Check Against Review Focus Areas

| # | Focus | Status | Evidence |
|---|-------|--------|----------|
| 1 | `.goreleaser.yaml` v2 correctness (version: 2, CGO=0, -trimpath, ldflags, 6-arch, zip override, sha256, changelog groups) | **PASS** | `.goreleaser.yaml:4,17-22,23-29,37-42,47-49,57-73`. All present and correctly shaped. |
| 2 | `release.yml` trigger, fetch-depth, permissions, action pins, secret wiring | **PASS** | `release.yml:5-8,10-11,21,30-36`. `goreleaser-action@v6`, `checkout@v4`, `setup-go@v5` all pinned. |
| 3 | `ci.yml` 3-OS matrix, fail-fast false, Go 1.25 pin, -race on linux+mac only, golangci-lint-action@v8 at v2.11.4, release dry-run | **PASS** | `ci.yml:66-76,21,72-76,24-26,94-152`. All compliant. |
| 4 | `.golangci.yml` v2 schema, formatters/linters separation | **PASS** | `.golangci.yml:11-18,46-48`. Correctly split; `gofmt` under `formatters:`. |
| 5 | New tests exercise intended paths + no races + env propagation | **PASS** with caveat | `TestConcurrentWaiters_AllSeeResolve` at `integration_test.go:570-645`; `TestStatus_PurgedMidPoll_Exit1` at 651-708. Both use `context.WithTimeout` + `sync.WaitGroup` + channels; both set `cmd.Env = env` on resolve/purge helpers per N-3. **Caveat: both tagged `//go:build integration` and NOT run in CI matrix — see WR-01.** |
| 6 | `fmt.Fprint*` errcheck cleanup correctness (no silent error swallowing) | **PASS** | 21 call sites across `main.go:52`, `table.go:51,53` (tabwriter → errors surface on `Flush` at `:61`), `list.go:31,47,50,56,60` (stderr writes — acceptable drop), `purge.go:43,60,76,79,82`, `resolve.go:37,55,63,66,69,72`, `status.go:41,60,66,69,72,75`. All discarded via `_, _ =`; all contexts are stderr/tabwriter where drop is correct. |
| 7 | `ResolveIn(out)` S1016 simplification correctness | **PASS** | `RegisterOut{ID string}` and `ResolveIn{ID string}` are structurally identical (`tools.go:19-24`); Go permits named-type conversion. Verified `server_test.go:81,105,107,122` all compile. |
| 8 | `internal/store/lock.go` govet shadow fix | **PASS** | `lock.go:36,68`: inner `err`→`lerr` on `fl.Lock()` and `fl.RLock()` — outer `err` is the named return, which the inner shadow was hiding. Rename is mechanical, no control-flow change. |
| 9 | Makefile targets (release-snapshot, ci-local) | **PASS** with polish suggestion | `Makefile:43-55`. `release-snapshot` invokes the exact GoReleaser incantation from CONTEXT.md §specifics; `ci-local` sequences `lint build size-check startup-check stdout-check test`. See IN-04 for minor polish. |
| 10 | Token-budget principle — any new model-facing strings? | **PASS** | No new tool-description strings in Phase 9. Existing tool prompts unchanged. Only new error strings (`"mcp-chain: ..."` lines in `list/purge/resolve/status.go`) are shell-facing, not model-facing. No token-budget regressions. |

---

_Reviewed: 2026-04-24T20:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
