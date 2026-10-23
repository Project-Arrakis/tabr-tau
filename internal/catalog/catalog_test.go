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
