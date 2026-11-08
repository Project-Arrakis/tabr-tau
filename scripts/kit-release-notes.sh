#!/usr/bin/env bash
# Prints the release notes of a kit tag: the entry of kit/CHANGELOG.md whose heading is "## vX.Y.Z" (optionally followed by a
# space and a date). Fails when the entry is missing or empty, so a tag without a changelog entry makes no release.
#
#   bash scripts/kit-release-notes.sh kit/v0.1.0
set -euo pipefail

tag="${1:?usage: kit-release-notes.sh kit/vX.Y.Z}"
version="${tag#kit/}"
root="$(git rev-parse --show-toplevel)"

notes="$(awk -v h="## ${version}" '
  index($0, h) == 1 && (length($0) == length(h) || substr($0, length(h) + 1, 1) == " ") { on = 1; next }
  /^## / { on = 0 }
  on
' "$root/kit/CHANGELOG.md")"

if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
  echo "ERROR: kit/CHANGELOG.md has no entry '## ${version}' with text under it" >&2
  exit 1
fi
printf '%s\n' "$notes"
