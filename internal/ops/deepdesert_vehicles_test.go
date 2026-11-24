package ops

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/vehicles"
)

// Taking the player's vehicles out of the Deep Desert for the layout roll (#1174), on made-up saves built from the real schema. The character (id 1, body 2) stands at (100, 200, 300) on
// Hagga Basin and faces east (turnTo(0)); ground vehicles therefore go to x 1100 (10 m ahead), aircraft to x 3100 and 7 m up. Nothing here is from a real save.

// ddVehicle is a vehicle actor with its rows: the class's own properties (with the customization id unless custom is ""), the vehicles row, an owner record when owner is not 0, a cargo
// inventory and an item in it, so that a test can show the cargo is left alone.
func ddVehicle(id int, key, custom string, owner int, mp string, x, y, z float64) string {
	vc, ok := vehicles.ClassFor(key)
	if !ok {
		panic("no such class " + key)
	}
	props := "{}"
	if custom != "" {
		props = fmt.Sprintf(`{"%s":{"m_HarnessedVehicleId":"!!act#0","m_CustomizationId":"%s"}}`, vc.PropsKey, custom)
	}
	s := fmt.Sprintf("insert into actors(id,class,map,location_x,location_y,location_z,rotation_z,rotation_w,state,properties) values (%d,'%s','%s',%v,%v,%v,0.5,0.5,0,jsonb('%s'));\n"+
		"insert into vehicles(id) values (%d);\n"+
		"insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (%d,%d,0,20,2000.0);\n"+
		"insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,%d,7,0,'CopperOre',0,1790000000,'{}',0);\n",
		id, vc.ActorClass, mp, x, y, z, props, id, 9000+id, id, 90000+id, 9000+id)
	if owner != 0 {
		s += fmt.Sprintf("insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (%d,'d',2,3,0);\n"+
			"insert into permission_actor_rank(permission_actor_id,player_id,rank) values (%d,%d,1);\n", id, id, owner)
	}
	return s
}

func ddOps(t *testing.T, deg float64, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, turnTo(deg)+"\n"+extra)}
}

// snapshot is every row of every table, as text, counted (so a row that is changed shows as one row gone and one new).
func snapshot(t *testing.T, s *save.Save) map[string]map[string]int {
	t.Helper()
	out := map[string]map[string]int{}
	for _, tb := range testsave.TableNames(t) {
		rows, err := s.Query(`select * from "` + tb + `"`)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]int{}
		for _, r := range rows {
			m[fmt.Sprint(map[string]any(r))]++
		}
		out[tb] = m
	}
	return out
}

// changedRows lists the rows that differ between two snapshots: "- table row" for a row that is gone, "+ table row" for a new one.
func changedRows(before, after map[string]map[string]int) []string {
	var out []string
	for tb, b := range before {
		a := after[tb]
		for r, n := range b {
			if a[r] < n {
				out = append(out, "- "+tb+" "+r)
			}
		}
		for r, n := range a {
			if b[r] < n {
				out = append(out, "+ "+tb+" "+r)
			}
		}
	}
	sort.Strings(out)
	return out
}

func changeCounts(rows []string) map[string]int {
	m := map[string]int{}
	for _, r := range rows {
		f := strings.Fields(r)
		m[f[0]+" "+f[1]]++
	}
	return m
}

func actorOf(t *testing.T, o *Ops, id int) save.Row {
	t.Helper()
	r, err := o.S.One(`select map, dimension_index d, location_x x, location_y y, location_z z, state st from actors where id=?`, id)
	if err != nil || r == nil {
		t.Fatal(err, r)
	}
	return r
}

func plan(t *testing.T, o *Ops) map[int64]VehicleMove {
	t.Helper()
	ps, err := o.PlanDeepDesertVehicles()
	if err != nil {
		t.Fatal(err)
	}
	m := map[int64]VehicleMove{}
	for _, p := range ps {
		m[p.ID] = p
	}
	return m
}

func TestAScoutIsStoredWithTheGamesOwnRowsAndNothingElseChanges(t *testing.T) {
	for _, c := range []struct {
		key, custom string
	}{{"OrnithopterLight", "MTX_WaterS_Light_Orni"}, {"Sandbike", "B1C3_Harkonnen_Sandbike_Variant"}} {
		o := ddOps(t, 0, ddVehicle(310, c.key, c.custom, 1, "DeepDesert", -284008, -597286, 3929))
		before := snapshot(t, o.S)
		done, err := o.MoveDeepDesertVehicles()
		if err != nil || len(done) != 1 || done[0].Action != "store" {
			t.Fatalf("%s: %v %+v", c.key, err, done)
		}
		got := changedRows(before, snapshot(t, o.S))
		// exactly: the actor row (its state 0 -> 2) and ONE new backup_vehicles row (the character, the vehicle, the customization id the vehicle's own properties hold)
		if len(got) != 3 || changeCounts(got)["- actors"] != 1 || changeCounts(got)["+ actors"] != 1 || changeCounts(got)["+ backup_vehicles"] != 1 {
			t.Fatalf("%s: rows changed: %v", c.key, got)
		}
		a := actorOf(t, o, 310)
		if a["st"] != int64(2) || a["map"] != "DeepDesert" || num(a["x"]) != -284008 {
			t.Fatalf("%s: the actor: %v", c.key, a)
		}
		bak, _ := o.S.Query(`select character_id, vehicle_id, customization_id from backup_vehicles`)
		if len(bak) != 1 || bak[0]["character_id"] != int64(1) || bak[0]["vehicle_id"] != int64(310) || bak[0]["customization_id"] != c.custom {
			t.Fatalf("%s: the backup row: %v", c.key, bak)
		}
		if !strings.Contains(done[0].Words, "stored in your vehicle backup") {
			t.Fatalf("words: %q", done[0].Words)
		}
	}
}

// The game's backup holds ONE vehicle for each character (UNIQUE(character_id)). With a Scout and a Sandbike both in the Deep Desert the Scout is stored and the Sandbike is moved,
// whichever of the two comes first in the save (operator rule, 2026-11-24); one alone is stored; two of one type: the lowest vehicle number is stored.
func TestOnlyOneVehicleFitsTheBackupTheScoutIsStoredTheSandbikeIsMovedAndSaysWhy(t *testing.T) {
	for _, c := range []struct {
		name         string
		extra        string
		stored       int64
		moved        []int64
		reasonSubstr string
	}{
		{"bike first", ddVehicle(310, "Sandbike", "A", 1, "DeepDesert", 5, 5, 5) + ddVehicle(311, "OrnithopterLight", "B", 1, "DeepDesert", 6, 6, 6), 311, []int64{310}, "is stored in it first"},
		{"scout first", ddVehicle(310, "OrnithopterLight", "B", 1, "DeepDesert", 6, 6, 6) + ddVehicle(311, "Sandbike", "A", 1, "DeepDesert", 5, 5, 5), 310, []int64{311}, "is stored in it first"},
		{"lone scout", ddVehicle(310, "OrnithopterLight", "B", 1, "DeepDesert", 6, 6, 6), 310, nil, ""},
		{"lone bike", ddVehicle(310, "Sandbike", "A", 1, "DeepDesert", 5, 5, 5), 310, nil, ""},
		{"two scouts", ddVehicle(320, "OrnithopterLight", "B", 1, "DeepDesert", 6, 6, 6) + ddVehicle(310, "OrnithopterLight", "C", 1, "DeepDesert", 7, 7, 7), 310, []int64{320}, "is stored in it first"},
		{"two bikes", ddVehicle(320, "Sandbike", "B", 1, "DeepDesert", 6, 6, 6) + ddVehicle(310, "Sandbike", "C", 1, "DeepDesert", 7, 7, 7), 310, []int64{320}, "is stored in it first"},
		{"two bikes and a scout, a buggy too", ddVehicle(309, "Sandbike", "A", 1, "DeepDesert", 5, 5, 5) + ddVehicle(310, "Sandbike", "B", 1, "DeepDesert", 6, 6, 6) +
			ddVehicle(330, "OrnithopterLight", "C", 1, "DeepDesert", 7, 7, 7) + ddVehicle(311, "Buggy", "", 1, "DeepDesert", 8, 8, 8), 330, []int64{309, 310, 311}, ""},
		// the first by number has no customization in the save: the next one that can be stored is, a Scout still first
		{"scout without customization", ddVehicle(310, "OrnithopterLight", "", 1, "DeepDesert", 6, 6, 6) + ddVehicle(311, "Sandbike", "A", 1, "DeepDesert", 5, 5, 5), 311, []int64{310}, "customization"},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := ddOps(t, 0, c.extra)
			p := plan(t, o)
			n := 0
			for id, v := range p {
				switch {
				case id == c.stored:
					n++
					if v.Action != "store" || v.Reason != "" {
						t.Fatalf("vehicle %d: %+v", id, v)
					}
				case v.Action != "move" || v.To == nil || !strings.Contains(v.Reason+v.Words, c.reasonSubstr) && v.Name != "Buggy":
					t.Fatalf("vehicle %d should be moved: %+v", id, v)
				}
			}
			if n != 1 || len(p)-1 != len(c.moved) {
				t.Fatalf("plan: %+v", p)
			}
			for _, id := range c.moved {
				if p[id].Action != "move" {
					t.Fatalf("vehicle %d: %+v", id, p[id])
				}
				if v := p[id]; (v.Name == "Sandbike" || strings.Contains(v.Class, "Ornithopter")) && !strings.Contains(v.Words, "not stored because") {
					t.Fatalf("the words do not say why: %q", v.Words)
				}
			}
			// the plan is the same every time, and the edit does what the plan says
			again := plan(t, o)
			for id, v := range p {
				if again[id].Action != v.Action {
					t.Fatalf("the plan changed: %+v vs %+v", p, again)
				}
			}
			if _, err := o.MoveDeepDesertVehicles(); err != nil {
				t.Fatal(err)
			}
			bak, _ := o.S.Query(`select character_id, vehicle_id from backup_vehicles`)
			if len(bak) != 1 || bak[0]["character_id"] != int64(1) || bak[0]["vehicle_id"] != c.stored {
				t.Fatalf("backup rows: %v", bak)
			}
		})
	}
}

// A character whose backup already holds a vehicle, of ANY type (a Sandbike, a Scout, a buggy), stores nothing: every Scout and Sandbike in the Deep Desert is moved. A vehicle that is
// already stored is not touched. The slot is the character's own: a row of another character does not take it.
func TestAFullBackupMovesEveryScoutAndSandbikeWhateverTheStoredVehicleIs(t *testing.T) {
	both := ddVehicle(310, "Sandbike", "A", 1, "DeepDesert", 5, 5, 5) + ddVehicle(311, "OrnithopterLight", "B", 1, "DeepDesert", 6, 6, 6)
	for _, stored := range []string{"Sandbike", "OrnithopterLight", "Buggy"} {
		o := ddOps(t, 0, both+ddVehicle(312, stored, "C", 1, "HaggaBasin", 7, 7, 7)+
			"update actors set state=2 where id=312; insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,312,'C');")
		before := snapshot(t, o.S)
		p := plan(t, o)
		for _, id := range []int64{310, 311} {
			if p[id].Action != "move" || !strings.Contains(p[id].Reason, "already holds a vehicle") || !strings.Contains(p[id].Words, "not stored because") {
				t.Fatalf("%s stored, vehicle %d: %+v", stored, id, p[id])
			}
		}
		if _, ok := p[312]; ok || len(p) != 2 {
			t.Fatalf("%s stored: the stored vehicle is in the plan: %+v", stored, p)
		}
		if _, err := o.MoveDeepDesertVehicles(); err != nil {
			t.Fatal(err)
		}
		for _, r := range changedRows(before, snapshot(t, o.S)) {
			if strings.Contains(r, "backup_vehicles") || strings.Contains(r, "HaggaBasin") && strings.Contains(r, "312") {
				t.Fatalf("%s stored: the backup or the stored vehicle changed: %s", stored, r)
			}
		}
		if bak, _ := o.S.Query(`select vehicle_id from backup_vehicles`); len(bak) != 1 || bak[0]["vehicle_id"] != int64(312) {
			t.Fatalf("%s stored: backup rows %v", stored, bak)
		}
		if a := actorOf(t, o, 312); a["st"] != int64(2) {
			t.Fatalf("%s stored: its state: %v", stored, a)
		}
	}
	// a backup row of ANOTHER character (the save has two) does not take this character's slot; the character is the first player record
	o := ddOps(t, 0, both+"insert into player_state(id, account_id, character_name) values (2, 2, 'Other');"+ddVehicle(312, "Sandbike", "C", 2, "HaggaBasin", 7, 7, 7)+
		"update actors set state=2 where id=312; insert into backup_vehicles(character_id,vehicle_id,customization_id) values (2,312,'C');")
	if p := plan(t, o); p[311].Action != "store" || p[310].Action != "move" {
		t.Fatalf("%+v", p)
	}
	if _, err := o.MoveDeepDesertVehicles(); err != nil {
		t.Fatal(err)
	}
	if n, _ := o.S.One(`select count(*) n from backup_vehicles where character_id=2`); n["n"] != int64(1) {
		t.Fatalf("the other character's backup row changed: %v", n)
	}
}

func TestAScoutWhoseCustomizationIsNotRecordedIsMovedNotStored(t *testing.T) {
	o := ddOps(t, 0, ddVehicle(310, "OrnithopterLight", "", 1, "DeepDesert", 5, 5, 5))
	p := plan(t, o)
	if p[310].Action != "move" || !strings.Contains(p[310].Reason, "customization") {
		t.Fatalf("%+v", p)
	}
}

// A buggy and a crawler are moved to Hagga Basin in a line in front of the character: 10 m ahead for the first, and the second as far on as the two footprints need. The rows written are
// the map and the place (what Bring vehicle writes); the cargo and everything else stay.
func TestABuggyAndACrawlerAreMovedInALineInFrontOfTheCharacter(t *testing.T) {
	o := ddOps(t, 0, ddVehicle(310, "Buggy", "", 1, "DeepDesert", -285156, -598287, 4084)+ddVehicle(311, "Sandcrawler", "", 1, "DeepDesert", -284254, -597608, 5241))
	before := snapshot(t, o.S)
	done, err := o.MoveDeepDesertVehicles()
	if err != nil || len(done) != 2 {
		t.Fatal(err, done)
	}
	got := changedRows(before, snapshot(t, o.S))
	if len(got) != 4 || changeCounts(got)["- actors"] != 2 || changeCounts(got)["+ actors"] != 2 {
		t.Fatalf("only the two actor rows change (no inventory, item or backup row): %v", got)
	}
	b, c := actorOf(t, o, 310), actorOf(t, o, 311)
	if b["map"] != "HaggaBasin" || b["d"] != int64(0) || num(b["x"]) != 1100 || num(b["y"]) != 200 || num(b["z"]) != 450 || b["st"] != int64(0) {
		t.Fatalf("the buggy: %v", b)
	}
	// 12 m is not enough for a crawler beside a buggy: the line goes on by both footprints and their margins (400 + 2200 + 200)
	if c["map"] != "HaggaBasin" || num(c["x"]) != 100+1000+2800 || num(c["y"]) != 200 || num(c["z"]) != 450 {
		t.Fatalf("the crawler: %v", c)
	}
	if d := math.Hypot(num(b["x"])-num(c["x"]), num(b["y"])-num(c["y"])); d < 400+2200+200 {
		t.Fatalf("the two overlap: %v apart", d)
	}
	if !strings.Contains(done[0].To.Phrase, "10 m in front of you (east)") || !strings.Contains(done[0].Words, "HaggaBasin, x 1100, y 200, z 450") {
		t.Fatalf("words: %q", done[0].Words)
	}
	// the guard now finds nothing of them
	h, err := o.DeepDesertHolds()
	if err != nil || h.Blocked() {
		t.Fatalf("%v %+v", err, h)
	}
}

func TestAnAircraftIsMovedThirtyMetresAheadAndSevenUp(t *testing.T) {
	for _, key := range []string{"OrnithopterMedium", "OrnithopterTransport"} {
		o := ddOps(t, 90, ddVehicle(310, key, "", 1, "DeepDesert", 1, 2, 3)) // facing south (+y)
		if _, err := o.MoveDeepDesertVehicles(); err != nil {
			t.Fatal(err)
		}
		a := actorOf(t, o, 310)
		if a["map"] != "HaggaBasin" || math.Abs(num(a["x"])-100) > 0.01 || math.Abs(num(a["y"])-3200) > 0.01 || math.Abs(num(a["z"])-1000) > 0.01 {
			t.Fatalf("%s: %v", key, a)
		}
	}
}

// A vehicle with no clear spot stops the whole edit, names the vehicle, and the save is exactly as it was.
func TestAVehicleWithNoClearSpotRefusesAndChangesNothing(t *testing.T) {
	// base pieces fill the ground all round except a clearing 10 m ahead that holds a buggy but not a crawler: the buggy (311) is placed, the crawler (312) has no spot
	var field []testPiece
	for x := -11500.0; x <= 11700; x += 512 {
		for y := -11300.0; y <= 11900; y += 512 {
			if math.Hypot(x-1100, y-200) > 1100 {
				field = append(field, testPiece{"Sardaukar_Foundation", x, y, 0, 0})
			}
		}
	}
	o := ddOps(t, 0, piecesSQL(7001, field)+ddVehicle(311, "Buggy", "", 1, "DeepDesert", 2, 2, 2)+ddVehicle(312, "Sandcrawler", "", 1, "DeepDesert", 3, 3, 3)+
		ddVehicle(313, "Sandbike", "X", 1, "DeepDesert", 4, 4, 4))
	before := snapshot(t, o.S)
	if _, err := o.PlanDeepDesertVehicles(); err == nil || !strings.Contains(err.Error(), "sandcrawler (vehicle 312)") || !strings.Contains(err.Error(), "no clear spot") {
		t.Fatalf("plan: %v", err)
	}
	_, err := o.MoveDeepDesertVehicles()
	if err == nil || !strings.Contains(err.Error(), "vehicle 312") || !strings.Contains(err.Error(), "sandcrawler") {
		t.Fatalf("the refusal does not name the vehicle: %v", err)
	}
	if got := changedRows(before, snapshot(t, o.S)); len(got) != 0 {
		t.Fatalf("the save changed: %v", got)
	}
	if o.S.Dirty() {
		t.Fatal("an edit is waiting")
	}
}

// The world's vehicles, other players' vehicles, vehicles on other maps and stored vehicles are not touched.
func TestOnlyTheOwnVehiclesInTheDeepDesertAreTouched(t *testing.T) {
	o := ddOps(t, 0, ddVehicle(310, "Buggy", "", 1, "DeepDesert", 11, 12, 13)+
		ddVehicle(320, "Buggy", "", 0, "DeepDesert", 21, 22, 23)+ // a world vehicle: no owner record
		ddVehicle(321, "Buggy", "", 5, "DeepDesert", 31, 32, 33)+ // someone else's
		ddVehicle(322, "Buggy", "", 1, "HaggaBasin", 41, 42, 43)+ // on another map
		ddVehicle(323, "Sandbike", "S", 1, "DeepDesert", 51, 52, 53)+ // stored for the character
		"update actors set state=2 where id=323; insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,323,'S');")
	before := snapshot(t, o.S)
	done, err := o.MoveDeepDesertVehicles()
	if err != nil || len(done) != 1 || done[0].ID != 310 {
		t.Fatal(err, done)
	}
	got := changedRows(before, snapshot(t, o.S))
	if len(got) != 2 {
		t.Fatalf("only the one vehicle's actor row: %v", got)
	}
	for _, id := range []int{320, 321, 322, 323} {
		a := actorOf(t, o, id)
		if a["x"] == nil || num(a["x"]) < 20 || a["map"] == "" {
			t.Fatalf("%d: %v", id, a)
		}
	}
	if a := actorOf(t, o, 320); a["map"] != "DeepDesert" || num(a["x"]) != 21 {
		t.Fatalf("the world vehicle: %v", a)
	}
}

// The plan alone writes nothing (a dry run).
func TestThePlanAloneChangesNothing(t *testing.T) {
	o := ddOps(t, 0, ddVehicle(310, "Buggy", "", 1, "DeepDesert", 1, 2, 3)+ddVehicle(311, "Sandbike", "S", 1, "DeepDesert", 4, 5, 6))
	before := snapshot(t, o.S)
	ps, err := o.PlanDeepDesertVehicles()
	if err != nil || len(ps) != 2 {
		t.Fatal(err, ps)
	}
	if got := changedRows(before, snapshot(t, o.S)); len(got) != 0 || o.S.Dirty() {
		t.Fatalf("%v dirty=%v", got, o.S.Dirty())
	}
	lines := VehicleMoveLines(ps)
	if len(lines) != 2 || !strings.Contains(lines[0], "vehicle 310 (Buggy): moved from DeepDesert, x 1, y 2, z 3 to HaggaBasin, x 1100, y 200, z 450") || !strings.Contains(lines[1], "stored in your vehicle backup") {
		t.Fatalf("%q", lines)
	}
}

// A stored vehicle that keeps a place in the Deep Desert is not "in the Deep Desert": the guard does not count it (the open question of #1174).
func TestTheGuardDoesNotCountAStoredVehicle(t *testing.T) {
	o := ddOps(t, 0, ddVehicle(310, "Sandbike", "S", 1, "DeepDesert", 1, 2, 3)+"update actors set state=2 where id=310; insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,310,'S');"+
		ddVehicle(311, "Buggy", "", 1, "DeepDesert", 1, 2, 3)+"update actors set state=4 where id=311;")
	h, err := o.DeepDesertHolds()
	if err != nil || h.Blocked() || h.Checked.Vehicles != 2 {
		t.Fatalf("%v %+v", err, h)
	}
	// and one that stands in the world still counts
	o = ddOps(t, 0, ddVehicle(312, "Buggy", "", 1, "DeepDesert", 1, 2, 3))
	if h, _ := o.DeepDesertHolds(); len(h.Vehicles) != 1 {
		t.Fatalf("%+v", h)
	}
}

// What cannot be placed is refused with the save as it was: the character in the Deep Desert, a character whose body is parked on another map, a vehicle whose map is not recorded.
func TestWhatCannotBePlacedIsRefusedAndChangesNothing(t *testing.T) {
	for _, c := range []struct {
		name, sql, want string
	}{
		{"the character is in the Deep Desert", "update actors set map='DeepDesert' where id in (1,2,3);", "in the Deep Desert"},
		{"the body is on another map", "update actors set map='Arrakeen' where id=2;", "another map"},
		{"a vehicle has no map", ddVehicle(311, "Buggy", "", 1, "", 1, 2, 3), "no map recorded"},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := ddOps(t, 0, ddVehicle(310, "Buggy", "", 1, "DeepDesert", 1, 2, 3)+c.sql)
			before := snapshot(t, o.S)
			if _, err := o.MoveDeepDesertVehicles(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("%v", err)
			}
			if got := changedRows(before, snapshot(t, o.S)); len(got) != 0 || o.S.Dirty() {
				t.Fatalf("%v", got)
			}
		})
	}
}

// A read-back that differs undoes the whole edit: every row is as it was and nothing waits to be written.
func TestAReadBackThatDiffersRollsTheWholeEditBack(t *testing.T) {
	o := ddOps(t, 0, ddVehicle(310, "Buggy", "", 1, "DeepDesert", 1, 2, 3)+ddVehicle(311, "Sandbike", "S", 1, "DeepDesert", 4, 5, 6))
	before := snapshot(t, o.S)
	old := vehicleReadBack
	defer func() { vehicleReadBack = old }()
	seen := 0
	vehicleReadBack = func(q rowReader, p player, plan []VehicleMove) error {
		seen = len(plan)
		if err := old(q, p, plan); err != nil { // the real check passes: the rows are written...
			return err
		}
		return errors.New("read back: a row differs") // ...and the check then reports a difference
	}
	if _, err := o.MoveDeepDesertVehicles(); err == nil || !strings.Contains(err.Error(), "a row differs") || seen != 2 {
		t.Fatalf("%v %d", err, seen)
	}
	if got := changedRows(before, snapshot(t, o.S)); len(got) != 0 || o.S.Dirty() {
		t.Fatalf("the edit was not undone: %v", got)
	}
}

// The check itself finds a row that is not what was meant.
func TestTheReadBackFindsADifferentRow(t *testing.T) {
	o := ddOps(t, 0, ddVehicle(310, "Buggy", "", 1, "DeepDesert", 1, 2, 3)+ddVehicle(311, "Sandbike", "S", 1, "DeepDesert", 4, 5, 6))
	done, err := o.MoveDeepDesertVehicles()
	if err != nil {
		t.Fatal(err)
	}
	if err := o.VerifyVehicleMoves(done); err != nil {
		t.Fatalf("the rows just written do not verify: %v", err)
	}
	for _, c := range []struct{ name, sql, want string }{
		{"the place", "update actors set location_x=location_x+1 where id=310;", "not where it was put"},
		{"the map", "update actors set map='DeepDesert' where id=310;", "not where it was put"},
		{"the state of the stored one", "update actors set state=0 where id=311;", "not the stored one"},
		{"the customization", "update backup_vehicles set customization_id='other';", "backup row"},
		{"the backup row is gone", "delete from backup_vehicles;", "backup rows"},
		{"the backup row is on the moved vehicle", "update backup_vehicles set vehicle_id=310;", "backup row"},
	} {
		o2 := ddOps(t, 0, ddVehicle(310, "Buggy", "", 1, "DeepDesert", 1, 2, 3)+ddVehicle(311, "Sandbike", "S", 1, "DeepDesert", 4, 5, 6))
		done, err := o2.MoveDeepDesertVehicles()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := o2.S.Mutate("tamper", func(m *save.Mut) error { _, err := m.Exec(c.sql); return err }); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if err := o2.VerifyVehicleMoves(done); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}
