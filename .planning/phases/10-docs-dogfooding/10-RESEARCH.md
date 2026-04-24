# Phase 10: Docs & Dogfooding Polish - Research

**Researched:** 2026-04-24
**Domain:** User-facing documentation + end-to-end dogfooding smoke + pre-tag polish
**Confidence:** HIGH (every claim is either grounded in the files in this repo at HEAD or in a CONTEXT.md locked decision)

## Summary

Phase 10 is **docs + glue**, not code. Nothing in `internal/**` or `cmd/**` changes. The output is five artifacts:
a single `README.md` at repo root (the new doc of record for external readers), a surgical prose swap in
`REQUIREMENTS.md` for CMD-01..04 and HELPER-01, a committed `scripts/dogfood.sh` end-to-end smoke, a manual
`DOGFOOD.md` checklist that gates the first tag, and a one-line PATH guard on the existing
`scripts/smoke-chain-wait.sh`. The `v0.1.0` tag push is author-driven and gated on dogfood green.

The risk surface is entirely about **drift**: README examples going stale when command names change, the
dogfood script racing `serve` startup, bash-3.2 vs bash-4+ pitfalls on the macOS-primary audience, and the
`chain-wait.sh` path moving from `scripts/` to `plugin/scripts/` in Phase 8 (REQUIREMENTS.md HELPER-01 and
CONTEXT.md both still say `scripts/chain-wait.sh` — that's a prose-swap target).

**Primary recommendation:** Treat README as the single source of truth for the external reader; keep it
≤300 lines; every shell block must be copy-paste-runnable against the binary actually shipped by Phase 9
GoReleaser.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**README Shape & Structure**
- **D-01:** Single `README.md` at repo root — one-page comprehensive doc (not multi-file `docs/`). Sections in this order: (1) Why / what it does, (2) Install (plugin path first, manual/binary path second), (3) Usage (slash commands `/mcp-chain:reg|wait|list|purge` first, manual CLI `status|list|purge|resolve` second), (4) State file path + permissions, (5) NFS / networked-filesystem caveat, (6) Upgrade / reload (`/mcp` list → restart), (7) Security notes (`$ARGUMENTS` shell-injection norm), (8) Troubleshooting, (9) License placeholder ("License TBD — see roadmap").
- **D-02:** Plugin install is the primary path. `/plugin install anthropics/mcp-chain` (or equivalent) demonstrated first. `go install` + manual `.mcp.json` wiring is shown second as the "without Claude Code" fallback.
- **D-03:** Usage examples use realistic word-IDs from the EFF list (e.g., `otter`, `acid`, `cable`) — not `foo`/`bar` placeholders.
- **D-04:** Code blocks are copy-paste shell (not narrative with placeholders). Each install/usage example is a single fenced block.

**Namespace Prose Update**
- **D-05:** Update `REQUIREMENTS.md` prose in CMD-01..04 and HELPER-01 so textual references read `/mcp-chain:reg`, `/mcp-chain:wait`, `/mcp-chain:list`, `/mcp-chain:purge`. No requirement-ID renumbering; prose-only swap.
- **D-06:** Do NOT rewrite PROJECT.md — leaving PROJECT.md as-is avoids diffs with no product value.
- **D-07:** Add a single sentence to README's "Commands" section noting that Claude Code prefixes plugin commands with `<plugin>:`, so `reg.md` is invoked as `/mcp-chain:reg`.

**Dogfooding Pass**
- **D-08:** Ship `scripts/dogfood.sh` — end-to-end smoke: build → serve background → `status <fake-id>` exit 1 → register → status exit 2 (pending) → resolve → status exit 0 → list shows entry → purge → list empty. Bash 3.2 safe. Runs ~2s.
- **D-09:** Ship `.planning/phases/10-docs-dogfooding/DOGFOOD.md` — manual checklist covering the full "2 sessions, real Claude Code plugin install, register in session A, wait in session B, resolve in A, see `continue` in B" path.
- **D-10:** Dogfooding success criteria: Linux run + macOS run both tick every box. If macOS run cannot be performed, annotate with "macOS: deferred pending runner access" and treat as `human_needed`, NOT a blocker for tagging.

**Phase 8/9 Deferred Items Triage**
- **D-11:** Fix `scripts/smoke-chain-wait.sh` PATH handling — add pre-flight `command -v go || { echo "…" >&2; exit 127; }` guard.
- **D-12:** Add README "Security notes" section covering (a) `$ARGUMENTS` un-sanitized; (b) plugin invocations assume trusted local shell; (c) not for hostile-tenant isolation.
- **D-13:** Do NOT fix the MCP SDK `net/http` regression. Reaffirm the 2026-04-24 MCP-01 acceptance.

**First Tag Strategy**
- **D-14:** First tag is `v0.1.0` — semver-zero because breaking changes remain possible before a `1.0.0` commitment.
- **D-15:** Tag push gated on: (a) README merged, (b) dogfood.sh green on Linux, (c) DOGFOOD.md checklist Linux-complete + macOS-complete or annotated-deferred. Author pushes `git tag v0.1.0 && git push --tags`.

### Claude's Discretion

- Exact README wording, tone, ordering within each section.
- Whether to include a badge row (CI status, Go version) at top.
- Whether the dogfood script uses `trap` vs explicit cleanup.
- Whether to include an architecture diagram.

### Deferred Ideas (OUT OF SCOPE)

- v1.0.0 promotion — after 1–2 real external users.
- MCP SDK `net/http` carve-out.
- Homebrew / Scoop packaging.
- CI badge, coverage badge, go-report badge (only if planner decides it doesn't clutter).
- `mcp-chain doctor` subcommand (OBS-03).
- Shell completions (OBS-02).
- asciinema/video demo.
- License allocation.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIST-04 | README covering why, install (plugin + manual), usage examples for all four slash commands + manual CLI | README skeleton below in §1; install/usage blocks sourced from `plugin/.mcp.json`, `plugin/commands/*.md`, `cmd/mcp-chain/main.go`; full §8 sections defined in §1 |
| MCP-01 | MCP SDK via stdio; `net/http` accepted as upstream regression on 2026-04-24 | Per D-13, README may optionally mention "What's in the binary" note but NO code fix. Phase 9 SUMMARY §Next-Phase-Readiness lists this explicitly pre-existing. No new research needed |
| CMD-01..04 | `/chain-reg|wait|list|purge` (legacy prose in REQUIREMENTS.md) → update to `/mcp-chain:reg|wait|list|purge` | Surgical swaps in §2. Commands already renamed on disk in Phase 8 (`plugin/commands/{reg,wait,list,purge}.md`). Prose is the only drift |
| HELPER-01 | `scripts/chain-wait.sh` path (legacy prose) — actual file lives at `plugin/scripts/chain-wait.sh` after Phase 8 LD-1 revision | §2 includes this path swap. Verified via `ls plugin/scripts/` |
| CORE-06 | State path resolution `$XDG_STATE_HOME/mcp-chain/state.json` or `~/.mcp-chain/state.json` fallback; 0700 dir, 0600 file | README §4 documents verbatim from `internal/statepath/statepath.go` package doc |

</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| User-facing install & usage docs | Documentation (README) | — | Rendered by GitHub + pkg.go.dev; first contact with external reader |
| Internal phase traceability | Planning docs (`.planning/**`) | — | Private project context; MUST NOT be linked from README |
| End-to-end smoke (automated) | Shell + build tooling (`scripts/dogfood.sh`) | Makefile (optional target) | Bash 3.2 safe; runs locally in ~2s; exercises compiled binary, not source |
| End-to-end smoke (manual) | Human checklist (`DOGFOOD.md`) | Claude Code runtime | Covers the only path that cannot be automated: two real Claude Code sessions with the plugin installed |
| Pre-flight guard on smoke-chain-wait.sh | Shell (`scripts/smoke-chain-wait.sh`) | — | One-line `command -v go` check; no behavior change when `go` is on PATH |
| Security disclosure | README §7 | Phase 8 WR-02 | Explicit acceptance of the `$ARGUMENTS` shell-injection norm |
| Release tag push | Human (git CLI) | GoReleaser (CI) | v0.1.0 push triggers Phase 9 release pipeline; human gates on DOGFOOD.md |

**Tier notes:** README is the ONLY user-facing surface this phase touches. Everything under `.planning/` stays internal. The dogfood script lives in `scripts/` (sibling of `smoke-chain-wait.sh`), not `plugin/scripts/` (that directory is shipped-to-the-user; dogfood is dev-machine-only).

## Standard Stack

No new libraries. Phase 10 is docs + shell. Reference the existing stack:

### Core (already installed)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| bash | 3.2+ | `dogfood.sh`, `smoke-chain-wait.sh` | macOS default. Project convention since Phase 8 |
| GitHub Flavored Markdown | — | README rendering on GitHub + pkg.go.dev | Audience-native |
| GoReleaser v2.15.x | pinned via `~> v2` in action | Tag-triggered release | Phase 9 pipeline fires on `v*` — no changes |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Single `README.md` | Multi-file `docs/` | D-01 rejects: one-page is better for a 4-command plugin |
| Makefile `make dogfood` target | Plain `bash scripts/dogfood.sh` | Discretion per D-08; recommend adding `dogfood:` phony to `Makefile` so it appears beside `release-snapshot` / `ci-local` (existing Phase 9 pattern) |
| asciinema recording | Plain-text 2-session dialogue | Deferred per CONTEXT.md |

**Installation:** No new commands. Existing `go install` path is referenced in README §2.

## Architecture Patterns

### System Architecture Diagram

```
                     GitHub repo (public)
                             │
           ┌─────────────────┼─────────────────────────────┐
           │                 │                             │
      README.md        .planning/**               /plugin + /cmd
   (user-facing)     (internal, private)            (shipped code)
      │                                                     │
      │ reads                                               │ compiled by
      ▼                                                     ▼
  External user ──── /plugin install ─────▶ Claude Code + mcp-chain binary
      │                                                     │
      │ reads §2 "Install" block                            │ state.json
      │ runs `/mcp-chain:reg "build passes"` in session A   │ under flock
      │ copies word-ID `otter` to session B                 │
      │ runs `/mcp-chain:wait otter` in session B           │
      │ runs `/mcp-chain:resolve otter` in session A        │
      │ session B prints `continue`                         │
      ▼                                                     ▼
  Dogfooded end-to-end          scripts/dogfood.sh reproduces CLI-side

                     Gate to v0.1.0
                             │
                    ┌────────┴────────┐
         scripts/dogfood.sh       DOGFOOD.md (manual)
         (CI-automatable)         (requires real Claude Code)
                    └────────┬────────┘
                             ▼
                  author: git tag v0.1.0 && git push --tags
                             │
                             ▼
              .github/workflows/release.yml (Phase 9)
                             │
                             ▼
              6 archives + checksums.txt on GitHub Releases
```

### Recommended Project Structure

```
.
├── README.md                  # NEW (Phase 10 primary artifact)
├── scripts/
│   ├── dogfood.sh             # NEW (D-08)
│   ├── smoke-chain-wait.sh    # MODIFIED (D-11 PATH guard)
│   └── … (8 existing check-*.sh)
├── .planning/
│   ├── phases/10-docs-dogfooding/
│   │   ├── DOGFOOD.md         # NEW (D-09 manual checklist)
│   │   ├── 10-CONTEXT.md      # (exists)
│   │   └── 10-RESEARCH.md     # (this file)
│   └── REQUIREMENTS.md        # MODIFIED (D-05 prose swap)
├── Makefile                   # OPTIONAL: add `dogfood:` phony target
└── (everything else unchanged)
```

### Pattern 1: README sections as single-responsibility fenced blocks
**What:** Each install/usage example is one fenced code block the reader can copy verbatim — no `<YOUR_ID>` placeholders, no `...` ellipses in commands.
**When to use:** Every install and usage example in README.
**Example:**
```markdown
## Install

### With Claude Code (recommended)

\`\`\`
/plugin install anthropics/mcp-chain
\`\`\`

### Without Claude Code

\`\`\`
go install github.com/anthropics/mcp-chain/cmd/mcp-chain@latest
\`\`\`

Then add to your MCP client config:

\`\`\`json
{
  "mcpServers": {
    "mcp-chain": {
      "command": "mcp-chain",
      "args": ["serve"]
    }
  }
}
\`\`\`
```

### Anti-Patterns to Avoid
- **Linking README to `.planning/**`:** Never. It's project-internal scaffolding.
- **Placeholder IDs (`foo`, `bar`, `<ID>`):** D-03 forbids. Use real EFF words: `otter`, `acid`, `cable`, `prune`.
- **Multi-paragraph security advisory:** D-12 says 3 bullets max.
- **Re-documenting `forbidigo` / `errcheck` / internal lint rules:** These are internal; README audience doesn't care.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Test that serve process started" | Sleep-loop polling PID + strace for socket | Short `sleep` + first `status` call will naturally retry; the startup budget is <100ms (verified Phase 1) | serve is stdio, not socket — no port to poll |
| Automate "2 Claude Code sessions talking" | MCP protocol replay harness | `DOGFOOD.md` manual checklist | Claude Code plugin install is user-interactive; automating it is 10x the work of a checklist |
| `timeout` wrapper portability | Custom PID + SIGKILL + wait dance | macOS bash 3.2 has no `timeout(1)` by default — use a `&`+`sleep`+`kill` pattern as in `smoke-chain-wait.sh` already does | Matches existing project idiom |
| README table-of-contents generator | Auto-inject TOC via CI | Hand-written, kept ≤300 lines | One-page doc doesn't need a TOC |
| "Lint README for broken links" | `markdown-link-check` GH action | Phase 10 ships no new CI | Links are few and hand-audited in plan-check |

**Key insight:** This phase produces four text artifacts and one one-line shell patch. Any added automation or tooling is out of scope per CONTEXT.md.

## Runtime State Inventory

> Phase 10 is docs + scripts only. No runtime state changes.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no state schema, database, or persistent key changes | None |
| Live service config | None — plugin/*.json manifests unchanged | None |
| OS-registered state | None | None |
| Secrets/env vars | None — README documents `XDG_STATE_HOME` and `MCP_CHAIN_BIN` which are user-set, not stored | None |
| Build artifacts | First `v0.1.0` tag will produce 6 new archives under `dist/` (GoReleaser) — that's the release itself | After tag push, verify CI uploads to GitHub Releases |

**Nothing found in Stored data / Live config / OS state / Secrets — verified by reading the full CONTEXT.md "In scope" list: README, REQUIREMENTS prose swap, dogfood.sh, DOGFOOD.md, smoke-chain-wait.sh PATH guard. None modify state.json, plugin manifests, MCP tool descriptions, or env var semantics.**

## Common Pitfalls

### Pitfall 1: README examples go stale when commands rename
**What goes wrong:** Phase 8 already renamed legacy `/chain-*` → `/mcp-chain:*`. If README hard-codes `/mcp-chain:reg` and a future phase namespaces differently, README silently rots.
**Why it happens:** No automated check ties README to `plugin/commands/*.md`.
**How to avoid:** Add a single grep-based plan-check validation row: `grep -E '/mcp-chain:(reg|wait|list|purge)' README.md | wc -l` ≥ 4. Include this in plan validation table.
**Warning signs:** User reports "command not found" after a plugin update.

### Pitfall 2: `dogfood.sh` races `serve` startup
**What goes wrong:** Script launches `serve` in background, immediately calls `status <id>` via a second `mcp-chain` process. But `serve` is only needed for MCP tool calls (register/resolve over stdio). The CLI subcommands (`status`, `list`, `purge`, `resolve`) read the state file DIRECTLY under flock — they do NOT speak to the serve process. **So `dogfood.sh` doesn't actually need `serve` at all for D-08's flow** — it can use `mcp-chain resolve --force` instead of an MCP `resolve` call.
**Why it happens:** Mental model mismatch between MCP server and CLI. Phase 9's `TestConcurrentWaiters_AllSeeResolve` already uses the `resolve --force` shortcut to sidestep this.
**How to avoid:** Follow Phase 9 N-3 pattern — use CLI-only flow (`register` MCP tool not required: dogfood.sh can seed via `scripts/internal/seed_pending.go` like `smoke-chain-wait.sh` does, OR can launch `serve` + pipe an `initialize`+`register` JSON-RPC sequence on stdin). Recommendation: use the seed helper (simpler, already exists, no bg process).
**Warning signs:** Flaky CI runs that only reproduce on slower runners.

### Pitfall 3: bash 3.2 vs bash 4+ divergence
**What goes wrong:** Developer writes `dogfood.sh` on Linux (bash 5), ships, macOS CI fails on `declare -A`, `[[ … =~ … ]]` PCRE, `${var,,}`, process substitution.
**Why it happens:** Linux-first development, macOS-secondary testing.
**How to avoid:** Lint with existing `scripts/check-chain-wait-bashisms.sh` pattern. Concretely disallow: `declare -A`, `readarray`, `mapfile`, `[[ =~ ]]`, `${var,,}`, `${var^^}`, `${var:-}` is fine but `${var:+}` should be vetted, `<()` process substitution. The `smoke-chain-wait.sh` is the canonical reference — if it parses with `bash -n` under macOS bash 3.2, dogfood.sh follows its style.
**Warning signs:** `dogfood.sh` runs locally, fails on macOS GitHub runner.

### Pitfall 4: `command -v go` stderr/stdout behavior varies by shell
**What goes wrong:** `command -v go` writes to stdout when found (path), nothing when not found. Some shells (ash, dash busybox variants) accept `command -v` silently; others may emit errors. Under `set -eu`, an absent binary causes exit code 1, which we want to translate to a readable error.
**Why it happens:** `command` is a builtin; its behavior is POSIX-specified but corner cases exist.
**How to avoid:** Use the guarded form `if ! command -v go >/dev/null 2>&1; then … exit 127; fi`, NOT `command -v go || exit 127` — the latter can be short-circuited by `set -e` interactions. Route error message to stderr, use exit 127 (matches bash's "command not found" convention, matching Phase 9 §"Next-Phase-Readiness" recommendation).
**Warning signs:** CI fails with opaque "command not found" without the descriptive error.

### Pitfall 5: Claude Code plugin reload semantics vary by version
**What goes wrong:** User updates the plugin (new tag pushed, GoReleaser attaches new binaries), but Claude Code keeps serving the old cached plugin. README tells user to "restart Claude Code" but the actual reload step may be `/plugin reload` OR `/mcp` list + pick + restart OR full Claude Code restart depending on version.
**Why it happens:** Claude Code plugin reload surface is still evolving.
**How to avoid:** README §6 should cite BOTH: "`/mcp` list → pick mcp-chain → restart" AND "if that doesn't work, fully restart Claude Code." Don't claim a single command is authoritative.
**Warning signs:** User reports "I installed v0.1.0 but my session still uses dev binary."

### Pitfall 6: `chain-wait.sh` path drift (REQUIREMENTS.md vs actual)
**What goes wrong:** REQUIREMENTS.md HELPER-01 says `scripts/chain-wait.sh` but Phase 8 LD-1 revision moved it to `plugin/scripts/chain-wait.sh`. Stale prose.
**Why it happens:** Phase 8 SUMMARY logged the move; REQUIREMENTS.md never caught up.
**How to avoid:** Include the path fix in the D-05 surgical swap. See §2 below — HELPER-01 gets a `plugin/` prefix swap too.
**Warning signs:** Reader follows REQUIREMENTS.md and can't find the monitor script.

## Code Examples

### README §2: Install block (copy-paste)
```markdown
### With Claude Code (recommended)

\`\`\`
/plugin install anthropics/mcp-chain
\`\`\`

Claude Code will download the latest release binary for your OS/arch from GitHub
Releases and install it under `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`.

### Without Claude Code

Install the binary:

\`\`\`
go install github.com/anthropics/mcp-chain/cmd/mcp-chain@latest
\`\`\`

Then wire it into your MCP client. For a generic `.mcp.json`:

\`\`\`json
{
  "mcpServers": {
    "mcp-chain": {
      "command": "mcp-chain",
      "args": ["serve"]
    }
  }
}
\`\`\`
```

### README §3: Two-session usage demo (verbatim text)
```markdown
### Two-session demo

Session A (the registrant):

\`\`\`
/mcp-chain:reg build passes
→ otter
\`\`\`

Session B (the waiter), started in a different terminal or conversation:

\`\`\`
/mcp-chain:wait otter
(polls once per second, silent until resolve)
\`\`\`

Back in Session A, once the condition is satisfied:

\`\`\`
/mcp-chain:resolve otter
\`\`\`

Session B prints:

\`\`\`
continue
\`\`\`

and unblocks.
```

### README §4: State path (sourced verbatim from `internal/statepath/statepath.go`)
```markdown
## State file

mcp-chain writes to a single JSON file under an XDG-compliant path:

\`\`\`
$XDG_STATE_HOME/mcp-chain/state.json       (if $XDG_STATE_HOME is set)
~/.mcp-chain/state.json                    (fallback)
\`\`\`

Permissions: parent directory `0700`, file `0600`. Writes are atomic
(temp-file + `fsync` + rename under `flock(2)` exclusive lock).
```

### README §5: NFS caveat (one paragraph, bold)
```markdown
## Networked filesystems

**Do not place the state file on NFS, SMB, or CIFS.** `flock(2)` semantics
over networked filesystems are not reliable — you can get lost updates,
duplicate word-IDs, and corrupt state. Use a local filesystem (ext4, APFS,
NTFS on NTFS-backed drives). If `$HOME` is on an NFS mount, set
`$XDG_STATE_HOME` to a local path.
```

### README §7: Security notes (3 bullets, per D-12)
```markdown
## Security notes

- `/mcp-chain:wait` and `/mcp-chain:purge` pass `$ARGUMENTS` to a bash
  subprocess without per-token sanitization. Same threat model as every
  Claude Code plugin: the user is assumed to trust their own slash-command
  input.
- Plugin invocations run with the same privilege as your Claude Code
  session. mcp-chain does not sandbox, chroot, or drop privileges.
- Do not use mcp-chain for trust isolation on multi-tenant machines.
  The state file is per-user; it assumes a single trusted operator.
```

## State of the Art

No state-of-the-art deltas. Phase 10 ships no library upgrades.

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Legacy command names `/chain-reg|wait|list|purge` | Namespaced `/mcp-chain:reg|wait|list|purge` | Phase 8 LD-14 (2026-04-24) | Files already renamed on disk; REQUIREMENTS.md prose still legacy — this phase fixes prose only |
| `scripts/chain-wait.sh` | `plugin/scripts/chain-wait.sh` | Phase 8 LD-1 revision | REQUIREMENTS.md HELPER-01 prose still legacy — this phase fixes |

**Deprecated/outdated:**
- Legacy `/chain-*` invocations — any documentation mentioning them needs the `/mcp-chain:*` form.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `/plugin install anthropics/mcp-chain` is the correct Claude Code install syntax for plugins hosted under `anthropics/mcp-chain` | §1 README skeleton §Install; CONTEXT.md D-02 references it | If syntax differs (e.g., needs a different verb or marketplace URL), README §2 example is wrong. Mitigation: verify against the live Claude Code plugin docs and against plugin-ref URL in canonical_refs when drafting README. Not researched in this session |
| A2 | `go install github.com/anthropics/mcp-chain/cmd/mcp-chain@latest` resolves to the correct import path | §1 README skeleton §Install fallback | Module path is in go.mod — planner should `grep "^module" go.mod` before writing the block verbatim |
| A3 | GitHub Releases latest asset URL pattern matches Phase 9 GoReleaser output | §6 tag procedure | Phase 9 SUMMARY confirms archive names. If user wants a direct-download install line in README, planner should cite the actual archive name format |
| A4 | `/plugin reload` or equivalent exists in current Claude Code | Pitfall 5 | If not, README §6 should say "fully restart Claude Code" as the only supported reload path. Low impact — worst case the extra sentence is removed |

**If this table looks short:** It's because Phase 10 scope is tightly bounded by CONTEXT.md and doesn't introduce new technology choices. The above four items are the only un-verified assumptions.

## Open Questions

1. **Does CONTEXT.md CMD-04 description in REQUIREMENTS.md need the "errors" wording updated?**
   - What we know: Current REQUIREMENTS.md line 34 says "bare `/chain-purge` errors". After the swap it will read "bare `/mcp-chain:purge` errors" — that's syntactically fine but a `:` followed by nothing is awkward visually.
   - What's unclear: Whether to hyphen-escape or inline-code the namespaced form. Recommendation: inline-code it as `` `/mcp-chain:purge` ``, which the existing prose already does for `/chain-purge`.
   - Recommendation: Sed swap preserves surrounding backticks.

2. **Does README need a `license TBD` section at all?**
   - What we know: CONTEXT.md D-01 §9 says yes (placeholder).
   - What's unclear: Whether an unlicensed README on GitHub creates issues for users who want to redistribute.
   - Recommendation: Include a single line `License: TBD (see project roadmap)`. Keep short.

3. **Should `dogfood.sh` be wired into CI?**
   - What we know: Phase 9 runs `go test -race` and release-dry-run on PR. CONTEXT.md D-08 says `scripts/dogfood.sh` runs locally in ~2s.
   - What's unclear: Whether to add a CI step that runs `dogfood.sh` on ubuntu-latest + macos-latest (would catch regressions during future work).
   - Recommendation: OUT OF SCOPE per CONTEXT.md boundary. Ship dogfood.sh as a committed script; future phase can add a CI matrix entry.

## Environment Availability

Phase 10 runs entirely on dev machine + GitHub Actions runners. Dependencies required:

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| bash | dogfood.sh, smoke-chain-wait.sh | ✓ | 3.2+ assumed | — (mandatory) |
| go | dogfood.sh (for `go build`), smoke-chain-wait.sh (for `go build` + `go run`) | ✓ | 1.25 (go.mod) | — (mandatory; PATH guard per D-11) |
| git | tag push | ✓ | any | — |
| GitHub Actions | Phase 9 release workflow on tag | ✓ | — | None — upstream requirement |
| Claude Code plugin runtime | DOGFOOD.md manual checklist | ✓ on author's machine | latest | "macOS: deferred pending runner access" escape hatch per D-10 |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — all tooling is already on the target machine based on Phase 1..9 green status.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Shell scripts + grep-based validation (no Go test changes) |
| Config file | `.golangci.yml` (unchanged); Makefile optional dogfood target |
| Quick run command | `bash scripts/dogfood.sh` |
| Full suite command | `make ci-local && bash scripts/dogfood.sh && bash scripts/smoke-chain-wait.sh` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| DIST-04 / SC-1 | README covers `/mcp-chain:reg\|wait\|list\|purge` + manual CLI | grep gate | `grep -cE '/mcp-chain:(reg\|wait\|list\|purge)' README.md` returns ≥ 4; plus `grep 'go install' README.md \|\| grep -i 'manual' README.md` | ❌ Wave 0 (README new) |
| DIST-04 / SC-2 | README documents state path + NFS caveat + reload | grep gate | `grep -iE 'XDG_STATE_HOME\|~/.mcp-chain' README.md` and `grep -iE 'NFS\|networked.*filesystem\|flock.*reliable' README.md` and `grep -iE '/mcp\|restart' README.md` | ❌ Wave 0 |
| DIST-04 (security) | README §7 covers `$ARGUMENTS` norm | grep gate | `grep -E '\$ARGUMENTS\|shell.*injection' README.md` returns ≥ 1 | ❌ Wave 0 |
| DIST-04 / SC-3 | dogfood.sh exits 0 end-to-end | integration | `bash scripts/dogfood.sh; echo "exit=$?"` expect `exit=0` | ❌ Wave 0 (script new) |
| DIST-04 / SC-3 | DOGFOOD.md Linux manual path | manual | Human checkbox run; no automatic command | ❌ Wave 0 (doc new) |
| HELPER-01 | `smoke-chain-wait.sh` PATH guard fires on missing go | shell test | `PATH=/nonexistent bash scripts/smoke-chain-wait.sh; echo "exit=$?"` expect `exit=127` | ✓ script exists; guard is modification |
| CMD-01..04 + HELPER-01 | REQUIREMENTS.md prose swapped | grep gate | `! grep -E '/chain-(reg\|wait\|list\|purge)' .planning/REQUIREMENTS.md` (should return 1, meaning 0 matches) AND `grep -c '/mcp-chain:(reg\|wait\|list\|purge)' .planning/REQUIREMENTS.md` ≥ 5 | ✓ (file exists; prose-only edit) |
| — (release gate) | v0.1.0 tag triggers CI and uploads 6 archives | observational | `gh release view v0.1.0 --json assets --jq '.assets\|length'` == 7 (6 archives + checksums.txt) | Post-tag |

### Sampling Rate
- **Per task commit:** `bash scripts/dogfood.sh` (for dogfood task); `make ci-local` (for surgical edits)
- **Per wave merge:** `make ci-local && bash scripts/dogfood.sh && bash scripts/smoke-chain-wait.sh && grep-based README audits`
- **Phase gate:** all above green + DOGFOOD.md checklist Linux-complete + tag pushed by human

### Wave 0 Gaps
- [ ] `README.md` — new file, must be created before SC-1/SC-2 grep gates run
- [ ] `scripts/dogfood.sh` — new file, must exist before dogfood gate runs
- [ ] `.planning/phases/10-docs-dogfooding/DOGFOOD.md` — new manual checklist
- [ ] `.planning/REQUIREMENTS.md` — in-place edit (not a new file)
- [ ] `scripts/smoke-chain-wait.sh` — in-place edit (PATH guard)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | n/a (single-user local tool) |
| V3 Session Management | no | n/a |
| V4 Access Control | partial | OwnerToken session-link (already in CORE-08; not touched by Phase 10) |
| V5 Input Validation | partial | `$ARGUMENTS` passthrough accepted by Phase 8 WR-02; documented, not enforced |
| V6 Cryptography | no | n/a — no new crypto |
| V14 Configuration | partial | State file 0600, dir 0700; already enforced by CORE-05 |

### Known Threat Patterns for shell scripts + markdown docs

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| `$ARGUMENTS` unquoted → shell injection at Claude Code prompt-render time | Tampering + Elevation | Documented explicitly in README §7 per D-12; threat model accepts this as Claude Code ecosystem norm |
| `dogfood.sh` creates tempfiles without mode check | Information Disclosure | Use `mktemp -d` (as `smoke-chain-wait.sh` does); tempdir 0700 by default on Linux/macOS |
| README curl-pipe-bash patterns | Tampering (upstream artifact substitution) | README does NOT recommend `curl | bash`. Install is `/plugin install` (Claude Code verifies) or `go install` (Go toolchain verifies via checksums) |
| Stale docs that mis-represent current CLI | Repudiation (accidental) | grep gates in Validation Architecture §Phase Requirements → Test Map |

---

## §1. README Draft Skeleton

**Target length:** ≤300 lines, fits one GitHub scroll.

| Section | Purpose | Target lines | Notes |
|---------|---------|--------------|-------|
| Title + one-liner tagline | Hook; rendered as h1 + first p on GitHub | 2 | "mcp-chain — register/wait/resolve locks for N Claude Code sessions." |
| Why / what it does | 3-line context without prior knowledge | 8–12 | Paraphrase PROJECT.md §"Core Value" tuned for external reader |
| Install | Plugin-first block, then manual fallback (per D-02) | 30–40 | Two fenced blocks + 2 prose lines |
| Usage | 2-session text demo (per D-03, D-04); CLI subcommands table | 40–50 | Real IDs `otter`, `acid`, `cable`. CLI table: `serve`, `status <id>`, `list`, `purge`, `resolve <id> [--force]`, `--version` |
| Commands | D-07 sentence about `/mcp-chain:` prefix + single table mapping command to `plugin/commands/*.md` | 15–20 | |
| State file | Path (XDG + HOME fallback), permissions, atomic-write note | 15 | Verbatim from `internal/statepath/statepath.go` |
| Networked filesystems (NFS caveat) | Single paragraph, bold lead (per §specifics) | 6–8 | |
| Upgrade / reload | `/plugin reload` or "restart Claude Code"; `/mcp` list for diagnostic (per D-01 §6) | 10 | Acknowledge uncertainty per Pitfall 5 |
| Security notes | 3 bullets (per D-12) | 10 | |
| Troubleshooting | 4–5 common failures (binary not found, state.json corrupt, unknown ID, NFS, bash 3.2) | 20–30 | Each with symptom → fix |
| License | One line placeholder (`License: TBD — see roadmap`) | 2 | |
| Contributing (optional) | One line pointing at `CLAUDE.md` IF useful for external contributors; D-14 semver warning | 4–6 | Discretion — include only if it's ≤6 lines |

**Total est:** 170–240 content lines, well under 300 cap.

**Copy-paste block shapes** (already sketched in §Code Examples above):
1. Plugin install (one fenced block, ~3 lines)
2. `go install` + `.mcp.json` (two blocks, ~12 lines combined)
3. 2-session demo (one blockquote-style "asciinema text" block, ~18 lines)
4. State-path fenced block (4 lines)
5. NFS paragraph (no code block, prose only)

## §2. REQUIREMENTS.md Surgical Edits (sed-ready)

**Grep first** to confirm every `/chain-` reference in REQUIREMENTS.md:

```bash
grep -nE '/chain-(reg|wait|list|purge)' .planning/REQUIREMENTS.md
```

**Expected matches (verified via `grep -rn` in this session):**

| Line | Current (legacy) | Target (namespaced) |
|------|------------------|---------------------|
| 31 | `` **CMD-01**: `/chain-reg [condition]` `` | `` **CMD-01**: `/mcp-chain:reg [condition]` `` |
| 32 | `` **CMD-02**: `/chain-wait [id] [--timeout DURATION]` `` | `` **CMD-02**: `/mcp-chain:wait [id] [--timeout DURATION]` `` |
| 33 | `` **CMD-03**: `/chain-list` `` | `` **CMD-03**: `/mcp-chain:list` `` |
| 34 | `` **CMD-04**: `/chain-purge [id \| --all \| --resolved]` … `/chain-purge` errors `` | `` **CMD-04**: `/mcp-chain:purge [id \| --all \| --resolved]` … `/mcp-chain:purge` errors `` (two occurrences on this line) |
| 39 | `` **HELPER-01**: Repo ships `scripts/chain-wait.sh` … `/chain-wait` instructs Claude `` | `` **HELPER-01**: Repo ships `plugin/scripts/chain-wait.sh` … `/mcp-chain:wait` instructs Claude `` (also fixes the path drift per Pitfall 6) |
| 86 | `` \| Auto-expiration … Explicit `/chain-purge` only `` | `` \| Auto-expiration … Explicit `/mcp-chain:purge` only `` |
| 137 | `` … slash-command wrapper prompts land in Phase 8 (`/chain-list`, `/chain-purge`) `` | `` … slash-command wrapper prompts land in Phase 8 (`/mcp-chain:list`, `/mcp-chain:purge`) `` |

**Sed-ready commands** (BSD sed-safe and GNU sed-safe; use an in-place with an empty `-i ''` for macOS compatibility):

```bash
# Do these in order; the HELPER-01 path swap MUST precede the generic command renames
sed -i.bak 's|`scripts/chain-wait.sh`|`plugin/scripts/chain-wait.sh`|g' .planning/REQUIREMENTS.md
sed -i.bak 's|/chain-reg|/mcp-chain:reg|g'   .planning/REQUIREMENTS.md
sed -i.bak 's|/chain-wait|/mcp-chain:wait|g' .planning/REQUIREMENTS.md
sed -i.bak 's|/chain-list|/mcp-chain:list|g' .planning/REQUIREMENTS.md
sed -i.bak 's|/chain-purge|/mcp-chain:purge|g' .planning/REQUIREMENTS.md
rm .planning/REQUIREMENTS.md.bak
```

**Post-swap verification:**
```bash
# Should return 0 lines:
grep -nE '/chain-(reg|wait|list|purge)' .planning/REQUIREMENTS.md
# Should return ≥ 7 lines (legacy counts):
grep -c '/mcp-chain:' .planning/REQUIREMENTS.md
```

**Do NOT touch:**
- PROJECT.md (per D-06)
- `.planning/research/**` (internal research files; stale by design)
- `.planning/phases/*/` prior-phase artifacts (history)
- `.planning/ROADMAP.md` (this is a DIFFERENT scope decision — NOT in CONTEXT.md; planner must confirm before touching. Currently roadmap SC-1 line 139 still says `/chain-reg` etc. Recommendation: ROADMAP.md is a planning artifact, not user-facing, and D-05 scope is tight — leave ROADMAP alone)

## §3. `scripts/dogfood.sh` Skeleton

**Design:** Mirror `scripts/smoke-chain-wait.sh` structure (bash 3.2, `set -eu`, `mktemp -d`, `trap cleanup EXIT`, stderr-only progress). Does NOT launch `serve` — uses CLI-only flow per Pitfall 2, with the `scripts/internal/seed_pending.go` helper that already exists from Phase 8. Runs ~2s.

```bash
#!/usr/bin/env bash
# dogfood.sh — end-to-end smoke test for mcp-chain CLI flow.
# build → status unknown (1) → seed pending → status pending (2) → resolve → status resolved (0)
#      → list nonempty → purge --all → list empty
# Requires: bash 3.2+, go in PATH, mktemp.

set -eu

# -------- PATH guard (matches D-11 pattern) ---------------------------------
if ! command -v go >/dev/null 2>&1; then
  echo "dogfood: go not found on PATH" >&2
  exit 127
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
BIN="$WORK/mcp-chain"
export XDG_STATE_HOME="$WORK/state"
mkdir -p "$XDG_STATE_HOME"

cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

echo "dogfood: building binary" >&2
go build -o "$BIN" "$ROOT/cmd/mcp-chain"

# ----- step 1: unknown id → exit 1 ------------------------------------------
set +e
"$BIN" status no-such-id >/dev/null 2>&1
CODE=$?
set -e
[ "$CODE" -eq 1 ] || { echo "dogfood: step 1 FAIL unknown-id exit=$CODE want 1" >&2; exit 1; }
echo "dogfood: step 1 OK (unknown exit 1)" >&2

# ----- step 2: seed pending entry → status exit 2 ---------------------------
ID="$(go run "$ROOT/scripts/internal/seed_pending.go")"
[ -n "$ID" ] || { echo "dogfood: step 2 FAIL empty id from seed" >&2; exit 1; }
set +e
"$BIN" status "$ID" >/dev/null 2>&1
CODE=$?
set -e
[ "$CODE" -eq 2 ] || { echo "dogfood: step 2 FAIL pending exit=$CODE want 2" >&2; exit 1; }
echo "dogfood: step 2 OK (pending exit 2 for id=$ID)" >&2

# ----- step 3: resolve --force → status exit 0 ------------------------------
"$BIN" resolve "$ID" --force >/dev/null
set +e
"$BIN" status "$ID" >/dev/null 2>&1
CODE=$?
set -e
[ "$CODE" -eq 0 ] || { echo "dogfood: step 3 FAIL resolved exit=$CODE want 0" >&2; exit 1; }
echo "dogfood: step 3 OK (resolved exit 0)" >&2

# ----- step 4: list shows the entry -----------------------------------------
OUT="$("$BIN" list)"
echo "$OUT" | grep -q "$ID" || { echo "dogfood: step 4 FAIL list missing $ID" >&2; exit 1; }
echo "dogfood: step 4 OK (list shows entry)" >&2

# ----- step 5: purge --all → list empty -------------------------------------
"$BIN" purge --all >/dev/null
OUT="$("$BIN" list)"
# "empty" means list prints 0 data rows; accept either no output or header-only.
DATA_LINES="$(echo "$OUT" | grep -cE '^[a-z]+[[:space:]]+' || true)"
[ "$DATA_LINES" -eq 0 ] || { echo "dogfood: step 5 FAIL list nonempty after purge" >&2; exit 1; }
echo "dogfood: step 5 OK (list empty after purge)" >&2

echo "dogfood: all steps passed" >&2
```

**Expected exit codes at each step:**
| Step | Expected | Source |
|------|----------|--------|
| 1 unknown-id | 1 | Phase 6 kong exit code convention |
| 2 pending | 2 | Phase 6 CORE-01 |
| 3 resolved | 0 | Phase 6 CORE-01 |
| 4 list non-empty | 0 (CLI) | Phase 7 |
| 5 list empty after purge | 0 (CLI) + grep count = 0 | Phase 7 purge |

**Pitfalls to call out in plan actions:**
- `trap cleanup EXIT` — ensures `$WORK` is removed on failure or success.
- NO background `serve` process — avoids Pitfall 2 race. Uses `resolve --force` per Phase 9 N-3 lesson.
- `XDG_STATE_HOME` exported before first binary invocation, and because dogfood.sh invokes CLI commands directly (not via subprocesses-of-subprocesses), no further env propagation is needed — unlike Phase 9 integration tests that needed `cmd.Env = env` on each spawn.
- Bash 3.2 safe: no `[[ ]]`, no `$'...'`, no process substitution, no `declare -A`. Passes `scripts/check-chain-wait-bashisms.sh` if copied.
- `scripts/internal/seed_pending.go` is already a `//go:build ignore` file — `go run` works but `go build ./...` ignores it (verified in Phase 8 SUMMARY).

## §4. `scripts/smoke-chain-wait.sh` PATH Guard — Exact Diff

**Current file line 10** (from the Read result): `set -eu` immediately followed by `SMOKE_ROOT="$(cd ...)"`.

**Insertion point:** Immediately after `set -eu` (line 10), before any other command runs. Preserves existing functional path — when `go` is on PATH, the added lines are 3 fast no-ops.

```diff
--- a/scripts/smoke-chain-wait.sh
+++ b/scripts/smoke-chain-wait.sh
@@ -8,6 +8,12 @@
 
 set -eu
 
+# PATH pre-flight (Phase 10 D-11): fail fast with a readable error
+# if `go` is not on PATH, instead of an obscure subshell failure below.
+if ! command -v go >/dev/null 2>&1; then
+  echo "smoke: go not found on PATH (needed to build and seed state)" >&2
+  exit 127
+fi
+
 SMOKE_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
 MONITOR="$SMOKE_ROOT/plugin/scripts/chain-wait.sh"
 SEED="$SMOKE_ROOT/scripts/internal/seed_pending.go"
```

**Why `command -v go >/dev/null 2>&1` instead of `command -v go || exit 127`:** Per Pitfall 4, the guarded `if` form is robust against `set -e` corner cases.

**Why exit 127:** Matches bash's "command not found" convention and matches Phase 9 §"Next-Phase-Readiness" note that the script "ran fine when PATH points at the Go SDK, fails in minimal environments."

## §5. `DOGFOOD.md` Checklist (10–15 manual steps)

**Location:** `.planning/phases/10-docs-dogfooding/DOGFOOD.md`
**Audience:** The author (or any human who can install a Claude Code plugin and open two sessions).
**Gate:** Linux run + (macOS run OR "deferred pending runner access" annotation) before `v0.1.0` tag push.

**Skeleton:**

```markdown
# mcp-chain — Manual Dogfood Checklist (Phase 10 gate for v0.1.0)

Linux and macOS each need a green run (or a documented deferral) before tagging.

| # | Step | Done means | Linux | macOS |
|---|------|------------|-------|-------|
| 1 | Tag a snapshot tag locally: `git tag v0.1.0-rc1 && make release-snapshot` | `dist/` contains 6 archives + checksums.txt | ☐ | ☐ |
| 2 | Install plugin from snapshot path: `/plugin install <local-path-to-plugin>` or push a `v0.1.0-rc1` tag + `/plugin install anthropics/mcp-chain@v0.1.0-rc1` | Claude Code reports "installed"; `/mcp` list shows `mcp-chain` | ☐ | ☐ |
| 3 | `mcp-chain --version` from a plain shell | Prints `mcp-chain 0.1.0` (NOT `dev`, NOT `0.0.1-snapshot-none`) | ☐ | ☐ |
| 4 | Open Session A in Claude Code; run `/mcp-chain:reg build passes` | Response includes a single EFF-word ID (e.g., `otter`) | ☐ | ☐ |
| 5 | Run `/mcp-chain:list` in Session A | Table shows `otter`, status `pending`, condition `build passes` | ☐ | ☐ |
| 6 | Open Session B in a separate Claude Code conversation; run `/mcp-chain:wait otter` | Monitor starts; no output yet; stays running | ☐ | ☐ |
| 7 | In Session A, say "build passed, resolve otter" — let Claude call the MCP `resolve` tool (NOT the `--force` CLI escape hatch) | Session A reports resolved | ☐ | ☐ |
| 8 | Session B within 2 seconds prints `continue` and exits | Session B unblocked | ☐ | ☐ |
| 9 | `/mcp-chain:list` from either session | Shows `otter` status `resolved` with a `resolved_at` timestamp | ☐ | ☐ |
| 10 | `/mcp-chain:purge --resolved` | No output on success; subsequent `/mcp-chain:list` empty | ☐ | ☐ |
| 11 | `/mcp-chain:purge` with no args | Errors with "requires --all, --resolved, or <id>" (CLI writes to stderr) | ☐ | ☐ |
| 12 | `/mcp-chain:wait nonexistent` | Exits 1 immediately with "unknown id" on stderr | ☐ | ☐ |
| 13 | Inspect `~/.mcp-chain/state.json` OR `$XDG_STATE_HOME/mcp-chain/state.json` | File permissions `0600`, parent dir `0700`, JSON valid | ☐ | ☐ |
| 14 | Kill Claude Code mid-wait in Session B, reopen, `/mcp-chain:wait <id-still-pending>` | Waiter resumes cleanly | ☐ | ☐ |
| 15 | (Optional) Reload plugin: `/mcp` list → select `mcp-chain` → restart | Plugin reconnects without error | ☐ | ☐ |

**macOS deferral option:** If no macOS machine available, write "macOS: deferred pending runner access" under the macOS column header and annotate the phase verifier accordingly. Treats as `human_needed`, not a blocker per D-10.

**Pre-tag sign-off:** Both columns complete (or macOS explicitly deferred). Initial and date.
```

Step numbering spans 15, covers: install → register → wait/resolve path → list → purge → error cases → state file permissions → plugin reload. Each "done means" is an objective observable — no "seems to work" or "looks good."

## §6. Tag Push Procedure

**Pre-push gate (from D-15):**
- [ ] README.md merged to main
- [ ] `bash scripts/dogfood.sh` exits 0 on Linux
- [ ] DOGFOOD.md Linux column fully ticked
- [ ] DOGFOOD.md macOS column fully ticked OR annotated "deferred pending runner access"
- [ ] `.planning/REQUIREMENTS.md` prose swap merged
- [ ] `scripts/smoke-chain-wait.sh` PATH guard merged

**Push commands** (author-driven, per D-15):

```bash
# Verify current state
git status
git log --oneline -5

# Dry-run what will ship (optional)
make release-snapshot

# Tag and push
git tag v0.1.0
git push origin v0.1.0
# (or: git push --tags if no other un-pushed tags exist)
```

**What CI does afterward** (per Phase 9 contract, verified in `09-01-SUMMARY.md`):
1. `.github/workflows/release.yml` triggers on `push: tags: ['v*']`.
2. Checks out at `fetch-depth: 0`, installs Go per `go-version-file: go.mod` (currently 1.25).
3. Runs `goreleaser release` — cross-compiles 6 arch combos (darwin/linux/windows × amd64/arm64) with `CGO_ENABLED=0` and `-s -w -trimpath`.
4. Generates `checksums.txt` (sha256).
5. Attaches 6 archives + `checksums.txt` to the GitHub Release for `v0.1.0` (GitHub auto-creates the release from the tag).
6. `--version` in the release binaries reads `mcp-chain 0.1.0` (not `dev`, not `0.0.1-snapshot-none`), via the ldflags path `-X main.version=0.1.0` confirmed live in Phase 9.

**Gotchas:**
- `v0.1.0` → GoReleaser `{{.Version}}` expands to `0.1.0` (it strips the leading `v`). The ldflag injection pattern is `-X main.version={{.Version}}` per `.goreleaser.yaml`. This is why step 3 of DOGFOOD.md asserts `mcp-chain 0.1.0` not `mcp-chain v0.1.0`.
- Release workflow needs `permissions: contents: write` (already set per Phase 9 SUMMARY) — no action required.
- Do NOT push `v0.1.0-rc1` to the public `anthropics/mcp-chain` repo unless you want the rc to be the "latest" advertised release. For dogfooding, prefer `make release-snapshot` (local only) or push to a fork.
- If the first tag push fails mid-workflow (e.g., GoReleaser action breaks on a SDK issue), `git tag -d v0.1.0` locally + `git push --delete origin v0.1.0` to clean up, then retry.
- No Homebrew tap, no Scoop manifest, no Docker image — per D-deferred. Users install via `/plugin install` or `go install`.

## §7. Landmines

1. **README command examples drift.** Phase 8 LD-14 already renamed commands once. If a future phase renames again, README goes stale. *Mitigation:* plan-check validation row that greps for `/mcp-chain:` and `/chain-` counts; future rename phases must update README in the same PR.

2. **`dogfood.sh` races `serve` startup.** Addressed by CLI-only design (Pitfall 2). If a future enhancement adds an MCP-tool step to dogfood, use the Phase 5 integration test pattern: pipe JSON-RPC on stdin, not subprocess timing.

3. **bash 3.2 vs bash 4+ pitfalls.** macOS's default `/bin/bash` is 3.2. Developer on Linux bash 5 may write `[[ $a =~ regex ]]` or `declare -A` and not notice until CI/macOS runner fails. *Mitigation:* `scripts/check-chain-wait-bashisms.sh` can be invoked on `dogfood.sh` too — recommend planner add a line to the lint gate or copy its pattern.

4. **`command -v go` output suppression differences.** On some POSIX-strict shells `command -v` writes path to stdout even on no-argument; different shells handle `2>&1` around the `if !` differently. *Mitigation:* use the explicit `>/dev/null 2>&1` redirection inside the `if !` test, and route the error message to stderr. This is what D-11 specified verbatim.

5. **Claude Code plugin reload semantics vary by version.** Per Pitfall 5. *Mitigation:* README §6 cites both `/mcp` list and full restart, so instructions remain valid across versions.

6. **`REQUIREMENTS.md` prose swap collides with ROADMAP.md legacy references.** ROADMAP.md line 139 still says `/chain-reg` etc. CONTEXT.md D-05 scope is "REQUIREMENTS.md CMD-01..04 + HELPER-01" — ROADMAP.md is NOT in scope. *Decision:* leave ROADMAP.md alone; add an explicit note in plan that this is intentional (per D-06's spirit — internal planning docs stay as historical record).

7. **First-tag version mismatch.** Plan steps might ship `"version": "1.0.0"` in `plugin/.claude-plugin/plugin.json` (currently says `1.0.0` — verified above). The plugin manifest version is NOT the same as the binary `--version` (which comes from the git tag via ldflags). *Decision:* do NOT fix plugin.json `version` in Phase 10 — it's a separate decision, not in CONTEXT.md scope. Flag as an open question for planner to confirm.

8. **`.mcp.json` uses absolute `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`.** The manual-install block in README must wire it differently (just `"command": "mcp-chain"` assuming `$GOBIN` on PATH). Don't accidentally copy the plugin path into the manual-install README example.

9. **Seed helper path.** `scripts/internal/seed_pending.go` is `//go:build ignore` — accessible via `go run <path>` but NOT via `go build ./...`. `dogfood.sh` relies on `go run` which is correct. If a future refactor deletes this helper, dogfood.sh breaks. *Mitigation:* plan-check validation row `test -f scripts/internal/seed_pending.go`.

10. **macOS deferral can slide into "never done."** D-10 explicitly allows a `human_needed` deferral but the gate says "Linux + macOS ticked OR macOS deferred." Risk: `v0.1.0` ships with macOS untested, an external macOS user hits an issue. *Mitigation:* even if deferral is taken, run `make release-snapshot` locally — that cross-compiles the darwin/amd64 + darwin/arm64 binaries, giving at least build-level macOS confidence.

## §8. Validation Architecture (SC-1 / SC-2 / SC-3 → concrete checks)

From ROADMAP.md Phase 10 Success Criteria:

| SC | Criterion | Concrete Gate |
|----|-----------|---------------|
| SC-1 | README covers why, install (plugin + manual), usage for 4 slash commands + manual CLI | `test -f README.md && grep -qE '/mcp-chain:(reg\|wait\|list\|purge)' README.md && [ "$(grep -cE '/mcp-chain:(reg\|wait\|list\|purge)' README.md)" -ge 4 ] && grep -q 'go install' README.md && grep -iq 'install' README.md && grep -qE 'status\|list\|purge\|resolve' README.md` |
| SC-2 | State-file path + NFS caveat + reload documented | `grep -iqE 'XDG_STATE_HOME\|\.mcp-chain' README.md && grep -iqE 'NFS\|networked.*filesystem\|flock.*reliable' README.md && grep -iqE '/mcp\|restart' README.md` |
| SC-3 | Dogfood register→wait→resolve end-to-end on Linux + macOS | `bash scripts/dogfood.sh; test $? -eq 0` (automated portion) + DOGFOOD.md Linux and macOS columns both ticked OR macOS annotated deferred (manual portion) |

**Additional auxiliary gates (for plan-check verification row completeness):**

| Gate | Command | Expected |
|------|---------|----------|
| No legacy `/chain-` in REQUIREMENTS.md | `grep -cE '/chain-(reg\|wait\|list\|purge)' .planning/REQUIREMENTS.md` | `0` |
| Namespaced commands present in REQUIREMENTS.md | `grep -c '/mcp-chain:' .planning/REQUIREMENTS.md` | `≥ 7` |
| `smoke-chain-wait.sh` PATH guard present | `grep -q 'command -v go' scripts/smoke-chain-wait.sh` | exit 0 |
| `smoke-chain-wait.sh` PATH guard fires | `PATH=/nonexistent bash scripts/smoke-chain-wait.sh; echo $?` | `127` |
| `dogfood.sh` is bash 3.2 safe | `bash scripts/check-chain-wait-bashisms.sh` extended (or manual visual check for `[[`, `declare -A`, `<()`, `${var,,}`) | 0 hits |
| `README.md` ≤ 300 lines | `[ "$(wc -l < README.md)" -le 300 ]` | exit 0 |
| Security section present | `grep -qE '\$ARGUMENTS\|shell.*inject' README.md` | exit 0 |
| `/mcp-chain:` prefix explanation | `grep -qE 'plugin.*prefix\|<plugin>:' README.md` | exit 0 (D-07) |
| Plugin version bumped (optional, flag only) | `grep '"version"' plugin/.claude-plugin/plugin.json` | currently `1.0.0` — planner decides if this needs a change alongside `v0.1.0` tag |

## Sources

### Primary (HIGH confidence — files read this session)
- `/home/alpine/mcp-chain/.planning/phases/10-docs-dogfooding/10-CONTEXT.md` — 15 locked decisions
- `/home/alpine/mcp-chain/.planning/REQUIREMENTS.md` — DIST-04, MCP-01, CMD-01..04, HELPER-01, CORE-06
- `/home/alpine/mcp-chain/.planning/PROJECT.md` — audience, constraints
- `/home/alpine/mcp-chain/.planning/ROADMAP.md` — Phase 10 goal + SC-1/2/3
- `/home/alpine/mcp-chain/.planning/phases/09-ci-release/09-01-SUMMARY.md` — release artifact naming + verification results including two pre-existing failures (`net/http`, `smoke-chain-wait.sh` PATH)
- `/home/alpine/mcp-chain/.planning/phases/08-plugin-packaging/08-01-SUMMARY.md` — plugin layout, LD-1 revision (chain-wait.sh → plugin/scripts/)
- `/home/alpine/mcp-chain/.planning/phases/08-plugin-packaging/08-REVIEW.md` — WR-02 `$ARGUMENTS` hazard analysis (informs README §7)
- `/home/alpine/mcp-chain/plugin/.mcp.json` — exact install-time command wiring
- `/home/alpine/mcp-chain/plugin/.claude-plugin/plugin.json` — exact manifest fields
- `/home/alpine/mcp-chain/.claude-plugin/marketplace.json` — marketplace shape
- `/home/alpine/mcp-chain/plugin/commands/{reg,wait,list,purge}.md` — current prompt bodies
- `/home/alpine/mcp-chain/cmd/mcp-chain/main.go` — `--version` output format (line 52: `mcp-chain <ver>`)
- `/home/alpine/mcp-chain/internal/statepath/statepath.go` — authoritative XDG resolution logic (sourced for README §4 verbatim)
- `/home/alpine/mcp-chain/scripts/smoke-chain-wait.sh` — pattern source for dogfood.sh; exact line number for PATH guard insertion
- `/home/alpine/mcp-chain/plugin/scripts/chain-wait.sh` — confirms file location (not `scripts/chain-wait.sh`)
- `/home/alpine/mcp-chain/.planning/config.json` — `commit_docs: true`, `nyquist_validation: true`, `code_review: standard`

### Secondary (MEDIUM — cross-checked against multiple files)
- `code.claude.com` plugin reference — cited in CONTEXT.md `canonical_refs`; used for A1 assumption and D-02 install syntax
- XDG Base Directory Specification — referenced in `internal/statepath/statepath.go` package doc and CONTEXT.md

### Tertiary (LOW — needs validation at plan time)
- A1..A4 in Assumptions Log — specifically `/plugin install anthropics/mcp-chain` syntax and `/plugin reload` verb existence

## Metadata

**Confidence breakdown:**
- README structure & scope: HIGH — fully pinned by CONTEXT.md D-01..D-13
- REQUIREMENTS.md surgical edits: HIGH — grep output verified in this session; exact line numbers cited
- `dogfood.sh` design: HIGH — mirrors existing smoke-chain-wait.sh; exit codes verified via Phase 6/7 SUMMARY
- `smoke-chain-wait.sh` PATH guard: HIGH — insertion point verified from file read
- DOGFOOD.md checklist: MEDIUM — exact step count is a planner decision; shape is HIGH
- Tag push procedure: HIGH — Phase 9 pipeline verified via SUMMARY §T-09 Verification Results
- Plugin install syntax (A1): LOW — not verified against live Claude Code docs this session
- Plugin reload verb (A4): LOW — documented ambiguity in Pitfall 5

**Research date:** 2026-04-24
**Valid until:** 2026-05-24 (30 days, stable — no fast-moving dependencies; re-verify if Phase 9 SDK changes or Claude Code plugin install surface shifts)
