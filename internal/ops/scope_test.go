package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The routes that edit an item or give into an inventory by id only reach what the page offers (ARCH-9 of the 2026-10-15 audit):
// the player's carried inventories and the containers of the player's bases. World loot, a container outside a base, and an
// inventory of another actor are refused, and a refusal changes nothing.
const scopeWorld = storageWorld + `
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (9100,7008,4,0,'CopperOre',0,1790000000,'{}',0),
  (9101,7006,4,0,'CopperOre',0,1790000000,'{}',0),
  (9102,7004,4,0,'Oil',0,1790000000,'{}',0);`

func scopeOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, scopeWorld)} }

func TestItemEditsReachOnlyCarriedAndBaseInventories(t *testing.T) {
	o := scopeOps(t)
	pack := mustOne(t, o, `select i.id id from items i join inventories v on v.id=i.inventory_id join actors a on a.id=v.actor_id where v.inventory_type=0 limit 1`)
	for name, id := range map[string]int64{"an item in the player's backpack": pack["id"].(int64), "an item in a base container": 9001, "an item in a base refinery": 9003} {
		if _, err := o.SetItem(Args{"id": float64(id), "stack_size": 7.0}); err != nil {
			t.Errorf("%s must be editable: %v", name, err)
		}
	}
	o.S.Discard()
	for name, id := range map[string]int64{"world loot": 9100, "a container outside any base": 9101, "a fuel slot of a generator": 9102} {
		if _, err := o.SetItem(Args{"id": float64(id), "stack_size": 7.0}); err == nil || !strings.Contains(err.Error(), "not in your") {
			t.Errorf("an edit of %s must be refused: %v", name, err)
		}
		if _, err := o.DeleteItem(Args{"id": float64(id)}); err == nil || !strings.Contains(err.Error(), "not in your") {
			t.Errorf("a delete of %s must be refused: %v", name, err)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused edit must not change the save")
	}
	if _, err := o.SetItem(Args{"id": 424242.0, "stack_size": 7.0}); err == nil || !strings.Contains(err.Error(), "item not found") {
		t.Errorf("an id that is no item says so: %v", err)
	}
	if _, err := o.DeleteItem(Args{"id": 9001.0}); err != nil {
		t.Errorf("an item in a base container can be deleted: %v", err)
	}
}

func TestGiveToInventoryReachesOnlyBaseContainers(t *testing.T) {
	o := scopeOps(t)
	if _, err := o.GiveToInventory(Args{"inventory_id": 7001.0, "template_id": "MelangeSpice", "quantity": 1.0}); err != nil {
		t.Fatalf("a container of the base takes an item: %v", err)
	}
	o.S.Discard()
	backpack := float64(mustOne(t, o, `select v.id id from inventories v where v.inventory_type=0 limit 1`)["id"].(int64))
	for name, inv := range map[string]float64{"world loot": 7008, "a container outside any base": 7006, "a generator's fuel slot": 7004, "the backpack (it has its own route)": backpack, "no inventory at all": 31337} {
		if _, err := o.GiveToInventory(Args{"inventory_id": inv, "template_id": "MelangeSpice", "quantity": 1.0}); err == nil || !strings.Contains(err.Error(), "not a container at one of your bases") {
			t.Errorf("a give into %s must be refused: %v", name, err)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused give must not change the save")
	}
}

// Every table the Tables view locks is locked for a SQL write too: one list drives both (ARCH-7).
func TestSQLWriteLocksEveryLockedTable(t *testing.T) {
	for name := range lockedTables {
		if err := vetWrite("update " + name + " set a=1"); err == nil {
			t.Errorf("a SQL write to %s must be refused", name)
		}
		if err := vetWrite(`delete from "` + name + `"`); err == nil {
			t.Errorf("a quoted SQL write to %s must be refused", name)
		}
	}
	if err := vetWrite("update items set stack_size=1 where id=1"); err != nil {
		t.Errorf("an ordinary table stays writable: %v", err)
	}
}
