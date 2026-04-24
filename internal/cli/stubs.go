// Package cli holds CLI subcommand types wired into the kong grammar at
// cmd/mcp-chain/main.go. Phase 1 implements every command as a stub that
// exits with ExitCodeNotImplemented after writing a one-line operator
// message to stderr; Phases 5/6/7 replace the stub bodies with real logic.
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

// StatusCmd reports the status of a registered id (Phase 6 target).
// Real exit codes will be 0 (resolved), 1 (unknown), 2 (pending).
type StatusCmd struct {
	ID string `arg:"" help:"Id to check."`
}

func (c *StatusCmd) Run() error {
	fmt.Fprintln(os.Stderr, "mcp-chain status: not implemented (Phase 6)")
	os.Exit(ExitCodeNotImplemented)
	return nil
}

// ListCmd prints a human-readable table of all entries (Phase 7 target).
type ListCmd struct{}

func (c *ListCmd) Run() error {
	fmt.Fprintln(os.Stderr, "mcp-chain list: not implemented (Phase 7)")
	os.Exit(ExitCodeNotImplemented)
	return nil
}

// PurgeCmd deletes entries. Accepts <id>, --all, or --resolved. The xor tag
// makes --all and --resolved mutually exclusive; one-of-required enforcement
// (including rejecting bare `purge`) lands in Phase 7 when the real
// implementation replaces this stub.
type PurgeCmd struct {
	ID       string `arg:"" optional:"" help:"Id to purge."`
	All      bool   `help:"Purge all entries." xor:"target"`
	Resolved bool   `help:"Purge only resolved entries." xor:"target"`
}

func (c *PurgeCmd) Run() error {
	fmt.Fprintln(os.Stderr, "mcp-chain purge: not implemented (Phase 7)")
	os.Exit(ExitCodeNotImplemented)
	return nil
}
