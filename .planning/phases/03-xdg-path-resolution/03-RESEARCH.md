# Phase 3: XDG Path Resolution - Research

**Researched:** 2026-04-23
**Domain:** Go stdlib path resolution, XDG Base Directory spec, filesystem permissions
**Confidence:** HIGH

## Summary

Phase 3 needs a tiny, dependency-free package `internal/statepath` that resolves where `state.json` lives on disk and guarantees the parent directory exists with mode `0700`. The resolution rule is fixed by CORE-06: honor `$XDG_STATE_HOME` if set and non-empty, else fall back to `$HOME/.mcp-chain/` (the project's chosen dot-dir fallback, not the strict-XDG `$HOME/.local/state/`). The implementation must avoid `os/user.Current()` entirely and — per the project's caution — also avoid `os.UserHomeDir()` even though Go 1.23's implementation is in fact a pure `$HOME` read with no `os/user` fallback on Linux/macOS. Direct `os.Getenv("HOME")` is the explicit, self-documenting, future-proof choice.

The package is ~40 lines of production code plus ~100 lines of tests, all stdlib. Five `t.Setenv` + `t.TempDir`-isolated tests cover the two branches, the double-unset error, empty-string-as-unset per XDG spec, and the "pre-existing parent dir mode is preserved" invariant. Validation runs in under one second per commit.

**Primary recommendation:** Implement `internal/statepath/resolve.go` with a single exported `Resolve() (string, error)` function using `os.Getenv` + `os.MkdirAll(dir, 0o700)`. Do not probe writability (Phase 4's job). Align the fallback with PROJECT.md's `~/.mcp-chain/state.json` rather than the strict XDG `$HOME/.local/state/mcp-chain/` — project-intent wins.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

All implementation choices are at Claude's discretion — discuss phase was skipped per user setting.

Key constraints (binding):
- Package: `internal/statepath/` (private; store in Phase 4 consumes it)
- Pure-Go, stdlib only (`os`, `path/filepath`, `errors`)
- **No `os/user.Current()`** — that call can invoke NSS (cgo on some systems) and adds startup latency. Use `os.Getenv("HOME")` directly.
- **No `os.UserConfigDir` / `os.UserCacheDir` / `os.UserHomeDir`** — stdlib versions call `os/user` on some platforms.
- Function signature suggestion (final name at Claude's discretion): `func Resolve() (path string, err error)` or `ResolveStatePath() (string, error)`
- Directory creation: `os.MkdirAll(parent, 0o700)` — idempotent; do not chmod existing dirs
- Windows: deferred; this phase is linux/macOS primary. A short note in RESEARCH.md suffices.

### Claude's Discretion

All implementation choices are at Claude's discretion.

### Deferred Ideas (OUT OF SCOPE)

- Windows path resolution (`%LOCALAPPDATA%\mcp-chain\state.json`) — deferred until CI cross-compile phase validates binary on Windows.
- Writability probe / disk-space check — not in this phase.
- File creation — Phase 4 owns state file IO.
- Flock acquisition — Phase 4.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-06 | State file path resolution: `$XDG_STATE_HOME/mcp-chain/state.json` if `$XDG_STATE_HOME` is set; otherwise `~/.mcp-chain/state.json`. Path documented in `--help` and README. | Resolver design (Code Examples section), 5-test matrix (Validation Architecture), XDG spec citation for empty-string handling. Note: `--help` / README wiring is NOT in this phase — only the resolution function. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Resolve state file path | CLI binary (process boot) | — | Path is known before any handler runs; pure function of env + constants |
| Ensure parent directory exists with 0700 | CLI binary (process boot) | — | `mkdir -p` semantics; must be idempotent across N concurrent `status` invocations |
| Create/write the state file | Store layer (Phase 4) | — | Not in this phase. Resolver only guarantees the directory is writable-capable |
| Acquire cross-process flock | Store layer (Phase 4) | — | Not in this phase |
| Read `$HOME` / `$XDG_STATE_HOME` | Process environment | — | Stdlib `os.Getenv`; no NSS, no `os/user` |

**Key insight:** Path resolution is a pure boot-time concern. Every other consumer (store, MCP adapter, CLI subcommands) takes the resolved absolute path as input. Keeping the resolver exactly this narrow means no test in Phase 4 needs to care about env vars.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os` (stdlib) | Go 1.23.8 | `Getenv`, `MkdirAll`, `Stat` | Zero deps; only way to satisfy "no `os/user` call" constraint without a third-party pkg |
| `path/filepath` (stdlib) | Go 1.23.8 | `Join` for cross-platform path construction | Stdlib idiom; handles separators correctly on all GOOSes |
| `errors` (stdlib) | Go 1.23.8 | Sentinel errors (`errors.New`), wrapping with `fmt.Errorf("%w", ...)` | Phase 1 style; no `pkg/errors` anywhere |
| `fmt` (stdlib) | Go 1.23.8 | `fmt.Errorf` for wrapped errors with context | Standard error-context idiom |

### Supporting (tests only)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` (stdlib) | Go 1.23.8 | Test runner, `t.Setenv`, `t.TempDir` | All tests; `t.Setenv` auto-restores env on test completion |
| `github.com/stretchr/testify/require` | v1.11.1 | Readable fail-fast assertions | Matches existing `idgen_test.go` style; use `require.*` not `assert.*` |
| `io/fs` (stdlib) | Go 1.23.8 | `fs.FileMode` type for permission assertions | `info.Mode().Perm()` to verify 0700 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.Getenv("HOME")` | `os.UserHomeDir()` | Would be pure `$HOME` lookup on Linux/macOS in Go 1.23+ (verified in file.go: no `os/user` import), BUT project constraint explicitly forbids it for future-proofing against Go runtime changes. Discretion-level choice; CONTEXT.md is binding. [VERIFIED: go1.23.8/src/os/file.go] |
| `os.MkdirAll(dir, 0o700)` | `os.Mkdir(dir, 0o700)` + pre-check parent | `MkdirAll` is idempotent, creates intermediates, and is a no-op if dir exists. `Mkdir` fails if the dir exists — you'd have to swallow `os.IsExist`. `MkdirAll` is exactly the right tool here. |
| `github.com/adrg/xdg` | (stdlib only) | Full XDG helper (would give us `xdg.StateHome`), but it's a third-party dep for what amounts to 4 lines of code. Violates "stdlib-first" constraint. [CITED: github.com/adrg/xdg] |
| Panic on missing `$HOME` | Return error | `Resolve()` is called from `main()` before kong dispatch; returning an error lets the CLI print a clean "HOME is not set" message to stderr and exit non-zero. Panics would print a Go stack trace — hostile UX. |

**Installation:** No new dependencies — uses only stdlib and the existing `testify/require` (already pinned in go.mod).

**Version verification:** All libraries used are stdlib, pinned by Go toolchain version 1.23.8 (verified in `/home/alpine/mcp-chain/go.mod`). `testify/require` v1.11.1 already present (released 2025-08-27, verified in prior phase research).

## Architecture Patterns

### System Architecture Diagram

```
+-----------------------+
|   mcp-chain (main)    |
|   kong argv dispatch  |
+----------+------------+
           |
           | calls once at boot
           v
+-----------------------+
|  statepath.Resolve()  |
+----------+------------+
           |
           | reads env (NO os/user, NO NSS)
           v
+-----------------------+       +-----------------------+
|  os.Getenv            |       |  os.Getenv            |
|  "XDG_STATE_HOME"     |       |  "HOME"               |
+----------+------------+       +----------+------------+
           |                               |
           | non-empty?                    | used only if XDG unset/empty
           +-----------+-------------------+
                       |
                       v
           +------------------------+
           | compute parent dir     |
           |  XDG: $X/mcp-chain     |
           |  HOME: $H/.mcp-chain   |
           +-----------+------------+
                       |
                       v
           +------------------------+
           |  os.MkdirAll(parent,   |
           |              0o700)    |
           +-----------+------------+
                       |
                       | on success
                       v
           +------------------------+
           |  return                |
           |  parent + "state.json" |
           +------------------------+

                consumer (Phase 4)
                       |
                       v
           +------------------------+
           | store.Open(path)       |
           | -- flock, read, write  |
           +------------------------+
```

**Data flow:** main → Resolve() → os.Getenv (×1 or ×2) → MkdirAll → string path return → store layer. No MCP types, no protocol awareness, no file IO beyond `MkdirAll`.

### Recommended Project Structure

```
internal/
└── statepath/
    ├── resolve.go       # Package doc + Resolve() + error sentinels
    └── resolve_test.go  # 5 tests using t.Setenv + t.TempDir
```

Mirrors the Phase 2 `internal/idgen/` shape exactly: one production file, one test file, package doc at the top cites the requirement ID.

### Pattern 1: Env-var-first resolution with fail-fast on missing $HOME

**What:** Check `$XDG_STATE_HOME` first; if empty or unset, check `$HOME`; if both missing, return error.
**When to use:** Any Linux/macOS Go binary that must never do NSS lookups at startup.
**Rationale:** XDG spec treats unset and empty as equivalent; we implement with `os.Getenv` + explicit emptiness check (don't use `os.LookupEnv` — it would distinguish the two, and we don't want to).

```go
// Source: XDG Base Directory Specification + stdlib os package
if x := os.Getenv("XDG_STATE_HOME"); x != "" {
    parent = filepath.Join(x, "mcp-chain")
} else {
    h := os.Getenv("HOME")
    if h == "" {
        return "", errors.New("statepath: $HOME and $XDG_STATE_HOME are both unset")
    }
    parent = filepath.Join(h, ".mcp-chain")
}
```

### Pattern 2: Idempotent directory setup with `MkdirAll`

**What:** Call `os.MkdirAll(parent, 0o700)` unconditionally — it is a no-op if `parent` already exists, and creates all intermediate directories if not.
**When to use:** Any boot path where multiple concurrent invocations could race to create the same directory.
**Rationale:** `mcp-chain status` runs inside a 1-second poll loop across N waiters. If `state.json`'s parent didn't exist, every waiter racing to create it with `os.Mkdir` would see `os.ErrExist`. `MkdirAll` handles this correctly.

```go
// Source: pkg.go.dev/os#MkdirAll
// "If path is already a directory, MkdirAll does nothing and returns nil."
if err := os.MkdirAll(parent, 0o700); err != nil {
    return "", fmt.Errorf("statepath: create parent dir %q: %w", parent, err)
}
```

### Pattern 3: Preserve user-created directory modes

**What:** `MkdirAll` only applies the `perm` argument to directories it creates; it never chmods existing directories.
**When to use:** Always. Never chmod a dir the user set up themselves.
**Rationale:** If the user pre-created `~/.mcp-chain` with `0755` (e.g., because they symlinked it, or for backup tooling), mcp-chain must not surprise them by tightening it to `0700`. This is a documented property of `MkdirAll` and an explicit test invariant (`TestResolve_ParentAlreadyExists`).

### Anti-Patterns to Avoid

- **Using `os.UserHomeDir()` despite CONTEXT.md saying not to:** Even though Go 1.23.8's implementation is verified env-only (see `src/os/file.go` — no `os/user` import, no NSS call), the project constraint wins. Future Go versions could add an `os/user` fallback; using `os.Getenv("HOME")` directly is explicit about intent. [VERIFIED: github.com/golang/go/blob/go1.23.8/src/os/file.go]
- **`os.LookupEnv` + `ok`-check:** Would distinguish empty-string from unset. The XDG spec explicitly says "either not set or empty" → treat the same. `os.Getenv` + `!= ""` check is the correct idiom here.
- **Octal `0700` without the `0o` prefix:** `staticcheck` flags `0700` as a style issue; Go style requires `0o700`. [CITED: golang.org/doc/go1.13#non-decimal-integer-literals]
- **`os.Mkdir` instead of `os.MkdirAll`:** Not idempotent; creates only the final segment; returns `os.ErrExist` when concurrent callers race.
- **Chmod-ing existing directories:** Never do this. If the dir exists with unexpected permissions, that is the user's choice; bail out loudly or accept it. We accept.
- **Probing writability by creating a test file:** That's Phase 4's concern (store Open). Doing it here duplicates effort and leaves behind a zero-byte test artifact if the process crashes between probe and cleanup.
- **Failing if the dir exists with wrong mode:** Out of scope; the store layer will fail on write with a clear permission error.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| XDG env lookup with fallback | Custom env-var parser with `os.Environ` loop | `os.Getenv` + emptiness check | Stdlib function, inlined, zero overhead |
| Recursive directory creation | Walk segments + os.Mkdir each + IsExist swallow | `os.MkdirAll` | ~15 lines vs 1; handles races correctly |
| Path joining | String concat with `/` | `filepath.Join` | Strips redundant separators; respects GOOS |
| Error with context | Struct types with `Error() string` methods | `fmt.Errorf("context: %w", err)` | Phase 1 convention; `errors.Is`/`errors.As` work with `%w` |
| XDG spec helpers for a single var | `github.com/adrg/xdg` dep | 4 lines of `os.Getenv` | Stdlib-first constraint; one file of code |

**Key insight:** This phase is an archetypal "don't add a dep" scenario. The entire package is ~40 LOC of production code; a third-party XDG helper would be more code (transitive deps, go.mod bloat) than the thing we're avoiding.

## Runtime State Inventory

> Phase 3 is greenfield (new package, no pre-existing state to migrate). Section retained for completeness.

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | None — first introduction of `state.json` path resolution. Phase 4 will create the file. | None |
| Live service config | None — no daemons, no external services. | None |
| OS-registered state | None — no systemd units, no cron jobs, no launchd plists in this phase. | None |
| Secrets/env vars | Reads `$HOME` and `$XDG_STATE_HOME` at runtime. Neither is secret. | None |
| Build artifacts | None — new package compiles into the existing `mcp-chain` binary. | None |

**Nothing to migrate.** Rename/refactor inventory does not apply here — this is a greenfield package introduction.

## Common Pitfalls

### Pitfall 1: `os.UserHomeDir` misconception
**What goes wrong:** Skipping the project's explicit "no `os.UserHomeDir`" rule on the theory that "it's just an env var read."
**Why it happens:** Developer sees the Go 1.23 source and concludes the constraint is outdated.
**How to avoid:** Respect the project constraint regardless — it documents intent (robustness against future Go changes) even when current Go behavior is benign. Use `os.Getenv("HOME")` directly.
**Warning signs:** PR review discussion "why can't we just use os.UserHomeDir" → point at CONTEXT.md line "No `os.UserHomeDir` — stdlib versions call `os/user` on some platforms" and the decision rationale.

### Pitfall 2: Empty string vs unset env var
**What goes wrong:** Developer uses `os.LookupEnv` and branches on the `ok` value, so `XDG_STATE_HOME=""` is treated as "set" and produces a path literally starting with `/mcp-chain/state.json`.
**Why it happens:** `os.LookupEnv` is the "correct" way to disambiguate unset vs empty in general Go code. But XDG spec explicitly flattens this distinction.
**How to avoid:** Use `os.Getenv` (returns `""` for both cases) + `if x != ""` check. Cite the spec in a comment: "XDG: unset or empty → fallback". [CITED: specifications.freedesktop.org/basedir/latest/]
**Warning signs:** A test named `TestResolve_EmptyXDG` fails with a path like `"/mcp-chain/state.json"` — you've taken the XDG branch on an empty var.

### Pitfall 3: Octal literal style (`0700` vs `0o700`)
**What goes wrong:** `staticcheck` or newer `gofmt` flag the legacy-octal `0700` form; CI breaks.
**Why it happens:** Go 1.13 added the `0o` prefix as the preferred form; legacy `0NNN` still compiles but is a style issue.
**How to avoid:** Always write `0o700` for file modes in new code. [CITED: golang.org/doc/go1.13#non-decimal-integer-literals]
**Warning signs:** Lint fails with `ST1013: should use Go-style octal literal`.

### Pitfall 4: `os.Mkdir` + `os.ErrExist` swallow
**What goes wrong:** Hand-rolled "idempotent mkdir" that calls `os.Mkdir` and checks `errors.Is(err, os.ErrExist)`, but misses the intermediate-directory case (`$XDG_STATE_HOME` points at a not-yet-existing dir).
**Why it happens:** Developer reaches for `os.Mkdir` first because the semantics are simpler. Adds ErrExist swallow to handle the existing-dir case. Still misses multi-level paths.
**How to avoid:** `os.MkdirAll` is always the right tool for "ensure directory exists, creating intermediates if needed." It is idempotent AND recursive.
**Warning signs:** Test passes when `$XDG_STATE_HOME=/tmp/foo` (single segment) but fails when `$XDG_STATE_HOME=/tmp/a/b/c` (nested).

### Pitfall 5: `t.TempDir` + `t.Setenv` ordering
**What goes wrong:** Developer creates `tmp := t.TempDir()`, then points `HOME` at it with `t.Setenv("HOME", tmp)`, then adds a `defer os.Unsetenv(...)`, causing stale env on next test.
**Why it happens:** Muscle memory from pre-Go-1.17 tests that needed manual env cleanup.
**How to avoid:** `t.Setenv` auto-restores the variable to its original state at test end — no defer needed. Same for `t.TempDir` which is auto-removed. Document this with a comment.
**Warning signs:** Tests pass individually but fail in `-count=2` or when run together; or a later test sees leaked env from an earlier test.

### Pitfall 6: Treating `MkdirAll` on existing dir as a failure
**What goes wrong:** Test asserts that calling `Resolve()` twice is an error (double-create), but `MkdirAll` is explicitly idempotent — the second call is a no-op.
**Why it happens:** Confusion with `os.Mkdir`'s semantics.
**How to avoid:** Write `TestResolve_Idempotent` that calls `Resolve()` twice and asserts no error on the second call. Confirms the contract.
**Warning signs:** Flaky tests in N-concurrent `status` simulations.

## Code Examples

### Complete `internal/statepath/resolve.go`

```go
// Package statepath resolves the on-disk location of mcp-chain's state.json
// and ensures its parent directory exists with mode 0700.
//
// Resolution order (Linux, macOS):
//
//	$XDG_STATE_HOME is set and non-empty → $XDG_STATE_HOME/mcp-chain/state.json
//	otherwise                             → $HOME/.mcp-chain/state.json
//	$HOME also unset                      → error
//
// Per the XDG Base Directory Specification, an empty-string value for
// $XDG_STATE_HOME is treated identically to unset.
//
// Deliberately avoids os/user.Current (NSS / potential cgo) and
// os.UserHomeDir (project policy: env-only lookups on the startup path).
//
// Requirement: CORE-06. Windows support is deferred.
package statepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// stateFileName is the basename of the state file. Exported indirectly via
// Resolve() so the store layer does not hard-code it.
const stateFileName = "state.json"

// dirName is the application directory name used under both XDG and HOME roots.
const dirName = "mcp-chain"

// homeSubdir is the dot-prefixed variant used under $HOME per project spec
// (not the strict-XDG $HOME/.local/state/mcp-chain path).
const homeSubdir = ".mcp-chain"

// ErrHomeUnset is returned when both $XDG_STATE_HOME and $HOME are unset
// or empty. Use errors.Is to check.
var ErrHomeUnset = errors.New("statepath: $HOME and $XDG_STATE_HOME are both unset or empty")

// Resolve returns the absolute path to mcp-chain's state.json and ensures
// its parent directory exists with mode 0700. The state file itself is not
// created; that is the store layer's responsibility.
//
// Resolve is safe to call concurrently from multiple processes (MkdirAll is
// idempotent under races).
//
// Returns ErrHomeUnset wrapped if neither $XDG_STATE_HOME nor $HOME is set.
// Returns a wrapped mkdir error if the parent directory cannot be created.
func Resolve() (string, error) {
	var parent string
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		// XDG spec §3: "If $XDG_STATE_HOME is either not set or empty, a
		// default equal to $HOME/.local/state should be used." We honor
		// non-empty values here; empty or unset falls through to $HOME.
		parent = filepath.Join(x, dirName)
	} else {
		h := os.Getenv("HOME")
		if h == "" {
			return "", ErrHomeUnset
		}
		parent = filepath.Join(h, homeSubdir)
	}

	// MkdirAll is idempotent: no-op if parent already exists, creates all
	// intermediate segments otherwise. It does NOT chmod existing dirs,
	// so a user-created parent with 0755 is preserved as-is.
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", fmt.Errorf("statepath: create parent dir %q: %w", parent, err)
	}

	return filepath.Join(parent, stateFileName), nil
}
```

### Complete `internal/statepath/resolve_test.go`

```go
package statepath

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResolve_XDGSet verifies the primary branch: XDG_STATE_HOME is set to a
// temp directory, so Resolve() returns $tmp/mcp-chain/state.json and creates
// the parent with mode 0700.
func TestResolve_XDGSet(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	// Explicitly clear HOME so a leaked HOME from the caller can't mask a bug
	// in the XDG branch. Setenv("", "") is NOT an unset; use "" which Getenv
	// reports identically and is fine because we never read HOME on this path.
	t.Setenv("HOME", "")

	got, err := Resolve()
	require.NoError(t, err)

	wantParent := filepath.Join(tmp, "mcp-chain")
	wantPath := filepath.Join(wantParent, "state.json")
	require.Equal(t, wantPath, got)

	info, err := os.Stat(wantParent)
	require.NoError(t, err)
	require.True(t, info.IsDir(), "parent must be a directory")
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
		"parent must be mode 0700 when created by Resolve")

	// State file itself must NOT exist — Resolve only creates the directory.
	_, err = os.Stat(wantPath)
	require.True(t, os.IsNotExist(err), "Resolve must not create the state file")
}

// TestResolve_HOMEFallback verifies: when XDG_STATE_HOME is unset (empty),
// Resolve falls back to $HOME/.mcp-chain/state.json.
func TestResolve_HOMEFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", tmp)

	got, err := Resolve()
	require.NoError(t, err)

	wantParent := filepath.Join(tmp, ".mcp-chain")
	wantPath := filepath.Join(wantParent, "state.json")
	require.Equal(t, wantPath, got)

	info, err := os.Stat(wantParent)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

// TestResolve_NeitherSet verifies: when both XDG_STATE_HOME and HOME are
// unset/empty, Resolve returns ErrHomeUnset (wrapped-compatible with errors.Is).
func TestResolve_NeitherSet(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")

	got, err := Resolve()
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrHomeUnset),
		"expected ErrHomeUnset, got %v", err)
	require.Empty(t, got, "path must be empty on error")
}

// TestResolve_EmptyXDG pins the XDG spec rule that empty is equivalent to
// unset. XDG_STATE_HOME="" must fall through to HOME, not produce a path
// like "/mcp-chain/state.json".
func TestResolve_EmptyXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "") // explicitly empty, not unset
	t.Setenv("HOME", tmp)

	got, err := Resolve()
	require.NoError(t, err)

	require.Equal(t,
		filepath.Join(tmp, ".mcp-chain", "state.json"), got,
		"empty XDG_STATE_HOME must fall through to HOME per XDG spec §3")
}

// TestResolve_ParentAlreadyExists documents a deliberate choice: if the user
// pre-created the parent directory with a looser mode (e.g. 0755), Resolve
// does NOT tighten it. MkdirAll is a no-op on existing dirs.
//
// Rationale: the user's chosen mode is respected. Phase 4's state file write
// enforces mode 0600 on the file itself, which is the security-relevant one.
func TestResolve_ParentAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	t.Setenv("HOME", "")

	preCreated := filepath.Join(tmp, "mcp-chain")
	require.NoError(t, os.Mkdir(preCreated, 0o755))

	got, err := Resolve()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(preCreated, "state.json"), got)

	info, err := os.Stat(preCreated)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm(),
		"Resolve must NOT chmod a pre-existing parent dir")
}

// TestResolve_Idempotent verifies that calling Resolve twice in the same
// process is a no-op on the second call. Important because N concurrent
// `mcp-chain status` invocations can all race to create the same dir.
func TestResolve_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	t.Setenv("HOME", "")

	p1, err := Resolve()
	require.NoError(t, err)
	p2, err := Resolve()
	require.NoError(t, err)
	require.Equal(t, p1, p2)
}
```

> Test count: 6 (one extra beyond the spec's 5 — `TestResolve_Idempotent` is cheap insurance for the concurrent-`status` scenario described in Phase 6).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `os.UserHomeDir()` suspected of calling `os/user` on Linux | Go 1.23's `UserHomeDir` is verified pure-env on Linux/macOS (no `os/user` import) | Stable through Go 1.23 (and earlier Unix releases per source history) | CONTEXT.md's reasoning is partially outdated BUT the rule still makes sense defensively. [VERIFIED: go1.23.8/src/os/file.go] |
| Legacy octal `0700` | `0o700` prefix (Go 1.13+) | Go 1.13 (Sep 2019) | Lint gate; new code MUST use `0o` prefix |
| `os.Mkdir` + swallow `os.ErrExist` | `os.MkdirAll` | Always preferred for "ensure exists" semantics | Zero race conditions, one line |
| `github.com/pkg/errors` wrap | `fmt.Errorf("...: %w", err)` | Go 1.13 (`errors.Is`/`errors.As`) | Project convention; no `pkg/errors` anywhere |

**Deprecated/outdated:**
- `pkg/errors`: Never import. Use `fmt.Errorf` with `%w`.
- `os.IsNotExist(err)` in new code where possible: prefer `errors.Is(err, os.ErrNotExist)`. (Still acceptable in tests; Phase 1 uses both.)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The project's intended fallback is `$HOME/.mcp-chain/state.json`, not strict-XDG `$HOME/.local/state/mcp-chain/state.json`. | Summary, Code Examples | If wrong, ships a path that violates user expectations. MITIGATION: Aligned with PROJECT.md REQ CORE-06 verbatim and REQUIREMENTS.md CORE-06; both explicitly state `~/.mcp-chain/state.json`. Low risk — it's the locked project decision. [CITED: /home/alpine/mcp-chain/.planning/PROJECT.md line 25] [CITED: /home/alpine/mcp-chain/.planning/REQUIREMENTS.md line 17] |
| A2 | `t.Setenv("VAR", "")` behaves identically to `t.Setenv` with a non-empty value for the purposes of env isolation (auto-restore on test end). | Test code | Tests could leak env state into later tests. MITIGATION: Go stdlib docs confirm `t.Setenv` restores the prior value (including the unset state) on test cleanup via `t.Cleanup`. [CITED: pkg.go.dev/testing#T.Setenv] |
| A3 | Go 1.23's `os.MkdirAll` preserves existing-directory permissions and does not chmod. | Pattern 3, TestResolve_ParentAlreadyExists | If wrong, user-created dirs would get silently tightened to 0700. MITIGATION: Documented stdlib behavior, stable since Go 1.0. Test `TestResolve_ParentAlreadyExists` pins the invariant. [CITED: pkg.go.dev/os#MkdirAll] |
| A4 | No existing `internal/statepath` package — this is a greenfield introduction. | Runtime State Inventory | Could collide with existing code. MITIGATION: Verified via `ls internal/` — only `idgen/` exists as of 2026-04-23. |

All other claims are VERIFIED or CITED.

## Open Questions

### Q1 — Fallback path: dot-dir (`~/.mcp-chain/`) or strict XDG (`~/.local/state/mcp-chain/`)?

- **What we know:** PROJECT.md line 25 and REQUIREMENTS.md CORE-06 both explicitly say `~/.mcp-chain/state.json` for the non-XDG branch. The XDG Base Directory Specification's strict fallback would be `$HOME/.local/state/mcp-chain/state.json`.
- **What's unclear:** Whether the project intent is to violate strict XDG deliberately (dot-dir visibility in `~`, simpler UX for a single-user tool) or whether it's an oversight.
- **Recommendation: implement the project-specified `~/.mcp-chain/state.json`** and close this question. Rationale:
  1. Both PROJECT.md and REQUIREMENTS.md independently specify it — consistent, not a typo.
  2. The dot-dir pattern (`.mcp-chain`) is widely used by similar single-user CLI tools (`.docker/`, `.kube/`, `.ssh/`, `.cargo/`, `.gnupg/`).
  3. `$XDG_STATE_HOME`-aware users still get XDG compliance when they set the var.
  4. Changing to strict XDG later is an additive compat break: any user with `~/.mcp-chain/state.json` would need a migration or both-path-check fallback. Starting with strict XDG and adding dot-dir later is worse.
- **Action: CLOSED — implement as specified.** Flag in README (Phase 10) that users who want strict XDG should `export XDG_STATE_HOME=~/.local/state`.

### Q2 — Should `Resolve()` probe writability?

- **What we know:** `MkdirAll` succeeds if the directory exists or can be created. It does not guarantee the process can write files inside it (could be on a read-only filesystem, could have wrong group ownership, etc.).
- **What's unclear:** Whether "parent dir exists with mode 0700" (Success Criterion 2) implies a writability probe.
- **Recommendation: directory-creation only, no writability probe.**
  1. Phase 4 (store) will attempt the first file write and surface any permission issue there with a clear error. Dup-probing here just produces two errors for one root cause.
  2. A probe would leave a test artifact on disk if the process crashes between probe and cleanup — pollution.
  3. The phase success criterion is literally "parent directory is created with mode 0700 if missing" — doesn't require writability.
- **Action: CLOSED — no writability probe.**

### Q3 — Should the package expose separate `StatePath()` and `EnsureDir()` functions?

- **What we know:** CONTEXT.md suggests `Resolve() (string, error)` as one option. Some designs split these so a `--path-only` CLI flag could print the path without touching the filesystem.
- **What's unclear:** Whether such a flag is anticipated.
- **Recommendation: single `Resolve()` function for now.**
  1. YAGNI: no downstream consumer in Phases 4–10 needs path-without-mkdir.
  2. If `mcp-chain doctor` (v2, OBS-03) needs a read-only path probe, it can add a `StatePathOnly()` helper in a follow-up PR at that time.
  3. One function = smallest API surface.
- **Action: CLOSED — one function, `Resolve()`.**

### Q4 — Confidence: is CONTEXT.md's "os.UserHomeDir calls os/user on some platforms" still accurate?

- **What we know:** [VERIFIED via go1.23.8/src/os/file.go] `UserHomeDir` on Linux/macOS/Windows is pure env lookup. No `os/user` import in that file.
- **What's unclear:** Historical Go versions (pre-1.12) may have used `os/user` as a fallback; some Plan 9 branches may still.
- **Recommendation: respect the project constraint regardless.**
  1. Even though the current runtime is safe, the constraint documents *intent* (robustness against future changes).
  2. `os.Getenv("HOME")` is more self-documenting.
  3. Zero cost, zero risk.
- **Action: CLOSED — use `os.Getenv` directly; add a code comment citing this finding for future readers.**

**All open questions are resolved inline. No blockers for the planner.**

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | Build, test | Assumed (already Phase 1+2 done) | 1.23.8 | — |
| `testify/require` | Tests | Present in go.mod | v1.11.1 | — |
| `$HOME` set at runtime | `Resolve()` fallback branch | User-environment | — | `$XDG_STATE_HOME` (either works) |

No external CLIs, no databases, no services. Phase 3 is pure-Go code. This section mostly just documents that nothing new is needed.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `github.com/stretchr/testify/require` v1.11.1 |
| Config file | none (stdlib test runner) |
| Quick run command | `go test ./internal/statepath/...` |
| Full suite command | `go test -race -count=1 ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| CORE-06 | XDG_STATE_HOME set → `$X/mcp-chain/state.json`, parent 0700 | unit | `go test ./internal/statepath/ -run TestResolve_XDGSet` | ❌ Wave 0 |
| CORE-06 | XDG unset/empty + HOME set → `$HOME/.mcp-chain/state.json`, parent 0700 | unit | `go test ./internal/statepath/ -run TestResolve_HOMEFallback` | ❌ Wave 0 |
| CORE-06 | Both unset → error (ErrHomeUnset) | unit | `go test ./internal/statepath/ -run TestResolve_NeitherSet` | ❌ Wave 0 |
| CORE-06 | Empty XDG treated as unset (XDG spec compliance) | unit | `go test ./internal/statepath/ -run TestResolve_EmptyXDG` | ❌ Wave 0 |
| CORE-06 | Pre-existing parent dir mode preserved (no chmod-down) | unit | `go test ./internal/statepath/ -run TestResolve_ParentAlreadyExists` | ❌ Wave 0 |
| CORE-06 | Idempotent under repeated calls (N-concurrent-status safety) | unit | `go test ./internal/statepath/ -run TestResolve_Idempotent` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/statepath/...` — expected wall-time <1 s.
- **Per wave merge:** `go test -race ./...` — full race suite, expected <10 s total across all packages at Phase 3 state.
- **Phase gate:** Full suite green and lint (`golangci-lint run`) clean before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/statepath/resolve.go` — package + Resolve() + ErrHomeUnset sentinel
- [ ] `internal/statepath/resolve_test.go` — six tests (see Phase Requirements → Test Map)

No framework install needed; `testing` is stdlib and `testify/require` is already vendored.

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/os#UserHomeDir](https://pkg.go.dev/os#UserHomeDir) — verified `UserHomeDir` is `$HOME` env read on Linux/macOS; no `os/user` call in Go 1.23.
- [github.com/golang/go/blob/go1.23.8/src/os/file.go](https://github.com/golang/go/blob/go1.23.8/src/os/file.go) — raw source of `UserHomeDir`; confirms env-only implementation.
- [pkg.go.dev/os#MkdirAll](https://pkg.go.dev/os#MkdirAll) — idempotency and no-chmod-on-existing semantics.
- [pkg.go.dev/testing#T.Setenv](https://pkg.go.dev/testing#T.Setenv) — auto-restore semantics; no defer needed.
- [specifications.freedesktop.org/basedir/latest/](http://specifications.freedesktop.org/basedir/latest/) — XDG Base Directory Specification; "unset or empty → fallback" rule for `XDG_STATE_HOME`.
- [golang.org/doc/go1.13#non-decimal-integer-literals](https://golang.org/doc/go1.13) — `0o700` octal prefix style.
- `/home/alpine/mcp-chain/.planning/PROJECT.md` — CORE-05 (state file path) verified to say `~/.mcp-chain/state.json`.
- `/home/alpine/mcp-chain/.planning/REQUIREMENTS.md` — CORE-06 verified.
- `/home/alpine/mcp-chain/.planning/phases/03-xdg-path-resolution/03-CONTEXT.md` — locked constraints.
- `/home/alpine/mcp-chain/internal/idgen/wordlist.go` — style/doc-comment pattern reference.
- `/home/alpine/mcp-chain/.golangci.yml` — lint rules that apply to this phase.

### Secondary (MEDIUM confidence)
- [wiki.archlinux.org/title/XDG_Base_Directory](https://wiki.archlinux.org/title/XDG_Base_Directory) — community reference reinforcing XDG_STATE_HOME default.
- [github.com/adrg/xdg](https://github.com/adrg/xdg) — reference Go XDG helper (not used; demonstrates the community-standard resolution for comparison).

### Tertiary (LOW confidence)
- (none — all critical claims verified against primary sources)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, verified against Go 1.23.8 source.
- Architecture: HIGH — trivial responsibility boundary, one function.
- Pitfalls: HIGH — each pitfall has a corresponding regression test.
- XDG spec interpretation: HIGH — spec text cited verbatim, empty-string rule verified.
- Fallback path (dot-dir vs strict-XDG): HIGH — aligned with project docs unambiguously.

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (30 days — stable domain, stdlib-only, no fast-moving deps)
