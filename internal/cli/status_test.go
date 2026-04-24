package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tkr41850-debug/mcp-chain/internal/cli"
	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

// Helper — seed a store with a single resolved record, return (path, id).
func seedResolved(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("owner-token", "cond")
	require.NoError(t, err)
	require.NoError(t, st.Resolve(id, "owner-token", store.ResolveOptions{}))
	return path, id
}

// Helper — seed a store with a single pending record, return (path, id).
func seedPending(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("owner-token", "cond")
	require.NoError(t, err)
	return path, id
}

func TestRunStatus_Resolved_Exit0(t *testing.T) {
	path, id := seedResolved(t)

	var out, errW bytes.Buffer
	code := cli.RunStatus(&out, &errW, path, id)

	require.Equal(t, 0, code)
	require.Equal(t, "resolved\n", out.String())
	require.Empty(t, errW.String(), "LD-1 resolved row: stderr must be empty")
}

func TestRunStatus_Pending_Exit2(t *testing.T) {
	path, id := seedPending(t)

	var out, errW bytes.Buffer
	code := cli.RunStatus(&out, &errW, path, id)

	require.Equal(t, 2, code)
	require.Equal(t, "pending\n", out.String())
	require.Empty(t, errW.String(), "LD-1 pending row: stderr must be empty (LD-4: no ExitCoder).")
}

func TestRunStatus_Unknown_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// Open+close once so the file exists with an empty-map state.
	st, err := store.Open(path)
	require.NoError(t, err)
	_ = st // no registrations

	var out, errW bytes.Buffer
	code := cli.RunStatus(&out, &errW, path, "nonexistent")

	require.Equal(t, 1, code)
	require.Empty(t, out.String(), "LD-1 unknown row: stdout must be empty")
	require.Equal(t, "mcp-chain: unknown id: nonexistent\n", errW.String())
}

func TestRunStatus_StdoutIsJustStatus(t *testing.T) {
	path, id := seedResolved(t)

	var out, errW bytes.Buffer
	code := cli.RunStatus(&out, &errW, path, id)

	require.Equal(t, 0, code)
	// Byte-exact: no banner, no prefix, no id echo.
	require.Equal(t, []byte("resolved\n"), out.Bytes(),
		"LD-1: stdout on resolved is byte-exact 'resolved\\n'. Any deviation breaks Phase 8 shell contracts.")
	require.Empty(t, errW.String())
}

func TestRunStatus_GenericError_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// Write a non-JSON byte sequence to force store.Open or store.Get into
	// the error path. The specific error class (corrupt / schema) is not
	// asserted — LD-6 folds both into the generic exit-1 default branch.
	require.NoError(t, os.WriteFile(path, []byte("not valid json"), 0o600))

	var out, errW bytes.Buffer
	code := cli.RunStatus(&out, &errW, path, "anyid")

	require.Equal(t, 1, code)
	require.Empty(t, out.String(), "LD-6: generic-error row — stdout empty")
	require.Contains(t, errW.String(), "mcp-chain: ",
		"LD-6: generic-error row — stderr prefixed with 'mcp-chain: '")
	require.NotContains(t, errW.String(), "unknown id:",
		"generic error must NOT take the unknown-id wording")
}
