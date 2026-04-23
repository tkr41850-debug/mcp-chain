# Pitfalls Research

**Domain:** Go MCP server + Claude Code plugin + shared-file coordination
**Researched:** 2026-04-23
**Confidence:** HIGH (most pitfalls drawn from official docs, issue trackers, and well-known community post-mortems; specific threshold numbers MEDIUM)

## Critical Pitfalls

### Pitfall 1: Corrupting the MCP stdio wire with stdout output

**What goes wrong:**
Any byte the process writes to stdout that is not a newline-delimited JSON-RPC message corrupts the protocol. The client (Claude Code) reports a parse error and disconnects. Symptoms are maddening: the server starts, appears to connect, then immediately drops; or it works until a specific tool call runs a code path that emits a stray `fmt.Println` or a log line from a transitive dependency.

**Why it happens:**
- Developers reflexively use `log.Println`, `fmt.Println`, `print()` for debugging — all go to stdout by default in Go.
- `log.Default()` writes to `os.Stderr` but many loggers (including some popular structured loggers) default to stdout.
- Transitive dependencies may print warnings/banners to stdout (e.g., on init).
- Cobra's `--help`, `--version`, and error output all go to stdout by default.
- Panics in a goroutine print to stderr (safe), but a `recover` handler that prints the error via `fmt.Println` is not safe.

**How to avoid:**
- `mark3labs/mcp-go`'s `NewStdioServer` default error logger discards output — do not "helpfully" redirect it to stdout.
- Pick a single logging sink early and hard-code it to `os.Stderr`: `log.SetOutput(os.Stderr)` in `main()` before anything else runs.
- Forbid `fmt.Print*` and `log.Print*` in the `serve` command path via a lint rule (e.g., `forbidigo` or a custom `go vet` check).
- Audit every dependency by running `go build ./...` and then `./mcp-chain serve </dev/null 2>/tmp/stderr >/tmp/stdout`; after killing it, `/tmp/stdout` must be empty (or contain only valid JSON-RPC frames if you piped an init handshake).
- In Cobra, explicitly set `cmd.SetOut(os.Stderr)` on the `serve` subcommand so help/error output goes to stderr.
- Integration test: launch the binary, send a malformed request, assert stdout contains *only* JSON-RPC; any non-JSON byte fails the test.

**Warning signs:**
- Claude Code logs show `JSON parse error` or "unexpected token" near the start of a session.
- Server process stays alive but tools don't appear.
- Running the binary manually prints a banner/version/help line to the terminal before waiting for input.

**Phase to address:**
Phase 1 (Core MCP server scaffolding) — lock down stderr-only logging before any tool handler is added. Verified in Phase QA by an integration test.

---

### Pitfall 2: Missing or malformed MCP initialization handshake

**What goes wrong:**
MCP is JSON-RPC 2.0 with a required three-step handshake: client sends `initialize`, server responds with capabilities + `protocolVersion`, client sends `initialized` notification. If the server responds with the wrong `protocolVersion`, omits required capability fields, or tries to send a `notifications/tools/list_changed` before `initialized`, the client treats the session as broken. Newline framing is strict: each message is exactly one JSON object, terminated by a single `\n`, with no embedded newlines.

**Why it happens:**
- Hand-rolling the protocol instead of using a battle-tested SDK.
- Pretty-printing JSON response bodies (introduces embedded newlines inside a message frame).
- Negotiating a protocol version the client doesn't support (spec bumped versions; using the wrong literal breaks).
- Sending server-initiated notifications before the handshake completes.

**How to avoid:**
- Use `github.com/mark3labs/mcp-go` or `github.com/modelcontextprotocol/go-sdk` — both handle framing and the handshake. Do not hand-roll.
- If encoding JSON manually anywhere, use `json.Marshal` (compact, no indent); never `json.MarshalIndent`.
- Treat the `protocolVersion` field as data from the SDK — do not hard-code.
- Integration test: pipe a recorded `initialize` request, assert a valid `initialize` response, then pipe `notifications/initialized`, then a `tools/list` — full handshake before any tool call.

**Warning signs:**
- Client disconnects immediately after launching.
- Tools never appear in `/mcp` list.
- Stderr shows "unexpected notification before initialized" or similar.

**Phase to address:**
Phase 1 (Core MCP server) — lock in by depending on a mature SDK rather than hand-rolling.

---

### Pitfall 3: Non-compact JSON on the wire (embedded newlines)

**What goes wrong:**
MCP's stdio framing rule: *messages MUST NOT contain embedded newlines*. Using `json.MarshalIndent`, echoing a multiline `error.Error()` into a tool result `text` field, or embedding a literal `\n` in a registered tool's description string breaks framing. The client reads the first line as a truncated JSON object, parse-errors, and drops the connection.

**Why it happens:**
- Developers reach for `json.MarshalIndent` when logging/debugging, then forget to swap it back.
- Tool results with multi-line text (e.g., a stack trace, a file listing) get naively stringified — JSON encoding escapes `\n` to `\\n` inside a string value, which is safe; but pre-encoded raw JSON blobs may not be re-escaped.
- YAML/TOML tool description loaders that preserve literal newlines.

**How to avoid:**
- Always `json.Marshal` (compact); ban `MarshalIndent` anywhere in the `serve` path.
- Keep tool descriptions on a single line. If the description needs structure, use short sentences separated by periods, not newlines.
- Pre-commit/test: grep tool registrations for `\n` in the `Description` field; fail the build if found.
- The SDK usually handles this — another reason not to hand-roll.

**Warning signs:**
- Tools appear in Claude Code but tool calls return parse errors.
- Client logs: "unexpected end of JSON input" or similar mid-message.

**Phase to address:**
Phase 1 (Core MCP server) — enforce by test + build check. Also revisit in Phase 2 (tool description authoring) when writing the registration/resolve/status descriptions.

---

### Pitfall 4: Verbose MCP tool descriptions burn tokens on every turn

**What goes wrong:**
MCP tool schemas — names, descriptions, JSON-schema of params, and return descriptions — are injected into Claude's context on every turn where the tool is available. A 500-token description, used in a 50-turn session, costs 25,000 tokens the user never sees value from. For a project whose stated principle is terse model-facing text, this directly violates the constraint.

**Why it happens:**
- Developers write docstrings for humans, not for models.
- Temptation to include "usage examples," "when to use vs. not use," and error-handling guidance in the description — all of which belong in prompt-engineering elsewhere, not per-turn.
- JSON-schema `description` fields on every parameter compound the cost.
- "Defense in depth" instinct: "If I explain more, the model will invoke it correctly."

**How to avoid:**
- Budget: ≤ 1 short sentence per tool description. ≤ 1 short phrase per parameter description. No examples in descriptions.
- Follow the "lead with core functionality" rule: first 8 words state what the tool does.
- Measure: after registering all tools, dump the registered tool list and count tokens (use `tiktoken` or the cheap approximation `len(text)/4`). Budget per tool: < 40 tokens. Budget for the whole server's tool list: < 200 tokens.
- Validate with a probe session: ask Claude to invoke each tool from natural-language prompts; if it fails, tighten *wording* before adding words.
- Too-terse failure mode (model can't invoke reliably): almost always fixed by renaming the tool or a parameter, not by expanding prose.

**Warning signs:**
- Tool description > 100 characters.
- JSON-schema of a tool has > 3 keys with long `description` fields.
- Users report "it keeps calling the wrong tool" — fix by renaming, not prose.

**Phase to address:**
Phase 2 (tool definitions / registration API) — write descriptions with a token budget gate. Re-evaluate in Phase QA.

---

### Pitfall 5: Shipping an MCP server that expects `npm`/`uvx` when distributing a Go binary

**What goes wrong:**
The `.mcp.json` fragment many examples show uses `"command": "npx"` or `"command": "uvx"`. If you copy-paste that into a Go-distribution plugin, install silently fails for users without Node or Python. Worse, if you hard-code `"command": "/Users/me/.local/bin/mcp-chain"`, it works only on your machine.

**Why it happens:**
- Most MCP examples online are Node.js or Python.
- Developers test on their own machine with a binary at a personal path.
- Forgetting that plugin installations land in different locations across OS and install method (marketplace, local, enterprise).

**How to avoid:**
- In the plugin's `.mcp.json`, set `"command": "${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain"` (or OS-specific subdirectory) — Claude Code substitutes this variable to the plugin install dir at runtime.
- Ship per-OS binaries under `bin/linux-amd64/mcp-chain`, `bin/darwin-arm64/mcp-chain`, `bin/windows-amd64/mcp-chain.exe` etc., with a small shim or a post-install step that symlinks/copies the right one to `bin/mcp-chain`.
- Do *not* require the user to `go install` or put the binary on `$PATH`.
- Test install flow end-to-end on at least Linux + macOS before release.

**Warning signs:**
- README instructs the user to run `brew install` or `go install` as a prereq — UX smell.
- `.mcp.json` contains any absolute path.
- Install fails silently on a machine without Node/Python.

**Phase to address:**
Phase 4 (Plugin packaging and distribution) — this is the primary gate before tagging a release.

---

### Pitfall 6: Lost-update race — read/modify/write outside the flock critical section

**What goes wrong:**
Two Claude Code sessions call `register` simultaneously. Process A acquires the flock, reads `state.json`, releases the lock, modifies the in-memory copy, re-acquires the lock, writes. Between A's first release and re-acquire, process B does the same sequence. Result: A's write overwrites B's write (or vice versa). Two sessions get the same word-ID, or one registration is silently dropped.

**Why it happens:**
- Developers acquire the lock only for the I/O step ("locks are for I/O"), not for the logical operation.
- Helper functions split `read` and `write` into separate calls, each taking the lock independently.
- `defer unlock()` inside a tight helper looks correct but only covers that helper's scope.

**How to avoid:**
- Rule: **lock covers the entire logical operation — acquire, read, mutate, write, unlock, in that order, in one function.**
- Never expose a "load state" helper that acquires the lock internally; instead have one `withLockedState(fn func(*State) error) error` that takes an exclusive lock, reads the JSON, passes the parsed state to `fn`, writes back if `fn` returns nil, and releases.
- Use `github.com/gofrs/flock` with `Lock()` (exclusive) for all mutations; never `RLock()` during mutation paths (and on some UNIXes, shared locks can be transparently upgraded then accidentally released — prefer exclusive for simplicity).
- Test: spawn 50 goroutines (and separately, 50 subprocess instances) that all register concurrently; assert 50 unique word-IDs in the final state.

**Warning signs:**
- Test with 2 concurrent processes occasionally produces duplicate IDs.
- A "re-read after lock" pattern appears anywhere in the codebase.
- Wordlist allocation helper doesn't take a state pointer — it probably loads state internally.

**Phase to address:**
Phase 3 (state file + flock layer) — design the `withLockedState` API up front; enforce via code review checklist.

---

### Pitfall 7: Atomic-rename without directory fsync — zero-length state.json after crash

**What goes wrong:**
The standard atomic-write recipe is `write tmp → fsync(tmp) → rename(tmp, target)`. But POSIX `rename` is only atomic under *normal* operation — on crash, the directory entry change may not be durable. After power loss or kernel panic, the user may find `state.json` is zero bytes, or the old pre-rename name reappears. On Windows, `os.Rename` is not atomic at all if the destination already exists (and even the non-destructive variant has edge cases).

**Why it happens:**
- Developers believe "rename is atomic" means "rename is durable." It isn't.
- Skipping the `fsync` of the *parent directory* after rename; it's necessary to persist the directory entry.
- Skipping `fsync` of the tmp file before rename; the rename may succeed but the file content is still in the page cache.
- Rolling your own atomic write rather than using `github.com/google/renameio` or `github.com/natefinch/atomic`.

**How to avoid:**
- Use `github.com/google/renameio/v2` (pure Go, handles fsync of tmp and dir on Linux; falls back to best-effort on Windows). Accept the Windows limitation as documented.
- If rolling your own: write tmp in the *same directory* as target (rename-atomic only works within one filesystem), `fsync(tmpfd)`, `os.Rename(tmp, target)`, open parent dir, `fsync(dirfd)`.
- Test crash-safety: use `fsync` tracing (or just trust renameio) — don't try to exhaustively verify by kill -9 loops.
- Accept that on Windows, atomic-rename isn't strictly atomic; mitigate by keeping a `state.json.bak` as last-known-good and validating JSON on load, falling back to `.bak` on parse failure.

**Warning signs:**
- State file is zero bytes after an unclean shutdown.
- `os.WriteFile(path, ...)` anywhere in the write path — this is non-atomic by definition.
- No `fsync` call appears in the write path.

**Phase to address:**
Phase 3 (state persistence layer) — pick `renameio` up front; add a "simulate crash" test (kill the process mid-write with SIGKILL, then verify on restart the state is either the prior good state or the new good state, never corrupt).

---

### Pitfall 8: flock on NFS silently does nothing

**What goes wrong:**
User runs `mcp-chain` across two terminals where `$HOME` is an NFS mount (corporate dev environment, network-attached storage). `flock(2)` on Linux over NFSv3 depends on the NFS server version and mount options — historically it did nothing at all (locks were held only locally on the client), meaning the mutual-exclusion guarantee is silently absent. State corruption is rare but possible.

**Why it happens:**
- `flock()` does not work reliably over NFS; `fcntl()` byte-range locks are the NFS-safe choice but have their own gotchas.
- Developers test on local disk and never hit the failure mode.
- `gofrs/flock` uses `flock(2)` on Linux — not NFS-safe.

**How to avoid:**
- Document the constraint: *state file must be on a local filesystem.* The default (`~/.mcp-chain/state.json` or `$XDG_STATE_HOME/mcp-chain/state.json`) is usually local.
- At startup, `statfs` the state file's filesystem; if it's NFS/CIFS/SMB, print a stderr warning (not an error — some NFS setups work).
- In README: "do not put the state file on NFS or SMB."
- Do not attempt to support NFS robustly — this project explicitly rejects networked coordination.

**Warning signs:**
- User reports "I got two sessions with the same ID" on a machine with network home dirs.
- Filesystem of state file (from `/proc/mounts` or `statfs`) is one of: nfs, cifs, smbfs.

**Phase to address:**
Phase 3 (state persistence) — add the startup warning. Phase 6 (docs) — mention in README.

---

### Pitfall 9: Windows has no flock — `gofrs/flock` uses `LockFileEx`, but semantics differ

**What goes wrong:**
On Windows, `gofrs/flock` uses `LockFileEx`, which is a mandatory lock (kernel-enforced) rather than POSIX advisory. It also locks byte ranges, not whole files. If the Go process crashes while holding a lock, Windows releases it — so far so good. But the library's behavior around `TryLock` vs `Lock`, blocking semantics, and interaction with `os.Rename` (a file held open cannot be replaced on Windows) differs from Unix in ways that break the atomic-rename pattern.

**Why it happens:**
- Developer tests only on macOS/Linux and cross-compiles for Windows "because Go makes it trivial."
- The atomic-write pattern on Windows: you cannot rename over a file that another process has open, so the moment a reader holds the file open, the writer's rename fails.
- `LockFileEx` locks byte ranges on the *file being locked*. If the reader holds the state file open, the writer can't rename into that path — even with the lock.

**How to avoid:**
- Use a separate lock file (`state.json.lock`), never the state file itself. Lock the lock file; operate on the state file only while holding the lock. This decouples locking from the file being atomically replaced.
- `renameio` handles Windows-specific quirks internally — use it, don't roll your own.
- CI matrix must include `windows-amd64` for tests (not just build). Run the full flock integration test on Windows.
- Document Windows as "supported via CI cross-compile" but de-prioritize deep testing if it's not a primary platform.

**Warning signs:**
- Concurrent register/resolve test that passes on Linux/macOS fails intermittently on Windows.
- Errors like "The process cannot access the file because it is being used by another process."

**Phase to address:**
Phase 3 (locking) — adopt separate-lock-file pattern. Phase 5 (CI) — enable Windows in test matrix, even if with reduced coverage.

---

### Pitfall 10: Wordlist reuse race — next-word allocation outside critical section

**What goes wrong:**
The EFF short wordlist has 1296 entries. "Allocate next unused word" naively reads the state, finds used words, picks an unused one, returns it. If this runs outside the flock critical section, two registrations happening within microseconds of each other both see the same "used set" and both pick the same word. Hash collisions on short IDs are hard to recover from because the collision means two sessions coordinate on the *same* channel unintentionally.

**Why it happens:**
- Developers write a `nextWord(state) string` pure function, then in registration: `state = load(); word = nextWord(state); state.Add(word); save(state)` — without holding a lock across the read-allocate-write.
- Or worse: the allocator reads state independently, not from the passed-in snapshot.

**How to avoid:**
- `nextWord` must be a pure function of a *given state snapshot* — the caller is responsible for holding the lock across read→allocate→write.
- Enforce via API shape: `withLockedState(func(s *State) error { word := s.AllocateNextWord(); s.Register(word, ...); return nil })`.
- Test: 50 concurrent goroutines + 50 concurrent subprocesses both register; assert 100 unique words in final state.
- Stretch: deterministic allocation order (lowest-index unused) so tests can assert exact sequences.

**Warning signs:**
- Allocator function reads from `os.Open` or calls `Load()` internally.
- "Generate random word and retry on collision" pattern — slower and still racy if you re-check outside the lock.

**Phase to address:**
Phase 3 (wordlist + state layer — same phase as flock critical section design).

---

### Pitfall 11: Go test parallelism + shared state file = flaky tests

**What goes wrong:**
Go runs tests within a package in parallel when `t.Parallel()` is called, and runs different packages in parallel by default. If two parallel tests both touch `~/.mcp-chain/state.json` (or any fixed path), they race. The race detector typically doesn't catch inter-process races — it only flags in-process memory races — so the tests may pass on CI and fail locally, or vice versa.

**Why it happens:**
- Tests default to "easy" paths like `os.TempDir() + "/mcp-chain-test.json"` — shared across tests.
- `t.Parallel()` is sprinkled without tracking file-level dependencies.
- Integration tests spawn subprocesses and forget to pass a unique state file path.

**How to avoid:**
- Every test gets its own state file via `t.TempDir()` (auto-cleaned). Never a fixed path.
- The state-file path must be configurable via env var (e.g., `MCP_CHAIN_STATE`) for tests. Code should already do this anyway for XDG compliance.
- When spawning subprocesses from tests, pass `MCP_CHAIN_STATE=<t.TempDir()>/state.json` explicitly.
- Run `go test -race ./...` in CI as a blocking step (already in CORE-03 plans).
- The race detector will not catch flock-level races across processes — rely on integration tests that spawn real subprocesses with counts >> 1.

**Warning signs:**
- A test that passes in isolation fails when run with `-parallel 8`.
- Errors about "file not found" or unexpected state values in integration tests.
- Tests leave files in `/tmp` after completion.

**Phase to address:**
Phase QA (testing) — set up test scaffolding that always uses `t.TempDir()`; add a lint/review rule forbidding hard-coded paths in tests.

---

### Pitfall 12: Binary bloat from transitive `net/http` + `crypto/tls` pulls

**What goes wrong:**
The 15 MB binary budget is tight. Pulling `net/http` pulls `crypto/tls` pulls all the cipher suites, X.509, and the Mozilla CA bundle (if `crypto/x509` is forced to embed — it isn't by default, but some libs import `golang.org/x/crypto/...` that balloons further). A simple test: `go build` a `hello world`: ~2MB. Add `net/http`: jumps to 7-10 MB. Add a logger like `zap`: another 1-2 MB. Plus reflection-heavy JSON libs (`mapstructure`, some config libs): more bloat. Easy to blow past 15 MB.

**Why it happens:**
- MCP SDK might pull `net/http` even for stdio-only transport (to support the streamable-HTTP transport too).
- Cobra pulls `pflag` but is otherwise light; still ~1-2 MB.
- Third-party logging libraries tend to be heavy.
- UPX can compress but hurts startup time and triggers antivirus false positives.

**How to avoid:**
- Benchmark early and often: CI should report binary size per arch; fail the build if any exceeds 15 MB.
- Use `go tool nm -size $(go env GOCACHE)/... ` and `go-size-analyzer` to see what's dominating.
- Build with `-trimpath -ldflags="-s -w"` — saves 25-30% off the top.
- Avoid structured logging libs; stdlib `log` + stderr is sufficient.
- If the chosen MCP SDK has a build tag to strip HTTP transport, use it.
- Prefer stdlib `encoding/json` over `jsoniter`/`easyjson` unless profiling demands it.
- Avoid UPX — it breaks macOS Gatekeeper signatures and regresses startup; the 15 MB budget should be achievable without it.

**Warning signs:**
- `go build` produces > 10 MB for a hello-world-sized main package.
- `go-size-analyzer` shows > 30% of the binary in a single non-core dep.

**Phase to address:**
Phase 1 (core scaffolding) — set up a CI size check immediately. Reject PRs that grow the binary beyond budget.

---

### Pitfall 13: `init()` functions kill the 100ms startup budget

**What goes wrong:**
Go runs all `init()` functions across the entire dependency graph sequentially, before `main` runs. A heavy init — parsing a large embedded config, lazy-loading CA roots, pre-compiling regexes, constructing a logger with a file handle — easily adds 50-100ms. Cumulative across dependencies, this can blow the 100ms startup budget on cold-cache first invocation.

**Why it happens:**
- Developers use `init()` as a "construction" shortcut for globals.
- Embedded assets (1296-word EFF list) being pre-parsed into a map/slice at init.
- Dependencies do work at init and you can't control it.
- `crypto/tls`'s lazy root pool loading does a filesystem scan on first use (not init, but often at startup).
- On glibc, `os/user.Lookup*` uses NSS which may hit the network — never call this at startup.

**How to avoid:**
- No `init()` functions in this codebase except trivially constant registrations. If a dep has heavy init, consider alternatives.
- Lazy-load the wordlist: the embedded byte slice is cheap; parse it only on first `register` call (but cache after).
- Profile: `time ./mcp-chain serve < /dev/null` on cold cache. Target < 100ms. Build `./mcp-chain -cpuprofile=startup.prof` and inspect.
- Avoid `os/user.Current()` at startup (hits NSS on glibc); precompute from `$HOME`/`$USER` env vars if possible.
- Avoid `time.LoadLocation` at startup; use UTC by default.
- Don't load `crypto/x509.SystemCertPool()` eagerly — network-free tools shouldn't need it.

**Warning signs:**
- `time ./mcp-chain serve` > 100ms on a cold page cache.
- `go tool trace` shows significant pre-main time.
- Dependency graph includes libs that log "loaded X config" during init.

**Phase to address:**
Phase 1 (core scaffolding) — add a startup-time benchmark to CI from the start. Re-check whenever a new dep is added.

---

### Pitfall 14: GitHub Actions release misses an arch, ships without checksums, or bakes in dirty version strings

**What goes wrong:**
Tagging a release and missing a critical combo (e.g., `darwin/arm64`, which is Apple Silicon). Users on M-series Macs silently can't install. Or: release assets lack `checksums.txt`, so users can't verify integrity. Or: `-ldflags="-X main.Version=$GITHUB_SHA"` injects a commit SHA that doesn't match the tag because the tag was created on a stale ref. Or: `GOFLAGS` leaks a `-mod=vendor` from the CI environment and breaks reproducibility.

**Why it happens:**
- Hand-rolling a release workflow instead of using `goreleaser`.
- Matrix build that silently skips a combo due to a typo in the matrix key.
- Version string pulled from `$GITHUB_REF` without stripping `refs/tags/` prefix.
- Forgetting to upload per-file checksums.

**How to avoid:**
- Use `goreleaser` — it generates the matrix, checksums, changelog, and uploads in one step. Widely battle-tested.
- Use its reproducible-build recipe: `CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.CommitDate}}"`, `mod_timestamp: '{{ .CommitTimestamp }}'`.
- Explicitly list the matrix: `linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64`.
- Enable `checksums:` block in `.goreleaser.yaml` — yields SHA256SUMS file.
- Run `goreleaser release --snapshot --clean` locally before tagging to catch config errors.
- Test the download+install flow on at least Linux and macOS after tagging.

**Warning signs:**
- GitHub release page missing an expected arch.
- `mcp-chain --version` prints the branch name or a short SHA instead of a version tag.
- No `checksums.txt` in release assets.
- Binary embeds a dirty/stale SHA.

**Phase to address:**
Phase 5 (CI + release automation) — adopt goreleaser from day one rather than retrofit.

---

### Pitfall 15: Double-resolve and unknown-ID error paths not distinct

**What goes wrong:**
CORE-03 requires "double-resolve returns a clear error." Easy to conflate "ID never existed" and "ID exists but already resolved" into a single "unknown ID" error. Users get confused: did my ID get purged? Did I typo? Was it already resolved by a stale monitor? These are different bugs with different fixes.

**Why it happens:**
- Error types collapse during refactoring for "simplicity."
- Single string error message "id not found" used for both cases.
- MCP tool return values don't distinguish; client sees one opaque error.

**How to avoid:**
- Distinct error values: `ErrUnknownID`, `ErrAlreadyResolved`. Both are represented in the tool result with a distinct `error_code` field (`"unknown_id"` vs `"already_resolved"`).
- Descriptions short but specific: `"already resolved"` vs `"unknown id"`.
- Unit tests assert the exact error code returned in each case, not just that an error occurred.

**Warning signs:**
- Single generic error path for all failure cases.
- String matching on error messages in tests.

**Phase to address:**
Phase 2 (tool handlers) — lock in distinct error codes as part of the tool-result contract.

---

### Pitfall 16: No "registering session" identity → anyone can resolve anyone's ID

**What goes wrong:**
PROJECT.md flags this as an open research question: can the MCP server reliably distinguish "the session that registered an ID" from some other session? If not, any session can resolve any ID. For a single-user local tool this is not a security risk, but it can cause correctness bugs: a session running `/chain-reg` and `/chain-wait` in the same conversation could accidentally self-resolve if the model misreads the flow.

**Why it happens:**
- MCP stdio transport spawns one server process per client connection; the server sees one stdin stream per connection with no inherent cross-connection identity.
- MCP's protocol does include session IDs during initialization, but the reliability for "is this the same session that called register earlier" depends on transport and implementation.
- For stdio, *each server process is already the session* — but this project uses a single daemon-ish server per connection, so if the user's model is one server-instance-per-Claude-Code-session, identity may be per-process and not shareable across the shared state file.

**How to avoid:**
- Treat this as a known-limitation of the MVP: document that "any session can resolve any ID." PROJECT.md already notes the user is OK with this default.
- Do an explicit investigation in Phase 0/1: instrument the server, log a synthetic connection ID per MCP connection, and observe whether Claude Code reuses connections across tool calls in one session (it does — one process, one stream) versus across sessions (separate processes, separate streams).
- If per-connection identity is desired, store a connection-scoped UUID in state at register time and require it for resolve — but this requires the *client* (Claude Code) to remember and pass it, which MCP doesn't facilitate natively. Realistically: out of scope for MVP.
- Do NOT attempt to authenticate via environment variables or PID — both are spoofable and brittle.

**Warning signs:**
- Design discussions of "auth token" or "session PID" — red flag for over-engineering.
- Assumption that MCP provides cross-process identity — it doesn't, for stdio.

**Phase to address:**
Phase 1 (research spike) — resolve the open question. Phase 2 (resolve tool) — implement either with or without identity based on the spike's findings.

---

### Pitfall 17: Slash command prompts that emit multi-paragraph instructions

**What goes wrong:**
Slash command files (`/chain-reg`, `/chain-wait`, etc.) are markdown/prompt templates that get injected into Claude's context. A 200-token slash command, invoked 20 times per day, costs 4000 tokens/day per user *for command invocation alone* — before any tool use. Written like human-facing docs, they blow the token budget that PROJECT.md explicitly calls out.

**Why it happens:**
- Developers write slash commands like README excerpts: background, options, examples, caveats.
- The slash command file looks like a Markdown tutorial rather than a terse instruction to the model.
- Copy-pasted from other plugins whose authors weren't token-budget-conscious.

**How to avoid:**
- Budget: ≤ 40 tokens per slash command prompt. One sentence that tells Claude what to do and which tool to call.
- Format: `Call chain_register with the user-supplied condition. Print the returned word-ID.` — not a paragraph, not a list of edge cases.
- Use `$ARGUMENTS` / `$1` substitution directly — don't explain the substitution in prose.
- Put detailed docs in the README, not in the slash command prompt. Claude doesn't need motivation; it needs instruction.
- Measure: run `wc -w` on each slash command file; aim for < 30 words.

**Warning signs:**
- Slash command file > 10 lines.
- Slash command contains "note:", "for example:", or "if the user wants X, do Y; if they want Z, do W" (branching logic in prose — better done in the tool itself).

**Phase to address:**
Phase 4 (slash commands + plugin packaging) — enforce token budget per slash command; review as part of merge criteria.

---

### Pitfall 18: Plugin doesn't auto-restart its MCP server after upgrade

**What goes wrong:**
User installs v1.0, then upgrades to v1.1. The plugin directory has a new binary, but Claude Code may still have the v1.0 process alive from the previous session. User sees stale tool descriptions, or worse, tool calls that hit a binary that no longer matches the state-file schema.

**Why it happens:**
- Claude Code starts the MCP server process when the plugin loads; doesn't auto-restart on upgrade.
- State-file schema changes between versions without a migration or version check.
- Plugin doesn't bump a "reload required" flag.

**How to avoid:**
- Version the state file with a `schema_version` field. On startup, the server refuses to load state with a newer schema_version than it knows (tell the user to upgrade their binary) or migrates older versions forward.
- In README install instructions, after upgrade: `/mcp` list, then restart Claude Code. Make this explicit.
- Consider a `mcp-chain --check` subcommand that verifies binary version vs state schema version before serving.
- Avoid schema changes until absolutely necessary; for a single-user tool with local state, user can `/chain-purge --all` as an escape hatch.

**Warning signs:**
- No `schema_version` field in state.json.
- Tool calls fail with opaque JSON parse errors after upgrade.

**Phase to address:**
Phase 3 (state format) — add schema_version from v1.0. Phase 4 (distribution) — document reload step.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip `fsync` on state writes | Slightly faster writes | Corrupt state on power loss / kernel panic | Never — use `renameio` |
| Hand-roll MCP JSON-RPC framing | Avoid one dependency | Framing bugs, handshake bugs, spec drift | Never for this project |
| `os.WriteFile` for state.json | One line instead of five | Partial writes visible to concurrent readers | Never; always atomic-rename |
| Use `panic` for protocol errors | Easy to write | Panics can hit stdout via goroutine writeback in some configs | Never in serve path; return JSON-RPC errors |
| Skip CI Windows build | Faster CI | Cross-compile failures ship in release | Acceptable for MVP if stated in README; must add before 1.0 |
| Embedded YAML/TOML parser for config | Flexibility | +1-2 MB binary, extra init time | Never for this project — env vars + flags only |
| Hard-coded `~/.mcp-chain/state.json` | Simpler code | Can't unit-test without races; violates XDG | Never — always read `$MCP_CHAIN_STATE` override |
| Fixed-string error messages | Tests easy to write | Error type confusion across callers | Acceptable with `errors.Is` comparison; never with string matching |
| `log.Fatal` in serve path | One-liner error exit | Writes to stderr then exits, but may partial-write to stdout via buffered logger | Only after confirming logger is stderr-only |
| Skip the schema_version field | Less boilerplate | Painful forward migration; silent corruption on upgrade | Never — add from v1.0 |

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Claude Code plugin `.mcp.json` | Hard-coded absolute path or `npx`/`uvx` command | `"command": "${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain"` with per-OS binaries |
| Claude Code slash command | Multi-paragraph prompt with examples/caveats | One-sentence instruction, ≤ 40 tokens |
| Claude Code MCP restart | Assuming users restart on upgrade | Document in README; add `schema_version` check |
| flock across platforms | Using `syscall.Flock` directly | Use `github.com/gofrs/flock` (abstracts Windows) — but know its limitations |
| JSON state file | `os.WriteFile` | `github.com/google/renameio/v2` — fsync+rename |
| Cross-compile to Windows | Test only on macOS/Linux | CI matrix must include `windows-amd64` for build AND integration tests |
| MCP SDK selection | Hand-rolled framing | `github.com/mark3labs/mcp-go` or official Go SDK |
| Stdio wire | `fmt.Println` for debugging | `log.SetOutput(os.Stderr)` in `main()`; forbid `fmt.Print*` in serve path |
| Cobra (if used) | Default help output goes to stdout | `cmd.SetOut(os.Stderr)` for the serve subcommand |
| GoReleaser | Hand-rolled matrix in GitHub Actions | Use `goreleaser` from day one |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Linear scan of wordlist for "first unused" | O(n·m) per register | Pre-sort + bitmap of used indexes in memory while holding lock | ~500+ active registrations |
| JSON state file grown to MB | Slow reads under lock; noticeable register latency | Encourage `/chain-purge --resolved`; document expected entry counts | ~10,000 entries, or ~1MB state |
| Full-file rewrite on each update | Fsync cost; filesystem cache churn | Acceptable at this scale; revisit only if it becomes measurable | Never in practice for <10k entries |
| Init function doing work | Startup > 100ms | No init functions; lazy-init heavy structures | Cold-cache on slow disk |
| Binary > 15 MB | Slow plugin install/upgrade; antivirus attention | CI size gate; `-ldflags="-s -w"` | Any time adding a new dep |
| `os/user.Current` on glibc | Startup latency spike if NSS is slow | Use `$HOME` env directly; avoid `os/user.*` at startup | Network-backed NSS, LDAP-joined machines |
| Polling interval too short in wait monitor | CPU/disk load from N monitors polling | 1 s default; never < 250 ms | N=10+ concurrent waiters on slow disk |

## Security Mistakes

Domain-specific security issues beyond general web security. Note: this is a single-user local tool, so the risk surface is narrow.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Writing secrets/conditions to world-readable state file | Conditions could contain sensitive context | Set `state.json` perms to `0600`; parent dir `0700` |
| Trusting MCP client-provided "session identity" | Spoofed resolves | Don't attempt session-identity auth in MVP; accept "any session can resolve" |
| Logging conditions to stderr | Leaks to terminal scrollback / log collectors | Log IDs, not conditions, in non-debug mode |
| Binary unsigned on macOS | Gatekeeper warning; users may run random tar.gz | Sign release artifacts (or document the "xattr -d com.apple.quarantine" workaround) |
| Lock file perms too permissive | Other users on shared host can lock the state | `0600` on state and lock files; `0700` on dir |
| UPX compression | macOS Gatekeeper flags as potentially malicious; antivirus false positives | Don't use UPX |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Confusing exit codes for `status` subcommand | Bash monitor can't reliably detect states | PROJECT.md already specifies 0=resolved, 2=pending, 1=unknown — document and test exit codes |
| Word-IDs that look like English words but are easy to mistype | Users copy wrong ID into `/chain-wait` | EFF short wordlist is designed for this; accept 4-letter prefix matching as a future enhancement if reported |
| `/chain-wait` silently hangs forever | User thinks something is broken | Default is unbounded per spec; echo timeout at start if specified; document "it's supposed to hang" |
| Double-resolve silently succeeds | Hides bugs | Clear error: "already resolved" |
| Unknown ID error without "did you mean?" | User can't find the typo | Low priority; fixable later — print `/chain-list` suggestion |
| Install requires manual PATH or env setup | Plugin install feels broken | Claude Code plugin with bundled binary per OS; `${CLAUDE_PLUGIN_ROOT}` path |
| Stderr spam during normal operation | Users see scary log lines | Silent by default; `MCP_CHAIN_DEBUG=1` env to opt in |
| `/chain-purge --all` with no confirmation | User wipes all state by accident | Require `--yes` flag or TTY confirmation in plugin wrapper |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **MCP server:** runs locally — verify no stdout writes by piping stdout to `/dev/null` during integration tests; any byte on stdout that isn't a JSON-RPC frame is a bug
- [ ] **flock:** works in single-process tests — verify by spawning N subprocesses that all hammer the state file; assert no lost updates
- [ ] **Atomic write:** rename succeeds — verify `fsync` of parent directory happens (use `renameio`); kill-9 mid-write leaves either old or new, never corrupt
- [ ] **Word-ID allocator:** unique in tests — verify under concurrent registration across *processes*, not just goroutines
- [ ] **Tool descriptions:** sound helpful — verify word count; budget < 40 tokens per tool; test with a probe prompt
- [ ] **Slash commands:** work in a demo — verify token budget; < 30 words per file
- [ ] **Release binary:** downloads fine — verify `SHA256SUMS` attached; verify `--version` prints the tag, not a SHA; verify all 6 OS/arch combos present
- [ ] **Plugin install:** installs — verify on a fresh machine (no Go, no Node, no Python); verify `${CLAUDE_PLUGIN_ROOT}` substitution works
- [ ] **Cross-platform:** builds — verify CI *runs* tests on Windows, not just cross-compiles
- [ ] **State file schema:** migrates — verify `schema_version` field exists and server refuses to load unknown schema
- [ ] **XDG compliance:** honors `$XDG_STATE_HOME` — verify by setting the var and confirming the state file lands there
- [ ] **Startup time:** fast enough — verify `time ./mcp-chain serve </dev/null` is < 100ms on cold cache
- [ ] **Memory footprint:** small — verify RSS stays < 20 MB during steady-state operation with ~100 entries
- [ ] **Binary size:** under budget — verify `ls -la ./mcp-chain` < 15 MB for every target arch
- [ ] **Exit codes:** documented — verify `status` subcommand exit codes in docs match implementation (0/1/2)

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| stdout corruption discovered post-release | MEDIUM | Patch release; add CI test to prevent regression; grep codebase for all `fmt.Print*` |
| Lost-update / duplicate word-ID in production | HIGH | User-side: `/chain-purge --all`. Code-side: audit all state mutations for lock coverage; add concurrent-process test |
| Zero-byte state file after crash | MEDIUM | User-side: `rm ~/.mcp-chain/state.json` to restart clean. Code-side: switch to `renameio`; add `.bak` fallback |
| Binary too large (> 15 MB) | LOW | `go-size-analyzer` → identify culprit dep → remove or replace → retry |
| Startup too slow (> 100ms) | LOW | Profile init; remove init funcs; lazy-load |
| Tool descriptions burn tokens | LOW | Rewrite descriptions; no user-facing breakage |
| Plugin install fails on user's OS | MEDIUM | Add OS to CI matrix; publish patch release; document workaround |
| flock on NFS produces duplicates | HIGH for user, LOW for us | Document "local filesystem only"; add startup warning; user relocates state file |
| Windows concurrent test fails | MEDIUM | Switch state-path locking to separate lock file; re-run CI |
| MCP handshake incompatible with new Claude Code | MEDIUM | Update SDK dep; retest; ship patch release |
| Double-resolve not distinguishable from unknown-ID | LOW | Split error types; add test; release |
| User on NFS silently gets corrupted state | HIGH | Add startup warning; user moves state file |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| stdout wire corruption (#1) | Phase 1 (core scaffolding) | Integration test: pipe stdout to file; must be empty or valid JSON-RPC only |
| MCP handshake (#2) | Phase 1 | Use mature SDK; assert full handshake in integration test |
| Non-compact JSON (#3) | Phase 1 + Phase 2 | Build-time grep for `MarshalIndent` in serve path |
| Verbose tool descriptions (#4) | Phase 2 (tool handlers) | Token count per tool < 40; aggregate < 200 |
| npm/uvx path mistakes (#5) | Phase 4 (plugin packaging) | Fresh-machine install test (no Node/Python) |
| Lost-update race (#6) | Phase 3 (state layer) | 50-goroutine + 50-process concurrent registration test |
| Missing fsync on rename (#7) | Phase 3 | Use `renameio`; kill-9 mid-write test |
| flock on NFS (#8) | Phase 3 + Phase 6 (docs) | Startup warning on NFS; README caveat |
| Windows flock semantics (#9) | Phase 3 + Phase 5 (CI) | Separate lock file; CI runs tests on Windows |
| Wordlist race (#10) | Phase 3 | Concurrent-allocate test with distinct-IDs assertion |
| Test parallelism flake (#11) | Phase QA | `t.TempDir()` everywhere; `-race` in CI |
| Binary bloat (#12) | Phase 1 + Phase 5 | CI size check; fail if > 15 MB |
| Slow startup (#13) | Phase 1 | CI benchmark `time ./mcp-chain serve </dev/null` |
| Release misses arch / no checksums (#14) | Phase 5 (CI) | Goreleaser; verify all 6 combos + SHA256SUMS on tag release |
| Double-resolve vs unknown-ID (#15) | Phase 2 | Distinct error codes + tests |
| Session identity unknown (#16) | Phase 1 (research spike) → Phase 2 | Document limitation in PROJECT.md; spike before resolve tool design |
| Slash command prompt bloat (#17) | Phase 4 | Word count ≤ 30 per slash command file |
| No auto-restart on upgrade (#18) | Phase 3 + Phase 6 | Schema-version field; README documents reload |

## Sources

- MCP stdio corruption and stderr-only logging: [stdio mode corrupted by stdout log messages — claude-flow Issue #835](https://github.com/ruvnet/claude-flow/issues/835), [Build an MCP server — Model Context Protocol](https://modelcontextprotocol.io/docs/develop/build-server), [Error Handling And Debugging MCP Servers — Stainless](https://www.stainless.com/mcp/error-handling-and-debugging-mcp-servers), [MCP Server Troubleshooting 2026 — MCP Playground](https://mcpplaygroundonline.com/blog/mcp-server-troubleshooting-common-errors-fix)
- MCP protocol framing and handshake: [Transports — Model Context Protocol spec 2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports), [Understanding MCP Through Raw STDIO Communication](https://foojay.io/today/understanding-mcp-through-raw-stdio-communication/), [MCP Transport: Architecture, Boundaries, and Failure Modes](https://www.pgedge.com/blog/mcp-transport-architecture-boundaries-and-failure-modes)
- Claude Code slash-command token efficiency: [Stop Wasting Tokens: Developer's Guide to Claude Code Cleanup](https://naqeebali-shamsi.medium.com/stop-wasting-tokens-a-developers-guide-to-claude-code-cleanup-de842f6403e5), [Best Practices for Claude Code (official)](https://code.claude.com/docs/en/best-practices), [Claude Code Token Optimization: Full System Guide 2026](https://buildtolaunch.substack.com/p/claude-code-token-optimization)
- MCP tool description conciseness: [MCP tool descriptions: overview, examples, and best practices — Merge](https://www.merge.dev/blog/mcp-tool-description), [Writing Effective Tools for Agents — MCP docs](https://modelcontextprotocol.info/docs/tutorials/writing-effective-tools/), [SEP-1576: Mitigating Token Bloat in MCP — modelcontextprotocol Issue #1576](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1576), [MCP Token Optimization Strategies — Tetrate](https://tetrate.io/learn/ai/mcp/token-optimization-strategies)
- Claude Code plugin structure + paths: [Plugins reference — Claude Code Docs](https://code.claude.com/docs/en/plugins-reference), [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces), [Your Claude Plugin Marketplace Needs More Than a Git Repo](https://www.mpt.solutions/your-claude-plugin-marketplace-needs-more-than-a-git-repo/)
- flock cross-platform / NFS: [Everything you never wanted to know about file locking — apenwarr](https://apenwarr.ca/log/20101213), [Cross Platform File Locking with Go — Chrono](https://www.chronohq.com/blog/cross-platform-file-locking-with-go), [Flock() and fcntl() and Linux NFS (v3)](https://utcc.utoronto.ca/~cks/space/blog/linux/FlockFcntlAndNFS), [gofrs/flock package docs](https://pkg.go.dev/github.com/gofrs/flock)
- Atomic file writes + fsync: [Atomically writing files in Go (2017) — Michael Stapelberg](https://michael.stapelberg.ch/posts/2017-01-28-golang_atomically_writing/), [renameio package — google/renameio](https://pkg.go.dev/github.com/google/renameio), [Files are hard — Dan Luu](https://danluu.com/file-consistency/), [A way to do atomic writes — LWN.net](https://lwn.net/Articles/789600/)
- Go binary size: [How to Profile and Reduce Go Binary Size (Jan 2026)](https://oneuptime.com/blog/post/2026-01-07-go-reduce-binary-size/view), [Analyzing Go Binary Sizes — howardjohn](https://blog.howardjohn.info/posts/go-binary-size/), [Shrink your Go binaries with this one weird trick — Filippo](https://words.filippo.io/shrink-your-go-binaries-with-this-one-weird-trick/)
- Go startup and init: [Understanding the Go Runtime: The Bootstrap](https://internals-for-interns.com/posts/understanding-go-runtime/), [Embed CA Root Certificates in Go Programs](https://breml.github.io/blog/2021/01/17/embed-ca-root-certificates-in-go-programs/)
- Go embed pitfalls: [embed package — Go docs](https://pkg.go.dev/embed), [How to Bundle Static Assets into Go Binaries with go:embed (Jan 2026)](https://oneuptime.com/blog/post/2026-01-25-bundle-static-assets-go-embed/view)
- TOCTOU and locking critical sections: [Understanding TOCTOU: The Race Condition Hiding in Your Code](https://dev.to/codewithveek/understanding-toctou-the-race-condition-hiding-in-your-code-43nh), [CWE-367: Time-of-check Time-of-use](https://cwe.mitre.org/data/definitions/367.html), [FIO45-C. Avoid TOCTOU race conditions — CERT](https://wiki.sei.cmu.edu/confluence/display/c/FIO45-C.+Avoid+TOCTOU+race+conditions+while+accessing+files)
- Go test parallelism + race detector: [Data Race Detector — Go official](https://go.dev/doc/articles/race_detector), [t.Parallel in a subtest masks races — golang/go Issue #35670](https://github.com/golang/go/issues/35670), [Go runs package tests in parallel — TMPDIR forum](https://community.tmpdir.org/t/go-runs-package-tests-in-parallel-with-other-packages-tests/610)
- GoReleaser + reproducible builds: [How I build and ship Go binaries with GoReleaser and GitHub Actions](https://gitgist.com/posts/goreleaser-and-github-actions/), [Reproducible Builds — GoReleaser](https://goreleaser.com/blog/reproducible-builds/), [GoReleaser GitHub Actions docs](https://goreleaser.com/customization/ci/actions/)
- MCP Go SDK: [mark3labs/mcp-go GitHub](https://github.com/mark3labs/mcp-go), [MCP Go SDK (official) — modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk), [Building a Model Context Protocol Server in Go — Navendu](https://navendu.me/posts/mcp-server-go/)
- MCP per-session identity: [Connect Claude Code to tools via MCP — official](https://code.claude.com/docs/en/mcp), [Per-session MCP server profiles — claude-code Issue #45293](https://github.com/anthropics/claude-code/issues/45293)

---
*Pitfalls research for: Go MCP server + Claude Code plugin + shared-file coordination*
*Researched: 2026-04-23*
