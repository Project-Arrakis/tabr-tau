package catalog

import (
	"regexp"
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
