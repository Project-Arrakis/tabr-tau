package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"
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

// D1-1: for a faction the character is not in, Fill faction goal writes the faction's progress and the win, and no row of the character's (the character's own faction is unchanged,
// see landsraad_goal_test.go). The result says the personal row was skipped.
func TestFillFactionGoalForAnotherFactionWritesTheFactionsWinAndNoPersonalRow(t *testing.T) {
	o := goalOps(t)
	before := goalTables(t, o)
	got, err := o.FillFactionGoal(goalArgs(176, 1)) // the character is in Harkonnen; 1 is Atreides
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["changed"].(int64) != 1 || m["faction"] != "Atreides" || m["progress_changed"] != true || m["completed_changed"] != true || m["contribution_changed"] != false ||
		m["personal_skipped"] != true || m["has_character"] != true {
		t.Fatalf("answer: %v", m)
	}
	if h := houseRow(t, o, 176); h["c"].(int64) != 1 || h["w"].(int64) != 1 || h["ct"].(int64) != 150 {
		t.Fatalf("won by the chosen faction at the play clock: %v", h)
	}
	if v, _ := progress(t, o, 1, 176); v != 4000 {
		t.Fatalf("the chosen faction's progress is the goal: %v", v)
	}
	if _, has := progress(t, o, 2, 176); has {
		t.Fatal("the character's own faction got a progress row")
	}
	// the whole before/after: only the house and the faction's progress row changed; the personal and reward tables are as they were
	after := goalTables(t, o)
	for _, q := range []string{"landsraad_task_player_contributions", "landsraad_house_rewards"} {
		if section(before, q) != section(after, q) {
			t.Fatalf("%s changed:\n%s\n--- after\n%s", q, section(before, q), section(after, q))
		}
	}
	if n := mustOne(t, o, `select count(*) n from landsraad_task_player_contributions where task_id=176`); n["n"].(int64) != 0 {
		t.Fatal("no contribution row under the other faction")
	}
}

// the section of goalTables for one table: the lines from its query up to the next query
func section(text, table string) string {
	i := strings.Index(text, "from "+table)
	if i < 0 {
		return ""
	}
	rest := text[i:]
	if j := strings.Index(rest[1:], "select "); j >= 0 {
		return rest[:j+1]
	}
	return rest
}

// The case the audit reproduced: house 175 has the character's 1,300 under faction 2; a fill for faction 1 must not add the row (1, 1, 175, 6000) beside it.
func TestFillFactionGoalForAnotherFactionDoesNotAddAContributionRowNextToTheCharactersOwn(t *testing.T) {
	o := goalOps(t)
	if _, err := o.FillFactionGoal(goalArgs(175, 1)); err != nil {
		t.Fatal(err)
	}
	rows, _ := o.S.Query(`select faction_id f, amount a from landsraad_task_player_contributions where player_id=1 and task_id=175`)
	if len(rows) != 1 || rows[0]["f"].(int64) != 2 || rows[0]["a"].(float64) != 1300 {
		t.Fatalf("the character's rows are as they were: %v", rows)
	}
	if h := houseRow(t, o, 175); h["w"].(int64) != 1 {
		t.Fatalf("the faction still won the house: %v", h)
	}
	// and a second run for the character's own faction then writes its row, as before
	if _, err := o.FillFactionGoal(goalArgs(175, 2)); err != nil {
		t.Fatal(err)
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatalf("own faction: %v", c)
	}
}

func TestFillFactionGoalForACharacterInNoFactionWritesTheFactionPartAndNoPersonalRow(t *testing.T) {
	o := goalOps(t)
	setup(t, o, `delete from player_faction`)
	before := goalTables(t, o)
	got, err := o.FillFactionGoal(goalArgs(176, 2))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["personal_skipped"] != true || m["completed_changed"] != true || m["contribution_changed"] != false {
		t.Fatalf("answer: %v", m)
	}
	after := goalTables(t, o)
	if section(before, "landsraad_task_player_contributions") != section(after, "landsraad_task_player_contributions") {
		t.Fatal("a Neutral character gets no personal row")
	}
	if h := houseRow(t, o, 176); h["w"].(int64) != 2 {
		t.Fatalf("the house is won: %v", h)
	}
}

// The character's own faction is filled as before, personal row included.
func TestFillFactionGoalForTheCharactersOwnFactionStillWritesTheRow(t *testing.T) {
	o := goalOps(t)
	got, err := o.FillFactionGoal(goalArgs(176, 2))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["personal_skipped"] != false || m["contribution_changed"] != true {
		t.Fatalf("answer: %v", m)
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
	if m := got.(map[string]any); m["has_character"] != false || m["completed_changed"] != true || m["contribution_changed"] != false || m["personal_skipped"] != false {
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
