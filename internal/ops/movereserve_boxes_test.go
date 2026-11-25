package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Move the Reserve into any number of boxes named DD-1 ... DD-6 (#1214). The world: the character's Reserve (inventory 12) and a base of the character's (totem 601) that holds the
// Developer storage containers each test asks for. Box k (the k-th of the list) has actor 700+k and inventory 7000+k.

type tbox struct {
	name string  // the name the person gave the box in the game
	cap  int     // slots (max_item_count)
	vol  float64 // volume limit; 0 = none recorded
}

// ddBoxes are boxes named DD-1 ... DD-n, each with the same slots and volume limit.
func ddBoxes(n, cap int, vol float64) []tbox {
	var b []tbox
	for i := 1; i <= n; i++ {
		b = append(b, tbox{fmt.Sprintf("DD-%d", i), cap, vol})
	}
	return b
}

// stoneUnit is the unit volume of Stone in the game's item table.
func stoneUnit(t *testing.T) float64 {
	t.Helper()
	v, ok := catalog.Volume("Stone")
	if !ok || v <= 0 {
		t.Fatal("the catalog has no volume for Stone: this test needs it")
	}
	return v
}

// boxesWorld is the world; stacks are the stack sizes of the Reserve, all of Stone, at positions 0, 1, 2 ... (items 8001, 8002 ...).
func boxesWorld(boxes []tbox, stacks ...int) string {
	var sb strings.Builder
	sb.WriteString(`
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (12,2,33,-1,0.0);
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0,0,0,1,0,0,10,1,jsonb('{}'));
insert into fgl_entities(entity_id,components) values (5764607523034234881, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,5764607523034234881,'Actor');
insert into totems(id) values (601);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (601,'My base',3,3,0,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values (601,5764607523034234881,'Totem_Placeable',1,0);
`)
	for i, b := range boxes {
		k := i + 1
		fmt.Fprintf(&sb, `insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values (%d,'/Game/BP_Box.BP_Box_C','HaggaBasin',5001,6000,7,0,0,0,1,0,0,5,1,jsonb('{}'));
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (%d,'%s',1,3,1,0);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values (%d,5764607523034234881,'Developer_StorageContainer_Placeable',1,0);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (%d,%d,4,%d,%g);
`, 700+k, 700+k, b.name, 700+k, 7000+k, 700+k, b.cap, b.vol)
	}
	for i, n := range stacks {
		fmt.Fprintf(&sb, "insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,12,%d,%d,'Stone',0,%d,'{}',0);\n", 8001+i, n, i, 1790000001+i)
	}
	return sb.String()
}

func boxOps(t *testing.T, boxes []tbox, stacks ...int) *Ops {
	t.Helper()
	stoneUnit(t)
	return &Ops{S: testsave.PlayerWithSQL(t, boxesWorld(boxes, stacks...))}
}

// inBox is the number of stacks in the k-th box of the world (1-based).
func inBox(t *testing.T, o *Ops, k int) int64 {
	t.Helper()
	return rmCount(t, o, `items where inventory_id=?`, 7000+k)
}

func boxNamesOf(r map[string]any) []string { return r["boxNames"].([]string) }

func summaryOf(r map[string]any) string { return strings.Join(r["summary"].([]string), "\n") }

// 1, 2, 3 and 6 boxes: with one slot in each and as many stacks as boxes, every box takes one stack, in order, and the answer names the boxes it found.
func TestMoveReserveFindsAsManyBoxesAsRunInARowFromDD1(t *testing.T) {
	for _, n := range []int{1, 2, 3, 6} {
		t.Run(fmt.Sprintf("%d boxes", n), func(t *testing.T) {
			stacks := make([]int, n)
			for i := range stacks {
				stacks[i] = 10 + i
			}
			o := boxOps(t, ddBoxes(n, 1, 0), stacks...)
			r := moveApply(t, o, nil, MoveReserveOptions{})
			if r["autoDetected"] != true || len(boxNamesOf(r)) != n || boxNamesOf(r)[n-1] != fmt.Sprintf("DD-%d", n) {
				t.Fatalf("found %v", r["boxNames"])
			}
			for k := 1; k <= n; k++ {
				if inBox(t, o, k) != 1 {
					t.Fatalf("box DD-%d holds %d stacks, want 1", k, inBox(t, o, k))
				}
				// the k-th stack of the Reserve (item 8000+k) went into box k
				if rmCount(t, o, `items where inventory_id=? and id=?`, 7000+k, 8000+k) != 1 {
					t.Fatalf("stack %d is not in DD-%d", 8000+k, k)
				}
			}
			if rmCount(t, o, `items where inventory_id=12`) != 0 {
				t.Fatal("the Reserve is not empty")
			}
			if !strings.Contains(summaryOf(r), fmt.Sprintf("found %d: DD-1", n)) {
				t.Fatalf("the answer does not say what was found:\n%s", summaryOf(r))
			}
			if len(r["ignoredBoxes"].([]string)) != 0 {
				t.Fatalf("ignored: %v", r["ignoredBoxes"])
			}
		})
	}
}

// A missing number ends the row: DD-4 is ignored when DD-3 is missing, and the answer says so, in the preview and when it does not fit.
func TestMoveReserveStopsAtTheFirstMissingBoxAndSaysWhatItIgnored(t *testing.T) {
	boxes := []tbox{{"DD-1", 1, 0}, {"DD-2", 1, 0}, {"DD-4", 1000, 0}}
	t.Run("the preview says it", func(t *testing.T) {
		o := boxOps(t, boxes, 5, 6)
		r := moveApply(t, o, nil, MoveReserveOptions{})
		if fmt.Sprint(boxNamesOf(r)) != "[DD-1 DD-2]" || fmt.Sprint(r["ignoredBoxes"]) != "[DD-4]" {
			t.Fatalf("found %v ignored %v", r["boxNames"], r["ignoredBoxes"])
		}
		s := summaryOf(r)
		for _, w := range []string{"found 2: DD-1, DD-2", "Ignored: DD-4", "DD-3 is missing"} {
			if !strings.Contains(s, w) {
				t.Errorf("the answer lacks %q:\n%s", w, s)
			}
		}
		if inBox(t, o, 3) != 0 {
			t.Fatal("DD-4 was used although DD-3 is missing") // the third box of the world is the one named DD-4
		}
	})
	t.Run("a Reserve that needs the ignored box is refused and the refusal names it", func(t *testing.T) {
		o := boxOps(t, boxes, 5, 6, 7)
		before := itemsSnapshot(t, o)
		_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
		for _, w := range []string{"add DD-3", "Ignored: DD-4", "DD-3 is missing", `"DD-1" has 1 slots free`, `"DD-2" has 1 slots free`} {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("the refusal lacks %q: %v", w, err)
			}
		}
		if itemsSnapshot(t, o) != before {
			t.Fatal("a refusal changed the save")
		}
	})
}

// DD-1 is the minimum, and the refusal says what to do.
func TestMoveReserveNeedsDD1(t *testing.T) {
	o := boxOps(t, []tbox{{"DD-2", 10, 0}, {"DD-3", 10, 0}}, 1)
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrNotFound, "No Developer storage box named DD-1")
	if !strings.Contains(err.Error(), `"DD-2" (1)`) || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("%v", err)
	}
	if itemsSnapshot(t, o) != before {
		t.Fatal("a refusal changed the save")
	}
	// the editor's line says the same in plain words
	d, err := o.DetectReserveBoxes()
	if err != nil || !strings.Contains(d.(map[string]any)["text"].(string), "No Developer storage box named DD-1") || len(d.(map[string]any)["found"].([]string)) != 0 {
		t.Fatalf("%v %v", d, err)
	}
}

// The editor's line: what was found, and what was ignored.
func TestDetectReserveBoxesSaysWhatItFoundAndIgnored(t *testing.T) {
	o := boxOps(t, []tbox{{"DD-1", 10, 0}, {"DD-2", 10, 0}, {"DD-3", 10, 0}, {"DD-5", 10, 0}}, 1)
	d, err := o.DetectReserveBoxes()
	if err != nil {
		t.Fatal(err)
	}
	m := d.(map[string]any)
	txt := m["text"].(string)
	if fmt.Sprint(m["found"]) != "[DD-1 DD-2 DD-3]" || fmt.Sprint(m["ignored"]) != "[DD-5]" || m["gap"] != "DD-4" || m["max"] != MaxReserveBoxes {
		t.Fatalf("%v", m)
	}
	for _, w := range []string{"Storage boxes: DD-1 ... DD-6", "found 3: DD-1, DD-2, DD-3", "Ignored: DD-5", "DD-4 is missing"} {
		if !strings.Contains(txt, w) {
			t.Errorf("the line lacks %q: %s", w, txt)
		}
	}
	if len(o.S.Ops()) != 0 {
		t.Fatal("the detection wrote something")
	}
}

// Two boxes with the same name: refused, naming both, never one of them guessed.
func TestMoveReserveRefusesTwoBoxesWithTheSameName(t *testing.T) {
	o := boxOps(t, []tbox{{"DD-1", 10, 0}, {"DD-2", 10, 0}, {"DD-2", 10, 0}}, 1)
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, `2 Developer storage containers are named "DD-2"`)
	if !strings.Contains(err.Error(), "number 702") || !strings.Contains(err.Error(), "number 703") {
		t.Fatalf("the refusal does not name both boxes: %v", err)
	}
	if itemsSnapshot(t, o) != before {
		t.Fatal("a refusal changed the save")
	}
}

// A box that cannot be used is refused by name, not skipped: the later boxes are not used instead.
func TestMoveReserveRefusesAFoundBoxWithoutASlotCountInsteadOfSkippingIt(t *testing.T) {
	o := boxOps(t, []tbox{{"DD-1", 10, 0}, {"DD-2", -1, 0}, {"DD-3", 10, 0}}, 1)
	_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, `The box "DD-2"`)
	if !strings.Contains(err.Error(), "no slot count above 0") {
		t.Fatal(err)
	}
}

// A stack that fits only in a later box goes there, and the next stack still goes into the first box that has room (first fit, whole stacks).
func TestMoveReserveAStackThatFitsOnlyInALaterBox(t *testing.T) {
	u := stoneUnit(t)
	o := boxOps(t, []tbox{{"DD-1", 1000, 5 * u}, {"DD-2", 1000, 0}, {"DD-3", 1000, 0}}, 10, 3)
	r := moveApply(t, o, nil, MoveReserveOptions{})
	// 10 Stone is 10u by volume: DD-1 holds 5u at most, so it goes to DD-2; 3 Stone (3u) fits DD-1
	if rmCount(t, o, `items where inventory_id=7002 and id=8001`) != 1 || rmCount(t, o, `items where inventory_id=7001 and id=8002`) != 1 || inBox(t, o, 3) != 0 {
		t.Fatalf("%v", r["boxes"])
	}
}

// Volume-limited fill across three boxes: each takes one stack of 3u under a limit of 5u; a fourth stack is short by volume, and the refusal says so and says "add DD-4".
func TestMoveReserveVolumeLimitedFillAcrossBoxes(t *testing.T) {
	u := stoneUnit(t)
	boxes := ddBoxes(3, 1000, 5*u)
	t.Run("three stacks fit one per box", func(t *testing.T) {
		o := boxOps(t, boxes, 3, 3, 3)
		moveApply(t, o, nil, MoveReserveOptions{})
		for k := 1; k <= 3; k++ {
			if inBox(t, o, k) != 1 {
				t.Fatalf("box DD-%d holds %d, want 1 (a volume check that is missing puts them all in DD-1)", k, inBox(t, o, k))
			}
		}
	})
	t.Run("a fourth is short and the refusal says add DD-4", func(t *testing.T) {
		o := boxOps(t, boxes, 3, 3, 3, 3)
		before := itemsSnapshot(t, o)
		_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
		for _, w := range []string{"Needed: 4 stacks", fmt.Sprintf("(volume %s) that fit in none of the boxes", fmtVol(3*u)), "add DD-4", `"DD-3" has 1,000 slots free and ` + fmtVol(5*u) + " volume free"} {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("the refusal lacks %q: %v", w, err)
			}
		}
		if strings.Contains(err.Error(), "All six boxes") || itemsSnapshot(t, o) != before {
			t.Fatalf("%v", err)
		}
	})
}

// Short by stacks: the refusal for two boxes says add DD-3, and for six boxes says all six are in use (no DD-7).
func TestMoveReserveShortMessageNamesTheNextBoxOrSaysAllSixAreUsed(t *testing.T) {
	t.Run("two boxes", func(t *testing.T) {
		o := boxOps(t, ddBoxes(2, 1, 0), 1, 1, 1)
		_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
		if !strings.Contains(err.Error(), "add DD-3") || !strings.Contains(err.Error(), "name it DD-3") || !strings.Contains(err.Error(), "Needed: 3 stacks") {
			t.Fatalf("%v", err)
		}
		if !IsMoveNoRoom(err) {
			t.Fatal("the refusal must be the no-room kind")
		}
	})
	t.Run("six boxes", func(t *testing.T) {
		o := boxOps(t, ddBoxes(6, 1, 0), 1, 1, 1, 1, 1, 1, 1)
		_, err := o.PreviewMoveReserve(nil, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "All six boxes (DD-1 to DD-6) are in use")
		if strings.Contains(err.Error(), "DD-7") || !strings.Contains(err.Error(), `"DD-6" has 1 slots free`) {
			t.Fatalf("%v", err)
		}
	})
}

// An explicit list of names overrides the scan: its order is the order of use, a box outside the list is not touched, and more than six names are refused.
func TestMoveReserveExplicitNamesOverrideTheScan(t *testing.T) {
	o := boxOps(t, ddBoxes(3, 1000, 0), 1, 2, 3)
	r := moveApply(t, o, []string{"DD-3", "DD-1"}, MoveReserveOptions{})
	if r["autoDetected"] != false || fmt.Sprint(boxNamesOf(r)) != "[DD-3 DD-1]" {
		t.Fatalf("%v", r["boxNames"])
	}
	if inBox(t, o, 3) != 3 || inBox(t, o, 1) != 0 || inBox(t, o, 2) != 0 {
		t.Fatalf("DD-3 %d DD-1 %d DD-2 %d", inBox(t, o, 3), inBox(t, o, 1), inBox(t, o, 2))
	}
	if strings.Contains(summaryOf(r), "Storage boxes:") {
		t.Fatal("an explicit list says nothing of a scan")
	}
	o2 := boxOps(t, ddBoxes(3, 1000, 0), 1)
	_, err := o2.PreviewMoveReserve([]string{"a", "b", "c", "d", "e", "f", "g"}, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrBadArgument, "at most 6")
}

// The write path with no names: the fingerprint the preview gave is the one the write needs, and a change of the boxes between them refuses the write.
func TestMoveReserveAutoWriteNeedsTheFingerprintOfTheBoxesItSaw(t *testing.T) {
	o := boxOps(t, ddBoxes(3, 1, 0), 1, 2, 3)
	pv := movePreview(t, o, nil, MoveReserveOptions{})
	// DD-4 appears between the question and the write: the boxes are not the ones the question showed
	if _, err := o.S.Mutate("add a box", func(m *save.Mut) error {
		for _, q := range []string{
			`insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values (790,'/Game/BP_Box.BP_Box_C','HaggaBasin',5001,6000,7,0,0,0,1,0,0,5,1,jsonb('{}'))`,
			`insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (790,'DD-4',1,3,1,0)`,
			`insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values (790,5764607523034234881,'Developer_StorageContainer_Placeable',1,0)`,
			`insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (7790,790,4,10,0)`} {
			if _, err := m.Exec(q); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_, err := o.MoveReserve(nil, pv["fingerprint"].(string), MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, "have changed since the question was shown")
	if rmCount(t, o, `items where inventory_id=12`) != 3 {
		t.Fatal("a refused write moved something")
	}
}

// The read back with three boxes: a stack that comes back to the Reserve, or one that is in the wrong box, fails the whole edit and rolls everything back.
func TestMoveReserveReadBackThatDiffersWithThreeBoxesRollsEverythingBack(t *testing.T) {
	for name, tamper := range map[string]string{
		"a stack came back to the Reserve":       `update items set inventory_id=12 where id=8003`,
		"a stack of the third box is in the 1st": `update items set inventory_id=7001, position_index=9 where id=8003`,
		"a stack of the third box was deleted":   `delete from items where id=8003`,
		"a column other than the place changed":  `update items set stack_size=stack_size+1 where id=8002`,
	} {
		t.Run(name, func(t *testing.T) {
			o := boxOps(t, ddBoxes(3, 1, 0), 1, 2, 3)
			pv := movePreview(t, o, nil, MoveReserveOptions{})
			before := itemsSnapshot(t, o)
			moveReserveBeforeReadBack = func(m *save.Mut) {
				if _, err := m.Exec(tamper); err != nil {
					t.Errorf("tamper: %v", err)
				}
			}
			defer func() { moveReserveBeforeReadBack = nil }()
			_, err := o.MoveReserve(nil, pv["fingerprint"].(string), MoveReserveOptions{})
			if err == nil || !strings.Contains(err.Error(), "Nothing was changed") {
				t.Fatalf("a read back that differs must refuse: %v", err)
			}
			if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
				t.Fatal("a failed read back must roll the whole edit back")
			}
		})
	}
}

// dumpExceptPlace hashes every row of every table; for the items table the inventory and the position of a row are left out, so that two saves have the same dump when nothing but
// the place of a row differs.
func dumpExceptPlace(t *testing.T, s *save.Save) string {
	t.Helper()
	tabs, err := s.Query(`select name from sqlite_master where type='table' order by name`)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	for _, tb := range tabs {
		name := tb["name"].(string)
		rows, err := s.Query(`select * from "` + name + `"`)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s %d\n", name, len(rows))
		for _, r := range rows {
			keys := make([]string, 0, len(r))
			for k := range r {
				if name == "items" && (k == "inventory_id" || k == "position_index") {
					continue
				}
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(h, "%s=%v;", k, r[k])
			}
			h.Write([]byte{'\n'})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Whole-save equality: a move into four boxes changes nothing but the inventory and the position of the Reserve's rows.
func TestMoveReserveChangesNothingOutsideTheMovedRows(t *testing.T) {
	o := boxOps(t, ddBoxes(4, 2, 0), 5, 6, 7, 8, 9, 10, 11)
	before := dumpExceptPlace(t, o.S)
	whole := dumpSave(t, o.S)
	r := moveApply(t, o, nil, MoveReserveOptions{})
	if r["changed"] != true || rmCount(t, o, `items where inventory_id=12`) != 0 {
		t.Fatalf("%v", r)
	}
	if got := dumpExceptPlace(t, o.S); got != before {
		t.Fatal("something besides the place of the moved rows changed")
	}
	if dumpSave(t, o.S) == whole {
		t.Fatal("the rows did not move")
	}
	// the boxes hold 2+2+2+1 stacks, in order
	if inBox(t, o, 1) != 2 || inBox(t, o, 2) != 2 || inBox(t, o, 3) != 2 || inBox(t, o, 4) != 1 {
		t.Fatalf("%d %d %d %d", inBox(t, o, 1), inBox(t, o, 2), inBox(t, o, 3), inBox(t, o, 4))
	}
	// a preview leaves the save as it was
	o2 := boxOps(t, ddBoxes(4, 2, 0), 5, 6, 7)
	w := dumpSave(t, o2.S)
	movePreview(t, o2, nil, MoveReserveOptions{})
	if dumpSave(t, o2.S) != w {
		t.Fatal("a preview changed the save")
	}
}

// An empty Reserve is not an error: nothing is written, and the answer still says which boxes were found.
func TestMoveReserveEmptyReserveStillSaysWhichBoxesWereFound(t *testing.T) {
	o := boxOps(t, ddBoxes(3, 10, 0))
	r := movePreview(t, o, nil, MoveReserveOptions{})
	if r["nothingToDo"] != true || !strings.Contains(summaryOf(r), "found 3: DD-1, DD-2, DD-3") || !strings.Contains(summaryOf(r), "The Reserve is empty") || !strings.Contains(summaryOf(r), `Box "DD-3" (in "My base"): 0 stacks there; 10 slots free.`) {
		t.Fatalf("%v", r)
	}
}
