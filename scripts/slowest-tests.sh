#!/usr/bin/env bash
# Print the slowest tests of a `go test -v` log, so a test that gets slow shows up in the CI log before it costs a job (#437).
#
# Usage: bash scripts/slowest-tests.sh test.log [count]
# Reads the "--- PASS: TestName (1.23s)" lines (also FAIL and SKIP, and subtests) and prints the `count` longest (default 10), longest first.
# It only reports: it never fails the build on a slow test, because the limit that fails a build is the test timeout in the workflow.
set -euo pipefail

log="${1:?usage: slowest-tests.sh test.log [count]}"
count="${2:-10}"

echo "The ${count} slowest tests (seconds, test):"
grep -E '^[[:space:]]*--- (PASS|FAIL|SKIP): ' "$log" |
  sed -E 's/^[[:space:]]*--- (PASS|FAIL|SKIP): (.*) \(([0-9.]+)s\).*$/\3 \2/' |
  sort -rn |
  awk -v n="$count" 'NR <= n { t = $1; $1 = ""; printf "%8.2f %s\n", t, substr($0, 2) }'
# The last stage reads everything sort writes. With `head -n` there instead, head leaves after n lines, sort is killed by SIGPIPE if it still has output
# to write, and `set -o pipefail` turns that into exit 141 (a log of about 1,500 tests did it on the Windows runner, #480).
