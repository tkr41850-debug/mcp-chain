package cli_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// TestStubsExitCodes verifies every subcommand stub exits with ExitCodeNotImplemented (3).
// Builds the binary fresh via `go build` into a temp file to avoid polluting the repo root.
func TestStubsExitCodes(t *testing.T) {
	binPath := buildBinary(t)

	tests := []struct {
		name            string
		args            []string
		wantExit        int
		wantErrContains string // substring expected in stderr
		wantStdoutEmpty bool
	}{
		// Phase 5 wired `serve` to real logic — it no longer exits 3.
		// Its behaviour is covered by internal/mcpserver (unit + integration).
		// Phase 6 wired `status` (see status.go / status_test.go / integration_test.go)
		// so that row is no longer a stub.
		{"list", []string{"list"}, 3, "not implemented (Phase 7)", true},
		{"purge-all", []string{"purge", "--all"}, 3, "not implemented (Phase 7)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binPath, tt.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()

			exitErr, ok := err.(*exec.ExitError)
			if !ok && err != nil {
				t.Fatalf("unexpected error kind: %v", err)
			}
			gotExit := 0
			if exitErr != nil {
				gotExit = exitErr.ExitCode()
			}
			if gotExit != tt.wantExit {
				t.Errorf("exit code: got %d, want %d (stderr=%q)", gotExit, tt.wantExit, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantErrContains) {
				t.Errorf("stderr: got %q, want to contain %q", stderr.String(), tt.wantErrContains)
			}
			if tt.wantStdoutEmpty && stdout.Len() != 0 {
				t.Errorf("stdout: expected empty, got %q (MCP-02 violation)", stdout.String())
			}
		})
	}
}

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
