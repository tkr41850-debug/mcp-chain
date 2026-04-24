# Roadmap: mcp-chain

## Overview

mcp-chain ships as a single static Go binary (≤15 MB, ≤100 ms startup, ≤20 MB RSS) distributed as a Claude Code plugin that gives N parallel Claude Code sessions a register/wait/resolve primitive over a flock-protected JSON state file. The roadmap is strictly bottom-up: module scaffold + enforcement gates first (so correctness rails exist from day one), then the SDK-agnostic core (wordlist, path resolution, store + flock + atomic write + OwnerToken session-link), then the two thin adapters (MCP server, CLI subcommands), then distribution (plugin packaging, bash monitor), then CI release automation and test-coverage gates, and finally docs plus dogfooding polish. The hexagonal split isolates the MCP SDK choice to one phase (~200 lines), front-loads the highest-risk layer (the store), and lets every downstream phase benefit from early lint, size, and startup gates rather than retrofit them.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation & Enforcement Gates** - Module scaffold, stdout discipline, lint/size/startup CI gates
- [x] **Phase 2: Wordlist & ID Allocator** - EFF wordlist embed + deterministic ID allocator with hex fallback
- [x] **Phase 3: XDG Path Resolution** - State file path resolution with XDG compliance and directory permissions (completed 2026-04-23)
- [x] **Phase 4: Store Core, Flock & Atomic Writes** - Flock-protected JSON store with atomic RMW, schema versioning, and OwnerToken session-link (completed 2026-04-23)
- [x] **Phase 5: MCP Server Adapter** - MCP stdio server with register/resolve tools and terse descriptions (completed 2026-04-24)
- [ ] **Phase 6: CLI Dispatch & Status Subcommand** - kong-based argv dispatch with the status subcommand (shared-lock reads, exit codes 0/1/2)
- [ ] **Phase 7: CLI Formatters (list, purge, resolve)** - Table formatter and administrative subcommands over the shared store
- [ ] **Phase 8: Plugin Packaging & Bash Monitor** - Claude Code plugin manifest, slash commands, and chain-wait.sh poll helper
- [ ] **Phase 9: CI Release, Cross-compile & Test Gates** - GoReleaser cross-compile matrix, race gate, comprehensive unit + integration suites
- [ ] **Phase 10: Docs & Dogfooding Polish** - README (install, usage, why) and dogfooding-driven polish

## Phase Details

### Phase 1: Foundation & Enforcement Gates
**Goal**: Module scaffold and the correctness rails (stdout discipline, lint, binary-size ceiling, startup-time budget) are in place before any production code is written, so downstream phases never retrofit them.
**Depends on**: Nothing (first phase)
**Requirements**: CORE-01 (skeleton only — kong `--version` + subcommand stubs), MCP-02, DIST-03, QA-04
**Success Criteria** (what must be TRUE):
  1. `go build ./...` produces a `mcp-chain` binary that supports `--version` and stubs for `serve`/`status`/`list`/`purge` via `alecthomas/kong`
  2. `mcp-chain serve </dev/null` writes exactly zero bytes to stdout (stderr-only logging hard-set in `main()`, `fmt.Print*` forbidden in serve path via a `forbidigo`/lint rule)
  3. CI fails if the stripped binary exceeds 15 MB or if `time mcp-chain --version` exceeds 100 ms on the cold-cache smoke runner
  4. `go vet` + `staticcheck` (or equivalent) run in CI and block on non-zero exit
**Plans**: 3 plans
  - [ ] 01-01-PLAN.md — Module scaffold + kong wiring + stdout discipline (CORE-01 skeleton, MCP-02)
  - [ ] 01-02-PLAN.md — Lint config + gate scripts + Makefile (QA-04, DIST-03)
  - [ ] 01-03-PLAN.md — CI workflow + in-repo smoke tests (DIST-03, QA-04)

### Phase 2: Wordlist & ID Allocator
**Goal**: A pure, deterministic `idgen.Allocate(counter uint64) string` is available over the embedded EFF short wordlist with a clean hex fallback past 1296 — fully tested in isolation, zero dependency on store or filesystem.
**Depends on**: Phase 1
**Requirements**: CORE-07
**Success Criteria** (what must be TRUE):
  1. The EFF short wordlist (1296 unique lowercase words) is embedded via `//go:embed` with a startup-time test asserting count and uniqueness
  2. `idgen.Allocate(0..1295)` returns `words[i]` deterministically; `Allocate(1296+)` returns a deterministic hex-suffix ID (`hex-0001`, `hex-0002`, …)
  3. A table-driven unit test covers boundary indices (0, 1, 1295, 1296, large values) with no filesystem or concurrency dependency
**Plans**: 1 plan
  - [ ] 02-01-PLAN.md — EFF wordlist embed + pure Allocate(counter) + boundary tests (CORE-07)

### Phase 3: XDG Path Resolution
**Goal**: The state file path is resolved per spec and its parent directory is guaranteed to exist with correct permissions, so the store layer can assume a ready writable location.
**Depends on**: Phase 1
**Requirements**: CORE-06
**Success Criteria** (what must be TRUE):
  1. When `$XDG_STATE_HOME` is set, resolved path is `$XDG_STATE_HOME/mcp-chain/state.json`; otherwise `~/.mcp-chain/state.json`
  2. Parent directory is created with mode `0700` if missing; no NSS/`os/user.Current` call runs on the startup path (uses `$HOME` env directly)
  3. Unit tests cover both XDG and HOME branches via env-var manipulation, with `t.TempDir()` isolation
**Plans**: 1 plan
  - [x] 03-01-PLAN.md — statepath.Resolve() with XDG + HOME fallback, 0700 MkdirAll, 6 env-isolated tests (CORE-06)

### Phase 4: Store Core, Flock & Atomic Writes
**Goal**: The SDK-agnostic hexagonal core — `Register`, `Resolve`, `Get`, `List`, `Purge` over a flock-protected versioned JSON file — is complete, correct under concurrent cross-process load, and exposes the OwnerToken session-link design so both adapters can enforce ownership later.
**Depends on**: Phase 2, Phase 3
**Requirements**: CORE-04, CORE-05, CORE-08, CORE-09 (state schema, OwnerToken field, session-link design land here per architecture resolution)
**Success Criteria** (what must be TRUE):
  1. A single `withLockedState(fn)` API performs read → modify → atomic-rename write under `LOCK_EX` using `gofrs/flock` + `google/renameio/v2`; `Get`/`List` acquire `LOCK_SH` so concurrent readers never serialize
  2. State file is JSON with top-level `version: 1` + monotonic `counter` (never decremented on purge); file mode `0600`, parent dir `0700`; unknown `version` returns an actionable error
  3. Records carry an `owner_token` field populated on `Register`; `Resolve` returns distinct sentinel errors `ErrUnknownID`, `ErrAlreadyResolved`, `ErrNotOwner`; CLI `--force` path can bypass the OwnerToken check
  4. Integration test spawns two processes each registering 100 entries concurrently and asserts 200 unique word-IDs with no lost updates and no corrupt JSON after a kill-9 mid-write
  5. Windows build uses a separate lock file (`state.json.lock`) and `MoveFileEx`-backed atomic replace via `renameio` so reader-holds-file does not break writer rename
**Plans**: TBD

### Phase 5: MCP Server Adapter
**Goal**: A thin MCP stdio adapter exposes `register` and `resolve` tools over the store with terse descriptions, a per-process OwnerToken, and zero MCP types leaking into the core — the only phase tied to the MCP SDK choice.
**Depends on**: Phase 4
**Requirements**: CORE-02, CORE-03, CORE-10, MCP-01, MCP-03
**Success Criteria** (what must be TRUE):
  1. `mcp-chain serve` runs `modelcontextprotocol/go-sdk` v1.5+ over `StdioTransport`, completes the MCP initialize handshake, and exposes `register(condition) → id` and `resolve(id)` tools; no `net/http` import in the resulting binary (CI grep gate)
  2. On `serve` startup a 128-bit `crypto/rand` `OwnerToken` is generated and stamped onto every `register` record; `resolve` returns `ErrNotOwner` on mismatch with a distinct wire-level error code separate from `ErrUnknownID` and `ErrAlreadyResolved`
  3. Every tool description is ≤ 1 short sentence / ≤ 40 tokens, total tool-list ≤ 200 tokens (CI token-budget probe); no MCP types appear in any `internal/store` identifier
  4. Integration test pipes a recorded `initialize` + `register` + `resolve` sequence and asserts stdout is exclusively valid compact JSON-RPC with no embedded newlines or banners
**Plans**: TBD

### Phase 6: CLI Dispatch & Status Subcommand
**Goal**: The full argv dispatch is wired via `kong` and `mcp-chain status <id>` returns the scriptable exit codes (0 resolved, 2 pending, 1 unknown) the bash monitor depends on, reading under `LOCK_SH` so N concurrent waiters do not serialize.
**Depends on**: Phase 4
**Requirements**: CORE-01
**Success Criteria** (what must be TRUE):
  1. `mcp-chain status <id>` exits 0 on resolved, 2 on pending, 1 on unknown ID — asserted by a table-driven integration test invoking the compiled binary
  2. Status reads use `LOCK_SH`, so 10 concurrent `status` processes against the same state file complete in under one second wall-time with no serialization (timing test)
  3. Unknown subcommands, bad arguments, and `--help` write to stderr (not stdout); `status` writes nothing to stdout beyond its single-line result
**Plans**: TBD

### Phase 7: CLI Formatters (list, purge, resolve)
**Goal**: Administrative subcommands `list`, `purge`, and CLI-only `resolve --force` provide human-readable output and safe cleanup semantics over the shared store, with formatters isolated so they do not leak into core.
**Depends on**: Phase 4, Phase 6
**Requirements**: CMD-03, CMD-04 (CLI semantics; slash-command prompt wiring lands in Phase 8)
**Success Criteria** (what must be TRUE):
  1. `mcp-chain list` prints an aligned human-readable table (ID, status, condition, created_at, resolved_at) over a shared-lock read
  2. `mcp-chain purge` requires at least one of `<id>` / `--all` / `--resolved`; bare `mcp-chain purge` exits non-zero with an error on stderr; counter is never decremented
  3. `mcp-chain resolve <id> [--force]` mirrors the MCP resolve tool for scripting; `--force` bypasses the OwnerToken check as the documented operator-driven recovery escape hatch
**Plans**: TBD

### Phase 8: Plugin Packaging & Bash Monitor
**Goal**: The compiled binary is installable with zero additional config as a Claude Code plugin via `/plugin install`, with four token-budgeted slash commands and a POSIX-bash monitor script that calls back into `mcp-chain status` on a 1-second poll.
**Depends on**: Phase 5, Phase 6, Phase 7
**Requirements**: CMD-01, CMD-02, CMD-03 (slash-command prompt), CMD-04 (slash-command prompt), CMD-05, HELPER-01, HELPER-02, DIST-01
**Success Criteria** (what must be TRUE):
  1. `plugin/.claude-plugin/plugin.json`, `plugin/.mcp.json` (using `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve`, no absolute paths, no npm/uvx), and four `commands/*.md` slash-command files ship in the repo and install cleanly via `/plugin install` on a fresh machine with no Go/Node/Python
  2. Every slash-command prompt body is ≤ 30 words and tells Claude exactly which MCP tool to call with which arguments — no branching prose, no examples, no boilerplate
  3. `scripts/chain-wait.sh` wraps `mcp-chain status <id>` in a 1-second poll loop, echoes `--timeout DURATION` (accepts `30s`/`1m`/`1h`/`24h`/`168h`) on invocation, prints `continue` on resolve, errors to stderr on unknown/purged ID mid-wait, and runs under macOS's default bash 3.2 with no bashisms
  4. `/chain-reg [condition]` registers (prompting for condition if omitted), `/chain-wait [id] [--timeout D]` runs the monitor, `/chain-list` prints the table, and `/chain-purge [id | --all | --resolved]` refuses bare invocation — end-to-end demo flow verified on Linux and macOS
**Plans**: TBD
**UI hint**: yes

### Phase 9: CI Release, Cross-compile & Test Gates
**Goal**: Release tooling is production-grade from day one — tagged releases ship six arch combos with checksums and reproducible builds, and `go test -race` plus the comprehensive unit and integration suites gate every push/PR.
**Depends on**: Phase 4, Phase 5, Phase 6, Phase 7
**Requirements**: DIST-02, QA-01, QA-02, QA-03
**Success Criteria** (what must be TRUE):
  1. Push/PR CI runs `go test -race -count=1 ./...` on Linux + macOS runners and the non-race suite on Windows; any failure blocks merge
  2. Tagging `v*` triggers GoReleaser to cross-compile `linux/darwin/windows × amd64/arm64`, strip with `-s -w -trimpath`, attach all six archives plus `checksums.txt` to the GitHub release, and embed the tag in `--version` (not a dirty SHA)
  3. Unit suite covers wordlist allocation determinism, counter monotonicity, hex fallback, state schema round-trip, timeout parsing, path resolution, ID lookup, and every state transition (pending → resolved, double-resolve, unknown, OwnerToken mismatch)
  4. Integration suite covers end-to-end register → status pending → resolve → status resolved, N concurrent waiters, double-resolve + unknown-ID + purge-mid-wait + OwnerToken-mismatch errors, and two-process cross-flock safety with 100 entries per process with zero lost updates
**Plans**: TBD

### Phase 10: Docs & Dogfooding Polish
**Goal**: A brief, accurate `README.md` lets a new user install and use mcp-chain from either the plugin or manual paths, and dogfooding the end-to-end flow has surfaced and fixed any small polish items.
**Depends on**: Phase 8, Phase 9
**Requirements**: DIST-04
**Success Criteria** (what must be TRUE):
  1. `README.md` covers why the tool exists, plugin install + manual install steps, usage examples for `/chain-reg`, `/chain-wait`, `/chain-list`, `/chain-purge`, and manual CLI usage of `status`/`list`/`purge`/`resolve`
  2. State-file path and the NFS / networked-filesystem caveat are documented; the upgrade/reload step for Claude Code (`/mcp` list → restart) appears in the install section
  3. A dogfooding pass (register in one session, wait from another, resolve, verify `continue`) runs successfully end-to-end on Linux and macOS before the first tagged release
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10

**Parallelization opportunities** (yolo mode, auto-advance active):
- Phase 2 and Phase 3 can execute in parallel after Phase 1 (both depend only on Phase 1 and are fully independent)
- Phase 5 and Phase 6 can execute in parallel after Phase 4 (MCP adapter and CLI dispatch both sit on the store but do not touch each other)
- Phase 9 (CI + test gate) overlaps with Phase 8 (packaging) after Phase 7 — test authoring is continuous across all phases; Phase 9 is the gate-enforcement phase

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation & Enforcement Gates | 0/3 | Planned | - |
| 2. Wordlist & ID Allocator | 0/1 | Planned | - |
| 3. XDG Path Resolution | 1/1 | Complete   | 2026-04-23 |
| 4. Store Core, Flock & Atomic Writes | 0/TBD | Not started | - |
| 5. MCP Server Adapter | 0/TBD | Not started | - |
| 6. CLI Dispatch & Status Subcommand | 0/TBD | Not started | - |
| 7. CLI Formatters (list, purge, resolve) | 0/TBD | Not started | - |
| 8. Plugin Packaging & Bash Monitor | 0/TBD | Not started | - |
| 9. CI Release, Cross-compile & Test Gates | 0/TBD | Not started | - |
| 10. Docs & Dogfooding Polish | 0/TBD | Not started | - |
