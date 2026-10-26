package web

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The automatic pass when the editor opens (#352): one edit for every job that is switched on, written at once with one backup and one write-log line.

// autoPassSQL gives every job something to do on top of the player save: a cistern (cisternSQL), two owned vehicles with worn parts, a worn
// helmet, an empty Literjon and a character that is 70 % full of water.
const autoPassSQL = cisternSQL + twoVehiclesSQL + `
insert into fgl_entities(entity_id,components) values
  (-211, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":10.0}]}')),(-212, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":20.0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (211,-211,'Actor'),(212,-212,'Actor');
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (201,21,1,0,'Helmet',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.0,"MaxDurability":100.0}]}',0),
  (220,1,1,3,'Literjon',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (222,1,1,4,'Bloodsack_01',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{}]}',0);
update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":"70.000000","CurrentValue":"70.000000"}}}') where id=2;`

func autoServer(t *testing.T, sql string, st settings.Settings) (*Server, string) {
	t.Helper()
	s := New(testsave.PlayerWithSQL(t, sql), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, st); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func logLines(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// what the written file holds for each job
func autoPassFile(t *testing.T, path string) map[string]any {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	one := func(q string) any {
		r, err := chk.One(q)
		if err != nil || r == nil {
			t.Fatalf("%s: %v %v", q, r, err)
		}
		return r["v"]
	}
	return map[string]any{
		"water":     one(`select json_extract(components,'$.FWaterStorageComponent[1].m_WaterStored') v from fgl_entities where entity_id=-901`),
		"vehicle":   one(`select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') v from vehicle_modules where id=11`),
		"gear":      one(`select json_extract(stats,'$.FItemStackAndDurabilityStats[1].CurrentDurability') v from items where id=201`),
		"container": one(`select coalesce(json_extract(stats,'$.FFillableItemStats[1].CurrentAmount'),0) v from items where id=220`),
		"blood":     one(`select coalesce(json_extract(stats,'$.FFillableItemStats[1].CurrentAmount'),0) v from items where id=222`),
		"fuel":      one(`select json_extract(components,'$.FVehicleComponent[1].CurrentFuel') v from fgl_entities where entity_id=-211`),
		"hydration": one(`select json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') v from actors where id=2`),
	}
}

func TestAutoPassIsOneEditOneBackupOneLogLine(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true}) // the job switches say nothing: all on
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	for _, want := range []string{"Automatic refill saved ", "1 Water Cistern", "3 vehicle parts repaired", "1 worn or loadout item repaired", "1 water container filled", "1 blood sack filled", "2 vehicles refuelled", "your water filled", "backed up as"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q lacks %q", note, want)
		}
	}
	if warn || len(s.Save.Pending()) != 0 || s.Save.Dirty() {
		t.Fatalf("written at once: warn=%v pending=%v", warn, s.Save.Pending())
	}
	got := autoPassFile(t, s.Save.Path)
	if got["water"] != int64(5000) || got["vehicle"] != 250.0 || got["gear"] != 100.0 || got["container"] != 1000.0 || got["blood"] != 3000.0 || got["fuel"] != 100.0 || got["hydration"] != "100.000000" {
		t.Fatalf("the written file: %v", got)
	}
	if n := len(s.Save.Backups()); n != 1 {
		t.Fatalf("one backup for the whole pass, found %d", n)
	}
	lines := logLines(t, dir)
	if len(lines) != 1 || !strings.Contains(lines[0], `"edits":1`) || !strings.Contains(lines[0], "automatic pass: ") {
		t.Fatalf("one write-log line for the whole pass: %v", lines)
	}
	if l := s.Save.LastFlush(); l == nil || !l.Saved || len(l.Ops) != 1 || !strings.HasPrefix(l.Ops[0], "automatic pass: ") {
		t.Fatalf("%+v", l)
	}
	w := postEditGet(t, s, "/api/settings")
	if !strings.Contains(w, "Automatic refill saved 1 Water Cistern") || !strings.Contains(w, `"lastProblem":""`) {
		t.Fatalf("the card keeps what the pass did: %s", w)
	}

	// opening again finds everything done: no write, no backup, no log line
	before, _ := os.ReadFile(s.Save.Path)
	s.RunStartupTasks()
	note, _ = s.takeStartup()
	if !strings.Contains(note, "already full or in good repair") {
		t.Fatalf("note %q", note)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) || len(s.Save.Backups()) != 1 || len(logLines(t, dir)) != 1 || len(s.Save.Pending()) != 0 {
		t.Fatal("a pass with nothing to do wrote something")
	}
}

func postEditGet(t *testing.T, s *Server, route string) string {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	return do(t, s, "GET", "http://127.0.0.1"+route, loop, h, nil).Body.String()
}

// A job that is switched off is not run; the master switch off runs nothing at all; the master switch on with every job off runs nothing either.
func TestAutoPassHonoursTheSwitches(t *testing.T) {
	off := false
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true, AutoRepairVehicles: &off, AutoFillHydration: &off})
	s.RunStartupTasks()
	got := autoPassFile(t, s.Save.Path)
	if got["vehicle"] != 100.0 || got["hydration"] != "70.000000" {
		t.Fatalf("switched-off jobs ran: %v", got)
	}
	if got["water"] != int64(5000) || got["gear"] != 100.0 || got["container"] != 1000.0 {
		t.Fatalf("the others must run: %v", got)
	}
	if n, _ := s.takeStartup(); strings.Contains(n, "vehicle parts") || strings.Contains(n, "your water") {
		t.Fatalf("the note claims jobs that did not run: %q", n)
	}

	for name, st := range map[string]settings.Settings{
		"master off": {AutoRefillOnOpen: false},
		"all jobs off": {AutoRefillOnOpen: true, AutoRefillWater: &off, AutoRefillPower: &off, AutoRepairVehicles: &off, AutoRepairGear: &off,
			AutoFillContainers: &off, AutoFillHydration: &off, AutoFillBloodSacks: &off, AutoRefuelVehicles: &off},
	} {
		s2, dir := autoServer(t, autoPassSQL, st)
		before, _ := os.ReadFile(s2.Save.Path)
		s2.RunStartupTasks()
		after, _ := os.ReadFile(s2.Save.Path)
		if n, _ := s2.takeStartup(); n != "" || !bytes.Equal(before, after) || len(s2.Save.Pending()) != 0 || len(s2.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
			t.Fatalf("%s: nothing may happen (note %q)", name, n)
		}
	}
}

// A job that fails (here the character's water is not a number) rolls the whole pass back and the card names the job; the jobs before it leave
// nothing in the file, no backup, no log line, nothing queued.
func TestAutoPassThatFailsChangesNothingAndNamesTheJob(t *testing.T) {
	bad := autoPassSQL + `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":"x","CurrentValue":"x"}}}') where id=2;`
	s, dir := autoServer(t, bad, settings.Settings{AutoRefillOnOpen: true})
	before, _ := os.ReadFile(s.Save.Path)
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if !warn || !strings.Contains(note, `the job "Fill your water" failed`) || !strings.Contains(note, "Nothing was changed") {
		t.Fatalf("note %q warn %v", note, warn)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) || len(s.Save.Pending()) != 0 || s.Save.Dirty() || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
		t.Fatal("a failed pass changed or queued something")
	}
	if got := autoPassFile(t, s.Save.Path); got["water"] != int64(0) || got["vehicle"] != 100.0 {
		t.Fatalf("the jobs before the failing one must not have been applied: %v", got)
	}
	w := postEditGet(t, s, "/api/settings")
	if !strings.Contains(w, "Fill your water") || strings.Contains(w, `"lastProblem":""`) || !strings.Contains(w, `"last":""`) {
		t.Fatalf("the card shows the error and claims no result: %s", w)
	}
}

// While a single-player session is running the pass writes nothing and leaves nothing pending, with every job on.
func TestAutoPassWritesNothingDuringASinglePlayerSession(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before, _ := os.ReadFile(s.Save.Path)
	defer save.ActiveSessionForTests()()
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if !strings.Contains(note, "queued") || !strings.Contains(note, "the game is running") || !strings.Contains(note, "applied when it closes") || !warn {
		t.Fatalf("note %q warn %v", note, warn)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) || len(s.Save.Pending()) != 0 || s.Save.Dirty() || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
		t.Fatal("the pass touched the save during a session")
	}
}

// #419: the pass ran at the character-selection screen (a game process, no single-player session in the log) and wrote the save. Whatever the log says, a game process
// means the pass writes nothing and queues nothing now, says so plainly, and runs on the fresh file once the game has closed.
func TestAutoPassWaitsForTheGameToCloseWhateverTheLogSays(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before, _ := os.ReadFile(s.Save.Path)
	restore := save.GameAtMenuForTests()
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if note != "Automatic refill queued: the game is running; applied when it closes." || !warn {
		t.Fatalf("note %q warn %v", note, warn)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) || s.Save.Dirty() || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
		t.Fatal("the pass touched the save while a game process existed")
	}
	s.retryWaitingPass() // the game is still there: nothing happens
	if after, _ = os.ReadFile(s.Save.Path); !bytes.Equal(before, after) || s.Save.Dirty() {
		t.Fatal("the retry wrote while the game was still running")
	}
	restore() // the game closes
	s.retryWaitingPass()
	if note, _ := s.takeStartup(); !strings.Contains(note, "Automatic refill saved ") {
		t.Fatalf("after the game closed the pass should run: %q", note)
	}
	if got := autoPassFile(t, s.Save.Path); got["water"] != int64(5000) || got["gear"] != 100.0 {
		t.Fatalf("the written file: %v", got)
	}
	s.retryWaitingPass() // it runs once
	if n, _ := s.takeStartup(); n != "" {
		t.Fatalf("the pass ran twice: %q", n)
	}
}

// A game check that fails counts as the game running.
func TestAutoPassWaitsWhenTheGameCheckFails(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before, _ := os.ReadFile(s.Save.Path)
	defer save.FailedGameCheckForTests("process snapshot: access denied")()
	s.RunStartupTasks()
	if note, warn := s.takeStartup(); !warn || !strings.Contains(note, "queued") {
		t.Fatalf("note %q warn %v", note, warn)
	}
	if after, _ := os.ReadFile(s.Save.Path); !bytes.Equal(before, after) || s.Save.Dirty() {
		t.Fatal("the pass wrote when the check could not say")
	}
}

// A save that holds the water as numbers (the game's own shape) is filled as a number by the pass at launch, and the log line says so.
func TestAutoPassFillsNumericHydrationAsANumber(t *testing.T) {
	sql := autoPassSQL + `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":59.289321,"CurrentValue":59.289321}}}') where id=2;`
	s, dir := autoServer(t, sql, settings.Settings{AutoRefillOnOpen: true})
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if warn || !strings.Contains(note, "your water filled (kept as number)") {
		t.Fatalf("note %q warn=%v", note, warn)
	}
	if got := autoPassFile(t, s.Save.Path); got["hydration"] != 100.0 {
		t.Fatalf("the written file holds %#v, want the number 100.0", got["hydration"])
	}
	chk, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select json_type(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.BaseValue') b, json_type(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') c from actors where id=2`)
	if err != nil || r["b"] != "real" || r["c"] != "real" {
		t.Fatalf("%v %v", r, err)
	}
	if l := strings.Join(logLines(t, dir), " "); !strings.Contains(l, "kept as number") {
		t.Fatalf("the write log does not say the type: %s", l)
	}
}
