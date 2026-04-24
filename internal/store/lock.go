package store

import (
	"fmt"

	"github.com/gofrs/flock"
)

// lockFilePath returns the flock target path for a given state path.
//
// We always use a sibling .lock file on both unix and windows:
//   - Uniform cross-platform semantics.
//   - Avoids the POSIX "rename changes inode, held-lock continues on
//     orphan inode" latent race (RESEARCH §Open Questions #1).
//   - Windows cannot lock a file under active rename; a sibling lock is
//     the standard solution (RESEARCH §Pattern 5).
func lockFilePath(statePath string) string {
	return statePath + ".lock"
}

// withLockedState serialises an exclusive read-modify-write cycle over
// state.json. The mutation closure fn receives a decoded *state; on
// return, the state is re-marshalled and written atomically via
// saveStateFn.
//
// Locking order: s.mu (same-process serialise) → flock.Lock (cross-
// process serialise). The mutex MUST be taken before flock: two
// goroutines in the same process share a single file-descriptor-level
// flock, so without the mutex both would "acquire" the lock and
// interleave their RMW (RESEARCH §Pitfall 4).
func (s *Store) withLockedState(fn func(*state) error) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(lockFilePath(s.path))
	if lerr := fl.Lock(); lerr != nil {
		return fmt.Errorf("store: acquire exclusive lock on %s: %w", fl.Path(), lerr)
	}
	defer func() {
		if uerr := fl.Unlock(); uerr != nil && err == nil {
			err = fmt.Errorf("store: release exclusive lock: %w", uerr)
		}
	}()

	st, err := loadState(s.path)
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		return err
	}
	return saveStateFn(s.path, st)
}

// withSharedLock serialises a read-only load of state.json under a
// shared flock. Multiple readers across processes can hold it
// concurrently; writers (withLockedState) block until all readers
// release.
//
// s.mu is taken even on the read path (RESEARCH §Open Questions #2):
// cheap, avoids surprises when callers mix read+write from the same
// process. An RWMutex optimisation is deferred until profiled need.
func (s *Store) withSharedLock(fn func(*state) error) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(lockFilePath(s.path))
	if lerr := fl.RLock(); lerr != nil {
		return fmt.Errorf("store: acquire shared lock on %s: %w", fl.Path(), lerr)
	}
	defer func() {
		if uerr := fl.Unlock(); uerr != nil && err == nil {
			err = fmt.Errorf("store: release shared lock: %w", uerr)
		}
	}()

	st, err := loadState(s.path)
	if err != nil {
		return err
	}
	return fn(st)
}
