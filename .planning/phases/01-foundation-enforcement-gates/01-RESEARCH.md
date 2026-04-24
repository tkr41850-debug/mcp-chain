# Phase 1: Foundation & Enforcement Gates - Research

**Researched:** 2026-04-23
**Domain:** Go module scaffold + CI enforcement gates (stdout discipline, lint, size, startup)
**Confidence:** HIGH for kong/lint/CI patterns (verified against official docs); HIGH for stdout discipline mechanics; MEDIUM for exact startup-time measurement methodology (budget comes from PROJECT.md, not empirical)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Key constraints from PROJECT.md / REQUIREMENTS.md / STACK.md research (not negotiable):
- Go ≥ 1.22 (stack says 1.23+ per `modelcontextprotocol/go-sdk` minimum — Phase 1 sets floor at 1.23)
- Pure-Go deps (no cgo)
- `alecthomas/kong` v1.15.0 for CLI dispatch
- Stripped binary ≤ 15 MB, cold startup ≤ 100 ms, RSS ≤ 20 MB
- Stdout reserved for MCP wire traffic; all logs go to stderr via `log/slog`
- Enforcement via lint (`forbidigo`) + CI gates (size + startup)
- `staticcheck` v2025.1.1 for static analysis

### Claude's Discretion
Everything else — file layout specifics, exact YAML shape, exit codes for unimplemented stubs, Makefile vs bare scripts, measurement methodology for the startup gate, how aggressively to front-load CI matrix scope.

### Deferred Ideas (OUT OF SCOPE)
- MCP tool handlers (Phase 5)
- Store / flock / state file (Phase 4)
- Wordlist, XDG, idgen (Phases 2–3)
- CLI formatters & `status` exit code semantics (Phases 6–7)
- GoReleaser cross-compile matrix (Phase 9 — but lint/size/startup CI workflow lands here)
- Windows/macOS full CI matrix (Phase 9; Phase 1 is Linux-only minimum)
- Slash commands, bash monitor, plugin packaging (Phase 8)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-01 | Single Go binary with subcommands via `alecthomas/kong` (serve/status/list/purge + `--version`) | Section "Kong scaffold & --version pattern" — `kong.VersionFlag` + `kong.Vars{"version":...}` + `Run(...)` dispatch; ldflags version injection |
| MCP-02 | Strict stdout discipline — only JSON-RPC on stdout; all logs to stderr via `log/slog` | Section "Stdout discipline enforcement" — `log.SetOutput(os.Stderr)` + `slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr,...)))` in main(); `forbidigo` YAML pattern forbidding `fmt.Print*` + `println`/`print` in serve path |
| DIST-03 | CI size gate ≤ 15 MB; startup gate ≤ 100 ms on cold-cache runner | Section "Binary size gate" (`-ldflags="-s -w" -trimpath`, `wc -c` check with 15728640-byte threshold) + Section "Startup budget gate" (P95 of 5 runs via bash+`$EPOCHREALTIME` or `hyperfine` when available) |
| QA-04 | CI lint gate — `go vet` + `staticcheck` (or equivalent); non-zero exit blocks merge | Section "Static analysis" — `golangci-lint` v2 with `govet` + `staticcheck` + `forbidigo` enabled; single invocation covers all three, staticcheck v2025.1.1 pinned via tool dependency |
</phase_requirements>

## Summary

Phase 1 is a pure infrastructure phase: no business logic ships, but four enforcement rails (stdout discipline, lint, binary-size ceiling, startup-time budget) are installed and proven to block regressions so every downstream phase inherits them. The scaffold is small — a `cmd/mcp-chain/main.go` with a `kong`-struct-dispatched CLI, stub `Run()` methods that print "not implemented" and exit with a reserved non-zero code, and stderr-only logging wired in `main()` before any third-party code runs. The CI workflow (`.github/workflows/ci.yml`) runs `golangci-lint` (govet + staticcheck + forbidigo), builds with `-ldflags="-s -w" -trimpath`, asserts stripped size ≤ 15 728 640 bytes, and times `./mcp-chain --version` over 5 iterations with a P95 ceiling of 100 ms.

Three decisions need the planner's attention early because they shape every file touched:

1. **Module path is `github.com/tkr41850-debug/mcp-chain`** — placeholder; explicitly flagged `[ASSUMED]` because the author's GitHub handle isn't resolved from repo context. Treat as `<TBD: replace with actual GitHub owner before `go mod init`>` in plans.
2. **Entry point at `cmd/mcp-chain/main.go`** — idiomatic for single-binary Go projects per [Go module layout](https://go.dev/doc/modules/layout) and already established in ARCHITECTURE.md. Do NOT put `main.go` at repo root.
3. **Version injection via ldflags** — `-X main.version={{.Version}}` wired in Phase 1 with a `dev` fallback so `go build` (without ldflags) still works for local dev; GoReleaser in Phase 9 will fill in the real tag. Kong's `VersionFlag` reads from `kong.Vars{"version": version}`.

**Primary recommendation:** Ship the minimal workflow — `cmd/mcp-chain/main.go` + `internal/cli/stubs.go` + `.golangci.yml` + `.github/workflows/ci.yml` + `Makefile` (five files, ~150 LOC total). Gate everything on Linux only; defer cross-platform matrix to Phase 9. Every gate is a script in `scripts/` that the CI workflow calls — keeps CI config terse and tests reusable locally.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Argv parsing & subcommand dispatch | `cmd/mcp-chain/main.go` | — | Single entry point; all argv-level concerns live here to keep `internal/cli` adapter-free at Phase 1 scale |
| Subcommand stub bodies (`not implemented`) | `internal/cli` | — | Hexagonal boundary already planned by ARCHITECTURE.md — stubs live where real impls will land in Phases 5–7 |
| Stdout discipline bootstrap (`log.SetOutput`, `slog.SetDefault`) | `cmd/mcp-chain/main.go` | — | Must run before ANY third-party code — earliest possible in `main()`, cannot live in a package init() that the module system orders unpredictably |
| Version string | `cmd/mcp-chain/main.go` | GoReleaser (Phase 9) | `var version = "dev"` with ldflags override; kong `VersionFlag` reads via `kong.Vars` |
| Lint rules (forbidigo + staticcheck + govet) | `.golangci.yml` at repo root | CI workflow | Repo-level policy file consumed by both local devs (`golangci-lint run`) and CI — single source of truth |
| Binary-size gate | `scripts/check-size.sh` | CI workflow | Reusable from local dev and CI; shell script keeps CI yaml declarative |
| Startup-time gate | `scripts/check-startup.sh` | CI workflow | Same rationale as size gate; measurement uses `$EPOCHREALTIME` (bash 5+) or `/usr/bin/time -f '%e'` |
| CI orchestration | `.github/workflows/ci.yml` | — | Calls scripts; fails pipeline on non-zero exit from any gate |

## Standard Stack

### Core (Phase 1 only — Phases 4+ pull the rest)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.23+ | Language / runtime | Single static binary, pure-Go deps, cross-compile trivial. `go-sdk` demands 1.23; lock Phase 1 at 1.23 to match `[VERIFIED: STACK.md section "Version Compatibility"]` |
| `github.com/alecthomas/kong` | v1.15.0 | CLI dispatch | Declarative struct-tags, 4-subcommand fit in ~40 LOC, smallest stripped binary of big three CLI frameworks `[CITED: STACK.md]` |

**Phase 1 dependency graph:** `kong` is the ONLY runtime dependency imported in Phase 1. The `go-sdk`, `flock`, `renameio`, `testify` imports are deferred to their phases (5, 4, 4, 9 respectively) — pulling them in Phase 1 inflates the binary unnecessarily and makes the size gate a false positive when real code lands.

### Development Tooling (Phase 1 installs; all subsequent phases inherit)

| Tool | Version | Purpose | Notes |
|------|---------|---------|-------|
| `golangci-lint` | v2.x (v2 config syntax required) | Lint runner | Bundles `govet`, `staticcheck`, `forbidigo` as built-in linters — no separate installs needed `[VERIFIED: golangci-lint.run docs]`. v2 **merges staticcheck+stylecheck+gosimple into one linter called `staticcheck`** `[VERIFIED: golangci-lint v2 migration guide]` |
| `honnef.co/go/tools/cmd/staticcheck` | v2025.1.1 | Static analysis (standalone fallback) | Already included in `golangci-lint` v2 — only install standalone if opting out of the wrapper. Phase 1 uses the golangci-lint-bundled version |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `golangci-lint` wrapper | Direct `go vet ./... && staticcheck ./...` + custom forbidigo-like grep | +1 CI step simpler but loses forbidigo's AST-aware matching; `grep -rn "fmt.Print"` gives false positives inside comments/strings. Recommend wrapper. |
| `kong.VersionFlag` built-in type | Hand-rolled `--version` via custom `BeforeApply` method | `VersionFlag` is the library's own exported type — idiomatic, 2-line usage vs ~8 lines hand-rolled. No reason to hand-roll. `[VERIFIED: pkg.go.dev/github.com/alecthomas/kong VersionFlag type]` |
| `Makefile` | `justfile` / `Taskfile.yaml` / shell scripts only | Makefile has zero install footprint, universal on dev machines and CI runners, and declares dependencies between targets (build→size-check, build→startup-check). `just` requires an extra install. Recommend Makefile. |
| Shell `wc -c` for size | `stat -c '%s'` (Linux) / `stat -f '%z'` (macOS) | `wc -c < file` is POSIX-portable across Linux+macOS without flag variance. Recommend `wc -c`. |
| Bash `$EPOCHREALTIME` for timing | `hyperfine` statistical timer | `hyperfine` gives robust P95 but needs install step; `$EPOCHREALTIME` is bash 5+ built-in (Ubuntu 22.04 runners have bash 5.1). Recommend `$EPOCHREALTIME` loop with 5 runs, fail on max > 100ms. Add `hyperfine` later if the measurement proves flaky. |

### Installation

```bash
# Phase 1 module init
go mod init github.com/tkr41850-debug/mcp-chain  # placeholder - TBD replace with real owner
go mod tidy

# Phase 1 runtime dep
go get github.com/alecthomas/kong@v1.15.0

# Dev dep — install golangci-lint binary (CI uses action; local dev installs once)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Verify
golangci-lint version  # should say v2.x.y
```

**Version verification:** Confirm `kong` v1.15.0 is the current tag before writing go.sum:
```bash
go list -m -versions github.com/alecthomas/kong | tr ' ' '\n' | tail -5
```
STACK.md marks v1.15.0 as current as of 2026-04-23; re-verify on Phase 1 execution day.

## Architecture Patterns

### System Architecture Diagram

```
                ┌──────────────────────┐
  developer     │   go build ./...     │
  or CI         │   +ldflags="-s -w"   │
  invokes ────▶ │   +trimpath          │
                │   +-X main.version=X │
                └──────────┬───────────┘
                           │
                           ▼
              ┌──────────────────────────────┐
              │   mcp-chain  (stripped)      │
              │                              │
              │   main()  ────▶ log.SetOutput(os.Stderr)
              │       │         slog.SetDefault(stderrHandler)
              │       │
              │       ▼
              │   kong.Parse(&cli, kong.Vars{"version": version})
              │       │
              │       ├─ --version   ──▶ VersionFlag prints & exits 0
              │       ├─ serve       ──▶ ServeCmd.Run()   ─▶ exit 3 "not implemented"
              │       ├─ status <id> ──▶ StatusCmd.Run()  ─▶ exit 3
              │       ├─ list        ──▶ ListCmd.Run()    ─▶ exit 3
              │       └─ purge       ──▶ PurgeCmd.Run()   ─▶ exit 3
              │
              │   All "not implemented" writes to STDERR via slog; stdout stays clean.
              └──────────────────────────────┘

              ┌──────────────────────────────┐
  CI gate     │   .github/workflows/ci.yml   │
              │                              │
              │   jobs:                      │
              │     lint    ─▶ golangci-lint run (govet + staticcheck + forbidigo)
              │     build   ─▶ go build w/ ldflags
              │     size    ─▶ scripts/check-size.sh    (wc -c ≤ 15 728 640)
              │     startup ─▶ scripts/check-startup.sh (P95 of 5 runs ≤ 100ms)
              │     stdout  ─▶ scripts/check-stdout-silence.sh (mcp-chain serve </dev/null, stdout = 0 bytes)
              │                              │
              │   Any non-zero exit ─▶ block merge
              └──────────────────────────────┘
```

### Recommended Project Structure (Phase 1 close)

```
mcp-chain/
├── cmd/
│   └── mcp-chain/
│       └── main.go                  # ~40 LoC: stdout discipline + kong.Parse + dispatch
├── internal/
│   └── cli/
│       └── stubs.go                 # ServeCmd/StatusCmd/ListCmd/PurgeCmd with Run() → exit 3
├── scripts/
│   ├── check-size.sh                # binary size gate
│   ├── check-startup.sh             # startup time gate
│   └── check-stdout-silence.sh      # stdout-zero-bytes gate
├── .github/
│   └── workflows/
│       └── ci.yml                   # lint + build + 3 gates
├── .golangci.yml                    # govet + staticcheck + forbidigo config
├── .gitignore                       # /mcp-chain (binary), /dist/
├── Makefile                         # build, lint, test, size-check, startup-check, all
├── go.mod
└── go.sum
```

**What's explicitly NOT in Phase 1 (deferred):**
- `internal/store/`, `internal/wordlist/`, `internal/idgen/`, `internal/xdg/`, `internal/mcpserver/` — land in their respective phases
- `plugin/` — Phase 8
- `.goreleaser.yaml` — Phase 9
- `release.yml` workflow — Phase 9
- `README.md` beyond a placeholder — Phase 10

### Pattern 1: Stdout discipline bootstrap (MCP-02 core)

**What:** In `main()`, before anything else runs (before `kong.Parse`, before any package function call), hard-wire the two stdlib log outputs to `os.Stderr`.

**Why:** Go's default `log.Default()` writes to `os.Stderr` already — BUT any transitive dependency can mutate `log.SetOutput(os.Stdout)` at init time, and popular libraries sometimes do. `slog`'s default handler writes to `os.Stderr` too, but we replace the default handler explicitly to make the intent visible and greppable. Setting both at the TOP of main() makes the discipline robust against init-order surprises from future dependencies.

**Example (copy-paste-ready):**

```go
// cmd/mcp-chain/main.go
package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"

	"github.com/tkr41850-debug/mcp-chain/internal/cli"  // adjust module path
)

// version is set by -ldflags="-X main.version=..." at build time.
// Defaults to "dev" for local `go build` without ldflags.
var version = "dev"

// CLI is the root kong grammar. Every subcommand has a Run() method in internal/cli.
type CLI struct {
	Version kong.VersionFlag `help:"Print version and exit."`

	Serve  cli.ServeCmd  `cmd:"" help:"Run MCP stdio server."`
	Status cli.StatusCmd `cmd:"" help:"Check status of an id. Exits 0 resolved, 2 pending, 1 unknown."`
	List   cli.ListCmd   `cmd:"" help:"List all registered ids."`
	Purge  cli.PurgeCmd  `cmd:"" help:"Purge entries. Requires one of <id>, --all, or --resolved."`
}

func main() {
	// Stdout discipline: hard-set ALL logging to stderr BEFORE any other code runs.
	// MCP stdio reserves stdout for JSON-RPC; a single stray byte corrupts the wire.
	log.SetOutput(os.Stderr)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	var root CLI
	kctx := kong.Parse(&root,
		kong.Name("mcp-chain"),
		kong.Description("Chain Claude Code sessions via shared register/wait/resolve locks."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	)
	// kong.UsageOnError writes usage to os.Stderr by default — safe.
	// VersionFlag's BeforeReset hook prints "mcp-chain <version>" to os.Stdout
	// and exits; that's the ONE sanctioned stdout write in this binary until
	// Phase 5 adds MCP wire traffic.
	kctx.FatalIfErrorf(kctx.Run())
}
```

Source: [pkg.go.dev/github.com/alecthomas/kong](https://pkg.go.dev/github.com/alecthomas/kong) — `VersionFlag` type and `kong.Vars` interpolation pattern.

**Caveat about `VersionFlag` writing to stdout:** `kong.VersionFlag`'s default `BeforeReset` writes the version string to `os.Stdout`. This is the single sanctioned stdout write in Phase 1. When `mcp-chain serve </dev/null` runs (the stdout-silence gate), `--version` is NOT invoked — so stdout stays at zero bytes as required. The gate script explicitly invokes `serve`, not `--version`, to check this.

### Pattern 2: Subcommand stubs that honor stdout discipline

```go
// internal/cli/stubs.go
package cli

import (
	"fmt"
	"os"
)

// ExitCodeNotImplemented is returned by every Phase-1 subcommand stub.
// Phases 5 (serve), 6 (status), 7 (list/purge) replace this with real logic
// and real exit codes. We reserve 3 so it never collides with the documented
// 0/1/2 exit codes for `status` (CORE-01).
const ExitCodeNotImplemented = 3

type ServeCmd struct{}

func (c *ServeCmd) Run() error {
	// stderr write (via fmt.Fprintln to os.Stderr) is safe — it does NOT go to stdout.
	fmt.Fprintln(os.Stderr, "mcp-chain serve: not implemented (Phase 5)")
	os.Exit(ExitCodeNotImplemented)
	return nil // unreachable
}

type StatusCmd struct {
	ID string `arg:"" help:"Id to check."`
}

func (c *StatusCmd) Run() error {
	fmt.Fprintln(os.Stderr, "mcp-chain status: not implemented (Phase 6)")
	os.Exit(ExitCodeNotImplemented)
	return nil
}

type ListCmd struct{}

func (c *ListCmd) Run() error {
	fmt.Fprintln(os.Stderr, "mcp-chain list: not implemented (Phase 7)")
	os.Exit(ExitCodeNotImplemented)
	return nil
}

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
```

Why `fmt.Fprintln(os.Stderr, ...)` not `slog.Info(...)`: deliberate — this is a one-line operator message, not structured application logging. It's the pattern `go` tooling itself uses for "not implemented" messaging. `forbidigo` will forbid `fmt.Print*` (no `f`) but explicitly ALLOW `fmt.Fprintln(os.Stderr, ...)` via a tightly-scoped rule (see next section).

**Critical:** the `forbidigo` pattern must forbid `fmt.Print` / `fmt.Println` / `fmt.Printf` (bare, stdout-by-default) but NOT `fmt.Fprint*` (takes an explicit writer, safe). See `.golangci.yml` section below.

### Pattern 3: Version injection via ldflags

**Build command (makefile target, also used by CI):**

```makefile
# Makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GO_LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build
build:
	go build -trimpath -ldflags="$(GO_LDFLAGS)" -o mcp-chain ./cmd/mcp-chain
```

`-s` strips symbol table, `-w` strips DWARF debug info — together ≈ 25% size reduction `[CITED: PITFALLS.md #12]`. `-trimpath` removes absolute-path leak in binaries (reproducible builds).

GoReleaser in Phase 9 uses the same ldflags spec — this Makefile pattern stays working.

### Anti-Patterns to Avoid

- **`init()` for stdout setup:** tempting to put `log.SetOutput(os.Stderr)` in a package init(). Don't — init order across packages is unspecified, and a dep's init() could run before ours. Put it in `main()` first line `[CITED: PITFALLS.md #13]`.
- **Hard-coding version at source level:** `const version = "0.1.0"` in source drifts from git tags. Use `var version = "dev"` + ldflags override `[VERIFIED: kong docs on `kong.Vars` pattern]`.
- **`go build` at repo root with `main.go` at root:** breaks `go build ./cmd/mcp-chain` pattern and blurs `cmd/` vs `internal/` boundary `[CITED: Go module layout docs https://go.dev/doc/modules/layout]`.
- **Enabling all golangci-lint linters (`default: all`):** produces noise and slows CI. Start with `default: standard` + explicit `enable: [forbidigo]` `[VERIFIED: golangci-lint v2 docs]`.
- **Running `go vet` and `staticcheck` as separate CI steps when golangci-lint is already configured:** wastes CI time; v2 golangci-lint runs both under one invocation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Forbidden-function detection | Grep / sed script in CI | `forbidigo` in golangci-lint | AST-aware — doesn't false-positive on strings/comments; supports per-package scope |
| `--version` flag | Manual `os.Args[1] == "--version"` prefix check | `kong.VersionFlag` + `kong.Vars` | 2-line idiomatic kong pattern; handles `-V`, help text, etc. |
| Binary size measurement | `ls -la` parsing | `wc -c < file` | POSIX portable, single number output, no flag variance |
| Startup timing in CI | Manually timing via date-math | Bash `$EPOCHREALTIME` (bash 5.0+) | Sub-ms precision, no subprocess fork, no install needed on GitHub runners |
| Exit-code constants | Magic numbers | Named constant (`ExitCodeNotImplemented = 3`) + package-level docs | Phases 5/6/7 replace stubs cleanly without constant collision with `status`'s 0/1/2 |

**Key insight:** Phase 1 is small enough that every "shortcut" (hand-rolled --version, ad-hoc size check, inline grep for forbidden functions) costs MORE than the standard tool. The standard tools are already installed on the GitHub Actions Ubuntu runner — golangci-lint has an official action, Go is pre-installed, bash 5 is default.

## Runtime State Inventory

Greenfield phase — **section not applicable**. No existing code, no existing data, no runtime state to migrate. `go mod init` creates the module from scratch.

## Common Pitfalls

### Pitfall 1: Binary bloat from transitive `net/http` pull

**What goes wrong:** Adding a dependency in Phase 1 that transitively pulls `net/http` balloons the binary from ~2 MB (hello-world) to 7–10 MB, eating into the 15 MB budget before Phase 5's MCP SDK arrives.

**Why it happens:** `kong` itself is ~3.4 MB stripped and pulls no `net/http`. But if a dev casually adds `github.com/stretchr/testify` to the main package (not test files), it pulls `net/http` indirectly via `testify/http` subpackages. `go-sdk` may also pull `net/http` for HTTP transport even when stdio-only is used.

**How to avoid:**
- Phase 1 runtime dep list is locked to **kong only**. No other imports in `cmd/mcp-chain/main.go` or `internal/cli/stubs.go` besides stdlib + kong.
- CI gate: `go list -deps ./... | grep -q '^net/http$' && exit 1 || exit 0` — fails Phase 1 build if `net/http` appears anywhere. This gate continues into Phase 5; MCP-01 requires `net/http` NOT appear even after MCP SDK lands (stdio-only; SDK must not force HTTP transport code path).
- Measure early: after `go build`, run `go tool nm -size ./mcp-chain | sort -n | tail -30` to see top-30 largest symbols. Single-file projects should dominate; any third-party dep taking > 500 KB is a red flag.

**Warning signs:** Phase 1 binary > 8 MB (should be 3–5 MB with only kong). `go.sum` has more than 2–3 transitive dep lines.

### Pitfall 2: `log.SetOutput` in package init runs too late

**What goes wrong:** Developer puts `func init() { log.SetOutput(os.Stderr) }` in `internal/cli/init.go`. Another dep's init() runs first, logs to stdout, corrupts wire before `main()` sees control.

**Why it happens:** Go init() order across packages is specified only by import graph — not by package name or file name. A dep imported transitively from a blank-imported package can run first.

**How to avoid:**
- Put `log.SetOutput(os.Stderr)` and `slog.SetDefault(...)` as the FIRST two statements in `main()`. This runs before Go dispatches control to anything.
- Test it: add a `package-init-test.go` that does `init() { fmt.Fprintln(os.Stdout, "SHOULD NOT APPEAR") }` in a leaf package, run `./mcp-chain serve </dev/null >/tmp/out 2>/dev/null` — `/tmp/out` must still be empty because main() redirected `log` to stderr, but fmt.Fprintln bypassing log is not caught. This confirms our linter must forbid `fmt.Print*` anywhere in the module.

**Warning signs:** Any init() function that touches `log.*` or does I/O. Only variable initialization belongs in init() for this project.

### Pitfall 3: `forbidigo` pattern too narrow — misses `println()` built-in

**What goes wrong:** Forbidigo default pattern is `^(fmt\.Print(|f|ln)|print|println)$` but dev overrides it without the bare `print`/`println` builtins, so `println("debug")` slips through. Go's `println` built-in writes to stderr (safe) but `print` built-in also writes to stderr — both fine from an MCP perspective but flagged here because they're debugging residue that betrays unfinished code.

**Why it happens:** Developers think "the built-in `println` goes to stderr, so it's fine" — forgetting PROJECT.md's signal-not-noise goal. Also sometimes a library uses `print` as a receiver name and the linter complains on false positive.

**How to avoid:** Keep the default forbidigo `forbid` list AND extend it with a project-specific pattern that scopes to `cmd/` and `internal/cli/` only (the serve-path). Leave `internal/store/` etc. unrestricted for debug-scaffolding phases (Phases 2–4 may temporarily use fmt.Println during development; gate re-enables at Phase 5 close).

**Warning signs:** `golangci-lint run` passes but `./mcp-chain serve </dev/null >/tmp/out 2>/dev/null && [ -s /tmp/out ]` is non-empty.

### Pitfall 4: GitHub Actions cache miss inflates startup timing false positives

**What goes wrong:** The first run in a fresh container has cold page cache, which the 100-ms budget is defined against — good. But on the second run of the same workflow (re-run on a PR), the Go binary is already cached, so timing drops to ~5 ms and the gate passes trivially without actually exercising cold-cache behavior.

**Why it happens:** GitHub-hosted runners don't persist state across jobs, but WITHIN a job, the cache is warm after first invocation. A 5-run-P95 measurement could report 8 ms when real-world cold invocation is 90 ms.

**How to avoid:**
- For the startup gate, prefix each timing iteration with `sync` (flush pending writes) — doesn't drop cache but is a weak signal.
- Use `/usr/bin/time -f '%e'` with a loop that does `cp ./mcp-chain /tmp/mcp-chain-$i && /tmp/mcp-chain-$i --version` — copying to a fresh inode on each iteration approximates cold cache.
- **Preferred:** accept the measurement limitation. The startup gate's PRIMARY purpose is regression detection. A 5-run loop with P95 ≤ 100 ms is a strict-enough ceiling that if a regression inflates it to even 50 ms (still warm-cache), the gate flags. Document this in `scripts/check-startup.sh` header.
- Do NOT attempt `echo 3 > /proc/sys/vm/drop_caches` — requires root, not available on GitHub runners without `sudo`.

**Warning signs:** `--version` local timing reports < 10 ms on every run. That's fine from a pass/fail standpoint but means the gate isn't actually exercising cold cache. Budget regression to 110 ms is what the gate catches; cold-vs-warm nuance is Phase 10 polish.

### Pitfall 5: Kong's help text defaults to stdout on `--help`, breaking MCP-02 semantics

**What goes wrong:** `mcp-chain --help` or a usage error prints to stdout by default in many CLI libraries. For mcp-chain specifically, `mcp-chain --help` running under a live MCP stdio session would corrupt the wire.

**Why it happens:** `mcp-chain --help` isn't meant to run during `serve` — Claude Code runs `mcp-chain serve`, not `mcp-chain --help`. But a future user runs `mcp-chain` with no args; kong's default is to print usage to stdout and exit.

**How to avoid:**
- Pass `kong.UsageOnError()` — routes usage output to stderr on parse error `[VERIFIED: kong options docs]`.
- Verify behavior: `./mcp-chain --help 2>/dev/null` should output nothing to stdout (help should all go to stderr). If kong's default routes `--help` to stdout, wrap with `kong.Writers(os.Stderr, os.Stderr)` (sets both output and error writers to stderr) — safe because Phase 1 has no legitimate stdout use case.
- Best defense: the stdout-silence gate (`mcp-chain serve </dev/null`, stdout = 0 bytes) checks the precise scenario MCP cares about. `--help` corruption is a different scenario (Claude Code doesn't run `--help` during serve). Document the behavior, don't over-engineer.

### Pitfall 6: Placeholder module path leaks to released binaries

**What goes wrong:** `go mod init github.com/tkr41850-debug/mcp-chain` uses a placeholder owner. Dev forgets to update before Phase 9 release; import paths throughout the codebase use the wrong path; the binary "works" but `go install github.com/real-owner/mcp-chain@latest` fails for external users.

**Why it happens:** Placeholders feel temporary; real owner is "obvious" at commit time and nobody checks.

**How to avoid:**
- `[ASSUMED: module path is github.com/tkr41850-debug/mcp-chain]` — flag in RESEARCH.md, planner adds a Phase 1 task "confirm actual module path with user before `go mod init`"; if not resolved, ship with `github.com/tkr41850-debug/mcp-chain` and `[ASSUMED]` tag in the plan so it's caught.
- Add a Phase 1 gate: grep `go.mod` first line and `cmd/mcp-chain/main.go` import paths match. Easy shell check.

## Code Examples

### Complete `.golangci.yml` (Phase 1 baseline)

```yaml
# .golangci.yml — Phase 1 baseline lint config.
# v2 config format (top-level `version: "2"` required).
# Enables govet + staticcheck + forbidigo.
# Expands at Phase 5+ with import restrictions (ban net/http) and tool-desc token budget probes.
version: "2"

run:
  timeout: 5m
  tests: true

linters:
  default: standard
  enable:
    - forbidigo
    - staticcheck  # v2 bundles stylecheck + gosimple under this name
    # govet is in the default `standard` set already; listed for visibility.
    - govet

  settings:
    forbidigo:
      # Forbid fmt.Print / Println / Printf (bare — write to stdout by default).
      # Allow fmt.Fprint* (takes explicit writer — safe).
      # Forbid bare `print` and `println` builtins (debug residue, MCP-wire risk).
      forbid:
        - pattern: ^fmt\.Print(|f|ln)$
          msg: "Use fmt.Fprint*(os.Stderr, ...) or slog; stdout is reserved for MCP wire traffic (MCP-02)."
        - pattern: ^println$
          msg: "println() is debug residue; use slog.Info or fmt.Fprintln(os.Stderr, ...)."
        - pattern: ^print$
          msg: "print() is debug residue; use slog.Info or fmt.Fprintln(os.Stderr, ...)."
      analyze-types: true
      exclude-godoc-examples: true

    staticcheck:
      # Defaults are fine for Phase 1. Phase 5+ may add SA1019-ignore for specific
      # go-sdk APIs if they land as deprecated-but-stable.
      checks:
        - all

    govet:
      enable:
        - shadow
        # Default govet checks apply; shadow is opt-in and helpful.

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

**Scoping forbidigo to specific paths (optional Phase 1 refinement):** If Phase 2–4 temporarily need debug prints in `internal/store/`, add a path-based exclude in `issues.exclude-rules`. Phase 1 starts with no exclusions — forbidigo applies module-wide. Tighter scoping is reserved for Phase 5 close.

Source: [golangci-lint v2 configuration docs](https://golangci-lint.run/docs/linters/configuration/) + [forbidigo settings](https://golangci-lint.run/docs/linters/configuration/#forbidigo).

### Complete `.github/workflows/ci.yml`

```yaml
# .github/workflows/ci.yml — Phase 1 CI.
# Runs on push and PR. Linux-only by design; Phase 9 expands to the cross-compile matrix.
name: ci

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v8
        with:
          version: latest   # pins to v2.x current; pin to a specific version (e.g. v2.6.0) before release
          args: --timeout=5m

  build-and-gate:
    name: Build + Gates
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true

      - name: Build (stripped + trimpath)
        run: make build

      - name: Size gate (≤ 15 MB stripped)
        run: ./scripts/check-size.sh ./mcp-chain

      - name: Startup gate (≤ 100 ms P95 of 5 runs)
        run: ./scripts/check-startup.sh ./mcp-chain

      - name: Stdout silence gate (serve </dev/null writes 0 bytes to stdout)
        run: ./scripts/check-stdout-silence.sh ./mcp-chain

      - name: Ban net/http import (MCP-01 prep; enforced from Phase 1)
        run: |
          if go list -f '{{ join .Deps "\n" }}' ./... | grep -q '^net/http$'; then
            echo "ERROR: net/http is in the dependency graph. stdio-only; see MCP-01."
            exit 1
          fi
```

Source: [golangci-lint GitHub Action docs](https://github.com/golangci/golangci-lint-action) — `golangci-lint-action@v8` is current for v2 configs; v7 and earlier are v1-only.

### `scripts/check-size.sh`

```bash
#!/usr/bin/env bash
# scripts/check-size.sh — fail if stripped binary exceeds 15 MB.
# Usage: check-size.sh <path-to-binary>
set -euo pipefail

BIN="${1:?usage: check-size.sh <binary>}"
MAX_BYTES=15728640  # 15 * 1024 * 1024

if [[ ! -f "$BIN" ]]; then
  echo "ERROR: binary not found: $BIN" >&2
  exit 2
fi

SIZE=$(wc -c < "$BIN")

printf 'binary: %s\n' "$BIN"
printf 'size:   %d bytes (%.2f MB)\n' "$SIZE" "$(awk "BEGIN {print $SIZE/1048576}")"
printf 'limit:  %d bytes (15 MB)\n' "$MAX_BYTES"

if (( SIZE > MAX_BYTES )); then
  echo "FAIL: binary exceeds 15 MB budget (DIST-03)." >&2
  exit 1
fi

echo "OK: within budget."
```

### `scripts/check-startup.sh`

```bash
#!/usr/bin/env bash
# scripts/check-startup.sh — fail if `binary --version` startup exceeds 100 ms P95 of 5 runs.
# Uses bash 5's $EPOCHREALTIME (microsecond-precision wall clock).
# Usage: check-startup.sh <path-to-binary>
#
# Measurement caveats (PITFALLS.md #4):
# - CI runners are warm-cache after first invocation. The P95 of 5 is a regression
#   gate, not a cold-cache simulation. A 10ms→110ms regression will still fail.
# - If flakes appear, swap to `hyperfine --warmup 1 --runs 20 --max-runs 20` and
#   parse its JSON output. For Phase 1, keep it dependency-free.
set -euo pipefail

BIN="${1:?usage: check-startup.sh <binary>}"
MAX_MS=100
RUNS=5

if [[ ! -f "$BIN" ]]; then
  echo "ERROR: binary not found: $BIN" >&2
  exit 2
fi

# bash 5 $EPOCHREALTIME produces "SECONDS.MICROSECONDS" e.g. "1714068000.123456"
if [[ -z "${EPOCHREALTIME:-}" ]]; then
  echo "ERROR: bash 5.0+ required (no \$EPOCHREALTIME). Install bash 5 or use hyperfine." >&2
  exit 2
fi

declare -a TIMES_MS
echo "measuring ${BIN} --version (${RUNS} runs):"
for ((i=1; i<=RUNS; i++)); do
  START="$EPOCHREALTIME"
  "$BIN" --version >/dev/null 2>&1
  END="$EPOCHREALTIME"
  # Multiply (END - START) by 1000 to get ms. awk handles the float math.
  MS=$(awk "BEGIN { printf \"%.3f\", ($END - $START) * 1000 }")
  TIMES_MS+=("$MS")
  printf '  run %d: %s ms\n' "$i" "$MS"
done

# Compute max — with only 5 runs, max IS the P95. For larger N use a sort+index.
MAX=$(printf '%s\n' "${TIMES_MS[@]}" | sort -g | tail -1)

printf 'max (P95 of %d): %s ms\n' "$RUNS" "$MAX"
printf 'limit:           %d ms\n' "$MAX_MS"

# Integer comparison via awk (bash can't compare floats).
if awk "BEGIN { exit !($MAX > $MAX_MS) }"; then
  echo "FAIL: startup exceeds ${MAX_MS}ms budget (DIST-03)." >&2
  exit 1
fi

echo "OK: within budget."
```

### `scripts/check-stdout-silence.sh`

```bash
#!/usr/bin/env bash
# scripts/check-stdout-silence.sh — assert `mcp-chain serve </dev/null` writes 0 bytes to stdout.
# This is the Phase 1 validator for MCP-02 (stdout discipline).
#
# Why this works even though `serve` isn't implemented yet:
#   - Phase 1 stub exits immediately with code 3 ("not implemented") after writing
#     ONLY to stderr (via fmt.Fprintln(os.Stderr, ...)).
#   - The test captures stdout into a tmp file and asserts the file is zero bytes.
#   - If someone in Phase 5 accidentally adds a fmt.Println, this gate catches the
#     regression — the gate carries forward unchanged.
set -euo pipefail

BIN="${1:?usage: check-stdout-silence.sh <binary>}"

if [[ ! -f "$BIN" ]]; then
  echo "ERROR: binary not found: $BIN" >&2
  exit 2
fi

STDOUT=$(mktemp)
STDERR=$(mktemp)
trap 'rm -f "$STDOUT" "$STDERR"' EXIT

# serve stub exits 3; we tolerate that exit code and check only stdout contents.
"$BIN" serve </dev/null >"$STDOUT" 2>"$STDERR" || true

STDOUT_BYTES=$(wc -c < "$STDOUT")

printf 'stdout bytes: %d (expect: 0)\n' "$STDOUT_BYTES"

if (( STDOUT_BYTES > 0 )); then
  echo "FAIL: serve wrote $STDOUT_BYTES bytes to stdout — MCP-02 violation." >&2
  echo "--- stdout contents ---" >&2
  cat "$STDOUT" >&2
  exit 1
fi

echo "OK: stdout is silent."
```

### Complete `Makefile`

```makefile
# Makefile — mcp-chain Phase 1 targets.
# Targets usable locally and from CI. CI workflow calls `make build` + gate scripts directly.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GO_LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all
all: lint build size-check startup-check stdout-check

.PHONY: build
build:
	go build -trimpath -ldflags="$(GO_LDFLAGS)" -o mcp-chain ./cmd/mcp-chain

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: size-check
size-check: build
	./scripts/check-size.sh ./mcp-chain

.PHONY: startup-check
startup-check: build
	./scripts/check-startup.sh ./mcp-chain

.PHONY: stdout-check
stdout-check: build
	./scripts/check-stdout-silence.sh ./mcp-chain

.PHONY: clean
clean:
	rm -f mcp-chain
	rm -rf dist/

.PHONY: tidy
tidy:
	go mod tidy
```

### `.gitignore`

```gitignore
# Phase 1 binary artifacts
/mcp-chain
/mcp-chain.exe
/dist/

# Go coverage
*.out
coverage.*

# Editor & OS
.idea/
.vscode/
*.swp
.DS_Store
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `golangci-lint` v1 with separate `staticcheck` + `gosimple` + `stylecheck` linters | v2 merges these into single `staticcheck` linter | v2 release 2025 `[VERIFIED: golangci-lint changelog]` | Simpler `.golangci.yml`; remove `gosimple` and `stylecheck` entries |
| `log` only for logging | `log/slog` (stdlib, Go 1.21+) for structured logging with stderr default | Go 1.21, 2023 | Use `slog` for app logs; `log` kept only as belt-and-suspenders `SetOutput` call |
| Hand-rolled `--version` with os.Args check | `kong.VersionFlag` built-in type | kong v0.8+ | 2-line wiring via `kong.Vars{"version": ...}` |
| `github.com/pkg/errors` for wrap/unwrap | stdlib `errors.Is` / `errors.As` / `fmt.Errorf("...: %w", err)` | Go 1.13, 2019 | Zero deps for error wrapping; `pkg/errors` is deprecated `[CITED: STACK.md "What NOT to Use"]` |

**Deprecated/outdated patterns to avoid in Phase 1:**
- `log.Fatal` in serve path — writes to stderr then exits, but may interleave with buffered logger state — safe in Phase 1 stubs but flag for Phase 5 review
- `panic` for operator errors — use `os.Exit(code)` with explicit exit codes
- `init()` functions for "setup" work — push to `main()` first statements

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Module path is `github.com/tkr41850-debug/mcp-chain` | Installation, main.go import example | Import paths throughout codebase need find-replace; cheap at Phase 1 scale but grows with every phase. **Resolve with user on Phase 1 kickoff.** |
| A2 | Go version floor is 1.23 | Standard Stack, ci.yml | Lower (1.22) would work for Phase 1 but fails when `go-sdk` v1.5.0 lands in Phase 5 (SDK declares 1.23 minimum per STACK.md). Setting 1.23 now is forward-compatible. LOW risk. |
| A3 | `$EPOCHREALTIME` is available on GitHub-hosted ubuntu-latest runners | check-startup.sh | If bash is < 5, script exits with setup error. Fallback: `/usr/bin/time -f '%e'` or `hyperfine`. Verified `ubuntu-22.04` and `ubuntu-latest` (= 24.04 as of 2026) ship bash 5.x. LOW risk. |
| A4 | GitHub Actions Ubuntu runner startup-time P95 of 5 runs is a stable measurement | check-startup.sh, Pitfall 4 | If flaky (rare false positives), Phase 1 may need re-tuning. Recommend monitoring first 10 CI runs post-Phase-1-close; if P95 variance > 30ms, swap to `hyperfine`. MEDIUM risk. |
| A5 | `kong.VersionFlag`'s default `BeforeReset` writes to `os.Stdout` and exits with code 0 | Pattern 1 caveat | Kong internal implementation detail — verified via [pkg.go.dev kong.VersionFlag docs] but API wording ambiguous on "exits" vs "returns error". If VersionFlag returns instead of exiting, our `kctx.Run()` path proceeds to dispatch a no-subcommand state, which kong handles with usage-to-stderr. LOW risk either way (both paths end with stderr, not stdout of any stray bytes; just exit-code differs). |
| A6 | `golangci-lint v2` is stable and current at Phase 1 execution time | .golangci.yml, ci.yml | v2 was released in 2025 per web search; stable as of 2026-04. Action pinning to `latest` is acceptable; re-evaluate pin to concrete version (e.g. v2.6.0) before first release tag. LOW risk. |
| A7 | Exit code 3 for "not implemented" does not collide with kong's error codes | internal/cli/stubs.go | kong docs don't specify reserved exit codes. kong's `FatalIfErrorf` exits with code 1 on parse error. Our stubs use `os.Exit(3)` directly; the path through kong's error handling uses exit 1. No collision with status's documented 0/1/2. LOW risk. |

## Open Questions (RESOLVED)

1. **Actual GitHub owner for `go mod init` module path**
   - What we know: Placeholder `github.com/tkr41850-debug/mcp-chain` is what ARCHITECTURE.md examples use; PROJECT.md says distribution is via "a public GitHub repo" but doesn't name the owner.
   - RESOLVED: Plan 01-01 Task 1 derives the module path from `git remote get-url origin` at execution time. If no remote is set, use placeholder `github.com/tkr41850-debug/mcp-chain` and add a Phase 10 README task to re-verify the module path matches the real GitHub repo. No user prompt required.

2. **Should Phase 1 include a `test` job in CI, given there are no tests yet?**
   - What we know: REQUIREMENTS.md QA-03 says `go test -race ./...` runs in CI on push/PR. Traceability table maps QA-03 to Phase 9.
   - RESOLVED: YES — wire `go test ./...` (no `-race`) in Phase 1 CI. No-op today; natural home for tests from Phase 2 forward. Implemented in Plan 01-03's ci.yml. The `-race` flag is deferred to Phase 9 per QA-03.

3. **Does `golangci-lint-action` cache golangci-lint's own binary across runs?**
   - What we know: The action has built-in caching of the lint results and the binary.
   - RESOLVED: Accept defaults — no explicit cache configuration needed. First CI run will be slow (~2 min); subsequent runs hit cache. Re-evaluate if first 10 CI runs show consistent cold-cache pain; otherwise close.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All Phase 1 work | Not installed locally (verified via `command -v go`) | — | Install Go 1.23 before Phase 1 starts; GitHub runners have it pre-installed via `actions/setup-go@v5` |
| `golangci-lint` v2 | Lint gate | Not installed locally | — | GitHub runner installs via `golangci-lint-action@v8`; dev installs via `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` |
| Bash 5.0+ (for `$EPOCHREALTIME`) | `scripts/check-startup.sh` | Not verified locally (alpine may ship bash 5+; check at execution time) | — | `/usr/bin/time -f '%e'` (GNU time; available on Ubuntu runners) |
| `wc`, `awk`, `sort` | Gate scripts | Standard on any Linux/macOS | POSIX | None needed |
| `git` | Makefile `VERSION ?= $(shell git describe...)` | Standard | — | Makefile falls back to "dev" if git describe fails |
| `hyperfine` | Optional — alternative startup timer | Not installed | — | `$EPOCHREALTIME` is sufficient; hyperfine only if measurement proves flaky |

**Missing dependencies with fallback:**
- Local Go install — CI does not require local Go; developer installs when needed. Not blocking for Phase 1 planning.

**Missing dependencies with no fallback:**
- None. All Phase 1 gates run on GitHub-hosted Ubuntu runners which have all required tools pre-installed.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | stdlib `testing` (Phase 1 has no test files yet; framework is chosen but unexercised) |
| Config file | none — stdlib testing needs no config |
| Quick run command | `go test ./...` |
| Full suite command | `go test -race -count=1 -timeout 60s ./...` (race flag enforced in Phase 9) |

### Phase Requirements → Test Map

Phase 1 is predominantly infrastructure; most validation is via shell-script CI gates, not Go tests. The REQ-to-test mapping reflects this.

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CORE-01 | `mcp-chain --version` prints version and exits 0 | smoke | `./mcp-chain --version \| grep -qE '^mcp-chain'` | ❌ Wave 0 (`scripts/check-version.sh` or inline in ci.yml) |
| CORE-01 | `mcp-chain serve\|status\|list\|purge` dispatch to stubs | smoke | `./mcp-chain serve </dev/null; [ $? -eq 3 ]` (and equivalents) | ❌ Wave 0 (`scripts/check-stubs.sh`) |
| MCP-02 | `mcp-chain serve </dev/null` writes 0 bytes to stdout | smoke | `./scripts/check-stdout-silence.sh ./mcp-chain` | ✅ written above |
| DIST-03 | Binary ≤ 15 MB stripped | smoke | `./scripts/check-size.sh ./mcp-chain` | ✅ written above |
| DIST-03 | Startup ≤ 100 ms (P95 of 5) | smoke | `./scripts/check-startup.sh ./mcp-chain` | ✅ written above |
| QA-04 | `go vet` clean | unit | `golangci-lint run` (covers govet) | ✅ via .golangci.yml |
| QA-04 | `staticcheck` clean | unit | `golangci-lint run` (covers staticcheck) | ✅ via .golangci.yml |
| QA-04 | `forbidigo` rejects `fmt.Print*` in stubs | unit | `golangci-lint run` (covers forbidigo) | ✅ via .golangci.yml |
| — | `net/http` not in dep graph | smoke | `go list -f '{{ join .Deps "\n" }}' ./... \| grep -q '^net/http$' && exit 1` | ✅ inline in ci.yml |

### Sampling Rate
- **Per task commit:** `make lint && make build && make size-check` (≤ 30 seconds total on dev box)
- **Per wave merge:** `make all` (lint + build + size + startup + stdout gates; ~1 min)
- **Phase gate:** Full CI workflow green (all jobs pass) before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `scripts/check-size.sh` — covers DIST-03 (size); **authored in this research, needs to be written as a file in Wave 1**
- [ ] `scripts/check-startup.sh` — covers DIST-03 (startup)
- [ ] `scripts/check-stdout-silence.sh` — covers MCP-02
- [ ] `.golangci.yml` — covers QA-04 (lint)
- [ ] `.github/workflows/ci.yml` — orchestrates all gates
- [ ] `Makefile` — developer-facing orchestration
- [ ] Framework install: N/A (stdlib `testing` is built-in; no test files in Phase 1; `go test ./...` is a no-op that still succeeds)
- [ ] Optional: `scripts/check-stubs.sh` — verifies each subcommand stub exits with code 3 (covers CORE-01 skeleton requirement); can be inlined in ci.yml if preferred

**Framework-install requirement:** None. `go test` is built-in; Phase 1 has no `*_test.go` files yet (tests land from Phase 2 onward). The CI workflow can still run `go test ./...` as a no-op that succeeds trivially — preps the plumbing for Phase 2.

## Project Constraints (from CLAUDE.md)

Extracted actionable directives from `/home/alpine/mcp-chain/CLAUDE.md` that constrain Phase 1 implementation:

| Directive | Source | Compliance Path |
|-----------|--------|-----------------|
| Go, pure-Go deps only (no cgo) | Project "Constraints" | Phase 1 imports: `kong` only. `kong` is pure-Go. No cgo-adjacent deps this phase. |
| Stripped binary ≤ 15 MB | Project "Constraints" | `scripts/check-size.sh` enforces. |
| Cold startup ≤ 100 ms | Project "Constraints" | `scripts/check-startup.sh` enforces (with measurement caveat per Pitfall 4). |
| Resident memory ≤ 20 MB | Project "Constraints" | No Phase 1 gate — RSS measurement deferred to Phase 5 (when real allocations start). Phase 1 binary's RSS is trivial (~5 MB). |
| MCP tool descriptions / model-facing text kept bare-minimum | Project "Constraints" | N/A Phase 1 — no MCP tools yet. Phase 5 owns. |
| Linux + macOS primary; Windows via CI cross-compile | Project "Constraints" | Phase 1 CI: Linux only. Full matrix is Phase 9. Documented. |
| Stdlib-first; only add dep if meaningful | Project "Constraints" | Phase 1 adds only `kong`. `stretchr/testify` deferred to Phase 2+ when first test file lands. `forbidigo`/`staticcheck`/`golangci-lint` are dev tools, not runtime deps. |
| Distribute via Claude Code plugin from public GitHub repo | Project "Constraints" | Phase 1 does not ship — Phase 8 plugin packaging + Phase 9 release. Scaffold's module path must be public-GitHub-URL-compatible (`github.com/<owner>/mcp-chain`). |
| **GSD workflow enforcement: don't bypass `/gsd-*` for file edits** | CLAUDE.md "GSD Workflow Enforcement" | Planning enforcement — research & planning agents respect. Implementer (`gsd-task-executor`) reads the same CLAUDE.md. |

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/github.com/alecthomas/kong](https://pkg.go.dev/github.com/alecthomas/kong) — `VersionFlag`, `kong.Vars`, `Run()` dispatch pattern. Source confirmed via WebFetch 2026-04-23.
- [github.com/alecthomas/kong README](https://github.com/alecthomas/kong/blob/master/README.md) — subcommand dispatch, `Run(*Context)` idiom, `kong.UsageOnError()`
- [Go module layout — go.dev](https://go.dev/doc/modules/layout) — `cmd/<name>/main.go` convention
- [golangci-lint v2 configuration docs](https://golangci-lint.run/docs/linters/configuration/) — forbidigo settings, staticcheck+stylecheck merger, govet options
- [golangci-lint v2 configuration file format](https://golangci-lint.run/docs/configuration/file/) — `version: "2"`, `linters.default`, `linters.enable`
- [golangci-lint migration guide v1→v2](https://golangci-lint.run/docs/product/migration-guide/) — staticcheck+stylecheck+gosimple merger
- [github.com/golangci/golangci-lint-action](https://github.com/golangci/golangci-lint-action) — `@v8` pin for v2 configs
- [actions/setup-go@v5](https://github.com/actions/setup-go) — Go install + cache on GitHub runners
- [STACK.md](../../research/STACK.md) — pinned versions, pure-Go dep set, ldflags recipe
- [PITFALLS.md](../../research/PITFALLS.md) — pitfalls #1 stdout corruption, #12 binary bloat, #13 startup init

### Secondary (MEDIUM confidence)
- [Daniel Michaels — How I write Go CLI tools today (using Kong)](https://danielms.site/zet/2024/how-i-write-golang-cli-tools-today-using-kong/) — community idioms for kong usage, cross-check for version-flag pattern
- [PITFALLS.md #4](../../research/PITFALLS.md) — verbose MCP tool descriptions (applies from Phase 5, not Phase 1 directly, but informs the Phase 1 infrastructure that supports the Phase 5 budget gate)

### Tertiary (LOW confidence — needs validation during implementation)
- `$EPOCHREALTIME` stability on GitHub Actions ubuntu-latest (Assumption A4) — verify by running `bash -c 'echo $BASH_VERSION; echo $EPOCHREALTIME'` in a test workflow during Phase 1 execution
- Actual GitHub owner for module path (Assumption A1) — needs user confirmation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — kong/go-vet/staticcheck/forbidigo are mature tools with stable APIs, all verified via current docs
- Architecture: HIGH — module layout is well-established Go convention; stub-with-exit-3 pattern is trivially correct
- Pitfalls: HIGH for structural pitfalls (stdout corruption, init ordering, binary bloat); MEDIUM for specific numeric thresholds (which come from PROJECT.md as design targets, not empirical measurements — confirm first CI run)
- CI patterns: HIGH — GitHub Actions + golangci-lint-action + setup-go are the de facto standard, widely documented

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (30 days — stable infrastructure; only concern is a golangci-lint v2 breaking release, which is not on the roadmap per changelog)
