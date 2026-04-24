package mcpserver

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// hexToken32 matches a 32-character lowercase-hex string. NewOwnerToken
// must produce exactly this shape: 16 random bytes hex-encoded.
var hexToken32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// TestNewOwnerToken_IsHex32Chars asserts the token is exactly 32
// characters of lowercase hex. Any change in byte length or encoder
// breaks SC #2 (128-bit OwnerToken identity).
func TestNewOwnerToken_IsHex32Chars(t *testing.T) {
	tok, err := NewOwnerToken()
	require.NoError(t, err)
	require.Len(t, tok, 32)
	require.True(t, hexToken32.MatchString(tok),
		"token %q does not match ^[0-9a-f]{32}$", tok)
}

// TestNewOwnerToken_Uniqueness draws 1000 tokens and asserts all are
// distinct. A math/rand slip (default seed collisions) would fail this;
// crypto/rand should not.
func TestNewOwnerToken_Uniqueness(t *testing.T) {
	const N = 1000
	seen := make(map[string]struct{}, N)
	for i := 0; i < N; i++ {
		tok, err := NewOwnerToken()
		require.NoError(t, err)
		seen[tok] = struct{}{}
	}
	require.Len(t, seen, N, "expected %d unique tokens, got %d", N, len(seen))
}
