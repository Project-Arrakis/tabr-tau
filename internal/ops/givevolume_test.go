package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Give and the volume limit (#357). Unit volumes are the game's item table values (catalog.Volume): Aluminum Ingot 0.7 (stack 500), Spice-infused Fuel Cell 0.2 (stack 500),
// Literjon 2, Solari 0. The inventory under test is 31, one of the player's, with no slot limit, so that only the volume decides.
func volumeOps(t *testing.T, limit string, items string) *Ops {
	t.Helper()
	sql := fmt.Sprintf(`insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (31,2,1,0,%s);`, limit)
	if items != "" {
		sql += `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values ` + items + `;`
	}
	return &Ops{S: testsave.PlayerWithSQL(t, sql)}
}

func giveTo(o *Ops, tmpl string, qty float64) (map[string]any, error) {
	r, err := o.GiveItem(Args{"template_id": tmpl, "quantity": qty, "inventory_id": 31.0})
	if err != nil {
		return nil, err
	}
	return r.(map[string]any), nil
}

func stacksOf(t *testing.T, o *Ops, tmpl string) string {
	t.Helper()
	rows, err := o.S.Query(`select stack_size from items where inventory_id=31 and template_id=? order by position_index`, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		out = append(out, fmt.Sprint(r["stack_size"]))
	}
	return strings.Join(out, ",")
}

func TestGiveStopsAtTheVolumeLimitAndSaysWhy(t *testing.T) {
	o := volumeOps(t, "100.0", "")
	r, err := giveTo(o, "AluminiumBar", 1000)
	if err != nil {
		t.Fatal(err)
	}
	// 0.7 per unit: 142 units are 99.4, 143 would be 100.1
	if r["given"] != int64(142) || r["requested"] != int64(1000) || r["limit"] != "volume" || r["reason"] != "the inventory is full (volume)" || r["volumeChecked"] != true || r["volumeNote"] != "" {
		t.Fatalf("%v", r)
	}
	if got := stacksOf(t, o, "AluminiumBar"); got != "142" {
		t.Fatalf("stacks %s", got)
	}
	// the write is a calculation only: volume_override stays NULL, the game's own convention
	if rows, _ := o.S.Query(`select count(*) n from items where inventory_id=31 and volume_override is not null`); rows[0]["n"] != int64(0) {
		t.Fatalf("volume_override was written: %v", rows)
	}
	// full now: another give is refused whole and writes nothing
	before, _ := o.S.Query(`select count(*) n from items`)
	if _, err := giveTo(o, "AluminiumBar", 1); !isNoRoom(err) || !strings.Contains(err.Error(), "full (volume)") {
		t.Fatalf("a give into a full inventory: %v", err)
	}
	if after, _ := o.S.Query(`select count(*) n from items`); after[0]["n"] != before[0]["n"] {
		t.Fatal("a refused give wrote rows")
	}
}

func TestGiveCountsWhatIsThereAndTheRowsOwnPerUnitOverride(t *testing.T) {
	// 50 Aluminum Ingots (35) and 10 Literjons whose row says 2.5 per unit (25, not the table's 2): 60 used, 40 left, 57 ingots of 0.7 fit
	o := volumeOps(t, "100.0", `(41,31,50,0,'AluminiumBar',0,1790000000,'{}',0,NULL),(42,31,10,1,'Literjon',0,1790000000,'{}',0,2.5)`)
	r, err := giveTo(o, "AluminiumBar", 500)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(57) || r["limit"] != "volume" {
		t.Fatalf("%v", r)
	}
	// the stack that was there is filled first (50 -> 107), no new row
	if got := stacksOf(t, o, "AluminiumBar"); got != "107" {
		t.Fatalf("stacks %s", got)
	}
	if rows, _ := o.S.Query(`select volume_override v from items where id=42`); rows[0]["v"] != 2.5 {
		t.Fatalf("the existing override changed: %v", rows)
	}
}

func TestGiveThatExactlyFillsTheVolumeIsWhole(t *testing.T) {
	o := volumeOps(t, "70.0", "")
	r, err := giveTo(o, "AluminiumBar", 100) // 100 x 0.7 is 70.00000000000001 in floating point
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(100) || r["limit"] != "" || r["reason"] != "" {
		t.Fatalf("%v", r)
	}
	if _, err := giveTo(o, "AluminiumBar", 1); !isNoRoom(err) {
		t.Fatalf("one more: %v", err)
	}
}

func TestVolumeAndSlotsTogetherTheTighterOneIsNamed(t *testing.T) {
	// room for 1,428 ingots by volume (1000 / 0.7), one slot of 500 by slots: the slots are tighter
	o := volumeOps(t, "1000.0", "")
	if _, err := o.S.Mutate("slots", func(m *save.Mut) error { _, e := m.Exec(`update inventories set max_item_count=1 where id=31`); return e }); err != nil {
		t.Fatal(err)
	}
	r, err := giveTo(o, "AluminiumBar", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(500) || r["limit"] != "slots" {
		t.Fatalf("slots are tighter: %v", r)
	}
	// room for 142 by volume, plenty of slots: the volume is tighter
	o2 := volumeOps(t, "100.0", "")
	r, err = giveTo(o2, "AluminiumBar", 1000)
	if err != nil || r["limit"] != "volume" {
		t.Fatalf("volume is tighter: %v %v", r, err)
	}
}

func TestVolumeIsNotCheckedForAnItemWithNoKnownVolumeOrAnInventoryWithNoLimit(t *testing.T) {
	// an id the game's tables do not list
	o := volumeOps(t, "1.0", `(41,31,1,0,'BrandNewGameItem_X',0,1790000000,'{}',0,NULL)`)
	r, err := giveTo(o, "BrandNewGameItem_X", 7)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(7) || r["volumeChecked"] != false || !strings.Contains(r["volumeNote"].(string), "no volume for this item") {
		t.Fatalf("%v", r)
	}
	// a limit that is NULL or 0 is no limit
	for _, lim := range []string{"NULL", "0.0"} {
		o := volumeOps(t, lim, "")
		r, err := giveTo(o, "AluminiumBar", 400)
		if err != nil {
			t.Fatal(err)
		}
		if r["given"] != int64(400) || r["volumeChecked"] != false || !strings.Contains(r["volumeNote"].(string), "no volume limit recorded") {
			t.Fatalf("limit %s: %v", lim, r)
		}
	}
}

func TestItemsOfUnknownVolumeAlreadyThereCountAsNoneAndAreSaid(t *testing.T) {
	o := volumeOps(t, "10.0", `(41,31,40,0,'BrandNewGameItem_X',0,1790000000,'{}',0,NULL)`)
	r, err := giveTo(o, "AluminiumBar", 100) // 10 / 0.7: 14 fit
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(14) || r["limit"] != "volume" || !strings.Contains(r["volumeNote"].(string), "40 units already there have no known volume") {
		t.Fatalf("%v", r)
	}
}

func TestAnItemThatTakesNoVolumeIsNeverCut(t *testing.T) {
	o := volumeOps(t, "1.0", `(41,31,10,0,'Literjon',0,1790000000,'{}',0,NULL)`) // 20 used of 1: already over
	r, err := giveTo(o, "SolarisCoin", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(5000) || r["limit"] != "" || r["volumeChecked"] != true {
		t.Fatalf("%v", r)
	}
	// an inventory already over its limit takes no more of anything that has volume
	if _, err := giveTo(o, "AluminiumBar", 1); !isNoRoom(err) {
		t.Fatalf("over the limit: %v", err)
	}
}

func TestAQueuedGiveSeesTheVolumeTheEarlierLinesTook(t *testing.T) {
	// the queue gives to the backpack: no slot limit, 50 volume, empty
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=0, max_item_volume=50.0 where id=1; delete from items where inventory_id=1;`)}
	r, err := o.GiveItems(Args{"items": []any{
		map[string]any{"template_id": "AluminiumBar", "quantity": 60.0}, // 60 x 0.7 = 42
		map[string]any{"template_id": "Literjon", "quantity": 10.0},     // 8 left: 4 Literjons of 2
	}})
	if err != nil {
		t.Fatal(err)
	}
	lines := r.(map[string]any)["lines"].([]map[string]any)
	if lines[0]["given"] != int64(60) || lines[1]["given"] != int64(4) || lines[1]["limit"] != "volume" || lines[1]["reason"] != "the backpack is full (volume)" {
		t.Fatalf("%v", lines)
	}
}

func TestBloodMarksTakeNoVolume(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_volume=0.5 where id=1; delete from items where inventory_id=1;`)}
	if _, err := o.GiveBloodMarks(Args{"amount": 100.0}); err != nil {
		t.Fatalf("blood marks have volume 0 in the game's tables: %v", err)
	}
}
