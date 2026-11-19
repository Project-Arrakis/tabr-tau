package ops

import (
	"encoding/json"
	"os"
	"testing"
)

// The record vouches for stretches of ground, one per entry (#1002): the lowest floor over every faction's entry no longer vouches for the ground between two stretches.

func fl(x float64) *float64 { return &x }

func TestPaidSpansOfTheRecord(t *testing.T) {
	e := func(faction int64, paid float64, floor *float64) paidNote {
		return paidNote{Player: 1, Term: 7, House: "A", Faction: faction, Paid: paid, Floor: floor}
	}
	for _, c := range []struct {
		name  string
		es    []paidNote
		top   float64
		spans []paidSpan
		known bool
	}{
		{"the case of #1002: two stretches with a gap between them", []paidNote{e(1, 2400, fl(1300)), e(2, 6000, fl(5000))}, 6000, []paidSpan{{1300, 2400}, {5000, 6000}}, true},
		{"the same, given in the other order", []paidNote{e(2, 6000, fl(5000)), e(1, 2400, fl(1300))}, 6000, []paidSpan{{1300, 2400}, {5000, 6000}}, true},
		{"stretches that touch are one", []paidNote{e(1, 2400, fl(0)), e(2, 4800, fl(2400))}, 6000, []paidSpan{{0, 4800}}, true},
		{"stretches that overlap are one", []paidNote{e(1, 2400, fl(0)), e(2, 3600, fl(1200))}, 6000, []paidSpan{{0, 3600}}, true},
		{"one inside the other", []paidNote{e(1, 6000, fl(0)), e(2, 3600, fl(1200))}, 6000, []paidSpan{{0, 6000}}, true},
		{"the highest tier that stands cuts every stretch", []paidNote{e(1, 2400, fl(1300)), e(2, 6000, fl(5000))}, 3600, []paidSpan{{1300, 2400}}, true},
		{"a stretch the cut leaves empty is gone", []paidNote{e(1, 6000, fl(4800))}, 4800, nil, true},
		{"a floor equal to the ground paid vouches for nothing", []paidNote{e(1, 2400, fl(2400))}, 6000, nil, true},
		{"an entry that does not know where it started makes the whole unknown", []paidNote{e(1, 2400, fl(0)), e(2, 6000, nil)}, 6000, nil, false},
		{"nothing recorded", nil, 6000, nil, false},
		{"another house, term or character is not this one's", []paidNote{{Player: 2, Term: 7, House: "A", Faction: 1, Paid: 2400, Floor: fl(0)}, {Player: 1, Term: 8, House: "A", Faction: 1, Paid: 2400, Floor: fl(0)}, {Player: 1, Term: 7, House: "B", Faction: 1, Paid: 2400, Floor: fl(0)}}, 6000, nil, false},
	} {
		got, known := spansOf(c.es, 1, 7, "A", c.top)
		if known != c.known || len(got) != len(c.spans) {
			t.Errorf("%s: %v %v, want %v %v", c.name, got, known, c.spans, c.known)
			continue
		}
		for i := range got {
			if got[i] != c.spans[i] {
				t.Errorf("%s: %v, want %v", c.name, got, c.spans)
			}
		}
	}
}

func TestPaidAStretchHoldsTheTiersOfARowOnlyWhole(t *testing.T) {
	g := groundPaid{spans: []paidSpan{{1300, 2400}, {5000, 6000}}, known: true}
	for _, c := range []struct {
		low, top float64
		want     bool
	}{
		{1500, 2400, true}, {2400, 2400, true}, {5100, 6000, true},
		{1300, 2400, false}, // the first tier of the stretch is at its start: the game may have written it
		{3000, 3000, false}, // in the gap between the two stretches
		{2400, 3600, false}, // begins in a stretch and ends in the gap
		{2000, 5500, false}, // two stretches
		{4800, 6000, false}, // at or below where the second began
		{1500, 2500, false}, // above what the first paid
	} {
		if got := g.vouches(c.low, c.top); got != c.want {
			t.Errorf("tiers %v to %v: %v, want %v", c.low, c.top, got, c.want)
		}
	}
	if (groundPaid{spans: g.spans}).vouches(1500, 2400) {
		t.Error("a record that does not know where it started vouched for a tier")
	}
	if s, ok := g.spanOf(5500, 5500); !ok || s.lo != 5000 {
		t.Errorf("the stretch of a tier: %v %v", s, ok)
	}
}

// What the record says by hand, for a test that needs two factions' entries: it carries the save's path the way the ledger writes it.
func writeRecord(t *testing.T, o *Ops, path string, es ...paidNote) {
	t.Helper()
	b, err := json.Marshal(paidFile{Version: paidFileVersion, Save: o.Paid.save, Entries: es})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The case of #1002 through the board and the write: another tool set the contribution to 6,000 with no events and left a waiting row of tier 3,600; the record holds the editor's raise of
// 1,300 to 2,400 under one faction and of 4,800 to 6,000 under the character's. Tier 3,600 lies in the gap between the two stretches: nobody vouches for it, so the removal leaves the row.
func TestTakeBackNeverTakesARowInTheGapBetweenTwoFactionsStretches(t *testing.T) {
	for _, c := range []struct {
		name  string
		other paidNote // the other faction's entry
		take  bool
	}{
		{"a gap", paidNote{Faction: 1, Paid: 2400, Floor: fl(1300)}, false},
		{"stretches that touch: the row is in a stretch", paidNote{Faction: 1, Paid: 4800, Floor: fl(1300)}, true},
	} {
		o, path := paidOps(t, "")
		setup(t, o, `update landsraad_task_player_contributions set amount=6000 where player_id=1 and task_id=175`)
		setup(t, o, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 50, 'T6RangeFinder', 150)`)
		other, mine := c.other, paidNote{Faction: 2, Paid: 6000, Floor: fl(4800)}
		if !c.take {
			mine.Floor = fl(4800)
		} else {
			mine.Floor = fl(3000) // continues the first stretch
		}
		for _, e := range []*paidNote{&other, &mine} {
			e.Player, e.Term, e.House = 1, 7, "DA_HouseKenola"
		}
		writeRecord(t, o, path, other, mine)
		a, w := outcomeOf(t, o, 3600)
		if c.take != (a == tbTake) || (!c.take && (a != tbUnsure || w != tbWhyBelow)) {
			t.Errorf("%s: the board, tier 3,600: %q %q", c.name, a, w)
		}
		m := removeKenola(t, o, 6000, 2400)
		got := counts(m)
		wantRows := [4]int64{0, 0, 1, 0}
		if c.take {
			wantRows = [4]int64{1, 0, 0, 0}
		}
		if got != wantRows {
			t.Errorf("%s: the answer counts %v, want %v", c.name, got, wantRows)
		}
		if gone := rowOf(t, o, "T6RangeFinder") == ""; gone != c.take {
			t.Errorf("%s: the waiting row of tier 3,600 is gone: %v, want %v", c.name, gone, c.take)
		}
	}
}

// A take-back writes the floor of the stretch that vouched for its highest tier into the record, not the lowest floor of every faction's entry: the other faction's low stretch is not
// claimed for this faction by it.
func TestTakeBackTheNoteCarriesTheFloorOfTheStretchThatVouched(t *testing.T) {
	o, path := paidOps(t, "")
	setup(t, o, `update landsraad_task_player_contributions set amount=6000 where player_id=1 and task_id=175`)
	setup(t, o, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 1, 'Schematic.L', 150)`)
	other := paidNote{Player: 1, Term: 7, House: "DA_HouseKenola", Faction: 1, Paid: 2400, Floor: fl(1300)}
	mine := paidNote{Player: 1, Term: 7, House: "DA_HouseKenola", Faction: 2, Paid: 6000, Floor: fl(4000)}
	writeRecord(t, o, path, other, mine)
	if m := removeKenola(t, o, 6000, 3600); counts(m) != [4]int64{1, 0, 0, 0} { // tier 4,800 is in the stretch (4,000, 6,000]
		t.Fatalf("the answer: %v", m)
	}
	flush(t, o)
	f := readPaid(t, path)
	for _, e := range f.Entries {
		switch e.Faction {
		case 1:
			if e.Floor == nil || *e.Floor != 1300 || e.Paid != 2400 {
				t.Errorf("the other faction's entry changed: %+v", e)
			}
		case 2:
			if e.Floor == nil || *e.Floor != 4000 || e.Back != 4800 {
				t.Errorf("this faction's entry carries the floor of its own stretch and the ceiling of the take-back: %+v", e)
			}
		}
	}
}
