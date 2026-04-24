package mcpserver

// Tool descriptions are model-facing surfaces. Every token matters
// (CLAUDE.md + CORE-10). Keep them to one short sentence each;
// aggregate budget <= 200 tokens enforced by tools_test.go. Revise
// wording only after re-running TestToolListUnderBudget.
const (
	registerDescription = "Register a coordination lock; returns a short id."
	resolveDescription  = "Resolve a lock id you previously registered."
)

// RegisterIn / RegisterOut / ResolveIn / ResolveOut are the typed
// payloads for AddTool generics. Field tags drive SDK schema
// auto-generation; `jsonschema` tag values are the param docs the
// LLM sees in tools/list.
type RegisterIn struct {
	Condition string `json:"condition" jsonschema:"when this id is considered resolvable"`
}
type RegisterOut struct {
	ID string `json:"id"`
}
type ResolveIn struct {
	ID string `json:"id" jsonschema:"the short id returned by register"`
}
type ResolveOut struct{}
