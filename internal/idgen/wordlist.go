// Package idgen generates short, deterministic IDs for chain entries.
// Entries 0..1295 come from the EFF short wordlist (embedded below).
// Entries >= 1296 fall back to zero-padded lowercase hex ("hex-0001" onward).
//
// The EFF short wordlist (eff_short_wordlist_1.txt) was downloaded from
//
//	https://www.eff.org/files/2016/09/08/eff_short_wordlist_1.txt
//
// and is redistributed here verbatim under the Creative Commons
// Attribution 4.0 International License (CC-BY 4.0). See NOTICE for details.
package idgen

import (
	_ "embed"
	"fmt"
	"strings"
)

// wordlistSize is the invariant count of entries in the EFF short wordlist.
// If EFF ever re-releases the file with a different count, the build fails
// loudly at init() rather than silently shifting IDs.
const wordlistSize = 1296

//go:embed eff_short_wordlist_1.txt
var rawWordlist string

// words is the parsed wordlist. Package-private; read-only after init().
// Safe for concurrent read access from any goroutine.
var words []string

func init() {
	words = parseAndValidate(rawWordlist)
}

// parseAndValidate splits the TSV, extracts the word column, and enforces:
//   - exactly wordlistSize entries
//   - every line has a tab separator
//   - every word matches [a-z-]+ (the hyphen admits exactly one EFF entry: "yo-yo")
//   - no word begins with "hex-" (would collide with the fallback scheme)
//   - no duplicate words
//
// Any violation panics. A broken wordlist is a broken build.
func parseAndValidate(raw string) []string {
	// EFF file is LF-terminated ASCII; accept an optional single trailing newline.
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) != wordlistSize {
		panic(fmt.Sprintf("idgen: wordlist line count = %d, want %d", len(lines), wordlistSize))
	}

	out := make([]string, 0, wordlistSize)
	seen := make(map[string]struct{}, wordlistSize)
	for i, line := range lines {
		_, word, ok := strings.Cut(line, "\t")
		if !ok {
			panic(fmt.Sprintf("idgen: line %d missing tab separator: %q", i+1, line))
		}
		if word == "" {
			panic(fmt.Sprintf("idgen: line %d has empty word", i+1))
		}
		if !isValidWord(word) {
			panic(fmt.Sprintf("idgen: line %d has invalid word %q (want [a-z-]+)", i+1, word))
		}
		if strings.HasPrefix(word, "hex-") {
			panic(fmt.Sprintf("idgen: line %d word %q collides with hex- fallback prefix", i+1, word))
		}
		if _, dup := seen[word]; dup {
			panic(fmt.Sprintf("idgen: line %d duplicate word %q", i+1, word))
		}
		seen[word] = struct{}{}
		out = append(out, word)
	}
	return out
}

// isValidWord returns true iff w contains only lowercase ASCII letters
// or ASCII hyphen. The hyphen is present in exactly one EFF entry ("yo-yo",
// line 1286); every other entry is 3-5 lowercase letters.
func isValidWord(w string) bool {
	for _, r := range w {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
