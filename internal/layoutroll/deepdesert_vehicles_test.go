package layoutroll

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/vehicles"
)

// The vehicle step of the roll (#1174): the vehicles of the player in the Deep Desert are stored or moved out, in one edit that is read back and written through the editor's own write
// path, and then the guard finds none. Made-up saves from the real schema; the character (controller 1, body 2) stands at (100, 200, 300) on Hagga Basin and faces east.

const facingEast = "update actors set rotation_x=0, rotation_y=0, rotation_z=0, rotation_w=1 where id in (1,2,3);\n"

// lrVehicle is a vehicle actor with its rows: the class's own properties (with the customization id), the vehicles row, the owner record (owner 0: none, a world vehicle) and a cargo
// inventory with an item in it.
func lrVehicle(id int, key, custom string, owner int, mp string) string {
	vc, ok := vehicles.ClassFor(key)
	if !ok {
		panic("no such class " + key)
	}
	s := fmt.Sprintf("insert into actors(id,class,map,location_x,location_y,location_z,rotation_z,rotation_w,state,properties) values (%d,'%s','%s',%d,%d,%d,0.5,0.5,0,jsonb('{\"%s\":{\"m_CustomizationId\":\"%s\"}}'));\n"+
		"insert into vehicles(id) values (%d);\n"+
		"insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (%d,%d,0,20,2000.0);\n"+
		"insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,%d,7,0,'CopperOre',0,1790000000,'{}',0);\n",
		id, vc.ActorClass, mp, -284000-id, -597000-id, 4000, vc.PropsKey, custom, id, 9000+id, id, 90000+id, 9000+id)
	if owner != 0 {
		s += fmt.Sprintf("insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (%d,'d',2,3,0);\n"+
			"insert into permission_actor_rank(permission_actor_id,player_id,rank) values (%d,%d,1);\n", id, id, owner)
	}
	return s
}

// clearingOnly is a field of foundations over 230 m by 230 m round the character with a clearing of 11 m radius 10 m ahead of the character (who is at 100, 200 and faces east).
func clearingOnly() string {
	var b strings.Builder
	b.WriteString("insert into actors(id,class,map,location_x,location_y,location_z) values (7001,'/Game/Dune/Systems/Building/Pieces/BP_DuneBuildingBase.BP_DuneBuildingBase_C','HaggaBasin',0,0,0);\ninsert into buildings(id) values (7001);\n")
	n := 0
	for x := -11500.0; x <= 11700; x += 512 {
		for y := -11300.0; y <= 11900; y += 512 {
			if math.Hypot(x-1100, y-200) > 1100 {
				n++
				fmt.Fprintf(&b, "insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_z,rotation_w) values (7001,%d,'Sardaukar_Foundation',%v,%v,0,0,1);\n", n, x, y)
			}
		}
	}
	return b.String()
}

// ddFleet is a Sandbike, a buggy, a crawler and an assault ornithopter of the player in the Deep Desert, a world buggy there, and a buggy of the player on Hagga Basin.
func ddFleet() string {
	return facingEast + lrVehicle(310, "Sandbike", "B1C3_Harkonnen_Sandbike_Variant", 1, "DeepDesert") + lrVehicle(311, "Buggy", "x", 1, "DeepDesert") +
		lrVehicle(312, "Sandcrawler", "y", 1, "DeepDesert") + lrVehicle(313, "OrnithopterMedium", "z", 1, "DeepDesert") +
		lrVehicle(320, "Buggy", "w", 0, "DeepDesert") + lrVehicle(321, "Buggy", "v", 1, "HaggaBasin")
}

func openFor(t *testing.T, p string) *ops.Ops {
	t.Helper()
	s, err := save.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return &ops.Ops{S: s}
}

func rowOf(t *testing.T, o *ops.Ops, q string, a ...any) save.Row {
	t.Helper()
	r, err := o.S.One(q, a...)
	if err != nil || r == nil {
		t.Fatalf("%s: %v %v", q, err, r)
	}
	return r
}

func fnum(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	}
	return -1
}

// The whole run on a save with vehicles in the Deep Desert: checkpoint, vehicles out (one stored, three moved in a line), the guard finds none, and only then Game.ini is rolled.
func TestTheRollTakesTheVehiclesOutFirstAndThenRolls(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
	orig, err := os.ReadFile(e.save)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Roll(e.opts())
	out := strings.Join(r.Lines, "\n")
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, out)
	}
	// the order: checkpoint -> vehicles -> the guard again -> the roll
	last := -1
	for _, want := range []string{"Checkpoint of the save:", "Took 4 vehicle(s) of yours out of the Deep Desert", "Deep Desert check again: 0 vehicle(s)", "Changed Game.ini from layout 11"} {
		i := strings.Index(out, want)
		if i < 0 || i < last {
			t.Fatalf("%q is missing or out of order:\n%s", want, out)
		}
		last = i
	}
	// the checkpoint is the save as it was before the vehicles were touched
	cps := checkpoints(t, e.save)
	if len(cps) != 1 {
		t.Fatalf("checkpoints: %v", cps)
	}
	if b, _ := os.ReadFile(cps[0]); string(b) != string(orig) {
		t.Fatal("the checkpoint is not the save as it was before")
	}
	cur, _ := os.ReadFile(e.save)
	if string(cur) == string(orig) {
		t.Fatal("the save was not written")
	}
	o := openFor(t, e.save)
	// 310: stored, as the game does it
	if a := rowOf(t, o, `select state st, map from actors where id=310`); a["st"] != int64(2) || a["map"] != "DeepDesert" {
		t.Fatalf("the Sandbike: %v", a)
	}
	bak, _ := o.S.Query(`select character_id c, vehicle_id v, customization_id k from backup_vehicles`)
	if len(bak) != 1 || bak[0]["c"] != int64(1) || bak[0]["v"] != int64(310) || bak[0]["k"] != "B1C3_Harkonnen_Sandbike_Variant" {
		t.Fatalf("backup rows: %v", bak)
	}
	// 311, 312: moved in a line in front (east): the buggy 10 m ahead, the crawler as far on as the two footprints need
	b := rowOf(t, o, `select map, location_x x, location_y y, location_z z from actors where id=311`)
	c := rowOf(t, o, `select map, location_x x, location_y y, location_z z from actors where id=312`)
	if b["map"] != "HaggaBasin" || fnum(b["x"]) != 1100 || fnum(b["y"]) != 200 || fnum(b["z"]) != 450 || c["map"] != "HaggaBasin" || fnum(c["x"]) != 3900 || fnum(c["y"]) != 200 {
		t.Fatalf("buggy %v crawler %v", b, c)
	}
	// 313: an aircraft: last in the line (the ground vehicles come first), ahead in the air: 30 m out and as far on as the buggy and the crawler need
	if m := rowOf(t, o, `select map, location_x x, location_z z from actors where id=313`); m["map"] != "HaggaBasin" || fnum(m["x"]) != 9500 || fnum(m["z"]) != 1000 {
		t.Fatalf("the aircraft: %v", m)
	}
	// the world's vehicle and the one on Hagga are where they were
	if w := rowOf(t, o, `select map, location_x x from actors where id=320`); w["map"] != "DeepDesert" || fnum(w["x"]) != -284320 {
		t.Fatalf("the world vehicle: %v", w)
	}
	if n := rowOf(t, o, `select count(*) n from items where id in (90310,90311,90312,90313)`); n["n"] != int64(4) {
		t.Fatal("the cargo changed")
	}
	// the record
	stt := readState(t, e.state)
	if stt.LastCheck == nil || len(stt.LastCheck.Vehicles) != 4 || len(stt.LastCheck.Blockers) != 0 || stt.LastRoll == nil || stt.LastCheck.Checkpoint == "" {
		t.Fatalf("%+v", stt.LastCheck)
	}
	// nothing else was written beside the save: no base file folder
	if got, _ := filepath.Glob(filepath.Join(filepath.Dir(e.state), exportFolderPrefix+"*")); len(got) != 0 {
		t.Fatal("a base folder was made")
	}
}

// A dry run says exactly what would be stored and what moved, with the words and the coordinates, and changes nothing.
func TestADryRunSaysWhichVehiclesWouldBeStoredAndWhichMovedAndChangesNothing(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
	before := sha(t, e.save)
	o := e.opts()
	o.DryRun = true
	r, err := Roll(o)
	out := strings.Join(r.Lines, "\n")
	if err != nil || r.Wrote {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{
		"Dry run: would take 4 vehicle(s) of yours out of the Deep Desert",
		"vehicle 310 (Sandbike): stored in your vehicle backup",
		"vehicle 311 (Buggy): moved from DeepDesert, x -284311, y -597311, z 4000 to HaggaBasin, x 1100, y 200, z 450, 10 m in front of you (east)",
		"vehicle 312 (SandCrawler): moved from DeepDesert",
		"vehicle 313 (MediumOrnithopter): moved from DeepDesert",
		"94 m in front of you (east) and 7 m above you",
		"Dry run: would change Game.ini",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "vehicle 320") || strings.Contains(out, "vehicle 321") {
		t.Errorf("a vehicle that is not the player's, or not in the Deep Desert, is named:\n%s", out)
	}
	if sha(t, e.save) != before || len(checkpoints(t, e.save)) != 0 || e.ini() != iniWith("11") {
		t.Fatal("the dry run wrote something")
	}
	if _, err := os.Stat(e.state); err == nil {
		t.Fatal("the dry run wrote the state file")
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(e.save), "tabr-tau-backups", "*")); len(m) != 0 {
		t.Fatalf("the dry run wrote into the backup folder: %v", m)
	}
}

// A dry run that cannot place a vehicle says it would stop, and a real run refuses and changes nothing but the checkpoint.
func TestAVehicleWithNoClearSpotRefusesTheRollAndChangesNothing(t *testing.T) {
	// base pieces fill the ground all round except a clearing 10 m ahead that holds a buggy but not a crawler
	world := facingEast + clearingOnly() + lrVehicle(310, "Buggy", "a", 1, "DeepDesert") + lrVehicle(312, "Sandcrawler", "c", 1, "DeepDesert")
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, world).Path
	before := sha(t, e.save)
	r, err := Roll(e.opts())
	if err == nil || !strings.Contains(err.Error(), "sandcrawler (vehicle 312)") || !strings.Contains(err.Error(), "not changed") || r.Wrote {
		t.Fatalf("%v", err)
	}
	if sha(t, e.save) != before || e.ini() != iniWith("11") || len(e.backups()) != 0 {
		t.Fatal("something was written")
	}
	if len(checkpoints(t, e.save)) != 1 {
		t.Fatal("the checkpoint is taken before")
	}
	stt := readState(t, e.state)
	if stt.LastCheck == nil || !strings.Contains(strings.Join(stt.LastCheck.Blockers, ";"), "vehicle 312") || stt.LastRoll != nil {
		t.Fatalf("%+v", stt)
	}
	// the dry run says so as well
	o := e.opts()
	o.DryRun = true
	r, err = Roll(o)
	if err != nil || !strings.Contains(strings.Join(r.Lines, "\n"), "Dry run: it would stop here") || !strings.Contains(strings.Join(r.Lines, "\n"), "vehicle 312") {
		t.Fatalf("%v\n%s", err, strings.Join(r.Lines, "\n"))
	}
}

// A run that will refuse for another reason does not touch the vehicles either: a base in the Deep Desert, or the character there.
func TestARunThatRefusesForAnotherReasonLeavesTheVehiclesAlone(t *testing.T) {
	for _, c := range []struct{ name, sql, want string }{
		{"a base stands there", baseInDD + "\n" + ddFleet(), "base 601"},
		{"the character is there", "update actors set map='DeepDesert' where id=1;\n" + ddFleet(), "the character"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQL(t, c.sql).Path
			before := sha(t, e.save)
			r, err := Roll(e.opts())
			out := strings.Join(r.Lines, "\n")
			if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
				t.Fatalf("%v\n%s", err, out)
			}
			for _, want := range []string{c.want, "vehicle 310 (Sandbike) is in the Deep Desert", "stored or moved by this run itself once nothing else blocks it"} {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q:\n%s", want, out)
				}
			}
		})
	}
}

// The write goes through the editor's write gate: with the game running at that moment nothing is written, the edit is thrown away, and Game.ini is not touched.
func TestTheWriteGateStopsTheVehicleEditWhenTheGameIsRunning(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
	before := sha(t, e.save)
	defer save.ActiveSessionForTests()() // the game's own session is active when the write is attempted (the run's own process check said no a moment before)
	_, err := Roll(e.opts())
	if err == nil || !strings.Contains(err.Error(), "could not be written") || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	if stt := readState(t, e.state); stt.LastRun == nil || !strings.Contains(stt.LastRun.Outcome, "could not be written") || stt.LastRoll != nil {
		t.Fatalf("%+v", stt)
	}
}

// Every step of the vehicle step that can fail leaves the save and Game.ini as they were, and says so.
func TestAFailedVehicleStepLeavesTheSaveAsItWas(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func()
		want string
	}{
		{"the save is open in another window", func() {
			lockSave = func(string) (func(), error) { return nil, errors.New("another TABR TAU has it") }
		}, "open in another"},
		{"the edit fails (a read-back that differs rolls it back)", func() {
			moveVehicles = func(*save.Save) ([]ops.VehicleMove, error) { return nil, errors.New("read back: a row differs") }
		}, "a row differs"},
		{"the plan is empty although the check found vehicles", func() {
			moveVehicles = func(*save.Save) ([]ops.VehicleMove, error) { return nil, nil }
		}, "plan for them is empty"},
		{"the write is refused", func() {
			flushSave = func(*save.Save) save.Flush { return save.Flush{Why: "the game is running"} }
		}, "the game is running"},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func(a func(string) (func(), error), b, d func(*save.Save) ([]ops.VehicleMove, error), f func(*save.Save) save.Flush) {
				lockSave, planVehicles, moveVehicles, flushSave = a, b, d, f
			}(lockSave, planVehicles, moveVehicles, flushSave)
			c.set()
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
			before := sha(t, e.save)
			r, err := Roll(e.opts())
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "not changed") || r.Wrote {
				t.Fatalf("%v", err)
			}
			if sha(t, e.save) != before || e.ini() != iniWith("11") || len(e.backups()) != 0 {
				t.Fatal("something was written")
			}
		})
	}
}

// If the file that was written does not read back as meant, the run stops, says the save WAS changed and names the checkpoint that restores it; Game.ini stays.
func TestAWrittenSaveThatDoesNotReadBackStopsTheRunAndNamesTheCheckpoint(t *testing.T) {
	defer func(f func(string, []ops.VehicleMove) error) { verifyWritten = f }(verifyWritten)
	verifyWritten = func(string, []ops.VehicleMove) error {
		return errors.New("read back: the actor row of vehicle 311 is not where it was put")
	}
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
	r, err := Roll(e.opts())
	stt := readState(t, e.state)
	if err == nil || !strings.Contains(err.Error(), "the save WAS changed") || !strings.Contains(err.Error(), stt.LastCheck.Checkpoint) || stt.LastCheck.Checkpoint == "" || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v %+v", err, stt.LastCheck)
	}
	if !strings.HasPrefix(stt.LastRun.Outcome, "stopped") || stt.LastRoll != nil {
		t.Fatalf("%+v", stt)
	}
}

// If the check after the edit still finds a vehicle of the player in the Deep Desert (not expected), the layout is not rolled, and the run says the save WAS changed and names the checkpoint.
func TestAVehicleStillThereAfterTheEditStopsTheRunAndNamesTheCheckpoint(t *testing.T) {
	defer func(f func(*save.Save) (ops.DeepDesertHolds, error)) { inspectSave = f }(inspectSave)
	calls := 0
	real := inspectSave
	inspectSave = func(s *save.Save) (ops.DeepDesertHolds, error) {
		h, err := real(s)
		calls++
		if calls == 2 {
			h.Vehicles = append(h.Vehicles, ops.HoldVehicle{ID: 310, Name: "Sandbike", Map: "DeepDesert"})
		}
		return h, err
	}
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, ddFleet()).Path
	r, err := Roll(e.opts())
	stt := readState(t, e.state)
	if err == nil || !strings.Contains(err.Error(), "the save WAS changed") || !strings.Contains(err.Error(), stt.LastCheck.Checkpoint) || stt.LastCheck.Checkpoint == "" || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v %+v", err, stt.LastCheck)
	}
	if !strings.HasPrefix(stt.LastRun.Outcome, "stopped") || stt.LastRoll != nil {
		t.Fatalf("%+v", stt)
	}
}

// Real saves, as read-only inputs: each is copied to a temporary folder first. The folder that holds them is named by TABR_VEHICLES_REAL_DIR, and the test skips without it. A base of the
// player stands in the Deep Desert in these saves, so the copy is first made to look as if the game's own tool had picked it up (state 5); then the whole run is made on the copy.
func TestTheRollOnCopiesOfRealSaves(t *testing.T) {
	dir := os.Getenv("TABR_VEHICLES_REAL_DIR")
	if dir == "" {
		t.Skip("no folder of real saves named (TABR_VEHICLES_REAL_DIR)")
	}
	ran := 0
	for _, pat := range []string{"*scout-standing*", "*scout-stored*"} {
		found, _ := filepath.Glob(filepath.Join(dir, pat, "game.db"))
		for _, src := range found {
			ran++
			t.Run(filepath.Base(filepath.Dir(src)), func(t *testing.T) {
				cp := filepath.Join(t.TempDir(), "game.db")
				f, err := os.Open(src)
				if err != nil {
					t.Skipf("not readable here: %v", err)
				}
				g, err := os.Create(cp)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.Copy(g, f); err != nil {
					t.Fatal(err)
				}
				f.Close()
				g.Close()
				// the copy: the base in the Deep Desert is taken out of the question
				s, err := save.Open(cp)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.Mutate("test: pick up the base", func(m *save.Mut) error {
					_, err := m.Exec(`update actors set state=5 where id in (select id from totems)`)
					return err
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := s.CommitForTests(); err != nil {
					t.Fatal(err)
				}
				s.Close()
				e := newEnv(t, iniWith("11"))
				e.save = cp
				o := e.opts()
				o.DryRun = true
				dr, err := Roll(o)
				if err != nil || !strings.Contains(strings.Join(dr.Lines, "\n"), "Dry run: would take 4 vehicle(s)") {
					t.Fatalf("dry run: %v\n%s", err, strings.Join(dr.Lines, "\n"))
				}
				t.Log("\n" + strings.Join(dr.Lines, "\n"))
				before := sha(t, cp)
				r, err := Roll(e.opts())
				if err != nil || !r.Wrote {
					t.Fatalf("%v\n%s", err, strings.Join(r.Lines, "\n"))
				}
				if sha(t, cp) == before {
					t.Fatal("the save was not written")
				}
				op := openFor(t, cp)
				h, err := op.DeepDesertHolds()
				if err != nil || h.Blocked() {
					t.Fatalf("%v %+v", err, h)
				}
			})
		}
	}
	if ran == 0 {
		t.Skip("none of the named saves is in the folder")
	}
}
