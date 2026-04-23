// Package statepath resolves the on-disk location of mcp-chain's state.json
// and ensures its parent directory exists with mode 0700.
//
// Resolution order (Linux, macOS):
//
//	$XDG_STATE_HOME is set and non-empty → $XDG_STATE_HOME/mcp-chain/state.json
//	otherwise                             → $HOME/.mcp-chain/state.json
//	$HOME also unset                      → error
//
// Per the XDG Base Directory Specification, an empty-string value for
// $XDG_STATE_HOME is treated identically to unset.
//
// Deliberately avoids os/user.Current (NSS / potential cgo) and
// os.UserHomeDir (project policy: env-only lookups on the startup path).
//
// Requirement: CORE-06. Windows support is deferred.
package statepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// stateFileName is the basename of the state file. Exported indirectly via
// Resolve() so the store layer does not hard-code it.
const stateFileName = "state.json"

// dirName is the application directory name used under both XDG and HOME roots.
const dirName = "mcp-chain"

// homeSubdir is the dot-prefixed variant used under $HOME per project spec
// (not the strict-XDG $HOME/.local/state/mcp-chain path).
const homeSubdir = ".mcp-chain"

// ErrHomeUnset is returned when both $XDG_STATE_HOME and $HOME are unset
// or empty. Use errors.Is to check.
var ErrHomeUnset = errors.New("statepath: $HOME and $XDG_STATE_HOME are both unset or empty")

// Resolve returns the absolute path to mcp-chain's state.json and ensures
// its parent directory exists with mode 0700. The state file itself is not
// created; that is the store layer's responsibility.
//
// Resolve is safe to call concurrently from multiple processes (MkdirAll is
// idempotent under races).
//
// Returns ErrHomeUnset wrapped if neither $XDG_STATE_HOME nor $HOME is set.
// Returns a wrapped mkdir error if the parent directory cannot be created.
func Resolve() (string, error) {
	var parent string
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		// XDG spec §3: "If $XDG_STATE_HOME is either not set or empty, a
		// default equal to $HOME/.local/state should be used." We honor
		// non-empty values here; empty or unset falls through to $HOME.
		parent = filepath.Join(x, dirName)
	} else {
		h := os.Getenv("HOME")
		if h == "" {
			return "", ErrHomeUnset
		}
		parent = filepath.Join(h, homeSubdir)
	}

	// MkdirAll is idempotent: no-op if parent already exists, creates all
	// intermediate segments otherwise. It does NOT chmod existing dirs,
	// so a user-created parent with 0755 is preserved as-is.
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", fmt.Errorf("statepath: create parent dir %q: %w", parent, err)
	}

	return filepath.Join(parent, stateFileName), nil
}
