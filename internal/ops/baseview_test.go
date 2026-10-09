package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// One base (totem 601, entity -601) with two generators (one holds 200 oil, one is empty), a windtrap, a wind turbine, two water
// devices, a storage chest, a refinery and a building piece; plus a second base (totem 602, entity -602) with a generator and a
// water device, and a hologram generator that must not count.
const baseViewWorld = `
insert into actors(id,class,map,location_x,location_y,location_z) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7),(602,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',90000,90000,0),
  (701,'p','HaggaBasin',0,0,0),(702,'p','HaggaBasin',0,0,0),(703,'p','HaggaBasin',0,0,0),(704,'p','HaggaBasin',0,0,0),(705,'p','HaggaBasin',0,0,0),
  (706,'p','HaggaBasin',0,0,0),(707,'p','HaggaBasin',0,0,0),(708,'p','HaggaBasin',0,0,0),(709,'p','HaggaBasin',0,0,0),(710,'p','HaggaBasin',0,0,0),(711,'p','HaggaBasin',0,0,0);
insert into totems(id) values (601),(602);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}')), (-602, jsonb('{}')),
  (-705, jsonb('{"FWaterStorageComponent":[[],{"m_WaterStored":5000}]}')), (-706, jsonb('{"FWaterStorageComponent":[[],{"m_WaterStored":10000}]}')),
  (-710, jsonb('{"FWaterStorageComponent":[[],{"m_WaterStored":5000}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor'),(602,-602,'Actor'),(705,-705,'Water'),(706,-706,'Water'),(710,-710,'Water');
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id,is_hologram) values
  (601,-601,'Totem_Placeable',1,0),
  (701,-601,'Generator_Placeable',1,0),(702,-601,'Generator_Placeable',1,0),(703,-601,'Windtrap_Placeable',1,0),(704,-601,'WindTurbineOmnidirectional_Placeable',1,0),
  (705,-601,'WaterCistern_Placeable',1,0),(706,-601,'MediumWaterCistern_Placeable',1,0),(707,-601,'GenericContainer_Placeable',1,0),(708,-601,'SmallOreRefinery_Placeable',1,0),
  (709,-601,'Generator_Placeable',1,1),(710,-602,'WaterCistern_Placeable',1,0),(711,-602,'Generator_Placeable',1,0);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (601,'Advanced Sub-Fief Console',1,3,1),(707,'Ore Out',1,3,1);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values
  (7001,701,3,5,100.0),(7002,702,3,5,100.0),(7004,704,3,5,100.0),(7007,707,4,20,1000.0),(7008,708,12,5,1000.0),(7011,711,3,5,100.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (9001,7001,200,0,'Oil',0,1790000000,'{}',0),(9004,7004,50,0,'WindTurbineLubricant1',0,1790000000,'{}',0),(9007,7007,3,0,'CopperOre',0,1790000000,'{}',0),
  (9008,7008,7,0,'IronOre',0,1790000000,'{}',0),(9011,7011,500,0,'Oil',0,1790000000,'{}',0);
insert into building_instances(building_id,instance_id,building_type,owner_entity_id) values (601,1,'Foundation',-601),(601,2,'Wall',-601),(602,1,'Foundation',-602);`

func baseOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, baseViewWorld)} }

func TestBaseListHasOneRowPerBaseWithItsCounts(t *testing.T) {
	r, err := baseOps(t).BaseList()
	if err != nil {
		t.Fatal(err)
	}
	rows := r.([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("two bases: %v", rows)
	}
	b := rows[0]
	if b["base_id"].(int64) != 601 || b["name"] != "Advanced Sub-Fief Console" || b["base_type"] != "Advanced Sub-Fief" || b["owner"] != "Tester" || b["map"] != "HaggaBasin" || b["x"].(float64) != 5000 {
		t.Fatalf("first base: %v", b)
	}
	// its pieces: 2 building instances; its placeables: the totem, 3 generator/windtrap devices... counted by what it owns
	if b["pieces"].(int64) != 2 || b["placeables"].(int64) != 9 || b["generators"].(int64) != 3 {
		t.Fatalf("counts: pieces=%v placeables=%v generators=%v", b["pieces"], b["placeables"], b["generators"])
	}
	if rows[1]["base_id"].(int64) != 602 || rows[1]["generators"].(int64) != 1 || rows[1]["pieces"].(int64) != 1 {
		t.Fatalf("second base: %v", rows[1])
	}
}

func TestBasePowerGroupsDevicesAndCountsWhatTheyHold(t *testing.T) {
	r, err := baseOps(t).BasePower(601)
	if err != nil {
		t.Fatal(err)
	}
	types := r.(map[string]any)["types"].([]map[string]any)
	by := map[string]map[string]any{}
	for _, ty := range types {
		by[ty["kind"].(string)] = ty
	}
	g := by["generator_placeable"]
	// two generators (the hologram and the other base's do not count), one holds 200 oil, one is empty; full is 500 each
	if g["devices"].(int64) != 2 || g["queued"].(int64) != 200 || g["empty"].(int64) != 1 || g["capacity"].(int64) != 1000 || g["fuel"] != "Oil" || g["refillable"] != true {
		t.Fatalf("generators: %v", g)
	}
	if w := by["windtrap_placeable"]; w["devices"].(int64) != 1 || w["empty"].(int64) != 1 {
		t.Fatalf("windtrap: %v", w)
	}
	if tb := by["windturbineomnidirectional_placeable"]; tb["queued"].(int64) != 50 || tb["refillable"] != false {
		t.Fatalf("wind turbine is shown but not refillable: %v", tb)
	}
	if _, has := by["spicegenerator_placeable"]; has {
		t.Fatal("a kind the base does not have is not listed")
	}
}

func TestBaseWaterSumsStoredAndCapacityPerType(t *testing.T) {
	r, err := baseOps(t).BaseWater(601)
	if err != nil {
		t.Fatal(err)
	}
	types := r.(map[string]any)["types"].([]map[string]any)
	if len(types) != 2 {
		t.Fatalf("two kinds in this base: %v", types)
	}
	by := map[string]map[string]any{}
	for _, ty := range types {
		by[ty["kind"].(string)] = ty
	}
	m := by["mediumwatercistern_placeable"]
	if m["name"] != "Medium Water Cistern" || m["devices"].(int64) != 1 || m["stored"].(int64) != 10000 || m["capacity"].(int64) != 25000 || m["fillPercent"].(float64) != 40 {
		t.Fatalf("medium cistern: %v", m)
	}
	if c := by["watercistern_placeable"]; c["stored"].(int64) != 5000 || c["capacity"].(int64) != 5000 || c["fillPercent"].(float64) != 100 {
		t.Fatalf("cistern: %v", c)
	}
}

func TestBaseInventoryHasTheFourGroupsForOneBase(t *testing.T) {
	r, err := baseOps(t).BaseInventory(601)
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	counts := map[string]int{}
	for _, g := range res["groups"].([]map[string]any) {
		counts[g["id"].(string)] = g["count"].(int)
	}
	// the chest is storage, the ore refinery refining, the totem "other" (its own inventory row has none here, so it is not listed)
	if counts["storage"] != 1 || counts["refining"] != 1 || counts["crafting"] != 0 || counts["other"] != 0 {
		t.Fatalf("groups: %v", counts)
	}
	for _, c := range res["containers"].([]map[string]any) {
		if c["base_id"].(int64) != 601 {
			t.Errorf("a container of another base: %v", c)
		}
	}
	chest := res["containers"].([]map[string]any)[0]
	if chest["name"] != "Ore Out" || chest["group"] != "storage" || chest["type"] != "Chest" {
		t.Fatalf("chest: %v", chest)
	}
}
