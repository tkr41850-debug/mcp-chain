// Package store persists mcp-chain's state.json under cross-process file
// locks with atomic writes. It is the SDK-agnostic hexagonal core consumed
// by both the MCP-server adapter (Phase 5) and the CLI adapter (Phase 7).
//
// The package exposes Register/Resolve/Get/List/Purge over a versioned
// JSON file. Concurrent writers across processes are serialised via
// gofrs/flock; concurrent readers share the lock. All mutations are
// crash-safe via atomic rename (renameio/v2 on POSIX, stdlib rename on
// Windows under flock protection).
//
// Requirements: CORE-04, CORE-05, CORE-08, CORE-09.
package store

import "time"

// schemaVersion is the single supported state-file schema version. An
// on-disk state file whose top-level `version` differs produces
// ErrSchemaVersion. There is no migration path in v1 by design.
const schemaVersion = 1

// state is the on-disk shape of state.json. Fields are exported for
// encoding/json; the struct itself is unexported (package-internal).
type state struct {
	Version uint64            `json:"version"`
	Counter uint64            `json:"counter"`
	Records map[string]record `json:"records"`
}

// record is a single chain entry. ResolvedAt is a pointer so that an
// unresolved record marshals as JSON `null` rather than the zero time.
type record struct {
	ID         string     `json:"id"`
	Condition  string     `json:"condition"`
	Status     string     `json:"status"`
	OwnerToken string     `json:"owner_token"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

// Record status values. Kept as package-internal constants so callers
// cannot manufacture arbitrary status strings via the exported Record.
const (
	statusPending  = "pending"
	statusResolved = "resolved"
)
