package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Group D1 of the delta audit (#611), the rest of it: a win taken away is named (D1-3), the term window and the clock repair are said (D1-4), a single completion checks term and
// faction (D1-5) and every Landsraad edit says which houses it is about (D1-6). The worlds are the ones of landsraad_goal_test.go (character in Harkonnen, 2; Atreides is 1;
// current term 7 of 100 to 200; houses 174, 175, 176; 149 of the earlier term 6) and landsraad_clock_test.go (term 10, two houses holding Unix time). Every test runs on a temp copy.

// the description of the last edit in the queue
func lastEdit(t *testing.T, o *Ops) string {
	t.Helper()
	p := o.S.Pending()
	if len(p) == 0 {
		t.Fatal("no edit was made")
	}
	return p[len(p)-1]
}

// house 175 (Kenola) was won by Atreides before the test starts
func wonByAtreides(t *testing.T, o *Ops) {
	t.Helper()
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=1, completion_time=150 where id=175`)
}

func TestTheBoardApplyNamesTheWinItTakesAway(t *testing.T) {
	o := goalOps(t)
	wonByAtreides(t, o)
	m := mustApply(t, o, applyArgs(house(175, Args{"complete_for": 2.0, "was_done": true, "was_win": 1.0})))
	r := m["replaced"].([]map[string]any)
	if len(r) != 1 || r[0]["house"] != "Kenola" || r[0]["previous_winner"] != "Atreides" {
		t.Fatalf("answer: %v", m)
	}
	if d := lastEdit(t, o); !strings.Contains(d, "replaces the win of Atreides on Kenola") {
		t.Fatalf("write log: %q", d)
	}
}

// ---------------------------------------------------------------- D1-4: the term window and the clock repair

func TestSetTermRefusesAnEndBeforeTheStartAndWritesNothing(t *testing.T) {
	o := goalOps(t)
	before := mustOne(t, o, `select start_time s, end_time e from landsraad_decree_term where term_id=7`)
	_, err := o.SetTerm(Args{"end_time": 99.0}) // the term starts at 100
	if !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("an end before the start is refused: %v", err)
	}
	if after := mustOne(t, o, `select start_time s, end_time e from landsraad_decree_term where term_id=7`); after["e"] != before["e"] {
		t.Fatalf("nothing is written: %v -> %v", before, after)
	}
	if len(o.S.Pending()) != 0 {
		t.Fatal("nothing is queued")
	}
	// the same call that also sets a decree writes nothing at all
	if _, err := o.SetTerm(Args{"reigning_faction_id": 1.0, "end_time": 5.0}); err == nil {
		t.Fatal("refused")
	}
	if r := mustOne(t, o, `select reigning_faction_id f from landsraad_decree_term where term_id=7`); r["f"] != nil {
		t.Fatalf("the other fields of a refused request are not written: %v", r)
	}
}

func TestSetTermAcceptsAnEndAtOrAfterTheStart(t *testing.T) {
	o := goalOps(t)
	for _, e := range []float64{100, 250} {
		if _, err := o.SetTerm(Args{"end_time": e}); err != nil {
			t.Fatalf("end %v: %v", e, err)
		}
		if r := mustOne(t, o, `select end_time e from landsraad_decree_term where term_id=7`); r["e"].(int64) != int64(e) {
			t.Fatalf("end %v: %v", e, r)
		}
	}
}

func TestTheClockRepairIsSaidInTheAnswerAndTheWriteLog(t *testing.T) {
	// houses 228 and 240 hold Unix time, outside the window of term 10; completing 233 and 234 moves those two as well
	o := clockOps(t)
	got, err := completeBoth(o, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := got.(map[string]any)["repaired"].(int64); n != 2 {
		t.Fatalf("answer: %v", got)
	}
	if d := lastEdit(t, o); !strings.Contains(d, "2 other completion times moved") {
		t.Fatalf("write log: %q", d)
	}

	o = clockOps(t)
	got, err = clockApply(o, house(233, Args{"complete_for": 2.0}))
	if err != nil {
		t.Fatal(err)
	}
	if n := got.(map[string]any)["repaired"].(int64); n != 2 {
		t.Fatalf("single completion answer: %v", got)
	}
	if d := lastEdit(t, o); !strings.Contains(d, "2 other completion times") {
		t.Fatalf("write log: %q", d)
	}

	// a reopen that changes nothing else still says what it moved
	o = clockOps(t)
	got, err = clockApply(o, house(233, Args{"reopen": true}))
	if err != nil {
		t.Fatal(err)
	}
	if n := got.(map[string]any)["repaired"].(int64); n != 2 {
		t.Fatalf("answer: %v", got)
	}
	if d := lastEdit(t, o); d != "landsraad: 2 other completion times moved" {
		t.Fatalf("write log: %q", d)
	}
}

func TestNothingToRepairLeavesTheWriteLogAlone(t *testing.T) {
	o := clockOps(t, `update landsraad_tasks set completion_time=312000 where id in (228, 240)`)
	if _, err := completeBoth(o, false); err != nil {
		t.Fatal(err)
	}
	if d := lastEdit(t, o); strings.Contains(d, "moved") {
		t.Fatalf("write log: %q", d)
	}
}

// the list of houses in a description is short
func TestAListOfManyHousesInADescriptionIsCutShort(t *testing.T) {
	if got := listText([]string{"a", "b", "c", "d", "e", "f", "g", "h"}); got != "a, b, c, d, e, f and 2 more" {
		t.Fatal(got)
	}
	if got := listText([]string{"a", "b"}); got != "a, b" {
		t.Fatal(got)
	}
}

// ---------------------------------------------------------------- the board apply, the only write the page makes

func TestBoardApplyDescribesOneHouseByWhatWasDoneToIt(t *testing.T) {
	for _, c := range []struct {
		name string
		h    map[string]any
		want string
	}{
		{"complete", house(175, Args{"complete_for": 2.0}), "landsraad Kenola: completed for Harkonnen"},
		{"reopen", house(174, Args{"was_done": true, "reopen": true}), "landsraad Maros: reopened"},
		{"raise", house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0}), "landsraad Kenola: your contribution 1300 -> 2400; 1 mission: +6,000 Solari, +1,000 House Scrip, +1,500 Gathering XP; 1 reward row"},
		{"lower", house(175, Args{"was_mine": 1300.0, "mine_to": 1200.0, "lower": true}), "landsraad Kenola: your contribution 1300 -> 1200"},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := goalOps(t)
			mustApply(t, o, applyArgs(c.h))
			if d := lastEdit(t, o); d != c.want {
				t.Fatalf("write log %q, want %q", d, c.want)
			}
		})
	}
}

func TestBoardApplyDescribesSeveralHousesByTheirNames(t *testing.T) {
	o := goalOps(t)
	mustApply(t, o, applyArgs(house(175, Args{"complete_for": 2.0}), house(176, Args{"complete_for": 2.0})))
	if d := lastEdit(t, o); d != "landsraad: 2 houses completed for Harkonnen; houses: Kenola, Vernius" {
		t.Fatalf("write log: %q", d)
	}
}

func TestBoardApplyOfAHouseAlreadyInThatStateAddsNoDescription(t *testing.T) {
	o := goalOps(t)
	mustApply(t, o, applyArgs(house(176, Args{"was_done": false, "reopen": true})))
	if len(o.S.Pending()) != 0 {
		t.Fatalf("nothing changed, nothing is queued: %v", o.S.Pending())
	}
}

func TestWinsTakenAwayAreGroupedByTheFactionThatHeldThem(t *testing.T) {
	if got := replacedText([]replacedWin{{"A", "Atreides"}, {"B", "Atreides"}, {"C", "Harkonnen"}, {"D", "Atreides"}}); got != "; replaces 4 wins (Atreides: A, B and 1 more; Harkonnen: C)" {
		t.Fatal(got)
	}
	if got := replacedText([]replacedWin{{"A", "Atreides"}}); got != "; replaces the win of Atreides on A" {
		t.Fatal(got)
	}
}

func TestAHouseNameIsClippedInADescription(t *testing.T) {
	long := strings.Repeat("x", 90)
	if got := houseOf(map[string]any{"house_name": "DA_House" + long}); len([]rune(got)) != maxHouseName+3 || !strings.HasSuffix(got, "...") {
		t.Fatal(got)
	}
	if got := houseOf(map[string]any{"house_name": "DA_HouseKenola"}); got != "Kenola" {
		t.Fatal(got)
	}
}

// The write log cuts a description at 200 characters: with 25 houses in the request the list is what gets cut, never the wins taken away or the times moved.
func TestTheWriteLogKeepsTheFactsOfA25HouseDescriptionAndCutsOnlyTheList(t *testing.T) {
	var names []string
	var replaced []replacedWin
	for i := 0; i < 25; i++ {
		n := fmt.Sprintf("House%02d", i)
		names = append(names, n)
		w := "Atreides"
		if i%2 == 1 {
			w = "Harkonnen"
		}
		replaced = append(replaced, replacedWin{house: n, winner: w})
	}
	d := landsraadDesc("landsraad: 25 houses changed from the board", replaced, 3, names)
	if i := strings.Index(d, "; houses:"); i < 0 || len([]rune(d[:i])) > 190 {
		t.Fatalf("the facts come first and fit: %q", d)
	}
	kept := save.LogDescForTests(d)
	for _, w := range []string{"replaces 25 wins", "Atreides:", "Harkonnen:", "3 other completion times moved", "; houses: "} {
		if !strings.Contains(kept, w) {
			t.Fatalf("the log keeps %q: %q", w, kept)
		}
	}
	if len([]rune(kept)) > 203 {
		t.Fatalf("the log is cut: %d", len([]rune(kept)))
	}
}

// The same with what a bulk raise writes: the experience that did not fit comes with the facts, after the wins taken away and the times moved, and the amounts come after those, so the
// log's cut (200 characters) takes the amounts and the list of houses, never a win taken away, the times moved or the experience that was left out (#752).
func TestTheWriteLogKeepsTheFactsOfABulkRaiseAndCutsTheAmountsFirst(t *testing.T) {
	var names []string
	var replaced []replacedWin
	for i := 0; i < 25; i++ {
		n := fmt.Sprintf("House%02d", i)
		names = append(names, n)
		w := "Atreides"
		if i%2 == 1 {
			w = "Harkonnen"
		}
		replaced = append(replaced, replacedWin{house: n, winner: w})
	}
	rw := rewardResult{missions: 40, solari: 240000, scrip: 40000, rewardRows: 90,
		spec:     map[string]int64{"Crafting": 15000, "Gathering": 15000, "Exploration": 15000, "Combat": 15000, "Sabotage": 0},
		specLeft: map[string]int64{"Sabotage": 17818}}
	d := landsraadDescG("landsraad: 25 houses changed from the board", grantDesc(rw, true), replaced, 3, names)
	kept := save.LogDescForTests(d)
	for _, w := range []string{"replaces 25 wins", "Atreides:", "Harkonnen:", "3 other completion times moved", "17,818 XP not added"} {
		if !strings.Contains(kept, w) {
			t.Fatalf("the log keeps %q: %q (%d characters)", w, kept, len([]rune(d)))
		}
	}
	if len([]rune(kept)) > 203 {
		t.Fatalf("the log is cut: %d", len([]rune(kept)))
	}
	// one house retaken, on a track at its last level: the win taken away and the experience left out are kept, the amounts are what is cut
	one := landsraadDescG("landsraad Kenola: your contribution 1300 -> 6000, completed for Harkonnen", grantDesc(rewardResult{missions: 40, solari: 240000, scrip: 40000, rewardRows: 90,
		spec: map[string]int64{"Gathering": 15000}, specLeft: map[string]int64{"Gathering": 9000}}, false), replacedOf("Kenola", "Atreides"), 2, nil)
	kept = save.LogDescForTests(one)
	for _, w := range []string{"replaces the win of Atreides on Kenola", "2 other completion times moved", "9,000 Gathering XP not added (last level)"} {
		if !strings.Contains(kept, w) {
			t.Fatalf("the log keeps %q: %q (%d characters)", w, kept, len([]rune(one)))
		}
	}
	// the full description, the one the page lists, still carries the amounts
	if !strings.Contains(one, "40 missions: +240,000 Solari") || !strings.Contains(d, "40 missions: +240,000 Solari, +40,000 House Scrip, +60,000 XP; 90 reward rows; houses: ") {
		t.Fatalf("the amounts are missing from the description: %q / %q", one, d)
	}
}

// ---------------------------------------------------------------- a save with no term

func TestTheBoardApplyOfASaveWithNoTermIsRefusedAndWritesNothing(t *testing.T) {
	// a save whose term row is gone (the foreign keys are off while the fixture is built): there is no current term to change a house of
	o := &Ops{S: testsave.PlayerWithSQL(t, clockWorld+"\npragma foreign_keys=off;\ndelete from landsraad_decree_term;\n")}
	before := mustOne(t, o, `select completed c, winning_faction_id w, completion_time ct from landsraad_tasks where id=231`)
	for _, h := range []map[string]any{house(233, Args{"complete_for": 2.0}), house(231, Args{"was_done": true, "was_win": 2.0, "reopen": true})} {
		if _, err := clockApply(o, h); !errors.Is(err, apierr.ErrConflict) {
			t.Fatalf("no term is a conflict, the page's term is not the save's: %v", err)
		}
	}
	if after := mustOne(t, o, `select completed c, winning_faction_id w, completion_time ct from landsraad_tasks where id=231`); fmt.Sprint(after) != fmt.Sprint(before) || len(o.S.Pending()) != 0 {
		t.Fatalf("nothing is written: %v -> %v", before, after)
	}
}
