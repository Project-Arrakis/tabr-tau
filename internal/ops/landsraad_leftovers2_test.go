package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Leftovers of the reviews of #771, #773 and #776 (#775, #778): the head of a several-house description, the bounds of a name, a number written in full, and the tests the grounds of a
// raise after a faction change were missing. Every test runs on a temp copy.

// ---------------------------------------------------------------- #775.1: the head of a several-house description says the action and the faction

func TestSeveralHousesCompletedOrReopenedSayItInTheHead(t *testing.T) {
	o := goalOps(t)
	mustApply(t, o, applyArgs(house(175, Args{"complete_for": 1.0}), house(176, Args{"complete_for": 1.0})))
	if d := lastEdit(t, o); d != "landsraad: 2 houses completed for Atreides; houses: Kenola, Vernius" {
		t.Fatalf("complete: %q", d)
	}
	o = goalOps(t)
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=2, completion_time=150 where id in (175, 176)`)
	mustApply(t, o, applyArgs(house(175, Args{"reopen": true, "was_done": true, "was_win": 2.0}), house(176, Args{"reopen": true, "was_done": true, "was_win": 2.0})))
	if d := lastEdit(t, o); d != "landsraad: 2 houses reopened; houses: Kenola, Vernius" {
		t.Fatalf("reopen: %q", d)
	}
}

func TestSeveralHousesWithDifferentChangesStayChangedFromTheBoard(t *testing.T) {
	o := goalOps(t)
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=2, completion_time=150 where id=176`)
	mustApply(t, o, applyArgs(house(175, Args{"complete_for": 2.0}), house(176, Args{"reopen": true, "was_done": true, "was_win": 2.0})))
	if d := lastEdit(t, o); d != "landsraad: 2 houses changed from the board; houses: Kenola, Vernius" {
		t.Fatalf("a mix of actions: %q", d)
	}
	// the same words with a different faction each are not "the same thing" either
	o = goalOps(t)
	mustApply(t, o, applyArgs(house(175, Args{"complete_for": 1.0}), house(176, Args{"complete_for": 2.0})))
	if d := lastEdit(t, o); !strings.HasPrefix(d, "landsraad: 2 houses changed from the board") {
		t.Fatalf("two factions: %q", d)
	}
}

func TestSeveralHeadOnlyNamesAPlainCompletion(t *testing.T) {
	for _, c := range []struct {
		done []string
		want string
	}{
		{[]string{"completed for Atreides", "completed for Atreides"}, "landsraad: 2 houses completed for Atreides"},
		{[]string{"reopened", "reopened"}, "landsraad: 2 houses reopened"},
		{[]string{"completed for Atreides", "reopened"}, "landsraad: 2 houses changed from the board"},
		{[]string{"your contribution 1300 -> 6000", "your contribution 1300 -> 6000"}, "landsraad: 2 houses changed from the board"},
		{[]string{"completed for A; B", "completed for A; B"}, "landsraad: 2 houses changed from the board"}, // a name that would break the line is not put in the head
		{[]string{"completed for A, B", "completed for A, B"}, "landsraad: 2 houses changed from the board"},
		{nil, "landsraad: 2 houses changed from the board"},
	} {
		if got := severalHead(2, c.done); got != c.want {
			t.Fatalf("%v: %q, want %q", c.done, got, c.want)
		}
	}
}

// ---------------------------------------------------------------- #775.6: a faction name is bounded like a house name

func TestAFactionNameFromASaveBuiltByHandIsClippedInTheDescription(t *testing.T) {
	long := strings.Repeat("Z", 100)
	o := goalOps(t)
	setup(t, o, `update factions set name=? where id=1`, long)
	mustApply(t, o, applyArgs(house(175, Args{"complete_for": 1.0})))
	d := lastEdit(t, o)
	if strings.Contains(d, long) || !strings.Contains(d, "completed for "+strings.Repeat("Z", maxHouseName)+"...") {
		t.Fatalf("the faction named in the description is clipped: %q", d)
	}
	// the faction whose win is taken away
	o = goalOps(t)
	setup(t, o, `update factions set name=? where id=1`, long)
	setup(t, o, `update landsraad_tasks set completed=1, winning_faction_id=1, completion_time=150 where id=175`)
	m := mustApply(t, o, applyArgs(house(175, Args{"complete_for": 2.0, "was_done": true, "was_win": 1.0})))
	d = lastEdit(t, o)
	if strings.Contains(d, long) || !strings.Contains(d, "replaces the win of "+strings.Repeat("Z", maxHouseName)+"...") {
		t.Fatalf("the winner taken away is clipped: %q", d)
	}
	if w := m["replaced"].([]map[string]any)[0]["previous_winner"].(string); strings.Contains(w, long) {
		t.Fatalf("and in the answer: %q", w)
	}
}

// ---------------------------------------------------------------- #775.7: a number is written in full

func TestNumbersInMessagesAreWrittenInFull(t *testing.T) {
	for in, want := range map[float64]string{1200: "1200", 1000000: "1000000", 123456789: "123456789", 1234.5: "1234.5", 0: "0"} {
		if got := numText(in); got != want {
			t.Fatalf("%v: %q, want %q", in, got, want)
		}
	}
	o := goalOps(t)
	setup(t, o, `update landsraad_task_player_contributions set amount=1000000 where player_id=1 and task_id=175`)
	_, err := o.LandsraadApply(applyArgs(house(175, Args{"was_mine": 1000000.0, "mine_to": 1200.0})))
	if !errors.Is(err, apierr.ErrBadArgument) || !strings.Contains(err.Error(), "go down from 1000000 to 1200") {
		t.Fatalf("a large number is not written as 1e+06: %v", err)
	}
}

// ---------------------------------------------------------------- #778 L1, L2: what the character reached under another faction

// After a faction change the character's row of the old faction is ground reached elsewhere: the board shows it as my_elsewhere, the character's own contribution is the row of the faction it is in now
// (none yet), and the rows of the own faction and of simulated guilds are not counted as elsewhere. A character in no faction has none.
func TestTheBoardShowsWhatTheCharacterReachedUnderAnotherFaction(t *testing.T) {
	o := rewardOps(t, `update player_faction set faction_id=1 where actor_id=1;
		update landsraad_task_player_contributions set amount=6000 where player_id=1 and faction_id=2 and task_id=175;
		insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 1, 174, 1200);`)
	k := taskByID(t, o, 175)
	if k["my_elsewhere"] != 6000.0 || k["my_contribution"] != 0.0 {
		t.Fatalf("Kenola after a faction change: elsewhere %v, own %v", k["my_elsewhere"], k["my_contribution"])
	}
	// Maros: 7,850 under Harkonnen (elsewhere now), 1,200 under Atreides (own now, not counted as elsewhere); the guild row of another actor is nobody's elsewhere
	mr := taskByID(t, o, 174)
	if mr["my_elsewhere"] != 7850.0 || mr["my_contribution"] != 1200.0 {
		t.Fatalf("Maros: elsewhere %v, own %v", mr["my_elsewhere"], mr["my_contribution"])
	}
	// a character in no faction has no "elsewhere": there is no own faction to be elsewhere from
	n := rewardOps(t, `delete from player_faction where actor_id=1`)
	if k := taskByID(t, n, 175); k["my_elsewhere"] != 0.0 {
		t.Fatalf("a Neutral character: %v", k["my_elsewhere"])
	}
}

// What the character reached under several other factions is the highest of them, not their sum: both on the board and in the raise that works out what it pays.
func TestGroundUnderSeveralOtherFactionsIsTheHighestNotTheSum(t *testing.T) {
	// the character is in Atreides (1) now; it reached 2,400 under Harkonnen (2) and 1,300 under the save's None (3)
	extra := `update player_faction set faction_id=1 where actor_id=1;
		update landsraad_task_player_contributions set amount=2400 where player_id=1 and faction_id=2 and task_id=175;
		insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 3, 175, 1300);`
	o := rewardOps(t, extra)
	if k := taskByID(t, o, 175); k["my_elsewhere"] != 2400.0 {
		t.Fatalf("the board adds the factions up or takes the wrong one: %v (want 2400, not 3700)", k["my_elsewhere"])
	}
	// a raise to 3,600 pays the one mission above 2,400 (ceil(3600/1000) - ceil(2400/1000)); the sum, 3,700, would pay nothing
	m := applyWith(t, rewardOps(t, extra), DefaultGrantSettings(), house(175, Args{"was_mine": 0.0, "mine_to": 3600.0}))
	if m["missions"].(int64) != 1 || m["solari"].(int64) != 6000 {
		t.Fatalf("the raise adds the other factions up: %v", m)
	}
}
