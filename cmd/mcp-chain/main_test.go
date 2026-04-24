// Package main tests — in-repo smoke tests that mirror the CI enforcement
// gates (scripts/check-*.sh). Running `go test ./...` locally reproduces
// the same invariants CI checks, without shelling out to Makefile gates.
package main_test

import (
	"bytes"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestVersionOutput asserts `mcp-chain --version` writes a line matching
// ^mcp-chain to stdout and exits 0. This is the ONE sanctioned stdout
// write in Phase 1 (per RESEARCH.md Pattern 1 caveat).
func TestVersionOutput(t *testing.T) {
	bin := buildMain(t)
	cmd := exec.Command(bin, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("--version returned error: %v (stderr=%q)", err, stderr.String())
	}

	got := stdout.String()
	re := regexp.MustCompile(`^mcp-chain\s+\S+`)
	if !re.MatchString(strings.TrimSpace(got)) {
		t.Errorf("stdout=%q; expected to match %q", got, re.String())
	}
}

// TestServeStdoutSilence asserts `mcp-chain serve </dev/null` writes exactly
// 0 bytes to stdout, regardless of exit code. This is the Go mirror of
// scripts/check-stdout-silence.sh and enforces MCP-02 from the earliest
// possible point (go test). Phase 1 stub exits 3; real Phase 5 serve will
// keep stdout silent except for MCP wire traffic (which this test doesn't
// exercise — by Phase 5 this test adapts to pipe an initialize handshake).
func TestServeStdoutSilence(t *testing.T) {
	bin := buildMain(t)
	cmd := exec.Command(bin, "serve")
	cmd.Stdin = strings.NewReader("") // </dev/null equivalent
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Exit code is tolerated — Phase 1 stub exits 3. We only assert on stdout.
	_ = cmd.Run()

	if stdout.Len() != 0 {
		t.Errorf("serve wrote %d bytes to stdout (expected 0): %q [MCP-02 violation]",
			stdout.Len(), stdout.String())
	}
	// Stderr should contain the stub message — sanity check that stub ran.
	if !strings.Contains(stderr.String(), "not implemented") {
		t.Logf("stderr did not contain stub message; may indicate real serve has landed (update this test)")
	}
}

// buildMain compiles cmd/mcp-chain into a tempdir. Returns the binary path.
// Skips the calling test if `go build` is unavailable in the environment.
func buildMain(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	binPath := tmp + "/mcp-chain"
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	build := exec.Command("go", "build", "-o", binPath, ".")
	var stderr bytes.Buffer
	build.Stderr = &stderr
	if err := build.Run(); err != nil {
		if _, ok := err.(*exec.Error); ok {
			t.Skipf("go build unavailable in this environment: %v", err)
		}
		t.Fatalf("go build failed: %v\n%s", err, stderr.String())
	}
	return binPath
}
