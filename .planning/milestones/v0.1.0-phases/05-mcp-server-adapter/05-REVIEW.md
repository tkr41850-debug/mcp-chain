---
phase: 05-mcp-server-adapter
reviewed: 2026-04-23T00:00:00Z
depth: deep
files_reviewed: 10
files_reviewed_list:
  - internal/mcpserver/owner.go
  - internal/mcpserver/owner_test.go
  - internal/mcpserver/tools.go
  - internal/mcpserver/tools_test.go
  - internal/mcpserver/errors.go
  - internal/mcpserver/server.go
  - internal/mcpserver/server_test.go
  - internal/mcpserver/integration_test.go
  - internal/cli/stubs.go
  - internal/cli/stubs_test.go
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
verdict: APPROVED
---

# Phase 5 Plan 01: Code Review Report

**Reviewed:** 2026-04-23
**Depth:** deep (cross-file, SC-aligned)
**Files Reviewed:** 10 source + `go.mod` / `go.sum`
**Status:** issues_found (non-blocking — all Warnings are minor quality issues)
**Verdict:** APPROVED

## Summary

Phase 5 delivers a genuinely thin MCP stdio adapter (~190 LOC of non-test source) that faithfully realizes the locked CONTEXT. Every hard gate passes:

- **Stdout discipline (SC #4 / MCP-02):** zero `fmt.Print*`, zero `log.Print*`, zero `os.Stdout.Write*` anywhere in `internal/mcpserver/` or `internal/cli/`. Every `fmt.Fprint*` call targets `os.Stderr` explicitly (7 sites audited). The existing `scripts/check-stdout-silence.sh` gate still holds.
- **MCP-01 amended (no `net/http` in our code):** `grep -rnE '"net/http"' internal/ cmd/` returns empty. Any transitive SDK pull is accepted per the 2026-04-24 amendment.
- **Store isolation (MCP-03):** `git log HEAD~10..HEAD -- internal/store/` is empty — Phase 5 did not touch Phase 4's store. `grep -rn 'mcp\|jsonrpc\|modelcontextprotocol' internal/store/ --include='*.go'` returns empty. `crypto/subtle.ConstantTimeCompare` still in place at `internal/store/store.go:106`. Phase 4 guarantees intact.
- **SDK import isolation:** only `internal/mcpserver/*.go` imports `github.com/modelcontextprotocol/go-sdk/mcp` (errors.go, server.go, server_test.go). No leakage.
- **OwnerToken (SC #2):** `crypto/rand.Read` on a `[16]byte` + `hex.EncodeToString` → 32-char lowercase hex. `math/rand` is not imported anywhere. Per-process generation (called once in `ServeCmd.Run`), captured by handler closures — Pitfall 4 cleanly avoided.
- **Distinct wire codes (SC #2 / CORE-03):** `unknown_id`, `already_resolved`, `not_owner`, `schema_error`, `internal` all emitted as separate strings via `mapStoreError`. Each of the three owner-relevant codes has an explicit `require.Equal(t, "...", body.Code)` assertion — in `server_test.go` (handler-level) AND in `integration_test.go` for `not_owner` cross-process (strongest SC #2 coverage).
- **Token budget (CORE-10):** `len(registerDescription) = 49`, `len(resolveDescription) = 44`. Neither is padded — they genuinely are one short sentence. Aggregate 93 bytes is ~23 tokens — an order of magnitude under the 200-token budget.
- **Integration tests hit the wire:** `TestServe_StdioFullHandshake` actually spawns the re-exec child, writes NDJSON frames to stdin, `bufio.Scanner` reads per-line, `json.Unmarshal` each frame. `TestServe_StdoutIsPureJSONRPC` iterates `bytes.Split(raw, '\n')` and `json.Unmarshal`s every non-empty line — any banner byte would fail the test. Not stubs.
- **SDK usage idiomatic:** `mcp.AddTool` generic form (not deprecated `AddTools`), `*mcp.StdioTransport{}` zero-value, handler closures over `(*store.Store, ownerToken)` captured at construction — one-shot setup.
- **`stubs_test.go` removal:** the diff is surgical — the `serve` row is replaced by a comment explaining Phase 5 took over; `status`, `list`, `purge-all` rows still assert exit 3. Not suppressed.

**Recommendation:** ship as-is. Address IN-items in Phase 9 if desired.

## Warnings

### WR-01: `schema_error` wire code has no dedicated test

**File:** `internal/mcpserver/errors.go:44` + `internal/mcpserver/server_test.go`
**Issue:** `mapStoreError` emits `"schema_error"` for `store.ErrSchemaVersion`, but there is no unit test asserting this branch is reached. The other three sentinels (`ErrUnknownID`, `ErrAlreadyResolved`, `ErrNotOwner`) each have a handler-level test. This branch is genuinely hard to trigger without a schema-corruption fixture, but the comment at `errors.go:35` explicitly calls out the routing decision (OQ-3) as a product-surface commitment — an untested commitment regresses silently.
**Fix:** add a thin unit test that calls `mapStoreError(store.ErrSchemaVersion)` directly and asserts the returned `*mcp.CallToolResult` carries `{"code":"schema_error"}`. No store fixture needed — it's a pure function-level assertion:
```go
func TestMapStoreError_SchemaVersion(t *testing.T) {
    res := mapStoreError(store.ErrSchemaVersion)
    body := decodeErrorBody(t, res)
    require.Equal(t, "schema_error", body.Code)
}
```
Five lines; closes the gap. Non-blocking — the production path still works.

### WR-02: `errors.go:35` comment promises a `slog` log that doesn't exist

**File:** `internal/mcpserver/errors.go:33-35`
**Issue:** the docstring for `mapStoreError` says *"the operator sees details via stderr slog"* on the `schema_error` path, but `mapStoreError` never calls `slog.*` — nor does any caller log the returned `*CallToolResult` before/after. If a schema-version mismatch occurs, the operator sees nothing on stderr; the session continues silently with the client holding a `schema_error` code they likely can't act on.
**Fix:** either (a) add a `slog.Error("schema version mismatch", "err", err)` call on the schema branch, or (b) update the comment to say "operator reads the structured code via the MCP client" and drop the stderr promise. Option (a) is preferable for operational visibility — it's the exact scenario where log-to-stderr earns its keep. Suggest:
```go
case errors.Is(err, store.ErrSchemaVersion):
    slog.Error("store schema mismatch", "err", err)
    return errorContent("schema_error", err.Error())
```
Requires adding `"log/slog"` to the import block. Non-blocking — Phase 9 can address.

## Info

### IN-01: `RegisterIn.Condition` is unvalidated — empty string accepted

**File:** `internal/mcpserver/tools.go:16-18` + `internal/mcpserver/server.go:14-22`
**Issue:** the register handler passes `in.Condition` through to `store.Register` without checking for empty string. The SDK's JSON-schema validation from struct tags enforces *presence* of the field in the request but not non-emptiness; an empty `"condition": ""` will be stamped into state. REQUIREMENTS does not explicitly mandate non-empty, and the store may or may not validate (not reviewed here), but an empty condition is semantically useless and likely to confuse downstream tooling.
**Fix:** OPTIONAL — could add `jsonschema:"...,minLength=1"` to the field tag or a handler-level check returning `errorContent("invalid_argument", "condition must not be empty")`. Defer to Phase 9 / product decision. Flagging as Info only.

### IN-02: `scripts/check-store-isolation.sh` from RESEARCH.md not materialised

**File:** `scripts/` directory
**Issue:** RESEARCH.md §"Wave 0 Gaps" recommended a `scripts/check-store-isolation.sh` that runs the MCP-03 grep + `go list` import audit. The gaps also noted this could live in Phase 9 CI. Currently `scripts/` contains `check-size.sh`, `check-startup.sh`, `check-stdout-silence.sh` but not `check-store-isolation.sh`. The isolation property is currently verified only by hand-grep and by the fact that no offending imports exist today.
**Fix:** OPTIONAL — Phase 9 task. Not blocking v1 ship; property holds by construction right now.

### IN-03: `errors.go:24` `//nolint:errcheck` masks a `json.Marshal` error that is genuinely impossible

**File:** `internal/mcpserver/errors.go:24`
**Issue:** `json.Marshal(errorBody{...})` where `errorBody` has only two string fields cannot fail — all inputs are valid UTF-8 Go strings, no channels/funcs/recursive types. The `//nolint:errcheck` directive and discarded `err` are defensible (correct by construction) but the comment "errorBody is statically valid" could be slightly more explicit about *why* (two string fields, no exotic types). Pure doc polish.
**Fix:** OPTIONAL — consider tightening the comment or dropping the nolint if the linter is happy without it.

---

_Reviewed: 2026-04-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
