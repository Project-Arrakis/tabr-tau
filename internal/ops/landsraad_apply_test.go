package ops

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// The board's write (part 2 of the tab rework, refs #599, #440): every action runs on a copy of a save (the test database) and the rows it leaves are asserted.
// World: goalWorld (landsraad_goal_test.go). The character is player 1 in Harkonnen (2); the other faction is Atreides (1). House 174 (Maros) is completed with no winner, the
// character is at 7,850 there and Harkonnen's progress is 4,250 (the goal is 4,000); 175 (Kenola) is open, the character is at 1,300 and Harkonnen's progress is 1,000;
// 176 (Vernius) is open with no row of any kind. The play clock is 150 s, inside the term's window.

// boardRows is every row the board's write can touch, in a fixed order, as one string.
func boardRows(t *testing.T, o *Ops) string { return boardTables(t, o, true) }

// boardTables is boardRows with or without the delivery rows (the rewards of part 3 are written by the same request).
func boardTables(t *testing.T, o *Ops, withRewards bool) string {
	t.Helper()
	var out string
	qs := []string{
		`select id, completed, winning_faction_id, completion_time from landsraad_tasks order by id`,
		`select player_id, faction_id, task_id, amount from landsraad_task_player_contributions order by player_id, faction_id, task_id`,
		`select faction_id, task_id, amount from landsraad_task_faction_contributions order by faction_id, task_id`,
	}
	if withRewards {
		qs = append(qs, `select player_id, house_name, amount, template_id, last_updated from landsraad_house_rewards order by player_id, house_name, template_id`)
	}
	for _, q := range qs {
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			out += fmt.Sprint(r) + "\n"
		}
		out += "--\n"
	}
	return out
}

func rewardRows(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select player_id, house_name, amount, template_id, last_updated from landsraad_house_rewards order by player_id, house_name, template_id`)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(rows)
}

func house(task float64, extra Args) map[string]any {
	h := map[string]any{"task_id": task}
	for k, v := range extra {
		h[k] = v
	}
	return h
}

func applyArgs(houses ...map[string]any) Args {
	hs := make([]any, 0, len(houses))
	for _, h := range houses {
		hs = append(hs, h)
	}
	return Args{"term_id": 7.0, "houses": hs}
}

func mustApply(t *testing.T, o *Ops, a Args) map[string]any {
	t.Helper()
	got, err := o.LandsraadApply(a)
	if err != nil {
		t.Fatal(err)
	}
	return got.(map[string]any)
}

func wantApplyError(t *testing.T, o *Ops, a Args, kind error) {
	t.Helper()
	before := boardRows(t, o)
	_, err := o.LandsraadApply(a)
	if !errors.Is(err, kind) {
		t.Fatalf("error = %v, want %v", err, kind)
	}
	if after := boardRows(t, o); after != before {
		t.Fatalf("a refused request wrote something:\n%s\n---\n%s", before, after)
	}
}

func TestApplyFillRaisesTheContributionToTheTopTierAndNothingElse(t *testing.T) {
	o := goalOps(t)
	before := boardRows(t, o)
	m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	if m["changed"].(int64) != 1 || m["contribution_raised"].(int64) != 1 || m["completed"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatalf("the contribution is the top tier: %v", c)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 0 || h["w"] != nil || h["ct"] != nil {
		t.Fatalf("Fill does not take the house: %v", h)
	}
	if v, _ := progress(t, o, 2, 175); v != 1000 {
		t.Fatalf("Fill does not touch the faction's progress: %v", v)
	}
	after := boardRows(t, o)
	if after == before {
		t.Fatal("nothing changed")
	}
}

func TestApplyOnlyARaisedContributionWritesRewardRows(t *testing.T) {
	// the delivery rows are the same before and after a removal, a Complete and a Reopen; a raised contribution writes them (landsraad_rewards_test.go)
	for name, hs := range map[string][]map[string]any{
		"tier remove": {house(174, Args{"was_mine": 7850.0, "mine_to": 4800.0, "lower": true})},
		"complete":    {house(175, Args{"complete_for": 1.0})},
		"reopen":      {house(174, Args{"was_done": true, "reopen": true})},
	} {
		o := goalOps(t)
		before := rewardRows(t, o)
		mustApply(t, o, applyArgs(hs...))
		if after := rewardRows(t, o); after != before {
			t.Errorf("%s: the reward rows changed:\n%s\n%s", name, before, after)
		}
	}
}

func TestApplyTierGiveCreatesTheRowAndRaisesIt(t *testing.T) {
	o := goalOps(t)
	mustApply(t, o, applyArgs(house(176, Args{"mine_to": 3600.0}))) // 176: no row yet
	if c := contribution(t, o, 1, 176); c != 3600 {
		t.Fatalf("tier 3: %v", c)
	}
	mustApply(t, o, applyArgs(house(176, Args{"was_mine": 3600.0, "mine_to": 4800.0})))
	if c := contribution(t, o, 1, 176); c != 4800 {
		t.Fatalf("tier 4: %v", c)
	}
	if v, ok := progress(t, o, 2, 176); ok {
		t.Fatalf("a tier click writes no faction progress: %v", v)
	}
	if h := houseRow(t, o, 176); h["c"].(int64) != 0 {
		t.Fatalf("a tier click does not take the house: %v", h)
	}
}

func TestApplyTierRemovalIsTheOnlyWayDown(t *testing.T) {
	o := goalOps(t)
	// the highest tier of Maros (the character is at 7,850, above tier 5) removed: the contribution drops to the tier below, 4,800
	m := mustApply(t, o, applyArgs(house(174, Args{"was_mine": 7850.0, "mine_to": 4800.0, "lower": true})))
	if m["contribution_lowered"].(int64) != 1 || m["contribution_raised"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if c := contribution(t, o, 1, 174); c != 4800 {
		t.Fatalf("lowered to the tier below: %v", c)
	}
	if c := contribution(t, o, 50, 174); c != 1000 {
		t.Fatalf("a simulated guild's row is not touched: %v", c)
	}
	// tier 1 removed: the row is gone (none recorded)
	mustApply(t, o, applyArgs(house(176, Args{"mine_to": 1200.0})))
	mustApply(t, o, applyArgs(house(176, Args{"was_mine": 1200.0, "mine_to": 0.0, "lower": true})))
	rows, err := o.S.Query(`select 1 from landsraad_task_player_contributions where player_id=1 and task_id=176`)
	if err != nil || len(rows) != 0 {
		t.Fatalf("the row of tier 1 is removed: %v %v", rows, err)
	}
	// a request that would lower without saying so is refused, and nothing is written
	wantApplyError(t, o, applyArgs(house(174, Args{"was_mine": 4800.0, "mine_to": 2400.0})), apierr.ErrBadArgument)
	// Fill never lowers: the top tier is 6,000 and the character is above it
	o2 := goalOps(t)
	wantApplyError(t, o2, applyArgs(house(174, Args{"was_mine": 7850.0, "mine_to": 6000.0})), apierr.ErrBadArgument)
}

func TestApplyCompleteForTheOwnFaction(t *testing.T) {
	o := goalOps(t)
	m := mustApply(t, o, applyArgs(house(175, Args{"complete_for": 2.0})))
	if m["completed"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 || h["ct"].(int64) != 150 {
		t.Fatalf("complete for the faction, at the play clock: %v", h)
	}
	if v, _ := progress(t, o, 2, 175); v != 4000 {
		t.Fatalf("the faction's progress is the goal: %v", v)
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("Complete does not touch the character's contribution: %v", c)
	}
}

func TestApplyCompleteForAnotherFactionSkipsThePersonalRow(t *testing.T) {
	o := goalOps(t)
	mustApply(t, o, applyArgs(house(176, Args{"complete_for": 1.0})))
	if h := houseRow(t, o, 176); h["c"].(int64) != 1 || h["w"].(int64) != 1 {
		t.Fatalf("taken by Atreides: %v", h)
	}
	if v, _ := progress(t, o, 1, 176); v != 4000 {
		t.Fatalf("Atreides' progress is the goal: %v", v)
	}
	if _, ok := progress(t, o, 2, 176); ok {
		t.Fatal("the character's own faction gets no progress row")
	}
	rows, err := o.S.Query(`select 1 from landsraad_task_player_contributions where task_id=176`)
	if err != nil || len(rows) != 0 {
		t.Fatalf("no personal contribution row for a faction the character is not in: %v %v", rows, err)
	}
}

func TestApplyCompleteNeverLowersProgress(t *testing.T) {
	// Maros: Harkonnen's progress is 4,250, above the goal; completing the house for Harkonnen leaves 4,250
	o := goalOps(t)
	mustApply(t, o, applyArgs(house(174, Args{"was_done": true, "complete_for": 2.0})))
	if v, _ := progress(t, o, 2, 174); v != 4250 {
		t.Fatalf("progress above the goal is not lowered: %v", v)
	}
	if h := houseRow(t, o, 174); h["w"].(int64) != 2 {
		t.Fatalf("the winner is set: %v", h)
	}
}

func TestApplyReopenClearsTheCompletionAndTouchesNothingElse(t *testing.T) {
	o := goalOps(t)
	mustApply(t, o, applyArgs(house(175, Args{"complete_for": 2.0})))
	progressBefore, _ := progress(t, o, 2, 175)
	mustApply(t, o, applyArgs(house(175, Args{"was_done": true, "was_win": 2.0, "reopen": true})))
	if h := houseRow(t, o, 175); h["c"].(int64) != 0 || h["w"] != nil || h["ct"] != nil {
		t.Fatalf("reopened: %v", h)
	}
	if v, _ := progress(t, o, 2, 175); v != progressBefore {
		t.Fatalf("progress is left as it was: %v", v)
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("the contribution is left as it was: %v", c)
	}
}

func TestApplyFillFactionGoalLeavesTheStateOfTheExistingButton(t *testing.T) {
	a, b := goalOps(t), goalOps(t)
	if _, err := a.FillFactionGoal(goalArgs(175, 2)); err != nil {
		t.Fatal(err)
	}
	mustApply(t, b, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0})))
	// the existing button writes no reward rows; the board's write does (part 3), so only the other tables are compared
	if boardTables(t, a, false) != boardTables(t, b, false) {
		t.Fatalf("Fill faction goal and the board's write leave different rows:\n%s\n---\n%s", boardRows(t, a), boardRows(t, b))
	}
}

func TestApplySeveralHousesAreOneEdit(t *testing.T) {
	o := goalOps(t)
	m := mustApply(t, o, applyArgs(
		house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0}),
		house(176, Args{"complete_for": 2.0}),
		house(174, Args{"was_done": true, "reopen": true}),
	))
	if m["houses"].(int64) != 3 || m["changed"].(int64) != 3 || m["skipped"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	// a house that is already in the wanted state is counted as skipped
	m = mustApply(t, o, applyArgs(house(176, Args{"was_done": true, "was_win": 2.0, "complete_for": 2.0})))
	if m["changed"].(int64) != 0 || m["noop"] != true {
		t.Fatalf("nothing to change: %v", m)
	}
}

func TestApplyIsRefusedAsAWholeWhenAnythingIsNotWhatThePageShowed(t *testing.T) {
	o := goalOps(t)
	// the second house is stale (the contribution is 1,300, the page showed 1,200): the first is not written either
	wantApplyError(t, o, applyArgs(
		house(176, Args{"mine_to": 3600.0}),
		house(175, Args{"was_mine": 1200.0, "mine_to": 2400.0}),
	), apierr.ErrConflict)
	// the house was taken since the page showed it open
	wantApplyError(t, o, applyArgs(house(174, Args{"was_done": false, "complete_for": 2.0})), apierr.ErrConflict)
	wantApplyError(t, o, applyArgs(house(174, Args{"was_done": true, "was_win": 1.0, "reopen": true})), apierr.ErrConflict)
	// another term than the current one
	a := applyArgs(house(176, Args{"mine_to": 3600.0}))
	a["term_id"] = 6.0
	wantApplyError(t, o, a, apierr.ErrConflict)
	// a house of an earlier term
	wantApplyError(t, o, applyArgs(house(149, Args{"was_mine": 500.0, "mine_to": 1200.0})), apierr.ErrConflict)
}

func TestApplyRefusesWhatIsNotARequest(t *testing.T) {
	o := goalOps(t)
	for name, a := range map[string]Args{
		"no houses":                         {"term_id": 7.0},
		"an empty list":                     {"term_id": 7.0, "houses": []any{}},
		"no term":                           {"houses": []any{house(176, Args{"mine_to": 1200.0})}},
		"a house twice":                     applyArgs(house(176, Args{"mine_to": 1200.0}), house(176, Args{"mine_to": 2400.0})),
		"no change":                         applyArgs(house(176, Args{})),
		"complete and reopen":               applyArgs(house(176, Args{"complete_for": 2.0, "reopen": true})),
		"a contribution that is not a tier": applyArgs(house(176, Args{"mine_to": 1234.0})),
		"a negative contribution":           applyArgs(house(176, Args{"mine_to": -1.0})),
		"the None faction":                  applyArgs(house(176, Args{"complete_for": 3.0})),
		"an unknown faction":                applyArgs(house(176, Args{"complete_for": 99.0})),
		"an unknown house":                  applyArgs(house(999, Args{"mine_to": 1200.0})),
	} {
		before := boardRows(t, o)
		if _, err := o.LandsraadApply(a); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if boardRows(t, o) != before {
			t.Errorf("%s: wrote something", name)
		}
	}
}
