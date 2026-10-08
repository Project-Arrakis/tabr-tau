package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// One base (totem 601, entity -601) with two storage containers, plus everything that must not show up as base storage.
const storageWorld = `
insert into actors(id,class,map) values
  (601,'/Game/Dune/Systems/Building/Pieces/BP_Totem.BP_Totem_C','HaggaBasin'),
  (602,'/Game/Other/BP_NotATotem.BP_NotATotem_C','HaggaBasin'),
  (709,'p','HaggaBasin'),(710,'p','HaggaBasin'),(701,'p','HaggaBasin'),(702,'p','HaggaBasin'),(703,'p','HaggaBasin'),(704,'p','HaggaBasin'),(705,'p','HaggaBasin'),(706,'p','HaggaBasin'),(707,'p','HaggaBasin'),
  (708,'/Game/BP_LootContainer.BP_LootContainer_C','HaggaBasin');
insert into totems(id) values (601);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}')), (-602, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor'),(602,-602,'Actor');
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id) values
  (701,-601,'GenericContainer_Placeable',1),
  (702,-601,'SpiceSilo_Placeable',1),
  (703,-601,'SmallOreRefinery_Placeable',1),
  (704,-601,'Generator_Placeable',1),
  (705,NULL,'GenericContainer_Placeable',1),
  (706,-602,'GenericContainer_Placeable',1),
  (707,-601,'StorageContainer_Placeable',1);
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id,is_hologram) values (709,-601,'GenericContainer_Placeable',1,1);
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id) values (710,-601,'GenericContainer_Placeable',1);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (701,'Ore Storage',1,3,1),(702,'##SpiceSilo_Placeable',1,3,1),(710,'None',1,3,1);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values
  (7001,701,4,20,1000.0),(7002,702,4,10,1000.0),(7003,703,12,5,1000.0),(7004,704,3,5,1000.0),(7005,705,4,20,1000.0),(7006,706,4,20,1000.0),(7007,707,4,-1,1000.0),(7008,708,4,5,1000.0),(7009,709,4,20,1000.0),(7010,710,4,20,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (9001,7001,500,0,'CopperOre',0,1790000000,'{}',0),(9002,7001,3,1,'Knife',0,1790000000,'{}',0),(9003,7003,7,0,'IronOre',0,1790000000,'{}',0);`

func storageOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, storageWorld)} }

func TestStorageListsOnlyTheBasesStorageContainers(t *testing.T) {
	r, err := storageOps(t).Storage()
	if err != nil {
		t.Fatal(err)
	}
	rows := r.([]map[string]any)
	if len(rows) != 3 {
		t.Fatalf("want the 3 storage containers of the base (a hologram is not one), got %d: %v", len(rows), rows)
	}
	a, b, c := rows[0], rows[1], rows[2]
	if c["actor_id"].(int64) != 710 || c["name"] != "Chest" {
		t.Fatalf("a container named None shows its type name: %v", c)
	}
	if a["actor_id"].(int64) != 701 || a["name"] != "Ore Storage" || a["type"] != "Chest" || a["items"].(int64) != 2 || a["max_item_count"].(int64) != 20 || a["base_id"].(int64) != 601 {
		t.Fatalf("first container wrong: %v", a)
	}
	if b["actor_id"].(int64) != 702 || b["name"] != "Small Storage Container" || b["items"].(int64) != 0 || b["max_item_count"].(int64) != 10 {
		t.Fatalf("an unnamed container shows its type name and no items: %v", b)
	}
}

// Fuel slots, refineries, containers with no capacity, containers outside a base, vehicles and world loot never count as base storage.
func TestStorageExcludesEverythingElse(t *testing.T) {
	r, err := storageOps(t).Storage()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range r.([]map[string]any) {
		switch row["actor_id"].(int64) {
		case 703, 704, 705, 706, 707, 708, 709:
			t.Errorf("actor %v must not be listed as base storage", row["actor_id"])
		}
	}
}

func TestStorageEmptyWithoutABase(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	r, err := o.Storage()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.([]map[string]any)) != 0 {
		t.Fatalf("no base, no storage: %v", r)
	}
}
