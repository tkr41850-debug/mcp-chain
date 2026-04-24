# Phase 9: CI Release, Cross-compile & Test Gates - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in 09-CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-24
**Phase:** 09-ci-release
**Mode:** `--auto` (recommended options selected without interactive confirmation)
**Areas auto-resolved:** CI matrix shape, GoReleaser config, version embedding, test coverage closeout, static analysis gate, CI workflow structure, release cadence

---

## CI Matrix Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Linux-only | Simplest; ships CI gate but no cross-platform verification | |
| Linux + macOS + Windows with single Go version (1.23) | Matches QA-03 exactly; race on linux+mac, no-race on win | ✓ |
| Full OS × Go-version matrix | 6–9 jobs; maximum coverage but expensive | |

**Auto choice:** Three-OS matrix with Go 1.23 pinned (D-01/D-02).
**Rationale:** QA-03 explicitly calls for macOS + Linux race, Windows non-race. Single Go version keeps release binary reproducible against CI output.

---

## GoReleaser Configuration

| Option | Description | Selected |
|--------|-------------|----------|
| Defaults only | Minimal config; accept GoReleaser defaults | |
| Tightened v2 config — explicit matrix, archives, checksums | Documented, reviewable, pinned | ✓ |
| GitHub Actions matrix without GoReleaser | Hand-rolled tar/zip/checksum steps | |

**Auto choice:** Tightened v2 `.goreleaser.yaml` (D-05..D-10).
**Rationale:** CLAUDE.md stack doc already pins GoReleaser v2.15.4; ~15 lines of YAML replaces 50+ lines of hand-rolled matrix steps and gets changelog + checksums for free.

---

## Code Signing / Notarization / SBOM

| Option | Description | Selected |
|--------|-------------|----------|
| Include now | macOS notarization + cosign + SBOM | |
| Defer to post-v1.0 | Ship binaries unsigned; revisit when audience grows | ✓ |

**Auto choice:** Defer (D-09).
**Rationale:** Not in requirements; single-dev + hobby-scale audience; adds Apple Developer ID + CI secrets complexity for no committed user.

---

## Version Embedding Validation

| Option | Description | Selected |
|--------|-------------|----------|
| Trust tag-time release only | No CI-side validation; tag push is the first real run | |
| CI snapshot dry-run | `goreleaser release --snapshot --skip=publish` on every push | ✓ |

**Auto choice:** CI snapshot dry-run (D-12).
**Rationale:** Catches `.goreleaser.yaml` drift before a broken tag ships. Cheap — adds ~30s to CI.

---

## Test Coverage Closeout Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Bulk-author full QA-01/02 suite from scratch | Assume prior phases left large gaps | |
| Gap audit first, targeted fills | Cross-reference existing tests against requirements | ✓ |

**Auto choice:** Audit-then-fill (D-13).
**Rationale:** Phases 2–8 authored tests alongside implementation. Bulk re-authoring duplicates coverage; audit surfaces real gaps.

---

## Static Analysis Gate

| Option | Description | Selected |
|--------|-------------|----------|
| Separate staticcheck job in CI | Dedicated linter action | |
| Roll staticcheck into existing golangci-lint | One tool, one report | ✓ |

**Auto choice:** Roll-in (D-18/D-19).
**Rationale:** `golangci-lint` v2 ships `staticcheck` as a default-enabled linter. Second tool reports the same issues twice — adds noise without new signal.

---

## Release Cadence

| Option | Description | Selected |
|--------|-------------|----------|
| Tag-driven manual | Author pushes `git tag v*`; GoReleaser reacts | ✓ |
| Scheduled nightly + manual tags | Continuous snapshot artifacts | |
| release-please automation | PRs auto-cut release branches from conventional commits | |

**Auto choice:** Manual tag-driven (D-23).
**Rationale:** Single-dev cadence. release-please overhead is worth it at team scale; not here.

---

## Claude's Discretion

- Exact `.goreleaser.yaml` layout (comments, `name_template` phrasing) — planner chooses per GoReleaser v2 idioms.
- Whether to add `macos-13` runner for amd64 macOS coverage alongside `macos-latest` (arm64) — planner decides based on runner minute budget.
- Whether to consolidate snapshot dry-run into `ci.yml` or extract to `release-dry-run.yml` — planner decides for minimal file count.
- Makefile target additions to mirror local dev against CI invocations.

## Deferred Ideas

- Homebrew / deb / rpm packaging.
- Code signing / notarization / SBOM.
- Continuous benchmarking.
- Scheduled / automated release cadence.
- Windows race detector (pending Go upstream support).
