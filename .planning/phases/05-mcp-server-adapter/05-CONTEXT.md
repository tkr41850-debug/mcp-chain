# Phase 5: MCP Server Adapter - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

A thin MCP stdio adapter exposes `register` and `resolve` tools over the Phase 4 `internal/store`, with terse descriptions, a per-process `OwnerToken`, and zero MCP types leaking into the core. This is the only phase tied to the MCP SDK choice — all MCP-specific code lives in `internal/mcpserver/`.

**Success Criteria:**
1. `mcp-chain serve` runs `modelcontextprotocol/go-sdk` v1.5+ over `StdioTransport`, completes the MCP initialize handshake, and exposes `register(condition) → id` and `resolve(id)` tools; **no `net/http` import in any `internal/**` or `cmd/**` source file** (the SDK transitively pulls `net/http` via its SSE/streamable files even under stdio use — amended 2026-04-24, accepted deviation). Stripped binary ≤ 15 MB is the binding size gate.
2. On `serve` startup a 128-bit `crypto/rand` `OwnerToken` is generated (`hex.EncodeToString` → 32-char string) and stamped onto every `register` record; `resolve` returns `ErrNotOwner` on mismatch with a distinct wire-level error code separate from `ErrUnknownID` and `ErrAlreadyResolved`
3. Every tool description is ≤ 1 short sentence / ≤ 40 tokens, total tool-list ≤ 200 tokens (CI token-budget probe); no MCP types appear in any `internal/store` identifier
4. Integration test pipes a recorded `initialize` + `register` + `resolve` sequence and asserts stdout is exclusively valid compact JSON-RPC with no embedded newlines or banners (stdout-discipline gate — the MCP wire must stay pristine)

**Requirements:** CORE-02, CORE-03, CORE-10, MCP-01, MCP-03

**Depends on:** Phase 4 (internal/store)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion. Key constraints:

- **Package**: `internal/mcpserver/` — MCP adapter (only package allowed to import `modelcontextprotocol/go-sdk`)
- **Dep to add**: `github.com/modelcontextprotocol/go-sdk` v1.5.0 (verified pure-Go; zero cgo)
- **Entry point**: wire `serve` subcommand in `internal/cli/stubs.go` (currently a stub that exits 3) to call into `internal/mcpserver`
- **File layout (suggested — at Claude's discretion):**
  - `internal/mcpserver/server.go` — `Run(ctx, store, ownerToken) error` — the top-level server loop
  - `internal/mcpserver/tools.go` — tool handlers (`registerHandler`, `resolveHandler`); terse descriptions as string constants at the top
  - `internal/mcpserver/owner.go` — `NewOwnerToken() (string, error)` via `crypto/rand`
  - `internal/mcpserver/errors.go` — mapping `store.ErrNotOwner` / `ErrUnknownID` / `ErrAlreadyResolved` → MCP tool-result errors with distinct codes
  - `internal/mcpserver/server_test.go` — in-process MCP client/server roundtrip
  - `internal/mcpserver/integration_test.go` — spawned-binary stdio test (pipe JSON-RPC, assert wire cleanliness)

### OwnerToken generation (locked)
```go
func NewOwnerToken() (string, error) {
    var buf [16]byte
    if _, err := rand.Read(buf[:]); err != nil {
        return "", err
    }
    return hex.EncodeToString(buf[:]), nil
}
```
32-char hex string, 128 bits of entropy. Generated once at `serve` startup, passed into each `Register` call.

### Tool descriptions (token-minimal — CLAUDE.md principle)
These are model-facing surfaces — every token matters. Target shape (final wording at Claude's discretion, but be ruthless):

- `register`: "Register a coordination lock; returns a short id." + param doc "condition: when this id is considered resolvable"
- `resolve`: "Resolve a lock id you previously registered." + param doc "id: the short id returned by register"

Aim for <40 tokens per description, <200 tokens for the whole tool-list block. CI gate in Phase 9 will enforce.

### Store / MCP boundary
- `internal/store` has NO knowledge of MCP. `internal/mcpserver` imports `internal/store`, never the reverse.
- Grep gate: `grep -r 'mcp\|MCP\|jsonrpc\|modelcontextprotocol' internal/store/` returns empty.

### Error mapping (sentinel → wire)
- `store.ErrUnknownID` → MCP tool-result error, code `"unknown_id"`, message `"unknown lock id"`
- `store.ErrAlreadyResolved` → code `"already_resolved"`
- `store.ErrNotOwner` → code `"not_owner"` (distinct wire code per SC #2)
- `store.ErrSchemaVersion` → code `"schema_error"` (rare; surface as protocol-level error so the session aborts rather than silently recording corruption)
- Other errors → generic `"internal"` with the error string

### Stdout discipline (hard constraint)
- MCP wire IS stdout. The server MUST NOT print anything to stdout except JSON-RPC frames.
- All logs go to stderr via `log/slog` with `slog.NewTextHandler(os.Stderr, ...)`.
- No banners, no `Println`, no `fmt.Print*` to stdout.
- This is already set up in `main()` (Phase 1 wired stdout redirection); the adapter must not re-redirect.

### Integration test strategy
- Spawn the compiled test binary (via `TestMain` re-exec, same pattern as Phase 4 integration tests, OR via `go build` at test setup).
- Pipe a canned `initialize` → `notifications/initialized` → `tools/list` → `tools/call register` → `tools/call resolve` sequence via stdin.
- Assert stdout is strictly newline-framed compact JSON-RPC (parse every line as JSON; fail if any non-JSON byte appears).
- Re-exec pattern preferred over go-build-at-setup (test-local, no dependency on Phase 6 kong wiring).

### Non-goals
- No resource prompts, no MCP completions, no server-side cancellation — just `register` and `resolve` tools for v1.
- No Streamable HTTP transport — stdio only.
- No session tracking via MCP `Mcp-Session-Id` (that's HTTP-only per spec) — OwnerToken is the entire session-link story.
- No config file, no flags beyond what `mcp-chain serve` needs (which is essentially nothing — state path resolves via Phase 3 `statepath.Resolve`).

</decisions>

<code_context>
## Existing Code Insights

- Phase 1: `internal/cli/stubs.go` has `ServeCmd.Run` stubbed — exits 3 with "not implemented"
- Phase 1: `main()` in `cmd/mcp-chain/main.go` redirects `log.SetOutput(os.Stderr)` + `slog.SetDefault(slog.NewTextHandler(os.Stderr, ...))` as first statements
- Phase 4: `internal/store` public API — `Open(path string) (*Store, error)`, `Register(ownerToken, condition string) (id string, err error)`, `Resolve(id, token string, force bool) error` (exact signature TBD from Phase 4 source), sentinel errors `ErrUnknownID`, `ErrAlreadyResolved`, `ErrNotOwner`
- Phase 3: `statepath.Resolve() (string, error)` — returns the state-file path
- go.mod: Go 1.23.8 (flock v0.12.1 is the ceiling until toolchain bump)

</code_context>

<specifics>
## Specific Ideas

No specific user requirements — discuss phase skipped. Refer to success criteria + CLAUDE.md stack guidance.

</specifics>

<deferred>
## Deferred Ideas

- MCP `resource` and `prompt` surfaces — not in v1 scope
- Streamable HTTP transport — not in v1 scope
- Per-tool authorization / rate limiting — not in v1 scope
- Graceful shutdown beyond EOF on stdin — not in v1 scope
- Server telemetry / OTel — not in v1 scope

</deferred>
