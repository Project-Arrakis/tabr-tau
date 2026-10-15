package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The whole-save, one-click operations had no tests at all (QA-3 of the 2026-10-15 audit). Each one is checked with rows it must change and
// rows it must not.
const buildingWorld = `
insert into actors(id,class,map) values (801,'c','HaggaBasin'),(802,'c','HaggaBasin'),(851,'c','HaggaBasin'),(852,'c','HaggaBasin'),(853,'c','HaggaBasin');
insert into building_instances(building_id,instance_id,building_type,health,sand_buildup) values
  (801,1,'Foundation',7000.0,0),(801,2,'Foundation',3500.0,40),(801,3,'Wall',5600.0,0),(802,1,'Wall',1000.0,12),(802,2,'Wall',2000.0,0);
insert into placeables(id,health,building_type) values (851,1250.0,'WaterCistern_Placeable'),(852,300.0,'WaterCistern_Placeable'),(853,2500.0,'Generator_Placeable');`

func healths(t *testing.T, o *Ops, q string) []float64 {
	t.Helper()
	rows, err := o.S.Query(q)
	if err != nil {
		t.Fatal(err)
	}
	var out []float64
	for _, r := range rows {
		out = append(out, r["h"].(float64))
	}
	return out
}

func TestRepairBuildingsRaisesEachPieceToTheBestOfItsType(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, buildingWorld)}
	r, err := o.RepairBuildings()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["pieces"].(int64) != 3 || res["placeables"].(int64) != 1 { // the 3500 Foundation, the 1000 and 2000 Walls; the 300 cistern
		t.Fatalf("counts: %v", res)
	}
	// Foundation: 7000, 3500 -> 7000. Wall: 5600, 1000, 2000 -> 5600.
	got := healths(t, o, `select health h from building_instances order by building_id, instance_id`)
	want := []float64{7000, 7000, 5600, 5600, 5600}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("building healths %v, want %v", got, want)
		}
	}
	// placeables: the damaged cistern goes up to 1250, the generator (a type of its own) and the full cistern do not move
	pl := healths(t, o, `select health h from placeables order by id`)
	if pl[0] != 1250 || pl[1] != 1250 || pl[2] != 2500 {
		t.Fatalf("placeable healths %v", pl)
	}
	// nothing is ever lowered, and a second run changes nothing
	r2, _ := o.RepairBuildings()
	if r2.(map[string]any)["pieces"].(int64) != 0 || r2.(map[string]any)["placeables"].(int64) != 0 {
		t.Fatalf("a repeat must find nothing: %v", r2)
	}
}

func TestClearSandOnlyTouchesSand(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, buildingWorld)}
	r, err := o.ClearSand()
	if err != nil || r.(map[string]any)["pieces"].(int64) != 2 {
		t.Fatalf("%v %v", r, err)
	}
	row := mustOne(t, o, `select sum(sand_buildup) s, sum(health) h from building_instances`)
	if row["s"].(int64) != 0 || row["h"].(float64) != 7000+3500+5600+1000+2000 {
		t.Fatalf("sand must be gone and health untouched: %v", row)
	}
}

func TestResetVendorsOnlyClearsThisPlayersLimits(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map) values (9,'c','HaggaBasin');
insert into vendor_stock_state(vendor_id,player_id,template_id,amount_bought) values ('v1',1,'a',3),('v2',1,'b',4),('v1',9,'a',5);
insert into vendor_stock_cycle(vendor_id,player_id,last_interacted_timestamp) values ('v1',1,10),('v2',1,20),('v1',9,30);`)}
	count := func(q string) int64 { return mustOne(t, o, q)["c"].(int64) }
	// one vendor
	if _, err := o.ResetVendors(Args{"vendor_id": "v1"}); err != nil {
		t.Fatal(err)
	}
	if count(`select count(*) c from vendor_stock_state where player_id=1`) != 1 || count(`select count(*) c from vendor_stock_state where player_id=9`) != 1 ||
		count(`select count(*) c from vendor_stock_cycle where player_id=1`) != 1 || count(`select count(*) c from vendor_stock_cycle where player_id=9`) != 1 {
		t.Fatal("resetting v1 must clear only the player's v1 rows")
	}
	// all vendors
	if _, err := o.ResetVendors(Args{}); err != nil {
		t.Fatal(err)
	}
	if count(`select count(*) c from vendor_stock_state where player_id=1`) != 0 || count(`select count(*) c from vendor_stock_state where player_id=9`) != 1 ||
		count(`select count(*) c from vendor_stock_cycle where player_id=1`) != 0 || count(`select count(*) c from vendor_stock_cycle where player_id=9`) != 1 {
		t.Fatal("resetting all must clear the player's rows and nobody else's")
	}
}
