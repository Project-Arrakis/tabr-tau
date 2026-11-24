package layoutroll

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Round 3 of the review of #1200: a file the earlier run could not read back is never accepted, the wording of the settling, the reason texts of the preflight, and the gaps in the tests.

// tookEffect runs the first of two runs of a world with two bases, the state file refusing every write after the save was written: base 1 is recycled and written, the state file
// still holds the unconfirmed note. It returns the environment for the second run, with the game having saved since (the file differs from before).
func tookEffect(t *testing.T) (*env, Options) {
	t.Helper()
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(1, 1))
	fail := true
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if fail && s.LastRun != nil && !strings.Contains(s.LastRun.Outcome, "about to write") {
			return errors.New("the disk is full")
		}
		return writeState(p, s)
	})
	if _, err := Roll(o); err == nil {
		t.Fatal("run 1 should stop")
	}
	fail = false
	editSave(t, e.save, `update inventories set max_item_count=10 where id in (7301,7302)`)
	e.now = e.now.Add(time.Minute)
	return e, o
}

// F1: a write that was not reported as saved and that does not read back as planned leaves a note marked as read back and failed; the next run stops on it, even when the file would
// otherwise look like "none of the bases is gone", and says so.
func TestAFileThatDidNotReadBackAsPlannedIsNeverAcceptedByTheNextRun(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	first := true
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		if first {
			first = false
			editSave(t, e.save, `update actors set location_x=location_x+1 where id=2`) // another program changes the file; the recycle is not written
			return save.Flush{Why: "the written file could not be verified"}
		}
		return s.Flush()
	})
	if _, err := Roll(o); err == nil {
		t.Fatal("run 1 should stop")
	}
	h := readState(t, e.state).RecycleHistory
	if len(h) != 1 || !h[0].Unconfirmed || !h[0].ReadBackFailed {
		t.Fatalf("the note is not marked: %+v", h)
	}
	before := sha(t, e.save)
	e.now = e.now.Add(time.Minute)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "the read-back FAILED", h[0].Checkpoint, "cannot be accepted as written", "remove the entry marked \"unconfirmed\"")
	if h2 := readState(t, e.state).RecycleHistory; len(h2) != 1 || !h2[0].ReadBackFailed {
		t.Fatalf("the note must stay: %+v", h2)
	}
	if first {
		t.Fatal("the write was never tried")
	}
}

// F2: a planned base that is gone without its Solido is a file that could be read and does not hold the plan, not a failure to read it.
func TestAPlannedBaseThatIsGoneWithoutItsSolidoIsNotCalledAReadFailure(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	en := staleEntry(t, e, 99999) // a base that is not in the file at all, so it counts as gone; the Solido 9100 of the note was never made
	putState(t, e, en)
	editSave(t, e.save, `update actors set location_x=location_x+1 where id=2`) // the file differs from before
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "The file could be read, but it does not hold what was planned", "a planned base is gone without its Solido rows as planned", "Check that base in the save", "remove the entry marked \"unconfirmed\"")
	if strings.Contains(err.Error(), "reading it to settle the note failed") {
		t.Fatalf("called a read failure: %v", err)
	}
}

// F6: the file of the note cannot be hashed while settling: the run stops and says that the file cannot be read now.
func TestASaveFileThatCannotBeHashedWhileSettlingStopsTheRun(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	putState(t, e, staleEntry(t, e, 1010))
	swapVar(t, &fileSHA, func(string) (string, error) { return "", errors.New("access denied") })
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "the save file cannot be read now (access denied)", "cannot be settled")
	if h := readState(t, e.state).RecycleHistory; len(h) != 1 || !h[0].Unconfirmed {
		t.Fatalf("the note must stay: %+v", h)
	}
}

// F6: the state file cannot be written after a note was settled: a warning, the run goes on, the note is settled again next time.
func TestAStateFileThatCannotBeWrittenAfterSettlingIsAWarning(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	putState(t, e, staleEntry(t, e, 1010))
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if s.LastRun != nil && strings.Contains(s.LastRun.Outcome, "unconfirmed write was settled") {
			return errors.New("the disk is full")
		}
		return writeState(p, s)
	})
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "WARNING: the state file", "could not be updated after the earlier run's note was settled (the disk is full)", "will be settled again by the next run")
}

// F6: two unconfirmed notes are both settled in one run.
func TestTwoUnconfirmedNotesAreBothSettled(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	en1 := staleEntry(t, e, 1010)
	en2 := staleEntry(t, e, 1010)
	en2.At, en2.Checkpoint = en2.At.Add(-time.Hour), "cp-earlier2.db"
	if err := writeState(e.state, state{RecycleHistory: []recycleHistoryEntry{en2, en1}}); err != nil {
		t.Fatal(err)
	}
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if n := strings.Count(lines(r), "Earlier run settled"); n != 2 {
		t.Fatalf("%d notes settled:\n%s", n, lines(r))
	}
	for _, h := range readState(t, e.state).RecycleHistory {
		if h.Unconfirmed || strings.HasPrefix(h.Checkpoint, "cp-earlier") {
			t.Fatalf("%+v", h)
		}
	}
}

// F6: when the second of two notes cannot be settled the run stops and the first is not dropped on disk (nothing is written then).
func TestASecondNoteThatCannotBeSettledStopsTheRunAndLeavesBothOnDisk(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	en1 := staleEntry(t, e, 1010)
	en2 := staleEntry(t, e, 1010)
	en2.At, en2.Checkpoint, en2.ReadBackFailed = en2.At.Add(time.Minute), "cp-earlier2.db", true
	if err := writeState(e.state, state{RecycleHistory: []recycleHistoryEntry{en1, en2}}); err != nil {
		t.Fatal(err)
	}
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "cp-earlier2.db", "the read-back FAILED")
	// the first note was settled (the stop writes the state file as it is then); the second stays for the operator
	if h := readState(t, e.state).RecycleHistory; len(h) != 1 || h[0].Checkpoint != "cp-earlier2.db" || !h[0].Unconfirmed {
		t.Fatalf("%+v", h)
	}
}

// F6: settling a note clears the intent that the last check still holds for the same checkpoint, and leaves the one of another checkpoint.
func TestSettlingANoteClearsTheIntentOfTheLastCheckOfTheSameCheckpoint(t *testing.T) {
	for _, tc := range []struct {
		name, cp string
		cleared  bool
	}{{"same checkpoint", "cp-earlier.db", true}, {"another checkpoint", "cp-other.db", false}} {
		t.Run(tc.name, func(t *testing.T) {
			e, o := mvEnv(t, rcWorld()+rcBoxes(10, 10))
			en := staleEntry(t, e, 1010)
			st := state{RecycleHistory: []recycleHistoryEntry{en}, LastCheck: &checkRecord{Checkpoint: tc.cp, Intent: &writeIntent{Bases: []int64{1010}}}}
			if err := writeState(e.state, st); err != nil {
				t.Fatal(err)
			}
			if msg := settleIntents(o, &Result{}, &st, e.now); msg != "" {
				t.Fatal(msg)
			}
			if c := st.LastCheck; c == nil || c.Checkpoint != tc.cp || (c.Intent == nil) != tc.cleared {
				t.Fatalf("%+v", c)
			}
			if h := readState(t, e.state).RecycleHistory; len(h) != 0 {
				t.Fatalf("the settled note is still on disk: %+v", h)
			}
		})
	}
}

// F3: a dry run says "would be", settles nothing and writes nothing, whichever way the note would be settled. With the guard that keeps the state file alone in a dry run taken away the
// state file would change.
func TestADryRunSaysWhatItWouldDoWithAStaleNoteAndWritesNothing(t *testing.T) {
	t.Run("the save is as it was", func(t *testing.T) {
		e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
		putState(t, e, staleEntry(t, e, 1010))
		o.DryRun = true
		stateBefore, saveBefore := fileBytes(t, e.state), sha(t, e.save)
		r, err := Roll(o)
		if err != nil || r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "the note would be dropped (a dry run changes nothing)")
		if strings.Contains(lines(r), "the note is dropped") || string(fileBytes(t, e.state)) != string(stateBefore) || sha(t, e.save) != saveBefore {
			t.Fatalf("a dry run settled or wrote:\n%s", lines(r))
		}
	})
	t.Run("the write took effect", func(t *testing.T) {
		e, o := tookEffect(t)
		o.DryRun = true
		stateBefore, saveBefore := fileBytes(t, e.state), sha(t, e.save)
		r, err := Roll(o)
		if err != nil || r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "It would be recorded as written (a dry run changes nothing)")
		if strings.Contains(lines(r), "It is recorded as written") || string(fileBytes(t, e.state)) != string(stateBefore) || sha(t, e.save) != saveBefore {
			t.Fatalf("a dry run settled or wrote:\n%s", lines(r))
		}
	})
	t.Run("none of the bases is gone", func(t *testing.T) {
		e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
		putState(t, e, staleEntry(t, e, 1010))
		editSave(t, e.save, `update actors set location_x=location_x+1 where id=2`)
		o.DryRun = true
		stateBefore := fileBytes(t, e.state)
		r, err := Roll(o)
		if err != nil || r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "so that write did not take effect; the note would be dropped (a dry run changes nothing)")
		if string(fileBytes(t, e.state)) != string(stateBefore) {
			t.Fatal("a dry run wrote the state file")
		}
	})
}

// F4: the reason of a missing box fits a run of one base, and says why nothing is recycled for a run of several.
func TestTheReasonForAMissingBoxFitsOneBaseAndManyBases(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...))
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "0 of 1 base(s) can be recycled now", "the boxes DD-1 and DD-2 cannot be used for the Reserve", "Nothing was recycled and nothing was exported")
	if strings.Contains(err.Error(), "which each base needs") || strings.Contains(err.Error(), "after every base") {
		t.Fatalf("a text for many bases in a run of one: %v", err)
	}
	_, o2 := mvEnv(t, rcWorld(ddBases(2)...))
	_, err = Roll(o2)
	if err == nil {
		t.Fatal("a missing box must refuse")
	}
	mustContain(t, err.Error(), "0 of 2 base(s) can be recycled now", "the boxes DD-1 and DD-2 cannot be used for the Reserve", "moved into them after every base")
}

// F5: the rule text no longer promises that a missing box recycles the bases that fit.
func TestTheReserveRuleTextSaysThatAMissingBoxRecyclesNothing(t *testing.T) {
	mustContain(t, reserveRule, "a box is missing, misnamed or otherwise cannot be used, nothing is recycled", "when there is no room in them the run recycles the bases that fit and stops before the roll")
	if strings.Contains(reserveRule, "a box is missing, or there is no room") {
		t.Fatal(reserveRule)
	}
}

// F8: the room cannot be read after the Reserve an earlier run left was moved: base 1 was never tried and is not labelled as a base that failed to recycle.
func TestARoomThatCannotBeReadAfterTheEarlierReserveWasMovedIsNotABaseThatFailed(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcReserveItem)
	armed, calls := false, 0
	realMove, realRoom := emptyReserveInto, roomOf
	swapVar(t, &emptyReserveInto, func(s *save.Save, exclude []int64, previewOnly bool) (movedReserve, error) {
		mv, err := realMove(s, exclude, previewOnly)
		if err == nil && !previewOnly {
			armed = true
		}
		return mv, err
	})
	swapVar(t, &roomOf, func(s *save.Save) (ops.RecycleRoom, error) {
		if armed {
			if calls++; calls == 2 { // the first is the check inside the move, the second the read for base 1
				return ops.RecycleRoom{}, errors.New("the save could not be read")
			}
		}
		return realRoom(s)
	})
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	out := lines(r) + err.Error()
	mustContain(t, out, "base 1010 \"Base 1\": not tried (", "the room could not be read after the Reserve was moved", "NOTHING is changed")
	if strings.Contains(out, "FAILED") || strings.Contains(out, "a base failed to recycle") {
		t.Fatalf("a base that was never tried is reported as failed:\n%s", out)
	}
}
