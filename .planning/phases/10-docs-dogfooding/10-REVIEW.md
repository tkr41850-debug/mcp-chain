---
phase: 10-docs-dogfooding
reviewed: 2026-04-24T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - README.md
  - scripts/dogfood.sh
  - scripts/smoke-chain-wait.sh
  - .planning/REQUIREMENTS.md
  - .planning/phases/10-docs-dogfooding/DOGFOOD.md
findings:
  critical: 0
  warning: 1
  info: 6
  total: 7
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-04-24
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Phase 10 is a docs + dogfooding polish phase. Reviewed files are all
user-facing documentation or shell scripts — no Go source changes. The
artifacts are in good shape overall: `dogfood.sh` is bash-3.2-safe with
proper trap cleanup, the PATH guard on `smoke-chain-wait.sh` fires as
designed, and the REQUIREMENTS.md prose swap is clean (zero legacy
`/chain-*` command references; the only `/chain-wait` substring match
is inside the file path `plugin/scripts/chain-wait.sh`, which is the
actual current filename).

One warning: a minor but real inaccuracy in the README's CLI subcommand
table claims `mcp-chain purge` exits `1 if no target given`, when the
actual implementation also returns exit 1 on unknown-id and any store
error. A user scripting against the CLI could be misled.

Six info-level items follow — small wording drifts between
documentation and actual binary behavior (error-message text in
DOGFOOD.md step 11 doesn't match the code; README Troubleshooting
references a repo-relative path that won't exist in a plugin-install
context; etc.), plus two minor hygiene notes on dogfood.sh.

No critical findings. No security issues. No blockers for v0.1.0 tag
push.

## Warnings

### WR-01: README purge exit-code description under-specifies error paths

**File:** `README.md:91`
**Issue:** The CLI subcommands table entry for `purge` reads:

```
0 on success, 1 if no target given
```

The actual implementation (`internal/cli/purge.go:56-85`) returns exit
1 on multiple failure modes, not just "no target given":
- bare `purge` with no args (store.ErrPurgeArgRequired) → 1
- unknown `<id>` (store.ErrUnknownID) → 1
- any store.Open / underlying I/O error → 1

A user writing a script like `mcp-chain purge "$id" || echo "id not
given"` will mishandle the unknown-id case. The description should
read something like "`0` on success, `1` on error (bare invocation,
unknown id, or I/O failure)" — aligned with the `status` row's style.

**Fix:**

```markdown
| `mcp-chain purge <id> \| --all \| --resolved` | Delete entries; bare `purge` errors | `0` on success, `1` on error (bare invocation, unknown id, or I/O failure) |
```

## Info

### IN-01: DOGFOOD.md step 11 error-text quote does not match actual message

**File:** `.planning/phases/10-docs-dogfooding/DOGFOOD.md:17`
**Issue:** Step 11's "Done means" column reads:

```
Errors with "requires --all, --resolved, or <id>" (CLI writes to stderr)
```

The actual stderr message emitted by `internal/cli/purge.go:76` is:

```
mcp-chain: purge requires <id>, --all, or --resolved
```

The quoted argument order differs (docs: `--all, --resolved, or <id>`;
code: `<id>, --all, or --resolved`). A meticulous tester using the
quoted text as a grep string will not find a match. Semantic intent
(bare `purge` must error to stderr with actionable arg list) is clearly
met — this is wording drift only.

**Fix:** Update DOGFOOD.md step 11 to quote the actual message:

```markdown
| 11 | `/mcp-chain:purge` with no args | Errors with `mcp-chain: purge requires <id>, --all, or --resolved` on stderr | ☐ | ☐ |
```

### IN-02: README Troubleshooting cites repo-relative path a plugin-install user won't have

**File:** `README.md:167-168`
**Issue:** The "Monitor script hangs" entry suggests:

```
Run `bash -n plugin/scripts/chain-wait.sh` to syntax-check.
```

For a user installed via `/plugin install anthropics/mcp-chain` (the
recommended path in §Install), the script lives at
`${CLAUDE_PLUGIN_ROOT}/plugin/scripts/chain-wait.sh`, not under their
cwd. Running the suggested command from an arbitrary shell will fail
with "No such file or directory". Users who cloned the repo for
development are fine.

**Fix:** Qualify the path or drop the bare command:

```markdown
Run `bash -n <plugin-root>/plugin/scripts/chain-wait.sh` (the plugin
root is printed by `/mcp` in Claude Code) to syntax-check.
```

or leave it as "syntax-check the monitor script in the plugin root."

### IN-03: dogfood.sh uses `go build` path resolution that depends on module-root cwd

**File:** `scripts/dogfood.sh:25`
**Issue:** `go build -o "$BIN" "$ROOT/cmd/mcp-chain"` is invoked without
first `cd`-ing into `$ROOT`. Go module builds require either a path
relative to the module root OR running from inside the module (so
`go.mod` is found). In practice, passing an absolute path like
`/home/alpine/mcp-chain/cmd/mcp-chain` to `go build` works because the
Go toolchain walks up from the target to find `go.mod`. But if a
future reader assumes `$ROOT` can be relocated, or if the script is
copied into another repo context, this would silently fail.

Not a bug today — `go build` does find the module root via the absolute
import path — but a one-line `cd "$ROOT"` before `go build` would make
the intent explicit and match `smoke-chain-wait.sh`'s convention (which
also passes `$SMOKE_ROOT/cmd/mcp-chain` directly without cd, consistent
with existing behavior). Noting for future-proofing only.

**Fix:** Optional — prefix with `cd "$ROOT"` for clarity, or leave as-is
(consistent with smoke-chain-wait.sh).

### IN-04: dogfood.sh step 5 `grep -cE` relies on list-row prefix being lowercase

**File:** `scripts/dogfood.sh:63`
**Issue:** The "list empty after purge" check is:

```bash
DATA_LINES="$(echo "$OUT" | grep -cE '^[a-z]+[[:space:]]+' || true)"
```

This depends on two implementation details of `list`:
1. Header row starts with `ID` (uppercase), so it's correctly excluded.
2. Data rows start with a lowercase EFF word (e.g., `otter  pending...`).

Both hold today (verified in `internal/cli/format/table.go:45`). But if
the ID format ever changes (e.g., the hex fallback `hex-0001` starts
with `h` — still lowercase, still matches — so that's fine), or if the
header row is ever lowercased, the check silently breaks in either
direction. A more robust check would be "list stdout is empty" (since
`list.go:55-58` emits zero stdout bytes on empty state, writing "no
entries" to stderr instead).

**Fix:** Optional — replace with:

```bash
[ -z "$OUT" ] || { echo "dogfood: step 5 FAIL list nonempty after purge: $OUT" >&2; exit 1; }
```

This matches the actual contract (stdout is empty on empty store) and
removes the regex coupling to row format. Current check works; this is
hardening only.

### IN-05: dogfood.sh leaves seed_pending's $WORK-relative lifecycle undocumented

**File:** `scripts/dogfood.sh:36`
**Issue:** `go run "$ROOT/scripts/internal/seed_pending.go"` inherits
the parent's `XDG_STATE_HOME`, which is `$WORK/state`. The seed binary
writes state there, and the subsequent `$BIN status` reads from the
same location. This works but is load-bearing on environment
inheritance through `go run`'s temp build — a subtle coupling.

A top-of-file comment block already mentions "bash 3.2+, go in PATH,
mktemp" but doesn't explicitly name the XDG_STATE_HOME inheritance
contract. Future maintainer changing the seed helper to read its path
from argv would break this silently.

**Fix:** Optional — add a one-line comment at line 18:

```bash
# seed_pending.go inherits XDG_STATE_HOME via `go run` env passthrough;
# keep the export above the first `go run` invocation.
export XDG_STATE_HOME="$WORK/state"
```

### IN-06: README "polls once per second" is narrative — no programmatic way to change it

**File:** `README.md:63`
**Issue:** Usage demo caption reads `(polls once per second, silent
until resolve)`. This is accurate (`chain-wait.sh:137` is
`sleep 1`, no flag to change it). Not a bug — just worth noting that
a future reader asking "can I change the poll interval?" will not find
an answer in README. The Troubleshooting section could absorb a
one-line "poll interval is fixed at 1s; edit `chain-wait.sh` if you
need a different cadence" if space permits.

**Fix:** Optional. No code change needed.

---

_Reviewed: 2026-04-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
