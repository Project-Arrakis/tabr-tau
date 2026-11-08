#!/usr/bin/env bash
# Prints the release notes of a design tag: the entry of design/CHANGELOG.md whose heading is "## vX.Y.Z" (optionally followed by a
# space and a date). Fails when the entry is missing or empty, so a tag without a changelog entry makes no release.
#
#   bash scripts/design-release-notes.sh design/v0.1.0
set -euo pipefail

tag="${1:?usage: design-release-notes.sh design/vX.Y.Z}"
version="${tag#design/}"
root="$(git rev-parse --show-toplevel)"

notes="$(awk -v h="## ${version}" '
  index($0, h) == 1 && (length($0) == length(h) || substr($0, length(h) + 1, 1) == " ") { on = 1; next }
  /^## / { on = 0 }
  on
' "$root/design/CHANGELOG.md")"

if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
  echo "ERROR: design/CHANGELOG.md has no entry '## ${version}' with text under it" >&2
  exit 1
fi
printf '%s\n' "$notes"
