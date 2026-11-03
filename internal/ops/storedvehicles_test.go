package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #528: the card of stored vehicles lists every stored vehicle of the character, however the save says so. 301 is a recovered_vehicles row (state 4, owner record gone),
// 302 is the character's row in backup_vehicles (state 2, still owned), 303 is owned with state 2 and no row at all and 304 stands in the world.
const storedWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,state) values
  (301,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',1,1,0,4),
  (302,'/Game/BP_LightOrnithopter_Choam.BP_LightOrnithopter_Choam_C','DeepDesert',2,2,0,2),
  (303,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',3,3,0,2),
  (304,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',4,4,0,0);
insert into vehicles(id) values (301),(302),(303),(304);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (302,'b',2,3,0),(303,'c',2,3,0),(304,'d',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (302,1,1),(303,1,1),(304,1,1);
insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,302,'None');
insert into recovered_vehicles(character_id,vehicle_id,time_stored,chassis_durability,vehicle_name,reason) values (1,301,1700000000,0.5,'BP_Buggy_CHOAM',1);`

func storedList(t *testing.T, o *Ops) (map[string]any, []map[string]any) {
	t.Helper()
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	return m, m["stored"].([]map[string]any)
}

func TestStoredListHasTheRecoveredTheBackedUpAndTheStateOnlyVehicle(t *testing.T) {
	m, list := storedList(t, &Ops{S: testsave.PlayerWithSQL(t, storedWorld)})
	if len(list) != 3 {
		t.Fatalf("three stored vehicles, got %d: %v", len(list), list)
	}
	byID := map[int64]map[string]any{}
	for _, e := range list {
		byID[e["vehicle_id"].(int64)] = e
	}
	want := map[int64][3]string{301: {"recovery", "Stored for recovery", "Buggy"}, 302: {"backup", "In the vehicle backup", "LightOrnithopter"}, 303: {"backup", "In the vehicle backup", "Sandbike"}}
	for id, w := range want {
		e := byID[id]
		if e == nil {
			t.Fatalf("vehicle %d is not in the list: %v", id, list)
		}
		if e["kind"] != w[0] || e["label"] != w[1] || e["type"] != w[2] {
			t.Errorf("vehicle %d: %v, want kind/label/type %v", id, e, w)
		}
	}
	if byID[301]["chassis_durability"] != 0.5 || byID[301]["time_stored"] != int64(1700000000) {
		t.Errorf("the recovered row's durability and time are carried: %v", byID[301])
	}
	if _, ok := byID[302]["chassis_durability"]; ok {
		t.Errorf("the backup row has no durability, none is invented: %v", byID[302])
	}
	if byID[304] != nil {
		t.Errorf("a vehicle standing in the world is not stored: %v", byID[304])
	}
	// the raw rows stay in the answer
	if len(m["recovered"].([]map[string]any)) != 1 || len(m["backups"].([]map[string]any)) != 1 {
		t.Errorf("recovered and backups are kept: %v %v", m["recovered"], m["backups"])
	}
}

func TestStoredListAndTheVehicleTableAgree(t *testing.T) {
	m, list := storedList(t, &Ops{S: testsave.PlayerWithSQL(t, storedWorld)})
	kinds := map[int64]string{}
	for _, e := range list {
		kinds[e["vehicle_id"].(int64)] = e["kind"].(string)
	}
	owned := 0
	for _, v := range m["vehicles"].([]map[string]any) {
		owned++
		id := v["id"].(int64)
		s, _ := v["stored"].(string)
		if s != kinds[id] {
			t.Errorf("vehicle %d: the table says %q, the card says %q", id, s, kinds[id])
		}
		if s != "" && v["storedLabel"] != storedLabel(s) {
			t.Errorf("vehicle %d: label %v", id, v["storedLabel"])
		}
	}
	if owned != 3 {
		t.Errorf("302, 303 and 304 are owned: %d", owned)
	}
	// every stored vehicle the table labels is in the card, which also holds 301, owned by no one now
	for _, id := range []int64{301, 302, 303} {
		if kinds[id] == "" {
			t.Errorf("vehicle %d missing from the card", id)
		}
	}
}

func TestStoredListIsEmptyWhenNothingIsStored(t *testing.T) {
	m, list := storedList(t, &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map,location_x,location_y,location_z,state) values (304,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',4,4,0,0);
insert into vehicles(id) values (304);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (304,'d',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (304,1,1);`)})
	if len(list) != 0 || list == nil {
		t.Fatalf("an empty list, not null: %v", list)
	}
	if len(m["vehicles"].([]map[string]any)) != 1 {
		t.Errorf("the world vehicle is still listed: %v", m["vehicles"])
	}
}

// A stored row whose actor is gone from the class column still gets a type from the row's own name.
func TestStoredListTypeFallsBackToTheRowName(t *testing.T) {
	_, list := storedList(t, &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map,location_x,location_y,location_z,state) values (310,'','HaggaBasin',0,0,0,4);
insert into vehicles(id) values (310);
insert into recovered_vehicles(character_id,vehicle_id,time_stored,chassis_durability,vehicle_name) values (1,310,1700000000,1.0,'BP_Scout_CHOAM');`)})
	if len(list) != 1 || list[0]["type"] != "Scout" {
		t.Fatalf("type from the row name: %v", list)
	}
}
