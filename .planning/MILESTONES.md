# Milestones

History of shipped versions of mcp-chain.

---

## v0.1.0 — Initial Release

**Shipped:** 2026-04-24
**Tag:** v0.1.0
**Phases:** 10 | **Plans:** 11 | **Status:** Complete

### Delivered

A lightweight MCP server distributed as a Claude Code plugin that lets N parallel Claude Code sessions coordinate via shared register/wait/resolve locks over a flock-protected JSON state file. Single static Go binary (≤15 MB, ≤100 ms startup, ≤20 MB RSS), pure-Go (no cgo), Linux + macOS + Windows via GoReleaser cross-compile.

### Key Accomplishments

1. **Hexagonal core** — `internal/store` exposes `Register`/`Resolve`/`Get`/`List`/`Purge` over a versioned JSON file under flock + atomic rename, with a per-process OwnerToken session-link enforced by both adapters.
2. **MCP stdio adapter** — terse `register`/`resolve` tools (token budget enforced by `tools_test.go`) with a 128-bit per-process OwnerToken and distinct wire error codes (`unknown_id`, `already_resolved`, `not_owner`).
3. **CLI subcommands** — kong-based dispatch for `serve`/`status`/`list`/`purge`/`resolve` with locked exit-code contract (resolved=0, pending=2, unknown=1) and strict stdout discipline.
4. **Plugin packaging** — Claude Code plugin manifest, slash commands (`/mcp-chain:reg`, `:wait`, `:list`, `:purge`), `chain-wait.sh` poll helper, and a self-installing wrapper that fetches the pinned binary from GitHub Releases on first invocation.
5. **CI/release pipeline** — GoReleaser cross-compile matrix (linux/darwin/windows × amd64/arm64), `-race` gate, lint gate (`golangci-lint`), binary-size + startup-time guards, and a tag-driven release workflow.
6. **Docs + dogfooding** — README at repo root, `scripts/dogfood.sh` end-to-end smoke, and a 15-step 2-session manual checklist exercised live on Linux through plugin-install → wrapper self-install → register/wait/resolve flow.

### Known Deferred Items

| Category | Item | Status |
|----------|------|--------|
| verification | Phase 08 plugin install on macOS | human_needed (deferred per D-10 — no macOS runner) |
| verification | Phase 09 first-tag GitHub Actions release | human_needed (gated on v0.1.0 push to remote) |
| verification | Phase 10 macOS dogfood walkthrough | human_needed (deferred per D-10 — no macOS runner) |
| context_oq | Phase 08 OQ-1/OQ-2/OQ-3 | resolved during execution; CONTEXT not retroactively struck through |

See `STATE.md` Deferred Items and `milestones/v0.1.0-MILESTONE-AUDIT.md` for full audit.

### Archive

- [milestones/v0.1.0-ROADMAP.md](milestones/v0.1.0-ROADMAP.md)
- [milestones/v0.1.0-REQUIREMENTS.md](milestones/v0.1.0-REQUIREMENTS.md)
- [milestones/v0.1.0-MILESTONE-AUDIT.md](milestones/v0.1.0-MILESTONE-AUDIT.md)
