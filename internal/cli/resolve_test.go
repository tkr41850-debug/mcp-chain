package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anthropics/mcp-chain/internal/cli"
	"github.com/anthropics/mcp-chain/internal/store"
)

func TestRunResolve_Force_Success_Exit0(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c")
	require.NoError(t, err)

	var out, errW bytes.Buffer
	code := cli.RunResolve(&out, &errW, path, id, true)

	require.Equal(t, 0, code)
	require.Empty(t, out.String())
	require.Empty(t, errW.String())

	r, err := st.Get(id)
	require.NoError(t, err)
	require.Equal(t, "resolved", r.Status)
}

func TestRunResolve_NoForce_NotOwner_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c") // owner-stamped — non-empty token
	require.NoError(t, err)

	var out, errW bytes.Buffer
	code := cli.RunResolve(&out, &errW, path, id, false)

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Equal(t,
		"mcp-chain: not owner (use --force to override)\n",
		errW.String(),
		"LD-7: CLI surfaces --force hint on ErrNotOwner branch")
}

func TestRunResolve_UnknownID_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// No registration.

	var out, errW bytes.Buffer
	code := cli.RunResolve(&out, &errW, path, "nonexistent", true)

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Equal(t, "mcp-chain: unknown id: nonexistent\n", errW.String())
}

func TestRunResolve_AlreadyResolved_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)
	id, err := st.Register("tok", "c")
	require.NoError(t, err)
	require.NoError(t, st.Resolve(id, "tok", store.ResolveOptions{}), "first resolve with correct token")

	var out, errW bytes.Buffer
	code := cli.RunResolve(&out, &errW, path, id, true) // --force, already resolved

	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Equal(t, "mcp-chain: already resolved\n", errW.String())
}
