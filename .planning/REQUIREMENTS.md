# Requirements: mcp-chain

**Defined:** 2026-04-23
**Core Value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with minimal overhead (fast startup, low memory, terse tool prompts, small binary).

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Core MCP & State

- [ ] **CORE-01**: Single Go binary with subcommands dispatched via `alecthomas/kong`: `serve` (MCP stdio server), `status <id>` (CLI poll: exit 0 resolved, 2 pending, 1 unknown), `list` (human table), `purge` (cleanup), `--version` flag
- [ ] **CORE-02**: MCP tool `register(condition string) -> id string` — creates a new chain entry with a natural-language resolution condition and returns a short word-ID
- [ ] **CORE-03**: MCP tool `resolve(id string) -> void` — marks an ID resolved. Errors distinctly on: unknown ID, already-resolved ID, OwnerToken mismatch
- [ ] **CORE-04**: State persisted to a shared JSON file with `gofrs/flock` exclusive lock during read-modify-write. Flock critical section covers the full RMW, never held across MCP tool invocations. `list`/`status` acquire a shared lock for reads
- [ ] **CORE-05**: Atomic writes via `google/renameio/v2` (temp-file + rename + directory fsync). State file mode `0600`. Parent directory created with mode `0700` if missing
- [x] **CORE-06**: State file path resolution — `$XDG_STATE_HOME/mcp-chain/state.json` if `$XDG_STATE_HOME` is set; otherwise `~/.mcp-chain/state.json`. Path documented in `--help` and README
- [ ] **CORE-07**: Word-ID generator uses the EFF short wordlist (1296 words, embedded via `go:embed`). Monotonic `counter` in state selects next word; once exhausted, falls back to hex counter. Counter never decremented on purge (prevents ID reuse)
- [ ] **CORE-08**: Per-process `OwnerToken` (128-bit crypto/rand) generated at `serve` startup, stored with each registration, and required to match on `resolve`. Provides session-link enforcement without relying on MCP protocol identity. CLI `resolve` escape hatch with `--force` flag for operator-driven recovery
- [ ] **CORE-09**: State schema versioned via top-level `version` field; unknown versions error clearly. Corrupt JSON error returns actionable message (file path + repair guidance)
- [ ] **CORE-10**: All MCP tool descriptions, slash-command prompts, and model-facing strings kept bare-minimum. Tool descriptions ≤1 short sentence. Every description reviewed during implementation for "can this be shorter?"

### MCP Server

- [ ] **MCP-01**: Use `github.com/modelcontextprotocol/go-sdk` v1.5+ with `StdioTransport`. No HTTP transport. No `net/http` import path allowed (enforce via CI size check)
- [ ] **MCP-02**: Strict stdout discipline — only JSON-RPC traffic on stdout. All logging goes to stderr via stdlib `log/slog`. No dep that logs to stdout
- [ ] **MCP-03**: MCP server layer is a thin adapter over `internal/store`; no MCP types leak into core logic (hexagonal boundary)

### Slash Commands (Claude Code plugin)

- [ ] **CMD-01**: `/chain-reg [condition]` — registers the current session and prints the word-ID. If `[condition]` is omitted, prompts the user in-conversation for the condition before registering
- [ ] **CMD-02**: `/chain-wait [id] [--timeout DURATION]` — runs the bash monitor that polls `mcp-chain status <id>` every 1s and prints `continue` on resolve. `--timeout` accepts Go-style duration strings (`30s`, `1m`, `1h`, `24h`, `168h` for 1w); timeout value is echoed on invocation so it appears in the monitor log. Without `--timeout`, wait is unbounded. Errors immediately if the ID doesn't exist at start. Monitor also errors if the ID is purged mid-wait
- [ ] **CMD-03**: `/chain-list` — prints a human-readable table of all entries (ID, status, condition, created_at, resolved_at)
- [ ] **CMD-04**: `/chain-purge [id | --all | --resolved]` — explicit cleanup. Requires at least one of the three arguments; bare `/chain-purge` errors
- [ ] **CMD-05**: All slash-command prompt bodies are terse — single paragraph max. No boilerplate, no multi-paragraph explanations. Prompts tell Claude exactly what MCP tool to call with what args

### Bash Monitor Helper

- [ ] **HELPER-01**: Repo ships `scripts/chain-wait.sh` that wraps `mcp-chain status <id>` in a 1-second poll loop, prints `continue` on resolve, errors to stderr on unknown ID (mid-wait purge), and supports `--timeout DURATION`. `/chain-wait` instructs Claude to run it via the monitor tool
- [ ] **HELPER-02**: Script is POSIX-bash compatible, no bashisms that break on macOS's default bash 3.2

### Distribution & Install

- [ ] **DIST-01**: Packaged as a Claude Code plugin — `plugin.json` manifest, `commands/*.md` slash commands, `.mcp.json` referencing `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain serve`. Installable via `/plugin install <github-repo>` with zero additional config
- [ ] **DIST-02**: GitHub Actions CI via GoReleaser — on push/PR runs `go test -race ./...` and `go build`; on tag (`v*`) cross-compiles `linux/darwin/windows` × `amd64/arm64`, generates checksums, attaches binaries to the GitHub release
- [ ] **DIST-03**: CI size gate — fail build if binary exceeds 15MB stripped (`-ldflags="-s -w"`). Fail startup-time smoke test if binary takes >100ms to print `--version`
- [ ] **DIST-04**: Brief `README.md` covering why it exists, install steps (plugin install + manual install), usage examples for `/chain-reg`, `/chain-wait`, `/chain-list`, `/chain-purge`, and manual CLI usage

### Quality & Testing

- [ ] **QA-01**: Unit tests for all core logic — wordlist allocation determinism, counter monotonicity, hex fallback, state schema round-trip, timeout parsing, path resolution, ID lookup, state transitions (pending → resolved, double-resolve error, unknown error, OwnerToken mismatch error)
- [ ] **QA-02**: Integration tests exercising full flows — end-to-end register → status pending → resolve → status resolved; N concurrent waiters on the same ID all see resolve; double-resolve error; unknown-ID error; purge-mid-wait error; OwnerToken mismatch error; cross-process flock safety (spawn two processes, register 100 entries each concurrently, verify no lost updates and no duplicate IDs)
- [ ] **QA-03**: `go test -race ./...` runs in CI on every push/PR. Failing tests block merges. Also run race tests on macOS + Linux runners; Windows runs non-race test suite
- [ ] **QA-04**: CI lint gate — `go vet` + `staticcheck` (or equivalent); non-zero exit blocks merge

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Observability

- **OBS-01**: Structured `--json` output mode for `list` and `status` (scripting-friendly)
- **OBS-02**: Shell completion generation (`mcp-chain completion bash|zsh|fish`)
- **OBS-03**: `mcp-chain doctor` subcommand — sanity check state file, verify permissions, report version

### UX Polish

- **UX-01**: Color output in `list` (TTY-only, respects `NO_COLOR`)
- **UX-02**: Fuzzy prefix match on `status <id-prefix>` (e.g., `mcp-chain status ott` matches `otter`)

### Lifecycle

- **LC-01**: Optional TTL on registration (`register(condition, ttl)`) with auto-GC
- **LC-02**: Export/import of state file for backup

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Networked / multi-machine coordination | Out of scope — local filesystem only; use etcd/Consul for distributed locks |
| Daemon / socket server | Rejected in favor of shared JSON file; simpler lifecycle, no background process |
| SQLite / embedded DB | Overkill for expected entry counts; JSON + flock is sufficient and keeps binary small |
| Programmatic conditions (polled shell expressions) | Rejected; natural-language Claude-judged resolution is simpler and more flexible |
| Auto-expiration of resolved IDs (v1) | Explicit `/chain-purge` only — predictable, no surprising GC. TTL deferred to v2 (LC-01) |
| DAG execution / task scheduling | Not a workflow engine; use Airflow/Taskfile/Make for DAGs |
| Artifact passing between sessions | Out of scope; mcp-chain is a signal primitive, not a pub/sub or message queue |
| Heartbeat / stale-lock detection | Rejected; explicit purge is the model. Adds complexity without clear value for single-user local use |
| MCP HTTP transport | Stdio only; matches Claude Code's plugin model |
| OAuth / auth beyond OwnerToken | Single-user local tool; no multi-user access control |
| License selection | Deferred to pre-release polish; not a blocker for v1 build |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CORE-01 | Phase 1 (skeleton) + Phase 6 (complete) | Pending |
| CORE-02 | Phase 5 | Pending |
| CORE-03 | Phase 5 | Pending |
| CORE-04 | Phase 4 | Pending |
| CORE-05 | Phase 4 | Pending |
| CORE-06 | Phase 3 | Complete |
| CORE-07 | Phase 2 | Pending |
| CORE-08 | Phase 4 | Pending |
| CORE-09 | Phase 4 | Pending |
| CORE-10 | Phase 5 | Pending |
| MCP-01 | Phase 5 | Pending |
| MCP-02 | Phase 1 | Pending |
| MCP-03 | Phase 5 | Pending |
| CMD-01 | Phase 8 | Pending |
| CMD-02 | Phase 8 | Pending |
| CMD-03 | Phase 7 (CLI) + Phase 8 (slash command) | Pending |
| CMD-04 | Phase 7 (CLI) + Phase 8 (slash command) | Pending |
| CMD-05 | Phase 8 | Pending |
| HELPER-01 | Phase 8 | Pending |
| HELPER-02 | Phase 8 | Pending |
| DIST-01 | Phase 8 | Pending |
| DIST-02 | Phase 9 | Pending |
| DIST-03 | Phase 1 | Pending |
| DIST-04 | Phase 10 | Pending |
| QA-01 | Phase 9 | Pending |
| QA-02 | Phase 9 | Pending |
| QA-03 | Phase 9 | Pending |
| QA-04 | Phase 1 | Pending |

**Coverage:**
- v1 requirements: 28 total
- Mapped to phases: 28
- Unmapped: 0

**Notes:**
- Unit and integration tests are authored continuously alongside implementation in Phases 2-8 as normal engineering practice. QA-01 and QA-02 land in Phase 9 as the *requirement verification* — that is where the comprehensive suite becomes the blocking gate (via `go test -race` in CI, QA-03).
- CORE-01 is split: the subcommand skeleton with `--version` lands in Phase 1 (needed by the startup/size smoke test); `status` completes in Phase 6; `list`/`purge`/`resolve` complete in Phase 7. The full requirement is satisfied at Phase 7 close; Phase 1 ownership is for the skeleton + dispatch wiring.
- CMD-03 and CMD-04 split by surface: CLI semantics land in Phase 7 (`mcp-chain list`, `mcp-chain purge`); slash-command wrapper prompts land in Phase 8 (`/chain-list`, `/chain-purge`). Each REQ-ID is satisfied end-to-end only when both phases close.
- Session-link (CORE-08) and state schema (CORE-09) live in Phase 4 (the store phase) per research resolution — not a separate phase.
- Stdout discipline (MCP-02) and size/startup gates (DIST-03) are established in Phase 1, before any MCP handler code is written (Phase 5).
- License selection is deferred to pre-release polish and is intentionally *not* a roadmap phase.

---
*Requirements defined: 2026-04-23*
*Last updated: 2026-04-23 after roadmap creation (phase mappings)*
