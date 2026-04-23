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
