package ops

import (
	"errors"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Fill faction goal as a completion at the top tier (#601), on the world of landsraad_mine_test.go (character 1 in Harkonnen, 2) plus the third house of manyWorld and a play
// clock of 150 seconds inside the current term's window (100 to 200). House 174 (Maros) is complete with the faction at 4,250 and the character at 7,850 (above the top tier, 6,000);
// 175 (Kenola) is open with the faction at 1,000 and the character at 1,300; 176 (Vernius) is open with no row of any kind; 149 belongs to the earlier term 6.
const goalWorld = manyWorld + `
insert into farm_variables(one_row, farm_id, universe_time_timestamp, universe_lastactive_timestamp, down_time_accumulation) values (1, null, 1000000000000000, 1000000150000000, 0);
`

func goalOps(t *testing.T) *Ops { return mineOps(t, goalWorld) }

func goalArgs(task, faction float64) Args {
	return Args{"task_id": task, "faction_id": faction, "term_id": 7.0}
}

func houseRow(t *testing.T, o *Ops, id int64) map[string]any {
	t.Helper()
	return mustOne(t, o, `select completed c, winning_faction_id w, completion_time ct from landsraad_tasks where id=?`, id)
}

func TestFillFactionGoalCompletesTheHouseForTheFactionAtTheTopTier(t *testing.T) {
	o := goalOps(t)
	rewards := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)
	got, err := o.FillFactionGoal(goalArgs(175, 2))
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["changed"].(int64) != 1 || m["noop"] != false || m["house"] != "Kenola" || m["faction"] != "Harkonnen" || m["tier"].(float64) != 6000 ||
		m["progress_changed"] != true || m["completed_changed"] != true || m["contribution_changed"] != true || m["contribution_before"].(float64) != 1300 || m["has_character"] != true {
		t.Fatalf("answer: %v", m)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 || h["ct"].(int64) != 150 {
		t.Fatalf("complete for the faction, at the play clock: %v", h)
	}
	if v, _ := progress(t, o, 2, 175); v != 4000 {
		t.Fatalf("the faction's progress is the goal: %v", v)
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatalf("the character's contribution is the top tier: %v", c)
	}
	if c := contribution(t, o, 50, 175); c != 3000 {
		t.Fatalf("a simulated guild's row is not touched: %v", c)
	}
	if after := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`); after["n"] != rewards["n"] || after["s"] != rewards["s"] {
		t.Fatalf("no reward row is written: %v -> %v", rewards, after)
	}
}

// The state is the one the two existing paths leave together: Complete chosen for the faction and Fill chosen at the highest tier of the house.
func TestFillFactionGoalLeavesTheStateOfCompleteChosenPlusFillChosenAtTheTopTier(t *testing.T) {
	a, b := goalOps(t), goalOps(t)
	if _, err := a.FillFactionGoal(goalArgs(176, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CompleteChosen(Args{"term_id": 7.0, "task_ids": ids(176), "faction_id": 2.0, "expected": 1.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.FillMyContributions(Args{"term_id": 7.0, "items": items(176, 6000), "expected": 1.0}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`select completed, winning_faction_id, completion_time from landsraad_tasks where id=176`,
		`select faction_id, amount from landsraad_task_faction_contributions where task_id=176 order by faction_id`,
		`select player_id, faction_id, amount from landsraad_task_player_contributions where task_id=176 order by player_id, faction_id`,
	} {
		ra, _ := a.S.Query(q)
		rb, _ := b.S.Query(q)
		if len(ra) == 0 || len(ra) != len(rb) {
			t.Fatalf("%s: %v vs %v", q, ra, rb)
		}
		for i := range ra {
			for k, v := range ra[i] {
				if rb[i][k] != v {
					t.Errorf("%s: %s differs: %v vs %v", q, k, v, rb[i][k])
				}
			}
		}
	}
}

func TestFillFactionGoalNeverLowersAndSaysNothingToChangeWhenDone(t *testing.T) {
	o := goalOps(t)
	setup(t, o, `update landsraad_tasks set winning_faction_id=2, completion_time=130 where id=174`) // Maros is marked complete in the fixture with no winner: give it the faction
	got, err := o.FillFactionGoal(goalArgs(174, 2))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 0 || m["noop"] != true || m["progress_changed"] != false || m["completed_changed"] != false || m["contribution_changed"] != false {
		t.Fatalf("Maros is complete for the faction, at the goal and above the top tier: %v", m)
	}
	if v, _ := progress(t, o, 2, 174); v != 4250 {
		t.Fatalf("a progress above the goal stays: %v", v)
	}
	if c := contribution(t, o, 1, 174); c != 7850 {
		t.Fatalf("a contribution above the top tier stays: %v", c)
	}
	if h := houseRow(t, o, 174); h["ct"].(int64) != 130 {
		t.Fatalf("nothing was written, the time is as it was: %v", h)
	}
}

func TestFillFactionGoalWritesOnlyWhatIsMissing(t *testing.T) {
	o := goalOps(t)
	// complete for the faction with the progress at the goal, but the personal contribution below the top tier
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=2, completion_time=120 where id=175`)
	setup(t, o, `update landsraad_task_faction_contributions set amount=4000 where task_id=175`)
	got, err := o.FillFactionGoal(goalArgs(175, 2))
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["changed"].(int64) != 1 || m["progress_changed"] != false || m["completed_changed"] != false || m["contribution_changed"] != true {
		t.Fatalf("only the contribution is missing: %v", m)
	}
	if h := houseRow(t, o, 175); h["ct"].(int64) != 120 {
		t.Fatalf("a completion that is already there keeps its time: %v", h)
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatalf("contribution: %v", c)
	}
}

func TestFillFactionGoalForAnotherFactionWritesThatFactionsRowsOnly(t *testing.T) {
	o := goalOps(t)
	if _, err := o.FillFactionGoal(goalArgs(176, 1)); err != nil {
		t.Fatal(err)
	}
	if h := houseRow(t, o, 176); h["w"].(int64) != 1 {
		t.Fatalf("won by the chosen faction: %v", h)
	}
	if v, _ := progress(t, o, 1, 176); v != 4000 {
		t.Fatalf("the chosen faction's progress: %v", v)
	}
	if r := mustOne(t, o, `select faction_id f, amount a from landsraad_task_player_contributions where player_id=1 and task_id=176`); r["f"].(int64) != 1 || r["a"].(float64) != 6000 {
		t.Fatalf("the character's row is under the chosen faction: %v", r)
	}
	if _, has := progress(t, o, 2, 176); has {
		t.Fatal("the character's own faction got a row")
	}
}

func TestFillFactionGoalOnAHouseWithNoTierStillCompletesIt(t *testing.T) {
	o := goalOps(t)
	setup(t, o, `delete from landsraad_task_rewards where task_id=176`)
	got, err := o.FillFactionGoal(goalArgs(176, 2))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 1 || m["contribution_changed"] != false || m["completed_changed"] != true {
		t.Fatalf("answer: %v", m)
	}
	if n := mustOne(t, o, `select count(*) n from landsraad_task_player_contributions where task_id=176`); n["n"].(int64) != 0 {
		t.Fatal("no tier, no personal row")
	}
}

func TestFillFactionGoalWithNoCharacterWritesTheFactionPartOnly(t *testing.T) {
	o := goalOps(t)
	setup(t, o, `delete from player_state`)
	got, err := o.FillFactionGoal(goalArgs(175, 2))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["has_character"] != false || m["contribution_changed"] != false || m["completed_changed"] != true {
		t.Fatalf("answer: %v", m)
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("the contribution is as it was: %v", c)
	}
}

func TestFillFactionGoalRefusesWhatCompleteChosenRefuses(t *testing.T) {
	o := goalOps(t)
	for name, c := range map[string]struct {
		a    Args
		want error
	}{
		"no faction":             {Args{"task_id": 175.0, "term_id": 7.0}, apierr.ErrBadArgument},
		"the None faction":       {goalArgs(175, 3), apierr.ErrBadArgument},
		"a faction that is not":  {goalArgs(175, 99), apierr.ErrBadArgument},
		"no term":                {Args{"task_id": 175.0, "faction_id": 2.0}, apierr.ErrBadArgument},
		"a term that is not it":  {Args{"task_id": 175.0, "faction_id": 2.0, "term_id": 6.0}, apierr.ErrConflict},
		"a house of an old term": {goalArgs(149, 2), apierr.ErrConflict},
		"a house that is not":    {goalArgs(999, 2), apierr.ErrNotFound},
	} {
		if _, err := o.FillFactionGoal(c.a); err == nil || !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 0 || contribution(t, o, 1, 175) != 1300 {
		t.Fatal("a refusal writes nothing")
	}
}
