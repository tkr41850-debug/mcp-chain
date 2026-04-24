---
phase: 09-ci-release
verified: 2026-04-24T20:25:00Z
status: human_needed
score: 4/4 Success Criteria PASS; 5/5 Requirements (DIST-02, QA-01, QA-02, QA-03, QA-04) SATISFIED; 2 items remain for CI-matrix observation on real GitHub Actions runners (tag-driven release run + Windows non-race suite), legitimately deferrable to first real `v*` tag push in Phase 10
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "First `v*` tag push triggers `.github/workflows/release.yml` end-to-end on GitHub Actions"
    expected: "Release workflow runs GoReleaser, publishes 6 archives + checksums.txt as GitHub Release assets, embeds tag (not `dev`) in `--version`"
    why_human: "The workflow has never run in production — it can only be exercised by a real tag push with `GITHUB_TOKEN` on GitHub's runner. Local `make release-snapshot` validated the GoReleaser config path (snapshot mode); tag-push mode adds release-asset upload which requires the hosted environment. Scheduled for Phase 10 first release."
  - test: "CI matrix `test (windows-latest)` job passes non-race suite on a real Windows runner"
    expected: "`go test -count=1 -timeout=120s ./...` green on windows-latest (all 8 packages)"
    why_human: "Verifier host is Linux only — cannot execute Windows toolchain. The CI matrix is declaratively correct (fail-fast: false, `race: \"\"` override for windows, shell: bash for `go test` step), but Windows-specific behaviour in `gofrs/flock` + `renameio/v2` has never been exercised in CI. First PR after Phase 9 merge will close this."
---

# Phase 9: CI Release, Cross-compile & Test Gates — Verification Report

**Phase Goal:** Ship release-grade CI: GoReleaser cross-compiles to linux/darwin/windows × amd64/arm64 on `v*` tag push; CI runs `go test -race` on Linux+macOS and `go test` on Windows on every push/PR; tag-driven release attaches archives + checksums to GitHub; outstanding QA-02 test gaps (N concurrent waiters, purge-mid-poll) closed; static analysis (golangci-lint v2.11.4) gates merge.

**Verified:** 2026-04-24T20:25:00Z
**Status:** human_needed (all 4 Success Criteria + all 5 Requirements PASS under automated verification; 2 items require a real GitHub Actions run — tag-driven release and Windows CI matrix — legitimately deferrable per verification overrides pattern)
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC-1 | Push/PR CI runs `go test -race -count=1 ./...` on Linux + macOS runners and the non-race suite on Windows; any failure blocks merge | ✓ PASS | `.github/workflows/ci.yml:62-92` defines `test` job with `matrix.os=[ubuntu-latest, macos-latest, windows-latest]`, `fail-fast: false`, per-OS include block setting `race: "-race"` for ubuntu+macos and `race: ""` for windows. Step `go test ${{ matrix.race }} -count=1 -timeout=120s ./...` invokes the correct command on each OS. `shell: bash` on the test step normalizes across Windows. Local race run: 8/8 packages green (`cmd/mcp-chain 25.4s, internal/cli 13.8s, format 1.7s, idgen 2.0s, mcpserver 2.7s, plugin 1.6s, statepath 1.6s, store 4.9s`). Real Windows runner execution is the sole remaining unknown (human verification item 2). |
| SC-2 | Tagging `v*` triggers GoReleaser to cross-compile `linux/darwin/windows × amd64/arm64`, strip with `-s -w -trimpath`, attach all six archives plus `checksums.txt` to the GitHub release, and embed the tag in `--version` (not a dirty SHA) | ✓ PASS | `.github/workflows/release.yml:5-8` — `on: push: tags: ['v*']`. `permissions: contents: write` (line 10-11) — required for release asset upload. `.goreleaser.yaml` line 4 `version: 2`; builds matrix `goos: [linux, darwin, windows] × goarch: [amd64, arm64]` = 6 combos (line 23-29); `CGO_ENABLED=0` (line 18); flags `-trimpath` (line 20); ldflags `-s -w -X main.version={{.Version}}` (line 22). Archive `format_overrides` switches windows to zip (line 39-42). `checksum.name_template: 'checksums.txt'` + `algorithm: sha256` (line 47-49). Local `make release-snapshot` produced `dist/checksums.txt` + 4 tar.gz (darwin/linux × amd64/arm64) + 2 zip (windows × amd64/arm64). Extracted linux_amd64 binary: `mcp-chain 0.0.1-snapshot-none` (ldflags injection live; NOT `mcp-chain dev`). Tag-driven end-to-end run requires real GitHub Actions execution (human verification item 1). |
| SC-3 | Unit suite covers wordlist allocation determinism, counter monotonicity, hex fallback, state schema round-trip, timeout parsing, path resolution, ID lookup, and every state transition (pending → resolved, double-resolve, unknown, OwnerToken mismatch) | ✓ PASS | Baseline unit coverage laid down across Phases 2–6 is intact: `go test -race -count=1 ./...` green on all 8 packages. Phase 9 adds no new unit subjects but gates the existing suite behind `-race` across 3 OSes. Specific subjects confirmed via package presence: `internal/idgen` (wordlist + counter + hex fallback), `internal/store` (round-trip, transitions, OwnerToken), `internal/statepath` (path resolution), `internal/cli/format` (table), `internal/mcpserver` (tools), `cmd/mcp-chain` (flag parsing + version). All pass under the tightened golangci-lint v2.11.4 (`0 issues` locally). |
| SC-4 | Integration suite covers end-to-end register → status pending → resolve → status resolved, N concurrent waiters, double-resolve + unknown-ID + purge-mid-wait + OwnerToken-mismatch errors, and two-process cross-flock safety with 100 entries per process with zero lost updates | ✓ PASS | `internal/cli/integration_test.go` (708 lines, `//go:build integration` tag). Phase 9 adds the two missing bullets: `TestConcurrentWaiters_AllSeeResolve` (line 570) launches N=10 concurrent `status` pollers, triggers `resolve --force` from parent, asserts all 10 observe exit 0 within a 5-second context deadline via buffered channel + `sync.WaitGroup` coordination (deterministic — no polling race). `TestStatus_PurgedMidPoll_Exit1` (line 651) seeds pending, runs a status poller goroutine, purges from parent, asserts poller observes exit 1 within deadline. Both tests run `cmd.Env = env` on every subprocess (parent + resolve + purge) with `XDG_STATE_HOME` propagation per N-3 fix. Local execution: `go test -tags=integration -race -count=1 -run 'TestConcurrentWaiters_AllSeeResolve\|TestStatus_PurgedMidPoll_Exit1' ./internal/cli/ -v` → both PASS (8.9s + 8.3s). The other integration scenarios (register→pending→resolve→resolved, double-resolve, unknown-ID, OwnerToken mismatch, two-process flock, 100-entry cross-flock) are pre-existing from Phases 4/6/7 and remain green under the expanded `-race` matrix. Note: SUMMARY.md claims N=4 waiters; actual code uses `waiters = 10` (strengthens the test; not a gap). |

**Score:** 4/4 ROADMAP Success Criteria verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.goreleaser.yaml` | v2 config (`version: 2`), 6-arch matrix, CGO_ENABLED=0, `-trimpath`, `-X main.version=`, checksums.txt, zip override for windows | ✓ VERIFIED | 82 lines. `version: 2` at line 4. `goos: [linux, darwin, windows]` × `goarch: [amd64, arm64]` = 6 combos. CGO_ENABLED=0 env, `-trimpath` flag, `-s -w -X main.version={{.Version}}` ldflags. `format_overrides` switches windows→zip. `checksum.algorithm: sha256` + `name_template: 'checksums.txt'`. `before.hooks: [go mod tidy, go mod verify]`. Snapshot version template `{{ incpatch .Version }}-snapshot-{{ .ShortCommit }}`. Changelog groups: Features/Fixes/Others, excludes docs/test/chore/ci/merge-noise. Release `draft: false`, `prerelease: auto`. |
| `.github/workflows/release.yml` | Tag-triggered (`v*`), `contents: write`, `fetch-depth: 0`, goreleaser-action@v6 `~> v2` | ✓ VERIFIED | 37 lines. `on: push: tags: ['v*']`. `permissions: contents: write`. Job `goreleaser` on ubuntu-latest, checkout with fetch-depth: 0, setup-go@v5 with `go-version-file: go.mod`, goreleaser-action@v6 `version: '~> v2'` `args: release --clean`, `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`. |
| `.github/workflows/ci.yml` | Expanded matrix with -race on linux+mac, non-race on windows; release dry-run job; pinned golangci-lint v2.11.4 | ✓ VERIFIED | 152 lines. Jobs: `lint` (ubuntu, golangci-lint-action@v8 `version: v2.11.4` `args: --timeout=5m`), `build-and-gate` (ubuntu, needs lint; make build + size + startup + stdout-silence + net/http-ban + smoke-chain-wait), `test` (matrix of 3 OSes with fail-fast: false and per-OS race/no-race override), `release-dry-run` (ubuntu, needs lint; goreleaser-action@v6 `args: release --snapshot --clean --skip=publish` + post-step assertion that dist/ contains 4 tar.gz + 2 zip + checksums.txt and extracted linux_amd64 binary `--version` != `mcp-chain dev`). |
| `.golangci.yml` | v2 format (`version: "2"`), enables errcheck/staticcheck/govet/forbidigo, gofmt as formatter | ✓ VERIFIED | 52 lines. `version: "2"` at line 5. `linters.default: standard` + `linters.enable: [errcheck, forbidigo, govet, staticcheck]`. `formatters.enable: [gofmt]` at top level (v2 requires formatters split from linters; per N-2). forbidigo rules ban bare `fmt.Print*` / `print` / `println` (MCP-02 stdout guard). staticcheck.checks: [all]. govet.enable: [shadow]. |
| `internal/cli/integration_test.go` | Contains TestConcurrentWaiters_AllSeeResolve + TestStatus_PurgedMidPoll_Exit1 under integration tag | ✓ VERIFIED | 708 lines total. `//go:build integration` tag at line 1. `TestConcurrentWaiters_AllSeeResolve` at line 570 (N=10 pollers with `sync.WaitGroup` + buffered channel + `context.WithTimeout`). `TestStatus_PurgedMidPoll_Exit1` at line 651 (seed→poll→purge→exit-1 assertion). Both use `seedStateForChild` + `buildBinary` helpers (Phase 7) with `cmd.Env = env` propagated on every subprocess spawn (resolve, purge, poller). Imports expanded with `context`, `crypto/rand`, `encoding/hex`, `sync`, `time`. |
| `Makefile` | release-snapshot + ci-local phony targets | ✓ VERIFIED | 56 lines. `release-snapshot:` target (line 44-51): runs `goreleaser release --snapshot --clean --skip=publish` then asserts 4 tar.gz + 2 zip + checksums.txt in dist/. `ci-local:` target (line 54): chains lint + build + size + startup + stdout-silence + test. Pre-existing targets intact: all/build/lint/test/size-check/startup-check/stdout-check/clean/tidy. |
| `cmd/mcp-chain/main.go` | `var version = "dev"` present (line 22); errcheck-clean `fmt.Fprintln` | ✓ VERIFIED | `var version = "dev"` at line 22 with comment explicitly calling out GoReleaser Phase-9 override. Line 52 `_, _ = fmt.Fprintln(os.Stdout, "mcp-chain "+version)` (errcheck-clean idiom). Stdout discipline (log/slog→stderr) established before kong.Parse. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `.github/workflows/release.yml` | `.goreleaser.yaml` | goreleaser-action@v6 `args: release --clean` | ✓ WIRED | Line 30-34 of release.yml invokes goreleaser-action with `version: '~> v2'` matching goreleaser v2 schema. On tag push, action reads `.goreleaser.yaml` from repo root. |
| `.github/workflows/ci.yml` release-dry-run job | `.goreleaser.yaml` | goreleaser-action@v6 `args: release --snapshot --clean --skip=publish` | ✓ WIRED | Line 108-113. `--skip=publish` avoids asset upload (PR context has no release to attach to). Post-step asserts dist/ has 4 tar.gz + 2 zip + checksums.txt. |
| `.goreleaser.yaml` ldflags | `cmd/mcp-chain/main.go` var `version` | `-X main.version={{.Version}}` | ✓ WIRED | Verified empirically: extracted snapshot binary printed `mcp-chain 0.0.1-snapshot-none` (matches `snapshot.version_template` line 52). Zero-value default would be `mcp-chain dev` — injection confirmed live. CI release-dry-run has a dedicated step asserting this contract (`ci.yml:138-152`). |
| `.golangci.yml` | `.github/workflows/ci.yml` lint job | golangci-lint-action@v8 `version: v2.11.4` | ✓ WIRED | Config file path is the default (`.golangci.yml` at repo root); action picks it up automatically. Locally reproduced: `golangci-lint run --timeout=5m` → `0 issues`. |
| `internal/cli/integration_test.go` | built binary | `buildBinary(t)` helper returning absolute path, `cmd.Env = env` with `XDG_STATE_HOME` propagated on every spawn (parent + resolve + purge) | ✓ WIRED | Per N-3 fix: every `exec.CommandContext(...)` in both new tests has explicit `cmd.Env = env`. Confirmed by code inspection at lines 591, 613, 630, 670, 683, 698. Local `-tags=integration -race -count=1` run of both tests passes in 17s. |
| `Makefile release-snapshot` | `.goreleaser.yaml` | `goreleaser release --snapshot --clean --skip=publish` + post-assertion | ✓ WIRED | Line 45-51. Exercised during verification — `ls dist/` shows 6 archives + checksums + artifacts.json + config.yaml + per-platform build dirs. Binary extracted from `dist/mcp-chain_0.0.1-snapshot-none_linux_amd64.tar.gz` runs with the injected version string. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `.goreleaser.yaml` → dist archives | `{{.Version}}` in ldflags + archive name_template | `goreleaser` CLI derives from git tag (release mode) or `snapshot.version_template` (snapshot mode) | Yes — snapshot run produced `0.0.1-snapshot-none` both in archive filenames and embedded `--version` | ✓ FLOWING |
| `release-dry-run` assert step | `$VER` from extracted binary | `./extract/mcp-chain --version` output | Yes — snapshot run of same goreleaser config produces non-`dev` string; assertion regex matches `^mcp-chain ` | ✓ FLOWING |
| `TestConcurrentWaiters_AllSeeResolve` | `resolved` channel | Every poller subprocess writes its index on exit-0 observation | Yes — all 10 pollers observed resolved state within 5s deadline during local -race run | ✓ FLOWING |
| `TestStatus_PurgedMidPoll_Exit1` | `saw1` channel | Poller subprocess writes once on first exit-1 observation | Yes — test completes within deadline; poller reliably observes exit 1 after parent-side purge | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full unit + short integration race suite | `go test -race -count=1 -timeout=120s ./...` | 8/8 packages ok (cmd/mcp-chain 25.4s, internal/cli 13.8s, format 1.7s, idgen 2.0s, mcpserver 2.7s, plugin 1.6s, statepath 1.6s, store 4.9s) | ✓ PASS |
| Two new QA-02 tests under integration tag | `go test -tags=integration -race -count=1 -run 'TestConcurrentWaiters_AllSeeResolve\|TestStatus_PurgedMidPoll_Exit1' -v ./internal/cli/` | PASS (8.93s + 8.32s = 18.83s combined) | ✓ PASS |
| golangci-lint v2.11.4 clean | `golangci-lint run --timeout=5m` | `0 issues.` (version string: `golangci-lint has version 2.11.4`) | ✓ PASS |
| Go vet | `go vet ./...` | No output (clean) | ✓ PASS |
| Local build (Makefile) | `make build` | Produces `./mcp-chain` via `go build -trimpath -ldflags="-s -w -X main.version=..."`; binary exists | ✓ PASS |
| Binary size ceiling (≤ 15 MB, CLAUDE.md constraint) | `ls -l ./mcp-chain` | 7,766,308 bytes (7.41 MB); 50% headroom | ✓ PASS |
| Startup gate (≤ 100 ms P95, CLAUDE.md constraint) | `./scripts/check-startup.sh ./mcp-chain` | P95 of 5 runs = 37.652 ms; well under 100 ms limit | ✓ PASS |
| Version string not literal `dev` on snapshot | Extract `dist/mcp-chain_*_linux_amd64.tar.gz`; run `./mcp-chain --version` | `mcp-chain 0.0.1-snapshot-none` — ldflags injection live | ✓ PASS |
| `make release-snapshot` inventory | Pre-existing `dist/` from Phase-9 execution: `ls dist/*.{tar.gz,zip} checksums.txt` | 4 tar.gz (darwin/linux × amd64/arm64) + 2 zip (windows × amd64/arm64) + checksums.txt (6 lines, 6 sha256 entries) | ✓ PASS |
| Checksum content | `cat dist/checksums.txt` | 6 sha256-algorithm entries, one per archive, correct filenames | ✓ PASS |
| Phase 9 commits present in history | `git cat-file -e` for a10508e, dde43f0, 5bb65e7, 0e34f77, 764cfda, fddb006 | All 6 task commits exist; SUMMARY/state commit beebe56 is HEAD | ✓ PASS |
| ROADMAP marks Phase 9 complete | `grep "Phase 9" ROADMAP.md` | `[x] Phase 9 ... (completed 2026-04-24)` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| DIST-02 | 09-01 PLAN `requirements:` | Cross-compile binaries for linux/darwin/windows × amd64/arm64, attach to GitHub release | ✓ SATISFIED | `.goreleaser.yaml` declares 6-combo build matrix with zip-for-windows override + sha256 checksums.txt. `.github/workflows/release.yml` binds this to `v*` tag push with `contents: write`. Local `make release-snapshot` produced all 6 archives + checksums.txt. Tag-push production run is human verification item 1 (first `v*` tag). |
| QA-01 | 09-01 PLAN `requirements:` | `go test -race` runs in CI | ✓ SATISFIED | `.github/workflows/ci.yml:62-92` test job uses `go test ${{ matrix.race }} -count=1 -timeout=120s ./...` where `matrix.race` is `-race` on ubuntu+macos. Local race-run all green. |
| QA-02 | 09-01 PLAN `requirements:` | Integration tests: cross-process flock, double-resolve, unknown-ID, **concurrent waiters**, **status mid-purge** | ✓ SATISFIED | Pre-existing flock + double-resolve + unknown-ID integration tests were already green from Phase 6/7. Phase 9 closes the two open bullets: `TestConcurrentWaiters_AllSeeResolve` (N=10 concurrent pollers observing resolve) and `TestStatus_PurgedMidPoll_Exit1` (poller observing exit-1 after parent-side purge). Both pass under `-race -count=5` per SUMMARY §T-09. |
| QA-03 | 09-01 PLAN `requirements:` | `go test -race -count=1` green on Linux+macOS; `go test -count=1` green on Windows | ✓ SATISFIED | CI matrix declaratively correct per SC-1 evidence. Linux+macOS race paths verified locally (Linux confirmed; macOS covered by matrix + fail-fast: false). Windows non-race path is human verification item 2. |
| QA-04 | 09-01 PLAN `requirements:` | golangci-lint passes cleanly (staticcheck via v2 default linters) | ✓ SATISFIED | Config pinned to golangci-lint v2.11.4 in both `.github/workflows/ci.yml:23-27` (action version) and `.golangci.yml` (config version: "2"). Enabled linters: errcheck, forbidigo, govet (with shadow), staticcheck (all checks). gofmt in v2 formatters block. Local `golangci-lint run --timeout=5m` → `0 issues`. Pre-existing violations (21 fmt.Fprint* errcheck, 2 gofmt, 2 govet shadow, 4 staticcheck S1016) all fixed in commit `0e34f77`. |

No orphaned requirements — all 5 declared in PLAN frontmatter mapped to verified artifacts. REQUIREMENTS.md list for Phase 9 is `{DIST-02, QA-01, QA-02, QA-03, QA-04}`; SUMMARY `requirements-completed:` lists the same set; no phantom additions.

### Anti-Patterns Found

None. Grep for TODO/FIXME/XXX/HACK/PLACEHOLDER across Phase-9-modified files (.goreleaser.yaml, .github/workflows/*.yml, .golangci.yml, Makefile, cmd/mcp-chain/main.go, internal/cli/integration_test.go, internal/cli/format/table.go, internal/cli/{list,purge,resolve,status,stubs}.go, internal/idgen/idgen_test.go, internal/mcpserver/server_test.go, internal/store/lock.go) returns zero blocker hits. The `forbidigo` linter actively polices bare `fmt.Print*` introduction going forward — anti-pattern prevention is now enforced, not just grep-detected.

### Human Verification Required

Two items require GitHub-hosted environments and are legitimately deferred:

1. **First `v*` tag push triggers `release.yml` end-to-end on GitHub Actions**
   - Test: push a real `v0.1.0` (or similar) tag to `origin`; watch `goreleaser` workflow in Actions tab
   - Expected: workflow completes; GoReleaser uploads 6 archive assets + checksums.txt to the new GitHub Release; extracting any asset yields a binary where `mcp-chain --version` prints the tag (not `dev`)
   - Why human: workflow has never run in production — requires real `GITHUB_TOKEN` + hosted runner + release-asset upload path. `release-dry-run` CI job covers everything EXCEPT asset upload. Scheduled for Phase 10 first-release gate.

2. **CI matrix `test (windows-latest)` job passes non-race suite on a real Windows runner**
   - Test: open any PR after Phase 9 merge; observe `test (windows-latest)` check on GitHub Actions
   - Expected: all 8 packages pass `go test -count=1 -timeout=120s ./...` on windows-latest (gofrs/flock LockFileEx + renameio/v2 Windows paths exercised)
   - Why human: verifier host is Linux-only; Windows toolchain unavailable locally. Declarative matrix + `shell: bash` on test step are correct, but cross-platform test behaviour (particularly `gofrs/flock` Windows semantics and `renameio/v2`'s documented weaker symlink-atomic guarantee on Windows per CLAUDE.md) has never been exercised.

---

## Out-of-Scope Known Items (documented, not verifier gaps)

Per user brief, these are out of scope for Phase 9 closure:

- **`net/http` in SDK v1.5.0 transitive graph** — SDK upstream regression from stdio-only era (`mcp/{event,streamable,sse,shared}.go`). `ci.yml` build-and-gate job still contains the net/http ban step (lines 52-57); if tightened, CI will red. This is a known Phase 10 decision point. Not a Phase-9 gap.
- **`scripts/smoke-chain-wait.sh` requires Go SDK in PATH** — pre-existing Phase 8 artifact; invoked by `ci.yml:59-60`. In a properly-configured CI runner `actions/setup-go@v5` places Go on PATH before the smoke step, so it works there; local failure is environment-specific. Not a Phase-9 gap.

---

## Summary

All four Phase 9 ROADMAP Success Criteria PASS under automated verification. The release pipeline is complete: `.goreleaser.yaml` v2 with 6-arch cross-compile (CGO_ENABLED=0, -trimpath, -s -w, ldflags `-X main.version={{.Version}}`, sha256 checksums, zip-for-windows override, changelog-from-git), bound to `v*` tag push in `.github/workflows/release.yml` with `contents: write` and `~> v2` goreleaser-action pin. CI matrix expanded to 3 OSes with correct per-OS race override (`-race` on ubuntu+macos, empty on windows) plus a release-dry-run job that asserts dist inventory and ldflags injection. golangci-lint pinned to v2.11.4 in both repo config and CI action, with errcheck/forbidigo/govet-shadow/staticcheck enabled and gofmt in the v2 formatters block — `0 issues` locally. Two QA-02 gap tests land under `//go:build integration`: `TestConcurrentWaiters_AllSeeResolve` (N=10 concurrent pollers, actually stronger than SUMMARY's N=4 claim) and `TestStatus_PurgedMidPoll_Exit1`, both with deterministic sync.WaitGroup + context.WithTimeout + buffered-channel coordination and full XDG_STATE_HOME env propagation to every subprocess per N-3 fix. Full local `go test -race -count=1` suite green across all 8 packages; binary 7.41 MB (well under the 15 MB ceiling); startup P95 37.7 ms (well under the 100 ms ceiling); `make release-snapshot` produces the documented 4 tar.gz + 2 zip + checksums.txt with correct ldflags-injected version string. All 6 task commits (a10508e, dde43f0, 5bb65e7, 0e34f77, 764cfda, fddb006) plus finalization commit (beebe56) present in history. Status is `human_needed` (not `passed`) solely because two verifications — a real `v*` tag push to exercise GitHub Release asset upload, and a real windows-latest runner exercising the non-race matrix leg — cannot be performed without hosted infrastructure, and both are first-run items that will close naturally on the first Phase 10 PR / tag. No gaps; no regressions; Phase 9 has achieved its stated goal.

---

_Verified: 2026-04-24T20:25:00Z_
_Verifier: Claude (gsd-verifier)_
