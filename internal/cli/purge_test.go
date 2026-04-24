package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tkr41850-debug/mcp-chain/internal/cli"
	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

func TestRunPurge_ByID_Success_Exit0(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c")
	require.NoError(t, err)

	var out, errW bytes.Buffer
	code := cli.RunPurge(&out, &errW, path, id, false, false)

	require.Equal(t, 0, code)
	require.Empty(t, out.String(), "LD-1 purge-success: stdout empty")
	require.Empty(t, errW.String(), "LD-1 purge-success: stderr empty")

	_, err = st.Get(id)
	require.ErrorIs(t, err, store.ErrUnknownID, "record must be gone")
}

func TestRunPurge_All_Success_Exit0(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	_, err = st.Register("tok", "c1")
	require.NoError(t, err)
	_, err = st.Register("tok", "c2")
	require.NoError(t, err)

	var out, errW bytes.Buffer
	code := cli.RunPurge(&out, &errW, path, "", true, false)

	require.Equal(t, 0, code)
	require.Empty(t, out.String())
	require.Empty(t, errW.String())

	recs, err := st.List()
	require.NoError(t, err)
	require.Empty(t, recs, "--all must remove every record")
}

func TestRunPurge_Resolved_OnlyResolvedGone_Exit0(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	idR, err := st.Register("tok", "resolved-one")
	require.NoError(t, err)
	_, err = st.Register("tok", "pending-one")
	require.NoError(t, err)
	require.NoError(t, st.Resolve(idR, "tok", store.ResolveOptions{}))

	var out, errW bytes.Buffer
	code := cli.RunPurge(&out, &errW, path, "", false, true)

	require.Equal(t, 0, code)
	require.Empty(t, out.String())
	require.Empty(t, errW.String())

	recs, err := st.List()
	require.NoError(t, err)
	require.Len(t, recs, 1, "--resolved leaves pending records")
	require.Equal(t, "pending", recs[0].Status, "only pending survives")
}

func TestRunPurge_NoArgs_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	var out, errW bytes.Buffer
	code := cli.RunPurge(&out, &errW, path, "", false, false)

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Contains(t, errW.String(), "purge requires <id>, --all, or --resolved",
		"LD-6 / pitfall 5: store.ErrPurgeArgRequired surfaces the locked line")
}

func TestRunPurge_UnknownID_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// No registration — any id is unknown.

	var out, errW bytes.Buffer
	code := cli.RunPurge(&out, &errW, path, "nonexistent", false, false)

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Equal(t, "mcp-chain: unknown id: nonexistent\n", errW.String())
}
