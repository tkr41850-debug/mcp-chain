// Package cli: list subcommand (Phase 7).
//
// ListCmd is the kong handler for `mcp-chain list`.
// runList is the exit-code decision tree extracted for
// unit-testability via captured io.Writer pairs (LD-2 / LD-3).
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/anthropics/mcp-chain/internal/cli/format"
	"github.com/anthropics/mcp-chain/internal/statepath"
	"github.com/anthropics/mcp-chain/internal/store"
)

// ListCmd prints all chain entries as an aligned table.
//
// Exit codes (LOCKED — LD-1):
//
//	0  N entries printed to stdout (or 0 entries → "no entries" to stderr, still exit 0)
//	1  any load / render error — stderr: "mcp-chain: <err>\n"
type ListCmd struct{}

// Run wires statepath → store → runList. Returns nil on success; exits
// non-zero via os.Exit when runList signals an error (LD-4: no ExitCoder).
func (c *ListCmd) Run() error {
	path, err := statepath.Resolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-chain: %v\n", err)
		os.Exit(1)
	}
	code := runList(os.Stdout, os.Stderr, path)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

// runList maps (path → exit code) with explicit writers. Pure with
// respect to env vars (LD-3). Empty-store hint goes to stderr with
// exit 0 (LD-5: POSIX-idiom; pitfall 6).
func runList(out, errW io.Writer, path string) int {
	st, err := store.Open(path)
	if err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	records, err := st.List()
	if err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Fprintln(errW, "mcp-chain: no entries")
		return 0
	}
	if err := format.WriteTable(out, records); err != nil {
		fmt.Fprintf(errW, "mcp-chain: %v\n", err)
		return 1
	}
	return 0
}
