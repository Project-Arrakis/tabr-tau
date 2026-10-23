package ops

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Clear a container (#338), on the real schema. The character (pawn 2) has a backpack (inventory 1: Solari 10, Knife 11, a Bag 12 that owns inventory 40 holding
// item 13, and a Spice 14 with a building blueprint), worn gear (inventory 2: items 20, 21), a loadout (inventory 3: item 22) and unique schematics (inventory 4: item 23).
// The base of storageWorld has a chest (7001: items 9001, 9002, and a Solari stack 9004), an empty silo (7002) and a refinery (7003: item 9003).
const clearWorld = storageWorld + `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (2,2,1,10,1000.0),(3,2,15,10,1000.0),(4,2,30,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (12,1,1,2,'Bag',0,1790000000,'{}',0),
  (14,1,5,3,'MelangeSpice',0,1790000000,'{}',0),
  (20,2,1,0,'Helmet',0,1790000000,'{}',0),(21,2,1,1,'Boots',0,1790000000,'{}',0),
  (22,3,1,0,'Rifle',0,1790000000,'{}',0),
  (23,4,1,0,'Schematic',0,1790000000,'{}',0),
  (9004,7001,250,2,'SolarisCoin',0,1790000000,'{}',0);
insert into inventories(id,item_id,inventory_type,max_item_count) values (40,12,3,10);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (13,40,2,0,'InnerThing',0,1790000000,'{}',0);
insert into building_blueprints(id,item_id) values (1,14);
update items_id_sequencer set next_id=10000;`

func clearOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, clearWorld)} }

func clearArgs(inv, count int64, currency ...bool) Args {
	a := Args{"inventory_id": float64(inv), "count": float64(count)}
	if len(currency) > 0 {
		a["include_currency"] = currency[0]
	}
	return a
}

func itemIDs(t *testing.T, o *Ops, where string) string {
	t.Helper()
	rows, err := o.S.Query(`select id from items where ` + where + ` order by id`)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		out = append(out, strconv.FormatInt(r["id"].(int64), 10))
	}
	return strings.Join(out, ",")
}

// rowCounts is every table's row count, to show that a clear changed only the tables it should.
func rowCounts(t *testing.T, o *Ops) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, tb := range testsave.TableNames(t) {
		r, err := o.S.One(`select count(*) n from "` + tb + `"`)
		if err != nil {
			t.Fatal(err)
		}
		out[tb] = r["n"].(int64)
	}
	return out
}

func TestClearingTheBackpackRemovesItsItemsAndWhatHangsFromThemAndKeepsCurrency(t *testing.T) {
	o := clearOps(t)
	before := rowCounts(t, o)
	res, err := o.ClearContainer(clearArgs(1, 3)) // Knife, Bag, Spice; the Solari stack is kept
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["removed"].(int64) != 3 || m["kept"].(int64) != 1 || m["message"] != nil {
		t.Fatalf("answer: %v", m)
	}
	if got := itemIDs(t, o, `inventory_id=1`); got != "10" {
		t.Fatalf("only the Solari stack stays in the backpack, got %q", got)
	}
	after := rowCounts(t, o)
	// the Bag took its inventory and what is in it, the Spice its blueprint: nothing item-linked is left behind
	want := map[string]int64{"items": -4, "inventories": -1, "building_blueprints": -1}
	for tb, n := range before {
		if after[tb]-n != want[tb] {
			t.Errorf("table %s changed by %d, want %d", tb, after[tb]-n, want[tb])
		}
	}
	if n := countWhere(t, o, `select count(*) n from inventories where item_id is not null and item_id not in (select id from items)`); n != 0 {
		t.Errorf("%d inventories point at an item that is gone", n)
	}
	if n := countWhere(t, o, `select count(*) n from building_blueprints where item_id not in (select id from items)`); n != 0 {
		t.Errorf("%d blueprints point at an item that is gone", n)
	}
	// the other containers are untouched
	for inv, ids := range map[int64]string{2: "20,21", 3: "22", 4: "23", 7001: "9001,9002,9004", 7003: "9003"} {
		if got := itemIDs(t, o, `inventory_id=`+dec(inv)); got != ids {
			t.Errorf("container %d changed: %q, want %q", inv, got, ids)
		}
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "Cleared container 1 (3 items)" {
		t.Fatalf("one edit, described without anything the page did not show: %v", p)
	}
	if v := fkViolations(t, o); v != 0 {
		t.Errorf("%d dangling references", v)
	}
	if r := mustOne(t, o, `pragma foreign_keys`); r["foreign_keys"].(int64) != 0 {
		t.Error("foreign_keys must be switched back off after the cascading edit")
	}
	assertDBSound(t, o)
}

func TestClearingWithCurrencyIncludedRemovesTheSolariStackToo(t *testing.T) {
	o := clearOps(t)
	if _, err := o.ClearContainer(clearArgs(1, 4, true)); err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(t, o, `inventory_id=1`); got != "" {
		t.Fatalf("the backpack is empty, got %q", got)
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "Cleared container 1 (4 items, currency included)" {
		t.Fatalf("%v", p)
	}
	assertDBSound(t, o)
}

func TestClearingWornGearAndTheLoadoutTouchesOnlyThatContainer(t *testing.T) {
	for _, c := range []struct {
		inv   int64
		count int64
		left  map[int64]string
	}{
		{2, 2, map[int64]string{1: "10,11,12,14", 3: "22", 4: "23"}},
		{3, 1, map[int64]string{1: "10,11,12,14", 2: "20,21", 4: "23"}},
	} {
		o := clearOps(t)
		if _, err := o.ClearContainer(clearArgs(c.inv, c.count)); err != nil {
			t.Fatalf("container %d: %v", c.inv, err)
		}
		if got := itemIDs(t, o, `inventory_id=`+dec(c.inv)); got != "" {
			t.Errorf("container %d is not empty: %q", c.inv, got)
		}
		for inv, ids := range c.left {
			if got := itemIDs(t, o, `inventory_id=`+dec(inv)); got != ids {
				t.Errorf("clearing %d changed container %d: %q, want %q", c.inv, inv, got, ids)
			}
		}
		assertDBSound(t, o)
	}
}

func TestClearingABaseContainerKeepsItsCurrencyAndEveryOtherContainer(t *testing.T) {
	o := clearOps(t)
	if _, err := o.ClearContainer(clearArgs(7001, 2)); err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(t, o, `inventory_id=7001`); got != "9004" {
		t.Fatalf("the chest keeps its Solari: %q", got)
	}
	if got := itemIDs(t, o, `inventory_id=7003`); got != "9003" {
		t.Fatalf("the refinery is untouched: %q", got)
	}
	if got := itemIDs(t, o, `inventory_id=1`); got != "10,11,12,14" {
		t.Fatalf("the backpack is untouched: %q", got)
	}
	if _, err := o.ClearContainer(clearArgs(7003, 1)); err != nil {
		t.Fatalf("a refinery is a container of the base too: %v", err)
	}
	assertDBSound(t, o)
}

func TestAWrongCountIsRefusedAndNothingChanges(t *testing.T) {
	o := clearOps(t)
	for _, c := range []struct {
		name  string
		count int64
		cur   bool
	}{
		{"fewer than the container holds", 2, false},
		{"more than the container holds", 4, false},
		{"the count that includes the currency, without the flag", 4, false},
		{"the count that leaves the currency out, with the flag", 3, true},
		{"zero for a container that holds items", 0, false},
	} {
		_, err := o.ClearContainer(clearArgs(1, c.count, c.cur))
		if !errors.Is(err, apierr.ErrConflict) {
			t.Errorf("%s: want a conflict, got %v", c.name, err)
		}
		if o.S.Dirty() {
			t.Fatalf("%s: a refused clear must not change the save", c.name)
		}
	}
	if got := itemIDs(t, o, `inventory_id=1`); got != "10,11,12,14" {
		t.Fatalf("backpack changed: %q", got)
	}
}

func TestAnEmptyContainerIsNotAnEditAndSaysSo(t *testing.T) {
	o := clearOps(t)
	res, err := o.ClearContainer(clearArgs(7002, 0))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["removed"].(int64) != 0 || !strings.Contains(m["message"].(string), "Nothing to clear") {
		t.Fatalf("answer: %v", m)
	}
	if o.S.Dirty() || len(o.S.Pending()) != 0 {
		t.Fatal("clearing an empty container is not an edit")
	}
}

func TestAContainerThatHoldsOnlyCurrencyIsNothingToClearUntilCurrencyIsIncluded(t *testing.T) {
	o2 := clearOps(t)
	if _, err := o2.ClearContainer(clearArgs(1, 3)); err != nil {
		t.Fatal(err)
	}
	res, err := o2.ClearContainer(clearArgs(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if msg := res.(map[string]any)["message"].(string); !strings.Contains(msg, "only currency") {
		t.Fatalf("a currency-only container says so: %q", msg)
	}
	if got := len(o2.S.Pending()); got != 1 {
		t.Fatalf("the second call added no edit: %d pending", got)
	}
}

func TestOnlyTheCharactersCarriedContainersAndTheBasesContainersCanBeCleared(t *testing.T) {
	o := clearOps(t)
	for name, inv := range map[string]int64{
		"the unique schematics":            4,
		"world loot":                       7008,
		"a container outside any base":     7006,
		"a container of another base-less": 7005,
		"a generator's fuel slot":          7004,
		"the inventory a bag owns":         40,
		"a hologram's container":           7009,
		"a container with no capacity":     7007,
		"an inventory that does not exist": 31337,
	} {
		_, err := o.ClearContainer(clearArgs(inv, 1))
		if err == nil {
			t.Errorf("%s must be refused", name)
		}
		if o.S.Dirty() {
			t.Fatalf("%s: a refused clear must not change the save", name)
		}
	}
	if _, err := o.ClearContainer(clearArgs(31337, 0)); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("a missing container is not found: %v", err)
	}
	if _, err := o.ClearContainer(clearArgs(4, 1)); !errors.Is(err, apierr.ErrBadArgument) {
		t.Errorf("an out-of-scope container is a bad argument: %v", err)
	}
}

func TestClearContainerRefusesMalformedRequests(t *testing.T) {
	o := clearOps(t)
	for name, a := range map[string]Args{
		"no container":            {"count": 3.0},
		"container zero":          {"inventory_id": 0.0, "count": 0.0},
		"a fractional container":  {"inventory_id": 1.5, "count": 3.0},
		"no count":                {"inventory_id": 1.0},
		"a negative count":        {"inventory_id": 1.0, "count": -1.0},
		"a fractional count":      {"inventory_id": 1.0, "count": 2.5},
		"a count of text":         {"inventory_id": 1.0, "count": "three"},
		"a currency flag of text": {"inventory_id": 1.0, "count": 3.0, "include_currency": "yes"},
		"a currency flag of 1":    {"inventory_id": 1.0, "count": 3.0, "include_currency": 1.0},
		"a huge count":            {"inventory_id": 1.0, "count": 1e300},
	} {
		_, err := o.ClearContainer(a)
		if !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("%s: want a bad argument, got %v", name, err)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused request must not change the save")
	}
}

// The clear reads the container inside the edit, so a queued copy is put back only onto a file where the container still holds what it held.
func TestAQueuedClearIsHeldWhenTheGameAddedAnItemToTheContainer(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := clearOps(t)
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.ClearContainer(clearArgs(1, 3)); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (15,1,1,4,'Rifle',0,1790000000,'{}',0)`)
	mustBeHeld(t, o.S)
}

func TestAQueuedClearIsWrittenWhenTheGameChangedAnotherContainer(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := clearOps(t)
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.ClearContainer(clearArgs(1, 3)); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `update items set stack_size=9 where id=20`)
	if f := o.S.Flush(); !f.Saved || o.S.Held() != nil {
		t.Fatalf("the game changed another container: the clear is written: %+v", f)
	}
	if got := itemIDs(t, o, `inventory_id=1`); got != "10" {
		t.Fatalf("the clear is in the file: %q", got)
	}
	if n := countWhere(t, o, `select stack_size n from items where id=20`); n != 9 {
		t.Fatalf("the game's change is kept: %d", n)
	}
	assertDBSound(t, o)
}

// The clear is put back with the cascade, so the bag's inventory and the blueprint go on the replay as they did the first time.
func TestAQueuedClearIsPutBackWhenTheEditorStartsAgain(t *testing.T) {
	s1 := testsave.PlayerWithSQL(t, clearWorld)
	jp := filepath.Join(t.TempDir(), "pending.json")
	s1.EnableJournal(jp)
	if _, err := (&Ops{S: s1}).ClearContainer(clearArgs(1, 3)); err != nil {
		t.Fatal(err)
	}
	pending := s1.Pending()
	s1.Close()

	s2, err := save.Open(s1.Path) // the file still holds the backpack as it was
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s2.Close)
	s2.EnableJournal(jp)
	o := &Ops{S: s2}
	if got := itemIDs(t, o, `inventory_id=1`); got != "10,11,12,14" {
		t.Fatalf("the file still holds the backpack: %q", got)
	}
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("the clear is put back whole: %+v %v", r, err)
	}
	if got := itemIDs(t, o, `inventory_id=1`); got != "10" {
		t.Fatalf("the backpack is cleared again: %q", got)
	}
	if n := countWhere(t, o, `select count(*) n from inventories where id=40`) + countWhere(t, o, `select count(*) n from building_blueprints`); n != 0 {
		t.Errorf("what hung from the items went too, %d rows remain", n)
	}
	if got := s2.Pending(); len(got) != 1 || got[0] != pending[0] {
		t.Fatalf("the same edit is pending again: %v", got)
	}
	assertDBSound(t, o)
}

func dec(n int64) string { return strconv.FormatInt(n, 10) }
