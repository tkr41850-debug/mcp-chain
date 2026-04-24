package cli_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// TestVersionFlagWritesToStdout asserts --version exits 0 and writes to stdout.
// This is the ONE sanctioned stdout write in Phase 1.
func TestVersionFlagWritesToStdout(t *testing.T) {
	binPath := buildBinary(t)
	cmd := exec.Command(binPath, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("--version returned error: %v (stderr=%q)", err, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Errorf("expected version string on stdout; got empty")
	}
	if !strings.Contains(stdout.String(), "mcp-chain") {
		t.Errorf("stdout=%q; expected to contain \"mcp-chain\"", stdout.String())
	}
}

// buildBinary compiles the mcp-chain binary into a tempdir and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	binPath := tmp + "/mcp-chain"
	build := exec.Command("go", "build", "-o", binPath, "../../cmd/mcp-chain")
	var stderr bytes.Buffer
	build.Stderr = &stderr
	if err := build.Run(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, stderr.String())
	}
	return binPath
}
