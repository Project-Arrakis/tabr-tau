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
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #1201: the state-file lock, the intent record of the vehicle step and the order of the leftover Reserve against the slot limit.

// countLock wraps the real lock of the save so that a test sees how often it was taken and given back; it stays a real lock.
func countLock(t *testing.T) (took, gave *int, paths *[]string) {
	t.Helper()
	took, gave, paths = new(int), new(int), new([]string)
	real := lockSave
	swapVar(t, &lockSave, func(p string) (func(), error) {
		*paths = append(*paths, p)
		rel, err := real(p)
		if err != nil {
			return nil, err
		}
		*took++
		return func() { *gave++; rel() }, nil
	})
	return took, gave, paths
}

// A second layout roll that starts while one is running is refused, at once, and the first run is not harmed. The second one is started from inside the first (when it asks whether the
// game runs, which it does after it has taken the lock), so the two really overlap.
func TestASecondLayoutRollWhileOneIsRunningIsRefused(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	o := e.opts()
	second := e.opts()
	var r2 Result
	var err2 error
	started := false
	inner := o.GameRunning
	o.GameRunning = func() (bool, string) {
		if !started {
			started = true
			r2, err2 = Roll(second)
		}
		return inner()
	}
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("the first run was harmed: %v\n%s", err, lines(r))
	}
	if !started || err2 == nil || r2.Wrote {
		t.Fatalf("the second run was not refused: %v\n%s", err2, lines(r2))
	}
	mustContain(t, err2.Error(), "not changed", "another layout roll is running", "did not wait")
	if len(r2.Lines) != 0 {
		t.Fatalf("the refused run read the state before it had the lock:\n%s", lines(r2))
	}
	if n := len(e.backups()); n != 1 {
		t.Fatalf("the Game.ini backups are %d, one expected", n)
	}
	stt := readState(t, e.state)
	if stt.LastRoll == nil || stt.LastRoll.To != r.New {
		t.Fatalf("%+v", stt.LastRoll)
	}
}

// The race of the issue: the scheduled run has written its note and not yet written the save (the file is exactly as it was), and a manual run starts. It used to see the hash equal and
// drop the note, and the two runs overwrote each other's state file. Now it is refused and the note stays.
func TestASecondRollBetweenTheNoteAndTheWriteCannotDropTheNote(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	manual := e.opts()
	manual.RecycleBases = true
	var r2 Result
	var err2 error
	var noteSeen bool
	real := flushSave
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		r2, err2 = Roll(manual)
		h := readState(t, e.state).RecycleHistory
		noteSeen = len(h) == 1 && h[0].Unconfirmed && h[0].BeforeSHA256 != ""
		return real(s)
	})
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if err2 == nil || r2.Wrote {
		t.Fatalf("the second run was not refused: %v\n%s", err2, lines(r2))
	}
	mustContain(t, err2.Error(), "another layout roll is running")
	if !noteSeen {
		t.Fatal("the note of the first run was dropped or changed by the second run")
	}
	stt := readState(t, e.state)
	if len(stt.RecycleHistory) != 1 || stt.RecycleHistory[0].Unconfirmed || stt.LastRoll == nil || stt.LastCheck == nil || len(stt.LastCheck.Recycled) != 1 {
		t.Fatalf("the state of the first run was not kept: %+v", stt)
	}
}

// The lock is taken before the state file is read and before the notes of earlier runs are settled: a run that cannot take it reads nothing and changes nothing, the note included.
func TestARunThatCannotTakeTheLockLeavesTheNoteOfAnEarlierRunAlone(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	putState(t, e, staleEntry(t, e, 1010))
	beforeState, err := os.ReadFile(e.state)
	if err != nil {
		t.Fatal(err)
	}
	swapVar(t, &lockSave, func(string) (func(), error) { return nil, errors.New("held by another run") })
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "another layout roll is running", "another TABR TAU window", "held by another run")
	after, _ := os.ReadFile(e.state)
	if string(after) != string(beforeState) {
		t.Fatalf("the state file was written:\n%s", after)
	}
	if len(checkpoints(t, e.save)) != 0 || len(e.backups()) != 0 {
		t.Fatal("something was done before the lock")
	}
}

// The lock is the one of the save, taken on the save file itself (a folder given is the game.db in it), once for the whole run, and given back on every way out of the run.
func TestTheLockIsTakenOnceAndGivenBackOnEveryWayOut(t *testing.T) {
	type scenario struct {
		name  string
		setup func(t *testing.T) (*env, Options)
		check func(t *testing.T, r Result, err error)
	}
	cases := []scenario{
		{"a roll", func(t *testing.T) (*env, Options) { e := newEnv(t, iniWith("11")); return e, e.opts() },
			func(t *testing.T, r Result, err error) {
				if err != nil || !r.Wrote {
					t.Fatalf("%v", err)
				}
			}},
		{"a roll that has already happened this week (skipped)", func(t *testing.T) (*env, Options) {
			e := newEnv(t, iniWith("11"))
			if _, err := Roll(e.opts()); err != nil {
				t.Fatal(err)
			}
			return e, e.opts()
		}, func(t *testing.T, r Result, err error) {
			if err != nil || r.Wrote {
				t.Fatalf("%v", err)
			}
		}},
		{"the game is running", func(t *testing.T) (*env, Options) {
			e := newEnv(t, iniWith("11"))
			e.running = true
			return e, e.opts()
		},
			func(t *testing.T, r Result, err error) {
				if err == nil {
					t.Fatal("a refusal was expected")
				}
			}},
		{"a state file that is not valid", func(t *testing.T) (*env, Options) {
			e := newEnv(t, iniWith("11"))
			if err := os.MkdirAll(filepath.Dir(e.state), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(e.state, []byte("{not json"), 0o600); err != nil {
				t.Fatal(err)
			}
			return e, e.opts()
		}, func(t *testing.T, r Result, err error) {
			if err == nil {
				t.Fatal("a refusal was expected")
			}
		}},
		{"the character in the Deep Desert (a stop of the guard)", func(t *testing.T) (*env, Options) {
			e, o := rcEnv(t, rcWorld(ddBases(1)...)+"update actors set map='DeepDesert' where id=1;")
			return e, o
		}, func(t *testing.T, r Result, err error) {
			if err == nil {
				t.Fatal("a refusal was expected")
			}
		}},
		{"a note that cannot be settled (a stop)", func(t *testing.T) (*env, Options) {
			e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
			en := staleEntry(t, e, 1010)
			en.ReadBackFailed = true
			putState(t, e, en)
			return e, o
		}, func(t *testing.T, r Result, err error) {
			if err == nil {
				t.Fatal("a stop was expected")
			}
		}},
		{"bases recycled and rolled", func(t *testing.T) (*env, Options) { return mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)) },
			func(t *testing.T, r Result, err error) {
				if err != nil || !r.Wrote {
					t.Fatalf("%v\n%s", err, lines(r))
				}
			}},
		{"the state file cannot be written before the save is (an error)", func(t *testing.T) (*env, Options) {
			e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
			swapVar(t, &writeStateFile, func(p string, s state) error {
				if s.LastRun != nil && strings.Contains(s.LastRun.Outcome, "about to write") {
					return errors.New("the disk is full")
				}
				return writeState(p, s)
			})
			return e, o
		}, func(t *testing.T, r Result, err error) {
			if err == nil {
				t.Fatal("a refusal was expected")
			}
		}},
		{"vehicles taken out and rolled", func(t *testing.T) (*env, Options) {
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
			return e, e.opts()
		}, func(t *testing.T, r Result, err error) {
			if err != nil || !r.Wrote {
				t.Fatalf("%v\n%s", err, lines(r))
			}
		}},
		{"a save that is given as its folder", func(t *testing.T) (*env, Options) {
			e := newEnv(t, iniWith("11"))
			o := e.opts()
			o.SavePath = filepath.Dir(e.save)
			return e, o
		}, func(t *testing.T, r Result, err error) {
			if err != nil || !r.Wrote {
				t.Fatalf("%v", err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, o := c.setup(t)
			took, gave, paths := countLock(t)
			r, err := Roll(o)
			c.check(t, r, err)
			if *took != 1 || *gave != 1 {
				t.Fatalf("the lock was taken %d time(s) and given back %d time(s); once each was expected", *took, *gave)
			}
			if len(*paths) != 1 || (*paths)[0] != e.save {
				t.Fatalf("the lock was asked for on %v, not on the save file %s", *paths, e.save)
			}
			// really free: another taker gets it now
			rel, lerr := save.LockSave(e.save)
			if lerr != nil {
				t.Fatalf("the lock is still held after the run: %v", lerr)
			}
			rel()
		})
	}
}

// A dry run writes nothing, so it takes no lock (it must keep working while the editor has the save open) and says that it did not.
func TestADryRunTakesNoLockAndSaysSo(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	took, _, _ := countLock(t)
	hold, err := save.LockSave(e.save) // the save is held by someone else: a dry run goes on
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	o := e.opts()
	o.DryRun = true
	r, err := Roll(o)
	if err != nil || r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if *took != 0 {
		t.Fatalf("a dry run took the lock %d time(s)", *took)
	}
	mustContain(t, lines(r), "Dry run: the lock on the save", "is not taken: a dry run writes nothing")
}

// A run that has no save to check (nothing to lock) is refused by the guard as before; it changes nothing.
func TestARunWithNoSaveTakesNoLockAndIsRefusedAsBefore(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	took, _, _ := countLock(t)
	o := e.opts()
	o.SavePath = ""
	_, err := Roll(o)
	if err == nil || *took != 0 || e.ini() != iniWith("11") {
		t.Fatalf("%v (lock taken %d)", err, *took)
	}
}

// ---- the vehicle step's note (#1201) ----

func vehicleEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
	return e
}

// The note is in the state file when the save is written, and gone once the write is confirmed; the record of the run holds the vehicles.
func TestTheVehicleStepNotesItsIntentBeforeTheWriteAndClearsItAfter(t *testing.T) {
	e := vehicleEnv(t)
	var during *vehicleNote
	var intentOnRecord bool
	real := flushSave
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		stt := readState(t, e.state)
		during = stt.VehicleNote
		intentOnRecord = stt.LastCheck != nil && stt.LastCheck.Intent != nil
		return real(s)
	})
	before, err := fileSHA(e.save)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Roll(e.opts())
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if during == nil || during.BeforeSHA256 != before || during.SaveFile != e.save || during.Checkpoint == "" || len(during.Vehicles) != 4 || !intentOnRecord {
		t.Fatalf("the note before the write: %+v (intent on record %v)", during, intentOnRecord)
	}
	var stored, moved int
	for _, v := range during.Vehicles {
		switch v.Action {
		case "store":
			stored++
		case "move":
			if v.ToMap == "" || v.ToMap == v.FromMap {
				t.Fatalf("a moved vehicle without a place to: %+v", v)
			}
			moved++
		}
		if v.FromMap != "DeepDesert" {
			t.Fatalf("%+v", v)
		}
	}
	if stored != 1 || moved != 3 {
		t.Fatalf("stored %d, moved %d", stored, moved)
	}
	stt := readState(t, e.state)
	if stt.VehicleNote != nil || stt.LastCheck == nil || stt.LastCheck.Intent != nil || len(stt.LastCheck.Vehicles) != 4 || stt.LastRoll == nil {
		t.Fatalf("after the run: %+v", stt)
	}
}

// A state file that cannot be written before the write refuses the write: the save is not changed and no note is left.
func TestAStateFileThatCannotBeWrittenBeforeTheVehicleWriteRefusesIt(t *testing.T) {
	e := vehicleEnv(t)
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if s.LastRun != nil && strings.Contains(s.LastRun.Outcome, "about to write") {
			return errors.New("the disk is full")
		}
		return writeState(p, s)
	})
	before := sha(t, e.save)
	r, err := Roll(e.opts())
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the state file cannot be written", "the disk is full", "the save was not written")
	if stt := readState(t, e.state); stt.VehicleNote != nil || (stt.LastCheck != nil && stt.LastCheck.Intent != nil) {
		t.Fatalf("%+v", stt)
	}
	// and the save is taken out by a run without the fault
	swapVar(t, &writeStateFile, writeState)
	e.now = e.now.Add(time.Minute)
	if r, err := Roll(e.opts()); err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
}

// A write that is refused (the save is left as it was) takes the note back.
func TestARefusedVehicleWriteTakesTheNoteBack(t *testing.T) {
	e := vehicleEnv(t)
	swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
	before := sha(t, e.save)
	_, err := Roll(e.opts())
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the disk is full", "the save was not changed")
	if stt := readState(t, e.state); stt.VehicleNote != nil || (stt.LastCheck != nil && stt.LastCheck.Intent != nil) {
		t.Fatalf("%+v", stt)
	}
}

// The case of the issue: the state file cannot be written after the vehicles were written, with no base recycled. It is said loudly (not dropped), and the note stays for the next run.
func TestAStateFailureAfterTheVehicleWriteIsNotDroppedSilently(t *testing.T) {
	e := vehicleEnv(t)
	fail := true
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if fail && s.LastRun != nil && !strings.Contains(s.LastRun.Outcome, "about to write") {
			return errors.New("the disk is full")
		}
		return writeState(p, s)
	})
	r, err := Roll(e.opts())
	if err != nil {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "WARNING: the state file", "could not be updated after the save was written", "the disk is full",
		"WARNING: the record of this roll and of the vehicles taken out of the Deep Desert could not be saved", "NOTE: 4 vehicle(s) WERE taken out of the Deep Desert and written to the save")
	// the note is still on disk: it holds the checkpoint and the plan
	n := readState(t, e.state).VehicleNote
	if n == nil || len(n.Vehicles) != 4 || n.Checkpoint == "" {
		t.Fatalf("%+v", n)
	}
	// the next run settles it: the file differs (the game saved since) and every planned vehicle is out as planned
	fail = false
	editSave(t, e.save, `update actors set location_x=location_x+1 where id=1`)
	e.now = e.now.Add(time.Minute)
	r2, err := Roll(e.opts())
	if err != nil {
		t.Fatalf("%v\n%s", err, lines(r2))
	}
	mustContain(t, lines(r2), "WARNING: the run of", "every one of the 4 planned vehicle(s) is out of the Deep Desert as planned", "recorded as written")
	if readState(t, e.state).VehicleNote != nil {
		t.Fatal("the note was not settled")
	}
}

// A write that was not reported as saved, that changed the file and does not read back as planned: the note stays, marked, and the next run stops on it and changes nothing.
func TestAVehicleWriteThatDoesNotReadBackLeavesAMarkedNoteThatStopsTheNextRun(t *testing.T) {
	e := vehicleEnv(t)
	real := flushSave
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := real(s)
		return save.Flush{Why: "it could not be read back in time", Saved: false, Warning: f.Warning}
	})
	swapVar(t, &verifyWritten, func(string, []ops.VehicleMove) error { return errors.New("read back: a row differs") })
	_, err := Roll(e.opts())
	if err == nil {
		t.Fatal("the run should stop")
	}
	mustContain(t, err.Error(), "does not read back as the vehicles taken out")
	n := readState(t, e.state).VehicleNote
	if n == nil || !n.ReadBackFailed {
		t.Fatalf("%+v", n)
	}
	// the next run: the faults are gone, and it still stops
	swapVar(t, &flushSave, real)
	swapVar(t, &verifyWritten, func(path string, plan []ops.VehicleMove) error {
		fresh, err := openSave(path)
		if err != nil {
			return err
		}
		defer fresh.Close()
		return (&ops.Ops{S: fresh}).VerifyVehicleMoves(plan)
	})
	before := sha(t, e.save)
	iniBefore := e.ini()
	e.now = e.now.Add(time.Minute)
	r, err := Roll(e.opts())
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniBefore {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "read-back FAILED", "is not settled by looking at the same file again")
}

// settleEnv is a vehicle save with a note for the vehicles of the fleet (a Sandbike stored, a buggy moved) written by an earlier run that did not record how it went.
func settleEnv(t *testing.T) *env {
	e := vehicleEnv(t)
	h, err := fileSHA(e.save)
	if err != nil {
		t.Fatal(err)
	}
	n := &vehicleNote{At: e.now.Add(-time.Hour), Checkpoint: "cp-earlier.db", CheckpointPath: "C:\\x\\cp-earlier.db", SaveFile: e.save, BeforeSHA256: h,
		Vehicles: []plannedVehicle{{ID: 310, Action: "store", FromMap: "DeepDesert"}, {ID: 311, Action: "move", FromMap: "DeepDesert", ToMap: "HaggaBasin"}}}
	if err := writeState(e.state, state{VehicleNote: n}); err != nil {
		t.Fatal(err)
	}
	return e
}

// The settling of a note of the vehicle step, the same rules as for the bases.
func TestAVehicleNoteOfAnEarlierRunIsSettledBeforeAnythingElse(t *testing.T) {
	t.Run("the file is as it was: the write never happened, the note is dropped and the run goes on", func(t *testing.T) {
		e := settleEnv(t)
		r, err := Roll(e.opts())
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "Earlier run settled", "the write did not happen; the note is dropped")
		if readState(t, e.state).VehicleNote != nil {
			t.Fatal("the note stays")
		}
	})
	t.Run("a dry run says it would drop the note and drops nothing", func(t *testing.T) {
		e := settleEnv(t)
		o := e.opts()
		o.DryRun = true
		before, _ := os.ReadFile(e.state)
		r, err := Roll(o)
		if err != nil {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "the note would be dropped (a dry run changes nothing)")
		after, _ := os.ReadFile(e.state)
		if string(before) != string(after) {
			t.Fatal("a dry run wrote the state file")
		}
	})
	t.Run("the file is different and no planned vehicle has left: the write did not take effect, dropped loudly", func(t *testing.T) {
		e := settleEnv(t)
		editSave(t, e.save, `update actors set location_x=location_x+1 where id=1`)
		r, err := Roll(e.opts())
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "WARNING: the run of", "none of the 2 planned vehicle(s) has left, so that write did not take effect")
		if readState(t, e.state).VehicleNote != nil {
			t.Fatal("the note stays")
		}
	})
	t.Run("a part of it is out: the run stops and changes nothing", func(t *testing.T) {
		e := settleEnv(t)
		editSave(t, e.save, `update actors set map='HaggaBasin' where id=311`)
		before := sha(t, e.save)
		r, err := Roll(e.opts())
		if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
			t.Fatalf("%v", err)
		}
		mustContain(t, err.Error(), "1 of the 2 planned vehicle(s) are out and 1 still stand where they were")
		if readState(t, e.state).VehicleNote == nil {
			t.Fatal("the note was dropped")
		}
	})
	t.Run("a vehicle that is neither as it was nor as planned: the run stops", func(t *testing.T) {
		e := settleEnv(t)
		editSave(t, e.save, `update actors set state=7 where id=310`)
		before := sha(t, e.save)
		_, err := Roll(e.opts())
		if err == nil || sha(t, e.save) != before {
			t.Fatalf("%v", err)
		}
		mustContain(t, err.Error(), "does not hold what was planned", "vehicle 310 was to be stored")
	})
	t.Run("a note that does not say enough: the run stops", func(t *testing.T) {
		e := settleEnv(t)
		stt := readState(t, e.state)
		stt.VehicleNote.BeforeSHA256 = ""
		if err := writeState(e.state, stt); err != nil {
			t.Fatal(err)
		}
		_, err := Roll(e.opts())
		if err == nil || !strings.Contains(err.Error(), "cannot be settled") {
			t.Fatalf("%v", err)
		}
	})
}

// With bases recycled and vehicles taken out in the same run, both notes are kept in turn and both are gone at the end; a vehicle write that is refused after the bases were written
// says so and names the way back, and leaves no vehicle note.
func TestBasesAndVehiclesInOneRun(t *testing.T) {
	world := rcWorld(ddBases(1)...) + rcBoxes(10, 10) + facingEast + lrVehicle(311, "Buggy", "x", 1, "DeepDesert")
	t.Run("both are written and recorded", func(t *testing.T) {
		e, o := mvEnv(t, world)
		r, err := Roll(o)
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		stt := readState(t, e.state)
		if stt.VehicleNote != nil || len(stt.RecycleHistory) != 1 || stt.RecycleHistory[0].Unconfirmed || stt.LastCheck == nil || len(stt.LastCheck.Recycled) != 1 || len(stt.LastCheck.Vehicles) != 1 || stt.LastCheck.Intent != nil {
			t.Fatalf("%+v", stt)
		}
	})
	t.Run("the vehicle write is refused after the bases were written", func(t *testing.T) {
		e, o := mvEnv(t, world)
		real := flushSave
		calls := 0
		swapVar(t, &flushSave, func(s *save.Save) save.Flush {
			calls++
			if calls == 2 {
				return save.Flush{Why: "the disk is full"}
			}
			return real(s)
		})
		r, err := Roll(o)
		if err == nil || r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, err.Error(), "the disk is full", "NOTE: 1 base(s) WERE recycled and written")
		stt := readState(t, e.state)
		if stt.VehicleNote != nil || len(stt.RecycleHistory) != 1 || stt.RecycleHistory[0].Unconfirmed {
			t.Fatalf("%+v", stt)
		}
	})
}

// ---- the leftover Reserve against the slot limit (#1201) ----

// A Reserve with a slot limit that its leftover fills: moving the leftover into the boxes first makes room for the first base, so the run recycles the bases instead of giving 0 of N.
// Each base brings 3 stacks to the Reserve; the Reserve holds 1 stack and has 3 slots.
func TestALeftoverReserveIsMovedBeforeTheRoomForTheFirstBaseIsAskedAbout(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcReserveItem+"update inventories set max_item_count=3 where id=12;")
	var seen []string
	real := emptyReserveInto
	swapVar(t, &emptyReserveInto, func(s *save.Save, exclude []int64, previewOnly bool) (movedReserve, error) {
		seen = append(seen, fmt.Sprint(previewOnly))
		return real(s, exclude, previewOnly)
	})
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if len(standingIDs(t, e.save)) != 0 || inInv(t, e.save, 12) != 0 {
		t.Fatalf("standing %v, Reserve %d", standingIDs(t, e.save), inInv(t, e.save, 12))
	}
	// the leftover (item 8000) went to the boxes first, and the run recorded that move before the first base
	if !itemIn(t, e.save, 8000, 7301) {
		t.Fatal("the leftover stack is not in the first box")
	}
	c := readState(t, e.state).LastCheck
	if c == nil || len(c.ReserveMoves) != 3 || c.ReserveMoves[0].AfterBase != 0 || c.ReserveMoves[0].Stacks != 1 {
		t.Fatalf("%+v", c)
	}
	// the question order: the preview of the boxes, the move played in the preflight, then the real moves of the loop
	if len(seen) < 3 || seen[0] != "true" || seen[1] != "false" {
		t.Fatalf("%v", seen)
	}
	mustContain(t, lines(r), "before the first base (what an earlier run left)")
}

// When even the emptied Reserve has no room for the base, it is 0 of N before anything is exported or changed, and the words say that the leftover was taken into account.
func TestALimitedReserveThatCannotTakeABaseEvenWhenEmptiedIsRefusedBeforeTheExport(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcReserveItem+"update inventories set max_item_count=2 where id=12;")
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "0 of 2 base(s) can be recycled now", "no room for it, even after the Reserve an earlier run left is moved into the boxes", "Nothing was recycled and nothing was exported")
	noExportFolder(t, e)
	if inInv(t, e.save, 12) != 1 {
		t.Fatal("the leftover was touched")
	}
}

// A leftover that the boxes cannot take stays what it was: 0 of N with the reason of the Reserve, nothing exported.
func TestALimitedReserveWhoseLeftoverCannotBeMovedIsRefusedWithTheReasonOfTheBoxes(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcReserveItem+"update inventories set max_item_count=3 where id=12;")
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "0 of 1 base(s) can be recycled now", "the Reserve is not empty (it holds 1 stack(s))")
	noExportFolder(t, e)
}
