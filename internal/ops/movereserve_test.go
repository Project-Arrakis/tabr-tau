package ops

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Move the Reserve into two named storage boxes (#1191). The world: the character's Reserve (inventory 12, type 33) with five stacks; a base of the character's (totem 601) with the
// Developer storage containers "Box A" (703) and "Box B" (704) and several that must be refused or never found (two named "Twin", a hologram, one in the normal-state check, a
// Storage Container that is not a Developer one, one never renamed); a second base of the character's that is being recycled (611) with "Box C"; a base of the character's in the Deep
// Desert (621) with "DD Box"; and a base of another player (801) with "Theirs".
//
// The Reserve, in its order: Scrap Metal (300, position 0), a Copper Ingot (1, position 1, durability and grade), Stone 500 (position 2), Stone 250 (position 3), and Scrap Metal 7 at position 5 with a
// volume_override of 2.5 per unit and a stat blob, so that a move that touches anything but the place of a row shows.
func moveWorld(aCap, bCap int, aVol, bVol float64) string {
	return fmt.Sprintf(`
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (12,2,33,-1,0.0);
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0,0,0,1,0,0,10,1,jsonb('{}')),
  (611,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',6000,6000,7,0,0,0,1,0,0,10,1,jsonb('{}')),
  (621,'/Game/BP_Totem.BP_Totem_C','DeepDesert',100,100,7,0,0,0,1,0,0,10,1,jsonb('{}')),
  (801,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',9000,9000,7,0,0,0,1,0,0,10,1,jsonb('{}')),
  (703,'/Game/BP_Box.BP_Box_C','HaggaBasin',5001,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (704,'/Game/BP_Box.BP_Box_C','HaggaBasin',5002,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (705,'/Game/BP_Box.BP_Box_C','HaggaBasin',5003,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (706,'/Game/BP_Box.BP_Box_C','HaggaBasin',5004,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (707,'/Game/BP_Box.BP_Box_C','HaggaBasin',5005,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (708,'/Game/BP_Box.BP_Box_C','HaggaBasin',9001,9000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (709,'/Game/BP_Box.BP_Box_C','HaggaBasin',5006,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (710,'/Game/BP_Box.BP_Box_C','HaggaBasin',5007,6000,7,0,0,0,1,0,5,5,1,jsonb('{}')),
  (711,'/Game/BP_Box.BP_Box_C','HaggaBasin',5008,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (712,'/Game/BP_Box.BP_Box_C','HaggaBasin',5009,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (713,'/Game/BP_Box.BP_Box_C','HaggaBasin',6001,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (721,'/Game/BP_Box.BP_Box_C','DeepDesert',101,100,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (722,'/Game/BP_Box.BP_Box_C',null,5010,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (631,'/Game/BP_Totem.BP_Totem_C',null,100,100,7,0,0,0,1,0,0,10,1,jsonb('{}')),
  (641,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',100,100,7,0,0,0,1,0,5,10,1,jsonb('{}')),
  (651,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',100,100,7,0,0,0,1,0,0,10,1,jsonb('{}')),
  (723,'/Game/BP_Box.BP_Box_C','HaggaBasin',5011,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (724,'/Game/BP_Box.BP_Box_C','HaggaBasin',5012,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (725,'/Game/BP_Box.BP_Box_C','HaggaBasin',5013,6000,7,0,0,0,1,0,0,5,1,jsonb('{}')),
  (726,'/Game/BP_Box.BP_Box_C','HaggaBasin',5014,6000,7,0,0,0,1,0,0,5,1,jsonb('{}'));
insert into fgl_entities(entity_id,components) values (5764607523034234881, jsonb('{}')),(5764607523034234882, jsonb('{}')),(5764607523034234883, jsonb('{}')),(5764607523034234884, jsonb('{}')),(5764607523034234885, jsonb('{}'));
insert into fgl_entities(entity_id,components) values (5764607523034234886, jsonb('{}')),(5764607523034234887, jsonb('{}')),(5764607523034234888, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,5764607523034234881,'Actor'),(611,5764607523034234882,'Actor'),(621,5764607523034234883,'Actor'),(801,5764607523034234884,'Actor'),
  (631,5764607523034234886,'Actor'),(641,5764607523034234887,'Actor'),(651,5764607523034234888,'Actor');
insert into totems(id) values (601),(611),(621),(801),(631),(641),(651);
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (3,1,'##Totem_Placeable',1);
insert into base_backup_linked_actors(id,actor_id) values (3,651);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values
  (601,'My base',3,3,0,0),(611,'Old base',3,3,0,0),(621,'Deep base',3,3,0,0),(801,'Other base',3,3,0,0),
  (703,'Box A',1,3,1,0),(704,'Box B',1,3,1,0),(705,'Twin',1,3,1,0),(706,'Twin',1,3,1,0),(707,'Holo',1,3,1,0),(708,'Theirs',1,3,1,0),(709,'Chest',1,3,1,0),(710,'Picked',1,3,1,0),
  (711,'##Developer_StorageContainer_Placeable',1,3,1,0),(712,'Uncapped',1,3,1,0),(713,'Box C',1,3,1,0),(721,'DD Box',1,3,1,0),(722,'NoMap',1,3,1,0),(723,'TotemDD',1,3,1,0),(724,'TotemNoMap',1,3,1,0),(725,'TotemState',1,3,1,0),(726,'TotemLinked',1,3,1,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(611,1,1),(621,1,1),(801,99,1);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support,is_hologram) values
  (601,5764607523034234881,'Totem_Placeable',1,0),(611,5764607523034234882,'Totem_Placeable',1,0),(621,5764607523034234883,'Totem_Placeable',1,0),(801,5764607523034234884,'Totem_Placeable',1,0),
  (703,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),(704,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),
  (705,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),(706,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),
  (707,5764607523034234881,'Developer_StorageContainer_Placeable',1,1),(708,5764607523034234884,'Developer_StorageContainer_Placeable',1,0),
  (709,5764607523034234881,'StorageContainer_Placeable',1,0),(710,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),
  (711,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),(712,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),
  (713,5764607523034234882,'Developer_StorageContainer_Placeable',1,0),(721,5764607523034234883,'Developer_StorageContainer_Placeable',1,0),(722,5764607523034234881,'Developer_StorageContainer_Placeable',1,0),
  (723,5764607523034234883,'Developer_StorageContainer_Placeable',1,0),(724,5764607523034234886,'Developer_StorageContainer_Placeable',1,0),
  (725,5764607523034234887,'Developer_StorageContainer_Placeable',1,0),(726,5764607523034234888,'Developer_StorageContainer_Placeable',1,0);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values
  (7003,703,4,%d,%g),(7004,704,4,%d,%g),(7005,705,4,10,1000),(7006,706,4,10,1000),(7007,707,4,10,1000),(7008,708,4,10,1000),(7009,709,4,10,1000),(7010,710,4,10,1000),
  (7011,711,4,10,1000),(7012,712,4,-1,0),(7013,713,4,10,1000),(7021,721,4,10,1000),(7022,722,4,10,1000),(7023,723,4,10,1000),(7024,724,4,10,1000),(7025,725,4,10,1000),(7026,726,4,10,1000);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values
  (8001,12,300,0,'ScrapMetal',0,1790000001,'{"a":1}',0,null),
  (8002,12,1,1,'CopperBar',1,1790000002,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":7.5}]}',3,null),
  (8003,12,500,2,'Stone',0,1790000003,'{}',0,null),
  (8004,12,250,3,'Stone',0,1790000004,'{}',0,null),
  (8005,12,7,5,'ScrapMetal',1,1790000005,'{"b":[1,2]}',2,2.5),
  (9001,7003,40,0,'Stone',0,1790000009,'{}',0,null);`, aCap, aVol, bCap, bVol)
}

const (
	bigVol = 1e9
)

func moveOps(t *testing.T, aCap, bCap int) *Ops {
	t.Helper()
	for _, id := range []string{"ScrapMetal", "CopperBar", "Stone"} {
		if _, ok := catalog.Volume(id); !ok {
			t.Fatalf("the catalog has no volume for %s: this test needs it", id)
		}
	}
	return &Ops{S: testsave.PlayerWithSQL(t, moveWorld(aCap, bCap, bigVol, bigVol))}
}

var boxAB = []string{"Box A", "Box B"}

func itemsSnapshot(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select id, inventory_id, stack_size, position_index, template_id, is_new, acquisition_time, stats, quality_level, volume_override from items order by id`)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(rows)
}

// columnsOtherThanPlace lists every item row without its inventory and position: what a move must leave exactly as it was.
func columnsOtherThanPlace(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select id, stack_size, template_id, is_new, acquisition_time, stats, quality_level, volume_override from items order by id`)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(rows)
}

func movePreview(t *testing.T, o *Ops, names []string, opt MoveReserveOptions) map[string]any {
	t.Helper()
	pv, err := o.PreviewMoveReserve(names, opt)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	return pv.(map[string]any)
}

func moveApply(t *testing.T, o *Ops, names []string, opt MoveReserveOptions) map[string]any {
	t.Helper()
	pv := movePreview(t, o, names, opt)
	r, err := o.MoveReserve(names, pv["fingerprint"].(string), opt)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	return r.(map[string]any)
}

func where(t *testing.T, o *Ops, id int64) (inv, pos int64) {
	t.Helper()
	r, err := o.S.One(`select inventory_id i, position_index p from items where id=?`, id)
	if err != nil || r == nil {
		t.Fatalf("item %d: %v %v", id, r, err)
	}
	return r["i"].(int64), r["p"].(int64)
}

// Everything moves, whole and unchanged: only inventory_id and position_index of a row change, the rows keep their ids, and nothing else of the save changes.
func TestMoveReserveMovesEveryStackWholeAndUnchanged(t *testing.T) {
	o := moveOps(t, 3, 10) // Box A has one stack in it and room for two more; Box B is empty
	before := itemsSnapshot(t, o)
	otherColumns := columnsOtherThanPlace(t, o)
	pv := movePreview(t, o, boxAB, MoveReserveOptions{})
	if pv["dryRun"] != true || pv["changed"] != true || pv["stacks"] != 5 || pv["items"] != int64(1058) {
		t.Fatalf("preview: %v", pv)
	}
	if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 || o.S.Dirty() {
		t.Fatal("the preview changed the save")
	}
	// the other rows of the save: a snapshot that covers every table the move could touch by mistake
	other := everything(t, o)

	r := moveApply(t, o, boxAB, MoveReserveOptions{})
	if r["dryRun"] != false || r["changed"] != true || r["stacks"] != 5 {
		t.Fatalf("answer: %v", r)
	}
	// the first two rows fill Box A's two free slots (positions 1 and 2; 0 is taken), the other three go to Box B's first positions: first box first, in the Reserve's order
	for id, want := range map[int64][2]int64{8001: {7003, 1}, 8002: {7003, 2}, 8003: {7004, 0}, 8004: {7004, 1}, 8005: {7004, 2}} {
		if i, p := where(t, o, id); i != want[0] || p != want[1] {
			t.Errorf("item %d is in inventory %d at %d, want %d at %d", id, i, p, want[0], want[1])
		}
	}
	if i, p := where(t, o, 9001); i != 7003 || p != 0 {
		t.Errorf("the stack that was in Box A moved: %d %d", i, p)
	}
	if n := rmCount(t, o, `items where inventory_id=12`); n != 0 {
		t.Fatalf("%d stacks are left in the Reserve", n)
	}
	// every other column of every row is as it was: compare with the snapshot with inventory and position taken out
	if got := columnsOtherThanPlace(t, o); got != otherColumns {
		t.Fatalf("a column other than the place of a row changed:\nbefore %s\nafter  %s", otherColumns, got)
	}
	for _, c := range []struct {
		id         int64
		stats      string
		quality    int64
		isNew      int64
		override   any
		stackWants int64
	}{{8002, `{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":7.5}]}`, 3, 1, nil, 1}, {8005, `{"b":[1,2]}`, 2, 1, 2.5, 7}} {
		row, _ := o.S.One(`select stats, quality_level q, is_new n, volume_override o, stack_size s from items where id=?`, c.id)
		if row["stats"] != c.stats || row["q"] != c.quality || row["n"] != c.isNew || row["o"] != c.override || row["s"] != c.stackWants {
			t.Errorf("item %d changed: %v", c.id, row)
		}
	}
	if got := everything(t, o); got == other {
		// everything() lists the items table with places, which must differ: it proves the check looks at what moved
		t.Fatal("the snapshot did not see the move")
	}
	if len(o.S.Ops()) != 1 || !strings.HasPrefix(o.S.Ops()[0].Desc, "Move the Reserve into storage") {
		t.Fatalf("one named edit: %v", o.S.Ops())
	}

	// a second run: the Reserve is empty, nothing to do, nothing written
	n := len(o.S.Ops())
	pv2 := movePreview(t, o, boxAB, MoveReserveOptions{})
	if pv2["nothingToDo"] != true || pv2["changed"] != false {
		t.Fatalf("empty Reserve: %v", pv2)
	}
	r2, err := o.MoveReserve(boxAB, pv2["fingerprint"].(string), MoveReserveOptions{})
	if err != nil || r2.(map[string]any)["nothingToDo"] != true || len(o.S.Ops()) != n {
		t.Fatalf("nothing to do must write nothing: %v %v", err, o.S.Ops())
	}
}

// Room: exactly enough is fine, one slot short is a refusal that changes nothing and says what was needed and what is free.
func TestMoveReserveRoomBySlots(t *testing.T) {
	// total room: Box A 3-1=2 free + Box B cap
	t.Run("spills from box 1 into box 2", func(t *testing.T) {
		o := moveOps(t, 3, 10)
		r := moveApply(t, o, boxAB, MoveReserveOptions{})
		bx := r["boxes"].([]map[string]any)
		if bx[0]["stacks"] != int64(2) || bx[1]["stacks"] != int64(3) {
			t.Fatalf("boxes: %v", bx)
		}
	})
	t.Run("everything fits in box 1 alone when it has room", func(t *testing.T) {
		o := moveOps(t, 10, 10)
		r := moveApply(t, o, boxAB, MoveReserveOptions{})
		bx := r["boxes"].([]map[string]any)
		if bx[0]["stacks"] != int64(5) || bx[1]["stacks"] != int64(0) {
			t.Fatalf("boxes: %v", bx)
		}
		if rmCount(t, o, `items where inventory_id=7004`) != 0 {
			t.Fatal("box 2 stays empty")
		}
	})
	t.Run("exactly fits", func(t *testing.T) {
		o := moveOps(t, 3, 3) // 2 free in A, 3 in B: 5 stacks
		r := moveApply(t, o, boxAB, MoveReserveOptions{})
		if r["changed"] != true || rmCount(t, o, `items where inventory_id=12`) != 0 {
			t.Fatalf("%v", r)
		}
		bx := r["boxes"].([]map[string]any)
		if bx[0]["stacks"] != int64(2) || bx[1]["stacks"] != int64(3) {
			t.Fatalf("boxes: %v", bx)
		}
	})
	t.Run("one slot short is refused and nothing changes", func(t *testing.T) {
		o := moveOps(t, 3, 2) // 2 free in A, 2 in B: 4 of 5
		before := itemsSnapshot(t, o)
		_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "does not fit")
		for _, w := range []string{"Needed: 5 stacks", `"Box A" has 2 slots free`, `"Box B" has 2 slots free`, "Short by 1 stacks", "nothing was changed"} {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("the refusal lacks %q: %v", w, err)
			}
		}
		_, err = o.MoveReserve(boxAB, "anything", MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "does not fit") // the plan is refused before the fingerprint is looked at
		if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
			t.Fatal("a refusal changed the save")
		}
	})
}

func TestMoveReserveRoomByVolume(t *testing.T) {
	ore, _ := catalog.Volume("ScrapMetal")
	stone, _ := catalog.Volume("Stone")
	knife, _ := catalog.Volume("CopperBar")
	reserve := 300*ore + knife + 500*stone + 250*stone + 7*2.5 // the Reserve's volume; the last stack has its own volume per unit
	mk := func(aVol, bVol float64) *Ops {
		return &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, aVol, bVol))}
	}
	t.Run("volume spills from box 1 to box 2", func(t *testing.T) {
		// Box A has 40 Stone in it (volume 40*stone). Give it room for the first two stacks only
		first := 300*ore + knife
		o := mk(40*stone+first+0.01, bigVol)
		r := moveApply(t, o, boxAB, MoveReserveOptions{})
		bx := r["boxes"].([]map[string]any)
		if bx[0]["stacks"] != int64(2) || bx[1]["stacks"] != int64(3) {
			t.Fatalf("by volume 2 and 3: %v", bx)
		}
	})
	t.Run("exactly the volume that is free fits", func(t *testing.T) {
		o := mk(40*stone+reserve, 1)
		r := moveApply(t, o, boxAB, MoveReserveOptions{})
		bx := r["boxes"].([]map[string]any)
		if bx[0]["stacks"] != int64(5) {
			t.Fatalf("all five in Box A: %v", bx)
		}
	})
	t.Run("short by volume is refused with the numbers and nothing changes", func(t *testing.T) {
		o := mk(40*stone+0.1, 0.1) // neither box has room for even the first stack
		before := itemsSnapshot(t, o)
		_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "does not fit")
		for _, w := range []string{"Needed: 5 stacks", "0.1 volume free", "Short by 5 stacks"} {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("the refusal lacks %q: %v", w, err)
			}
		}
		if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
			t.Fatal("a refusal changed the save")
		}
	})
	t.Run("a row's own volume_override counts per unit", func(t *testing.T) {
		// only the override stack (7 units at 2.5) is left to place: the room is 17.4, just short of 17.5
		o := mk(40*stone+300*ore+knife+500*stone+250*stone+17.4, 1)
		_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
	})
	t.Run("no volume limit recorded means no volume check", func(t *testing.T) {
		o := mk(0, 0)
		r := moveApply(t, o, boxAB, MoveReserveOptions{})
		if r["changed"] != true || rmCount(t, o, `items where inventory_id=12`) != 0 {
			t.Fatalf("%v", r)
		}
	})
}

// Which boxes: a name that matches none or more than one, a box that must not receive, and the same box twice are refused naming what was found; nothing is changed.
func TestMoveReserveRefusesBoxesItCannotFindOrMayNotUse(t *testing.T) {
	for _, c := range []struct {
		name  string
		names []string
		opt   MoveReserveOptions
		code  error
		want  string
	}{
		{"no name matches", []string{"Box A", "Nowhere"}, MoveReserveOptions{}, apierr.ErrNotFound, `No Developer storage container in this save is named "Nowhere"`},
		{"two boxes have the name", []string{"Box A", "Twin"}, MoveReserveOptions{}, apierr.ErrConflict, `2 Developer storage containers are named "Twin" (number 705 on HaggaBasin; number 706 on HaggaBasin)`},
		{"the same name twice", []string{"Box A", "Box A"}, MoveReserveOptions{}, apierr.ErrBadArgument, `"Box A" is named twice`},
		{"the same name twice in another case", []string{"Box A", " box a "}, MoveReserveOptions{}, apierr.ErrBadArgument, "is named twice"},
		{"a box on the Deep Desert", []string{"Box A", "DD Box"}, MoveReserveOptions{}, apierr.ErrConflict, `"DD Box" (number 721) is on the Deep Desert`},
		{"a box in a base that is being recycled", []string{"Box A", "Box C"}, MoveReserveOptions{ExcludeBases: []int64{611}}, apierr.ErrConflict, "stands in base 611, which is being recycled"},
		{"a box of another player", []string{"Box A", "Theirs"}, MoveReserveOptions{}, apierr.ErrConflict, "which is not yours"},
		{"a hologram", []string{"Box A", "Holo"}, MoveReserveOptions{}, apierr.ErrConflict, "only a hologram"},
		{"a box that is not in the normal state", []string{"Box A", "Picked"}, MoveReserveOptions{}, apierr.ErrConflict, "not in the normal state"},
		{"a Storage Container that is not a Developer one", []string{"Box A", "Chest"}, MoveReserveOptions{}, apierr.ErrNotFound, `named "Chest"`},
		{"a box never renamed has no name", []string{"Box A", "##Developer_StorageContainer_Placeable"}, MoveReserveOptions{}, apierr.ErrBadArgument, "not a name a box can be found by"},
		{"an empty name", []string{"Box A", " "}, MoveReserveOptions{}, apierr.ErrBadArgument, "not a name a box can be found by"},
		{"a box with no map recorded", []string{"Box A", "NoMap"}, MoveReserveOptions{}, apierr.ErrConflict, "has no map recorded"},
		{"a box in a base on the Deep Desert", []string{"Box A", "TotemDD"}, MoveReserveOptions{}, apierr.ErrConflict, "which is on the Deep Desert"},
		{"a box in a base with no map recorded", []string{"Box A", "TotemNoMap"}, MoveReserveOptions{}, apierr.ErrConflict, "which has no map recorded"},
		{"a box in a base that is not in the normal state", []string{"Box A", "TotemState"}, MoveReserveOptions{}, apierr.ErrConflict, "base 641, which is not in the normal state"},
		{"a box in a base that is part of a base backup", []string{"Box A", "TotemLinked"}, MoveReserveOptions{}, apierr.ErrConflict, "base 651, which is not in the normal state"},
		{"a box with no slot count", []string{"Box A", "Uncapped"}, MoveReserveOptions{}, apierr.ErrConflict, "no slot count above 0"},
		{"no names at all and no box named DD-1", nil, MoveReserveOptions{}, apierr.ErrNotFound, "No Developer storage box named DD-1"},
		{"a name is case sensitive", []string{"Box A", "box b"}, MoveReserveOptions{}, apierr.ErrNotFound, `named "box b"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := moveOps(t, 10, 10)
			before := itemsSnapshot(t, o)
			_, err := o.PreviewMoveReserve(c.names, c.opt)
			refusedWith(t, err, c.code, c.want)
			if !strings.Contains(err.Error(), "Nothing was changed") {
				t.Errorf("the refusal says nothing was changed: %v", err)
			}
			if _, err := o.MoveReserve(c.names, "x", c.opt); err == nil {
				t.Fatal("the write must refuse too")
			}
			if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 || o.S.Dirty() {
				t.Fatal("a refusal changed the save")
			}
		})
	}
}

// An empty Reserve is not an error, and it writes nothing.
func TestMoveReserveEmptyReserveWritesNothing(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, bigVol, bigVol)+`delete from items where inventory_id=12;`)}
	before := itemsSnapshot(t, o)
	pv := movePreview(t, o, boxAB, MoveReserveOptions{})
	if pv["nothingToDo"] != true || pv["stacks"] != 0 {
		t.Fatalf("%v", pv)
	}
	r, err := o.MoveReserve(boxAB, pv["fingerprint"].(string), MoveReserveOptions{})
	if err != nil || r.(map[string]any)["changed"] != false {
		t.Fatalf("%v %v", r, err)
	}
	if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 || o.S.Dirty() {
		t.Fatal("an empty Reserve wrote something")
	}
	// the boxes are still checked: a bad name is a refusal even when there is nothing to move
	if _, err := o.PreviewMoveReserve([]string{"Box A", "Nowhere"}, MoveReserveOptions{}); err == nil {
		t.Fatal("a name that finds no box is refused whatever the Reserve holds")
	}
}

func TestMoveReserveWithoutOrWithAStaleFingerprintWritesNothing(t *testing.T) {
	o := moveOps(t, 10, 10)
	_, err := o.MoveReserve(boxAB, "", MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrBadArgument, "fingerprint is missing")
	_, err = o.MoveReserve(boxAB, "nope", MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, "have changed since the question was shown")
	// the Reserve changes between the question and the answer: the stale fingerprint is refused
	pv := movePreview(t, o, boxAB, MoveReserveOptions{})
	testsave.Exec(t, o.S, `update items set stack_size=301 where id=8001`)
	_, err = o.MoveReserve(boxAB, pv["fingerprint"].(string), MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, "have changed since the question was shown")
	// so does a different box set
	pv = movePreview(t, o, boxAB, MoveReserveOptions{})
	_, err = o.MoveReserve([]string{"Box B", "Box A"}, pv["fingerprint"].(string), MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, "have changed since the question was shown")
	if rmCount(t, o, `items where inventory_id=12`) != 5 {
		t.Fatal("nothing moved")
	}
}

// A read back that differs from the plan fails the whole edit: nothing is moved, nothing is queued.
func TestMoveReserveReadBackThatDiffersRefusesTheWholeEdit(t *testing.T) {
	for name, tamper := range map[string]string{
		"a stack came back to the Reserve":               `update items set inventory_id=12 where id=8004`,
		"a stack changed its size":                       `update items set stack_size=stack_size+1 where id=8003`,
		"a stack changed its stats":                      `update items set stats='{}' where id=8001`,
		"a stack is in the wrong slot":                   `update items set position_index=9 where id=8002`,
		"two stacks share a position":                    `update items set position_index=0 where id=8004`,
		"a stack in the box is over its slot count":      `update items set position_index=50 where id=8001`,
		"a stack that was moved was deleted":             `delete from items where id=8005`,
		"a stack that was in the box lost its place":     `delete from items where id=9001`,
		"an item of the Reserve appeared after the move": `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (9999,12,1,9,'Stone',0,1,'{}',0)`,
	} {
		t.Run(name, func(t *testing.T) {
			o := moveOps(t, 10, 10)
			pv := movePreview(t, o, boxAB, MoveReserveOptions{})
			before := itemsSnapshot(t, o)
			moveReserveBeforeReadBack = func(m *save.Mut) {
				if _, err := m.Exec(tamper); err != nil {
					t.Errorf("tamper: %v", err)
				}
			}
			defer func() { moveReserveBeforeReadBack = nil }()
			_, err := o.MoveReserve(boxAB, pv["fingerprint"].(string), MoveReserveOptions{})
			if err == nil || !strings.Contains(err.Error(), "Nothing was changed") {
				t.Fatalf("a read back that differs must refuse: %v", err)
			}
			if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
				t.Fatal("a failed read back must roll the whole edit back")
			}
		})
	}
}

// A queued move is put back onto the save the game wrote last, and dropped when the game changed anything the move read.
func TestMoveReserveQueuedWhileTheGameRunsIsDroppedWhenTheGameChangedWhatItRead(t *testing.T) {
	setup := func(t *testing.T) (*Ops, func()) {
		o := moveOps(t, 10, 10)
		o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
		restore := save.ActiveSessionForTests()
		moveApply(t, o, boxAB, MoveReserveOptions{})
		if !o.S.Dirty() || len(o.S.Pending()) != 1 {
			t.Fatalf("the move is queued as one edit: %v", o.S.Pending())
		}
		return o, restore
	}
	gameWrites := func(t *testing.T, o *Ops, sql string) {
		other, err := save.Open(o.S.Path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := other.Mutate("the game's change", func(m *save.Mut) error { _, err := m.Exec(sql); return err }); err != nil {
			t.Fatal(err)
		}
		if _, err := other.CommitForTests(); err != nil {
			t.Fatal(err)
		}
		other.Close()
	}
	t.Run("the game changed a stack of the Reserve", func(t *testing.T) {
		o, restore := setup(t)
		restore() // the game has closed, having written the file meanwhile
		gameWrites(t, o, `update items set stack_size=999 where id=8003`)
		if f := o.S.Flush(); f.Saved || o.S.Held() == nil {
			t.Fatalf("the queued move conflicts with the game's file and is held, not written: %+v", f)
		}
		file, err := save.Open(o.S.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		r, _ := file.One(`select inventory_id i, stack_size s from items where id=8003`)
		if r["i"] != int64(12) || r["s"] != int64(999) {
			t.Fatalf("the game's file is as the game left it: %v", r)
		}
	})
	t.Run("the game changed a stack of a box", func(t *testing.T) {
		o, restore := setup(t)
		restore()
		gameWrites(t, o, `update items set stack_size=41 where id=9001`)
		if f := o.S.Flush(); f.Saved || o.S.Held() == nil {
			t.Fatalf("held: %+v", f)
		}
	})
	t.Run("a change to something the move never read does not hold it back", func(t *testing.T) {
		o, restore := setup(t)
		restore()
		gameWrites(t, o, `update actors set location_x=1 where id=601`)
		if f := o.S.Flush(); !f.Saved || o.S.Held() != nil {
			t.Fatalf("written: %+v", f)
		}
		file, err := save.Open(o.S.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if n, _ := file.One(`select count(*) n from items where inventory_id=12`); n["n"] != int64(0) {
			t.Fatalf("the Reserve was emptied in the file: %v", n)
		}
	})
}

// The refusal for a Reserve that is not exactly one inventory.
func TestMoveReserveNeedsExactlyOneReserve(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, bigVol, bigVol)+`insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (13,2,33,-1,0);`)}
	_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, "2 Reserve inventories")
}

// A name that finds no box says which names the editor did read from the save, so the person can see what to type.
func TestMoveReserveNoMatchListsTheNamesTheEditorRead(t *testing.T) {
	o := moveOps(t, 10, 10)
	_, err := o.PreviewMoveReserve([]string{"Box A", "DD-2"}, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrNotFound, `named "DD-2"`)
	for _, w := range []string{`"Box A" (1)`, `"Box B" (1)`, `"Twin" (2)`, "were never renamed", "matched exactly, capitals included"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("the refusal lacks %q: %v", w, err)
		}
	}
}

// The answer says where everything went and how much room is left.
func TestMoveReserveAnswerNamesTheBoxesAndTheirRoom(t *testing.T) {
	o := moveOps(t, 3, 10)
	r := moveApply(t, o, boxAB, MoveReserveOptions{})
	s := strings.Join(r["summary"].([]string), "\n")
	for _, w := range []string{"Moved: 5 item stacks (1,058 items)", `Box "Box A" (in "My base"): 2 stacks`, `Box "Box B"`, "The Reserve is empty afterwards"} {
		if !strings.Contains(s, w) {
			t.Errorf("the summary lacks %q:\n%s", w, s)
		}
	}
}

// An item with no known volume (a Knife has none in the game's item table) is moved, counts as no volume, and the answer says how many units that was.
func TestMoveReserveSaysWhenAnItemHasNoKnownVolume(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 0, 0)+`insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8006,12,3,7,'Knife',0,1,'{}',0);`)}
	r := moveApply(t, o, boxAB, MoveReserveOptions{})
	w := fmt.Sprint(r["warnings"])
	if r["stacks"] != 6 || !strings.Contains(w, "3 units in the Reserve have no known volume") {
		t.Fatalf("%v", r)
	}
}

// Units whose volume is not known (cosmetics, uniques, placeables: no entry in the game's item table, no volume of their own) are accepted into a box with a volume limit, as Give
// item accepts them: they count as none by volume, take one slot per stack, and the answer says how many units that was.
func TestMoveReserveAcceptsUnknownVolumeUnderAVolumeLimitAndCountsItAsNone(t *testing.T) {
	stone, _ := catalog.Volume("Stone")
	unknown := `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8006,12,10000,7,'Knife',0,1,'{}',0);`
	reserve := moveReserveVolume()
	// Box A has a volume limit that holds the known stacks exactly, Box B a tiny one: 10,000 units would not fit if they counted, the stack still goes in
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 40*stone+reserve, 0.5)+unknown)}
	r := moveApply(t, o, boxAB, MoveReserveOptions{})
	if r["changed"] != true || r["stacks"] != 6 || rmCount(t, o, `items where inventory_id=12`) != 0 {
		t.Fatalf("%v", r)
	}
	if w := fmt.Sprint(r["warnings"]); !strings.Contains(w, "10,000 units in the Reserve have no known volume and were counted as none") {
		t.Fatalf("the answer must say how many units had no known volume: %v", r["warnings"])
	}
	// the volume the answer reports is the known volume only
	if v, _ := r["volume"].(float64); v < reserve-1e-6 || v > reserve+1e-6 {
		t.Fatalf("volume %v, want %v", v, reserve)
	}
	// a limit on one box only behaves the same
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 0, 0.5)+unknown)}
	if r := moveApply(t, o, boxAB, MoveReserveOptions{}); r["changed"] != true || rmCount(t, o, `items where inventory_id=12`) != 0 {
		t.Fatalf("%v", r)
	}
	// the slot limit still holds for them, one slot per stack: Box A has 1 slot free (2 slots, one stack in it) and Box B has 1, so two stacks fit and three do not
	two := `delete from items where inventory_id=12;
		insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8101,12,50,0,'Knife',0,1,'{}',0),(8102,12,60,1,'Knife',0,1,'{}',0);`
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(2, 1, 1, 1)+two)}
	if r := moveApply(t, o, boxAB, MoveReserveOptions{}); r["stacks"] != 2 {
		t.Fatalf("%v", r)
	}
	three := two + `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8103,12,1,2,'Knife',0,1,'{}',0);`
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(2, 1, 1, 1)+three)}
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
	if !IsMoveNoRoom(err) || itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
		t.Fatalf("a lack of slots is the no-room kind and changes nothing: %v", err)
	}
}

// moveReserveVolume is the volume of the Reserve of moveWorld: the unit volumes of the game's item table, and 2.5 per unit for the row that has one of its own.
func moveReserveVolume() float64 {
	ore, _ := catalog.Volume("ScrapMetal")
	stone, _ := catalog.Volume("Stone")
	bar, _ := catalog.Volume("CopperBar")
	return 300*ore + bar + 500*stone + 250*stone + 7*2.5
}

// The move and Give item agree. For the same box (its slots, its volume limit, what is already in it) and the same item, the move accepts K whole stacks exactly when a give of K full
// stacks of that item is given in full: the give is run on an identical fixture through the give's own code (giveUpToInTx), for every K from 1 to 14, with limits by slots, by volume,
// both, and none, with an item that has no known volume, with a box that already holds a row with a volume of its own, and with Reserve stacks that carry a volume of their own (#1199).
func TestMoveReserveAgreesWithGiveItemOnWhatFits(t *testing.T) {
	ore, _ := catalog.Volume("ScrapMetal")
	stone, _ := catalog.Volume("Stone")
	stack, _ := catalog.MaxStack("ScrapMetal")
	if stack != 500 || ore <= 0 {
		t.Fatalf("the test needs ScrapMetal with stack 500 and a volume: %d %v", stack, ore)
	}
	per := float64(stack) * ore // the volume of one full stack
	cases := []struct {
		name     string
		cap      int
		vol      float64 // Box A's volume limit (0 = none)
		template string
		extra    string // sql run on the world
		// ovr is the volume per unit the rows of the Reserve carry of their own (0 = none: the row has NULL). The give is run with the item table's volume and no override, so a case with
		// ovr a multiple of the item's volume gives the give a limit of giveVol (0 = vol) that is the same multiple smaller.
		ovr, giveVol float64
	}{
		{"slots only", 5, 0, "ScrapMetal", "", 0, 0}, // Box A holds 40 Stone in one slot: 4 free
		{"volume only", 1000, 40*stone + 3.5*per, "ScrapMetal", "", 0, 0},
		{"volume exactly", 1000, 40*stone + 3*per, "ScrapMetal", "", 0, 0},
		{"volume a hair short", 1000, 40*stone + 3*per - 0.05, "ScrapMetal", "", 0, 0},
		{"slots tighter than volume", 3, 40*stone + 9*per, "ScrapMetal", "", 0, 0},
		{"volume tighter than slots", 12, 40*stone + 2.2*per, "ScrapMetal", "", 0, 0},
		{"no limits", 1000, 0, "ScrapMetal", "", 0, 0},
		{"unknown volume: slots decide", 5, 40*stone + 0.1, "Knife", "", 0, 0},
		{"a row of its own volume already there", 1000, 40*2.5 + 4.5*per, "ScrapMetal", `update items set volume_override=2.5 where id=9001;`, 0, 0},
		{"reserve rows carry the item table's volume as their own: exactly", 1000, 40*stone + 3*per, "ScrapMetal", "", ore, 0},
		{"reserve rows carry the item table's volume as their own: a hair short", 1000, 40*stone + 3*per - 0.05, "ScrapMetal", "", ore, 0},
		{"reserve rows carry twice the volume: per unit, exactly", 1000, 6 * per, "ScrapMetal", `delete from items where id=9001;`, 2 * ore, 3 * per},
		{"reserve rows carry twice the volume: per unit, a hair short", 1000, 6*per - 0.1, "ScrapMetal", `delete from items where id=9001;`, 2 * ore, 3*per - 0.05},
		{"reserve rows carry twice the volume: slots tighter", 2, 60 * per, "ScrapMetal", `delete from items where id=9001;`, 2 * ore, 30 * per},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for k := 1; k <= 14; k++ {
				var rows []string
				for i := 0; i < k; i++ {
					override := "null"
					if c.ovr > 0 {
						override = fmt.Sprintf("%g", c.ovr)
					}
					rows = append(rows, fmt.Sprintf("(%d,12,%d,%d,'%s',0,1,'{}',0,%s)", 8200+i, stack, i, c.template, override))
				}
				insert := c.extra + `delete from items where inventory_id=12; insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values ` + strings.Join(rows, ",") + `;`
				world := moveWorld(c.cap, 10, c.vol, bigVol) + insert
				giveWorld := world
				if c.giveVol > 0 {
					giveWorld = moveWorld(c.cap, 10, c.giveVol, bigVol) + insert
				}

				// the move: only Box A is named, so everything must fit in it
				o := &Ops{S: testsave.PlayerWithSQL(t, world)}
				_, mvErr := o.PreviewMoveReserve([]string{"Box A"}, MoveReserveOptions{})
				if mvErr != nil && !IsMoveNoRoom(mvErr) {
					t.Fatalf("K=%d: the move was refused for another reason: %v", k, mvErr)
				}

				// the give: the same units of the same item into the same box (inventory 7003)
				g := &Ops{S: testsave.PlayerWithSQL(t, giveWorld)}
				var out giveOutcome
				if _, err := g.S.Mutate("give", func(m *save.Mut) error {
					var err error
					out, err = giveUpToInTx(m, 7003, c.template, int64(k)*stack, 0)
					return err
				}); err != nil {
					t.Fatalf("K=%d: give: %v", k, err)
				}
				if c.template == "Knife" {
					// an item the give does not know goes in as one row, so the comparison is the slots both count: free slots by the give's count against one slot per stack
					if out.Given != out.Requested || out.VolumeChecked {
						t.Fatalf("K=%d: an item with no known volume is not checked by volume in a give: %+v", k, out)
					}
					if fits := int64(k) <= out.FreeSlots; fits != (mvErr == nil) {
						t.Fatalf("K=%d: the give counts %d free slots but the move said %v", k, out.FreeSlots, mvErr)
					}
					continue
				}
				if whole := out.Given == out.Requested; whole != (mvErr == nil) {
					t.Fatalf("K=%d: the give placed %d of %d units (stopped by %q) but the move said %v", k, out.Given, out.Requested, out.Limit, mvErr)
				}
			}
		})
	}
}

// The refusal of a Reserve that does not fit can be told from every other refusal by its kind, not by its words.
func TestMoveReserveNoRoomHasItsOwnKind(t *testing.T) {
	o := moveOps(t, 3, 2)
	_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
	if !IsMoveNoRoom(err) || !errors.Is(err, apierr.ErrNoRoom) {
		t.Fatalf("%v", err)
	}
	for _, other := range []error{nil, errors.New("does not fit"), apierr.New(apierr.ErrNoRoom, "The Reserve does not fit into the boxes"), apierr.New(apierr.ErrConflict, "x")} {
		if IsMoveNoRoom(other) {
			t.Errorf("%v is not the refusal of a Reserve that does not fit", other)
		}
	}
	_, err = o.PreviewMoveReserve([]string{"Box A", "Nowhere"}, MoveReserveOptions{})
	if IsMoveNoRoom(err) {
		t.Fatal("a missing box is not a lack of room")
	}
}

// A name is compared without the spaces around it on both sides: "Box B " in the save is "Box B", and two boxes that differ only in such spaces are two matches.
func TestMoveReserveNamesAreComparedTrimmedOnBothSides(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, bigVol, bigVol)+`update permission_actor set actor_name='Box B ' where actor_id=704;`)}
	r := moveApply(t, o, []string{"Box A", " Box B"}, MoveReserveOptions{})
	if r["changed"] != true {
		t.Fatalf("%v", r)
	}
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, bigVol, bigVol)+`update permission_actor set actor_name='Box B ' where actor_id=704; update permission_actor set actor_name='Box B' where actor_id=705;`)}
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve([]string{"Box A", "Box B"}, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, `2 Developer storage containers are named "Box B"`)
	if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
		t.Fatal("a refusal changed the save")
	}
}

// A box whose contents are in places the game would not use (two stacks on one slot, a stack beyond the slot count) is refused when the boxes are looked at.
func TestMoveReserveRefusesABoxWithItemsInPlacesTheGameWouldNotUse(t *testing.T) {
	row := `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (9002,7003,1,%d,'Stone',0,1,'{}',0);`
	for name, c := range map[string]struct {
		extra string
		cap   int
	}{
		"two stacks share a slot":       {fmt.Sprintf(row, 0), 3},
		"a stack beyond the slot count": {fmt.Sprintf(row, 3), 3},
		"a stack far beyond":            {fmt.Sprintf(row, 900), 10},
	} {
		t.Run(name, func(t *testing.T) {
			o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(c.cap, 10, bigVol, bigVol)+c.extra)}
			before := itemsSnapshot(t, o)
			_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
			refusedWith(t, err, apierr.ErrConflict, "has items in places the game would not use")
			if !strings.Contains(err.Error(), `"Box A"`) || !strings.Contains(err.Error(), "it was not touched") && !strings.Contains(err.Error(), "so it cannot receive the Reserve") {
				t.Errorf("the refusal names the box: %v", err)
			}
			if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
				t.Fatal("a refusal changed the save")
			}
		})
	}
}

// A volume of its own below 0 is refused, in the Reserve and in a box.
func TestMoveReserveRefusesANegativeVolumeOverride(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, bigVol, bigVol)+`update items set volume_override=-1 where id=8005;`)}
	_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, "volume of its own below 0")
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, bigVol, bigVol)+`update items set volume_override=-0.5 where id=9001;`)}
	_, err = o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrConflict, "volume of their own below 0")
}

// The digest that proves a move changed nothing but the place of a row lists its columns by hand. This fails when the save's own items table has a column the digest (or the two
// columns that make the place) does not name, so a column the game adds later is not left unchecked.
func TestTheMoveDigestNamesEveryColumnOfTheItemsTable(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "kit", "schema", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	i := strings.Index(text, "CREATE TABLE items (")
	if i < 0 {
		t.Fatal("the schema has no items table")
	}
	body := text[i:]
	body = body[:strings.Index(body, ") STRICT")]
	var cols []string
	for _, l := range strings.Split(body, "\n")[1:] {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, `"`) {
			cols = append(cols, l[1:strings.Index(l[1:], `"`)+1])
		}
	}
	want := map[string]bool{"inventory_id": true, "position_index": true}
	for _, c := range digestColumns {
		want[c] = true
	}
	got := map[string]bool{}
	for _, c := range cols {
		got[c] = true
		if !want[c] {
			t.Errorf("the items table has the column %q, which the move's digest does not list", c)
		}
	}
	for c := range want {
		if !got[c] {
			t.Errorf("the digest lists %q, which the items table does not have", c)
		}
	}
	if len(cols) < 8 {
		t.Fatalf("the columns were not read: %v", cols)
	}
}

// realVolumeItems lists the items of the game's item table that have a volume above 0, with the volume of a unit and the stack size, for the tests that need real magnitudes: units from
// 0.001 to 125.
func realVolumeItems(t *testing.T) (ids []string, unit []float64, stack []int64) {
	t.Helper()
	for _, e := range catalog.All() {
		v, ok := catalog.Volume(e.ID)
		s, _ := catalog.MaxStack(e.ID)
		if !ok || v <= 0 || s <= 0 {
			continue
		}
		ids, unit, stack = append(ids, e.ID), append(unit, v), append(stack, s)
	}
	if len(ids) < 100 {
		t.Fatalf("the item table has only %d items with a volume", len(ids))
	}
	return ids, unit, stack
}

// stackSizes are the stack sizes the Reserve of a recycle holds most (#1199 simulated 5, 50 and 500).
var stackSizes = []int64{5, 50, 500}

// An exact fill by volume (#1199): a box whose free volume equals the volume of what is moved to the digit takes the last stack, with the real unit volumes of the game's item table
// (0.001 to 125) and stacks of 5, 50 and 500, however the float sums are ordered; a box short by one smallest unit (0.001) still refuses it. The old count (the give's, 1e-9 of a unit)
// is run on the same data to show that the data bites: it refuses some of the exact fills.
func TestStackFitsByVolumeExactFillWithRealMagnitudes(t *testing.T) {
	ids, unit, stack := realVolumeItems(t)
	_ = ids
	rng := rand.New(rand.NewSource(1199))
	var oldRefusals, trials int
	for trial := 0; trial < 4000; trial++ {
		nBox, nRes := 1+rng.Intn(20), 1+rng.Intn(120)
		if trial%4 == 0 {
			nBox, nRes = 20+rng.Intn(200), 200+rng.Intn(1300) // the Reserve of several recycles
		}
		pick := func() (float64, int64) {
			i := rng.Intn(len(unit))
			n := min(stackSizes[rng.Intn(len(stackSizes))], stack[i])
			return unit[i], n
		}
		var contents float64
		for i := 0; i < nBox; i++ {
			u, n := pick()
			contents += u * float64(n)
		}
		type st struct {
			u float64
			n int64
		}
		var res []st
		for i := 0; i < nRes; i++ {
			u, n := pick()
			if i == nRes-1 && trial%2 == 0 {
				// the last stack decides an exact fill: half of the runs end on one of the items with the smallest unit volumes (0.001 to 0.05), where the float noise weighs most
				for u > 0.05 {
					u, n = pick()
				}
			}
			res = append(res, st{u, n})
		}
		// the limit: contents plus the Reserve, added in the opposite order to the plan's, as a box that was filled by hand in the game would add up
		limit := 0.0
		for i := len(res) - 1; i >= 0; i-- {
			limit += res[i].u * float64(res[i].n)
		}
		limit += contents
		// the plan: the box first, then the stacks in order
		used, oldOK := contents, true
		for _, s := range res {
			if !stackFitsByVolume(limit, used, s.u, s.n) {
				t.Fatalf("trial %d: an exact fill was refused (limit %v, used %v, stack %d x %v)", trial, limit, used, s.n, s.u)
			}
			if unitsLeftByVolume(limit, used, s.u) < s.n {
				oldOK = false
			}
			used += s.u * float64(s.n)
		}
		trials++
		if !oldOK {
			oldRefusals++
		}
		// the same, one smallest unit of volume short: the last stack must be refused (the stacks before it fit, as they did)
		short := limit - 0.001
		used = contents
		for i, s := range res {
			fits := stackFitsByVolume(short, used, s.u, s.n)
			if i < len(res)-1 && used+s.u*float64(s.n) <= short-1e-6 && !fits {
				t.Fatalf("trial %d: stack %d of %d was refused though it fits well (short limit %v, used %v)", trial, i, len(res), short, used)
			}
			if i == len(res)-1 && fits {
				t.Fatalf("trial %d: the last stack was taken into a box short by 0.001 (limit %v, used %v, stack %d x %v)", trial, short, used, s.n, s.u)
			}
			used += s.u * float64(s.n)
		}
	}
	if oldRefusals == 0 {
		t.Fatalf("the data does not show the problem: the give's count refused none of %d exact fills", trials)
	}
	t.Logf("the give's count alone refused %d of %d exact fills", oldRefusals, trials)
}

// The tolerance is relative to the limit and safe at the magnitudes of the item table. The move's answer is the read back's rule (volumeWithinLimit) and nothing else; the last column
// of a row says what the give's own count answers for the same numbers, so that a row about the difference between the two cannot pass without showing it.
func TestStackFitsByVolumeTolerance(t *testing.T) {
	for _, c := range []struct {
		name           string
		limit, used, u float64
		n              int64
		want, give     bool
	}{
		{"one smallest unit over, small box", 100, 99.5, 0.001, 501, false, false},
		{"exactly full, small box", 100, 99.5, 0.001, 500, true, true},
		{"a float sum one step over the limit still fits", 50000, math.Nextafter(49999.5, math.Inf(1)), 0.001, 500, true, false},
		{"over the limit by less than the slack", 50000, 49999.5 + 2e-7, 0.001, 500, true, false},
		{"over the limit by more than the slack", 50000, 49999.5 + 2e-6, 0.001, 500, false, false},
		{"past the slack although the give's 1e-9 of a unit lets it in", 1000, 0, 125.00000001, 8, false, true},
		{"contents off the grid: past the slack although the give lets it in", 1000, 7 * 125.00000001, 125, 1, false, true},
		{"a full box takes a stack smaller than the slack, where the give gives nothing", 50000, 50000, 1e-8, 1, true, false},
		{"one smallest unit over, 5,000,000 volume", 5e6, 5e6 - 0.5, 0.001, 501, false, false},
		{"exactly full, 5,000,000 volume", 5e6, 5e6 - 0.5, 0.001, 500, true, true},
		{"the largest unit over by a unit", 12500, 0, 125, 101, false, false},
		{"the largest unit exactly", 12500, 0, 125, 100, true, true},
		{"a box that is already over takes nothing with volume", 10, 40, 0.001, 1, false, false},
		{"no limit recorded", 0, 1e9, 125, 1000, true, false},
		{"no volume", 10, 1e9, 0, 1000, true, false},
	} {
		if got := stackFitsByVolume(c.limit, c.used, c.u, c.n); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
		// the give's count is only asked where it is defined: with a limit and a unit volume above 0
		if c.limit > 0 && c.u > 0 {
			if got := unitsLeftByVolume(c.limit, c.used, c.u) >= c.n; got != c.give {
				t.Errorf("%s: the give's count says %v, the row expects %v", c.name, got, c.give)
			}
		}
	}
}

// The plan and the read back use one rule (#1199): a stack that the give's own count would let in (1e-9 of a unit over) but that leaves the box over its limit by more than the read
// back's slack is not planned into that box. It goes into the next box, or the move is refused as no room, which a layout roll reads as "the boxes are full"; before, the plan took it
// and the read back then refused the whole edit as a conflict. It takes a volume of its own that is finer than the item table's 0.001.
func TestMoveReserveThePlanRefusesWhatTheReadBackWouldRefuse(t *testing.T) {
	for name, extra := range map[string]string{
		"the Reserve's stack has a volume of its own off the grid": `delete from items where id=9001 or inventory_id=12;
			insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8401,12,8,0,'Stone',0,1,'{}',0,125.00000001);`,
		"the box's contents have a volume of their own off the grid": `update items set stack_size=7, volume_override=125.00000001 where id=9001; delete from items where inventory_id=12;
			insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8401,12,1,0,'Stone',0,1,'{}',0,125);`,
	} {
		t.Run(name, func(t *testing.T) {
			// only Box A named: refused as no room, nothing changed
			o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 1000, bigVol)+extra)}
			before := itemsSnapshot(t, o)
			_, err := o.PreviewMoveReserve([]string{"Box A"}, MoveReserveOptions{})
			refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
			if !IsMoveNoRoom(err) || itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
				t.Fatalf("no room: refused, nothing changed: %v", err)
			}
			// with a second box named, the stack goes there
			moveApply(t, o, boxAB, MoveReserveOptions{})
			if i, _ := where(t, o, 8401); i != 7004 {
				t.Fatalf("the stack goes into Box B, not into inventory %d", i)
			}
		})
	}
}

// An exact fill of a round limit (1,000 and 10,000,000), through the whole move (#1199): the Reserve's stacks add up to the limit to the digit in thousandths, and the float sum, in the
// plan's order, comes out a hair over it, far enough that the give's own count refuses the last stack. The move takes every stack and the read back accepts the box; this fails when the
// plan loses its slack. A limit one smallest unit (0.001) lower is refused with nothing changed.
func TestMoveReserveExactFillOfARoundLimitThroughTheWholeMove(t *testing.T) {
	// per sets the size of the stacks, so that the Reserve has a few hundred: enough sums for the float noise to show, within the box's 1,000 slots
	for _, c := range []struct {
		limit float64
		per   int64
	}{{1000, 200}, {1e7, 50}} {
		limit := c.limit
		t.Run(fmt.Sprintf("limit %.0f", limit), func(t *testing.T) {
			rng := rand.New(rand.NewSource(1199))
			var rows []string
			for attempt := 0; attempt < 5000 && rows == nil; attempt++ {
				var r []string
				used := 0.0
				add := func(milli, n int64) { // a stack of n units of milli thousandths each, as a volume of its own
					r = append(r, fmt.Sprintf("(%d,12,%d,%d,'Stone',0,1,'{}',0,%d.%03d)", 8400+len(r), n, len(r), milli/1000, milli%1000))
					used += float64(milli) / 1000 * float64(n)
				}
				rest := int64(limit) * 1000 // what is left to fill, in thousandths
				for {
					n := stackSizes[rng.Intn(len(stackSizes))]
					milli := 1 + rng.Int63n(min(125000, int64(limit)*1000/c.per/n))
					if milli*n > rest-1000 {
						break
					}
					add(milli, n)
					rest -= milli * n
				}
				add(rest-1, 1)
				// the premise: the give's count refuses the last stack (one smallest unit), and with it the float sum is over the limit
				refused := unitsLeftByVolume(limit, used, 0.001) < 1
				add(1, 1)
				if refused && used > limit && len(r) <= 1000 {
					rows = r
				}
			}
			if rows == nil {
				t.Fatal("no Reserve was found that shows the float noise: the test would prove nothing")
			}
			reserve := `delete from items where id=9001 or inventory_id=12; insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values ` + strings.Join(rows, ",") + `;`
			mk := func(aVol float64) *Ops {
				return &Ops{S: testsave.PlayerWithSQL(t, moveWorld(1000, 1000, aVol, 0.0004)+reserve)}
			}
			o := mk(limit)
			r := moveApply(t, o, boxAB, MoveReserveOptions{})
			if bx := r["boxes"].([]map[string]any); bx[0]["stacks"] != int64(len(rows)) || rmCount(t, o, `items where inventory_id=12`) != 0 {
				t.Fatalf("every stack must go into Box A: %v", bx)
			}
			o = mk(limit - 0.001)
			before := itemsSnapshot(t, o)
			_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
			refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
			if !IsMoveNoRoom(err) || itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
				t.Fatalf("a box short by 0.001 must refuse and change nothing: %v", err)
			}
		})
	}
}

// An exact fill, through the whole move (#1199): the Reserve is made of real items at real magnitudes, the limit of Box A is the volume of its contents plus the Reserve (to the digit),
// Box B can hold nothing, so every stack must go into Box A and the read back must accept the box as full. One smallest unit (0.001) less is refused with nothing changed.
func TestMoveReserveExactFillByVolumeThroughTheWholeMove(t *testing.T) {
	ids, unit, stack := realVolumeItems(t)
	stone, _ := catalog.Volume("Stone")
	for seed := int64(0); seed < 8; seed++ {
		rng := rand.New(rand.NewSource(seed + 1199))
		var rows []string
		total := 0.0
		var plan []float64
		count := 200 + rng.Intn(400)
		for i := 0; i < count; i++ {
			j := rng.Intn(len(ids))
			for i == count-1 && unit[j] > 0.05 { // the last stack decides an exact fill: it has one of the smallest unit volumes
				j = rng.Intn(len(ids))
			}
			n := min(stackSizes[rng.Intn(len(stackSizes))], stack[j])
			rows = append(rows, fmt.Sprintf("(%d,12,%d,%d,'%s',0,1,'{}',0,null)", 8400+i, n, i, ids[j]))
			plan = append(plan, unit[j]*float64(n))
		}
		for i := len(plan) - 1; i >= 0; i-- {
			total += plan[i]
		}
		limit := 40*stone + total // Box A holds 40 Stone already
		reserve := `delete from items where inventory_id=12; insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values ` + strings.Join(rows, ",") + `;`
		mk := func(aVol float64) *Ops {
			return &Ops{S: testsave.PlayerWithSQL(t, moveWorld(1000, 1000, aVol, 0.0004)+reserve)}
		}
		o := mk(limit)
		r := moveApply(t, o, boxAB, MoveReserveOptions{})
		bx := r["boxes"].([]map[string]any)
		if bx[0]["stacks"] != int64(len(rows)) || bx[1]["stacks"] != int64(0) || rmCount(t, o, `items where inventory_id=12`) != 0 {
			t.Fatalf("seed %d: every stack must go into Box A: %v", seed, bx)
		}
		o = mk(limit - 0.001)
		before := itemsSnapshot(t, o)
		_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
		refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
		if !IsMoveNoRoom(err) || itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
			t.Fatalf("seed %d: a box short by 0.001 must refuse and change nothing: %v", seed, err)
		}
	}
}

// The read back re-sums the volume of every box that received something, with the give's own count (units of unknown volume count as none), and refuses the whole edit when a box is
// over its volume limit, as it already did for the slots (#1199).
func TestMoveReserveReadBackRefusesABoxOverItsVolumeLimit(t *testing.T) {
	stone, _ := catalog.Volume("Stone")
	reserve := moveReserveVolume()
	mk := func() *Ops {
		return &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 40*stone+reserve, bigVol)+`insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8006,12,10000,7,'Knife',0,1,'{}',0);`)}
	}
	// without a tamper: the box is exactly full, the unknown-volume stack counts as none, and the read back accepts it
	o := mk()
	if r := moveApply(t, o, boxAB, MoveReserveOptions{}); r["changed"] != true {
		t.Fatalf("%v", r)
	}
	for name, tamper := range map[string]string{
		"a stack already in the box grew": `update items set stack_size=stack_size+1000 where id=9001`,
		"the box's limit was lowered":     `update inventories set max_item_volume=1 where id=7003`,
	} {
		t.Run(name, func(t *testing.T) {
			o := mk()
			pv := movePreview(t, o, boxAB, MoveReserveOptions{})
			before := itemsSnapshot(t, o)
			moveReserveBeforeReadBack = func(m *save.Mut) {
				if _, err := m.Exec(tamper); err != nil {
					t.Errorf("tamper: %v", err)
				}
			}
			defer func() { moveReserveBeforeReadBack = nil }()
			_, err := o.MoveReserve(boxAB, pv["fingerprint"].(string), MoveReserveOptions{})
			if err == nil || !strings.Contains(err.Error(), "over its volume limit") || !strings.Contains(err.Error(), "Nothing was changed") {
				t.Fatalf("a box over its volume limit after the move must refuse: %v", err)
			}
			if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
				t.Fatal("a failed read back must roll the whole edit back")
			}
		})
	}
	t.Run("a stack in the box that turns into units of unknown volume counts as none", func(t *testing.T) {
		o := mk()
		pv := movePreview(t, o, boxAB, MoveReserveOptions{})
		moveReserveBeforeReadBack = func(m *save.Mut) {
			if _, err := m.Exec(`update items set template_id='Knife' where id=9001`); err != nil {
				t.Errorf("tamper: %v", err)
			}
		}
		defer func() { moveReserveBeforeReadBack = nil }()
		if _, err := o.MoveReserve(boxAB, pv["fingerprint"].(string), MoveReserveOptions{}); err != nil {
			t.Fatalf("units with no known volume are no volume: %v", err)
		}
	})
}

// The Reserve's stacks may carry a volume of their own (the game reads it per unit). The move counts it as the give counts a row's override, and the Reserve here has one stack with
// 2.5 per unit (8005) next to stacks without.
func TestMoveReserveOwnVolumeOfAReserveStackIsCountedPerUnit(t *testing.T) {
	stone, _ := catalog.Volume("Stone")
	reserve := moveReserveVolume() // 8005: 7 units x 2.5
	// a box that has room for the Reserve without the override stack's 17.5 and not with it: the whole Reserve must go to the second box
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 40*stone+reserve-17.4, bigVol))}
	r := moveApply(t, o, boxAB, MoveReserveOptions{})
	if i, _ := where(t, o, 8005); i != 7004 {
		t.Fatalf("the stack with a volume of its own (17.5) does not fit in Box A (17.4 short) and goes to Box B: %v", r["boxes"])
	}
	// the override is per unit, not a total: 7 x 2.5 is 17.5; had it been counted as a total (2.5) the stack would fit
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 40*stone+reserve-14, bigVol))}
	moveApply(t, o, boxAB, MoveReserveOptions{})
	if i, _ := where(t, o, 8005); i != 7004 {
		t.Fatal("a row's own volume is per unit")
	}
}

// Two boxes, volume spills from the first into the second, and a stack of units with no known volume sits between known stacks: it costs a slot and no volume, so it goes into the first
// box that has a free slot, even when that box is full by volume, exactly as a give of such an item is not stopped by volume. Known stacks that no longer fit go on to Box B.
func TestMoveReserveVolumeSpillWithAnUnknownVolumeStack(t *testing.T) {
	ore, _ := catalog.Volume("ScrapMetal")
	stone, _ := catalog.Volume("Stone")
	reserve := `delete from items where inventory_id=12;
		insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
		(8301,12,300,0,'ScrapMetal',0,1,'{}',0),(8302,12,10,1,'Knife',0,1,'{}',0),(8303,12,500,2,'Stone',0,1,'{}',0),(8304,12,5,3,'Knife',0,1,'{}',0),(8305,12,100,4,'ScrapMetal',0,1,'{}',0);`
	// Box A (40 Stone in it) has room for the first stack and nothing more; Box B for the Stone and the last stack, to the digit
	o := &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 40*stone+300*ore, 500*stone+100*ore)+reserve)}
	r := moveApply(t, o, boxAB, MoveReserveOptions{})
	for id, want := range map[int64][2]int64{8301: {7003, 1}, 8302: {7003, 2}, 8304: {7003, 3}, 8303: {7004, 0}, 8305: {7004, 1}} {
		if i, p := where(t, o, id); i != want[0] || p != want[1] {
			t.Errorf("item %d is in inventory %d at %d, want %d at %d", id, i, p, want[0], want[1])
		}
	}
	if w := fmt.Sprint(r["warnings"]); !strings.Contains(w, "15 units in the Reserve have no known volume") {
		t.Errorf("the answer must say how many units had no known volume: %v", r["warnings"])
	}
	bx := r["boxes"].([]map[string]any)
	if bx[0]["stacks"] != int64(3) || bx[1]["stacks"] != int64(2) {
		t.Errorf("3 and 2 stacks: %v", bx)
	}
	// the same Reserve, Box B one smallest unit short: the last stack fits nowhere and the whole move is refused
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(10, 10, 40*stone+300*ore, 500*stone+100*ore-0.001)+reserve)}
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve(boxAB, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
	if itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
		t.Fatal("a refusal changed the save")
	}
}

// A box that already holds more than its volume limit (the game lets that happen: a limit lowered, or items put in by other means). Give item calls such a box full for every item that has
// a volume (it gives nothing) and still gives an item with no volume while a slot is free; the move does the same, because it uses the give's count: the box takes no stack that has a
// volume, a stack with no known volume still goes in, and when the box is the only one named and the Reserve has a stack with a volume the move is refused as no room.
func TestMoveReserveABoxAlreadyOverItsVolumeLimitIsFullForStacksWithVolume(t *testing.T) {
	stone, _ := catalog.Volume("Stone")
	knife := `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8006,12,3,7,'Knife',0,1,'{}',0);`
	world := moveWorld(10, 10, 10*stone, bigVol) // Box A holds 40 Stone and may hold 10 Stone's volume

	// the give, as the reference: nothing with a volume, the unknown item yes
	g := &Ops{S: testsave.PlayerWithSQL(t, world)}
	var withVolume, withoutVolume giveOutcome
	if _, err := g.S.Mutate("give", func(m *save.Mut) error {
		var err error
		if withVolume, err = giveUpToInTx(m, 7003, "ScrapMetal", 1, 0); err != nil {
			return err
		}
		withoutVolume, err = giveUpToInTx(m, 7003, "Knife", 3, 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if withVolume.Given != 0 || withVolume.Limit != limitVolume || withoutVolume.Given != 3 {
		t.Fatalf("the give: %+v / %+v", withVolume, withoutVolume)
	}

	// the move: the Reserve (five stacks with a volume) all goes to Box B, and Box A is not touched and not blamed
	o := &Ops{S: testsave.PlayerWithSQL(t, world+knife)}
	r := moveApply(t, o, boxAB, MoveReserveOptions{})
	bx := r["boxes"].([]map[string]any)
	if bx[0]["stacks"] != int64(1) || bx[1]["stacks"] != int64(5) {
		t.Fatalf("only the stack with no volume goes into Box A: %v", bx)
	}
	if i, _ := where(t, o, 8006); i != 7003 {
		t.Fatal("the stack with no known volume takes a slot of Box A")
	}

	// only Box A named: the Reserve has stacks with a volume, so it is refused as no room, and nothing changes
	o = &Ops{S: testsave.PlayerWithSQL(t, world)}
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve([]string{"Box A"}, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrNoRoom, "Short by 5 stacks")
	if !IsMoveNoRoom(err) || itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
		t.Fatalf("a box over its volume limit has no room for stacks with a volume: refused, nothing changed: %v", err)
	}
}

// A box that holds a stack of the same item that is not full: a give tops that stack up and needs no slot; the move never tops up (a row is never merged or split) and needs a slot for
// the Reserve's stack. So a box with no free slot takes a give of the item and refuses the move. This is documented in docs/recycle-a-base.md.
func TestMoveReserveNeedsASlotWhereAGiveWouldTopUpANonFullStack(t *testing.T) {
	if s, _ := catalog.MaxStack("Stone"); s <= 41 {
		t.Fatalf("the test needs Stone to stack past 41: %d", s)
	}
	one := `delete from items where inventory_id=12; insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8401,12,1,0,'Stone',0,1,'{}',0);`
	// Box A has one slot and holds 40 Stone in it (not full)
	world := moveWorld(1, 10, 0, 0) + one

	g := &Ops{S: testsave.PlayerWithSQL(t, world)}
	var out giveOutcome
	if _, err := g.S.Mutate("give", func(m *save.Mut) error {
		var err error
		out, err = giveUpToInTx(m, 7003, "Stone", 1, 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if out.Given != 1 || out.FreeSlots != 0 || len(out.IDs) != 0 {
		t.Fatalf("the give tops up the stack that is there and needs no slot: %+v", out)
	}
	if n, _ := g.S.One(`select stack_size s from items where id=9001`); n["s"] != int64(41) {
		t.Fatalf("the stack was topped up: %v", n)
	}

	// the move into the same box: no free slot, so it is refused as no room, although a give took the unit
	o := &Ops{S: testsave.PlayerWithSQL(t, world)}
	before := itemsSnapshot(t, o)
	_, err := o.PreviewMoveReserve([]string{"Box A"}, MoveReserveOptions{})
	refusedWith(t, err, apierr.ErrNoRoom, "Short by 1 stacks")
	if !IsMoveNoRoom(err) || itemsSnapshot(t, o) != before || len(o.S.Ops()) != 0 {
		t.Fatalf("no free slot: refused, nothing changed: %v", err)
	}

	// with a free slot the stack goes into a row of its own: the Stone that was there is not topped up, and the Reserve's row keeps its size
	o = &Ops{S: testsave.PlayerWithSQL(t, moveWorld(2, 10, 0, 0)+one)}
	moveApply(t, o, []string{"Box A"}, MoveReserveOptions{})
	if i, p := where(t, o, 8401); i != 7003 || p != 1 {
		t.Errorf("the Reserve's stack is a row of its own in Box A: %d %d", i, p)
	}
	if n, _ := o.S.One(`select stack_size s from items where id=9001`); n["s"] != int64(40) {
		t.Errorf("the stack that was there is not topped up: %v", n)
	}
	if n, _ := o.S.One(`select stack_size s from items where id=8401`); n["s"] != int64(1) {
		t.Errorf("the Reserve's stack is not changed: %v", n)
	}
}
