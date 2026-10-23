package ops

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A give puts in as much as fits (slots, stack size, the bound) and says what it did. The fixture's backpack (inventory 1) has 5 slots: the Solari in slot 0, a Knife in slot 1, so
// 3 are free. Spice-infused Fuel Cells stack to 500; a Literjon takes a slot for each copy.

const fuel = "SpicedFuelCell"
const fuelLabel = "Spice-infused Fuel Cell"

func fuelStacks(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select stack_size from items where template_id=? order by position_index`, fuel)
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int64
	for _, r := range rows {
		sizes = append(sizes, r["stack_size"].(int64))
	}
	return fmt.Sprint(sizes)
}

func give(t *testing.T, o *Ops, tmpl string, qty float64) map[string]any {
	t.Helper()
	r, err := o.GiveItem(Args{"template_id": tmpl, "quantity": qty})
	if err != nil {
		t.Fatalf("give %v of %s: %v", qty, tmpl, err)
	}
	return r.(map[string]any)
}

// everything but the rows of one template, and the id sequencer, as text: shows that a give changed nothing else.
func rowsExcept(t *testing.T, o *Ops, tmpl string) string {
	t.Helper()
	var all []string
	for _, tb := range testsave.TableNames(t) {
		q := `select * from "` + tb + `"`
		switch tb {
		case "items":
			q += ` where template_id<>'` + tmpl + `'`
		case "items_id_sequencer":
			continue
		}
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			all = append(all, tb+": "+fmt.Sprint(r))
		}
	}
	sort.Strings(all)
	return strings.Join(all, "\n")
}

func isNoRoom(err error) bool { return errors.Is(err, apierr.ErrNoRoom) }

// Slots: 2,000 needs four stacks and three slots are free, so 1,500 go in and the answer says why the rest did not.
func TestAGiveThatNeedsMoreSlotsThanAreFreeGivesWhatFits(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	before := rowsExcept(t, o, fuel)
	r := give(t, o, fuel, 2000)
	if r["requested"] != int64(2000) || r["given"] != int64(1500) || r["limit"] != "slots" || r["reason"] != "the backpack is full" || r["stacks"] != 3 {
		t.Fatalf("the answer: %v", r)
	}
	if got := fuelStacks(t, o); got != "[500 500 500]" {
		t.Fatalf("stacks %s", got)
	}
	if rowsExcept(t, o, fuel) != before {
		t.Fatal("something else changed")
	}
	assertDBSound(t, o)
}

// Stacks that are there are filled first and need no slot: 120 + 3,000 into three free slots is a top-up of 380 and three new stacks.
func TestExistingStacksAreToppedUpBeforeNewOnesAreMade(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 1, 120, 2, '`+fuel+`', '{}');`)}
	r := give(t, o, fuel, 3000) // free slots: 2
	if r["given"] != int64(380+1000) || r["limit"] != "slots" || r["stacks"] != 2 {
		t.Fatalf("the answer: %v", r)
	}
	if got := fuelStacks(t, o); got != "[500 500 500]" {
		t.Fatalf("stacks %s", got)
	}
	assertDBSound(t, o)
}

// Nothing but the top-up fits: the backpack is full, and the stack that has room is still filled.
func TestAFullBackpackStillTakesWhatFitsOnAStackThatIsThere(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=3 where id=1;insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 1, 120, 2, '`+fuel+`', '{}');`)}
	r := give(t, o, fuel, 1000)
	if r["given"] != int64(380) || r["requested"] != int64(1000) || r["reason"] != "the backpack is full" || r["stacks"] != 0 {
		t.Fatalf("the answer: %v", r)
	}
	if got := fuelStacks(t, o); got != "[500]" {
		t.Fatalf("stacks %s", got)
	}
	assertDBSound(t, o)
}

// An exact fit is a full give with no reason.
func TestAGiveThatFitsExactlyIsWholeAndHasNoReason(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	r := give(t, o, fuel, 1500) // three free slots, three full stacks
	if r["given"] != int64(1500) || r["requested"] != int64(1500) || r["reason"] != "" || r["limit"] != "" {
		t.Fatalf("the answer: %v", r)
	}
}

// Nothing fits: a refusal (409, no_room) that writes nothing.
func TestAGiveThatFitsNothingIsRefusedAndWritesNothing(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;`)}
	before := rowsExcept(t, o, fuel)
	_, err := o.GiveItem(Args{"template_id": fuel, "quantity": 100.0})
	if !isNoRoom(err) || !strings.Contains(err.Error(), "Nothing was given") || !strings.Contains(err.Error(), "full") {
		t.Fatalf("the person is told why: %v", err)
	}
	if o.S.Dirty() || rowsExcept(t, o, fuel) != before || fuelStacks(t, o) != "[]" {
		t.Fatal("a give that fits nothing leaves nothing behind")
	}
	// a stack that is there but full does not help either
	o2 := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=3 where id=1;insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 1, 500, 2, '`+fuel+`', '{}');`)}
	if _, err := o2.GiveItem(Args{"template_id": fuel, "quantity": 1.0}); !isNoRoom(err) || o2.S.Dirty() {
		t.Fatalf("%v dirty=%v", err, o2.S.Dirty())
	}
}

// An item that does not stack takes a slot for each copy, so the slots decide how many copies go in.
func TestCopiesOfAnItemThatDoesNotStackAreLimitedBySlots(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	r := give(t, o, "Literjon", 10)
	if r["given"] != int64(3) || r["requested"] != int64(10) || r["stacks"] != 3 || r["limit"] != "slots" {
		t.Fatalf("the answer: %v", r)
	}
	if n := countWhere(t, o, `select count(*) n from items where template_id='Literjon'`); n != 3 {
		t.Fatalf("%d copies", n)
	}
	assertDBSound(t, o)
}

// An item the catalog does not know goes in as one row: it needs one slot, and a full backpack refuses it.
func TestAnItemTheCatalogDoesNotKnowNeedsOneSlot(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 5, 1, 0, 'BrandNewGameItem_X', '{}');`+
		`insert into inventories(id, actor_id, inventory_type, max_item_count, max_item_volume) values (5, 2, 1, 10, 1000.0);`)}
	r := give(t, o, "BrandNewGameItem_X", 7)
	if r["given"] != int64(7) || r["stacks"] != 1 {
		t.Fatalf("%v", r)
	}
	o2 := &Ops{S: testsave.PlayerWithSQL(t, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 5, 1, 0, 'BrandNewGameItem_X', '{}');`+
		`insert into inventories(id, actor_id, inventory_type, max_item_count, max_item_volume) values (5, 2, 1, 10, 1000.0);update inventories set max_item_count=2 where id=1;`)}
	if _, err := o2.GiveItem(Args{"template_id": "BrandNewGameItem_X", "quantity": 7.0}); !isNoRoom(err) {
		t.Fatalf("%v", err)
	}
}

// The bound (at most 1,000 stacks in one give) is a limit like the others: the give is cut at it and says so.
func TestTheBoundCutsAGiveAndSaysSo(t *testing.T) {
	exact := &Ops{S: testsave.PlayerWithSQL(t, noSlotLimitSQL)}
	if r := give(t, exact, "Literjon", maxGiveStacks); r["given"] != int64(maxGiveStacks) || r["reason"] != "" || r["limit"] != "" {
		t.Fatalf("the bound itself is a whole give: %v", r)
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, noSlotLimitSQL)}
	r := give(t, o, "Literjon", 1200)
	if r["given"] != int64(maxGiveStacks) || r["limit"] != "bound" || !strings.Contains(r["reason"].(string), "1,000") {
		t.Fatalf("%v", r)
	}
	// slots and the bound together: whichever is smaller decides, and it is the one named
	o2 := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=1200 where id=1;`)}
	r = give(t, o2, "Literjon", 2000) // 1,198 slots free, the bound is 1,000
	if r["given"] != int64(maxGiveStacks) || r["limit"] != "bound" {
		t.Fatalf("%v", r)
	}
	r = give(t, o2, "Literjon", 2000) // 198 slots free now
	if r["given"] != int64(198) || r["limit"] != "slots" {
		t.Fatalf("%v", r)
	}
}

// One edit, one line in the write log with the real amount; a whole give keeps its old wording.
func TestAPartialGiveIsOneEditWithTheRealAmountInTheLog(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.Player(t)}
	give(t, o, fuel, 2000)
	want := "Give 1,500 of 2,000 x " + fuelLabel + " (grade 0)"
	if p := o.S.Pending(); len(p) != 1 || p[0] != want {
		t.Fatalf("pending: %v", p)
	}
	o2 := &Ops{S: testsave.Player(t)}
	give(t, o2, fuel, 100)
	if p := o2.S.Pending(); len(p) != 1 || p[0] != "Give 100 x "+fuelLabel+" (grade 0)" {
		t.Fatalf("a whole give: %v", p)
	}
	if f := o.S.Flush(); !f.Saved || len(f.Ops) != 1 || f.Ops[0] != want {
		t.Fatalf("%+v", f)
	}
}

// The room is read inside the edit, so a queued copy is put back only onto a file where the inventory still has the room it had.
func TestAQueuedPartialGiveIsHeldWhenTheGameUsedAFreeSlot(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.Player(t)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	give(t, o, fuel, 2000)
	gameChanges(t, o.S.Path, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (30, 1, 1, 2, 'Rifle', '{}')`)
	mustBeHeld(t, o.S)
}

func TestAQueuedPartialGiveIsWrittenWhenTheGameChangedSomethingElse(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.Player(t)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	give(t, o, fuel, 2000)
	gameChanges(t, o.S.Path, `update actors set location_x=location_x+10 where id=1`)
	if f := o.S.Flush(); !f.Saved || o.S.Held() != nil {
		t.Fatalf("%+v", f)
	}
	if got := fuelStacks(t, o); got != "[500 500 500]" {
		t.Fatalf("%s", got)
	}
	assertDBSound(t, o)
}

// The edit is put back whole (the same partial amount) when the editor starts again with the queue still waiting.
func TestAQueuedPartialGiveIsPutBackWhenTheEditorStartsAgain(t *testing.T) {
	s1 := testsave.Player(t)
	jp := filepath.Join(t.TempDir(), "pending.json")
	s1.EnableJournal(jp)
	give(t, &Ops{S: s1}, fuel, 2000)
	pending := s1.Pending()
	s1.Close()
	s2, err := save.Open(s1.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s2.Close)
	s2.EnableJournal(jp)
	o := &Ops{S: s2}
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("put back whole: %+v %v", r, err)
	}
	if got := fuelStacks(t, o); got != "[500 500 500]" {
		t.Fatalf("%s", got)
	}
	if got := s2.Pending(); len(got) != 1 || got[0] != pending[0] {
		t.Fatalf("the same edit is pending again: %v", got)
	}
	assertDBSound(t, o)
}

// The queue of items: each line gives what fits, the answer has a line for each, and a queue where nothing fits is refused.
func TestTheGiveQueueGivesWhatFitsForEachLine(t *testing.T) {
	o := &Ops{S: testsave.Player(t)} // 3 free slots
	r, err := o.GiveItems(Args{"items": []any{
		map[string]any{"template_id": fuel, "quantity": 1000.0},
		map[string]any{"template_id": "Literjon", "quantity": 5.0},
	}})
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	lines := m["lines"].([]map[string]any)
	if m["requested"] != int64(1005) || m["given"] != int64(1001) || len(lines) != 2 || lines[0]["given"] != int64(1000) || lines[1]["given"] != int64(1) || lines[1]["limit"] != "slots" || m["reason"] != "the backpack is full" {
		t.Fatalf("%v", m)
	}
	assertDBSound(t, o)
	o2 := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;`)}
	if _, err := o2.GiveItems(Args{"items": []any{map[string]any{"template_id": fuel, "quantity": 5.0}}}); !isNoRoom(err) || o2.S.Dirty() {
		t.Fatalf("%v dirty=%v", err, o2.S.Dirty())
	}
}

// A give into a base container is the same give.
func TestAGiveIntoABaseContainerGivesWhatFits(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := scopeOps(t)
	free := func() int64 {
		return mustOne(t, o, `select v.max_item_count - (select count(distinct position_index) from items where inventory_id=v.id) n from inventories v where v.id=7001`)["n"].(int64)
	}
	f := free()
	if f < 1 {
		t.Fatalf("the fixture's container is full: %d", f)
	}
	r, err := o.GiveToInventory(Args{"inventory_id": 7001.0, "template_id": "Literjon", "quantity": float64(f + 5)})
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["given"] != f || m["requested"] != f+5 || m["reason"] != "the container is full" {
		t.Fatalf("%v (free %d)", m, f)
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != fmt.Sprintf("Give %d of %d x %s to container 7001", f, f+5, itemLabel("Literjon")) {
		t.Fatalf("the log says what really went in: %v", p)
	}
	if _, err := o.GiveToInventory(Args{"inventory_id": 7001.0, "template_id": "Literjon", "quantity": 1.0}); !isNoRoom(err) {
		t.Fatalf("now it is full: %v", err)
	}
}

// Solari follow the strict give: a request that needs more stacks than the bound is refused whole.
func TestSolariPastTheBoundIsRefusedWhole(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, noSlotLimitSQL)}
	_, err := o.AddSolari(Args{"amount": 60_000_000.0})
	if err == nil || !strings.Contains(err.Error(), "separate stacks") {
		t.Fatalf("%v", err)
	}
	if o.S.Dirty() {
		t.Fatal("nothing was added")
	}
}
