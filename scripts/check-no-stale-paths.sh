#!/bin/sh
# check-no-stale-paths.sh — enforce LD-14 command rename.
#
# Commands were renamed from /chain-reg, /chain-wait, /chain-list,
# /chain-purge to /mcp-chain:reg, /mcp-chain:wait, /mcp-chain:list,
# /mcp-chain:purge (Phase 8, user-approved 2026-04-24). This gate
# greps user-facing surfaces for stale slash-command references.
#
# Excluded (legitimately mention the old names):
#   - .planning/             historical decisions, not user-facing
#   - scripts/smoke-chain-wait.sh        script named after the monitor,
#                                         not the slash command
#   - scripts/check-no-stale-paths.sh    this file
#   - scripts/check-chain-wait-bashisms.sh  docstring references the
#                                            monitor script path
#   - **/*_test.go           negative assertions (NotContains) inside
#                            tests are self-enforcing
#   - filename references like "chain-wait.sh" (filtered via .sh suffix)
set -eu

PATTERN='/chain-(reg|wait|list|purge)'

HITS="$(git grep -InE "$PATTERN" -- \
  ':!.planning' \
  ':!scripts/smoke-chain-wait.sh' \
  ':!scripts/check-no-stale-paths.sh' \
  ':!scripts/check-chain-wait-bashisms.sh' \
  ':!**/*_test.go' \
  2>/dev/null | grep -vE '/chain-(reg|wait|list|purge)\.sh' || true)"

if [ -n "$HITS" ]; then
  echo "FAIL: stale slash-command references found (LD-14 forbids /chain-* slash commands):" >&2
  echo "$HITS" >&2
  exit 1
fi

echo "No stale /chain-* slash-command references."
