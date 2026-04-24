#!/bin/sh
# check-prompt-wordcount.sh — enforce SC-2 / LD-3: every plugin/commands/*.md
# prompt BODY (YAML frontmatter stripped) <= 30 words.
set -eu

MAX=30
FAIL=0

for md in plugin/commands/*.md; do
  [ -f "$md" ] || continue
  # Strip frontmatter: lines between the first two '---' markers are discarded.
  # If no frontmatter, print everything. Awk state machine:
  #   n==0 (before first ---): print everything EXCEPT '---' itself
  #   n==1 (inside frontmatter): skip
  #   n==2 (after second ---): print
  body="$(awk '/^---$/{n++; next} n==1{next} {print}' "$md")"
  words="$(printf '%s\n' "$body" | wc -w | tr -d ' ')"
  if [ "$words" -gt "$MAX" ]; then
    echo "FAIL: $md has $words words (max $MAX)" >&2
    FAIL=1
  else
    echo "OK: $md has $words words"
  fi
done

if [ "$FAIL" -eq 0 ]; then
  echo "All prompt word-counts within budget."
fi
exit "$FAIL"
