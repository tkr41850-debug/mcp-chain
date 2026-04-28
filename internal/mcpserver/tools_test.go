package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestToolDescriptionsUnderBudget asserts each tool description is
// within the CORE-10 per-tool byte budget (<= 40-token proxy = 160 bytes).
func TestToolDescriptionsUnderBudget(t *testing.T) {
	require.NotEmpty(t, registerDescription)
	require.NotEmpty(t, resolveDescription)
	require.NotEmpty(t, registerWithIDDescription)
	require.LessOrEqual(t, len(registerDescription), 160,
		"registerDescription exceeds 160-byte (~40-token) budget")
	require.LessOrEqual(t, len(resolveDescription), 160,
		"resolveDescription exceeds 160-byte (~40-token) budget")
	require.LessOrEqual(t, len(registerWithIDDescription), 160,
		"registerWithIDDescription exceeds 160-byte (~40-token) budget")
}

// TestToolListUnderBudget asserts the aggregate of all tool descriptions
// is within the CORE-10 tool-list byte budget (<= 200-token proxy = 800 bytes).
func TestToolListUnderBudget(t *testing.T) {
	total := len(registerDescription) + len(resolveDescription) + len(registerWithIDDescription)
	require.LessOrEqual(t, total, 800,
		"aggregate tool-list description bytes exceeds 800 (~200-token) budget")
}
