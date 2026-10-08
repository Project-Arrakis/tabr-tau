package ops

import (
	"fmt"
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
