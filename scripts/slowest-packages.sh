#!/usr/bin/env bash
# Print the slowest packages of a `go test` log, and write a warning annotation for a package that took more than half of the test limit, so a package that is
# getting close to its limit shows up on the run's page before it fails a job (#767, follow-up of #756).
#
# Usage: bash scripts/slowest-packages.sh race.log limit_minutes [count]
# Reads the "ok   <package>   12.3s" and "FAIL   <package>   12.3s" lines of `go test` (with or without -v and coverage; a "(cached)" package has no time and is skipped).
# It only reports: it never fails the build, because the limit that fails a build is the test timeout in the workflow.
set -euo pipefail

log="${1:?usage: slowest-packages.sh test.log limit_minutes [count]}"
limit_min="${2:?usage: slowest-packages.sh test.log limit_minutes [count]}"
count="${3:-10}"

echo "The ${count} slowest packages (seconds, package), limit ${limit_min} min per package:"
# The last stage reads everything sort writes (see slowest-tests.sh, #480): no `head -n` after sort under pipefail.
{ grep -E '^(ok|FAIL)[[:space:]]+[^[:space:]]+[[:space:]]+[0-9.]+s([[:space:]]|$)' "$log" || true; } |
  awk '{ t = $3; sub(/s$/, "", t); print t, $2 }' |
  sort -rn |
  awk -v n="$count" -v limit="$limit_min" '
    { t = $1 + 0; pkg = $2 }
    NR <= n { printf "%8.1f %s\n", t, pkg }
    t * 2 > limit * 60 { printf "::warning title=slow test package::%s took %.0f s, more than half of the %s minute test limit\n", pkg, t, limit }
  '
