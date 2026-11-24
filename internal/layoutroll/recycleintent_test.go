package layoutroll

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The review round of #1200: the unconfirmed intent of an earlier run is settled by the next run, the history cap, the warnings that must reach the state file, the wording, and the
// gaps of the first round.

// staleEntry is the unconfirmed entry an earlier run leaves behind, for the save file as it is now.
func staleEntry(t *testing.T, e *env, bases ...int64) recycleHistoryEntry {
	t.Helper()
	h, err := fileSHA(e.save)
	if err != nil {
		t.Fatal(err)
	}
	en := recycleHistoryEntry{At: e.now.Add(-time.Hour), Checkpoint: "cp-earlier.db", CheckpointPath: "C:\\x\\cp-earlier.db", SaveFile: e.save, Bases: bases, Unconfirmed: true, BeforeSHA256: h}
	for i := range bases {
		en.SolidoItems = append(en.SolidoItems, int64(9100+i))
		en.SolidoBlueprints = append(en.SolidoBlueprints, int64(1+i))
	}
	return en
}

func putState(t *testing.T, e *env, en recycleHistoryEntry) {
	t.Helper()
	if err := writeState(e.state, state{RecycleHistory: []recycleHistoryEntry{en}}); err != nil {
		t.Fatal(err)
	}
}

// A run that died before the write left an unconfirmed note, and the save file is exactly as it was: the next run drops the note and goes on.
func TestAStaleIntentWithTheSaveUnchangedIsDroppedAndTheRunGoesOn(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	putState(t, e, staleEntry(t, e, 1010))
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "Earlier run settled", "the write did not happen; the note is dropped")
	stt := readState(t, e.state)
	if len(stt.RecycleHistory) != 1 || stt.RecycleHistory[0].Unconfirmed || stt.RecycleHistory[0].Checkpoint == "cp-earlier.db" {
		t.Fatalf("%+v", stt.RecycleHistory)
	}
}

// The same note with no base recycled at all (the early return of a run with nothing to recycle) is settled before it, and a Reserve left over stays where it is.
func TestAStaleIntentIsSettledEvenWhenNoBaseStands(t *testing.T) {
	e, o := mvEnv(t, rcWorld(rcBase(1, "HaggaBasin", 1))+rcBoxes(10, 10)+rcReserveItem)
	putState(t, e, staleEntry(t, e, 1010))
	before := sha(t, e.save)
	r, err := Roll(o)
	if err != nil || !r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "Earlier run settled", "no base stands in the Deep Desert, so nothing is recycled")
	if inInv(t, e.save, 12) != 1 {
		t.Fatal("the leftover Reserve was touched although no base was recycled")
	}
	if h := readState(t, e.state).RecycleHistory; len(h) != 0 {
		t.Fatalf("%+v", h)
	}
}

// A write that took effect and was never recorded (the state file could not be written afterwards): the next run finds the planned base gone with its Solido in place, says so loudly,
// records it as written and goes on.
func TestAnUnconfirmedWriteThatTookEffectIsRecordedLoudlyByTheNextRun(t *testing.T) {
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
	if got := standingIDs(t, e.save); fmt.Sprint(got) != "[1020]" {
		t.Fatalf("standing %v", got)
	}
	if h := readState(t, e.state).RecycleHistory; len(h) != 1 || !h[0].Unconfirmed || len(h[0].SolidoBlueprints) != 1 {
		t.Fatalf("the state file does not hold the unconfirmed note: %+v", h)
	}
	fail = false
	editSave(t, e.save, `update inventories set max_item_count=10 where id in (7301,7302)`) // the game saved since: the file differs from before
	e.now = e.now.Add(time.Minute)
	r, err := Roll(o)
	if err != nil || !r.Wrote || len(standingIDs(t, e.save)) != 0 {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "WARNING: the run of", "every one of the 1 planned base(s) is gone with its Solido in place", "recorded as written")
	stt := readState(t, e.state)
	if len(stt.RecycleHistory) != 2 || stt.RecycleHistory[0].Unconfirmed || stt.RecycleHistory[1].Unconfirmed {
		t.Fatalf("%+v", stt.RecycleHistory)
	}
}

// A different file with none of the planned bases gone: the write did not take effect; the note is dropped, loudly, and the run goes on with the base that still stands.
func TestAnUnconfirmedNoteWithADifferentFileAndNoBaseGoneIsDroppedLoudly(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	en := staleEntry(t, e, 1010)
	putState(t, e, en)
	editSave(t, e.save, `update actors set location_x=location_x+1 where id=2`) // the game saved since
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "WARNING: the run of", "none of the 1 planned base(s) is gone, so that write did not take effect")
	if h := readState(t, e.state).RecycleHistory; len(h) != 1 || h[0].Unconfirmed || h[0].Checkpoint == "cp-earlier.db" {
		t.Fatalf("%+v", h)
	}
}

// A note whose planned bases are part gone and part standing cannot be settled: the run stops, changes nothing and does not roll.
func TestAnUnconfirmedNoteThatCannotBeSettledStopsTheRunBeforeAnythingElse(t *testing.T) {
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
	// the note is made to say that BOTH bases were planned; only 1010 is gone
	stt := readState(t, e.state)
	en := stt.RecycleHistory[0]
	en.Bases = append(en.Bases, 1020)
	en.SolidoItems = append(en.SolidoItems, 9999)
	en.SolidoBlueprints = append(en.SolidoBlueprints, 9)
	stt.RecycleHistory = []recycleHistoryEntry{en}
	if err := writeState(e.state, stt); err != nil {
		t.Fatal(err)
	}
	editSave(t, e.save, `update inventories set max_item_count=10 where id in (7301,7302)`)
	before := sha(t, e.save)
	e.now = e.now.Add(time.Minute)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "noted that it was about to write the save", "1 of the 2 planned base(s) are gone and 1 still stand", "Look at the save", "remove the entry marked \"unconfirmed\"")
	if h := readState(t, e.state).RecycleHistory; len(h) != 1 || !h[0].Unconfirmed {
		t.Fatalf("the note must stay: %+v", h)
	}
}

// A dry run settles nothing and writes nothing, and says what a real run would stop for.
func TestADryRunDoesNotSettleAStaleNote(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	putState(t, e, recycleHistoryEntry{At: e.now.Add(-time.Hour), Checkpoint: "cp-earlier.db", Unconfirmed: true, Bases: []int64{1010}})
	o.DryRun = true
	stateBefore := fileBytes(t, e.state)
	r, err := Roll(o)
	if err != nil || r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "Dry run: it would stop here", "does not say which save file it was")
	if string(fileBytes(t, e.state)) != string(stateBefore) {
		t.Fatal("a dry run wrote the state file")
	}
}

// The history cap: noting an intent does not push the oldest checkpoint out before anything is written; confirming it does.
func TestTheHistoryCapDoesNotCountAnIntent(t *testing.T) {
	var s state
	for i := 0; i < maxRecycleHistory; i++ {
		s.recordRecycleRun(&checkRecord{At: time.Unix(int64(1000+i), 0), Checkpoint: fmt.Sprint("cp", i)})
	}
	if len(s.RecycleHistory) != maxRecycleHistory {
		t.Fatalf("%d", len(s.RecycleHistory))
	}
	rec := &checkRecord{At: time.Unix(5000, 0), Checkpoint: "cp-new", Intent: &writeIntent{Bases: []int64{1010}, BeforeSHA256: "abc"}}
	s.recordRecycleRun(rec)
	if len(s.RecycleHistory) != maxRecycleHistory+1 || s.RecycleHistory[0].Checkpoint != "cp0" || !s.RecycleHistory[maxRecycleHistory].Unconfirmed || s.RecycleHistory[maxRecycleHistory].BeforeSHA256 != "abc" {
		t.Fatalf("the intent evicted the oldest entry: %d, first %s", len(s.RecycleHistory), s.RecycleHistory[0].Checkpoint)
	}
	// the write was not done: the intent is taken back, nothing was lost
	s.dropRecycleRun(rec.At)
	if len(s.RecycleHistory) != maxRecycleHistory || s.RecycleHistory[0].Checkpoint != "cp0" {
		t.Fatalf("%d", len(s.RecycleHistory))
	}
	// the write was confirmed: now the cap applies
	s.recordRecycleRun(rec)
	rec.Intent, rec.Recycled = nil, []recycledRecord{{ID: 1010, SolidoItem: 5}}
	s.recordRecycleRun(rec)
	if len(s.RecycleHistory) != maxRecycleHistory || s.RecycleHistory[0].Checkpoint != "cp1" || s.RecycleHistory[maxRecycleHistory-1].Unconfirmed {
		t.Fatalf("%d, first %s", len(s.RecycleHistory), s.RecycleHistory[0].Checkpoint)
	}
}

// A state file that cannot be written after the intent was withdrawn (the save was not written) is not silent: the file on disk still holds the note, and the run says so.
func TestAFailedStateWriteAfterAWithdrawnIntentIsSaidLoudly(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
	swapVar(t, &writeStateFile, func(p string, s state) error {
		if s.LastRun != nil && !strings.Contains(s.LastRun.Outcome, "about to write") {
			return errors.New("the state file is locked")
		}
		return writeState(p, s)
	})
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save was not changed", "WARNING: the state file", "the state file is locked", "still holds the note that the save was about to be written")
	mustContain(t, lines(r), "WARNING: the state file")
}

// reserveMoves in the state file lists only moves that were written: a run whose write did not happen records none.
func TestReserveMovesAreRecordedOnlyWhenTheyWereWritten(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10))
	swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
	before := sha(t, e.save)
	if _, err := Roll(o); err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	c := readState(t, e.state).LastCheck
	if c == nil || len(c.ReserveMoves) != 0 || len(c.Recycled) != 0 || c.Intent != nil || c.ReserveNote != "" {
		t.Fatalf("%+v", c)
	}
	if h := readState(t, e.state).RecycleHistory; len(h) != 0 {
		t.Fatalf("the withdrawn intent is still in the history: %+v", h)
	}
}

// With a leftover Reserve and no base standing, nothing is moved and nothing is written: the Reserve stays as it is.
func TestZeroBasesWithALeftoverReserveMovesNothing(t *testing.T) {
	e, o := mvEnv(t, rcWorld()+rcBoxes(10, 10)+rcReserveItem)
	before := sha(t, e.save)
	r, err := Roll(o)
	if err != nil || !r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "no base stands in the Deep Desert, so nothing is recycled")
	if inInv(t, e.save, 12) != 1 || inInv(t, e.save, 7301) != 0 {
		t.Fatal("the Reserve was moved with no base to recycle")
	}
}

// A dry run with a Reserve an earlier run left: the plan has the move before the first base, and nothing is written.
func TestADryRunWithALeftoverReservePlansTheMoveFirst(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcReserveItem)
	o.DryRun = true
	before := sha(t, e.save)
	r, err := Roll(o)
	if err != nil || r.Wrote || sha(t, e.save) != before {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	out := lines(r)
	mustContain(t, out, "The Reserve would be moved into the boxes DD-1 and DD-2 (before the first base (what an earlier run left)): 1 stack(s)", "2 of 2 base(s) can be recycled in this run")
	if strings.Count(out, "would be moved into the boxes") != 3 {
		t.Fatalf("a move before the first base and one after each base were expected:\n%s", out)
	}
	if inInv(t, e.save, 12) != 1 {
		t.Fatal("a dry run moved the Reserve")
	}
}

// The REAL move is made after base 1 and then something is wrong with what it left (a stray stack is in the Reserve again: the check after the move fails; the save layer of the test
// database does not allow a trigger that could make the real move itself fail): the run stops, nothing is written, and the output does not call base 1 a base that failed to recycle.
func TestARealMoveThatLeavesTheReserveFullAfterBase1IsNotReportedAsABaseThatFailed(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10))
	real := emptyReserveInto
	swapVar(t, &emptyReserveInto, func(s *save.Save, exclude []int64, previewOnly bool) (movedReserve, error) {
		mv, err := real(s, exclude, previewOnly)
		if err == nil && !previewOnly && mv.Stacks > 0 {
			_, err = s.Mutate("a stray stack", func(m *save.Mut) error {
				_, err := m.Exec(`insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8802,12,1,41,'CopperOre',0,1790000000,'{}',0,null)`)
				return err
			})
		}
		return mv, err
	})
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "the Reserve could not be moved into the boxes after base 1010", "still holds 1 stack(s)", "NOTHING is changed")
	out := lines(r) + err.Error()
	mustContain(t, out, "recycled and verified in the working copy only (discarded); then moving its Reserve into the boxes FAILED", "base 1020 \"Base 2\": not tried")
	if strings.Contains(out, "a base failed to recycle") || strings.Contains(out, "base 1010 \"Base 1\": FAILED") {
		t.Fatalf("the move is reported as a failed base:\n%s", out)
	}
}

// A file that cannot be read after an unconfirmed write is not called a different file.
func TestAnUnreadableFileAfterAnUnconfirmedWriteIsNotCalledDifferent(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
	calls := 0
	real := fileSHA
	swapVar(t, &fileSHA, func(p string) (string, error) {
		calls++
		if calls > 1 {
			return "", errors.New("access denied")
		}
		return real(p)
	})
	swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the written file could not be verified"} })
	_, err := Roll(o)
	if err == nil || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "NOT known what happened to the save file", "the save file cannot be read now (access denied)")
	if strings.Contains(err.Error(), "is different from before the write") {
		t.Fatalf("a file that could not be read is called different: %v", err)
	}
}

func fileBytes(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
