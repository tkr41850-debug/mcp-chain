// Package cli: resolve subcommand (Phase 7).
//
// ResolveCmd is the kong handler for `mcp-chain resolve <id> [--force]`.
// runResolve is the exit-code decision tree (LD-2 / LD-3 / LD-7).
//
// Because the CLI has no MCP session and therefore no OwnerToken, the
// happy path REQUIRES --force. This is the documented operator escape
// hatch (07-CONTEXT.md §Resolve semantics; LD-7 / pitfall 4).
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/mcp-chain/internal/statepath"
	"github.com/anthropics/mcp-chain/internal/store"
)

// ResolveCmd marks an id resolved from the CLI.
//
// Exit codes (LOCKED — LD-1):
//
//	0  resolved — stdout empty, stderr empty
//	1  unknown id, not-owner (without --force), already-resolved, any other error
type ResolveCmd struct {
	ID    string `arg:"" help:"Id to resolve."`
	Force bool   `help:"Bypass the OwnerToken check (operator escape hatch)."`
}

// Run wires statepath → store → runResolve.
func (c *ResolveCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
		os.Exit(1)
	}
	code := runResolve(os.Stdout, os.Stderr, path, c.ID, c.Force)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runResolve passes an empty ownerToken; the store constant-time-
// compares it against the record's stamped token (Phase 5 stamps a
// non-empty 32-char hex token on every register). Without --force,
// this always fails for Phase 5 records — pitfall 4 / LD-7. The CLI
// surfaces the --force hint in the ErrNotOwner branch.
func runResolve(out, errW io.Writer, path, id string, force bool) int {
	_ = out // resolve emits nothing to stdout (LD-1 success row is empty)
	st, err := store.Open(path)
	if err != nil {
		_, _ = fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	err = st.Resolve(id, "", store.ResolveOptions{Force: force})
	switch {
	case err == nil:
		return 0
	case errors.Is(err, store.ErrUnknownID):
		_, _ = fmt.Fprintf(errW, "mcp-chain: unknown id: %s\n", id)
		return 1
	case errors.Is(err, store.ErrNotOwner):
		_, _ = fmt.Fprintln(errW, "mcp-chain: not owner (use --force to override)")
		return 1
	case errors.Is(err, store.ErrAlreadyResolved):
		_, _ = fmt.Fprintln(errW, "mcp-chain: already resolved")
		return 1
	default:
		_, _ = fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
}
