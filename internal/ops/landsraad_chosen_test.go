package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Completing and reopening the chosen houses (#451, now the board's complete_for and reopen), on the world of landsraad_mine_test.go plus a third house of the current term (176, Vernius) with no progress row.
// House 174 (Maros) is marked completed but has no winner, and the Harkonnen (2) progress there is 4,250, above the goal of 4,000; 175 (Kenola) is open with 1,000.
func chosenOps(t *testing.T) *Ops { return mineOps(t, manyWorld) }

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
