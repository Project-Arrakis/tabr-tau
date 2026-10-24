package ops

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

const schema = `
create table player_state(id integer primary key, account_id integer, character_name text, player_pawn_id integer);
create table actors(id integer primary key, class text, map text, location_x real, location_y real, location_z real, owner_account_id integer);
create table inventories(id integer primary key, actor_id integer, item_id integer, inventory_type integer, max_item_count integer, max_item_volume real);
create table items(id integer primary key, inventory_id integer, stack_size integer, position_index integer, template_id text, is_new integer, acquisition_time integer, stats text, quality_level integer);
create table items_id_sequencer(next_id integer);
create table journey_story_node(character_id integer, story_node_id text, has_pending_reward integer, complete_condition_state blob, reveal_condition_state blob, fail_condition_state blob, metadata_state blob, reset_group integer, primary key(character_id, story_node_id));
create table landsraad_tasks(id integer primary key, completed integer, winning_faction_id integer, completion_time integer);
create table landsraad_task_faction_contributions(faction_id integer, task_id integer, amount real, primary key(faction_id, task_id));
insert into player_state(id, account_id, character_name) values (1, 1, 'Tester');
insert into actors values (2, '/Game/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C', 'HaggaBasin', 1, 2, 3, 1);
insert into inventories values (1, 2, null, 0, 3, 100);
insert into items values (10, 1, 100, 0, 'SolarisCoin', 0, 0, '{"FItemStackAndDurabilityStats":[[],{}]}', 0);
insert into items values (11, 1, 1, 1, 'Knife', 0, 0, '{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.0,"MaxDurability":100.0}]}', 0);
insert into items_id_sequencer values (500);
insert into journey_story_node values (1, 'Q', 0, x'0c', x'0c', x'0c', x'0c', 0), (1, 'Q.child', 0, x'0c', x'0c', x'0c', x'0c', 0);
insert into landsraad_tasks values (1, 0, null, null);
`

// newOps opens the toy save. extra is SQL run on the file before the game's encoding is applied: damage the game itself wrote (a stored document that is not JSON), which an
// edit through the editor could no longer put there (the JSON check refuses it).
func newOps(t *testing.T, extra ...string) *Ops {
	t.Helper()
	dir := t.TempDir()
	plain := filepath.Join(dir, "p.sqlite")
	db, _ := sql.Open("sqlite", plain)
	// The toy tables below predate the real-schema fixtures (audit F-02). A save whose triggers differ from the
	// game's is deliberately read-only, so give the toy save the game's own trigger (and the table it hangs on).
	trigger := regexp.MustCompile(`(?s)CREATE TRIGGER actor_fgl_entities_cleanup_orphaned_entities.*?\nEND;`).FindString(save.KnownSchema)
	if trigger == "" {
		t.Fatal("game trigger not found in the known schema")
	}
	if _, err := db.Exec(schema + "create table actor_fgl_entities(actor_id integer, entity_id integer, slot_name text);" + trigger); err != nil {
		t.Fatal(err)
	}
	for _, q := range extra {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	raw, _ := os.ReadFile(plain)
	blob, _ := save.Encode(raw)
	p := filepath.Join(dir, "game.db")
	os.WriteFile(p, blob, 0o644)
	s, err := save.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return &Ops{S: s}
}

func TestGiveItemUsesSequencerAndFreeSlot(t *testing.T) {
	o := newOps(t)
	r, err := o.GiveItem(Args{"template_id": "MelangeSpice", "quantity": 5.0, "quality": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if r.(map[string]any)["itemId"].(int64) != 500 {
		t.Fatalf("expected id from sequencer, got %v", r)
	}
	row, _ := o.S.One(`select stack_size, quality_level, position_index from items where id=500`)
	if row["stack_size"].(int64) != 5 || row["quality_level"].(int64) != 2 || row["position_index"].(int64) != 2 {
		t.Fatalf("bad row %v", row)
	}
	seq, _ := o.S.One(`select next_id from items_id_sequencer`)
	if seq["next_id"].(int64) != 501 {
		t.Fatalf("sequencer not advanced: %v", seq)
	}
	if _, err := o.GiveItem(Args{"template_id": "More", "quantity": 1.0}); err == nil {
		t.Fatal("backpack has 3 slots and is now full")
	}
}

func TestGiveItemsQueueIsAtomicAndUsesDistinctSlots(t *testing.T) {
	o := newOps(t) // backpack of 3 slots, 2 used (slots 0 and 1 are taken, the sequencer starts at 500)
	r, err := o.GiveItems(Args{"items": []any{map[string]any{"template_id": "MelangeSpice", "quantity": 5.0, "quality": 2.0}}})
	if err != nil {
		t.Fatal(err)
	}
	if ids := r.(map[string]any)["itemIds"].([]int64); len(ids) != 1 || ids[0] != 500 {
		t.Fatalf("ids: %v", r)
	}
	// the backpack now has one free slot less than the queue needs: nothing from the queue may stay behind
	o2 := newOps(t)
	_, err = o2.GiveItems(Args{"items": []any{map[string]any{"template_id": "A", "quantity": 1.0}, map[string]any{"template_id": "B", "quantity": 1.0}}})
	if err == nil {
		t.Fatal("two items do not fit one free slot")
	}
	if n, _ := o2.S.One(`select count(*) c from items where template_id in ('A','B')`); n["c"].(int64) != 0 {
		t.Fatalf("a failed queue must leave nothing behind: %v", n)
	}
}

func TestGiveItemsRejectsBadQueues(t *testing.T) {
	o := newOps(t)
	big := make([]any, maxGiveBatch+1)
	for i := range big {
		big[i] = map[string]any{"template_id": "X", "quantity": 1.0}
	}
	for _, a := range []Args{{}, {"items": []any{}}, {"items": big}, {"items": []any{"Spice"}},
		{"items": []any{map[string]any{"template_id": "a b", "quantity": 1.0}}},
		{"items": []any{map[string]any{"template_id": "X", "quantity": 0.0}}},
		{"items": []any{map[string]any{"template_id": "X", "quantity": 1.0, "quality": 9.0}}}} {
		if _, err := o.GiveItems(a); err == nil {
			t.Fatalf("expected rejection for %v", a)
		}
	}
	if o.S.Dirty() {
		t.Fatal("rejected input must not dirty the save")
	}
}

func TestGiveItemRejectsBadInput(t *testing.T) {
	o := newOps(t)
	for _, a := range []Args{{"template_id": "a b", "quantity": 1.0}, {"template_id": "X", "quantity": 0.0}, {"template_id": "X", "quantity": 1.0, "quality": 9.0}, {"template_id": "x; drop table items;--", "quantity": 1.0}} {
		if _, err := o.GiveItem(a); err == nil {
			t.Fatalf("expected rejection for %v", a)
		}
	}
	if o.S.Dirty() {
		t.Fatal("rejected input must not dirty the save")
	}
}

func TestSolariAddRemove(t *testing.T) {
	o := newOps(t)
	if r, err := o.AddSolari(Args{"amount": 50.0}); err != nil || r.(map[string]any)["solari"].(int64) != 150 {
		t.Fatalf("%v %v", r, err)
	}
	if r, _ := o.AddSolari(Args{"amount": -150.0}); r.(map[string]any)["solari"].(int64) != 0 {
		t.Fatalf("%v", r)
	}
	if _, err := o.AddSolari(Args{"amount": -1.0}); err == nil {
		t.Fatal("cannot remove Solari the player does not have")
	}
	if _, err := o.AddSolari(Args{"amount": 0.0}); err == nil {
		t.Fatal("zero amount rejected")
	}
}

func TestRepairGearRepairsWornItemsAndLoadoutsOnly(t *testing.T) {
	dur := func(cur float64) string {
		return fmt.Sprintf(`{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":%.1f,"MaxDurability":100.0}]}`, cur)
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0),(23,2,15,8,1000.0),(25,2,30,50,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (201,21,1,0,'Helmet',0,1790000000,'`+dur(10)+`',0),(203,23,1,0,'Sword',0,1790000000,'`+dur(20)+`',0),
  (205,25,1,0,'Schematic',0,1790000000,'`+dur(30)+`',0),(206,1,1,5,'BackpackTool',0,1790000000,'`+dur(40)+`',0),
  (207,21,1,1,'Cloak',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":50.0,"MaxDurability":100.0}],"FAugmentedItemStats":[[],{"AppliedAugments":[{"Name":"X"}]}]}',0);`)}
	r, err := o.RepairGear()
	if err != nil {
		t.Fatal(err)
	}
	if r.(map[string]any)["repaired"].(int) != 3 {
		t.Fatalf("worn (201, 207) and loadout (203) items: %v", r)
	}
	cur := func(id int) float64 {
		row, _ := o.S.One(`select stats from items where id=?`, id)
		c, _ := durability(row["stats"].(string))
		return c.(float64)
	}
	if cur(201) != 100 || cur(203) != 100 || cur(207) != 100 {
		t.Fatalf("worn and loadout items are at full durability: %v %v %v", cur(201), cur(203), cur(207))
	}
	if cur(205) != 30 || cur(206) != 40 {
		t.Fatalf("items outside types 1 and 15 are left alone: %v %v", cur(205), cur(206))
	}
	row, _ := o.S.One(`select stats from items where id=207`)
	if !strings.Contains(row["stats"].(string), "AppliedAugments") {
		t.Fatalf("a repair must not lose the item's other stats: %v", row["stats"])
	}
}

// A repair makes an item as good as new: full durability, and a decayed maximum that workbench repairs wore down goes back to the full one.
// An item with no maximum recorded is full at 100, and a decayed maximum of 0 (none recorded) is left as it is. (An item already above its
// full durability is not lowered: see the first test.)
func TestRepairGearRestoresTheDecayedMaximum(t *testing.T) {
	st := func(body string) string {
		return `{"FItemStackAndDurabilityStats":[[],{` + body + `}]}`
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (301,21,1,0,'Engine',0,1790000000,'`+st(`"CurrentDurability":29.769889,"DecayedMaxDurability":79.668724`)+`',0),
  (302,21,1,1,'Light',0,1790000000,'`+st(`"CurrentDurability":99.9,"DecayedMaxDurability":99.6`)+`',0),
  (303,21,1,2,'Tool',0,1790000000,'`+st(`"CurrentDurability":97.2,"MaxDurability":101.0,"DecayedMaxDurability":69.388015`)+`',0),
  (304,21,1,3,'Plain',0,1790000000,'`+st(`"CurrentDurability":12.5`)+`',0),
  (305,21,1,4,'Zero',0,1790000000,'`+st(`"CurrentDurability":40.0,"DecayedMaxDurability":0.0`)+`',0);`)}
	r, err := o.RepairGear()
	if err != nil {
		t.Fatal(err)
	}
	cur := func(id int) float64 {
		row, _ := o.S.One(`select stats from items where id=?`, id)
		c, _ := durability(row["stats"].(string))
		return c.(float64)
	}
	if got := r.(map[string]any)["repaired"].(int); got != 5 {
		t.Fatalf("every item was below its full durability or had a worn-down maximum: %v", r)
	}
	if cur(301) != 100 || cur(302) != 100 || cur(303) != 101 || cur(304) != 100 || cur(305) != 100 {
		t.Fatalf("durability wrong: %v %v %v %v %v", cur(301), cur(302), cur(303), cur(304), cur(305))
	}
	decayed := func(id int) any {
		row, _ := o.S.One(`select json_extract(stats,'$.FItemStackAndDurabilityStats[1].DecayedMaxDurability') d from items where id=?`, id)
		return row["d"]
	}
	// the worn-down maximums go back to full, an item that never recorded one gets none, and 0 stays 0
	if got := fmt.Sprint(decayed(301), decayed(302), decayed(303), decayed(304), decayed(305)); got != "100 100 101 <nil> 0" {
		t.Fatalf("decayed maximums wrong: %v", got)
	}
}

func TestJourneyCompleteCascadesAndResets(t *testing.T) {
	o := newOps(t)
	o.JourneySet(Args{"node_id": "Q", "complete": true})
	rows, _ := o.S.Query(`select count(*) c from journey_story_node where complete_condition_state=x'01'`)
	if rows[0]["c"].(int64) != 2 {
		t.Fatalf("expected node and child complete: %v", rows)
	}
	o.JourneySet(Args{"node_id": "Q", "complete": false})
	rows, _ = o.S.Query(`select count(*) c from journey_story_node where complete_condition_state=x'01'`)
	if rows[0]["c"].(int64) != 0 {
		t.Fatalf("reset failed: %v", rows)
	}
	if _, err := o.JourneySet(Args{"node_id": "bad node!"}); err == nil {
		t.Fatal("invalid id must be rejected")
	}
}

func TestLandsraadTask(t *testing.T) {
	o := newOps(t)
	if _, err := o.SetTaskProgress(Args{"task_id": 1.0, "faction_id": 2.0, "amount": 4000.0}); err != nil {
		t.Fatal(err)
	}
	o.SetTaskProgress(Args{"task_id": 1.0, "faction_id": 2.0, "amount": 10.0}) // upsert
	r, _ := o.S.One(`select count(*) c, max(amount) a from landsraad_task_faction_contributions`)
	if r["c"].(int64) != 1 || r["a"].(float64) != 10 {
		t.Fatalf("%v", r)
	}
	o.CompleteTask(Args{"task_id": 1.0, "faction_id": 2.0})
	r, _ = o.S.One(`select completed, winning_faction_id from landsraad_tasks`)
	if r["completed"].(int64) != 1 || r["winning_faction_id"].(int64) != 2 {
		t.Fatalf("%v", r)
	}
	o.CompleteTask(Args{"task_id": 1.0, "reset": true})
	if r, _ = o.S.One(`select completed from landsraad_tasks`); r["completed"].(int64) != 0 {
		t.Fatal("reopen failed")
	}
}

func TestSQLModes(t *testing.T) {
	o := newOps(t)
	if _, err := o.SQL("delete from items"); err == nil {
		t.Fatal("read-only SQL must refuse writes")
	}
	if _, err := o.ExecSQL("attach database 'x.db' as x"); err == nil {
		t.Fatal("ATTACH must be blocked")
	}
	r, err := o.ExecSQL("update items set stack_size=stack_size+1 where id=10; update items set stack_size=stack_size+1 where id=11")
	if err != nil || r.(map[string]any)["changes"].(int64) != 2 || !o.S.Dirty() {
		t.Fatalf("%v %v", r, err)
	}
	if _, err := o.ExecSQL("update items set nope=1"); err == nil {
		t.Fatal("bad SQL must error")
	}
	if _, err := o.UpdateRow(Args{"table": "items; drop table items", "column": "a", "rowid": 1.0}); err == nil {
		t.Fatal("identifier injection must be rejected")
	}
}

// Free text is not an item (UX-02 of the 2026-10-15 audit): typing "Spice" gave an item no game has. An id must be in the catalog or already
// on an item in the save; everything else is refused with a sentence that says what to do, and nothing is written.
func TestGiveRefusesAnIdThatIsNotAnItem(t *testing.T) {
	o := newOps(t)
	for _, bad := range []string{"Spice", "spice melange", "NotAnItem_99", "melangespice"} { // wrong name, a name with a space, a typo, wrong case
		if _, err := o.GiveItem(Args{"template_id": bad, "quantity": 1.0}); err == nil || !strings.Contains(err.Error(), "pick an item from the list") {
			t.Errorf("%q must be refused with advice: %v", bad, err)
		}
		if _, err := o.GiveItems(Args{"items": []any{map[string]any{"template_id": bad, "quantity": 1.0}}}); err == nil {
			t.Errorf("%q must be refused in the queue as well", bad)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused give must not change the save")
	}
	if _, err := o.GiveItem(Args{"template_id": "MelangeSpice", "quantity": 1.0}); err != nil {
		t.Fatalf("a catalog id is fine: %v", err)
	}
}

// An id the catalog lacks but the save already holds (a new game item) can still be given.
func TestGiveAcceptsAnIdAlreadyInTheSave(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (901,2,30,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (9001,901,1,0,'BrandNewGameItem_X',0,1790000000,'{}',0);`)}
	if _, err := o.GiveItem(Args{"template_id": "BrandNewGameItem_X", "quantity": 1.0}); err != nil {
		t.Fatalf("an id that is on an item in the save must be accepted: %v", err)
	}
	if _, err := o.GiveItem(Args{"template_id": "BrandNewGameItem_Y", "quantity": 1.0}); err == nil {
		t.Fatal("an id that is neither in the catalog nor in the save must be refused")
	}
}

// A slot holds no more than the item's stack size (the game's tables: Spice-infused Fuel Cell 500). Giving 1,200 makes three stacks; a stack already there is filled first.
func TestGiveHonorsTheStackSize(t *testing.T) {
	o := newOps(t) // backpack of 3 slots, 2 used
	if _, err := o.S.Mutate("seed", func(m *save.Mut) error {
		_, err := m.Exec(`update inventories set max_item_count=10, max_item_volume=0 where id=1`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.GiveItem(Args{"template_id": "SpicedFuelCell", "quantity": 120.0}); err != nil {
		t.Fatal(err)
	}
	r, err := o.GiveItem(Args{"template_id": "SpicedFuelCell", "quantity": 1200.0})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := o.S.Query(`select stack_size from items where template_id='SpicedFuelCell' order by stack_size`)
	var sizes []int64
	for _, row := range rows {
		sizes = append(sizes, row["stack_size"].(int64))
	}
	if fmt.Sprint(sizes) != "[320 500 500]" { // 120 + 1200 = 1320: the first stack was topped up to 500, then two more
		t.Fatalf("stacks %v", sizes)
	}
	if r.(map[string]any)["stacks"].(int) != 2 {
		t.Fatalf("two new stacks: %v", r)
	}
	// one that does not fit the slots left gives what fits (it used to be refused whole): 4,000 needs 8 stacks, 5 slots are free, the 320 stack is filled (180) and 5 new stacks fill the slots, so 2,680 go in
	r, err = o.GiveItem(Args{"template_id": "SpicedFuelCell", "quantity": 4000.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["given"] != int64(2680) || m["requested"] != int64(4000) || m["limit"] != "slots" {
		t.Fatalf("the stack topped up and 5 new ones: %v", m)
	}
	if n, _ := o.S.One(`select sum(stack_size) s from items where template_id='SpicedFuelCell'`); n["s"].(int64) != 4000 {
		t.Fatalf("1,320 + 2,680: %v", n)
	}
}

func TestAStackEditCannotPassTheStackSize(t *testing.T) {
	o := newOps(t)
	r, err := o.GiveItem(Args{"template_id": "SpicedFuelCell", "quantity": 5.0})
	if err != nil {
		t.Fatal(err)
	}
	id := float64(r.(map[string]any)["itemId"].(int64))
	if _, err := o.SetItem(Args{"id": id, "stack_size": 5000.0}); err == nil || !strings.Contains(err.Error(), "stacks to 500") {
		t.Fatalf("a stack of 5,000 must be refused: %v", err)
	}
	if _, err := o.SetItem(Args{"id": id, "stack_size": 500.0}); err != nil {
		t.Fatal(err)
	}
}
