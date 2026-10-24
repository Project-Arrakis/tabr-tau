package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// F-17 and DBA-8 of #18 / #258: a save is data from outside. Its tables are not compared with the game's at open, so a column the game declares NOT NULL can hold NULL
// (or text) in a file built by hand. A value read from it must be checked before it is used as a number: an unchecked assertion panics, and net/http answers a panic with a
// reset connection that the page cannot explain.

// noPanic runs f and turns a panic into a test failure that names it.
func noPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s panicked: %v", what, r)
		}
	}()
	f()
}

func TestAGiveSurvivesAnItemIdCounterThatHoldsNoNumber(t *testing.T) {
	for name, row := range map[string]string{"NULL": "NULL", "text": "'x'"} {
		t.Run(name, func(t *testing.T) {
			o := &Ops{S: testsave.PlayerWithSQL(t, `drop table items_id_sequencer; create table items_id_sequencer(next_id); insert into items_id_sequencer values (`+row+`);`)}
			var err error
			noPanic(t, "Give", func() { _, err = o.GiveItem(Args{"template_id": "Knife", "quantity": 1.0}) })
			if err != nil {
				t.Fatalf("a counter with no number is rebuilt from the highest id, not a reason to refuse: %v", err)
			}
			r, err := o.S.One(`select max(id) m, (select next_id from items_id_sequencer) n, (select typeof(next_id) from items_id_sequencer) ty from items`)
			if err != nil || r == nil {
				t.Fatal(err)
			}
			if r["m"].(int64) <= 11 || r["n"].(int64) <= r["m"].(int64) || r["ty"] != "integer" {
				t.Fatalf("the new item has a fresh id and the counter points after it: %v", r)
			}
		})
	}
}

func TestThePlayerRecordWithoutAnAccountIsAnErrorNotAPanic(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `pragma foreign_keys=off; drop table player_state;
		create table player_state(id integer primary key, account_id, character_name, player_controller_id, player_pawn_id, player_state_id);
		insert into player_state(id, account_id, character_name, player_controller_id, player_pawn_id, player_state_id) values (1, NULL, 'Tester', 1, 2, 3);`)}
	var err error
	noPanic(t, "player", func() { _, err = o.player() })
	if err == nil || !strings.Contains(err.Error(), "player") {
		t.Fatalf("a player with no account id is refused with a sentence: %v", err)
	}
}

// The time of a new item is a number: strftime answers text, and a column without a declared type would keep it as text.
func TestANewItemsTimeIsStoredAsANumber(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `pragma foreign_keys=off; drop table items;
		create table items(id integer primary key, inventory_id, stack_size, position_index, template_id, is_new, acquisition_time, stats, quality_level, volume_override);
		insert into items values (10, 1, 100, 0, 'SolarisCoin', 0, 1790000000, '{"FItemStackAndDurabilityStats":[[],{}],"FCustomizationStats":[[],{}]}', 0, NULL);`)}
	tmpl := oneSlotItem(t)
	if _, err := o.GiveItem(Args{"template_id": tmpl, "quantity": 1.0}); err != nil {
		t.Fatal(err)
	}
	rows, err := o.S.Query(`select id, typeof(acquisition_time) ty from items where template_id=?`, tmpl)
	if err != nil || len(rows) != 1 {
		t.Fatalf("%v %v", rows, err)
	}
	if rows[0]["ty"] != "integer" {
		t.Fatal(fmt.Sprint("acquisition_time is stored as ", rows[0]["ty"]))
	}
}
