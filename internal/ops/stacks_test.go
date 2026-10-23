package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The real schema, a backpack of 40 slots (the fixture's has 5) with the 100 Solari the fixture holds in item 10 (slot 0), and, where a test needs them, more stacks.
const roomySQL = `update inventories set max_item_count=40 where id=1;`

const moreSolariSQL = roomySQL + `
insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (12, 1, 50000, 2, 'SolarisCoin', '{}');
insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (13, 1, 25000, 3, 'SolarisCoin', '{}');
`

func solariStacks(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select stack_size from items where template_id='SolarisCoin' order by position_index`)
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int64
	for _, r := range rows {
		sizes = append(sizes, r["stack_size"].(int64))
	}
	return fmt.Sprint(sizes)
}

func addSolari(t *testing.T, o *Ops, n float64) int64 {
	t.Helper()
	r, err := o.AddSolari(Args{"amount": n})
	if err != nil {
		t.Fatalf("add %v: %v", n, err)
	}
	return r.(map[string]any)["solari"].(int64)
}

// A slot holds no more Solari than the item tables give (50,000), as Give and SetItem already hold for every item: a wallet past that spans stacks.
func TestAddingSolariPastTheStackSizeMakesMoreStacks(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL)}
	if got := addSolari(t, o, 125_000); got != 125_100 {
		t.Fatalf("the player holds %d, want 125,100", got)
	}
	if got := solariStacks(t, o); got != "[50000 50000 25100]" { // the stack that was there is filled first
		t.Fatalf("stacks %s", got)
	}
	assertDBSound(t, o)
}

func TestSolariFillAStackExactlyAndThenStartANewOne(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL)}
	addSolari(t, o, 49_900) // 100 + 49,900 = 50,000: one full stack
	if got := solariStacks(t, o); got != "[50000]" {
		t.Fatalf("stacks %s", got)
	}
	addSolari(t, o, 1)
	if got := solariStacks(t, o); got != "[50000 1]" {
		t.Fatalf("one more Solari starts a stack: %s", got)
	}
	assertDBSound(t, o)
}

// The backpack's slots are what limits it: a wallet that needs more stacks than there are free slots is refused whole, with the reason.
func TestSolariThatNeedMoreSlotsThanTheBackpackHasAreRefusedWhole(t *testing.T) {
	o := &Ops{S: testsave.Player(t)} // 5 slots, 2 used
	_, err := o.AddSolari(Args{"amount": 200_000.0})
	if err == nil || !strings.Contains(err.Error(), "not enough room") || !strings.Contains(err.Error(), "50,000") {
		t.Fatalf("the person is told why: %v", err)
	}
	if got := solariStacks(t, o); got != "[100]" || o.S.Dirty() {
		t.Fatalf("a refused add leaves nothing behind: %s dirty=%v", got, o.S.Dirty())
	}
}

// The first Solari of a player who has none go into the backpack, in stacks.
func TestSolariAddedToAPlayerWithNoneMakeStacksInTheBackpack(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL+`delete from items where id=10;`)}
	if got := addSolari(t, o, 60_000); got != 60_000 {
		t.Fatalf("holds %d", got)
	}
	if got := solariStacks(t, o); got != "[50000 10000]" {
		t.Fatalf("stacks %s", got)
	}
	if r := mustOne(t, o, `select count(*) n from items where template_id='SolarisCoin' and inventory_id=1`); r["n"].(int64) != 2 {
		t.Fatal("both stacks are in the backpack")
	}
	assertDBSound(t, o)
}

// Removing takes from every stack it needs, from the last slot back, and deletes a stack it empties.
func TestRemovingSolariTakesFromSeveralStacks(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, moreSolariSQL)} // 100 + 50,000 + 25,000
	if got := addSolari(t, o, -30_000); got != 45_100 {
		t.Fatalf("the player holds %d, want 45,100", got)
	}
	if got := solariStacks(t, o); got != "[100 45000]" { // the 25,000 stack is gone, the 5,000 that was left to take came off the one before it
		t.Fatalf("stacks %s", got)
	}
	if got := addSolari(t, o, -45_100); got != 0 {
		t.Fatalf("everything can be removed, %d left", got)
	}
	if got := solariStacks(t, o); got != "[]" {
		t.Fatalf("no stack is left behind: %s", got)
	}
	assertDBSound(t, o)
}

// A removal of more than the player holds used to delete the first stack and report success; it is refused and nothing changes.
func TestRemovingMoreSolariThanThePlayerHoldsIsRefused(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, moreSolariSQL)}
	_, err := o.AddSolari(Args{"amount": -80_000.0})
	if err == nil || !strings.Contains(err.Error(), "not enough Solari") || !strings.Contains(err.Error(), "75,100") {
		t.Fatalf("the person is told what the backpack holds: %v", err)
	}
	if got := solariStacks(t, o); got != "[100 50000 25000]" || o.S.Dirty() {
		t.Fatalf("a refused removal changes nothing: %s dirty=%v", got, o.S.Dirty())
	}
	if _, err := o.AddSolari(Args{"amount": -1.0}); err != nil {
		t.Fatal(err)
	}
}

func TestRemovingSolariFromAPlayerWithNoneIsRefused(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `delete from items where id=10;`)}
	if _, err := o.AddSolari(Args{"amount": -5.0}); err == nil || !strings.Contains(err.Error(), "no Solari") {
		t.Fatalf("%v", err)
	}
}

// ---- stack sizes of Give and SetItem (QA-11)

// A partial stack of another grade is not topped up: a grade is part of what makes two stacks the same item.
func TestGiveDoesNotTopUpAStackOfAnotherGrade(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL)}
	give := func(qty, grade float64) {
		t.Helper()
		if _, err := o.GiveItem(Args{"template_id": "SpicedFuelCell", "quantity": qty, "quality": grade}); err != nil {
			t.Fatal(err)
		}
	}
	stacks := func() string {
		rows, _ := o.S.Query(`select quality_level q, stack_size n from items where template_id='SpicedFuelCell' order by position_index`)
		var out []string
		for _, r := range rows {
			out = append(out, fmt.Sprintf("g%d:%d", r["q"], r["n"]))
		}
		return strings.Join(out, " ")
	}
	give(120, 0)
	give(100, 3)
	if got := stacks(); got != "g0:120 g3:100" {
		t.Fatalf("a stack of another grade stays apart: %s", got)
	}
	give(100, 0)
	if got := stacks(); got != "g0:220 g3:100" {
		t.Fatalf("only the stack of the same grade is topped up: %s", got)
	}
	assertDBSound(t, o)
}

func TestGiveThatNeedsMoreSlotsThanAreFreeSaysSo(t *testing.T) {
	o := &Ops{S: testsave.Player(t)} // 3 free slots
	_, err := o.GiveItem(Args{"template_id": "SpicedFuelCell", "quantity": 4000.0})
	if err == nil || !strings.Contains(err.Error(), "not enough room") || !strings.Contains(err.Error(), "8 stacks of up to 500") {
		t.Fatalf("the person is told how many stacks it needs: %v", err)
	}
	if n := mustOne(t, o, `select count(*) n from items where template_id='SpicedFuelCell'`)["n"].(int64); n != 0 || o.S.Dirty() {
		t.Fatalf("a refused give leaves nothing behind: %d", n)
	}
}

// 500 is the most one slot of Spice-infused Fuel Cells holds: 500 is accepted, 501 is not, and the stack is left as it was.
func TestAStackEditIsAcceptedAtTheStackSizeAndRefusedOneAbove(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL+`insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (30, 1, 7, 4, 'SpicedFuelCell', '{}');`)}
	stack := func() int64 { return mustOne(t, o, `select stack_size n from items where id=30`)["n"].(int64) }
	if _, err := o.SetItem(Args{"id": 30.0, "stack_size": 501.0}); err == nil || !strings.Contains(err.Error(), "stacks to 500") || !strings.Contains(err.Error(), "501") {
		t.Fatalf("501 is refused with the reason: %v", err)
	}
	if stack() != 7 {
		t.Fatal("a refused edit changes nothing")
	}
	if _, err := o.SetItem(Args{"id": 30.0, "stack_size": 500.0}); err != nil || stack() != 500 {
		t.Fatalf("500 is accepted: %v %d", err, stack())
	}
	// an item the catalog does not list has no stack size to honor
	if _, err := o.SetItem(Args{"id": 11.0, "stack_size": 9.0}); err != nil {
		t.Fatalf("an item with no known stack size is not capped: %v", err)
	}
}

// A new item takes the shape of its stats from the oldest item of its template in the save, whichever order the items were added in, so giving the same item twice (or giving
// it again when the edit is put back after a restart) writes the same stats.
func TestANewItemTakesItsStatsFromTheOldestItemOfTheTemplate(t *testing.T) {
	const older = `{"FCustomizationStats":[[],{}],"FItemStackAndDurabilityStats":[[],{"CurrentDurability":88.0,"MaxDurability":100.0}]}`
	const newer = `{"FCustomizationStats":[[],{}],"FItemStackAndDurabilityStats":[[],{"CurrentDurability":3.0,"MaxDurability":100.0}]}`
	o := &Ops{S: testsave.PlayerWithSQL(t, roomySQL+
		`insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (60, 1, 1, 6, 'Knife', '`+newer+`');`+
		`insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (50, 1, 1, 5, 'Knife', '`+older+`');`+
		`delete from items where id=11;`)}
	r, err := o.GiveItem(Args{"template_id": "Knife", "quantity": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	id := r.(map[string]any)["itemId"].(int64)
	if got := mustOne(t, o, `select stats s from items where id=?`, id)["s"]; got != older {
		t.Fatalf("the new item has the stats of item 50, the oldest: %v", got)
	}
}
