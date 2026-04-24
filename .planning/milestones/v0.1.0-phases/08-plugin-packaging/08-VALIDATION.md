---
phase: 8
slug: plugin-packaging
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-24
---

# Phase 8 — Validation Strategy

> Per-phase validation contract. Content derived from `08-RESEARCH.md` §"Validation Architecture" and `08-CONTEXT.md` §"Testability pattern".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 for Go; `bash -n` + `jq` + `grep` for shell gates |
| **Config file** | none |
| **Quick run command** | `scripts/check-plugin-manifests.sh && scripts/check-prompt-wordcount.sh && bash -n plugin/scripts/chain-wait.sh` |
| **Full suite command** | `go test -race -count=1 -timeout 60s ./... && scripts/smoke-chain-wait.sh` |
| **Integration command** | `bash scripts/smoke-chain-wait.sh` (builds mcp-chain via `go build`, seeds tmp state, exercises resolve/unknown/timeout) |
| **Smoke gates** | `jq empty` on each JSON manifest; ≤30-word gate on each `plugin/commands/*.md` body; `bash -n` on `plugin/scripts/chain-wait.sh`; bashism grep |
| **Estimated runtime** | ~1 s static lint; ~5–10 s integration smoke |

---

## Sampling Rate

- **After every task commit:** `bash -n plugin/scripts/chain-wait.sh && scripts/check-prompt-wordcount.sh && jq empty < plugin/.claude-plugin/plugin.json && jq empty < plugin/.mcp.json && jq empty < .claude-plugin/marketplace.json` (~1s)
- **After every plan wave:** `scripts/smoke-chain-wait.sh` (builds binary, exercises chain-wait.sh integration paths)
- **Before `/gsd-verify-work`:** Full race suite + all shell gates + manual `--plugin-dir ./plugin` dev-install smoke on Linux host
- **Max feedback latency:** <2 s per-commit; <15 s per-wave

---

## Per-Task Verification Map

| Wave | Test | Target file | Assertion | SC | Automated |
|------|------|-------------|-----------|----|-----------|
| 0 | `TestPluginJSON_ValidJSON` | `plugin/.claude-plugin/plugin.json` | `jq empty < plugin/.claude-plugin/plugin.json` exits 0 | 1 | Y |
| 0 | `TestPluginJSON_RequiredKeys` | `plugin/.claude-plugin/plugin.json` | `name`, `version`, `description`, `author` fields present (no top-level `$schema`/`schemaVersion` per research §"Plugin manifest spec") | 1 | Y |
| 0 | `TestMarketplaceJSON_ValidJSON` | `.claude-plugin/marketplace.json` | `jq empty < .claude-plugin/marketplace.json` exits 0 (repo-root location, NOT inside `plugin/`) | 1 | Y |
| 0 | `TestMarketplaceJSON_RequiredKeys` | `.claude-plugin/marketplace.json` | `name`, `owner`, `plugins[]` present; plugin entry has `name`, `source: "./plugin"`, `description` | 1 | Y |
| 0 | `TestMcpJSON_ValidJSON` | `plugin/.mcp.json` | `jq empty < plugin/.mcp.json` exits 0 | 1 | Y |
| 0 | `TestMcpJSON_CommandPath` | `plugin/.mcp.json` | `mcpServers.mcp-chain.command` equals `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`; `args` equals `["serve"]`; no npm/uvx/node/python | 1 | Y |
| 0 | `TestMcpJSON_NoAbsolutePaths` | `plugin/.mcp.json` | No occurrence of `/Users/`, `/home/`, `C:\\`, or literal absolute path; every path starts with `${CLAUDE_PLUGIN_ROOT}` | 1 | Y |
| 1 | `TestPromptWordCount_reg` | `plugin/commands/reg.md` | Body (post-frontmatter) word count ≤30 via `scripts/check-prompt-wordcount.sh` | 2 | Y |
| 1 | `TestPromptWordCount_wait` | `plugin/commands/wait.md` | Body ≤30 words | 2 | Y |
| 1 | `TestPromptWordCount_list` | `plugin/commands/list.md` | Body ≤30 words | 2 | Y |
| 1 | `TestPromptWordCount_purge` | `plugin/commands/purge.md` | Body ≤30 words | 2 | Y |
| 1 | `TestPromptArgumentHints` | `plugin/commands/{reg,wait,purge}.md` | `argument-hint:` frontmatter present on arg-taking commands; `list.md` may omit | 2 | Y |
| 1 | `TestPromptMentionsMCPTool` | `plugin/commands/*.md` | Each body names exactly one MCP tool (`register`/`status`/`list`/`purge`) or the `chain-wait.sh` script path — no branching prose, no examples | 2 | Y |
| 2 | `TestChainWait_BashSyntaxParse` | `plugin/scripts/chain-wait.sh` | `bash -n plugin/scripts/chain-wait.sh` exits 0 | 3 | Y |
| 2 | `TestChainWait_BashismFree` | `plugin/scripts/chain-wait.sh` | `grep -nE '\\[\\[|\\(\\(|=~\\|mapfile\\|readarray\\|<\\(\\|&>\\|declare\\|\\$\\{[a-zA-Z_]+,,\\}' plugin/scripts/chain-wait.sh` returns nothing (no bashisms per LD-4) | 3 | Y |
| 2 | `TestChainWait_DurationParser_30s` | `plugin/scripts/chain-wait.sh` | Stubbed `mcp-chain` + `--timeout 30s` → internal budget computes to 30 seconds (assert via stderr "waiting for <id> (timeout: 30s)") | 3 | Y |
| 2 | `TestChainWait_DurationParser_1m_1h_24h_168h` | `plugin/scripts/chain-wait.sh` | `1m` → 60, `1h` → 3600, `24h` → 86400, `168h` → 604800 (each verified by inducing timeout with `sleep`-stubbed mcp-chain and asserting exit 124 timing OR via stderr echo) | 3 | Y |
| 2 | `TestChainWait_DurationParser_Invalid` | `plugin/scripts/chain-wait.sh` | `1h30m`, `1d`, `1w`, `abc`, `0.5h` → stderr "mcp-chain: bad duration" + exit 1 (LD-6) | 3 | Y |
| 2 | `TestChainWait_ExitCode_Resolved_Continue` | `plugin/scripts/chain-wait.sh` | Stub `mcp-chain` returning exit 0 → script prints `continue` to stdout, exits 0 (LD-5) | 3 | Y |
| 2 | `TestChainWait_ExitCode_Pending_Loops` | `plugin/scripts/chain-wait.sh` | Stub returning exit 2 then exit 0 → script sleeps, retries, prints `continue`, exits 0 | 3 | Y |
| 2 | `TestChainWait_ExitCode_Unknown_Exit1` | `plugin/scripts/chain-wait.sh` | Stub returning exit 1 → script echoes error to stderr, exits 1 (LD-5) | 3 | Y |
| 3 | `TestChainWait_Integration_Resolve` | `scripts/smoke-chain-wait.sh` | Real `mcp-chain register` → spawn `chain-wait.sh <id> --timeout 10s` → `mcp-chain resolve <id> --force` after 2s → assert stdout `continue` + exit 0 within 4s | 3, 4 | Y |
| 3 | `TestChainWait_Integration_UnknownID` | `scripts/smoke-chain-wait.sh` | `chain-wait.sh never-registered-id` → stderr error + exit 1 | 3 | Y |
| 3 | `TestChainWait_Integration_Timeout_Exit124` | `scripts/smoke-chain-wait.sh` | Pending id + `--timeout 3s` → stderr `mcp-chain: timeout` + exit 124 (LD-7) | 3 | Y |
| 3 | `TestE2E_RegWaitResolve_FlowDemo` | `scripts/smoke-chain-wait.sh` (E2E block) | Full demo: `mcp-chain register "cond"` → parses word-id from stdout → `chain-wait.sh <id>` in bg → `mcp-chain resolve <id> --force` → monitor prints `continue` | 4 | Y |
| 3 | `TestE2E_ListTableOutput` | `scripts/smoke-chain-wait.sh` (E2E block) | `mcp-chain list` after register prints tabwriter table with ID/Condition/Created columns to stdout | 4 | Y |
| 3 | `TestE2E_PurgeRefusesBare` | `scripts/smoke-chain-wait.sh` (E2E block) | `mcp-chain purge` (no args) → stderr + exit 1 (LD-11, Phase 7 contract) | 4 | Y |
| 4 | `TestNoStalePathReferences` | all `plugin/commands/*.md`, `plugin/.mcp.json` | `grep -rE 'scripts/chain-wait\\.sh\|/chain-reg\|/chain-wait\|/chain-list\|/chain-purge' plugin/` returns nothing (no old command names, no outside-plugin script refs) | 1, 2 | Y |
| 4 | `TestReadmePhase10TODO` | `README.md` or `.planning/phases/10-*` TODO list | If present, includes entries for marketplace.json, renamed commands (`/mcp-chain:reg` etc.), and `${CLAUDE_PLUGIN_ROOT}` path conventions — else no-op pass | 1, 4 | Y |

*Row count: 29*

---

## Wave 0 Requirements

- [ ] `plugin/.claude-plugin/plugin.json` — valid JSON; `name`, `version`, `description`, `author` top-level keys per `code.claude.com/docs/en/plugins-reference`
- [ ] `.claude-plugin/marketplace.json` (at repo root) — valid JSON; `name: "mcp-chain-marketplace"`, `owner: {...}`, `plugins: [{name, source: "./plugin", description}]`
- [ ] `plugin/.mcp.json` — valid JSON; `mcpServers.mcp-chain.command` = `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`; `args` = `["serve"]`; no absolute paths
- [ ] `scripts/check-plugin-manifests.sh` — `jq empty` against all three JSON files; greps for `${CLAUDE_PLUGIN_ROOT}` literal in `.mcp.json`
- [ ] `scripts/check-prompt-wordcount.sh` — strips YAML frontmatter from each `plugin/commands/*.md`, asserts body word count ≤30

**Framework install:** None. Zero new `go.mod` deps. Requires `jq` (already assumed per project convention) and stdlib bash.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `/plugin install` from fresh macOS host with no Go/Node/Python | DIST-01, SC-1 | Requires a clean VM / fresh shell; not automatable in Phase 8 without containerized Claude Code driver | Phase 10 dogfooding: `/plugin marketplace add <owner>/mcp-chain` → `/plugin install mcp-chain@mcp-chain-marketplace` → verify commands appear in slash list |
| macOS bash 3.2 compatibility of `chain-wait.sh` | HELPER-02, SC-3 | Linux CI runners ship bash 4+; true bash-3.2 verification requires macOS runner | Phase 9 macOS CI matrix runs the integration smoke; Phase 8 uses Linux `bash --posix` + bashism grep as proxy |
| Visual polish of slash-command output in real Claude Code session | CMD-05, SC-2 | Subjective — "does the prompt produce the right tool call reliably?" | Phase 10 dogfooding: register/wait/list/purge against a real session on both platforms |
| `/plugin install` on fresh Linux host with no Go/Node/Python | DIST-01, SC-1 | Same as macOS — needs clean environment | Phase 10 dogfooding end-to-end demo |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All Success Criteria have ≥1 automated test (SC-1 → 7 manifest rows + stale-path row; SC-2 → 6 prompt rows; SC-3 → 9 chain-wait.sh rows; SC-4 → 3 E2E rows)
- [x] Sampling continuity: every test maps to a commit-level verify (<2 s static); integration gate at wave close (<15 s)
- [x] Wave 0 self-contained (manifests + schema) — no runtime dependencies
- [x] No watch-mode flags
- [x] Feedback latency < 60 s (1 s lint; 10 s integration)
- [x] `nyquist_compliant: true` set
- [x] All 4 SC mapped to ≥1 test
- [x] Manual gates scoped to Phase 9/10 (clean-VM install, macOS runner, dogfooding polish)

**Approval:** approved 2026-04-24 (autonomous mode — all prior open questions resolved by 08-RESEARCH.md; marketplace.json path and command naming locked via LD-13/LD-14)

---

## Open Questions

None. 08-RESEARCH.md resolved all three CONTEXT.md OQs (OQ-1 plugin.json schema → confirmed 4-key minimal; OQ-2 frontmatter → optional, `argument-hint` recognized; OQ-3 bash 3.2 timeout → date-math preferred over SIGALRM) plus the research-introduced OQ-4 (marketplace requirement) which became LD-13. No residual ambiguity requires a default to be applied.
