---
phase: 08-plugin-packaging
plan: 08-01
status: complete
completed: 2026-04-24
---

# Phase 8 Plan 01 — Summary

## Goal Delivered

mcp-chain ships as an installable Claude Code plugin:

- `.claude-plugin/marketplace.json` at repo root (LD-13) — single-plugin entry pointing to `./plugin`.
- `plugin/.claude-plugin/plugin.json` — plugin manifest (name, version, description, author, repository, keywords).
- `plugin/.mcp.json` — MCP server wiring via `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve`; no npm/uvx/node/python.
- `plugin/commands/{reg,wait,list,purge}.md` — four slash-command prompts, each body ≤30 words (8/17/18/23). Commands namespaced as `/mcp-chain:reg`, `/mcp-chain:wait`, etc. (LD-14, user-approved).
- `plugin/scripts/chain-wait.sh` — bash 3.2-safe poll loop; maps Phase 6 exit codes 0/2/1 to `continue`/`sleep 1`/error; `--timeout DURATION` accepts `30s|1m|1h|24h|168h` with 604800s clamp; exits 124 on timeout (LD-5, LD-6, LD-7).
- `plugin/bin/` placeholder — binary drop site for Phase 9 GoReleaser.
- `scripts/internal/seed_pending.go` — `//go:build ignore` helper for smoke harness (outside production build).
- `scripts/smoke-chain-wait.sh` — E2E harness exercising resolve/unknown/timeout paths.
- `internal/plugin/manifest_test.go` — 5 stdlib Go tests validating JSON shape, required keys, no-OWNER-placeholder, LD-14 compliance.
- Five shell lint gates: `check-plugin-manifests.sh`, `check-prompt-wordcount.sh`, `check-chain-wait-bashisms.sh`, `check-no-stale-paths.sh`, plus the `smoke-chain-wait.sh` harness.

## Commits (6, on main)

| SHA       | Subject                                                                          |
| --------- | -------------------------------------------------------------------------------- |
| `86135f0` | feat(08-01): plugin + marketplace + .mcp.json manifests with Go validation tests |
| `df0d38b` | feat(08-01): four slash-command prompts + word-count gate                        |
| `4f75622` | feat(08-01): chain-wait.sh bash 3.2 monitor + bashism lint gate                  |
| `aa99b6d` | feat(08-01): smoke-chain-wait.sh E2E harness + seed helper                       |
| `f8ca835` | chore(08-01): stale-path gate + final phase lint                                 |
| `38719ea` | fix(08): replace OWNER placeholder in plugin.json + regression gate (WR-01)      |

## Gates (all green)

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test -race -count=1 ./...` — 8 packages pass
- `bash scripts/check-plugin-manifests.sh` — pass (now includes OWNER guard)
- `bash scripts/check-prompt-wordcount.sh` — 8/17/18/23 words
- `bash scripts/check-chain-wait-bashisms.sh` — pass
- `bash scripts/check-no-stale-paths.sh` — pass
- `bash scripts/smoke-chain-wait.sh` — resolve/unknown/timeout all pass
- `git diff 42d2526 -- go.mod go.sum` — empty (SC-4: no new deps)
- Stripped binary 7.82 MB (≤15 MB)

## Execution-time Pivots

- **N-1 resolved:** the plan's `$TMP/seed.go` path failed module resolution. Pivoted to `scripts/internal/seed_pending.go` with `//go:build ignore` — stays inside the module tree so internal imports (`statepath`, `store`) resolve, stays out of the production binary.

## Quality Gates

- Code review (`08-REVIEW.md`): 0 HI / 2 ME / 6 LO.
  - WR-01 (OWNER placeholder) — **FIXED** in 38719ea + regression gate.
  - WR-02 (`$ARGUMENTS` shell-injection surface in wait.md/purge.md) — **deferred**. Known Claude Code plugin-ecosystem norm; threat model assumes trusted slash-command invocation. Documentation note for Phase 10 README.
  - 6 LO items all deferred-polish (stderr quality, double-binary-run race, leading-dash id, validation-row gaps).
- Verifier (`08-VERIFICATION.md`): 4/4 SC PASS, 29/29 validation rows. Status `human_needed` only because `/plugin install` on a fresh VM + macOS bash 3.2 runtime + dogfooding are by design deferred to Phase 9/10.

## Locked Decisions Honored

LD-1..LD-14 all preserved. Notable: LD-13 (marketplace.json at repo root), LD-14 (commands renamed to `reg`/`wait`/`list`/`purge`), LD-1 revision (monitor under `plugin/scripts/`).

## Deferred to Phase 10

- README documentation of the `$ARGUMENTS` shell-injection norm (WR-02).
- Phase 10 also updates REQUIREMENTS.md CMD-01..04 prose to reflect `/mcp-chain:X` namespacing.

## Next

Phase 9 — CI Release, Cross-compile & Test Gates (GoReleaser matrix + CI race gate).
