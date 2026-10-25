package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #258 / #18 (a new item's stats): a plain resource that the save has never held. In a played save of the character (and 30 of its older copies) 704 of 705
// resource rows the game wrote carry a durability record of exactly {"DecayedMaxDurability": 0.0}, and no resource row has a current durability. So a new resource
// with no item of its template to copy takes that record; every other kind keeps the record it had (the kinds differ item by item and are not settled).
func TestANewResourceWithNoSampleGetsTheGamesPlainDurabilityRecord(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL)}
	r, err := o.GiveItem(Args{"template_id": "AzuriteOre", "quantity": 5.0})
	if err != nil {
		t.Fatal(err)
	}
	id := r.(map[string]any)["itemId"].(int64)
	row := mustOne(t, o, `select json_extract(stats,'$.FItemStackAndDurabilityStats[1].DecayedMaxDurability') d,
		json_type(stats,'$.FItemStackAndDurabilityStats[1].CurrentDurability') c, json_type(stats,'$.FItemStackAndDurabilityStats[1]') t from items where id=?`, id)
	if row["t"] != "object" || row["c"] != nil {
		t.Fatalf("the durability record is an object with no current durability: %v", row)
	}
	if d, ok := row["d"].(float64); !ok || d != 0 {
		t.Fatalf("decayed maximum durability must be 0.0: %v", row)
	}
}

// An item that is not a resource keeps the empty record, and a resource that is in the save is still a copy of the oldest one.
func TestOtherKindsAndSampledResourcesAreUnchanged(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL+
		`insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (50, 1, 500, 5, 'AluminiumBar', '{"FItemStackAndDurabilityStats":[[],{"DecayedMaxDurability":7.0}]}');`)}
	r, err := o.GiveItem(Args{"template_id": "AluminiumBar", "quantity": 5.0})
	if err != nil {
		t.Fatal(err)
	}
	id := r.(map[string]any)["itemId"].(int64)
	if got := mustOne(t, o, `select json_extract(stats,'$.FItemStackAndDurabilityStats[1].DecayedMaxDurability') d from items where id=?`, id)["d"]; got != 7.0 {
		t.Fatalf("a sampled resource copies its sample: %v", got)
	}
	r, err = o.GiveItem(Args{"template_id": "Ammo", "quantity": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	id = r.(map[string]any)["itemId"].(int64)
	if got := mustOne(t, o, `select json_type(stats,'$.FItemStackAndDurabilityStats[1].DecayedMaxDurability') t from items where id=?`, id)["t"]; got != nil {
		t.Fatalf("a weapon-category item keeps the empty record: %v", got)
	}
}
