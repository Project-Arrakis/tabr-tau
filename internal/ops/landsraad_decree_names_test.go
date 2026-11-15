package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #608: every decree id the editor knows has the name the game shows, and an id it does not know still comes through.
func TestEveryKnownDecreeIdHasAPlayerFacingName(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, decreeWorld)}
	rows := decreeRows(t, o)
	if len(rows) != 11 {
		t.Fatalf("the test world has %d decrees, want 11", len(rows))
	}
	for id, d := range rows {
		name, _ := d["display_name"].(string)
		if name == "" {
			t.Errorf("%s has no display_name", id)
		}
		if name == id || !strings.Contains(name, " ") && id != "SpecialVendorActive" {
			t.Errorf("%s: display_name %q is the internal id, not a name", id, name)
		}
	}
	for id := range decreeNames {
		if n, _, _ := decreeDisplay(id); n == "" {
			t.Errorf("decree-names.json has an empty name for %s", id)
		}
	}
}

func TestDecreeNamesAreTheGamesOwn(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, decreeWorld)}
	rows := decreeRows(t, o)
	want := map[string]string{
		"ExperienceRateIncrease":        "Scholarly Collaboration",
		"RangedDamageIncreased":         "In Honor of the Fallen (Ranged)",
		"MeleeDamageIncreased":          "In Honor of the Fallen (Melee)",
		"CraftingCostReduced":           "CHOAM Production Contract",
		"DropInventoryOnDefeatActive":   "The Right of Salvage",
		"RepairAndRefiningTimes":        "CHOAM Refining Contract",
		"SpecialVendorActive_Vehicles":  "Exclusive Vehicle Import Rights",
		"SpecialVendorActive_Weapons":   "Exclusive Weapon Import Rights",
		"SpecialVendorActive_Armor":     "Exclusive Armor Import Rights",
		"SpecialVendorActive_Utilities": "Exclusive Tool Import Rights",
	}
	for id, w := range want {
		d := rows[id]
		if d["display_name"] != w || d["name_known"] != true {
			t.Errorf("%s: display_name %v known %v, want %q known", id, d["display_name"], d["name_known"], w)
		}
		if e, _ := d["effect"].(string); e == "" {
			t.Errorf("%s has no effect line", id)
		}
	}
	// the plain row has no row in the game's table: the editor's own wording, marked as not the game's
	p := rows["SpecialVendorActive"]
	if p["display_name"] != "Special Vendor" || p["name_known"] != false {
		t.Errorf("SpecialVendorActive: %v known %v", p["display_name"], p["name_known"])
	}
	if rows["RangedDamageIncreased"]["display_name"] == rows["MeleeDamageIncreased"]["display_name"] {
		t.Error("the Ranged and Melee decrees share one game name and must be told apart")
	}
}

func TestAnUnknownDecreeIdStillComesThroughWithoutAName(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, decreeWorld+`insert into landsraad_decrees(id, decree_name, disabled, weight) values (99, 'SomeNewDecreeFromAPatch', 0, 1.0);`)}
	d := decreeRows(t, o)["SomeNewDecreeFromAPatch"]
	if d == nil {
		t.Fatal("the unknown decree is missing from the read")
	}
	if d["display_name"] != "" || d["name_known"] != false || d["decree_name"] != "SomeNewDecreeFromAPatch" {
		t.Errorf("unknown decree: %v", d)
	}
}
