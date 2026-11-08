#!/usr/bin/env bash
# Checks that the kit module (./kit) stands on its own and that the editor builds against it.
#
#   1. The editor's go.mod requires the kit module and replaces it with ./kit, so a pull request builds the code in the tree.
#   2. The editor builds, as one program, against the module in the tree.
#   3. A clean export of the tracked files of ./kit (what a tag would publish) vets and passes its own tests, and a small program
#      outside the repository builds and runs against that export. Nothing from the editor may be needed.
#
# Needs a Go toolchain and git. Run from anywhere inside the repository, after the files are committed (step 3 uses HEAD):
#   bash scripts/check-kit.sh
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"
export GOWORK=off

mod=github.com/Project-Arrakis/tabr-tau/kit

replaced="$(go list -m -f '{{if .Replace}}{{.Replace.Path}}{{end}}' "$mod")"
if [ "$replaced" != "./kit" ]; then
  echo "ERROR: go.mod must replace $mod with ./kit (found '$replaced')" >&2
  exit 1
fi
echo "go.mod replaces $mod with ./kit"

go build ./...
echo "the editor builds against ./kit"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

git archive HEAD kit | tar -x -C "$tmp"
(cd "$tmp/kit" && go vet ./... && go test -count=1 ./...)

mkdir "$tmp/consumer"
cat > "$tmp/consumer/go.mod" <<'MOD'
module example.test/consumer

go 1.26.0

require github.com/Project-Arrakis/tabr-tau/kit v0.0.0-00010101000000-000000000000

replace github.com/Project-Arrakis/tabr-tau/kit => ../kit
MOD
cat > "$tmp/consumer/main.go" <<'GO'
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Project-Arrakis/tabr-tau/kit/durable"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/fillables"
	"github.com/Project-Arrakis/tabr-tau/kit/savefile"
)

func main() {
	raw := append([]byte("SQLite format 3\x00"), make([]byte, 4096)...)
	blob, err := savefile.Encode(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	back, err := savefile.Decode(blob)
	if err != nil || !bytes.Equal(back, raw) {
		fmt.Fprintln(os.Stderr, "round trip failed:", err)
		os.Exit(1)
	}
	fmt.Println("consumer: round trip ok")

	dir, err := os.MkdirTemp("", "consumer-durable-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "f")
	for _, text := range []string{"one", "two"} {
		if err := durable.WriteFile(p, []byte(text), 0o600, durable.Defaults()); err != nil {
			fmt.Fprintln(os.Stderr, "durable write failed:", err)
			os.Exit(1)
		}
	}
	if got, err := os.ReadFile(p); err != nil || string(got) != "two" {
		fmt.Fprintln(os.Stderr, "durable write did not replace the file:", err)
		os.Exit(1)
	}
	fmt.Println("consumer: durable write ok")

	if r, ok := fillables.Lookup("Literjon_T6"); !ok || r.MaxAmount != 20000 || r.Restriction != fillables.Water || !r.Container {
		fmt.Fprintln(os.Stderr, "fillables lookup failed:", r, ok)
		os.Exit(1)
	}
	fmt.Println("consumer: fillables lookup ok")
}
GO
(cd "$tmp/consumer" && go run .)
echo "kit checks passed"
