package ops

import (
	"fmt"
	"math"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
	"github.com/Project-Arrakis/tabr-tau/kit/ddgrid"
)

// #1175: a vehicle or a base in the Deep Desert says "Deep Desert" and its grid square in place of the "near <region>" text the Deep Desert has no names for.

func TestDeepDesertPlaceNamesTheSquareOfTheGameGrid(t *testing.T) {
	// the middle of a square: row letter from the north (A), column number from the west (1)
	mid := func(row, col int) (float64, float64) {
		return ddgrid.MinX + (float64(col)+0.5)*ddgrid.CellW, ddgrid.MaxY - (float64(row)+0.5)*ddgrid.CellH
	}
	for _, c := range []struct {
		row, col int
		want     string
	}{
		{0, 0, "Deep Desert, A1"}, {0, 8, "Deep Desert, A9"}, {8, 0, "Deep Desert, I1"}, {8, 8, "Deep Desert, I9"},
		{6, 4, "Deep Desert, G5"}, {4, 6, "Deep Desert, E7"}, {1, 7, "Deep Desert, B8"},
	} {
		x, y := mid(c.row, c.col)
		if got := deepDesertPlace(x, y); got != c.want {
			t.Errorf("row %d column %d: %q, want %q", c.row, c.col, got, c.want)
		}
	}
	// the edges: a square owns its north and west edges, and the whole grid's east and south edges are outside
	edge := []struct {
		name string
		x, y float64
		want string
	}{
		{"north-west corner", ddgrid.MinX, ddgrid.MaxY, "Deep Desert, A1"},
		{"one unit inside the south-east corner", ddgrid.MaxX - 1, ddgrid.MinY + 1, "Deep Desert, I9"},
		{"on the border of columns 4 and 5", ddgrid.MinX + 4*ddgrid.CellW, ddgrid.MaxY - 6.5*ddgrid.CellH, "Deep Desert, G5"},
		{"one unit west of that border", ddgrid.MinX + 4*ddgrid.CellW - 1, ddgrid.MaxY - 6.5*ddgrid.CellH, "Deep Desert, G4"},
		{"on the border of rows F and G", ddgrid.MinX + 4.5*ddgrid.CellW, ddgrid.MaxY - 6*ddgrid.CellH, "Deep Desert, G5"},
		{"one unit north of that border", ddgrid.MinX + 4.5*ddgrid.CellW, ddgrid.MaxY - 6*ddgrid.CellH + 1, "Deep Desert, F5"},
		{"the east edge of the grid is outside", ddgrid.MaxX, 0, "Deep Desert"},
		{"the south edge of the grid is outside", 0, ddgrid.MinY, "Deep Desert"},
		{"north of the grid", 0, ddgrid.MaxY + 1, "Deep Desert"},
		{"west of the grid", ddgrid.MinX - 1, 0, "Deep Desert"},
		{"far outside", 9e9, -9e9, "Deep Desert"},
		{"not finite", math.NaN(), 0, "Deep Desert"},
	}
	for _, c := range edge {
		if got := deepDesertPlace(c.x, c.y); got != c.want {
			t.Errorf("%s (%v, %v): %q, want %q", c.name, c.x, c.y, got, c.want)
		}
	}
	// a position the row does not hold is still on the Deep Desert's map
	if got := deepDesertPlace(nil, "x"); got != "Deep Desert" {
		t.Errorf("no position: %q", got)
	}
}

func TestOnlyADeepDesertRowGetsAPlaceText(t *testing.T) {
	f := (&Ops{S: testsave.PlayerWithSQL(t, regionWorld)}).regionFinder()
	for _, m := range []string{"HaggaBasin", "Survival_1", "", "DeepDesert_Travel"} {
		row := map[string]any{"x": float64(0), "y": float64(0)}
		f.label(row, m, "x", "y")
		_, has := row["place"]
		if want := isDeepDesertMap(m); has != want {
			t.Errorf("map %q: place present %v, want %v (%v)", m, has, want, row)
		}
	}
	row := map[string]any{"x": float64(0), "y": float64(0)}
	f.label(row, "DeepDesert", "x", "y")
	if _, has := row["region"]; has {
		t.Errorf("the Deep Desert has no region name: %v", row)
	}
}

// The lists carry the text: vehicles (also a stored one, whose tag stays) and bases of the Deep Desert say "Deep Desert, <square>", those of the Hagga Basin keep "region" and get no place.
func TestTheVehicleAndBaseListsNameTheDeepDesertSquare(t *testing.T) {
	x := func(col int) float64 { return ddgrid.MinX + (float64(col)+0.5)*ddgrid.CellW }
	y := func(row int) float64 { return ddgrid.MaxY - (float64(row)+0.5)*ddgrid.CellH }
	o := &Ops{S: testsave.PlayerWithSQL(t, fmt.Sprintf(`
insert into actors(id,class,map,location_x,location_y,location_z) values
  (601,'/Game/BP_Totem.BP_Totem_C','DeepDesert',%v,%v,0),
  (201,'/Game/BP_Sandbike.BP_Sandbike_C','HaggaBasin',150,120,0),
  (202,'/Game/BP_Buggy.BP_Buggy_C','DeepDesert',%v,%v,0),
  (203,'/Game/BP_Buggy.BP_Buggy_C','DeepDesert',9e9,9e9,0),
  (204,'/Game/BP_Buggy.BP_Buggy_C','DeepDesert',%v,%v,0);
insert into totems(id) values (601);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'a',2,3,0),(202,'b',2,3,0),(203,'c',2,3,0),(204,'d',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1),(202,1,1),(203,1,1),(204,1,1);
insert into vehicles(id) values (201),(202),(203),(204);
insert into recovered_vehicles(character_id,vehicle_id,time_stored,chassis_durability,vehicle_name) values (1,204,1700000000,1.0,'BP_Buggy');
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,area_id) values (1,0,'HaggaBasin','Cave',0,0,0,3);`,
		x(1), y(2), x(4), y(6), x(0), y(0)))}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	by := map[int64]map[string]any{}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		by[v["id"].(int64)] = v
	}
	if v := by[201]; v["region"] != "Hagga Rift" || v["place"] != nil {
		t.Errorf("a Hagga Basin vehicle keeps its region and has no place: %v", v)
	}
	if v := by[202]; v["place"] != "Deep Desert, G5" || v["region"] != nil {
		t.Errorf("vehicle 202: %v", v)
	}
	if v := by[203]; v["place"] != "Deep Desert" {
		t.Errorf("a vehicle outside the grid: %v", v)
	}
	if v := by[204]; v["place"] != "Deep Desert, A1" || v["stored"] != "recovery" || v["storedLabel"] != storedLabel("recovery") {
		t.Errorf("a stored vehicle keeps its stored label and gets the place: %v", v)
	}
	bl, err := o.BaseList()
	if err != nil {
		t.Fatal(err)
	}
	if b := bl.([]map[string]any); len(b) != 1 || b[0]["place"] != "Deep Desert, C2" {
		t.Errorf("bases: %v", b)
	}
}
