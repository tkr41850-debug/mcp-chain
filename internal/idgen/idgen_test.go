package idgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWordlistInvariants re-asserts at test time the invariants that init()
// already panics on. This means a CI `go test ./internal/idgen/...` run is
// sufficient to detect a corrupted wordlist file before any downstream code
// depends on it. (A broken init would surface here as a test-binary panic.)
func TestWordlistInvariants(t *testing.T) {
	require.Len(t, words, wordlistSize, "wordlist must contain exactly 1296 entries")

	seen := make(map[string]struct{}, len(words))
	for i, w := range words {
		require.NotEmpty(t, w, "entry %d is empty", i)
		require.True(t, isValidWord(w), "entry %d %q is not [a-z-]+", i, w)
		require.False(t, strings.HasPrefix(w, "hex-"),
			"entry %d %q collides with hex- fallback prefix", i, w)
		if _, dup := seen[w]; dup {
			t.Fatalf("entry %d %q is a duplicate", i, w)
		}
		seen[w] = struct{}{}
	}
}

// TestWordlistBoundaries pins the first and last entries of the list so an
// accidental EFF file reorder (or an off-by-one in the parser) fails loudly.
func TestWordlistBoundaries(t *testing.T) {
	require.Equal(t, "acid", words[0], "first word must be 'acid' (EFF line 1)")
	require.Equal(t, "zoom", words[wordlistSize-1], "last word must be 'zoom' (EFF line 1296)")
}
