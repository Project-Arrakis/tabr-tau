package ops

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A Solido of the base made first by the recycle (#1187). The world is the recycle's own, with: the character's loadout inventory (15, eight slots) holding a blank Solido device
// (8800), a flag on two pieces (the foundation provides stability; the door frame has the bit that the game leaves out), and a Pentashield with a scale. The building actor is at
// 5001 6001 8 with no rotation, so a piece's blueprint position is its position minus that.
const (
	devBlank  = `{"FItemStackAndDurabilityStats":[[],{"DecayedMaxDurability":0.0}],"FCustomizationStats":[[],{}]}`
	devFilled = `{"FItemStackAndDurabilityStats":[[],{"DecayedMaxDurability":0.0}],"FCustomizationStats":[[],{}],"FBuildingBlueprintItemStats":[[],{"PlayerBlueprintId":"!!bbp#7","PlayerBaseBackupId":{}}]}`
)

// devRecord is the blank device's stats with the blueprint record of blueprint n.
func devRecord(n int) string {
	return strings.Replace(devFilled, "#7", fmt.Sprintf("#%d", n), 1)
}

func solidoWorld(items string) string {
	return `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (15,2,15,8,0.0);
update building_instances set building_flags=1 where building_id=602 and instance_id=1;
update building_instances set building_flags=4 where building_id=602 and instance_id=4;
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (707,'/Game/BP_Pentashield.BP_Pentashield_C','HaggaBasin',5101,6201,18,0,0,0,1,0,0,6,1,jsonb('{"BP_CHOAM_PentashieldSurfaceVertical_C":{"m_PersistentScale":{"X":8.000014,"Y":6.000018,"Z":1.0}}}'));
insert into placeables(id,owner_entity_id,health,building_type,has_hit_ground,has_buildable_support,is_hologram,last_placed_by_player_id) values
  (707,5764607523034234881,60.0,'Choam_PentashieldSurfaceVertical_Placeable',1,1,0,1);
` + items
}

const blankDevice = `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8800,15,1,2,'BuildingBlueprint_CopyDevice',0,1790000010,'` + devBlank + `',0);`

func solidoOps(t *testing.T, items string) *Ops { return recycleOps(t, solidoWorld(items)) }

// solidoState is every row a Solido touches, and the loadout, as text.
func solidoState(t *testing.T, o *Ops) string {
	t.Helper()
	var b strings.Builder
	for _, q := range []string{
		`select * from building_blueprints order by id`, `select * from building_blueprint_instances order by 1,2`, `select * from building_blueprint_placeables order by 1,2`,
		`select * from building_blueprint_pentashields order by 1,2`, `select id, inventory_id, position_index, template_id, stats from items where inventory_id in (1,15) order by id`,
	} {
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		fmt.Fprintln(&b, rows)
	}
	return b.String()
}

func recycleWithSolido(t *testing.T, o *Ops) map[string]any {
	t.Helper()
	pv, err := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	r, err := o.RecycleBaseWith(601, pv.(map[string]any)["fingerprint"].(string), RecycleOptions{Solido: true})
	if err != nil {
		t.Fatalf("recycle: %v", err)
	}
	return r.(map[string]any)
}

func TestRecycleWithASolidoMakesItFirstInTheSameEditAndTheBaseStillGoes(t *testing.T) {
	inEachLayout(t, recycleWorld+solidoWorld(blankDevice), func(t *testing.T, o *Ops) {
		before, all := solidoState(t, o), everything(t, o)
		pv, err := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
		if err != nil {
			t.Fatal(err)
		}
		p := pv.(map[string]any)
		s := p["solido"].(map[string]any)
		if p["dryRun"] != true || s["pieces"] != int64(3) || s["placeables"] != int64(6) || s["pentashields"] != int64(1) || s["itemCreated"] != false || s["item"] != int64(8800) ||
			s["piecesLeftOut"] != int64(1) || s["doorsLeftOut"] != int64(1) {
			t.Fatalf("the preview says: %v", s)
		}
		if solidoState(t, o) != before || everything(t, o) != all || len(o.S.Ops()) != 0 {
			t.Fatal("the preview changed the save")
		}

		r := recycleWithSolido(t, o)
		if !strings.Contains(r["summary"].([]string)[0], "Solido: made first from the base, in a blank Solido device that was already in your inventory (item 8800): 3 building pieces, 6 placeables (1 pentashields).") ||
			!strings.Contains(r["summary"].([]string)[0], "1 pieces the game leaves out, 1 doors, the claim totem") {
			t.Fatalf("the answer: %v", r["summary"])
		}
		// the base is gone as in any recycle
		if rmCount(t, o, `actors where id in (601,602,701,702,703,704,705,706,707)`) != 0 || rmCount(t, o, `totems where id=601`) != 0 || rmCount(t, o, `building_instances where building_id=602`) != 0 {
			t.Fatal("the base is not recycled")
		}
		// the blueprint: one row for the device, no owner, an empty map
		if rmCount(t, o, `building_blueprints where id=1 and item_id=8800 and player_id is null and building_blueprint_map=''`) != 1 || rmCount(t, o, `building_blueprints`) != 1 {
			t.Fatal("the blueprint row")
		}
		// the pieces: the door frame (bit 4) is out; positions are relative to the building actor (5001 6001 8); health 0, hologram 1; numbered from 0 in the base's own order
		ins, _ := o.S.Query(`select instance_id i, building_type t, transform_x x, transform_y y, transform_z z, transform_yaw yaw, provides_stability s, health h, hologram g from building_blueprint_instances order by 1`)
		if fmt.Sprint(ins) != "[map[g:1 h:0 i:0 s:1 t:Sardaukar_Foundation x:-1 y:-1 yaw:0 z:-1] map[g:1 h:0 i:1 s:0 t:Sardaukar_Wall_01 x:9 y:-1 yaw:0 z:-1] map[g:1 h:0 i:2 s:0 t:Sardaukar_Wall_01 x:19 y:-1 yaw:0 z:2]]" {
			t.Fatalf("pieces: %v", ins)
		}
		// the placeables: no totem and no door; a hologram and a Developer storage container stay; the angle columns are the game's own labels (pitch in transform_yaw, yaw in transform_pitch)
		pl, _ := o.S.Query(`select placeable_id i, building_type t, transform_x x, transform_y y, transform_z z, transform_yaw pitchcol, transform_pitch yawcol, transform_roll r, hologram g from building_blueprint_placeables order by 1`)
		var got []string
		for _, r := range pl {
			got = append(got, fmt.Sprintf("%v %v %v %v %v %v %v %v %v", r["i"], r["t"], r["x"], r["y"], r["z"], r["pitchcol"], r["yawcol"], r["r"], r["g"]))
		}
		want := []string{"0 SpiceSilo_Placeable 9 -1 -1 0 0 0 1", "1 Developer_StorageContainer_Placeable 29 -1 -1 0 0 0 1", "2 WindTurbineDirectional_Placeable 39 -1 -1 0 0 0 1",
			"3 Fabricator_Placeable 49 -1 -1 0 0 0 1", "4 LargeWaterCistern_Placeable 59 -1 -1 0 0 0 1", "5 Choam_PentashieldSurfaceVertical_Placeable 100 200 10 0 0 0 1"}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("placeables:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		if ps, _ := o.S.Query(`select placeable_id, scale_x, scale_y, scale_z from building_blueprint_pentashields`); fmt.Sprint(ps) != "[map[placeable_id:5 scale_x:80 scale_y:60 scale_z:10]]" {
			t.Fatalf("pentashields: %v", ps)
		}
		// the device: the same item, in the same place, with the blueprint record added after its own two
		dev, _ := o.S.One(`select inventory_id inv, position_index pos, stats from items where id=8800`)
		if dev["inv"] != int64(15) || dev["pos"] != int64(2) || dev["stats"] != devRecord(1) {
			t.Fatalf("the device: %v", dev)
		}
		if rmCount(t, o, `items where template_id='BuildingBlueprint_CopyDevice'`) != 1 {
			t.Fatal("no second device")
		}
		if o.S.Ops() == nil || len(o.S.Ops()) != 1 {
			t.Fatalf("one edit: %v", o.S.Ops())
		}
	})
}

func TestRecycleWithoutASolidoIsWhatItWasAndLeavesTheDeviceAlone(t *testing.T) {
	o := solidoOps(t, blankDevice)
	plain, perr := o.PreviewBaseRecycle(601)
	if perr != nil {
		t.Fatal(perr)
	}
	off, _ := o.PreviewBaseRecycleWith(601, RecycleOptions{})
	on, err := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
	if err != nil {
		t.Fatal(err)
	}
	if plain.(map[string]any)["fingerprint"] != off.(map[string]any)["fingerprint"] || off.(map[string]any)["fingerprint"] == on.(map[string]any)["fingerprint"] {
		t.Fatal("the fingerprint of a recycle with no Solido is what it was, and a recycle with one is another")
	}
	if _, has := off.(map[string]any)["solido"]; has {
		t.Fatal("no Solido was asked for and the answer has one")
	}
	// a question that was shown for one choice is not the answer to the other
	pv, _ := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
	if _, err := o.RecycleBase(601, pv.(map[string]any)["fingerprint"].(string)); err == nil {
		t.Fatal("a fingerprint of a recycle with a Solido was taken for one without")
	}
	if _, err := o.RecycleBaseWith(601, off.(map[string]any)["fingerprint"].(string), RecycleOptions{Solido: true}); err == nil {
		t.Fatal("a fingerprint of a recycle without a Solido was taken for one with")
	}
	before := solidoState(t, o)
	r := recycle(t, o, 601)
	if r["solido"] != nil || strings.Contains(strings.Join(r["summary"].([]string), "\n"), "Solido") || solidoState(t, o) != before {
		t.Fatal("a recycle with no Solido writes no blueprint and leaves the device alone")
	}
}

func TestWhichDeviceTheSolidoGoesIn(t *testing.T) {
	filled := func(id int, pos int) string {
		return fmt.Sprintf(`insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,15,1,%d,'BuildingBlueprint_CopyDevice',0,1790000010,'%s',0);
insert into building_blueprints(id,item_id,player_id,building_blueprint_map) values (7,%d,null,'');`, id, pos, devFilled, id)
	}
	blank := func(id, pos, inv int) string {
		return fmt.Sprintf(`insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,%d,1,%d,'BuildingBlueprint_CopyDevice',0,1790000010,'%s',0);`, id, inv, pos, devBlank)
	}
	fill := func(inv, n, first int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,%d,1,%d,'CopperOre',0,1790000010,'{}',0);`, first+i, inv, i)
		}
		return b.String()
	}
	stats := func(o *Ops, id any) string {
		r, err := o.S.One(`select stats from items where id=?`, id)
		if err != nil || r == nil {
			t.Fatalf("item %v: %v", id, err)
		}
		return fmt.Sprint(r["stats"])
	}
	t.Run("a filled one only: a new one is made, the filled one is not touched", func(t *testing.T) {
		o := solidoOps(t, filled(8700, 0))
		before, _ := o.S.One(`select id, inventory_id, position_index, stack_size, stats from items where id=8700`)
		r := recycleWithSolido(t, o)
		s := r["solido"].(map[string]any)
		id := s["item"].(int64)
		if s["itemCreated"] != true || id == 8700 || !strings.Contains(r["summary"].([]string)[0], "a new Solido device in your backpack") {
			t.Fatalf("%v", s)
		}
		after, _ := o.S.One(`select id, inventory_id, position_index, stack_size, stats from items where id=8700`)
		if fmt.Sprint(before) != fmt.Sprint(after) {
			t.Fatalf("the filled Solido changed:\n%v\n%v", before, after)
		}
		if rmCount(t, o, `building_blueprints where id=7 and item_id=8700`) != 1 || rmCount(t, o, `building_blueprint_instances where building_blueprint_id=7`) != 0 {
			t.Fatal("the filled Solido's blueprint changed")
		}
		n, _ := o.S.One(`select inventory_id inv, position_index pos, stack_size n, quality_level q, template_id t from items where id=?`, id)
		if fmt.Sprint(n) != "map[inv:1 n:1 pos:2 q:0 t:BuildingBlueprint_CopyDevice]" {
			t.Fatalf("the new item: %v", n)
		}
		want := `{"FItemStackAndDurabilityStats":[[],{"DecayedMaxDurability":0.0}],"FCustomizationStats":[[],{}],"FBuildingBlueprintItemStats":[[],{"PlayerBlueprintId":"!!bbp#` + fmt.Sprint(s["blueprint"]) + `","PlayerBaseBackupId":{}}]}`
		if got := stats(o, id); got != want || s["blueprint"] == int64(7) {
			t.Fatalf("the new item's stats: %s", got)
		}
		if rmCount(t, o, `building_blueprints where item_id=?`, id) != 1 || rmCount(t, o, `building_blueprints`) != 2 {
			t.Fatal("one new blueprint")
		}
	})
	t.Run("a filled one and a blank one: the blank one is used", func(t *testing.T) {
		o := solidoOps(t, filled(8700, 0)+blank(8900, 3, 15)+blank(8800, 2, 15))
		s := recycleWithSolido(t, o)["solido"].(map[string]any)
		if s["item"] != int64(8800) || s["itemCreated"] != false || !strings.Contains(stats(o, 8800), "!!bbp#"+fmt.Sprint(s["blueprint"])) || strings.Contains(stats(o, 8900), "FBuildingBlueprintItemStats") {
			t.Fatalf("%v", s)
		}
		if stats(o, 8700) != devFilled {
			t.Fatal("the filled one changed")
		}
	})
	t.Run("a device with a blueprint row but no record in its stats is not blank", func(t *testing.T) {
		o := solidoOps(t, blank(8700, 0, 15)+`insert into building_blueprints(id,item_id,player_id,building_blueprint_map) values (7,8700,null,'');`)
		s := recycleWithSolido(t, o)["solido"].(map[string]any)
		if s["itemCreated"] != true || s["item"] == int64(8700) || stats(o, 8700) != devBlank {
			t.Fatalf("%v", s)
		}
	})
	t.Run("none at all: a new one in the backpack, not in the loadout", func(t *testing.T) {
		o := solidoOps(t, "")
		s := recycleWithSolido(t, o)["solido"].(map[string]any)
		if s["itemCreated"] != true || s["itemWhere"] != "backpack" || rmCount(t, o, `items where inventory_id=1 and template_id='BuildingBlueprint_CopyDevice' and stats like '%!!bbp#1"%'`) != 1 ||
			rmCount(t, o, `items where inventory_id=15`) != 0 {
			t.Fatalf("%v", s)
		}
	})
	t.Run("a second Solido: the loadout is full and the first device is in the backpack, a new one is made in the backpack", func(t *testing.T) {
		o := solidoOps(t, fill(15, 8, 8700)+filled(8600, 0)+`update items set inventory_id=1 where id=8600;`)
		s := recycleWithSolido(t, o)["solido"].(map[string]any)
		if s["itemCreated"] != true || s["itemWhere"] != "backpack" || rmCount(t, o, `items where inventory_id=1 and template_id='BuildingBlueprint_CopyDevice'`) != 2 || stats(o, 8600) != devFilled {
			t.Fatalf("%v", s)
		}
	})
	t.Run("the backpack is full and the loadout is free: the new one goes in the loadout", func(t *testing.T) {
		o := solidoOps(t, fill(1, 5, 8700))
		r := recycleWithSolido(t, o)
		s := r["solido"].(map[string]any)
		if s["itemCreated"] != true || s["itemWhere"] != "loadout" || !strings.Contains(r["summary"].([]string)[0], "a new Solido device in your loadout") ||
			rmCount(t, o, `items where inventory_id=15 and template_id='BuildingBlueprint_CopyDevice'`) != 1 || rmCount(t, o, `items where inventory_id=1 and template_id='BuildingBlueprint_CopyDevice'`) != 0 {
			t.Fatalf("%v", s)
		}
	})
	t.Run("the backpack has room for slots but not for the volume: the loadout", func(t *testing.T) {
		o := solidoOps(t, `update inventories set max_item_volume=0.1 where id=1;`)
		if s := recycleWithSolido(t, o)["solido"].(map[string]any); s["itemWhere"] != "loadout" {
			t.Fatalf("%v", s)
		}
	})
	t.Run("no backpack at all: the loadout", func(t *testing.T) {
		o := solidoOps(t, `delete from inventories where id=1;`)
		if s := recycleWithSolido(t, o)["solido"].(map[string]any); s["itemWhere"] != "loadout" {
			t.Fatalf("%v", s)
		}
	})
	t.Run("a blank one in the backpack counts, as the console puts it there", func(t *testing.T) {
		o := solidoOps(t, blank(8800, 4, 1))
		if s := recycleWithSolido(t, o)["solido"].(map[string]any); s["item"] != int64(8800) || s["itemCreated"] != false {
			t.Fatalf("%v", s)
		}
	})
	t.Run("a blank one that is not the character's is not used", func(t *testing.T) {
		o := solidoOps(t, blank(8800, 0, 7001))
		if s := recycleWithSolido(t, o)["solido"].(map[string]any); s["itemCreated"] != true {
			t.Fatalf("%v", s)
		}
	})
}

func TestNoRoomForANewDeviceRefusesTheSolidoByNameAndChangesNothing(t *testing.T) {
	var full strings.Builder
	for inv, n := range map[int]int{15: 8, 1: 5} { // the loadout and the backpack both full
		for i := 0; i < n; i++ {
			fmt.Fprintf(&full, `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,%d,1,%d,'CopperOre',0,1790000010,'{}',0);`, 8700+inv*10+i, inv, i)
		}
	}
	o := solidoOps(t, full.String())
	before, all := solidoState(t, o), everything(t, o)
	_, err := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
	refusedWith(t, err, apierr.ErrConflict, "no free slot")
	if !strings.Contains(err.Error(), "Solido") || !strings.Contains(err.Error(), "Nothing was changed") || !strings.Contains(err.Error(), "the backpack has no free slot") || !strings.Contains(err.Error(), "the loadout has no free slot") {
		t.Fatalf("the refusal names the Solido, the backpack and the loadout: %v", err)
	}
	if solidoState(t, o) != before || everything(t, o) != all || len(o.S.Ops()) != 0 {
		t.Fatal("nothing changes")
	}
	// the same base without the Solido recycles as ever
	if r := recycle(t, o, 601); r["solido"] != nil {
		t.Fatal("no Solido")
	}
	// neither inventory at all
	o = recycleOps(t, solidoWorld("")+`delete from inventories where id in (1,15);`)
	_, err = o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
	refusedWith(t, err, apierr.ErrConflict, "there is no backpack")
}

func TestASolidoThatDoesNotReadBackFailsTheWholeEdit(t *testing.T) {
	for name, tamper := range map[string]string{
		"a piece is missing":        `delete from building_blueprint_instances where instance_id=1`,
		"a piece is off":            `update building_blueprint_instances set transform_x=transform_x+1 where instance_id=0`,
		"a placeable is another":    `update building_blueprint_placeables set building_type='Fabricator_Placeable' where placeable_id=0`,
		"a scale is off":            `update building_blueprint_pentashields set scale_x=1`,
		"the record is gone":        `update items set stats='{}' where id=8800`,
		"the blueprint has no item": `update building_blueprints set item_id=null`,
	} {
		t.Run(name, func(t *testing.T) {
			o := solidoOps(t, blankDevice)
			pv, err := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
			if err != nil {
				t.Fatal(err)
			}
			before, all := solidoState(t, o), everything(t, o)
			recycleBeforeReadBack = func(m *save.Mut) {
				if _, err := m.Exec(tamper); err != nil {
					t.Errorf("tamper: %v", err)
				}
			}
			defer func() { recycleBeforeReadBack = nil }()
			_, err = o.RecycleBaseWith(601, pv.(map[string]any)["fingerprint"].(string), RecycleOptions{Solido: true})
			refusedWith(t, err, apierr.ErrConflict, "The recycle was not written")
			if solidoState(t, o) != before || everything(t, o) != all || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
				t.Fatal("a failed read back rolls the Solido and the recycle back together")
			}
		})
	}
}

func TestTheSolidoIsRefusedForABaseItWasNotMeasuredFor(t *testing.T) {
	for name, c := range map[string]struct{ sql, want string }{
		"two building actors": {`insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values (603,'/Game/BP_Building.BP_Building_C','HaggaBasin',5001,6001,8,0,0,0,1,0,0,22,1,jsonb('{}'));
insert into buildings(id,owner_id) values (603,null);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,owner_entity_id,building_flags,health,shelter,sand_buildup,last_placed_by_player_id)
 values (603,1,'Sardaukar_Wall_01',5000,6000,7,0,0,0,1,5764607523034234881,0,1.0,0,0,1);`, "2 building actors"},
		"a tilted piece":                              {`update building_instances set rotation_x=0.3826834, rotation_w=0.9238795 where building_id=602 and instance_id=2`, "is tilted"},
		"a tilted placeable":                          {`update actors set rotation_y=0.3826834, rotation_w=0.9238795 where id=701`, "is tilted"},
		"a tilted building actor":                     {`update actors set rotation_y=0.3826834, rotation_w=0.9238795 where id=602`, "building actor is tilted"},
		"a Pentashield with no scale":                 {`update actors set properties=jsonb('{}') where id=707`, "scale of the Pentashield 707 could not be read"},
		"a Pentashield with a scale of no whole step": {`update actors set properties=jsonb('{"C":{"m_PersistentScale":{"X":8.07,"Y":6.0,"Z":1.0}}}') where id=707`, "not a whole number of steps"},
	} {
		t.Run(name, func(t *testing.T) {
			o := solidoOps(t, blankDevice+c.sql)
			before, all := solidoState(t, o), everything(t, o)
			_, err := o.PreviewBaseRecycleWith(601, RecycleOptions{Solido: true})
			refusedWith(t, err, apierr.ErrConflict, c.want)
			if !strings.Contains(err.Error(), "Solido") || !strings.Contains(err.Error(), "Nothing was changed") {
				t.Fatalf("the refusal names the Solido: %v", err)
			}
			if solidoState(t, o) != before || everything(t, o) != all || len(o.S.Ops()) != 0 {
				t.Fatal("nothing changes")
			}
		})
	}
}

func TestADeepDesertBaseMakesItsSolidoTheSameWay(t *testing.T) {
	o := recycleOps(t, solidoWorld(blankDevice)+`update actors set map='DeepDesert' where id in (601,602,701,702,703,704,705,706,707);
update markers set map_name='DeepDesert' where marker_hash_id=111; update player_markers set map_name='DeepDesert' where marker_hash_id=111;`)
	r := recycleWithSolido(t, o)
	if s := r["solido"].(map[string]any); s["pieces"] != int64(3) || s["placeables"] != int64(6) || r["costPercent"] != int64(50) {
		t.Fatalf("%v", r)
	}
}

// What the conversion does with a building actor that has a yaw: a piece's place is turned by minus the actor's yaw, and its yaw is its own minus the actor's.
func TestTheConversionTurnsEveryPlaceByTheBuildingActorsYaw(t *testing.T) {
	q := func(deg float64) (float64, float64) { r := deg * math.Pi / 360; return math.Sin(r), math.Cos(r) } // z, w of a yaw
	oz, ow := q(90)
	pz, pw := q(-30) // the piece's world yaw is -30, so its yaw relative to the actor is -120
	rows, err := convertSolido(solidoSource{OriginX: 100, OriginY: 200, OriginZ: 10, OriginQZ: oz, OriginQW: ow, TotemID: 1,
		Pieces:     []solidoSrcPiece{{ID: 5, Type: "P", X: 100, Y: 300, Z: 15, QZ: pz, QW: pw, Flags: 1}, {ID: 4, Type: "Q", X: 90, Y: 200, Z: 10, QZ: oz, QW: ow, Flags: 2}},
		Placeables: []solidoSrcPlaceable{{ID: 1, Type: "Totem_Placeable", QW: 1}, {ID: 9, Type: "Table_Placeable", X: 100, Y: 200, Z: 40, QZ: oz, QW: ow}}})
	if err != nil {
		t.Fatal(err)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	// a point 100 along the actor's own forward (its yaw 90 makes that world +Y) is 100 along the actor's x; one 10 along world -X is 10 along its y... (-10, 0) turned by -90 is (0, 10)
	i0, i1 := rows.Instances[0], rows.Instances[1]
	if i0.Source != 4 || i1.Source != 5 || rows.Instances[0].ID != 0 || i1.ID != 1 {
		t.Fatalf("pieces are numbered by their own ids: %v", rows.Instances)
	}
	if !near(i1.X, 100) || !near(i1.Y, 0) || !near(i1.Z, 5) || !near(i1.Yaw, -120) || i1.ProvidesStability != 1 {
		t.Fatalf("piece: %+v", i1)
	}
	if !near(i0.X, 0) || !near(i0.Y, 10) || !near(i0.Yaw, 0) || i0.ProvidesStability != 0 {
		t.Fatalf("piece: %+v", i0)
	}
	if len(rows.Placeables) != 1 || rows.SkippedTotems != 1 || !near(rows.Placeables[0].Z, 30) || !near(rows.Placeables[0].Yaw, 0) {
		t.Fatalf("placeables: %+v", rows.Placeables)
	}
	// an angle comes out in (-180, 180]
	z, w := q(170)
	rows, err = convertSolido(solidoSource{OriginQW: 1, Pieces: []solidoSrcPiece{{ID: 1, Type: "P", QZ: z, QW: w}, {ID: 2, Type: "P", QZ: -z, QW: w}}})
	if err != nil || !near(rows.Instances[0].Yaw, 170) || !near(rows.Instances[1].Yaw, -170) {
		t.Fatalf("%v %+v", err, rows)
	}
	if _, err := convertSolido(solidoSource{OriginQW: 1}); err == nil {
		t.Fatal("a base with nothing in it has no Solido")
	}
}

func TestThePentashieldScaleIsTenTimesEachNumberToTheNearestStep(t *testing.T) {
	if sc, err := solidoScale([3]float64{8.000014, 6.000018, 1.0}, true); err != nil || sc != [3]int64{80, 60, 10} {
		t.Fatalf("%v %v", sc, err)
	}
	if sc, err := solidoScale([3]float64{1.999981, 2.0, 5.0}, true); err != nil || sc != [3]int64{20, 20, 50} {
		t.Fatalf("%v %v", sc, err)
	}
	for _, bad := range [][3]float64{{8.07, 1, 1}, {-1, 1, 1}, {1, 1, 1e6}, {math.NaN(), 1, 1}} {
		if _, err := solidoScale(bad, true); err == nil {
			t.Errorf("%v is not a scale", bad)
		}
	}
	if _, err := solidoScale([3]float64{}, false); err == nil {
		t.Error("no scale")
	}
}

func TestTheBlueprintRecordIsAddedAfterTheItemsOwnKeys(t *testing.T) {
	for in, want := range map[string]string{
		devBlank: devRecord(3),
		`{}`:     `{"FBuildingBlueprintItemStats":[[],{"PlayerBlueprintId":"!!bbp#3","PlayerBaseBackupId":{}}]}`,
	} {
		got, err := withBlueprintStats(in, 3)
		if err != nil || got != want {
			t.Errorf("%s:\n%s %v", in, got, err)
		}
	}
	for _, bad := range []string{``, `[]`, `null`, `{"a":1`, devFilled} {
		if _, err := withBlueprintStats(bad, 3); err == nil {
			t.Errorf("%q has no place for a record", bad)
		}
	}
}

// A Solido that is queued while the game runs is put back onto the file the game saved meanwhile only when the next blueprint number is still the one the edit worked out
// (the number is read through the guarded reader and written as given). The game made a Solido of its own in between: the queued one is dropped, and nothing is written.
func TestAQueuedSolidoIsDroppedWhenTheGameMadeABlueprintMeanwhile(t *testing.T) {
	gameDoes := func(o *Ops, sql string) {
		t.Helper()
		other, err := save.Open(o.S.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer other.Close()
		if _, err := other.Mutate("the game", func(m *save.Mut) error {
			_, err := m.Exec(sql)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := other.CommitForTests(); err != nil {
			t.Fatal(err)
		}
	}
	queue := func() *Ops {
		o := solidoOps(t, blankDevice)
		o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
		recycleWithSolido(t, o)
		if !o.S.Dirty() || rmCount(t, o, `building_blueprints where id=1`) != 1 {
			t.Fatal("the edit is queued, with the blueprint number 1")
		}
		return o
	}
	onDisk := func(o *Ops, q string) int64 {
		chk, err := save.Open(o.S.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer chk.Close()
		r, err := chk.One(`select count(*) n from ` + q)
		if err != nil {
			t.Fatal(err)
		}
		return r["n"].(int64)
	}
	untouched := func(o *Ops, blueprints int64) {
		t.Helper()
		if onDisk(o, `building_blueprints`) != blueprints || onDisk(o, `building_blueprints where item_id=8800`) != 0 || onDisk(o, `building_blueprint_instances`) != 0 || onDisk(o, `actors where id in (601,602,701)`) != 3 ||
			onDisk(o, `items where id=8800 and stats like '%FBuildingBlueprintItemStats%'`) != 0 {
			t.Fatal("nothing of the queued Solido or the recycle is in the file")
		}
	}
	dropped := func(o *Ops) {
		t.Helper()
		r, err := o.S.Rebase()
		if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 1 {
			t.Fatalf("the edit is reported, not forced: %+v %v", r, err)
		}
		if f := o.S.Flush(); f.Saved && o.S.Dirty() {
			t.Fatalf("%+v", f)
		}
	}

	// the game made a blueprint (its own number 1) for an unrelated item: the queued Solido would have been number 1 too
	o := queue()
	gameDoes(o, `insert into building_blueprints(id,item_id,player_id,building_blueprint_map) values (1,10,null,'')`)
	dropped(o)
	untouched(o, 1)

	// the game made a blueprint and threw it away: no row is left, but the table's counter has moved on, so the next number is not 1
	o = queue()
	gameDoes(o, `insert into building_blueprints(id,item_id,player_id,building_blueprint_map) values (5,10,null,''); delete from building_blueprints where id=5`)
	dropped(o)
	untouched(o, 0)

	// a game save that changed nothing the edit read puts it back, with the number it worked out
	o = queue()
	gameDoes(o, `update actors set location_x=location_x+1 where id=99`)
	if r, err := o.S.Rebase(); err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("an unrelated change: %+v %v", r, err)
	}
	if f := o.S.Flush(); !f.Saved {
		t.Fatalf("%+v", f)
	}
	if onDisk(o, `building_blueprints where id=1 and item_id=8800`) != 1 || onDisk(o, `actors where id in (601,602,701)`) != 0 {
		t.Fatal("the Solido and the recycle are in the file")
	}
}
