package idgen

import "fmt"

// Allocate returns a deterministic ID for the given monotonic counter.
//
//	counter ∈ [0, 1295]            → words[counter]
//	counter ∈ [1296, 1296+65535]   → "hex-0001" .. "hex-ffff" (4-digit zero-pad)
//	counter > 1296+65535           → "hex-<5+ hex digits>" (%04x is min-width,
//	                                  not truncation — widens automatically)
//
// Pure, allocation-light (one string for the hex branch), and safe for
// concurrent use (words is immutable after init).
//
// Requirement: CORE-07.
func Allocate(counter uint64) string {
	if counter < uint64(len(words)) {
		return words[counter]
	}
	// Offset so the first fallback (counter=1296) renders as "hex-0001",
	// not "hex-0000". Subtract len(words)-1 instead of len(words).
	return fmt.Sprintf("hex-%04x", counter-uint64(len(words)-1))
}
