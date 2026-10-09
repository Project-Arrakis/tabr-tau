package ops

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

const schema = `
create table player_state(id integer primary key, account_id integer, character_name text);
create table actors(id integer primary key, class text, map text, location_x real, location_y real, location_z real, owner_account_id integer);
create table inventories(id integer primary key, actor_id integer, item_id integer, inventory_type integer, max_item_count integer, max_item_volume real);
create table items(id integer primary key, inventory_id integer, stack_size integer, position_index integer, template_id text, is_new integer, acquisition_time integer, stats text, quality_level integer);
create table items_id_sequencer(next_id integer);
create table journey_story_node(character_id integer, story_node_id text, has_pending_reward integer, complete_condition_state blob, reveal_condition_state blob, fail_condition_state blob, metadata_state blob, reset_group integer, primary key(character_id, story_node_id));
create table landsraad_tasks(id integer primary key, completed integer, winning_faction_id integer, completion_time integer);
create table landsraad_task_faction_contributions(faction_id integer, task_id integer, amount real, primary key(faction_id, task_id));
insert into player_state values (1, 1, 'Tester');
insert into actors values (2, '/Game/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C', 'HaggaBasin', 1, 2, 3, 1);
insert into inventories values (1, 2, null, 0, 3, 100);
insert into items values (10, 1, 100, 0, 'SolarisCoin', 0, 0, '{"FItemStackAndDurabilityStats":[[],{}]}', 0);
insert into items values (11, 1, 1, 1, 'Knife', 0, 0, '{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.0,"MaxDurability":100.0}]}', 0);
insert into items_id_sequencer values (500);
insert into journey_story_node values (1, 'Q', 0, x'0c', x'0c', x'0c', x'0c', 0), (1, 'Q.child', 0, x'0c', x'0c', x'0c', x'0c', 0);
insert into landsraad_tasks values (1, 0, null, null);
`

func newOps(t *testing.T) *Ops {
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
	r, err := o.GiveItem(Args{"template_id": "Spice", "quantity": 5.0, "quality": 2.0})
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
	r, err := o.GiveItems(Args{"items": []any{map[string]any{"template_id": "Spice", "quantity": 5.0, "quality": 2.0}}})
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

func TestRepairGear(t *testing.T) {
	o := newOps(t)
	r, _ := o.RepairGear()
	if r.(map[string]any)["repaired"].(int) != 1 {
		t.Fatalf("%v", r)
	}
	row, _ := o.S.One(`select stats from items where id=11`)
	cur, _ := durability(row["stats"].(string))
	if cur.(float64) != 100 {
		t.Fatalf("durability %v", cur)
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
