package ops

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Audit L2 G09 (#166): a refill or repair read its rows and built its plan before the transaction, so two overlapping calls (a double
// click) both planned from the same stale state and both wrote. Each of these runs the same call from many goroutines at once and checks that
// the result is the one a single call would have made.

// concurrently runs fn n times at once, released together, and returns once all have finished.
func concurrently(n int, fn func(i int)) {
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		ready.Add(1)
		done.Add(1)
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			fn(i)
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()
}

func TestOverlappingGeneratorRefillsDoNotOverfill(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, baseFixture+turbineWorld)}
	results := make([]map[string]any, 12)
	concurrently(len(results), func(i int) {
		r, err := o.RefillGenerators()
		if err != nil {
			t.Error(err)
			return
		}
		results[i] = r.(map[string]any)
	})
	filled := 0
	for _, r := range results {
		if r != nil {
			filled += r["filled"].(int)
		}
	}
	// 111 and 113 (oil), 901 and 902 (omnidirectional lubricant), 904 (directional), 911, 912, 913 (windtraps): the other call sees all full
	if filled != 8 {
		t.Fatalf("every device is filled by exactly one of the overlapping calls, got %d fills", filled)
	}
	if _, n, total := stacks(t, o, 303); n != 1 || total != 499 {
		t.Fatalf("the empty generator holds one stack of 499 Oil, not %d rows / %d: an overlapping call planned from stale state", n, total)
	}
	if _, n, total := stacks(t, o, 952); n != 5 || total != 499 {
		t.Fatalf("the empty turbine holds five rows / 499, not %d / %d", n, total)
	}
	if _, n, total := stacks(t, o, 961); n != 1 || total != 5 {
		t.Fatalf("a windtrap holds one stack of 5 filters, not %d / %d", n, total)
	}
	assertDBSound(t, o)
}

func TestOverlappingWaterRefillsFillEachDeviceOnce(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, baseFixture)}
	filled := make([]int, 12)
	concurrently(len(filled), func(i int) {
		r, err := o.RefillBaseWater()
		if err != nil {
			t.Error(err)
			return
		}
		filled[i] = r.(map[string]any)["filled"].(int)
	})
	sum := 0
	for _, n := range filled {
		sum += n
	}
	if sum != 2 { // the empty cistern and the nearly full one
		t.Fatalf("each device is filled by exactly one call, got %d fills in total", sum)
	}
	if ops := len(o.S.Pending()); ops != 1 {
		t.Fatalf("the overlapping calls left %d pending edits, want 1", ops)
	}
}

func TestOverlappingSpecialisationXPAddsUp(t *testing.T) {
	o := specOps(t)
	nums := map[string]int64{"Gathering": 2}
	concurrently(10, func(int) {
		if _, err := o.AddSpecXP(nums, Args{"track": "Gathering", "amount": 100.0}); err != nil {
			t.Error(err)
		}
	})
	if xp := mustOne(t, o, `select xp_amount x from specialization_tracks where player_id=1 and track_type=2`)["x"].(int64); xp != 3000+10*100 {
		t.Fatalf("ten additions of 100 must add up to 4000, got %d (lost updates)", xp)
	}
}

func TestOverlappingCustomizationGrantsGiveEachItemOnce(t *testing.T) {
	o := bigBackpack(t, ``)
	granted := make([]int, 8)
	concurrently(len(granted), func(i int) {
		r, err := o.GrantCustomizations(Args{"group": "filmic-archive"})
		if err != nil {
			t.Error(err)
			return
		}
		granted[i] = r.(map[string]any)["granted"].(int)
	})
	sum := 0
	for _, n := range granted {
		sum += n
	}
	if sum != 3 {
		t.Fatalf("the set has three items and one call grants them, got %d", sum)
	}
	if n := mustOne(t, o, `select count(*) c from items where template_id in ('MTX_Fremen_FedaykinArmor_SetVariant','MTX_Atre_CaladanTrenchcoat_SetVariant','MTX_Sard_Scout_SetVariant')`)["c"].(int64); n != 3 {
		t.Fatalf("each item exists once, found %d", n)
	}
}

func TestOverlappingGearRepairsRepairEachItemOnce(t *testing.T) {
	const items = 300
	var ins []string
	for i := 0; i < items; i++ {
		ins = append(ins, fmt.Sprintf(`(%d,21,1,%d,'Piece',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.0,"MaxDurability":100.0}]}',0)`, 200+i, i))
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,1000,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values `+strings.Join(ins, ",")+`;`)}
	repaired := make([]int, 16)
	concurrently(len(repaired), func(i int) {
		r, err := o.RepairGear()
		if err != nil {
			t.Error(err)
			return
		}
		repaired[i] = r.(map[string]any)["repaired"].(int)
	})
	sum := 0
	for _, n := range repaired {
		sum += n
	}
	if sum != items {
		t.Fatalf("each item is repaired by exactly one call, got %d repairs in total for %d items", sum, items)
	}
}

// Audit L2 G10 (#167): Repair Gear decoded the whole stats document into float64 values and wrote it back, so every integer above 2^53
// lost its low digits, every 100.0 became 100, and the keys were re-ordered. It now patches only the durability keys in the stored text.
func TestRepairGearChangesOnlyTheDurabilityKeys(t *testing.T) {
	const before = `{"FCustomizationStats":[[],{"Seed":9007199254740993,"Scale":1.0}],"FItemStackAndDurabilityStats":[[],{"Zeta":2.50,"CurrentDurability":10.0,"MaxDurability":100.0,"DecayedMaxDurability":80.0,"Alpha":1e2,"Id":9007199254740993}],"FAugmentedItemStats":[[],{"AppliedAugments":[{"Name":"X"}],"AppliedAugmentQualities":[1.0]}]}`
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (201,21,1,0,'Helmet',0,1790000000,'`+before+`',0);`)}
	if _, err := o.RepairGear(); err != nil {
		t.Fatal(err)
	}
	got := mustOne(t, o, `select stats s from items where id=201`)["s"].(string)
	want := strings.NewReplacer(`"CurrentDurability":10.0`, `"CurrentDurability":100.0`, `"DecayedMaxDurability":80.0`, `"DecayedMaxDurability":100.0`).Replace(before)
	if got != want {
		t.Fatalf("only the two durability values may change.\n got: %s\nwant: %s", got, want)
	}
	if mustOne(t, o, `select typeof(stats) t from items where id=201`)["t"] != "text" {
		t.Fatal("the stats column stays text")
	}
}

// The repair rules at their edges (QA-10 of the same audit): an item exactly at its maximum is not touched, one above it is not lowered, a
// recorded maximum of 0 means none (100 is used), and a worn-down maximum alone is enough to rewrite an item.
func TestRepairGearEdges(t *testing.T) {
	st := func(body string) string { return `{"FItemStackAndDurabilityStats":[[],{` + body + `}]}` }
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (401,21,1,0,'AtMax',0,1790000000,'`+st(`"CurrentDurability":100.0,"MaxDurability":100.0`)+`',0),
  (402,21,1,1,'Above',0,1790000000,'`+st(`"CurrentDurability":150.0,"MaxDurability":100.0`)+`',0),
  (403,21,1,2,'ZeroMax',0,1790000000,'`+st(`"CurrentDurability":50.0,"MaxDurability":0.0`)+`',0),
  (404,21,1,3,'WornOnly',0,1790000000,'`+st(`"CurrentDurability":100.0,"MaxDurability":100.0,"DecayedMaxDurability":60.0`)+`',0),
  (405,21,1,4,'NoRecord',0,1790000000,'{"FCustomizationStats":[[],{}]}',0),
  (406,21,1,5,'NoCurrent',0,1790000000,'`+st(`"MaxDurability":100.0`)+`',0);`)}
	r, err := o.RepairGear()
	if err != nil {
		t.Fatal(err)
	}
	if got := r.(map[string]any)["repaired"].(int); got != 2 {
		t.Fatalf("only the zero-max item (to 100) and the worn-down maximum are repaired: %v", r)
	}
	stats := func(id int) string {
		return mustOne(t, o, `select stats s from items where id=?`, id)["s"].(string)
	}
	if stats(401) != st(`"CurrentDurability":100.0,"MaxDurability":100.0`) {
		t.Fatalf("an item at its maximum is left byte for byte as it was: %s", stats(401))
	}
	if stats(402) != st(`"CurrentDurability":150.0,"MaxDurability":100.0`) {
		t.Fatalf("an item above its maximum is never lowered: %s", stats(402))
	}
	if stats(403) != st(`"CurrentDurability":100.0,"MaxDurability":0.0`) {
		t.Fatalf("a recorded maximum of 0 means none: the item is as good as new at 100, and the 0 is left alone: %s", stats(403))
	}
	if stats(404) != st(`"CurrentDurability":100.0,"MaxDurability":100.0,"DecayedMaxDurability":100.0`) {
		t.Fatalf("a worn-down maximum goes back to full: %s", stats(404))
	}
	if stats(405) != `{"FCustomizationStats":[[],{}]}` || stats(406) != st(`"MaxDurability":100.0`) {
		t.Fatalf("items with no durability record or no current value are left alone: %s %s", stats(405), stats(406))
	}
	if n := len(o.S.Pending()); n != 1 || !strings.Contains(fmt.Sprint(o.S.Pending()), "repair gear: 2 items") {
		t.Fatalf("one pending edit that names the count: %v", o.S.Pending())
	}
}
