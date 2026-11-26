package layoutroll

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #1201: layout-roll.json is one file for every save, so the lock of one save does not protect it: a run on ANOTHER save (--save) must be refused too.

// The race of the issue with two saves: the first run has written its note and not yet written its save, and a run given another save starts. That second save's lock is free, so only the
// lock of the record itself keeps it from seeing the first save unchanged, dropping the note and writing over the record.
func TestARunOnAnotherSaveCannotDropTheNoteOfARunningRoll(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	other := e.opts()
	other.SavePath = testsave.PlayerWithSQL(t, "").Path
	otherBefore := sha(t, other.SavePath)
	var r2 Result
	var err2 error
	var noteSeen bool
	var iniDuring string
	var stateBefore, stateAfter []byte
	real := flushSave
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		stateBefore, _ = os.ReadFile(e.state)
		r2, err2 = Roll(other)
		stateAfter, _ = os.ReadFile(e.state)
		iniDuring = e.ini()
		h := readState(t, e.state).RecycleHistory
		noteSeen = len(h) == 1 && h[0].Unconfirmed && h[0].BeforeSHA256 != ""
		return real(s)
	})
	r, err := Roll(o)
	if err2 == nil || r2.Wrote || iniDuring != iniWith("11") {
		t.Fatalf("the run on the other save was not refused: %v\n%s", err2, lines(r2))
	}
	mustContain(t, err2.Error(), "not changed", "another layout roll is running", "layout-roll.json", "did not wait")
	if !noteSeen || len(stateBefore) == 0 || string(stateAfter) != string(stateBefore) {
		t.Fatalf("the record of the running roll was changed:\n%s\n---\n%s", stateBefore, stateAfter)
	}
	if len(r2.Lines) != 0 || sha(t, other.SavePath) != otherBefore || len(checkpoints(t, other.SavePath)) != 0 {
		t.Fatalf("the refused run did something:\n%s", lines(r2))
	}
	if err != nil || !r.Wrote {
		t.Fatalf("the first run was harmed: %v\n%s", err, lines(r))
	}
	b, rerr := os.ReadFile(filepath.Join(filepath.Dir(e.state), LogName))
	if rerr != nil {
		t.Fatal(rerr)
	}
	mustContain(t, string(b), "layout roll refused: another layout roll is running", "nothing changed")
}

// The lock of the record is given back when the run ends: a later run on another save goes ahead, the first run ever (the folder of the record not there yet) holds it too, and a dry run
// takes none.
func TestTheLockOfTheRecordIsHeldFromTheFirstRunAndGivenBack(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.state = filepath.Join(t.TempDir(), "not-there-yet", "layout-roll.json")
	var heldDuring bool
	o := e.opts()
	inner := o.GameRunning
	o.GameRunning = func() (bool, string) {
		if release, err := save.LockSave(e.state); err != nil {
			heldDuring = true
		} else {
			release()
		}
		return inner()
	}
	if r, err := Roll(o); err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if !heldDuring {
		t.Fatal("the record was not locked during the run")
	}
	release, err := save.LockSave(e.state)
	if err != nil {
		t.Fatalf("the lock of the record was not given back: %v", err)
	}
	defer release()
	dry := e.opts()
	dry.DryRun = true
	if _, err := Roll(dry); err != nil {
		t.Fatalf("a dry run was refused although it writes nothing: %v", err)
	}
}
