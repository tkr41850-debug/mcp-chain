package store

import "errors"

// Sentinel errors returned by Store methods. Callers MUST match with
// errors.Is, never direct equality (err == ErrX), because method
// implementations wrap these with additional context via fmt.Errorf.

// ErrUnknownID is returned by Resolve/Get/Purge when the supplied id
// is not present in state.json.
var ErrUnknownID = errors.New("store: unknown id")

// ErrAlreadyResolved is returned by Resolve when the target record's
// status is already "resolved". Resolve is not idempotent by design —
// callers surface this to the user as a correctness signal.
var ErrAlreadyResolved = errors.New("store: already resolved")

// ErrNotOwner is returned by Resolve when the supplied ownerToken does
// not match the token stamped at Register time and ResolveOptions.Force
// is false. Token comparison is constant-time (crypto/subtle).
var ErrNotOwner = errors.New("store: not owner")

// ErrSchemaVersion is returned by load-time operations when state.json
// has a top-level "version" field different from the schemaVersion this
// binary supports. Wrapped messages name the file path and both versions.
var ErrSchemaVersion = errors.New("store: unsupported schema version")

// ErrCorruptJSON is returned when state.json cannot be decoded. Wrapped
// messages name the file path and include recovery guidance ("back up
// and remove to reset").
var ErrCorruptJSON = errors.New("store: corrupt state file")

// ErrPurgeArgRequired is returned by Purge when PurgeOptions has zero
// targets set (or more than one) — exactly one of ID / All / Resolved
// must be true.
var ErrPurgeArgRequired = errors.New("store: purge requires --id, --all, or --resolved")

// ErrIDTaken is returned by RegisterWithID when the caller-supplied id
// is already present in state.json (status irrelevant — pending and
// resolved both block re-registration).
var ErrIDTaken = errors.New("store: id already registered")

// ErrInvalidID is returned by RegisterWithID when the caller-supplied
// id fails validation: must be 1–64 chars, start with [a-z0-9], contain
// only [a-z0-9._-]. Wrapped messages name the offending id.
var ErrInvalidID = errors.New("store: invalid id")
