# Project Research Summary

**Project:** mcp-chain
**Domain:** Go MCP stdio server + Claude Code plugin for local session coordination (register / wait / resolve primitive)
**Researched:** 2026-04-23
**Confidence:** HIGH

## Executive Summary

mcp-chain is a single-binary Go tool that ships as a Claude Code plugin and gives N parallel Claude Code sessions a shared register/wait/resolve primitive over a flock-protected JSON state file — a category that sits mechanically between `flock` and job-queue semantics but is differentiated from both by natural-language resolution conditions and memorable word-IDs. The recommended stack is the official `modelcontextprotocol/go-sdk` v1.5.0 for MCP, `alecthomas/kong` v1.15.0 for the 4-subcommand CLI (`serve`/`status`/`list`/`purge`), `gofrs/flock` + `google/renameio` for durable cross-process state, and GoReleaser for the cross-compile matrix. All dependencies are pure-Go, total 5 direct deps, with a comfortably-met binary budget of ~6–9 MB against the 15 MB ceiling.

Architecturally the design is hexagonal: `internal/store` owns domain + on-disk state and stays MCP-agnostic; `internal/mcpserver` and `internal/cli` are thin adapters over the same store functions. This boundary is the single most important one in the project — it localizes the MCP SDK choice to ~200 lines and lets every business-logic test run without any JSON-RPC scaffolding. Every mutation is a read-modify-write under exclusive flock with a same-directory temp-file-plus-rename save, and every read uses a shared flock so N waiters polling once a second don't serialize.

The highest-risk areas are concentrated in one place (the state layer: flock correctness, atomic writes, wordlist-allocation race, schema versioning) and are addressed first in the build order. The second concentration is MCP stdio hygiene (stdout corruption from stray `fmt.Println`, non-compact JSON, verbose tool descriptions burning per-turn tokens) — all solved by leaning on the SDK and enforcing lint/test gates from Phase 1. The single most important open question from PROJECT.md — "can the server distinguish the registering session?" — has a clean answer (see below) that closes the design with no SDK feature required.

## Key Findings

### Session-Link Resolution — the key design finding (closes PROJECT.md's open question)

This is the single most important cross-cutting finding and it resolves PROJECT.md's flagged open question.

- MCP stdio transport provides **no** per-connection session ID. The official `go-sdk`'s stdio transport explicitly returns `""` from `SessionID()`; `Mcp-Session-Id` is defined only for the HTTP transport in the MCP spec. Trying to derive identity from MCP on stdio is a dead end.
- **But the architecture still enforces ownership naturally**: Claude Code spawns a separate `mcp-chain serve` subprocess per Claude Code session (that's how `.mcp.json` works). The subprocess's lifetime is 1:1 with the Claude Code session's lifetime.
- **Design:** at `serve` startup, generate a random 128-bit `OwnerToken` (held in process memory only, hex-encoded, prefixed `srv-`). Write it into each record on `register`. On `resolve`, compare the in-memory token to the record's stored token; mismatch → `ErrNotOwner`. The CLI path (`mcp-chain resolve <id>`) bypasses the check as an administrative escape hatch for the "serve crashed and restarted" edge case.
- **Cost:** ~30 lines of code. No MCP feature required. No cross-process identity problem. Graceful degradation.
- **Recommendation:** ship this as the primary design. The PROJECT.md-approved fallback ("any connection can resolve once") remains available by setting `OwnerToken = ""`, which makes the check a no-op — zero refactor if we ever need to turn it off.

This finding interlocks three research files: STACK identified the SDK constraint, ARCHITECTURE turned it into a deployment-shape insight, PITFALLS flagged it as avoid-at-all-costs over-engineering if pursued via MCP identity. The per-process OwnerToken threads the needle.

### Recommended Stack

Pure-Go, 5 direct deps, sub-10 MB binary, single-file install path via Claude Code plugin. No cgo, no npm/uvx bridge needed — Claude Code plugins natively run compiled binaries via `${CLAUDE_PLUGIN_ROOT}/bin/<name>`.

**Core technologies:**
- **Go 1.23+** — language; single static binary, pure-Go deps, trivial GOOS/GOARCH cross-compile
- **`modelcontextprotocol/go-sdk` v1.5.0** — official MCP SDK, Google-maintained, stable 1.x, stdio transport out of the box. Supersedes training-data-era community libs.
- **`alecthomas/kong` v1.15.0** — declarative struct-tag CLI; smallest stripped binary of the big three; fits 4 subcommands in ~50 LOC with no globals or codegen
- **`gofrs/flock` v0.13.0** — cross-platform flock (Linux/macOS `flock(2)`, Windows `LockFileEx`); pure-Go; trivial API
- **`google/renameio/v2` v2.0.2** — atomic temp-file-plus-rename with fsync-before-rename; the idiomatic Go answer to "atomic file write"
- **`stretchr/testify/require`** + stdlib `testing` — use `require` (fail-fast) for unit tests; stdlib runner for `go test -race` CI gate
- **GoReleaser v2.15.4** — cross-compile matrix + checksums + release attachment in ~15 lines of YAML

**Explicitly rejected:** `metoro-io/mcp-golang` (low activity, pre-1.0), `spf13/cobra`+`viper` (heavy transitive deps for 4 subcommands), any cgo dep, `github.com/pkg/errors` (deprecated since Go 1.13), runtime wordlist generators (use `//go:embed`), structured loggers like zap/logrus (stdlib `log/slog → os.Stderr` is sufficient and avoids stdout-corruption risk).

### Expected Features

PROJECT.md's Active requirements already cover the full table-stakes set for this category. Cross-reference against adjacent tools (flock, etcd/Consul, GitHub Actions `needs:`, `wait-for-it`, Taskwarrior) surfaces zero missing must-haves. The differentiators are sharp and category-novel.

**Must have (all in PROJECT.md Active):**
- Register with natural-language condition, returns short word-ID (CORE-02, CORE-06)
- Resolve (CORE-03) with distinct `ErrUnknownID` vs `ErrAlreadyResolved` error codes
- Status subcommand with scriptable exit codes 0/1/2 (CORE-01)
- flock-safe state file at XDG path (CORE-04, CORE-05)
- Slash commands `/chain-reg`, `/chain-wait`, `/chain-list`, `/chain-purge` (CMD-01..04)
- Bash monitor polling helper (HELPER-01) — `/chain-wait` depends on `status`
- `--timeout` on wait (CMD-02)
- Terse tool descriptions (CORE-07) — this is a product principle, not a polish item
- Plugin distribution + CI release artifacts (DIST-01..03)

**Should have (differentiators — these are why pick mcp-chain over `flock`+file):**
- Natural-language resolution conditions (LLM-judged, human-readable) — category-unique
- EFF-wordlist memorable IDs (tangerine-squirrel beats `5f3e2a8c-...` for a human-in-the-loop tool)
- Zero-install via Claude Code plugin
- MCP-native terse tool descriptions optimized for per-turn token cost

**Defer (v2+):**
- Explicit `/chain-cancel` (if purge-to-abort causes real user confusion)
- Resolution note field, audit log, tags, shell completion, `mcp-chain --version` — all polish, not launch
- Session-link *tightening beyond OwnerToken* (per-user-request if the model ever misreads flow)

**Anti-features — never build** (all rejected in PROJECT.md and re-validated by feature research): auto-TTL/heartbeat, cron scheduling, DAG execution, artifact passing, daemon, SQLite, web UI, programmatic conditions.

### Architecture Approach

Hexagonal split with three layers: pure domain core (`internal/store`), two thin adapters (`internal/mcpserver`, `internal/cli`), and small utility packages (`internal/idgen`, `internal/wordlist`, `internal/xdg`). Single binary at `cmd/mcp-chain/main.go` does argv dispatch only (~50 LOC). The state file is the only cross-process coordination channel — no sockets, no daemon, no IPC.

**Major components:**
1. **`internal/store`** — `Register`/`Resolve`/`Get`/`List`/`Purge` over a flock-protected JSON file. Owns the `State` struct, `Record` type, schema version, atomic read-modify-write machinery. **No MCP or CLI types leak in.** Exposes `Store` interface + sentinel errors (`ErrUnknownID`, `ErrAlreadyResolved`, `ErrNotOwner`).
2. **`internal/mcpserver`** — MCP protocol adapter (depends on SDK choice). Wires `register` and `resolve` tools to store functions; per-process `OwnerToken` lives here; terse tool descriptions enforced with a token budget.
3. **`internal/cli`** — `status`/`list`/`resolve`/`purge` subcommand functions returning exit codes; output formatting (table rendering) lives here.
4. **`internal/idgen` + `internal/wordlist`** — pure `Allocate(counter uint64) string`; EFF wordlist via `//go:embed`; deterministic hex fallback beyond 1296.
5. **`internal/store/flock_{unix,windows}.go`** — build-tagged locking primitives; separate lock-file pattern on Windows (the lock file must not be the state file because Windows `rename` fails on open files).
6. **`plugin/`** — static Claude Code plugin content at repo root: `.claude-plugin/plugin.json`, `commands/*.md`, `.mcp.json` (pointing at `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`), `scripts/chain-wait.sh` monitor.

**State-file schema (versioned from v1):** flat `{id → Record}` map with top-level `version`, monotonically-increasing `counter` (never decremented on purge — reusing counter values would reissue word-IDs and violate uniqueness expectations), ISO-8601 timestamps, nullable `resolved_at`, omitempty `owner_token`.

**Read path uses `LOCK_SH`, write path uses `LOCK_EX`**. This is a non-trivial optimization: N waiters polling once/second each would otherwise serialize behind a single exclusive lock.

### Critical Pitfalls

Top pitfalls ordered by cost-of-finding-late. Each is linked to the phase that prevents it.

1. **MCP stdout corruption from stray `fmt.Println` / library banners** — any non-JSON-RPC byte on stdout breaks the protocol. Symptom: Claude Code parse errors, session drops. Prevention: hard-set `log.SetOutput(os.Stderr)` in `main()`, forbid `fmt.Print*` in serve path via `forbidigo` lint, integration test that pipes stdout and asserts empty-or-valid-JSON-only. **Lock down in Phase 1 before any tool is added.**
2. **Lost-update race on registration (wordlist collision)** — if read-modify-write isn't atomic under a single lock hold, two concurrent registers get the same word-ID. Prevention: one `withLockedState(fn func(*State) error)` API; never expose a "load then save" helper. Test: 50 goroutines × 50 subprocesses concurrent register, assert 100 unique IDs.
3. **Atomic-write without fsync** — `rename` is atomic but not durable on crash; the zero-byte-state-file failure mode is silent. Prevention: use `google/renameio/v2` (handles fsync of tmp + parent dir); kill-9 mid-write test. **Do not roll your own.**
4. **Windows flock + rename semantics** — `LockFileEx` is mandatory (not advisory) and `os.Rename` fails over an open file on Windows. Prevention: use a separate lock file (`state.json.lock`), never the state file itself; CI matrix must include `windows-amd64` for tests, not just cross-compile.
5. **Verbose MCP tool descriptions burn tokens every turn** — PROJECT.md's token-budget principle is a hard constraint. Budget: ≤ 1 short sentence per tool, ≤ 40 tokens per tool, < 200 tokens total. Probe with natural-language invocation tests; fix "model picks wrong tool" by renaming, not by adding prose.
6. **Plugin distribution mistakes** — `npx`/`uvx` commands copy-pasted from Node examples, or absolute paths baked into `.mcp.json`, silently break install. Prevention: `"command": "${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain"`; per-OS binaries under `bin/<os>-<arch>/`; fresh-machine install test with no Go/Node/Python.
7. **Counter reuse after purge** — tempting shortcut `if len(entries) == 0 { counter = 0 }` reissues old word-IDs. Test with PR review checklist; counter is monotonic forever, hex fallback is fine.

## Implications for Roadmap

Research points to a clear bottom-up build order: **the store layer carries nearly all the correctness risk, is SDK-agnostic, and is the dependency for everything else — so it ships first, standalone, fully tested.** The MCP adapter (the only layer tied to the SDK choice) comes second. Distribution and polish come last. This sequencing means a flock bug is caught with zero MCP scaffolding in the way, and swapping SDKs later is a ~200-line change in one package.

### Phase 1: Foundation — wordlist, XDG, store, flock

**Rationale:** The state layer is the highest-risk single component (flock correctness, atomic writes, wordlist-allocation race). It is also SDK-agnostic — pure Go, pure business logic, testable without any MCP machinery. Fail here and everything downstream fails; get this right and downstream is mostly wiring.

**Delivers:** `internal/wordlist`, `internal/idgen`, `internal/xdg`, `internal/store` with `Open`/`Register`/`Resolve`/`Get`/`List`/`Purge`, schema_version from v1, atomic RMW via `renameio`, flock machinery (Unix + Windows build-tagged with separate-lock-file pattern on Windows), sentinel errors.

**Addresses (FEATURES):** CORE-04, CORE-05, CORE-06, and the invariants underneath CORE-02/03 (register/resolve semantics).

**Avoids (PITFALLS):** #6 lost-update, #7 missing-fsync, #8 flock-on-NFS (add startup warning), #9 Windows flock, #10 wordlist race, #15 error-code distinctness, #18 schema-versioning.

**Also in this phase:** CI skeleton with `go test -race -count=1` gate (QA-03), binary-size check (<15 MB), startup-time check (<100 ms on cold cache). Pitfalls #11, #12, #13 are one-time setup costs best paid now.

### Phase 2: MCP server adapter + tool handlers

**Rationale:** This is the **only** phase tied to the MCP SDK choice. With the store locked down, the adapter is a thin translation layer. Separating it from Phase 1 means the SDK choice is a localized ~200-line swap if v1.5.0 of the official SDK ever regresses.

**Delivers:** `internal/mcpserver` with `register` and `resolve` tools, terse descriptions under a 40-token-per-tool budget, stderr-only logging hard-set in `main()`, per-process `OwnerToken` generation + enforcement in resolve, stdio handshake via the official SDK.

**Uses (STACK):** `modelcontextprotocol/go-sdk` v1.5.0 (`AddTool[Args]` with reflection-derived JSON schema), stdlib `crypto/rand` for OwnerToken.

**Implements (ARCHITECTURE):** hexagonal adapter pattern (MCP types stay in this package, never leak to store); SessionID-finding design (per-process OwnerToken, not SDK-provided identity).

**Avoids (PITFALLS):** #1 stdout corruption (lint + integration test), #2 handshake (SDK does it), #3 non-compact JSON (SDK does it), #4 verbose descriptions (token-budget gate), #15 distinct error codes on the wire, #16 session identity (resolved via OwnerToken, not attempted via MCP).

### Phase 3: CLI subcommands + argv dispatch

**Rationale:** Trivial layer once the store is done; mostly formatters and exit-code translation. Parallel-executable with Phase 2 if desired, but grouping separately keeps Phase 2 narrow and reviewable.

**Delivers:** `internal/cli` with `status`, `list`, `resolve`, `purge` (table formatter for list, filter semantics for purge); `cmd/mcp-chain/main.go` dispatch with `kong`; wait's unknown-ID path during monitor mid-flight (not just at start).

**Addresses (FEATURES):** CORE-01 (serve/status/list/resolve/purge subcommands), CMD-04 purge semantics, implicit cancel behavior during `/chain-wait`.

**Avoids (PITFALLS):** exit-code confusion (tests assert 0/1/2), `status` runs under `LOCK_SH` not `LOCK_EX` (anti-pattern #6).

### Phase 4: Claude Code plugin packaging + bash monitor

**Rationale:** Distribution can't ship until there's a binary to distribute, but is mostly independent of Phases 2–3 in terms of code touched. Gates on release tagging.

**Delivers:** `plugin/.claude-plugin/plugin.json`, four `commands/*.md` with ≤30-word token-budgeted prompts, `.mcp.json` using `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`, `scripts/chain-wait.sh` monitor with `--timeout` duration parsing.

**Addresses (FEATURES):** DIST-01 plugin install, CMD-01..04 slash commands, HELPER-01 bash monitor.

**Avoids (PITFALLS):** #5 npm/uvx path mistakes, #17 slash-command prompt bloat (30-word budget).

### Phase 5: CI release + cross-compile + packaging

**Rationale:** GoReleaser config and Windows CI matrix come last because they require a stable binary to release. Retrofitting GoReleaser later is painful; adopt from day one of this phase.

**Delivers:** `.github/workflows/test.yml` (go test -race on push/PR; Windows matrix with actual test runs), `.github/workflows/release.yml` + `.goreleaser.yaml` (tag-triggered cross-compile matrix linux/darwin/windows × amd64/arm64, SHA256SUMS, reproducible builds with `-trimpath -ldflags="-s -w -X main.version=..."`), binary-size and startup-time CI gates graduated to blocking.

**Addresses (FEATURES):** DIST-02 CI release, QA-03 race-gate (graduated to blocking).

**Avoids (PITFALLS):** #14 release misses arch / missing checksums / dirty version strings.

### Phase 6: Docs + dogfooding + README

**Rationale:** Last because the README needs real usage examples, and dogfooding surfaces polish items (empty-state messages, unknown-ID "did you mean?") cheaply.

**Delivers:** DIST-03 README (install, usage, why), NFS caveat documented, upgrade/reload instructions, `mcp-chain --version` (trivial addition caught by dogfooding).

### Phase Ordering Rationale

- **Store first** because it carries nearly all correctness risk and is SDK-agnostic — failure here is localized, success here de-risks every subsequent phase.
- **MCP adapter second** because it's the only SDK-coupled layer; isolating it behind the hexagonal boundary means SDK churn is a bounded-blast-radius event.
- **CLI third** because it shares the store interface with MCP and benefits from the same invariants, but is simpler (no wire protocol, no handshake).
- **Plugin fourth** because it depends on a working binary across platforms.
- **CI/release fifth** because it depends on a stable binary to release; retrofitting release tooling is painful.
- **Docs last** because real usage informs what to document.

This ordering also concentrates the high-cost-if-wrong decisions (flock, atomic write, counter monotonicity, schema version) in Phase 1 where they're cheapest to fix, and pushes low-risk/high-churn decisions (tool descriptions, README wording) to later phases where iteration is cheap.

### Research Flags

Phases likely needing **deeper research during planning** (`/gsd-research-phase`):

- **Phase 1:** probably **not** needed — flock/atomic-write patterns are well-covered in PITFALLS.md; `renameio` and `gofrs/flock` APIs are small and documented. The one spike worth doing is validating the **Windows separate-lock-file pattern** end-to-end before writing production code (cheap to do in a throwaway branch).
- **Phase 2:** **worth a small spike** — the exact API shape of `modelcontextprotocol/go-sdk` `AddTool[Args]` generics + `StdioTransport` is pinned but worth 30 minutes of "hello world" validation. Also confirm the token-count measurement approach for the description budget (tiktoken vs `len/4` approximation).
- **Phase 4:** **worth validation** — `${CLAUDE_PLUGIN_ROOT}` substitution across OSes, plugin install on a fresh machine with no dev tooling installed. Plugin docs are clear but end-to-end install has historically been where tools stumble.
- **Phase 5:** **no deep research needed** — GoReleaser config is well-documented and we have a recipe in STACK.md.

Phases with standard patterns (skip research):

- **Phase 3:** CLI subcommand implementation over a known interface — pure execution.
- **Phase 6:** documentation — pure writing.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versions verified via GitHub API on research date; official SDK docs confirmed for go-sdk stdio transport; binary-size numbers from community comparison (MEDIUM on exact byte counts, HIGH on relative ordering). |
| Features | HIGH | Table-stakes derived from well-documented adjacent tools (flock, etcd, Consul, wait-for-it, GitHub Actions, Taskwarrior); differentiators cross-checked against PROJECT.md scope; zero material gaps found. |
| Architecture | HIGH | Go layout (cmd/ + internal/) is official-docs-blessed convention; hexagonal split is straightforward for this LOC scale; flock + atomic-rename pattern is well-documented; session-link finding verified against go-sdk source code directly. MEDIUM on exact API surface of the chosen MCP SDK (pinned but not prototyped yet). |
| Pitfalls | HIGH | Most pitfalls drawn from official docs, issue trackers, and well-known post-mortems; specific numeric thresholds (100ms startup, 15MB binary, 40-token description) are MEDIUM — these are budgets from PROJECT.md, not empirical measurements. |

**Overall confidence:** HIGH.

### Gaps to Address

- **MCP SDK API surface prototyping** — the SDK is pinned at v1.5.0 but the exact tool-handler signature, context lifecycle, and error-translation conventions haven't been exercised. Address in Phase 2 with a 30-minute "hello world" spike before touching `internal/mcpserver`.
- **Windows-specific flock + rename edge cases** — `gofrs/flock` abstracts `LockFileEx` but the separate-lock-file pattern interaction with `renameio` on Windows hasn't been end-to-end verified. Address in Phase 1 with a small throwaway test on a Windows CI runner before production code ships.
- **Token-count measurement tooling** — CORE-07's "terse" principle needs a measurable gate. Decide in Phase 2 whether to vendor `tiktoken-go` (adds a dep) or use the cheap `len(text)/4` approximation (good enough for budget gates).
- **Session-link enforcement UX** — the OwnerToken design is right, but the CLI-resolve escape hatch UX (does it silently bypass, require `--force`, or prompt?) needs a call during Phase 2. Default recommendation: no flag required, document the behavior, file-permission 0600 on state is sufficient single-user auth.

## Sources

### Primary (HIGH confidence)
- [Claude Code plugins reference](https://code.claude.com/docs/en/plugins-reference) — plugin.json schema, `.mcp.json`, `${CLAUDE_PLUGIN_ROOT}`
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) + [pkg.go.dev](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — v1.5.0 API, StdioTransport, AddTool; source confirmation that stdio `SessionID()` returns `""`
- [MCP transports spec 2025-06-18 / 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports) — session-ID is HTTP-only
- [alecthomas/kong](https://github.com/alecthomas/kong), [gofrs/flock](https://github.com/gofrs/flock), [google/renameio](https://github.com/google/renameio), [stretchr/testify](https://github.com/stretchr/testify) — versions pinned via GitHub API
- [goreleaser.com/ci/actions](https://goreleaser.com/ci/actions/) + [reproducible builds](https://goreleaser.com/blog/reproducible-builds/)
- [Go module layout](https://go.dev/doc/modules/layout), [embed package](https://pkg.go.dev/embed), [go/issues/8914](https://github.com/golang/go/issues/8914) on Windows rename atomicity
- [MCP stdio corruption post-mortems](https://github.com/ruvnet/claude-flow/issues/835), [MCP server troubleshooting](https://mcpplaygroundonline.com/blog/mcp-server-troubleshooting-common-errors-fix)
- [flock(2)](https://man7.org/linux/man-pages/man2/flock.2.html), [flock(1)](https://man7.org/linux/man-pages/man1/flock.1.html), [apenwarr on file locking](https://apenwarr.ca/log/20101213), [Files are hard — Dan Luu](https://danluu.com/file-consistency/)
- [etcd lease docs](https://etcd.io/docs/v3.5/learning/why/), [Consul sessions](https://developer.hashicorp.com/consul/docs/dynamic-app-config/sessions), [wait-for-it](https://github.com/vishnubob/wait-for-it)

### Secondary (MEDIUM confidence)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout) — community reference
- [go-cli-comparison binary sizes](https://github.com/gschauer/go-cli-comparison) — relative ordering stable, absolute numbers Go 1.14 era
- [MCP tool description best practices (Merge)](https://www.merge.dev/blog/mcp-tool-description), [SEP-1576 token bloat](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1576) — community consensus on token budgets
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) — alternative SDK, used for cross-validation of session-ID semantics

### Tertiary (LOW confidence — inference, not primary sources)
- Specific numeric budgets (100 ms startup, 15 MB binary, 40 tokens/tool) — these come from PROJECT.md as design targets; verify empirically during Phase 1 CI setup

---
*Research completed: 2026-04-23*
*Ready for roadmap: yes*
