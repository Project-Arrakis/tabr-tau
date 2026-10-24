package ops

import (
	"errors"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Several houses at once (#364), on the world of landsraad_mine_test.go plus a third house of the current term (176, Vernius) where the character has no row.
const manyWorld = `
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (176, 7, 2, 'DA_HouseVernius', 4000);
insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values
  (176, 1200, 'SolarisCoin', 10000), (176, 2400, 'Schematic.V', 1), (176, 3600, 'T6Machinery', 50), (176, 4800, 'Schematic.W', 1), (176, 6000, 'Swatch.Vernius', 1);
`

func manyOps(t *testing.T) *Ops { return mineOps(t, manyWorld) }

func items(pairs ...float64) []any {
	var out []any
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, map[string]any{"task_id": pairs[i], "tier": pairs[i+1]})
	}
	return out
}

func fillAll(t *testing.T, o *Ops, extra Args) (any, error) {
	t.Helper()
	a := Args{"term_id": 7.0, "items": items(174, 6000, 175, 6000, 176, 3600), "expected": 2.0}
	for k, v := range extra {
		a[k] = v
	}
	return o.FillMyContributions(a)
}

func TestFillSeveralHousesIsOneEditAndTouchesOnlyTheCharactersRows(t *testing.T) {
	o := manyOps(t)
	rewards := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)
	got, err := fillAll(t, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["changed"].(int64) != 2 || m["noop"] != false || m["houses"].(int64) != 3 {
		t.Fatalf("answer: %v", m)
	}
	if c := contribution(t, o, 1, 174); c != 7850 {
		t.Fatalf("Maros was above the tier and stays: %v", c)
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatalf("Kenola = %v, want 6000", c)
	}
	if c := contribution(t, o, 1, 176); c != 3600 {
		t.Fatalf("Vernius = %v, want 3600 (the row is created for the character's faction)", c)
	}
	if r := mustOne(t, o, `select faction_id f from landsraad_task_player_contributions where player_id=1 and task_id=176`); r["f"].(int64) != 2 {
		t.Fatalf("the new row is the character's faction's: %v", r)
	}
	if c := contribution(t, o, 50, 175); c != 3000 {
		t.Fatalf("a guild's row changed: %v", c)
	}
	if mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)["s"] != rewards["s"] {
		t.Fatal("the reward rows must not be written")
	}
	if n := len(o.S.Pending()); n != 1 {
		t.Fatalf("one edit for the whole selection, got %d: %v", n, o.S.Pending())
	}
	assertDBSound(t, o)
}

func TestFillSeveralHousesRefusesWholeWhenOneHouseIsNotOfTheCurrentTerm(t *testing.T) {
	o := manyOps(t)
	_, err := o.FillMyContributions(Args{"term_id": 7.0, "items": items(175, 6000, 149, 1200), "expected": 1.0})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("an earlier term's house: %v", err)
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("the valid house was filled although the request was refused: %v", c)
	}
	if len(o.S.Pending()) != 0 {
		t.Fatalf("a refused fill leaves no edit: %v", o.S.Pending())
	}
}

func TestFillSeveralHousesRefusesWhenTheConfirmedCountIsNoLongerTheSet(t *testing.T) {
	o := manyOps(t)
	for name, extra := range map[string]Args{
		"more than confirmed":  {"expected": 1.0},
		"fewer than confirmed": {"expected": 3.0},
		"another term":         {"term_id": 6.0},
	} {
		if _, err := fillAll(t, o, extra); !errors.Is(err, apierr.ErrConflict) {
			t.Errorf("%s: err = %v, want a conflict", name, err)
		}
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("written although refused: %v", c)
	}
	// the set changed between the page and the click: Vernius was filled meanwhile
	if _, err := o.FillMyContribution(Args{"task_id": 176.0, "tier": 3600.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := fillAll(t, o, nil); !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("the count the person confirmed (2) is 1 now: %v", err)
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("written although the count was stale: %v", c)
	}
}

func TestFillSeveralHousesWithNothingToRaiseWritesNothing(t *testing.T) {
	o := manyOps(t)
	got, err := o.FillMyContributions(Args{"term_id": 7.0, "items": items(174, 6000), "expected": 0.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["noop"] != true || m["changed"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if len(o.S.Pending()) != 0 {
		t.Fatalf("no edit when nothing changes: %v", o.S.Pending())
	}
}

func TestFillSeveralHousesCountsAHouseWhoseFactionRowOnlyChanges(t *testing.T) {
	o := manyOps(t)
	// without the box Maros does not count; with it the faction's 4,250 is above the goal and still does not count, Kenola's 1,000 does
	if _, err := o.FillMyContributions(Args{"term_id": 7.0, "items": items(174, 6000, 175, 1200), "expected": 0.0}); err != nil {
		t.Fatalf("without the box nothing changes: %v", err)
	}
	got, err := o.FillMyContributions(Args{"term_id": 7.0, "items": items(174, 6000, 175, 1200), "expected": 1.0, "with_faction": true})
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 1 || m["faction_changed"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if v := mustOne(t, o, `select amount a from landsraad_task_faction_contributions where task_id=175`)["a"].(float64); v != 4000 {
		t.Fatalf("faction progress = %v, want the goal", v)
	}
	if v := mustOne(t, o, `select amount a from landsraad_task_faction_contributions where task_id=174`)["a"].(float64); v != 4250 {
		t.Fatalf("an amount above the goal was lowered: %v", v)
	}
}

func TestFillSeveralHousesRefusesBadRequests(t *testing.T) {
	o := manyOps(t)
	ok := Args{"term_id": 7.0, "items": items(175, 1200), "expected": 1.0}
	with := func(k string, v any) Args {
		a := Args{}
		for kk, vv := range ok {
			a[kk] = vv
		}
		if v == nil {
			delete(a, k)
		} else {
			a[k] = v
		}
		return a
	}
	for name, a := range map[string]Args{
		"no items":            with("items", nil),
		"empty items":         with("items", []any{}),
		"items not a list":    with("items", "all"),
		"an item not object":  with("items", []any{"x"}),
		"a house twice":       with("items", items(175, 1200, 175, 2400)),
		"a tier of zero":      with("items", items(175, 0)),
		"no expected":         with("expected", nil),
		"negative expected":   with("expected", -1.0),
		"no term":             with("term_id", nil),
		"a flag that is text": with("with_faction", "yes"),
		"a tier not offered":  with("items", items(175, 1500)),
	} {
		if _, err := o.FillMyContributions(a); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("%s: err = %v, want a bad argument", name, err)
		}
	}
	var many []any
	for i := 0; i < maxFillHouses+1; i++ {
		many = append(many, map[string]any{"task_id": float64(1000 + i), "tier": 1200.0})
	}
	if _, err := o.FillMyContributions(Args{"term_id": 7.0, "items": many, "expected": 1.0}); !errors.Is(err, apierr.ErrBadArgument) {
		t.Errorf("more houses than the limit: %v", err)
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("a refused request wrote: %v", c)
	}
	if len(o.S.Pending()) != 0 {
		t.Fatalf("a refused request leaves no edit: %v", o.S.Pending())
	}
}

func TestFillSeveralHousesNeedsAFaction(t *testing.T) {
	o := mineOps(t, manyWorld+`update player_faction set faction_id=3 where actor_id=1;`)
	if _, err := fillAll(t, o, nil); !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("a Neutral character: %v", err)
	}
}
