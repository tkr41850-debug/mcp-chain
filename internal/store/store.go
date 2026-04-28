package store

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/tkr41850-debug/mcp-chain/internal/idgen"
)

// Store is a handle to a state.json file. Construct via Open. Methods are
// safe for concurrent use from multiple goroutines in the same process
// (serialised by an internal sync.Mutex) and from multiple processes
// (serialised by a sibling .lock file via gofrs/flock).
type Store struct {
	path string
	mu   sync.Mutex
}

// Record is the exported view of a chain entry returned by Get/List.
// A distinct type (vs. the unexported record) keeps on-disk-shape
// decisions private and prevents callers from mutating internal state
// by holding a reference.
type Record struct {
	ID         string
	Condition  string
	Status     string
	OwnerToken string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// PurgeOptions selects which records Purge removes. Exactly one of ID /
// All / Resolved must be set — zero targets or multiple targets both
// return ErrPurgeArgRequired.
type PurgeOptions struct {
	ID       string
	All      bool
	Resolved bool
}

// ResolveOptions modifies Resolve behaviour. Force: true bypasses the
// OwnerToken check (operator recovery; CLI --force in Phase 7).
type ResolveOptions struct {
	Force bool
}

// Open returns a Store handle for the given state-file path. No I/O is
// performed here: the file is read lazily by the first Register/Resolve/
// Get/List/Purge call. Parent-directory existence is the caller's
// responsibility (Phase 3's statepath.Resolve handles it).
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("store: Open requires non-empty path")
	}
	return &Store{path: path}, nil
}

// Register creates a new pending record, allocates a deterministic ID
// via idgen.Allocate, stamps the OwnerToken, and persists atomically
// under LOCK_EX. The returned id is the chain entry's word-ID (or
// hex-NNNN fallback once the wordlist is exhausted).
func (s *Store) Register(ownerToken, condition string) (string, error) {
	var id string
	err := s.withLockedState(func(st *state) error {
		// Allocate BEFORE incrementing: first Register at counter=0
		// must yield words[0] == "acid" (Pitfall 7).
		id = idgen.Allocate(st.Counter)
		st.Counter++
		st.Records[id] = record{
			ID:         id,
			Condition:  condition,
			Status:     statusPending,
			OwnerToken: ownerToken,
			CreatedAt:  time.Now().UTC(),
			ResolvedAt: nil,
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// RegisterWithID creates a new pending record with a caller-supplied
// id (the slug). Used by callers that need a stable, externally-known
// id — e.g. a Claude Code plan slug. Differs from Register in two
// ways: (1) id comes from the caller, not idgen.Allocate; (2) the
// monotonic Counter is NOT incremented (the wordlist allocation space
// is independent of the slug namespace).
//
// Returns ErrInvalidID if id fails validation (1–64 chars, starts with
// [a-z0-9], contains only [a-z0-9._-]) and ErrIDTaken if a record with
// that id already exists. Validation runs before the lock so an
// invalid id never touches state.json.
func (s *Store) RegisterWithID(ownerToken, id, condition string) error {
	if err := validateID(id); err != nil {
		return err
	}
	return s.withLockedState(func(st *state) error {
		if _, ok := st.Records[id]; ok {
			return fmt.Errorf("%w: %q", ErrIDTaken, id)
		}
		st.Records[id] = record{
			ID:         id,
			Condition:  condition,
			Status:     statusPending,
			OwnerToken: ownerToken,
			CreatedAt:  time.Now().UTC(),
			ResolvedAt: nil,
		}
		return nil
	})
}

// validateID enforces the slug grammar accepted by RegisterWithID:
// 1–64 chars, first char [a-z0-9], remaining chars [a-z0-9._-].
// Hand-rolled to avoid pulling regexp into production code (see
// CLAUDE.md "stdlib-first"); table-tested.
func validateID(id string) error {
	n := len(id)
	if n == 0 || n > 64 {
		return fmt.Errorf("%w: length %d (must be 1–64)", ErrInvalidID, n)
	}
	for i := 0; i < n; i++ {
		c := id[i]
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if i > 0 {
			ok = ok || c == '.' || c == '_' || c == '-'
		}
		if !ok {
			return fmt.Errorf("%w: %q (byte %d: %q)", ErrInvalidID, id, i, c)
		}
	}
	return nil
}

// Resolve marks the record with the given id as resolved. Returns
// ErrUnknownID if no such record, ErrAlreadyResolved if it's already
// resolved, and ErrNotOwner if ownerToken does not match the stored
// token (unless opts.Force is true). Token comparison is constant-time
// (crypto/subtle) to prevent byte-by-byte timing side channels on the
// 32-char hex OwnerToken.
func (s *Store) Resolve(id, ownerToken string, opts ResolveOptions) error {
	return s.withLockedState(func(st *state) error {
		r, ok := st.Records[id]
		if !ok {
			return ErrUnknownID
		}
		if r.Status == statusResolved {
			return ErrAlreadyResolved
		}
		if !opts.Force {
			if subtle.ConstantTimeCompare([]byte(ownerToken), []byte(r.OwnerToken)) != 1 {
				return ErrNotOwner
			}
		}
		now := time.Now().UTC()
		r.Status = statusResolved
		r.ResolvedAt = &now
		st.Records[id] = r // map-of-structs: must re-assign.
		return nil
	})
}

// Get returns the Record with the given id under LOCK_SH. Returns
// ErrUnknownID if absent.
func (s *Store) Get(id string) (Record, error) {
	var out Record
	err := s.withSharedLock(func(st *state) error {
		r, ok := st.Records[id]
		if !ok {
			return ErrUnknownID
		}
		out = recordToRecord(r)
		return nil
	})
	return out, err
}

// List returns all Records under LOCK_SH. Order is unspecified (map
// iteration). Empty slice on no records; nil error on missing state
// file.
func (s *Store) List() ([]Record, error) {
	var out []Record
	err := s.withSharedLock(func(st *state) error {
		out = make([]Record, 0, len(st.Records))
		for _, r := range st.Records {
			out = append(out, recordToRecord(r))
		}
		return nil
	})
	return out, err
}

// Purge removes records per opts. Exactly one of opts.ID / opts.All /
// opts.Resolved must be set — zero or multiple targets both return
// ErrPurgeArgRequired. Counter is NEVER decremented; ID allocation is
// monotonic across the lifetime of the state file (CORE-09, Pitfall 8).
func (s *Store) Purge(opts PurgeOptions) (int, error) {
	// Count truthy targets. Exactly one must be set.
	targets := 0
	if opts.ID != "" {
		targets++
	}
	if opts.All {
		targets++
	}
	if opts.Resolved {
		targets++
	}
	if targets != 1 {
		return 0, ErrPurgeArgRequired
	}

	var removed int
	err := s.withLockedState(func(st *state) error {
		switch {
		case opts.ID != "":
			if _, ok := st.Records[opts.ID]; !ok {
				return ErrUnknownID
			}
			delete(st.Records, opts.ID)
			removed = 1
		case opts.All:
			removed = len(st.Records)
			st.Records = map[string]record{}
		case opts.Resolved:
			for id, r := range st.Records {
				if r.Status == statusResolved {
					delete(st.Records, id)
					removed++
				}
			}
		}
		// Counter is intentionally NOT touched (CORE-09).
		return nil
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}

// loadState reads and decodes state.json. Returns a fresh empty state
// (version=1, counter=0, empty records) if the file does not exist.
// Surfaces ErrCorruptJSON or ErrSchemaVersion (both wrapped with
// actionable context) on malformed or mismatched content.
func loadState(path string) (*state, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &state{
			Version: schemaVersion,
			Counter: 0,
			Records: map[string]record{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%w: %s: %v (back up and remove to reset)", ErrCorruptJSON, path, err)
	}
	if s.Version != schemaVersion {
		return nil, fmt.Errorf("%w: got version %d, want %d (file: %s)", ErrSchemaVersion, s.Version, schemaVersion, path)
	}
	if s.Records == nil {
		s.Records = map[string]record{}
	}
	return &s, nil
}

// saveState marshals s and persists it atomically. Separated from
// withLockedState so tests can intercept via saveStateFn.
// writeStateAtomic lives in atomic_unix.go / atomic_windows.go.
func saveState(path string, s *state) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal state: %w", err)
	}
	data = append(data, '\n')
	return writeStateAtomic(path, data)
}

// saveStateFn is the write injection point. Production value is
// saveState; integration tests override via an exported-for-test setter
// (see export_test.go) to inject fault scenarios like slow writes.
var saveStateFn = saveState

// recordToRecord converts the internal record to the exported Record.
// ResolvedAt is deep-copied so callers cannot mutate internal state by
// modifying the returned pointer.
func recordToRecord(r record) Record {
	var resolvedAt *time.Time
	if r.ResolvedAt != nil {
		t := *r.ResolvedAt
		resolvedAt = &t
	}
	return Record{
		ID:         r.ID,
		Condition:  r.Condition,
		Status:     r.Status,
		OwnerToken: r.OwnerToken,
		CreatedAt:  r.CreatedAt,
		ResolvedAt: resolvedAt,
	}
}
