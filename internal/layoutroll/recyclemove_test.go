package layoutroll

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The move of the Reserve into the two storage boxes between the bases of a run (#1191 wired into #1167), on made-up saves. Every recycled base brings 3 stacks to the Reserve in
// these worlds (the one stack its silo held, 300 CopperOre, and two refund stacks).

// rcBoxes are the two storage boxes of the operator: Developer storage containers named DD-1 (actor 3001, inventory 7301) and DD-2 (3002, 7302) in a base of the character's on
// Hagga Basin (base 5). capA and capB are their slots; the boxes have no volume limit.
func rcBoxes(capA, capB int) string {
	const et = 5764607523034230050 // the entity of base 5's totem (rcBase)
	return rcBase(5, "HaggaBasin", 1) + fmt.Sprintf(`
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (3001,'/Game/BP_Box.BP_Box_C','HaggaBasin',5001,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (3002,'/Game/BP_Box.BP_Box_C','HaggaBasin',5002,6000,7,0,0,0,1,0,0,5,1,jsonb('{}'));
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (3001,'DD-1',1,3,1,0),(3002,'DD-2',1,3,1,0);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values
  (3001,%[1]d,'Developer_StorageContainer_Placeable',1,0),(3002,%[1]d,'Developer_StorageContainer_Placeable',1,0);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (7301,3001,4,%[2]d,0.0),(7302,3002,4,%[3]d,0.0);
`, et, capA, capB)
}

func inInv(t *testing.T, path string, inv int64) int64 {
	t.Helper()
	return count(t, path, `select count(*) n from items where inventory_id=?`, inv)
}

func itemIn(t *testing.T, path string, item, inv int64) bool {
	t.Helper()
	return count(t, path, `select count(*) n from items where id=? and inventory_id=?`, item, inv) == 1
}

// noExportFolder fails when a run left a folder of exported base files.
func noExportFolder(t *testing.T, e *env) {
	t.Helper()
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(e.state), exportFolderPrefix+"*")); len(m) != 0 {
		t.Fatalf("an export folder was left: %v", m)
	}
}

// Two bases, two boxes: every stack of each base lands in a box, in the order the Reserve held them, and the Reserve ends empty; the roll goes on.
func TestTwoBasesTwoBoxesEveryStackLandsInABoxInOrder(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(3, 3))
	r, err := Roll(o)
	if err != nil || !r.Wrote || e.ini() == iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	// base 1's three stacks fill DD-1; base 2's find DD-1 full and fill DD-2
	if inInv(t, e.save, 12) != 0 || inInv(t, e.save, 7301) != 3 || inInv(t, e.save, 7302) != 3 {
		t.Fatalf("Reserve %d, DD-1 %d, DD-2 %d", inInv(t, e.save, 12), inInv(t, e.save, 7301), inInv(t, e.save, 7302))
	}
	if !itemIn(t, e.save, 9101, 7301) || !itemIn(t, e.save, 9102, 7302) {
		t.Fatal("the stored stack of base 1 is not in DD-1, or the one of base 2 is not in DD-2")
	}
	// the stored stack is whole and unchanged
	if count(t, e.save, `select count(*) n from items where id=9101 and stack_size=300 and template_id='CopperOre'`) != 1 {
		t.Fatal("the stored stack changed")
	}
	if count(t, e.save, `select count(distinct position_index) n from items where inventory_id=7301`) != 3 {
		t.Fatal("two stacks share a slot")
	}
	stt := readState(t, e.state)
	c := stt.LastCheck
	if c == nil || len(c.Recycled) != 2 || len(c.ReserveMoves) != 2 || c.ReserveMoves[0].AfterBase != 1010 || c.ReserveMoves[1].AfterBase != 1020 || c.ReserveMoves[0].Stacks != 3 || c.Intent != nil {
		t.Fatalf("%+v", c)
	}
	if len(stt.RecycleHistory) != 1 || stt.RecycleHistory[0].Unconfirmed {
		t.Fatalf("%+v", stt.RecycleHistory)
	}
	mustContain(t, lines(r), "The Reserve moved into the boxes DD-1 and DD-2", "every stack that was moved is in a box", "Recycled 2 base(s)")
}

// The names of the boxes and the bases left out of them are exactly these, on every call.
func TestTheMoveIsAskedWithTheTwoBoxNamesAndEveryBaseOfTheRun(t *testing.T) {
	if fmt.Sprint(reserveBoxNames()) != "[DD-1 DD-2]" {
		t.Fatalf("%v", reserveBoxNames())
	}
	e, o := rcEnv(t, rcWorld(ddBases(3)...)+rcBoxes(10, 10))
	var seen []string
	real := emptyReserveInto
	swapVar(t, &emptyReserveInto, func(s *save.Save, exclude []int64, previewOnly bool) (movedReserve, error) {
		seen = append(seen, fmt.Sprint(exclude, previewOnly))
		return real(s, exclude, previewOnly)
	})
	if r, err := Roll(o); err != nil {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if len(seen) != 3 {
		t.Fatalf("%v", seen)
	}
	for _, s := range seen {
		if s != "[1010 1020 1030] false" {
			t.Fatalf("%v", seen)
		}
	}
	if inInv(t, e.save, 7301) != 9 {
		t.Fatal("nine stacks are in the first box")
	}
}

// A box that stands in a base of this run is never taken. The base's own actors must all be on the Deep Desert, so such a box is on the Deep Desert too and the move refuses it;
// asked about with what an earlier run left in the Reserve, the run recycles nothing and exports nothing.
func TestABoxStandingInABaseOfTheRunIsNeverUsed(t *testing.T) {
	const et1 = 5764607523034230010 // the entity of base 1's totem
	world := rcWorld(ddBases(2)...) + rcBoxes(10, 10) + rcReserveItem + fmt.Sprintf(`update placeables set owner_entity_id=%d where id=3001; update actors set map='DeepDesert' where id=3001;`, et1)
	e, o := rcEnv(t, world)
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "0 of 2 base(s) can be recycled now", "the Reserve is not empty", `The box "DD-1"`, "cannot receive the Reserve")
	noExportFolder(t, e)
}

// The box is full after base 1: base 1 is recycled, the move is refused for room, the run stops BEFORE the roll with base 2 left and says so. A re-run with room in the boxes moves what
// the Reserve holds first, recycles base 2, and rolls.
func TestABoxThatIsFullAfterBase1StopsTheRunWithBase2LeftAndAReRunContinues(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(1, 1))
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("it should stop before the roll: %v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "1 of 2 base(s) recycled", "base 1020", "does not fit into the boxes", "DD-1 and DD-2", "Not rolled", "the remaining base(s) (base 1020", "NOTE: 1 base(s) WERE recycled")
	if got := standingIDs(t, e.save); fmt.Sprint(got) != "[1020]" {
		t.Fatalf("standing %v", got)
	}
	// the Reserve holds base 1's stacks (the refused move changed nothing), the boxes are empty
	if inInv(t, e.save, 12) != 3 || inInv(t, e.save, 7301) != 0 || inInv(t, e.save, 7302) != 0 {
		t.Fatal("the refused move changed something")
	}
	c := readState(t, e.state).LastCheck
	if c == nil || len(c.Recycled) != 1 || len(c.ReserveMoves) != 0 || c.Intent != nil {
		t.Fatalf("%+v", c)
	}

	// room is made in the boxes: the next run moves the Reserve first, then recycles base 2 and rolls
	editSave(t, e.save, `update inventories set max_item_count=10 where id in (7301,7302)`)
	e.now = e.now.Add(time.Minute)
	r, err = Roll(o)
	if err != nil || !r.Wrote || e.ini() == iniWith("11") || len(standingIDs(t, e.save)) != 0 {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if inInv(t, e.save, 12) != 0 || inInv(t, e.save, 7301) != 6 {
		t.Fatalf("Reserve %d, DD-1 %d", inInv(t, e.save, 12), inInv(t, e.save, 7301))
	}
	c = readState(t, e.state).LastCheck
	if len(c.ReserveMoves) != 2 || c.ReserveMoves[0].AfterBase != 0 || c.ReserveMoves[1].AfterBase != 1020 {
		t.Fatalf("%+v", c.ReserveMoves)
	}
	mustContain(t, lines(r), "before the first base (what an earlier run left)")
}

const rcReserveItem2 = `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8001,12,3,2,'CopperOre',0,1790000000,'{}',0,null),(8002,12,4,3,'CopperOre',0,1790000000,'{}',0,null);`

// A run that finds the Reserve full and cannot move it (the boxes are too small) recycles nothing and exports nothing.
func TestAFullReserveThatCannotBeMovedRecyclesNothing(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(1, 1)+rcReserveItem+rcReserveItem2)
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "0 of 2 base(s) can be recycled now", "the Reserve is not empty", "does not fit into the boxes")
	noExportFolder(t, e)
}

// No boxes in the save: base 1 fits (the Reserve was empty), the move is refused, and the run stops before the roll with the other bases left. Nothing beyond what fits is recycled.
func TestNoBoxesStopsTheRunAfterWhatFitsAndSaysWhichBasesAreLeft(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(3)...))
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "1 of 3 base(s) recycled", `No Developer storage container in this save is named "DD-1"`, "the remaining base(s) (base 1020", "base 1030")
	if got := standingIDs(t, e.save); fmt.Sprint(got) != "[1020 1030]" {
		t.Fatalf("standing %v", got)
	}
	if inInv(t, e.save, 12) != 3 {
		t.Fatal("the Reserve of base 1 is not there")
	}
}

// With only one base left and no boxes, the base is recycled and the roll goes on (nothing is left to hold back); the Reserve stays full and the run says so.
func TestTheLastBaseIsRecycledAndRolledEvenWhenTheReserveCannotBeMoved(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	r, err := Roll(o)
	if err != nil || !r.Wrote || e.ini() == iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "Warning: after the last base the Reserve still holds 3 stack(s)", "make room in the boxes DD-1 and DD-2")
	if inInv(t, e.save, 12) != 3 {
		t.Fatal("the Reserve was emptied without boxes")
	}
}

// A dry run asks the whole chain (the recycle plan and the move plan after each base) and writes nothing: not the save, not the state file, no checkpoint, no export.
func TestADryRunPrintsTheMovePlanAfterEachBaseAndWritesNothing(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(3, 3))
	o.DryRun = true
	before := sha(t, e.save)
	r, err := Roll(o)
	if err != nil || sha(t, e.save) != before || e.ini() != iniWith("11") || r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	out := lines(r)
	mustContain(t, out, "The Reserve would be moved into the boxes DD-1 and DD-2 (after it): 3 stack(s)", "Would move: 3 item stacks", "2 of 2 base(s) can be recycled in this run", "Dry run: that is what a real run does", "nothing was written")
	if strings.Count(out, "would be moved into the boxes") != 2 {
		t.Fatalf("a move plan for each base was expected:\n%s", out)
	}
	if _, err := os.Stat(e.state); err == nil {
		t.Fatal("a dry run wrote the state file")
	}
	if len(checkpoints(t, e.save)) != 0 {
		t.Fatal("a dry run took a checkpoint")
	}
	noExportFolder(t, e)
}

// A dry run with a box that is full after base 1 shows the k-of-N stop and still writes nothing.
func TestADryRunWithAFullBoxShowsTheStop(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(1, 1))
	o.DryRun = true
	before := sha(t, e.save)
	r, err := Roll(o)
	if err != nil || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "Dry run: 1 of 2 base(s) recycled", "does not fit into the boxes", "Nothing was written")
}

// verifyBasesWritten after a real write: a stack that is not in the box it was moved to is found.
func TestTheWrittenFileWithAMovedStackMissingFromItsBoxIsRefused(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := s.Flush()
		editSave(t, e.save, `delete from items where id=9101`)
		return f
	})
	_, err := Roll(o)
	if err == nil {
		t.Fatal("it should stop")
	}
	mustContain(t, err.Error(), "the save WAS changed", "the storage box (inventory 7301) holds 2 stack(s), and 3 were planned")
}

// verifyBasesWritten: the Reserve of the file holds a stack although all were moved.
func TestTheWrittenFileWithAStackLeftInTheReserveIsRefused(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := s.Flush()
		editSave(t, e.save, `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8800,12,1,40,'CopperOre',0,1790000000,'{}',0,null)`)
		return f
	})
	_, err := Roll(o)
	if err == nil {
		t.Fatal("it should stop")
	}
	mustContain(t, err.Error(), "the save WAS changed", "the Reserve holds 1 stacks, and 0 were planned")
}

// A character with no Reserve is refused in the preview, before anything is exported.
func TestACharacterWithNoReserveIsRefusedBeforeTheExport(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10)+"delete from inventories where id=12;")
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the character has no Reserve", "nothing was exported")
	noExportFolder(t, e)
}

// N2: the state file holds the intent (the checkpoint, the planned bases, the file's hash) BEFORE the save is written.
func TestTheStateFileHoldsTheIntentBeforeTheSaveIsWritten(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10))
	var seen state
	var shaThen [32]byte
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		seen = readState(t, e.state)
		shaThen = sha(t, e.save)
		return s.Flush()
	})
	before := sha(t, e.save)
	if r, err := Roll(o); err != nil {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	c := seen.LastCheck
	if shaThen != before || c == nil || c.Intent == nil || fmt.Sprint(c.Intent.Bases) != "[1010 1020]" || len(c.Intent.BeforeSHA256) != 64 || c.Checkpoint == "" || c.CheckpointPath == "" || len(c.Recycled) != 0 {
		t.Fatalf("the intent was not on record before the write: %+v", c)
	}
	if !strings.Contains(seen.LastRun.Outcome, "about to write") || len(seen.RecycleHistory) != 1 || !seen.RecycleHistory[0].Unconfirmed || seen.RecycleHistory[0].Checkpoint != c.Checkpoint {
		t.Fatalf("%+v %+v", seen.LastRun, seen.RecycleHistory)
	}
	// after the run the intent is gone and the history entry is confirmed
	end := readState(t, e.state)
	if end.LastCheck.Intent != nil || len(end.RecycleHistory) != 1 || end.RecycleHistory[0].Unconfirmed {
		t.Fatalf("%+v %+v", end.LastCheck, end.RecycleHistory)
	}
}

// N2: a state file that cannot be written before the save refuses the write: the save is not touched.
func TestAStateFileThatCannotBeWrittenBeforeTheSaveRefusesTheWrite(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if s.LastRun != nil && strings.Contains(s.LastRun.Outcome, "about to write") {
			return errors.New("the disk is full")
		}
		return writeState(p, s)
	})
	called := false
	swapVar(t, &flushSave, func(s *save.Save) save.Flush { called = true; return s.Flush() })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || called || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v called=%v", err, called)
	}
	mustContain(t, err.Error(), "the state file cannot be written", "the disk is full", "the save was not written")
	if st := readState(t, e.state); len(st.RecycleHistory) != 0 || (st.LastCheck != nil && st.LastCheck.Intent != nil) {
		t.Fatalf("the intent was left: %+v", st)
	}
}

// N2: a state file that cannot be written after the save was written is said loudly, and the run still names the way back.
func TestAStateFileThatCannotBeWrittenAfterTheSaveWasWrittenIsSaidLoudly(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(1, 1))
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if s.LastRun != nil && !strings.Contains(s.LastRun.Outcome, "about to write") {
			return errors.New("the disk is full")
		}
		return writeState(p, s)
	})
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "WARNING: the state file", "the disk is full", "NOTE: 1 base(s) WERE recycled", "Copy-Item")
	mustContain(t, lines(r), "WARNING: the state file")
}

// N2: and when the run goes on to the roll, the failed record of it is a WARNING with the way back too.
func TestAFailedRecordOfTheRollAfterRecycledBasesIsAWarning(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if s.LastRoll != nil {
			return errors.New("the disk is full")
		}
		return writeState(p, s)
	})
	r, err := Roll(o)
	if err != nil || !r.Wrote || e.ini() == iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "WARNING: the record of this roll and of the recycled bases could not be saved", "NOTE: 1 base(s) WERE recycled", "Copy-Item")
}

// N1: a write that is not reported as saved, with a file that is different, but NOT what was planned (another program changed it): the run does not say bases were recycled.
func TestAnUnconfirmedWriteThatDoesNotReadBackAsPlannedDoesNotClaimTheBasesWereRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		editSave(t, e.save, `update actors set location_x=location_x+1 where id=2`) // another program changes the file; the recycle is not written
		return save.Flush{Why: "the written file could not be verified"}
	})
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "the save file is different from before the write", "does not read back as the recycle that was planned", "NOT known what changed the file", "Copy-Item -Force", "Look at it before the game starts")
	if strings.Contains(err.Error(), "WERE recycled") || strings.Contains(err.Error(), "WAS changed (1 base") {
		t.Fatalf("it claims the bases were recycled: %v", err)
	}
	c := readState(t, e.state).LastCheck
	if c == nil || len(c.Recycled) != 0 || c.CheckpointPath == "" || c.Intent == nil {
		t.Fatalf("%+v", c)
	}
}

// N1: a file that cannot be read before the write is refused before the write; the save is not touched.
func TestASaveFileThatCannotBeReadBeforeTheWriteIsNotWritten(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &fileSHA, func(string) (string, error) { return "", errors.New("access denied") })
	called := false
	swapVar(t, &flushSave, func(s *save.Save) save.Flush { called = true; return s.Flush() })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || called || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v called=%v", err, called)
	}
	mustContain(t, err.Error(), "the save file cannot be read before the write", "access denied", "the save was not written")
}

// N1, the vehicle step: the same two rules.
func TestTheVehicleStepDoesNotClaimVehiclesWereTakenOutWhenTheFileDoesNotReadBackAsPlanned(t *testing.T) {
	e, o := rcEnv(t, rcWorld()+vehicleSQL(311, "DeepDesert", 0, 1))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		editSave(t, e.save, `update actors set location_x=location_x+1 where id=2`)
		return save.Flush{Why: "the written file could not be verified"}
	})
	_, err := Roll(o)
	if err == nil || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save file is different from before the write", "does not read back as the vehicles taken out", "NOT known what changed the file", "Database > Save backups")
	if strings.Contains(err.Error(), "vehicle(s) taken out of the Deep Desert)") {
		t.Fatalf("it claims the vehicles were taken out: %v", err)
	}
}

func TestTheVehicleStepRefusesTheWriteWhenTheFileCannotBeReadFirst(t *testing.T) {
	e, o := rcEnv(t, rcWorld()+vehicleSQL(311, "DeepDesert", 0, 1))
	swapVar(t, &fileSHA, func(string) (string, error) { return "", errors.New("access denied") })
	called := false
	swapVar(t, &flushSave, func(s *save.Save) save.Flush { called = true; return s.Flush() })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || called || sha(t, e.save) != before {
		t.Fatalf("%v called=%v", err, called)
	}
	mustContain(t, err.Error(), "the save file cannot be read before the write", "the save was not written")
}

// N4: the written file holds as many bases as expected, but not the ones expected.
func TestTheWrittenFileWithADifferentBaseStandingThanExpectedIsRefused(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcBoxes(1, 1))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := s.Flush()
		// base 1020 loses its owner (it no longer counts as the player's) and base 1030 of the player appears: one base stands, as expected, but it is another one
		editSave(t, e.save, `delete from permission_actor_rank where permission_actor_id=1020`, rcBase(3, "DeepDesert", 1))
		return f
	})
	_, err := Roll(o)
	if err == nil {
		t.Fatal("it should stop")
	}
	mustContain(t, err.Error(), "the save WAS changed", "base 1030 stands in the Deep Desert, and base 1020 was expected")
}

// N4 as a direct call: the same count, another id.
func TestVerifyBasesWrittenNamesTheBaseThatStandsInsteadOfTheExpectedOne(t *testing.T) {
	e, _ := rcEnv(t, rcWorld(ddBases(2)...))
	err := verifyBasesWritten(e.save, nil, []ops.HoldBase{{ID: 1010}, {ID: 1030}}, writtenExpect{})
	if err == nil || !strings.Contains(err.Error(), "base 1020 stands in the Deep Desert, and base 1030 was expected") {
		t.Fatalf("%v", err)
	}
}

// N5: the note names the checkpoints of the EARLIER runs, never this run's own.
func TestTheNoteDoesNotListThisRunsOwnCheckpointUnderEarlierRuns(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(3)...)+rcBoxes(1, 1))
	e.now = e.now.Add(time.Minute)
	_, err := Roll(o) // run 1: base 1010 recycled, stops (the boxes are too small)
	if err == nil {
		t.Fatal("run 1 should stop")
	}
	if strings.Contains(err.Error(), "Checkpoints of the earlier runs") {
		t.Fatalf("run 1 has no earlier run: %v", err)
	}
	first := readState(t, e.state).RecycleHistory[0].Checkpoint
	editSave(t, e.save, `delete from items where inventory_id=12`) // the person emptied the Reserve in the game
	e.now = e.now.Add(time.Minute)
	_, err = Roll(o) // run 2: base 1020 recycled, stops again
	if err == nil {
		t.Fatal("run 2 should stop")
	}
	second := readState(t, e.state).RecycleHistory[1].Checkpoint
	_, tail, ok := strings.Cut(err.Error(), "Checkpoints of the earlier runs")
	if !ok || !strings.Contains(tail, first) || strings.Contains(tail, second) {
		t.Fatalf("the note lists the wrong checkpoints (first %s, second %s):\n%v", first, second, err)
	}
}
