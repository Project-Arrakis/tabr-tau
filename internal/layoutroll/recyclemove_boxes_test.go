package layoutroll

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The move of the Reserve into any number of boxes DD-1 ... DD-6 between the bases of a run (#1214), on the made-up saves of recyclemove_test.go. Every recycled base brings 3 stacks to
// the Reserve in these worlds.

// rcExtraBox is one more Developer storage container of the operator's, in the same base as DD-1 and DD-2 (rcBoxes): actor 3000+k, inventory 7300+k, no volume limit.
func rcExtraBox(k int, name string, capacity int) string {
	const et = 5764607523034230050 // the entity of base 5's totem (rcBase)
	return fmt.Sprintf(`
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (%[1]d,'/Game/BP_Box.BP_Box_C','HaggaBasin',5001,6000,7,0,0,0,1,0,0,5,1,jsonb('{}'));
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (%[1]d,'%[2]s',1,3,1,0);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values (%[1]d,%[3]d,'Developer_StorageContainer_Placeable',1,0);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (%[4]d,%[1]d,4,%[5]d,0.0);
`, 3000+k, name, et, 7300+k, capacity)
}

// Three bases bring nine stacks; boxes of three slots take three each, so the third base needs DD-3. With DD-1 and DD-2 only, the Reserve of the third base cannot be moved.
func TestThreeBasesNeedTheThirdBox(t *testing.T) {
	t.Run("with DD-3 every base is recycled and every box holds three stacks", func(t *testing.T) {
		e, o := mvEnv(t, rcWorld(ddBases(3)...)+rcBoxes(3, 3)+rcExtraBox(3, "DD-3", 3))
		r, err := Roll(o)
		if err != nil || !r.Wrote || e.ini() == iniWith("11") {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		if inInv(t, e.save, 12) != 0 || inInv(t, e.save, 7301) != 3 || inInv(t, e.save, 7302) != 3 || inInv(t, e.save, 7303) != 3 {
			t.Fatalf("Reserve %d, DD-1 %d, DD-2 %d, DD-3 %d", inInv(t, e.save, 12), inInv(t, e.save, 7301), inInv(t, e.save, 7302), inInv(t, e.save, 7303))
		}
		if !itemIn(t, e.save, 9101, 7301) || !itemIn(t, e.save, 9102, 7302) || !itemIn(t, e.save, 9103, 7303) {
			t.Fatal("the stored stack of a base is not in the box of its turn")
		}
		mustContain(t, lines(r), "The Reserve moved into the boxes DD-1, DD-2 and DD-3", "The Reserve was moved into the boxes DD-1, DD-2 and DD-3", "every stack that was moved is in a box", "Recycled 3 base(s)")
		c := readState(t, e.state).LastCheck
		if c == nil || len(c.ReserveMoves) != 3 || len(c.ReserveMoves[2].Boxes) != 3 || c.ReserveMoves[2].Boxes[2].Name != "DD-3" || c.ReserveMoves[2].Boxes[2].Inventory != 7303 || c.ReserveMoves[2].Boxes[2].StacksAfter != 3 || c.Intent != nil {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("without DD-3 the Reserve of the last base stays in the Reserve and the run says to add DD-3", func(t *testing.T) {
		e, o := mvEnv(t, rcWorld(ddBases(3)...)+rcBoxes(3, 3))
		r, err := Roll(o)
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		if inInv(t, e.save, 12) != 3 || inInv(t, e.save, 7301) != 3 || inInv(t, e.save, 7302) != 3 {
			t.Fatalf("Reserve %d, DD-1 %d, DD-2 %d", inInv(t, e.save, 12), inInv(t, e.save, 7301), inInv(t, e.save, 7302))
		}
		mustContain(t, lines(r), "Warning: after the last base the Reserve still holds 3 stack(s)", "add DD-3")
	})
}

// A gap in the numbers: DD-4 stands behind a missing DD-3, so the run is one of two boxes, and the refusal says that DD-4 is ignored.
func TestABoxBehindAGapIsNotUsedAndTheRunSaysSo(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(3)...)+rcBoxes(3, 3)+rcExtraBox(4, "DD-4", 1000))
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v%s", err, lines(r))
	}
	// the Reserve of the last base is left (DD-1 and DD-2 are full and DD-4 is not in the row), and the warning says what to do
	if inInv(t, e.save, 12) != 3 || inInv(t, e.save, 7304) != 0 {
		t.Fatalf("Reserve %d, DD-4 %d: DD-4 was used although DD-3 is missing", inInv(t, e.save, 12), inInv(t, e.save, 7304))
	}
	mustContain(t, lines(r), "Warning: after the last base the Reserve still holds 3 stack(s)", "Ignored: DD-4", "DD-3 is missing", "add DD-3")
}

// The preflight and the dry run name every box found, and a Reserve that only the third box can take does not stop the run before base 1.
func TestTheDryRunNamesTheBoxesItFoundAndTheRollPreflightAcceptsThreeBoxes(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(3)...)+rcBoxes(3, 3)+rcExtraBox(3, "DD-3", 3))
	o.DryRun = true
	before := sha(t, e.save)
	r, err := Roll(o)
	if err != nil || r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "found 3 usable: DD-1, DD-2, DD-3", "3 of 3 base(s) can be recycled")
	noExportFolder(t, e)
}

// The state file of build 556 (two boxes): its shape is unchanged (a move's boxes are a list of name, inventory and stacksAfter, the intent has no boxes), so the file is read as it is, and
// a note it left is settled by the next run.
func TestAnOldTwoBoxStateFileIsReadAndItsNoteSettled(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10)+rcExtraBox(3, "DD-3", 10))
	sum, err := fileSHA(e.save)
	if err != nil {
		t.Fatal(err)
	}
	at := e.now.Add(-time.Hour).UTC().Format(time.RFC3339)
	old := fmt.Sprintf(`{
 "lastCheck": {"at": %[1]q, "checkpoint": "cp-earlier.db", "reserveMoves": [{"afterBase": 1010, "stacks": 3, "items": 405,
    "boxes": [{"name": "DD-1", "inventory": 7301, "stacksAfter": 3}, {"name": "DD-2", "inventory": 7302, "stacksAfter": 0}]}],
   "writeIntent": {"bases": [1010], "solidoItems": [9100], "solidoBlueprints": [1], "saveFile": %[2]q, "saveSha256Before": %[3]q}},
 "recycleHistory": [{"at": %[1]q, "checkpoint": "cp-earlier.db", "checkpointPath": "C:\\x\\cp-earlier.db", "saveFile": %[2]q, "bases": [1010], "solidoItems": [9100],
    "unconfirmed": true, "saveSha256Before": %[3]q, "solidoBlueprints": [1]}]
}`, at, e.save, sum)
	if err := os.MkdirAll(filepath.Dir(e.state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.state, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	st := readState(t, e.state)
	if st.LastCheck == nil || len(st.LastCheck.ReserveMoves) != 1 || len(st.LastCheck.ReserveMoves[0].Boxes) != 2 || st.LastCheck.ReserveMoves[0].Boxes[0].Name != "DD-1" ||
		st.LastCheck.ReserveMoves[0].Boxes[0].StacksAfter != 3 || st.LastCheck.Intent == nil || len(st.RecycleHistory) != 1 || !st.RecycleHistory[0].Unconfirmed {
		t.Fatalf("the old shape was not read: %+v", st)
	}
	// the save file is exactly as it was before that run: the note is dropped and the run goes on, with the boxes found now (three of them)
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "Earlier run settled", "the write did not happen; the note is dropped")
	if h := readState(t, e.state).RecycleHistory; len(h) != 1 || h[0].Unconfirmed || h[0].Checkpoint == "cp-earlier.db" {
		t.Fatalf("%+v", h)
	}
}

// #1218: the preflight asks about the boxes before any base is touched, and what it found (a box ignored behind a gap, a typo in a name, only DD-1) is said then, once, before the first
// base is recycled or exported, in the dry run and in the real run alike.
func TestThePreflightSaysWhatItFoundAboutTheBoxesBeforeTheFirstBase(t *testing.T) {
	// DD-2 is misnamed "DD2" and DD-3 stands behind the gap
	const world = `update permission_actor set actor_name='DD2' where actor_id=3002;`
	check := func(t *testing.T, out string, firstMark string) {
		t.Helper()
		mustContain(t, out, "Boxes for the Reserve, checked before any base is recycled:", "Ignored: DD-3 (DD-2 is missing", `Developer storage boxes with other names: "DD2"`)
		pre := strings.Index(out, "checked before any base is recycled")
		first := strings.Index(out, firstMark)
		if pre < 0 || first < 0 || pre > first {
			t.Fatalf("the box lines must come before %q:\n%s", firstMark, out)
		}
		if n := strings.Count(out, "checked before any base is recycled"); n != 1 {
			t.Fatalf("said %d times, not once:\n%s", n, out)
		}
	}
	t.Run("dry run", func(t *testing.T) {
		e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcExtraBox(3, "DD-3", 10)+world)
		o.DryRun = true
		r, err := Roll(o)
		if err != nil || r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		check(t, lines(r), "Dry run: would first write")
		noExportFolder(t, e)
	})
	t.Run("real run", func(t *testing.T) {
		_, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcExtraBox(3, "DD-3", 10)+world)
		r, err := Roll(o)
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		check(t, lines(r), "Exported and read back")
	})
	t.Run("only DD-1", func(t *testing.T) {
		_, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10)+world)
		o.DryRun = true
		r, err := Roll(o)
		if err != nil {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "checked before any base is recycled", "only DD-1 was found")
	})
}
