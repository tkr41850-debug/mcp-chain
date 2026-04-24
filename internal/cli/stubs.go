// Package cli holds CLI subcommand types wired into the kong grammar at
// cmd/mcp-chain/main.go. Phase 5 wired serve, Phase 6 wired status (see
// status.go), Phase 7 wired list/purge/resolve (see list.go, purge.go,
// resolve.go); only ServeCmd remains in this file. The
// ExitCodeNotImplemented constant is retained as a documented reserved
// value (do not collide with status 0/1/2 in CORE-01).
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthropics/mcp-chain/internal/mcpserver"
	"github.com/anthropics/mcp-chain/internal/statepath"
	"github.com/anthropics/mcp-chain/internal/store"
)

// ExitCodeNotImplemented is returned by every Phase-1 subcommand stub.
// Phase 5 (serve), Phase 6 (status), Phase 7 (list/purge/resolve) replace
// the stub bodies with real logic and real exit codes. 3 is reserved so it
// does not collide with the documented 0/1/2 exit codes for `status` in
// REQUIREMENTS.md CORE-01.
const ExitCodeNotImplemented = 3

// Version is the build version string. Set by release tooling via
// -ldflags="-X github.com/anthropics/mcp-chain/internal/cli.Version=...".
// Used by ServeCmd to populate the MCP Implementation.Version advertised
// on initialize. Phase 9 wires the ldflags; for now this stays "dev".
var Version = "dev"

// ServeCmd runs the MCP stdio server (Phase 5 target).
type ServeCmd struct{}

// Run starts the MCP stdio server. Resolves the state path (Phase 3),
// opens the store (Phase 4), generates a fresh per-process OwnerToken
// (Phase 5), then hands stdin/stdout to mcpserver.Run under a context
// cancelled by SIGINT/SIGTERM or stdin EOF.
func (c *ServeCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		return fmt.Errorf("serve: resolve state path: %w", err)
	}
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("serve: open store: %w", err)
	}
	token, err := mcpserver.NewOwnerToken()
	if err != nil {
		return fmt.Errorf("serve: generate owner token: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return mcpserver.Run(ctx, st, token, Version)
}

