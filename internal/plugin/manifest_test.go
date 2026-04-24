package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoRoot returns the absolute path of the mcp-chain repository root,
// located via runtime.Caller(0) so the tests are CWD-independent.
// Layout assumption: this file lives at internal/plugin/manifest_test.go,
// i.e. two levels below the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed")
	pkgDir := filepath.Dir(thisFile)
	root := filepath.Join(pkgDir, "..", "..")
	abs, err := filepath.Abs(root)
	require.NoError(t, err)
	return abs
}

func readJSON(t *testing.T, path string, v any) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", path)
	require.NoError(t, json.Unmarshal(data, v), "unmarshaling %s", path)
	return data
}

func TestPluginJSON_Valid(t *testing.T) {
	var m struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Author      struct {
			Name string `json:"name"`
		} `json:"author"`
	}
	path := filepath.Join(repoRoot(t), "plugin", ".claude-plugin", "plugin.json")
	readJSON(t, path, &m)
	require.Equal(t, "mcp-chain", m.Name, "plugin name drives /mcp-chain:* namespacing (LD-14)")
	require.NotEmpty(t, m.Version, "version is the cache key (OQ-1)")
	require.GreaterOrEqual(t, len(m.Description), 10, "description should be a real sentence")
	require.NotEmpty(t, m.Author.Name)
}

func TestMarketplaceJSON_Valid(t *testing.T) {
	var m struct {
		Name  string `json:"name"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
		Plugins []struct {
			Name        string `json:"name"`
			Source      string `json:"source"`
			Description string `json:"description"`
		} `json:"plugins"`
	}
	// LD-13: repo-root location, NOT inside plugin/.
	path := filepath.Join(repoRoot(t), ".claude-plugin", "marketplace.json")
	readJSON(t, path, &m)
	require.Equal(t, "mcp-chain-marketplace", m.Name)
	require.NotEmpty(t, m.Owner.Name)
	require.Len(t, m.Plugins, 1, "single plugin entry (v1)")
	require.Equal(t, "mcp-chain", m.Plugins[0].Name)
	require.Equal(t, "./plugin", m.Plugins[0].Source, "relative source path per LD-13")
	require.NotEmpty(t, m.Plugins[0].Description)
}

func TestMcpJSON_Valid(t *testing.T) {
	var m struct {
		McpServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	path := filepath.Join(repoRoot(t), "plugin", ".mcp.json")
	raw := readJSON(t, path, &m)
	entry, ok := m.McpServers["mcp-chain"]
	require.True(t, ok, "mcpServers must contain key 'mcp-chain'")
	require.Equal(t, "${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain", entry.Command,
		"literal ${CLAUDE_PLUGIN_ROOT} — Claude Code substitutes at spawn time (LD-2)")
	require.Equal(t, []string{"serve"}, entry.Args)

	// DIST-01 reject-list: no absolute paths, no npm/uvx/node/python shims.
	rawStr := string(raw)
	for _, forbidden := range []string{"/Users/", "/home/", "C:\\", "\"npm\"", "\"uvx\"", "\"node\"", "\"python\""} {
		require.NotContains(t, rawStr, forbidden,
			".mcp.json MUST NOT contain %q (LD-2 reject-list)", forbidden)
	}
}
