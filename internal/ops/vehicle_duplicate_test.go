package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Duplicate this vehicle (#112, the console's Spawn Vehicle): a copy of one of the character's own vehicles, made of the rows a vehicle is in a
// save the game wrote (read from a real one: the actor, the vehicles row, the permission record and its rank, two entities linked as Actor and
// VehicleInventory whose ids share a prefix and count up in the low bits, three inventories with their component hashes, and the modules).
//
// The Sandbike 201 of vehicleWorld is the reference: it gets two entities, a cargo hold with a stack in it, a component hold, and the hashes. 202 is a
// world vehicle (no permission record) and 203 belongs to someone else.
const duplicateWorld = vehicleDetailsWorld + `
update actors set rotation_x=0.0, rotation_y=0.0, rotation_z=0.5, rotation_w=0.8660254, serial=103,
  properties=jsonb('{"BP_Sandbike_CHOAM_C":{"m_HarnessedVehicleId":"!!act#0","m_InitialFuel":0.0,"m_AbandonedTimestamp":"2020.01.02-03.04.05","m_bIsInSafezone":true},"DamageableActorComponent":{"m_CurrentMaxHealth":400.0}}') where id=201;
update fgl_entities set entity_id=2077000000100015 where entity_id=-201;
update actor_fgl_entities set entity_id=2077000000100015 where entity_id=-201;
insert into fgl_entities(entity_id,components) values (2077000000100010, jsonb('{"VehicleInventory":true}')), (2077000000900050, jsonb('{"Other":1}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (201,2077000000100010,'VehicleInventory');
insert into actor_inventories(inventory_id,component_name_hash) values (8001,-72424899),(8002,462019337);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8190,8001,9,0,'CopperOre',0,1790000000,'{}',0);`

func dupOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, duplicateWorld)} }

func newVehicleID(t *testing.T, r any) int64 {
	t.Helper()
	m, ok := r.(map[string]any)
	if !ok {
		t.Fatalf("answer: %v", r)
	}
	id, ok := m["id"].(int64)
	if !ok || id <= 0 {
		t.Fatalf("the answer names the new vehicle: %v", m)
	}
	return id
}

func TestDuplicateVehicleMakesACopyNextToTheCharacterWithTheRowsAVehicleIsMadeOf(t *testing.T) {
	o := dupOps(t)
	r, err := o.DuplicateVehicle(Args{"id": 201.0})
	if err != nil {
		t.Fatal(err)
	}
	id := newVehicleID(t, r)
	if id == 201 {
		t.Fatal("a new vehicle")
	}
	a := mustOne(t, o, `select class, map, location_x x, location_y y, location_z z, rotation_z rz, rotation_w rw, serial, json_extract(properties,'$.BP_Sandbike_CHOAM_C.m_AbandonedTimestamp') ts,
		json_extract(properties,'$.BP_Sandbike_CHOAM_C.m_bIsInSafezone') sz from actors where id=?`, id)
	// next to the character (100, 200, 300 on HaggaBasin), as Bring vehicle puts a vehicle; the turn and the rest are the reference's
	if a["class"] != "/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C" || a["map"] != "HaggaBasin" || a["x"] != 900.0 || a["y"] != 200.0 || a["z"] != 450.0 || a["rz"] != 0.5 || a["serial"] != int64(103) {
		t.Errorf("the actor: %v", a)
	}
	if a["ts"] == "2020.01.02-03.04.05" || !strings.HasPrefix(fmt.Sprint(a["ts"]), "20") || len(fmt.Sprint(a["ts"])) != len("2020.01.02-03.04.05") {
		t.Errorf("the vehicle is not abandoned: its abandoned time is now, in the game's format: %v", a["ts"])
	}
	if a["sz"] == nil {
		t.Errorf("the other properties are copied: %v", a)
	}
	// the permission record and the rank make it the character's
	if n := mustOne(t, o, `select count(*) c from vehicles where id=?`, id)["c"].(int64); n != 1 {
		t.Error("vehicles row")
	}
	pa := mustOne(t, o, `select actor_name n, actor_type t, access_level l from permission_actor where actor_id=?`, id)
	if pa["n"] != "mine" || pa["t"] != int64(2) || pa["l"] != int64(3) {
		t.Errorf("permission record: %v", pa)
	}
	if rk := mustOne(t, o, `select player_id p, rank r from permission_actor_rank where permission_actor_id=?`, id); rk["p"] != int64(1) || rk["r"] != int64(1) {
		t.Errorf("the rank: %v", rk)
	}
	// two entities with the slots of the reference, new ids that share the reference's prefix, and the same components
	ents, _ := o.S.Query(`select a.slot_name s, a.entity_id e, json(f.components) c from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id where a.actor_id=? order by a.slot_name`, id)
	if len(ents) != 2 || ents[0]["s"] != "Actor" || ents[1]["s"] != "VehicleInventory" {
		t.Fatalf("entities: %v", ents)
	}
	ref := mustOne(t, o, `select entity_id e from actor_fgl_entities where actor_id=201 and slot_name='VehicleInventory'`)["e"].(int64)
	for _, e := range ents {
		if e["e"].(int64) == ref || e["e"].(int64)>>32 != ref>>32 {
			t.Errorf("a new entity id with the reference's prefix: %v vs %v", e["e"], ref)
		}
	}
	if ents[1]["c"] != `{"VehicleInventory":true}` || !strings.Contains(fmt.Sprint(ents[0]["c"]), `"CurrentFuel":90.5`) {
		t.Errorf("components: %v", ents)
	}
	if ents[0]["e"].(int64)-ents[1]["e"].(int64) != 5 {
		t.Errorf("the Actor entity keeps its distance from the VehicleInventory one: %v", ents)
	}
	if low := ents[1]["e"].(int64) & 0xFFFFFFFF; low <= 2077000000900050&0xFFFFFFFF {
		t.Errorf("above every id the prefix has: %x", low)
	}
	// inventories: the cargo hold and the component hold, empty, with the hashes
	invs, _ := o.S.Query(`select v.id i, v.inventory_type t, v.max_item_count n, v.max_item_volume vol, h.component_name_hash h,
		(select count(*) from items where inventory_id=v.id) items from inventories v left join actor_inventories h on h.inventory_id=v.id where v.actor_id=? order by v.id`, id)
	if len(invs) != 2 || invs[0]["t"] != int64(0) || invs[0]["n"] != int64(15) || invs[0]["vol"] != 500.0 || invs[0]["h"] != int64(-72424899) || invs[1]["t"] != nil || invs[1]["h"] != int64(462019337) {
		t.Errorf("inventories: %v", invs)
	}
	if invs[0]["items"] != int64(0) || invs[1]["items"] != int64(0) {
		t.Errorf("a new vehicle's holds are empty: %v", invs)
	}
	// the modules, with the reference's stats and new ids
	mods, _ := o.S.Query(`select template_id t, json(stats) s from vehicle_modules where vehicle_id=? order by template_id`, id)
	refm, _ := o.S.Query(`select template_id t, json(stats) s from vehicle_modules where vehicle_id=201 order by template_id`)
	if len(mods) != 3 || len(mods) != len(refm) {
		t.Fatalf("modules: %v", mods)
	}
	for i := range mods {
		if mods[i]["t"] != refm[i]["t"] || mods[i]["s"] != refm[i]["s"] {
			t.Errorf("module %d: %v vs %v", i, mods[i], refm[i])
		}
	}
	if n := mustOne(t, o, `select count(*) c from vehicle_modules where id in (select id from vehicle_modules where vehicle_id=201) and vehicle_id=?`, id)["c"].(int64); n != 0 {
		t.Error("new module rows, not the reference's")
	}
}

func TestDuplicateVehicleChangesNothingOfTheReferenceAndCanBeRepeated(t *testing.T) {
	o := dupOps(t)
	refRows := func() string {
		var out []string
		for _, q := range []string{
			`select * from actors where id=201`, `select * from vehicles where id=201`, `select * from permission_actor where actor_id=201`, `select * from permission_actor_rank where permission_actor_id=201`,
			`select * from actor_fgl_entities where actor_id=201`, `select * from inventories where actor_id=201`, `select * from items where inventory_id in (8001,8002)`,
			`select * from vehicle_modules where vehicle_id=201`, `select * from actor_inventories where inventory_id in (8001,8002)`,
		} {
			rows, err := o.S.Query(q)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprint(rows))
		}
		return strings.Join(out, "\n")
	}
	before := refRows()
	r1, err := o.DuplicateVehicle(Args{"id": 201.0})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := o.DuplicateVehicle(Args{"id": 201.0})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := newVehicleID(t, r1), newVehicleID(t, r2); a == b {
		t.Fatal("two copies are two vehicles")
	}
	if refRows() != before {
		t.Fatal("the reference vehicle, its cargo and its modules are as they were")
	}
	if n := mustOne(t, o, `select count(distinct entity_id) c from actor_fgl_entities where actor_id in (201,?,?)`, newVehicleID(t, r1), newVehicleID(t, r2))["c"].(int64); n != 6 {
		t.Errorf("six distinct entities for three vehicles: %d", n)
	}
	v, _ := o.Vehicles()
	if n := len(v.(map[string]any)["vehicles"].([]map[string]any)); n != 3 {
		t.Errorf("the character owns three vehicles now: %d", n)
	}
	if rows, err := o.S.Query(`pragma foreign_key_check`); err != nil || len(rows) != 0 {
		t.Errorf("foreign keys: %v %v", rows, err)
	}
	if r := mustOne(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Errorf("integrity: %v", r)
	}
}

func TestDuplicateVehicleRefusesWhatIsNotTheCharactersVehicleAndChangesNothing(t *testing.T) {
	o := dupOps(t)
	for name, a := range map[string]Args{
		"a vehicle that is someone else's": {"id": 203.0},
		"a world vehicle":                  {"id": 202.0},
		"an actor that is not a vehicle":   {"id": 9.0},
		"an id that does not exist":        {"id": 424242.0},
		"no id":                            {},
		"a negative id":                    {"id": -201.0},
	} {
		if _, err := o.DuplicateVehicle(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused copy must not change the save")
	}
	_, err := o.DuplicateVehicle(Args{"id": 203.0})
	wantRefusal(t, err, apierr.ErrNotFound, "someone else's vehicle")
}

func TestDuplicateVehicleWithoutEntitiesStillMakesAVehicleTheGameCanLoad(t *testing.T) {
	// a vehicle with no entity row has nothing to copy for them; a vehicle with an inventory that hangs from an item or a module is refused, because
	// where the copy's would hang from is not something the editor can know
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld)}
	r, err := o.DuplicateVehicle(Args{"id": 201.0})
	if err != nil {
		t.Fatal(err)
	}
	id := newVehicleID(t, r)
	if n := mustOne(t, o, `select count(*) c from actor_fgl_entities where actor_id=?`, id)["c"].(int64); n != 0 {
		t.Errorf("no entities to copy: %d", n)
	}
	o = &Ops{S: testsave.PlayerWithSQL(t, duplicateWorld+`update inventories set vehicle_module_id=11 where id=8002;`)}
	if _, err := o.DuplicateVehicle(Args{"id": 201.0}); err == nil || !strings.Contains(err.Error(), "does not know how to copy") {
		t.Errorf("an inventory that hangs from a module is not copied blindly: %v", err)
	}
	if o.S.Dirty() {
		t.Fatal("nothing may change")
	}
}

func TestEntityIDsOfACopyKeepThePrefixAndLeaveRoomForTheGame(t *testing.T) {
	ids := []int64{0x7b81742000019d60, 0x7b81742000019d65}
	taken := map[int64]bool{0x7b81742000019d60: true, 0x7b81742000019d65: true, 0x7b8174200001a000: true}
	got, err := newEntityIDs(ids, taken)
	if err != nil {
		t.Fatal(err)
	}
	if got[0]>>32 != 0x7b817420 || got[1]-got[0] != 5 {
		t.Errorf("same prefix, same distance: %x %x", got[0], got[1])
	}
	if got[0]&0xFFFFFFFF <= 0x1a000 {
		t.Errorf("above everything the prefix already has: %x", got[0])
	}
	for _, g := range got {
		if taken[g] {
			t.Errorf("%x is taken", g)
		}
	}
	// the last room of a prefix is not enough: refuse rather than wrap into another prefix
	if _, err := newEntityIDs([]int64{0x7b817420fffffff8}, map[int64]bool{0x7b817420fffffff8: true}); err == nil {
		t.Error("an exhausted prefix must be refused")
	}
}

// CR-10: the copy gets the character's rank only. Another player's rank on the original (here the rank 2 of actor 203) is not carried over.
func TestDuplicateVehicleCopiesOnlyTheCharactersRankNotAnotherPlayers(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, duplicateWorld+`
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,203,2);`)}
	if n := mustOne(t, o, `select count(*) c from permission_actor_rank where permission_actor_id=201`)["c"].(int64); n != 2 {
		t.Fatalf("the reference has two ranks: %d", n)
	}
	r, err := o.DuplicateVehicle(Args{"id": 201.0})
	if err != nil {
		t.Fatal(err)
	}
	id := newVehicleID(t, r)
	rows, _ := o.S.Query(`select player_id p, rank r from permission_actor_rank where permission_actor_id=?`, id)
	if len(rows) != 1 || rows[0]["p"] != int64(1) || rows[0]["r"] != int64(1) {
		t.Fatalf("only the character's rank is copied: %v", rows)
	}
	if n := mustOne(t, o, `select count(*) c from permission_actor_rank where permission_actor_id=201`)["c"].(int64); n != 2 {
		t.Fatalf("the reference keeps its ranks: %d", n)
	}
}
