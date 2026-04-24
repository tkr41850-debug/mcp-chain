// Package mcpserver is the MCP stdio adapter that exposes the Phase 4
// internal/store via register/resolve tools. It is the ONLY package
// permitted to import github.com/modelcontextprotocol/go-sdk (MCP-03).
package mcpserver

import (
	"crypto/rand"
	"encoding/hex"
)

// NewOwnerToken returns a 32-character hex string carrying 128 bits
// of crypto/rand entropy. Called exactly once per `mcp-chain serve`
// process (Pitfall 4 — never per handler call).
func NewOwnerToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
