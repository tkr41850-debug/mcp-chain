# Phase 10: Docs & Dogfooding Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in 10-CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-24
**Phase:** 10-docs-dogfooding
**Mode:** `--auto` (recommended options selected without interactive confirmation)
**Areas auto-resolved:** README shape, Install path ordering, Example style, Namespace prose scope, Dogfooding pass shape, Phase 8/9 deferred triage, First tag strategy

---

## README Shape

| Option | Description | Selected |
|--------|-------------|----------|
| One-page comprehensive `README.md` at repo root | Sections: why, install, usage, state, NFS, upgrade, security, troubleshooting | ✓ |
| Short README + separate `docs/` directory | Extensibility; allows per-topic files | |
| README + GitHub Wiki | Wiki becomes authoritative; README is a pointer | |

**Auto choice:** One-page (D-01).
**Rationale:** Audience is small; single page keeps the install→usage→troubleshoot loop in one Ctrl-F. GitHub auto-renders it; `pkg.go.dev` also picks it up. No `docs/` tree to maintain.

---

## Install Path Ordering

| Option | Description | Selected |
|--------|-------------|----------|
| Plugin-first, manual `go install` second | Matches PROJECT.md audience (Claude Code users) | ✓ |
| Manual/binary first, plugin as convenience | Appeals to the "I want a binary" crowd | |
| Both equal with tabs | GitHub doesn't render tabs cleanly in raw README | |

**Auto choice:** Plugin-first (D-02).
**Rationale:** PROJECT.md locks the primary audience as Claude Code operators. Binary-first inverts the message; tabs don't render on GitHub.

---

## Example Style

| Option | Description | Selected |
|--------|-------------|----------|
| Copy-paste shell blocks with realistic word-IDs | Each example is a fenced block the user can paste | ✓ |
| Narrative with placeholders (`<your-id>`) | More explanation; less "paste-and-go" | |
| asciinema embeds | Prettier but breaks on GitHub mobile and in text-mode renders | |

**Auto choice:** Copy-paste with realistic IDs (D-03, D-04).
**Rationale:** The tool is a CLI. Copy-paste friction is the metric that matters. EFF-wordlist IDs (`otter`, `acid`) are more memorable and set the tone vs. `<placeholder>`.

---

## Namespace Prose Update Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Update REQUIREMENTS.md CMD-01..04 + HELPER-01 only | Prose swap only; no ID renumbering; PROJECT.md untouched | ✓ |
| Deep rewrite REQUIREMENTS.md + PROJECT.md | Full consistency; maximal diff | |
| Leave prose as-is; add an "aliases" note | Minimal diff; reader mental-model burden remains | |

**Auto choice:** REQUIREMENTS.md only (D-05, D-06).
**Rationale:** PROJECT.md uses descriptive terms (`register/wait/resolve`) that are namespace-agnostic. Rewriting it for no product value is churn. REQUIREMENTS.md CMD-IDs do reference the literal slash form — those need the correction.

---

## Dogfooding Pass Shape

| Option | Description | Selected |
|--------|-------------|----------|
| `scripts/dogfood.sh` (automated) + `DOGFOOD.md` (manual 2-session checklist) | Script covers CLI-only loop; checklist covers real plugin install on Linux+macOS | ✓ |
| Manual checklist only, no script | Less automation; every run is toil | |
| Video / asciinema only | Not reproducible by reviewers; not a test gate | |

**Auto choice:** Script + checklist (D-08, D-09, D-10).
**Rationale:** Script gates the CLI-level loop deterministically; checklist gates the end-to-end Claude Code flow that can't be automated without a headless Claude runner. Two artifacts, one reviewable pass.

---

## Phase 8/9 Deferred-Item Triage

| Item | Option | Selected |
|------|--------|----------|
| `smoke-chain-wait.sh` PATH fragility (Phase 9 out-of-scope) | Add `command -v go` pre-flight guard (D-11) | ✓ |
| `smoke-chain-wait.sh` PATH fragility | Document only; don't fix | |
| `$ARGUMENTS` shell-injection (Phase 8 WR-02) | Add "Security notes" section to README (D-12) | ✓ |
| `$ARGUMENTS` shell-injection | Silent inheritance of Claude Code ecosystem norm | |
| SDK `net/http` regression (Phase 9 pre-existing) | Reaffirm MCP-01 2026-04-24 acceptance; no fix (D-13) | ✓ |
| SDK `net/http` regression | Fork/vendor SDK subset | |

**Rationale:** Fix the script because the diff is tiny and makes the gate honest. Document the shell-injection norm because invisible security assumptions rot. Don't re-open MCP-01 because the transitive import is upstream behavior over a transport we don't expose.

---

## First Tag Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| `v0.1.0` — semver-zero first tag | Allows breaking changes before 1.0 commitment | ✓ |
| `v1.0.0` immediately | Locks the public surface | |
| Defer tagging past Phase 10 | Nothing ships from GoReleaser | |

**Auto choice:** `v0.1.0` (D-14).
**Rationale:** PROJECT.md's risk posture (single-dev, hobby-scale) + the 4-command slash surface isn't hardened against breaking refactors yet (e.g., `purge --all` default, status exit codes). Semver-zero buys a 0.x ramp before a 1.0 promise.

---

## Claude's Discretion

- Exact README wording, tone, and ordering within each section.
- Whether to include a badge row (CI, Go version) at top of README — planner chooses.
- Architecture diagram inclusion (likely no — PROJECT.md has one; README shouldn't duplicate).
- Whether `dogfood.sh` uses `trap` vs explicit cleanup on failure.

## Deferred Ideas

- `v1.0.0` promotion — gated on external-user validation post-tag.
- MCP SDK `net/http` carve-out — stays deferred; re-open only on user-reported incident.
- Homebrew / Scoop / deb / rpm packaging — post-v1.0.
- CI / coverage / go-report badges — planner discretion.
- `mcp-chain doctor` (OBS-03), shell completions (OBS-02), fuzzy prefix match (UX-02) — future milestones.
- asciinema video — post-tag polish.
