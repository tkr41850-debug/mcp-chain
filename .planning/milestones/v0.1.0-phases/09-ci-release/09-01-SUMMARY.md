---
phase: 09-ci-release
plan: 01
subsystem: infra
tags: [goreleaser, github-actions, golangci-lint, cross-compile, release, ci, race-detector, errcheck, forbidigo, staticcheck, integration-tests]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: Makefile targets, Phase 1 budget gates (size ≤15 MB, startup ≤100 ms P95, stdout silence)
  - phase: 08-plugin-packaging
    provides: plugin.json, marketplace.json, .mcp.json manifest topology (release artifacts consumed by Claude Code plugin install)
provides:
  - Tag-driven GoReleaser v2 release workflow — `v*` push produces 6 archives + checksums.txt attached to the GitHub release
  - 3-OS CI matrix with -race on Linux+macOS (non-race on Windows) via actions/setup-go@v5
  - Release dry-run job (`release --snapshot --clean --skip=publish`) catching regressions per-PR
  - Static analysis gate: golangci-lint v2.11.4 pinned via golangci-lint-action@v8, enabling errcheck + gofmt + forbidigo + staticcheck + govet
  - Two QA-02 gap-fill tests: TestConcurrentWaiters_AllSeeResolve (N=4 waiters) + TestStatus_PurgedMidPoll_Exit1
  - Makefile `release-snapshot` + `ci-local` convenience targets
  - Codebase-wide errcheck cleanup (21 fmt.Fprint* call sites) + staticcheck S1016 cleanup + govet shadow fixes
affects: [10-docs-release, future phases consuming ldflags version string, future plugin versioning]

# Tech tracking
tech-stack:
  added:
    - GoReleaser v2.15.x (pin `~> v2` via goreleaser-action@v6)
    - goreleaser/goreleaser-action@v6
    - actions/setup-go@v5
    - golangci-lint@v2.11.4 via golangci-lint-action@v8
    - golangci-lint v2 config format (formatters/linters split)
  patterns:
    - "Version injection via ldflags: -X main.version={{.Version}} from GoReleaser to cmd/mcp-chain/main.go var version"
    - "Sub-process env propagation in integration tests: pass XDG_STATE_HOME + STORE_PATH via cmd.Env to every child invocation, not just serve"
    - "Deterministic concurrency tests: sync.WaitGroup + context.WithTimeout + chan to coordinate N goroutines with bounded-time assertions"
    - "errcheck idiom for tabwriter: _, _ = w.Fprintln; errors surface on Flush() below"
    - "v2 golangci config split: formatters{gofmt} top-level, linters.enable{errcheck,forbidigo,govet,staticcheck}"

key-files:
  created:
    - .goreleaser.yaml
    - .github/workflows/release.yml
    - .planning/phases/09-ci-release/09-01-SUMMARY.md
  modified:
    - .github/workflows/ci.yml
    - .golangci.yml
    - cmd/mcp-chain/main.go
    - internal/cli/format/table.go
    - internal/cli/list.go
    - internal/cli/purge.go
    - internal/cli/resolve.go
    - internal/cli/status.go
    - internal/cli/stubs.go
    - internal/cli/integration_test.go
    - internal/idgen/idgen_test.go
    - internal/mcpserver/server_test.go
    - internal/store/lock.go
    - Makefile

key-decisions:
  - "Kept go.mod at `go 1.25.0` rather than downgrading to 1.23 (SDK v1.5.0 requires 1.25+); CI job installs go-version: \"1.25\" — go.mod is the source of truth"
  - "Moved gofmt to top-level `formatters:` block in .golangci.yml (v2 rejects it under `linters:`)"
  - "Pinned goreleaser-action@v6 with `~> v2` version constraint (allows 2.x minor bumps but not a future v3)"
  - "fail-fast: false on CI matrix (one OS failure doesn't cancel the others)"
  - "Release workflow uses fetch-depth: 0 + permissions: contents: write (required for tag + release asset upload)"
  - "Sub-process integration tests propagate XDG_STATE_HOME via explicit cmd.Env on every spawn — not just serve"

patterns-established:
  - "Version injection pattern: `var version = \"dev\"` at package main, `-X main.version=...` in all release builds (dev and snapshot paths)"
  - "Static analysis tightening is in-scope for the task that enables the linter: every call site flagged by the newly-enabled rule is fixed as part of Task 5"
  - "Cross-process integration tests must export XDG_STATE_HOME to child processes AND inherit it into grandchild resolves/purges"

requirements-completed: [DIST-02, QA-01, QA-02, QA-03, QA-04]

# Metrics
duration: ~95min
completed: 2026-04-24
---

# Phase 9 Plan 01: CI Release, Cross-compile & Test Gates Summary

**Tag-driven GoReleaser v2 pipeline producing 6 archives + checksums for v* tags, 3-OS CI matrix with -race, golangci-lint v2.11.4 pinned with errcheck/forbidigo/staticcheck, and two QA-02 concurrency gap-fill tests**

## Performance

- **Duration:** ~95 min (includes goreleaser install ~30 min, snapshot build ~32 min for windows/arm64 TLS compile)
- **Started:** 2026-04-24T18:20:00Z (approx)
- **Completed:** 2026-04-24T19:55:00Z (approx)
- **Tasks:** 9 (T-01 no-op + T-02..T-08 implemented + T-09 verification)
- **Files modified:** 14

## Accomplishments

- **Release pipeline:** `v*` tag push now triggers GoReleaser v2 to cross-compile 6 arch combos (darwin/linux/windows × amd64/arm64) with CGO_ENABLED=0, emit sha256 checksums.txt, and attach all artifacts to the GitHub release. Local `make release-snapshot` verified end-to-end.
- **CI matrix expansion:** ci.yml now runs `go test -race -count=1` on ubuntu-latest + macos-latest and `go test -count=1` on windows-latest (non-race), plus a release-dry-run job running `goreleaser release --snapshot --clean --skip=publish` per-PR.
- **Static analysis gate (QA-04):** golangci-lint v2.11.4 pinned in both repo config and CI action; enabled linters = {govet, staticcheck, errcheck, gofmt, forbidigo}. Pre-existing violations across CLI, mcpserver, store, idgen packages all cleaned up as part of Task 5.
- **QA-02 gap-fill (QA-02 bullets 2 + 5):** TestConcurrentWaiters_AllSeeResolve (N=4 status pollers, verifies all see resolved state within one poll interval after register→resolve) and TestStatus_PurgedMidPoll_Exit1 (register→status polling→purge race→exit code 1 for unknown-id) both pass under -race -count=5.
- **Version injection verified:** extracted snapshot binary reports `mcp-chain 0.0.1-snapshot-none` (not `mcp-chain dev`); ldflags `-X main.version=...` path confirmed.
- **Phase 1 budget gates still green:** binary 7.41 MB ≤ 15 MB; startup P95 38.4 ms ≤ 100 ms; stdout 0 bytes on `--version`.

## Task Commits

Each task was committed atomically:

1. **T-01: Align Go toolchain to 1.23** — NO-OP (see Deviation N-1); no commit produced
2. **T-02: Create .goreleaser.yaml v2** — `a10508e` (feat)
3. **T-03: Add tag-driven release workflow** — `dde43f0` (feat)
4. **T-04: Expand ci.yml matrix + release dry-run + pinned lint** — `5bb65e7` (feat)
5. **T-05: Tighten .golangci.yml + clean pre-existing violations** — `0e34f77` (feat)
6. **T-06+T-07: Add TestConcurrentWaiters_AllSeeResolve + TestStatus_PurgedMidPoll_Exit1** — `764cfda` (test)
7. **T-08: Add Makefile release-snapshot + ci-local targets** — `fddb006` (feat)
8. **T-09: Full verification sweep** — no commit (verification only)

**Plan metadata commit:** (this SUMMARY + STATE updates — forthcoming)

## Files Created/Modified

### Created
- `.goreleaser.yaml` — 81-line v2 config: 6-arch build matrix, CGO_ENABLED=0, sha256 checksums, windows zip override, changelog groups, ldflags injection
- `.github/workflows/release.yml` — tag-driven workflow (`on: push: tags: ['v*']`), permissions: contents: write, fetch-depth: 0, goreleaser-action@v6 with `~> v2`

### Modified
- `.github/workflows/ci.yml` — 3-OS matrix (ubuntu/macos -race, windows non-race), fail-fast: false, added release-dry-run job, pinned `version: v2.11.4` on golangci-lint-action@v8
- `.golangci.yml` — v2 config: top-level `formatters: enable: [gofmt]`, `linters.enable: [errcheck, forbidigo, govet, staticcheck]`
- `cmd/mcp-chain/main.go` — line 52: `_, _ = fmt.Fprintln(os.Stdout, "mcp-chain "+version)` for errcheck
- `internal/cli/format/table.go` — prefixed tabwriter `Fprintln/Fprintf` with `_, _ =` (errors surface on Flush)
- `internal/cli/{list,purge,resolve,status}.go` — 21 fmt.Fprint* call sites prefixed with `_, _ =`
- `internal/cli/stubs.go` — gofmt whitespace
- `internal/cli/integration_test.go` — +147 lines: imports (`context`, `crypto/rand`, `encoding/hex`), TestConcurrentWaiters_AllSeeResolve, TestStatus_PurgedMidPoll_Exit1; fix N-3: `cmd.Env = env` propagated to resolve + purge sub-processes
- `internal/idgen/idgen_test.go` — gofmt whitespace
- `internal/mcpserver/server_test.go` — staticcheck S1016: replaced 4× `ResolveIn{ID: out.ID}` with `ResolveIn(out)`
- `internal/store/lock.go` — govet shadow fix: renamed inner `err` → `lerr` at lines 36, 68
- `Makefile` — appended `release-snapshot` + `ci-local` phony targets

## Decisions Made

- **Kept go.mod at `go 1.25.0`**: The SDK `modelcontextprotocol/go-sdk` v1.5.0 declares `go 1.25.0` in its own go.mod; downgrading would break the transitive toolchain. CI now uses `go-version: "1.25"`; release workflow uses `go-version-file: go.mod` (single source of truth).
- **gofmt as v2 formatter, not linter**: golangci-lint v2 reorganized the config schema — `gofmt` is now a formatter, not a linter. Adopted the new schema.
- **Action version pinning strategy**: `golangci-lint-action@v8` with explicit `version: v2.11.4`; `goreleaser-action@v6` with `~> v2` (allows 2.x minor bumps). Chosen because golangci-lint changes linter behavior between patches (reproducibility critical), while goreleaser 2.x is more behavior-stable.
- **fail-fast: false on matrix**: Prefer to see all OS failures on a single run vs. cancel-on-first-fail.
- **Integration-test env propagation**: Every sub-process invocation (serve, resolve --force, purge) in the new concurrency tests must carry `XDG_STATE_HOME` via `cmd.Env`. Plan sketch omitted this for resolve/purge; added per deviation N-3.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] N-1: go.mod Go directive stays at 1.25.0 (not downgraded to 1.23)**
- **Found during:** T-01
- **Issue:** Plan T-01 instructed "align go.mod to `go 1.23`", but the direct dependency `modelcontextprotocol/go-sdk` v1.5.0 declares `go 1.25.0` in its own go.mod — downgrading would cause `go mod tidy` to either fail or auto-upgrade, making the intended change a no-op and CI would install an inadequate toolchain.
- **Fix:** Left go.mod at `go 1.25.0`; CI job uses `go-version: "1.25"` explicitly; release workflow uses `go-version-file: go.mod` (authoritative).
- **Files modified:** none (go.mod unchanged); ci.yml and release.yml written to match
- **Verification:** `go build ./...` succeeds; `go test -race ./...` green across all 8 packages
- **Committed in:** `5bb65e7` (the ci.yml change; no separate T-01 commit since nothing to commit)

**2. [Rule 3 — Blocking] N-2: gofmt relocated to `formatters:` block in .golangci.yml**
- **Found during:** T-05
- **Issue:** Plan's paste-ready YAML put `gofmt` under `linters.enable:`. golangci-lint v2.11.4 rejects this with `can't load config: gofmt is a formatter, not a linter`.
- **Fix:** Split the v2 config into `formatters: enable: [gofmt]` at top level and `linters: enable: [errcheck, forbidigo, govet, staticcheck]`.
- **Files modified:** .golangci.yml
- **Verification:** `golangci-lint run --timeout=5m` exits 0 with 0 issues
- **Committed in:** `0e34f77`

**3. [Rule 1 — Bug] N-3: Integration sub-process resolve/purge missing XDG_STATE_HOME env**
- **Found during:** T-06 (TestConcurrentWaiters_AllSeeResolve initially failed with `unknown id: acid`)
- **Issue:** Plan's sub-process sketch set `cmd.Env = env` for the serve process but not the parent's `resolve --force` and `purge` helper invocations. Children without `XDG_STATE_HOME` read a different state file and couldn't see the registered id.
- **Fix:** Added `resolveCmd.Env = env` and `purgeCmd.Env = env` to the helper sub-process calls.
- **Files modified:** internal/cli/integration_test.go
- **Verification:** Both new tests pass `-race -count=5`; 10x flakiness sweep all green
- **Committed in:** `764cfda`

**4. [Rule 2 — Missing Critical] N-4: errcheck pre-existing violations fixed codebase-wide as part of T-05**
- **Found during:** T-05 (enabling errcheck surfaced 21 call sites)
- **Issue:** Enabling errcheck without cleaning existing violations would leave CI red.
- **Fix:** Added `_, _ =` prefix to all 21 flagged `fmt.Fprint*` call sites across `cmd/mcp-chain/main.go`, `internal/cli/{format/table,list,purge,resolve,status}.go`; fixed 2 gofmt whitespace violations in `internal/cli/stubs.go` + `internal/idgen/idgen_test.go`; fixed 2 govet shadow violations in `internal/store/lock.go`; fixed 4 staticcheck S1016 violations in `internal/mcpserver/server_test.go`.
- **Files modified:** see Files Created/Modified list
- **Verification:** `golangci-lint run --timeout=5m` exits 0; full `go test -race -count=1 ./...` green
- **Committed in:** `0e34f77`

---

**Total deviations:** 4 auto-fixed (1 bug, 1 missing critical, 2 blocking)
**Impact on plan:** All auto-fixes required for the plan's own acceptance criteria to turn green. No scope creep — each deviation is directly caused by work the plan itself mandated.

## Issues Encountered

- **goreleaser install took ~30 min** (transitive deps include slack-go, teams-notify, quill). Not mitigatable; consequence of the `go install` path. CI uses `goreleaser-action@v6` which uses prebuilt binaries, avoiding this.
- **`make release-snapshot` took ~32 min** for windows/arm64 TLS compile. Only affects local snapshot; CI release is per-tag so amortized.

## T-09 Verification Results

| Check | Result |
|-------|--------|
| `go vet ./...` | PASS |
| `golangci-lint run --timeout=5m` | PASS (0 issues) |
| `go test -race -count=1 -timeout=120s ./...` | PASS (all 8 packages) |
| `go test -tags=integration -race -count=1 -timeout=180s ./internal/cli/...` | PASS |
| `go test -race -count=5 -timeout=300s ./...` | PASS (5x race sweep, no flakes) |
| 10x flakiness sweep on both new tests | PASS |
| `make build` | PASS — ./mcp-chain 7766308 bytes |
| Phase 1 size gate (≤ 15 MB) | PASS (7.41 MB) |
| Phase 1 startup gate (≤ 100 ms P95) | PASS (38.4 ms) |
| Phase 1 stdout silence gate (0 bytes) | PASS |
| `make release-snapshot` — dist inventory | PASS (4 tar.gz + 2 zip + checksums.txt) |
| Binary `--version` ≠ "mcp-chain dev" | PASS (`mcp-chain 0.0.1-snapshot-none`) |
| `go list -f '{{ join .Deps "\n" }}' ./... \| grep '^net/http'` returns no matches | **FAIL (pre-existing, out of scope)** — SDK v1.5.0 files `mcp/event.go`, `mcp/streamable.go`, `mcp/sse.go`, `mcp/shared.go` import `net/http` unconditionally. Verified pre-existing by checking HEAD before Phase 9 commits. |
| `./scripts/smoke-chain-wait.sh` | **FAIL (pre-existing, out of scope)** — script invokes plain `go` without setting PATH; local-environment issue independent of Phase 9 work. Works when PATH is set. |

## Release Artifacts (from `make release-snapshot`)

```
dist/
├── checksums.txt                                              (sha256 of all archives)
├── mcp-chain_0.0.1-snapshot-none_darwin_amd64.tar.gz          3,146,057 bytes
├── mcp-chain_0.0.1-snapshot-none_darwin_arm64.tar.gz          2,918,971 bytes
├── mcp-chain_0.0.1-snapshot-none_linux_amd64.tar.gz           3,098,093 bytes
├── mcp-chain_0.0.1-snapshot-none_linux_arm64.tar.gz           2,815,812 bytes
├── mcp-chain_0.0.1-snapshot-none_windows_amd64.zip            3,189,290 bytes
└── mcp-chain_0.0.1-snapshot-none_windows_arm64.zip            2,856,986 bytes
```

Snapshot version string injected by ldflags: **`mcp-chain 0.0.1-snapshot-none`** (confirms ldflags pipeline is live end-to-end).

## Requirements Traceability

| ID | Description | Status | Evidence |
|----|-------------|--------|----------|
| DIST-02 | Cross-compile binaries attached to GitHub release | PASS | `.goreleaser.yaml` 6-arch build, `.github/workflows/release.yml` tag trigger, `make release-snapshot` produces 7-file dist tree |
| QA-01 | `go test -race` in CI | PASS | `ci.yml` matrix runs `go test -race -count=1 ./...` on ubuntu + macos |
| QA-02 | Integration-level concurrency coverage — registered→N waiters see resolve; status returns exit 1 on purged id | PASS | `TestConcurrentWaiters_AllSeeResolve` (4 waiters); `TestStatus_PurgedMidPoll_Exit1`; both pass -race -count=5 |
| QA-03 | `go test -race -count=1` passes on Linux+macOS | PASS | CI matrix covers both; verified locally |
| QA-04 | Static analysis gate blocks merge | PASS | `.golangci.yml` pinned to v2.11.4; `ci.yml` uses `golangci-lint-action@v8` with `version: v2.11.4` |

## User Setup Required

None — release pipeline triggers on any `v*` tag push by the repo owner; no external secrets or dashboard configuration needed (GitHub provides `GITHUB_TOKEN` automatically).

## Next Phase Readiness

- **Ready:** CI/release foundation is complete; Phase 10 (docs + 1.0 release) can ship a real tag at any time and the binaries will publish automatically.
- **Known pre-existing items outside Phase 9 scope** (to be addressed in future phases if product-relevant):
  - SDK v1.5.0 transitively imports `net/http` in `mcp/{event,streamable,sse,shared}.go` — this is an SDK regression from earlier versions that was stdio-only. If Phase 10 wants to reassert the "no net/http in deps" product principle, it needs an upstream SDK pin fix or a vendored subset. Pre-existing; not caused by Phase 9.
  - `scripts/smoke-chain-wait.sh` runs `go run` directly without a portable PATH setup — ran fine when PATH points at the Go SDK, fails in minimal environments. Pre-existing from Phase 8; candidate for Phase 10 doc/dev-env cleanup.

## Self-Check: PASSED

Verified:
- `.goreleaser.yaml` exists
- `.github/workflows/release.yml` exists
- `.github/workflows/ci.yml` exists (modified)
- `.golangci.yml` exists (modified)
- `Makefile` exists (modified, contains `release-snapshot:` + `ci-local:`)
- `internal/cli/integration_test.go` contains `TestConcurrentWaiters_AllSeeResolve` and `TestStatus_PurgedMidPoll_Exit1`
- Commits present: a10508e, dde43f0, 5bb65e7, 0e34f77, 764cfda, fddb006
- `dist/checksums.txt` + 6 archives produced

---
*Phase: 09-ci-release*
*Completed: 2026-04-24*
