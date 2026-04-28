package store_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tkr41850-debug/mcp-chain/internal/idgen"
	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

// 32-char hex-like tokens. Match the MCP-server's 128-bit hex format in
// length and alphabet so tests exercise the real size without requiring
// crypto/rand at test time.
var (
	tokenA = strings.Repeat("a", 32)
	tokenB = strings.Repeat("b", 32)
)

// newStore returns a fresh Store backed by a tempdir state.json. The
// tempdir is cleaned up automatically at test end via t.TempDir.
func newStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := store.Open(path)
	require.NoError(t, err)
	return s, path
}

// readStateFile decodes state.json into a generic map for direct
// assertions on on-disk shape (e.g. "resolved_at: null").
func readStateFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	return m
}

// -----------------------------------------------------------------------
// CORE-04 / CORE-05 — schema, load, version
// -----------------------------------------------------------------------

func TestStore_OpenNoIO(t *testing.T) {
	// Open must not touch the filesystem. Pass a path under a directory
	// that does not exist; Open should still succeed.
	s, err := store.Open("/definitely/does/not/exist/state.json")
	require.NoError(t, err)
	require.NotNil(t, s)
}

func TestStore_RegisterAllocatesFirstWord(t *testing.T) {
	s, _ := newStore(t)
	id, err := s.Register(tokenA, "first cond")
	require.NoError(t, err)
	require.Equal(t, "acid", id, "first allocate must be words[0] (counter=0)")
}

func TestStore_RegisterMonotonicCounter(t *testing.T) {
	s, path := newStore(t)

	id1, err := s.Register(tokenA, "c1")
	require.NoError(t, err)
	id2, err := s.Register(tokenA, "c2")
	require.NoError(t, err)

	require.Equal(t, "acid", id1)
	require.Equal(t, "acorn", id2, "second allocate must be words[1]")

	disk := readStateFile(t, path)
	counter, ok := disk["counter"].(float64)
	require.True(t, ok, "counter must be JSON number")
	require.Equal(t, float64(2), counter, "counter persists as 2 after two Registers")
}

func TestStore_SchemaVersionMismatchErrors(t *testing.T) {
	s, path := newStore(t)

	// Write a state file with wrong version BEFORE any operation.
	require.NoError(t, os.WriteFile(path, []byte(`{"version":2,"counter":0,"records":{}}`), 0o600))

	_, err := s.Register(tokenA, "cond")
	require.Error(t, err)
	require.Truef(t, errors.Is(err, store.ErrSchemaVersion),
		"expected ErrSchemaVersion, got %v", err)
	require.Contains(t, err.Error(), path, "error should name the path")
	require.Contains(t, err.Error(), "version 2")
}

func TestStore_CorruptJSONErrors(t *testing.T) {
	s, path := newStore(t)

	require.NoError(t, os.WriteFile(path, []byte(`not json at all`), 0o600))

	_, err := s.Register(tokenA, "cond")
	require.Error(t, err)
	require.Truef(t, errors.Is(err, store.ErrCorruptJSON),
		"expected ErrCorruptJSON, got %v", err)
	require.Contains(t, err.Error(), path)
	require.Contains(t, err.Error(), "back up and remove")
}

func TestStore_StateFileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not enforced on Windows; see CORE-11")
	}
	s, path := newStore(t)
	_, err := s.Register(tokenA, "cond")
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"state.json must be mode 0600 regardless of umask (renameio.IgnoreUmask)")
}

func TestStore_ResolvedAtNullInJSON(t *testing.T) {
	s, path := newStore(t)
	_, err := s.Register(tokenA, "cond")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), `"resolved_at": null`,
		"unresolved records must marshal resolved_at as JSON null, not zero-time (Pitfall 6)")
}

func TestStore_LoadMissingStateReturnsEmpty(t *testing.T) {
	s, _ := newStore(t)
	// No Register yet — state.json does not exist.
	recs, err := s.List()
	require.NoError(t, err)
	require.Empty(t, recs, "missing state file yields empty slice with no error")
}

// -----------------------------------------------------------------------
// CORE-08 — OwnerToken semantics
// -----------------------------------------------------------------------

func TestStore_RegisterStoresOwnerToken(t *testing.T) {
	s, path := newStore(t)
	id, err := s.Register(tokenA, "cond")
	require.NoError(t, err)

	disk := readStateFile(t, path)
	records, ok := disk["records"].(map[string]any)
	require.True(t, ok)
	r, ok := records[id].(map[string]any)
	require.True(t, ok)
	require.Equal(t, tokenA, r["owner_token"], "owner_token must be persisted verbatim")
}

func TestStore_ResolveOwnerOk(t *testing.T) {
	s, _ := newStore(t)
	id, err := s.Register(tokenA, "cond")
	require.NoError(t, err)

	require.NoError(t, s.Resolve(id, tokenA, store.ResolveOptions{}))

	r, err := s.Get(id)
	require.NoError(t, err)
	require.Equal(t, "resolved", r.Status)
}

func TestStore_ResolveWrongOwnerReturnsErrNotOwner(t *testing.T) {
	s, _ := newStore(t)
	id, err := s.Register(tokenA, "cond")
	require.NoError(t, err)

	err = s.Resolve(id, tokenB, store.ResolveOptions{})
	require.Truef(t, errors.Is(err, store.ErrNotOwner),
		"expected ErrNotOwner, got %v", err)
}

func TestStore_ResolveForceBypassesOwnerCheck(t *testing.T) {
	s, _ := newStore(t)
	id, err := s.Register(tokenA, "cond")
	require.NoError(t, err)

	require.NoError(t, s.Resolve(id, tokenB, store.ResolveOptions{Force: true}))

	r, err := s.Get(id)
	require.NoError(t, err)
	require.Equal(t, "resolved", r.Status)
}

func TestStore_ResolveAlreadyResolvedReturnsErr(t *testing.T) {
	s, _ := newStore(t)
	id, err := s.Register(tokenA, "cond")
	require.NoError(t, err)
	require.NoError(t, s.Resolve(id, tokenA, store.ResolveOptions{}))

	err = s.Resolve(id, tokenA, store.ResolveOptions{})
	require.Truef(t, errors.Is(err, store.ErrAlreadyResolved),
		"expected ErrAlreadyResolved, got %v", err)
}

func TestStore_ResolveUnknownIDReturnsErr(t *testing.T) {
	s, _ := newStore(t)
	err := s.Resolve("not-an-id", tokenA, store.ResolveOptions{})
	require.Truef(t, errors.Is(err, store.ErrUnknownID),
		"expected ErrUnknownID, got %v", err)
}

func TestStore_ResolveSetsResolvedAt(t *testing.T) {
	s, path := newStore(t)
	id, err := s.Register(tokenA, "cond")
	require.NoError(t, err)
	require.NoError(t, s.Resolve(id, tokenA, store.ResolveOptions{}))

	// In-memory: ResolvedAt set + status resolved.
	r, err := s.Get(id)
	require.NoError(t, err)
	require.Equal(t, "resolved", r.Status)
	require.NotNil(t, r.ResolvedAt)

	// On disk: resolved_at is a non-null string (RFC3339).
	disk := readStateFile(t, path)
	records := disk["records"].(map[string]any)
	rec := records[id].(map[string]any)
	require.NotNil(t, rec["resolved_at"])
	_, ok := rec["resolved_at"].(string)
	require.True(t, ok, "resolved_at persists as RFC3339 string, got %T", rec["resolved_at"])
	require.Equal(t, "resolved", rec["status"])
}

// -----------------------------------------------------------------------
// CORE-04 — Get / List
// -----------------------------------------------------------------------

func TestStore_Get(t *testing.T) {
	s, _ := newStore(t)
	id, err := s.Register(tokenA, "get-cond")
	require.NoError(t, err)

	r, err := s.Get(id)
	require.NoError(t, err)
	require.Equal(t, id, r.ID)
	require.Equal(t, "get-cond", r.Condition)
	require.Equal(t, tokenA, r.OwnerToken)
	require.Equal(t, "pending", r.Status)
}

func TestStore_GetUnknownIDReturnsErr(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.Get("not-an-id")
	require.Truef(t, errors.Is(err, store.ErrUnknownID),
		"expected ErrUnknownID, got %v", err)
}

func TestStore_List(t *testing.T) {
	s, _ := newStore(t)
	var ids []string
	for i := 0; i < 3; i++ {
		id, err := s.Register(tokenA, fmt.Sprintf("cond-%d", i))
		require.NoError(t, err)
		ids = append(ids, id)
	}

	recs, err := s.List()
	require.NoError(t, err)
	require.Len(t, recs, 3)

	got := make(map[string]struct{}, 3)
	for _, r := range recs {
		got[r.ID] = struct{}{}
	}
	for _, want := range ids {
		_, ok := got[want]
		require.Truef(t, ok, "List missing id %q", want)
	}
}

func TestStore_ListEmptyWhenNoState(t *testing.T) {
	s, _ := newStore(t)
	recs, err := s.List()
	require.NoError(t, err)
	require.Empty(t, recs)
	require.NotNil(t, recs, "List returns non-nil empty slice, not nil")
}

// -----------------------------------------------------------------------
// CORE-09 — Purge
// -----------------------------------------------------------------------

func TestStore_PurgeByID(t *testing.T) {
	s, _ := newStore(t)
	var ids []string
	for i := 0; i < 3; i++ {
		id, err := s.Register(tokenA, fmt.Sprintf("c-%d", i))
		require.NoError(t, err)
		ids = append(ids, id)
	}

	removed, err := s.Purge(store.PurgeOptions{ID: ids[1]})
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	recs, err := s.List()
	require.NoError(t, err)
	require.Len(t, recs, 2)

	// The purged ID is gone.
	_, err = s.Get(ids[1])
	require.Truef(t, errors.Is(err, store.ErrUnknownID),
		"purged id should not resolve in Get, got %v", err)
}

func TestStore_PurgeByIDUnknownReturnsErr(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.Purge(store.PurgeOptions{ID: "not-an-id"})
	require.Truef(t, errors.Is(err, store.ErrUnknownID),
		"purge unknown id → ErrUnknownID, got %v", err)
}

func TestStore_PurgeAll(t *testing.T) {
	s, _ := newStore(t)
	for i := 0; i < 3; i++ {
		_, err := s.Register(tokenA, fmt.Sprintf("c-%d", i))
		require.NoError(t, err)
	}

	removed, err := s.Purge(store.PurgeOptions{All: true})
	require.NoError(t, err)
	require.Equal(t, 3, removed)

	recs, err := s.List()
	require.NoError(t, err)
	require.Empty(t, recs)
}

func TestStore_PurgeResolved(t *testing.T) {
	s, _ := newStore(t)
	var ids []string
	for i := 0; i < 3; i++ {
		id, err := s.Register(tokenA, fmt.Sprintf("c-%d", i))
		require.NoError(t, err)
		ids = append(ids, id)
	}
	// Resolve ids[0] and ids[2].
	require.NoError(t, s.Resolve(ids[0], tokenA, store.ResolveOptions{}))
	require.NoError(t, s.Resolve(ids[2], tokenA, store.ResolveOptions{}))

	removed, err := s.Purge(store.PurgeOptions{Resolved: true})
	require.NoError(t, err)
	require.Equal(t, 2, removed)

	recs, err := s.List()
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, ids[1], recs[0].ID, "only the non-resolved record should remain")
}

func TestStore_PurgeDoesNotDecrementCounter(t *testing.T) {
	s, path := newStore(t)

	// Register 3 → counter=3.
	for i := 0; i < 3; i++ {
		_, err := s.Register(tokenA, fmt.Sprintf("c-%d", i))
		require.NoError(t, err)
	}
	removed, err := s.Purge(store.PurgeOptions{All: true})
	require.NoError(t, err)
	require.Equal(t, 3, removed)

	// Counter on disk must still be 3.
	disk := readStateFile(t, path)
	require.Equal(t, float64(3), disk["counter"], "Purge must NOT decrement counter")

	// Register 1 more → id must be idgen.Allocate(3), NOT "acid" again.
	newID, err := s.Register(tokenA, "after-purge")
	require.NoError(t, err)
	require.NotEqual(t, "acid", newID, "counter must not have reset to 0")
	require.Equal(t, idgen.Allocate(3), newID,
		"new id must match idgen.Allocate(3) — the monotonic counter post-purge")
}

func TestStore_PurgeRequiresTarget(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.Purge(store.PurgeOptions{})
	require.Truef(t, errors.Is(err, store.ErrPurgeArgRequired),
		"zero targets → ErrPurgeArgRequired, got %v", err)

	// Also fail with multiple targets set (no precedence).
	_, err = s.Purge(store.PurgeOptions{All: true, Resolved: true})
	require.Truef(t, errors.Is(err, store.ErrPurgeArgRequired),
		"multiple targets → ErrPurgeArgRequired, got %v", err)
}

// -----------------------------------------------------------------------
// reg-plan slice 1 — RegisterWithID (caller-supplied id)
// -----------------------------------------------------------------------

func TestStore_RegisterWithID_HappyPath(t *testing.T) {
	s, path := newStore(t)

	require.NoError(t, s.RegisterWithID(tokenA, "phase-05", "plan complete"))

	r, err := s.Get("phase-05")
	require.NoError(t, err)
	require.Equal(t, "phase-05", r.ID)
	require.Equal(t, "plan complete", r.Condition)
	require.Equal(t, "pending", r.Status)
	require.Equal(t, tokenA, r.OwnerToken)

	// Counter must NOT be incremented — slug namespace is independent
	// of the wordlist allocation counter.
	disk := readStateFile(t, path)
	require.Equal(t, float64(0), disk["counter"],
		"RegisterWithID must not increment counter")
}

func TestStore_RegisterWithID_DoesNotConsumeWordlistSlot(t *testing.T) {
	s, _ := newStore(t)

	require.NoError(t, s.RegisterWithID(tokenA, "phase-05", "cond"))

	// First auto-allocate after RegisterWithID must still be words[0].
	id, err := s.Register(tokenA, "next")
	require.NoError(t, err)
	require.Equal(t, "acid", id,
		"RegisterWithID must not advance the wordlist counter")
}

func TestStore_RegisterWithID_CollisionWithPriorRegisterWithID(t *testing.T) {
	s, _ := newStore(t)
	require.NoError(t, s.RegisterWithID(tokenA, "phase-05", "first"))

	err := s.RegisterWithID(tokenA, "phase-05", "second")
	require.Truef(t, errors.Is(err, store.ErrIDTaken),
		"second RegisterWithID with same id → ErrIDTaken, got %v", err)
}

func TestStore_RegisterWithID_CollisionWithAutoAllocated(t *testing.T) {
	s, _ := newStore(t)

	// Auto-allocate gives us "acid" first.
	id, err := s.Register(tokenA, "auto")
	require.NoError(t, err)
	require.Equal(t, "acid", id)

	err = s.RegisterWithID(tokenA, "acid", "manual")
	require.Truef(t, errors.Is(err, store.ErrIDTaken),
		"RegisterWithID over an auto-allocated id → ErrIDTaken, got %v", err)
}

func TestStore_RegisterWithID_CollisionPreservesOriginalRecord(t *testing.T) {
	s, _ := newStore(t)
	require.NoError(t, s.RegisterWithID(tokenA, "phase-05", "first"))

	// Collision attempt with a different owner and condition.
	err := s.RegisterWithID(tokenB, "phase-05", "second")
	require.Truef(t, errors.Is(err, store.ErrIDTaken), "got %v", err)

	// Original record's owner and condition must be unchanged.
	r, err := s.Get("phase-05")
	require.NoError(t, err)
	require.Equal(t, "first", r.Condition)
	require.Equal(t, tokenA, r.OwnerToken)
}

func TestStore_RegisterWithID_OwnerSemanticsMatchRegister(t *testing.T) {
	s, _ := newStore(t)
	require.NoError(t, s.RegisterWithID(tokenA, "phase-05", "cond"))

	// Wrong owner cannot resolve.
	err := s.Resolve("phase-05", tokenB, store.ResolveOptions{})
	require.Truef(t, errors.Is(err, store.ErrNotOwner),
		"wrong owner → ErrNotOwner, got %v", err)

	// Correct owner can resolve.
	require.NoError(t, s.Resolve("phase-05", tokenA, store.ResolveOptions{}))

	r, err := s.Get("phase-05")
	require.NoError(t, err)
	require.Equal(t, "resolved", r.Status)
}

func TestStore_RegisterWithID_Validation(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"too long (65)", strings.Repeat("a", 65)},
		{"uppercase", "Phase-05"},
		{"leading dash", "-phase-05"},
		{"leading dot", ".phase"},
		{"leading underscore", "_phase"},
		{"space", "phase 05"},
		{"slash", "phase/05"},
		{"colon", "phase:05"},
		{"unicode", "phase-é"},
		{"only dots", "..."},
		{"trailing newline", "phase-05\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			err := s.RegisterWithID(tokenA, tc.id, "cond")
			require.Truef(t, errors.Is(err, store.ErrInvalidID),
				"id %q must yield ErrInvalidID, got %v", tc.id, err)
		})
	}
}

func TestStore_RegisterWithID_ValidationDoesNotTouchState(t *testing.T) {
	s, path := newStore(t)

	// First register a valid record to ensure state.json exists with counter=0.
	require.NoError(t, s.RegisterWithID(tokenA, "phase-05", "cond"))

	disk := readStateFile(t, path)
	preCounter := disk["counter"]

	// Now an invalid id — must error without modifying state.
	err := s.RegisterWithID(tokenA, "INVALID", "cond")
	require.Truef(t, errors.Is(err, store.ErrInvalidID), "got %v", err)

	disk = readStateFile(t, path)
	require.Equal(t, preCounter, disk["counter"],
		"invalid id must not modify counter")
	records := disk["records"].(map[string]any)
	require.Len(t, records, 1, "invalid id must not add records")
}

func TestStore_RegisterWithID_ValidIDsAccepted(t *testing.T) {
	cases := []string{
		"a",
		"0",
		"phase-05",
		"phase_05",
		"phase.05",
		"a.b-c_d",
		"0abc",
		strings.Repeat("a", 64), // exactly at limit
	}
	for _, id := range cases {
		id := id
		t.Run(id, func(t *testing.T) {
			s, _ := newStore(t)
			require.NoError(t, s.RegisterWithID(tokenA, id, "cond"),
				"id %q should be accepted", id)
		})
	}
}

// -----------------------------------------------------------------------
// SC #1, #4 — same-process goroutine concurrency (Task 2.1)
// -----------------------------------------------------------------------

// TestStore_SameProcessGoroutineConcurrency validates that Store's
// same-process sync.Mutex (BEFORE flock) prevents two goroutines in the
// same process from interleaving their read-modify-write cycles.
// Without the mutex, both goroutines would share a single file-
// descriptor-level flock and stomp each other's counter increments.
func TestStore_SameProcessGoroutineConcurrency(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)

	const N = 50
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.Register(tokenA, fmt.Sprintf("cond-%d", i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}

	recs, err := s.List()
	require.NoError(t, err)
	require.Len(t, recs, N, "%d goroutines must produce %d records", N, N)

	seen := make(map[string]struct{}, N)
	for _, r := range recs {
		_, dup := seen[r.ID]
		require.Falsef(t, dup, "duplicate id %s — missing sync.Mutex on Store?", r.ID)
		seen[r.ID] = struct{}{}
	}
}
