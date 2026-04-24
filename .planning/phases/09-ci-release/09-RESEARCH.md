# Phase 9: CI Release, Cross-compile & Test Gates — Research

**Researched:** 2026-04-24
**Domain:** Go release tooling (GoReleaser v2) + GitHub Actions cross-platform test matrix + static analysis
**Confidence:** HIGH (existing CI, test surface, and stack pins are all directly inspectable; GoReleaser v2 docs verified against current site)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**CI Matrix (D-01..D-04):**
- D-01: Three-OS matrix `ubuntu-latest`, `macos-latest`, `windows-latest`; single Go version `"1.23"` pinned across all runners (matches `go.mod` minimum). No Go version matrix.
- D-02: Linux + macOS run `go test -race -count=1 ./...`; Windows runs `go test -count=1 ./...` (no `-race`) per QA-03.
- D-03: Linux-only gates (size, startup, stdout silence, `net/http` ban, shell lint) stay on Linux — not duplicated on macOS/Windows.
- D-04: `lint` job stays a separate upstream step that blocks matrix `test` jobs. `staticcheck` folds into `golangci-lint` (it's already bundled in v2.x), no separate `staticcheck` job.

**GoReleaser + Release (D-05..D-10):**
- D-05: New `.goreleaser.yaml` (v2 — `version: 2` header). Matrix `goos: [linux, darwin, windows]` × `goarch: [amd64, arm64]`. `flags: [-trimpath]`, `ldflags: -s -w -X main.version={{.Version}}` — mirrors existing `Makefile` `GO_LDFLAGS`.
- D-06: Archives — `tar.gz` for linux/darwin, `zip` for windows. Include `LICENSE` (may not exist yet — soft), `README.md` (Phase 10 writes it — soft).
- D-07: `checksums.txt` at release root, SHA-256 (GoReleaser default).
- D-08: New `.github/workflows/release.yml`, trigger `on: push: tags: ['v*']`. Checkout `fetch-depth: 0`, `setup-go@v5` Go `"1.23"`, `goreleaser/goreleaser-action@v6` with `args: release --clean`. `GITHUB_TOKEN` from `secrets`.
- D-09: No code signing, no notarization, no SBOM. Deferred.
- D-10: `CGO_ENABLED=0` set at matrix level in `.goreleaser.yaml` (enforced at release time, not assumed).

**Version embedding (D-11..D-12):**
- D-11: `cmd/mcp-chain/main.go` already has `var version = "dev"` (line 22, verified). GoReleaser's `-X main.version={{.Version}}` injects the tag so `v0.1.0` → `mcp-chain v0.1.0`.
- D-12: **Release dry-run smoke** in CI — separate job, Linux-only, `goreleaser release --snapshot --clean --skip=publish`. Asserts 6 archives + `checksums.txt` land under `dist/`.

**Test coverage closeout (D-13..D-17):**
- D-13: Plan's first task = **gap audit** against QA-01/QA-02. Hypothesis: coverage near-complete. Stdlib tests only; no testify beyond `require`.
- D-14/D-15: Canonical QA-01 / QA-02 checklists enumerated in CONTEXT.md (see Test Coverage Gap Audit section below for mapping).
- D-16: Subprocess integration tests use `exec.Command` against test-built binary. No network transports.
- D-17: Flakiness budget zero — flaky tests get redesigned, not quarantined.

**Static analysis (D-18..D-19):**
- D-18: `.golangci.yml` single source of truth. Enabled: `govet`, `staticcheck`, `errcheck`, `gofmt`, `gosimple`. Pin `golangci-lint-action` to a specific v2.x tag before release.
- D-19: No separate staticcheck job — subsumed in golangci-lint v2.x (staticcheck is bundled).

**CI workflow structure (D-20..D-22):**
- D-20: `ci.yml` matrix restructure: `lint` (linux-only, unchanged) → `build-and-gate` (linux-only, preserved) + `test` (3-OS matrix). All three must pass for green.
- D-21: `actions/setup-go@v5` with `cache: true` on every job (module + build cache via free GitHub cache, Go 1.21+).
- D-22: No path filters — every PR runs full CI.

**Release cadence (D-23..D-24):**
- D-23: Tag-driven manual releases. `git tag v0.1.0 && git push --tags`.
- D-24: Semver. First release `v0.1.0`. Changelog auto-generated from conventional commits.

### Claude's Discretion

- Exact `.goreleaser.yaml` layout details (comments, `name_template`, changelog filters) — use v2 defaults.
- Whether to add macOS amd64 runner (`macos-13`) for amd64 coverage symmetry vs relying solely on `macos-latest` (arm64 Apple Silicon).
- Whether the dry-run is a step in `release.yml` or a separate workflow — prefer minimal file count.
- Minor `Makefile` refactors (e.g., `make release-snapshot` target) if useful to keep local dev symmetric with CI.

### Deferred Ideas (OUT OF SCOPE)

- Homebrew tap / deb / rpm packaging — GoReleaser supports but not required.
- Code signing + notarization — hobby-scale audience accepts macOS Gatekeeper warnings.
- SBOM generation.
- Continuous benchmarking in CI (size/startup gates are proxies).
- Nightly builds / release-please automation.
- Windows `-race` support — revisit when Windows promotes to primary platform.
- README content / dogfooding (Phase 10).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description (from REQUIREMENTS.md) | Research Support |
|----|------------------------------------|------------------|
| DIST-02 | GoReleaser CI: push/PR runs `go test -race` and `go build`; tag `v*` cross-compiles `linux/darwin/windows × amd64/arm64`, generates checksums, attaches to release | GoReleaser v2 Config Sketch + Release Workflow Sketch below — drop-in YAML that implements this |
| QA-01 | Unit tests for all core logic (wordlist, counter, hex fallback, schema, timeout, path, lookup, state transitions, OwnerToken) | Test Coverage Gap Audit — 8/8 bullets COVERED (with specific test names) |
| QA-02 | Integration tests — E2E flow, N concurrent waiters, double-resolve, unknown-ID, purge-mid-wait, OwnerToken mismatch, cross-process flock 100 entries each | Test Coverage Gap Audit — 5/7 COVERED; 2 GAPS (N concurrent waiters, purge-mid-wait) with proposed test signatures |
| QA-03 | `go test -race ./...` gate in CI; Linux+macOS race; Windows no-race; failing blocks merge | CI Matrix Expansion Sketch — exact YAML showing race conditional via matrix include |
| QA-04 | CI lint gate (`go vet` + `staticcheck` or equivalent); non-zero blocks merge | Static Analysis Wiring section — existing `.golangci.yml` already enables `staticcheck`+`govet`+`forbidigo`; add `errcheck`+`gofmt`+`gosimple`, pin lint-action version |
</phase_requirements>

## Research Summary

- **Coverage is near-complete.** Inventory of 14 `*_test.go` files shows QA-01 is fully satisfied (8/8 bullets) and QA-02 is 5/7 covered. The only two genuine gaps are **N concurrent waiters see resolve** (the canonical mcp-chain use case has no test) and **purge-mid-wait error** (currently only covered by the Phase 8 bash smoke harness, not the Go race-gated suite). Both are small, isolated additions — not bulk authoring.
- **Version embedding already works.** `cmd/mcp-chain/main.go:22` has `var version = "dev"`. GoReleaser's default `{{.Version}}` template injection via `-X main.version=...` drops in with zero code change. `Makefile` `GO_LDFLAGS` (line 5) uses the identical flags and injection, so release binaries and `make build` outputs are flag-compatible (a reproducible-build invariant the planner should preserve).
- **CI expansion is additive, not rewriting.** Existing `.github/workflows/ci.yml` has `lint` + `build-and-gate` jobs. Phase 9 adds a third `test` job with an OS matrix and a fourth `release-dry-run` job — no regression risk to phase 1–8 gates.
- **macOS runner landmine:** `macos-latest` is now arm64 Apple Silicon (since GitHub's runner rollout in 2024-2025). If the planner wants amd64 macOS coverage, it must explicitly add `macos-13` to the matrix. For mcp-chain (pure Go, stdlib-only system calls), arm64-only macOS testing is acceptable — but the decision should be explicit, not accidental.
- **GoReleaser v2 archives syntax changed.** The older `format:` (singular) + `format_overrides:` with `format:` nested key is soft-deprecated; the current v2 idiom is `formats: [tar.gz]` (plural list) and `format_overrides: [{goos: windows, formats: [zip]}]`. Both still parse on v2.15+, but the sketch below uses the current form.

**Primary recommendation:** The planner should produce three drop-in files (`.goreleaser.yaml`, `release.yml`, expanded `ci.yml`), two net-new tests in Go, and one minor `Makefile` target (`release-snapshot`). Everything else is pin-tightening.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Cross-compile matrix | GoReleaser (release-time) | GitHub Actions (runner) | GoReleaser owns the matrix → flags → archive pipeline; Actions just invokes it |
| Test execution per OS | GitHub Actions (runner matrix) | Go toolchain (test binary) | Actions owns OS selection; Go owns the test run itself |
| Race detection | Go toolchain (`-race`) | GitHub Actions matrix include | Go toolchain has first-class support on linux+darwin; Actions conditionally passes the flag |
| Static analysis | `golangci-lint` (v2.x) | GitHub Actions (single job) | Linter owns rule set; Actions pins version |
| Size / startup / stdout gates | Shell scripts (`scripts/check-*.sh`) | GitHub Actions (Linux job) | Gate logic is already shell; Actions just calls it — no change in Phase 9 |
| Version embedding | Go linker (`-X main.version`) | GoReleaser (template) | Linker does the injection; GoReleaser supplies the template value |

## Standard Stack

### Core (all already in-repo via go.mod / CLAUDE.md pins — Phase 9 adds tooling only)

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| GoReleaser | v2.15.4 (pinned in CLAUDE.md) | Cross-compile + release attach | Config-driven, 6-arch matrix in ~15 lines YAML, pure-Go projects are its sweet spot |
| `goreleaser/goreleaser-action` | v6 (pinned in CLAUDE.md) | GitHub Action wrapper | Official action; uses `actions/setup-go` it the caller sets up |
| `golangci/golangci-lint-action` | v8 → pin to `v8.0.0` OR upgrade to latest | Lint runner | Pinning matters for reproducibility (see Static Analysis section) |
| `golangci-lint` binary | v2.11.4 (latest stable as of 2026-03-22) | Lint (bundles staticcheck, gosimple) | Single tool for QA-04 instead of running `go vet` + `staticcheck` separately |
| `actions/checkout` | v4 | Checkout | Standard |
| `actions/setup-go` | v5 | Go setup + cache | `cache: true` handles module + build cache for free on Go 1.21+ |

### Alternatives Considered

| Instead of | Could Use | Why not |
|------------|-----------|---------|
| GoReleaser | Plain Actions matrix + `go build` per OS | Would lose archives, checksums, changelog. GoReleaser's 15 lines replace ~60 lines of hand-rolled matrix script |
| golangci-lint | Separate `go vet` + `staticcheck` jobs | D-19 rejects — two tools = two reporting surfaces, slower CI |
| `go test -race` split by matrix-include | Separate workflow for Windows | Matrix `include` with conditional step is idiomatic; separate workflow doubles maintenance |

## GoReleaser v2 Config Sketch

**Drop-in target:** `/.goreleaser.yaml` (net-new, repo root)

```yaml
# .goreleaser.yaml — Phase 9 release config (DIST-02).
# v2 format (version: 2 required).
# Mirrors Makefile flags: -trimpath + -s -w -X main.version=...
version: 2

project_name: mcp-chain

before:
  hooks:
    - go mod tidy
    - go mod verify

builds:
  - id: mcp-chain
    main: ./cmd/mcp-chain
    binary: mcp-chain
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    mod_timestamp: '{{ .CommitTimestamp }}'  # reproducible builds: same commit → same bytes

archives:
  - id: mcp-chain
    ids:
      - mcp-chain
    name_template: '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}'
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - LICENSE*
      - README*

checksum:
  name_template: 'checksums.txt'
  algorithm: sha256

snapshot:
  version_template: '{{ incpatch .Version }}-snapshot-{{ .ShortCommit }}'

changelog:
  sort: asc
  use: git
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Others
      order: 999
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
      - '^ci:'
      - Merge pull request
      - Merge branch

release:
  github:
    owner: anthropics    # TODO: verify repo org at release time
    name: mcp-chain
  draft: false
  prerelease: auto       # v0.1.0, v0.1.0-rc.1 → prerelease; v1.0.0 → stable
  name_template: '{{.ProjectName}} {{.Version}}'
```

**Caveats the planner must address:**
- `LICENSE*` / `README*` glob — will fail silently if files don't exist in repo root. `README.md` lands in Phase 10. For Phase 9, either (a) create a placeholder `LICENSE` (not in scope per CONTEXT — D-06 accepts soft failure) or (b) add a `README.md` stub. **Recommendation: the dry-run job's assertion must allow missing `README`/`LICENSE`** until Phase 10; GoReleaser logs a warning but doesn't fail the archive step.
- `release.github.owner` — placeholder; planner must confirm against actual GitHub org. If the repo is `tkr41850-debug/mcp-chain`, the value above is correct. If it's moved to another org, adjust.

## Release Workflow Sketch

**Drop-in target:** `/.github/workflows/release.yml` (net-new)

```yaml
# .github/workflows/release.yml — tag-driven release via GoReleaser.
# Triggered on: git push origin v*
name: release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # required for GoReleaser to upload release assets

jobs:
  goreleaser:
    name: GoReleaser
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0   # GoReleaser needs full git history for changelog

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'   # pin to v2 major; respects minor bumps
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Notes:**
- `go-version-file: go.mod` reads `1.25.0` from the current `go.mod` (note: `go.mod` says `1.25.0` but CLAUDE.md + all CI jobs pin `"1.23"`; see Known Landmines about this discrepancy).
- `permissions: contents: write` is **mandatory** since 2024's default-permissions tightening, or GoReleaser's asset upload will 403.
- `version: '~> v2'` (or just `version: latest`) — `~> v2` anchors to the GoReleaser v2 series, matching CLAUDE.md's pin.

## CI Matrix Expansion Sketch

**Drop-in target:** `/.github/workflows/ci.yml` (in-place edit, not replacement)

```yaml
# .github/workflows/ci.yml — Phase 9 expansion of the Phase 1 single-OS workflow.
# Preserves lint + build-and-gate Linux-only gates; adds 3-OS test matrix + release dry-run.
name: ci

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  # --- Unchanged from Phase 1 ------------------------------------------------
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v8
        with:
          version: v2.11.4   # pinned — reproducibility (D-18)
          args: --timeout=5m

  # --- Unchanged from Phase 1 (Linux-only size/startup/stdout/net-http gates) -
  build-and-gate:
    name: Build + Gates
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true

      - name: Build (stripped + trimpath)
        run: make build

      - name: Size gate (≤ 15 MB stripped)
        run: ./scripts/check-size.sh ./mcp-chain

      - name: Startup gate (≤ 100 ms P95 of 5 runs)
        run: ./scripts/check-startup.sh ./mcp-chain

      - name: Stdout silence gate (serve </dev/null writes 0 bytes to stdout)
        run: ./scripts/check-stdout-silence.sh ./mcp-chain

      - name: Ban net/http import (MCP-01 prep)
        run: |
          if go list -f '{{ join .Deps "\n" }}' ./... | grep -q '^net/http$'; then
            echo "ERROR: net/http is in the dependency graph. stdio-only; see MCP-01."
            exit 1
          fi

      - name: Smoke chain-wait (Phase 8 plugin monitor)
        run: ./scripts/smoke-chain-wait.sh

  # --- NEW in Phase 9: 3-OS test matrix (QA-03) ------------------------------
  test:
    name: Test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    needs: lint
    strategy:
      fail-fast: false   # see failures on all OSes in one run
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        include:
          - os: ubuntu-latest
            race: "-race"
          - os: macos-latest
            race: "-race"
          - os: windows-latest
            race: ""   # QA-03: Windows skips -race (race detector incomplete on Windows)
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true

      - name: go build
        run: go build ./...

      - name: go vet
        run: go vet ./...

      - name: go test
        shell: bash   # bash on all three OSes (Windows runners ship Git Bash)
        run: go test ${{ matrix.race }} -count=1 -timeout=120s ./...

  # --- NEW in Phase 9: release dry-run (D-12) --------------------------------
  release-dry-run:
    name: Release dry-run (snapshot)
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # GoReleaser needs full history even in snapshot mode

      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true

      - name: GoReleaser snapshot
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --snapshot --clean --skip=publish

      - name: Assert 6 archives + checksums.txt under dist/
        run: |
          set -eu
          cd dist
          # Expect: 3 tar.gz (linux+darwin × amd64/arm64 + linux arm64 or darwin arm64)
          # Actually: linux_amd64.tar.gz, linux_arm64.tar.gz, darwin_amd64.tar.gz,
          #          darwin_arm64.tar.gz, windows_amd64.zip, windows_arm64.zip, checksums.txt
          TAR_COUNT=$(ls *.tar.gz 2>/dev/null | wc -l)
          ZIP_COUNT=$(ls *.zip 2>/dev/null | wc -l)
          if [ "$TAR_COUNT" -ne 4 ]; then
            echo "ERROR: expected 4 tar.gz archives, got $TAR_COUNT"
            ls -la
            exit 1
          fi
          if [ "$ZIP_COUNT" -ne 2 ]; then
            echo "ERROR: expected 2 zip archives, got $ZIP_COUNT"
            ls -la
            exit 1
          fi
          if [ ! -f checksums.txt ]; then
            echo "ERROR: checksums.txt missing"
            ls -la
            exit 1
          fi
          echo "Archives + checksums OK (TAR=$TAR_COUNT ZIP=$ZIP_COUNT)"

      - name: Assert snapshot version embedded (not 'dev')
        run: |
          set -eu
          # Extract linux/amd64 binary and run --version; must NOT print "mcp-chain dev".
          mkdir -p extract
          tar -xzf dist/mcp-chain_*_linux_amd64.tar.gz -C extract
          VER=$(./extract/mcp-chain --version)
          echo "Version string: $VER"
          if echo "$VER" | grep -q '^mcp-chain dev$'; then
            echo "ERROR: snapshot build printed 'dev' — ldflags injection broken"
            exit 1
          fi
          if ! echo "$VER" | grep -q '^mcp-chain '; then
            echo "ERROR: unexpected version output: $VER"
            exit 1
          fi
```

**Why `fail-fast: false`:** a race failure on macOS should not cancel the Linux run — otherwise you can't see whether the problem is OS-specific. Dev-loop feedback quality matters more than saving a minute of CI time.

**Why `shell: bash` on test step:** Windows runners default to PowerShell, which has different `${{ matrix.race }}` interpolation semantics. Using bash is uniform and `git-bash` ships on the Windows runner image.

## Test Coverage Gap Audit

**Methodology:** Cross-referenced CONTEXT.md D-14 (QA-01) and D-15 (QA-02) bullet-by-bullet against the existing `*_test.go` inventory:

- `internal/idgen/idgen_test.go` (4 tests)
- `internal/statepath/resolve_test.go` (6 tests)
- `internal/store/store_test.go` (27 tests)
- `internal/store/integration_test.go` (2 cross-process tests + `TestMain`)
- `internal/mcpserver/server_test.go` (6 tests)
- `internal/mcpserver/integration_test.go` (3 stdio JSON-RPC tests + `TestMain`)
- `internal/mcpserver/owner_test.go` (2 tests)
- `internal/mcpserver/tools_test.go` (2 tests)
- `internal/cli/{status,list,purge,resolve}_test.go` (17 unit tests)
- `internal/cli/integration_test.go` (10 subprocess-spawning tests)
- `internal/cli/format/table_test.go` (4 tests)
- `internal/plugin/manifest_test.go` (7 tests)
- `cmd/mcp-chain/main_test.go` (2 tests)
- Phase 8 `scripts/smoke-chain-wait.sh` (3 cases — resolve / unknown / timeout)

### QA-01: Unit-level Coverage

| # | QA-01 Bullet | Status | Evidence |
|---|--------------|--------|----------|
| 1 | Wordlist allocation determinism | COVERED | `internal/idgen/idgen_test.go::TestWordlistInvariants` + `TestAllocate` (table-driven, boundary indices) |
| 2 | Counter monotonicity | COVERED | `internal/store/store_test.go::TestStore_RegisterMonotonicCounter` + `TestStore_PurgeDoesNotDecrementCounter` + `internal/cli/integration_test.go::TestPurge_CounterNotDecremented` (triple-coverage — CORE-09 is pinned) |
| 3 | Hex fallback past wordlist exhaustion | COVERED | `internal/idgen/idgen_test.go::TestAllocateMonotonicUniqueOverBoundary` (index 1296+) + `TestAllocate` table cases |
| 4 | State schema round-trip | COVERED | `internal/store/store_test.go::TestStore_SchemaVersionMismatchErrors` + `TestStore_CorruptJSONErrors` + `TestStore_ResolvedAtNullInJSON` |
| 5 | Timeout parsing | COVERED | No Go-side timeout parser exists (`chain-wait.sh` is shell). `scripts/smoke-chain-wait.sh` case 3 covers the shell parser end-to-end. Per D-14 explicit carve-out: "wait.sh is shell, not Go; count as covered via Phase 8 smoke harness." **COVERED with caveat** — no gap to fill. |
| 6 | Path resolution | COVERED | `internal/statepath/resolve_test.go` — 6 tests for XDG set, HOME fallback, neither set, empty XDG, parent-exists, idempotent |
| 7 | ID lookup | COVERED | `internal/store/store_test.go::TestStore_Get` + `TestStore_GetUnknownIDReturnsErr` |
| 8 | State transitions (pending→resolved, double-resolve err, unknown err, OwnerToken mismatch err) | COVERED | `internal/store/store_test.go::TestStore_ResolveOwnerOk` (pending→resolved), `TestStore_ResolveAlreadyResolvedReturnsErr` (double), `TestStore_ResolveUnknownIDReturnsErr`, `TestStore_ResolveWrongOwnerReturnsErrNotOwner`, `TestStore_ResolveForceBypassesOwnerCheck` |

**QA-01 Result: 8/8 COVERED. No gaps to fill.**

### QA-02: Integration Coverage

| # | QA-02 Bullet | Status | Evidence / Proposed Fill |
|---|--------------|--------|--------------------------|
| 1 | E2E register → status pending → resolve → status resolved | **COVERED (partial merge)** | The E2E chain splits across two test packages but every leg exists: (a) `internal/mcpserver/integration_test.go::TestServe_StdioFullHandshake` drives `register` → `resolve` over JSON-RPC stdio (store state verified via SDK response). (b) `internal/cli/integration_test.go::TestStatus_IntegrationExitCodes` drives `register-via-store → status pending` and `register-via-store → resolve-via-store → status resolved`. **RECOMMENDATION:** the planner may optionally add a single `TestE2E_RegisterWaitResolveChain` to `internal/cli/integration_test.go` that chains: spawn `serve`, send `register`, then spawn `status` in a loop until resolved is observed. Low cost (~40 LOC); but not strictly required for QA-02 closure since every link is independently verified. |
| 2 | N concurrent waiters on the same ID all see resolve | **GAP** | `TestStatus_Concurrent10WithinOneSecond` proves 10 concurrent `status` probes don't serialize, but all read the SAME already-resolved state. No test proves the register→N-waiters→resolve fan-out where waiters start pending and observe the transition. **RECOMMENDATION:** add `internal/cli/integration_test.go::TestConcurrentWaiters_AllSeeResolve` — seed a pending id, launch N=10 goroutines each running a 50ms-poll loop against `status <id>`, then in the parent call `resolve`, assert all 10 observe `resolved\n` stdout within 2s. Uses existing `buildBinary(t)` + `seedStateForChild(t)` helpers. ~60 LOC. |
| 3 | Double-resolve error | COVERED | `internal/store/store_test.go::TestStore_ResolveAlreadyResolvedReturnsErr` + `internal/cli/resolve_test.go::TestRunResolve_AlreadyResolved_Exit1` + `internal/mcpserver/server_test.go::TestResolveHandler_AlreadyResolved` (triple-coverage store/CLI/MCP) |
| 4 | Unknown-ID error | COVERED | `internal/store/store_test.go::TestStore_ResolveUnknownIDReturnsErr` + `internal/cli/resolve_test.go::TestRunResolve_UnknownID_Exit1` + `internal/cli/integration_test.go::TestStatus_IntegrationExitCodes` "unknown" case + `internal/mcpserver/server_test.go::TestResolveHandler_UnknownID` |
| 5 | Purge-mid-wait error | **GAP (Go-side)** | Covered at shell level by `scripts/smoke-chain-wait.sh` (implicit via unknown-id case), but there is NO Go integration test proving the `status <id>` exit-1 transition when a running waiter's id is purged between polls. **RECOMMENDATION:** add `internal/cli/integration_test.go::TestStatus_PurgedMidPoll_Exit1` — seed pending id, start a bg poll, from parent run `purge <id>`, next poll should exit 1 with "unknown id" on stderr. ~40 LOC. |
| 6 | OwnerToken mismatch error | COVERED | `internal/store/store_test.go::TestStore_ResolveWrongOwnerReturnsErrNotOwner` + `internal/mcpserver/server_test.go::TestResolveHandler_NotOwner` + `internal/mcpserver/integration_test.go::TestServe_ResolveNotOwnerWireCode` (wire-level across processes) + `internal/cli/resolve_test.go::TestRunResolve_NoForce_NotOwner_Exit1` |
| 7 | Cross-process flock safety with 100 entries each concurrently | COVERED | `internal/store/integration_test.go::TestStore_TwoProcessesConcurrentRegister` — spawns two subprocesses, each registers 100 entries, asserts 200 unique word-IDs (exactly the CONTEXT.md D-15 wording). Also `TestStore_KillMidWriteLeavesCoherentState` as a bonus SIGKILL invariant. |

**QA-02 Result: 5/7 COVERED. 2 concrete gaps.**

### Gap Fills — Planner Task List

The Phase 9 planner must author exactly **two new Go tests** in addition to the YAML work. Both go in `internal/cli/integration_test.go` (same package, reuses `buildBinary` + `seedStateForChild` helpers — no new subprocess scaffolding):

1. **`TestConcurrentWaiters_AllSeeResolve`** — fills QA-02 bullet 2 ("N concurrent waiters"). Sketch:
   ```go
   // Seed pending id; spawn N=10 poll loops in goroutines; resolve in parent;
   // assert every goroutine observes `resolved\n` + exit 0 within 2s.
   // Deterministic synchronization: use a sync.WaitGroup for poll-started, a
   // channel for resolve-observed; NO sleep-based coordination (flaky).
   ```

2. **`TestStatus_PurgedMidPoll_Exit1`** — fills QA-02 bullet 5 ("purge-mid-wait"). Sketch:
   ```go
   // Seed pending id; launch background poll (goroutine); from parent call
   // `purge <id>` via binary; next status call should exit 1 with "unknown id"
   // on stderr. Assert via channel handoff (no sleeps).
   ```

Both tests respect D-17 (zero flakiness) — no `time.Sleep` coordination, only sync primitives / channels.

**Optional (not required for QA-02 closure):** `TestE2E_RegisterWaitResolveChain` as a belt-and-braces single-test E2E — consolidates the QA-02 bullet 1 chain into one subtest. Planner discretion.

## Static Analysis Wiring

**Existing config at `/.golangci.yml`** (Phase 1, v2 format — verified):
- Already enables: `staticcheck` (bundles `gosimple` in v2.x), `govet` (with `shadow` enabled), `forbidigo` (MCP-02 stdout-discipline guard).

**Gaps vs CLAUDE.md recommendation `govet, staticcheck, errcheck, gofmt, gosimple`:**
- `gosimple` — **covered** (bundled in `staticcheck` from v2.x, per existing config comment).
- `gofmt` — **missing from enabled list.** `golangci-lint` v2 default set includes `gofmt` checks via `gofmt` formatter. Needs explicit enable.
- `errcheck` — **missing from enabled list.** Default v2 `standard` set includes it — needs verification but likely already active via `default: standard`.

**Recommended final `.golangci.yml` linters block:**
```yaml
linters:
  default: standard
  enable:
    - errcheck      # explicit (standard includes it but explicit is safer)
    - forbidigo     # existing — MCP-02 stdout guard
    - gofmt         # new — CLAUDE.md recommendation
    - gosimple      # bundled in staticcheck v2 — listed for visibility
    - govet         # existing
    - staticcheck   # existing
  # settings section unchanged
```

**Version pin for golangci-lint-action:**
- Current config: `version: latest` (line 26 of `ci.yml`) — **not reproducible.**
- **Recommendation:** pin to `v2.11.4` (released 2026-03-22, latest stable as of this research). Re-bump at the next scheduled maintenance pass.
- **Why pinning matters:** a `latest` fetch on a green merge, followed by a new lint rule in v2.12 added days later, fails the next green PR for reasons unrelated to the diff. Reproducibility > staying on the bleeding edge for a stability-focused release phase.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| Config file | `go.mod` / no separate config; matrix behavior in `.github/workflows/ci.yml` |
| Quick run (per commit) | `go test -race -count=1 -timeout=60s ./...` (matches Makefile `make test` target) |
| Full suite (per merge) | same command, plus `make release-snapshot` for config validation |
| Phase gate | full suite green on 3-OS matrix before tag push |

### Phase Requirements → Test Map (SC-1..SC-4 from ROADMAP)

| SC | Requirement (verbatim from ROADMAP.md Phase 9) | Test / Command |
|----|------------------------------------------------|----------------|
| SC-1 | Push/PR CI runs `go test -race -count=1 ./...` on Linux + macOS, non-race on Windows; failure blocks merge | `ci.yml::test` job matrix with conditional `${{ matrix.race }}` (see sketch above) |
| SC-2 | Tag `v*` triggers GoReleaser to cross-compile 6 arch combos, strip `-s -w -trimpath`, attach archives + `checksums.txt`, embed tag in `--version` | `release.yml` + `.goreleaser.yaml` (sketches above); `ci.yml::release-dry-run` verifies config on every PR |
| SC-3 | Unit suite covers wordlist, counter, hex fallback, schema, timeout, path, lookup, all state transitions | QA-01 audit above — 8/8 COVERED, no fills needed |
| SC-4 | Integration suite covers E2E + N concurrent waiters + double-resolve + unknown-ID + purge-mid-wait + OwnerToken + 2-process-100-entry flock | QA-02 audit above — 5/7 COVERED; fills `TestConcurrentWaiters_AllSeeResolve` + `TestStatus_PurgedMidPoll_Exit1` |

### Sampling Rate

- **Per task commit:** `go test -race -count=1 -timeout=60s ./...` (Linux dev box)
- **Per wave merge:** full 3-OS matrix via PR CI (`ci.yml`)
- **Phase gate:** 3-OS matrix green + `release-dry-run` green + static analysis green before merge to main

### Wave 0 Gaps

- [ ] `internal/cli/integration_test.go::TestConcurrentWaiters_AllSeeResolve` — new, ~60 LOC, closes QA-02 bullet 2
- [ ] `internal/cli/integration_test.go::TestStatus_PurgedMidPoll_Exit1` — new, ~40 LOC, closes QA-02 bullet 5

Existing test infrastructure (`buildBinary`, `seedStateForChild`, `bytes.Buffer` writer pattern) covers everything else. No framework install, no new fixture helpers needed.

## Known Landmines

1. **macOS runner architecture.** `macos-latest` has been arm64 (Apple Silicon) since GitHub rolled out M1 runners in late 2024. For mcp-chain's stdlib-only pure-Go surface, arm64 testing is sufficient (the Go toolchain's arm64 darwin is a primary platform). **If the planner wants amd64 macOS symmetry, explicitly add `macos-13` to the test matrix `include`.** Do NOT assume `macos-latest` is amd64 — it is not.

2. **`actions/setup-go@v5` + `cache: true` requires `go.sum`.** Repo has `/home/alpine/mcp-chain/go.sum` (verified 4332 bytes, last modified 2026-04-24). Cache will work. If someone deletes or rewrites `go.sum` during a refactor, the cache step silently skips — lint might suddenly fail on a new lint rule because the lint binary isn't cached. Keep `go.sum` committed.

3. **GoReleaser `fetch-depth: 0`.** `actions/checkout@v4`'s default `fetch-depth: 1` is a shallow clone. GoReleaser's changelog engine walks the tag→tag commit range and will emit `"WARN no previous tag found for current tag, using empty commit"` or fail entirely under shallow clones. **Both `release.yml` and `ci.yml::release-dry-run` must set `fetch-depth: 0`.**

4. **`.goreleaser.yaml` v2 header.** Missing `version: 2` at the file top causes GoReleaser v2 to error with `"config version 1 is not supported"`. The sketch above has it; do not let a lint auto-fixer strip it.

5. **`GITHUB_TOKEN` default permissions.** Since GitHub's 2023-2024 default-permissions tightening, a workflow that uploads release assets must declare `permissions: contents: write` at the workflow or job level. **Without this the release step 403s with a confusing error.** The `release.yml` sketch sets this.

6. **`go.mod` Go version discrepancy.** `go.mod` declares `go 1.25.0` but all CI jobs + CLAUDE.md pin `go-version: "1.23"`. This is intentional (CLAUDE.md wants reproducibility; `go 1.25.0` is the toolchain directive for newer features) but means `go-version-file: go.mod` would resolve to `1.25`, not `1.23`. **The `release.yml` sketch uses `go-version-file: go.mod`** to match the module's minimum; **the `ci.yml` sketch uses the literal `"1.23"`** to match CLAUDE.md. If this inconsistency is undesirable, the planner should either (a) align `go.mod` to `go 1.23` (a soft downgrade of the toolchain directive) or (b) align all CI to `go-version-file: go.mod`. **Recommendation (a):** downgrading `go.mod` to `go 1.23` matches CLAUDE.md's "single version across dev + CI + release" principle (D-01).

7. **Windows test shell.** `go test` is fine cross-platform, but if any test helper uses `exec.Command("go", "run", ...)` with paths or bash-style env-var syntax, Windows will fail. Existing integration tests use `os.Environ()` + key-value pairs (safe) and `filepath.Join` (safe) — verified. Smoke shell scripts (`check-*.sh`, `smoke-chain-wait.sh`) are Linux-only per D-03 and stay off the Windows runner.

8. **`forbidigo` on test files.** `.golangci.yml` currently has `tests: true` + `forbidigo` forbidding `fmt.Print*`. Any test using `fmt.Println` will fail lint. The planner's new tests must use `t.Logf` or `fmt.Fprintln(os.Stderr, ...)` for diagnostic output — not bare `fmt.Println`. Existing tests comply; don't regress.

9. **Archive contents missing LICENSE/README.** `archives.files: [LICENSE*, README*]` — GoReleaser v2 logs a warning but does not fail when a glob matches nothing. `README.md` is Phase 10; `LICENSE` is deferred per PROJECT.md. **The dry-run assertion must NOT grep for LICENSE inside archives** in Phase 9 — only assert count + checksums exist.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `macos-latest` is arm64 Apple Silicon (since 2024 rollout) | Known Landmines #1 | If still amd64 on project's runner tier, the "explicitly add macos-13" recommendation is redundant (low risk; worst case is an unnecessary matrix entry) |
| A2 | `golangci-lint` v2.11.4 is the latest stable as of research date | Static Analysis Wiring | If a newer stable shipped this week, pin is slightly stale (low risk; still reproducible) |
| A3 | GoReleaser v2 `formats: [tar.gz]` (plural) is the current idiom; singular `format:` still parses but is soft-deprecated | GoReleaser Config Sketch | If only plural parses in v2.15.4 exactly, older-style configs online might be misleading but sketch is correct |
| A4 | GitHub default token permissions require explicit `contents: write` for release asset upload | Known Landmines #5 | [VERIFIED via WebFetch of GoReleaser Actions page in CLAUDE.md sources] — this is cited, not assumed |
| A5 | Windows `go test -race` does not reliably work and is excluded per QA-03 | CI Matrix Expansion Sketch | [VERIFIED via CONTEXT.md D-02 quoting REQUIREMENTS QA-03] — locked decision, not assumed |

**If this table is empty:** it is not — 3 low-risk assumptions. A1 is the only one with material impact (extra macos-13 runner). Planner can confirm with one `gh run list` lookup at plan time.

## Open Questions

1. **Should `.goreleaser.yaml` enable `reproducible builds` via `mod_timestamp: '{{ .CommitTimestamp }}'`?**
   - What we know: PROJECT.md does not explicitly require bit-identical release binaries across time, but D-05 says "byte-for-byte comparable" between local `make build` and release.
   - What's unclear: `make build` uses `git describe --tags --always --dirty` for VERSION and no mod_timestamp. Local and release will diverge by `mod_timestamp` alone anyway.
   - Recommendation: include `mod_timestamp` in `.goreleaser.yaml` (sketch has it) — it's a reproducibility belt-and-braces that costs nothing and helps if anyone ever needs to diff two release tarballs.

2. **Should the release dry-run job run on every push, or only on PRs?**
   - What we know: D-12 says "triggered on PR/push."
   - What's unclear: a push-to-main (post-merge) dry-run is redundant with the PR dry-run that already passed.
   - Recommendation: keep on both per D-12 (matches existing `ci.yml` trigger). Three minutes of runner time per merge is cheap.

3. **Should `LICENSE` be created now (even as a placeholder) or deferred to Phase 10?**
   - What we know: PROJECT.md line 59 "License selection deferred." CONTEXT.md D-06 explicitly accepts graceful-failure.
   - What's unclear: whether the author plans an MIT/Apache/etc. pick before v0.1.0.
   - Recommendation: leave to Phase 10 per CONTEXT; GoReleaser's warning-not-error on missing `LICENSE*` glob is acceptable.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | `go test`, `go build` | ✓ (local) / ✓ (runners) | 1.23 (CI pin) / 1.25 (go.mod) | — |
| GoReleaser | Release + dry-run jobs | ✗ locally required; ✓ via Action in CI | v2.15.4 (pinned in CLAUDE.md) | Install via `go install github.com/goreleaser/goreleaser/v2@v2.15.4` for local `make release-snapshot` |
| `golangci-lint` | Lint job + `make lint` | ✓ on CI (action installs); ? local | v2.11.4 | Install via `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4` |
| `git` | Version stamping, changelog | ✓ | any recent | — |
| `bash` | Shell gates, smoke tests, CI test step | ✓ (Linux+macOS native; Windows via Git Bash shipped on runner) | 3.2+ | — |
| GitHub Actions runners | All CI jobs | ✓ (ubuntu-latest, macos-latest, windows-latest) | — | — |

**Missing with fallback:** GoReleaser + golangci-lint locally — author may want to install via `go install` for the `make release-snapshot` target Phase 9 adds.

**Missing, blocking:** none.

## Sources

### Primary (HIGH confidence)

- `/.github/workflows/ci.yml` (local) — existing Phase 1 CI, the file Phase 9 expands in-place.
- `/Makefile` (local) — confirms `-s -w -X main.version=...` + `-trimpath` flags that `.goreleaser.yaml` must mirror.
- `/cmd/mcp-chain/main.go:22` (local) — confirms `var version = "dev"` exists for `-X main.version=` injection.
- `/.golangci.yml` (local) — existing v2 lint config; Phase 9 only tightens.
- `/go.mod` + `/go.sum` (local) — dep set, Go toolchain directive (1.25.0), sum file present for cache.
- `.planning/phases/09-ci-release/09-CONTEXT.md` — all 24 locked decisions (D-01..D-24).
- `.planning/REQUIREMENTS.md` — DIST-02, QA-01..04 canonical wording.
- `.planning/ROADMAP.md` Phase 9 — SC-1..SC-4 verbatim.
- 14 `*_test.go` files — grep'd for test function names and inspected for the QA audit above.
- `scripts/smoke-chain-wait.sh` — Phase 8 bash harness; closes QA-02 bullet 5 at shell level.

### Secondary (MEDIUM-HIGH confidence via WebFetch this session)

- [GoReleaser v2 builds section](https://goreleaser.com/customization/builds/go/) — confirms `env: [CGO_ENABLED=0]`, `flags: [-trimpath]`, `ldflags: [-s -w -X main.version={{.Version}}]`, `goos`/`goarch` matrix syntax.
- [GoReleaser v2 archives](https://goreleaser.com/customization/archive/) — confirms `formats: [tar.gz]` + `format_overrides: [{goos: windows, formats: [zip]}]` current idiom, `name_template` default, `files: [LICENSE*, README*]` default.
- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases) — v2.11.4 released 2026-03-22 as latest stable.

### Tertiary (from CLAUDE.md source list — HIGH confidence, not re-verified)

- [goreleaser.com/ci/actions](https://goreleaser.com/ci/actions/) — `goreleaser/goreleaser-action@v6`, `args: release --clean`, `GITHUB_TOKEN` permission requirements.
- [github.com/actions/setup-go](https://github.com/actions/setup-go) — v5 `cache: true`, `go-version-file` support.

## Metadata

**Confidence breakdown:**
- GoReleaser config sketch: HIGH — verified against current docs + CLAUDE.md pin.
- CI matrix sketch: HIGH — expands an existing green workflow with well-documented GitHub Actions features.
- Test coverage audit: HIGH — every bullet traced to a specific test file:name or marked explicit GAP with proposed signature.
- Static analysis wiring: HIGH — existing `.golangci.yml` inspected; CLAUDE.md recommendation cross-referenced; version pin sourced.
- Known landmines: HIGH — each item is either verified (repo inspection) or cited from GoReleaser/Actions docs.

**Research date:** 2026-04-24
**Valid until:** 2026-05-24 (30 days for stable tools; re-verify GoReleaser minor and golangci-lint v2.x tags before next release)

## RESEARCH COMPLETE
