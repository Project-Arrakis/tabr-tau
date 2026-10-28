package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #448: the Landsraad page showed SpecialVendorActive as enabled while the decree in effect was SpecialVendorActive_Vehicles. The save holds five special vendor rows, not
// one, and the pool flag (disabled) is not the same thing as "in effect". These are the 11 decree names and flags of a save read in the game (term 10: active decree 8,
// elected none; the rotation of that save held 1, 10 and 4).
const decreeWorld = `
delete from landsraad_decree_rotation;
delete from landsraad_decree_term;
delete from landsraad_decrees;
insert into landsraad_decrees(id, decree_name, disabled, weight) values
  (1, 'ExperienceRateIncrease', 1, 1.0), (2, 'RangedDamageIncreased', 1, 1.0), (3, 'MeleeDamageIncreased', 1, 1.0), (4, 'CraftingCostReduced', 1, 1.0),
  (5, 'DropInventoryOnDefeatActive', 0, 1.0), (6, 'RepairAndRefiningTimes', 1, 1.0), (7, 'SpecialVendorActive', 0, 1.0), (8, 'SpecialVendorActive_Vehicles', 1, 1.0),
  (9, 'SpecialVendorActive_Weapons', 1, 1.0), (10, 'SpecialVendorActive_Armor', 1, 1.0), (11, 'SpecialVendorActive_Utilities', 1, 1.0);
insert into landsraad_decree_term(term_id, start_time, end_time, active_decree_id) values (9, 0, 100, 6), (10, 100, 200, 8);
insert into landsraad_decree_rotation(decree_id) values (1), (10), (4);
`

func decreeRows(t *testing.T, o *Ops) map[string]map[string]any {
	t.Helper()
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for _, d := range res.(map[string]any)["decrees"].([]map[string]any) {
		out[d["decree_name"].(string)] = d
	}
	return out
}

func TestEverySpecialVendorDecreeIsItsOwnRowAndOnlyTheActiveOneIsInEffect(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, decreeWorld)}
	rows := decreeRows(t, o)
	if len(rows) != 11 {
		t.Fatalf("%d decrees, want the 11 of the save", len(rows))
	}
	variants := []string{"SpecialVendorActive", "SpecialVendorActive_Vehicles", "SpecialVendorActive_Weapons", "SpecialVendorActive_Armor", "SpecialVendorActive_Utilities"}
	wantID := map[string]int64{"SpecialVendorActive": 7, "SpecialVendorActive_Vehicles": 8, "SpecialVendorActive_Weapons": 9, "SpecialVendorActive_Armor": 10, "SpecialVendorActive_Utilities": 11}
	for _, n := range variants {
		d := rows[n]
		if d == nil {
			t.Fatalf("%s missing from the decree list", n)
		}
		if d["id"].(int64) != wantID[n] {
			t.Errorf("%s has id %v, want %d", n, d["id"], wantID[n])
		}
		if want := n == "SpecialVendorActive_Vehicles"; d["active"] != want {
			t.Errorf("%s active = %v, want %v (the term's active decree is id 8)", n, d["active"], want)
		}
		if d["elected"] != false {
			t.Errorf("%s elected = %v, the term has no elected decree", n, d["elected"])
		}
	}
	// the flag is returned as the save holds it: the decree in effect has disabled = 1 and the base row has 0, so the flag cannot be what says "in effect"
	if rows["SpecialVendorActive"]["disabled"].(int64) != 0 || rows["SpecialVendorActive_Vehicles"]["disabled"].(int64) != 1 {
		t.Errorf("the pool flags are not returned as stored: base %v, Vehicles %v", rows["SpecialVendorActive"]["disabled"], rows["SpecialVendorActive_Vehicles"]["disabled"])
	}
	inRot := map[string]bool{"ExperienceRateIncrease": true, "CraftingCostReduced": true, "SpecialVendorActive_Armor": true}
	for n, d := range rows {
		if d["in_rotation"] != inRot[n] {
			t.Errorf("%s in_rotation = %v, want %v", n, d["in_rotation"], inRot[n])
		}
		if (n == "SpecialVendorActive_Vehicles") != (d["active"] == true) {
			t.Errorf("%s active = %v", n, d["active"])
		}
	}
}

func TestOnlyTheCurrentTermsDecreesAreMarkedActiveAndElected(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, decreeWorld+`update landsraad_decree_term set elected_decree_id=9 where term_id=10;`)}
	rows := decreeRows(t, o)
	if rows["RepairAndRefiningTimes"]["active"] != false {
		t.Error("decree 6 was active in the earlier term 9, not in the current term 10")
	}
	if rows["SpecialVendorActive_Weapons"]["elected"] != true || rows["SpecialVendorActive_Weapons"]["active"] != false {
		t.Errorf("Weapons (9) is elected, not active: %v", rows["SpecialVendorActive_Weapons"])
	}
	if rows["SpecialVendorActive_Vehicles"]["active"] != true || rows["SpecialVendorActive_Vehicles"]["elected"] != false {
		t.Errorf("Vehicles (8) is active, not elected: %v", rows["SpecialVendorActive_Vehicles"])
	}
}

func TestATermWithoutAnActiveDecreeMarksNoneActive(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, decreeWorld+`update landsraad_decree_term set active_decree_id=null where term_id=10;`)}
	for n, d := range decreeRows(t, o) {
		if d["active"] != false || d["elected"] != false {
			t.Errorf("%s: active %v elected %v, the term has neither", n, d["active"], d["elected"])
		}
	}
}

func TestSetDecreeChangesOnlyTheRowWithThatIDNotTheOtherSpecialVendorVariants(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, decreeWorld)}
	if _, err := o.SetDecree(Args{"id": float64(7), "disabled": true}); err != nil {
		t.Fatal(err)
	}
	rows := decreeRows(t, o)
	if rows["SpecialVendorActive"]["disabled"].(int64) != 1 {
		t.Errorf("SpecialVendorActive (7) flag = %v, want 1", rows["SpecialVendorActive"]["disabled"])
	}
	for _, n := range []string{"SpecialVendorActive_Vehicles", "SpecialVendorActive_Weapons", "SpecialVendorActive_Armor", "SpecialVendorActive_Utilities"} {
		if rows[n]["disabled"].(int64) != 1 {
			t.Errorf("%s flag changed to %v by an edit of id 7", n, rows[n]["disabled"])
		}
	}
	if _, err := o.SetDecree(Args{"id": float64(8), "disabled": false}); err != nil {
		t.Fatal(err)
	}
	rows = decreeRows(t, o)
	if rows["SpecialVendorActive_Vehicles"]["disabled"].(int64) != 0 || rows["SpecialVendorActive"]["disabled"].(int64) != 1 || rows["SpecialVendorActive_Weapons"]["disabled"].(int64) != 1 {
		t.Errorf("an edit of id 8 reached another row: base %v, Vehicles %v, Weapons %v", rows["SpecialVendorActive"]["disabled"], rows["SpecialVendorActive_Vehicles"]["disabled"], rows["SpecialVendorActive_Weapons"]["disabled"])
	}
	if rows["SpecialVendorActive_Vehicles"]["active"] != true {
		t.Error("changing the flag must not change which decree the term has active")
	}
}
