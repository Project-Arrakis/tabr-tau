package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Group D1 of the delta audit (#611), on the world of landsraad_goal_test.go: the character (player 1) is in Harkonnen (2); Atreides is 1.

// everything Fill faction goal can write, as one text
func goalTables(t *testing.T, o *Ops) string {
	t.Helper()
	var b strings.Builder
	for _, q := range []string{
		`select id, completed, winning_faction_id, completion_time from landsraad_tasks order by id`,
		`select faction_id, task_id, amount from landsraad_task_faction_contributions order by faction_id, task_id`,
		`select player_id, faction_id, task_id, amount from landsraad_task_player_contributions order by player_id, faction_id, task_id`,
		`select player_id, house_name, template_id, amount from landsraad_house_rewards order by player_id, house_name, template_id`,
	} {
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString(q + "\n")
		for _, r := range rows {
			fmt.Fprintf(&b, "%v\n", map[string]any(r)) // fmt prints a map with its keys in order
		}
	}
	return b.String()
}

// D1-1: a faction the character is not in is refused, with a clear message, and the save is not touched.
func TestFillFactionGoalRefusesAFactionTheCharacterIsNotInAndWritesNothing(t *testing.T) {
	o := goalOps(t)
	before := goalTables(t, o)
	got, err := o.FillFactionGoal(goalArgs(176, 1)) // the character is in Harkonnen; 1 is Atreides
	if err == nil || !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("want a bad-argument refusal, got %v, %v", got, err)
	}
	if msg := err.Error(); !strings.Contains(msg, "not in Atreides") || !strings.Contains(msg, "nothing was changed") {
		t.Fatalf("the message says who and that nothing changed: %s", msg)
	}
	if after := goalTables(t, o); after != before {
		t.Fatalf("a refusal leaves the save as it was:\n%s\n--- after\n%s", before, after)
	}
	if o.S.Dirty() {
		t.Fatal("a refusal queued or made a write")
	}
	if n := mustOne(t, o, `select count(*) n from landsraad_task_player_contributions where task_id=176`); n["n"].(int64) != 0 {
		t.Fatal("no contribution row under the other faction")
	}
}

// The case the audit reproduced: house 175 has the character's 1,300 under faction 2; a fill for faction 1 must not add the row (1, 1, 6000) beside it.
func TestFillFactionGoalForAnotherFactionDoesNotAddAContributionRowNextToTheCharactersOwn(t *testing.T) {
	o := goalOps(t)
	if _, err := o.FillFactionGoal(goalArgs(175, 1)); err == nil {
		t.Fatal("refused")
	}
	rows, _ := o.S.Query(`select faction_id f, amount a from landsraad_task_player_contributions where player_id=1 and task_id=175`)
	if len(rows) != 1 || rows[0]["f"].(int64) != 2 || rows[0]["a"].(float64) != 1300 {
		t.Fatalf("the character's rows are as they were: %v", rows)
	}
}

func TestFillFactionGoalRefusesAFactionForACharacterInNoFaction(t *testing.T) {
	o := goalOps(t)
	setup(t, o, `delete from player_faction`)
	before := goalTables(t, o)
	if _, err := o.FillFactionGoal(goalArgs(176, 2)); err == nil || !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("a Neutral character is in no faction: %v", err)
	}
	if goalTables(t, o) != before {
		t.Fatal("nothing is written")
	}
}

// The character's own faction is still filled, and a save with no character still gets the faction part for any faction (no personal row is written either way).
func TestFillFactionGoalForTheCharactersOwnFactionStillWritesTheRow(t *testing.T) {
	o := goalOps(t)
	if _, err := o.FillFactionGoal(goalArgs(176, 2)); err != nil {
		t.Fatal(err)
	}
	if c := contribution(t, o, 1, 176); c != 6000 {
		t.Fatalf("own faction: %v", c)
	}
}

func TestFillFactionGoalWithNoCharacterStillCompletesForAnyFactionWithoutAPersonalRow(t *testing.T) {
	o := goalOps(t)
	setup(t, o, `delete from player_state`)
	got, err := o.FillFactionGoal(goalArgs(176, 1))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["has_character"] != false || m["completed_changed"] != true || m["contribution_changed"] != false {
		t.Fatalf("answer: %v", m)
	}
	if n := mustOne(t, o, `select count(*) n from landsraad_task_player_contributions where task_id=176`); n["n"].(int64) != 0 {
		t.Fatal("no personal row")
	}
}

// D1-9: only "no player" is no character; a failed read of the player is an error and nothing is written.
func TestFillFactionGoalFailsOnAPlayerReadThatFailsInsteadOfReadingNoCharacter(t *testing.T) {
	o := goalOps(t)
	before := goalTables(t, o)
	setup(t, o, `drop table player_state`)
	got, err := o.FillFactionGoal(goalArgs(175, 2))
	if err == nil {
		t.Fatalf("a failed read must not read as no character: %v", got)
	}
	if errors.Is(err, errNoPlayer) {
		t.Fatalf("it is not the no-player answer: %v", err)
	}
	if goalTables(t, o) != before {
		t.Fatal("nothing is written")
	}
}

// D1-2: every read of the Landsraad page returns its error; a failed read is not an empty board. (The character's faction read joins the factions table, so a save without it fails
// there first; the read of the factions list itself has the same code path.)
func TestLandsraadReturnsTheErrorOfEveryReadThatFails(t *testing.T) {
	for _, c := range []struct{ what, table string }{
		{"the Landsraad term", "landsraad_decree_term"},
		{"the Landsraad tasks", "landsraad_tasks"},
		{"the factions' progress", "landsraad_task_faction_contributions"},
		{"the personal contributions", "landsraad_task_player_contributions"},
		{"the rewards delivered to your character", "landsraad_house_rewards"},
		{"the reward tiers", "landsraad_task_rewards"},
		{"the decrees", "landsraad_decrees"},
		{"the decree rotation", "landsraad_decree_rotation"},
		{"your character's faction", "player_faction"},
		{"your character's faction", "factions"},
	} {
		t.Run(c.table, func(t *testing.T) {
			o := goalOps(t)
			if _, err := o.Landsraad(); err != nil {
				t.Fatalf("the world reads before the table is gone: %v", err)
			}
			setup(t, o, `drop table `+c.table)
			got, err := o.Landsraad()
			if err == nil {
				t.Fatalf("a failed read of %s gave a board: %v", c.what, got)
			}
			if !strings.Contains(err.Error(), c.what) || !strings.Contains(err.Error(), c.table) {
				t.Fatalf("the error names the read and the cause: %v", err)
			}
		})
	}
}
