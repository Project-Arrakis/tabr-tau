package layoutroll

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The review round of #1205.

func logOf(t *testing.T, e *env) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.Dir(e.state), LogName))
	if err != nil {
		return ""
	}
	return string(b)
}

// F1: a run refused at the lock cannot write the state file, so it leaves one dated line in layout-roll.log; the state file stays byte for byte as it was, and the error is not nil.
func TestARunRefusedAtTheLockLeavesALogLineAndTheStateFileUntouched(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	putState(t, e, staleEntry(t, e, 1010))
	before, _ := os.ReadFile(e.state)
	hold, err := save.LockSave(e.save) // the open editor
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	for i := 0; i < 2; i++ { // two refusals: the log is append-only
		if _, err := Roll(o); err == nil || !strings.Contains(err.Error(), "another layout roll is running") {
			t.Fatalf("%v", err)
		}
	}
	after, _ := os.ReadFile(e.state)
	if string(after) != string(before) {
		t.Fatal("the state file was written")
	}
	lg := logOf(t, e)
	if strings.Count(lg, "\n") != 2 || !strings.HasPrefix(lg, e.now.Format("2006-01-02 15:04:05")+" layout roll refused: the save is open in another TABR TAU window (or another layout roll is running): nothing changed") {
		t.Fatalf("%q", lg)
	}
}

// A log that cannot be written never changes the run: the refusal is still an error.
func TestALogThatCannotBeWrittenDoesNotChangeTheRefusal(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	if err := os.MkdirAll(filepath.Join(filepath.Dir(e.state), LogName), 0o700); err != nil { // a folder where the log should be
		t.Fatal(err)
	}
	hold, err := save.LockSave(e.save)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	if _, err := Roll(e.opts()); err == nil || !strings.Contains(err.Error(), "another layout roll is running") {
		t.Fatalf("%v", err)
	}
}

// F3: a real run with no save to check settles nothing and writes no state file; it is refused before the note of an earlier run is looked at, and the refusal is in the log.
func TestARunWithNoSaveSettlesNothingAndWritesNoStateFile(t *testing.T) {
	for _, running := range []bool{false, true} { // the game-running refusal wrote the state file without the lock before
		t.Run(fmt.Sprint("game running ", running), func(t *testing.T) {
			e, _ := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
			putState(t, e, staleEntry(t, e, 1010)) // an unconfirmed note whose file is unchanged: a run that settled would drop it
			before, _ := os.ReadFile(e.state)
			e.running = running
			o := e.opts()
			o.SavePath, o.SaveProblem = "", "2 saves were found, so it is not clear which one to check"
			r, err := Roll(o)
			if err == nil || r.Wrote || e.ini() != iniWith("11") {
				t.Fatalf("%v", err)
			}
			if strings.Contains(lines(r), "Earlier run settled") {
				t.Fatalf("a note was settled without the lock:\n%s", lines(r))
			}
			after, _ := os.ReadFile(e.state)
			if string(after) != string(before) {
				t.Fatal("the state file was written by a run that holds no lock")
			}
			if !strings.Contains(logOf(t, e), "layout roll refused: ") {
				t.Fatalf("%q", logOf(t, e))
			}
		})
	}
}

// F2: a vehicle note that is settled as written leaves a record of the earlier run's checkpoint; every other branch leaves the state as the table says.
func TestWhatTheStateHoldsAfterEachSettleOfAVehicleNote(t *testing.T) {
	cases := []struct {
		name    string
		edit    []string
		wantErr bool
		note    bool // the note is still there
		history int  // entries in vehicleHistory
	}{
		{"file unchanged: dropped", nil, false, false, 0},
		{"file changed, none out: dropped", []string{`update actors set location_x=location_x+1 where id=1`}, false, false, 0},
		{"every vehicle out: recorded", []string{`update actors set map='HaggaBasin' where id=311`, `update actors set state=2 where id=310`,
			`insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,310,'B1C3_Harkonnen_Sandbike_Variant')`}, false, false, 1},
		{"part out: stop, note kept", []string{`update actors set map='HaggaBasin' where id=311`}, true, true, 0},
		{"neither as it was nor as planned: stop, note kept", []string{`update actors set state=7 where id=310`}, true, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := settleEnv(t)
			if len(c.edit) > 0 {
				editSave(t, e.save, c.edit...)
			}
			_, err := Roll(e.opts())
			if (err != nil) != c.wantErr {
				t.Fatalf("%v", err)
			}
			stt := readState(t, e.state)
			if (stt.VehicleNote != nil) != c.note || len(stt.VehicleHistory) != c.history {
				t.Fatalf("note %v, history %+v", stt.VehicleNote, stt.VehicleHistory)
			}
			if c.history == 1 {
				h := stt.VehicleHistory[0]
				if h.Checkpoint != "cp-earlier.db" || h.CheckpointPath != "C:\\x\\cp-earlier.db" || fmt.Sprint(h.Vehicles) != "[310 311]" || h.SettledAt.IsZero() {
					t.Fatalf("%+v", h)
				}
			}
		})
	}
}

// F5: after the first base was asked about with the leftover Reserve moved, the file is read again; if it is no longer the file the checkpoint was taken of, nothing is recycled.
func TestAFileThatChangedDuringTheFirstQuestionRefusesBeforeAnyWrite(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcReserveItem+"update inventories set max_item_count=3 where id=12;")
	real := emptyReserveInto
	swapVar(t, &emptyReserveInto, func(s *save.Save, exclude []int64, previewOnly bool) (movedReserve, error) {
		if !previewOnly {
			editSave(t, e.save, `update actors set location_x=location_x+1 where id=2`) // another program writes the save
		}
		return real(s, exclude, previewOnly)
	})
	_, err := Roll(o)
	if err == nil || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save file is not the one the checkpoint", "0 of 2 base(s) can be recycled now", "nothing was recycled or exported")
	if len(standingIDs(t, e.save)) != 2 {
		t.Fatal("a base was recycled")
	}
	noExportFolder(t, e)
}

// F4: the stop message of a stuck note names the key to remove and says that --force does not bypass it, and --force really does not.
func TestAStuckNoteNamesTheKeyAndForceDoesNotBypassIt(t *testing.T) {
	e := settleEnv(t)
	stt := readState(t, e.state)
	stt.VehicleNote.ReadBackFailed = true
	if err := writeState(e.state, stt); err != nil {
		t.Fatal(err)
	}
	o := e.opts()
	o.Force = true
	_, err := Roll(o)
	if err == nil {
		t.Fatal("--force bypassed the note")
	}
	mustContain(t, err.Error(), `remove the whole "vehicleNote" key`, "--force does not bypass it")
	if readState(t, e.state).VehicleNote == nil {
		t.Fatal("the note was dropped")
	}
	e2, o2 := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	en := staleEntry(t, e2, 1010)
	en.ReadBackFailed = true
	putState(t, e2, en)
	o2.Force = true
	if _, err := Roll(o2); err == nil || !strings.Contains(err.Error(), "--force does not bypass it") {
		t.Fatalf("%v", err)
	}
}
