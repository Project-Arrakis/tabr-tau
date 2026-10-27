package ops

import (
	"errors"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Complete chosen and Reopen chosen (#451), on the world of landsraad_mine_test.go plus a third house of the current term (176, Vernius) with no progress row.
// House 174 (Maros) is marked completed but has no winner, and the Harkonnen (2) progress there is 4,250, above the goal of 4,000; 175 (Kenola) is open with 1,000.
func chosenOps(t *testing.T) *Ops { return mineOps(t, manyWorld) }

func ids(v ...float64) []any {
	out := make([]any, 0, len(v))
	for _, x := range v {
		out = append(out, x)
	}
	return out
}

func completeArgs(extra Args) Args {
	a := Args{"term_id": 7.0, "task_ids": ids(174, 175, 176), "faction_id": 2.0, "expected": 3.0}
	for k, v := range extra {
		a[k] = v
	}
	return a
}

func taskRow(t *testing.T, o *Ops, id int64) map[string]any {
	t.Helper()
	return mustOne(t, o, `select completed, winning_faction_id w, completion_time ct from landsraad_tasks where id=?`, id)
}

func progress(t *testing.T, o *Ops, faction, task int64) (float64, bool) {
	t.Helper()
	rows, err := o.S.Query(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, faction, task)
	if err != nil || len(rows) == 0 {
		return 0, false
	}
	return rows[0]["amount"].(float64), true
}

func setup(t *testing.T, o *Ops, q string, args ...any) {
	t.Helper()
	if _, err := o.S.Mutate("test setup", func(m *save.Mut) error { _, err := m.Exec(q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteChosenFillsTheFactionProgressToTheGoalAndCompletesTheHouses(t *testing.T) {
	o := chosenOps(t)
	rewards := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)
	personal := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_player_contributions`)
	got, err := o.CompleteChosen(completeArgs(nil))
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["changed"].(int64) != 3 || m["skipped"].(int64) != 0 || m["noop"] != false || m["houses"].(int64) != 3 {
		t.Fatalf("answer: %v", m)
	}
	for _, id := range []int64{174, 175, 176} {
		r := taskRow(t, o, id)
		if r["completed"].(int64) != 1 || r["w"].(int64) != 2 || r["ct"] == nil {
			t.Errorf("house %d is complete for faction 2: %v", id, r)
		}
	}
	if v, _ := progress(t, o, 2, 174); v != 4250 {
		t.Errorf("a progress above the goal is not lowered: %v", v)
	}
	if v, _ := progress(t, o, 2, 175); v != 4000 {
		t.Errorf("Kenola's progress is the goal: %v", v)
	}
	if v, has := progress(t, o, 2, 176); !has || v != 4000 {
		t.Errorf("Vernius had no row: it is created at the goal: %v %v", v, has)
	}
	if _, has := progress(t, o, 1, 175); has {
		t.Error("another faction got a row")
	}
	if mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)["s"] != rewards["s"] {
		t.Error("the reward rows must not be written")
	}
	if r := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_player_contributions`); r["s"] != personal["s"] || r["n"] != personal["n"] {
		t.Errorf("a personal contribution changed: %v", r)
	}
	if n := len(o.S.Pending()); n != 1 {
		t.Fatalf("one edit for all the houses, got %d: %v", n, o.S.Pending())
	}
	assertDBSound(t, o)
}

func TestCompleteChosenSkipsAHouseThatIsCompleteForTheFactionWithItsProgressAtTheGoal(t *testing.T) {
	o := chosenOps(t)
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=2, completion_time=12345 where id=175`)
	setup(t, o, `insert or replace into landsraad_task_faction_contributions(faction_id, task_id, amount) values (2, 175, 4000)`)
	got, err := o.CompleteChosen(completeArgs(Args{"task_ids": ids(175, 176), "expected": 1.0}))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 1 || m["skipped"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if r := taskRow(t, o, 175); r["ct"].(int64) != 12345 {
		t.Errorf("a skipped house is not written again: %v", r)
	}
	if r := taskRow(t, o, 176); r["completed"].(int64) != 1 {
		t.Errorf("the other house is completed: %v", r)
	}
	// everything is complete now: a second pass is a no-op and makes no edit
	pending := len(o.S.Pending())
	got, err = o.CompleteChosen(completeArgs(Args{"task_ids": ids(175, 176), "expected": 0.0}))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["noop"] != true || m["changed"].(int64) != 0 || m["skipped"].(int64) != 2 {
		t.Fatalf("answer: %v", m)
	}
	if len(o.S.Pending()) != pending {
		t.Fatalf("no edit when nothing changes: %v", o.S.Pending())
	}
}

func TestCompleteChosenFillsTheProgressOfAHouseAlreadyCompleteWithLessThanTheGoalWithoutTouchingItsTime(t *testing.T) {
	o := chosenOps(t)
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=2, completion_time=12345 where id=175`)
	got, err := o.CompleteChosen(completeArgs(Args{"task_ids": ids(175), "expected": 1.0}))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if v, _ := progress(t, o, 2, 175); v != 4000 {
		t.Errorf("progress = %v", v)
	}
	if r := taskRow(t, o, 175); r["ct"].(int64) != 12345 {
		t.Errorf("the time of an earlier completion is kept: %v", r)
	}
}

func TestCompleteChosenForAnotherFactionTakesTheHouseOver(t *testing.T) {
	o := chosenOps(t)
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=2, completion_time=12345 where id=175`)
	setup(t, o, `insert or replace into landsraad_task_faction_contributions(faction_id, task_id, amount) values (2, 175, 4000)`)
	if _, err := o.CompleteChosen(completeArgs(Args{"task_ids": ids(175), "faction_id": 1.0, "expected": 1.0})); err != nil {
		t.Fatal(err)
	}
	if r := taskRow(t, o, 175); r["w"].(int64) != 1 || r["ct"].(int64) == 12345 {
		t.Errorf("the winner is the chosen faction and the time is new: %v", r)
	}
	if v, _ := progress(t, o, 1, 175); v != 4000 {
		t.Errorf("Atreides progress = %v", v)
	}
	if v, _ := progress(t, o, 2, 175); v != 4000 {
		t.Errorf("the other faction's progress is left: %v", v)
	}
}

func TestCompleteChosenRefusesAStaleBoardAndWritesNothing(t *testing.T) {
	o := chosenOps(t)
	for name, extra := range map[string]Args{
		"more than shown":        {"expected": 2.0},
		"fewer than shown":       {"expected": 4.0},
		"another term":           {"term_id": 6.0},
		"a house of an old term": {"task_ids": ids(175, 149), "expected": 2.0},
	} {
		if _, err := o.CompleteChosen(completeArgs(extra)); !errors.Is(err, apierr.ErrConflict) {
			t.Errorf("%s: err = %v, want a conflict", name, err)
		}
	}
	if r := taskRow(t, o, 175); r["completed"].(int64) != 0 {
		t.Errorf("written although refused: %v", r)
	}
	if v, _ := progress(t, o, 2, 175); v != 1000 {
		t.Errorf("progress written although refused: %v", v)
	}
	if len(o.S.Pending()) != 0 {
		t.Fatalf("a refused request leaves no edit: %v", o.S.Pending())
	}
}

func TestCompleteChosenChecksItsArguments(t *testing.T) {
	o := chosenOps(t)
	for name, a := range map[string]Args{
		"no houses":                    completeArgs(Args{"task_ids": ids()}),
		"a house twice":                completeArgs(Args{"task_ids": ids(175, 175)}),
		"a house that is not a number": completeArgs(Args{"task_ids": []any{"x"}}),
		"no expected":                  {"term_id": 7.0, "task_ids": ids(175), "faction_id": 2.0},
		"no faction":                   {"term_id": 7.0, "task_ids": ids(175), "expected": 1.0},
		"the None faction":             completeArgs(Args{"faction_id": 3.0, "expected": 3.0}),
		"an unknown faction":           completeArgs(Args{"faction_id": 99.0}),
		"no term":                      {"task_ids": ids(175), "faction_id": 2.0, "expected": 1.0},
	} {
		if _, err := o.CompleteChosen(a); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := o.CompleteChosen(completeArgs(Args{"task_ids": ids(175, 999999), "expected": 2.0})); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("a house that does not exist: %v", err)
	}
	if len(o.S.Pending()) != 0 {
		t.Fatalf("a refused request leaves no edit: %v", o.S.Pending())
	}
}

func TestReopenChosenReopensAndLeavesProgressAndContributionsAlone(t *testing.T) {
	o := chosenOps(t)
	if _, err := o.CompleteChosen(completeArgs(Args{"task_ids": ids(174, 175), "expected": 2.0})); err != nil {
		t.Fatal(err)
	}
	progressBefore := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_faction_contributions`)
	personalBefore := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_player_contributions`)
	rewardsBefore := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)
	got, err := o.ReopenChosen(Args{"term_id": 7.0, "task_ids": ids(174, 175, 176), "expected": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 2 || m["skipped"].(int64) != 1 || m["noop"] != false {
		t.Fatalf("answer: %v", m)
	}
	for _, id := range []int64{174, 175, 176} {
		if r := taskRow(t, o, id); r["completed"].(int64) != 0 || r["w"] != nil || r["ct"] != nil {
			t.Errorf("house %d is open with no winner and no time: %v", id, r)
		}
	}
	if mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_faction_contributions`)["s"] != progressBefore["s"] {
		t.Error("a faction's progress changed")
	}
	if r := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_player_contributions`); r["s"] != personalBefore["s"] || r["n"] != personalBefore["n"] {
		t.Errorf("a personal contribution changed: %v", r)
	}
	if mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)["s"] != rewardsBefore["s"] {
		t.Error("the reward rows must not be written")
	}
	assertDBSound(t, o)
}

func TestReopenChosenRefusesAStaleBoardAndSkipsOpenHouses(t *testing.T) {
	o := chosenOps(t)
	for name, a := range map[string]Args{
		"a count that is not the set": {"term_id": 7.0, "task_ids": ids(174, 175), "expected": 2.0},
		"another term":                {"term_id": 6.0, "task_ids": ids(174), "expected": 1.0},
		"a house of an old term":      {"term_id": 7.0, "task_ids": ids(174, 149), "expected": 1.0},
		"no count":                    {"term_id": 7.0, "task_ids": ids(174)},
	} {
		if _, err := o.ReopenChosen(a); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if r := taskRow(t, o, 174); r["completed"].(int64) != 1 {
		t.Errorf("written although refused: %v", r)
	}
	// 174 is complete, 175 and 176 are open: one house changes, two are skipped
	got, err := o.ReopenChosen(Args{"term_id": 7.0, "task_ids": ids(174, 175, 176), "expected": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 1 || m["skipped"].(int64) != 2 {
		t.Fatalf("answer: %v", m)
	}
	got, err = o.ReopenChosen(Args{"term_id": 7.0, "task_ids": ids(174, 175), "expected": 0.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["noop"] != true {
		t.Fatalf("answer: %v", m)
	}
}

// The faction the page offers first is the character's own (#451): the board says which it is, and none when the character is in no faction.
func TestTheBoardNamesTheCharactersFactionForTheDefaultOfTheFactionList(t *testing.T) {
	o := chosenOps(t)
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if ch := m["character"].(map[string]any); ch["faction_id"].(int64) != 2 {
		t.Fatalf("character: %v", ch)
	}
	found := false
	for _, f := range m["factions"].([]map[string]any) {
		if f["id"].(int64) == 2 && f["name"] == "Harkonnen" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the character's faction is in the list the page offers: %v", m["factions"])
	}
	setup(t, o, `update player_faction set faction_id=3 where actor_id=1`)
	res, _ = o.Landsraad()
	if ch := res.(map[string]any)["character"].(map[string]any); ch["faction_id"] != nil {
		t.Fatalf("a character in no faction has none: %v", ch)
	}
}
