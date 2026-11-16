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

// replacedText is the part of a description that names the wins taken away, "" for none. One win is named with its house; several are grouped by the faction that held
// them, so the part stays short enough for the write log, which cuts a description (the list of houses comes after it, see landsraadDesc).
func replacedText(r []replacedWin) string {
	switch len(r) {
	case 0:
		return ""
	case 1:
		return "; replaces the win of " + r[0].winner + " on " + r[0].house
	}
	var order []string
	by := map[string][]string{}
	for _, x := range r {
		if _, seen := by[x.winner]; !seen {
			order = append(order, x.winner)
		}
		by[x.winner] = append(by[x.winner], x.house)
	}
	parts := make([]string, 0, len(order))
	for _, w := range order {
		parts = append(parts, w+": "+listTextN(by[w], 2))
	}
	return fmt.Sprintf("; replaces %d wins (%s)", len(r), strings.Join(parts, "; "))
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
func listText(names []string) string { return listTextN(names, 6) }

// listTextN is listText with the number of names shown.
func listTextN(names []string, show int) string {
	if len(names) <= show {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:show], ", "), len(names)-show)
}

// repairText is the part of a description that says what the clock repair did (repairCompletionTimes): it puts every completed house of the term whose time lies outside the
// term's window back inside it, not only the houses of the request, so the write log says how many.
func repairText(n int64) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "; 1 other completion time moved"
	}
	return fmt.Sprintf("; %d other completion times moved", n)
}

// landsraadDesc builds the description of a Landsraad edit: the head says what was done, then the short facts (the wins taken away, the times moved), and the list of houses last,
// because the write log cuts a long description and what is cut must be the list, not the facts. A head of just "landsraad" (an edit that only moved times) gets a colon.
func landsraadDesc(head string, replaced []replacedWin, repaired int64, houses []string) string {
	return landsraadDescG(head, grantText{}, replaced, repaired, houses)
}

// landsraadDescG is landsraadDesc with what the missions of a raise wrote (grantDesc): the experience that did not fit goes with the facts, after the wins taken away and the times moved,
// and the amounts come last before the list of houses, so that the write log's cut takes the amounts and the list before any fact (#752).
func landsraadDescG(head string, g grantText, replaced []replacedWin, repaired int64, houses []string) string {
	facts := replacedText(replaced) + repairText(repaired) + g.left
	if head == "landsraad" && facts != "" {
		head, facts = "landsraad:", facts[1:] // "; x" -> " x"
	}
	d := head + facts + g.paid
	if len(houses) > 0 {
		d += "; houses: " + listText(houses)
	}
	return d
}

// replacedOf is the one-house list of taken wins for a winner that may be empty.
func replacedOf(house, winner string) []replacedWin {
	if winner == "" {
		return nil
	}
	return []replacedWin{{house: house, winner: winner}}
}
