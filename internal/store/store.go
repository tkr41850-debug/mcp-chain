package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
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

// Register is a stub wired in Task 1.4.
func (s *Store) Register(ownerToken, condition string) (string, error) {
	_ = ownerToken
	_ = condition
	return "", errors.New("store: not implemented")
}

// Resolve is a stub wired in Task 1.4.
func (s *Store) Resolve(id, ownerToken string, opts ResolveOptions) error {
	_ = id
	_ = ownerToken
	_ = opts
	return errors.New("store: not implemented")
}

// Get is a stub wired in Task 1.4.
func (s *Store) Get(id string) (Record, error) {
	_ = id
	return Record{}, errors.New("store: not implemented")
}

// List is a stub wired in Task 1.4.
func (s *Store) List() ([]Record, error) {
	return nil, errors.New("store: not implemented")
}

// Purge is a stub wired in Task 1.4.
func (s *Store) Purge(opts PurgeOptions) (int, error) {
	_ = opts
	return 0, errors.New("store: not implemented")
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
//
// NOTE: writeStateAtomic lives in atomic_unix.go / atomic_windows.go.
// This function is not called until Task 1.3 lands those files.
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
