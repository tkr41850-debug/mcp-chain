// Package main is the mcp-chain entry point. It establishes the stdout
// discipline (MCP-02: stdout reserved for MCP wire traffic) BEFORE any
// third-party code runs, handles --version manually (to keep it on
// stdout even with kong.Writers redirecting kong's own output to
// stderr), then hands argv to kong for subcommand dispatch.
package main

import (
	"fmt"
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

// CLI is the root kong grammar. `--version` is NOT declared here; see
// the manual pre-parse in main() for why (kong's version-flag hook
// hard-codes app.Stdout, which kong.Writers below repoints to stderr,
// so a kong-driven version flag would write to stderr and fail SC #3
// round-trip).
type CLI struct {
	Serve  cli.ServeCmd  `cmd:"" help:"Run MCP stdio server."`
	Status cli.StatusCmd `cmd:"" help:"Check status of an id. Exits 0 resolved, 2 pending, 1 unknown."`
	List   cli.ListCmd   `cmd:"" help:"List all registered ids."`
	Purge  cli.PurgeCmd  `cmd:"" help:"Purge entries. Requires one of <id>, --all, or --resolved."`
}

func main() {
	// Stdout discipline (MCP-02): hard-set ALL logging to stderr BEFORE
	// any other code runs. MCP stdio reserves stdout for JSON-RPC;
	// a single stray byte corrupts the wire. See PITFALLS.md #1, #13.
	// These two lines MUST be the first executable statements in main().
	log.SetOutput(os.Stderr)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Manual --version pre-parse. This is the ONE sanctioned stdout
	// write in the CLI surface. Must run before kong.Parse so the
	// kong.Writers redirect below does not capture it.
	for _, a := range os.Args[1:] {
		if a == "--version" {
			fmt.Fprintln(os.Stdout, "mcp-chain "+version)
			os.Exit(0)
		}
	}

	var root CLI
	kctx := kong.Parse(&root,
		kong.Name("mcp-chain"),
		kong.Description("Chain Claude Code sessions via shared register/wait/resolve locks."),
		// SC #3: kong's default HelpPrinter writes to ctx.Stdout (= os.Stdout).
		// Redirect BOTH writers to os.Stderr so --help, bad-args usage, and
		// unknown-command errors never touch stdout.
		kong.Writers(os.Stderr, os.Stderr),
		kong.UsageOnError(),
	)
	kctx.FatalIfErrorf(kctx.Run())
}
