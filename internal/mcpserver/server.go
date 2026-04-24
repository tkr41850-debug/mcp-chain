package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/anthropics/mcp-chain/internal/store"
)

// registerHandler returns the typed handler closure for the
// `register` tool. Factored out so tests can call it directly
// without spinning up a full SDK Server (OQ-1 fallback path).
func registerHandler(st *store.Store, ownerToken string) func(ctx context.Context, req *mcp.CallToolRequest, in RegisterIn) (*mcp.CallToolResult, RegisterOut, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in RegisterIn) (*mcp.CallToolResult, RegisterOut, error) {
		id, err := st.Register(ownerToken, in.Condition)
		if err != nil {
			return mapStoreError(err), RegisterOut{}, nil
		}
		return nil, RegisterOut{ID: id}, nil
	}
}

// resolveHandler mirrors registerHandler for the `resolve` tool.
func resolveHandler(st *store.Store, ownerToken string) func(ctx context.Context, req *mcp.CallToolRequest, in ResolveIn) (*mcp.CallToolResult, ResolveOut, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in ResolveIn) (*mcp.CallToolResult, ResolveOut, error) {
		if err := st.Resolve(in.ID, ownerToken, store.ResolveOptions{Force: false}); err != nil {
			return mapStoreError(err), ResolveOut{}, nil
		}
		return nil, ResolveOut{}, nil
	}
}
