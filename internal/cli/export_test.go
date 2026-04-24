package cli

import "io"

// RunStatus re-exports the unexported runStatus for xtests in the
// cli_test package (e.g. status_test.go). Keeping runStatus
// unexported in the production API while making it testable from
// an xtest preserves the public-API surface.
func RunStatus(out, errW io.Writer, path, id string) int {
	return runStatus(out, errW, path, id)
}
