# Phase 5: MCP Server Adapter - Research

**Researched:** 2026-04-23
**Domain:** MCP stdio server adapter over `modelcontextprotocol/go-sdk` v1.5.0
**Confidence:** HIGH

## Summary

Phase 5 is a thin adapter in `internal/mcpserver/` that binds `modelcontextprotocol/go-sdk` v1.5.0's `StdioTransport` + `Server` to the Phase-4 `internal/store`. The SDK surface is small and idiomatic: `mcp.NewServer(impl, opts)`, `mcp.AddTool[In, Out](server, tool, handler)`, and `server.Run(ctx, &mcp.StdioTransport{})`. Handlers are typed generics — Input/Output structs with `json` + `jsonschema` tags auto-generate the tool schemas. Two tools (`register`, `resolve`) close over an `ownerToken` captured at `serve` startup and a `*store.Store` handle. Error mapping converts the three store sentinels (`ErrUnknownID`, `ErrAlreadyResolved`, `ErrNotOwner`) into JSON tool-result errors (`IsError: true` with a structured code) so the calling LLM sees a distinguishable reason rather than a JSON-RPC protocol error. Framing is newline-delimited JSON over stdin/stdout — confirmed by reading the SDK's `transport.go` — so the integration test can scan stdout line-by-line and parse each as one JSON-RPC message.

**Primary recommendation:** Build four files (`server.go`, `tools.go`, `owner.go`, `errors.go`) plus two test files (`server_test.go` in-process, `integration_test.go` re-exec). Handler input/output structs live in `tools.go` next to the `AddTool` calls. The whole package should be ≤ 250 LOC excluding tests — there is almost nothing to do beyond glue.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| MCP protocol framing (JSON-RPC 2.0, NDJSON) | go-sdk (`mcp.StdioTransport`) | — | Handled entirely by SDK; we never touch raw bytes. |
| Tool schema generation | go-sdk (reflection on `In`/`Out` struct tags) | — | `jsonschema:"..."` tags drive the advertised schema automatically. |
| Handler dispatch | go-sdk (`Server.Run` loop) | — | We register handlers; SDK routes `tools/call` to them. |
| Business rules (register/resolve semantics) | `internal/store` | `internal/mcpserver` | Store owns state mutation; adapter just translates args. |
| Owner identity | `internal/mcpserver` (`NewOwnerToken`) | `internal/store` (stamps/compares) | Token is a per-process value created here, passed to store. |
| Error code mapping | `internal/mcpserver/errors.go` | — | Only this layer knows about MCP wire codes. Store exposes sentinels only. |
| Logging/diagnostics | stdlib `log/slog` → stderr | — | Already wired in `main()`; adapter MUST NOT re-redirect. |

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-02 | MCP tool `register(condition string) → id string` | §2 tool definitions, `register` handler with `RegisterIn`/`RegisterOut` structs |
| CORE-03 | MCP tool `resolve(id string) → void`; distinct errors for unknown / already-resolved / not-owner | §4 error mapping — three wire codes, matched via `errors.Is` |
| CORE-10 | Tool descriptions ≤ 1 short sentence | §7 token-budget measurement; `Description` field on `mcp.Tool` is the single locus |
| MCP-01 | Use go-sdk v1.5+ with `StdioTransport`; no `net/http` | §1 SDK surface; §5 stdout discipline; smoke test uses `go list -deps ./...` grep |
| MCP-03 | Thin adapter; no MCP types in store | §3 OwnerToken plumbing; grep gate on `internal/store/` |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/modelcontextprotocol/go-sdk` | v1.5.0 (2026-04-07) | MCP protocol, stdio transport, handler routing | [CITED: https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp] Official SDK, stable 1.x, Apache-2.0. Already locked in CLAUDE.md. |
| stdlib `crypto/rand` | Go 1.23 | 128-bit OwnerToken entropy source | [VERIFIED: CONTEXT.md locked decision] Code snippet specified verbatim. |
| stdlib `encoding/hex` | Go 1.23 | 32-char string encoding of token bytes | [VERIFIED: CONTEXT.md locked decision] |
| stdlib `context` | Go 1.23 | `signal.NotifyContext` → `Server.Run(ctx, ...)` for graceful shutdown | [CITED: go-sdk `Server.Run` signature] |
| stdlib `os/signal`, `syscall` | Go 1.23 | SIGINT/SIGTERM → context cancel | Standard Go pattern. |

### Supporting (test-only)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify/require` | v1.11.1 | Fail-fast assertions | Existing project convention (Phase 4 tests). |
| stdlib `os/exec` | — | Integration test: re-exec self with env var dispatch | Existing Phase-4 pattern in `internal/store/integration_test.go`. |
| stdlib `encoding/json` | — | Parse stdout frames in integration test to verify wire cleanliness | Any non-JSON byte on stdout → test fail. |
| stdlib `bufio` | — | `bufio.Scanner` on child stdout to split on `\n` | SDK uses newline-delimited framing (verified). |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `mcp.AddTool` (generics) | `server.AddTools(mcp.ServerTool{...})` (pre-1.0 style) | Generics form is idiomatic in v1.5; advertised as primary in README. Skip. |
| Tool-result error (`IsError: true`) | Protocol-level error (handler returns `err`) | Protocol error aborts the call; tool-result error lets the LLM read the failure reason (`code: not_owner`) and decide. CONTEXT locks the result-error path for the three store sentinels. |
| `google/uuid` for OwnerToken | `crypto/rand` + `hex.EncodeToString` (locked) | CLAUDE.md rules out UUID lib for one use-case. Locked. |
| `CommandTransport` | `StdioTransport` | CommandTransport is client-side (spawning a server); server uses `StdioTransport`. Not applicable. |

**Installation:**
```bash
go get github.com/modelcontextprotocol/go-sdk@v1.5.0
```

**Version verification:** [VERIFIED: pkg.go.dev release metadata 2026-04-23]
- `github.com/modelcontextprotocol/go-sdk` latest release: v1.5.0 published 2026-04-07
- Go 1.23 minimum — project is on 1.23.8, satisfied
- Zero cgo in go-sdk dep graph (verified via module docs; re-verify at planning time with `go list -deps` after `go get`)

## Architecture Patterns

### System Architecture Diagram

```
[Claude Code client]                                 [mcp-chain serve process]
        |                                                      |
        | JSON-RPC over stdio                                  |
        | (newline-delimited)                                  |
        |                                                      |
        v                                                      v
   +----+---------+                                   +--------+----------+
   | stdin pipe   | -----------------------------------> os.Stdin         |
   +--------------+                                   +--------+----------+
                                                               |
                                                               v
                                                   +-----------+-----------+
                                                   | mcp.StdioTransport    |
                                                   | (SDK; NDJSON framing) |
                                                   +-----------+-----------+
                                                               |
                                                               v
                                                   +-----------+-----------+
                                                   | mcp.Server.Run loop   |
                                                   | (dispatch by method)  |
                                                   +-----+-----------+-----+
                                                         |           |
                                 tools/call register     |           |  tools/call resolve
                                                         v           v
                                              +----------+--+    +---+--------------+
                                              | registerH() |    | resolveHandler() |
                                              | (tools.go)  |    | (tools.go)       |
                                              +------+------+    +------+-----------+
                                                     |                  |
                                                     |  closures over:  |
                                                     |    store *Store  |
                                                     |    ownerToken    |
                                                     v                  v
                                                   +-+------------------+-+
                                                   |    internal/store    |
                                                   |  Register / Resolve  |
                                                   +---+--------------+---+
                                                       | ErrUnknownID |
                                                       | ErrAlready.. |
                                                       | ErrNotOwner  |
                                                       v              v
                                                   +---+--------------+---+
                                                   | errors.go: mapErr()  |
                                                   |  →  &CallToolResult{ |
                                                   |      IsError: true,  |
                                                   |      Content: {code} |
                                                   |    }                 |
                                                   +---+--------------+---+
                                                                     |
   +--------------+                                   +--------+----------+
   | stdout pipe  | <----------------------------------- os.Stdout        |
   +----+---------+                                   +--------+----------+
        |                                                      |
   +----+--------------+                             +---------+---------+
   | stderr pipe (logs)| <--------------------------- slog → os.Stderr   |
   +-------------------+                             +-------------------+
```

Key data-flow invariants:
1. EVERY byte out of `os.Stdout` is one complete JSON-RPC frame followed by `\n`. SDK owns this.
2. Logs flow exclusively to `os.Stderr` via the `slog` handler installed in `main()` (Phase 1). Adapter MUST NOT call `fmt.Print*` or `log.Print*`.
3. Handler return tuple: `(*mcp.CallToolResult, Out, error)`. On business-logic errors we set the result's `IsError=true` with a structured code and return `nil` for the third value — this emits a valid JSON-RPC response (not a JSON-RPC error object) that the LLM client can inspect.

### Recommended Project Structure
```
internal/mcpserver/
├── server.go            # Run(ctx, store, ownerToken) error — wires NewServer + AddTool + StdioTransport
├── tools.go             # Handler funcs + In/Out structs + Description constants
├── owner.go             # NewOwnerToken() (string, error)
├── errors.go            # mapStoreError(err) *mcp.CallToolResult  (code, message)
├── owner_test.go        # TestNewOwnerToken_HexLength, _Uniqueness
├── tools_test.go        # TestToolDescriptionsUnderBudget  (static analysis — no SDK needed)
├── server_test.go       # In-process handler roundtrip using an in-memory transport (if SDK exposes) or direct handler call
└── integration_test.go  # Re-exec subprocess; pipe real JSON-RPC; assert wire cleanliness
```

### Pattern 1: Handler registration with typed generics
**What:** Each MCP tool = input struct, output struct, handler closure.
**When to use:** Always for v1.5+ SDK; the non-generic `AddTools` path is deprecated.
**Example:**
```go
// Source: https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#AddTool
//         [CITED: pkg.go.dev, README example]
type RegisterIn struct {
    Condition string `json:"condition" jsonschema:"when this id is considered resolvable"`
}
type RegisterOut struct {
    ID string `json:"id"`
}

mcp.AddTool(server, &mcp.Tool{
    Name:        "register",
    Description: registerDescription, // const string — ≤ 1 sentence
}, func(ctx context.Context, req *mcp.CallToolRequest, in RegisterIn) (*mcp.CallToolResult, RegisterOut, error) {
    id, err := st.Register(ownerToken, in.Condition)
    if err != nil {
        return mapStoreError(err), RegisterOut{}, nil // tool-result error
    }
    return nil, RegisterOut{ID: id}, nil // SDK auto-serialises Out into Content
})
```

Notes verified against SDK:
- Handler signature is `func(ctx, *CallToolRequest, In) (*CallToolResult, Out, error)` — [VERIFIED: pkg.go.dev `ToolHandlerFor[In, Out]`]
- Returning non-nil `*CallToolResult` takes precedence; SDK will use it verbatim [CITED: pkg.go.dev CallToolResult doc]
- Returning `nil` for `*CallToolResult` and a populated `Out` struct auto-wraps `Out` as JSON text content [CITED: "if Content is unset it will be populated with JSON text content" — SDK doc]
- Input validation (wrong type, missing required field) is done by the SDK BEFORE the handler runs — the handler receives a validated `In` value.

### Pattern 2: Server construction + Run
```go
// Source: https://github.com/modelcontextprotocol/go-sdk README
//         [CITED: verbatim from README hello-world]
func Run(ctx context.Context, st *store.Store, ownerToken string) error {
    impl := &mcp.Implementation{Name: "mcp-chain", Version: version}
    server := mcp.NewServer(impl, nil /* default ServerOptions */)

    registerTools(server, st, ownerToken) // calls AddTool twice

    return server.Run(ctx, &mcp.StdioTransport{})
}
```
- `server.Run` blocks until the transport's underlying connection closes (stdin EOF) or `ctx` is cancelled [CITED: pkg.go.dev Server.Run signature takes `context.Context, Transport`].
- `&mcp.StdioTransport{}` is a zero-value — no config needed; it wraps `os.Stdin`/`os.Stdout` internally [VERIFIED: `mcp/transport.go` `StdioTransport.Connect` impl returns `newIOConn(rwc{os.Stdin, nopCloserWriter{os.Stdout}})`].
- Version string should be the `-ldflags` value from `cmd/mcp-chain/main.go`. Planner: pass `version` through a constructor argument (not a global) so it's test-friendly.

### Pattern 3: CLI wiring — replace stub
In `internal/cli/stubs.go`, `ServeCmd.Run` (currently exits 3) becomes:

```go
func (c *ServeCmd) Run() error {
    path, err := statepath.Resolve()
    if err != nil { return err }
    st, err := store.Open(path)
    if err != nil { return err }
    token, err := mcpserver.NewOwnerToken()
    if err != nil { return err }

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()
    return mcpserver.Run(ctx, st, token)
}
```
Planner note: `ServeCmd` must be split — keep the struct in `internal/cli` but move `Run`'s imports (`mcpserver`, `statepath`, `store`, `signal`, `syscall`) there. Alternatively, the `Run` can be a two-liner delegating to `mcpserver.RunFromCLI()` if we want to keep `internal/cli` dependency-free. CONTEXT does not mandate; recommend the direct wiring above for simplicity.

### Anti-Patterns to Avoid
- **Calling `fmt.Printf`/`log.Printf` from a handler:** corrupts stdout. Use `slog.Info(...)` which already targets stderr.
- **Re-assigning `log.SetOutput` or `slog.SetDefault` in `mcpserver.Run`:** already done in `main()`; do not re-do. Re-doing inside a test with an in-process server creates a data race.
- **Holding a store lock across a handler boundary:** all mutation happens inside `store.Register`/`Resolve`, which own their flock lifecycle per call. Do not cache `*state` across calls in the adapter.
- **Importing `net/http` transitively:** the MCP SDK has Streamable-HTTP code in it, but only if you import `streamable_server.go` symbols. Importing only `mcp.StdioTransport` + `mcp.AddTool` + `mcp.NewServer` should not pull `net/http`. Planner: verify with `go list -deps ./internal/mcpserver | grep net/http` after implementation — if `net/http` appears, re-architect imports. This is a CI gate per CONTEXT SC #1.
- **Using `StructuredContent` hand-rolling:** let the SDK auto-wrap the `Out` struct. We only hand-build a `*CallToolResult` on the error path.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON-RPC framing | Custom NDJSON scanner on stdin | `mcp.StdioTransport` | Handles batch/non-batch, error framing, EOF semantics. |
| Tool input schema | JSON-Schema literal as a string | `jsonschema:"description"` struct tag | SDK generates schema via reflection; stays in lockstep with Go types. |
| Argument unmarshalling/validation | `json.Unmarshal` + manual required-field check | Typed `In` generic | SDK unmarshals, validates required fields, and calls handler with a ready-to-use struct. |
| Protocol version negotiation | Hardcode `"2025-06-18"` strings | SDK's `initialize` handler | SDK owns the handshake; client sends protocol version, SDK responds with what it supports. |
| Context propagation | Goroutine + channel to pass cancellation | Handler `ctx` arg | SDK passes per-call context automatically. |
| OwnerToken UUID | Pull `google/uuid` dep | `crypto/rand` + `hex.EncodeToString` (16 bytes → 32 chars) | 128 bits of entropy, zero deps, locked in CONTEXT. |
| Error wire format | Custom JSON error envelope | `&mcp.CallToolResult{IsError: true, Content: {TextContent{...}}}` | Standard MCP tool-result-error pattern; LLM client knows how to read it. |

**Key insight:** The entire `internal/mcpserver` package should be ~200 LOC. Anything longer suggests we are re-implementing something the SDK already provides. If you find yourself reading MCP spec documents to code up framing or schema logic, stop and search the SDK first.

## Common Pitfalls

### Pitfall 1: Handler panic crashes the session
**What goes wrong:** A nil-pointer or out-of-bounds panic in the handler propagates up, killing the `Server.Run` loop; the client sees an abrupt stdin close.
**Why it happens:** SDK does not wrap handlers in `recover()` by default (as of v1.5 — [ASSUMED], verify during implementation by reading `server.go` handler dispatch).
**How to avoid:** Handlers should never panic — they do small, fallible I/O (store calls) and trivial string copies. If a panic risk emerges, wrap with `defer func() { if r := recover(); r != nil { ... } }()` at the top of each handler and return a tool-result error with `code: "internal"`.
**Warning signs:** Integration test where the second `tools/call` hangs after the first handler call does something unexpected.

### Pitfall 2: Writes to stdout outside the SDK
**What goes wrong:** A stray `fmt.Println("ready")` anywhere in the call graph gets interleaved with JSON-RPC frames; the client's parser chokes on the unexpected bytes.
**Why it happens:** Classic Go habit — "temporary" debug prints left in code; also, some third-party libs log to stdout by default.
**How to avoid:** CI gate: integration test reads entire child stdout, JSON-decodes each `\n`-framed line; any line that fails to parse → fail. `slog.SetDefault` already redirects to stderr in `main()`. Do not `fmt.Print*` anywhere in `internal/mcpserver/`.
**Warning signs:** Client reports "invalid JSON at position N" during handshake.

### Pitfall 3: Version string drift
**What goes wrong:** `Implementation.Version` hardcoded to `"0.1.0"` ships with a release tagged `v0.2.0`.
**Why it happens:** Two sources of truth.
**How to avoid:** Thread the `var version string` from `cmd/mcp-chain/main.go` through `mcpserver.Run`. A single `-ldflags="-X main.version=..."` sets both.
**Warning signs:** MCP `initialize` response reports a stale version.

### Pitfall 4: OwnerToken regenerated per-handler
**What goes wrong:** Someone writes `NewOwnerToken()` inside a handler instead of once at `serve` startup; every `register` stamps a different token; `resolve` always returns `ErrNotOwner`.
**Why it happens:** Misunderstanding that the token is a per-*process* identity, not per-call.
**How to avoid:** `NewOwnerToken()` is called exactly once in `ServeCmd.Run`; the string is captured by the handler closures. A test (`TestRegisterHandler_StampsOwnerToken`) verifies successive registers from the same server stamp the same token.
**Warning signs:** `resolve` returns `not_owner` for IDs the same process just registered.

### Pitfall 5: Concurrent handler execution races in tests
**What goes wrong:** Integration test sends `register` + `resolve` in parallel using separate `go` routines; the SDK may dispatch handlers concurrently. Store is safe (flock + sync.Mutex), but test assertions that assume ordering break.
**Why it happens:** `server.Run`'s dispatch model is not documented as strictly serialised in v1.5.
**How to avoid:** Tests send requests *sequentially* on stdin (write msg N, read response N, then write N+1). For real concurrent clients, rely on store's safety.
**Warning signs:** Flaky integration tests where the second response sometimes arrives before the first.

### Pitfall 6: `StdioTransport` consumes os.Stdin
**What goes wrong:** Tests that instantiate `mcpserver.Run` in-process block forever waiting for stdin.
**Why it happens:** `StdioTransport` reads from `os.Stdin` — not a pluggable reader.
**How to avoid:** In-process tests (`server_test.go`) cannot use `StdioTransport`. Two options:
  - (a) Skip full-stack in-process tests; write **handler-level** unit tests that call `registerHandler(ctx, nil, RegisterIn{...})` directly, bypassing the transport.
  - (b) Use the SDK's `mcp.NewInMemoryTransports()` or equivalent paired-client/server transport if available (search SDK for `InMemoryTransport` or `LoopbackTransport`). [ASSUMED] v1.5 has an in-memory transport; confirm at plan time by checking `pkg.go.dev` for symbols matching `InMemory*`/`Loopback*`. If not, fall back to (a).
  - (c) Re-exec pattern (like Phase 4) — already required for `integration_test.go`; this is the authoritative end-to-end path.
**Warning signs:** `go test` hangs; `server_test.go` tries to feed stdin and deadlocks.

### Pitfall 7: Grep gate false negatives
**What goes wrong:** `grep -r 'mcp' internal/store/` matches `mcp` inside a comment or doc string.
**Why it happens:** Overly broad grep.
**How to avoid:** The grep gate is a belt-and-suspenders check; the *primary* enforcement is Go's import analysis. Use `go list -f '{{.Imports}}' ./internal/store/...` and assert none of them contain `modelcontextprotocol`. Ship both checks; the import check is authoritative, the grep is defense-in-depth against copy-pasted struct definitions.

### Pitfall 8: net/http leaks into the binary via the SDK
**What goes wrong:** The SDK ships a `streamable_server.go` that uses `net/http`; importing it transitively bloats binary by ~1.5 MB.
**Why it happens:** Importing `github.com/modelcontextprotocol/go-sdk/mcp` may pull the HTTP path depending on init() side effects.
**How to avoid:** Run `go list -deps ./cmd/mcp-chain/... | grep -E '^net/http$'` after implementation. If it appears, investigate which file pulls it; if the SDK's main package has unavoidable HTTP deps (an init-time pull), file this as a known issue and ensure the binary size still fits DIST-03's 15 MB budget. [ASSUMED: currently no transitive `net/http` for the stdio-only path; verify.]

## Runtime State Inventory

> Not applicable. This is a greenfield feature phase — no renames, refactors, or migrations. Section intentionally omitted.

## Code Examples

### Example 1: `NewOwnerToken` (verbatim from CONTEXT)
```go
// Source: /home/alpine/mcp-chain/.planning/phases/05-mcp-server-adapter/05-CONTEXT.md
//         [VERIFIED: locked decision]
package mcpserver

import (
    "crypto/rand"
    "encoding/hex"
)

// NewOwnerToken returns a 32-character hex string carrying 128 bits of
// crypto/rand entropy. Called exactly once per `mcp-chain serve` process.
func NewOwnerToken() (string, error) {
    var buf [16]byte
    if _, err := rand.Read(buf[:]); err != nil {
        return "", err
    }
    return hex.EncodeToString(buf[:]), nil
}
```

### Example 2: Tool-result error helper
```go
// Source: [CITED: pkg.go.dev mcp.CallToolResult, mcp.TextContent]
package mcpserver

import (
    "encoding/json"
    "errors"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/anthropics/mcp-chain/internal/store"
)

// errorBody is the JSON payload clients read from Content[0].Text when
// IsError is true. `code` is machine-readable, `message` human-readable.
type errorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func toolError(code, message string) *mcp.CallToolResult {
    payload, _ := json.Marshal(errorBody{Code: code, Message: message})
    return &mcp.CallToolResult{
        IsError: true,
        Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
    }
}

func mapStoreError(err error) *mcp.CallToolResult {
    switch {
    case errors.Is(err, store.ErrUnknownID):
        return toolError("unknown_id", "unknown lock id")
    case errors.Is(err, store.ErrAlreadyResolved):
        return toolError("already_resolved", "lock is already resolved")
    case errors.Is(err, store.ErrNotOwner):
        return toolError("not_owner", "this session did not register this lock")
    default:
        // Includes ErrSchemaVersion and any unexpected case. We still
        // return a tool-result error rather than a protocol error so
        // the session stays open — operator sees the message via CLI logs.
        return toolError("internal", err.Error())
    }
}
```

Notes: CONTEXT suggests routing `ErrSchemaVersion` as a *protocol-level* error to abort the session. Planner can choose either; protocol-level means the handler returns `(nil, out, err)` instead of a result. The recommended safer default is tool-result error with code `schema_error`, plus a `slog.Error` log to stderr — the operator sees it, but the session isn't yanked out from under them mid-workflow. Planner: decide and document.

### Example 3: Register handler wiring
```go
// Source: assembled from pkg.go.dev/AddTool and store.go Register signature
//         [VERIFIED against internal/store/store.go line 67]
package mcpserver

import (
    "context"

    "github.com/modelcontextprotocol/go-sdk/mcp"

    "github.com/anthropics/mcp-chain/internal/store"
)

const (
    registerDescription = "Register a coordination lock; returns a short id."
    resolveDescription  = "Resolve a lock id you previously registered."
)

type RegisterIn struct {
    Condition string `json:"condition" jsonschema:"when this id is considered resolvable"`
}
type RegisterOut struct {
    ID string `json:"id"`
}

type ResolveIn struct {
    ID string `json:"id" jsonschema:"the short id returned by register"`
}
type ResolveOut struct{} // empty — resolve is void

func registerTools(server *mcp.Server, st *store.Store, ownerToken string) {
    mcp.AddTool(server,
        &mcp.Tool{Name: "register", Description: registerDescription},
        func(ctx context.Context, req *mcp.CallToolRequest, in RegisterIn) (*mcp.CallToolResult, RegisterOut, error) {
            id, err := st.Register(ownerToken, in.Condition)
            if err != nil {
                return mapStoreError(err), RegisterOut{}, nil
            }
            return nil, RegisterOut{ID: id}, nil
        },
    )

    mcp.AddTool(server,
        &mcp.Tool{Name: "resolve", Description: resolveDescription},
        func(ctx context.Context, req *mcp.CallToolRequest, in ResolveIn) (*mcp.CallToolResult, ResolveOut, error) {
            if err := st.Resolve(in.ID, ownerToken, store.ResolveOptions{Force: false}); err != nil {
                return mapStoreError(err), ResolveOut{}, nil
            }
            return nil, ResolveOut{}, nil
        },
    )
}
```

### Example 4: Integration test skeleton (re-exec pattern)
```go
//go:build integration
//
// Source: adapted from /home/alpine/mcp-chain/internal/store/integration_test.go
//         [VERIFIED against Phase-4 re-exec pattern]
package mcpserver_test

import (
    "bufio"
    "context"
    "encoding/json"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"

    "github.com/stretchr/testify/require"

    "github.com/anthropics/mcp-chain/internal/mcpserver"
    "github.com/anthropics/mcp-chain/internal/store"
)

const childEnvVar = "MCP_CHAIN_MCPSERVER_TEST_CHILD"

func TestMain(m *testing.M) {
    if os.Getenv(childEnvVar) == "serve" {
        runChild()
        return
    }
    os.Exit(m.Run())
}

func runChild() {
    path := os.Getenv("MCP_CHAIN_STATE_PATH")
    st, _ := store.Open(path)
    tok, _ := mcpserver.NewOwnerToken()
    _ = mcpserver.Run(context.Background(), st, tok)
    os.Exit(0)
}

func TestServe_StdioFullHandshake(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "state.json")

    exe, err := os.Executable()
    require.NoError(t, err)
    cmd := exec.Command(exe)
    cmd.Env = append(os.Environ(),
        childEnvVar+"=serve",
        "MCP_CHAIN_STATE_PATH="+path,
    )
    stdin, err := cmd.StdinPipe(); require.NoError(t, err)
    stdout, err := cmd.StdoutPipe(); require.NoError(t, err)
    cmd.Stderr = os.Stderr
    require.NoError(t, cmd.Start())
    t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })

    send := func(msg string) { _, err := stdin.Write([]byte(msg + "\n")); require.NoError(t, err) }
    scanner := bufio.NewScanner(stdout)
    recv := func() map[string]any {
        require.True(t, scanner.Scan(), "expected JSON-RPC frame")
        var m map[string]any
        require.NoError(t, json.Unmarshal(scanner.Bytes(), &m), "stdout must be valid JSON")
        return m
    }

    // 1. initialize
    send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.0"}}}`)
    r := recv()
    require.Equal(t, float64(1), r["id"])
    // 2. notifications/initialized (no response expected)
    send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
    // 3. tools/list
    send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
    _ = recv()
    // 4. register
    send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"register","arguments":{"condition":"test"}}}`)
    _ = recv()
    // 5. resolve — capture id from (4); omitted for brevity
    // ...
    _ = stdin.Close()
    _ = cmd.Wait()
}

// TestServe_StdoutIsPureJSONRPC — drain stdout after a full session and
// assert every \n-framed chunk is valid JSON. No partial lines, no
// non-JSON bytes.
func TestServe_StdoutIsPureJSONRPC(t *testing.T) {
    // ... similar setup; after closing stdin and Wait(), read remaining
    // stdout and scan every line through json.Unmarshal.
    _ = io.EOF // suppress unused
    _ = time.Second
}
```

Planner: the "capture id from register response" step parses `result.content[0].text` (JSON-stringified `RegisterOut`). Document this in VALIDATION.md. The SDK wraps `Out` as JSON text content per its Content auto-wrapping behavior.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `server.AddTools(mcp.ServerTool{...})` literal | `mcp.AddTool[In, Out](s, t, h)` generic | go-sdk v1.0+ | Schema generation is automatic; less boilerplate. Only use the generic form. |
| Length-prefixed JSON-RPC framing | Newline-delimited JSON (NDJSON) | MCP spec 2025-06-18 | [VERIFIED via SDK `transport.go`] stdio transport uses `\n` separation. Scanner-friendly. |
| `metoro-io/mcp-golang` | `modelcontextprotocol/go-sdk` | 2026-Q1 (official SDK stable) | CLAUDE.md already mandates this. |
| `github.com/pkg/errors` wrapping | stdlib `errors.Is` + `fmt.Errorf("...%w", err)` | Go 1.13 | All error mapping uses `errors.Is`. |

**Deprecated/outdated:**
- JSON-RPC batching on stdio: disabled in MCP spec 2025-06-18 and later. SDK still parses batches but spec discourages. Non-issue for mcp-chain — single-call flow.
- `CommandTransport` for server-side: not applicable; that's a client-side transport for spawning a server process.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | v1.5 SDK does not wrap handlers in `recover()` | Pitfall 1 | A panic kills the session silently. Mitigate by never panicking in handlers (small, well-typed code) and/or adding a defer-recover at handler top. Verify during plan-time review by grepping SDK for `recover(` in `server.go`. |
| A2 | Stdio-only code path does not transitively import `net/http` | Pitfall 8, SC #1 | If wrong, binary bloats and CI grep gate fails. Measure after implementation with `go list -deps`; if `net/http` appears, escalate. |
| A3 | SDK exposes an in-memory/loopback transport suitable for `server_test.go` | Pitfall 6 option (b) | If wrong, fall back to option (a) direct handler unit tests. Verify at plan time. |
| A4 | Protocol version string in `initialize` does not need to be hardcoded in our code | Pattern 2 | SDK owns handshake. If wrong, add `ProtocolVersion` to `Implementation` or `ServerOptions`. Verify by running the integration test against a known-good client. |
| A5 | Token-budget measurement via byte-length proxy (`len(desc)/4 ≤ 40`) is acceptable for CI | §7 | If the proxy over/under-estimates real tokens, Phase 9 CI may pass descriptions that actually violate CORE-10. Acceptable first-pass; refine in Phase 9 if needed. |

## Open Questions

1. **In-memory transport availability for unit tests**
   - What we know: SDK exposes `StdioTransport` and `CommandTransport` on its public surface. README shows only these two.
   - What's unclear: Whether `mcp.NewInMemoryTransports()` or similar exists for paired in-process client/server testing.
   - Recommendation: Planner opens `pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp` and grep for `Memory`/`Loopback`/`Pipe` during Wave 0. If present, use it in `server_test.go`. If absent, use direct handler unit tests and rely on `integration_test.go` for the full-stack path. Either way, phase delivery is not blocked.

2. **Handler concurrency model**
   - What we know: Store is process-safe and goroutine-safe.
   - What's unclear: Does v1.5 `Server.Run` dispatch handlers serially or concurrently on a single session?
   - Recommendation: Test both modes. Since store is safe either way, the correctness doesn't hinge on this, but test assertions must not assume strict ordering across concurrent calls. Document in VALIDATION.md.

3. **`ErrSchemaVersion` wire routing**
   - What we know: CONTEXT floats two options — tool-result error vs. protocol-level error.
   - What's unclear: Product preference.
   - Recommendation: Default to tool-result error (code `"schema_error"`) so the session stays alive. Operator sees stderr log. If the user feels strongly, flip to protocol error in a future phase; it's a one-line change.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | Required | ≥1.23 (project is 1.23.8) | — |
| `modelcontextprotocol/go-sdk` v1.5.0 | MCP server | Pending `go get` | — | None — this is the locked choice |
| Internet access | `go get` first time | Assumed | — | If offline, vendor the dep |

**Missing dependencies with no fallback:**
- None at research time. First `go get github.com/modelcontextprotocol/go-sdk@v1.5.0` must succeed during plan execution.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `testify/require` v1.11.1 |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./internal/mcpserver/... -count=1 -timeout 30s` |
| Full suite command | `go test -race -tags=integration -count=1 -timeout 120s ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| CORE-02 | `register(condition) → id` succeeds, returns non-empty word-ID, persists to state | unit (handler) | `go test ./internal/mcpserver/ -run TestRegisterHandler_HappyPath -count=1` | Wave 0 |
| CORE-02 | `register` stamps OwnerToken from process-scope token (not per-call) | unit | `go test ./internal/mcpserver/ -run TestRegisterHandler_StampsOwnerToken -count=1` | Wave 0 |
| CORE-03 | `resolve` succeeds for owner match | unit | `go test ./internal/mcpserver/ -run TestResolveHandler_OwnerOk -count=1` | Wave 0 |
| CORE-03 | `resolve` returns wire code `"unknown_id"` (tool-result error) for unknown id | unit | `go test ./internal/mcpserver/ -run TestResolveHandler_UnknownID -count=1` | Wave 0 |
| CORE-03 | `resolve` returns wire code `"already_resolved"` on second resolve | unit | `go test ./internal/mcpserver/ -run TestResolveHandler_AlreadyResolved -count=1` | Wave 0 |
| CORE-03 | `resolve` returns wire code `"not_owner"` when token mismatches (distinct from the other two codes — SC #2) | unit | `go test ./internal/mcpserver/ -run TestResolveHandler_NotOwner -count=1` | Wave 0 |
| CORE-10 | Every tool description has ≤ 40 tokens (byte-proxy: `len(desc) ≤ 160`) | unit (static) | `go test ./internal/mcpserver/ -run TestToolDescriptionsUnderBudget -count=1` | Wave 0 |
| CORE-10 | Total tool-list advertises ≤ 200 tokens (byte-proxy: sum `len(desc)` ≤ 800) | unit (static) | `go test ./internal/mcpserver/ -run TestToolListUnderBudget -count=1` | Wave 0 |
| MCP-01 | Binary does not import `net/http` | smoke (build-gated) | `! go list -deps ./cmd/mcp-chain/... \| grep -qx net/http` | Wave 0 script |
| MCP-01 | Full MCP handshake (initialize → tools/list → register → resolve) completes over stdio | integration (re-exec) | `go test -tags=integration ./internal/mcpserver/ -run TestServe_StdioFullHandshake -count=1 -timeout 30s` | Wave 0 |
| MCP-02 | Stdout contains ONLY newline-framed JSON (every line parses as JSON-RPC; no banners) | integration | `go test -tags=integration ./internal/mcpserver/ -run TestServe_StdoutIsPureJSONRPC -count=1 -timeout 30s` | Wave 0 |
| MCP-03 | `internal/store` does not import any `modelcontextprotocol` or `jsonrpc` symbol | smoke | `! go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' ./internal/store/... \| grep -E 'modelcontextprotocol\|jsonrpc'` | Wave 0 script |
| MCP-03 | Grep gate: no `mcp|MCP|jsonrpc|modelcontextprotocol` identifiers in `internal/store/` (doc comments exempted) | smoke | `! grep -rE '\b(mcp\|jsonrpc\|modelcontextprotocol)\b' internal/store/ --include='*.go' \| grep -v '^[^:]*:[[:space:]]*//'` | Wave 0 script |
| SC #2 | `NewOwnerToken` returns exactly 32 hex chars, cryptographically unique | unit | `go test ./internal/mcpserver/ -run TestNewOwnerToken -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/mcpserver/... -count=1 -timeout 30s` (quick — unit tests only; <5s)
- **Per wave merge:** `go test -race -tags=integration ./... -count=1 -timeout 120s` (includes integration + store + idgen + statepath)
- **Phase gate:** Full suite green + grep/import gates pass before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/mcpserver/` directory + four source files (new package; does not exist yet)
- [ ] `internal/mcpserver/server_test.go` — covers CORE-02, CORE-03 handler-level
- [ ] `internal/mcpserver/owner_test.go` — covers SC #2
- [ ] `internal/mcpserver/tools_test.go` — covers CORE-10 description budget
- [ ] `internal/mcpserver/integration_test.go` — covers MCP-01 end-to-end, MCP-02 stdout discipline (build tag `integration`)
- [ ] `scripts/check-store-isolation.sh` — shell gate for MCP-03 grep + import audit (can live in Phase 9 CI or run ad-hoc; recommend repo script now)
- [ ] `go get github.com/modelcontextprotocol/go-sdk@v1.5.0` — adds dep; first task in Wave 1
- [ ] Update `go.mod` / `go.sum` accordingly
- [ ] Replace `ServeCmd.Run` body in `internal/cli/stubs.go` — remove the `os.Exit(3)` stub

## Project Constraints (from CLAUDE.md)

- **Stack discipline:** pure-Go only; no cgo. Pulling `modelcontextprotocol/go-sdk@v1.5.0` must not introduce cgo transitively. [VERIFIED: SDK docs]
- **Binary size:** ≤15 MB stripped — CI gate in DIST-03. Phase 5 adds one dep; verify post-implementation with `go build -ldflags="-s -w" && ls -l`.
- **Startup time:** ≤100 ms. `mcp-chain serve` startup is: `kong.Parse` + `statepath.Resolve` + `store.Open` + `NewOwnerToken` + `mcp.NewServer` + `AddTool`×2 + first stdin read. All O(µs) in Go.
- **Token budget:** tool descriptions ≤ 1 short sentence, total tool-list ≤ 200 tokens — CORE-10 enforcement via `TestToolDescriptionsUnderBudget`.
- **Stdout discipline:** MCP-02 — stdout is MCP wire only. `log/slog` → stderr (already wired in main). Adapter code must not `fmt.Print*`.
- **GSD workflow:** use `/gsd-execute-phase` for planned work. No direct Edit/Write outside a GSD flow.

## Security Domain

> `security_enforcement` is not explicitly configured in `.planning/config.json`; treating as enabled per default.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | yes (session-link) | `OwnerToken` — 128-bit `crypto/rand`, constant-time compare in store (already Phase 4) |
| V3 Session Management | partial | No cross-process session concept; process identity == OwnerToken. Sessions end on stdin EOF. |
| V4 Access Control | yes | Only the process that registered an ID can resolve it (via OwnerToken match). CLI `--force` is the escape hatch, not exposed here. |
| V5 Input Validation | yes | SDK validates JSON-RPC input against tool's auto-generated schema. `Condition`/`ID` are plain strings — no structured parsing risk. |
| V6 Cryptography | yes | Use `crypto/rand` (not `math/rand`). 128 bits = 2^128 guess space — brute-force infeasible. Constant-time compare (`subtle.ConstantTimeCompare`) in store. Never log the token. |
| V7 Error Handling | yes | Tool-result errors surface `code` + `message` — message must not leak OwnerToken bytes or internal file paths. `internal` fallback surfaces `err.Error()` — audit store sentinel wrappings to confirm no path leakage. |
| V8 Data Protection | yes | State file at `0600`, parent dir `0700` (Phase 3 already). No secrets in memory beyond the 32-char hex token. |
| V12 Files | yes | File I/O is Phase 4's job; adapter does not touch the filesystem. |

### Known Threat Patterns for MCP stdio + single-process Go

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Forged `resolve` from a different process/session | Spoofing | OwnerToken constant-time match (Phase 4 implements; Phase 5 plumbs). |
| Information leak via error message | Information disclosure | Structured `errorBody{Code, Message}` — never embed stack traces, paths, or tokens in `Message`. |
| Handler panic crashes session | Denial of service | Handlers do minimal work; small attack surface. Optional defer-recover if anxiety justifies it. |
| Stdout pollution (non-JSON) | Protocol confusion | Integration test scans every stdout line; any non-JSON fails. |
| Malicious client sends giant `condition` string | Resource exhaustion | Out of scope for v1 — single-user local tool. Store writes the string verbatim; no size cap. Phase 9 CI may add a reasonable limit. |

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev: modelcontextprotocol/go-sdk/mcp](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — `NewServer`, `AddTool`, `Server.Run`, `StdioTransport`, `Tool`, `CallToolResult`, `TextContent`, `Implementation`, `ServerOptions` signatures
- [pkg.go.dev: AddTool detail page](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#AddTool) — `ToolHandlerFor[In, Out]` handler signature, structured output auto-wrap behavior
- [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — README hello-world example, v1.5.0 release date (2026-04-07)
- [github.com/modelcontextprotocol/go-sdk/mcp/transport.go (StdioTransport.Connect impl)](https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/transport.go) — confirms `os.Stdin`/`os.Stdout` wiring, NDJSON (`\n` delimited) framing, no flush
- `/home/alpine/mcp-chain/internal/store/store.go` — exact `Register(ownerToken, condition string) (string, error)` and `Resolve(id, ownerToken string, opts ResolveOptions) error` signatures
- `/home/alpine/mcp-chain/internal/store/errors.go` — exact sentinel error names and semantics
- `/home/alpine/mcp-chain/internal/store/integration_test.go` — re-exec pattern to mimic
- `/home/alpine/mcp-chain/cmd/mcp-chain/main.go` — confirmed `log.SetOutput(os.Stderr)` + `slog.SetDefault(... os.Stderr ...)` is in place as first statements
- `/home/alpine/mcp-chain/internal/cli/stubs.go` — `ServeCmd.Run` stub location (lines 22-27)
- `/home/alpine/mcp-chain/internal/statepath/resolve.go` line 50 — `func Resolve() (string, error)`
- `/home/alpine/mcp-chain/CLAUDE.md` — stack constraints (pure Go, terse descriptions, stdio-only)

### Secondary (MEDIUM confidence)
- WebFetch extractions of pkg.go.dev pages — cross-verified with the README example and the `transport.go` source

### Tertiary (LOW confidence)
- [ASSUMED] SDK's internal panic handling (Pitfall 1, A1) — verify at plan time by reading `mcp/server.go` dispatch code
- [ASSUMED] In-memory transport existence (A3) — verify at plan time
- [ASSUMED] No `net/http` in stdio code path (A2) — verify post-implementation with `go list -deps`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — v1.5.0 signatures verified via pkg.go.dev and transport.go source
- Architecture: HIGH — handler pattern verified against README + pkg.go.dev example; wiring follows locked CONTEXT decisions
- Pitfalls: MEDIUM — three pitfalls tagged [ASSUMED]; most grounded in SDK source or stdlib behavior

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (SDK is stable 1.x; 30 days is reasonable)
