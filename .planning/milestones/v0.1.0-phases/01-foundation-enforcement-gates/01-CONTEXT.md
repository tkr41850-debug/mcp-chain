# Phase 1: Foundation & Enforcement Gates - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Module scaffold and the correctness rails (stdout discipline, lint, binary-size ceiling, startup-time budget) are in place before any production code is written, so downstream phases never retrofit them.

**Success Criteria:**
1. `go build ./...` produces a `mcp-chain` binary that supports `--version` and stubs for `serve`/`status`/`list`/`purge` via `alecthomas/kong`
2. `mcp-chain serve </dev/null` writes exactly zero bytes to stdout (stderr-only logging hard-set in `main()`, `fmt.Print*` forbidden in serve path via a `forbidigo`/lint rule)
3. CI fails if the stripped binary exceeds 15 MB or if `time mcp-chain --version` exceeds 100 ms on the cold-cache smoke runner
4. `go vet` + `staticcheck` (or equivalent) run in CI and block on non-zero exit

**Requirements:** CORE-01 (skeleton only — kong `--version` + subcommand stubs), MCP-02, DIST-03, QA-04

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Key constraints from PROJECT.md / REQUIREMENTS.md / STACK.md research:
- Go ≥ 1.22; pure-Go deps (no cgo)
- `alecthomas/kong` v1.15.0 for CLI
- Stripped binary ≤ 15MB, cold startup ≤ 100ms, RSS ≤ 20MB
- Stdout reserved for MCP wire traffic; all logs go to stderr via `log/slog`
- Enforcement via lint (`forbidigo`) + CI gates (size + startup)
- `staticcheck` v2025.1.1 for static analysis

</decisions>

<code_context>
## Existing Code Insights

Empty Go project — no existing code. This is the module scaffold phase.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
