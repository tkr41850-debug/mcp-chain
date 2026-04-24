package mcpserver

import (
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

// errorBody is the JSON payload clients read from Content[0].Text
// when IsError is true. `code` is machine-readable and stable across
// releases; `message` is human-readable and MAY change.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorContent builds a tool-result error with a structured code.
// SDK auto-serialises Content so the client sees `{"code":"...","message":"..."}`
// as the text of Content[0] with IsError=true.
func errorContent(code, message string) *mcp.CallToolResult {
	payload, _ := json.Marshal(errorBody{Code: code, Message: message}) //nolint:errcheck // errorBody is statically valid
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
	}
}

// mapStoreError translates a store sentinel error into a tool-result
// error with a distinct `code` per sentinel. Resolved decision OQ-3:
// ErrSchemaVersion emits `schema_error` as a tool-result error (not a
// protocol-level abort) to keep the session alive; the operator sees
// details via stderr slog.
func mapStoreError(err error) *mcp.CallToolResult {
	switch {
	case errors.Is(err, store.ErrUnknownID):
		return errorContent("unknown_id", "unknown lock id")
	case errors.Is(err, store.ErrAlreadyResolved):
		return errorContent("already_resolved", "lock is already resolved")
	case errors.Is(err, store.ErrNotOwner):
		return errorContent("not_owner", "this session did not register this lock")
	case errors.Is(err, store.ErrSchemaVersion):
		return errorContent("schema_error", err.Error())
	default:
		return errorContent("internal", err.Error())
	}
}
