#!/usr/bin/env bash
# Fail if any commit message in the checked range carries an AI co-author trailer or an
# AI attribution line. Operator standing rule; finding F-12 (issue #13).
#
# Range: on a pull request, BASE_SHA..HEAD_SHA. On a push, only the tip commit (HEAD_SHA).
# Also usable locally:  BASE_SHA=origin/main HEAD_SHA=HEAD bash scripts/check-commit-messages.sh
set -euo pipefail

head_sha="${HEAD_SHA:-HEAD}"
base_sha="${BASE_SHA:-}"

if [ -n "$base_sha" ] && [ "$base_sha" != "0000000000000000000000000000000000000000" ]; then
  range="${base_sha}..${head_sha}"
  messages="$(git log --format='%H%n%B%n-----' "$range")"
else
  messages="$(git log -1 --format='%H%n%B%n-----' "$head_sha")"
fi

# Patterns: Co-Authored-By naming an AI assistant/vendor, or a "Generated with ..." attribution line.
pattern='^(co-authored-by:.*(claude|anthropic|openai|chatgpt|copilot|gemini|cursor)|.*generated (with|by) .*(claude|chatgpt|copilot|gemini))'

if hits="$(printf '%s\n' "$messages" | grep -inE "$pattern")"; then
  echo "ERROR: commit message policy violation (no AI co-author trailers or attribution lines):" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi
echo "commit messages OK"
