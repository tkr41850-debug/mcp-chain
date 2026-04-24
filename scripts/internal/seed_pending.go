//go:build ignore

// seed_pending.go — dev-time helper: register one pending chain entry in
// the store pointed to by XDG_STATE_HOME and print the allocated
// word-ID on stdout. Invoked by scripts/smoke-chain-wait.sh.
// Excluded from the production binary via the //go:build ignore tag.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/tkr41850-debug/mcp-chain/internal/statepath"
	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

func main() {
	path, err := statepath.Resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed: statepath:", err)
		os.Exit(2)
	}
	st, err := store.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed: open:", err)
		os.Exit(2)
	}
	var tok [16]byte
	if _, err := rand.Read(tok[:]); err != nil {
		fmt.Fprintln(os.Stderr, "seed: rand:", err)
		os.Exit(2)
	}
	id, err := st.Register(hex.EncodeToString(tok[:]), "smoke condition")
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed: register:", err)
		os.Exit(2)
	}
	fmt.Println(id)
}
