package catalog

import (
	"os"
	"strings"
	"testing"
)

// #405: the catalog follows the console's item file. These tests name what the refresh added and the rules that settle the
// differences, so a later copy of the file cannot lose them without a test saying so.

// The item the console added in v1.4.52 is in the catalog, and it is an item of the game: its id is in the game's own
// item tables (volumes.json is read from them).
func TestTheIxianIconIsInTheCatalog(t *testing.T) {
	const id = "MTX_IxianTotem_Patent"
	if _, ok := Volume(id); !ok {
		t.Fatalf("%s is expected in the game's item tables (volumes.json); the test's premise is wrong", id)
	}
	e, ok := byID[id]
	if !ok {
		t.Fatalf("%s is in the game's item tables but not in the catalog", id)
	}
	if e.Name != "Ixian Icon" || e.Category != "buildings" || e.Source != "BuildingSets" {
		t.Errorf("%s: %+v", id, e)
	}
	if n, ok := MaxStack(id); !ok || n != 1 {
		t.Errorf("a building-set patent holds one per slot: %d %v", n, ok)
	}
}

// The seven ids the console lists twice (once as a placeable, once as a weapon) keep one entry. The rule: the first entry wins, and
// in the console's file the placeable comes first. Which of the two names the game itself shows for UniqueThumper_02 and _03 cannot be
// found out from the save or the item tables (they carry no names), so the placeable's name stays and the other is written down here.
func TestTheSevenDuplicatedIdsKeepTheirPlaceableEntry(t *testing.T) {
	want := map[string][2]string{ // id -> name, name the console's weapon entry gives
		"RespawnBeacon":       {"Respawn Beacon", "Respawn Beacon"},
		"Stilltent":           {"Stilltent", "Stilltent"},
		"Thumper":             {"Thumper", "Thumper"},
		"UniqueThumper":       {"Clapper Mk4", "Clapper Mk4"},
		"UniqueThumper_02":    {"Clapper Mk2", "Wormsong Echo"},
		"UniqueThumper_03":    {"Clapper Mk3", "Song of Shai-Hulud"},
		"Stilltent_Unique_01": {"Double-sealed Stilltent", "Double-sealed Stilltent"},
	}
	for id, names := range want {
		e, ok := byID[id]
		if !ok || e.Category != "placeables" || e.Name != names[0] {
			t.Errorf("%s: %+v %v, want the placeable %q", id, e, ok, names[0])
		}
	}
	for id := range want {
		n := 0
		for _, e := range entries {
			if e.ID == id {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s has %d entries, want 1", id, n)
		}
	}
}

// The console's group of an item (component, refined resource, raw resource) and the Steam DLC id of the eight Filmic Archive items.
func TestGroupAndSteamDlcAreCarried(t *testing.T) {
	for id, want := range map[string]string{
		"FremenComponent1": "component", "AluminiumBar": "refined_resource", "AzuriteOre": "raw_resource", "Mouse_Corpse": "raw_resource", "Oil": "raw_resource",
	} {
		if got := Group(id); got != want {
			t.Errorf("group of %s: %q, want %q", id, got, want)
		}
	}
	for _, id := range []string{"Literjon", "NotAnItem", "HighCapacityLiterjon_06"} {
		if g := Group(id); g != "" {
			t.Errorf("%s has no group, got %q", id, g)
		}
	}
	counts := map[string]int{}
	for _, e := range entries {
		if e.Group != "" {
			counts[e.Group]++
		}
	}
	if counts["component"] != 59 || counts["refined_resource"] != 21 || counts["raw_resource"] != 19 {
		t.Errorf("group sizes: %v, want 59 components, 21 refined, 19 raw resources", counts)
	}
	steam := 0
	for _, e := range entries {
		if e.RequiredSteamDLC != "" {
			steam++
			if e.RequiredDLC != "Filmic Archive" || e.RequiredSteamDLC != "4857580" {
				t.Errorf("%s: %+v", e.ID, e)
			}
		}
	}
	if steam != 8 {
		t.Errorf("%d items carry a Steam DLC id, want 8", steam)
	}
}

// The provenance file names the commit the catalog was copied from and says what the refresh did.
func TestProvenanceNamesTheCatalogCommitAndTheDuplicateRule(t *testing.T) {
	b, err := os.ReadFile("PROVENANCE.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ca3f4cd7", "c685d266", "MTX_IxianTotem_Patent", "first entry", "UniqueThumper_02", "`group`", "`requiredSteamDlc`"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("PROVENANCE.md does not say %q", want)
		}
	}
}
