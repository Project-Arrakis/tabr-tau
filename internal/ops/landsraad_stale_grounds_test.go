package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// #805: a raise is compared with the two other grounds the sentence before the click is worked out from, like was_events: what the editor itself paid before (was_paid, and
// was_paid_rows for the ground whose reward rows are delivered) and what the character reached under another faction (was_elsewhere). A page that shows another number than the save holds
// is refused with nothing written; a request that does not send them is not refused; a removal never depends on them.

// the numbers the board shows for Kenola, as the page would send them back
func boardGrounds(t *testing.T, o *Ops) Args {
	t.Helper()
	b := taskByID(t, o, 175)
	return Args{"was_paid": b["my_paid"], "was_paid_rows": b["my_paid_rows"], "was_elsewhere": b["my_elsewhere"]}
}

func withGrounds(g Args, extra Args) Args {
	out := Args{}
	for k, v := range g {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestStaleGroundsTheNumbersTheBoardShowsAreAcceptedAndStaleOnesRefused(t *testing.T) {
	// the editor paid 1,300 to 6,000, then took the tiers back down to 2,400 (the paid ground stays 6,000, the rows delivered at the House go to none), and the character also reached 2,400 under
	// the other faction
	o, _ := paidOps(t, `insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 1, 175, 2400);`)
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	removeKenola(t, o, 6000, 2400)
	flush(t, o)
	g := boardGrounds(t, o)
	if g["was_paid"] != 6000.0 || g["was_paid_rows"] != 0.0 || g["was_elsewhere"] != 2400.0 {
		t.Fatalf("the board does not show the grounds the test needs: %v", g)
	}
	raise := func(extra Args) error {
		_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, withGrounds(Args{"was_mine": 2400.0, "mine_to": 6000.0}, extra))))
		return err
	}
	before := grantRows(t, o)
	for _, c := range []struct {
		name  string
		extra Args
		say   string
	}{
		{"the ground the editor paid", Args{"was_paid": 0.0}, "the ground the editor paid before on Kenola is not what you were shown"},
		{"the ground the editor paid, a little less", Args{"was_paid": 4800.0}, "the ground the editor paid before"},
		{"the ground whose rows are delivered", Args{"was_paid_rows": 2400.0}, "the ground the editor paid before"},
		{"the ground reached under another faction", Args{"was_elsewhere": 0.0}, "what you reached on Kenola under another faction is not what you were shown"},
	} {
		err := raise(withGrounds(g, c.extra))
		if err == nil {
			t.Fatalf("%s: the raise was not refused", c.name)
		}
		wantRefusal2(t, err, apierr.ErrConflict, c.say, "nothing was changed", "The board was read again")
		if grantRows(t, o) != before {
			t.Fatalf("%s: a refused request changed the save", c.name)
		}
	}
	// each number alone is compared too
	for _, k := range []string{"was_paid", "was_paid_rows", "was_elsewhere"} {
		if err := raise(Args{k: 1.0}); err == nil {
			t.Fatalf("%s alone: the raise was not refused", k)
		}
	}
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
	// a removal does not depend on them
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, withGrounds(Args{"was_mine": 2400.0, "mine_to": 1200.0, "lower": true}, Args{"was_paid": 0.0, "was_paid_rows": 0.0, "was_elsewhere": 0.0})))); err != nil {
		t.Fatalf("a removal with stale grounds: %v", err)
	}
	// what the board shows is accepted
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, withGrounds(Args{"was_mine": 1200.0, "mine_to": 6000.0}, boardGrounds(t, o))))); err != nil {
		t.Fatalf("the numbers the board shows: %v", err)
	}
}

func TestStaleGroundsARequestWithoutTheNumbersIsNotRefused(t *testing.T) {
	o, _ := paidOps(t, `insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 1, 175, 2400);`)
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	removeKenola(t, o, 6000, 2400)
	flush(t, o)
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 2400.0, "mine_to": 6000.0}))); err != nil {
		t.Fatalf("no numbers: %v", err)
	}
	// null is the same as not sent
	o2, _ := paidOps(t, "")
	if _, err := o2.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0, "was_paid": nil, "was_paid_rows": nil, "was_elsewhere": nil}))); err != nil {
		t.Fatalf("null numbers: %v", err)
	}
}

// A second tab showed the board before the other tab raised the same house: the number the page holds is behind the record, so the raise is refused instead of paying fewer missions than
// its sentence said. This is the case of the issue.
func TestStaleGroundsASecondTabThatRaisedTheHouseFirstRefusesTheFirstTabsRaise(t *testing.T) {
	o, _ := paidOps(t, "")
	shown := boardGrounds(t, o) // the first tab: nothing paid yet
	if shown["was_paid"] != 0.0 {
		t.Fatalf("the board before any raise: %v", shown)
	}
	fillKenolaDoubled(t, o, 1300, 4800) // the second tab
	flush(t, o)
	removeKenola(t, o, 4800, 1200)
	flush(t, o)
	before := grantRows(t, o)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, withGrounds(Args{"was_mine": 1200.0, "mine_to": 6000.0}, shown))))
	wantRefusal2(t, err, apierr.ErrConflict, "the ground the editor paid before", "nothing was changed")
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
	// the page that was read again pays what its sentence says
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, withGrounds(Args{"was_mine": 1200.0, "mine_to": 6000.0}, boardGrounds(t, o))))); err != nil {
		t.Fatal(err)
	}
}

// A queued raise whose record is not written yet is part of the ground the board shows, and the write counts it the same way, so the page that read the board after the queued raise is not refused.
func TestStaleGroundsTheGroundOfAQueuedRaiseIsTheSameOnTheBoardAndInTheWrite(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 4800) // queued, the file not written, so the record not either
	g := boardGrounds(t, o)
	if g["was_paid"] != 4800.0 {
		t.Fatalf("the board does not count the queued raise: %v", g)
	}
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, withGrounds(Args{"was_mine": 4800.0, "mine_to": 6000.0}, g)))); err != nil {
		t.Fatalf("the numbers the board shows after a queued raise: %v", err)
	}
}

func TestStaleGroundsANumberThatIsNotANumberIsABadRequest(t *testing.T) {
	o, _ := paidOps(t, "")
	for _, k := range []string{"was_paid", "was_paid_rows", "was_elsewhere"} {
		_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0, k: "lots"})))
		wantRefusal2(t, err, apierr.ErrBadArgument)
	}
}
