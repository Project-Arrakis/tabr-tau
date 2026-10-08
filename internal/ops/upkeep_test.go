package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// baseFixture adds placeables on top of the real-schema player save: two cisterns (empty, nearly full), a large
// one already full, an unlisted device with a water component, and three generators (low, full, empty).
const baseFixture = `
insert into actors(id,class,map) values (101,'c','HaggaBasin'),(102,'c','HaggaBasin'),(103,'c','HaggaBasin'),(104,'c','HaggaBasin'),
  (111,'c','HaggaBasin'),(112,'c','HaggaBasin'),(113,'c','HaggaBasin'),(120,'c','HaggaBasin');
insert into fgl_entities(entity_id,components) values
  (-1001, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":0}],"FHealthComponent":[0,{"m_CurrentHealth":1}]}')),
  (-1002, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":4900}]}')),
  (-1003, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":100000}]}')),
  (-1004, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":7}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (101,-1001,'a'),(102,-1002,'a'),(103,-1003,'a'),(104,-1004,'a');
insert into placeables(id,health,building_type) values (101,1250,'WaterCistern_Placeable'),(102,1250,'WaterCistern_Placeable'),
  (103,1250,'LargeWaterCistern_Placeable'),(104,1,'MysteryTank_Placeable'),
  (111,2500,'Generator_Placeable'),(112,2500,'Generator_Placeable'),(113,2500,'Generator_Placeable');
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (301,111,3,5,100.0),(302,112,3,5,100.0),(303,113,3,5,100.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (401,301,228,0,'Oil',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (402,302,500,0,'Oil',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0);
`

func water(t *testing.T, o *Ops, entity int) int64 {
	t.Helper()
	return mustOne(t, o, `select json_extract(components,'$.FWaterStorageComponent[1].m_WaterStored') w from fgl_entities where entity_id=?`, entity)["w"].(int64)
}

func TestRefillBaseWater(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, baseFixture)}
	r, err := o.RefillBaseWater()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["filled"].(int) != 2 || res["alreadyFull"].(int) != 1 {
		t.Fatalf("counts wrong: %v", res)
	}
	if sk := res["skippedUnknown"].([]string); len(sk) != 1 || sk[0] != "mysterytank x1" {
		t.Fatalf("unlisted device not reported: %v", sk)
	}
	if water(t, o, -1001) != 5000 || water(t, o, -1002) != 5000 || water(t, o, -1003) != 100000 || water(t, o, -1004) != 7 {
		t.Fatalf("levels wrong: %d %d %d %d", water(t, o, -1001), water(t, o, -1002), water(t, o, -1003), water(t, o, -1004))
	}
	// the other components survive and the column is still a JSONB blob
	row := mustOne(t, o, `select json_extract(components,'$.FHealthComponent[1].m_CurrentHealth') h, typeof(components) ty, json_valid(components,8) ok from fgl_entities where entity_id=-1001`)
	if row["h"].(int64) != 1 || row["ty"] != "blob" || row["ok"].(int64) != 1 {
		t.Fatalf("components damaged: %v", row)
	}
	if again, _ := o.RefillBaseWater(); again.(map[string]any)["filled"].(int) != 0 {
		t.Fatalf("second run must find nothing: %v", again)
	}
	assertDBSound(t, o)
}

func TestRefillGenerators(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, baseFixture)}
	r, err := o.RefillGenerators()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["filled"].(int) != 2 || res["alreadyFull"].(int) != 1 {
		t.Fatalf("counts wrong: %v", res)
	}
	if mustOne(t, o, `select stack_size s from items where id=401`)["s"].(int64) != 500 {
		t.Fatal("low generator not topped up to 500")
	}
	empty := mustOne(t, o, `select template_id t, stack_size s, position_index p from items where inventory_id=303`)
	if empty["t"] != "Oil" || empty["s"].(int64) != 500 || empty["p"].(int64) != 0 {
		t.Fatalf("empty generator not given a full stack: %v", empty)
	}
	if n := mustOne(t, o, `select count(*) n from items where inventory_id in (301,302,303)`)["n"].(int64); n != 3 {
		t.Fatalf("expected exactly one stack per generator, got %d items", n)
	}
	if again, _ := o.RefillGenerators(); again.(map[string]any)["filled"].(int) != 0 {
		t.Fatalf("second run must find nothing: %v", again)
	}
	assertDBSound(t, o)
	if !strings.Contains(mustOne(t, o, `select stats s from items where inventory_id=303`)["s"].(string), "FItemStackAndDurabilityStats") {
		t.Fatal("new fuel stack has no stats")
	}
}

func TestRepairVehicles(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map) values (201,'c','HaggaBasin');
insert into vehicles(id) values (201);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'mine',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1);
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (1,201,'Engine_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"DecayedMaxDurability":300.5,"LastDeteriorationCause":"x"}],"FCustomizationStats":[[],{}]}')),
  (2,201,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (3,201,'Gen_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":250.0,"DecayedMaxDurability":250.0}]}')),
  (4,201,'Seat_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":90.0}]}'));`)}
	r, err := o.RepairVehicles()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["repaired"].(int64) != 2 || res["withoutKnownMax"].(int64) != 1 || res["modules"].(int64) != 4 {
		t.Fatalf("counts wrong: %v", res)
	}
	cur := func(id int) float64 {
		return mustOne(t, o, `select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') c from vehicle_modules where id=?`, id)["c"].(float64)
	}
	if cur(1) != 300.5 || cur(2) != 250 || cur(3) != 250 || cur(4) != 90 {
		t.Fatalf("durability wrong: %v %v %v %v", cur(1), cur(2), cur(3), cur(4))
	}
	row := mustOne(t, o, `select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].LastDeteriorationCause') c, json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') d, typeof(stats) ty from vehicle_modules where id=1`)
	if row["c"] != "x" || row["d"].(float64) != 300.5 || row["ty"] != "blob" {
		t.Fatalf("other keys or blob type damaged: %v", row)
	}
	assertDBSound(t, o)
}

// Every listed water device type must reach its own capacity, and a damaged component must not stop the others.
func TestRefillBaseWaterEveryTypeAndDamagedComponent(t *testing.T) {
	caps := map[string]int64{"WaterCistern_Placeable": 5000, "MediumWaterCistern_Placeable": 25000, "LargeWaterCistern_Placeable": 100000,
		"WindTrap_Placeable": 500, "LargeWindTrap_Placeable": 500, "BloodWaterExtractor_Placeable": 1000, "BloodWaterExtractionAdvanced_Placeable": 1000}
	sql := ""
	id := 700
	var ids = map[string]int{}
	for typ := range caps {
		id++
		ids[typ] = id
		sql += fmt.Sprintf(`insert into actors(id,class,map) values (%d,'c','HaggaBasin');
			insert into fgl_entities(entity_id,components) values (-%d, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":0.0}]}'));
			insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (%d,-%d,'a');
			insert into placeables(id,health,building_type) values (%d,1,'%s');`, id, id, id, id, id, typ)
	}
	sql += `insert into actors(id,class,map) values (790,'c','HaggaBasin');
		insert into fgl_entities(entity_id,components) values (-790, x'ff00ff');
		insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (790,-790,'a');
		insert into placeables(id,health,building_type) values (790,1,'WaterCistern_Placeable');`
	o := &Ops{S: testsave.PlayerWithSQL(t, sql)}
	r, err := o.RefillBaseWater()
	if err != nil {
		t.Fatalf("a damaged component must be skipped, not abort: %v", err)
	}
	if r.(map[string]any)["filled"].(int) != len(caps) {
		t.Fatalf("%v", r)
	}
	for typ, want := range caps {
		if got := water(t, o, -ids[typ]); got != want {
			t.Errorf("%s: %d want %d", typ, got, want)
		}
	}
}

// A spice generator takes Spice-infused Fuel Cells; a generator whose slots are all taken is skipped and reported
// while the others are still filled.
func TestRefillGeneratorsSpiceAndFullInventory(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map) values (801,'c','HaggaBasin'),(802,'c','HaggaBasin'),(803,'c','HaggaBasin');
insert into placeables(id,health,building_type) values (801,1,'SpiceGenerator_Placeable'),(802,1,'Generator_Placeable'),(803,1,'Generator_Placeable');
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (811,801,3,5,100.0),(812,802,3,1,100.0),(813,803,3,5,100.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (821,812,1,0,'Junk',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0);`)}
	r, err := o.RefillGenerators()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["filled"].(int) != 2 || len(res["skipped"].([]string)) != 1 {
		t.Fatalf("%v", res)
	}
	if mustOne(t, o, `select template_id t from items where inventory_id=811`)["t"] != "SpicedFuelCell" || mustOne(t, o, `select template_id t from items where inventory_id=813`)["t"] != "Oil" {
		t.Fatal("wrong fuel for the generator type")
	}
	if n := mustOne(t, o, `select count(*) n from items where inventory_id=812`)["n"].(int64); n != 1 {
		t.Fatalf("the full generator must be left alone, has %d items", n)
	}
}

// Nothing to fill must leave no pending edit behind (the open-time refill relies on this).
func TestRefillsWithNothingToDoLeaveNoPendingEdit(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, baseFixture)}
	if _, err := o.RefillBaseWater(); err != nil {
		t.Fatal(err)
	}
	if _, err := o.RefillGenerators(); err != nil {
		t.Fatal(err)
	}
	before := len(o.S.Pending())
	if r, _ := o.RefillBaseWater(); r.(map[string]any)["filled"].(int) != 0 {
		t.Fatalf("%v", r)
	}
	if r, _ := o.RefillGenerators(); r.(map[string]any)["filled"].(int) != 0 {
		t.Fatalf("%v", r)
	}
	if after := len(o.S.Pending()); after != before {
		t.Fatalf("a refill that did nothing added %d pending edits", after-before)
	}
}
