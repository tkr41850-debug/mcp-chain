//go:build integration && !windows

// Integration tests for cross-process store coordination. Gated by the
// `integration` build tag so the hot unit suite stays ~3s; gated by
// !windows because Windows CI runs integration behaviour deferred to
// Phase 9 (RESEARCH §Windows Test Strategy).
//
// Test strategy: TestMain re-exec dispatch. When a parent test spawns
// `os.Executable()`, the child process runs this same binary with an
// env var instructing which worker to execute (register / slow-write);
// TestMain intercepts before calling m.Run. This avoids depending on
// the Phase 6 CLI (not yet built).

package store_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

const childEnvVar = "MCP_CHAIN_STORE_TEST_CHILD"

func TestMain(m *testing.M) {
	switch os.Getenv(childEnvVar) {
	case "register":
		childRegister()
		return // unreachable — childRegister calls os.Exit.
	case "slow-write":
		childSlowWrite()
		return
	}
	os.Exit(m.Run())
}

// childRegister is the subprocess worker for
// TestStore_TwoProcessesConcurrentRegister. Reads the state-file path,
// OwnerToken, and iteration count from env vars, then loops Register.
// Exit codes:
//
//	0   — success
//	10  — Open failed
//	11  — Register failed
//	12  — bad MCP_CHAIN_N env
func childRegister() {
	path := os.Getenv("MCP_CHAIN_STATE_PATH")
	token := os.Getenv("MCP_CHAIN_OWNER_TOKEN")
	n, err := strconv.Atoi(os.Getenv("MCP_CHAIN_N"))
	if err != nil {
		os.Exit(12)
	}
	s, err := store.Open(path)
	if err != nil {
		os.Exit(10)
	}
	for i := 0; i < n; i++ {
		if _, err := s.Register(token, fmt.Sprintf("cond-%d", i)); err != nil {
			os.Exit(11)
		}
	}
	os.Exit(0)
}

// childSlowWrite is the subprocess worker for
// TestStore_KillMidWriteLeavesCoherentState. Installs a synthetic write
// delay via the test-only hook, then calls Register — which blocks
// inside the overridden saveStateFn. The parent SIGKILLs the child
// mid-sleep, asserting state.json is byte-identical to its pre-Register
// content.
//
// Exit codes (unreachable under normal test flow — parent sends
// SIGKILL before Register returns):
//
//	10  — Open failed (parent will see this as a broken-test signal)
//	14  — bad MCP_CHAIN_TEST_WRITE_DELAY env
func childSlowWrite() {
	path := os.Getenv("MCP_CHAIN_STATE_PATH")
	delay, err := time.ParseDuration(os.Getenv("MCP_CHAIN_TEST_WRITE_DELAY"))
	if err != nil {
		os.Exit(14)
	}
	// Install the write-delay hook; no need to defer restore — this
	// process is about to be SIGKILLed.
	_ = store.SetWriteDelayForTest(delay)

	s, err := store.Open(path)
	if err != nil {
		os.Exit(10)
	}
	// This Register will block inside the overridden saveStateFn.
	// Parent SIGKILLs us mid-sleep; os.Exit(0) is unreachable.
	token := hex.EncodeToString(bytes.Repeat([]byte{0xCC}, 16))
	_, _ = s.Register(token, "slow")
	os.Exit(0)
}

// TestStore_TwoProcessesConcurrentRegister spawns two subprocesses,
// each registering 100 entries concurrently, and asserts the combined
// store has 200 unique IDs with no corruption. Validates SC #4 — cross-
// process flock serialises writes; monotonic counter works across
// processes.
func TestStore_TwoProcessesConcurrentRegister(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	exe, err := os.Executable()
	require.NoError(t, err)

	mkCmd := func(token string) *exec.Cmd {
		c := exec.Command(exe)
		c.Env = append(os.Environ(),
			childEnvVar+"=register",
			"MCP_CHAIN_STATE_PATH="+path,
			"MCP_CHAIN_OWNER_TOKEN="+token,
			"MCP_CHAIN_N=100",
		)
		// Children must not write to stdout (CLAUDE.md MCP-02). We
		// redirect to the test harness's stderr for diagnosability.
		c.Stdout = os.Stderr
		c.Stderr = os.Stderr
		return c
	}

	tok1 := hex.EncodeToString(bytes.Repeat([]byte{0xAA}, 16))
	tok2 := hex.EncodeToString(bytes.Repeat([]byte{0xBB}, 16))
	c1, c2 := mkCmd(tok1), mkCmd(tok2)

	require.NoError(t, c1.Start())
	require.NoError(t, c2.Start())
	require.NoError(t, c1.Wait(), "child 1 failed (non-zero exit)")
	require.NoError(t, c2.Wait(), "child 2 failed (non-zero exit)")

	// Open the store from the parent and inspect final state.
	s, err := store.Open(path)
	require.NoError(t, err)
	records, err := s.List()
	require.NoError(t, err)
	require.Len(t, records, 200, "two processes each registering 100 → 200 total records")

	seen := make(map[string]struct{}, 200)
	for _, r := range records {
		_, dup := seen[r.ID]
		require.Falsef(t, dup, "duplicate id %s across processes — flock not serialising?", r.ID)
		seen[r.ID] = struct{}{}
	}
}

// TestStore_KillMidWriteLeavesCoherentState verifies the atomic-rename
// crash invariant: if a writer is killed BEFORE the rename, state.json
// is byte-identical to its pre-write content. Validates SC #4 — no
// half-written state under failure.
func TestStore_KillMidWriteLeavesCoherentState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// 1. Seed a valid state with one record.
	s, err := store.Open(path)
	require.NoError(t, err)
	token := hex.EncodeToString(bytes.Repeat([]byte{0xDD}, 16))
	_, err = s.Register(token, "seed")
	require.NoError(t, err)

	seeded, err := os.ReadFile(path)
	require.NoError(t, err)

	// 2. Spawn the slow-write child. It will enter saveStateFn, sleep
	//    for 5s inside the hook, and never reach the atomic rename
	//    because the parent kills it first.
	exe, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		childEnvVar+"=slow-write",
		"MCP_CHAIN_STATE_PATH="+path,
		"MCP_CHAIN_TEST_WRITE_DELAY=5s",
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())

	// 3. Give the child time to acquire the flock and enter the sleep.
	//    1s is ample — Open is O(ms), flock is O(µs), and the hook
	//    sleeps BEFORE delegating to saveState.
	time.Sleep(1 * time.Second)

	// 4. SIGKILL mid-sleep — no cleanup runs, no rename happens.
	require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	_ = cmd.Wait() // non-zero exit on SIGKILL; don't assert.

	// 5. state.json must be valid JSON (no half-written bytes).
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	var parsed any
	require.NoError(t, json.Unmarshal(after, &parsed),
		"state.json must be valid JSON after SIGKILL")

	// 6. Tighter: state.json is byte-identical to the seed — because
	//    the kill landed BEFORE the atomic rename.
	require.Equal(t, seeded, after,
		"SIGKILL before atomic rename must leave old state intact")
}
