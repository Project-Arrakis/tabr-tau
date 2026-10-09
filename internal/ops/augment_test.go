package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/augments"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

const helmet = "Combat_Heavy_Unique_Reinforced_Helmet_06" // a Bulwark Helmet: clothing, two augment slots

// The player's backpack (inventory 1, 5 slots, two used) gets a plain helmet and a helmet that already carries a grade-3 augment,
// another actor has a helmet of their own, and the save's keystone map has the augment-slot keystones.
const augmentWorld = `
insert into actors(id,class,map,location_x,location_y,location_z) values (9,'/Game/Other.Other_C','HaggaBasin',0,0,0);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (31,9,0,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (301,1,1,2,'` + helmet + `',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":50.0}]}',5),
  (302,1,1,3,'` + helmet + `',0,1790000000,'{"FAugmentedItemStats":[[],{"AppliedAugmentQualities":[3],"AppliedAugmentRollData":[{"AppliedEffectIndices":[],"StatRolls":[1,1]}],"AppliedAugments":[{"Name":"T6_Augment_Armor10"}]}],"FItemStackAndDurabilityStats":[[],{"CurrentDurability":50.0}]}',5),
  (303,31,1,0,'` + helmet + `',0,1790000000,'{}',0);
insert into specialization_keystones_map(id,name) values
  (42,'Crafting_CraftingKeystone_ArmoirAugmentSlots10'),(43,'Crafting_CraftingKeystone_ArmoirAugmentSlots42'),
  (44,'Crafting_CraftingKeystone_MeleeWeaponAugmentSlots3'),(47,'Crafting_CraftingKeystone_RangedWeaponAugmentSlots1'),
  (41,'Combat_CombatKeystone_Hat');`

func statsOf(t *testing.T, o *Ops, id int64) string {
	t.Helper()
	r, _ := o.S.One(`select stats from items where id=?`, id)
	return r["stats"].(string)
}

func TestAugmentItemWritesTheGamesShapeAndUnlocksOnlyTheMatchingSlots(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	r, err := o.AugmentItem(Args{"item_id": 301.0, "augments": []any{"T6_Augment_Armor16", "T6_Augment_Armor10"}, "grade": 5.0, "unlock_slots": true})
	if err != nil {
		t.Fatal(err)
	}
	if r.(map[string]any)["slotsUnlocked"].(int64) != 2 {
		t.Fatalf("clothing opens the two armour slots only: %v", r)
	}
	want := `{"FAugmentedItemStats":[[],{"AppliedAugmentQualities":[5,5],"AppliedAugmentRollData":[{"AppliedEffectIndices":[],"StatRolls":[1,1,1,1]},{"AppliedEffectIndices":[],"StatRolls":[1,1]}],"AppliedAugments":[{"Name":"T6_Augment_Armor16"},{"Name":"T6_Augment_Armor10"}]}],"FItemStackAndDurabilityStats":[[],{"CurrentDurability":50.0}]}`
	if got := statsOf(t, o, 301); got != want {
		t.Fatalf("stats:\n got %s\nwant %s", got, want)
	}
	ks, _ := o.S.Query(`select keystone_id from purchased_specialization_keystones order by 1`)
	if len(ks) != 2 || ks[0]["keystone_id"].(int64) != 42 || ks[1]["keystone_id"].(int64) != 43 {
		t.Fatalf("keystones: %v", ks)
	}
	// asking again changes nothing and unlocks nothing new
	r, err = o.AugmentItem(Args{"item_id": 301.0, "augments": []any{"T6_Augment_Armor16", "T6_Augment_Armor10"}, "grade": 5.0, "unlock_slots": true})
	if err != nil || r.(map[string]any)["slotsUnlocked"].(int64) != 0 {
		t.Fatalf("%v %v", r, err)
	}
}

func TestAugmentItemWithoutUnlockLeavesKeystonesAlone(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	if _, err := o.AugmentItem(Args{"item_id": 301.0, "augments": []any{"T6_Augment_Armor10"}}); err != nil {
		t.Fatal(err)
	}
	if n, _ := o.S.One(`select count(*) c from purchased_specialization_keystones`); n["c"].(int64) != 0 {
		t.Fatalf("keystones must only change when asked: %v", n)
	}
	if !strings.Contains(statsOf(t, o, 301), `"AppliedAugmentQualities":[1]`) {
		t.Fatalf("grade defaults to 1: %s", statsOf(t, o, 301))
	}
}

func TestAugmentItemKeepsStoredGradeOfAugmentsAlreadyApplied(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	if _, err := o.AugmentItem(Args{"item_id": 302.0, "augments": []any{"T6_Augment_Armor10", "T6_Augment_Armor16"}, "grade": 5.0}); err != nil {
		t.Fatal(err)
	}
	if got := statsOf(t, o, 302); !strings.Contains(got, `"AppliedAugmentQualities":[3,5]`) {
		t.Fatalf("the augment already on the item keeps its grade 3, the new one gets 5: %s", got)
	}
	if _, err := o.AugmentItem(Args{"item_id": 302.0, "augments": []any{}}); err != nil {
		t.Fatal(err)
	}
	if got := statsOf(t, o, 302); strings.Contains(got, "FAugmentedItemStats") || !strings.Contains(got, "CurrentDurability") {
		t.Fatalf("an empty list removes the augments and nothing else: %s", got)
	}
}

func TestAugmentItemRefusesWhatDoesNotFit(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	for name, a := range map[string]Args{
		"a ranged-weapon augment on a helmet": {"item_id": 301.0, "augments": []any{"T6_Augment_Acuracy1"}},
		"three on clothing":                   {"item_id": 301.0, "augments": []any{"T6_Augment_Armor16", "T6_Augment_Armor10", "T6_Augment_Armor11"}},
		"an unknown augment":                  {"item_id": 301.0, "augments": []any{"Nope"}},
		"a bad grade":                         {"item_id": 301.0, "augments": []any{"T6_Augment_Armor10"}, "grade": 6.0},
		"another actor's item":                {"item_id": 303.0, "augments": []any{"T6_Augment_Armor10"}},
		"a missing item":                      {"item_id": 999.0, "augments": []any{"T6_Augment_Armor10"}},
		"a currency stack":                    {"item_id": 10.0, "augments": []any{"T6_Augment_Armor10"}},
		"a non-list":                          {"item_id": 301.0, "augments": "T6_Augment_Armor10"},
	} {
		if _, err := o.AugmentItem(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if o.S.Dirty() {
		t.Fatal("refused requests must not change the save")
	}
}

func TestAugmentOptionsForAnItemAndATemplate(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	r, err := o.AugmentOptions(Args{"item_id": 302.0})
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["limit"].(int) != 2 || m["kind"] != "clothing" || len(m["options"].([]augments.Option)) == 0 {
		t.Fatalf("%v", m)
	}
	ap := m["applied"].([]map[string]any)
	if len(ap) != 1 || ap[0]["id"] != "T6_Augment_Armor10" || ap[0]["quality"] != "3" {
		t.Fatalf("applied: %v", ap)
	}
	r, _ = o.AugmentOptions(Args{"template_id": "SolarisCoin"})
	if r.(map[string]any)["limit"].(int) != 0 {
		t.Fatalf("currency takes no augments: %v", r)
	}
	if _, err := o.AugmentOptions(Args{"item_id": 303.0}); err == nil {
		t.Fatal("another actor's item must not be inspected")
	}
}

func TestGiveItemsCanCarryAugmentsAndAPlainGiveNeverInheritsThem(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	r, err := o.GiveItems(Args{"items": []any{map[string]any{"template_id": helmet, "quantity": 1.0, "quality": 5.0, "augments": []any{"T6_Augment_Armor16"}, "grade": 4.0}}})
	if err != nil {
		t.Fatal(err)
	}
	id := r.(map[string]any)["itemIds"].([]int64)[0]
	if got := statsOf(t, o, id); !strings.Contains(got, `"AppliedAugmentQualities":[4]`) || !strings.Contains(got, "T6_Augment_Armor16") {
		t.Fatalf("given with its augment: %s", got)
	}
	// a plain give of the same template copies stats from an existing helmet, which must not bring that helmet's augments along
	o2 := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	r, err = o2.GiveItem(Args{"template_id": helmet, "quantity": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if got := statsOf(t, o2, r.(map[string]any)["itemId"].(int64)); strings.Contains(got, "FAugmentedItemStats") {
		t.Fatalf("a plain give carried another item's augments: %s", got)
	}
	// a queue line with an augment that does not fit stops the whole queue
	o3 := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	if _, err := o3.GiveItems(Args{"items": []any{map[string]any{"template_id": "SolarisCoin", "quantity": 1.0}, map[string]any{"template_id": helmet, "quantity": 1.0, "augments": []any{"T6_Augment_Acuracy1"}}}}); err == nil {
		t.Fatal("a bad augment must refuse the queue")
	}
	if o3.S.Dirty() {
		t.Fatal("nothing from a refused queue may be written")
	}
}

func TestInventoryRowsSayWhetherTheyTakeAugments(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, augmentWorld)}
	r, _ := o.Inventory()
	got := map[int64]int{}
	for _, it := range r.(map[string]any)["items"].([]map[string]any) {
		got[it["id"].(int64)] = it["aug_limit"].(int)
	}
	if got[301] != 2 || got[10] != 0 {
		t.Fatalf("aug_limit: %v", got)
	}
}
