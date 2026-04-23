//go:build windows

package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeStateAtomic writes data to path atomically using a same-
// directory temp file and os.Rename. On Windows, os.Rename is backed
// by MoveFileEx with MOVEFILE_REPLACE_EXISTING (Go 1.5+), which is
// atomic on the same NTFS volume.
//
// Safety depends on the invariant that no other process holds a handle
// on the target file during rename — enforced here by our separate
// state.json.lock file (the lock is never taken on state.json itself).
func writeStateAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("store: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	// defer unconditional remove: no-op after successful rename
	// (source gone); only effective on the error paths below.
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("store: chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("store: rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}
