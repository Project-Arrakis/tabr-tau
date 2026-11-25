package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Recycle a base and return everything to the Reserve (#1176). The world: the character's Reserve (inventory 12, type 33, with one stack in it), a standing base of the
// character's (totem 601; a building actor with a foundation, two walls and a door frame; a silo with two stacks; the frame's door; a Developer storage container with one
// stack; a hologram; a fabricator with an idle queue and a stack in a slot; a cistern with water; a map marker; land claim segments; fuel times), a base of another player, a
// base the character is only a member of, a base the game already picked up, a vehicle of the character's with a stack in its inventory, and four respawn points (at the
// character's base, at the other player's base, at the vehicle, and one that is at no actor).
//
// The building costs are the game's own (internal/catalog/buildcosts.json): Sardaukar_Foundation Stone 15, Sardaukar_Wall_01 Stone 10, Sardaukar_Door_Frame_Tall_L Stone 7,
// MTX_Sardaukar_TallDoor_Placeable_L Stone 10, Totem_Placeable ScrapMetal 40, SpiceSilo_Placeable ScrapMetal 35, Fabricator_Placeable ScrapMetal 75,
// LargeWaterCistern_Placeable DuraluminumRod 115 + Silicone 120 + T4HarkSpiceSiloComponent1 20.
const recycleWorld = `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (12,2,33,-1,0.0);
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (99,'/Game/BP_Other.BP_Other_C','HaggaBasin',0,0,0,0,0,0,1,0,0,1,1,jsonb('{}')),
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000.9,6000.4,7.7,0,0,0,1,0,0,10,1,jsonb('{}')),
  (602,'/Game/BP_Building.BP_Building_C','HaggaBasin',5001,6001,8,0,0,0,1,0,0,22,1,jsonb('{}')),
  (701,'/Game/BP_Silo.BP_Silo_C','HaggaBasin',5010,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (702,'/Game/BP_Door.BP_Door_C','HaggaBasin',5020,6000,7,0,0,0,1,0,0,6,1,jsonb('{}')),
  (703,'/Game/BP_DevBox.BP_DevBox_C','HaggaBasin',5030,6000,7,0,0,0,1,0,0,6,1,jsonb('{}')),
  (704,'/Game/BP_Turbine.BP_Turbine_C','HaggaBasin',5040,6000,7,0,0,0,1,0,0,6,1,jsonb('{}')),
  (705,'/Game/BP_Fabricator.BP_Fabricator_C','HaggaBasin',5050,6000,7,0,0,0,1,0,0,6,1,jsonb('{}')),
  (706,'/Game/BP_Cistern.BP_Cistern_C','HaggaBasin',5060,6000,7,0,0,0,1,0,0,6,1,jsonb('{}')),
  (801,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',90000,90000,0,0,0,0,1,0,0,7,1,jsonb('{}')),
  (802,'/Game/BP_Building.BP_Building_C','HaggaBasin',90000,90000,0,0,0,0,1,0,0,8,1,jsonb('{}')),
  (811,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',80000,80000,0,0,0,0,1,0,0,7,1,jsonb('{}')),
  (812,'/Game/BP_Building.BP_Building_C','HaggaBasin',80000,80000,0,0,0,0,1,0,0,8,1,jsonb('{}')),
  (901,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',70000,70000,0,0,0,0,1,0,5,3,1,jsonb('{}')),
  (902,'/Game/BP_Building.BP_Building_C','HaggaBasin',70000,70000,0,0,0,0,1,0,5,4,1,jsonb('{}')),
  (950,'/Game/BP_Buggy.BP_Buggy_C','HaggaBasin',5005,6005,7,0,0,0,1,0,0,4,1,jsonb('{}'));
insert into fgl_entities(entity_id,components) values
  (5764607523034234881, jsonb('{"FTotemComponent":[16,{}]}')),(5764607523034234882, jsonb('{}')),(5764607523034234883, jsonb('{}')),(5764607523034234884, jsonb('{}')),
  (5764607523034234885, jsonb('{"FItemCraftingComponent":[0,{"RequestsQueue":[],"State":"Idle"}]}')),
  (5764607523034234886, jsonb('{"FWaterStorageComponent":[1,{"m_WaterStored":500}]}')),
  (5764607523034234890, jsonb('{}')),
  (5764607523034235001, jsonb('{}')),(5764607523034235101, jsonb('{}')),(5764607523034235201, jsonb('{}')),(5764607523034235301, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values
  (601,5764607523034234881,'Actor'),(701,5764607523034234882,'Actor'),(701,5764607523034234883,'ContainerInventory'),(703,5764607523034234884,'Actor'),
  (705,5764607523034234885,'ItemCraftingComponent'),(706,5764607523034234886,'Actor'),(602,5764607523034234890,'Actor'),
  (801,5764607523034235001,'Actor'),(811,5764607523034235101,'Actor'),(901,5764607523034235201,'Actor'),(950,5764607523034235301,'Actor');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values
  (601,'My base',3,3,0,0),(701,'##SpiceSilo_Placeable',1,3,1,0),(702,'##Door',1,3,1,0),(801,'Other base',3,3,0,0),(811,'Member base',3,3,0,0),(950,'Buggy',2,3,0,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(701,1,1),(801,99,1),(811,99,1),(811,1,2),(950,1,1);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values
  (7000,601,3,5,50.0),(7001,701,4,10,250.0),(7003,703,4,20,1000.0),(7005,705,12,4,0.0),(7006,705,12,4,0.0),(7050,950,4,10,100.0);
insert into actor_inventories(inventory_id,component_name_hash) values (7000,111),(7001,111),(7003,111),(7005,222),(7006,333),(7050,111);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values
  (9001,7001,300,0,'CopperOre',0,1790000001,'{"a":1}',0,null),
  (9002,7001,2,1,'Knife',1,1790000002,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":7.5}]}',3,null),
  (9003,7003,40,5,'WindTrapFilter4',0,1790000003,'{}',0,2.5),
  (9005,7005,10,0,'HeavyAmmo',0,1790000005,'{}',0,null),
  (9050,7050,9,0,'CopperOre',0,1790000050,'{}',0,null);
insert into buildings(id,owner_id) values (602,null),(802,null),(812,null),(902,null);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,owner_entity_id,building_flags,health,shelter,sand_buildup,last_placed_by_player_id) values
  (602,1,'Sardaukar_Foundation',5000,6000,7,0,0,0,1,5764607523034234881,0,100.0,0,0,1),
  (602,2,'Sardaukar_Wall_01',5010,6000,7,0,0,0,1,5764607523034234881,0,90.5,1,2,1),
  (602,3,'Sardaukar_Wall_01',5020,6000,10,0,0,0,1,5764607523034234881,0,80.0,1,0,1),
  (602,4,'Sardaukar_Door_Frame_Tall_L',5020,6000,10,0,0,0,1,5764607523034234881,0,80.0,1,0,1),
  (802,1,'Sardaukar_Foundation',90000,90000,0,0,0,0,1,5764607523034235001,0,100.0,0,0,1),
  (812,1,'Sardaukar_Foundation',80000,80000,0,0,0,0,1,5764607523034235101,0,100.0,0,0,1),
  (902,1,'Sardaukar_Foundation',70000,70000,0,0,0,0,1,5764607523034235201,0,100.0,0,0,1);
insert into placeables(id,owner_entity_id,health,building_type,has_hit_ground,has_buildable_support,is_hologram,last_placed_by_player_id) values
  (601,5764607523034234881,1000.0,'Totem_Placeable',1,1,0,1),(701,5764607523034234881,50.0,'SpiceSilo_Placeable',1,1,0,1),
  (702,5764607523034234881,60.0,'MTX_Sardaukar_TallDoor_Placeable_L',1,1,0,1),(703,5764607523034234881,60.0,'Developer_StorageContainer_Placeable',1,1,0,1),
  (704,5764607523034234881,60.0,'WindTurbineDirectional_Placeable',1,1,1,1),(705,5764607523034234881,60.0,'Fabricator_Placeable',1,1,0,1),
  (706,5764607523034234881,60.0,'LargeWaterCistern_Placeable',1,0,0,1),
  (801,5764607523034235001,1000.0,'Totem_Placeable',1,1,0,1),(811,5764607523034235101,1000.0,'Totem_Placeable',1,1,0,1),(901,5764607523034235201,1000.0,'Totem_Placeable',1,1,0,1);
insert into totems(id,landclaim_vertical_level,last_backup_timestamp,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values
  (601,0,0,5000,6000,7,90),(801,0,0,90000,90000,0,0),(811,0,0,80000,80000,0,0),(901,0,0,70000,70000,0,0);
insert into totem_fuel_update_times(totem_id,last_fuel_update_farm_uptime,last_fuel_update_universe_time) values (601,5,6),(801,5,6);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,0,0),(601,1,0),(801,7,7);
insert into vehicles(id) values (950);
insert into player_respawn_locations(id,character_id,"group",locator_actor_id,map,dimension) values
  (x'01',1,'BaseTotem',601,'HaggaBasin',0),(x'02',1,'BaseTotem',801,'HaggaBasin',0),(x'03',1,'Vehicle',950,'HaggaBasin',0),(x'04',1,'Checkpoint',null,'HaggaBasin',0);
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (3,1,'##Totem_Placeable',1);
insert into base_backup_linked_actors(id,actor_id) values (3,901),(3,902);
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
  (111,0,'HaggaBasin','HomeBase',5000,6000,7),(222,0,'HaggaBasin','HomeBase',90000,90000,0),(333,0,'HaggaBasin','Cave',5000,6000,7);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,111,0,'HaggaBasin',3,10),(1,222,0,'HaggaBasin',3,10),(1,333,0,'HaggaBasin',3,10);`

func recycleOps(t *testing.T, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, recycleWorld+extra)}
}

func recycle(t *testing.T, o *Ops, totem int64) map[string]any {
	t.Helper()
	pv, err := o.PreviewBaseRecycle(totem)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	fp, _ := pv.(map[string]any)["fingerprint"].(string)
	if fp == "" {
		t.Fatal("the preview gave no fingerprint")
	}
	r, err := o.RecycleBase(totem, fp)
	if err != nil {
		t.Fatalf("recycle: %v", err)
	}
	return r.(map[string]any)
}

// refundOf is the refund as "item count/stacks" words, in the order of the answer.
func refundOf(r map[string]any) string {
	var out []string
	for _, l := range r["refund"].([]RefundLine) {
		out = append(out, fmt.Sprintf("%s %d/%d", l.Item, l.Count, l.Stacks))
	}
	return strings.Join(out, ", ")
}

// everything is what a refused or rolled back edit must leave exactly as it was: the rows of every table the recycle touches.
func everything(t *testing.T, o *Ops) string {
	t.Helper()
	var b strings.Builder
	for _, q := range []string{
		`select id, state, serial from actors order by id`, `select actor_id, entity_id, slot_name from actor_fgl_entities order by 1,2`, `select entity_id, json(components) c from fgl_entities order by 1`,
		`select id, actor_id, inventory_type from inventories order by id`, `select inventory_id from actor_inventories order by 1`,
		`select id, inventory_id, stack_size, position_index, template_id, is_new, acquisition_time, stats, quality_level, volume_override from items order by id`,
		`select building_id, instance_id, building_type from building_instances order by 1,2`, `select id from buildings order by 1`, `select id, building_type from placeables order by 1`,
		`select id from totems order by 1`, `select totem_id from totem_fuel_update_times order by 1`, `select totem_id, grid_location_x from landclaim_segments order by 1,2`,
		`select actor_id from permission_actor order by 1`, `select permission_actor_id, player_id, rank from permission_actor_rank order by 1,2`,
		`select marker_hash_id from ` + o.S.PlayerMarkersTable() + ` pm order by 1`, `select id from base_backups order by 1`, `select id from vehicles order by 1`,
		`select hex(id) id, locator_actor_id from player_respawn_locations order by 1`, `select id, hex(pending_respawn_location_id) p from player_state order by 1`,
	} {
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		fmt.Fprintln(&b, rows)
	}
	return b.String()
}

func TestRecycleDeletesTheBaseAndReturnsEverythingToTheReserve(t *testing.T) {
	inEachLayout(t, recycleWorld, func(t *testing.T, o *Ops) {
		before := everything(t, o)
		pv, err := o.PreviewBaseRecycle(601)
		if err != nil {
			t.Fatal(err)
		}
		p := pv.(map[string]any)
		if p["dryRun"] != true || p["name"] != "My base" || p["pieces"] != int64(4) || p["placeables"] != int64(6) || p["itemStacksMoved"] != int64(4) || p["itemsMoved"] != int64(352) ||
			p["holograms"] != int64(1) || p["devStorage"] != int64(1) || p["waterDropped"] != int64(500) || p["costPercent"] != int64(100) || p["destination"] != "Reserve" {
			t.Fatalf("preview: %v", p)
		}
		// the refund: 100% of the table cost. Stone: foundation 15, two walls 20, the frame 7, the door 10 once. ScrapMetal: totem 40, silo 35, fabricator 75. The cistern's three
		// materials. IronBar: 10 for the Developer storage container, not its table cost of 20. Nothing for the hologram turbine.
		if got, want := refundOf(p), "DuraluminumRod 115/1, IronBar 10/1, ScrapMetal 150/1, Silicone 120/1, Stone 52/1, T4HarkSpiceSiloComponent1 20/1"; got != want {
			t.Fatalf("refund: %s, want %s", got, want)
		}
		// a preview writes nothing
		if everything(t, o) != before || len(o.S.Ops()) != 0 {
			t.Fatal("the preview changed the save")
		}

		r := recycle(t, o, 601)
		// the structure of the deletions: the pattern of the game's own recycle (and the land claim segments a standing base still has)
		want := map[string]int64{"actors": 8, "actor_fgl_entities": 7, "fgl_entities": 7, "inventories": 5, "actor_inventories": 5, "building_instances": 4, "buildings": 1, "placeables": 7,
			"totems": 1, "totem_fuel_update_times": 1, "landclaim_segments": 2, "permission_actor": 3, "permission_actor_rank": 2, "player_markers": 1, "player_respawn_locations": 1}
		if got := r["deleted"].(map[string]int64); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("deleted rows: %v, want %v", got, want)
		}
		for _, q := range []string{
			`actors where id in (601,602,701,702,703,704,705,706)`, `actor_fgl_entities where actor_id in (601,602,701,702,703,704,705,706)`,
			`fgl_entities where entity_id between 5764607523034234881 and 5764607523034234890`, `inventories where id in (7000,7001,7003,7005,7006)`,
			`actor_inventories where inventory_id in (7000,7001,7003,7005,7006)`, `building_instances where building_id=602`, `buildings where id=602`,
			`placeables where id between 601 and 706`, `totems where id=601`, `totem_fuel_update_times where totem_id=601`, `landclaim_segments where totem_id=601`,
			`permission_actor where actor_id in (601,701,702)`, `permission_actor_rank where permission_actor_id in (601,701)`,
		} {
			if n := rmCount(t, o, q); n != 0 {
				t.Fatalf("%d rows left in %s", n, q)
			}
		}
		// the respawn point at the base goes with it (the save's own layout deletes it with the totem's actor); the points at another base, at the vehicle and at no actor stay
		rp, _ := o.S.Query(`select hex(id) id from player_respawn_locations order by 1`)
		if fmt.Sprint(rp) != "[map[id:02] map[id:03] map[id:04]]" || r["respawnPoints"] != int64(1) {
			t.Fatalf("respawn points: %v (the answer says %v)", rp, r["respawnPoints"])
		}
		// no base backup is written, and the one that was there stays
		if rmCount(t, o, `base_backups`) != 1 || rmCount(t, o, `base_backup_linked_actors`) != 2 {
			t.Fatal("a recycle writes no base backup row")
		}
		// the base's marker is gone; another base's marker and a marker of another kind at the same place stay; the markers table itself is not changed
		pm, _ := o.S.Query(`select marker_hash_id h from ` + o.S.PlayerMarkersTable() + ` pm order by 1`)
		if fmt.Sprint(pm) != "[map[h:222] map[h:333]]" || rmCount(t, o, o.S.MarkersTable()+` mk`) != 3 || r["markerRemoved"] != true {
			t.Fatalf("markers: %v", pm)
		}
		// the Reserve receives the stored items unchanged: only the inventory and the position of a row change (the Reserve is empty before a recycle, #1202)
		moved, _ := o.S.Query(`select id, inventory_id inv, position_index pos, stack_size n, template_id t, is_new new, acquisition_time at, stats, quality_level q, volume_override v from items where id in (9001,9002,9003,9005) order by id`)
		wantMoved := "[map[at:1790000001 id:9001 inv:12 n:300 new:0 pos:0 q:0 stats:{\"a\":1} t:CopperOre v:<nil>] " +
			"map[at:1790000002 id:9002 inv:12 n:2 new:1 pos:1 q:3 stats:{\"FItemStackAndDurabilityStats\":[[],{\"CurrentDurability\":7.5}]} t:Knife v:<nil>] " +
			"map[at:1790000003 id:9003 inv:12 n:40 new:0 pos:2 q:0 stats:{} t:WindTrapFilter4 v:2.5] " +
			"map[at:1790000005 id:9005 inv:12 n:10 new:0 pos:3 q:0 stats:{} t:HeavyAmmo v:<nil>]]"
		if fmt.Sprint(moved) != wantMoved {
			t.Fatalf("moved items:\n%v\nwant\n%s", moved, wantMoved)
		}
		// the refund: new rows in the Reserve, each in a free position
		ref, _ := o.S.Query(`select template_id t, sum(stack_size) n, count(*) rows from items where inventory_id=12 and id>9050 group by 1 order by 1`)
		if fmt.Sprint(ref) != "[map[n:115 rows:1 t:DuraluminumRod] map[n:10 rows:1 t:IronBar] map[n:150 rows:1 t:ScrapMetal] map[n:120 rows:1 t:Silicone] map[n:52 rows:1 t:Stone] map[n:20 rows:1 t:T4HarkSpiceSiloComponent1]]" {
			t.Fatalf("refund rows: %v", ref)
		}
		if rmCount(t, o, `items where inventory_id=12`) != 10 || rmCount(t, o, `(select distinct position_index from items where inventory_id=12)`) != 10 {
			t.Fatal("every stack of the Reserve has its own position")
		}
		// vehicles are untouched, with what they hold; so are the other bases
		if rmCount(t, o, `actors where id=950 and state=0 and serial=4`) != 1 || rmCount(t, o, `vehicles where id=950`) != 1 || rmCount(t, o, `items where id=9050 and inventory_id=7050 and stack_size=9 and position_index=0`) != 1 ||
			rmCount(t, o, `inventories where id=7050`) != 1 || rmCount(t, o, `permission_actor_rank where permission_actor_id=950`) != 1 || rmCount(t, o, `actor_fgl_entities where actor_id=950`) != 1 {
			t.Fatal("the vehicle was touched")
		}
		if rmCount(t, o, `actors where id in (801,802,811,812) and state=0`) != 4 || rmCount(t, o, `actors where id in (901,902) and state=5`) != 2 || rmCount(t, o, `permission_actor_rank where permission_actor_id in (801,811)`) != 3 ||
			rmCount(t, o, `landclaim_segments`) != 1 || rmCount(t, o, `totem_fuel_update_times`) != 1 || rmCount(t, o, `building_instances`) != 3 {
			t.Fatal("another base was changed")
		}
		// the answer says it in words
		words := strings.Join(r["summary"].([]string), "\n")
		for _, w := range []string{`Deleted: the base "My base" with 4 building pieces, its totem and 6 placeables (8 actors`, "Moved: 4 item stacks (352 items)", "Refund (100% of the building cost, as 6 new stacks)",
			"52 Granite Stone", "150 Salvaged Metal", "10 Iron Ingot", "Deleted with the base: 1 respawn points at it", "1 holograms give nothing back", "1 Developer storage containers give 10 Iron Ingot each", "Dropped: 500 water stored in 1 devices",
			"Everything returned is in the character's Reserve", "No base backup was written", "vehicles were not touched"} {
			if !strings.Contains(words, w) {
				t.Fatalf("the answer does not say %q:\n%s", w, words)
			}
		}
		if !strings.Contains(fmt.Sprint(r["warnings"]), "1 placeables of this base have no buildable support") {
			t.Fatalf("warnings: %v", r["warnings"])
		}
		// one edit, with a description; and the base is gone for the next question
		if n := o.S.Ops(); len(n) != 1 || !strings.HasPrefix(n[0].Desc, `Recycle base "My base": 4 pieces, 6 placeables, 4 item stacks`) {
			t.Fatalf("one edit with a description: %v", n)
		}
		_, err = o.PreviewBaseRecycle(601)
		refusedWith(t, err, apierr.ErrNotFound, "no base with the totem")
	})
}

func TestRecycleInTheDeepDesertGivesBackTheFullTableCost(t *testing.T) {
	// the same base, on a map where the game's table lets a piece cost half (the Deep Desert) and on one where it costs the full table cost: the refund is the same (#1211). The
	// odd costs (foundation 15, frame 7, silo 35) are the ones a half would have rounded.
	const want = "DuraluminumRod 115/1, IronBar 10/1, ScrapMetal 150/1, Silicone 120/1, Stone 52/1, T4HarkSpiceSiloComponent1 20/1"
	outside := recycle(t, recycleOps(t, ``), 601)
	if got := refundOf(outside); got != want {
		t.Fatalf("refund outside the Deep Desert: %s, want %s", got, want)
	}
	o := recycleOps(t, `update actors set map='DeepDesert' where id in (601,602,701,702,703,704,705,706);
update markers set map_name='DeepDesert' where marker_hash_id=111; update player_markers set map_name='DeepDesert' where marker_hash_id=111;`)
	r := recycle(t, o, 601)
	if got := refundOf(r); got != want || got != refundOf(outside) {
		t.Fatalf("Deep Desert refund: %s, want %s (the same as outside)", got, want)
	}
	words := strings.Join(r["summary"].([]string), "\n")
	if r["costPercent"] != int64(100) || strings.Contains(words, "which is") || !strings.Contains(words, "Refund (100% of the building cost,") || r["markerRemoved"] != true {
		t.Fatalf("%v", r)
	}
	// no base backup: the Deep Desert never has one
	if rmCount(t, o, `base_backups`) != 1 {
		t.Fatal("a base backup row was written")
	}
}

func TestTheRefundIsWrittenAsStacksOfTheItemsOwnSize(t *testing.T) {
	// 120 more foundations: Stone 15 x 121 + 20 + 7 + 10 = 1,852, which is three stacks of 500 and one of 352
	var b strings.Builder
	b.WriteString(`insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values `)
	for i := 0; i < 120; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "(602,%d,'Sardaukar_Foundation',5764607523034234881,1.0)", 100+i)
	}
	o := recycleOps(t, b.String()+";")
	r := recycle(t, o, 601)
	if !strings.Contains(refundOf(r), "Stone 1852/4") {
		t.Fatalf("refund: %s", refundOf(r))
	}
	rows, _ := o.S.Query(`select stack_size n from items where inventory_id=12 and template_id='Stone' order by id`)
	if fmt.Sprint(rows) != "[map[n:500] map[n:500] map[n:500] map[n:352]]" {
		t.Fatalf("Stone stacks: %v", rows)
	}
}

func TestATypeWithNoCostIsARefusalThatNamesItNeverAFreePiece(t *testing.T) {
	o := recycleOps(t, `insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,9,'Unheard_Of_Wall',5764607523034234881,1.0);
update placeables set building_type='Unheard_Of_Lamp_Placeable' where id=705;`)
	before := everything(t, o)
	_, err := o.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "The building cost of Unheard_Of_Lamp_Placeable, Unheard_Of_Wall is not known")
	_, err = o.RecycleBase(601, "anything")
	refusedWith(t, err, apierr.ErrConflict, "is not known")
	if everything(t, o) != before || len(o.S.Ops()) != 0 {
		t.Fatal("a refusal must change nothing")
	}
	// a hologram of a type with no cost is not a gap: it gives nothing either way
	o2 := recycleOps(t, `update placeables set building_type='Unheard_Of_Lamp_Placeable' where id=704;`)
	if r := recycle(t, o2, 601); r["holograms"] != int64(1) {
		t.Fatalf("%v", r)
	}
}

func TestARunningCraftingQueueIsRefused(t *testing.T) {
	for name, comp := range map[string]string{
		"running":    `{"FItemCraftingComponent":[0,{"RequestsQueue":[{"RequestId":4}],"State":"Crafting"}]}`,
		"unreadable": `{"FItemCraftingComponent":[0,{"State":"Idle"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			o := recycleOps(t, `update fgl_entities set components=jsonb('`+comp+`') where entity_id=5764607523034234885;`)
			before := everything(t, o)
			_, err := o.PreviewBaseRecycle(601)
			refusedWith(t, err, apierr.ErrConflict, "A crafting queue is running in this base (1 stations)")
			if everything(t, o) != before {
				t.Fatal("a refusal must change nothing")
			}
		})
	}
}

func TestOnlyTheCharactersOwnStandingBasesCanBeRecycled(t *testing.T) {
	o := recycleOps(t, "")
	before := everything(t, o)
	for _, c := range []struct {
		totem int64
		code  error
		want  string
	}{
		{801, apierr.ErrConflict, "belongs to another player"},
		{811, apierr.ErrConflict, "belongs to another player"},
		{12345, apierr.ErrNotFound, "no base with the totem"},
		{950, apierr.ErrNotFound, "no base with the totem"}, // a vehicle is not a base
	} {
		_, err := o.PreviewBaseRecycle(c.totem)
		refusedWith(t, err, c.code, c.want)
		_, err = o.RecycleBase(c.totem, "anything")
		refusedWith(t, err, c.code, c.want)
	}
	if everything(t, o) != before || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
		t.Fatal("a refusal must change nothing and queue nothing")
	}
	if _, err := o.RecycleBase(601, ""); err == nil || !strings.Contains(err.Error(), "fingerprint is missing") {
		t.Fatalf("a write with no fingerprint: %v", err)
	}
	if _, err := o.PreviewBaseRecycle(0); err == nil {
		t.Fatal("base 0 is not a base")
	}
	// a member (a rank below owner) of a base nobody else owns
	o2 := recycleOps(t, `delete from permission_actor_rank where permission_actor_id=811 and player_id=99;`)
	_, err := o2.PreviewBaseRecycle(811)
	refusedWith(t, err, apierr.ErrConflict, "only a member")
	// a base nobody holds the owner rank over
	o3 := recycleOps(t, `delete from permission_actor_rank where permission_actor_id=601;`)
	_, err = o3.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "Nobody holds the owner rank")
	// a base in another state, a base with an actor in another state, a base an actor of which is a vehicle
	o4 := recycleOps(t, `update actors set state=2 where id=601;`)
	_, err = o4.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "not in the normal state (state 2)")
	o5 := recycleOps(t, `update actors set state=5 where id=701;`)
	_, err = o5.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "1 actors of this base are not in the normal state")
	o6 := recycleOps(t, `insert into vehicles(id) values (705);`)
	_, err = o6.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "Vehicles are never recycled")
	// a save in which the character has no Reserve
	o7 := recycleOps(t, `delete from items where inventory_id=12; delete from inventories where id=12;`)
	_, err = o7.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "0 Reserve inventories")
	// a building actor that also holds another base's pieces
	o8 := recycleOps(t, `insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,9,'Sardaukar_Wall_01',5764607523034235001,1.0);`)
	_, err = o8.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "also holds 1 pieces of another base")
}

// Every reference the save's layout declares to a row the recycle deletes is either handled by it or a refusal that names the table: nothing is left dangling, and nothing is
// found out only when the save layer refuses the finished edit.
func TestARowOfAnotherTableThatPointsAtTheBaseIsARefusalThatNamesIt(t *testing.T) {
	for name, c := range map[string]struct{ sql, want string }{
		"a game event of a placeable": {`insert into game_events(actor_id,universe_time,map,partition_id,event_type,x,y,z) values (701,1,'HaggaBasin',0,3,0,0,0),(701,2,'HaggaBasin',0,3,0,0,0);`, "game_events.actor_id (2 rows)"},
		"a spawner's actor":           {`insert into actor_spawners(id,map,name,dimension_index) values (5,'HaggaBasin','s',0); insert into actor_spawner_actors(spawner_id,actor_id) values (5,705);`, "actor_spawner_actors.actor_id (1 rows)"},
		"a travelling actor":          {`insert into travel_actor_parent(id,parent_id,is_instigator) values (950,601,0);`, "travel_actor_parent.parent_id (1 rows)"},
		"an actor that travels":       {`insert into travel_actor_parent(id,parent_id,is_instigator) values (702,2,0);`, "travel_actor_parent.id (1 rows)"},
		"a rank held by an actor":     {`insert into permission_actor_rank(permission_actor_id,player_id,rank) values (801,704,3);`, "permission_actor_rank.player_id (1 rows)"},
		"a vehicle module inventory":  {`insert into vehicle_module_inventories(inventory_id,vehicle_module_inventory_type) values (7003,1);`, "vehicle_module_inventories.inventory_id (1 rows)"},
	} {
		t.Run(name, func(t *testing.T) {
			o := recycleOps(t, c.sql)
			before := everything(t, o)
			_, err := o.PreviewBaseRecycle(601)
			refusedWith(t, err, apierr.ErrConflict, "Rows of other tables of the save point at this base")
			if !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "Nothing was changed") {
				t.Fatalf("the refusal names the table and the rows (%s): %v", c.want, err)
			}
			_, err = o.RecycleBase(601, "anything")
			refusedWith(t, err, apierr.ErrConflict, c.want)
			if everything(t, o) != before || len(o.S.Ops()) != 0 {
				t.Fatal("a refusal must change nothing")
			}
		})
	}
	// the same rows at another base, or at the vehicle, are no reason to refuse
	o := recycleOps(t, `insert into game_events(actor_id,universe_time,map,partition_id,event_type,x,y,z) values (801,1,'HaggaBasin',0,3,0,0,0),(950,1,'HaggaBasin',0,3,0,0,0);`)
	if r := recycle(t, o, 601); r["respawnPoints"] != int64(1) || rmCount(t, o, `game_events`) != 2 {
		t.Fatalf("%v", r)
	}
}

// Every reference to actors, inventories and entities that the real layout declares is either one the recycle handles or one the plan looks up: the list of handled ones names
// only references that exist, so a column the game renames is noticed here.
func TestTheHandledReferencesAreReferencesOfTheSavesLayout(t *testing.T) {
	inEachLayout(t, recycleWorld, func(t *testing.T, o *Ops) {
		fks, err := o.S.Query(`select m.name tbl, f."from" col, f."table" ref from sqlite_master m join pragma_foreign_key_list(m.name) f where m.type='table'`)
		if err != nil {
			t.Fatal(err)
		}
		declared := map[string]bool{}
		for _, f := range fks {
			declared[fmt.Sprintf("%v.%v>%v", f["tbl"], f["col"], f["ref"])] = true
		}
		for _, refs := range []map[string]bool{recycleHandledRefs, recyclePickedUpRefs} {
			for k := range refs {
				if !declared[k] {
					t.Errorf("%s is handled by the recycle and is not a reference of the save's layout", k)
				}
			}
		}
		if !declared["player_respawn_locations.locator_actor_id>actors"] || !declared["game_events.actor_id>actors"] || len(declared) < 60 {
			t.Fatalf("the layout's references were not read: %d", len(declared))
		}
	})
}

func TestABaseACharacterIsWaitingToRespawnAtIsRefused(t *testing.T) {
	o := recycleOps(t, `update player_state set pending_respawn_location_id=x'01' where id=1;`)
	before := everything(t, o)
	_, err := o.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "A character is waiting to respawn at this base")
	if everything(t, o) != before {
		t.Fatal("a refusal must change nothing")
	}
	// waiting to respawn somewhere else is no reason to refuse, and that pointer is not touched
	o2 := recycleOps(t, `update player_state set pending_respawn_location_id=x'04' where id=1;`)
	recycle(t, o2, 601)
	if rmCount(t, o2, `player_state where id=1 and hex(pending_respawn_location_id)='04'`) != 1 {
		t.Fatal("the pending respawn point of the character was changed")
	}
}

func TestTheRecycleIsRefusedWhenTheBaseChangedSinceTheQuestion(t *testing.T) {
	for name, change := range map[string]string{
		"a piece was added":      `insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,9,'Sardaukar_Wall_01',5764607523034234881,1.0)`,
		"a stack was taken out":  `delete from items where id=9002`,
		"a stack changed size":   `update items set stack_size=299 where id=9001`,
		"the cistern was filled": `update fgl_entities set components=jsonb('{"FWaterStorageComponent":[1,{"m_WaterStored":900}]}') where entity_id=5764607523034234886`,
	} {
		t.Run(name, func(t *testing.T) {
			o := recycleOps(t, "")
			pv, err := o.PreviewBaseRecycle(601)
			if err != nil {
				t.Fatal(err)
			}
			testsave.Exec(t, o.S, change)
			before := everything(t, o)
			_, err = o.RecycleBase(601, pv.(map[string]any)["fingerprint"].(string))
			refusedWith(t, err, apierr.ErrConflict, "changed since the question")
			if everything(t, o) != before {
				t.Fatal("a refused write changed the save")
			}
		})
	}
}

func TestAReadBackThatDiffersFromThePlanFailsTheWholeEdit(t *testing.T) {
	for name, tamper := range map[string]string{
		"a moved stack lost items":    `update items set stack_size=1 where id=9001`,
		"a moved stack is elsewhere":  `update items set inventory_id=1, position_index=40 where id=9003`,
		"a refund stack is short":     `update items set stack_size=stack_size-1 where inventory_id=12 and template_id='Stone'`,
		"a row of the base is back":   `insert into totems(id) values (601)`,
		"two stacks share a position": `update items set position_index=1 where id=9005`,
	} {
		t.Run(name, func(t *testing.T) {
			o := recycleOps(t, "")
			pv, err := o.PreviewBaseRecycle(601)
			if err != nil {
				t.Fatal(err)
			}
			before := everything(t, o)
			recycleBeforeReadBack = func(m *save.Mut) {
				if _, err := m.Exec(tamper); err != nil {
					t.Errorf("tamper: %v", err)
				}
			}
			defer func() { recycleBeforeReadBack = nil }()
			_, err = o.RecycleBase(601, pv.(map[string]any)["fingerprint"].(string))
			refusedWith(t, err, apierr.ErrConflict, "The recycle was not written")
			if !strings.Contains(err.Error(), "Nothing was changed") {
				t.Fatalf("the refusal says nothing was changed: %v", err)
			}
			if everything(t, o) != before || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
				t.Fatal("a failed read back must roll the whole edit back")
			}
		})
	}
}

// The preview runs the save layer's reference check before it rolls back: an edit the write would be refused for is refused when the question is asked, not after the answer.
func TestThePreviewRefusesAnEditThatWouldLeaveADanglingReference(t *testing.T) {
	o := recycleOps(t, "")
	before := everything(t, o)
	// something the plan does not know about: a row that points at the totem appears while the edit runs
	recycleBeforeReadBack = func(m *save.Mut) {
		if _, err := m.Exec(`insert into game_events(actor_id,universe_time,map,partition_id,event_type,x,y,z) values (601,1,'HaggaBasin',0,3,0,0,0)`); err != nil {
			t.Errorf("tamper: %v", err)
		}
	}
	defer func() { recycleBeforeReadBack = nil }()
	_, err := o.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "would leave dangling references")
	if !strings.Contains(err.Error(), "game_events") || !strings.Contains(err.Error(), "This base cannot be recycled by this version") {
		t.Fatalf("the refusal names the table: %v", err)
	}
	if everything(t, o) != before || len(o.S.Ops()) != 0 || rmCount(t, o, `game_events`) != 0 {
		t.Fatal("a refused preview must change nothing")
	}
}

// A Reserve with a slot limit (none was seen in a real save, where it is -1) that has no room for everything is a refusal, never a partial return.
func TestAReserveWithNoRoomForEverythingIsRefused(t *testing.T) {
	// the base returns 4 stored stacks and 6 refund stacks; the Reserve is empty (a recycle needs that, #1202)
	o := recycleOps(t, `update inventories set max_item_count=9 where id=12;`)
	before := everything(t, o)
	_, err := o.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "The Reserve of this save has room for 9 stacks and the base returns 10")
	if everything(t, o) != before || len(o.S.Ops()) != 0 {
		t.Fatal("a refusal must change nothing")
	}
	// exactly enough room is enough
	o2 := recycleOps(t, `update inventories set max_item_count=10 where id=12;`)
	if r := recycle(t, o2, 601); r["itemStacksMoved"] != int64(4) || r["refundStacks"] != int64(6) || rmCount(t, o2, `items where inventory_id=12`) != 10 {
		t.Fatalf("%v", r)
	}
}

// An item that has its own inventory (a container item) lying in a chest of the base moves to the Reserve with its inventory and what is in it: the inventory hangs from the
// item, not from an actor of the base, so it is neither deleted nor emptied.
func TestAContainerItemInAChestKeepsItsOwnInventoryAndContents(t *testing.T) {
	o := recycleOps(t, `insert into inventories(id,item_id,inventory_type,max_item_count,max_item_volume) values (7100,9002,7,4,10.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (9100,7100,5,0,'CopperOre',0,1790000100,'{}',0,null);`)
	r := recycle(t, o, 601)
	if r["itemStacksMoved"] != int64(4) || r["itemsMoved"] != int64(352) {
		t.Fatalf("only the rows of the base's own inventories are counted as moved: %v", r)
	}
	if rmCount(t, o, `items where id=9002 and inventory_id=12`) != 1 || rmCount(t, o, `inventories where id=7100 and item_id=9002 and actor_id is null`) != 1 ||
		rmCount(t, o, `items where id=9100 and inventory_id=7100 and stack_size=5 and position_index=0`) != 1 {
		t.Fatal("the container item, its inventory or what it holds did not come through unchanged")
	}
}

func TestRecycleIsOneQueuedEditAndARefusedBaseQueuesNothing(t *testing.T) {
	o := recycleOps(t, "")
	recycle(t, o, 601)
	p := o.S.Pending()
	if len(p) != 1 || !strings.HasPrefix(p[0], `Recycle base "My base"`) {
		t.Fatalf("pending: %v", p)
	}
	if _, err := o.RecycleBase(801, "x"); err == nil || len(o.S.Pending()) != 1 {
		t.Fatalf("a refused base must queue nothing: %v", o.S.Pending())
	}
}

func TestABaseOfATotemAloneAndAMarkerThatIsNotClearlyItsOwn(t *testing.T) {
	// no pieces, no placeables but the totem, nothing stored: the totem's cost comes back and nothing else
	o := recycleOps(t, `delete from building_instances where building_id=602; delete from placeables where id between 701 and 706; delete from items where inventory_id in (7001,7003,7005);
delete from player_markers where marker_hash_id=111;`)
	r := recycle(t, o, 601)
	if refundOf(r) != "ScrapMetal 40/1" || r["pieces"] != int64(0) || r["itemStacksMoved"] != int64(0) || r["markerRemoved"] != false || !strings.Contains(fmt.Sprint(r["warnings"]), "No map marker of this base") {
		t.Fatalf("%v", r)
	}
	if rmCount(t, o, `player_markers`) != 2 || rmCount(t, o, `actors where id=601`) != 0 {
		t.Fatal("the totem goes, the other markers stay")
	}
	// the building actor and the placeables that belonged to nobody after the edit above are not this base's: they stay
	if rmCount(t, o, `actors where id in (602,701,702,703,704,705,706)`) != 7 {
		t.Fatal("actors that are not the base's were deleted")
	}
}

// The game allows one Reserve, so Recycle base refuses while it holds any stack (#1202), in the question and in the write, with the words of the layout roll and a pointer to the move
// into storage. An empty Reserve behaves as it always did, and so does the Solido option.
const reserveStacks = `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values
  (8000,12,7,1,'CopperOre',0,1790000000,'{}',0,null),(8001,12,3,2,'Stone',0,1790000000,'{}',0,null),(8002,12,1,3,'Knife',0,1790000000,'{}',0,null);`

func TestARecycleIsRefusedWhileTheReserveHoldsAStack(t *testing.T) {
	for name, c := range map[string]struct {
		extra string
		n     int
	}{
		"one stack":      {reserveStacks[:strings.Index(reserveStacks, ",(8001")] + ";", 1},
		"several stacks": {reserveStacks, 3},
	} {
		t.Run(name, func(t *testing.T) {
			for _, opt := range []RecycleOptions{{RequireEmptyReserve: true}, {Solido: true, RequireEmptyReserve: true}} {
				o := recycleOps(t, c.extra)
				before := everything(t, o)
				want := fmt.Sprintf("The Reserve is not empty (it holds %d stack(s)) and no recycle can happen until it is empty", c.n)
				_, err := o.PreviewBaseRecycleWith(601, opt)
				refusedWith(t, err, apierr.ErrConflict, want)
				if !strings.Contains(err.Error(), "Move the Reserve into storage") || !strings.Contains(err.Error(), "Nothing was changed") || !strings.Contains(err.Error(), ReserveNotEmptyWhy(int64(c.n))[1:]) {
					t.Fatalf("the refusal points at the move into storage: %v", err)
				}
				_, err = o.RecycleBaseWith(601, "any", opt)
				refusedWith(t, err, apierr.ErrConflict, want)
				if everything(t, o) != before || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
					t.Fatal("a refusal must change nothing")
				}
			}
		})
	}
}

func TestARecycleIsAllowedWhenTheReserveIsEmptyAndTheSolidoIsUnaffected(t *testing.T) {
	o := recycleOps(t, "")
	if r := recycle(t, o, 601); r["ok"] != true || r["itemStacksMoved"] != int64(4) || r["refundStacks"] != int64(6) {
		t.Fatalf("%v", r)
	}
	// the rule counts stacks in the Reserve only: a stack in the backpack or in a chest is not in the way
	o2 := recycleOps(t, `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8100,1,5,30,'Stone',0,1790000000,'{}',0,null);`)
	if r := recycle(t, o2, 601); r["ok"] != true {
		t.Fatalf("%v", r)
	}
}

// The game may save a change to the Reserve between the question and the write: the write plans again inside its own transaction and refuses, whatever the question saw.
func TestARecycleIsRefusedAtWriteTimeWhenTheQuestionSawAnEmptyReserve(t *testing.T) {
	o := recycleOps(t, "")
	pv, err := o.PreviewBaseRecycle(601)
	if err != nil {
		t.Fatal(err)
	}
	testsave.Exec(t, o.S, reserveStacks)
	before, pending := everything(t, o), len(o.S.Pending())
	_, err = o.RecycleBase(601, pv.(map[string]any)["fingerprint"].(string))
	refusedWith(t, err, apierr.ErrConflict, "The Reserve is not empty (it holds 3 stack(s)) and no recycle can happen until it is empty")
	if everything(t, o) != before || len(o.S.Pending()) != pending {
		t.Fatal("a refused write changed the save")
	}
	// emptied again, the same question's answer is good: the refusal was about the Reserve, not about the base
	testsave.Exec(t, o.S, `delete from items where inventory_id=12;`)
	if _, err := o.RecycleBase(601, pv.(map[string]any)["fingerprint"].(string)); err != nil {
		t.Fatalf("an emptied Reserve recycles with the same question: %v", err)
	}
}

// The layout roll does not set the option: it asks about its bases before it moves the Reserve into the boxes and keeps its own rule (internal/layoutroll), so its question is not refused here.
func TestTheLayoutRollsQuestionWithoutTheOptionIsNotRefusedForAReserveThatHoldsStacks(t *testing.T) {
	o := recycleOps(t, reserveStacks)
	if _, err := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true}); err != nil {
		t.Fatal(err)
	}
}
