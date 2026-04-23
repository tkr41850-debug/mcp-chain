# Feature Research

**Domain:** Local session-coordination primitive (register → wait → resolve) for Claude Code multi-session workflows
**Researched:** 2026-04-23
**Confidence:** HIGH (table stakes derived from well-documented adjacent tools; differentiators and anti-features cross-referenced against PROJECT.md scope)

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist in any register/wait/resolve primitive. Missing these = product feels incomplete or broken.

| Feature | Why Expected | Complexity | Notes / PROJECT.md Coverage |
|---------|--------------|------------|----------------------------|
| Register a coordination entity and get back an identifier | Core of the register step; every lock/lease/job system returns a handle (etcd lease ID, Consul session ID, flock fd, job ID) | LOW | Covered: CORE-02 returns word-ID |
| Wait for resolution with blocking semantics | Central promise; flock blocks, named pipes block, `wait-for-it` blocks, `needs:` gates. Users expect "when I wait, my shell doesn't return until the thing is done" | LOW | Covered: CMD-02 via bash monitor + CORE-01 `status` |
| Resolve / signal / release | Every coordination tool has a release step (flock -u, consul kv delete, lease revoke, job complete). Asymmetry with register would feel broken | LOW | Covered: CORE-03 |
| List / enumerate outstanding entities | Universal: `fuser`, `flock` via `/proc/locks`, `consul kv list`, `etcdctl lease list`, `task list`. Users need to see what's pending without opening state files | LOW | Covered: CMD-03 |
| Status query for a single ID | `etcdctl lease timetolive`, `consul kv get`, job-status endpoints. Required for poll loops and scripting | LOW | Covered: CORE-01 `status <id>` |
| Clear error for unknown/missing IDs | `flock: /path: No such file` — typos and stale refs must fail fast with readable messages, not hang or silently succeed | LOW | Covered: CMD-02 "errors immediately if the ID doesn't exist" |
| Cleanup / removal / purge | Every tool has it: `rm lockfile`, `consul kv delete`, `task done`, queue-purge. Accumulation is the #1 complaint about coordination tools (PlanetScale blog: "database cannot clean up faster than new work accumulates") | LOW | Covered: CMD-04 (manual, not automatic — see anti-features) |
| Safe concurrent access to state | flock, fcntl, row-level locks, lease transactions. Race-free state updates are the one thing coordination tools cannot get wrong | MEDIUM | Covered: CORE-04 (flock on JSON) |
| Cross-platform path conventions | XDG on Linux, `~/Library` on macOS. Users expect the tool to live where other well-behaved CLI tools live | LOW | Covered: CORE-05 (XDG_STATE_HOME) |
| Human-readable state / debuggability | Every file-based tool exposes state a human can inspect (lockfiles, Procfile, Makefile). "I want to cat the file" is a universal debugging expectation | LOW | Covered implicitly by CORE-04 (JSON) |
| Exit codes meaningful for scripting | `wait-for-it` exit codes, `flock` exit codes, `test` exit codes. Shell users expect 0/non-0 discrimination | LOW | Covered: CORE-01 (0/1/2) |
| Timestamps on entries | `ls -l` on lockfiles, `etcd` keys carry revision+mod_revision, job creation/updated_at. "When was this registered?" is the first debugging question | LOW | Covered implicitly by CMD-03 "timestamps" |
| Timeouts on waits | `wait-for-it -t`, `flock -w`, `consul lock -timeout`, `curl --max-time`. Unbounded waits are user-hostile in scripts | LOW | Covered: CMD-02 `--timeout DURATION` |
| Double-resolve / double-release error | flock on already-unlocked fd errors; consul session destroy twice errors. Idempotency is nice but an explicit error is clearer than silent no-op for this class of tool | LOW | Covered: CORE-03 "already resolved" |

### Differentiators (Competitive Advantage)

Features that set mcp-chain apart from generic coordination primitives. These lean into its niche: Claude-driven, MCP-native, session-aware.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Natural-language resolution conditions | flock/etcd/Consul all require programmatic conditions (a file to delete, a lease to expire, a shell exit code). mcp-chain lets Claude register "resolve when integration tests pass and the PR is merged" as a human-readable note the resolving session judges against its context. No other coordination primitive does this | LOW (it's just metadata) | Covered: CORE-02. This is the single sharpest differentiator — it's why you'd pick mcp-chain over `flock` + a file |
| Short memorable word-IDs (EFF wordlist) | UUIDs, hex hashes, and numeric job IDs dominate the space. A word-ID ("tangerine-squirrel") is copy-pasteable between terminals, speakable, memorable for minutes. Better UX for a human-in-the-loop tool than `5f3e2a8c-...` | LOW | Covered: CORE-06. Cross-ref: just/task use numeric, consul uses UUIDs — this is genuinely novel for a coordination tool |
| Zero-install for the target audience | Ships as a Claude Code plugin with slash commands. No `apt install`, no `brew install`, no venv. Adjacent tools (overmind, consul, pueue) all require separate install steps and config | LOW (distribution mechanics) | Covered: DIST-01 |
| MCP-native tool descriptions optimized for token budget | LLM-consumable tools are a new category — most MCP servers have verbose tool descriptions. Token-minimal MCP prompts are a real differentiator when Claude is the primary consumer | LOW | Covered: CORE-07 |
| Single-binary, sub-100ms startup | Consul agent (~50MB, multi-second start), etcd (~50MB, slow), pueued (daemon). Even overmind requires tmux and a config file. mcp-chain: ~10MB Go binary, <50ms startup | MEDIUM (Go constraint discipline) | Covered: Constraints section |
| `/chain-list` as human table, not JSON | Most coordination tools (consul kv list, etcdctl) dump raw. Tools that ship a readable list (`docker ps`, `kubectl get`, `task list`) feel more polished. Trivial to build, big UX win | LOW | Covered: CMD-03 "human-readable table" |
| Composable in arbitrary fan-in / fan-out shapes | The core pattern (1 registrant, N waiters) composes with itself: a session can register one ID and wait on another. This naturally handles handoff chains (A→B→C) and barriers. Unlike GitHub Actions `needs:` (DAG is statically declared), mcp-chain builds ad-hoc at runtime | LOW (emergent from CORE-02/03) | Covered implicitly. See "Concurrency Patterns" below |
| Session-link identity (if feasible) | If MCP per-connection identity is reliable, enforcing "only the registering session can resolve" prevents a whole class of mistakes. No other local coordination tool has this (process PID ≠ session identity). Flagged in PROJECT.md as open research question | MEDIUM | Partial: PROJECT.md line 91 flags this as TBD. If reliable → differentiator; if not → fall back to "any session resolves once" |

### Anti-Features (Commonly Requested, Often Problematic)

Features adjacent tools have that mcp-chain should deliberately NOT build. Each one pushes mcp-chain out of its niche (local, single-user, Claude-driven, terse).

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Automatic TTL / lease expiration | etcd, Consul, and Redis locks auto-expire. "What if a session crashes before resolving?" is a natural question. Users will ask | Implies time-based state changes → requires background timer/daemon → requires deciding "how long is too long?" → every user disagrees. PROJECT.md explicitly rejects this (line 54). Keep-until-purge is simpler, predictable, and appropriate for human-in-the-loop | Manual `/chain-purge`. If a session crashes, the ID sits around as a visible reminder; user purges when they notice. This is a feature, not a bug, for a single-user tool |
| Heartbeat / liveness detection | Consul sessions auto-invalidate when agent fails health checks; some lockfile tools check `/proc/PID` | Requires a background process or polling loop. Adds failure modes (network partitions, clock skew, false-positive stale detection). Overkill for local single-user — "crashes" are rare and visible | If a registrant crashes, user sees the dangling ID in `/chain-list` and purges it. Visible > automatic |
| Cron-style scheduling / recurring resolution | Taskwarrior has recurring tasks; some job queues support scheduled jobs | Turns a coordination primitive into a scheduler. Out of scope — use `cron` or `systemd timers` for scheduling | Users who want recurring coordination compose mcp-chain with cron themselves |
| DAG / declarative workflow execution | Airflow, Argo, GitHub Actions `needs:`, `make`. "Can I define A→B→C once and have it run?" is natural | mcp-chain is a *primitive*, not a workflow engine. DAG execution requires: dependency parsing, topological sort, retry policies, failure handling, artifact passing. Each of these is a whole product | Composition at the Claude session layer: sessions compose fan-in/fan-out themselves by calling register/wait. If a user needs declarative DAGs, they want Airflow, not mcp-chain |
| Artifact / data passing between sessions | GitHub Actions artifacts, Airflow XCom, job queue payloads. "I resolved — here's the result, pass it to waiters" | Turns a signal into a message bus. Requires serialization, size limits, retention policy, schema decisions. Waiters can read the session output directly (they're Claude sessions with tool access) — they don't need mcp-chain to carry the payload | Resolution is boolean — "unblocked / not unblocked". Payloads travel out-of-band (files, PR comments, stdout) |
| Distributed / multi-machine coordination | Consul, etcd exist for this. "What if sessions are on different machines?" | PROJECT.md rejects this (line 55). Adds networking, auth, consensus, clock sync — all of which explode complexity | Users needing multi-machine coordination should use Consul or etcd (which exist and work) |
| Programmatic / shell-exit conditions | `wait-for-it` polls a TCP port; `flock` watches a file; `make` checks mtime | PROJECT.md rejects this (line 58). Ties server to shell semantics; Claude-judged natural-language conditions are more flexible and match the actual use case (Claude is the resolver) | Natural-language condition (CORE-02) is the intentional replacement |
| Embedded database (SQLite / bbolt) | "What if state grows?" is a natural worry | PROJECT.md rejects this (line 57). JSON file is fine for expected entry counts (tens to low-hundreds active IDs); flock handles concurrency. SQLite adds a dep, a schema, and a migration story for zero benefit at this scale | JSON + flock. Revisit if real-world usage ever hits 10k+ active IDs |
| Daemon / long-running socket server | Pueue has a daemon; Consul has an agent; overmind has a master process | PROJECT.md rejects this (line 56). Daemons need lifecycle management (start/stop/restart, crash recovery, PID files, logs). Stateless binary + shared file is simpler and has no "is the daemon running?" debugging step | Short-lived `serve` invocations spawned by MCP stdio; state lives in the file |
| Auth / ACLs beyond session-link | Consul has ACLs; etcd has RBAC; job queues have per-queue permissions | PROJECT.md rejects this (line 61). Single-user local tool — the OS user permission on the state file is sufficient. Anyone with read on `~/.mcp-chain/state.json` is already the same user | File permissions (0600 on the state file). No in-tool ACL layer |
| Cancellation of waiters from registrant side | "Registrant wants to abort — unblock all waiters with error" | Not clearly needed: registrant can just `/chain-purge <id>`, waiters' next poll sees unknown ID, they error out (CMD-02 "errors immediately if the ID doesn't exist"). Works, but it's implicit. Could add an explicit `/chain-cancel` later if user feedback demands it | `/chain-purge` + waiter's unknown-ID error path. Monitor current behavior. v2 candidate if ambiguous |
| Export / import / backup | Taskwarrior sync, consul snapshot | State file is JSON — `cp ~/.mcp-chain/state.json backup.json` is the export. No need for a dedicated feature at this scope | `cp` the JSON file. If sync across machines ever matters, revisit (but PROJECT.md says local-only) |
| Tags / categories / metadata on IDs beyond condition | Job queues commonly support tags/labels; Taskwarrior has projects and tags | Adds query complexity (`/chain-list --tag=foo`) for a tool with <100 active entries. The natural-language condition IS the metadata; project/tag layers are redundant at this scale | Condition text is searchable by eye in `/chain-list`. If users consistently want filters, add later |
| Priority / ordering between waiters | Some lock systems have queues with priority | Waiters are independent — all unblock simultaneously on resolve. There's no "ordering" to prioritize. This is a category error for the primitive | N/A — waiters are peers |
| Retry / resurrection of purged IDs | "I purged it by accident, can I un-purge?" | Adds a tombstone/trash state, undo semantics, garbage collection. Use word-IDs — regenerate and re-register | Re-register. Word-IDs are cheap |
| Web UI / dashboard | Pueue has a TUI; Consul has a web UI; Airflow has a big dashboard | Out of scope for a terse MCP tool. `/chain-list` (CMD-03) is the UI. Target user runs Claude Code in a terminal; a web dashboard is friction, not value | `/chain-list` is the entire UI surface |

## Feature Dependencies

```
register (CORE-02)
    ├──required-by──> wait (CMD-02)
    ├──required-by──> resolve (CORE-03)
    ├──required-by──> status (CORE-01)
    ├──required-by──> list (CMD-03)
    └──required-by──> purge (CMD-04)

state persistence (CORE-04, CORE-05)
    └──required-by──> ALL operations

flock safety (CORE-04)
    └──required-by──> concurrent register/resolve/purge from multiple sessions

word-ID generator (CORE-06)
    └──required-by──> register

wait (CMD-02) ──requires──> bash monitor (HELPER-01) ──requires──> status (CORE-01)

session-link identity (TBD)
    ├──enhances──> resolve (restricts to registering session)
    └──CONFLICTS-WITH──> daemon-less model IF MCP stdio identity not reliable across invocations
```

### Dependency Notes

- **All operations depend on state persistence (CORE-04/05):** This is the one piece that must be rock-solid before anything else works. Flock correctness is the highest-risk single thing to get right.
- **Wait depends on bash monitor depends on status:** Three-layer dependency means breaking `status` breaks the whole wait flow. Status subcommand must be ruthlessly simple (read file, print state, exit code).
- **Session-link identity may conflict with stateless model:** If MCP stdio doesn't give stable per-session identity across invocations, enforcing "only registrant resolves" would require persistent session-auth state, which is new surface area. Open research question per PROJECT.md line 91.
- **Purge depends on register existing:** Obvious but worth noting for phase ordering — purge is always *last* in the lifecycle and can ship after register/wait/resolve are validated.

## MVP Definition

### Launch With (v1) — maps directly to PROJECT.md Active requirements

Minimum viable product — what's needed to validate the Claude-session coordination concept.

- [ ] **register with natural-language condition** (CORE-02) — core differentiator; without it, it's just `flock`
- [ ] **word-ID generator** (CORE-06) — the memorable-ID UX is what makes this Claude-friendly
- [ ] **resolve** (CORE-03) — symmetry with register
- [ ] **status subcommand** (CORE-01) — the primitive the bash monitor depends on
- [ ] **state file + flock** (CORE-04, CORE-05) — correctness foundation
- [ ] **`/chain-reg`, `/chain-wait`, `/chain-list`, `/chain-purge`** (CMD-01..04) — the user-facing surface
- [ ] **bash monitor helper** (HELPER-01) — makes wait actually block
- [ ] **unknown-ID error + double-resolve error** (CMD-02, CORE-03) — table-stakes error behavior
- [ ] **wait timeout** (CMD-02 `--timeout`) — table-stakes ergonomics
- [ ] **terse tool descriptions** (CORE-07) — product principle, needs to be right on day one or it's a breaking change
- [ ] **plugin distribution + CI release artifacts** (DIST-01..03) — zero-install is a differentiator; must ship with it

### Add After Validation (v1.x)

Features to add once core is working and user (the author) has flow hours with it.

- [ ] **Session-link resolve enforcement** — ship with "any session resolves once" initially, add session-pinning if MCP stdio identity research resolves favorably and the looser behavior has caused actual mistakes
- [ ] **Explicit `/chain-cancel`** — if users keep typing `/chain-purge` to abort a registrant and waiters get confused by the "unknown ID" error message, add a dedicated cancel with clearer semantics
- [ ] **`/chain-list` filters** (e.g., `--pending`, `--resolved`) — only if unfiltered list gets too long in practice
- [ ] **Resolution note / reason field** — optional message attached to resolve ("tests passed on PR #123") visible in `/chain-list`. Low complexity, medium value for audit trail

### Future Consideration (v2+)

Features to defer until product-market fit is established (i.e., the author has used this for 6+ months and has identified pain points from real use).

- [ ] **Condition editing after register** — if users discover they want to tweak conditions without re-registering
- [ ] **Tags / labels** — only if active entry count regularly exceeds ~30 and `/chain-list` becomes noisy
- [ ] **History / audit log of resolved and purged IDs** — currently purged = gone. A `~/.mcp-chain/history.jsonl` append-only log could enable "what did I coordinate last week?" retrospection
- [ ] **Resolution webhooks / pingbacks** — "when X resolves, POST to Y". Out of scope for local-only, but if Claude Code plugin ecosystem evolves, could integrate with other plugins

### Explicitly Out (not future — not ever, unless scope fundamentally changes)

Per PROJECT.md Out of Scope + anti-features table above: auto-expiration, networked/multi-machine, daemon, SQLite, programmatic conditions, granular auth, DAG execution, artifact passing, cron scheduling, web UI.

## Feature Prioritization Matrix

Maps PROJECT.md requirements to user-value × implementation-cost.

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| register with condition (CORE-02) | HIGH | LOW | P1 |
| resolve (CORE-03) | HIGH | LOW | P1 |
| status subcommand (CORE-01) | HIGH | LOW | P1 |
| state file + flock (CORE-04) | HIGH | MEDIUM | P1 |
| word-ID generator (CORE-06) | HIGH | LOW | P1 |
| `/chain-wait` with bash monitor (CMD-02, HELPER-01) | HIGH | LOW | P1 |
| `/chain-reg` slash command (CMD-01) | HIGH | LOW | P1 |
| `/chain-list` (CMD-03) | HIGH | LOW | P1 |
| `/chain-purge` (CMD-04) | MEDIUM | LOW | P1 |
| terse tool descriptions (CORE-07) | MEDIUM | LOW | P1 (product principle) |
| XDG state path (CORE-05) | MEDIUM | LOW | P1 |
| wait `--timeout` (CMD-02) | MEDIUM | LOW | P1 |
| unknown-ID / double-resolve errors | HIGH | LOW | P1 |
| plugin distribution (DIST-01) | HIGH | MEDIUM | P1 |
| CI release binaries (DIST-02) | MEDIUM | MEDIUM | P1 |
| README (DIST-03) | MEDIUM | LOW | P1 |
| unit + integration tests (QA-01..03) | HIGH | MEDIUM | P1 (correctness gate) |
| session-link resolve enforcement | MEDIUM | MEDIUM-HIGH | P2 (gated on MCP identity research) |
| explicit `/chain-cancel` | LOW | LOW | P2 (add if real confusion emerges) |
| resolution note field | LOW | LOW | P2 |
| history / audit log | LOW | MEDIUM | P3 |
| tags / labels | LOW | MEDIUM | P3 |
| condition editing | LOW | LOW | P3 |

**Priority key:**
- P1: Must have for launch (all PROJECT.md Active items)
- P2: Post-launch, add when real use reveals the need
- P3: Deferred; likely never unless scope changes

## Concurrency Patterns — Does the Current Design Support Each?

| Pattern | Description | Supported by Current Design? | Notes |
|---------|-------------|------------------------------|-------|
| **Barrier** (N waiters, 1 signal) | N sessions wait for 1 session's resolve | YES — this is the default shape (PROJECT.md line 5, 68) | One register, many `/chain-wait <id>`, one resolve unblocks all |
| **Fan-in aggregation** (1 waiter needs N signals) | 1 session waits for N independent completions | YES via composition | Session registers N separate IDs, waits on each sequentially, or orchestrates via a single "aggregator" registration that it resolves only when all sub-conditions met. Not a first-class primitive — it's user-composed. This is the right tradeoff (primitive, not framework) |
| **Handoff chain** (A→B→C) | Session A resolves → Session B unblocks and does work → Session B resolves → Session C unblocks | YES — chains naturally | Each link is an independent register/wait pair. Session B does `/chain-wait <A's id>` then `/chain-reg <B's condition>` and publishes its own ID |
| **Fan-out** (1 registrant, N waiters doing parallel work) | 1 session signals N sessions to start work | YES — isomorphic to barrier | Same primitive; N waiters each do their own thing post-resolve |
| **Mutual exclusion** (like `flock -x`) | Only one session holds the "lock" at a time | NO — not the use case | mcp-chain is a signal primitive, not a mutex. Users needing mutex should use `flock` directly |
| **Condition variable** (wait-notify-many with rearm) | Wait, get notified, go back to waiting | NO | Once resolved, stays resolved until purged. Register a new ID for the next cycle. This matches the keep-until-purge lifecycle |
| **Semaphore** (N concurrent holders) | Bounded concurrent access | NO | Out of scope — this is a mutex/semaphore concern, not a signal concern |

**Design note:** The primitive is "one-shot signal" (edge-triggered, not level-triggered in the rearm sense — once resolved, stays resolved). This is sufficient for the stated use case and keeps the primitive irreducibly simple. Users wanting rearm re-register; users wanting mutex use `flock`; users wanting semaphores use a different tool.

## Adjacent-Tool Feature Analysis

Cross-reference with the tools the milestone_context called out.

| Feature | flock / lockfiles | GitHub Actions `needs:` | etcd / Consul | Airflow / DAG tools | just / tmuxinator | mcp-chain |
|---------|-------------------|-------------------------|---------------|---------------------|-------------------|-----------|
| Register/create | creat/open | `jobs:` declaration | `grant` lease | task definition | task def in file | CORE-02 word-ID |
| Wait/block | `flock -w` | `needs:` implicit | `Wait` on lease | scheduler waits | (parallel, not wait) | CMD-02 poll loop |
| Resolve/release | `flock -u` / close fd | job completes | `revoke` / delete | task success/fail | task exit | CORE-03 |
| List outstanding | `/proc/locks`, `fuser` | workflow run UI | `lease list` | UI | `task --list` | CMD-03 |
| Auto-expire (TTL) | NO | job timeout | YES (lease TTL) | DAG timeout | NO | NO (anti-feature) |
| Heartbeat / liveness | NO | runner heartbeat | YES | scheduler heartbeat | NO | NO (anti-feature) |
| Human-readable conditions | NO | NO | NO | NO | NO | YES (differentiator) |
| Memorable IDs | path-based | numeric | UUID | task_id | task name | WORD-ID (differentiator) |
| Cross-process safety | kernel-guaranteed | workflow-scoped | consensus | scheduler-mediated | N/A | flock-guaranteed |
| Zero daemon | YES | N/A (cloud) | NO (agent) | NO (scheduler) | YES | YES |
| Natural-language conditions | N/A | N/A | N/A | N/A | N/A | YES (unique) |
| Multi-machine | NFS (sketchy) | YES (cloud) | YES | YES | NO | NO (out of scope) |
| Payload / artifact passing | NO | YES (artifacts) | key-value | XCom | NO | NO (anti-feature) |

**Pattern recognition:** mcp-chain sits closest to `flock` in mechanism (filesystem-based, no daemon) but closest to job-queue semantics in UX (register handle → wait on it → resolve). The natural-language condition is genuinely a category-new feature enabled by being Claude-native.

## Cross-Reference Against PROJECT.md — Gaps Found

Flagging any feature appearing in adjacent tools as table-stakes but missing from PROJECT.md Active requirements.

| Gap Candidate | Assessment | Recommendation |
|---------------|------------|----------------|
| State file permissions (0600) | NOT in PROJECT.md explicitly | Add as implementation detail — single-user tool, state file should be user-read-only. Probably a 1-liner in CORE-04's implementation |
| Behavior when state file is corrupt / unreadable | NOT specified | Add: on unparseable JSON, fail loudly with path + parse error; do NOT auto-truncate. Users can move the file aside |
| Concurrent register collision on same word-ID | Likely handled by CORE-06's "deterministic allocation" but not spelled out | Add to QA-02 integration tests: two parallel registers never get the same ID |
| Empty state file vs missing state file | NOT specified | Should both be treated as "no entries" and create on demand. Trivial but worth a test |
| `/chain-wait` exit behavior on purge-while-waiting | Ambiguous: does the monitor error out, or hang? Implied by "errors immediately if the ID doesn't exist" but not clear what happens mid-wait | Add: monitor should detect unknown-ID during poll (not just at start) and exit non-zero with clear message. This is the implicit cancel path discussed above |
| `/chain-list` empty-state message | Not specified | Small UX thing: if no entries, print "No active sessions" not an empty table. Low priority |
| Shell completion (bash/zsh/fish) | NOT in PROJECT.md | P2 — nice polish but not table stakes for a single-user tool where most interaction is via slash commands |
| Version subcommand / flag | NOT in PROJECT.md | Add: `mcp-chain --version`. Standard CLI table stakes; trivial to implement |

None of these gaps invalidate the current requirement list — they're mostly edge-case specifications that will likely surface during QA-02 integration testing or early dogfooding.

## Sources

- [flock(2) Linux manual page](https://man7.org/linux/man-pages/man2/flock.2.html) — baseline primitive semantics (HIGH)
- [flock(1) shell manual page](https://man7.org/linux/man-pages/man1/flock.1.html) — CLI flock features, exit codes (HIGH)
- [Open-Technology-Foundation/locks GitHub](https://github.com/Open-Technology-Foundation/locks) — stale lock detection patterns (MEDIUM, community tool)
- [etcd lease / TTL documentation](https://etcd.io/docs/v3.5/learning/why/) — lease + put-if-absent for distributed locks, TTL semantics (HIGH)
- [Consul sessions and distributed locks](https://developer.hashicorp.com/consul/docs/dynamic-app-config/sessions) — session TTL, heartbeat, lock-delay (HIGH)
- [Consul Lock command](https://developer.hashicorp.com/consul/commands/lock) — CLI semantics for wait + release (HIGH)
- [GitHub Actions job dependencies (`needs:`)](https://oneuptime.com/blog/post/2025-12-20-job-dependencies-github-actions/view) — fan-out/fan-in DAG coordination at declarative level (MEDIUM)
- [wait-for-it GitHub](https://github.com/vishnubob/wait-for-it) — minimalist block-until-ready CLI; exit-code-for-scripting pattern (HIGH)
- [eficode/wait-for](https://github.com/eficode/wait-for) — POSIX variant, same pattern (HIGH)
- [Overmind process manager](https://github.com/DarthSim/overmind) — process coordination with tmux; shows daemon-ful alternative (HIGH)
- [Taskwarrior task states](https://taskwarrior.org/docs/task/) — local task lifecycle (pending/completed/deleted/waiting), list reports (HIGH)
- [Taskwarrior list reports](https://taskwarrior.org/docs/commands/list/) — how list UIs look in mature local tools (HIGH)
- [Named pipes (FIFO) Wikipedia](https://en.wikipedia.org/wiki/Named_pipe) — Unix IPC blocking semantics baseline (HIGH)
- [Procrastinate queueing locks](https://procrastinate.readthedocs.io/en/stable/howto/advanced/queueing_locks.html) — queue lock/execution lock patterns (MEDIUM)
- [PlanetScale: Keeping a Postgres queue healthy](https://planetscale.com/blog/keeping-a-postgres-queue-healthy) — queue accumulation as the central operational failure mode (MEDIUM)
- [Bazel dependencies concepts](https://bazel.build/concepts/dependencies) — target DAG formalism (HIGH)
- [Airflow DAGs](https://airflow.apache.org/docs/apache-airflow/stable/core-concepts/dags.html) — full-featured DAG orchestrator (the thing we're NOT building) (HIGH)

---
*Feature research for: local session-coordination primitive for Claude Code*
*Researched: 2026-04-23*
