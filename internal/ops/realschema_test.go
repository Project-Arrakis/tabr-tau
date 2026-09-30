package ops

import (
	"encoding/json"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func mustOne(t *testing.T, o *Ops, q string, a ...any) map[string]any {
	t.Helper()
	r, err := o.S.One(q, a...)
	if err != nil || r == nil {
		t.Fatalf("query %q: row=%v err=%v", q, r, err)
	}
	return r
}

// Give-item against the real game schema: the insert must satisfy STRICT typing and every CHECK and foreign key,
// and leave the database structurally sound. The old fixture could not detect a violation of any of these.
func TestGiveItemOnRealSchema(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	res, err := o.GiveItem(Args{"template_id": "Spice", "quantity": 5.0, "quality": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	id := res.(map[string]any)["itemId"].(int64)
	if id != 500 {
		t.Fatalf("id should come from items_id_sequencer (500), got %d", id)
	}
	it := mustOne(t, o, `select * from items where id=?`, id)
	if it["template_id"] != "Spice" || it["stack_size"].(int64) != 5 || it["quality_level"].(int64) != 2 {
		t.Fatalf("wrong row: %v", it)
	}
	if it["volume_override"] != nil {
		t.Errorf("volume_override should stay NULL like game-written rows, got %v", it["volume_override"])
	}
	var stats map[string]any
	if err := json.Unmarshal([]byte(it["stats"].(string)), &stats); err != nil {
		t.Fatalf("stats is not valid JSON: %v", err)
	}
	for _, k := range []string{"FItemStackAndDurabilityStats", "FCustomizationStats"} {
		if _, ok := stats[k]; !ok {
			t.Errorf("stats missing %s: %v", k, stats)
		}
	}
	if n := mustOne(t, o, `select next_id n from items_id_sequencer`)["n"].(int64); n != 501 {
		t.Errorf("sequencer should advance to 501, got %d", n)
	}
	assertDBSound(t, o)
}

func TestGiveItemRefusesFullBackpackOnRealSchema(t *testing.T) {
	o := &Ops{S: testsave.Player(t)} // 5 slots, 2 used
	for i := 0; i < 3; i++ {
		if _, err := o.GiveItem(Args{"template_id": "Spice", "quantity": 1.0}); err != nil {
			t.Fatalf("give %d: %v", i, err)
		}
	}
	if _, err := o.GiveItem(Args{"template_id": "Spice", "quantity": 1.0}); err == nil {
		t.Fatal("a full backpack must refuse the give")
	}
	assertDBSound(t, o)
}

func TestSolariAddRemoveOnRealSchema(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	if _, err := o.AddSolari(Args{"amount": 250.0}); err != nil {
		t.Fatal(err)
	}
	if n := mustOne(t, o, `select stack_size n from items where template_id='SolarisCoin'`)["n"].(int64); n != 350 {
		t.Fatalf("want 350 solari, got %d", n)
	}
	assertDBSound(t, o)
}

// assertDBSound is the invariant set every mutator test must end with (plan section 4.2 step 4).
func assertDBSound(t *testing.T, o *Ops) {
	t.Helper()
	if r := mustOne(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Fatalf("integrity_check: %v", r)
	}
	if rows, err := o.S.Query(`pragma foreign_key_check`); err != nil || len(rows) != 0 {
		t.Fatalf("foreign_key_check violations: %v (err %v)", rows, err)
	}
	if n := mustOne(t, o, `select next_id n from items_id_sequencer`)["n"].(int64); n <= mustOne(t, o, `select max(id) m from items`)["m"].(int64) {
		t.Fatalf("items_id_sequencer.next_id (%d) must stay above max(items.id)", n)
	}
}
