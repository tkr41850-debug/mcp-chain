// Package main is the mcp-chain entry point. It establishes the stdout
// discipline (MCP-02: stdout reserved for MCP wire traffic) BEFORE any
// third-party code runs, then hands argv to kong for subcommand dispatch.
package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"

	"github.com/anthropics/mcp-chain/internal/cli"
)

// version is set by -ldflags="-X main.version=..." at build time.
// Defaults to "dev" so local `go build` (without ldflags) still works.
// GoReleaser (Phase 9) fills the tag at release time.
var version = "dev"

// CLI is the root kong grammar. Every subcommand type (Serve/Status/List/Purge)
// lives in internal/cli; main.go stays argv-layer only.
type CLI struct {
	Version kong.VersionFlag `help:"Print version and exit."`

	Serve  cli.ServeCmd  `cmd:"" help:"Run MCP stdio server."`
	Status cli.StatusCmd `cmd:"" help:"Check status of an id. Exits 0 resolved, 2 pending, 1 unknown."`
	List   cli.ListCmd   `cmd:"" help:"List all registered ids."`
	Purge  cli.PurgeCmd  `cmd:"" help:"Purge entries. Requires one of <id>, --all, or --resolved."`
}

func main() {
	// Stdout discipline (MCP-02): hard-set ALL logging to stderr BEFORE any
	// other code runs. MCP stdio reserves stdout for JSON-RPC; a single
	// stray byte corrupts the wire. See PITFALLS.md #1, #13. These two
	// lines MUST be the first executable statements in main() — not in
	// a package init() (unspecified ordering risks a dep logging first).
	log.SetOutput(os.Stderr)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	var root CLI
	kctx := kong.Parse(&root,
		kong.Name("mcp-chain"),
		kong.Description("Chain Claude Code sessions via shared register/wait/resolve locks."),
		kong.Vars{"version": "mcp-chain " + version},
		kong.UsageOnError(),
	)
	// kong.UsageOnError routes usage output to stderr on parse error.
	// kong.VersionFlag prints the `version` variable to stdout and exits 0
	// — the one sanctioned stdout write in Phase 1.
	kctx.FatalIfErrorf(kctx.Run())
}
