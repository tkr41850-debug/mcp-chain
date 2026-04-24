# mcp-chain

Lightweight MCP server for chaining Claude Code sessions at fan-in/fan-out.

## Why

When you run more than one Claude Code session against a shared codebase, you
often want Session B to wait until Session A finishes something — a build, a
deploy, a review, a prompt you're composing. mcp-chain gives any session a
short word-ID it can publish and any number of other sessions a way to block
on that ID until the first session resolves it. One binary, one JSON file,
one `flock(2)`.

## Install

### With Claude Code (recommended)

```
/plugin install anthropics/mcp-chain
```

Claude Code downloads the latest release binary for your OS/arch from GitHub
Releases and installs it under `${CLAUDE_PLUGIN_ROOT}/bin/mcp-chain`. No
further wiring needed — the plugin ships its own `.mcp.json`, slash commands,
and monitor script.

### Without Claude Code

Install the binary:

```
go install github.com/anthropics/mcp-chain/cmd/mcp-chain@latest
```

Then wire it into your MCP client. For a generic `.mcp.json`:

```json
{
  "mcpServers": {
    "mcp-chain": {
      "command": "mcp-chain",
      "args": ["serve"]
    }
  }
}
```

## Usage

### Two-session demo

Session A (the registrant):

```
/mcp-chain:reg build passes
→ otter
```

Session B (the waiter), started in a different terminal or conversation:

```
/mcp-chain:wait otter
(polls once per second, silent until resolve)
```

Back in Session A, once the condition is satisfied:

```
/mcp-chain:resolve otter
```

Session B prints:

```
continue
```

and unblocks.

### CLI subcommands

The same binary backs both the MCP server and a small CLI surface for
scripting and recovery:

| Command | Purpose | Exit codes |
|---------|---------|------------|
| `mcp-chain serve` | Start the MCP stdio server (called by your MCP client) | — |
| `mcp-chain status <id>` | Query one entry | `0` resolved, `2` pending, `1` unknown / error |
| `mcp-chain list` | Print all entries as a table | `0` always |
| `mcp-chain resolve <id> [--force]` | Mark an ID resolved; `--force` bypasses the OwnerToken session check | `0` on success, `1` on error |
| `mcp-chain purge <id> \| --all \| --resolved` | Delete entries; bare `purge` errors | `0` on success, `1` if no target given |
| `mcp-chain --version` | Print the binary version (e.g. `mcp-chain 0.1.0`) | `0` |

## Commands

Claude Code prefixes plugin commands with `<plugin>:`, so the files under
`plugin/commands/` are invoked in chat with the `/mcp-chain:` namespace:

| Slash command | Source prompt | What it does |
|---------------|---------------|--------------|
| `/mcp-chain:reg [condition]` | `plugin/commands/reg.md` | Register a new chain entry; returns a word-ID (e.g. `otter`) |
| `/mcp-chain:wait <id> [--timeout DURATION]` | `plugin/commands/wait.md` | Poll until the ID resolves; prints `continue` on success |
| `/mcp-chain:list` | `plugin/commands/list.md` | Dump the full table of entries |
| `/mcp-chain:purge [<id> \| --all \| --resolved]` | `plugin/commands/purge.md` | Explicit cleanup |

## State file

mcp-chain writes to a single JSON file under an XDG-compliant path:

```
$XDG_STATE_HOME/mcp-chain/state.json       (if $XDG_STATE_HOME is set)
~/.mcp-chain/state.json                    (fallback)
```

Permissions: parent directory `0700`, file `0600`. Writes are atomic
(temp-file + `fsync` + rename under `flock(2)` exclusive lock). Reads take a
shared lock; only mutations take the exclusive lock.

## Networked filesystems

**Do not place the state file on NFS, SMB, or CIFS.** `flock(2)` semantics
over networked filesystems are not reliable — you can get lost updates,
duplicate word-IDs, and corrupt state. Use a local filesystem (ext4, APFS,
NTFS on NTFS-backed drives). If `$HOME` is on an NFS mount, set
`$XDG_STATE_HOME` to a local path such as `/var/tmp/$USER/state`.

## Upgrade / reload

To pick up a new release, run `/mcp` in Claude Code, select `mcp-chain`, and
restart. If Claude Code does not list a restart option for your version,
fully quit and relaunch Claude Code. Claude Code's plugin-reload surface has
evolved across releases; either path reconnects the MCP server.

To upgrade the plugin itself, re-run `/plugin install anthropics/mcp-chain`;
Claude Code will fetch the latest release asset.

## Security notes

- `/mcp-chain:wait` and `/mcp-chain:purge` pass `$ARGUMENTS` to a bash
  subprocess without per-token sanitization. Same threat model as every
  Claude Code plugin: the user is assumed to trust their own slash-command
  input.
- Plugin invocations run with the same privilege as your Claude Code
  session. mcp-chain does not sandbox, chroot, or drop privileges.
- Do not use mcp-chain for trust isolation on multi-tenant machines.
  The state file is per-user; it assumes a single trusted operator.

## Troubleshooting

**`mcp-chain: command not found`** — The plugin install should put the
binary on PATH automatically. If you installed via `go install`, confirm
`$GOBIN` (or `$HOME/go/bin`) is on PATH. Run `mcp-chain --version` to
verify; you should see `mcp-chain 0.1.0` or newer.

**`error: unknown id: otter`** — The ID was either never registered in this
process's state file, or it was purged. Run `mcp-chain list` to see what's
actually there. IDs are global per state file, not per session.

**State file appears corrupt or permission-denied** — Check the path printed
in the error (`$XDG_STATE_HOME/mcp-chain/state.json` or
`~/.mcp-chain/state.json`). File mode must be `0600`, parent dir `0700`. If
the JSON is malformed (rare; only possible from a disk failure during
write), move it aside and let mcp-chain recreate it.

**Monitor script hangs or behaves oddly** — The bash monitor targets bash
3.2 (macOS default). If you've edited it, avoid `[[ ]]`, `=~`, `declare -A`,
and process substitution. Run `bash -n plugin/scripts/chain-wait.sh` to
syntax-check.

**Slow startup on NFS / shared $HOME** — See the Networked filesystems
section above. Set `$XDG_STATE_HOME` to a local path.

**About the binary:** mcp-chain uses the official
`github.com/modelcontextprotocol/go-sdk`, which transitively imports
`net/http` via its SSE/streamable transport files. The stdio transport the
binary actually runs does not use `net/http` at runtime; the import is
unavoidable without forking the SDK.

## License

License: TBD — see project roadmap.
