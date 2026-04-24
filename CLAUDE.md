<!-- GSD:project-start source:PROJECT.md -->
## Project

**mcp-chain**

A lightweight MCP server (distributed as a Claude Code plugin) that lets you chain Claude Code sessions together at arbitrary fan-in/fan-out. Any session can register a lock with a resolution condition and get back a short word-ID; any number of other sessions (in different conversations) can wait on that ID until the registering session resolves it. Primary user: the author, for orchestrating multi-session Claude Code workflows at scale.

**Core Value:** N Claude Code sessions can coordinate via shared locks — register in one, any number of others wait on it, resolve when ready — with **minimal overhead** (fast startup, low memory, terse tool prompts, small binary).

### Constraints

- **Tech stack:** Go (pure-Go deps only — no cgo)
- **Performance:** Startup ≤100ms; resident memory ≤20MB during normal use; binary size ≤15MB
- **Token budget:** MCP tool descriptions and any model-facing text (slash-command prompts, instruction strings) kept as terse as possible — this is a product principle, not a style note. Every tool description should be reviewable for "can this be shorter?"
- **Platform:** Linux + macOS primary; Windows supported via CI cross-compile
- **Dependencies:** Minimize. Stdlib-first. Only add a dep if it saves meaningful code
- **Distribution:** Must install via Claude Code plugin mechanism from a public GitHub repo
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## TL;DR
- **MCP:** official `github.com/modelcontextprotocol/go-sdk` v1.5.0 (stable, Google-collab, active)
- **CLI:** `github.com/alecthomas/kong` v1.15.0 (smallest of the three big frameworks, most parsimonious for 3–4 subcommands with struct-tag DX)
- **File lock:** `github.com/gofrs/flock` v0.13.0 (thin cross-platform wrapper; zero runtime overhead)
- **Atomic JSON write:** `github.com/google/renameio/v2` v2.0.2 (temp-file + rename, umask-correct)
- **Release:** GoReleaser v2.15.4 via `goreleaser/goreleaser-action` (config-driven cross-compile matrix, attaches release assets)
- **Testing:** stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1; integration tests spawn `go run` subprocess pairs
- **Claude Code plugin:** YES, can bundle a Go binary directly — `.mcp.json` accepts `${CLAUDE_PLUGIN_ROOT}/path/to/binary` as `command`. No npm/uvx bridge needed.
## Critical Finding: Plugin Distribution of a Go Binary
## Recommended Stack
### Core Technologies
| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.23+ | Language / runtime | Single static binary, pure-Go deps satisfy cgo-free constraint, cross-compile via GOOS/GOARCH is trivial |
| `github.com/modelcontextprotocol/go-sdk` | v1.5.0 (2026-04-07) | MCP protocol | Official SDK, Google-maintained, stable 1.x with backward-compat guarantees, supports `StdioTransport` out of the box. Training-data-era community libs are now superseded. |
| `github.com/alecthomas/kong` | v1.15.0 | CLI parser | Declarative struct-tags fit a terse 3–4 subcommand CLI in ~50 LOC; smallest stripped binary among the big three (3.4 MB vs 3.4–3.7 MB for cobra/urfave); no code generator, no config files, no globals. Clean for `serve` / `status <id>` / `list` / `purge` |
| `github.com/gofrs/flock` | v0.13.0 (2025-10-09) | Advisory file lock | Cross-platform (Linux/macOS `flock(2)`, Windows `LockFileEx`), pure-Go, trivial API (`f.Lock()`, `f.Unlock()`, `f.TryLock()`). Actively maintained. Avoids hand-rolling build-constrained `x/sys` wrappers |
| `github.com/google/renameio/v2` | v2.0.2 | Atomic file replace | Writes to temp file in same dir, fsyncs, renames. Prevents half-written state.json on crash. The idiomatic Go answer to "atomic file write" |
| `encoding/json` | stdlib | State (de)serialization | Fast enough at expected entry counts (≤100s); zero deps; human-readable on disk as required |
### Supporting Libraries
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify/require` | v1.11.1 | Test assertions | Use `require.*` (fail-fast) for readable unit tests. `require` over `assert` — mcp-chain tests should halt on first failure rather than cascading. Do NOT pull `testify/mock` or `testify/suite` |
| stdlib `testing` | — | Test runner, `-race` | All tests use stdlib runner; `go test -race ./...` is the CI gate for QA-03 |
| stdlib `embed` | — | Embed EFF wordlist | `//go:embed eff_short_wordlist.txt` — zero runtime cost, single binary, satisfies CORE-06 |
| stdlib `os/exec` | — | Integration tests spawn subprocesses | For cross-process flock tests (QA-02): spawn two `go run ./cmd/mcp-chain serve` processes, feed them MCP messages, verify no state corruption |
### Development Tools
| Tool | Purpose | Notes |
|------|---------|-------|
| GoReleaser | Cross-compile + attach binaries to GitHub releases | v2.15.4. Config in `.goreleaser.yaml`. Use `goreleaser/goreleaser-action@v6` in CI. Handles linux/macos/windows × amd64/arm64 matrix with ~15 lines of YAML |
| `golangci-lint` | Lint | Default config fine; include `govet`, `staticcheck`, `errcheck`, `gofmt`, `gosimple`. Runs in CI alongside tests |
| GitHub Actions | CI | `actions/checkout@v4`, `actions/setup-go@v5`, `goreleaser/goreleaser-action@v6`. `go test -race -count=1 ./...` on push/PR; GoReleaser runs on tag |
## Installation
# Initialize module
# Core deps (add as imports, then `go get`)
# Dev dep
## Minimal stdio server sketch (official SDK)
## Minimal kong CLI sketch
## MCP Session Identity — Design Note
- On server startup, generate a process-unique session token (UUIDv7 or crypto/rand 128-bit). Persist `{id → {condition, sessionToken, ...}}` on `register`.
- On `resolve`, compare caller's in-memory session token to the stored one. Reject mismatch with `ErrNotOwner`.
- A restarted server process cannot resolve sessions it previously registered. This is actually desirable — it matches user intent (a session that crashed shouldn't silently resolve).
- If this proves too strict in practice, fall back to PROJECT.md's documented default: "any connection can resolve once".
## Alternatives Considered
| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `modelcontextprotocol/go-sdk` v1.5.0 | `mark3labs/mcp-go` v0.49.0 | If you need Streamable HTTP + SSE transport maturity today and accept pre-1.0 API churn. mcp-chain is stdio-only, so this advantage doesn't apply |
| `modelcontextprotocol/go-sdk` v1.5.0 | `metoro-io/mcp-golang` v0.16.1 | **Not recommended.** Last release Feb 2026; low activity; known JSON-RPC edge cases. Use only if refactoring a legacy dependency |
| Kong | Cobra | Large apps with many nested commands, auto-generated completion scripts, or when you already use viper/cobra ecosystem. Overkill here; brings `spf13/pflag` + `spf13/viper` transitive weight and a `cobra-cli` codegen culture |
| Kong | urfave/cli v3 | Minimalism with functional-style config. Comparable to kong. Choose kong for struct-tag DX; urfave for App{Commands:...} literal DX. Either is fine; kong edges out on stripped size and declarative clarity |
| Kong | stdlib `flag` | Only if you want zero deps and <2 subcommands. With 4 subcommands each having their own flag sets, hand-rolling flag-parsing with `os.Args[1]` switch is ~100 LOC you don't need to maintain |
| `gofrs/flock` | `golang.org/x/sys/unix.Flock` + build-tagged Windows impl | If you want absolute zero external deps. Costs ~40 LOC of build-tagged platform code. `gofrs/flock` is pure-Go, ~400 LOC, stable since 2019; net negative to reinvent |
| `renameio/v2` | Hand-rolled `os.CreateTemp` + `os.Rename` | If you want zero deps. `renameio` adds fsync-before-rename for durability and handles symlink edge cases; worth one small dep |
| GoReleaser | Plain GitHub Actions matrix | If you only need linux binaries, no checksums, no release notes automation. GoReleaser adds 15 lines of YAML and gives you: multi-arch archives (`.tar.gz` + `.zip`), `checksums.txt`, SBOM optional, release body from changelog, and zero CGO-related fragility (mcp-chain is pure Go, avoiding the one thing GoReleaser doesn't do well) |
| JSON + flock | SQLite (`modernc.org/sqlite`, pure-Go) | If entry counts scale to thousands and queries get complex. Rejected in PROJECT.md for good reason; JSON is right at expected scale |
| JSON + flock | bbolt | Same rationale as SQLite; adds dep, harder to inspect, overkill |
## What NOT to Use
| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `metoro-io/mcp-golang` | Low maintenance velocity, known JSON-RPC bugs, pre-1.0 with no stability commitment | Official `modelcontextprotocol/go-sdk` |
| `spf13/cobra` + `spf13/viper` stack | Heavy transitive deps (~2 MB extra at import), codegen culture, overkill for 4 subcommands | `alecthomas/kong` |
| `mattn/go-sqlite3` / any cgo deps | Breaks "no cgo" constraint; complicates cross-compile (CGO_ENABLED=0 is the default here) | Stay with JSON + flock |
| `google/uuid` if only used for session tokens | 100 KB+ and transitive, for one use-case | `crypto/rand` + `encoding/hex` in ~5 LOC |
| Custom wordlist generator at runtime | Adds startup cost, binary size, and a failure mode | `//go:embed` the EFF short wordlist, allocate deterministically |
| `github.com/pkg/errors` | Deprecated since Go 1.13 introduced `errors.Is`/`errors.As`/`%w` | stdlib `errors` + `fmt.Errorf("context: %w", err)` |
| `logrus` / `zap` / `zerolog` for this project | Overkill; MCP stdio server must not write logs to stdout (it's the MCP channel). Use `log/slog` to stderr only, or silence logging entirely | stdlib `log/slog` → `os.Stderr`, or write to a file under `$XDG_STATE_HOME/mcp-chain/log` |
| Goroutine pools / `errgroup` for state access | Over-engineered for a file-lock-serialized store | Plain functions + `sync.Mutex` inside a process, `flock` across processes |
| `npm`-wrapped or `uvx`-wrapped distribution | Unnecessary — Claude Code plugins run native binaries directly via `${CLAUDE_PLUGIN_ROOT}` | Bundle the platform binary directly; detect platform at install via a `SessionStart` hook if needed |
## Release Pipeline (GoReleaser)
## Testing Strategy
- stdlib `testing` + `testify/require`
- Table-driven tests for wordlist allocation, ID lookup, timeout parsing, state transitions
- No mocks — touch the real filesystem via `t.TempDir()`
- Cross-process flock safety: `exec.Command("go", "run", "./cmd/mcp-chain", "serve")` × 2, feed both via stdin pipes, verify serialized state writes
- Double-resolve: register → resolve → resolve (expect error)
- Unknown-ID: resolve a never-registered id (expect error)
- Concurrent waiters: spawn N `status` polls, resolve, verify all see resolved within one poll interval
- run: go test -race -count=1 -timeout 60s ./...
## Version Compatibility
| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `modelcontextprotocol/go-sdk` v1.5.0 | Go 1.23+ | SDK declares Go 1.23 as minimum in its go.mod |
| `alecthomas/kong` v1.15.0 | Go 1.20+ | No constraint in practice |
| `gofrs/flock` v0.13.0 | Go 1.20+ | Uses `golang.org/x/sys` internally |
| `renameio/v2` v2.0.2 | Go 1.17+ | No symlink atomic guarantees on Windows (accept this — Windows is secondary platform) |
| GoReleaser | v2.15.x | v2 config format (`version: 2` at top of `.goreleaser.yaml`) is required; v1 configs won't work |
## Stack Patterns by Variant
- Reconsider JSON + flock → move to `modernc.org/sqlite` (pure-Go, cgo-free SQLite)
- Threshold is when the file read/write per-op cost exceeds ~5 ms on a SATA SSD
- Out of scope per PROJECT.md, but the escape hatch is the Streamable HTTP transport in the MCP SDK + a small coordinator service
- Do not attempt to share state.json over NFS — flock semantics over NFS are unreliable
- Alternative install path: single `go install github.com/owner/mcp-chain@latest` plus a user-scope `.mcp.json` pointing at `$GOBIN/mcp-chain` — loses zero-config install but recovers full filesystem access. Plugin path is preferred for the stated audience.
## Sources
- [code.claude.com — Plugins reference](https://code.claude.com/docs/en/plugins-reference) — HIGH confidence; plugin.json schema, `.mcp.json` format, `${CLAUDE_PLUGIN_ROOT}`, bin/ auto-PATH
- [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — HIGH; v1.5.0 released 2026-04-07 (verified via GitHub API)
- [pkg.go.dev: modelcontextprotocol/go-sdk/mcp](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — HIGH; StdioTransport, AddTool, Server API
- [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) — HIGH; v0.49.0 released 2026-04-21 (still pre-1.0)
- [github.com/metoro-io/mcp-golang](https://github.com/metoro-io/mcp-golang) — HIGH; v0.16.1 released 2026-02-25 (low activity)
- [github.com/alecthomas/kong](https://github.com/alecthomas/kong) — HIGH; v1.15.0 latest tag (verified via GitHub API)
- [github.com/gofrs/flock](https://github.com/gofrs/flock) — HIGH; v0.13.0 released 2025-10-09
- [github.com/google/renameio](https://github.com/google/renameio) — HIGH; v2.0.2 latest tag; v2 umask behavior change documented
- [goreleaser.com/ci/actions](https://goreleaser.com/ci/actions/) — HIGH; last updated 2026-03-22; v2.15.4 current
- [modelcontextprotocol.io/specification/2025-06-18/basic/transports](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports) — HIGH; confirms `Mcp-Session-Id` is HTTP-only, not stdio
- [github.com/gschauer/go-cli-comparison](https://github.com/gschauer/go-cli-comparison) — MEDIUM; binary-size numbers (kong 3.4 MB stripped, cobra 3.4 MB, urfave 3.7 MB) are Go 1.14 era but relative ordering is stable
- [github.com/stretchr/testify](https://github.com/stretchr/testify) — HIGH; v1.11.1 released 2025-08-27
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
