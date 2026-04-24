---
phase: 10-docs-dogfooding
verified: 2026-04-24T00:00:00Z
status: human_needed
score: 8/8 must-haves verified (automated half); 1 human gate pending (v0.1.0 tag push)
overrides_applied: 0
human_verification:
  - test: "DOGFOOD.md Linux column walkthrough — 15 steps, 2 Claude Code sessions"
    expected: "All 15 Linux-column boxes tick; Session A register → Session B wait → Session A resolve via MCP tool → Session B prints `continue`"
    why_human: "Requires two live Claude Code sessions (plugin-install flow + real /mcp-chain:reg/wait/resolve), which automation cannot drive"
  - test: "DOGFOOD.md macOS column walkthrough OR explicit `macOS: deferred pending runner access` annotation"
    expected: "macOS column all-ticked OR the deferral text present under the macOS header (per D-10)"
    why_human: "Requires a macOS machine; per D-10 explicit deferral is allowed and not a blocker"
  - test: "v0.1.0 tag push: `git tag v0.1.0 && git push origin v0.1.0` once DOGFOOD.md gates are met"
    expected: "GitHub Release v0.1.0 materializes with 7 assets (6 archives + checksums.txt); `--version` on released binary prints `mcp-chain 0.1.0`"
    why_human: "Tag-push is author-only per D-15 + plan Task 7; triggers Phase 9 GoReleaser workflow"
---

# Phase 10: Docs & Dogfooding Polish — Verification Report

**Phase Goal:** Ship the final v1 polish — user-facing README at repo root, committed dogfooding smoke, manual 2-session checklist, and REQUIREMENTS prose fixes — and leave the repo one tag push away from v0.1.0.

**Verified:** 2026-04-24
**Status:** human_needed (all automated gates green; DOGFOOD.md manual walkthrough + tag push remain)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (merged from ROADMAP SC + PLAN frontmatter)

| #  | Truth                                                                                                                                                  | Status        | Evidence                                                                                                                                            |
| -- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | README.md at repo root documents install (plugin-first), usage with all 4 `/mcp-chain:*` commands + manual `go install` path (SC-1)                    | VERIFIED      | README.md:19 `/plugin install`; README.md:32 `go install github.com/anthropics/mcp-chain/cmd/mcp-chain@latest`; 7 `/mcp-chain:(reg|wait|list|purge)` matches |
| 2  | README install section cites state-file path (`$XDG_STATE_HOME` or `~/.mcp-chain`) + NFS caveat + Claude Code reload step (SC-2)                       | VERIFIED      | README.md:111 `$XDG_STATE_HOME/mcp-chain/state.json`; README.md:121 bold NFS warning; README.md:127 `## Upgrade / reload` cites `/mcp` list + full restart |
| 3  | Dogfooding pass: automated `scripts/dogfood.sh` exits 0 AND DOGFOOD.md exists with 15-step checklist for Linux + macOS (SC-3 automated half)           | VERIFIED      | `bash scripts/dogfood.sh` → exit 0, 5 "step N OK" lines + "all steps passed"; DOGFOOD.md has 15 numbered rows, Linux + macOS columns, `otter` example |
| 4  | README shows 2-session usage demo with real EFF word-IDs (otter/acid/cable), not foo/bar placeholders                                                  | VERIFIED      | README.md:50–78 uses `otter`; zero `\bfoo\b|\bbar\b|<ID>` matches                                                                                   |
| 5  | README documents state file path, 0600/0700 permissions, and the NFS caveat                                                                            | VERIFIED      | README.md:115 "Permissions: parent directory `0700`, file `0600`. Writes are atomic"; README.md:121 NFS bold paragraph                              |
| 6  | README security section covers the `$ARGUMENTS` shell-injection norm in 3 bullets                                                                      | VERIFIED      | README.md:137–146 — 3 bullets: $ARGUMENTS norm, privilege=session, not-for-multi-tenant                                                              |
| 7  | REQUIREMENTS.md prose for CMD-01..04 and HELPER-01 reads `/mcp-chain:reg|wait|list|purge` (no legacy `/chain-*` command refs) AND HELPER-01 path reads `plugin/scripts/chain-wait.sh` | VERIFIED      | Zero `/chain-(reg|list|purge)` matches; the one `/chain-wait` hit at REQUIREMENTS.md:39 is the filename `chain-wait.sh` (not a command ref); 8 `/mcp-chain:` occurrences |
| 8  | `scripts/smoke-chain-wait.sh` fails fast with exit 127 and a readable stderr message when go is not on PATH                                             | VERIFIED      | `PATH=/nonexistent:/bin:/usr/bin bash scripts/smoke-chain-wait.sh` → exit 127; stderr: "smoke: go not found on PATH (needed to build and seed state)" |

**Score:** 8/8 truths verified (automated)

Note: Three additional truths require human verification:
- Full DOGFOOD.md Linux column walkthrough (Session A + Session B real Claude Code plugin demo)
- DOGFOOD.md macOS column (or explicit deferral)
- v0.1.0 tag push (SC-3 closure + release observation)

These are correctly gated on a human per D-10 + D-15 + Task 7.

### Required Artifacts

| Artifact                                                       | Expected                                                                | Status   | Details                                                                                                                            |
| -------------------------------------------------------------- | ----------------------------------------------------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `README.md`                                                    | User-facing install/usage/state/NFS/security/troubleshooting docs       | VERIFIED | 181 lines (≤300 cap); 11 D-01 sections present; 7 `/mcp-chain:` references; `otter` EFF word; no internal refs leaked              |
| `.planning/REQUIREMENTS.md`                                    | Prose aligned with Phase 8 command namespace + path relocation          | VERIFIED | 8 `/mcp-chain:` occurrences; zero legacy `/chain-{reg,list,purge}` command refs; HELPER-01 path `plugin/scripts/chain-wait.sh`; MCP-01 net/http note preserved |
| `scripts/dogfood.sh`                                           | End-to-end CLI smoke producing exit 0 in ~2s                            | VERIFIED | Executable (0755); 67 lines; bash 3.2 safe (no `[[ ]]`, `declare -A`, `readarray`, `mapfile`); `go build -o`; runs to exit 0       |
| `scripts/smoke-chain-wait.sh`                                  | E2E monitor smoke with go-on-PATH pre-flight guard                      | VERIFIED | `command -v go` guard at line 14; `exit 127` at line 16; end-to-end run exits 0 with go on PATH                                    |
| `.planning/phases/10-docs-dogfooding/DOGFOOD.md`               | Manual 2-session checklist gating v0.1.0 tag push                       | VERIFIED | Exactly 15 numbered step rows; Linux + macOS columns; uses `otter`; contains macOS deferral escape-hatch text; `mcp-chain 0.1.0` version assertion |

### Key Link Verification

| From                                                 | To                                                  | Via                                                          | Status | Details                                                                                      |
| ---------------------------------------------------- | --------------------------------------------------- | ------------------------------------------------------------ | ------ | -------------------------------------------------------------------------------------------- |
| `README.md`                                          | `plugin/commands/{reg,wait,list,purge}.md`          | `/mcp-chain:(reg|wait|list|purge)` references + path table   | WIRED  | README.md:99–104 4-row table maps each slash command → `plugin/commands/<name>.md`            |
| `README.md §State file`                              | `internal/statepath` package doc                    | verbatim state path + permissions                            | WIRED  | README.md:111–112 matches `internal/statepath` resolver ($XDG_STATE_HOME → ~/.mcp-chain fallback); README.md:115 0600/0700 matches |
| `scripts/dogfood.sh`                                 | `scripts/internal/seed_pending.go`                  | `go run` helper to seed pending entry                        | WIRED  | dogfood.sh:36 `ID="$(go run "$ROOT/scripts/internal/seed_pending.go")"`; helper exists at expected path |
| `.planning/REQUIREMENTS.md HELPER-01`                | `plugin/scripts/chain-wait.sh`                      | path prose after Phase 8 LD-1 relocation                     | WIRED  | REQUIREMENTS.md:39 HELPER-01 references `plugin/scripts/chain-wait.sh`; file exists (mode 0755) at that path |

All key links verified WIRED.

### Data-Flow Trace (Level 4)

Not applicable — Phase 10 ships docs + shell scripts; no data-rendering components.
`dogfood.sh` is tested behaviorally in Step 7b (produces real exit 0 via live binary + state file round-trip).

### Behavioral Spot-Checks

| Behavior                                                                                       | Command                                                                                                          | Result                                                          | Status |
| ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------- | ------ |
| dogfood.sh end-to-end (build → unknown-id → seed pending → resolve → list → purge → empty)     | `bash scripts/dogfood.sh`                                                                                        | exit 0; 5 "step N OK" lines + "all steps passed"                | PASS   |
| smoke-chain-wait.sh end-to-end (register/resolve, unknown-id, timeout)                          | `bash scripts/smoke-chain-wait.sh`                                                                               | exit 0; 3 cases OK + "all cases passed"                         | PASS   |
| PATH pre-flight guard: go absent → fast-fail with readable error                                | `PATH=/nonexistent:/bin:/usr/bin bash scripts/smoke-chain-wait.sh`                                               | exit 127; stderr "smoke: go not found on PATH..."               | PASS   |
| Full CI gate retained (lint + build + size + startup + stdout-silence + tests)                  | `make ci-local`                                                                                                  | exit 0; lint 0 issues; size 7.41 MB; startup P95 38.856 ms; stdout 0 bytes; 8 test packages pass | PASS   |
| README bashisms/parse syntax                                                                    | `bash -n scripts/dogfood.sh && bash -n scripts/smoke-chain-wait.sh`                                              | both exit 0                                                     | PASS   |

### Requirements Coverage

| Requirement | Source Plan | Description                                                                                   | Status     | Evidence                                                                                                              |
| ----------- | ----------- | --------------------------------------------------------------------------------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------- |
| DIST-04     | 10-01-PLAN  | README at repo root with why/install/usage/state/security/troubleshooting                     | SATISFIED  | README.md 181 lines, 11 D-01 sections present, all SC-1/SC-2 greps green                                              |
| MCP-01      | 10-01-PLAN  | SDK transitively imports `net/http`; 2026-04-24 acceptance note preserved                     | SATISFIED  | REQUIREMENTS.md:25 MCP-01 line contains the verbatim `net/http` + "accepted on 2026-04-24" note                        |
| CMD-01      | 10-01-PLAN  | `/mcp-chain:reg` slash command prose                                                          | SATISFIED  | Zero legacy `/chain-reg` refs; `/mcp-chain:reg` present in REQUIREMENTS.md (8 `/mcp-chain:` hits overall)               |
| CMD-02      | 10-01-PLAN  | `/mcp-chain:wait` slash command prose                                                         | SATISFIED  | Zero legacy `/chain-wait` command refs (the one `/chain-wait` hit is the filename `chain-wait.sh`); `/mcp-chain:wait` present |
| CMD-03      | 10-01-PLAN  | `/mcp-chain:list` slash command prose                                                         | SATISFIED  | Zero legacy `/chain-list` refs; `/mcp-chain:list` present                                                               |
| CMD-04      | 10-01-PLAN  | `/mcp-chain:purge` slash command prose                                                        | SATISFIED  | Zero legacy `/chain-purge` refs; `/mcp-chain:purge` present                                                             |
| HELPER-01   | 10-01-PLAN  | `plugin/scripts/chain-wait.sh` path in REQUIREMENTS.md                                        | SATISFIED  | REQUIREMENTS.md:39 HELPER-01 references `plugin/scripts/chain-wait.sh`; file exists at that path                        |

### Anti-Patterns Found

| File                                                         | Line | Pattern                           | Severity | Impact                                                                                                          |
| ------------------------------------------------------------ | ---- | --------------------------------- | -------- | --------------------------------------------------------------------------------------------------------------- |
| README.md                                                    | 181  | `License: TBD`                    | Info     | Deliberate per PROJECT.md ("License deferred — no phase allocated"); not a stub blocking any user path           |

No blockers; no warnings. The `License: TBD` entry is a documented deferral.

### Human Verification Required

Three items require human walkthrough and are correctly gated on a human per the plan (D-10, D-15, Task 7):

#### 1. DOGFOOD.md Linux column walkthrough

**Test:** Walk through the 15-step DOGFOOD.md checklist on Linux in two real Claude Code sessions. Tick each Linux-column box as you confirm the observable. Session A `/mcp-chain:reg build passes` → Session B `/mcp-chain:wait otter` → Session A asks Claude to resolve via MCP tool (NOT `--force`) → Session B prints `continue`.
**Expected:** All 15 Linux-column boxes ticked. Step 7 exercises the OwnerToken session-link path end-to-end.
**Why human:** Requires two live Claude Code sessions + plugin install + real MCP tool invocation; automation cannot drive the Claude Code UI.

#### 2. DOGFOOD.md macOS column (or explicit deferral)

**Test:** Repeat the 15-step DOGFOOD.md checklist on a macOS machine. If no macOS machine is available, write `macOS: deferred pending runner access` under the macOS column header.
**Expected:** Either all 15 macOS-column boxes ticked OR the explicit deferral text present. `make release-snapshot` cross-compiles darwin/amd64 + darwin/arm64 giving build-level confidence if deferred.
**Why human:** Requires a macOS machine; per D-10 explicit deferral is allowed and not a blocker for tagging.

#### 3. v0.1.0 tag push + release observation

**Test:** Once DOGFOOD.md Linux is green and macOS is green-or-deferred, run:

```bash
git status            # verify clean working tree on main
git tag v0.1.0
git push origin v0.1.0
```

Then verify the GoReleaser-produced release:

```bash
gh release view v0.1.0 --json assets --jq '.assets | length'   # expect 7
gh release view v0.1.0 --json assets --jq '.assets[].name'     # expect 6 archives + checksums.txt
```

**Expected:** GitHub Release `v0.1.0` has 7 assets (6 archives for darwin/linux/windows × amd64/arm64 + `checksums.txt`); released binary `--version` prints `mcp-chain 0.1.0`.
**Why human:** Tag-push is author-only per D-15 + plan Task 7 + user_setup.dashboard_config. Triggers the Phase 9 GoReleaser workflow.

### Gaps Summary

**No automated gaps.** All six automation tasks (1–6) completed successfully:

1. REQUIREMENTS.md prose swap clean (zero legacy `/chain-{reg,list,purge}` refs; HELPER-01 path correct; MCP-01 note preserved).
2. smoke-chain-wait.sh PATH guard fires at exit 127 with readable stderr.
3. dogfood.sh CLI-only end-to-end exits 0 (~2s, bash 3.2 safe).
4. DOGFOOD.md 15-step Linux + macOS checklist with `otter` example and deferral escape hatch.
5. README.md at repo root passes all SC-1 + SC-2 greps (7 `/mcp-chain:` references, `go install` block, XDG/NFS/reload/security all present, 181 lines ≤ 300 cap).
6. `make ci-local` green (lint 0 issues, size 7.41 MB, startup P95 38.856 ms, stdout 0 bytes, 8 test packages pass).

The only remaining work is the three human-gated items above, explicitly scoped out of this plan per Task 7.

---

_Verified: 2026-04-24_
_Verifier: Claude (gsd-verifier)_
