package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

const claimFixture = `
insert into actors(id,class,map) values (601,'/Game/Dune/Systems/Building/Pieces/BP_Totem.BP_Totem_C','HaggaBasin');
insert into totems(id,landclaim_vertical_level,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (601,1,0.0,0.0,0.0,0.0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,-1,0),(601,-1,-1);`

func cellCount(t *testing.T, o *Ops) int64 {
	return mustOne(t, o, `select count(*) n from landclaim_segments where totem_id=601`)["n"].(int64)
}

func TestLandClaimsListsCellsRingsAndLevel(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)}
	r, err := o.LandClaims()
	if err != nil {
		t.Fatal(err)
	}
	rows := r.([]map[string]any)
	if len(rows) != 1 || rows[0]["cells"].(int64) != 3 || rows[0]["rings"].(int64) != 0 || rows[0]["irregular"] != true || rows[0]["level"].(int64) != 1 || rows[0]["name"] != "BP_Totem" {
		t.Fatalf("%v", rows)
	}
}

func TestExpandLandClaimFillsTheSquareKeepsExistingAndIsConnected(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)}
	r, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "rings": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	// 5x5 = 25 cells including the totem's own; 2 are already stored, so 22 are added
	if res["added"].(int) != 22 || res["totalCells"].(int) != 25 || cellCount(t, o) != 24 {
		t.Fatalf("%v stored=%d", res, cellCount(t, o))
	}
	if n := mustOne(t, o, `select count(*) n from landclaim_segments where totem_id=601 and grid_location_x=0 and grid_location_y=0`)["n"].(int64); n != 0 {
		t.Fatal("the totem's own cell (0,0) must not be stored")
	}
	if mustOne(t, o, `select landclaim_vertical_level l from totems where id=601`)["l"].(int64) != 1 {
		t.Fatal("level must not change when only rings were requested")
	}
	if _, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "rings": 2.0}); err == nil {
		t.Fatal("repeating the same size must be refused, not silently succeed")
	}
	assertDBSound(t, o)
}

func TestExpandLandClaimLevelRules(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)}
	if _, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "level": 0.0}); err == nil {
		t.Fatal("lowering the level must be refused")
	}
	if _, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "level": 9.0}); err == nil {
		t.Fatal("a level above the maximum must be refused")
	}
	if _, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "level": 4.0}); err != nil {
		t.Fatal(err)
	}
	if mustOne(t, o, `select landclaim_vertical_level l from totems where id=601`)["l"].(int64) != 4 || cellCount(t, o) != 2 {
		t.Fatal("level raised, cells untouched")
	}
}

func TestExpandLandClaimRefusesBadInput(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)}
	for name, a := range map[string]Args{
		"no change":      {"totem_id": 601.0},
		"too many rings": {"totem_id": 601.0, "rings": 6.0},
		"zero rings":     {"totem_id": 601.0, "rings": 0.0},
		"unknown totem":  {"totem_id": 999.0, "rings": 1.0},
		"no id":          {"rings": 1.0},
	} {
		if _, err := o.ExpandLandClaim(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if cellCount(t, o) != 2 {
		t.Fatal("a refused request must not write")
	}
}

func TestLandClaimsReportsFullSquareOnly(t *testing.T) {
	// a stored (0,0) row, a full 3x3 and one far cell: rings is 1, and the claim is irregular
	sql := `insert into actors(id,class,map) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin');
insert into totems(id) values (601);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,0,0),(601,3,0)`
	for _, c := range [][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
		sql += fmt.Sprintf(",(601,%d,%d)", c[0], c[1])
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, sql)}
	r, _ := o.LandClaims()
	row := r.([]map[string]any)[0]
	if row["rings"].(int64) != 1 || row["irregular"] != true || row["cells"].(int64) != 10 || row["level"].(int64) != 0 {
		t.Fatalf("%v", row)
	}
}

func TestExpandLandClaimRingsAndLevelTogetherMaxAndMessages(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into actors(id,class,map) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin'),(602,'/Game/BP_TotemSmall.BP_TotemSmall_C','HaggaBasin');
insert into totems(id,landclaim_vertical_level) values (601,NULL),(602,2);`)}
	r, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "rings": 5.0, "level": 3.0})
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["added"].(int) != 120 || res["totalCells"].(int) != 121 || res["level"].(int64) != 3 {
		t.Fatalf("%v", res)
	}
	if n := mustOne(t, o, `select count(*) n from landclaim_segments where totem_id=602`)["n"].(int64); n != 0 {
		t.Fatal("another totem must not be touched")
	}
	if mustOne(t, o, `select landclaim_vertical_level l from totems where id=602`)["l"].(int64) != 2 {
		t.Fatal("another totem's level must not change")
	}
	_, err = o.ExpandLandClaim(Args{"totem_id": 601.0, "level": 3.0})
	if err == nil || err.Error() != "the vertical level is already 3" {
		t.Fatalf("a level-only repeat needs its own message, got %v", err)
	}
	if _, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "rings": 3.0}); err == nil || err.Error() != "the claim already covers that size" {
		t.Fatalf("got %v", err)
	}
	assertDBSound(t, o)
}

// A claim with its origin at (1000, 2000) turned 90 degrees: a piece 5120 units along world +Y from the origin sits in
// claim cell (1, 0), because local = R(-yaw)(offset).
const shrinkFixture = `
insert into actors(id,class,map,location_x,location_y) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',1000.0,2000.0),
  (701,'c','HaggaBasin',1000.0,2000.0),(702,'c','HaggaBasin',1000.0,7120.0);
insert into totems(id,landclaim_vertical_level,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (601,0,1000.0,2000.0,0.0,90.0);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z) values (701,1,'Floor',1000.0,2000.0,0.0),(702,1,'Floor',1000.0,7120.0,0.0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,-1,0),(601,0,-1),(601,1,0),(601,2,2);`

func TestShrinkLandClaimRemovesEmptyCellsOutsideTheSquare(t *testing.T) {
	// piece 702 is in cell (1,0); every other stored cell is empty
	o := &Ops{S: testsave.PlayerWithSQL(t, shrinkFixture)}
	r, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["removed"].(int) != 1 || res["remaining"].(int) != 4 {
		t.Fatalf("%v", res)
	}
	left := mustOne(t, o, `select count(*) n from landclaim_segments where totem_id=601 and grid_location_x=2`)["n"].(int64)
	if left != 0 {
		t.Fatal("the cell outside the square must be gone")
	}
	if mustOne(t, o, `select count(*) n from landclaim_segments where totem_id=601`)["n"].(int64) != 3 {
		t.Fatal("cells inside the square must stay")
	}
	assertDBSound(t, o)
}

func TestShrinkLandClaimNeverRemovesACellThatHoldsPieces(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, shrinkFixture)}
	_, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 0.0})
	if err == nil || !strings.Contains(err.Error(), "hold 1 building pieces") {
		t.Fatalf("a cell with a piece in it must block the request: %v", err)
	}
	if mustOne(t, o, `select count(*) n from landclaim_segments where totem_id=601`)["n"].(int64) != 4 {
		t.Fatal("a refused request must remove nothing")
	}
	// move that piece into the totem's own cell and everything can go
	testsave.Exec(t, o.S, `update building_instances set location_y=2000.0 where building_id=702`)
	r, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 0.0})
	if err != nil || r.(map[string]any)["removed"].(int) != 4 || r.(map[string]any)["remaining"].(int) != 1 {
		t.Fatalf("%v %v", r, err)
	}
	if _, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 0.0}); err == nil {
		t.Fatal("nothing left to remove must be reported, not silently succeed")
	}
}

func TestShrinkLandClaimRefusesWithoutAnOriginAndBadInput(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)} // totem 601 has an origin of 0,0 yaw 0: fine; make one with none
	testsave.Exec(t, o.S, `update totems set landclaim_original_global_location_x=NULL where id=601`)
	if _, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 0.0}); err == nil || !strings.Contains(err.Error(), "no recorded origin") {
		t.Fatalf("a claim without an origin cannot be matched to pieces: %v", err)
	}
	for name, a := range map[string]Args{"unknown totem": {"totem_id": 999.0, "rings": 0.0}, "no rings": {"totem_id": 601.0}, "too many": {"totem_id": 601.0, "rings": 9.0}} {
		if _, err := o.ShrinkLandClaim(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if cellCount(t, o) != 2 {
		t.Fatal("refused requests must not write")
	}
}

func TestLandClaimsCountsPiecesInsideAndOutsideTheClaim(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, shrinkFixture)}
	testsave.Exec(t, o.S, `insert into actors(id,class,map) values (703,'c','HaggaBasin'); insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z) values (703,1,'Floor',1000.0,20000.0,0.0)`)
	r, _ := o.LandClaims()
	row := r.([]map[string]any)[0]
	if row["piecesInClaim"].(int64) != 2 || row["piecesOutside"].(int64) != 1 {
		t.Fatalf("%v", row)
	}
}

func TestShrinkLandClaimCountsRemainingWithAStoredCentreRow(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into actors(id,class,map) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin');
insert into totems(id,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (601,0.0,0.0,0.0,0.0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,0,0),(601,2,2)`)}
	r, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if r.(map[string]any)["remaining"].(int) != 1 {
		t.Fatalf("only the centre is left: %v", r)
	}
}

// Pieces are counted against the claim that owns them; a piece with no owner link counts for every claim.
func TestShrinkLandClaimOnlyCountsThisClaimsPieces(t *testing.T) {
	sql := `insert into actors(id,class,map,location_x,location_y) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',0.0,0.0),(602,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',0.0,0.0),(701,'c','HaggaBasin',0.0,0.0);
insert into totems(id,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (601,0.0,0.0,0.0,0.0),(602,0.0,0.0,0.0,0.0);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}')), (-602, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'a'),(602,-602,'a');
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,owner_entity_id) values (701,1,'Floor',5120.0,0.0,0.0,-602);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,1,0)`
	o := &Ops{S: testsave.PlayerWithSQL(t, sql)}
	// the only piece in cell (1,0) belongs to the other totem (602): claim 601 may shrink
	if _, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 0.0}); err != nil {
		t.Fatalf("another claim's piece must not block this claim: %v", err)
	}
	// with no owner link the same piece counts for every claim and blocks
	testsave.Exec(t, o.S, `insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,1,0); update building_instances set owner_entity_id=NULL`)
	if _, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 0.0}); err == nil {
		t.Fatal("a piece with no owner link must block")
	}
}

func TestLandClaimsGridHasEveryCellWithItsPieceCount(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, shrinkFixture)} // cells (-1,0) (0,-1) (1,0) (2,2) plus the totem's own; piece 702 sits in (1,0), 701 in (0,0)
	r, err := o.LandClaims()
	if err != nil {
		t.Fatal(err)
	}
	grid := r.([]map[string]any)[0]["grid"].([]map[string]any)
	by := map[[2]int64]map[string]any{}
	for _, c := range grid {
		by[[2]int64{c["x"].(int64), c["y"].(int64)}] = c
	}
	if len(by) != 5 {
		t.Fatalf("five cells: %v", grid)
	}
	for _, k := range [][2]int64{{0, 0}, {-1, 0}, {0, -1}, {1, 0}, {2, 2}} {
		if c := by[k]; c == nil || c["in"] != true {
			t.Fatalf("cell %v must be in the claim: %v", k, c)
		}
	}
	if by[[2]int64{1, 0}]["n"].(int64) != 1 || by[[2]int64{0, 0}]["n"].(int64) != 1 || by[[2]int64{2, 2}]["n"].(int64) != 0 {
		t.Fatalf("piece counts: %v", grid)
	}
	// rows come in order, so the picture can be drawn straight from the list
	for i := 1; i < len(grid); i++ {
		a, b := grid[i-1], grid[i]
		if a["y"].(int64) > b["y"].(int64) || (a["y"].(int64) == b["y"].(int64) && a["x"].(int64) >= b["x"].(int64)) {
			t.Fatalf("grid not in row order: %v", grid)
		}
	}
}

func TestLandClaimsGridShowsPiecesOutsideTheClaim(t *testing.T) {
	// a piece one cell beyond the claim (cell (3,0)): shown as outside the claim
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture+`
insert into actors(id,class,map,location_x,location_y) values (701,'c','HaggaBasin',15360.0,0.0);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z) values (701,1,'Floor',15360.0,0.0,0.0);`)}
	r, err := o.LandClaims()
	if err != nil {
		t.Fatal(err)
	}
	var outside int
	for _, c := range r.([]map[string]any)[0]["grid"].([]map[string]any) {
		if c["in"] == false && c["n"].(int64) > 0 {
			outside++
		}
	}
	if outside != 1 {
		t.Fatalf("one outside cell with a piece: %v", r)
	}
}
