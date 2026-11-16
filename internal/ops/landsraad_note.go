package ops

import (
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// What a Landsraad edit says about itself in the write log and in its answer (D1-3, D1-4 and D1-6 of the delta audit, #611): which houses, which faction, whose win it takes
// away and which other completion times it moved. The text is the description of the queued change too, so the page can say what is waiting.

// replacedWin is a win that completing a house for another faction takes away.
type replacedWin struct{ house, winner string }

// replacedWinner names the faction whose win of the house completing it for faction takes away: "" when the house is open, is already the faction's own or has no winner. t is the row
// as chosenTask reads it (completed and winning_faction_id).
func replacedWinner(m *save.Mut, t save.Row, faction int64) (string, error) {
	if t["completed"] != int64(1) {
		return "", nil
	}
	w, ok := t["winning_faction_id"].(int64)
	if !ok || w == faction {
		return "", nil
	}
	f, err := m.One(`select name from factions where id=?`, w)
	if err != nil {
		return "", err
	}
	if f == nil {
		return fmt.Sprintf("faction %d", w), nil
	}
	return fmt.Sprint(f["name"]), nil
}

// replacedText is the part of a description that names the wins taken away, "" for none.
func replacedText(r []replacedWin) string {
	if len(r) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r))
	for _, x := range r {
		parts = append(parts, x.winner+" on "+x.house)
	}
	return "; replaces the win of " + listText(parts)
}

// replacedAnswer is the list of taken wins as the answer carries it (never nil, so the page can read it).
func replacedAnswer(r []replacedWin) []map[string]any {
	out := make([]map[string]any, 0, len(r))
	for _, x := range r {
		out = append(out, map[string]any{"house": x.house, "previous_winner": x.winner})
	}
	return out
}

// listText joins names for a description: the first six, then how many more.
func listText(names []string) string {
	const show = 6
	if len(names) <= show {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:show], ", "), len(names)-show)
}

// noteRepair adds to the description of the edit what the clock repair did (repairCompletionTimes): it puts every completed house of the term whose time lies outside the term's
// window back inside it, not only the houses of this request, so the write log says how many.
func noteRepair(m *save.Mut, n int64) {
	if n == 0 {
		return
	}
	what := fmt.Sprintf("%d other completion times of the term put back inside its window", n)
	if n == 1 {
		what = "1 other completion time of the term put back inside its window"
	}
	if m.Desc == "" || m.Desc == "landsraad" {
		m.Desc = "landsraad: " + what
		return
	}
	m.Desc += "; " + what
}

// replacedOf is the one-house list of taken wins for a winner that may be empty.
func replacedOf(house, winner string) []replacedWin {
	if winner == "" {
		return nil
	}
	return []replacedWin{{house: house, winner: winner}}
}

// nilIfEmpty gives an answer field no value instead of an empty text.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
