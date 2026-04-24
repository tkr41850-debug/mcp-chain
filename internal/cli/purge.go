// Package cli: purge subcommand (Phase 7).
//
// PurgeCmd is the kong handler for `mcp-chain purge [<id>|--all|--resolved]`.
// runPurge is the exit-code decision tree (LD-2 / LD-3 / LD-6).
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/mcp-chain/internal/statepath"
	"github.com/anthropics/mcp-chain/internal/store"
)

// PurgeCmd deletes entries. Exactly one of <id>, --all, --resolved is
// required.
//
// Arg-shape enforcement strategy (LD-6 / pitfall 5):
//   - xor:"target" on --all + --resolved → kong rejects both flags together.
//   - "at least one target" — NOT enforced by kong (xor does not apply
//     to positionals in kong v1.15). runPurge calls store.Purge with
//     zero-valued PurgeOptions on a bare invocation; store returns
//     ErrPurgeArgRequired which runPurge translates to the locked
//     stderr line.
//
// Exit codes (LOCKED — LD-1):
//
//	0  any N-removed (including N=0 for --resolved on empty store)
//	1  kong parse failure (both flags), bare invocation (no target), unknown <id>, or any other error
type PurgeCmd struct {
	ID       string `arg:"" optional:"" help:"Id to purge (mutually exclusive with --all and --resolved)."`
	All      bool   `help:"Purge all entries." xor:"target"`
	Resolved bool   `help:"Purge only resolved entries." xor:"target"`
}

// Run wires statepath → store → runPurge. Returns nil on success; exits
// non-zero via os.Exit when runPurge signals an error (LD-4).
func (c *PurgeCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
		os.Exit(1)
	}
	code := runPurge(os.Stdout, os.Stderr, path, c.ID, c.All, c.Resolved)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runPurge maps (path, id, all, resolvedOnly → exit code) with explicit
// writers. Translates store.ErrPurgeArgRequired / ErrUnknownID into the
// locked stderr lines. Emits NOTHING to stdout on any branch (LD-1).
func runPurge(out, errW io.Writer, path string, id string, all, resolvedOnly bool) int {
	_ = out // purge emits nothing to stdout (LD-1 success rows all empty)
	st, err := store.Open(path)
	if err != nil {
		_, _ = fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	_, err = st.Purge(store.PurgeOptions{
		ID:       id,
		All:      all,
		Resolved: resolvedOnly,
	})
	switch {
	case err == nil:
		return 0
	case errors.Is(err, store.ErrPurgeArgRequired):
		// Bare `purge` (no args/flags). Kong accepted it because the
		// positional is optional and neither flag was set. Store is
		// the single source of truth for the "exactly one target"
		// check (LD-6 / pitfall 5).
		_, _ = fmt.Fprintln(errW, "mcp-chain: purge requires <id>, --all, or --resolved")
		return 1
	case errors.Is(err, store.ErrUnknownID):
		_, _ = fmt.Fprintf(errW, "mcp-chain: unknown id: %s\n", id)
		return 1
	default:
		_, _ = fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
}
