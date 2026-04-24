---
phase: 10-docs-dogfooding
plan: 01
subsystem: docs+dogfooding+release-gate
one_liner: User-facing README at repo root + CLI dogfood smoke + manual 2-session checklist + PATH guard on chain-wait smoke + REQUIREMENTS prose swap — Phase 10 ships everything except the human v0.1.0 tag push
tags: [docs, dogfood, shell, readme, release-gate]
requires:
  - Phase 8 plugin packaging (slash-command namespace /mcp-chain:*, plugin/scripts/chain-wait.sh)
  - Phase 9 CI release pipeline (GoReleaser, make ci-local, release-snapshot)
  - scripts/internal/seed_pending.go (go:build ignore helper from Phase 8)
provides:
  - README.md at repo root (DIST-04)
  - scripts/dogfood.sh (CLI-only end-to-end smoke, exit 0 on Linux in ~2s)
  - .planning/phases/10-docs-dogfooding/DOGFOOD.md (15-step manual checklist, Linux+macOS columns)
  - scripts/smoke-chain-wait.sh PATH pre-flight guard (exits 127 with readable stderr when go absent)
  - .planning/REQUIREMENTS.md prose aligned with Phase 8 command namespace + path relocation
affects: []
tech_stack:
  added: []
  patterns:
    - bash 3.2–safe: `[ ... ]` only, no `[[ ... ]]` test construct (POSIX bracket expressions like `[[:space:]]` inside regex are fine and appear in `grep -E '^[a-z]+[[:space:]]+'`)
    - PATH pre-flight pattern (`if ! command -v go >/dev/null 2>&1; then echo ...; exit 127; fi`) — Phase 10 D-11, reused across dogfood.sh and smoke-chain-wait.sh
    - CLI-only smoke (no `serve` race) — uses `resolve --force` + `scripts/internal/seed_pending.go` per Phase 9 N-3 lesson
    - README verbatim sections sourced from RESEARCH.md §Code Examples — state path, NFS caveat, security bullets all copied without paraphrase
key_files:
  created:
    - README.md
    - scripts/dogfood.sh
    - .planning/phases/10-docs-dogfooding/DOGFOOD.md
  modified:
    - .planning/REQUIREMENTS.md
    - scripts/smoke-chain-wait.sh
decisions:
  - Phase 10 D-05/D-06: REQUIREMENTS.md prose only — PROJECT.md and ROADMAP.md intentionally NOT touched (ROADMAP historical record preserved; PROJECT is out per D-06)
  - Phase 10 D-08: dogfood.sh is CLI-only, no background `serve` — matches Phase 9 N-3 concurrency lesson
  - Phase 10 D-10: macOS DOGFOOD.md column may be explicitly deferred with "deferred pending runner access" text; `make release-snapshot` already cross-compiles darwin/{amd64,arm64} giving build-level confidence
  - Phase 10 D-11: PATH guard uses `if ! command -v go` (guarded form), not `||` short-circuit, to avoid `set -e` corner cases
  - Phase 10 D-12: 3 security bullets copied verbatim from RESEARCH.md — $ARGUMENTS passthrough norm, plugin privilege = session privilege, not for multi-tenant trust isolation
  - Phase 10 D-13: MCP-01 net/http regression NOT re-opened; 2026-04-24 acceptance note preserved in REQUIREMENTS.md; README adds optional transparency note in Troubleshooting
  - Phase 10 D-15: v0.1.0 tag push is human-only; executor does NOT run `git tag` or `git push`
metrics:
  duration_minutes: ~25
  completed_at: 2026-04-24
---

# Phase 10 Plan 01: Docs & Dogfooding Polish Summary

User-facing README at repo root + CLI dogfood smoke + manual 2-session checklist + PATH guard on chain-wait smoke + REQUIREMENTS prose swap — Phase 10 ships everything except the human v0.1.0 tag push.

## What Was Built

Six artifacts landed across five atomic commits:

1. **.planning/REQUIREMENTS.md prose swap** (`3620728`) — `/chain-{reg,wait,list,purge}` → `/mcp-chain:{reg,wait,list,purge}` across CMD-01..04 + CMD-03/04 traceability notes + DIST-04 prose + Out-of-Scope auto-expiration row (8 lines changed). HELPER-01 path updated from `scripts/chain-wait.sh` to `plugin/scripts/chain-wait.sh`. MCP-01 `net/http` acceptance note preserved verbatim per D-13.

2. **scripts/smoke-chain-wait.sh PATH guard** (`7da601c`) — 6-line insertion after `set -eu`. If `go` is not on PATH, script exits 127 with `"smoke: go not found on PATH"` on stderr instead of the prior obscure subshell failure. Guard uses `if ! command -v go` (guarded form, not `||` short-circuit) per D-11 and Pitfall 4.

3. **scripts/dogfood.sh** (`53cc6a0`, new) — 67-line CLI-only end-to-end smoke. Exercises: build → `status no-such-id` (exit 1) → seed pending via `go run scripts/internal/seed_pending.go` → `status <id>` (exit 2) → `resolve --force` → `status <id>` (exit 0) → `list` contains id → `purge --all` → `list` empty. Clean exit 0 in ~2s on Linux with go on PATH. bash 3.2 compatible (`[ ]` tests only; POSIX bracket char classes like `[[:space:]]` inside regex are fine).

4. **.planning/phases/10-docs-dogfooding/DOGFOOD.md** (`ac1c4d2`, new) — 15-step manual 2-session checklist with Linux + macOS columns. Step 7 explicitly requires the MCP `resolve` tool (NOT `--force` CLI escape hatch) — exercises OwnerToken session-link end-to-end, the one path automation cannot cover. Explicit macOS deferral escape hatch per D-10 (`macOS: deferred pending runner access`).

5. **README.md at repo root** (`c1de99a`, new, DIST-04) — 181 lines (cap 300). 11-section structure per D-01:
   - Title + tagline
   - Why (3 lines, external-reader framing)
   - Install (plugin-first per D-02: `/plugin install tkr41850-debug/mcp-chain`; manual `go install github.com/tkr41850-debug/mcp-chain/cmd/mcp-chain@latest` + generic `.mcp.json` snippet using `"command": "mcp-chain"`)
   - Usage: 2-session demo with real EFF word `otter` (D-03/D-04); CLI subcommand table with exit codes (0/2/1 for status; 0/1 for resolve; 0 for list; 0 or 1 for purge)
   - Commands: 4-row table mapping `/mcp-chain:{reg,wait,list,purge}` → `plugin/commands/*.md` prompts
   - State file (XDG_STATE_HOME + ~/.mcp-chain fallback + 0600/0700 + atomic write)
   - Networked filesystems (bold NFS caveat)
   - Upgrade / reload (cites BOTH `/mcp` list→restart AND full Claude Code restart per Pitfall 5)
   - Security notes (3 bullets per D-12, verbatim from RESEARCH.md)
   - Troubleshooting (5 symptoms + fixes; plus optional net/http transparency note per D-13)
   - License: TBD placeholder

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | `3620728` | docs(10-01): swap /chain-* → /mcp-chain:* + fix HELPER-01 path in REQUIREMENTS.md |
| 2 | `7da601c` | fix(10-01): add go-on-PATH pre-flight guard to smoke-chain-wait.sh (D-11) |
| 3 | `53cc6a0` | feat(10-01): add scripts/dogfood.sh CLI-only end-to-end smoke (D-08) |
| 4 | `ac1c4d2` | docs(10-01): add DOGFOOD.md 15-step manual checklist (D-09) |
| 5 | `c1de99a` | docs(10-01): add user-facing README with install, usage, state, security sections (DIST-04) |

Task 6 was verification-only (no commit). Task 7 is a human checkpoint — executor did NOT run `git tag v0.1.0 && git push` per D-15 + plan instructions.

## Verification (Task 6)

All gates green:

```
bash scripts/dogfood.sh          → exit 0 (5 step OK lines + "all steps passed")
bash scripts/smoke-chain-wait.sh → exit 0 (3 cases OK + "all cases passed")
make ci-local                    → exit 0:
  - golangci-lint run            → 0 issues
  - go build -ldflags="-s -w"    → binary 7.41 MB (under 15 MB cap)
  - check-startup.sh             → P95 40.4 ms (under 100 ms cap)
  - check-stdout-silence.sh      → 0 bytes on stdout
  - go test -race -count=1 ./... → 8 packages, all pass (cmd/mcp-chain, internal/cli, internal/cli/format, internal/idgen, internal/mcpserver, internal/plugin, internal/statepath, internal/store)
```

SC-1 audits: README has 7 `/mcp-chain:(reg|wait|list|purge)` occurrences (≥4 required); manual `go install` block present; 181 lines (under 300 cap).

SC-2 audits: `XDG_STATE_HOME` + `~/.mcp-chain` state path documented; NFS caveat present; reload guidance present; `$ARGUMENTS` security note present; uses `otter` EFF word; zero `foo`/`bar`/`<ID>` placeholders; zero `.planning/` or `CLAUDE.md` internal refs leaked.

REQUIREMENTS.md audit: zero legacy `/chain-{reg,wait,list,purge}` *command* references; HELPER-01 path reads `plugin/scripts/chain-wait.sh`; MCP-01 `net/http` acceptance note preserved verbatim.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Task 1 HELPER-01 path corrupted by sed ordering**

- **Found during:** Task 1 verification
- **Issue:** The plan's sed sequence ran `s|\`scripts/chain-wait.sh\`|\`plugin/scripts/chain-wait.sh\`|` FIRST (expecting to protect the HELPER-01 line), but then `s|/chain-wait|/mcp-chain:wait|g` fired second and rewrote the newly-added path: `plugin/scripts/chain-wait.sh` contains the substring `/chain-wait` (prefix `/chain-wait` + suffix `.sh`), so it became `plugin/scripts/mcp-chain:wait.sh` — a nonexistent file. The plan's ordering guarded against collisions only for the bare backtick-wrapped `scripts/chain-wait.sh` literal, not the result of its own prefix insertion.
- **Fix:** Manually restored HELPER-01 line 39 from `plugin/scripts/mcp-chain:wait.sh` → `plugin/scripts/chain-wait.sh` via Edit.
- **Files modified:** `.planning/REQUIREMENTS.md` (one line)
- **Commit:** `3620728` (committed together with the rest of the prose swap)

**2. [Plan Verify - Over-broad Regex] Phase 1/6 verify greps match filename as legacy command**

- **Found during:** Task 1 verification, Task 6 audit
- **Issue:** Plan verify regex `grep -nE "/chain-(reg|wait|list|purge)"` over-matches — it flags the file path `plugin/scripts/chain-wait.sh` as a legacy command reference, because the substring `/chain-wait` appears inside the path. This is a false positive: the file is literally named `chain-wait.sh` and that's its correct, current name. The plan author did not anticipate that the new path prefix would itself trigger the legacy-pattern grep.
- **Resolution:** Semantic intent is met — there are zero standalone `/chain-{reg,wait,list,purge}` *command references* in REQUIREMENTS.md; only the filename within a path. Ignored the false positive; did not rename the script file (renaming `chain-wait.sh` is out of scope and would break Phase 8's `plugin/commands/wait.md` + Phase 8 SUMMARY).
- **No file change.** Noted here + in Task 1 commit body for audit.

**3. [Plan Verify - Over-broad Regex] Task 3 bashism check flags POSIX bracket expression**

- **Found during:** Task 3 verification
- **Issue:** Plan verify regex `grep -nE "\[\["` intends to reject the bash `[[ ... ]]` test construct. It also matches the POSIX bracket character class `[[:space:]]` inside a grep regex (dogfood.sh line 63: `grep -cE '^[a-z]+[[:space:]]+'`). These are different language features; `[[:space:]]` is POSIX-standard and bash 3.2-safe.
- **Resolution:** dogfood.sh is verified bash 3.2–compatible by a stricter regex (`^\s*\[\[|if \[\[|then \[\[`) that targets the test construct only. The script runs green under `bash -n` parse and end-to-end (exit 0, ~2s). No change to the script needed.
- **No file change.** Verified via replacement-regex audit.

### No architectural changes (Rule 4)

None. Plan executed within scope boundaries (internal/**, cmd/**, plugin/**, .goreleaser.yaml, .github/workflows/** untouched; PROJECT.md and ROADMAP.md untouched per D-06).

### Authentication gates

None.

## DOGFOOD.md Manual Gate Status

| Column | Status | Notes |
|--------|--------|-------|
| Linux | **pending** | Awaiting human operator walk-through of 15 steps |
| macOS | **pending** (may be deferred per D-10) | Per D-10, author may write `macOS: deferred pending runner access` if no macOS runner is available; `make release-snapshot` cross-compiles darwin/amd64 + darwin/arm64 giving build-level confidence |

## v0.1.0 Tag Push Status

**deferred to Task 7 human checkpoint.** Executor did NOT run `git tag v0.1.0 && git push origin v0.1.0` per D-15 + plan instructions (`user_setup.dashboard_config` + Task 7 `<resume-signal>` protocol). Once DOGFOOD.md Linux is green and macOS is green-or-deferred, the author runs the tag push locally; GitHub Actions `.github/workflows/release.yml` (Phase 9 contract) cross-compiles 6 archives + checksums.txt and attaches all 7 assets to the v0.1.0 GitHub Release.

## Requirements Closed

- **DIST-04** — README at repo root with why/install/usage/state/security/troubleshooting
- **CMD-01..04** — prose updated to `/mcp-chain:{reg,wait,list,purge}` namespace (Phase 8 implementation was already correct; this plan aligned REQUIREMENTS.md prose)
- **HELPER-01** — prose updated to `plugin/scripts/chain-wait.sh` (Phase 8 LD-1 path relocation)
- **MCP-01** — 2026-04-24 `net/http` acceptance note preserved verbatim per D-13 (not re-opened)

## Known Stubs

None. README has a `License: TBD` placeholder but that's a deliberate deferral per PROJECT.md — not a stub blocking any user path.

## Self-Check: PASSED

- [x] `README.md` exists (`FOUND: /home/alpine/mcp-chain/README.md`, 181 lines)
- [x] `scripts/dogfood.sh` exists (executable, exits 0)
- [x] `.planning/phases/10-docs-dogfooding/DOGFOOD.md` exists (15-step table)
- [x] `scripts/smoke-chain-wait.sh` has PATH guard (exits 127 on missing go)
- [x] `.planning/REQUIREMENTS.md` prose swap applied (7 `/mcp-chain:` occurrences, HELPER-01 path updated, MCP-01 note preserved)
- [x] Commit `3620728` found in git log (Task 1)
- [x] Commit `7da601c` found in git log (Task 2)
- [x] Commit `53cc6a0` found in git log (Task 3)
- [x] Commit `ac1c4d2` found in git log (Task 4)
- [x] Commit `c1de99a` found in git log (Task 5)
- [x] `make ci-local` green (lint 0 issues, size 7.41 MB, startup P95 40.4 ms, stdout 0 bytes, tests 8 packages pass)
- [x] `bash scripts/dogfood.sh` exit 0
- [x] `bash scripts/smoke-chain-wait.sh` exit 0
