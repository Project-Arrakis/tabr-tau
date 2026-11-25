package layoutroll

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The Deep Desert markers step of the roll (#1216), on made-up saves of both layouts of the markers tables (by map name and by map id). Nothing here touches a real save or Game.ini.

// mkWorld: markers on four maps (one map's name only starts like the Deep Desert's), what the character discovered on each, and resource fields. Written for the older layout; testsave
// converts it for the newer one. The Deep Desert has 3 markers, 2 discovered rows and 3 resource-field rows.
const mkWorld = `
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
  (1,0,'HaggaBasin','AzuritePickup',-1000,2000,0),
  (2,0,'HaggaBasin','EnemyCamp',3000,4000,0),
  (1,0,'DeepDesert','Ecolab',5,5,0),
  (3,0,'DeepDesert','Cave',6,6,0),
  (4,1,'DeepDesert','SpiceField',7,7,0),
  (9,0,'DeepDesertTravel','Gate',8,8,0),
  (10,0,'Arrakeen','Vendor',9,9,0);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values
  (1,1,0,'HaggaBasin',3,1),
  (1,1,0,'DeepDesert',2,1),
  (1,3,0,'DeepDesert',1,1),
  (1,9,0,'DeepDesertTravel',1,1),
  (1,10,0,'Arrakeen',1,1);
insert into resourcefield_state(field_id,map,dimension_index,spawn_time,value_remaining) values
  (10,'HaggaBasin',0,100.5,40),
  (11,'DeepDesert',0,100.5,50),
  (12,'DeepDesert',0,200.5,60),
  (11,'DeepDesert',1,300.5,70),
  (13,'DeepDesertTravel',0,100.5,80),
  (11,'Arrakeen',0,100.5,90);`

const mkWords = "3 marker(s), 2 discovered row(s) and 3 resource-field row(s)"

// inBothLayouts runs a test against a save of each layout of the markers tables (a world written for the older layout, plus extra statements).
func inBothLayouts(t *testing.T, world string, fn func(t *testing.T, e *env)) {
	t.Helper()
	for _, l := range testsave.Layouts {
		t.Run(l.String(), func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQLIn(t, l, world).Path
			fn(t, e)
		})
	}
}

// survey counts the three tables of the save file, the Deep Desert's rows and the others'.
func survey(t *testing.T, path string) ops.MarkerSurvey {
	t.Helper()
	s, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	sv, err := (&ops.Ops{S: s}).SurveyDeepDesertMarkers()
	if err != nil {
		t.Fatal(err)
	}
	return sv
}

// othersDump is every row of markers, player_markers and resourcefield_state that is not of the Deep Desert, whole, in a fixed order.
func othersDump(t *testing.T, path string) []save.Row {
	t.Helper()
	s, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var out []save.Row
	for _, q := range []string{
		`select m.* from ` + s.MarkersTable() + ` m where m.map_name is not 'DeepDesert' order by m.map_name, m.marker_hash_id, m.dimension_index`,
		`select p.* from ` + s.PlayerMarkersTable() + ` p where p.map_name is not 'DeepDesert' order by p.map_name, p.marker_hash_id, p.dimension_index, p.player_id`,
		`select * from resourcefield_state where map is not 'DeepDesert' order by map, field_id, dimension_index`,
	} {
		rows, err := s.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rows...)
	}
	return out
}

// actorsDump is the actors, whole.
func actorsDump(t *testing.T, path string) []save.Row {
	t.Helper()
	s, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.Query(`select * from actors order by id`)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func logText(t *testing.T, e *env) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.Dir(e.state), LogName))
	if err != nil {
		return ""
	}
	return string(b)
}

// A real roll removes the Deep Desert's rows of the three tables and nothing else, after the checkpoint and before Game.ini, and says what it removed in its output, the state file and
// the log.
func TestARealRollRemovesOnlyTheDeepDesertMarkers(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		orig, _ := os.ReadFile(e.save)
		othersBefore, actorsBefore := othersDump(t, e.save), actorsDump(t, e.save)
		if len(othersBefore) < 10 {
			t.Fatalf("the test world has %d rows of other maps", len(othersBefore))
		}
		r, err := Roll(e.opts())
		out := lines(r)
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, out)
		}
		sv := survey(t, e.save)
		if sv.DeepDesert.Total() != 0 || sv.Others != (ops.MarkerCounts{Markers: 4, Discovered: 3, ResourceFields: 3}) {
			t.Fatalf("after the roll: %+v", sv)
		}
		if !reflect.DeepEqual(othersDump(t, e.save), othersBefore) || !reflect.DeepEqual(actorsDump(t, e.save), actorsBefore) {
			t.Fatal("a row of another map (or an actor) changed")
		}
		// the order: checkpoint -> markers -> Game.ini
		last := -1
		for _, want := range []string{"Checkpoint of the save:", "Removed the Deep Desert markers from the save", mkWords, "Changed Game.ini from layout 11"} {
			i := strings.Index(out, want)
			if i < 0 || i < last {
				t.Fatalf("%q is missing or out of order:\n%s", want, out)
			}
			last = i
		}
		// the way back holds the markers
		cps := checkpoints(t, e.save)
		if len(cps) != 1 {
			t.Fatalf("checkpoints: %v", cps)
		}
		if b, _ := os.ReadFile(cps[0]); string(b) != string(orig) {
			t.Fatal("the checkpoint is not the save as it was")
		}
		if sv := survey(t, cps[0]); sv.DeepDesert != (ops.MarkerCounts{Markers: 3, Discovered: 2, ResourceFields: 3}) {
			t.Fatalf("the checkpoint: %+v", sv)
		}
		// the state file and the log say what was removed
		stt := readState(t, e.state)
		if stt.LastCheck == nil || stt.LastCheck.Markers != mkWords || !strings.Contains(stt.LastRun.Outcome, "removed the Deep Desert markers: "+mkWords) || stt.LastRoll == nil {
			t.Fatalf("%+v %+v", stt.LastCheck, stt.LastRun)
		}
		if lg := logText(t, e); !strings.Contains(lg, "removed the Deep Desert markers from the save ("+mkWords+")") || !strings.Contains(lg, stt.LastCheck.Checkpoint) {
			t.Fatalf("log: %q", lg)
		}
	})
}

// A dry run says how many rows it would remove and writes nothing: not the save, not the state file, not the log.
func TestADryRunCountsTheMarkersAndWritesNothing(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		before := sha(t, e.save)
		o := e.opts()
		o.DryRun = true
		r, err := Roll(o)
		out := lines(r)
		if err != nil || r.Wrote {
			t.Fatalf("%v\n%s", err, out)
		}
		mustContain(t, out, "Dry run: would remove the Deep Desert markers from the save", mkWords, "Dry run: would change Game.ini")
		if sha(t, e.save) != before || e.ini() != iniWith("11") {
			t.Fatal("a dry run changed the save or Game.ini")
		}
		if _, err := os.Stat(e.state); err == nil {
			t.Fatal("a dry run wrote the state file")
		}
		if len(checkpoints(t, e.save)) != 0 || logText(t, e) != "" {
			t.Fatal("a dry run wrote a checkpoint or the log")
		}
		if sv := survey(t, e.save); sv.DeepDesert.Total() != 8 {
			t.Fatalf("%+v", sv)
		}
	})
}

// --keep-markers leaves them, in a real run and in a dry run, and the roll goes on.
func TestKeepMarkersLeavesThem(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		o := e.opts()
		o.KeepMarkers = true
		o.DryRun = true
		r, err := Roll(o)
		if err != nil || !strings.Contains(lines(r), "left as they are (--keep-markers)") || strings.Contains(lines(r), mkWords) {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		o.DryRun = false
		r, err = Roll(o)
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		if sv := survey(t, e.save); sv.DeepDesert != (ops.MarkerCounts{Markers: 3, Discovered: 2, ResourceFields: 3}) {
			t.Fatalf("the markers were removed: %+v", sv)
		}
		if stt := readState(t, e.state); stt.LastCheck.Markers != "" || strings.Contains(stt.LastRun.Outcome, "markers") {
			t.Fatalf("%+v", stt)
		}
		if logText(t, e) != "" {
			t.Fatalf("the log says something was removed: %q", logText(t, e))
		}
	})
}

// A save with no Deep Desert markers: nothing is removed, the save is not written, and the roll goes on.
func TestASaveWithNoDeepDesertMarkersIsNotWritten(t *testing.T) {
	world := strings.NewReplacer("'DeepDesert',", "'HaggaBasin2',").Replace(mkWorld)
	inBothLayouts(t, world, func(t *testing.T, e *env) {
		before := sha(t, e.save)
		r, err := Roll(e.opts())
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, lines(r))
		}
		mustContain(t, lines(r), "Deep Desert markers: none to remove.")
		if sha(t, e.save) != before {
			t.Fatal("the save was written although there was nothing to remove")
		}
		if readState(t, e.state).LastCheck.Markers != "" || logText(t, e) != "" {
			t.Fatal("something was recorded as removed")
		}
	})
}

// The read inside the edit fails (or the edit does): the save is as it was, Game.ini is not changed, and the run says so.
func TestAnEditThatFailsLeavesTheSaveAndGameIniAsTheyWere(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		swapVar(t, &clearMarkers, func(*save.Save) (ops.MarkerSurvey, error) { return ops.MarkerSurvey{}, errors.New("read back: the Deep Desert still has 1 marker row(s)") })
		before := sha(t, e.save)
		r, err := Roll(e.opts())
		if err == nil || r.Wrote {
			t.Fatalf("%v", err)
		}
		mustContain(t, err.Error(), "not changed", "the Deep Desert markers could not be removed", "still has 1 marker")
		if sha(t, e.save) != before || e.ini() != iniWith("11") {
			t.Fatal("the save or Game.ini changed")
		}
		stt := readState(t, e.state)
		if stt.LastRoll != nil || !strings.Contains(stt.LastRun.Outcome, "the Deep Desert markers could not be removed") {
			t.Fatalf("%+v", stt)
		}
		// nothing is left queued: a run without the fault removes them
		swapVar(t, &clearMarkers, func(s *save.Save) (ops.MarkerSurvey, error) { return (&ops.Ops{S: s}).ClearDeepDesertMarkers() })
		e.now = e.now.Add(time.Minute)
		if r, err := Roll(e.opts()); err != nil || !r.Wrote || survey(t, e.save).DeepDesert.Total() != 0 {
			t.Fatalf("%v\n%s", err, lines(r))
		}
	})
}

// A write that is not saved changes nothing.
func TestAMarkersWriteThatIsNotSavedChangesNothing(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
		before := sha(t, e.save)
		_, err := Roll(e.opts())
		if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
			t.Fatalf("%v", err)
		}
		mustContain(t, err.Error(), "the disk is full", "the save was not changed")
		if sv := survey(t, e.save); sv.DeepDesert.Total() != 8 {
			t.Fatalf("%+v", sv)
		}
	})
}

// The written file that does not read back as cleared is reported with the way back; Game.ini is not rolled.
func TestAWrittenSaveThatDoesNotReadBackAsClearedIsReported(t *testing.T) {
	inBothLayouts(t, mkWorld, func(t *testing.T, e *env) {
		swapVar(t, &verifyMarkers, func(string, ops.MarkerSurvey) error { return errors.New("read back: the Deep Desert still has 2 marker row(s)") })
		r, err := Roll(e.opts())
		if err == nil || r.Wrote || e.ini() != iniWith("11") {
			t.Fatalf("%v", err)
		}
		mustContain(t, err.Error(), "the save WAS changed", "does not read back as cleared", "Copy-Item -Force", "NOTE: the Deep Desert markers", mkWords)
		stt := readState(t, e.state)
		if stt.LastCheck == nil || stt.LastCheck.Markers != mkWords || stt.LastCheck.CheckpointPath == "" || stt.LastRoll != nil || !strings.Contains(stt.LastRun.Outcome, "does not read back") {
			t.Fatalf("%+v %+v", stt.LastCheck, stt.LastRun)
		}
		if _, serr := os.Stat(stt.LastCheck.CheckpointPath); serr != nil {
			t.Fatalf("the checkpoint named is not there: %v", serr)
		}
	})
}

// A run that stops before the write of Game.ini for any other reason has not touched the markers: the game starts during the run, the weekly rule skips it, a base blocks it.
func TestARunThatStopsEarlierHasNotClearedTheMarkers(t *testing.T) {
	t.Run("the game starts during the run", func(t *testing.T) {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, mkWorld).Path
		o := e.opts()
		calls := 0
		o.GameRunning = func() (bool, string) { calls++; return calls >= 2, "Dune.exe is running" } // not at the first check, but before the write
		before := sha(t, e.save)
		if _, err := Roll(o); err == nil {
			t.Fatal("no refusal")
		}
		if sha(t, e.save) != before || e.ini() != iniWith("11") {
			t.Fatal("something was written")
		}
	})
	t.Run("the weekly rule skips the run", func(t *testing.T) {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, mkWorld).Path
		o := e.opts()
		o.KeepMarkers = true
		if _, err := Roll(o); err != nil { // a roll this week
			t.Fatal(err)
		}
		before := sha(t, e.save)
		o.KeepMarkers = false
		e.now = e.now.Add(time.Hour)
		r, err := Roll(o)
		if err != nil || r.Wrote || !strings.Contains(lines(r), "Skipped") || sha(t, e.save) != before || survey(t, e.save).DeepDesert.Total() != 8 {
			t.Fatalf("%v\n%s", err, lines(r))
		}
	})
	t.Run("a base blocks the roll", func(t *testing.T) {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, mkWorld+baseInDD).Path
		before := sha(t, e.save)
		_, err := Roll(e.opts())
		if err == nil || sha(t, e.save) != before || survey(t, e.save).DeepDesert.Total() != 8 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("the game is running", func(t *testing.T) {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, mkWorld).Path
		e.running = true
		before := sha(t, e.save)
		if _, err := Roll(e.opts()); err == nil || sha(t, e.save) != before {
			t.Fatalf("%v", err)
		}
	})
}

// A save the markers cannot be counted in (the tables are not as the editor knows them) stops the run before Game.ini and changes nothing.
func TestMarkersThatCannotBeCountedStopTheRun(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, mkWorld).Path
	swapVar(t, &surveyMarkers, func(*save.Save) (ops.MarkerSurvey, error) { return ops.MarkerSurvey{}, errors.New("no such table") })
	before := sha(t, e.save)
	_, err := Roll(e.opts())
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the Deep Desert markers could not be counted", "no such table")
}

// The whole run: a base is recycled, a vehicle is taken out, the markers are removed, and only then Game.ini is rolled. A failure at the markers after the first two were written names all of it.
func TestBasesVehiclesAndMarkersInOneRun(t *testing.T) {
	world := rcWorld(ddBases(1)...) + rcBase(9, "HaggaBasin", 1) + vehicleSQL(311, "DeepDesert", 0, 1) + mkWorld
	t.Run("all three", func(t *testing.T) {
		e, o := rcEnv(t, world)
		othersBefore := othersDump(t, e.save)
		r, err := Roll(o)
		out := lines(r)
		if err != nil || !r.Wrote {
			t.Fatalf("%v\n%s", err, out)
		}
		last := -1
		for _, want := range []string{"Checkpoint of the save:", "Recycled 1 base(s)", "vehicle(s) of yours out of the Deep Desert", "Removed the Deep Desert markers from the save", "Changed Game.ini from layout 11"} {
			i := strings.Index(out, want)
			if i < 0 || i < last {
				t.Fatalf("%q is missing or out of order:\n%s", want, out)
			}
			last = i
		}
		if len(standingIDs(t, e.save)) != 0 || survey(t, e.save).DeepDesert.Total() != 0 {
			t.Fatal("a base stands or markers are left")
		}
		if !reflect.DeepEqual(othersDump(t, e.save), othersBefore) {
			t.Fatal("a row of another map changed")
		}
		if sv := survey(t, e.save); sv.Others != (ops.MarkerCounts{Markers: 4, Discovered: 3, ResourceFields: 3}) {
			t.Fatalf("%+v", sv)
		}
		stt := readState(t, e.state)
		if len(stt.LastCheck.Recycled) != 1 || stt.LastCheck.Markers != mkWords || len(stt.LastCheck.Vehicles) != 1 {
			t.Fatalf("%+v", stt.LastCheck)
		}
	})
	t.Run("the markers fail after the base and the vehicle were written", func(t *testing.T) {
		e, o := rcEnv(t, world)
		swapVar(t, &clearMarkers, func(*save.Save) (ops.MarkerSurvey, error) { return ops.MarkerSurvey{}, errors.New("a row could not be removed") })
		r, err := Roll(o)
		if err == nil || r.Wrote || e.ini() != iniWith("11") {
			t.Fatalf("%v", err)
		}
		mustContain(t, err.Error(), "a row could not be removed", "NOTE: 1 base(s) WERE recycled", "vehicle(s) WERE also taken out", "Copy-Item -Force")
		if survey(t, e.save).DeepDesert.Total() != 8 {
			t.Fatal("the markers were removed")
		}
		if stt := readState(t, e.state); stt.LastCheck == nil || stt.LastCheck.Markers != "" || stt.LastRoll != nil {
			t.Fatalf("%+v", stt)
		}
	})
}

// With --keep-markers the same run leaves them.
func TestKeepMarkersWithBasesAndVehicles(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+vehicleSQL(311, "DeepDesert", 0, 1)+mkWorld)
	o.KeepMarkers = true
	r, err := Roll(o)
	if err != nil || !r.Wrote || survey(t, e.save).DeepDesert.Total() != 8 || len(standingIDs(t, e.save)) != 0 {
		t.Fatalf("%v\n%s", err, lines(r))
	}
}

// A save that cannot be locked (the editor has it open) is refused before the markers are looked at, exactly as before.
func TestALockedSaveIsRefusedBeforeTheMarkers(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, mkWorld).Path
	release, err := save.LockSave(e.save)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before := sha(t, e.save)
	if _, err := Roll(e.opts()); err == nil || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
}

// markerWords and the note read as plain sentences.
func TestMarkerWords(t *testing.T) {
	if got := markerWords(ops.MarkerCounts{Markers: 1, Discovered: 2, ResourceFields: 3}); got != "1 marker(s), 2 discovered row(s) and 3 resource-field row(s)" {
		t.Fatal(got)
	}
}
