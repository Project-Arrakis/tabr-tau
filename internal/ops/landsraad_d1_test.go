package ops

import (
	"strings"
	"testing"
)

// Group D1 of the delta audit (#611), on the world of landsraad_goal_test.go: the character (player 1) is in Harkonnen (2); Atreides is 1.

// D1-2: every read of the Landsraad page returns its error; a failed read is not an empty board. (The character's faction read joins the factions table, so a save without it fails
// there first; the read of the factions list itself has the same code path.)
func TestLandsraadReturnsTheErrorOfEveryReadThatFails(t *testing.T) {
	t.Parallel()
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
