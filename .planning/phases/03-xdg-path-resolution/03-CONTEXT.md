# Phase 3: XDG Path Resolution - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The state file path is resolved per spec and its parent directory is guaranteed to exist with correct permissions, so the store layer can assume a ready writable location.

**Success Criteria:**
1. When `$XDG_STATE_HOME` is set, resolved path is `$XDG_STATE_HOME/mcp-chain/state.json`; otherwise `~/.mcp-chain/state.json`
2. Parent directory is created with mode `0700` if missing; no NSS/`os/user.Current` call runs on the startup path (uses `$HOME` env directly)
3. Unit tests cover both XDG and HOME branches via env-var manipulation, with `t.TempDir()` isolation

**Requirements:** CORE-06

**Depends on:** Phase 1 (module scaffold); no dependency on Phase 2 idgen

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting.

Key constraints:
- Package: `internal/statepath/` (private; store in Phase 4 consumes it)
- Pure-Go, stdlib only (`os`, `path/filepath`, `errors`)
- **No `os/user.Current()`** — that call can invoke NSS (cgo on some systems) and adds startup latency. Use `os.Getenv("HOME")` directly.
- **No `os.UserConfigDir` / `os.UserCacheDir` / `os.UserHomeDir`** — stdlib versions call `os/user` on some platforms.
- Function signature suggestion (final name at Claude's discretion): `func Resolve() (path string, err error)` or `ResolveStatePath() (string, error)`
- Directory creation: `os.MkdirAll(parent, 0o700)` — idempotent; do not chmod existing dirs
- Windows: deferred; this phase is linux/macOS primary. A short note in RESEARCH.md suffices.

### Non-goals
- No flock or state file IO — that's Phase 4
- No file creation — only directory creation

</decisions>

<code_context>
## Existing Code Insights

- Phase 1 scaffold: kong CLI, stdout discipline, lint gates, CI
- Phase 2: `internal/idgen/` — pattern for small pure packages (wordlist.go + idgen.go + tests)
- Module path: `github.com/anthropics/mcp-chain`
- Go 1.23+; pure-Go constraint
- Existing tests use `testify/require` v1.11.1

No existing `internal/statepath` package.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

- Windows path resolution (`%LOCALAPPDATA%\mcp-chain\state.json`) — deferred until CI cross-compile phase validates binary on Windows.

</deferred>
