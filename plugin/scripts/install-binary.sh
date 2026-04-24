#!/usr/bin/env bash
# install-binary.sh - Ensure ${CLAUDE_PLUGIN_DATA}/bin/mcp-chain is present
# and matches the version pinned in plugin.json.
#
# Idempotent: fast no-op when the binary already matches the pinned version.
# bash 3.2 safe (macOS default). POSIX tools only: uname, mkdir, sed,
# curl or wget, tar, unzip on Windows.
#
# Why this exists: Claude Code's plugin install only copies source files. It
# does NOT download release assets. This script runs on first slash-command
# / first MCP spawn and fetches the correct binary for the host's OS/arch
# into ${CLAUDE_PLUGIN_DATA}, which persists across plugin upgrades.

set -eu

# Claude Code sets CLAUDE_PLUGIN_ROOT / CLAUDE_PLUGIN_DATA for MCP server
# spawns and hook executions, but NOT when the LLM runs this script via its
# Bash tool in response to a slash-command prompt. Self-derive both so the
# script works in either context.

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-}"
if [ -z "$PLUGIN_ROOT" ]; then
  SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
  PLUGIN_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
fi

PLUGIN_DATA="${CLAUDE_PLUGIN_DATA:-}"
if [ -z "$PLUGIN_DATA" ]; then
  # Mirror Claude Code's own layout when possible: plugin cache lives at
  # .../plugins/cache/<marketplace>/<plugin>/<version> and the matching data
  # dir is .../plugins/data/<plugin>-<marketplace>. Fall back to PLUGIN_ROOT
  # itself (wiped on plugin upgrade, but idempotent re-install is cheap).
  case "$PLUGIN_ROOT" in
    */plugins/cache/*/*/*)
      # Layout: <prefix>/plugins/cache/<market>/<plugin>/<version>
      D1="${PLUGIN_ROOT%/*}"     # .../cache/<market>/<plugin>
      D2="${D1%/*}"              # .../cache/<market>
      D3="${D2%/*}"              # .../cache
      D4="${D3%/*}"              # .../plugins
      PLUGIN_DATA="$D4/data/${D1##*/}-${D2##*/}"
      ;;
    *)
      PLUGIN_DATA="$PLUGIN_ROOT"
      ;;
  esac
fi

BIN_DIR="$PLUGIN_DATA/bin"
BIN_PATH="$BIN_DIR/mcp-chain"
VERSION_FILE="$BIN_DIR/.version"
MANIFEST="$PLUGIN_ROOT/.claude-plugin/plugin.json"

# Parse "version": "X.Y.Z" from plugin.json without jq.
VERSION=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$MANIFEST" | head -1)
if [ -z "$VERSION" ]; then
  echo "mcp-chain: cannot parse version from $MANIFEST" >&2
  exit 1
fi

# Fast path: binary exists and version marker matches.
if [ -x "$BIN_PATH" ] && [ -f "$VERSION_FILE" ]; then
  INSTALLED=$(cat "$VERSION_FILE" 2>/dev/null || echo "")
  if [ "$INSTALLED" = "$VERSION" ]; then
    exit 0
  fi
fi

# Detect OS.
OS_RAW=$(uname -s)
case "$OS_RAW" in
  Linux)                    GOOS=linux ;;
  Darwin)                   GOOS=darwin ;;
  MINGW*|MSYS*|CYGWIN*)     GOOS=windows ;;
  *) echo "mcp-chain: unsupported OS: $OS_RAW" >&2; exit 1 ;;
esac

# Detect arch.
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
  x86_64|amd64)             GOARCH=amd64 ;;
  arm64|aarch64)            GOARCH=arm64 ;;
  *) echo "mcp-chain: unsupported arch: $ARCH_RAW" >&2; exit 1 ;;
esac

# Asset naming mirrors .goreleaser.yaml:
#   {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}.{tar.gz|zip}
EXT="tar.gz"
BIN_NAME="mcp-chain"
if [ "$GOOS" = "windows" ]; then
  EXT="zip"
  BIN_NAME="mcp-chain.exe"
fi

ARCHIVE="mcp-chain_${VERSION}_${GOOS}_${GOARCH}.${EXT}"
URL="https://github.com/tkr41850-debug/mcp-chain/releases/download/v${VERSION}/${ARCHIVE}"

TMPDIR=$(mktemp -d 2>/dev/null || mktemp -d -t mcp-chain)
trap 'rm -rf "$TMPDIR"' EXIT

echo "mcp-chain: fetching $ARCHIVE..." >&2
if command -v curl >/dev/null 2>&1; then
  curl --fail --location --silent --show-error "$URL" --output "$TMPDIR/$ARCHIVE"
elif command -v wget >/dev/null 2>&1; then
  wget --quiet "$URL" --output-document "$TMPDIR/$ARCHIVE"
else
  echo "mcp-chain: neither curl nor wget is available" >&2
  exit 1
fi

if [ "$EXT" = "zip" ]; then
  if ! command -v unzip >/dev/null 2>&1; then
    echo "mcp-chain: unzip not found (required on Windows)" >&2
    exit 1
  fi
  (cd "$TMPDIR" && unzip -q "$ARCHIVE")
else
  (cd "$TMPDIR" && tar xzf "$ARCHIVE")
fi

if [ ! -f "$TMPDIR/$BIN_NAME" ]; then
  echo "mcp-chain: $BIN_NAME missing from archive" >&2
  exit 1
fi

mkdir -p "$BIN_DIR"
mv "$TMPDIR/$BIN_NAME" "$BIN_PATH"
chmod +x "$BIN_PATH"
printf '%s' "$VERSION" > "$VERSION_FILE"

echo "mcp-chain: installed $VERSION to $BIN_PATH" >&2
