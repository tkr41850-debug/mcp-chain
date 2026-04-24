package cli

import "io"

// RunStatus re-exports the unexported runStatus for xtests in the
// cli_test package (e.g. status_test.go). Keeping runStatus
// unexported in the production API while making it testable from
// an xtest preserves the public-API surface.
func RunStatus(out, errW io.Writer, path, id string) int {
	return runStatus(out, errW, path, id)
}

// RunList re-exports runList for xtests (Phase 7).
func RunList(out, errW io.Writer, path string) int {
	return runList(out, errW, path)
}

// RunPurge re-exports runPurge for xtests (Phase 7).
func RunPurge(out, errW io.Writer, path, id string, all, resolvedOnly bool) int {
	return runPurge(out, errW, path, id, all, resolvedOnly)
}

// RunResolve re-exports runResolve for xtests (Phase 7).
func RunResolve(out, errW io.Writer, path, id string, force bool) int {
	return runResolve(out, errW, path, id, force)
}
