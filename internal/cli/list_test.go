package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tkr41850-debug/mcp-chain/internal/cli"
	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

func TestRunList_Empty_Exit0_HintToStderr(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// No registrations — state file does not exist. store.List returns
	// ([], nil) because loadState synthesizes an empty state on ErrNotExist.

	var out, errW bytes.Buffer
	code := cli.RunList(&out, &errW, path)

	require.Equal(t, 0, code, "LD-5: empty store must still succeed")
	require.Empty(t, out.String(), "empty store MUST NOT write stdout (LD-5)")
	require.Equal(t, "mcp-chain: no entries\n", errW.String())
}

func TestRunList_NEntries_SortedTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path)
	require.NoError(t, err)

	// 2ms sleep between registers → strict CreatedAt ordering (pitfall 2).
	id1, err := st.Register("tok", "first condition")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	id2, err := st.Register("tok", "second condition — this one is LOOOOONG and will be truncated to forty-eight chars with ellipsis")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	id3, err := st.Register("tok", "third")
	require.NoError(t, err)
	require.NoError(t, st.Resolve(id1, "tok", store.ResolveOptions{}))

	var out, errW bytes.Buffer
	code := cli.RunList(&out, &errW, path)

	require.Equal(t, 0, code)
	require.Empty(t, errW.String(), "non-empty list must not write stderr")

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	require.Len(t, lines, 4, "header + 3 rows")
	require.True(t, strings.HasPrefix(lines[0], "ID"), "header row starts with ID")
	require.Contains(t, lines[0], "STATUS")
	require.Contains(t, lines[0], "CONDITION")
	require.Contains(t, lines[0], "CREATED")
	require.Contains(t, lines[0], "RESOLVED")
	require.Contains(t, lines[1], id1, "oldest first (LD-8)")
	require.Contains(t, lines[2], id2)
	require.Contains(t, lines[3], id3)
	require.Contains(t, lines[2], "...", "long condition must be truncated with ellipsis (LD-9)")
}

func TestRunList_OtherError_Exit1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// Corrupt the state file: non-JSON bytes force the generic error
	// branch in runList (via either store.Open or store.List).
	require.NoError(t, os.WriteFile(path, []byte("not valid json"), 0o600))

	var out, errW bytes.Buffer
	code := cli.RunList(&out, &errW, path)

	require.Equal(t, 1, code)
	require.Empty(t, out.String(), "error path: stdout empty")
	require.Contains(t, errW.String(), "mcp-chain: ", "error must be prefixed")
	require.NotContains(t, errW.String(), "no entries", "must take error branch, not empty branch")
}
