package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The character's inventories as in a real save: backpack (0, the fixture's), worn gear (1), emotes (14), loadout (15),
// contract items (29) and unique schematics (30). Only 0, 1, 15 and 30 are the console's player inventory.
const inventoryWorld = `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values
  (21,2,1,10,1000.0),(22,2,14,100,1000.0),(23,2,15,8,1000.0),(24,2,29,-1,1000.0),(25,2,30,500,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (101,21,1,0,'Helmet_06',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":50.0,"MaxDurability":100.0}],"FAugmentedItemStats":[[],{"AppliedAugmentQualities":[5,3],"AppliedAugments":[{"Name":"T6_Augment_Armor16"},{"Name":"T6_Augment_Armor10"}]}]}',5),
  (102,22,1,0,'Emote_Wave',0,1790000000,'{}',0),
  (103,23,1,0,'Knife',0,1790000000,'{}',0),
  (104,24,1,0,'ContractItem_X',0,1790000000,'{}',0),
  (105,25,1,0,'Schematic_Y',0,1790000000,'{}',0);`

func TestInventoryIsTheConsolesFourGroups(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, inventoryWorld)}
	r, err := o.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	items := r.(map[string]any)["items"].([]map[string]any)
	got := map[int64]string{}
	for _, it := range items {
		got[it["id"].(int64)] = it["inventory_name"].(string)
	}
	// the fixture's own backpack holds 10 (Solari) and 11 (Knife); 102 (emote) and 104 (contract item) are not shown
	want := map[int64]string{10: "Backpack", 11: "Backpack", 101: "Character", 103: "Loadout", 105: "Unique schematics"}
	if len(got) != len(want) {
		t.Fatalf("items shown: %v, want %v", got, want)
	}
	for id, name := range want {
		if got[id] != name {
			t.Errorf("item %d is in %q, want %q", id, got[id], name)
		}
	}
	for _, inv := range r.(map[string]any)["inventories"].([]map[string]any) {
		switch inv["inventory_type"].(int64) {
		case 0, 1, 15, 30:
		default:
			t.Errorf("inventory type %v must not be listed", inv["inventory_type"])
		}
	}
}

func TestInventoryShowsAppliedAugmentsWithTheirQuality(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, inventoryWorld)}
	r, err := o.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range r.(map[string]any)["items"].([]map[string]any) {
		switch it["id"].(int64) {
		case 101:
			a := it["augments"].([]map[string]any)
			if len(a) != 2 || a[0]["name"] != "T6_Augment_Armor16" || a[0]["quality"].(float64) != 5 || a[1]["name"] != "T6_Augment_Armor10" || a[1]["quality"].(float64) != 3 {
				t.Fatalf("augments: %v", a)
			}
			if it["durability"].(float64) != 50 {
				t.Errorf("durability still read: %v", it["durability"])
			}
		case 11:
			if a, _ := it["augments"].([]map[string]any); len(a) != 0 {
				t.Errorf("an item without augments lists none: %v", a)
			}
		}
	}
}
