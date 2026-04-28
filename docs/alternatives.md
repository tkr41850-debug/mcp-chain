# Alternatives to mcp-chain

**Research date:** 2026-04-28
**Research method:** Four parallel WebSearch + WebFetch subagents.
**Scope:** Mechanisms for chaining or coordinating multiple Claude Code sessions with separate context windows.

> Findings are point-in-time as of **2026-04-28**. Anthropic's first-party tooling and the MCP server ecosystem are both moving fast — re-run before relying on competitive positioning. Cited URLs are inline.

## TL;DR

- Native Claude Code primitives (subagents, `--fork-session`, Agent Teams, async background subagents) cover **intra-session** and **single-team** coordination. They do not address arbitrary cross-session rendezvous.
- Two MCP servers occupy the same niche: **claude-presence** (named-resource locks) and **mclaude** (atomic locks + handoffs + memory graph).
- Mainstream community pattern is **git worktrees + tmux**, with the human as orchestrator. Most "orchestration wrappers" layer on subagents within one session — they sidestep cross-session sync rather than solve it.
- mcp-chain's defensible differentiation: ephemeral word-IDs for true N-to-N topology, in a single Go binary with a tight resource budget. "Lightweight cross-session sync" alone is no longer table-stakes-distinctive.

## Native Claude Code mechanisms

### Subagents (Task tool)

- Isolated context per agent; parent receives full transcript at `~/.claude/projects/{sessionId}/subagents/agent-{agentId}.jsonl`.
- **Cannot nest** — subagents cannot spawn subagents.
- Strictly intra-session.
- Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents.md)

### `claude -p` (headless mode)

- Fresh context per invocation; no state bridge between invocations. Chain via shell + files.
- `--continue` and `-p` don't compose.
- Source: [code.claude.com/docs/en/cli-reference](https://code.claude.com/docs/en/cli-reference.md)

### `--resume` vs `--fork-session`

- `--resume <id>` continues an existing session linearly.
- `--fork-session` preserves history up to the fork point and assigns a new session ID; can spawn multiple branches from one base.
- Neither offers wait/coordination primitives.
- Source: [code.claude.com/docs/en/how-claude-code-works](https://code.claude.com/docs/en/how-claude-code-works.md)

### Agent Teams (experimental)

- Gated by `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`.
- Shared task list with dependency tracking, file-locked claim semantics, automatic unblock-when-done, mailbox messaging.
- Scoped to one team-lead session + ~3–5 teammates — not arbitrary cross-conversation coordination.
- **Closest first-party competitor to mcp-chain conceptually.**
- Source: [code.claude.com/docs/en/agent-teams](https://code.claude.com/docs/en/agent-teams)

### Async background subagents (v2.0.64+)

- Background tasks wake the parent agent when done. Solves async waiting *within* a session tree.
- Source: [GitHub issue #9905](https://github.com/anthropics/claude-code/issues/9905)

### Hooks

- Cannot reliably shell out to launch a new `claude` session — racing against session exit.
- Workaround: write a sentinel file from a `Stop` hook; have an external scheduler poll it.
- Feature request [#4446](https://github.com/anthropics/claude-code/issues/4446) for chainable hooks remains open.
- Hooks fire identically in headless and interactive modes.

## Direct MCP-server competitors

| Project | Primitive | Distinctive | Status |
|---|---|---|---|
| [garniergeorges/claude-presence](https://github.com/garniergeorges/claude-presence) | Advisory resource locks + presence + broadcast inbox | **Closest analog.** Named resources (`ci`, `staging-db`); SQLite-backed; zero daemon | Active |
| [AnastasiyaW/mclaude](https://github.com/AnastasiyaW/mclaude) | Atomic locks + handoffs + messaging + memory graph | CLI + library + MCP surface; 166 tests; file-based | Active |
| [madebyaris/agent-orchestration](https://github.com/madebyaris/agent-orchestration) | `lock_acquire` / `lock_release` / `task_create` / `task_claim` | Bundles shared memory + AGENTS.md scaffolding; cross-IDE | v0.5.2; v0.6.0 roadmap |
| [Dicklesworthstone/mcp_agent_mail](https://github.com/Dicklesworthstone/mcp_agent_mail) | Async mailbox + advisory file leases | Mailbox semantics, not blocking rendezvous | Active (798 commits) |
| [gilbarbara/agent-hub-mcp](https://github.com/gilbarbara/agent-hub-mcp) | `register_agent` / `send_message` / `sync` | No explicit lock primitives | v0.4.0 (Sept 2025) |
| [rinadelph/Agent-MCP](https://github.com/rinadelph/Agent-MCP) | Task assignment + RAG + messaging | File locks internal, not exposed as tools | Active |

## Frameworks (in-process orchestration)

These implement fan-out/fan-in within a single agent runtime — different category from mcp-chain. They don't coordinate across separate Claude Code sessions.

- [lastmile-ai/mcp-agent](https://github.com/lastmile-ai/mcp-agent) — orchestrator-workers, parallelization
- [evalstate/fast-agent](https://github.com/evalstate/fast-agent) — agent framework with parallel patterns

## Community patterns

The mainstream "multi-Claude" pattern is **git worktrees + tmux**, with the human as orchestrator:

- [Dan Does Code: Parallel vibe coding with git worktrees](https://www.dandoescode.com/blog/parallel-vibe-coding-with-git-worktrees) (Feb 2026)
- [MindStudio: Claude Code git worktrees parallel branches](https://www.mindstudio.ai/blog/claude-code-git-worktrees-parallel-branches)
- [DEV.to: Running multiple Claude Code sessions with git worktree](https://dev.to/datadeer/part-2-running-multiple-claude-code-sessions-in-parallel-with-git-worktree-165i)
- [Adam Wulf — IttyBitty AI Agent Orchestrator](https://adamwulf.me/2026/01/itty-bitty-ai-agent-orchestrator/) (tmux panes spawn more Claudes)
- [Hwee-Boon Yar — tmux with Claude Code](https://hboon.com/using-tmux-with-claude-code/)

Orchestration wrappers — most layer on subagents within one session, sidestepping cross-session sync rather than solving it:

- [wshobson/agents](https://github.com/wshobson/agents) — 184 agents, 16 orchestrators
- [barkain/claude-code-workflow-orchestration](https://github.com/barkain/claude-code-workflow-orchestration) — plan-mode-integrated DAG
- [ruvnet/ruflo](https://github.com/ruvnet/ruflo)
- [maslennikov-ig/claude-code-orchestrator-kit](https://github.com/maslennikov-ig/claude-code-orchestrator-kit)
- [nyldn/claude-octopus](https://github.com/nyldn/claude-octopus) — multi-model consensus
- [Shipyard: Multi-agent orchestration roundup](https://shipyard.build/blog/claude-code-multi-agent/)

Anthropic's own published pattern ([engineering blog, June 2025](https://www.anthropic.com/engineering/multi-agent-research-system)) is hierarchical orchestrator-with-subagents, results returned through the lead. No cross-context locks.

## When to pick what

| If you need… | Use |
|---|---|
| One parent delegating an isolated task | Native subagents (Task tool) |
| Pre-scriptable pipeline of one-shot tasks | `claude -p` chained in shell |
| Multiple teammates with shared task list under one coordinator | Agent Teams (experimental) |
| Background work that wakes the parent | Async background subagents |
| Named-resource locks across sessions (e.g. `ci`, `staging-db`) | claude-presence |
| Atomic locks + structured handoffs + memory graph | mclaude |
| Lock + task queue + shared memory bundled | agent-orchestration |
| Async mailbox between agents | mcp_agent_mail |
| Lightweight ephemeral rendezvous between N sessions on an arbitrary condition | mcp-chain |

## Honest competitive read

What this research changed about our priors:

- **Niche is not empty.** claude-presence and mclaude occupy the same lane. Earlier internal positioning that called the niche "essentially empty" is wrong as of 2026-04-28.
- **Agent Teams + async background subagents are eroding the easy cases** on the first-party side.

Still defensible:

- **Ephemeral word-IDs** — different mental model from claude-presence's named resources. You rendezvous on a thing you just created, not a resource you both already know about.
- **Go single binary** — claude-presence is Python/SQLite, mclaude is file-based with a CLI surface. mcp-chain's resource budget (≤15 MB binary, ≤20 MB resident, ≤100 ms startup) is unmatched.
- **N-to-N truly arbitrary topology** — Agent Teams caps at one team-lead. claude-presence is closer in topology but tied to named resources.

No longer differentiating:

- "Lightweight cross-session sync primitive" alone — that's table stakes now.

## Methodology

Four parallel research subagents on **2026-04-28** investigated:

1. Claude Code native chaining mechanisms (subagents, headless, fork, hooks)
2. Coordination MCP server landscape (locks, queues, rendezvous)
3. Hooks system as a chaining primitive
4. Multi-session orchestration community patterns

Sources are cited inline. Re-run when the ecosystem shifts.
