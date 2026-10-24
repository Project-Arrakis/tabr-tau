package catalog

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Template ids are spliced into the save, so every one must fit the same pattern the write path accepts.
var idRe = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,120}$`)

func TestCatalogIsSane(t *testing.T) {
	if len(All()) < 2000 {
		t.Fatalf("only %d entries", len(All()))
	}
	seen := map[string]bool{}
	for _, e := range All() {
		if e.ID == "" || e.Name == "" {
			t.Fatalf("empty id or name: %+v", e)
		}
		if !idRe.MatchString(e.ID) {
			t.Errorf("id %q does not look like a template id", e.ID)
		}
		if seen[e.ID] {
			t.Errorf("duplicate id %s", e.ID)
		}
		seen[e.ID] = true
	}
}

func TestKnownNames(t *testing.T) {
	for id, want := range map[string]string{
		"Literjon":                "Literjon",
		"Literjon_T6":             "Literjon Mk6",
		"Decajon":                 "Decaliterjon",
		"HighCapacityLiterjon_06": "Hajra Literjon Mk6",
		"Oil":                     "Fuel Cell",
		"SpicedFuelCell":          "Spice-infused Fuel Cell",
	} {
		if got, ok := Name(id); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", id, got, ok, want)
		}
	}
	if _, ok := Name("NotAnItem"); ok {
		t.Error("an unknown id must not be found")
	}
}

func TestFirstEntryWinsAndNamesResolve(t *testing.T) {
	// items.json has no duplicate ids, but Name must still be deterministic if one is ever added.
	for _, e := range All() {
		if n, ok := Name(e.ID); !ok || n != e.Name {
			t.Fatalf("%s: %q %v", e.ID, n, ok)
		}
	}
}

func TestMaxStack(t *testing.T) {
	for id, want := range map[string]int64{"SpicedFuelCell": 500, "AluminiumBar": 500, "SpiceSand": 2500, "Literjon": 1} {
		if got, ok := MaxStack(id); !ok || got != want {
			t.Errorf("%s: %d %v, want %d", id, got, ok, want)
		}
	}
	if _, ok := MaxStack("NotAnItem"); ok {
		t.Error("an unknown id has no stack size")
	}
	for id, n := range stacks {
		if n < 2 {
			t.Errorf("%s: only items that stack are listed, got %d", id, n)
		}
	}
}

// The two datasets taken from the game's own tables must say where they come from (audit GRC-6): the provenance file names both and carries
// their real sizes, so a refresh of either file cannot leave the notice describing the old one.
func TestProvenanceDescribesTheGameTableDatasets(t *testing.T) {
	b, err := os.ReadFile("PROVENANCE.md")
	if err != nil {
		t.Fatal(err)
	}
	p := string(b)
	ids := map[string]bool{}
	entries := 0
	for _, list := range unlocks {
		for _, id := range list {
			ids[id] = true
			entries++
		}
	}
	for _, want := range []string{
		"`stacks.json`", "`customization_unlocks.json`",
		fmt.Sprintf("For %d item template ids", len(stacks)),
		fmt.Sprintf("For %d cosmetic item template ids", len(unlocks)),
		fmt.Sprintf("%d entries, %d distinct ids", entries, len(ids)),
	} {
		if !strings.Contains(p, want) {
			t.Errorf("PROVENANCE.md does not contain %q; update it with the data", want)
		}
	}
}

func TestVolume(t *testing.T) {
	// values the game's tables give, which the Dune Docker console's own measured values for its volume-tracked items also give (103 of its 106 agree)
	for id, want := range map[string]float64{"Mouse_Corpse": 5, "FuelCanister": 2, "AluminiumBar": 0.7, "AzuriteOre": 0.2, "Silicone": 0.1, "DuraluminumRod": 0.9, "T6ResourceB": 0.5, "WindTurbineLubricant2": 0.2, "T6Machinery": 0.1} {
		if got, ok := Volume(id); !ok || got != want {
			t.Errorf("%s: %v %v, want %v", id, got, ok, want)
		}
	}
	// an item the game lists with no volume is known to take none; an id the tables do not list, and one they list twice with two sizes, is unknown
	if v, ok := Volume("MTX_SardaukarFightingDummy_Patent"); !ok || v != 0 {
		t.Errorf("zero volume: %v %v", v, ok)
	}
	for _, id := range []string{"NotAnItem", "Thumper", "D_Turret", "Stilltent"} {
		if _, ok := Volume(id); ok {
			t.Errorf("%s must be unknown", id)
		}
	}
	if len(volumes) < 4000 {
		t.Errorf("only %d volumes", len(volumes))
	}
	for id, v := range volumes {
		if v < 0 || v > 1000 || id == "" {
			t.Errorf("odd entry %q = %v", id, v)
		}
	}
}

// PROVENANCE.md states how many volumes there are and how many are above zero; it must stay true.
func TestVolumesProvenanceSizesMatch(t *testing.T) {
	b, err := os.ReadFile("PROVENANCE.md")
	if err != nil {
		t.Fatal(err)
	}
	nonzero := 0
	for _, v := range volumes {
		if v > 0 {
			nonzero++
		}
	}
	text := strings.ReplaceAll(string(b), ",", "")
	for _, want := range []string{fmt.Sprintf("For %d item template ids the volume of one unit", len(volumes)), fmt.Sprintf("%d are above 0", nonzero)} {
		if !strings.Contains(text, want) {
			t.Errorf("PROVENANCE.md does not say %q", want)
		}
	}
}
