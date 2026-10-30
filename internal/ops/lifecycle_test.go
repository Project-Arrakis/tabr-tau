package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #403: the game gives every actor a lifecycle state (actors.state). Read from real saves (24 saves of one played character, 2026-10-25): a vehicle in the
// character's vehicle backup (backup_vehicles) has state 2, a vehicle stored for recovery (recovered_vehicles) has state 4, an ordinary vehicle 0. The
// backup one keeps its owner record (the character still ranks as its owner) and its last position, so it looked like a vehicle standing in the world.
//
// 201 stands in the world. 202 is in the vehicle backup: state 2, the backup row, and it is still owned. 203 has state 4 and is owned, with no table row.
// 204 has state 0 but a recovered_vehicles row. 205 is stored for recovery the way the game does it: state 4, the row, and no owner record.
const lifecycleWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,state) values
  (201,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',10,20,30,0),
  (202,'/Game/BP_LightOrnithopter_Choam.BP_LightOrnithopter_Choam_C','DeepDesert',5000,6000,0,2),
  (203,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',7000,8000,0,4),
  (204,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',7100,8100,0,0),
  (205,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',7200,8200,0,4);
insert into vehicles(id) values (201),(202),(203),(204),(205);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'a',2,3,0),(202,'b',2,3,0),(203,'c',2,3,0),(204,'d',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1),(202,1,1),(203,1,1),(204,1,1);
insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,202,'None');
insert into recovered_vehicles(character_id,vehicle_id,chassis_durability) values (1,204,0.5),(1,205,0.25);`

func lifecycleOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, lifecycleWorld)} }

func TestVehiclesSayWhichOnesAreStored(t *testing.T) {
	r, err := lifecycleOps(t).Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]string{}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		s, _ := v["stored"].(string)
		got[v["id"].(int64)] = s
		if s != "" && v["storedLabel"] == "" {
			t.Errorf("a stored vehicle carries the words for it: %v", v)
		}
	}
	want := map[int64]string{201: "", 202: "backup", 203: "recovery", 204: "recovery"}
	for id, w := range want {
		if g, ok := got[id]; !ok || g != w {
			t.Errorf("vehicle %d: %q (listed %v), want %q", id, g, ok, w)
		}
	}
	if len(got) != 4 {
		t.Errorf("the owned vehicles are all listed (205 has no owner record): %v", got)
	}
}

func TestStoredVehiclesAreNotPlottedOnTheSavedMapButAreCounted(t *testing.T) {
	r, err := lifecycleOps(t).LiveMap("HaggaBasin")
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	v := m["vehicles"].([]map[string]any)
	if len(v) != 1 || v[0]["id"].(int64) != 201 {
		t.Fatalf("only the vehicle standing in the world is plotted: %v", v)
	}
	// 203 and 204 are stored on this map, 202 is on the Deep Desert; the count is of the stored vehicles on this map
	if m["storedVehicles"].(int64) != 2 {
		t.Errorf("stored vehicles on this map: %v", m["storedVehicles"])
	}
	dd, _ := lifecycleOps(t).LiveMap("DeepDesert")
	if n := dd.(map[string]any)["storedVehicles"].(int64); n != 1 || len(dd.(map[string]any)["vehicles"].([]map[string]any)) != 0 {
		t.Errorf("the vehicle in the backup is not on the Deep Desert map: %v", dd)
	}
}

func TestBringRefusesAStoredVehicleAndMovesTheOthers(t *testing.T) {
	for _, id := range []float64{202, 203, 204} {
		o := lifecycleOps(t)
		before, _ := o.S.One(`select location_x x, location_y y from actors where id=?`, int64(id))
		_, err := o.BringVehicle(Args{"id": id})
		if err == nil || !strings.Contains(err.Error(), "stored") {
			t.Errorf("vehicle %v: want a refusal that says it is stored, got %v", id, err)
		}
		after, _ := o.S.One(`select location_x x, location_y y from actors where id=?`, int64(id))
		if before["x"] != after["x"] || before["y"] != after["y"] {
			t.Errorf("vehicle %v moved: %v -> %v", id, before, after)
		}
	}
	o := lifecycleOps(t)
	if _, err := o.BringVehicle(Args{"id": 201.0}); err != nil {
		t.Fatalf("a vehicle in the world is brought as before: %v", err)
	}
	if a, _ := o.S.One(`select location_x x from actors where id=201`); a["x"].(float64) != 100+800 {
		t.Errorf("vehicle 201 is next to the player: %v", a)
	}
}

func TestDuplicateRefusesAStoredVehicleAndChangesNothing(t *testing.T) {
	o := lifecycleOps(t)
	before, _ := o.S.One(`select count(*) n from actors`)
	for _, id := range []float64{202, 203, 204} {
		_, err := o.DuplicateVehicle(Args{"id": id})
		if err == nil || !strings.Contains(err.Error(), "stored") {
			t.Errorf("vehicle %v: want a refusal that says it is stored, got %v", id, err)
		}
		if err != nil && !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("a refusal of the request, not a fault: %v", err)
		}
	}
	after, _ := o.S.One(`select count(*) n from actors`)
	if before["n"] != after["n"] {
		t.Errorf("actors %v -> %v", before["n"], after["n"])
	}
}

func TestStoredVehiclesAreNotTeleportDestinations(t *testing.T) {
	o := lifecycleOps(t)
	d := listDestinations(t, o, groupVehicle, "")
	if len(d) != 1 || !strings.Contains(d[0].Label, "#201") {
		t.Fatalf("only the vehicle in the world: %+v", d)
	}
}

// A base that was picked up with the game's base tool keeps its structures but has no permission record, and its claim actor is linked in
// base_backup_linked_actors (the console's documentation of the tool; none of the saves at hand has one, so this is built from that description).
const pickedUpBase = `
insert into actors(id,class,map,location_x,location_y,location_z) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,0),(602,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',6000,6000,0),(603,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',7000,6000,0);
insert into totems(id) values (601),(602),(603);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (601,'Home',1,3,0);
insert into base_backups(id,player_id,base_backup_name) values (1,2,'Old camp');
insert into base_backup_linked_actors(id,actor_id) values (1,602);`

func TestBasesPickedUpWithTheBaseToolAreLabelled(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, pickedUpBase)}
	r, err := o.BaseList()
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]any{}
	for _, b := range r.([]map[string]any) {
		got[b["base_id"].(int64)] = b["picked_up"]
	}
	if got[601] != false || got[602] != true || got[603] != false {
		t.Errorf("picked_up by base: %v (601 is claimed, 602 is picked up, 603 is just unclaimed)", got)
	}
}

// A base that is claimed again keeps its backup rows in the table; the permission record says it is in use.
func TestAClaimedBaseIsNotCalledPickedUpEvenWithABackupRow(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, pickedUpBase+`insert into base_backup_linked_actors(id,actor_id) values (1,601);`)}
	r, _ := o.BaseList()
	for _, b := range r.([]map[string]any) {
		if b["base_id"].(int64) == 601 && b["picked_up"] != false {
			t.Errorf("601 has a permission record: %v", b)
		}
	}
}
