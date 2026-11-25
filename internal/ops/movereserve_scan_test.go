package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The scan of the boxes DD-1 ... DD-6 (#1214), held to what the docs and the changelog say: a container with such a name that cannot be used is REFUSED BY NAME, never skipped; a name is
// trimmed and matched exactly; a missing number lists the Developer storage boxes that have other names; DD-1 alone carries a warning.

// boxOpsExtra is boxOps with more SQL after the world (a container of another kind, a changed name ...).
func boxOpsExtra(t *testing.T, extra string, boxes []tbox, stacks ...int) *Ops {
	t.Helper()
	stoneUnit(t)
	return &Ops{S: testsave.PlayerWithSQL(t, boxesWorld(boxes, stacks...)+extra)}
}

// otherContainer is a container of another kind than a Developer storage box (a chest), named name, standing in the base 601, with an inventory.
func otherContainer(id int, name, buildingType string) string {
	return fmt.Sprintf(`insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values (%d,'/Game/BP_Chest.BP_Chest_C','HaggaBasin',5002,6000,7,0,0,0,1,0,0,5,1,jsonb('{}'));
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (%d,'%s',1,3,1,0);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values (%d,5764607523034234881,'%s',1,0);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (%d,%d,4,10,0.0);
`, id, id, name, id, buildingType, 9000+id, id)
}

// A lamp (no inventory) with a box name is not a container: it is no box, so the number is a gap, as before.
func aLamp(id int, name string) string {
	return fmt.Sprintf(`insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values (%d,'/Game/BP_Lamp.BP_Lamp_C','HaggaBasin',5003,6000,7,0,0,0,1,0,0,5,1,jsonb('{}'));
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (%d,'%s',1,3,1,0);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values (%d,5764607523034234881,'Lamp_Placeable',1,0);
`, id, id, name, id)
}

func detectOf(t *testing.T, o *Ops) map[string]any {
	t.Helper()
	d, err := o.DetectReserveBoxes()
	if err != nil {
		t.Fatal(err)
	}
	return d.(map[string]any)
}

// Finding 1: a container of ANOTHER kind named DD-3, with a Developer box named DD-4 behind it: the move is refused by name and DD-4 is not used in its place.
func TestAContainerOfAnotherKindNamedDD3IsRefusedByNameNotSkipped(t *testing.T) {
	boxes := []tbox{{"DD-1", 10, 0}, {"DD-2", 10, 0}, {"DD-4", 1000, 0}}
	o := boxOpsExtra(t, otherContainer(800, "DD-3", "Storage_Placeable"), boxes, 1, 2)
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, `The container named "DD-3" is a "Storage_Placeable" container, not a Developer storage container`)
	if !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatal(err)
	}
	if itemsSnapshot(t, o) != before || inBox(t, o, 3) != 0 {
		t.Fatal("DD-4 was used in place of the refused DD-3, or a refusal changed the save")
	}
	// the write is refused the same way
	if _, err := o.MoveReserve(nil, "x", MoveReserveOptions{}); err == nil {
		t.Fatal("the write must not go through")
	}
	// the editor's line names it instead of counting it
	d := detectOf(t, o)
	txt := d["text"].(string)
	if fmt.Sprint(d["found"]) != "[DD-1 DD-2 DD-4]" || len(d["unusable"].([]ReserveBoxProblem)) != 1 || !strings.Contains(txt, `DD-3 cannot be used: it is a "Storage_Placeable" container`) {
		t.Fatalf("%v", d)
	}
	if strings.Contains(txt, "found 4") {
		t.Fatalf("the unusable box is counted: %s", txt)
	}
	// the same chest with a Developer box of the same name beside it: the name does not find one box
	o2 := boxOpsExtra(t, otherContainer(800, "DD-2", "Storage_Placeable"), boxes, 1)
	_, err = o2.PreviewMoveReserve(nil, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, `The container named "DD-2" has the same name as a "Storage_Placeable" container`)
	// a lamp (no inventory) is no container: its number is a gap, and what stands behind it is ignored
	o3 := boxOpsExtra(t, aLamp(801, "DD-3"), boxes, 1)
	d3 := detectOf(t, o3)
	if fmt.Sprint(d3["found"]) != "[DD-1 DD-2]" || d3["gap"] != "DD-3" || fmt.Sprint(d3["ignored"]) != "[DD-4]" {
		t.Fatalf("%v", d3)
	}
}

// The name is trimmed (spaces, tabs, new lines) and matched exactly: DD-10, DD-0, dd-3, D-3, DD-03 and "DD-2x" are not DD-n.
func TestTheScanTrimsTheNameAndMatchesItExactly(t *testing.T) {
	t.Run("a padded name is found", func(t *testing.T) {
		o := boxOpsExtra(t, "update permission_actor set actor_name=char(32,9)||'DD-2'||char(10,32) where actor_id=702;", ddBoxes(2, 10, 0), 1)
		d := detectOf(t, o)
		if fmt.Sprint(d["found"]) != "[DD-1 DD-2]" {
			t.Fatalf("%v", d)
		}
		r := moveApply(t, o, nil, MoveReserveOptions{})
		if fmt.Sprint(boxNamesOf(r)) != "[DD-1 DD-2]" {
			t.Fatalf("%v", r["boxNames"])
		}
	})
	t.Run("names that are not DD-1 ... DD-6", func(t *testing.T) {
		boxes := []tbox{{"DD-1", 10, 0}, {"DD-10", 10, 0}, {"DD-0", 10, 0}, {"dd-3", 10, 0}, {"D-3", 10, 0}, {"DD-03", 10, 0}, {"DD-2x", 10, 0}, {"DD-7", 10, 0}}
		o := boxOps(t, boxes, 1)
		s, err := scanReserveBoxes(o.S)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(s.Found) != "[DD-1]" || s.Gap != "DD-2" || len(s.Ignored) != 0 {
			t.Fatalf("found %v gap %q ignored %v", s.Found, s.Gap, s.Ignored)
		}
		// each of them is listed as a Developer storage box with another name, with the typos
		h := s.Hint()
		for _, w := range []string{`"DD-10"`, `"DD-0"`, `"dd-3"`, `"D-3"`, `"DD-03"`, `"DD-2x"`, `"DD-7"`, "Developer storage boxes with other names"} {
			if !strings.Contains(h, w) {
				t.Errorf("the hint lacks %s: %s", w, h)
			}
		}
		if strings.Contains(h, `"DD-1"`) {
			t.Fatalf("a box that was found is listed as another name: %s", h)
		}
	})
}

// Two boxes named DD-1: refused, naming both.
func TestTwoBoxesNamedDD1AreRefusedNamingBoth(t *testing.T) {
	o := boxOps(t, []tbox{{"DD-1", 10, 0}, {"DD-1", 10, 0}, {"DD-2", 10, 0}}, 1)
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, `2 Developer storage containers are named "DD-1"`)
	if !strings.Contains(err.Error(), "number 701") || !strings.Contains(err.Error(), "number 702") || itemsSnapshot(t, o) != before || inBox(t, o, 3) != 0 {
		t.Fatalf("%v", err)
	}
	d := detectOf(t, o)
	if fmt.Sprint(d["found"]) != "[DD-2]" || !strings.Contains(d["text"].(string), "DD-1 cannot be used") {
		t.Fatalf("%v", d)
	}
}

// Finding 1 and 5: a scanned box that stands where a move would refuse it is refused BY NAME (the box behind it is not used instead), and the editor's line does not count it.
func TestAScannedBoxThatCannotBeUsedIsRefusedByNameAndNotCounted(t *testing.T) {
	for _, c := range []struct{ name, sql, why string }{
		{"on the Deep Desert", "update actors set map='DeepDesert_1' where id=702;", "is on the Deep Desert"},
		{"a hologram", "update placeables set is_hologram=1 where id=702;", "is only a hologram"},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := boxOpsExtra(t, c.sql, ddBoxes(3, 10, 0), 1)
			before := itemsSnapshot(t, o)
			_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
			refusedWith(t, err, apierr.ErrConflict, `The box "DD-2"`)
			if !strings.Contains(err.Error(), c.why) || itemsSnapshot(t, o) != before || inBox(t, o, 3) != 0 {
				t.Fatalf("%v", err)
			}
			d := detectOf(t, o)
			txt := d["text"].(string)
			if fmt.Sprint(d["found"]) != "[DD-1 DD-3]" || !strings.Contains(txt, "found 2 usable: DD-1, DD-3") || !strings.Contains(txt, "DD-2 cannot be used: ") || !strings.Contains(txt, c.why) {
				t.Fatalf("%v", d)
			}
		})
	}
	t.Run("not owned by the character", func(t *testing.T) {
		o := boxOpsExtra(t, "update permission_actor_rank set player_id=2 where permission_actor_id=601;", ddBoxes(3, 10, 0), 1)
		before := itemsSnapshot(t, o)
		_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrConflict, `The box "DD-1"`)
		if !strings.Contains(err.Error(), "is not yours") || itemsSnapshot(t, o) != before {
			t.Fatalf("%v", err)
		}
		d := detectOf(t, o)
		if fmt.Sprint(d["found"]) != "[]" || len(d["unusable"].([]ReserveBoxProblem)) != 3 || !strings.Contains(d["text"].(string), "found 0 usable") {
			t.Fatalf("%v", d)
		}
	})
	t.Run("in a base of the run", func(t *testing.T) {
		o := boxOps(t, ddBoxes(3, 10, 0), 1)
		before := itemsSnapshot(t, o)
		_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{ExcludeBases: []int64{601}})
		refusedWith(t, err, apierr.ErrConflict, `The box "DD-1"`)
		if !strings.Contains(err.Error(), "which is being recycled") || itemsSnapshot(t, o) != before {
			t.Fatalf("%v", err)
		}
	})
}

// Finding 2: DD-1 alone says so, and stops nothing. Two boxes do not carry the warning.
func TestOnlyDD1CarriesAWarningThatStopsNothing(t *testing.T) {
	const warn = "only DD-1 was found; DD-2 .. DD-6 are optional"
	o := boxOpsExtra(t, "update permission_actor set actor_name='DD2' where actor_id=702;", ddBoxes(2, 10, 0), 1)
	if d := detectOf(t, o); !strings.Contains(d["text"].(string), warn) || fmt.Sprint(d["found"]) != "[DD-1]" {
		t.Fatalf("the editor's line: %v", d)
	}
	pv := movePreview(t, o, nil, MoveReserveOptions{})
	if !strings.Contains(summaryOf(pv), warn) {
		t.Fatalf("the preview lacks the warning:\n%s", summaryOf(pv))
	}
	r := moveApply(t, o, nil, MoveReserveOptions{})
	if r["changed"] != true || inBox(t, o, 1) != 1 || !strings.Contains(summaryOf(r), warn) {
		t.Fatalf("DD-1 alone must still be used: %v", r)
	}
	// an empty Reserve says it too
	o2 := boxOpsExtra(t, "update permission_actor set actor_name='DD2' where actor_id=702;", ddBoxes(2, 10, 0))
	if r2 := movePreview(t, o2, nil, MoveReserveOptions{}); !strings.Contains(summaryOf(r2), warn) {
		t.Fatalf("%s", summaryOf(r2))
	}
	// two boxes: no warning
	o3 := boxOps(t, ddBoxes(2, 10, 0), 1)
	if d := detectOf(t, o3); strings.Contains(d["text"].(string), "only DD-1") {
		t.Fatalf("%v", d)
	}
}

// Finding 3: a missing number lists the Developer storage boxes with other names (the typo), in the editor's line and in the refusal of a Reserve that does not fit.
func TestAMissingNumberListsTheDeveloperBoxesWithOtherNames(t *testing.T) {
	const hint = `Developer storage boxes with other names: "D-3", "dd-3"`
	boxes := []tbox{{"DD-1", 1, 0}, {"DD-2", 1, 0}, {"D-3", 10, 0}, {"dd-3", 10, 0}}
	o := boxOps(t, boxes, 1, 1, 1)
	_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrNoRoom, "add DD-3")
	if !strings.Contains(err.Error(), hint) {
		t.Fatalf("the shortfall lacks the hint: %v", err)
	}
	if txt := detectOf(t, o)["text"].(string); !strings.Contains(txt, hint) {
		t.Fatalf("the gap wording lacks the hint: %s", txt)
	}
	// DD-03 is no DD-3 either
	o2 := boxOps(t, []tbox{{"DD-1", 1, 0}, {"DD-2", 1, 0}, {"DD-03", 10, 0}}, 1, 1, 1)
	if _, err := o2.PreviewMoveReserve(nil, MoveReserveOptions{}); err == nil || !strings.Contains(err.Error(), `"DD-03"`) {
		t.Fatalf("%v", err)
	}
	// DD-1 missing: the line lists them too
	o3 := boxOps(t, []tbox{{"D-1", 10, 0}}, 1)
	if txt := detectOf(t, o3)["text"].(string); !strings.Contains(txt, "No Developer storage box named DD-1") || !strings.Contains(txt, `with other names: "D-1"`) {
		t.Fatalf("%s", txt)
	}
	// all six found: no hint, and nothing listed twice
	o4 := boxOps(t, append(ddBoxes(6, 10, 0), tbox{"Dump", 10, 0}), 1)
	if txt := detectOf(t, o4)["text"].(string); strings.Contains(txt, "other names") {
		t.Fatalf("%s", txt)
	}
	// the list is capped, and says how many were left out
	many := []tbox{{"DD-1", 10, 0}}
	for i := 0; i < 33; i++ {
		many = append(many, tbox{fmt.Sprintf("Box %02d", i), 10, 0})
	}
	txt := detectOf(t, boxOps(t, many, 1))["text"].(string)
	if !strings.Contains(txt, "and 3 more") || strings.Contains(txt, `"Box 32"`) {
		t.Fatalf("%s", txt)
	}
}
