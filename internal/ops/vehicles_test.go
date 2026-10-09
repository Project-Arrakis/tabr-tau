package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Vehicles in one save: 201 is the player's (owner rank 1), 202 a world vehicle with no permission record, 203 owned by another
// actor, 204 where the player is only a co-owner (rank 2). actors.owner_account_id stays NULL, as in real saves.
const vehicleWorld = `
insert into actors(id,class,map,location_x,location_y,location_z) values
  (9,'/Game/Other.Other_C','HaggaBasin',0,0,0),
  (201,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',10,20,30),
  (202,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',1000,2000,3000),
  (203,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',1100,2100,3100),
  (204,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',1200,2200,3200);
insert into vehicles(id) values (201),(202),(203),(204);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'mine',2,3,0),(203,'theirs',2,3,0),(204,'shared',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1),(203,9,1),(204,9,1),(204,1,2);
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (1,201,'Hull_1',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (2,202,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (3,203,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (4,204,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}'));`

func vehicleOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld)} }

func TestVehiclesListsOnlyOwned(t *testing.T) {
	r, err := vehicleOps(t).Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	live := res["vehicles"].([]map[string]any)
	if len(live) != 1 || live[0]["id"].(int64) != 201 || live[0]["name"] == "" {
		t.Fatalf("only vehicle 201 must be listed, got %v", live)
	}
	if res["hidden"].(int64) != 3 {
		t.Fatalf("hidden world/other vehicles = %v, want 3", res["hidden"])
	}
}

// A save with no owner record at all lists nothing: not knowing the owner never counts as owning.
func TestVehiclesWithoutOwnerRecordAreNotListed(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into actors(id,class,map) values (301,'c','HaggaBasin'); insert into vehicles(id) values (301);`)}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if len(res["vehicles"].([]map[string]any)) != 0 || res["hidden"].(int64) != 1 {
		t.Fatalf("unexpected result %v", res)
	}
}

func TestRepairVehiclesLeavesWorldVehiclesAlone(t *testing.T) {
	o := vehicleOps(t)
	r, err := o.RepairVehicles()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["modules"].(int64) != 1 || res["repaired"].(int64) != 1 {
		t.Fatalf("only the owned vehicle's module counts: %v", res)
	}
	cur := func(id int) float64 {
		return mustOne(t, o, `select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') c from vehicle_modules where id=?`, id)["c"].(float64)
	}
	if cur(1) != 250 || cur(2) != 100 || cur(3) != 100 || cur(4) != 100 {
		t.Fatalf("durability after repair: %v %v %v %v", cur(1), cur(2), cur(3), cur(4))
	}
	assertDBSound(t, o)
}

func TestBringVehicleOnlyOwned(t *testing.T) {
	o := vehicleOps(t)
	for _, id := range []float64{202, 203, 204, 999} {
		if _, err := o.BringVehicle(Args{"id": id}); err == nil {
			t.Errorf("vehicle %v is not the player's and must not be moved", id)
		}
	}
	if x := mustOne(t, o, `select location_x x from actors where id=202`)["x"].(float64); x != 1000 {
		t.Fatalf("world vehicle was moved: x=%v", x)
	}
	if _, err := o.BringVehicle(Args{"id": 201.0}); err != nil {
		t.Fatal(err)
	}
	if x := mustOne(t, o, `select location_x x from actors where id=201`)["x"].(float64); x != 900 { // the player stands at x=100
		t.Fatalf("owned vehicle x=%v, want 900", x)
	}
}

func TestRecoveredVehiclesFollowTheCharacter(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld+`
insert into recovered_vehicles(character_id,vehicle_id,chassis_durability,vehicle_name) values (1,201,50.0,'Mine');`)}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	if rec := r.(map[string]any)["recovered"].([]map[string]any); len(rec) != 1 || rec[0]["vehicle_name"] != "Mine" {
		t.Fatalf("recovered = %v", rec)
	}
	if _, err := o.SetRecoveredDurability(Args{"vehicle_id": 201.0, "chassis_durability": 75.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.SetRecoveredDurability(Args{"vehicle_id": 202.0, "chassis_durability": 75.0}); err == nil {
		t.Fatal("a vehicle that is not the character's recovered vehicle must be refused")
	}
	if d := mustOne(t, o, `select chassis_durability d from recovered_vehicles where vehicle_id=201`)["d"].(float64); d != 75 {
		t.Fatalf("durability = %v", d)
	}
}

func TestRepairOneVehicleOnlyRepairsThatOwnedVehicle(t *testing.T) {
	o := vehicleOps(t)
	cur := func(id int) float64 {
		return mustOne(t, o, `select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') c from vehicle_modules where id=?`, id)["c"].(float64)
	}
	for _, id := range []int64{202, 203, 204, 999} {
		if _, err := o.RepairVehiclesWhere(100, id); err == nil {
			t.Errorf("vehicle %d is not the player's and must not be repaired", id)
		}
	}
	if o.S.Dirty() {
		t.Fatal("refused requests must not change the save")
	}
	r, err := o.RepairVehiclesWhere(100, 201)
	if err != nil || r.(map[string]any)["repaired"].(int64) != 1 {
		t.Fatalf("%v %v", r, err)
	}
	if cur(1) != 250 || cur(2) != 100 || cur(3) != 100 || cur(4) != 100 {
		t.Fatalf("only vehicle 201's module is raised: %v %v %v %v", cur(1), cur(2), cur(3), cur(4))
	}
}
