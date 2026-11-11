#!/usr/bin/env bash
# Checks that the design module (./design) stands on its own.
#
#   1. The editor's go.mod requires the design module and replaces it with ./design, so a pull request builds and serves the files in the tree (the editor adopted the
#      module with the Look slice, #570), and the editor builds against them.
#   2. A clean export of the tracked files of ./design (what a tag would publish) vets and passes its own tests, and carries
#      the files a tag must carry.
#   3. A small program outside the repository builds and runs against that export, serves the stylesheet and reads it.
#      Nothing from the editor may be needed.
#
# Needs a Go toolchain and git. Run from anywhere inside the repository, after the files are committed (step 2 uses HEAD):
#   bash scripts/check-design.sh
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"
export GOWORK=off

mod=github.com/Project-Arrakis/tabr-tau/design
replaced="$(go list -m -f '{{if .Replace}}{{.Replace.Path}}{{end}}' "$mod")"
if [ "$replaced" != "./design" ]; then
  echo "ERROR: go.mod must replace $mod with ./design (found '$replaced')" >&2
  exit 1
fi
echo "go.mod replaces $mod with ./design"

go build ./...
echo "the editor builds against ./design"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

git archive HEAD design | tar -x -C "$tmp"
for f in go.mod LICENSE NOTICE PROVENANCE.md VERSION CHANGELOG.md assets/tokens.css assets/tokens-fremen.css assets/components-fremen.css assets/components.css assets/motion.css assets/icons.svg assets/fonts/DuneRise-OFL.txt assets/fonts/SHA256SUMS kitchen-sink/index.html; do
  if [ ! -s "$tmp/design/$f" ]; then
    echo "ERROR: design/$f is missing or empty in the export" >&2
    exit 1
  fi
done
(cd "$tmp/design" && go vet ./... && go test -count=1 ./...)

# the version files agree with each other (the tests check the same; this fails the job with a plain message first)
version="$(tr -d '[:space:]' < "$tmp/design/VERSION")"
if ! grep -q "^## ${version}\( \|$\)" "$tmp/design/CHANGELOG.md"; then
  echo "ERROR: design/CHANGELOG.md has no entry '## ${version}'" >&2
  exit 1
fi
if ! grep -q "const Version = \"${version}\"" "$tmp/design/design.go"; then
  echo "ERROR: design.Version is not ${version}" >&2
  exit 1
fi

mkdir "$tmp/consumer"
cat > "$tmp/consumer/go.mod" <<'MOD'
module example.test/consumer

go 1.26.0

require github.com/Project-Arrakis/tabr-tau/design v0.0.0-00010101000000-000000000000

replace github.com/Project-Arrakis/tabr-tau/design => ../design
MOD
cat > "$tmp/consumer/main.go" <<'GO'
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/design"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle("/design/", http.StripPrefix("/design/", design.Handler()))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/design/tokens.css", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "--c-ground:") || w.Body.String() != string(design.Tokens()) {
		fmt.Fprintln(os.Stderr, "the stylesheet was not served:", w.Code)
		os.Exit(1)
	}
	fmt.Println("consumer: tokens served by", design.Version)
}
GO
(cd "$tmp/consumer" && go run .)
echo "design checks passed"
