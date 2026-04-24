package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/anthropics/mcp-chain/internal/store"
)

// newStoreForTest opens a fresh store under t.TempDir(). Mirrors the
// pattern used in internal/store/store_test.go.
func newStoreForTest(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	return st
}

// decodeErrorBody extracts the structured wire error payload from a
// tool-result error.
func decodeErrorBody(t *testing.T, res *mcp.CallToolResult) errorBody {
	t.Helper()
	require.NotNil(t, res, "expected a tool-result error, got nil")
	require.True(t, res.IsError, "expected IsError=true")
	require.NotEmpty(t, res.Content, "expected at least one Content entry")
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected Content[0] to be *mcp.TextContent, got %T", res.Content[0])
	var body errorBody
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &body))
	return body
}

func TestRegisterHandler_HappyPath(t *testing.T) {
	st := newStoreForTest(t)
	tok, err := NewOwnerToken()
	require.NoError(t, err)

	h := registerHandler(st, tok)
	res, out, herr := h(context.Background(), &mcp.CallToolRequest{}, RegisterIn{Condition: "ready"})
	require.NoError(t, herr)
	require.Nil(t, res, "happy path returns nil *CallToolResult")
	require.NotEmpty(t, out.ID)

	rec, err := st.Get(out.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", rec.Status)
	require.Equal(t, "ready", rec.Condition)
}

func TestRegisterHandler_StampsOwnerToken(t *testing.T) {
	st := newStoreForTest(t)
	tok, err := NewOwnerToken()
	require.NoError(t, err)
	h := registerHandler(st, tok)

	_, out1, _ := h(context.Background(), &mcp.CallToolRequest{}, RegisterIn{Condition: "a"})
	_, out2, _ := h(context.Background(), &mcp.CallToolRequest{}, RegisterIn{Condition: "b"})

	r1, err := st.Get(out1.ID)
	require.NoError(t, err)
	r2, err := st.Get(out2.ID)
	require.NoError(t, err)
	require.Equal(t, tok, r1.OwnerToken, "first register must stamp process token")
	require.Equal(t, tok, r2.OwnerToken, "second register must stamp the SAME process token")
}

func TestResolveHandler_OwnerOk(t *testing.T) {
	st := newStoreForTest(t)
	tok, _ := NewOwnerToken()
	reg := registerHandler(st, tok)
	res, out, _ := reg(context.Background(), &mcp.CallToolRequest{}, RegisterIn{Condition: "x"})
	require.Nil(t, res)

	resv := resolveHandler(st, tok)
	rres, _, rerr := resv(context.Background(), &mcp.CallToolRequest{}, ResolveIn(out))
	require.NoError(t, rerr)
	require.Nil(t, rres, "matching owner token -> nil result (success)")

	rec, _ := st.Get(out.ID)
	require.Equal(t, "resolved", rec.Status)
}

func TestResolveHandler_UnknownID(t *testing.T) {
	st := newStoreForTest(t)
	tok, _ := NewOwnerToken()
	resv := resolveHandler(st, tok)
	res, _, _ := resv(context.Background(), &mcp.CallToolRequest{}, ResolveIn{ID: "no-such-id"})
	body := decodeErrorBody(t, res)
	require.Equal(t, "unknown_id", body.Code)
}

func TestResolveHandler_AlreadyResolved(t *testing.T) {
	st := newStoreForTest(t)
	tok, _ := NewOwnerToken()
	reg := registerHandler(st, tok)
	_, out, _ := reg(context.Background(), &mcp.CallToolRequest{}, RegisterIn{Condition: "x"})

	resv := resolveHandler(st, tok)
	_, _, _ = resv(context.Background(), &mcp.CallToolRequest{}, ResolveIn(out))

	res, _, _ := resv(context.Background(), &mcp.CallToolRequest{}, ResolveIn(out))
	body := decodeErrorBody(t, res)
	require.Equal(t, "already_resolved", body.Code)
}

func TestResolveHandler_NotOwner(t *testing.T) {
	st := newStoreForTest(t)
	tokA, _ := NewOwnerToken()
	tokB, _ := NewOwnerToken()
	require.NotEqual(t, tokA, tokB)

	reg := registerHandler(st, tokA)
	_, out, _ := reg(context.Background(), &mcp.CallToolRequest{}, RegisterIn{Condition: "x"})

	resv := resolveHandler(st, tokB) // different session!
	res, _, _ := resv(context.Background(), &mcp.CallToolRequest{}, ResolveIn(out))
	body := decodeErrorBody(t, res)
	require.Equal(t, "not_owner", body.Code,
		"mismatched owner token MUST surface distinct wire code (SC #2)")
}
