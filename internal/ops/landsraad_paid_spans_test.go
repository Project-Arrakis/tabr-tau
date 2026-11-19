package ops

import (
	"encoding/json"
	"math/rand"
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

// #1002, the merge: a take-back note carries the floor of the stretch that vouched for its row, and that stretch can belong to ANOTHER faction's entry. Merged into the current faction's
// entry it must not stretch that entry down across the ground between the two: a note that ends below where the entry's stretch begins is dropped, not joined to it.
func TestMergePaidANoteBelowTheEntrysStretchDoesNotWidenTheEntry(t *testing.T) {
	e := func(faction int64, paid float64, floor *float64) paidNote {
		return paidNote{Player: 1, Term: 7, House: "A", Faction: faction, Paid: paid, Floor: floor}
	}
	rec := []paidNote{e(1, 2400, fl(0)), e(2, 6000, fl(4800))}
	note := paidNote{Player: 1, Term: 7, House: "A", Faction: 2, Paid: 2400, Back: 2400, Floor: fl(0)}
	got := mergePaid(rec, []paidNote{note})
	if len(got) != 2 {
		t.Fatalf("entries: %+v", got)
	}
	f2 := got[1]
	if f2.Floor == nil || *f2.Floor != 4800 || f2.Paid != 6000 || f2.Back != 2400 {
		t.Fatalf("the entry of the faction was widened or lost the ceiling: %+v", f2)
	}
	spans, _ := spansOf(got, 1, 7, "A", 6000)
	if want := []paidSpan{{0, 2400}, {4800, 6000}}; len(spans) != 2 || spans[0] != want[0] || spans[1] != want[1] {
		t.Fatalf("spans: %v, want %v", spans, want)
	}
	if (groundPaid{spans: spans, known: true}).vouches(3600, 3600) {
		t.Fatal("the gap is vouched for")
	}
	// a note that touches the entry's stretch is still joined to it, and one inside it changes nothing
	if f := mergeFloor(e(2, 6000, fl(4800)), paidNote{Paid: 4800, Floor: fl(3600)}); *f != 3600 {
		t.Fatalf("touching: %v", *f)
	}
	if f := mergeFloor(e(2, 6000, fl(4800)), paidNote{Paid: 5400, Floor: fl(5000)}); *f != 4800 {
		t.Fatalf("inside: %v", *f)
	}
	// a note with no floor, and an entry with none, are as before
	if f := mergeFloor(e(2, 6000, fl(4800)), paidNote{Paid: 2400}); *f != 4800 {
		t.Fatalf("no floor on the note, not higher: %v", *f)
	}
	if mergeFloor(e(2, 6000, nil), paidNote{Paid: 2400, Floor: fl(0)}) != nil {
		t.Fatal("an entry with no floor gained one")
	}
}

// #1011: a take-back note is merged as a take-back: where the faction has an entry it only raises that entry's Back. The note carries the ground and the floor of the stretch that vouched for the
// row, and that stretch can be another faction's; merged as a raise it replaced the faction's own lower stretch, and a row the editor wrote there later read as unsure.
func TestMergePaidATakeBackNoteDoesNotReplaceTheEntrysOwnLowerStretch(t *testing.T) {
	e := func(faction int64, paid float64, floor *float64) paidNote {
		return paidNote{Player: 1, Term: 7, House: "A", Faction: faction, Paid: paid, Floor: floor}
	}
	rec := []paidNote{e(1, 1200, fl(0)), e(2, 4800, fl(3600))}
	note := paidNote{Player: 1, Term: 7, House: "A", Faction: 1, Paid: 4800, Back: 4800, Floor: fl(3600), BackOnly: true}
	got := mergePaid(rec, []paidNote{note})
	if len(got) != 2 {
		t.Fatalf("entries: %+v", got)
	}
	if f1 := got[0]; f1.Paid != 1200 || f1.Floor == nil || *f1.Floor != 0 || f1.Back != 4800 || f1.BackOnly {
		t.Fatalf("the faction's own stretch was replaced or the ceiling is missing: %+v", f1)
	}
	if got[1] != rec[1] && (got[1].Paid != 4800 || *got[1].Floor != 3600) {
		t.Fatalf("the other entry changed: %+v", got[1])
	}
	spans, _ := spansOf(got, 1, 7, "A", 6000)
	if want := []paidSpan{{0, 1200}, {3600, 4800}}; len(spans) != 2 || spans[0] != want[0] || spans[1] != want[1] {
		t.Fatalf("spans: %v, want %v", spans, want)
	}
	if !(groundPaid{spans: spans, known: true}).vouches(1200, 1200) {
		t.Fatal("a row the editor wrote at tier 1,200 is no longer vouched for")
	}
	// the same note as a raise (what an older build does, and what a note without the flag is) shows what the flag prevents
	old := note
	old.BackOnly = false
	if f1 := mergePaid(rec, []paidNote{old})[0]; f1.Paid != 4800 || *f1.Floor != 3600 {
		t.Fatalf("the unflagged note is merged as before: %+v", f1)
	}
	// with no entry of the faction the note is recorded as it was, and the flag is not stored
	only := mergePaid([]paidNote{e(2, 4800, fl(3600))}, []paidNote{note})
	if len(only) != 2 || only[1].Faction != 1 || only[1].Paid != 4800 || only[1].Back != 4800 || only[1].BackOnly {
		t.Fatalf("no entry of the faction: %+v", only)
	}
}

// The same end to end on Kenola: the character's own entry holds [0, 1,200], the other faction's [4,000, 6,000]; the row of tier 4,800 that the other stretch vouched for is taken back, and
// the character's entry keeps its own stretch and gains only the ceiling.
func TestTakeBackNoteOfAnotherFactionsStretchKeepsTheCharactersOwnLowerStretch(t *testing.T) {
	o, path := paidOps(t, "")
	setup(t, o, `update landsraad_task_player_contributions set amount=6000 where player_id=1 and task_id=175`)
	setup(t, o, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 1, 'Schematic.L', 150)`)
	other := paidNote{Player: 1, Term: 7, House: "DA_HouseKenola", Faction: 1, Paid: 6000, Floor: fl(4000)}
	mine := paidNote{Player: 1, Term: 7, House: "DA_HouseKenola", Faction: 2, Paid: 1200, Floor: fl(0)}
	writeRecord(t, o, path, other, mine)
	if m := removeKenola(t, o, 6000, 3600); counts(m) != [4]int64{1, 0, 0, 0} {
		t.Fatalf("the answer: %v", m)
	}
	flush(t, o)
	for _, e := range readPaid(t, path).Entries {
		switch e.Faction {
		case 1:
			if e.Floor == nil || *e.Floor != 4000 || e.Paid != 6000 {
				t.Errorf("the other faction's entry changed: %+v", e)
			}
		case 2:
			if e.Floor == nil || *e.Floor != 0 || e.Paid != 1200 || e.Back != 4800 {
				t.Errorf("the character's own stretch was replaced, or the ceiling is missing: %+v", e)
			}
		}
	}
}

// The case of the review, end to end on Kenola (tiers 1,200 Solari, 2,400 Schematic.K, 3,600, 4,800 Schematic.L): the record holds one faction's raise to 1,200 from 0 and the
// character's own from 3,600 to 4,800; the game wrote Schematic.K (2,400, in the gap) and the character collected Schematic.L. Removing everything takes the Solari row (tier 1,200) and
// leaves K; the note that goes with it must not make the character's entry reach down to 0, or after a raise that pays nothing the second removal would delete the game's K row.
func TestTakeBackNoteOfAnotherFactionsStretchNeverMakesTheGapVouchedForAcrossRemoveAndRaiseCycles(t *testing.T) {
	o, path := paidOps(t, "")
	setup(t, o, `update landsraad_task_player_contributions set amount=4800 where player_id=1 and task_id=175`)
	setup(t, o, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values
		(1, 'DA_HouseKenola', 10000, 'SolarisCoin', 150), (1, 'DA_HouseKenola', 1, 'Schematic.K', 150), (1, 'DA_HouseKenola', 0, 'Schematic.L', 150)`)
	f1 := paidNote{Player: 1, Term: 7, House: "DA_HouseKenola", Faction: 1, Paid: 1200, Floor: fl(0)}
	f2 := paidNote{Player: 1, Term: 7, House: "DA_HouseKenola", Faction: 2, Paid: 4800, Floor: fl(3600)}
	writeRecord(t, o, path, f1, f2)
	for cycle := 1; cycle <= 3; cycle++ {
		m := removeKenola(t, o, 4800, 0)
		if got := rowOf(t, o, "Schematic.K"); got == "" {
			t.Fatalf("cycle %d: the game's Schematic.K row (tier 2,400, in the gap) was deleted: %v", cycle, m)
		}
		if cycle == 1 && rowOf(t, o, "SolarisCoin") != "" {
			t.Fatalf("the Solari row (tier 1,200, the other faction's stretch) was not taken back: %v", m)
		}
		flush(t, o)
		for _, e := range readPaid(t, path).Entries {
			if e.Faction == 2 && (e.Floor == nil || *e.Floor != 3600 || e.Paid != 4800) {
				t.Fatalf("cycle %d: the entry of the character's faction was widened: %+v", cycle, e)
			}
		}
		fillKenolaDoubled(t, o, 0, 4800)
		flush(t, o)
	}
}

// No sequence of raises and take-backs, under either faction, ever makes the record vouch for a tier no raise of the editor paid (#1002). The notes are the ones the editor writes (a raise
// carries the floor it began from, a take-back the floor of the stretch that vouched for its highest tier), merged in order the way the record merges them.
func TestPaidNoSequenceOfRaisesAndTakeBacksVouchesForGroundNoRaisePaid(t *testing.T) {
	tiers := []float64{0, 1200, 2400, 3600, 4800, 6000}
	r := newSeededRand(7)
	for run := 0; run < 3000; run++ {
		var rec []paidNote
		paid := map[int]bool{} // tier index paid by some raise: lo < th <= hi
		for step := 0; step < 14; step++ {
			faction := int64(1 + r.Intn(2))
			if r.Intn(2) == 0 { // a raise from lo to hi
				lo, hi := r.Intn(5), 1+r.Intn(5)
				if hi <= lo {
					lo, hi = hi-1, lo+1
					if lo < 0 || hi > 5 {
						continue
					}
				}
				fl := tiers[lo]
				rec = mergePaid(rec, []paidNote{{Player: 1, Term: 7, House: "A", Faction: faction, Paid: tiers[hi], Floor: &fl}})
				for k := lo + 1; k <= hi; k++ {
					paid[k] = true
				}
			} else { // a take-back of the highest vouched tier at or below a random one
				k := 1 + r.Intn(5)
				spans, _ := spansOf(rec, 1, 7, "A", 6000)
				g := groundPaid{spans: spans, known: true}
				if !g.vouches(tiers[k], tiers[k]) {
					continue
				}
				sp, _ := g.spanOf(tiers[k], tiers[k])
				lo := sp.lo
				rec = mergePaid(rec, []paidNote{{Player: 1, Term: 7, House: "A", Faction: faction, Paid: tiers[k], Back: tiers[k], Floor: &lo, BackOnly: true}})
			}
			spans, _ := spansOf(rec, 1, 7, "A", 6000)
			g := groundPaid{spans: spans, known: true}
			for k := 1; k <= 5; k++ {
				if g.vouches(tiers[k], tiers[k]) && !paid[k] {
					t.Fatalf("run %d step %d: tier %v is vouched for but no raise paid it: record %+v spans %v", run, step, tiers[k], rec, spans)
				}
			}
		}
	}
}

func newSeededRand(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }
