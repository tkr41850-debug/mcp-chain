---
phase: 5
slug: mcp-server-adapter
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-24
---

# Phase 5 — Validation Strategy

> Per-phase validation contract. Content derived from `05-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| **Config file** | none |
| **Quick run command** | `go test -race ./internal/mcpserver/... -count=1 -timeout 30s` |
| **Full suite command** | `go test -race -count=1 -timeout 60s ./...` |
| **Integration command** | `go test -race -count=1 -tags=integration -timeout 120s ./...` |
| **Smoke gates** | `go list -deps ./cmd/mcp-chain/... \| grep -qx net/http` → fails; `go list -f …` import audit for store isolation |
| **Estimated runtime** | ~2 s unit; ~15 s integration |

---

## Sampling Rate

- **After every task commit:** `go test -race ./internal/mcpserver/... -count=1 -timeout 30s` (~2 s)
- **After every plan wave:** `go test -race -tags=integration -count=1 -timeout 120s ./...` (+ net/http + store-isolation smoke gates)
- **Before `/gsd-verify-work`:** Full race suite + integration + all smoke gates + `go vet` + `make lint`
- **Max feedback latency:** <5 s per-commit

---

## Per-Task Verification Map

| Task ID | Wave | Requirement | SC | Test / Gate | Type | Automated Command | Status |
|---------|------|-------------|----|-------------|------|-------------------|--------|
| 5-0-dep | 0 | — | — | `go-sdk` v1.5 present + no cgo pulled | build | `grep -q 'modelcontextprotocol/go-sdk v1.5' go.mod && CGO_ENABLED=0 go build ./...` | ⬜ |
| 5-1-owner | 1 | CORE-10 | 2 | `TestNewOwnerToken_IsHex32Chars` — 32-char hex, 128 bits entropy | unit | `go test -race -run TestNewOwnerToken_IsHex32Chars ./internal/mcpserver` | ⬜ |
| 5-1-owner | 1 | CORE-10 | 2 | `TestNewOwnerToken_Uniqueness` — 1000 draws, no dupes | unit | `go test -race -run TestNewOwnerToken_Uniqueness ./internal/mcpserver` | ⬜ |
| 5-2-tools | 1 | CORE-10 | 3 | `TestToolDescriptionsUnderBudget` — each desc `len <= 160` | unit (static) | `go test -race -run TestToolDescriptionsUnderBudget ./internal/mcpserver` | ⬜ |
| 5-2-tools | 1 | CORE-10 | 3 | `TestToolListUnderBudget` — aggregate desc bytes `<= 800` | unit (static) | `go test -race -run TestToolListUnderBudget ./internal/mcpserver` | ⬜ |
| 5-3-register | 2 | CORE-02 | 1, 2 | `TestRegisterHandler_HappyPath` — returns word-ID; record persisted | unit (handler) | `go test -race -run TestRegisterHandler_HappyPath ./internal/mcpserver` | ⬜ |
| 5-3-register | 2 | CORE-02 | 2 | `TestRegisterHandler_StampsOwnerToken` — process-scope token on record | unit | `go test -race -run TestRegisterHandler_StampsOwnerToken ./internal/mcpserver` | ⬜ |
| 5-4-resolve | 2 | CORE-03 | 1 | `TestResolveHandler_OwnerOk` — matching token → success | unit | `go test -race -run TestResolveHandler_OwnerOk ./internal/mcpserver` | ⬜ |
| 5-4-resolve | 2 | CORE-03 | 2 | `TestResolveHandler_NotOwner` — wire code `not_owner` (distinct) | unit | `go test -race -run TestResolveHandler_NotOwner ./internal/mcpserver` | ⬜ |
| 5-4-resolve | 2 | CORE-03 | 2 | `TestResolveHandler_UnknownID` — wire code `unknown_id` | unit | `go test -race -run TestResolveHandler_UnknownID ./internal/mcpserver` | ⬜ |
| 5-4-resolve | 2 | CORE-03 | 2 | `TestResolveHandler_AlreadyResolved` — wire code `already_resolved` | unit | `go test -race -run TestResolveHandler_AlreadyResolved ./internal/mcpserver` | ⬜ |
| 5-5-isolation | 2 | MCP-03 | 3 | Store has no MCP/jsonrpc imports | smoke | `! go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' ./internal/store/... \| grep -E 'modelcontextprotocol\|jsonrpc'` | ⬜ |
| 5-5-isolation | 2 | MCP-03 | 3 | Store has no MCP/jsonrpc identifier leakage | smoke | `! grep -rE '\b(mcp\|jsonrpc\|modelcontextprotocol)\b' internal/store/ --include='*.go' \| grep -v '^[^:]*:[[:space:]]*//'` | ⬜ |
| 5-6-wiring | 3 | CORE-02 | 1 | `ServeCmd.Run` no longer stubs (exit 3) | smoke | `! grep -q 'not implemented' internal/cli/stubs.go` | ⬜ |
| 5-6-wiring | 3 | MCP-01 | 1 | Binary does not import `net/http` | smoke | `! go list -deps ./cmd/mcp-chain/... \| grep -qx net/http` | ⬜ |
| 5-7-integration | 3 | MCP-01 | 1, 4 | `TestServe_StdioFullHandshake` — initialize + list + register + resolve over re-exec stdio | integration | `go test -race -tags=integration -run TestServe_StdioFullHandshake ./internal/mcpserver` | ⬜ |
| 5-7-integration | 3 | CORE-10 / MCP-02 | 4 | `TestServe_StdoutIsPureJSONRPC` — every stdout line parses as JSON-RPC; no banners | integration | `go test -race -tags=integration -run TestServe_StdoutIsPureJSONRPC ./internal/mcpserver` | ⬜ |
| 5-7-integration | 3 | CORE-03 | 2 | `TestServe_ResolveNotOwnerWireCode` — distinct `not_owner` observable on the wire | integration | `go test -race -tags=integration -run TestServe_ResolveNotOwnerWireCode ./internal/mcpserver` | ⬜ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` addition: `github.com/modelcontextprotocol/go-sdk v1.5.0`
- [ ] `go.sum` updated; `CGO_ENABLED=0 go build ./...` green (pulls no cgo deps)
- [ ] `internal/mcpserver/owner.go` — `NewOwnerToken() (string, error)` via `crypto/rand` + `hex.EncodeToString`
- [ ] `internal/mcpserver/tools.go` — terse tool descriptions as const strings; `registerTool`, `resolveTool` struct literals
- [ ] `internal/mcpserver/errors.go` — `errorContent(code, message string) *mcp.CallToolResult` helper + sentinel → code mapping
- [ ] `internal/mcpserver/server.go` — `Run(ctx, store, ownerToken)` wires SDK, registers tools, serves until EOF
- [ ] `internal/mcpserver/owner_test.go` — 2 tests
- [ ] `internal/mcpserver/tools_test.go` — 2 budget tests
- [ ] `internal/mcpserver/server_test.go` — 6 handler unit tests (direct handler invocation OR loopback if SDK exposes one)
- [ ] `internal/mcpserver/integration_test.go` (build tag `integration`) — `TestMain` re-exec dispatcher + 3 wire-level tests
- [ ] `internal/cli/stubs.go` — replace `ServeCmd.Run` stub with actual `mcpserver.Run(...)` invocation

**Framework install:** None beyond `go.mod` dep addition.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real Claude Code client invokes tools end-to-end | SC #1 | Requires a live MCP-capable client | Deferred to Phase 10 dogfooding (register a lock in one Claude Code session, resolve from another) |
| Token budget against real tokenizer (tiktoken) | CORE-10 | tiktoken has no pure-Go impl | Byte-length proxy (`len(desc) ≤ 160 ≈ 40 tokens`) is the CI gate; periodic manual check with `@dqbd/tiktoken` node CLI acceptable |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have automated verify or Wave 0 dependencies
- [x] Sampling continuity: every test maps to a commit-level verify
- [x] Wave 0 self-contained (depends only on Phase 4 store + Phase 3 statepath)
- [x] No watch-mode flags
- [x] Feedback latency < 60 s (<5 s unit; wave-end integration ~15 s)
- [x] `nyquist_compliant: true` set
- [x] All 4 SC mapped to ≥1 test (SC 1→integration+handler, SC 2→owner+resolve×4, SC 3→budget×2+isolation×2, SC 4→integration wire assertions)

**Approval:** approved 2026-04-24
