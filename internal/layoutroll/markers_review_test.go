package layoutroll

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The review of the Deep Desert markers step (#1216): the stops of the step after earlier steps wrote the save, the note left before the write, the checks right before Game.ini, the
// backup that is made last, and the wording of a dry run.

func wholeRunWorld() string {
	return rcWorld(ddBases(1)...) + rcBase(9, "HaggaBasin", 1) + vehicleSQL(311, "DeepDesert", 0, 1) + mkWorld
}

// A stop of the markers step before anything of it was written, after a base was recycled and a vehicle moved, says that the save WAS changed, names the checkpoint, and keeps what the
// earlier steps did in the state file.
func TestAMarkersStopAfterEarlierWritesNamesTheCheckpoint(t *testing.T) {
	check := func(t *testing.T, e *env, r Result, err error) {
		t.Helper()
		if err == nil || r.Wrote || e.ini() != iniWith("11") {
			t.Fatalf("%v", err)
		}
		stt := readState(t, e.state)
		if stt.LastCheck == nil || stt.LastCheck.Checkpoint == "" || stt.LastRoll != nil {
			t.Fatalf("%+v", stt)
		}
		mustContain(t, err.Error(), "the save WAS changed", "NOTE: 1 base(s) WERE recycled", "vehicle(s) WERE also taken out", stt.LastCheck.Checkpoint, "Copy-Item -Force")
		if strings.HasPrefix(err.Error(), "not changed") {
			t.Fatalf("it says nothing was changed: %v", err)
		}
		if len(stt.LastCheck.Recycled) != 1 || len(stt.LastCheck.Vehicles) != 1 {
			t.Fatalf("the earlier steps are not on record: %+v", stt.LastCheck)
		}
		mustContain(t, stt.LastRun.Outcome, "bases were recycled and written earlier in this run", "vehicles were taken out and written earlier in this run")
		if strings.HasPrefix(stt.LastRun.Outcome, "not changed") {
			t.Fatalf("the outcome says nothing was changed: %s", stt.LastRun.Outcome)
		}
		if survey(t, e.save).DeepDesert.Total() != 8 {
			t.Fatal("the markers were removed")
		}
	}
	t.Run("the markers cannot be counted", func(t *testing.T) {
		e, o := rcEnv(t, wholeRunWorld())
		swapVar(t, &surveyMarkers, func(*save.Save) (ops.MarkerSurvey, error) { return ops.MarkerSurvey{}, errors.New("no such table") })
		r, err := Roll(o)
		check(t, e, r, err)
		mustContain(t, err.Error(), "the Deep Desert markers could not be counted", "no such table")
	})
	t.Run("the save cannot be opened for the count", func(t *testing.T) {
		e, o := rcEnv(t, wholeRunWorld())
		// the first check of the run is before the guard, the second is the one right before the markers step: the earlier steps have opened the save by then
		real := openSave
		swapVar(t, &openSave, func(p string) (*save.Save, error) {
			if e.checks >= 2 {
				return nil, errors.New("the file is in use")
			}
			return real(p)
		})
		r, err := Roll(o)
		check(t, e, r, err)
		mustContain(t, err.Error(), "could not be read to count the Deep Desert markers", "the file is in use")
	})
	t.Run("the edit fails", func(t *testing.T) {
		e, o := rcEnv(t, wholeRunWorld())
		swapVar(t, &clearMarkers, func(*save.Save) (ops.MarkerSurvey, error) {
			return ops.MarkerSurvey{}, errors.New("a row could not be removed")
		})
		r, err := Roll(o)
		check(t, e, r, err)
		mustContain(t, err.Error(), "a row could not be removed")
	})
}

// The note before the write: killed between the write and the next record, the state file and the log still name the checkpoint. The simulation is a state file that cannot be written
// after the note.
func TestTheMarkersWriteLeavesANoteWithTheCheckpointBeforeIt(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		failing := false
		swapVar(t, &writeStateFile, func(path string, s state) error {
			if failing {
				return errors.New("the disk is full")
			}
			err := writeState(path, s)
			if s.LastRun != nil && strings.Contains(s.LastRun.Outcome, "about to write the save with the Deep Desert markers removed") {
				failing = true // every record after the note is lost, as if the process had died
			}
			return err
		})
		r, err := Roll(e.opts())
		if err != nil {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		if survey(t, e.save).DeepDesert.Total() != 0 {
			t.Fatal("the markers were not written")
		}
		cps := checkpoints(t, e.save)
		if len(cps) != 1 {
			t.Fatalf("checkpoints: %v", cps)
		}
		cp := filepath.Base(cps[0])
		stt := readState(t, e.state) // what the file on disk holds
		if stt.LastRun == nil || stt.LastRoll != nil || stt.LastCheck == nil {
			t.Fatalf("%+v", stt)
		}
		mustContain(t, stt.LastRun.Outcome, "about to write the save with the Deep Desert markers removed", mkWords, "may or may not have been written", cp)
		if stt.LastCheck.Checkpoint != cp {
			t.Fatalf("the state file names %q, the checkpoint is %q", stt.LastCheck.Checkpoint, cp)
		}
		mustContain(t, logText(t, e), "about to write the save with the Deep Desert markers removed", cp)
		// the later failures are said loudly, with the way back
		mustContain(t, lines(r), "WARNING", cp)
	})
}

// A state file that cannot be written for the note refuses the write of the save, as the vehicle and recycle steps do.
func TestAStateFileThatCannotTakeTheMarkersNoteRefusesTheWrite(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		swapVar(t, &writeStateFile, func(path string, s state) error {
			if s.LastRun != nil && strings.Contains(s.LastRun.Outcome, "about to write the save with the Deep Desert markers removed") {
				return errors.New("the disk is full")
			}
			return writeState(path, s)
		})
		before := sha(t, e.save)
		r, err := Roll(e.opts())
		if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
			t.Fatalf("%v", err)
		}
		mustContain(t, err.Error(), "the state file cannot be written", "the disk is full")
		if len(e.backups()) != 0 {
			t.Fatalf("a backup of Game.ini was made: %v", e.backups())
		}
	})
}

// markersNote repeats the warning about the state file, like the notes of the other steps.
func TestMarkersNoteRepeatsTheStateWarning(t *testing.T) {
	c := &checkRecord{Checkpoint: "cp.db", CheckpointPath: "C:\\x\\cp.db", SaveFile: "C:\\x\\game.db", markersWritten: true, Markers: mkWords, stateWarn: "WARNING: the state file could not be updated"}
	mustContain(t, c.markersNote(), "WARNING: the state file could not be updated", "cp.db")
	mustContain(t, c.writtenNote(), "WARNING: the state file could not be updated")
	c.written, c.Recycled = true, []recycledRecord{{}}
	mustContain(t, c.writtenNote(), "WARNING: the state file could not be updated", "WERE also removed")
}

// The game starts, or Game.ini is changed by another program, while the markers step runs: Game.ini is not written, the message names what was written and the way back, and no copy of
// Game.ini is made.
func TestGameIniChecksAreRepeatedAfterTheMarkersStep(t *testing.T) {
	real := clearMarkers
	t.Run("the game starts during the markers step", func(t *testing.T) {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, mkWorld).Path
		swapVar(t, &clearMarkers, func(s *save.Save) (ops.MarkerSurvey, error) {
			sv, err := real(s)
			e.running = true
			return sv, err
		})
		r, err := Roll(e.opts())
		if err == nil || r.Wrote || e.ini() != iniWith("11") {
			t.Fatalf("%v", err)
		}
		stt := readState(t, e.state)
		mustContain(t, err.Error(), "it started while this ran", "NOTE: the Deep Desert markers", stt.LastCheck.Checkpoint)
		mustContain(t, stt.LastRun.Outcome, "the save WAS changed earlier in this run", stt.LastCheck.Checkpoint)
		if len(e.backups()) != 0 {
			t.Fatalf("a backup was made: %v", e.backups())
		}
		if survey(t, e.save).DeepDesert.Total() != 0 {
			t.Fatal("the markers step did not write")
		}
	})
	t.Run("Game.ini changes during the markers step", func(t *testing.T) {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, mkWorld).Path
		swapVar(t, &clearMarkers, func(s *save.Save) (ops.MarkerSurvey, error) {
			sv, err := real(s)
			e.write(iniWith("4"))
			return sv, err
		})
		r, err := Roll(e.opts())
		if err == nil || r.Wrote || e.ini() != iniWith("4") {
			t.Fatalf("%v", err)
		}
		stt := readState(t, e.state)
		mustContain(t, err.Error(), "Game.ini changed while this ran", "NOTE: the Deep Desert markers", stt.LastCheck.Checkpoint)
		mustContain(t, stt.LastRun.Outcome, "the save WAS changed earlier in this run")
		if len(e.backups()) != 0 {
			t.Fatalf("a backup was made: %v", e.backups())
		}
	})
}

// A run that stops at the markers step (or before it) has made no copy of Game.ini; a run that goes on makes exactly one.
func TestNoBackupSlotIsUsedByARunThatStopsAtTheMarkers(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, mkWorld).Path
	swapVar(t, &clearMarkers, func(*save.Save) (ops.MarkerSurvey, error) {
		return ops.MarkerSurvey{}, errors.New("a row could not be removed")
	})
	if _, err := Roll(e.opts()); err == nil {
		t.Fatal("no stop")
	}
	if len(e.backups()) != 0 {
		t.Fatalf("a backup was made: %v", e.backups())
	}
	clearMarkers = func(s *save.Save) (ops.MarkerSurvey, error) { return (&ops.Ops{S: s}).ClearDeepDesertMarkers() }
	e.now = e.now.Add(time.Minute)
	if r, err := Roll(e.opts()); err != nil || !r.Wrote || len(e.backups()) != 1 {
		t.Fatalf("%v %v", err, e.backups())
	}
}

// A dry run that would stop at the markers step says so and does not go on to say what it would do to Game.ini; the counts it prints are said to be what a real run may find a few lower.
func TestADryRunThatWouldStopSaysNothingMore(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, mkWorld).Path
	o := e.opts()
	o.DryRun = true
	r, err := Roll(o)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, lines(r), "its counts can be a few lower", "Dry run: would change Game.ini")
	swapVar(t, &surveyMarkers, func(*save.Save) (ops.MarkerSurvey, error) { return ops.MarkerSurvey{}, errors.New("no such table") })
	r, err = Roll(o)
	out := lines(r)
	if err != nil || r.Wrote {
		t.Fatalf("%v\n%s", err, out)
	}
	mustContain(t, out, "Dry run: it would stop here", "no such table", "Nothing was written.")
	if strings.Contains(out, "would change Game.ini") || strings.Contains(out, "would first copy") {
		t.Fatalf("it went on to Game.ini:\n%s", out)
	}
	if _, serr := os.Stat(e.state); serr == nil {
		t.Fatal("a dry run wrote the state file")
	}
}

// The stop text of a save that changed in a way that was not confirmed is said once in what the command prints (the lines, then the error).
func TestTheMarkersStopTextIsSaidOnce(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, mkWorld).Path
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := s.Flush()
		f.Saved, f.Why = false, "the write was not confirmed in time"
		return f
	})
	r, err := Roll(e.opts())
	if err == nil || r.Wrote {
		t.Fatalf("%v", err)
	}
	all := lines(r) + "\n" + err.Error()
	if n := strings.Count(all, "the write was not reported as saved"); n != 1 {
		t.Fatalf("said %d times:\n%s", n, all)
	}
	mustContain(t, all, "the save WAS changed", "Copy-Item -Force")
}
