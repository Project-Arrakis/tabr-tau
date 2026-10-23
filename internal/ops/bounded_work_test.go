package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// SEC-11 and QA-13 of the 2026-10-22 audit: the work one request can put on the save's single lock is bounded.

// oneSlotItem is an item of the catalog that holds one per slot (gear), which a give turns into one row per piece.
func oneSlotItem(t *testing.T) string {
	t.Helper()
	for _, e := range catalog.All() {
		if n, ok := catalog.MaxStack(e.ID); ok && n == 1 && templateRe.MatchString(e.ID) {
			return e.ID
		}
	}
	t.Fatal("no one-per-slot item in the catalog")
	return ""
}

const noSlotLimitSQL = `update inventories set max_item_count=0 where id=1;`

func TestAWriteStatementThatNeverEndsIsStoppedAndNothingIsChanged(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	old := SQLWriteTimeout
	SQLWriteTimeout = 300 * time.Millisecond
	t.Cleanup(func() { SQLWriteTimeout = old })
	before := stackOf(t, o, 10)
	start := time.Now()
	_, err := o.ExecSQL(`update items set stack_size=(with recursive c(x) as (select 1 union all select x+1 from c) select count(*) from c) where id=10`)
	if err == nil || !strings.Contains(err.Error(), "was stopped after") || !strings.Contains(err.Error(), "nothing was changed") {
		t.Fatalf("the statement is stopped and the person is told: %v", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("it took %s to stop", d)
	}
	if o.S.Dirty() || stackOf(t, o, 10) != before {
		t.Fatal("a stopped statement changes nothing")
	}
	if _, err := o.ExecSQL(`update items set stack_size=stack_size+1 where id=10`); err != nil { // the lock was let go
		t.Fatalf("the next edit works: %v", err)
	}
}

// Before the fix a give read every item of the inventory again for each piece it added: the work grew with the square of the number of pieces. Now the inventory is read once
// per give, which the journal shows (it keeps a record of every read an edit made).
func TestGivingManyOneSlotItemsReadsTheInventoryOnceNotOncePerPiece(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, noSlotLimitSQL)}
	jp := filepath.Join(t.TempDir(), "pending.json")
	o.S.EnableJournal(jp)
	tmpl := oneSlotItem(t)
	if _, err := o.GiveItem(Args{"template_id": tmpl, "quantity": 600.0}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(jp)
	if err != nil {
		t.Fatal(err)
	}
	var j struct {
		Ops []struct {
			Stmts []struct {
				Rd bool `json:"rd"`
			} `json:"stmts"`
		} `json:"ops"`
	}
	if err := json.Unmarshal(b, &j); err != nil {
		t.Fatal(err)
	}
	reads, writes := 0, 0
	for _, op := range j.Ops {
		for _, st := range op.Stmts {
			if st.Rd {
				reads++
			} else {
				writes++
			}
		}
	}
	if reads > 20 {
		t.Fatalf("%d reads for 600 pieces: the inventory is read again for each piece", reads)
	}
	if writes < 600 {
		t.Fatalf("only %d writes for 600 pieces", writes)
	}
	r := mustOne(t, o, `select count(*) n, count(distinct position_index) slots, min(position_index) lo, max(position_index) hi, max(id) maxid from items where template_id=?`, tmpl)
	if r["n"].(int64) != 600 || r["slots"].(int64) != 600 || r["lo"].(int64) < 0 {
		t.Fatalf("600 pieces in 600 different slots: %v", r)
	}
	seq := mustOne(t, o, `select next_id n from items_id_sequencer`)["n"].(int64)
	if seq != r["maxid"].(int64)+1 {
		t.Fatalf("the game's id sequencer points after the last id used: %d, last id %v", seq, r["maxid"])
	}
	assertDBSound(t, o)
}

// A give past the bound gives up to the bound and says so (it used to be refused whole); the strict give that Solari use still refuses (TestSolariPastTheBoundIsRefusedWhole).
func TestAGiveOfMoreStacksThanTheBoundGivesTheBoundAndSaysSoAndTheBoundItselfIsAccepted(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, noSlotLimitSQL)}
	tmpl := oneSlotItem(t)
	r, err := o.GiveItem(Args{"template_id": tmpl, "quantity": float64(maxGiveStacks + 1)})
	if err != nil {
		t.Fatalf("a give past the bound gives what the bound allows: %v", err)
	}
	if m := r.(map[string]any); m["given"] != int64(maxGiveStacks) || m["requested"] != int64(maxGiveStacks+1) || m["limit"] != "bound" || !strings.Contains(m["reason"].(string), "at most 1,000 stacks") {
		t.Fatalf("the answer says what was given and why not more: %v", r)
	}
	if n := mustOne(t, o, `select count(*) n from items where template_id=?`, tmpl)["n"].(int64); n != maxGiveStacks {
		t.Fatalf("the bound was given: %d", n)
	}
	if _, err := o.S.Mutate("clean", func(m *save.Mut) error { _, e := m.Exec(`delete from items where template_id=?`, tmpl); return e }); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := o.GiveItem(Args{"template_id": tmpl, "quantity": float64(maxGiveStacks)}); err != nil {
		t.Fatalf("the bound itself is accepted: %v", err)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("a give at the bound took %s", d)
	}
	if n := mustOne(t, o, `select count(*) n from items where template_id=?`, tmpl)["n"].(int64); n != maxGiveStacks {
		t.Fatalf("%d pieces", n)
	}
}
