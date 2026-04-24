# Phase 9: CI Release, Cross-compile & Test Gates - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning
**Mode:** `--auto` (single-pass, all gray areas auto-resolved to recommended defaults)

<domain>
## Phase Boundary

Turn the existing green-on-Linux build into a **release-grade pipeline**:

1. A GoReleaser-driven tag release (`v*`) that cross-compiles the six arch combos (`linux/darwin/windows × amd64/arm64`), strips with `-s -w -trimpath`, embeds the tag in `--version`, emits `checksums.txt`, and attaches everything to the GitHub release.
2. A CI workflow that gates every push/PR with `go test -race -count=1 ./...` on Linux **and** macOS, and with the non-race suite on Windows (per QA-03).
3. A comprehensive test surface closeout — unit coverage for every locked-decision contract (QA-01) and integration coverage for every error path (QA-02).
4. Static analysis gate — `staticcheck` (or equivalent) wired into CI so non-zero exit blocks merge (QA-04).

**In scope:** `.goreleaser.yaml`, `.github/workflows/release.yml`, expansion of `.github/workflows/ci.yml` to a matrix, static analysis wiring, targeted test-gap fills if the planner's audit surfaces them.

**Out of scope:** README, dogfooding validation, code signing / notarization / SBOMs, homebrew taps / deb+rpm packaging, any new features in the binary itself. Those live in Phase 10 or are explicitly deferred.

</domain>

<decisions>
## Implementation Decisions

### CI Matrix Shape (LD-1)

- **D-01:** CI runs on three OS runners: `ubuntu-latest`, `macos-latest`, `windows-latest`. Go version pinned to `1.23` (the go.mod minimum) across all runners — single version keeps reproducibility tight between dev, CI, and the release binary. No Go matrix.
- **D-02:** Linux and macOS run `go test -race -count=1 ./...`. Windows runs `go test -count=1 ./...` (no `-race`) per QA-03 — matches PROJECT.md's "Windows supported via CI cross-compile" secondary-platform posture.
- **D-03:** The existing Linux-only gates (size gate, startup gate, stdout-silence gate, `net/http` ban, shell lint gates from Phase 8) stay on the Linux runner only. Duplicating them on macOS/Windows adds runtime without catching regressions the stripped/signed Linux binary wouldn't catch.
- **D-04:** `lint` job stays as a separate step that the matrix `test` jobs depend on — fail fast on lint before burning 3× runner time. `staticcheck` folds into the existing `golangci-lint` config (it's already one of the default linters in v2.x) rather than running as a separate step.

### GoReleaser & Release Workflow (LD-2)

- **D-05:** New file `.goreleaser.yaml` (v2 format — `version: 2` at top). Cross-compile matrix: `goos: [linux, darwin, windows]` × `goarch: [amd64, arm64]`. `flags: [-trimpath]` and `ldflags: -s -w -X main.version={{.Version}}` — `main.version` is the existing package-level variable in `cmd/mcp-chain/main.go` that `--version` already prints.
- **D-06:** Archive format: `tar.gz` for linux + darwin, `zip` for windows. Each archive includes the binary plus `LICENSE` (TODO if missing) and `README.md` (written in Phase 10 — archive-files directive will fail gracefully during Phase 9 release dry-run if missing).
- **D-07:** Checksums: `checksums.txt` at release root, SHA-256, GoReleaser default.
- **D-08:** Release workflow: new file `.github/workflows/release.yml`, trigger `on: push: tags: ['v*']`. Steps: checkout with `fetch-depth: 0` (GoReleaser needs full git history for changelog), `setup-go@v5` with Go `1.23`, `goreleaser/goreleaser-action@v6` with `args: release --clean`. Token: `GITHUB_TOKEN` from `secrets`.
- **D-09:** Code signing, notarization, SBOM generation — **deferred**. Not in requirements; single dev + hobby-scale audience; can be bolted on later without breaking the release format.
- **D-10:** CGO is disabled at the matrix level (`env: - CGO_ENABLED=0`) — the whole project is cgo-free and we want to guarantee it at release time.

### Version Embedding (LD-3)

- **D-11:** `cmd/mcp-chain/main.go` has `var version = "dev"` (Phase 1 skeleton). GoReleaser sets `-X main.version={{.Version}}` at release time → tag `v0.1.0` yields `--version` printing `mcp-chain v0.1.0`. Local dev builds (via `make build`) continue to print `mcp-chain dev`.
- **D-12:** Validation: add a **release dry-run smoke** to CI (separate job, linux-only, triggered on PR/push) that runs `goreleaser release --snapshot --clean --skip=publish` and asserts the six archives + `checksums.txt` appear under `dist/`. Also asserts that a binary inside a dist archive prints a non-`dev` version string (GoReleaser snapshot mode embeds the snapshot version). Keeps the release pipeline honest without waiting for a tag push to discover config drift.

### Test Coverage Closeout (LD-4)

- **D-13:** Planner's first task is a **gap audit** — cross-reference REQUIREMENTS.md QA-01/QA-02 bullets against existing `*_test.go` files and record which bullets are covered vs missing. Hypothesis going in: coverage is near-complete (Phases 2–8 built tests alongside implementation), so Phase 9 adds targeted fills rather than bulk test authoring. Cover each gap with a stdlib test (no new testify-beyond-`require`).
- **D-14:** The canonical QA-01 checklist to cross-reference:
  - wordlist allocation determinism (Phase 2)
  - counter monotonicity (Phase 4 — pinned in `TestPurge_CounterNotDecremented` per Phase 7 close)
  - hex fallback when wordlist exhausts (Phase 2)
  - state schema round-trip (Phase 4)
  - timeout parsing (Phase 6 status, Phase 8 chain-wait.sh duration parser — wait.sh is shell, not Go; count as covered via Phase 8 smoke harness)
  - path resolution (Phase 3)
  - ID lookup (Phase 4 store)
  - state transitions: pending → resolved, double-resolve error, unknown error, OwnerToken mismatch error (Phases 4/5/7)
- **D-15:** The canonical QA-02 checklist:
  - E2E register → status pending → resolve → status resolved (Phase 5 integration test? Phase 6 CLI integration test? Confirm or fill)
  - N concurrent waiters see resolve (verify exists — likely a gap)
  - double-resolve error (covered)
  - unknown-ID error (covered)
  - purge-mid-wait error (verify exists — likely a gap)
  - OwnerToken mismatch error (covered)
  - cross-process flock safety with 100 entries each concurrently (verify exists — Phase 4 listed `integration_test.go`; confirm scale)
- **D-16:** Integration tests that spawn subprocesses use `go run ./cmd/mcp-chain serve` or `exec.Command` against a test-built binary. No new network transports, no MCP client stubs — stdin/stdout JSON-RPC framed messages only. This matches existing Phase 5 patterns in `internal/mcpserver/integration_test.go`.
- **D-17:** Flakiness budget: zero. If a concurrency test is flaky under `-race -count=10`, the test gets redesigned (e.g., deterministic synchronization points, condition-variable waits instead of sleeps), not quarantined. A flaky test in the merge gate is worse than no test.

### Static Analysis Gate (QA-04)

- **D-18:** `.golangci.yml` is the single source of truth for linters. Enabled set: `govet`, `staticcheck`, `errcheck`, `gofmt`, `gosimple` (per CLAUDE.md recommendation). Existing `ci.yml` already runs `golangci-lint-action@v8` with `version: latest` — tighten to a pinned version (`v2.6.0` or whatever the contemporary stable is at planning time) before the first tagged release so that CI is reproducible. Decision on exact pin deferred to the planner's first task (one-command look-up).
- **D-19:** No separate `staticcheck` job — subsumed in golangci-lint. Avoids two tools reporting the same issue twice.

### CI Workflow Structure (LD-5)

- **D-20:** Matrix restructure of the existing `ci.yml`: `lint` job (linux-only, unchanged), `build-and-gate` job (linux-only, keeps the size/startup/stdout/net-http gates), new `test` job with `strategy.matrix.os: [ubuntu-latest, macos-latest, windows-latest]`. The matrix `test` job runs `go build ./...`, `go vet ./...`, then the test command (D-02 decides race flag). All three (lint, build-and-gate, test) must pass for the overall CI status to be green.
- **D-21:** Caching: `actions/setup-go@v5` with `cache: true` on every job — the free GitHub cache handles module + build cache for Go 1.21+. No manual `actions/cache@v4` steps.
- **D-22:** Path filters: none. Even a docs-only PR runs CI — tradeoff is a few minutes of runner time vs the guarantee that every merge passes the full gate. Single-dev project; cost is trivial.

### Release Cadence & Versioning (LD-6)

- **D-23:** Release is **tag-driven**, manual. No scheduled releases, no release-please automation. Author pushes `git tag v0.1.0 && git push --tags` when a release is ready; GoReleaser takes it from there. Phase 10 documents this flow in README.
- **D-24:** Semver, with the first release being `v0.1.0` (pre-1.0 — API may change). Changelog: GoReleaser auto-generates from commit messages (`feat(...)`, `fix(...)`, etc.) — we've been writing conventional commits throughout phases 1–8, so the changelog will be clean.

### Claude's Discretion

- Exact layout of the `.goreleaser.yaml` file (commented sections, archive `name_template`, changelog filters) — use GoReleaser v2 defaults unless a requirement forces otherwise.
- Exact matrix `include`/`exclude` entries if a macOS amd64 runner is needed for symmetry vs relying solely on `macos-latest` (which is arm64 on Apple Silicon runners).
- Whether to add a `release.yml` comment-only dry-run step or use a separate `release-dry-run.yml` workflow — either is fine; prefer minimal file count.
- Any refactor of existing Makefile targets (`make build`, etc.) needed to keep local dev symmetric with CI.

### Folded Todos

None — no outstanding todos in `.planning/`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level
- `.planning/PROJECT.md` — vision, non-negotiables (pure-Go, ≤15 MB, ≤100 ms), Windows-secondary posture
- `.planning/REQUIREMENTS.md` — DIST-02, DIST-03, QA-01, QA-02, QA-03, QA-04 acceptance criteria
- `.planning/ROADMAP.md` — Phase 9 goal + success criteria (4 items)
- `CLAUDE.md` §Technology Stack — GoReleaser v2.15.4, goreleaser-action@v6, stdlib+testify/require test policy
- `.planning/STATE.md` §Decisions — all Phase 1–8 locked decisions that Phase 9 must not regress

### Phase-prior CONTEXT.md (read all — LD accumulation matters)
- `.planning/phases/01-foundation-enforcement-gates/01-CONTEXT.md` — size gate, startup gate, stdout-silence gate, CI skeleton, `net/http` ban, Makefile build target
- `.planning/phases/04-store-core-flock-atomic-writes/04-CONTEXT.md` — cross-process flock test pattern, 100-entry benchmark reference
- `.planning/phases/05-mcp-server-adapter/05-CONTEXT.md` — integration_test.go subprocess harness
- `.planning/phases/06-cli-dispatch-status/06-CONTEXT.md` — `runStatus` pure-writer pattern reused in QA-01 test fills
- `.planning/phases/07-cli-formatters/07-CONTEXT.md` — `TestPurge_CounterNotDecremented` as the canonical counter-monotonicity pin
- `.planning/phases/08-plugin-packaging/08-CONTEXT.md` — chain-wait.sh smoke harness, bash 3.2 constraints (not re-tested in Go matrix)

### External docs (read as needed during research / planning)
- https://goreleaser.com/customization/ — v2 config reference; especially `builds`, `archives`, `checksum`, `release` sections
- https://goreleaser.com/ci/actions/ — `goreleaser/goreleaser-action@v6` invocation, `args: release --clean`, `GITHUB_TOKEN` permissions
- https://github.com/golangci/golangci-lint-action — v8 options; how to pin `version: v2.6.0`
- https://github.com/actions/setup-go — v5 `cache: true` behavior, Go version matching from `go-version-file: go.mod`
- https://pkg.go.dev/cmd/go#hdr-Testing_flags — `-race`, `-count`, `-timeout` reference
- https://go.dev/doc/articles/race_detector — race detector platform support (linux+macOS full; Windows partial, documented rationale for QA-03 exclusion)

### No external specs beyond the above — the test-gap audit is the research-heavy step, and it reads existing `*_test.go` source, not docs.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Makefile` (from Phase 1) already builds stripped + trimpath: `go build -ldflags="-s -w" -trimpath -o mcp-chain ./cmd/mcp-chain`. GoReleaser's `builds` block should mirror these flags so local `make build` and release binaries are byte-for-byte comparable.
- `scripts/check-size.sh`, `scripts/check-startup.sh`, `scripts/check-stdout-silence.sh` — already invoked by `build-and-gate`. Do NOT duplicate on macOS/Windows.
- `scripts/smoke-chain-wait.sh` (Phase 8) — linux smoke for the plugin monitor. Runs in the Linux gate; not part of the matrix test job.
- `cmd/mcp-chain/main.go` already has `var version = "dev"` for `-X main.version=...` injection. GoReleaser's default `main.version` ldflag mapping Just Works.

### Established Patterns
- Integration tests live next to their package: `internal/store/integration_test.go`, `internal/mcpserver/integration_test.go`, `internal/cli/integration_test.go`. Any QA-02 fills follow the same layout.
- `cli.run*(out, errW io.Writer, path string, ...) int` pure-writer signature across status/list/purge/resolve (Phases 6+7) — QA-01 fills use these directly with `bytes.Buffer`, no subprocess needed for pure-function coverage.
- Subprocess integration pattern: `exec.Command("go", "run", "./cmd/mcp-chain", "serve")` + stdin pipe of framed JSON-RPC. Uses `t.TempDir()` + `XDG_STATE_HOME` env for isolation (per Phase 5 conventions).
- `OwnerToken` test setup: tests that need owner-scoped operations generate a per-test 32-char hex token via `crypto/rand` + `encoding/hex`.
- Commit convention: `feat(XX-NN): ...`, `test(XX): ...`, `chore(XX): ...`, `docs(XX): ...`. Conventional commit compatible → GoReleaser auto-changelog works cleanly.

### Integration Points
- `.github/workflows/ci.yml` is the single existing CI file — Phase 9 expands it in-place (does not replace).
- `.goreleaser.yaml` is a net-new file at repo root.
- `.github/workflows/release.yml` is a net-new file.
- `.golangci.yml` exists (Phase 1) — Phase 9 may need a minor tighten pass (pin version, confirm `staticcheck` enabled).

</code_context>

<specifics>
## Specific Ideas

- The release flow should be **runnable as a snapshot from any branch** so the author can dogfood it before tagging. GoReleaser `release --snapshot --clean --skip=publish` is the canonical incantation — add it as a Makefile target `make release-snapshot` so that Phase 10 docs can show it under "Release dry-run".
- Binary naming in archives: default GoReleaser `{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}` is fine; don't over-customize.
- GitHub release body: default conventional-commit changelog. The commit log from phases 2–8 was written conventionally throughout — changelogs will be clean without per-commit tweaking.

</specifics>

<deferred>
## Deferred Ideas

- **Homebrew tap / deb / rpm packaging** — GoReleaser supports all three; not in requirements. Revisit post-v1.0.
- **Code signing & notarization** — deferred per D-09. macOS gatekeeper warnings are acceptable for hobby-scale audience.
- **SBOM generation** — deferred; supply-chain posture revisit post-v1.0.
- **Continuous benchmarking in CI** — the size/startup gates are proxies. No `go test -bench` + regression tracking until the perf budget is actually at risk.
- **Nightly builds / scheduled release cadence** — tag-driven manual releases are sufficient.
- **Windows -race support** — wait for Go's Windows race detector to mature; revisit if Windows becomes a primary platform (currently secondary per PROJECT.md).
- **Release automation via release-please or similar** — manual tags are fine at this project's cadence.

</deferred>

---

*Phase: 09-ci-release*
*Context gathered: 2026-04-24*
