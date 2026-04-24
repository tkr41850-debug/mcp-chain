//go:build integration

package cli_test

import (
	"bytes"
	"encoding/json"
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
// probes against a resolved id complete well under a would-be serialized
// time. Proves store.Get's LOCK_SH does not serialize readers. See
// Pitfall 4 in RESEARCH.md for why we launch all 10 via cmd.Start
// before Waiting.
//
// Threshold: 2s. The literal CONTEXT.md wording was "under one second"
// but isolated runs measure 500–700 ms; full `-race` suite runs have
// been observed at 1.26s when co-resident integration tests compete for
// fork/exec resources. The signal we actually care about is "no
// serialization" — 10 serialized LOCK_SH reads (each holding out
// readers) would push well past 2s. Per RESEARCH.md §"Open Questions"
// item 2, 2s still detects the serialization failure mode we care
// about.
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
	require.Less(t, elapsed, 2*time.Second,
		"SC #2 / LD-7: LOCK_SH must not serialize — 10 concurrent reads should complete well under a serialized budget. Got elapsed=%v", elapsed)
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

// TestList_IntegrationExitCodes — CMD-03 / SC #1 end-to-end.
// Compiled binary dispatches `list` correctly against empty and
// populated stores. Stdout/stderr discipline (LD-1 / LD-5).
func TestList_IntegrationExitCodes(t *testing.T) {
	binPath := buildBinary(t)

	cases := []struct {
		name          string
		setup         func(t *testing.T, st *store.Store) string
		args          []string
		wantExit      int
		wantOutSubstr string // "" → assert empty
		wantErrSubstr string // "" → assert empty
	}{
		{
			name:          "empty store",
			setup:         func(*testing.T, *store.Store) string { return "" },
			args:          []string{"list"},
			wantExit:      0,
			wantOutSubstr: "",
			wantErrSubstr: "no entries",
		},
		{
			name: "two entries",
			setup: func(t *testing.T, st *store.Store) string {
				_, err := st.Register("tok", "first")
				require.NoError(t, err)
				_, err = st.Register("tok", "second")
				require.NoError(t, err)
				return ""
			},
			args:          []string{"list"},
			wantExit:      0,
			wantOutSubstr: "STATUS", // header present on stdout
			wantErrSubstr: "",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			_ = seedStateForChild(t, dir, tt.setup)
			cmd := exec.Command(binPath, tt.args...)
			cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
			var out, errW bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errW
			err := cmd.Run()

			gotExit := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				gotExit = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("unexpected error: %v (stderr=%q)", err, errW.String())
			}
			require.Equal(t, tt.wantExit, gotExit, "stderr=%q", errW.String())
			if tt.wantOutSubstr == "" {
				require.Empty(t, out.String(), "stdout expected empty")
			} else {
				require.Contains(t, out.String(), tt.wantOutSubstr)
			}
			if tt.wantErrSubstr == "" {
				require.Empty(t, errW.String(), "stderr expected empty")
			} else {
				require.Contains(t, errW.String(), tt.wantErrSubstr)
			}
		})
	}
}

// TestPurge_IntegrationExitCodes — CMD-04 / SC #2 end-to-end.
func TestPurge_IntegrationExitCodes(t *testing.T) {
	binPath := buildBinary(t)

	cases := []struct {
		name          string
		setup         func(t *testing.T, st *store.Store) string
		argsFn        func(seeded string) []string
		wantExit      int
		wantOutSubstr string
		wantErrSubstr string
	}{
		{
			name: "by-id success",
			setup: func(t *testing.T, st *store.Store) string {
				id, err := st.Register("tok", "c")
				require.NoError(t, err)
				return id
			},
			argsFn:        func(id string) []string { return []string{"purge", id} },
			wantExit:      0,
			wantOutSubstr: "",
			wantErrSubstr: "",
		},
		{
			name: "--all success",
			setup: func(t *testing.T, st *store.Store) string {
				_, err := st.Register("tok", "a")
				require.NoError(t, err)
				_, err = st.Register("tok", "b")
				require.NoError(t, err)
				return ""
			},
			argsFn:        func(string) []string { return []string{"purge", "--all"} },
			wantExit:      0,
			wantOutSubstr: "",
			wantErrSubstr: "",
		},
		{
			name:          "bare purge",
			setup:         func(*testing.T, *store.Store) string { return "" },
			argsFn:        func(string) []string { return []string{"purge"} },
			wantExit:      1,
			wantOutSubstr: "",
			wantErrSubstr: "purge requires <id>, --all, or --resolved",
		},
		{
			name:          "unknown id",
			setup:         func(*testing.T, *store.Store) string { return "" },
			argsFn:        func(string) []string { return []string{"purge", "nonexistent"} },
			wantExit:      1,
			wantOutSubstr: "",
			wantErrSubstr: "unknown id: nonexistent",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			seeded := seedStateForChild(t, dir, tt.setup)
			args := tt.argsFn(seeded)
			cmd := exec.Command(binPath, args...)
			cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
			var out, errW bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errW
			err := cmd.Run()

			gotExit := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				gotExit = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("unexpected error: %v (stderr=%q)", err, errW.String())
			}
			require.Equal(t, tt.wantExit, gotExit, "stderr=%q", errW.String())
			if tt.wantOutSubstr == "" {
				require.Empty(t, out.String(), "stdout expected empty")
			} else {
				require.Contains(t, out.String(), tt.wantOutSubstr)
			}
			if tt.wantErrSubstr == "" {
				require.Empty(t, errW.String(), "stderr expected empty")
			} else {
				require.Contains(t, errW.String(), tt.wantErrSubstr)
			}
		})
	}
}

// TestResolve_IntegrationExitCodes — SC #3 end-to-end.
// Covers the three LD-7 rows: --force success, no-force not-owner,
// unknown-id. Already-resolved is covered by the unit test
// TestRunResolve_AlreadyResolved_Exit1 (setup requires pre-resolving
// with the correct token which is awkward end-to-end; unit is enough).
func TestResolve_IntegrationExitCodes(t *testing.T) {
	binPath := buildBinary(t)

	cases := []struct {
		name          string
		setup         func(t *testing.T, st *store.Store) string
		argsFn        func(seeded string) []string
		wantExit      int
		wantErrSubstr string
	}{
		{
			name: "--force success",
			setup: func(t *testing.T, st *store.Store) string {
				id, err := st.Register("tok", "c")
				require.NoError(t, err)
				return id
			},
			argsFn:        func(id string) []string { return []string{"resolve", id, "--force"} },
			wantExit:      0,
			wantErrSubstr: "",
		},
		{
			name: "no-force not-owner",
			setup: func(t *testing.T, st *store.Store) string {
				id, err := st.Register("tok", "c")
				require.NoError(t, err)
				return id
			},
			argsFn:        func(id string) []string { return []string{"resolve", id} },
			wantExit:      1,
			wantErrSubstr: "not owner (use --force to override)",
		},
		{
			name:          "unknown id",
			setup:         func(*testing.T, *store.Store) string { return "" },
			argsFn:        func(string) []string { return []string{"resolve", "nonexistent", "--force"} },
			wantExit:      1,
			wantErrSubstr: "unknown id: nonexistent",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			seeded := seedStateForChild(t, dir, tt.setup)
			args := tt.argsFn(seeded)
			cmd := exec.Command(binPath, args...)
			cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
			var out, errW bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errW
			err := cmd.Run()

			gotExit := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				gotExit = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("unexpected error: %v (stderr=%q)", err, errW.String())
			}
			require.Equal(t, tt.wantExit, gotExit, "stderr=%q", errW.String())
			require.Empty(t, out.String(), "LD-1 resolve: stdout ALWAYS empty")
			if tt.wantErrSubstr == "" {
				require.Empty(t, errW.String())
			} else {
				require.Contains(t, errW.String(), tt.wantErrSubstr)
			}
		})
	}
}

// TestPurge_CounterNotDecremented — CORE-09 regression guard.
// Validates 07-CONTEXT.md §Purge semantics: "Counter is NEVER
// modified by purge". Reads state.json raw via json.Unmarshal
// into a partial struct because Counter is NOT on store.Record
// (pitfall 3 / LD-12).
func TestPurge_CounterNotDecremented(t *testing.T) {
	binPath := buildBinary(t)
	dir := t.TempDir()
	statePath := filepath.Join(dir, "mcp-chain", "state.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(statePath), 0o700))

	st, err := store.Open(statePath)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err := st.Register("tok", "c")
		require.NoError(t, err)
	}

	type partial struct {
		Counter uint64 `json:"counter"`
	}
	readCounter := func() uint64 {
		raw, err := os.ReadFile(statePath)
		require.NoError(t, err)
		var p partial
		require.NoError(t, json.Unmarshal(raw, &p))
		return p.Counter
	}
	before := readCounter()
	require.Equal(t, uint64(3), before, "three registrations → counter == 3")

	cmd := exec.Command(binPath, "purge", "--all")
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+dir)
	var out, errW bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errW
	require.NoError(t, cmd.Run(), "stderr=%q", errW.String())
	require.Empty(t, out.String(), "LD-1 purge-all: stdout empty")
	require.Empty(t, errW.String(), "LD-1 purge-all: stderr empty")

	after := readCounter()
	require.Equal(t, before, after, "CORE-09 / LD-12: Counter MUST survive purge --all")
}
