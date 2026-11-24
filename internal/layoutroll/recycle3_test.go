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

// Tests of the review fixes of #1194 (F1 to F8).

// realFlushThenSaying writes the save for real and then answers as the editor does when the file was replaced but could not be read back in time: not saved, the file changed.
func realFlushThenSaying(why string) func(*save.Save) save.Flush {
	return func(s *save.Save) save.Flush {
		f := s.Flush()
		if !f.Saved {
			return f
		}
		return save.Flush{Why: why}
	}
}

// F1: a write that is not reported as saved but replaced the file is a CHANGED save, with the restore note, a record of the bases, and no roll.
func TestF1AnUnconfirmedWriteThatChangedTheFileIsReportedAsChanged(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &flushSave, realFlushThenSaying("the written file could not be verified"))
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") || sha(t, e.save) == before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save WAS changed", "the write was not reported as saved", "the written file could not be verified", "NOTE: 1 base(s) WERE recycled", "Copy-Item -Force")
	stt := readState(t, e.state)
	if stt.LastCheck == nil || len(stt.LastCheck.Recycled) != 1 || stt.LastCheck.CheckpointPath == "" || len(stt.RecycleHistory) != 1 {
		t.Fatalf("%+v", stt)
	}
	if _, serr := os.Stat(stt.LastCheck.CheckpointPath); serr != nil {
		t.Fatalf("the checkpoint named is not there: %v", serr)
	}
}

// F1: a write that is not saved and left the file as it was is still "not changed", and nothing stays queued.
func TestF1AWriteThatLeftTheFileAloneIsStillNotChanged(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save was not changed")
	if strings.Contains(err.Error(), "WAS changed") {
		t.Fatalf("%v", err)
	}
}

// F1, the vehicle step: the same answer from the write of the vehicles (file replaced, not reported as saved) is a changed save too.
func TestF1TheVehicleStepReportsAnUnconfirmedWriteThatChangedTheFile(t *testing.T) {
	e, o := rcEnv(t, rcWorld()+vehicleSQL(311, "DeepDesert", 0, 1))
	swapVar(t, &flushSave, realFlushThenSaying("the written file could not be verified"))
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") || sha(t, e.save) == before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save WAS changed", "vehicle(s) taken out of the Deep Desert", "the write was not reported as saved", "Database > Save backups")
}

// F1, the vehicle step: a write that did not touch the file stays "not changed".
func TestF1TheVehicleStepWithAFileLeftAloneIsNotChanged(t *testing.T) {
	e, o := rcEnv(t, rcWorld()+vehicleSQL(311, "DeepDesert", 0, 1))
	swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the vehicles were planned but the save could not be written")
}

// fileLeftAlone itself: unreadable and unknown-before count as changed.
func TestF1FileLeftAloneJudgesWhatItCannotSeeAsChanged(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.db")
	if err := os.WriteFile(p, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := fileSHA(p)
	if err != nil {
		t.Fatal(err)
	}
	if same, _ := fileLeftAlone(p, h); !same {
		t.Fatal("an untouched file is the same")
	}
	if same, why := fileLeftAlone(p, ""); same || why == "" {
		t.Fatal("a file whose earlier state is not known may have changed")
	}
	if err := os.WriteFile(p, []byte("abd"), 0o600); err != nil {
		t.Fatal(err)
	}
	if same, why := fileLeftAlone(p, h); same || why == "" {
		t.Fatal("a different file is changed")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if same, why := fileLeftAlone(p, h); same || why == "" {
		t.Fatal("a file that cannot be read may have changed")
	}
}

// F2: the state file holds the checkpoint and the recycled bases from the moment the save was written, before the read-back (a process that died there would still leave them).
func TestF2TheStateIsWrittenRightAfterTheWriteBeforeTheReadBack(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	var seen state
	swapVar(t, &verifyBasesWritten, func(string, []recycledRecord, []ops.HoldBase, writtenExpect) error {
		seen = readState(t, e.state)
		return errors.New("stop here")
	})
	if _, err := Roll(o); err == nil {
		t.Fatal("it should stop")
	}
	if seen.LastCheck == nil || len(seen.LastCheck.Recycled) != 1 || seen.LastCheck.CheckpointPath == "" || len(seen.RecycleHistory) != 1 || seen.LastRun == nil || !strings.Contains(seen.LastRun.Outcome, "in progress") {
		t.Fatalf("the state was not written before the read-back: %+v", seen)
	}
}

// F3, verifyBasesWritten, the count of the bases that stand: one of the remaining bases is gone from the written file.
func TestF3TheWrittenFileWithAMissingRemainingBaseIsRefused(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := s.Flush()
		editSave(t, e.save, `delete from permission_actor_rank where permission_actor_id=1020`)
		return f
	})
	_, err := Roll(o)
	if err == nil {
		t.Fatal("it should stop")
	}
	mustContain(t, err.Error(), "the save WAS changed", "stand in the Deep Desert, and 1 were expected")
}

// F3, verifyBasesWritten, a base of someone else in the written file.
func TestF3TheWrittenFileWithAnotherOwnersBaseIsRefused(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := s.Flush()
		editSave(t, e.save, rcBase(9, "DeepDesert", 1), `delete from permission_actor_rank where permission_actor_id in (1090,1092)`)
		return f
	})
	_, err := Roll(o)
	if err == nil {
		t.Fatal("it should stop")
	}
	mustContain(t, err.Error(), "the save WAS changed", "other base(s) stand in the Deep Desert")
}

// F3, verifyBasesWritten, the Reserve of the written file differs from the plan.
func TestF3TheWrittenFileWithAStrayReserveRowIsRefused(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &flushSave, func(s *save.Save) save.Flush {
		f := s.Flush()
		editSave(t, e.save, `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8800,12,1,40,'CopperOre',0,1790000000,'{}',0,null)`)
		return f
	})
	_, err := Roll(o)
	if err == nil {
		t.Fatal("it should stop")
	}
	mustContain(t, err.Error(), "the save WAS changed", "the Reserve holds", "were planned")
}

// F3, applyEach, the Reserve after a recycle on the working copy differs from what the recycle reported: the whole run stops and the file is untouched.
func TestF3AReserveThatDoesNotMatchTheRecycleStopsTheRun(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &recycleOne, func(s *save.Save, id int64, fp string) (map[string]any, error) {
		out, err := (&ops.Ops{S: s}).RecycleBaseWith(id, fp, ops.RecycleOptions{Solido: true})
		if err != nil {
			return nil, err
		}
		if _, err := s.Mutate("a stray row", func(m *save.Mut) error {
			_, err := m.Exec(`insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8801,12,1,41,'CopperOre',0,1790000000,'{}',0,null)`)
			return err
		}); err != nil {
			return nil, err
		}
		return asMap(out, nil)
	})
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "stacks after the recycle", "were planned", "NOTHING is changed")
}

// F3, the guard: a base that still stands after everything was written and read back stops the roll. The read-back is made to pass, so only the second look can find it.
func TestF3ABaseLeftStandingAfterTheRecycleStopsTheRoll(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &verifyBasesWritten, func(string, []recycledRecord, []ops.HoldBase, writtenExpect) error { return nil })
	swapVar(t, &recycleOne, func(s *save.Save, id int64, fp string) (map[string]any, error) {
		out, err := (&ops.Ops{S: s}).RecycleBaseWith(id, fp, ops.RecycleOptions{Solido: true})
		if err != nil {
			return nil, err
		}
		// a base of the player appears in the Deep Desert as the recycle ends (it is not one of those the run knew about)
		if _, err := s.Mutate("a base left", func(m *save.Mut) error {
			_, err := m.Exec(rcBase(9, "DeepDesert", 1))
			return err
		}); err != nil {
			return nil, err
		}
		return asMap(out, nil)
	})
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the check after the recycle still finds 1 base(s)", "NOTE: 1 base(s) WERE recycled")
}

// F4: every run that recycled bases adds an entry to the history of the state file (checkpoint, base ids, Solido device ids), and the next runs list the earlier checkpoints.
func TestF4TheStateKeepsTheCheckpointOfEveryRunThatRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(3)...))
	var firstCheckpoint string
	for run := 1; run <= 3; run++ {
		e.now = e.now.Add(time.Minute)
		r, err := Roll(o)
		if run < 3 && err == nil {
			t.Fatalf("run %d should stop before the roll", run)
		}
		if run == 3 && (err != nil || !r.Wrote) {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		stt := readState(t, e.state)
		if len(stt.RecycleHistory) != run {
			t.Fatalf("run %d: %d history entries: %+v", run, len(stt.RecycleHistory), stt.RecycleHistory)
		}
		h := stt.RecycleHistory[run-1]
		if h.Checkpoint == "" || h.CheckpointPath == "" || len(h.Bases) != 1 || h.Bases[0] != int64(1000+10*run) || len(h.SolidoItems) != 1 || h.SolidoItems[0] == 0 {
			t.Fatalf("run %d: %+v", run, h)
		}
		if run == 1 {
			firstCheckpoint = h.Checkpoint
		}
		if run > 1 {
			if !strings.Contains(lines(r)+fmt.Sprint(err), firstCheckpoint) {
				t.Fatalf("run %d does not list the checkpoint of the first run:\n%s\n%v", run, lines(r), err)
			}
			if stt.RecycleHistory[0].Checkpoint != firstCheckpoint {
				t.Fatalf("the first entry was lost: %+v", stt.RecycleHistory)
			}
		}
		editSave(t, e.save, `delete from items where inventory_id=12`)
	}
}

// F4: the history is bounded.
func TestF4TheHistoryIsBounded(t *testing.T) {
	var s state
	for i := 0; i < maxRecycleHistory+5; i++ {
		s.recordRecycleRun(&checkRecord{At: time.Unix(int64(1000+i), 0), Checkpoint: fmt.Sprint("cp", i)})
	}
	if len(s.RecycleHistory) != maxRecycleHistory || s.RecycleHistory[0].Checkpoint != "cp5" {
		t.Fatalf("%d %+v", len(s.RecycleHistory), s.RecycleHistory[0])
	}
	// the same run again replaces its entry
	s.recordRecycleRun(&checkRecord{At: time.Unix(int64(1000+maxRecycleHistory+4), 0), Checkpoint: "again"})
	if len(s.RecycleHistory) != maxRecycleHistory || s.RecycleHistory[len(s.RecycleHistory)-1].Checkpoint != "again" {
		t.Fatalf("%+v", s.RecycleHistory[len(s.RecycleHistory)-1])
	}
}

// F5: a dry run says that it takes a new checkpoint, not "the checkpoint of this run is ." with no name; the text for making room takes no checkpoint.
func TestF5TheDryRunNeverPrintsAnEmptyCheckpointName(t *testing.T) {
	_, o := rcEnv(t, rcWorld(ddBases(3)...))
	o.DryRun = true
	r, _ := Roll(o)
	out := lines(r)
	if strings.Contains(out, "checkpoint of this run is .") || strings.Contains(out, "checkpoint of this run is  ") || !strings.Contains(out, "checkpoint of this run is a new one") {
		t.Fatalf("%s", out)
	}
}

// F6: a base whose totem is on the Deep Desert but whose building actor is on another map is not recycled.
func TestF6ABaseWithAnActorOffTheDeepDesertIsNotRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+"update actors set map='HaggaBasin' where id=1011;")
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "has actors that are not on the Deep Desert", "nothing was recycled")
}

// F6: the map must be exactly the Deep Desert; a name that only starts with it is not.
func TestF6AMapThatOnlyStartsWithDeepDesertIsNotTheDeepDesert(t *testing.T) {
	for _, c := range []struct {
		mp   string
		want bool
	}{{"DeepDesert", true}, {"deepdesert", true}, {" DeepDesert ", true}, {"DeepDesert_Travel", false}, {"DeepDesert2", false}, {"", false}, {"HaggaBasin", false}} {
		if got := isDeepDesert(c.mp); got != c.want {
			t.Errorf("isDeepDesert(%q) = %v", c.mp, got)
		}
	}
	e, o := rcEnv(t, rcWorld(rcBase(1, "DeepDesert_Travel", 1)))
	before := sha(t, e.save)
	_, err := Roll(o)
	// what the guard counts as a base in the Deep Desert is the guard's own definition; the recycle step adds the exact match on top of it
	if sha(t, e.save) != before {
		t.Fatalf("a base on a map that is not exactly the Deep Desert was written: %v", err)
	}
}

// F7: what the recycle itself says about each base (its summary and its warnings) is in the run's output.
func TestF7TheRecyclesOwnSummaryAndWarningsAreInTheOutput(t *testing.T) {
	_, o := rcEnv(t, rcWorld(ddBases(1)...))
	swapVar(t, &recycleOne, func(s *save.Save, id int64, fp string) (map[string]any, error) {
		v, err := (&ops.Ops{S: s}).RecycleBaseWith(id, fp, ops.RecycleOptions{Solido: true})
		out, err := asMap(v, err)
		if err != nil {
			return nil, err
		}
		out["summary"] = append(append([]string(nil), toStrings(out["summary"])...), "a summary line of the test")
		out["warnings"] = append(append([]string(nil), toStrings(out["warnings"])...), "a warning of the test")
		return out, nil
	})
	r, err := Roll(o)
	if err != nil {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "a summary line of the test", "Warning: a warning of the test")
}

func toStrings(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		var o []string
		for _, e := range x {
			o = append(o, fmt.Sprint(e))
		}
		return o
	}
	return nil
}

// F8: a run that is refused for lack of room leaves no export folder (and no export files), only the checkpoint.
func TestF8ARunRefusedForRoomExportsNothing(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(2)...)+rcReserveItem)
	before := sha(t, e.save)
	_, err := Roll(o)
	if err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(e.state), exportFolderPrefix+"*")); len(m) != 0 {
		t.Fatalf("an export folder was left: %v", m)
	}
	if len(checkpoints(t, e.save)) != 1 {
		t.Fatal("the checkpoint is taken before anything else")
	}
	// the run that fits exports (the Reserve is empty)
	editSave(t, e.save, `delete from items where inventory_id=12`)
	e.now = e.now.Add(time.Minute)
	if _, err := Roll(o); err == nil {
		t.Fatal("one of two bases only")
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(e.state), exportFolderPrefix+"*")); len(m) != 1 {
		t.Fatalf("the export of the run that recycled: %v", m)
	}
}
