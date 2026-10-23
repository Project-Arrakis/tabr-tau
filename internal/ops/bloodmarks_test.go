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

// Blood Marks (#344): the item HalloweenCoin, a stack of up to 50,000 in the backpack. The fixture's backpack (inventory 1) has 5 slots, with the Solari stack in slot 0 and a
// Knife in slot 1.

const coinSQL = `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 1, 420, 2, 'HalloweenCoin', '{}');`

// a backpack that is full: the Solari, the Knife and the coin stack fill its three slots
const fullBackpackSQL = `update inventories set max_item_count=3 where id=1;`

func coinStacks(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select stack_size from items where template_id='HalloweenCoin' order by position_index`)
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int64
	for _, r := range rows {
		sizes = append(sizes, r["stack_size"].(int64))
	}
	return fmt.Sprint(sizes)
}

func giveMarks(t *testing.T, o *Ops, n float64) map[string]any {
	t.Helper()
	r, err := o.GiveBloodMarks(Args{"amount": n})
	if err != nil {
		t.Fatalf("give %v: %v", n, err)
	}
	return r.(map[string]any)
}

// otherRows is every row of every table except the Blood Marks stacks and the id sequencer (a new stack takes an id from it), as text, to show that a give changed nothing else.
func otherRows(t *testing.T, o *Ops) string {
	t.Helper()
	var all []string
	for _, tb := range testsave.TableNames(t) {
		q := `select * from "` + tb + `"`
		switch tb {
		case "items":
			q += ` where template_id<>'HalloweenCoin'`
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

// 420 + 500 = 920 on the stack that is there; no second stack is made.
func TestBloodMarksAreAddedToTheStackThatIsThere(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, coinSQL)}
	before := otherRows(t, o)
	r := giveMarks(t, o, 500)
	if r["before"] != int64(420) || r["after"] != int64(920) || r["given"] != int64(500) || r["stacks"] != 0 {
		t.Fatalf("the answer: %v", r)
	}
	if got := coinStacks(t, o); got != "[920]" {
		t.Fatalf("stacks %s, want [920]", got)
	}
	if n := countWhere(t, o, `select count(*) n from items where template_id='HalloweenCoin'`); n != 1 {
		t.Fatalf("%d stacks", n)
	}
	if after := otherRows(t, o); after != before {
		t.Fatalf("something else changed:\n%s\n--- was\n%s", after, before)
	}
	if n := countWhere(t, o, `select next_id n from items_id_sequencer`); n != 500 {
		t.Fatalf("no row was added, so the sequencer stays: %d", n)
	}
	assertDBSound(t, o)
}

// With no stack, one is made in the first free slot of the backpack, and the game's own sequencer gives its id.
func TestBloodMarksMakeAStackWhenThereIsNone(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	before := otherRows(t, o)
	if got := o.bloodMarks(mustPlayer(t, o)); got != 0 {
		t.Fatalf("a backpack with none shows %d", got)
	}
	r := giveMarks(t, o, 500)
	if r["before"] != int64(0) || r["after"] != int64(500) || r["stacks"] != 1 {
		t.Fatalf("the answer: %v", r)
	}
	row := mustOne(t, o, `select id, inventory_id, position_index, stack_size, quality_level from items where template_id='HalloweenCoin'`)
	if row["id"] != int64(500) || row["inventory_id"] != int64(1) || row["position_index"] != int64(2) || row["stack_size"] != int64(500) || row["quality_level"] != int64(0) {
		t.Fatalf("the new stack: %v", row)
	}
	if n := countWhere(t, o, `select next_id n from items_id_sequencer`); n != 501 {
		t.Fatalf("the sequencer moved past the new row: %d", n)
	}
	if after := otherRows(t, o); after != before {
		t.Fatalf("something else changed:\n%s\n--- was\n%s", after, before)
	}
	assertDBSound(t, o)
}

func mustPlayer(t *testing.T, o *Ops) player {
	t.Helper()
	p, err := o.player()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A stack holds 50,000. What does not fit goes on a new stack when the backpack has a free slot (as the general give does), and the whole give is refused when it has none.
func TestBloodMarksFillTheStackToTheLimitAndStartANewOne(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, strings.Replace(coinSQL, "420", "49900", 1))}
	giveMarks(t, o, 500)
	if got := coinStacks(t, o); got != "[50000 400]" {
		t.Fatalf("stacks %s, want [50000 400]", got)
	}
	if got := o.bloodMarks(mustPlayer(t, o)); got != 50_400 {
		t.Fatalf("the figure is the sum of the stacks: %d", got)
	}
	assertDBSound(t, o)
}

func TestBloodMarksThatDoNotFitGiveWhatFitsAndNothingFitsIsRefused(t *testing.T) {
	for _, c := range []struct {
		name          string
		have          int
		amount        float64
		given         int64 // 0: refused
		stackAfter    string
		reasonContain string
	}{
		{"a full stack of 50,000 and no free slot", 50_000, 1, 0, "[50000]", "full"},
		{"49,900 and 500 more: only the 100 that fit on the stack go in", 49_900, 500, 100, "[50000]", "full"},
		{"49,900 and exactly the 100 that fit", 49_900, 100, 100, "[50000]", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := &Ops{S: testsave.PlayerWithSQL(t, fullBackpackSQL+strings.Replace(coinSQL, "420", fmt.Sprint(c.have), 1))}
			before := otherRows(t, o)
			r, err := o.GiveBloodMarks(Args{"amount": c.amount})
			if c.given == 0 {
				if !isNoRoom(err) || !strings.Contains(err.Error(), "Blood Marks") || !strings.Contains(err.Error(), c.reasonContain) {
					t.Fatalf("the person is told why: %v", err)
				}
				if got := coinStacks(t, o); got != fmt.Sprintf("[%d]", c.have) || o.S.Dirty() {
					t.Fatalf("a refused give changes nothing: %s dirty=%v", got, o.S.Dirty())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			m := r.(map[string]any)
			if m["given"] != c.given || m["requested"] != int64(c.amount) || !strings.Contains(m["reason"].(string), c.reasonContain) || (c.reasonContain == "") != (m["limit"] == "") {
				t.Fatalf("the answer: %v", m)
			}
			if m["before"] != int64(c.have) || m["after"] != int64(c.have)+c.given {
				t.Fatalf("before and after: %v", m)
			}
			if got := coinStacks(t, o); got != c.stackAfter {
				t.Fatalf("stacks %s", got)
			}
			if otherRows(t, o) != before {
				t.Fatal("something else changed")
			}
			assertDBSound(t, o)
		})
	}
}

func TestBloodMarksAreRefusedWhenTheBackpackIsFull(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;`)} // the Solari and the Knife fill it
	before := otherRows(t, o)
	_, err := o.GiveBloodMarks(Args{"amount": 100.0})
	if !isNoRoom(err) || !strings.Contains(err.Error(), "full") || !strings.Contains(err.Error(), "Blood Marks") {
		t.Fatalf("a full backpack is refused with the reason: %v", err)
	}
	if n := countWhere(t, o, `select count(*) n from items where template_id='HalloweenCoin'`); n != 0 || o.S.Dirty() {
		t.Fatalf("nothing was added: %d dirty=%v", n, o.S.Dirty())
	}
	if otherRows(t, o) != before {
		t.Fatal("something else changed")
	}
}

// The amount is a whole number from 1 to the size of one stack.
func TestBloodMarksAmountsAreChecked(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	for _, a := range []any{0.0, -5.0, 1.5, 50_001.0, 1e12, "abc", nil, true} {
		_, err := o.GiveBloodMarks(Args{"amount": a})
		if !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("amount %v (%T): %v", a, a, err)
		}
	}
	if _, err := o.GiveBloodMarks(Args{}); !errors.Is(err, apierr.ErrBadArgument) {
		t.Errorf("no amount: %v", err)
	}
	if o.S.Dirty() {
		t.Fatal("a refused amount changes nothing")
	}
	giveMarks(t, o, 50_000)
	if got := coinStacks(t, o); got != "[50000]" {
		t.Fatalf("50,000 is the most one give may add: %s", got)
	}
	giveMarks(t, o, 1)
	if got := coinStacks(t, o); got != "[50000 1]" {
		t.Fatalf("%s", got)
	}
}

// Only the backpack counts and only the backpack is filled: coins in the worn gear or in a base container are neither counted nor topped up.
func TestOnlyTheBackpackCountsForBloodMarks(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into inventories(id, actor_id, inventory_type, max_item_count, max_item_volume) values (2, 2, 1, 10, 1000.0);
		insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (21, 2, 7000, 0, 'HalloweenCoin', '{}');`)}
	if got := o.bloodMarks(mustPlayer(t, o)); got != 0 {
		t.Fatalf("coins outside the backpack are not the backpack's: %d", got)
	}
	r := giveMarks(t, o, 500)
	if r["before"] != int64(0) || r["after"] != int64(500) || r["stacks"] != 1 {
		t.Fatalf("%v", r)
	}
	if n := countWhere(t, o, `select stack_size n from items where id=21`); n != 7000 {
		t.Fatalf("the stack in the other container was touched: %d", n)
	}
	if n := countWhere(t, o, `select count(*) n from items where template_id='HalloweenCoin' and inventory_id=1`); n != 1 {
		t.Fatalf("the new stack is in the backpack: %d", n)
	}
	assertDBSound(t, o)
}

// The figure the Player tab shows is the sum of every stack in the backpack.
func TestThePlayerAnswerCarriesTheBloodMarks(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, coinSQL+`insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (21, 1, 80, 3, 'HalloweenCoin', '{}');`)}
	v, err := o.Player()
	if err != nil {
		t.Fatal(err)
	}
	if got := v.(map[string]any)["bloodMarks"]; got != int64(500) {
		t.Fatalf("420 + 80 = 500, the answer has %v", got)
	}
	o2 := &Ops{S: testsave.Player(t)}
	v, _ = o2.Player()
	if got := v.(map[string]any)["bloodMarks"]; got != int64(0) {
		t.Fatalf("none is 0, the answer has %v", got)
	}
}

// One edit, one line in the write log, in plain words.
func TestAGiveOfBloodMarksIsOneEditWithAPlainDescription(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, coinSQL)}
	giveMarks(t, o, 500)
	if p := o.S.Pending(); len(p) != 1 || p[0] != "gave 500 Blood Marks" {
		t.Fatalf("pending: %v", p)
	}
	giveMarks(t, o, 1_500)
	if p := o.S.Pending(); len(p) != 2 || p[1] != "gave 1,500 Blood Marks" {
		t.Fatalf("pending: %v", p)
	}
	if f := o.S.Flush(); !f.Saved || len(f.Ops) != 2 || f.Ops[0] != "gave 500 Blood Marks" {
		t.Fatalf("%+v", f)
	}
}

// A partial give is still one edit, and the line says what really went in.
func TestAPartialGiveOfBloodMarksIsLoggedWithTheRealAmount(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, fullBackpackSQL+strings.Replace(coinSQL, "420", "49900", 1))}
	giveMarks(t, o, 500)
	if p := o.S.Pending(); len(p) != 1 || p[0] != "gave 100 of 500 Blood Marks" {
		t.Fatalf("pending: %v", p)
	}
}

// The stacks are read inside the edit, so a queued copy is put back only onto a file whose Blood Marks are what they were.
func TestAQueuedGiveIsHeldWhenTheGameChangedTheStack(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, coinSQL)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	giveMarks(t, o, 500)
	gameChanges(t, o.S.Path, `update items set stack_size=900 where id=20`)
	mustBeHeld(t, o.S)
}

func TestAQueuedFirstGiveIsHeldWhenTheGameMadeAStackMeanwhile(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.Player(t)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	giveMarks(t, o, 500)
	gameChanges(t, o.S.Path, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (30, 1, 5, 2, 'HalloweenCoin', '{}')`)
	mustBeHeld(t, o.S)
}

func TestAQueuedGiveIsWrittenWhenTheGameChangedSomethingElse(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, coinSQL)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	giveMarks(t, o, 500)
	gameChanges(t, o.S.Path, `update actors set location_x=location_x+10 where id=1`)
	if f := o.S.Flush(); !f.Saved || o.S.Held() != nil {
		t.Fatalf("the game changed something the give did not read: it is written: %+v", f)
	}
	if got := coinStacks(t, o); got != "[920]" {
		t.Fatalf("%s", got)
	}
	assertDBSound(t, o)
}

// The edit is put back when the editor starts again with the queue still waiting.
func TestAQueuedGiveIsPutBackWhenTheEditorStartsAgain(t *testing.T) {
	s1 := testsave.PlayerWithSQL(t, coinSQL)
	jp := filepath.Join(t.TempDir(), "pending.json")
	s1.EnableJournal(jp)
	giveMarks(t, &Ops{S: s1}, 500)
	pending := s1.Pending()
	s1.Close()

	s2, err := save.Open(s1.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s2.Close)
	s2.EnableJournal(jp)
	o := &Ops{S: s2}
	if got := coinStacks(t, o); got != "[420]" {
		t.Fatalf("the file still holds the old stack: %s", got)
	}
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("put back whole: %+v %v", r, err)
	}
	if got := coinStacks(t, o); got != "[920]" {
		t.Fatalf("%s", got)
	}
	if got := s2.Pending(); len(got) != 1 || got[0] != pending[0] {
		t.Fatalf("the same edit is pending again: %v", got)
	}
	assertDBSound(t, o)
}
