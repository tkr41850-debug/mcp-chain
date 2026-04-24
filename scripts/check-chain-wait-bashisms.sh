#!/bin/sh
# check-chain-wait-bashisms.sh - grep-based lint for LD-4.
# Fails if plugin/scripts/chain-wait.sh contains any bash-4-only construct.
set -eu

FILE="plugin/scripts/chain-wait.sh"
FAIL=0

if [ ! -f "$FILE" ]; then
  echo "FAIL: $FILE missing" >&2
  exit 1
fi

# Each pattern + human-readable reason. Exclude comment lines
# (lines where the first non-whitespace char after the grep line-number prefix
# is '#') so narrative comments that MENTION a bashism for documentation
# do not trigger the lint.
check() {
  pattern="$1"; reason="$2"
  grep -nE "$pattern" "$FILE" 2>/dev/null | grep -v '^[0-9][0-9]*:[[:space:]]*#' > /tmp/cw-bashism.$$ || true
  if [ -s /tmp/cw-bashism.$$ ]; then
    echo "FAIL: $FILE contains $reason:" >&2
    cat /tmp/cw-bashism.$$ >&2
    FAIL=1
  fi
  rm -f /tmp/cw-bashism.$$
}

# [[ ... ]]  - bash 2+ only (not POSIX)
check '\[\[' "'[[' (use '[')"
# (( ... )) as a command at line start - bash arithmetic command, not POSIX
# (distinct from $((...)) which IS POSIX and remains allowed)
check '(^|[[:space:]])\(\([^(]' "'((' command (use '\$((...))' for arithmetic)"
# =~ regex match - bash 3.0+, not POSIX
check '=~' "'=~' regex match (use 'case' with globs)"
# mapfile / readarray - bash 4+
check 'mapfile|readarray' "'mapfile'/'readarray' (bash 4+)"
# Process substitution <( ... )
check '<\(' "'<(...)' process substitution"
# &> redirect
check '&>' "'&>' redirect (use '>file 2>&1')"
# declare / typeset with -A (associative array)
check 'declare[[:space:]]+-A|typeset[[:space:]]+-A' "'declare -A'/'typeset -A' associative array"
# ${var,,} / ${var^^} case conversion - bash 4+
check '\$\{[A-Za-z_][A-Za-z0-9_]*,,\}|\$\{[A-Za-z_][A-Za-z0-9_]*\^\^\}' "'\${var,,}'/'\${var^^}' case conversion (bash 4+)"

if [ "$FAIL" -eq 0 ]; then
  echo "No bashisms found in $FILE"
fi
exit "$FAIL"
