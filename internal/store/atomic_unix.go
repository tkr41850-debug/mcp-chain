//go:build !windows

package store

import (
	"fmt"

	"github.com/google/renameio/v2"
)

// writeStateAtomic writes data to path atomically with mode 0600
// (CORE-05). renameio writes to a sibling temp file, fsyncs, renames
// over the target, and fsyncs the parent directory — producing an
// either-old-or-new guarantee under crash.
//
// IgnoreUmask ensures the final file mode is exactly 0600 regardless
// of the process umask. renameio/v2 applies umask by default (v1 did
// not); without this option, an aggressive umask like 077 is a no-op
// but a permissive umask could leave extra bits. Be defensive.
func writeStateAtomic(path string, data []byte) error {
	if err := renameio.WriteFile(path, data, 0o600, renameio.IgnoreUmask()); err != nil {
		return fmt.Errorf("store: atomic write %s: %w", path, err)
	}
	return nil
}
