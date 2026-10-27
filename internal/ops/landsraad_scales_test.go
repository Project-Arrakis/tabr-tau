package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The Landsraad tab shows two scales (#363): the faction's goal for a house (goal_amount, 4000 in the term read so far) and the reward tiers of a player's own
// contribution (1200 to 6000). They are returned as the save holds them: the goal is not stretched to the last tier, a contribution above the goal is kept, and
// nothing is cut at the goal.
func landsraadSave(t *testing.T) *Ops {
	return &Ops{S: testsave.PlayerWithSQL(t, `
insert or ignore into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen');
insert into landsraad_decree_term(term_id, start_time, end_time) values (7, 0, 100);
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (900, 7, 0, 'DA_HouseVarota', 4000);
insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values
  (900, 1200, 'SolarisCoin', 10000), (900, 2400, 'Schematic.A', 1), (900, 3600, 'T6Machinery', 50), (900, 4800, 'Schematic.B', 1), (900, 6000, 'Swatch.A', 1);
insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) select min(id), 1, 900, 6500 from actors;
`)}
}

func TestLandsraadReturnsTheGoalAndTheTiersAsTwoScales(t *testing.T) {
	o := landsraadSave(t)
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	var task map[string]any
	for _, x := range res.(map[string]any)["tasks"].([]map[string]any) {
		if x["id"].(int64) == 900 {
			task = x
		}
	}
	if task == nil {
		t.Fatal("task 900 missing")
	}
	if g := task["goal_amount"].(int64); g != 4000 {
		t.Fatalf("goal = %d, want 4000 as stored", g)
	}
	rw := task["rewards"].([]any)
	if len(rw) != 5 {
		t.Fatalf("%d tiers, want 5", len(rw))
	}
	want := []int64{1200, 2400, 3600, 4800, 6000}
	for i, r := range rw {
		if got := r.(map[string]any)["threshold"].(int64); got != want[i] {
			t.Fatalf("tier %d = %d, want %d", i, got, want[i])
		}
	}
	pcs := task["player_contributions"].([]map[string]any)
	if len(pcs) != 1 || pcs[0]["amount"].(float64) != 6500 {
		t.Fatalf("a recorded contribution above the goal and above the last tier is returned unchanged: %v", pcs)
	}
}

func TestSetTaskProgressIsNotHeldToTheGoal(t *testing.T) {
	o := landsraadSave(t)
	if _, err := o.SetTaskProgress(Args{"task_id": 900.0, "faction_id": 1.0, "amount": 6000.0}); err != nil {
		t.Fatal(err)
	}
	r, _ := o.S.One(`select amount from landsraad_task_faction_contributions where task_id=900 and faction_id=1`)
	if r["amount"].(float64) != 6000 {
		t.Fatalf("an amount above the goal (4000) must be stored as given: %v", r)
	}
}

// CR-6: an amount is bounded like a tier is (maxTier); a number that only a hand-made request names is refused and nothing is written.
func TestSetTaskProgressRefusesAnAmountBeyondTheBound(t *testing.T) {
	o := landsraadSave(t)
	for _, amt := range []float64{maxTier + 1, 1e308} {
		_, err := o.SetTaskProgress(Args{"task_id": 900.0, "faction_id": 1.0, "amount": amt})
		if !errors.Is(err, apierr.ErrBadArgument) || !strings.Contains(err.Error(), "at most 1,000,000,000") {
			t.Fatalf("%g: %v", amt, err)
		}
	}
	if r, _ := o.S.One(`select amount from landsraad_task_faction_contributions where task_id=900 and faction_id=1`); r != nil {
		t.Fatalf("a refused amount was written: %v", r)
	}
	if _, err := o.SetTaskProgress(Args{"task_id": 900.0, "faction_id": 1.0, "amount": maxTier}); err != nil {
		t.Fatalf("the bound itself is allowed: %v", err)
	}
}
