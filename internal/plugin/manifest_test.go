package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	require.Equal(t, "${CLAUDE_PLUGIN_ROOT}/scripts/mcp-chain-exec.sh", entry.Command,
		"wrapper self-installs the release binary into ${CLAUDE_PLUGIN_DATA} on first use")
	require.Equal(t, []string{"serve"}, entry.Args)

	// DIST-01 reject-list: no absolute paths, no npm/uvx/node/python shims.
	rawStr := string(raw)
	for _, forbidden := range []string{"/Users/", "/home/", "C:\\", "\"npm\"", "\"uvx\"", "\"node\"", "\"python\""} {
		require.NotContains(t, rawStr, forbidden,
			".mcp.json MUST NOT contain %q (LD-2 reject-list)", forbidden)
	}
}

// stripFrontmatter returns the body of a commands/*.md file with YAML
// frontmatter (between --- markers) removed. Mirrors the awk logic in
// scripts/check-prompt-wordcount.sh.
func stripFrontmatter(raw string) string {
	// Tolerate CRLF checkouts on Windows (belt-and-suspenders with
	// .gitattributes eol=lf) — otherwise line == "---" never matches and
	// the whole file falls through to the body, inflating the word count.
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	n := 0
	var body []string
	for _, line := range lines {
		if line == "---" {
			n++
			continue
		}
		if n == 1 {
			continue // inside frontmatter
		}
		body = append(body, line)
	}
	return strings.Join(body, "\n")
}

// countWords is a whitespace tokenizer (runs of non-whitespace).
func countWords(s string) int {
	return len(strings.Fields(s))
}

func readPrompt(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "plugin", "commands", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestPromptReg_MentionsRegisterTool(t *testing.T) {
	raw := readPrompt(t, "reg.md")
	body := stripFrontmatter(raw)
	require.LessOrEqual(t, countWords(body), 30, "LD-3 word budget")
	require.Contains(t, body, "mcp-chain__register", "must name the MCP tool (LD-3)")
	require.NotContains(t, raw, "/chain-reg", "no double-prefix literal (LD-14)")
	require.Contains(t, raw, "description:", "frontmatter description field (LD-10)")
	require.Contains(t, raw, "argument-hint:", "frontmatter argument-hint (LD-10)")
}

func TestPromptWait_InvokesChainWaitScript(t *testing.T) {
	raw := readPrompt(t, "wait.md")
	body := stripFrontmatter(raw)
	require.LessOrEqual(t, countWords(body), 30)
	require.Contains(t, body, "${CLAUDE_PLUGIN_ROOT}/scripts/chain-wait.sh", "LD-9 wiring")
	require.Contains(t, body, "MCP_CHAIN_BIN=", "LD-9 env override")
	require.Contains(t, body, "$ARGUMENTS", "pass-through arg substitution")
	require.NotContains(t, raw, "/chain-wait ", "no double-prefix slash-command literal (LD-14)")
}

func TestPromptList_InvokesBinaryList(t *testing.T) {
	raw := readPrompt(t, "list.md")
	body := stripFrontmatter(raw)
	require.LessOrEqual(t, countWords(body), 30)
	require.Contains(t, body, "${CLAUDE_PLUGIN_ROOT}/scripts/mcp-chain-exec.sh")
	require.Contains(t, body, " list")
	require.NotContains(t, raw, "/chain-list", "no double-prefix literal (LD-14)")
}

func TestPromptPurge_TrustsBinaryForBareArgs(t *testing.T) {
	raw := readPrompt(t, "purge.md")
	body := stripFrontmatter(raw)
	require.LessOrEqual(t, countWords(body), 30)
	require.Contains(t, body, "purge $ARGUMENTS")
	require.NotContains(t, raw, "/chain-purge", "no double-prefix literal (LD-14)")
	// LD-11: prompt MUST NOT re-enforce arg-shape. Check for common
	// re-enforcement phrases that would violate the <=30-word budget anyway.
	for _, forbidden := range []string{"refuse", "if no argument provided", "you must provide"} {
		require.NotContains(t, strings.ToLower(body), forbidden,
			"LD-11: binary enforces arg-shape, prompt does not")
	}
}
