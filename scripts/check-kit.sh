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
	"strings"

	"github.com/Project-Arrakis/tabr-tau/kit/durable"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/fillables"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/regions"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/vehicles"
	"github.com/Project-Arrakis/tabr-tau/kit/gamefolder"
	"github.com/Project-Arrakis/tabr-tau/kit/savefile"
	"github.com/Project-Arrakis/tabr-tau/kit/schema"
	"github.com/Project-Arrakis/tabr-tau/kit/sessionlog"
	"github.com/Project-Arrakis/tabr-tau/kit/webguard"
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

	if f, ok := vehicles.ChassisFuel("BP_Buggy_CHOAM_C", "BuggyChassis_6"); !ok || f != 300 || len(vehicles.ChassisTanks()) != 6 {
		fmt.Fprintln(os.Stderr, "vehicles chassis tank failed:", f, ok)
		os.Exit(1)
	}
	fmt.Println("consumer: vehicles chassis tank ok")

	if regions.Name("HaggaBasin", 3) != "Hagga Rift" || regions.Name("HaggaBasin", 0) != "" || len(regions.Names("HaggaBasin")) != 12 {
		fmt.Fprintln(os.Stderr, "regions lookup failed")
		os.Exit(1)
	}
	fmt.Println("consumer: regions lookup ok")

	if !webguard.Host("localhost:8090") || webguard.Host("example.com") || !webguard.Peer("127.0.0.1:5") || webguard.Peer("192.168.0.4:5") ||
		webguard.OriginOf("http://127.0.0.1:8090/x") != "http://127.0.0.1:8090" || webguard.AllowedNavigation("http://127.0.0.1:8090", "http://example.com/") {
		fmt.Fprintln(os.Stderr, "webguard rules failed")
		os.Exit(1)
	}
	fmt.Println("consumer: webguard rules ok")

	if !strings.Contains(schema.DDL(), "CREATE TABLE vehicle_modules (") || schema.VehicleModules.Table != "vehicle_modules" ||
		schema.VehicleModuleCurrentDurability != "$.FVehicleModuleDurabilityStats[1].CurrentDurability" || schema.StoredStates != "2, 4" {
		fmt.Fprintln(os.Stderr, "schema failed")
		os.Exit(1)
	}
	fmt.Println("consumer: schema ok")

	if gamefolder.LogPath(gamefolder.SavedDir(`C:\L`)) != filepath.Join(`C:\L`, "DuneSandbox", "Saved", "Logs", "DuneSandbox.log") {
		fmt.Fprintln(os.Stderr, "gamefolder failed")
		os.Exit(1)
	}
	if _, err := gamefolder.LogPathFor(filepath.Join("elsewhere", "game.db")); err == nil {
		fmt.Fprintln(os.Stderr, "gamefolder accepted a save outside the tree")
		os.Exit(1)
	}
	fmt.Println("consumer: gamefolder ok")

	var tr sessionlog.Tracker
	tr.Feed(sessionlog.Policy{}, "[2026.10.08-16.22.23:461]LogNet: Display: Browse (net mode 0): /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen", nil)
	started := tr.SPActive
	tr.Feed(sessionlog.Policy{}, "[2026.10.08-17.10.52:973]LogNet: Display: Browse (net mode 2): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?closed", nil)
	if !started || tr.SPActive || tr.EndedAt.IsZero() || sessionlog.Classify(sessionlog.Policy{Strict: true}, "[x]LogChat: Display: Player: LogNet: Display: Browse (net mode 0): /Game/X?listen") != sessionlog.NoEvent {
		fmt.Fprintln(os.Stderr, "sessionlog failed")
		os.Exit(1)
	}
	fmt.Println("consumer: sessionlog ok")
}
GO
(cd "$tmp/consumer" && go run .)
echo "kit checks passed"
