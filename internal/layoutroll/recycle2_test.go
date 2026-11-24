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

// backpackSlots makes the backpack (two stacks in it) have free free slots.
func backpackSlots(free int) string {
	return fmt.Sprintf("update inventories set max_item_count=%d where id=1;", 2+free)
}

// The room logic on its own (the Reserve rule is off here, so that several bases fit one run): a new Solido device per base needs a slot, and the margin of two free slots must remain.
// With three bases and 2, 3, 4 and 5 free slots the run recycles 0, 1, 2 and 3 bases; the roll comes only with all three; a run that recycles fewer stops before it and names the base
// that did not fit and what is short.
func TestRoomForZeroOneTwoAndThreeBases(t *testing.T) {
	withoutReserveRule(t)
	for _, c := range []struct {
		free int
		k    int
	}{{2, 0}, {3, 1}, {4, 2}, {5, 3}} {
		t.Run(fmt.Sprintf("%d free slots", c.free), func(t *testing.T) {
			e, o := rcEnv(t, rcWorld(ddBases(3)...)+backpackSlots(c.free))
			before := sha(t, e.save)
			r, err := Roll(o)
			stt := readState(t, e.state)
			switch c.k {
			case 0:
				if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") || len(standingIDs(t, e.save)) != 3 {
					t.Fatalf("%v", err)
				}
				mustContain(t, err.Error(), "0 of 3 base(s) can be recycled now", "backpack", "short by 1", "margin of 2")
				if len(stt.LastCheck.Recycled) != 0 {
					t.Fatal("something is recorded as recycled")
				}
			case 3:
				if err != nil || !r.Wrote || e.ini() == iniWith("11") || len(standingIDs(t, e.save)) != 0 {
					t.Fatalf("%v\n%s", err, lines(r))
				}
				mustContain(t, lines(r), "3 of 3 base(s) can be recycled in this run: 3 Solido device(s)", "refund together:")
				// three different new Solido devices, each filled by its own base
				if got := count(t, e.save, `select count(distinct item_id) n from building_blueprints`); got != 3 {
					t.Fatalf("%d devices carry a Solido", got)
				}
			default:
				if err == nil || r.Wrote || e.ini() != iniWith("11") || len(standingIDs(t, e.save)) != 3-c.k {
					t.Fatalf("%v %v", err, standingIDs(t, e.save))
				}
				mustContain(t, err.Error(), fmt.Sprintf("%d of 3 base(s) recycled", c.k), "Not rolled", "the checkpoint of this run is", "NOTE:")
				if len(stt.LastCheck.Recycled) != c.k || stt.LastRoll != nil {
					t.Fatalf("%+v", stt.LastCheck)
				}
				// the bases that were done are the first ones, in order
				for i, d := range stt.LastCheck.Recycled {
					if d.ID != int64(1010+10*i) {
						t.Fatalf("%+v", stt.LastCheck.Recycled)
					}
				}
			}
		})
	}
}

// The Reserve must be empty before a base is recycled. With something in it nothing is recycled, nothing changes, and the message says to empty it.
func TestANonEmptyReserveStopsTheRunWithNothingChanged(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+rcReserveItem)
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") || len(checkpoints(t, e.save)) != 1 {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "0 of 1 base(s) can be recycled now", "the Reserve is not empty (it holds 1 stack(s))", "empty the Reserve in the game", "#1191")
	if len(readState(t, e.state).LastCheck.ExportFiles) != 1 {
		t.Fatal("the base was not exported first")
	}
}

// A refusal for lack of room at the Reserve's own slot limit, and at the backpack's volume, is a "does not fit" (0 of N), not a failure of the run.
func TestRealRefusalsForRoomAreHandledAsRoom(t *testing.T) {
	for name, sql := range map[string]string{
		"the Reserve has a slot limit that the base passes": "update inventories set max_item_count=1 where id=12;",
		"the backpack's volume limit has no room":           "update inventories set max_item_volume=0.0001 where id=1;",
	} {
		t.Run(name, func(t *testing.T) {
			e, o := rcEnv(t, rcWorld(ddBases(2)...)+sql)
			before := sha(t, e.save)
			_, err := Roll(o)
			if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
				t.Fatalf("%v", err)
			}
			mustContain(t, err.Error(), "0 of 2 base(s) can be recycled now", "no room")
		})
	}
}

// A base that is not the player's own stops the run, and nothing is recycled, the own bases included.
func TestABaseThatIsNotYoursStopsTheRunAndNothingIsRecycled(t *testing.T) {
	for name, extra := range map[string]string{
		"owned by another player": rcBase(5, "DeepDesert", 99),
		"owned by nobody":         rcBase(5, "DeepDesert", 99) + "delete from permission_actor_rank where permission_actor_id=1050;",
		"shared: you are a member": rcBase(5, "DeepDesert", 99) +
			"insert into permission_actor_rank(permission_actor_id,player_id,rank) values (1050,1,2);",
		"owned by you and by another player": rcBase(5, "DeepDesert", 1) + "insert into permission_actor_rank(permission_actor_id,player_id,rank) values (1050,99,1);",
	} {
		t.Run(name, func(t *testing.T) {
			e, o := rcEnv(t, rcWorld(ddBases(1)...)+extra)
			before := sha(t, e.save)
			r, err := Roll(o)
			if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
				t.Fatalf("%v", err)
			}
			if strings.Contains(name, "owned by you and") {
				// a shared base is the player's own base for the check, and the recycle itself refuses it: all or nothing
				mustContain(t, err.Error(), "cannot be recycled", "base 1050")
			} else {
				mustContain(t, err.Error(), "not yours", "base 1050", "nothing was recycled")
			}
			if len(standingIDs(t, e.save)) == 0 {
				t.Fatal("a base was recycled")
			}
		})
	}
}

// Another player's base on another map is none of this run's business; the base in the Deep Desert is recycled. A base of yours on another map is never touched either.
func TestBasesOnOtherMapsAreLeftAlone(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+rcBase(6, "HaggaBasin", 99)+rcBase(7, "HaggaBasin", 1))
	if _, err := Roll(o); err != nil {
		t.Fatal(err)
	}
	if count(t, e.save, `select count(*) n from totems where id in (1060,1070)`) != 2 || count(t, e.save, `select count(*) n from building_instances where building_id in (1061,1071)`) != 4 {
		t.Fatal("a base on another map was changed")
	}
}

// A base whose map the save does not record is not recycled: it cannot be told to be in the Deep Desert.
func TestABaseWithNoMapIsNotRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(rcBase(1, "", 1)))
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "map the save does not name")
}

// All or nothing at the question: one base that the recycle refuses for any reason other than room means that no base is recycled.
func TestOneBaseThatCannotBeRecycledMeansNoneIs(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(3)...)+"update building_instances set building_type='Unknown_Piece_X' where building_id=1021 and instance_id=1;")
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || len(standingIDs(t, e.save)) != 3 || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "1 of 3 base(s) cannot be recycled, so none was", "base 1020", "Unknown_Piece_X")
}

// A failure at the second base: the working copy is thrown away, the save file is not written, and the message says which bases were done in the copy, which failed and which were not tried.
func TestAFailingSecondBaseLeavesTheSaveAsItWas(t *testing.T) {
	withoutReserveRule(t)
	e, o := rcEnv(t, rcWorld(ddBases(3)...))
	real := recycleOne
	swapVar(t, &recycleOne, func(s *save.Save, id int64, fp string) (map[string]any, error) {
		if id == 1020 {
			return nil, errors.New("disk on fire")
		}
		return real(s, id, fp)
	})
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") || len(standingIDs(t, e.save)) != 3 {
		t.Fatalf("%v", err)
	}
	out := lines(r)
	mustContain(t, out, `base 1010 "Base 1": recycled and verified in the working copy only`, `base 1020 "Base 2": FAILED`, `base 1030 "Base 3": not tried`)
	mustContain(t, err.Error(), "disk on fire", "NOTHING is changed", "Database > Save backups")
	stt := readState(t, e.state)
	if stt.LastCheck == nil || stt.LastCheck.Checkpoint == "" || len(stt.LastCheck.Recycled) != 0 || stt.LastRoll != nil {
		t.Fatalf("%+v", stt.LastCheck)
	}
	// a re-run without the fault finishes the job
	swapVar(t, &recycleOne, real)
	e.now = e.now.Add(time.Minute)
	if r, err := Roll(o); err != nil || !r.Wrote || len(standingIDs(t, e.save)) != 0 {
		t.Fatalf("%v", err)
	}
}

// A base whose own read back fails (the verify of the base) stops the run before the next base and before the write.
func TestABaseThatDoesNotVerifyStopsBeforeTheNextOne(t *testing.T) {
	withoutReserveRule(t)
	e, o := rcEnv(t, rcWorld(ddBases(2)...))
	var verified []int64
	real := verifyBaseHere
	swapVar(t, &verifyBaseHere, func(s *save.Save, totem, item, bp int64) error {
		verified = append(verified, totem)
		if totem == 1010 {
			return errors.New("the Solido is not linked")
		}
		return real(s, totem, item, bp)
	})
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || len(verified) != 1 {
		t.Fatalf("%v %v", err, verified)
	}
	mustContain(t, err.Error(), "did not verify", "the Solido is not linked")
}

// A file that does not read back after the write: the message says the save WAS changed, names the checkpoint and gives the command to go back; the roll does not run.
func TestAWrittenSaveThatDoesNotReadBackIsReportedWithTheWayBack(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &verifyBasesWritten, func(string, []recycledRecord, []ops.HoldBase, ops.RecycleRoom) error {
		return errors.New("a row is missing")
	})
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save WAS changed", "a row is missing", "Copy-Item -Force", ".db", "NOTE: 1 base(s) WERE recycled")
	stt := readState(t, e.state)
	if stt.LastCheck == nil || stt.LastCheck.CheckpointPath == "" || len(stt.LastCheck.Recycled) != 1 || !strings.Contains(stt.LastRun.Outcome, "does not read back") {
		t.Fatalf("%+v", stt)
	}
	if _, serr := os.Stat(stt.LastCheck.CheckpointPath); serr != nil {
		t.Fatalf("the checkpoint named is not there: %v", serr)
	}
}

// The write that does not happen leaves everything as it was.
func TestAWriteThatIsNotSavedChangesNothing(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the disk is full", "the save was not changed")
	// nothing is left queued: the same save is recycled by a run without the fault
	swapVar(t, &flushSave, func(s *save.Save) save.Flush { return s.Flush() })
	e.now = e.now.Add(time.Minute)
	if _, err := Roll(o); err == nil || fmt.Sprint(standingIDs(t, e.save)) != "[1020]" {
		t.Fatalf("%v %v", err, standingIDs(t, e.save))
	}
}

// A save that another editor holds is not written.
func TestASaveThatIsLockedIsNotRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &lockSave, func(string) (func(), error) { return nil, errors.New("held by another window") })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || !strings.Contains(err.Error(), "another TABR TAU window") {
		t.Fatalf("%v", err)
	}
}

// The export comes first and a failed export stops the run before any base is recycled.
func TestAFailedExportStopsBeforeAnyBaseIsRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...))
	swapVar(t, &exportBaseFile, func(*save.Save, int64, time.Time, string) (*ops.BaseExport, error) {
		return nil, errors.New("no space for the file")
	})
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "no space for the file")
}

// The character in the Deep Desert: nothing is recycled, the run refuses as it always did (and exports the bases).
func TestTheCharacterInTheDeepDesertBlocksTheRecycle(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+"update actors set map='DeepDesert' where id=1;")
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, lines(r), "no base is recycled in this run", "the character is")
	if len(readState(t, e.state).LastCheck.ExportFiles) != 1 {
		t.Fatal("the base was not exported")
	}
}

// A vehicle plan that cannot be made stops the run before a base is recycled.
func TestAVehiclePlanThatFailsStopsBeforeAnyBaseIsRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+vehicleSQL(311, "DeepDesert", 0, 1))
	swapVar(t, &planVehicles, func(*save.Save) ([]ops.VehicleMove, error) { return nil, errors.New("no clear spot") })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "no clear spot", "no base was recycled")
}

// The vehicles of the player are taken out after the bases, in the same run, and then the roll.
func TestVehiclesAreMovedAfterTheBasesAndTheRollGoesOn(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+vehicleSQL(311, "DeepDesert", 0, 1))
	r, err := Roll(o)
	if err != nil || !r.Wrote || len(standingIDs(t, e.save)) != 0 {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	out := lines(r)
	if strings.Index(out, "Recycled 1 base(s)") > strings.Index(out, "vehicle(s) of yours out of the Deep Desert") || !strings.Contains(out, "vehicle 311") {
		t.Fatalf("the order of the run:\n%s", out)
	}
}

// A failure after the bases were written (here: the game starts while the run works) says so, with the way back.
func TestAFailureAfterTheBasesWereWrittenNamesTheWayBack(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	calls := 0
	o.GameRunning = func() (bool, string) {
		calls++
		return calls >= 2, "Dune.exe is running"
	}
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") || len(standingIDs(t, e.save)) != 0 {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "started while this ran", "NOTE: 1 base(s) WERE recycled", "Copy-Item")
}

// The dry run with the flag: the whole plan, and not one byte written anywhere.
func TestTheDryRunPrintsThePlanAndWritesNothing(t *testing.T) {
	for name, c := range map[string]struct {
		rule bool
		want []string
	}{
		"the Reserve rule on": {true, []string{"Rule: The Reserve must be empty before each base is recycled", "1 of 3 base(s) recycled", "base 1020", "Dry run: it would stop here", "#1191", "Reserve: empty before each base", "backpack: needs 1 slot(s)"}},
		"all three fit":       {false, []string{"3 of 3 base(s) can be recycled in this run: 3 Solido device(s)", "refund together:", "Dry run: that is what a real run does", "Dry run: would change Game.ini", "backpack: needs 3 slot(s)", `Base 1030 "Base 3"`}},
	} {
		t.Run(name, func(t *testing.T) {
			if !c.rule {
				withoutReserveRule(t)
			}
			e, o := rcEnv(t, rcWorld(ddBases(3)...))
			o.DryRun = true
			before := sha(t, e.save)
			r, err := Roll(o)
			if err != nil {
				t.Fatalf("a dry run is not a failure: %v", err)
			}
			mustContain(t, lines(r), c.want...)
			if sha(t, e.save) != before || len(checkpoints(t, e.save)) != 0 || e.ini() != iniWith("11") || r.Wrote {
				t.Fatal("the dry run wrote something")
			}
			if _, err := os.Stat(e.state); err == nil {
				t.Fatal("the dry run wrote the state file")
			}
			if got, _ := filepath.Glob(filepath.Join(filepath.Dir(e.state), exportFolderPrefix+"*")); len(got) != 0 {
				t.Fatal("the dry run exported a base")
			}
			if len(standingIDs(t, e.save)) != 3 {
				t.Fatal("a base is gone")
			}
		})
	}
}

// Everything that was recycled in a run is in the state file, with the checkpoint's name and the Solido device and blueprint of each base.
func TestTheStateFileRecordsEveryBase(t *testing.T) {
	withoutReserveRule(t)
	e, o := rcEnv(t, rcWorld(ddBases(2)...))
	if _, err := Roll(o); err != nil {
		t.Fatal(err)
	}
	c := readState(t, e.state).LastCheck
	if c == nil || len(c.Recycled) != 2 || c.Checkpoint == "" || c.ExportFolder == "" || len(c.ExportFiles) != 2 || len(c.RecycleSpace) == 0 {
		t.Fatalf("%+v", c)
	}
	if c.Recycled[0].SolidoItem == c.Recycled[1].SolidoItem || c.Recycled[0].SolidoBlueprint == c.Recycled[1].SolidoBlueprint {
		t.Fatalf("two bases share a Solido: %+v", c.Recycled)
	}
}

// The classifier of room refusals matches what the recycle really says.
func TestRoomRefusalClassifier(t *testing.T) {
	for _, m := range []string{"The Solido could not be made: The character has no blank Solido device, and a new one has no room: the backpack has no free slot. Nothing was changed.", "The Reserve of this save has room for 3 stacks and the base returns 9, so nothing was changed."} {
		if !ops.IsRoomRefusal(errors.New(m)) {
			t.Fatalf("%s", m)
		}
	}
	if ops.IsRoomRefusal(errors.New("A crafting queue is running")) || ops.IsRoomRefusal(nil) {
		t.Fatal("too wide")
	}
}

// A second guard besides the list of bases: the answer of the recycle itself must say the base is on the Deep Desert, or nothing is recycled.
func TestABaseThatTheRecycleSaysIsOnAnotherMapIsNeverRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	real := previewRecycle
	swapVar(t, &previewRecycle, func(s *save.Save, id int64) (map[string]any, error) {
		m, err := real(s, id)
		if err == nil {
			m["map"] = "HaggaBasin"
		}
		return m, err
	})
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || len(standingIDs(t, e.save)) != 1 {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "not the Deep Desert")
}
