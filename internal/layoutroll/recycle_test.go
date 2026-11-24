package layoutroll

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// --recycle-bases (#1167, step 4), on made-up saves built from the real schema. Character 1 (controller 1, body 2) has a backpack (inventory 1, five slots, two used) and, in these
// worlds, an empty Reserve (inventory 12, type 33, no slot limit). Every base is a totem, a building actor with two foundations and a silo that stores one stack of CopperOre, with
// the building types of the game's own cost table.

// rcBase is the rows of one base n (1, 2, 3...). mp is its map, and owner the player that holds the owner rank over its totem.
func rcBase(n int, mp string, owner int) string {
	t := 1000 + n*10
	b, s := t+1, t+2
	et, eb, es, ec := 5764607523034230000+int64(n)*10, 5764607523034230001+int64(n)*10, 5764607523034230002+int64(n)*10, 5764607523034230003+int64(n)*10
	return fmt.Sprintf(`
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,state,serial,owner_account_id,properties) values
  (%[1]d,'/Game/BP_Totem.BP_Totem_C','%[3]s',%[10]d,6000,7,0,0,0,1,0,0,%[11]d,1,jsonb('{}')),
  (%[2]d,'/Game/BP_Building.BP_Building_C','%[3]s',%[10]d,6001,8,0,0,0,1,0,0,%[12]d,1,jsonb('{}')),
  (%[4]d,'/Game/BP_Silo.BP_Silo_C','%[3]s',%[13]d,6000,7,0,0,0,1,0,0,%[14]d,1,jsonb('{}'));
insert into fgl_entities(entity_id,components) values (%[5]d,jsonb('{"FTotemComponent":[16,{}]}')),(%[6]d,jsonb('{}')),(%[7]d,jsonb('{}')),(%[8]d,jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (%[1]d,%[5]d,'Actor'),(%[2]d,%[6]d,'Actor'),(%[4]d,%[7]d,'Actor'),(%[4]d,%[8]d,'ContainerInventory');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (%[1]d,'Base %[9]d',3,3,0,0),(%[4]d,'##SpiceSilo_Placeable',1,3,1,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (%[1]d,%[15]d,1),(%[4]d,%[15]d,1);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (%[16]d,%[4]d,4,10,250.0);
insert into actor_inventories(inventory_id,component_name_hash) values (%[16]d,%[17]d);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (%[18]d,%[16]d,300,0,'CopperOre',0,1790000001,'{"a":1}',0,null);
insert into buildings(id,owner_id) values (%[2]d,null);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,owner_entity_id,building_flags,health,shelter,sand_buildup,last_placed_by_player_id) values
  (%[2]d,1,'Sardaukar_Foundation',%[10]d,6000,7,0,0,0,1,%[5]d,0,100.0,0,0,1),(%[2]d,2,'Sardaukar_Foundation',%[19]d,6000,7,0,0,0,1,%[5]d,0,100.0,0,0,1);
insert into placeables(id,owner_entity_id,health,building_type,has_hit_ground,has_buildable_support,is_hologram,last_placed_by_player_id) values
  (%[1]d,%[5]d,1000.0,'Totem_Placeable',1,1,0,1),(%[4]d,%[5]d,50.0,'SpiceSilo_Placeable',1,1,0,1);
insert into totems(id,landclaim_vertical_level,last_backup_timestamp,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (%[1]d,0,0,%[10]d,6000,7,0);
`, t, b, mp, s, et, eb, es, ec, n, 5000+n*1000, 10+n*3, 11+n*3, 5010+n*1000, 12+n*3, owner, 7100+n, 100+n, 9100+n, 5020+n*1000)
}

const rcCommon = `insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (12,2,33,-1,0.0); update inventories set max_item_count=10 where id=1;`

// rcWorld is the common rows and the bases given.
func rcWorld(bases ...string) string { return rcCommon + strings.Join(bases, "\n") }

// ddBases are n bases of the player in the Deep Desert.
func ddBases(n int) []string {
	var out []string
	for i := 1; i <= n; i++ {
		out = append(out, rcBase(i, "DeepDesert", 1))
	}
	return out
}

const rcReserveItem = `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values (8000,12,7,1,'CopperOre',0,1790000000,'{}',0,null);`

// rcEnv is an env whose save holds the world, with the recycle switched on.
func rcEnv(t *testing.T, world string) (*env, Options) {
	t.Helper()
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, world).Path
	o := e.opts()
	o.RecycleBases = true
	return e, o
}

// editSave runs statements on the save file (the way a person would in the game): through the editor's own write path.
func editSave(t *testing.T, path string, stmts ...string) {
	t.Helper()
	s, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Mutate("test edit", func(m *save.Mut) error {
		for _, q := range stmts {
			if _, err := m.Exec(q); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if f := s.Flush(); !f.Saved {
		t.Fatalf("the test edit was not written: %+v", f)
	}
}

// count reads one number from the save file.
func count(t *testing.T, path, q string, args ...any) int64 {
	t.Helper()
	s, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.One(q, args...)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	switch n := r["n"].(type) {
	case int64:
		return n
	}
	t.Fatalf("%s: %v", q, r)
	return 0
}

// standingIDs are the totems of the player that stand in the Deep Desert.
func standingIDs(t *testing.T, path string) []int64 {
	t.Helper()
	s, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h, err := (&ops.Ops{S: s}).DeepDesertHolds()
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	for _, b := range h.Bases {
		out = append(out, b.ID)
	}
	return out
}


func lines(r Result) string { return strings.Join(r.Lines, "\n") }

func mustContain(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Fatalf("missing %q in:\n%s", p, text)
		}
	}
}

// withoutReserveRule shows the backpack and loadout room logic on its own: with the rule off, several bases fit one run.
func withoutReserveRule(t *testing.T) {
	t.Helper()
	old := enforceEmptyReserve
	enforceEmptyReserve = false
	t.Cleanup(func() { enforceEmptyReserve = old })
}

func swapVar[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

// Without the flag nothing changes: a standing base refuses the roll, as before.
func TestWithoutTheFlagAStandingBaseStillRefusesAndIsNotRecycled(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...))
	o.RecycleBases = false
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || !strings.Contains(err.Error(), "not changed") || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v %+v", err, r)
	}
	if got := standingIDs(t, e.save); len(got) != 1 {
		t.Fatalf("%v", got)
	}
}

// No base in the Deep Desert: the flag changes nothing, the roll goes on.
func TestTheFlagWithNoBaseRollsAsBefore(t *testing.T) {
	e, o := rcEnv(t, rcWorld(rcBase(1, "HaggaBasin", 1)))
	before := sha(t, e.save)
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, lines(r), "no base stands in the Deep Desert, so nothing is recycled")
	if sha(t, e.save) != before {
		t.Fatal("the save was written")
	}
}

// One base, empty Reserve: recycled with a Solido, written, read back, and the roll goes on. The stored items and the refund are in the Reserve, the Solido in the backpack.
func TestOneBaseIsRecycledWithItsSolidoAndTheRollGoesOn(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(1)...)+rcBase(9, "HaggaBasin", 1))
	r, err := Roll(o)
	if err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if e.ini() == iniWith("11") {
		t.Fatal("Game.ini was not rolled")
	}
	if got := standingIDs(t, e.save); len(got) != 0 {
		t.Fatalf("still standing: %v", got)
	}
	// the base on the other map is untouched
	if count(t, e.save, `select count(*) n from totems where id=1090`) != 1 || count(t, e.save, `select count(*) n from building_instances where building_id=1091`) != 2 {
		t.Fatal("the base on the other map was changed")
	}
	stt := readState(t, e.state)
	c := stt.LastCheck
	if c == nil || len(c.Recycled) != 1 || c.Checkpoint == "" || len(c.CheckpointSHA256) != 64 || len(c.ExportFiles) != 1 || c.ExportFolder == "" || stt.LastRoll == nil {
		t.Fatalf("%+v", c)
	}
	d := c.Recycled[0]
	if d.ID != 1010 || !d.SolidoNew || d.SolidoWhere != "backpack" || d.SolidoItem <= 0 || d.SolidoBlueprint <= 0 || d.StoredStacks != 1 || d.RefundStacks < 1 {
		t.Fatalf("%+v", d)
	}
	// the Solido is in the backpack and linked, the stored stack and the refund are in the Reserve
	if count(t, e.save, `select count(*) n from items where id=? and inventory_id=1 and json_extract(stats,'$.FBuildingBlueprintItemStats[1].PlayerBlueprintId')=?`, d.SolidoItem, fmt.Sprintf("!!bbp#%d", d.SolidoBlueprint)) != 1 {
		t.Fatal("the Solido device is not in the backpack with its blueprint")
	}
	if got := count(t, e.save, `select count(*) n from items where inventory_id=12`); got != d.StoredStacks+d.RefundStacks {
		t.Fatalf("the Reserve holds %d stacks, %d + %d expected", got, d.StoredStacks, d.RefundStacks)
	}
	if count(t, e.save, `select count(*) n from items where id=9101 and inventory_id=12 and stack_size=300`) != 1 {
		t.Fatal("the stored stack is not in the Reserve")
	}
	mustContain(t, lines(r), "Checkpoint of the save:", "Exported and read back:", "Recycled 1 base(s)", "Changed Game.ini")
	// the order of the run: the export folder exists before the base was gone (it holds the file of the base)
	if _, err := os.Stat(filepath.Join(c.ExportFolder, c.ExportFiles[0])); err != nil {
		t.Fatal(err)
	}
}

// Three bases and an empty Reserve: one base fits (each recycle fills the Reserve), the run stops before the roll and says what to do. A run again with the Reserve still full does
// nothing and says why; with the Reserve emptied it takes the next base; the roll comes only with the third base, in the run that recycles it.
func TestThreeBasesAreRecycledOneRunAtATimeAndTheRollWaitsForTheLast(t *testing.T) {
	e, o := rcEnv(t, rcWorld(ddBases(3)...))
	want := []int64{1020, 1030} // what stands after the first run

	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("it should stop before the roll: %v %v", err, r.Wrote)
	}
	mustContain(t, err.Error(), "1 of 3 base(s) recycled", "base 1020", "Reserve", "#1191", "the checkpoint of this run is", "NOTE: 1 base(s) WERE recycled", "Copy-Item")
	if got := standingIDs(t, e.save); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("standing %v, want %v", got, want)
	}
	stt := readState(t, e.state)
	if c := stt.LastCheck; c == nil || len(c.Recycled) != 1 || c.Recycled[0].ID != 1010 || len(c.ExportFiles) != 3 || stt.LastRoll != nil || !strings.Contains(stt.LastRun.Outcome, "1 of 3 bases recycled") {
		t.Fatalf("%+v %+v", stt.LastCheck, stt.LastRun)
	}

	// again, Reserve still full: nothing is done, nothing changes
	e.now = e.now.Add(time.Minute)
	before := sha(t, e.save)
	r, err = Roll(o)
	if err == nil || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "0 of 2 base(s) can be recycled now", "the Reserve is not empty", "#1191")

	// the Reserve emptied (in the game): the next base goes, the last one still stands
	editSave(t, e.save, `delete from items where inventory_id=12`)
	e.now = e.now.Add(time.Minute)
	r, err = Roll(o)
	if err == nil || e.ini() != iniWith("11") || fmt.Sprint(standingIDs(t, e.save)) != "[1030]" {
		t.Fatalf("%v %v", err, standingIDs(t, e.save))
	}
	mustContain(t, err.Error(), "1 of 2 base(s) recycled")

	// the last base: recycled, and the roll goes on in the same run
	editSave(t, e.save, `delete from items where inventory_id=12`)
	e.now = e.now.Add(time.Minute)
	r, err = Roll(o)
	if err != nil || !r.Wrote || e.ini() == iniWith("11") || len(standingIDs(t, e.save)) != 0 {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if readState(t, e.state).LastRoll == nil {
		t.Fatal("the roll is not recorded")
	}
}
