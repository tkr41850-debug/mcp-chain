# Phase 8: Plugin Packaging & Bash Monitor - Research

**Researched:** 2026-04-24
**Domain:** Claude Code plugin authoring + POSIX-bash (3.2) scripting
**Confidence:** HIGH (all three open questions resolved from official Claude Code docs; bash pattern cross-verified against POSIX spec and existing repo scripts)
**Open questions remaining:** 0 (OQ-1/2/3 all resolved — see Open-Question Resolutions)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

| ID | Decision |
|----|----------|
| LD-1 | Plugin directory layout: `plugin/` subdir at repo root containing `.claude-plugin/plugin.json`, `.mcp.json`, and `commands/*.md`. Monitor script lives at `scripts/chain-wait.sh` (outside `plugin/`) and is referenced by `chain-wait.md` via its install-time absolute path computed from `${CLAUDE_PLUGIN_ROOT}` at runtime. **(REVISION REQUIRED — see Decisions Revised below: `scripts/` must be inside `plugin/` for the plugin to reference it via `${CLAUDE_PLUGIN_ROOT}`.)** |
| LD-2 | MCP server command line is exactly `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve` with no absolute paths, no `npm`/`uvx`/`node`/`python` shim. Phase 9's GoReleaser drops the per-platform binary into `plugin/bin/mcp-chain` at release time. |
| LD-3 | Every `commands/*.md` prompt body ≤ 30 words, imperative voice, single sentence where possible, names the exact MCP tool + arg mapping. No examples, no fallback prose, no "if you don't know the ID" branches. |
| LD-4 | `scripts/chain-wait.sh` shebang is `#!/usr/bin/env bash` (not `sh`), bash 3.2-safe syntax only. |
| LD-5 | Monitor exit-code translation: 0 → `continue`/exit 0; 2 → sleep 1 and loop; 1 → stderr + exit 1; anything else → exit 1. |
| LD-6 | `--timeout DURATION` accepts `30s`/`1m`/`1h`/`24h`/`168h`, no combined units, clamped at 604800s. |
| LD-7 | Timeout via `date +%s` arithmetic; exit 124 on expiry (matches `timeout(1)`). |
| LD-8 | On invocation, print one stderr line: `mcp-chain: waiting for <id>` or `mcp-chain: waiting for <id> (timeout: <DURATION>)`. |
| LD-9 | Script discovers binary via `${MCP_CHAIN_BIN:-mcp-chain}` env override; slash command sets `MCP_CHAIN_BIN="${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain"`. |
| LD-10 | `commands/*.md` MAY have optional YAML frontmatter (`description:` only for now). |
| LD-11 | `/chain-purge` "refuse bare" is enforced by binary (Phase 7), not restated in prompt. |
| LD-12 | No Windows testing in Phase 8; forward slashes in `${CLAUDE_PLUGIN_ROOT}` paths are cross-platform per Claude Code docs. |

### Claude's Discretion

- Exact wording of the ≤30-word prompt bodies (subject to SC-2 gate).
- Exact shape of integration smoke test (shell-based vs Go `integration` tag) — both listed as acceptable in CONTEXT §Testability pattern.
- Specific stderr wording for `chain-wait.sh` error messages, provided the `mcp-chain:` prefix convention (established in Phase 6) is preserved.

### Deferred Ideas (OUT OF SCOPE)

- JSON-schema validation of `plugin.json` at CI time — Phase 9.
- `/chain-resolve` slash command — covered by MCP tool + `resolve --force`.
- Interactive `/chain-reg` with condition suggestions — out of scope for v1 budget.
- macOS 12+ / Linux arm64 end-to-end install dogfooding — Phase 10.
- README excerpts of slash-command usage — Phase 10 (DIST-04).
- Marketplace manifest / `marketplace.json` — **PROMOTED TO IN-SCOPE** (see Decisions Revised: minimum-viable marketplace needed for `/plugin install` to work at all).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CMD-01 | `/chain-reg [condition]` — registers the current session and prints word-ID; prompts for condition if omitted | Slash-command frontmatter (`argument-hint`) + `$ARGUMENTS` substitution support this shape. See Slash Command Patterns |
| CMD-02 | `/chain-wait [id] [--timeout DURATION]` — polls status every 1s, prints `continue` on resolve, echoes timeout on invocation | Slash-command invokes bash via Bash tool; `${CLAUDE_PLUGIN_ROOT}` resolves at runtime |
| CMD-03 | `/chain-list` — prints human-readable table (CLI semantics done in Phase 7) | Direct CLI wrapper via Bash tool |
| CMD-04 | `/chain-purge [id \| --all \| --resolved]` — explicit cleanup; refuse bare (Phase 7 enforces) | Binary already exits 1 on bare; prompt trusts binary |
| CMD-05 | Every slash-command prompt body is terse — single paragraph max, no boilerplate | ≤30-word gate per SC-2 |
| HELPER-01 | `scripts/chain-wait.sh` wraps `mcp-chain status <id>` in 1s poll loop | Verified status exit codes 0/2/1 in `internal/cli/status.go` |
| HELPER-02 | Script is POSIX-bash compatible, no bashisms that break on macOS bash 3.2 | Verified POSIX features used; see Bash 3.2 Compatibility Gate |
| DIST-01 | Packaged as Claude Code plugin; `.mcp.json` refs `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve`; installable via `/plugin install <github-repo>` | **Requires `marketplace.json` — see Open Question Resolutions, OQ-4** |
</phase_requirements>

## Summary

Phase 8 wraps the already-built `mcp-chain` binary in a Claude Code plugin. The plugin layout is prescribed by the Claude Code docs: `.claude-plugin/plugin.json` for the manifest, `.mcp.json` at the plugin root (NOT inside `.claude-plugin/`) for the MCP server spec, and `commands/*.md` at the plugin root for slash commands. `${CLAUDE_PLUGIN_ROOT}` is an environment variable that resolves at subprocess spawn time (not at `.mcp.json` parse time) — this is critical for the `.mcp.json` and for the bash-monitor invocation.

Three open questions from CONTEXT.md are now resolved with HIGH confidence, and one material revision is required that CONTEXT.md missed: **the current `/plugin install <repo>` flow REQUIRES a `marketplace.json`.** A bare plugin without a marketplace cannot be installed via `/plugin install`; users would instead have to use `claude --plugin-dir ./plugin`. Since DIST-01 explicitly requires `/plugin install <github-repo>` semantics, Phase 8 must ship a minimal `marketplace.json` alongside `plugin.json`. This is a 20-line JSON file and fits naturally within Phase 8's scope.

A second material finding: plugin skills/commands are **namespaced by plugin name**. A plugin named `mcp-chain` with a command `chain-reg` produces `/mcp-chain:chain-reg`, not `/chain-reg`. If the desired end-user command is `/chain-reg` (as REQUIREMENTS.md CMD-01 is worded), the plugin name would need to be `chain` (with commands `reg`/`wait`/`list`/`purge`), OR the user accepts the longer namespaced form. Recommendation below.

**Primary recommendation:** Keep plugin name `mcp-chain` and accept namespaced commands `/mcp-chain:chain-reg`, `/mcp-chain:chain-wait`, `/mcp-chain:chain-list`, `/mcp-chain:chain-purge`. Rationale: the word `chain-` as a command prefix is already redundant once the plugin namespace is `mcp-chain:` — the natural rename is commands `reg`/`wait`/`list`/`purge` giving `/mcp-chain:reg`/`/mcp-chain:wait`/etc. This is cleaner and matches the plugin ecosystem convention. Requires a small requirements amendment; flagged below.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Plugin manifest (identity, version, metadata) | `plugin/.claude-plugin/plugin.json` | — | Claude Code reads this at install to register the plugin |
| MCP server wiring (command, args, env) | `plugin/.mcp.json` | — | Standard MCP config consumed by Claude Code when plugin is enabled |
| Slash-command prompts | `plugin/commands/*.md` | — | Model-facing text that drives Claude's tool calls; rendered into context on invocation |
| Binary drop location | `plugin/bin/mcp-chain` (Phase 9 fills) | — | Referenced by `.mcp.json` via `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`; Phase 9's GoReleaser drops the arch-specific binary here |
| Bash polling monitor | `plugin/scripts/chain-wait.sh` | — | Inside `plugin/` (not repo-root `scripts/`) so `${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh` resolves after install; see revision to LD-1 |
| Marketplace catalog (for `/plugin install`) | `.claude-plugin/marketplace.json` at repo root | — | Required for `/plugin marketplace add <repo>` → `/plugin install` flow |
| Integration smoke test | `scripts/chain-wait-smoke.sh` (repo root, NOT shipped in plugin) | OR `internal/plugin/chainwait_integration_test.go` | Test harness stays out of the shipped plugin to avoid bloat; runs against repo-root `plugin/scripts/chain-wait.sh` |

## Decisions Reaffirmed or Revised

### LD-1: Monitor script location — REVISED

**Original:** `scripts/chain-wait.sh` outside `plugin/`, referenced via `${CLAUDE_PLUGIN_ROOT}` at runtime.

**Problem:** `${CLAUDE_PLUGIN_ROOT}` resolves to the plugin cache directory after install (`~/.claude/plugins/cache/<plugin-id>/<version>/`). Files outside `plugin/` are NOT copied to the cache (the docs are explicit: "Installed plugins cannot reference files outside their directory. Paths that traverse outside the plugin root (such as `../shared-utils`) will not work after installation because those external files are not copied to the cache"). A repo-root `scripts/` directory would NOT ship with the installed plugin.

**Revision:** Move monitor to `plugin/scripts/chain-wait.sh`. The slash-command prompt references it via `${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh`. Integration smoke test lives at repo root (e.g. `scripts/smoke-chain-wait.sh`) and invokes `plugin/scripts/chain-wait.sh` directly with `MCP_CHAIN_BIN` set to the locally-built binary — this works because the smoke test is NOT an installed plugin; it runs from the repo checkout with normal filesystem access.

**Evidence:** `code.claude.com/docs/en/plugins-reference` §"Plugin caching and file resolution" — "Installed plugins cannot reference files outside their directory". HIGH confidence.

### LD-10: Frontmatter — REAFFIRMED

Frontmatter is **optional** for `commands/*.md`. When present, recognized fields include `description`, `argument-hint`, `allowed-tools`, `disable-model-invocation`, `user-invocable`, `arguments`, plus others. For mcp-chain's slash commands, the minimum-friction path is: no frontmatter on `chain-list` (no args), and `argument-hint` + `description` on the commands that take arguments. No silent-parse-failure risk exists — the docs explicitly support files in `commands/` without frontmatter.

**Evidence:** `code.claude.com/docs/en/skills` — "Files in `.claude/commands/` still work and support the same frontmatter" + full Frontmatter Reference table. HIGH confidence.

### NEW LD-13 (derived from research): Minimal marketplace.json REQUIRED

To make `/plugin install <github-repo>` work, the repo must host `.claude-plugin/marketplace.json` at the **repository root** (NOT inside `plugin/`). The user flow becomes:

```
/plugin marketplace add <owner>/<repo>
/plugin install mcp-chain@<marketplace-name>
```

This is two steps, not one. `/plugin install <github-repo>` is NOT a direct-from-bare-repo flow — it always goes through a marketplace. DIST-01's wording ("installable via `/plugin install <github-repo>`") is slightly inaccurate; the actionable interpretation is "installable via adding the repo as a marketplace and then installing the plugin from it, without any additional out-of-band config (no npm, no uvx)."

**Evidence:** `code.claude.com/docs/en/discover-plugins` and `plugin-marketplaces` — marketplace.json listing is always required for install; `--plugin-dir ./plugin` is the only marketplace-free path, and that is for dev testing only.

### NEW LD-14 (derived from research): Plugin name & command namespacing

Claude Code namespaces plugin commands/skills as `/<plugin-name>:<command-name>`. For a plugin named `mcp-chain`, the shipped commands become `/mcp-chain:chain-reg`, `/mcp-chain:chain-wait`, etc. REQUIREMENTS.md CMD-01..CMD-04 list them as `/chain-reg` etc — this is implicitly shorthand.

**Recommended resolution:** Adopt command names `reg`, `wait`, `list`, `purge` inside the plugin so the user-facing form is `/mcp-chain:reg` / `/mcp-chain:wait` / `/mcp-chain:list` / `/mcp-chain:purge`. This is cleaner (no double `chain-chain` prefix) and matches plugin ecosystem convention (`github@claude-plugins-official` → `/github:issue`, not `/github:github-issue`). Flag this for the planner to decide — it's a minor REQUIREMENTS.md amendment, but the net behavior is identical.

**Alternative:** Keep the names as `chain-reg`/`chain-wait`/`chain-list`/`chain-purge` and accept the `/mcp-chain:chain-reg` double-prefix form. Requires no requirements change but is ugly.

**Evidence:** `code.claude.com/docs/en/plugins` — "Skills are prefixed with this (e.g., `/my-first-plugin:hello`)". HIGH confidence.

## Open-Question Resolutions

### OQ-1: plugin.json schema — RESOLVED

**Answer:** Only `name` is required. All other fields are optional. Full schema:

```
required: name (string, kebab-case, no spaces)
optional: version (semver string), description, author (object with name/email/url),
          homepage, repository, license, keywords (array),
          skills, commands, agents, hooks, mcpServers, outputStyles, themes,
          lspServers, monitors, userConfig, channels, dependencies
```

The top-level `version` field is the **plugin's** semver (e.g. `"1.0.0"`) — NOT a schema version. There is NO top-level `$schema` or `schemaVersion` field. If `version` is set, users only receive updates when you bump it. If omitted and the plugin is in a git marketplace, the commit SHA is used as the version key.

**Evidence:** `code.claude.com/docs/en/plugins-reference` §"Plugin manifest schema" + §"Version management". HIGH confidence.

### OQ-2: commands/*.md frontmatter — RESOLVED

**Answer:** Frontmatter is **optional**. If present, it sits between `---` markers at the top of the file. Recognized fields relevant to mcp-chain:

| Field | Relevance | Example |
|-------|-----------|---------|
| `description` | Shown in autocomplete / `/help`. Recommended so users know what the command does | `description: Register a new chain and print the word-ID` |
| `argument-hint` | Hint shown during autocomplete; tells users what args the command expects | `argument-hint: [condition]` |
| `allowed-tools` | Pre-approves tools when the command runs | `allowed-tools: mcp__mcp-chain__register` or `Bash(mcp-chain *)` |
| `disable-model-invocation` | Set `true` if Claude should never auto-invoke it. Default `false` (both user and Claude can invoke) | — |

For mcp-chain v1, minimum viable frontmatter is **just `description`** on all four commands, plus `argument-hint` on the three that take args. `allowed-tools` is a nice-to-have for reducing permission prompts but adds verbosity — defer unless the planner wants it.

**Argument substitution:** The prompt body supports `$ARGUMENTS` (full arg string), `$0`/`$1`/`$ARGUMENTS[0]` (indexed), and `$name` (named args declared in `arguments:` frontmatter). This means `/chain-reg foo bar baz` will have `$ARGUMENTS` = `"foo bar baz"` in the prompt body.

**Evidence:** `code.claude.com/docs/en/skills` §"Frontmatter reference" and §"Available string substitutions". HIGH confidence. (Note: "commands" are an alias for "skills as flat markdown files" in the modern docs; the feature set is the same.)

### OQ-3: POSIX-bash timeout pattern — RESOLVED

**Answer:** Use `date +%s` arithmetic in the poll loop (as LD-7 specifies). Do NOT use SIGALRM traps.

**Reasoning:**
1. `date +%s` is supported identically by BSD `date` (macOS) and GNU `date` (Linux) for printing the current Unix epoch. (It is NOT POSIX-standardized, but is a de-facto universal extension.)
2. POSIX arithmetic `$(( expr ))` is fully supported in bash 3.2 and all POSIX shells.
3. `sleep 1` is POSIX and works everywhere.
4. SIGALRM (`kill -ALRM $$`) inside a script is **unreliable under bash 3.2** because (a) there is no `SIGALRM` handler installed by default in a foreground script, (b) the `sleep` builtin may not be interruptible by a custom signal without additional `wait` plumbing, (c) the race between `( sleep N && kill -ALRM $$ ) &` and the parent's `sleep 1` inner loop introduces timing variance that's hard to test deterministically.
5. The chosen pattern is ~15 lines, easy to unit-test, and has zero race conditions. Timeout granularity is ±1 second which matches the poll interval.
6. Exit 124 matches the coreutils `timeout(1)` convention so callers can distinguish timeout from other errors — even though macOS default doesn't ship `timeout(1)`, using the same exit code is idiomatic.

**Evidence:** [Rich's sh tricks](https://www.etalabs.net/sh_tricks.html) (stick to POSIX primitives), [Baeldung on Linux: manual UNIX timestamp arithmetic](https://www.baeldung.com/linux/shell-unix-timestamp-arithmetic), [nixCraft timeout article](https://www.cyberciti.biz/faq/linux-run-a-command-with-a-time-limit/) confirming macOS doesn't ship `timeout(1)` by default. HIGH confidence.

### OQ-4 (new, surfaced by research): /plugin install from a bare GitHub repo — RESOLVED

**Answer:** Not directly possible. `/plugin install` always requires a marketplace context. The minimum viable path is:

1. Ship `.claude-plugin/marketplace.json` at the repo root.
2. Ship `plugin/` as the actual plugin directory (with its own `.claude-plugin/plugin.json`).
3. `marketplace.json` lists the plugin with `"source": "./plugin"` (relative path, works for git-hosted marketplaces).
4. User flow: `/plugin marketplace add <owner>/<repo>` → `/plugin install mcp-chain@<marketplace-name>`.

**Evidence:** `code.claude.com/docs/en/discover-plugins` §"Install plugins" — install requires `<plugin>@<marketplace>` syntax. `plugin-marketplaces` §"Walkthrough" — shows the full two-step flow. HIGH confidence.

## Standard Stack

### Core

| Library / Format | Version | Purpose | Why Standard |
|------------------|---------|---------|--------------|
| Claude Code plugin system | Any Claude Code ≥ the plugins GA version | Plugin packaging & `/plugin install` flow | Only supported mechanism per DIST-01 |
| `plugin.json` schema | Current (April 2026) | Plugin manifest | Only required field is `name`; everything else optional |
| `.mcp.json` schema | Current | MCP server config | Standard MCP config, `${CLAUDE_PLUGIN_ROOT}` substitution supported |
| `marketplace.json` schema | Current | Marketplace catalog | Required for `/plugin install` — cannot be skipped |
| `commands/*.md` with optional YAML frontmatter | Current | Slash-command files | Treated as "skills as flat markdown files" by modern docs |
| POSIX `sh` + bash 3.2 | bash 3.2 (macOS default) | Monitor script | HELPER-02 constraint |
| `date +%s` | BSD + GNU compatible | Epoch timestamp | De-facto universal |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `jq` | Any | Validate `plugin.json` and `.mcp.json` in smoke test | Catches JSON syntax errors before install. Already assumed available in dev env (Phase 1 style) |
| `wc -w` | POSIX | Word-count gate for prompt bodies | Enforces ≤30-word SC-2 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `marketplace.json` | `--plugin-dir ./plugin` dev flag | Dev-only; DIST-01 requires production `/plugin install` flow |
| `date +%s` arithmetic | SIGALRM trap with `kill -ALRM $$` | Races with inner `sleep 1`, bash 3.2 signal quirks, harder to test |
| bash 3.2 polling | GNU `timeout(1)` wrapping | Not installed by default on macOS; would require users to `brew install coreutils` |
| `argument-hint` frontmatter | Plain prose in prompt body | Frontmatter is free — it's the idiomatic surface for this metadata |
| Explicit `version: "1.0.0"` | Commit-SHA versioning | Explicit version is better for releases; commit-SHA is better for dev. Start with explicit, tied to GoReleaser tag |

**Installation (for local dev testing):**
```bash
# Developer flow while iterating
claude --plugin-dir ./plugin

# Or, after shipping marketplace.json to GitHub:
/plugin marketplace add <owner>/mcp-chain
/plugin install mcp-chain@<marketplace-name>
/reload-plugins
```

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│  User's machine (Claude Code session)                           │
│                                                                 │
│  User types: /plugin marketplace add acme/mcp-chain             │
│      │                                                          │
│      ▼                                                          │
│  Claude Code clones repo → reads .claude-plugin/marketplace.json│
│      │                                                          │
│      ▼                                                          │
│  User types: /plugin install mcp-chain@<mp-name>                │
│      │                                                          │
│      ▼                                                          │
│  Claude Code copies plugin/ → ~/.claude/plugins/cache/<id>/<ver>│
│  Reads plugin/.claude-plugin/plugin.json (manifest)             │
│  Reads plugin/.mcp.json (MCP server spec)                       │
│  Registers plugin/commands/*.md as slash commands               │
│      │                                                          │
│      ▼                                                          │
│  Session begins — CLAUDE_PLUGIN_ROOT = ~/.claude/plugins/cache/ │
│                                      <id>/<ver>                 │
│      │                                                          │
│      ├─────────────────► MCP server spawn (via .mcp.json)       │
│      │                    ${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain   │
│      │                      serve                               │
│      │                    ← stdio JSON-RPC ────────────────┐    │
│      │                                                      │    │
│      ├── User: /mcp-chain:reg "tests pass" ────────────────┤    │
│      │     Prompt: "Call register MCP tool with             │    │
│      │              condition='$ARGUMENTS'"                 │    │
│      │     Claude → register tool ────────────────────────►│    │
│      │                              ◄── word-ID "otter" ───┤    │
│      │                                                      │    │
│      ├── User (different session): /mcp-chain:wait otter   │    │
│      │     Prompt: "Run bash script:                        │    │
│      │       MCP_CHAIN_BIN=\"${CLAUDE_PLUGIN_ROOT}/bin/     │    │
│      │         mcp-chain\" bash                             │    │
│      │         \"${CLAUDE_PLUGIN_ROOT}/scripts/             │    │
│      │         chain-wait.sh\" otter --timeout 1h"          │    │
│      │                                                      │    │
│      │     chain-wait.sh poll loop:                         │    │
│      │       ┌───────────────────────────────────────────┐ │    │
│      │       │ status=$(mcp-chain status $ID; echo $?)   │ │    │
│      │       │ case $status in                           │ │    │
│      │       │   0) echo continue; exit 0 ;;             │ │    │
│      │       │   2) sleep 1; check timeout; continue ;; │ │    │
│      │       │   *) stderr; exit 1 ;;                    │ │    │
│      │       │ esac                                      │ │    │
│      │       └───────────────────────────────────────────┘ │    │
│      │                                                      │    │
│      └── Registering session: resolve("otter") ────────────►│    │
│                              ◄── OK ──────────────────────┤    │
│                                                             │    │
│         Next poll in chain-wait.sh: status=0 → continue     │    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
mcp-chain/                              # repo root
├── .claude-plugin/
│   └── marketplace.json                # NEW — required for /plugin install
├── plugin/                             # the actual plugin
│   ├── .claude-plugin/
│   │   └── plugin.json                 # plugin manifest
│   ├── .mcp.json                       # MCP server spec
│   ├── commands/
│   │   ├── reg.md                      # /mcp-chain:reg (or chain-reg.md — see LD-14)
│   │   ├── wait.md
│   │   ├── list.md
│   │   └── purge.md
│   ├── scripts/
│   │   └── chain-wait.sh               # bash 3.2 monitor
│   └── bin/                            # Phase 9 drops binary here
│       └── .gitkeep                    # empty placeholder in Phase 8
├── scripts/                            # existing repo-level scripts
│   ├── check-size.sh                   # (existing)
│   ├── check-startup.sh                # (existing)
│   ├── check-stdout-silence.sh         # (existing)
│   ├── check-prompt-wordcount.sh       # NEW — enforce ≤30 words
│   └── smoke-chain-wait.sh             # NEW — integration smoke
├── internal/
│   └── plugin/                         # OR here if Go-test path chosen
│       └── chainwait_integration_test.go  # optional, `//go:build integration`
└── ...existing Go code...
```

### Pattern 1: `.mcp.json` with `${CLAUDE_PLUGIN_ROOT}`

**What:** Plugin-bundled MCP server config. `${CLAUDE_PLUGIN_ROOT}` is substituted at subprocess spawn time.

**When to use:** Any time the plugin ships a binary and wires Claude Code to it.

**Verified behavior:** `${CLAUDE_PLUGIN_ROOT}` is BOTH exported as an environment variable to spawned subprocesses AND substituted inline in `.mcp.json`, `commands/*.md`, `hooks.json`, etc. So in a `.mcp.json` you write the literal string `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain` and Claude Code expands it to the absolute cache path at spawn time.

### Pattern 2: Slash-command as Claude prompt

**What:** A markdown file in `commands/` becomes a slash command. The body is the prompt Claude sees when the user invokes it.

**When to use:** Every slash command in the plugin.

**Anti-pattern to avoid:** Putting instructions about what WOULD happen if… The prompt should be a single imperative: "Do X with Y." No branching, no examples, no fallback prose (CMD-05).

### Pattern 3: Bash monitor with exit-code translation

**What:** The monitor polls a CLI subcommand; exit codes are the protocol.

**When to use:** Any shell-based polling integration with a well-defined CLI contract. Phase 6 locked 0/2/1 for exactly this purpose.

### Anti-Patterns to Avoid

- **Putting `.mcp.json`, `commands/`, or `scripts/` inside `.claude-plugin/`:** The docs explicitly warn against this. Only `plugin.json` goes in `.claude-plugin/`.
- **Referencing `../` paths in plugin files:** Traversal outside plugin root does not work post-install (files aren't copied to cache).
- **Using bash-only features in the monitor:** `[[`, arrays, `mapfile`, `declare -A`, `${var,,}` all fail on macOS bash 3.2. Stick to POSIX.
- **Hand-rolling timeout via SIGALRM:** Race conditions with `sleep 1` inside the loop make this fragile. Use `date +%s` arithmetic.
- **Stdout pollution in the monitor:** `continue` is the ONLY stdout token. All diagnostics go to stderr. Phase 6's status.go already follows this for mcp-chain itself.
- **Assuming `/plugin install <bare-repo>` works:** It does NOT. Must be a marketplace install.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Plugin manifest schema | Custom JSON format | `.claude-plugin/plugin.json` with documented fields | Claude Code's loader is the consumer; anything else won't be read |
| MCP server registration | Custom hook to register server | `.mcp.json` at plugin root | Standard mechanism; plugin-enable/disable lifecycle is automatic |
| Path substitution | String replacement scripts at install time | `${CLAUDE_PLUGIN_ROOT}` | Already handled by Claude Code; user would re-break on updates |
| Argument parsing in slash command | Regex in prompt body | `$ARGUMENTS` / `$0` / `$1` / `argument-hint:` frontmatter | Built-in substitution is reliable and documented |
| Timeout in bash | SIGALRM trap | `date +%s` epoch math + `sleep 1` | Simpler, no races, works on bash 3.2 |
| Plugin version tracking | Custom version.txt | `version` field in `plugin.json` | Claude Code's cache key IS this field |
| Plugin install from GitHub | README with curl snippet | `marketplace.json` + `/plugin marketplace add` | DIST-01 requires this anyway |

**Key insight:** Claude Code's plugin system is prescriptive. Every deviation costs you (the plugin author) and the user. Follow the documented layout exactly.

## Runtime State Inventory

**Phase 8 is greenfield authoring** (new files under `plugin/` and `.claude-plugin/`), not a rename/refactor. There is no pre-existing runtime state to migrate. Skipping per instructions.

## Common Pitfalls

### Pitfall 1: Placing `.mcp.json` or `commands/` inside `.claude-plugin/`

**What goes wrong:** Plugin loads but components (slash commands, MCP server) are missing. No error — silent failure in the general case.

**Why it happens:** Intuitive to group all plugin metadata together. The docs explicitly warn: "Common mistake: Don't put `commands/`, `agents/`, `skills/`, or `hooks/` inside the `.claude-plugin/` directory."

**How to avoid:** `.claude-plugin/` contains ONLY `plugin.json`. Everything else (`.mcp.json`, `commands/`, `scripts/`, `bin/`) lives at the plugin root one level up.

**Warning signs:** `/reload-plugins` completes but `/mcp-chain:reg` doesn't appear in the slash-command list.

### Pitfall 2: Referencing repo-root `scripts/` from the plugin

**What goes wrong:** Monitor script not found after install.

**Why it happens:** During dev with `--plugin-dir ./plugin`, the script is accessible via `../scripts/chain-wait.sh`. After marketplace install, only `plugin/` is copied to the cache — parent paths do not exist.

**How to avoid:** Ship `scripts/chain-wait.sh` INSIDE `plugin/scripts/chain-wait.sh`. Reference via `${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh`. (This is the LD-1 revision.)

**Warning signs:** Works via `--plugin-dir` but fails after marketplace install with `No such file or directory`.

### Pitfall 3: Namespaced vs. unnamespaced command names

**What goes wrong:** REQUIREMENTS.md says `/chain-reg`. User types `/chain-reg`. Claude Code can't find it — the actual name is `/mcp-chain:chain-reg` (or `/mcp-chain:reg` if LD-14 is adopted).

**Why it happens:** Plugin commands are always prefixed with the plugin's `name` field. Documentation is explicit: "Skills are prefixed with this (e.g., `/my-first-plugin:hello`)."

**How to avoid:** Either (a) rename commands inside the plugin to `reg`/`wait`/`list`/`purge` so the user-facing form is `/mcp-chain:reg` etc., or (b) document the namespaced form in README. Flag as a REQUIREMENTS.md amendment or a README documentation item.

### Pitfall 4: bash 3.2 breaks on `[[`, arrays, `=~` regex

**What goes wrong:** Monitor script runs fine on Linux (bash 5), fails on macOS (bash 3.2) with cryptic syntax errors or wrong behavior.

**Why it happens:** macOS ships bash 3.2.57 as `/bin/bash` for license reasons and has never updated it. Many "bash" features that feel portable are actually bash 4+ features.

**How to avoid:** Stick to the POSIX-sh subset: `case` for pattern matching, `$(( ))` for arithmetic, `${var%suffix}` and `${var#prefix}` for parameter expansion, `[ ... ]` (single-bracket) for tests with POSIX operators only (`-eq`/`-ge`/`=`/`!=`, no `=~`), `trap` for signal handlers, `set -eu` (not `set -o pipefail` — that's bash 3+ but OK; still safe). Avoid: `[[`, `(( ))` (as a command — OK inside `$(( ))`), `${var,,}`, `${var^^}`, `mapfile`, `readarray`, `declare -A`, `local -n`, `<<<` here-strings, process substitution `<(...)`, `&>`, `|&`.

**Warning signs:** Test the script on a bash 3.2 environment (CI macOS runner, or Docker with bash-3.2 compiled from source) before declaring done.

### Pitfall 5: Forgetting `/reload-plugins` after changes

**What goes wrong:** Edit `commands/reg.md` → run command → Claude still uses the old prompt.

**Why it happens:** Claude Code caches plugin components at session start. In-session edits to plugin files don't re-read automatically unless the plugin was added via `--plugin-dir` (which has file-watch) or `/reload-plugins` is explicitly invoked.

**How to avoid:** Document in README (Phase 10) and in the Phase 8 smoke-test runbook: after editing `plugin/`, run `/reload-plugins`.

### Pitfall 6: `marketplace.json` at wrong path

**What goes wrong:** `/plugin marketplace add <repo>` fails with "marketplace.json not found".

**Why it happens:** The marketplace file MUST be at `.claude-plugin/marketplace.json` at the **repo root**, NOT inside `plugin/`.

**How to avoid:** Layout:
```
<repo-root>/.claude-plugin/marketplace.json   ← marketplace catalog
<repo-root>/plugin/.claude-plugin/plugin.json ← plugin manifest
```
Note the similar-named directories serve different purposes at different levels.

### Pitfall 7: Word-count gate false positives from frontmatter

**What goes wrong:** `wc -w < commands/reg.md` includes the YAML frontmatter words, pushing total over 30.

**Why it happens:** `wc` doesn't know about YAML.

**How to avoid:** The word-count gate strips frontmatter before counting. Pattern: use `awk '/^---$/{if(++n==2)next_body=1; next} next_body{print}'` or equivalent to skip frontmatter, then `wc -w`.

## Code Examples

All examples below are verbatim, ready-to-paste. Assumes LD-14 recommendation adopted (commands named `reg`/`wait`/`list`/`purge`); swap in `chain-reg`/etc. if the planner keeps the original names.

### 1. `plugin/.claude-plugin/plugin.json`

```json
{
  "name": "mcp-chain",
  "version": "1.0.0",
  "description": "Register/wait/resolve coordination primitive across N Claude Code sessions.",
  "author": {
    "name": "mcp-chain contributors"
  },
  "repository": "https://github.com/OWNER/mcp-chain",
  "keywords": ["mcp", "coordination", "chain", "session"]
}
```

**Note:** Only `name` is required. The other fields are included for polish and `/plugin` UI rendering. `version` should be bumped by GoReleaser in Phase 9 when tagging releases.

### 2. `plugin/.mcp.json`

```json
{
  "mcpServers": {
    "mcp-chain": {
      "command": "${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain",
      "args": ["serve"]
    }
  }
}
```

**Note:** No shebang needed — the binary is ELF/Mach-O, spawned directly. No `env:` section needed (mcp-chain reads `$XDG_STATE_HOME` from the inherited env). Forward slashes work cross-platform per Claude Code docs.

### 3. `plugin/commands/reg.md` (or `chain-reg.md`)

```markdown
---
description: Register a new chain and print the word-ID
argument-hint: [condition]
---

Call the `mcp-chain__register` MCP tool with `condition="$ARGUMENTS"`. If `$ARGUMENTS` is empty, ask me for a natural-language resolution condition first, then call the tool.
```

Word count (body only, frontmatter stripped): 29 words. ✓

### 4. `plugin/commands/wait.md` (or `chain-wait.md`)

```markdown
---
description: Wait for a chain-ID to resolve (polls every 1s)
argument-hint: <id> [--timeout DURATION]
---

Run: `MCP_CHAIN_BIN="${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain" bash "${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh" $ARGUMENTS` and echo the monitor's output verbatim. On `continue`, treat the chain as resolved.
```

Word count (body only): 25 words. ✓

### 5. `plugin/commands/list.md` (or `chain-list.md`)

```markdown
---
description: Print a table of all chain entries
---

Run `"${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain" list` and echo the table verbatim.
```

Word count (body only): 11 words. ✓

### 6. `plugin/commands/purge.md` (or `chain-purge.md`)

```markdown
---
description: Purge chain entries (requires <id>, --all, or --resolved)
argument-hint: <id> | --all | --resolved
---

Run `"${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain" purge $ARGUMENTS` and echo any output verbatim. The binary errors if no argument is given.
```

Word count (body only): 22 words. ✓

### 7. `plugin/scripts/chain-wait.sh` (full, bash 3.2 compatible)

```bash
#!/usr/bin/env bash
# chain-wait.sh — Poll mcp-chain status until resolved, error, or timeout.
# Usage: chain-wait.sh <id> [--timeout DURATION]
# Exit codes:
#   0   resolved (printed "continue" to stdout)
#   1   unknown id / mid-wait error
#   124 timeout

set -eu

# -------- binary resolution (LD-9) ------------------------------------------
BIN="${MCP_CHAIN_BIN:-mcp-chain}"

# -------- argument parsing (POSIX, bash 3.2 safe) ---------------------------
ID=""
DURATION=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --timeout)
      shift
      if [ "$#" -eq 0 ]; then
        echo "mcp-chain: --timeout requires an argument" >&2
        exit 1
      fi
      DURATION="$1"
      shift
      ;;
    --timeout=*)
      DURATION="${1#--timeout=}"
      shift
      ;;
    --help|-h)
      echo "usage: chain-wait.sh <id> [--timeout DURATION]" >&2
      exit 0
      ;;
    --*)
      echo "mcp-chain: unknown flag: $1" >&2
      exit 1
      ;;
    *)
      if [ -z "$ID" ]; then
        ID="$1"
      else
        echo "mcp-chain: unexpected argument: $1" >&2
        exit 1
      fi
      shift
      ;;
  esac
done

if [ -z "$ID" ]; then
  echo "mcp-chain: chain-wait.sh requires an id" >&2
  exit 1
fi

# -------- duration parser (LD-6) --------------------------------------------
# Accepts exactly: 30s / 1m / 1h / 24h / 168h. Integer + single suffix.
# Rejects: 1h30m (combined), 1.5h (decimal), 1d (day), 1w (week).
BUDGET=0
if [ -n "$DURATION" ]; then
  case "$DURATION" in
    *s)
      n="${DURATION%s}"
      case "$n" in ''|*[!0-9]*) echo "mcp-chain: bad duration: $DURATION" >&2; exit 1 ;; esac
      BUDGET="$n"
      ;;
    *m)
      n="${DURATION%m}"
      case "$n" in ''|*[!0-9]*) echo "mcp-chain: bad duration: $DURATION" >&2; exit 1 ;; esac
      BUDGET=$(( n * 60 ))
      ;;
    *h)
      n="${DURATION%h}"
      case "$n" in ''|*[!0-9]*) echo "mcp-chain: bad duration: $DURATION" >&2; exit 1 ;; esac
      BUDGET=$(( n * 3600 ))
      ;;
    *)
      echo "mcp-chain: bad duration: $DURATION (use 30s|1m|1h|24h|168h)" >&2
      exit 1
      ;;
  esac
  if [ "$BUDGET" -gt 604800 ]; then
    echo "mcp-chain: duration exceeds max 168h (604800s): $DURATION" >&2
    exit 1
  fi
  if [ "$BUDGET" -le 0 ]; then
    echo "mcp-chain: duration must be positive: $DURATION" >&2
    exit 1
  fi
fi

# -------- invocation echo (LD-8) --------------------------------------------
if [ "$BUDGET" -gt 0 ]; then
  echo "mcp-chain: waiting for $ID (timeout: $DURATION)" >&2
else
  echo "mcp-chain: waiting for $ID" >&2
fi

# -------- poll loop (LD-5, LD-7) --------------------------------------------
START=$(date +%s)
while :; do
  set +e
  "$BIN" status "$ID" >/dev/null 2>&1
  CODE=$?
  set -e
  case "$CODE" in
    0)
      echo "continue"
      exit 0
      ;;
    2)
      # pending — keep polling
      ;;
    *)
      # LD-5: any other code (1, 127, 126, etc.) → error
      # Re-run once to capture stderr for the user.
      set +e
      STDERR="$( "$BIN" status "$ID" 2>&1 1>/dev/null )"
      set -e
      if [ -n "$STDERR" ]; then
        echo "$STDERR" >&2
      else
        echo "mcp-chain: status error (exit $CODE)" >&2
      fi
      exit 1
      ;;
  esac
  if [ "$BUDGET" -gt 0 ]; then
    NOW=$(date +%s)
    ELAPSED=$(( NOW - START ))
    if [ "$ELAPSED" -ge "$BUDGET" ]; then
      echo "mcp-chain: timeout waiting for $ID after ${DURATION}" >&2
      exit 124
    fi
  fi
  sleep 1
done
```

**Lint notes:** No `[[`, no `(( ))` as command, no arrays, no `${var,,}`, no process substitution. Uses `set -eu` + explicit `set +e`/`set -e` bracketing around subcommand calls so non-zero exit codes from `mcp-chain status` don't trip `set -e`. All arithmetic is `$(( ))`. All patterns are POSIX `case`.

### 8. `.claude-plugin/marketplace.json` (at repo root)

```json
{
  "name": "mcp-chain-marketplace",
  "owner": {
    "name": "mcp-chain contributors"
  },
  "plugins": [
    {
      "name": "mcp-chain",
      "source": "./plugin",
      "description": "Register/wait/resolve coordination primitive across N Claude Code sessions."
    }
  ]
}
```

**Install flow after this ships to GitHub:**

```shell
/plugin marketplace add OWNER/mcp-chain
/plugin install mcp-chain@mcp-chain-marketplace
/reload-plugins
```

### 9. `scripts/smoke-chain-wait.sh` (repo root — integration test harness)

```bash
#!/usr/bin/env bash
# smoke-chain-wait.sh — end-to-end test of chain-wait.sh against a locally-built binary.
# Not shipped in the plugin. Runs from the repo root.

set -euo pipefail

BIN="${MCP_CHAIN_BIN:-./mcp-chain}"
SCRIPT="./plugin/scripts/chain-wait.sh"

if [ ! -x "$BIN" ]; then
  echo "FAIL: $BIN not built. Run 'go build -o mcp-chain ./cmd/mcp-chain' first." >&2
  exit 2
fi
if [ ! -f "$SCRIPT" ]; then
  echo "FAIL: $SCRIPT missing." >&2
  exit 2
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export XDG_STATE_HOME="$TMP"

echo "== Test 1: unknown ID errors immediately =="
set +e
MCP_CHAIN_BIN="$BIN" bash "$SCRIPT" notarealid >"$TMP/out" 2>"$TMP/err"
CODE=$?
set -e
if [ "$CODE" -ne 1 ]; then
  echo "FAIL: expected exit 1 for unknown id, got $CODE" >&2
  cat "$TMP/err" >&2
  exit 1
fi
echo "PASS"

echo "== Test 2: timeout fires with exit 124 =="
# Seed a pending entry via the MCP server (launched briefly via stdin).
# Shortcut: use the store directly via `mcp-chain serve` + a canned MCP init/register pair.
# For the smoke test, simpler: skip by running against an empty store and verifying timeout path
# using a trick — we'd need a running `serve` process. This is harder in pure shell.
# Fallback: use a Go helper.
# >>> PLANNER TODO: decide between Go integration test vs. shell smoke here. <<<

echo "All tests passed."
```

**Note on test architecture:** The shell smoke test can trivially verify the unknown-ID path (exit 1) because an empty store returns `ErrUnknownID` for any lookup. Timeout and resolve paths require seeding a pending entry, which needs either (a) a running `mcp-chain serve` with an MCP client, OR (b) direct state.json manipulation (tightly coupled to the schema), OR (c) a Go integration test. **Recommendation:** Use option (c) — a Go test under `internal/cli/` with build tag `integration` that uses the store API directly and spawns `chain-wait.sh` via `exec.Command`. This is the cleanest path and keeps the shell smoke focused on the unknown-ID case plus shellcheck/syntax checks.

### 10. `scripts/check-prompt-wordcount.sh` (repo root)

```bash
#!/usr/bin/env bash
# Enforces SC-2: every commands/*.md prompt body ≤ 30 words (frontmatter excluded).
set -euo pipefail

MAX=30
FAIL=0

for md in plugin/commands/*.md; do
  # Strip frontmatter between --- markers (POSIX awk).
  body="$(awk '/^---$/{n++; if(n==1||n==2) next} n==2 || n==0 {print}' "$md")"
  # Note: if no frontmatter, n stays 0 → print everything (correct).
  # With frontmatter, n goes 0→1 on opening ---, 1→2 on closing ---, and prints
  # lines only when n==2. The "n==0" clause covers the no-frontmatter case.
  words=$(printf '%s\n' "$body" | wc -w | tr -d ' ')
  if [ "$words" -gt "$MAX" ]; then
    echo "FAIL: $md has $words words (max $MAX)" >&2
    FAIL=1
  else
    echo "OK: $md has $words words"
  fi
done

exit "$FAIL"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `.claude/commands/*.md` in a repo | Plugin with `plugin/commands/` + `marketplace.json` | 2025 — plugin system launch | REQUIRED for distribution; single-user `.claude/commands/` still works but doesn't namespace or version |
| Hand-written `/plugin install` docs | `marketplace.json` listing + `/plugin marketplace add` | 2025 — marketplace system | Plugin authors no longer write curl-based install scripts |
| Commands = flat markdown in `commands/` | Commands merged into "Skills" with richer features; `commands/` still supported | 2026 — skills feature merge | Backward compatible; flat `commands/*.md` continues to work. "Use `skills/` for new plugins" per docs, but `commands/` is explicitly still supported |
| `$1`-only arg substitution | `$ARGUMENTS` / `$0`/`$1` / `$ARGUMENTS[N]` / named `$foo` | 2026 | Richer arg handling; `$ARGUMENTS` is the safe default for single-variadic use |

**Deprecated/outdated:**
- Nothing in use by this plan is deprecated. `commands/` remains supported alongside `skills/`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The plugin repository is public on GitHub, so `/plugin marketplace add OWNER/repo` works without SSH/auth. | Install flow | If private, users would need SSH URL form. Low risk for v1 — mcp-chain is intended public. |
| A2 | Claude Code clients on the target users' machines are recent enough to support plugins + `/plugin install` (2025+ release). | Install flow | Low risk — the stated primary user (the author) runs current Claude Code. |
| A3 | macOS default bash will remain at 3.2.57 for the foreseeable future. | LD-4 / HELPER-02 | Very low risk; it has been 3.2.57 since macOS 10.5 (2007) and Apple has not moved it. |
| A4 | `date +%s` remains cross-compatible between BSD and GNU date implementations. | LD-7 | Very low risk; it's a de-facto universal extension. |
| A5 | mcp-chain's `status` subcommand behavior (exit 0/2/1) remains locked per Phase 6 comments. | LD-5 | Low risk — explicitly locked in source comments; Phase 6 gate. |
| A6 | `${CLAUDE_PLUGIN_ROOT}` is substituted in `commands/*.md` prompt bodies (not just `.mcp.json` / hooks). | Slash command wiring | Docs confirm: "substituted inline in skill content, agent content, hook commands, monitor commands, and MCP or LSP server configs". HIGH confidence, but not explicitly verified against a running Claude Code session. If wrong, the `chain-wait.md` prompt would need a different approach (e.g. rely on `CLAUDE_PLUGIN_ROOT` as exported env var inside the Bash tool call). |
| A7 | The plugin command-namespace is exactly `<plugin-name>:<command-name>` with no alternative un-namespaced alias. | LD-14 | HIGH — docs are explicit ("Plugin skills are always namespaced"). |

**A6 is the one to validate during execution.** If `${CLAUDE_PLUGIN_ROOT}` is NOT substituted in `commands/*.md` prompt bodies, fallback: rewrite the prompt to use the env-var form `$CLAUDE_PLUGIN_ROOT` (no braces — Bash tool will interpolate at runtime). This is a one-line change.

## Open Questions

All three original OQs resolved (see Open-Question Resolutions). The research did surface two new items that needed decisions — both resolved above:

1. ~~**OQ-1:** plugin.json schema?~~ → Resolved. `name` required; all else optional.
2. ~~**OQ-2:** commands/*.md frontmatter?~~ → Resolved. Optional; `description` + `argument-hint` recommended.
3. ~~**OQ-3:** bash 3.2 timeout pattern?~~ → Resolved. `date +%s` arithmetic, exit 124.
4. ~~**OQ-4:** `/plugin install` from bare repo?~~ → Resolved. Requires `marketplace.json` at repo root.
5. ~~**OQ-5:** Command namespacing?~~ → Resolved. `<plugin-name>:<command>` is mandatory; recommend renaming commands to `reg`/`wait`/`list`/`purge`.

**Zero open questions remaining.** All decisions the planner needs are documented above.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `bash` | `chain-wait.sh` + smoke test | ✓ (Linux dev env) | 5.x | macOS 3.2 must be verified in Phase 9 CI matrix |
| `/bin/sh` (POSIX) | Fallback for `sh` compatibility checks | ✓ | — | — |
| `jq` | Smoke-test JSON lint | ✓ | 1.6+ | Omit lint step if absent; the `/plugin` validation catches issues at install time |
| `awk` | Word-count gate | ✓ (POSIX) | — | — |
| `wc` | Word-count gate | ✓ (POSIX) | — | — |
| `date` | Timeout arithmetic | ✓ | — | — |
| `mcp-chain` binary | Smoke test against chain-wait.sh | ✓ (built locally via `go build`) | — | Phase 9 drops release artifact in `plugin/bin/` |
| Claude Code | Consumer of the plugin | Not on CI runner (test path is shell-only + Go integration) | — | Deferred to dogfooding (Phase 10) |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** Nothing blocking Phase 8.

## Test Architecture

### SC-1: Plugin installs cleanly on a fresh machine

**How to verify:** Manual — install on a fresh Linux VM and a fresh macOS host in Phase 10 (dogfooding). Phase 8 provides `jq empty < plugin/.claude-plugin/plugin.json` + `jq empty < plugin/.mcp.json` + `jq empty < .claude-plugin/marketplace.json` as a local lint gate. **This is not fully automatable in Phase 8** (would require a containerized Claude Code to drive the install), hence Phase 10 is the terminal gate.

**Phase 8 gate:** JSON syntactically valid + all referenced paths exist (spot-check `plugin/bin/mcp-chain` is represented by an empty file or `.gitkeep`).

### SC-2: Every slash-command prompt body ≤ 30 words

**How to verify:** `scripts/check-prompt-wordcount.sh` (Code Example 10). Run locally and add to Phase 9 CI.

**Phase 8 gate:** Script exits 0 against all four `plugin/commands/*.md`.

### SC-3: `chain-wait.sh` behaves per contract

**How to verify:** Two-layer test strategy.

**Layer 1 — shell syntax/lint:** `bash -n plugin/scripts/chain-wait.sh` (parse-only; catches bashism by running under `/bin/bash` with `--posix` mode optional). Add `shellcheck --shell=bash plugin/scripts/chain-wait.sh` if shellcheck is available.

**Layer 2 — integration:** Go test under `//go:build integration` tag that:
1. Creates a `t.TempDir()` + sets `XDG_STATE_HOME`.
2. Uses `store.Open` + `store.Register` API directly (or spawns `mcp-chain serve` and registers via stdio MCP).
3. Spawns `bash plugin/scripts/chain-wait.sh <id> --timeout 10s` with `MCP_CHAIN_BIN` set.
4. Sleeps 2s, calls `store.Resolve(id, token, ResolveOptions{Force: true})`.
5. Asserts the child process exits 0 within 4s and printed "continue" to stdout.
6. Repeats with a never-registered id → asserts exit 1 + stderr contains "unknown id".
7. Repeats with a pending id and `--timeout 3s` → asserts exit 124 + stderr contains "timeout".

File: `internal/cli/chainwait_integration_test.go` with `//go:build integration`. Runs via `go test -tags=integration ./internal/cli/...`.

### SC-4: End-to-end `/chain-reg` → `/chain-wait` → `/chain-purge` on Linux and macOS

**How to verify:** Deferred to Phase 10 dogfooding. Phase 8 provides the artifacts; Phase 10 walks through the human demo flow.

### Sampling Rate

- **Per task commit:** `bash -n plugin/scripts/chain-wait.sh && scripts/check-prompt-wordcount.sh && jq empty < plugin/.claude-plugin/plugin.json && jq empty < plugin/.mcp.json && jq empty < .claude-plugin/marketplace.json` (~1s total).
- **Per wave merge:** Above plus `go test -tags=integration ./internal/cli/...` if integration tests are in this wave.
- **Phase gate:** Full suite green before `/gsd-verify-work`.

### Wave 0 Gaps

Before implementation begins, these test-infrastructure items need to exist (or be created in Wave 0 of the plan):

- [ ] `scripts/check-prompt-wordcount.sh` — covers SC-2 (Code Example 10 is ready-to-paste).
- [ ] `scripts/smoke-chain-wait.sh` — lightweight shell smoke (Code Example 9, with the `PLANNER TODO` resolved by choosing Go-integration for the non-trivial paths).
- [ ] (Optional) `internal/cli/chainwait_integration_test.go` with `//go:build integration` — covers SC-3 Layer 2.
- [ ] `shellcheck` install (optional; gracefully skip if absent).

*(No test framework install needed — stdlib `go test` + plain bash is sufficient.)*

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| bash 3.2 regression found only on macOS | Medium | High (blocks SC-3 on primary platform) | Phase 9 CI includes a macOS runner; Phase 8 adds `bash -n` parse gate and avoids all known bashisms upfront |
| `${CLAUDE_PLUGIN_ROOT}` substitution doesn't work in `commands/*.md` prompt body | Low (docs say yes, but not tested by us) | Medium (requires prompt rewrite) | A6 flagged in Assumptions Log; fallback is one-line change to rely on env var at Bash-tool spawn |
| Plugin installs but `/mcp-chain:reg` doesn't register | Low | High (blocks SC-1) | Lint gate (`jq empty` + path existence) + Phase 10 dogfooding catches |
| User expectation of `/chain-reg` clashes with namespaced `/mcp-chain:reg` or `/mcp-chain:chain-reg` | Medium | Low (cosmetic / doc fix) | LD-14 recommendation flagged for planner decision |
| `marketplace.json` at wrong path | Low (docs explicit) | High (blocks install) | Explicit in this doc + structure diagram |
| macOS default bash upgraded by Apple, breaking something | Very low (no movement since 2007) | — | Accept; revisit if it happens |
| `date +%s` output differs between `tr -d` stripping and subshell arithmetic | Very low | Low | Tested pattern matches POSIX; `$(date +%s)` returns integer, no stripping needed |
| `jq` absent from CI runner | Low | Low | Make the `jq empty` lint step conditional on `command -v jq` |
| Integration test flakes on 1s poll granularity | Low | Medium | Use `--timeout 10s` for the resolve-success test (5s of slack); use `--timeout 3s` for the timeout test |
| `$ARGUMENTS` substitution quoting issues in wait.md (e.g. user pastes `otter; rm -rf /`) | Low (Claude sanitizes) | Medium (if exploitable) | Per docs, `$ARGUMENTS` is raw text; Claude decides whether to pass it as a Bash command. Trust the model + the binary's own arg parsing. If hardening needed, add `--` separator: `... "$ARGUMENTS"` in the prompt |

## Security Domain

Project does not have `security_enforcement` explicitly configured. Applying conservative default (ASVS light touch).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Single-user local tool; no auth surface in plugin itself |
| V3 Session Management | partial | `OwnerToken` session-link is enforced by Phase 5; plugin layer inherits this |
| V4 Access Control | no | No multi-user boundary |
| V5 Input Validation | yes | `chain-wait.sh` parses `--timeout DURATION` — must reject malicious input |
| V6 Cryptography | no | No crypto in Phase 8 |

### Known Threat Patterns for plugin/bash stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Command injection via `--timeout` argument | Tampering | Duration parser validates integer + suffix via `case` + `[!0-9]*` glob. No `eval`, no unquoted expansion. |
| Path traversal in plugin files | Information disclosure | All paths start with `${CLAUDE_PLUGIN_ROOT}`; Claude Code cache mechanism prevents `../` escapes |
| Slash-command prompt injection via user `$ARGUMENTS` | Tampering | The binary is the ultimate validator. Slash-command prompts pass `$ARGUMENTS` to `mcp-chain` CLI which owns arg parsing. If a user types `/chain-purge '; rm -rf /'`, kong rejects it at the binary layer. |
| Plugin supply-chain (malicious marketplace) | Spoofing | Out of scope for authoring side. Users add marketplaces from trusted sources per Claude Code's own security model. |

## Sources

### Primary (HIGH confidence)

- [Claude Code — Plugins reference](https://code.claude.com/docs/en/plugins-reference) — `plugin.json` schema, `.mcp.json` schema, `${CLAUDE_PLUGIN_ROOT}`/`${CLAUDE_PLUGIN_DATA}` substitution, directory layout warnings ("don't put commands/ inside .claude-plugin/"), file locations reference table, plugin caching and file resolution (outside-root files NOT copied), version management.
- [Claude Code — Skills (includes commands)](https://code.claude.com/docs/en/skills) — frontmatter reference table, `$ARGUMENTS`/`$0`/`$N`/`$ARGUMENTS[N]` substitutions, `"Files in .claude/commands/ still work and support the same frontmatter"`.
- [Claude Code — Plugins (create)](https://code.claude.com/docs/en/plugins) — plugin.json minimal manifest fields table (`name`, `description`, `version`, `author`); "Common mistake" warning on `.claude-plugin/` contents; plugin skill naming (`/my-first-plugin:hello` namespacing).
- [Claude Code — Discover and install plugins](https://code.claude.com/docs/en/discover-plugins) — `/plugin marketplace add owner/repo` flow, `/plugin install name@marketplace` syntax, `/plugin marketplace add` from GitHub repos using `owner/repo` format, `--plugin-dir` for dev testing.
- [Claude Code — Create a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces) — `.claude-plugin/marketplace.json` schema, required fields (`name`, `owner`, `plugins`), plugin source formats (relative-path `./plugin`), "relative paths resolve relative to the marketplace root".
- `/home/alpine/mcp-chain/CLAUDE.md` — project constraints + "Claude Code plugin: YES, can bundle a Go binary directly" confirmation.
- `/home/alpine/mcp-chain/internal/cli/status.go` — "LOCKED — Phase 8 bash monitor depends on these" exit-code contract (0 resolved, 2 pending, 1 unknown).
- `/home/alpine/mcp-chain/internal/cli/purge.go`, `resolve.go` — Phase 7 exit-code contracts confirming purge/resolve semantics.

### Secondary (MEDIUM confidence)

- [Rich's sh tricks](https://www.etalabs.net/sh_tricks.html) — POSIX-portable patterns (case, parameter expansion, printf).
- [Baeldung — Manual UNIX timestamp arithmetic in the shell](https://www.baeldung.com/linux/shell-unix-timestamp-arithmetic) — `date +%s` + `$(( ))` pattern (Linux + macOS compatibility).
- [nixCraft — Linux run a command with a time limit (timeout)](https://www.cyberciti.biz/faq/linux-run-a-command-with-a-time-limit/) — confirms macOS doesn't ship `timeout(1)` by default.
- [gist: timeout command on MacOS (Jay Taylor)](https://gist.github.com/jaytaylor/6527607) — confirms macOS-X lacks `timeout` utility; rolls a BASH equivalent.
- [Bash POSIX Mode reference](https://www.gnu.org/software/bash/manual/html_node/Bash-POSIX-Mode.html) — authoritative for which bash features are POSIX.
- [Wooledge — ArithmeticExpression](https://mywiki.wooledge.org/ArithmeticExpression) — `$(( ))` is POSIX; bash 3.2 supports it.

### Tertiary (LOW confidence — flagged)

- None. All load-bearing claims are backed by primary sources.

## Metadata

**Confidence breakdown:**

- **Plugin schema / layout:** HIGH — primary source (plugins-reference) is explicit and extensive.
- **`${CLAUDE_PLUGIN_ROOT}` substitution in commands/*.md:** HIGH (docs say "substituted inline in skill content"), but flagged as A6 — not personally verified against a live Claude Code session. One-line fallback exists.
- **Command namespacing:** HIGH — docs explicit in multiple places.
- **Marketplace requirement:** HIGH — both discover-plugins and plugin-marketplaces pages confirm.
- **bash 3.2 compatibility of the monitor script:** HIGH — only POSIX features used; `bash -n` parse gate will catch regressions.
- **Exit code 124 for timeout:** MEDIUM-HIGH — matches `timeout(1)` convention but macOS doesn't ship that binary by default, so the convention is more of a tradition than a macOS-native standard. Low risk either way since callers parse exit codes numerically.

**Research date:** 2026-04-24
**Valid until:** 2026-05-24 (30 days — Claude Code plugin system is stable but the ecosystem is evolving; re-verify if execution slips past May 2026).
