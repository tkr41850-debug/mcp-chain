# Phase 10: Docs & Dogfooding Polish - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning
**Mode:** `--auto` (recommended options selected without interactive confirmation)

<domain>
## Phase Boundary

Deliver the final v1 polish: a single `README.md` at repo root that lets a new user install (plugin path + manual/binary path) and use mcp-chain; a committed dogfooding smoke that registers→waits→resolves end-to-end; and the REQUIREMENTS/PROJECT prose updates carried over from Phases 8 and 9 (slash-command namespacing + MCP-01 `net/http` caveat). Scope anchor: close `DIST-04`, run the dogfood pass, and leave the repo ready for a first tag push.

**In scope:**
- `README.md` at repo root (new file).
- Slash-command namespacing prose update in `REQUIREMENTS.md` (CMD-01..04, HELPER-01) — carried over from Phase 8 LD-14.
- Dogfooding pass — committed script + DOGFOOD.md checklist covering register/wait/resolve/list/purge on Linux + macOS.
- Doc-only fixes for the two Phase 9 out-of-scope items: `scripts/smoke-chain-wait.sh` PATH handling + `$ARGUMENTS` shell-injection note in README security section.
- First tag strategy (version string + when to push).

**Out of scope:**
- MCP SDK fork / vendor carve-out to eliminate `net/http` imports (MCP-01 note was finalized 2026-04-24; accepted as upstream regression — not re-opened here).
- Homebrew / deb / rpm packaging (deferred in Phase 9).
- Code signing / notarization / SBOM (deferred in Phase 9).
- Shell completion generation (OBS-02), `doctor` subcommand (OBS-03), fuzzy prefix match (UX-02) — all in roadmap backlog, not v1.
- License allocation (STATE.md: "License deferred — no phase allocated").

</domain>

<decisions>
## Implementation Decisions

### README Shape & Structure
- **D-01:** Single `README.md` at repo root — one-page comprehensive doc (not multi-file `docs/`). Sections in this order: (1) Why / what it does, (2) Install (plugin path first, manual/binary path second), (3) Usage (slash commands `/mcp-chain:reg|wait|list|purge` first, manual CLI `status|list|purge|resolve` second), (4) State file path + permissions, (5) NFS / networked-filesystem caveat, (6) Upgrade / reload (`/mcp` list → restart), (7) Security notes (`$ARGUMENTS` shell-injection norm), (8) Troubleshooting, (9) License placeholder ("License TBD — see roadmap").
- **D-02:** Plugin install is the primary path. `/plugin install tkr41850-debug/mcp-chain` (or equivalent) demonstrated first. `go install` + manual `.mcp.json` wiring is shown second as the "without Claude Code" fallback. Order matches audience in PROJECT.md (plugin-first).
- **D-03:** Usage examples use realistic word-IDs from the EFF list (e.g., `otter`, `acid`, `cable`) — not `foo`/`bar` placeholders.
- **D-04:** Code blocks are copy-paste shell (not narrative with placeholders). Each install/usage example is a single fenced block the user can paste verbatim.

### Namespace Prose Update (carried from Phase 8 LD-14)
- **D-05:** Update `REQUIREMENTS.md` prose in CMD-01..04 and HELPER-01 so textual references read `/mcp-chain:reg`, `/mcp-chain:wait`, `/mcp-chain:list`, `/mcp-chain:purge` (not the legacy `/chain-reg` etc.). No requirement-ID renumbering; prose-only swap.
- **D-06:** Do NOT rewrite PROJECT.md — it already uses descriptive terminology ("register/wait/resolve") and the CMD-IDs there are prose; leaving PROJECT.md as-is avoids diffs with no product value.
- **D-07:** Add a single sentence to README's "Commands" section noting that Claude Code prefixes plugin commands with `<plugin>:`, so `reg.md` in `plugin/commands/` is invoked as `/mcp-chain:reg`.

### Dogfooding Pass
- **D-08:** Ship `scripts/dogfood.sh` — end-to-end smoke covering: build → serve in background → `mcp-chain status <fake-id>` returns exit 1 → register → status returns exit 2 (pending) → resolve → status returns exit 0 → list shows the entry → purge → list empty. Bash 3.2 safe. Runs locally in ~2 s.
- **D-09:** Also ship `.planning/phases/10-docs-dogfooding/DOGFOOD.md` — a manual checklist covering the full "2 sessions, real Claude Code plugin install, register in session A, wait in session B, resolve in A, see `continue` in B" path that cannot be automated. Checklist is the go/no-go gate for the first tag push.
- **D-10:** Dogfooding success criteria: the checklist's Linux run + macOS run both tick every box. If macOS run cannot be performed (no access), annotate the checklist with "macOS: deferred pending runner access" and treat as `human_needed`, NOT a blocker for tagging.

### Phase 8/9 Deferred Items Triage
- **D-11:** Fix `scripts/smoke-chain-wait.sh` PATH handling — script runs `go run` directly; replace with a pre-flight `command -v go || { echo "…" >&2; exit 127; }` guard so minimal-shell failures are explicit, not obscure. Small diff; no behavior change when Go is present.
- **D-12:** Add a "Security notes" section to README covering: (a) `$ARGUMENTS` is un-sanitized when slash commands call `chain-wait.sh`; (b) plugin invocations assume trusted local shell; (c) users with hostile tenants on shared infra should not rely on MCP lock primitives for trust isolation. Accept the Phase 8 norm explicitly; don't silently inherit it.
- **D-13:** Do NOT fix the MCP SDK `net/http` regression. Reaffirm the 2026-04-24 MCP-01 acceptance. README can optionally mention it under "What's in the binary" if space allows.

### First Tag Strategy
- **D-14:** First tag is `v0.1.0` — semver-zero (`0.x.y`) because the public surface is a 4-command slash namespace + a plugin manifest, and breaking changes (e.g., `purge --all` default behavior, `status` exit codes) remain possible before a `1.0.0` commitment. This matches the risk posture in PROJECT.md (single-dev, hobby-scale).
- **D-15:** Tag push gated on: (a) README merged, (b) dogfood.sh green on Linux, (c) DOGFOOD.md manual checklist Linux-complete + macOS-complete or annotated-deferred. Author pushes `git tag v0.1.0 && git push --tags`; GoReleaser does the rest (Phase 9 contract).

### Claude's Discretion
- Exact README wording, tone, ordering within each section.
- Whether to include a badge row (CI status, Go version) at top — planner decides based on how noisy the first page reads.
- Whether the dogfood script uses `trap` vs explicit cleanup — planner chooses per bash best practice.
- Whether to include an architecture diagram (likely no — PROJECT.md already has one; link to it).

### Folded Todos
_No open todos matched Phase 10 scope._

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope & Prior Decisions
- `.planning/ROADMAP.md` — Phase 10 entry (goal + 3 success criteria + DIST-04 binding).
- `.planning/REQUIREMENTS.md` — DIST-04, MCP-01 (2026-04-24 acceptance note), CMD-01..04, HELPER-01, CORE-06 (state path rules README must document).
- `.planning/PROJECT.md` — audience, constraints, non-negotiables; README tone should match.
- `.planning/STATE.md` §Accumulated Context — carries the Phase 8 LD-14 rename + Phase 8 WR-02 + Phase 9 `net/http` + Phase 9 `smoke-chain-wait.sh` deferrals.

### Prior Phase Artifacts (for cross-references inside README)
- `.planning/phases/08-plugin-packaging/08-01-SUMMARY.md` — plugin install flow + command file layout.
- `.planning/phases/08-plugin-packaging/08-REVIEW.md` WR-02 — `$ARGUMENTS` shell-injection note.
- `.planning/phases/09-ci-release/09-01-SUMMARY.md` — release artifact naming + checksum file shape.
- `.planning/phases/09-ci-release/09-VERIFICATION.md` — open hosted-verify items that affect first-tag README wording.

### Codebase Files Touched
- `plugin/.claude-plugin/plugin.json`, `plugin/.mcp.json`, `.claude-plugin/marketplace.json` — referenced verbatim in install section.
- `plugin/commands/{reg,wait,list,purge}.md` — linked from README commands section.
- `scripts/chain-wait.sh` — referenced for the `/mcp-chain:wait` behavior.
- `scripts/smoke-chain-wait.sh` — target of D-11 fix.
- `cmd/mcp-chain/main.go` — `--version` + state-path resolution (README state-path section).

### External Specs (authoritative, not produced by this project)
- [code.claude.com — Plugins reference](https://code.claude.com/docs/en/plugins-reference) — plugin install UX, `${CLAUDE_PLUGIN_ROOT}`, command namespacing rules. README install section must match current surface.
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html) — authoritative for `$XDG_STATE_HOME` fallback prose.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/mcp-chain/main.go:52` — `--version` output format (`mcp-chain <ver>`); README version-check example can cite it verbatim.
- `scripts/smoke-chain-wait.sh` — already exists as an E2E shape; `scripts/dogfood.sh` can follow its conventions (bash 3.2, set -eu, clear exit codes).
- `internal/statepath` — authoritative source for the state-path resolution prose in README §State file.
- `plugin/commands/*.md` — 8/17/18/23 word prompts; README examples reference these by path, not by inlining their text.

### Established Patterns
- Shell scripts are bash 3.2 safe (from Phase 8) — D-08's dogfood.sh inherits this.
- All CLI exit codes follow the 0/2/1 convention (status) or 0/1 convention (resolve/list/purge); README §CLI usage documents these succinctly.
- `_, _ = fmt.Fprint*` idiom established in Phase 9 lint cleanup — README's "developer" section (if any) should not re-explain this; it's internal.

### Integration Points
- README at repo root is auto-rendered on GitHub and on `pkg.go.dev`; first 2 paragraphs need to hook without Claude Code context.
- `CLAUDE.md` stack doc is private — do NOT link README to it; users shouldn't see it.
- `.planning/` is project-internal — do NOT link README to roadmap/requirements; all user-facing context lives in README itself.

</code_context>

<specifics>
## Specific Ideas

- README opens with a 2-line "What is it?" plus a 3-line "Why?" — matches the 5-line hook in PROJECT.md but tuned for an external reader with no prior context.
- Install section shows a single copy-paste block for the plugin path:
  ```
  /plugin install tkr41850-debug/mcp-chain
  ```
  followed by a "without Claude Code" block using `go install github.com/tkr41850-debug/mcp-chain/cmd/mcp-chain@latest`.
- Usage section shows a 2-session asciinema-style dialogue (plain text): Session A `/mcp-chain:reg "build passes"` → prints `otter`; Session B `/mcp-chain:wait otter`; Session A `/mcp-chain:resolve otter` (or Claude resolves via MCP); Session B prints `continue`. This is the canonical demo.
- State-file section shows:
  ```
  $ mcp-chain --help   # or read below
  State file: $XDG_STATE_HOME/mcp-chain/state.json
                or   ~/.mcp-chain/state.json (fallback)
  Permissions: 0600 (file), 0700 (directory)
  ```
- NFS caveat is one paragraph, explicit: "Do not place the state file on NFS/SMB/CIFS — `flock(2)` semantics over networked filesystems are not reliable. Local ext4/APFS/NTFS only."
- Security notes: 3 bullets max, link to Phase 8 WR-02 rationale via a terse paraphrase (don't paste the raw review).

</specifics>

<deferred>
## Deferred Ideas

- **v1.0.0 promotion** — after 1–2 real external users confirm the plugin works on their setup and no breaking changes land for 30 days. Not scoped to Phase 10.
- **MCP SDK `net/http` carve-out** — would require vendoring a stripped SDK subset or contributing upstream. MCP-01 note already accepts the regression. Re-open only if a user actually trips on the transitive dep (unlikely on stdio-only transport).
- **Homebrew / Scoop packaging** — defers to post-v1.0; no committed user yet.
- **CI badge, coverage badge, go-report badge** — only if planner decides it doesn't clutter the README fold.
- **`mcp-chain doctor` subcommand** (OBS-03) — belongs in a future OBS-* milestone.
- **Shell completions** (OBS-02) — belongs in a future UX milestone.
- **asciinema/video demo** — nice-to-have post-tag; a text-rendered session in README is sufficient for v0.1.0.

### Reviewed Todos (not folded)
_No todos matched Phase 10 scope._

</deferred>

---

*Phase: 10-docs-dogfooding*
*Context gathered: 2026-04-24*
