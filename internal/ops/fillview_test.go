package ops

import (
	"strconv"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func TestFillOf(t *testing.T) {
	water := func(kind string, n float64) string {
		return `{"FFillableItemStats":[[],{"CurrentAmount":` + ftoa(n) + `,"FillableType":"` + kind + `"}]}`
	}
	cases := []struct {
		name, tpl, stats string
		nilWant          bool
		kind, state      string
		amount, capacity float64
		percent          float64
	}{
		{name: "partial Literjon Mk6", tpl: "Literjon_T6", stats: water("Water", 11000), kind: "Water", state: "partial", amount: 11000, capacity: 20000, percent: 55},
		{name: "full Literjon", tpl: "Literjon", stats: water("Water", 1000), kind: "Water", state: "full", amount: 1000, capacity: 1000, percent: 100},
		{name: "over capacity", tpl: "Literjon", stats: water("Water", 1500), kind: "Water", state: "over", amount: 1500, capacity: 1000, percent: 150},
		{name: "empty object, known water template", tpl: "Decajon", stats: `{"FFillableItemStats":[[],{}]}`, kind: "Water", state: "empty", capacity: 10000, percent: 0},
		{name: "no key, known water template", tpl: "Decajon", stats: `{}`, kind: "Water", state: "empty", capacity: 10000, percent: 0},
		{name: "partial blood sack", tpl: "Bloodsack_Unique_Durable_06", stats: water("Blood", 12000), kind: "Blood", state: "partial", amount: 12000, capacity: 30000, percent: 40},
		{name: "full blood sack", tpl: "Bloodsack_01", stats: water("Blood", 3000), kind: "Blood", state: "full", amount: 3000, capacity: 3000, percent: 100},
		{name: "partial Bloodsack_02 (capacity 9000 from the game's data, #472)", tpl: "Bloodsack_02", stats: water("Blood", 1466), kind: "Blood", state: "partial", amount: 1466, capacity: 9000, percent: 16},
		{name: "empty Bloodsack_02 with no key is recognised as blood", tpl: "Bloodsack_02", stats: `{}`, kind: "Blood", state: "empty", capacity: 9000, percent: 0},
		{name: "full Bloodsack_T6", tpl: "Bloodsack_T6", stats: water("Blood", 24000), kind: "Blood", state: "full", amount: 24000, capacity: 24000, percent: 100},
		{name: "blood sack of unknown template holding blood: no capacity guessed", tpl: "Bloodsack_99", stats: water("Blood", 500), kind: "Blood", state: "unknown", amount: 500},
		{name: "empty blood sack of unknown template with no key", tpl: "Bloodsack_99", stats: `{}`, kind: "Blood", state: "empty"},
		{name: "unknown template with a water record", tpl: "SomeNewStillsuit", stats: water("Water", 700), kind: "Water", state: "unknown", amount: 700},
		{name: "a known water container holding blood has no known capacity", tpl: "Literjon", stats: water("Blood", 200), kind: "Blood", state: "unknown", amount: 200},
		{name: "empty record of an unknown template", tpl: "SomeNewStillsuit", stats: `{"FFillableItemStats":[[],{}]}`, kind: "", state: "empty"},
		{name: "not a container", tpl: "Knife", stats: `{"FItemStackAndDurabilityStats":[[],{}]}`, nilWant: true},
		{name: "unreadable stats on a plain item", tpl: "Knife", stats: `<nil>`, nilWant: true},
		{name: "negative amount counts as empty", tpl: "Literjon", stats: water("Water", -5), kind: "Water", state: "empty", capacity: 1000, percent: 0},
	}
	for _, c := range cases {
		got := fillOf(c.tpl, c.stats)
		if c.nilWant {
			if got != nil {
				t.Errorf("%s: want no fill, got %v", c.name, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("%s: want a fill, got none", c.name)
			continue
		}
		if got["kind"] != c.kind || got["state"] != c.state || got["amount"].(float64) != c.amount {
			t.Errorf("%s: got %v", c.name, got)
		}
		cp, hasCap := got["capacity"].(float64)
		if c.capacity == 0 {
			if hasCap || got["percent"] != nil {
				t.Errorf("%s: capacity must not be guessed: %v", c.name, got)
			}
		} else if !hasCap || cp != c.capacity || got["percent"].(float64) != c.percent {
			t.Errorf("%s: capacity/percent: %v", c.name, got)
		}
	}
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

const fillWorld = `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (201,21,1,0,'Literjon_T6',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":11000.0,"FillableType":"Water"}],"FItemStackAndDurabilityStats":[[],{"CurrentDurability":5.0}]}',1),
  (202,21,1,1,'Bloodsack_01',0,1790000000,'{}',1),
  (203,21,1,2,'Knife',0,1790000000,'{}',0);`

func TestInventoryShowsTheFillLevelOfLiquidContainers(t *testing.T) {
	s := testsave.PlayerWithSQL(t, fillWorld)
	before, _ := s.One(`select stats from items where id=201`)
	o := &Ops{S: s}
	r, err := o.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]map[string]any{}
	for _, it := range r.(map[string]any)["items"].([]map[string]any) {
		f, _ := it["fill"].(map[string]any)
		seen[it["id"].(int64)] = f
		if _, leaked := it["stats"]; leaked {
			t.Errorf("item %v still carries its stats", it["id"])
		}
	}
	if f := seen[201]; f == nil || f["state"] != "partial" || f["percent"].(float64) != 55 || f["capacity"].(float64) != 20000 {
		t.Errorf("Literjon Mk6: %v", f)
	}
	if f := seen[202]; f == nil || f["state"] != "empty" || f["kind"] != "Blood" {
		t.Errorf("empty blood sack: %v", f)
	}
	if f, ok := seen[203]; !ok || f != nil {
		t.Errorf("a knife has no fill: %v", f)
	}
	after, _ := s.One(`select stats from items where id=201`)
	if before["stats"] != after["stats"] {
		t.Error("reading the fill level changed the item")
	}
}

func TestStorageItemsShowTheFillLevelToo(t *testing.T) {
	s := testsave.PlayerWithSQL(t, fillWorld)
	o := &Ops{S: s}
	r, err := o.StorageItems(21)
	if err != nil {
		t.Fatal(err)
	}
	items := r.([]map[string]any)
	if len(items) != 3 {
		t.Fatalf("items: %v", items)
	}
	if f, _ := items[0]["fill"].(map[string]any); f == nil || f["amount"].(float64) != 11000 {
		t.Errorf("fill: %v", items[0]["fill"])
	}
	if _, leaked := items[0]["stats"]; leaked {
		t.Error("stats leaked")
	}
}
