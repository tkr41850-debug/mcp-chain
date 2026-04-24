//go:build integration

package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anthropics/mcp-chain/internal/store"
)

// seedStateForChild writes a state.json at the exact path
// statepath.Resolve() will produce when XDG_STATE_HOME=dir.
// Returns the seeded id (or "" for setups that register no id).
func seedStateForChild(t *testing.T, dir string, setup func(t *testing.T, st *store.Store) string) string {
	t.Helper()
	statePath := filepath.Join(dir, "mcp-chain", "state.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(statePath), 0o700))
	st, err := store.Open(statePath)
	require.NoError(t, err)
	return setup(t, st)
}

// TestStatus_IntegrationExitCodes — CORE-01 SC #1 end-to-end.
// Builds the binary, spawns it with a temp XDG_STATE_HOME, and
// asserts exit code + stdout + stderr for each contract row.
func TestStatus_IntegrationExitCodes(t *testing.T) {
	binPath := buildBinary(t)

	cases := []struct {
		name     string
		setup    func(t *testing.T, st *store.Store) string
		idFn     func(seeded string) string // lets the unknown case override to "nonexistent"
		wantExit int
		wantOut  string
		wantErr  string // substring; "" → assert empty
	}{
		{
			name: "resolved",
			setup: func(t *testing.T, st *store.Store) string {
				id, err := st.Register("tok", "c")
				require.NoError(t, err)
				require.NoError(t, st.Resolve(id, "tok", store.ResolveOptions{}))
				return id
			},
			idFn:     func(seeded string) string { return seeded },
			wantExit: 0,
			wantOut:  "resolved\n",
			wantErr:  "",
		},
		{
			name: "pending",
			setup: func(t *testing.T, st *store.Store) string {
				id, err := st.Register("tok", "c")
				require.NoError(t, err)
				return id
			},
			idFn:     func(seeded string) string { return seeded },
			wantExit: 2,
			wantOut:  "pending\n",
			wantErr:  "",
		},
		{
			name: "unknown",
			setup: func(t *testing.T, st *store.Store) string {
				return "" // no registration
			},
			idFn:     func(_ string) string { return "nonexistent" },
			wantExit: 1,
			wantOut:  "",
			wantErr:  "unknown id",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			seeded := seedStateForChild(t, dir, tt.setup)
			id := tt.idFn(seeded)

			cmd := exec.Command(binPath, "status", id)
			cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
			var out, errW bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errW
			err := cmd.Run()

			gotExit := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				gotExit = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("unexpected error kind: %v (stderr=%q)", err, errW.String())
			}
			require.Equal(t, tt.wantExit, gotExit, "stderr=%q", errW.String())
			require.Equal(t, tt.wantOut, out.String(), "stdout mismatch")
			if tt.wantErr == "" {
				require.Empty(t, errW.String(), "LD-1: expected empty stderr for this row")
			} else {
				require.Contains(t, errW.String(), tt.wantErr)
			}
		})
	}
}

// TestStatus_StdoutOnlyStatus — SC #3 end-to-end: the compiled
// binary's stdout is EXACTLY `resolved\n` on resolved, EMPTY on
// unknown. Belt-and-braces alongside TestRunStatus_StdoutIsJustStatus
// (unit) because an intermediate write from main.go or kong between
// argv parse and StatusCmd.Run would escape the unit test.
func TestStatus_StdoutOnlyStatus(t *testing.T) {
	binPath := buildBinary(t)

	// Resolved case — exactly one line.
	dir := t.TempDir()
	id := seedStateForChild(t, dir, func(t *testing.T, st *store.Store) string {
		id, err := st.Register("tok", "c")
		require.NoError(t, err)
		require.NoError(t, st.Resolve(id, "tok", store.ResolveOptions{}))
		return id
	})
	cmd := exec.Command(binPath, "status", id)
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
	var out bytes.Buffer
	cmd.Stdout = &out
	require.NoError(t, cmd.Run())
	require.Equal(t, "resolved\n", out.String())

	// Unknown case — stdout must be empty; exit 1 expected.
	dir2 := t.TempDir()
	_ = seedStateForChild(t, dir2, func(*testing.T, *store.Store) string { return "" })
	cmd2 := exec.Command(binPath, "status", "nope")
	cmd2.Env = append(os.Environ(), "XDG_STATE_HOME="+dir2)
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	err := cmd2.Run()
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "unknown id must exit non-zero")
	require.Equal(t, 1, exitErr.ExitCode())
	require.Empty(t, out2.String(), "LD-1 unknown row: stdout MUST be empty")
}

// TestStatus_Concurrent10WithinOneSecond — SC #2: 10 parallel status
// probes against a resolved id complete in <1s. Proves store.Get's
// LOCK_SH does not serialize readers. See Pitfall 4 in RESEARCH.md
// for why we launch all 10 via cmd.Start before Waiting.
func TestStatus_Concurrent10WithinOneSecond(t *testing.T) {
	binPath := buildBinary(t)

	dir := t.TempDir()
	id := seedStateForChild(t, dir, func(t *testing.T, st *store.Store) string {
		id, err := st.Register("tok", "c")
		require.NoError(t, err)
		require.NoError(t, st.Resolve(id, "tok", store.ResolveOptions{}))
		return id
	})

	const N = 10
	cmds := make([]*exec.Cmd, N)
	for i := 0; i < N; i++ {
		cmds[i] = exec.Command(binPath, "status", id)
		cmds[i].Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
	}

	start := time.Now()

	// Start all children concurrently so fork/exec overhead overlaps.
	var wgStart sync.WaitGroup
	for i := 0; i < N; i++ {
		wgStart.Add(1)
		go func(i int) {
			defer wgStart.Done()
			if err := cmds[i].Start(); err != nil {
				t.Errorf("child %d start: %v", i, err)
			}
		}(i)
	}
	wgStart.Wait()

	// Wait on each child's completion.
	errs := make([]error, N)
	var wgDone sync.WaitGroup
	for i := 0; i < N; i++ {
		wgDone.Add(1)
		go func(i int) {
			defer wgDone.Done()
			errs[i] = cmds[i].Wait()
		}(i)
	}
	wgDone.Wait()

	elapsed := time.Since(start)
	t.Logf("10 concurrent status probes: elapsed=%v", elapsed)
	for i, e := range errs {
		require.NoError(t, e, "child %d failed (resolved id should exit 0)", i)
	}
	require.Less(t, elapsed, 1*time.Second,
		"SC #2 / LD-7: LOCK_SH must not serialize — 10 concurrent reads should complete in <1s. Got elapsed=%v", elapsed)
}

// TestHelpGoesToStderrNotStdout — SC #3: --help routes to stderr.
// kong.Writers + manual --version pre-parse is the fix validated here.
func TestHelpGoesToStderrNotStdout(t *testing.T) {
	binPath := buildBinary(t)
	cmd := exec.Command(binPath, "--help")
	var out, errW bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errW
	_ = cmd.Run() // kong exits 0 on --help
	require.Empty(t, out.String(), "SC #3: --help MUST NOT write to stdout")
	require.Contains(t, errW.String(), "Usage:", "--help must print usage on stderr")
}

// TestBadArgsGoesToStderr — SC #3: missing positional arg routes usage to stderr.
func TestBadArgsGoesToStderr(t *testing.T) {
	binPath := buildBinary(t)
	cmd := exec.Command(binPath, "status") // missing <id>
	var out, errW bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errW
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "missing arg must exit non-zero")
	require.NotEqual(t, 0, exitErr.ExitCode())
	require.Empty(t, out.String(), "SC #3: bad args MUST NOT write to stdout")
	// kong's message for a missing positional varies slightly across
	// versions; we check the most stable substrings.
	combined := errW.String()
	require.True(t,
		bytes.Contains([]byte(combined), []byte("expected")) ||
			bytes.Contains([]byte(combined), []byte("Usage:")),
		"expected 'expected' or 'Usage:' in stderr, got %q", combined)
}

// TestUnknownCommandGoesToStderr — SC #3: nonexistent subcommand routes to stderr.
func TestUnknownCommandGoesToStderr(t *testing.T) {
	binPath := buildBinary(t)
	cmd := exec.Command(binPath, "nosuchcommand")
	var out, errW bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errW
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "unknown command must exit non-zero")
	require.NotEqual(t, 0, exitErr.ExitCode())
	require.Empty(t, out.String(), "SC #3: unknown command MUST NOT write to stdout")
	require.NotEmpty(t, errW.String(), "unknown command must produce stderr text")
}

// TestVersion_StdoutExit0 — regression: manual --version pre-parse
// keeps --version writing to stdout with exit 0 after main.go patch.
// (Also covered by TestVersionFlagWritesToStdout in stubs_test.go,
//
//	which is NOT gated by the integration tag; this is the -tag=integration
//	belt-and-braces.)
func TestVersion_StdoutExit0(t *testing.T) {
	binPath := buildBinary(t)
	cmd := exec.Command(binPath, "--version")
	var out, errW bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errW
	require.NoError(t, cmd.Run())
	require.NotEmpty(t, out.String(), "--version must write to stdout")
	require.Contains(t, out.String(), "mcp-chain")
	require.Empty(t, errW.String(), "--version must write nothing to stderr")
}
