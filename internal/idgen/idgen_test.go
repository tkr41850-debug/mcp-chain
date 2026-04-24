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

// TestAllocate is the table-driven boundary test required by CORE-07.
// Covers: lower bound, lower+1, wordlist end, fallback start, fallback end
// at 4-digit boundary, and widening past 4 digits.
func TestAllocate(t *testing.T) {
	tests := []struct {
		name    string
		counter uint64
		want    string
	}{
		{"first word", 0, "acid"},
		{"second word", 1, "acorn"},
		{"last word", 1295, "zoom"},
		{"first fallback (hex-0001)", 1296, "hex-0001"},
		{"second fallback (hex-0002)", 1297, "hex-0002"},
		{"fallback at hex-fffe", 66829, "hex-fffe"},  // 66829 - 1295 = 65534 = 0xfffe
		{"boundary before widen", 66830, "hex-ffff"}, // 66830 - 1295 = 65535 = 0xffff
		{"widen to 5 digits", 66831, "hex-10000"},    // 66831 - 1295 = 65536 = 0x10000
		{"widen + 1", 66832, "hex-10001"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, Allocate(tc.counter))
		})
	}
}

// TestParseAndValidate_CRLF asserts the parser accepts a CRLF-line-ended
// wordlist (Windows git checkout without .gitattributes eol=lf). Regression
// guard for the "acid\r" panic seen in CI test (windows-latest) in 2026-04.
func TestParseAndValidate_CRLF(t *testing.T) {
	crlf := strings.ReplaceAll(rawWordlist, "\n", "\r\n")
	got := parseAndValidate(crlf)
	require.Len(t, got, wordlistSize)
	require.Equal(t, "acid", got[0])
	require.Equal(t, "zoom", got[wordlistSize-1])
}

// TestAllocateMonotonicUniqueOverBoundary asserts that no two counters in
// [1290, 1310] produce the same string. Catches off-by-one at the
// wordlist→hex handoff.
func TestAllocateMonotonicUniqueOverBoundary(t *testing.T) {
	seen := make(map[string]uint64, 21)
	for c := uint64(1290); c <= 1310; c++ {
		got := Allocate(c)
		if prev, dup := seen[got]; dup {
			t.Fatalf("counter %d and %d both produced %q", prev, c, got)
		}
		seen[got] = c
	}
}
