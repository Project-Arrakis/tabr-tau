package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Back up and remove a base the way the game does (#1127). The world is a standing base of the character's (a totem, a building actor with three pieces and an entity, a chest with
// an item, a placeable with no buildable support, a map marker), a base of another player, a base the character is only a member of, and a base the game already picked up.
const removeEnt = int64(5764607523034234881)

const removeWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (99,'/Game/BP_Other.BP_Other_C','HaggaBasin',0,0,0,0,0,0,1,0,0,1,1,jsonb('{}')),
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000.9,6000.4,7.7,0,0,0,1,0,0,10,1,jsonb('{"m_SelfUniqueID":"!!act#601"}')),
  (602,'/Game/BP_Building.BP_Building_C','HaggaBasin',5001,6001,8,0,0,0.5,0.5,0,0,22,1,jsonb('{"DamageableActorComponent":{"x":1}}')),
  (701,'/Game/BP_Chest.BP_Chest_C','HaggaBasin',5010,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (702,'/Game/BP_Lamp.BP_Lamp_C','HaggaBasin',5020,6000,7,0,0,0,1,0,0,6,1,jsonb('{}')),
  (801,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',90000,90000,0,0,0,0,1,0,0,7,1,jsonb('{}')),
  (802,'/Game/BP_Building.BP_Building_C','HaggaBasin',90000,90000,0,0,0,0,1,0,0,8,1,jsonb('{}')),
  (811,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',80000,80000,0,0,0,0,1,0,0,7,1,jsonb('{}')),
  (812,'/Game/BP_Building.BP_Building_C','HaggaBasin',80000,80000,0,0,0,0,1,0,0,8,1,jsonb('{}')),
  (901,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',70000,70000,0,0,0,0,1,0,5,3,1,jsonb('{}')),
  (902,'/Game/BP_Building.BP_Building_C','HaggaBasin',70000,70000,0,0,0,0,1,0,5,4,1,jsonb('{}'));
insert into fgl_entities(entity_id,components) values
  (5764607523034234881, jsonb('{"Actor":[[],{}]}')),(5764607523034234882, jsonb('{"ContainerInventory":[[],{}]}')),(5764607523034234890, jsonb('{"FHealthComponent":[0,{"h":1}]}')),
  (5764607523034235001, jsonb('{"Actor":[[],{}]}')),(5764607523034235101, jsonb('{"Actor":[[],{}]}')),(5764607523034235201, jsonb('{"Actor":[[],{}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values
  (601,5764607523034234881,'Actor'),(701,5764607523034234882,'ContainerInventory'),(602,5764607523034234890,'Actor'),(801,5764607523034235001,'Actor'),(811,5764607523034235101,'Actor'),(901,5764607523034235201,'Actor');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values
  (601,'My base',3,3,0,0),(701,'Ore Out',1,3,1,0),(801,'Other base',3,3,0,0),(811,'Member base',3,3,0,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(801,99,1),(811,99,1),(811,1,2);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (7001,701,4,20,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (9001,7001,3,0,'CopperOre',0,1790000000,'{}',0,null);
insert into actor_inventories(inventory_id,component_name_hash) values (7001,111);
insert into buildings(id,owner_id) values (602,null),(802,null),(812,null),(902,null);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,owner_entity_id,building_flags,health,shelter,sand_buildup,last_placed_by_player_id) values
  (602,1,'Foundation',5000,6000,7,0,0,0,1,5764607523034234881,0,100.0,0,0,1),
  (602,2,'Wall',5010,6000,7,0,0,0.7071,0.7071,5764607523034234881,0,90.5,1,2,1),
  (602,3,'Roof',5020,6000,10,0,0,0,1,5764607523034234881,0,80.0,1,0,1),
  (802,1,'Foundation',90000,90000,0,0,0,0,1,5764607523034235001,0,100.0,0,0,1),
  (812,1,'Foundation',80000,80000,0,0,0,0,1,5764607523034235101,0,100.0,0,0,1),
  (902,1,'Foundation',70000,70000,0,0,0,0,1,5764607523034235201,0,100.0,0,0,1);
insert into placeables(id,owner_entity_id,health,building_type,has_hit_ground,has_buildable_support,is_hologram,last_placed_by_player_id) values
  (601,5764607523034234881,1000.0,'Totem_Placeable',1,1,0,1),(701,5764607523034234881,50.0,'GenericContainer_Placeable',1,1,0,1),(702,5764607523034234881,60.0,'Lamp_Placeable',1,0,0,1),
  (801,5764607523034235001,1000.0,'Totem_Placeable',1,1,0,1),(811,5764607523034235101,1000.0,'Totem_Placeable',1,1,0,1),(901,5764607523034235201,1000.0,'Totem_Placeable',1,1,0,1);
insert into totems(id,landclaim_vertical_level,last_backup_timestamp,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values
  (601,0,0,5000,6000,7,90),(801,0,0,90000,90000,0,0),(811,0,0,80000,80000,0,0),(901,0,0,70000,70000,0,0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,0,0),(601,1,0);
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (3,1,'##Totem_Placeable',1);
insert into base_backup_linked_actors(id,actor_id) values (3,901),(3,902);
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
  (111,0,'HaggaBasin','HomeBase',5000,6000,7),(222,0,'HaggaBasin','HomeBase',90000,90000,0),(333,0,'HaggaBasin','Cave',5000,6000,7);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,111,0,'HaggaBasin',3,10),(1,222,0,'HaggaBasin',3,10),(1,333,0,'HaggaBasin',3,10);`

func removeOps(t *testing.T, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, removeWorld+extra)}
}

func remove(t *testing.T, o *Ops, totem int64) map[string]any {
	t.Helper()
	pv, err := o.PreviewBaseRemoval(totem)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	fp, _ := pv.(map[string]any)["fingerprint"].(string)
	if fp == "" {
		t.Fatal("the preview gave no fingerprint")
	}
	r, err := o.BackUpAndRemoveBase(totem, fp)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	return r.(map[string]any)
}

func rmCount(t *testing.T, o *Ops, q string, args ...any) int64 {
	t.Helper()
	r, err := o.S.One(`select count(*) n from `+q, args...)
	if err != nil || r == nil {
		t.Fatalf("%s: %v", q, err)
	}
	return r["n"].(int64)
}

func refusedWith(t *testing.T, err error, code error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a refusal containing %q", want)
	}
	if !errors.Is(err, code) || !strings.Contains(err.Error(), want) {
		t.Fatalf("got %v, want %v containing %q", err, code, want)
	}
}

func TestBackUpAndRemoveWritesWhatTheGamesPickUpWrites(t *testing.T) {
	inEachLayout(t, removeWorld, func(t *testing.T, o *Ops) {
		pv, err := o.PreviewBaseRemoval(601)
		if err != nil {
			t.Fatal(err)
		}
		p := pv.(map[string]any)
		if p["dryRun"] != true || p["name"] != "My base" || p["pieces"] != int64(3) || p["placeables"] != int64(1) || p["leftStanding"] != int64(1) || p["storedItems"] != int64(3) {
			t.Fatalf("preview: %v", p)
		}
		// a preview writes nothing
		if rmCount(t, o, `actors where state=5`) != 2 || rmCount(t, o, `base_backups`) != 1 || len(o.S.Ops()) != 0 {
			t.Fatal("the preview changed the save")
		}
		r := remove(t, o, 601)
		if r["backupId"] != int64(4) || r["markerRemoved"] != true {
			t.Fatalf("result: %v", r)
		}
		newB := r["newBuildingActors"].([]int64)
		if len(newB) != 1 || newB[0] != 903 {
			t.Fatalf("the new building actor: %v", newB)
		}
		nb := newB[0]
		// the totem and the placeable: state 5 in place, serial one more; the lamp (no buildable support) is not taken
		got, _ := o.S.Query(`select id, state, serial from actors where id in (601,701,702) order by id`)
		if fmt.Sprint(got) != "[map[id:601 serial:11 state:5] map[id:701 serial:6 state:5] map[id:702 serial:6 state:0]]" {
			t.Fatalf("totem and placeables: %v", got)
		}
		// the building actor: gone, and made again with a new id, the same class, map, dimension and transform, state 5, serial 0, no properties, no entity
		if rmCount(t, o, `actors where id=602`) != 0 || rmCount(t, o, `buildings where id=602`) != 0 || rmCount(t, o, `actor_fgl_entities where actor_id=602`) != 0 || rmCount(t, o, `fgl_entities where entity_id=5764607523034234890`) != 0 {
			t.Fatal("the old building actor, its row or its entity is still there")
		}
		na, _ := o.S.One(`select class, map, location_x x, location_y y, location_z z, rotation_z rz, rotation_w rw, dimension_index d, state, serial, json(properties) props, json(gas_attributes) gas from actors where id=?`, nb)
		if na["class"] != "/Game/BP_Building.BP_Building_C" || na["map"] != "HaggaBasin" || na["x"] != 5001.0 || na["y"] != 6001.0 || na["z"] != 8.0 || na["rz"] != 0.5 || na["rw"] != 0.5 || na["d"] != int64(0) ||
			na["state"] != int64(5) || na["serial"] != int64(0) || na["props"] != "{}" || na["gas"] != "{}" {
			t.Fatalf("the new building actor: %v", na)
		}
		if rmCount(t, o, `actor_fgl_entities where actor_id=?`, nb) != 0 || rmCount(t, o, `buildings where id=? and owner_id is null`, nb) != 1 || rmCount(t, o, `building_instances where building_id=?`, nb) != 3 || rmCount(t, o, `building_instances where building_id=602`) != 0 {
			t.Fatal("the pieces and the buildings row did not move to the new building actor")
		}
		// the backup: the character's controller as player and last editor, the old permission name, one link for each actor
		bk, _ := o.S.One(`select player_id p, last_edited_by_player_id e, base_backup_name n from base_backups where id=4`)
		if bk["p"] != int64(1) || bk["e"] != int64(1) || bk["n"] != "My base" {
			t.Fatalf("backup row: %v", bk)
		}
		links, _ := o.S.Query(`select actor_id a from base_backup_linked_actors where id=4 order by 1`)
		if fmt.Sprint(links) != "[map[a:601] map[a:701] map[a:903]]" {
			t.Fatalf("links: %v", links)
		}
		// the claim is gone: no permission rows for the totem; the chest's own record stays
		if rmCount(t, o, `permission_actor where actor_id=601`) != 0 || rmCount(t, o, `permission_actor_rank where permission_actor_id=601`) != 0 || rmCount(t, o, `permission_actor where actor_id=701`) != 1 {
			t.Fatal("permission rows")
		}
		// the base's marker is gone; another base's marker and a marker of another kind at the same place stay
		pm, _ := o.S.Query(`select marker_hash_id h from ` + o.S.PlayerMarkersTable() + ` pm order by 1`)
		if fmt.Sprint(pm) != "[map[h:222] map[h:333]]" {
			t.Fatalf("player markers: %v", pm)
		}
		if rmCount(t, o, o.S.MarkersTable()+` mk`) != 3 {
			t.Fatal("the markers table itself is not changed")
		}
		// what is stored is not touched; nor are the totem row, the claim's segments, or the lamp
		if rmCount(t, o, `items where id=9001 and stack_size=3 and inventory_id=7001`) != 1 || rmCount(t, o, `inventories where id=7001 and actor_id=701`) != 1 || rmCount(t, o, `totems where id=601`) != 1 || rmCount(t, o, `landclaim_segments where totem_id=601`) != 2 || rmCount(t, o, `placeables where id=702`) != 1 {
			t.Fatal("something that is not part of the pick-up was changed")
		}
		// the other bases are as they were
		if rmCount(t, o, `actors where id in (801,802,811,812) and state=0`) != 4 || rmCount(t, o, `permission_actor_rank where permission_actor_id in (801,811)`) != 3 {
			t.Fatal("another base was changed")
		}
		// the Bases list says picked up
		rows, _ := o.baseListRows("x")
		for _, b := range rows {
			if b["base_id"] == int64(601) && b["picked_up"] != true {
				t.Fatalf("the Bases list does not say picked up: %v", b)
			}
		}
		if n := o.S.Ops(); len(n) != 1 || !strings.HasPrefix(n[0].Desc, `Back up and remove base "My base": 3 pieces, 1 placeables`) {
			t.Fatalf("one edit with a description: %v", n)
		}
		// asking again: it is picked up now
		_, err = o.PreviewBaseRemoval(601)
		refusedWith(t, err, apierr.ErrConflict, "already been picked up")
	})
}

func TestOnlyTheCharactersOwnStandingBasesCanBeBackedUp(t *testing.T) {
	o := removeOps(t, "")
	cases := []struct {
		totem int64
		code  error
		want  string
	}{
		{801, apierr.ErrConflict, "belongs to another player"},
		{811, apierr.ErrConflict, "belongs to another player"},
		{901, apierr.ErrConflict, "already been picked up"},
		{12345, apierr.ErrNotFound, "no base with the totem"},
	}
	before := o.S.Revision()
	for _, c := range cases {
		_, err := o.PreviewBaseRemoval(c.totem)
		refusedWith(t, err, c.code, c.want)
		pv := `x`
		_ = pv
		_, err = o.BackUpAndRemoveBase(c.totem, "anything")
		refusedWith(t, err, c.code, c.want)
	}
	// a member (a rank below owner) of a base nobody else owns
	o2 := removeOps(t, `delete from permission_actor_rank where permission_actor_id=811 and player_id=99;`)
	_, err := o2.PreviewBaseRemoval(811)
	refusedWith(t, err, apierr.ErrConflict, "only a member")
	// a base nobody holds the owner rank over
	o3 := removeOps(t, `delete from permission_actor_rank where permission_actor_id=601;`)
	_, err = o3.PreviewBaseRemoval(601)
	refusedWith(t, err, apierr.ErrConflict, "Nobody holds the owner rank")
	if o.S.Revision() != before || len(o.S.Ops()) != 0 || rmCount(t, o, `base_backups`) != 1 || rmCount(t, o, `actors where state=5`) != 2 {
		t.Fatal("a refusal must change nothing")
	}
	if _, err := o.BackUpAndRemoveBase(601, ""); err == nil || !strings.Contains(err.Error(), "fingerprint is missing") {
		t.Fatalf("a write with no fingerprint: %v", err)
	}
}

func TestTheWriteIsRefusedWhenTheBaseChangedSinceTheQuestion(t *testing.T) {
	o := removeOps(t, "")
	pv, err := o.PreviewBaseRemoval(601)
	if err != nil {
		t.Fatal(err)
	}
	fp := pv.(map[string]any)["fingerprint"].(string)
	testsave.Exec(t, o.S, `insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,4,'Wall',5764607523034234881,1.0)`)
	_, err = o.BackUpAndRemoveBase(601, fp)
	refusedWith(t, err, apierr.ErrConflict, "changed since the question")
	if rmCount(t, o, `actors where id=602`) != 1 || rmCount(t, o, `base_backups`) != 1 {
		t.Fatal("a refused write changed the save")
	}
}

func TestABuildingActorThatHoldsAnotherBasesPiecesIsRefused(t *testing.T) {
	o := removeOps(t, `insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,9,'Wall',5764607523035235001,1.0);
insert into fgl_entities(entity_id,components) values (5764607523035235001, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (801,5764607523035235001,'Actor');`)
	_, err := o.PreviewBaseRemoval(601)
	refusedWith(t, err, apierr.ErrConflict, "also holds 1 pieces of another base")
	// a piece that no base owns moves with the actor and is said
	o2 := removeOps(t, `insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,9,'Wall',NULL,1.0);`)
	r := remove(t, o2, 601)
	if r["pieces"] != int64(4) || !strings.Contains(fmt.Sprint(r["warnings"]), "belong to no base") {
		t.Fatalf("%v", r)
	}
}

func TestAMarkerIsRemovedOnlyWhenItIsClearlyThisBases(t *testing.T) {
	o := removeOps(t, `delete from player_markers where marker_hash_id=111;`)
	r := remove(t, o, 601)
	if r["markerRemoved"] != false || !strings.Contains(fmt.Sprint(r["warnings"]), "No map marker of this base") || rmCount(t, o, `player_markers`) != 2 {
		t.Fatalf("no marker: %v", r)
	}
	o2 := removeOps(t, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (444,0,'HaggaBasin','HomeBase',5000,6000,7);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,444,0,'HaggaBasin',3,10);`)
	r = remove(t, o2, 601)
	if r["markerRemoved"] != false || !strings.Contains(fmt.Sprint(r["warnings"]), "2 map markers could belong") || rmCount(t, o2, `player_markers`) != 4 {
		t.Fatalf("two markers: %v", r)
	}
}

func TestStoredItemsAreSaidAndLeftExactlyAsTheyAre(t *testing.T) {
	o := removeOps(t, "")
	before, _ := o.S.Query(`select id, inventory_id, stack_size, position_index, template_id from items order by id`)
	r := remove(t, o, 601)
	if !strings.Contains(fmt.Sprint(r["warnings"]), "stay where they are") {
		t.Fatalf("warnings: %v", r["warnings"])
	}
	after, _ := o.S.Query(`select id, inventory_id, stack_size, position_index, template_id from items order by id`)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("items changed: %v -> %v", before, after)
	}
}

// What the editor writes, the editor reads: the base backup it makes is listed, exports as a base file and the importer takes that file back as one more backup.
func TestTheBackupItMakesCanBeExportedAndImportedAgain(t *testing.T) {
	o := removeOps(t, `update actors set location_x=1,location_y=2,location_z=3,rotation_x=0,rotation_y=0,rotation_z=0,rotation_w=1 where location_x is null or location_x=5001 or id in (601,701);
update building_instances set location_x=1,location_y=2,location_z=3,rotation_x=0,rotation_y=0,rotation_z=0,rotation_w=1;`)
	// the standing base exports before, and refuses after (it is picked up)
	if _, err := o.ExportStandingBaseConsole(601, time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), "t"); err != nil {
		t.Fatalf("standing export before: %v", err)
	}
	r := remove(t, o, 601)
	_, err := o.ExportStandingBaseConsole(601, time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), "t")
	refusedWith(t, err, apierr.ErrConflict, "picked up")
	list, err := o.BaseBackups()
	if err != nil {
		t.Fatal(err)
	}
	var found save.Row
	for _, b := range list.([]save.Row) {
		if b["backup_id"] == r["backupId"] {
			found = b
		}
	}
	if found == nil || found["pieces"] != int64(3) || found["name"] == nil {
		t.Fatalf("the backup is not listed with its pieces: %v", list)
	}
	out, err := o.ExportBaseBackup(r["backupId"].(int64), time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), "t")
	if err != nil {
		t.Fatalf("export of the new backup: %v", err)
	}
	if _, err := basefile.Check(out.Data); err != nil {
		t.Fatalf("the file does not pass its own check: %v", err)
	}
	if _, err := o.ExportBaseBackupConsole(r["backupId"].(int64), time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), "t"); err != nil {
		t.Fatalf("console export of the new backup: %v", err)
	}
	if _, err := o.PreviewBaseImport(out.Data); err != nil {
		t.Fatalf("import preview: %v", err)
	}
	if _, err := o.ImportBaseBackup(out.Data); err != nil {
		t.Fatalf("import: %v", err)
	}
	if rmCount(t, o, `base_backups`) != 3 {
		t.Fatalf("the imported file is a further backup: %d", rmCount(t, o, `base_backups`))
	}
}

func TestBackUpAndRemoveIsHeldWhileAGameProcessExistsAndWritesNothingToTheFile(t *testing.T) {
	// the gate itself is the save layer's; this checks the edit is one queued edit with a description that names the base, and that a refused base queues nothing
	o := removeOps(t, "")
	remove(t, o, 601)
	p := o.S.Pending()
	if len(p) != 1 || !strings.HasPrefix(p[0], `Back up and remove base "My base"`) {
		t.Fatalf("pending: %v", p)
	}
	_, err := o.BackUpAndRemoveBase(801, "x")
	if err == nil || len(o.S.Pending()) != 1 {
		t.Fatalf("a refused base must queue nothing: %v", o.S.Pending())
	}
}
