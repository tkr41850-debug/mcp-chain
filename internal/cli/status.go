// Package cli: status subcommand (Phase 6).
//
// StatusCmd is the kong handler for `mcp-chain status <id>`.
// runStatus is the exit-code decision tree extracted for
// unit-testability via captured io.Writer pairs — no os.Exit,
// no os.Stdout / os.Stderr references, no env-var reads.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tkr41850-debug/mcp-chain/internal/statepath"
	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

// StatusCmd reports the status of a registered id.
//
// Exit codes (LOCKED — Phase 8 bash monitor depends on these):
//
//	0  id resolved — stdout: "resolved\n"
//	2  id pending  — stdout: "pending\n"
//	1  unknown id or any other error — stderr: "mcp-chain: ...\n"
//
// The `if mcp-chain status $id; then ...` shell idiom works
// intuitively: only a resolved id triggers the then-branch.
type StatusCmd struct {
	ID string `arg:"" help:"Id to check."`
}

// Run wires statepath → store → runStatus and translates the
// returned code into a process exit. Return-nil for code==0 so
// kong's FatalIfErrorf sees success; os.Exit for 1/2 so the
// shell observes the contract exit code without kong emitting
// an extra "error:" prefix on stderr (Pitfall 2 / LD-4).
func (c *StatusCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
		os.Exit(1)
	}
	code := runStatus(os.Stdout, os.Stderr, path, c.ID)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runStatus is the exit-code decision tree. Pure with respect to
// the environment: callers pass in the state file path so tests
// can use t.TempDir() without touching $XDG_STATE_HOME.
//
// Pre-condition: parent directory of `path` exists (statepath.Resolve
// guarantees this in production; tests MkdirAll themselves).
func runStatus(out, errW io.Writer, path, id string) int {
	st, err := store.Open(path)
	if err != nil {
		_, _ = fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	r, err := st.Get(id)
	switch {
	case err == nil && r.Status == "resolved":
		_, _ = fmt.Fprintln(out, "resolved")
		return 0
	case err == nil && r.Status == "pending":
		_, _ = fmt.Fprintln(out, "pending")
		return 2
	case errors.Is(err, store.ErrUnknownID):
		_, _ = fmt.Fprintf(errW, "mcp-chain: unknown id: %s\n", id)
		return 1
	default:
		_, _ = fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
}
