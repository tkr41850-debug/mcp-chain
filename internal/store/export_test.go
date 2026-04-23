package store

import "time"

// SetWriteDelayForTest installs a synthetic delay BEFORE each atomic
// state write. Returns a restore function that undoes the hook.
//
// Intended exclusively for fault-injection in integration tests (e.g.
// SIGKILL mid-write). The _test.go suffix confines this symbol to the
// test binary — no production code can reference it. This keeps the
// production store package zero-footprint for test hooks.
//
// The approved injection pattern (RESEARCH §Open Questions #3).
func SetWriteDelayForTest(d time.Duration) (restore func()) {
	orig := saveStateFn
	saveStateFn = func(path string, s *state) error {
		time.Sleep(d)
		return orig(path, s)
	}
	return func() { saveStateFn = orig }
}
