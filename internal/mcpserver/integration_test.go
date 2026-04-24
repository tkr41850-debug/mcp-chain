//go:build integration

// Package mcpserver integration tests use the TestMain re-exec pattern
// from Phase 4 (internal/store/integration_test.go). When a parent test
// spawns os.Executable() with MCP_CHAIN_MCPSERVER_TEST_CHILD=serve set,
// the child runs mcpserver.Run against a caller-supplied state path
// instead of the test harness. This avoids depending on Phase 6 kong
// wiring and keeps all integration code test-local.
package mcpserver_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anthropics/mcp-chain/internal/mcpserver"
	"github.com/anthropics/mcp-chain/internal/store"
)

const childEnvVar = "MCP_CHAIN_MCPSERVER_TEST_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(childEnvVar) == "serve" {
		runServeChild()
		return
	}
	os.Exit(m.Run())
}

func runServeChild() {
	path := os.Getenv("MCP_CHAIN_STATE_PATH")
	if path == "" {
		fmt.Fprintln(os.Stderr, "child: MCP_CHAIN_STATE_PATH unset")
		os.Exit(11)
	}
	st, err := store.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "child: store.Open: %v\n", err)
		os.Exit(12)
	}
	tok, err := mcpserver.NewOwnerToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "child: NewOwnerToken: %v\n", err)
		os.Exit(13)
	}
	if err := mcpserver.Run(context.Background(), st, tok, "test"); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "child: mcpserver.Run: %v\n", err)
		os.Exit(14)
	}
	os.Exit(0)
}

// childSession bundles a spawned child and its stdin/stdout pipes plus
// a combined-stdout buffer for later wire-discipline inspection.
type childSession struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdoutBuf *bytes.Buffer
	scanner   *bufio.Scanner
	nextID    int
}

// spawnChild starts a child against the given state path. Stdout is
// tee'd into stdoutBuf (for TestServe_StdoutIsPureJSONRPC) while also
// feeding a bufio.Scanner for per-line JSON-RPC recv().
func spawnChild(t *testing.T, statePath string) *childSession {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		childEnvVar+"=serve",
		"MCP_CHAIN_STATE_PATH="+statePath,
	)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr

	buf := &bytes.Buffer{}
	mu := &sync.Mutex{}
	teed := &teeReader{r: stdoutPipe, buf: buf, mu: mu}

	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = stdin.Close()
		// give the child a chance to exit cleanly, then kill
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})

	scan := bufio.NewScanner(teed)
	// JSON-RPC frames can be larger than bufio's default 64 KB buffer.
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	return &childSession{
		cmd:       cmd,
		stdin:     stdin,
		stdoutBuf: buf,
		scanner:   scan,
		nextID:    1,
	}
}

// teeReader copies every read through into buf while still handing the
// bytes to the caller. Goroutine-safe via mu (scanner reads in the
// test goroutine; buf is also read from the test goroutine after Wait).
type teeReader struct {
	r   io.Reader
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (t *teeReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.mu.Lock()
		t.buf.Write(p[:n])
		t.mu.Unlock()
	}
	return n, err
}

func (c *childSession) send(t *testing.T, raw string) {
	t.Helper()
	_, err := c.stdin.Write([]byte(raw + "\n"))
	require.NoError(t, err)
}

func (c *childSession) recv(t *testing.T) map[string]any {
	t.Helper()
	require.True(t, c.scanner.Scan(), "expected JSON-RPC frame; scanner err=%v", c.scanner.Err())
	var m map[string]any
	line := c.scanner.Bytes()
	require.NoError(t, json.Unmarshal(line, &m), "stdout must be valid JSON; got: %q", string(line))
	return m
}

func TestServe_StdioFullHandshake(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	c := spawnChild(t, path)

	// 1. initialize
	c.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.0"}}}`)
	r1 := c.recv(t)
	require.Equal(t, float64(1), r1["id"])
	require.NotNil(t, r1["result"])

	// 2. notifications/initialized (no response)
	c.send(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	// 3. tools/list
	c.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	r2 := c.recv(t)
	require.Equal(t, float64(2), r2["id"])

	// 4. tools/call register
	c.send(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"register","arguments":{"condition":"test"}}}`)
	r3 := c.recv(t)
	require.Equal(t, float64(3), r3["id"])
	// Extract id from result.content[0].text (SDK auto-wraps RegisterOut as JSON text).
	id := extractRegisteredID(t, r3)
	require.NotEmpty(t, id)

	// 5. tools/call resolve
	c.send(t, fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"resolve","arguments":{"id":%q}}}`, id))
	r4 := c.recv(t)
	require.Equal(t, float64(4), r4["id"])
	// A successful resolve should NOT be an error
	if result, ok := r4["result"].(map[string]any); ok {
		isErr, _ := result["isError"].(bool)
		require.False(t, isErr, "resolve on owned id should not be an error: %v", r4)
	}
}

// extractRegisteredID walks result.content[0].text (which is a JSON-
// stringified RegisterOut = {"id":"..."}). Returns the id field.
func extractRegisteredID(t *testing.T, resp map[string]any) string {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	require.True(t, ok, "expected result object, got %T: %v", resp["result"], resp)
	content, ok := result["content"].([]any)
	require.True(t, ok, "expected content array, got %T: %v", result["content"], result)
	require.NotEmpty(t, content)
	first, ok := content[0].(map[string]any)
	require.True(t, ok)
	text, _ := first["text"].(string)
	require.NotEmpty(t, text)
	var out struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	return out.ID
}

func TestServe_StdoutIsPureJSONRPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	c := spawnChild(t, path)

	// Drive a small session so there's something to inspect.
	c.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`)
	_ = c.recv(t)
	c.send(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	c.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	_ = c.recv(t)

	// Drain remaining frames if any, then close stdin and wait.
	_ = c.stdin.Close()
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("child did not exit after stdin close")
	}

	// Now scan the captured stdout: every non-empty \n-delimited
	// segment must parse as JSON. No banners, no partial frames.
	raw := c.stdoutBuf.Bytes()
	for i, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var any map[string]any
		if err := json.Unmarshal(line, &any); err != nil {
			t.Fatalf("stdout line %d is not valid JSON: %q (err=%v)", i, line, err)
		}
		// JSON-RPC response/notification must carry jsonrpc: "2.0"
		if v, ok := any["jsonrpc"]; ok {
			require.Equal(t, "2.0", v, "line %d: jsonrpc field != 2.0: %q", i, line)
		}
	}
}

func TestServe_ResolveNotOwnerWireCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Child 1 — register under owner-token A.
	c1 := spawnChild(t, path)
	c1.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"a","version":"0"}}}`)
	_ = c1.recv(t)
	c1.send(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	c1.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"register","arguments":{"condition":"x"}}}`)
	r := c1.recv(t)
	id := extractRegisteredID(t, r)
	_ = c1.stdin.Close()
	// Wait for c1 to release its lock on state.json before starting c2.
	done := make(chan error, 1)
	go func() { done <- c1.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("child 1 did not exit")
	}

	// Child 2 — different process → different OwnerToken → resolve
	// should come back with wire code `not_owner`.
	c2 := spawnChild(t, path)
	c2.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"b","version":"0"}}}`)
	_ = c2.recv(t)
	c2.send(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	c2.send(t, fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"resolve","arguments":{"id":%q}}}`, id))
	r2 := c2.recv(t)

	// Extract the tool-result error body.
	result, ok := r2["result"].(map[string]any)
	require.True(t, ok)
	require.True(t, result["isError"].(bool), "expected IsError=true for cross-process resolve")
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &body))
	require.Equal(t, "not_owner", body.Code,
		"cross-process resolve MUST surface distinct wire code `not_owner` (SC #2)")
}
