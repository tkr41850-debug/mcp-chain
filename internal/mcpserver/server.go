package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tkr41850-debug/mcp-chain/internal/store"
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

// registerWithIDHandler returns the typed handler closure for the
// `register_with_id` tool. Unlike registerHandler, the caller supplies
// the id directly (used by callers that key the lock to an external
// slug — e.g. a Claude Code plan).
func registerWithIDHandler(st *store.Store, ownerToken string) func(ctx context.Context, req *mcp.CallToolRequest, in RegisterWithIDIn) (*mcp.CallToolResult, RegisterWithIDOut, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in RegisterWithIDIn) (*mcp.CallToolResult, RegisterWithIDOut, error) {
		if err := st.RegisterWithID(ownerToken, in.ID, in.Condition); err != nil {
			return mapStoreError(err), RegisterWithIDOut{}, nil
		}
		return nil, RegisterWithIDOut{}, nil
	}
}

// serverName is the Implementation.Name advertised on initialize.
const serverName = "mcp-chain"

// Run builds an MCP server over StdioTransport and serves until the
// transport's connection closes (stdin EOF) or ctx is cancelled.
// Callers: internal/cli ServeCmd.Run. Tests: integration_test.go
// runs this in a spawned child process.
//
// version is the Implementation.Version advertised to clients; callers
// should pass cmd/mcp-chain/main.go's ldflags-injected `version` var.
func Run(ctx context.Context, st *store.Store, ownerToken, version string) error {
	impl := &mcp.Implementation{Name: serverName, Version: version}
	server := mcp.NewServer(impl, nil)

	mcp.AddTool(server,
		&mcp.Tool{Name: "register", Description: registerDescription},
		registerHandler(st, ownerToken),
	)
	mcp.AddTool(server,
		&mcp.Tool{Name: "resolve", Description: resolveDescription},
		resolveHandler(st, ownerToken),
	)
	mcp.AddTool(server,
		&mcp.Tool{Name: "register_with_id", Description: registerWithIDDescription},
		registerWithIDHandler(st, ownerToken),
	)

	return server.Run(ctx, &mcp.StdioTransport{})
}
