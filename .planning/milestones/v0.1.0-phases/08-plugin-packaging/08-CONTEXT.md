---
phase: 8
name: Plugin Packaging & Bash Monitor
depends_on: [5, 6, 7]
requirements: [CMD-01, CMD-02, CMD-03, CMD-04, CMD-05, HELPER-01, HELPER-02, DIST-01]
status: draft
---

# Phase 8: Plugin Packaging & Bash Monitor - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Phase 8 takes the completed `mcp-chain` binary (Phases 5/6/7) and wraps it in a Claude Code plugin: `plugin.json` manifest, `.mcp.json` server spec, four slash-command markdown files, and a POSIX-`sh` bash-3.2-compatible monitor script. No new Go code is required in `internal/**`; this phase is YAML/JSON/Markdown/shell authoring plus a narrowly-scoped integration smoke test. After this phase, `/plugin install <repo>` in a fresh Claude Code (no Go, no Node, no Python on the host) yields a working `/chain-reg`, `/chain-wait`, `/chain-list`, `/chain-purge` flow — subject to Phase 9 shipping the `bin/mcp-chain` artifact (Phase 8 wires the path; Phase 9 fills it).

**Phase Goal (verbatim from ROADMAP):** The compiled binary is installable with zero additional config as a Claude Code plugin via `/plugin install`, with four token-budgeted slash commands and a POSIX-bash monitor script that calls back into `mcp-chain status` on a 1-second poll.

**Success Criteria (verbatim from ROADMAP):**
1. `plugin/.claude-plugin/plugin.json`, `plugin/.mcp.json` (using `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve`, no absolute paths, no npm/uvx), and four `commands/*.md` slash-command files ship in the repo and install cleanly via `/plugin install` on a fresh machine with no Go/Node/Python.
2. Every slash-command prompt body is ≤ 30 words and tells Claude exactly which MCP tool to call with which arguments — no branching prose, no examples, no boilerplate.
3. `scripts/chain-wait.sh` wraps `mcp-chain status <id>` in a 1-second poll loop, echoes `--timeout DURATION` (accepts `30s`/`1m`/`1h`/`24h`/`168h`) on invocation, prints `continue` on resolve, errors to stderr on unknown/purged ID mid-wait, and runs under macOS's default bash 3.2 with no bashisms.
4. `/chain-reg [condition]` registers (prompting for condition if omitted), `/chain-wait [id] [--timeout D]` runs the monitor, `/chain-list` prints the table, and `/chain-purge [id | --all | --resolved]` refuses bare invocation — end-to-end demo flow verified on Linux and macOS.

**Requirements:** CMD-01, CMD-02, CMD-03 (slash-command prompt half), CMD-04 (slash-command prompt half), CMD-05, HELPER-01, HELPER-02, DIST-01

**Depends on:** Phase 5 (MCP tool surface: `register`/`resolve` names + arg shapes), Phase 6 (CLI `status` exit-code contract 0/2/1), Phase 7 (CLI `list`/`purge` exit-code contract)

</domain>

<decisions>
## Implementation Decisions (Locked)

### Deliverable map

Files created this phase (all under repo root):

| Path | Purpose |
|------|---------|
| `.claude-plugin/marketplace.json` | **Repo-root** marketplace catalog (required for `/plugin install` — see LD-13) |
| `plugin/.claude-plugin/plugin.json` | Plugin manifest (name, version, description, author) |
| `plugin/.mcp.json` | MCP server spec pointing at `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve` |
| `plugin/commands/reg.md` | `/mcp-chain:reg [condition]` prompt body (≤30 words) — see LD-14 |
| `plugin/commands/wait.md` | `/mcp-chain:wait [id] [--timeout D]` prompt body (≤30 words) |
| `plugin/commands/list.md` | `/mcp-chain:list` prompt body (≤30 words) |
| `plugin/commands/purge.md` | `/mcp-chain:purge [id \| --all \| --resolved]` prompt body (≤30 words) |
| `plugin/scripts/chain-wait.sh` | POSIX-bash polling monitor wrapper over `mcp-chain status <id>` (inside `plugin/` so `${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh` resolves after install) |
| `scripts/smoke-chain-wait.sh` (or `internal/plugin/chainwait_integration_test.go`) | Repo-root shell test harness invoking `plugin/scripts/chain-wait.sh`; seeds a temp state.json and exercises resolve + unknown-id + timeout paths |

No Go source edits expected; if the plan surfaces a need (e.g., a `--quiet` flag on `status` for the monitor), escalate to a cross-phase delta rather than in-lining it here.

### Locked decisions

| ID | Decision | Rationale |
|----|----------|-----------|
| LD-1 | Plugin directory layout: `plugin/` subdir at repo root containing `.claude-plugin/plugin.json`, `.mcp.json`, `commands/*.md`, and `scripts/chain-wait.sh`. Monitor script MUST live **inside** `plugin/scripts/chain-wait.sh` (NOT repo-root `scripts/`) because files outside the plugin root are not copied during `/plugin install`. Slash-command prompts reference `${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh` at runtime. Repo-root `scripts/smoke-chain-wait.sh` (test harness) invokes `plugin/scripts/chain-wait.sh` directly during CI — the harness itself is not shipped. | Matches the Claude Code plugins reference (`code.claude.com/docs/en/plugins-reference`); 08-RESEARCH.md §"Decisions Revised" confirms only the `plugin/` subtree is copied to `~/.claude/cache/plugins/...` during install, so an outside-`plugin/` script would be unreachable at `${CLAUDE_PLUGIN_ROOT}` runtime. |
| LD-2 | MCP server command line is exactly `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve` with no absolute paths, no `npm`/`uvx`/`node`/`python` shim. Phase 9's GoReleaser drops the per-platform binary into `plugin/bin/mcp-chain` at release time. | DIST-01 requires zero extra runtime; the CLAUDE.md stack notes `${CLAUDE_PLUGIN_ROOT}/path/to/binary` is accepted directly by `.mcp.json`. The binary is pre-built per-arch — installer picks the right asset (Phase 9 concern). |
| LD-3 | Every `commands/*.md` prompt body ≤ 30 words, imperative voice, single sentence where possible, names the exact MCP tool + arg mapping. No examples, no fallback prose, no "if you don't know the ID" branches. | CMD-05 product principle; SC-2 is the verifiable gate. A 30-word ceiling is ~45 tokens; the smallest description the model can still act on reliably for a single-tool single-arg call. |
| LD-4 | `scripts/chain-wait.sh` shebang is `#!/usr/bin/env bash` (not `sh`), but body uses ONLY bash 3.2-safe syntax: no `[[`, no `(( ))` arithmetic (use `expr` or `$(( ))` POSIX form), no arrays, no `declare`/`local -n`, no `${var,,}` lowercase, no `mapfile`/`readarray`, no process substitution `<(...)`, no `&>`. Traps and `set -eu` are permitted. | HELPER-02 mandates macOS default bash 3.2 compatibility. `bash` is required (not plain `sh`) because `trap`/signal handling on timeout is more reliable in bash than in dash/mksh. |
| LD-5 | Monitor exit-code translation is hard-coded to Phase 6's locked contract: `mcp-chain status <id>` returning **0 → "resolved"** → echo `continue` to stdout, exit 0; **2 → "pending"** → `sleep 1` and loop; **1 → unknown/error** → echo `mcp-chain status error` or the captured stderr line to stderr, exit 1. Any other exit code → treated as exit 1. | STATE.md entry 2026-04-24 confirms Phase 6 locked 0/2/1. `status.go` comments explicitly mark these as "LOCKED — Phase 8 bash monitor depends on these". The monitor reads nothing but exit codes — robust against stdout format changes. |
| LD-6 | `--timeout DURATION` parser accepts exactly `30s`/`1m`/`1h`/`24h`/`168h` shape: integer with single suffix char `s`/`m`/`h`, no combined `1h30m`, no decimal, no `d`/`w` (168h is the 1-week escape). Parser is pure POSIX: `case "$dur" in *s) n=${dur%s}; secs=$n;; *m) ...;; *h) ...;; *) error;; esac`. Max value clamped at 604800 (168h) — anything larger errors on stderr. No `--timeout` flag → unbounded wait (no accumulator, no exit on clock). | REQUIREMENT CMD-02 lists this exact suffix set. Rejecting `1h30m` and `1d`/`1w` keeps the parser under 10 lines and the rejection errors are cheap. 168h clamp is a sanity ceiling (matches the listed max). |
| LD-7 | Timeout is tracked via `start=$(date +%s)` and `now=$(date +%s)` in the loop; `elapsed=$((now - start))`; `[ "$elapsed" -ge "$budget" ] && { echo "mcp-chain: timeout" >&2; exit 124; }`. Exit 124 on timeout follows `timeout(1)`'s convention — distinguishable from the `unknown id` exit 1. No signal-trap-based timeout (avoids bash 3.2 signal edge cases and `kill -TERM $$` quirks in a script-in-a-script context). | POSIX arithmetic + `date +%s` works on macOS 12+ and every Linux tested; no `timeout` binary needed (macOS didn't ship one until the GNU coreutils install). 124 gives callers a distinct code. |
| LD-8 | On invocation, `chain-wait.sh` prints one line to stderr before the first poll: either `mcp-chain: waiting for <id>` (no timeout) or `mcp-chain: waiting for <id> (timeout: <DURATION>)` (timeout set). This is the "echo on invocation" HELPER-01 language. | Puts the monitored ID + timeout in the monitor log so a user glancing at Claude Code output knows what it is waiting on, without polluting stdout (which is reserved for the `continue` signal). |
| LD-9 | `scripts/chain-wait.sh` discovers the `mcp-chain` binary via `${MCP_CHAIN_BIN:-mcp-chain}` — env override for tests, `PATH` lookup in production. The slash-command prompt in `chain-wait.md` instructs Claude to `MCP_CHAIN_BIN="${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain" bash "${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh" <id> [...]`. | Keeps the script testable from the repo root without a global install and from the plugin install without PATH manipulation. One env var, one responsibility. |
| LD-10 | Slash-command prompt wiring rule: each `commands/*.md` begins with optional YAML frontmatter (`description:` field only) and then a single imperative paragraph. `description` is ≤12 words; body is ≤30 words (SC-2). No `allowed-tools` or `argument-hint` required until docs reference confirms they are recognized (open question OQ-2 below). | Minimizes the surface area that could break installs. If frontmatter field names are wrong, Claude Code likely ignores them silently — we can't afford a silent parse failure of the prompt body. |
| LD-11 | `/chain-purge` prompt's "refuse bare invocation" semantic is enforced by the binary (`mcp-chain purge` with no args exits 1 per Phase 7 LD); the slash-command prompt does NOT re-enforce it in model-facing text. The prompt says "call the `purge` MCP tool with the provided argument" and trusts the tool layer to error. | Avoids duplicating error logic in a prompt that must stay ≤30 words. Binary already owns the contract. |
| LD-12 | No Windows testing in this phase. `.mcp.json` path uses forward slashes (`${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`), which Claude Code documents as cross-platform. Windows smoke happens in Phase 9 via the GoReleaser matrix and is not a Phase 8 gate. | PROJECT.md lists Windows as secondary; SC-4 explicitly names Linux + macOS. |
| LD-13 | **Marketplace requirement (PROMOTED TO IN-SCOPE).** `/plugin install <github-repo>` requires a minimum-viable `.claude-plugin/marketplace.json` at the **repo root** (NOT inside `plugin/`). Ship a single-entry marketplace with `"name": "mcp-chain-marketplace"`, `owner` metadata, and one plugin entry whose `source` is `"./plugin"` (relative path resolves against the marketplace root). User flow: `/plugin marketplace add <owner>/mcp-chain` → `/plugin install mcp-chain@mcp-chain-marketplace`. | 08-RESEARCH.md §"Decisions Revised" + OQ-4 resolution: `/plugin install <bare-repo>` is not a flow — install always goes through a marketplace. DIST-01's wording "installable via `/plugin install <github-repo>`" is operationalized as "installable without additional out-of-band config (no npm, no uvx)" via the marketplace step. File sits at `.claude-plugin/marketplace.json` per `code.claude.com/docs/en/plugin-marketplaces`. |
| LD-14 | **Command naming (user-approved 2026-04-24).** Commands named `reg`/`wait`/`list`/`purge` (NOT `chain-reg`/etc.) because Claude Code forces `<plugin>:<command>` namespacing — final user-facing forms are `/mcp-chain:reg`, `/mcp-chain:wait`, `/mcp-chain:list`, `/mcp-chain:purge`. Matches plugin ecosystem convention (`/github:issue`, not `/github:github-issue`). REQUIREMENTS.md CMD-01..04 text will be updated in Phase 10 docs to reflect the namespaced form. | 08-RESEARCH.md §"LD-14" + resolved OQ: the literal `/chain-reg` form in CMD-01 cannot exist — Claude Code prepends `mcp-chain:`. `chain-` prefix inside the plugin is redundant once namespace is `mcp-chain:`. User confirmed this trade-off 2026-04-24. |

### Testability pattern

- **chain-wait.sh integration test** (`scripts/smoke-chain-wait.sh` at repo root, OR `internal/plugin/chainwait_integration_test.go` with `//go:build integration`):
  1. `tmp=$(mktemp -d)`; `export XDG_STATE_HOME=$tmp`
  2. Seed `$tmp/mcp-chain/state.json` with a pending entry (ID `alpha`)
  3. Spawn `bash plugin/scripts/chain-wait.sh alpha --timeout 10s` in background with `MCP_CHAIN_BIN=<locally-built-binary>`
  4. After 2 seconds, mutate state.json to mark `alpha` resolved (call `mcp-chain resolve alpha --force`)
  5. Assert the background process prints `continue` to stdout and exits 0 within 4 seconds
  6. Repeat with a never-registered ID → assert stderr error + exit 1
  7. Repeat with a pending ID and `--timeout 3s` → assert stderr timeout message + exit 124

- **bash 3.2 compatibility gate**: run the test under `/bin/bash --posix` on a Linux runner (not identical to macOS 3.2 but catches the common bashisms); plan for macOS verification in Phase 9's macOS runner.

- **Plugin manifest lint**: `jq empty < plugin/.mcp.json` + `jq empty < plugin/.claude-plugin/plugin.json` + `jq empty < .claude-plugin/marketplace.json` in the smoke script to catch hand-edit JSON errors before install-time failure.

- **Slash-command word-count gate**: a 5-line bash helper that reads each `plugin/commands/*.md`, strips frontmatter, and asserts body word count ≤30. Runnable locally and in Phase 9 CI.

### Non-goals (explicit scope boundary)

- **Windows plugin install verification** — secondary platform, deferred to Phase 9 CI matrix
- **GoReleaser integration / actually building the binary for bundling** — Phase 9's job; Phase 8 authors the path `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve` but does not populate `plugin/bin/`
- **Full CI gate on plugin install** — the smoke test script is in-scope; wiring it into GitHub Actions is Phase 9
- **Slash-command autocompletion hints / `argument-hint` frontmatter** — deferred pending OQ-2 resolution
- **Token-savings telemetry on the prompts** — not a shipping feature; `rtk gain` analytics apply to ops tooling only
- **Dogfooding pass on macOS** — Phase 10's job (DIST-04 + README end-to-end demo)

</decisions>

<code_context>
## Existing Code Insights

- `internal/cli/status.go` (Phase 6) — comment block on `StatusCmd` documents exit codes 0/2/1 as "LOCKED — Phase 8 bash monitor depends on these". This is the contract `chain-wait.sh` consumes via `$?`. No wrapper, no `--quiet` flag needed — stdout "resolved"/"pending" is redundant to exit code and the monitor ignores it.
- `internal/cli/list.go`, `internal/cli/purge.go`, `internal/cli/resolve.go` (Phase 7) — already return the exit codes the slash-commands need via the MCP tool path (plus the `--force` recovery hatch in `resolve`). No Go edits in Phase 8.
- `internal/cli/format/table.go` (Phase 7) — `/chain-list` prompt just tells Claude to call the `list` MCP tool; formatting is already correct.
- `internal/statepath.Resolve()` (Phase 3) — binary reads `$XDG_STATE_HOME`; `chain-wait.sh` does not need to know this — the binary handles it transparently.
- `scripts/check-size.sh`, `scripts/check-startup.sh`, `scripts/check-stdout-silence.sh` — existing Phase 1 scripts demonstrate the POSIX-shell house style (sh shebang, `set -eu`, stderr for diagnostics). `chain-wait.sh` mirrors this style but uses `bash` shebang for trap reliability (LD-4).
- `CLAUDE.md` Tech Stack section — "Claude Code plugin: YES, can bundle a Go binary directly — `.mcp.json` accepts `${CLAUDE_PLUGIN_ROOT}/path/to/binary` as `command`. No npm/uvx bridge needed." This is the DIST-01 green-light.
- `cmd/mcp-chain/main.go` — `kong.Writers(os.Stderr, os.Stderr)` ensures `--help` and bad-args go to stderr; `chain-wait.sh` does not parse mcp-chain stderr — it reacts to exit codes only.

</code_context>

<specifics>
## Specific Ideas

No user-provided specifics — discuss phase skipped. Refer to Success Criteria + the Locked Decisions table.

</specifics>

<deferred>
## Deferred Ideas

- JSON-schema validation of `plugin.json` at CI time — deferred (may land in Phase 9 with the release lint pass)
- `/chain-resolve` slash command (operator-facing resolve escape hatch) — deferred; MCP tool + `mcp-chain resolve --force` cover the use cases
- Interactive `/chain-reg` variant that suggests conditions based on current context — out of scope for v1 token budget
- macOS 12+ / Linux arm64 end-to-end install dogfooding — Phase 10
- README excerpts of slash-command usage — Phase 10 (DIST-04)

</deferred>

<open_questions>
## Open Questions (for RESEARCH — keep tight)

- **OQ-1**: What is the exact current schema version for `plugin.json`? Check `code.claude.com/docs/en/plugins-reference` for the required top-level fields (`name`, `version`, `description` confirmed; `author`, `mcpServers`, `commands` arrays?). Research must confirm so the manifest installs without a parse warning.
- **OQ-2**: Does `commands/*.md` require YAML frontmatter, and if so which fields are recognized (`description`, `argument-hint`, `allowed-tools`)? If frontmatter is optional, LD-10 holds as written; if required, the plan adjusts. No silent-failure mode acceptable.
- **OQ-3**: Best pattern for timeout in bash 3.2 — confirm that `date +%s` + arithmetic polling (LD-7) is preferable to a SIGALRM-based trap. A quick spike: `trap 'kill -ALRM $$' ALRM; ( sleep N && kill -ALRM $$ ) &` under bash 3.2 — does it reliably interrupt the `sleep 1` inner loop, or race? If the date-math approach is clean, skip signals entirely.

</open_questions>
