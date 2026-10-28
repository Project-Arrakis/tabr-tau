package ops

import (
	"encoding/json"
	"strings"
)

// The fill level of a liquid container, for the Inventory tables (issue #367). It is read-only: the fill is in the item's own stats as
// {"FFillableItemStats":[[],{"CurrentAmount":N,"FillableType":"Water"|"Blood"}]}, an empty container has an empty object or no key, and the
// capacities are the tables next to the refill code (waterCapacity, bloodCapacity in fillables.go). A capacity that is not in those tables is not
// guessed: the amount is shown on its own and the view says the capacity is unknown.

// fillOf describes what the container holds, or returns nil when the item is not a liquid container. It never fails: stats that cannot be read count as an
// empty record. The result has: kind ("Water", "Blood" or "" when an empty container of an unknown kind), amount, state ("empty", "partial", "full",
// "over" or "unknown" when the capacity is not known) and, when the capacity is known, capacity and percent (rounded; it can exceed 100). A worn stillsuit
// (stillsuitCapacity) is shown against its capacity as Water with "selfFilled" true: the game fills it, the editor never does (issue #482).
func fillOf(template, stats string) map[string]any {
	var rec map[string]json.RawMessage
	hasKey := false
	var pair []json.RawMessage
	if json.Unmarshal([]byte(stats), &rec) == nil {
		if raw, ok := rec["FFillableItemStats"]; ok {
			hasKey = true
			_ = json.Unmarshal(raw, &pair)
		}
	}
	_, inWater := waterCapacity[template]
	_, inBlood := bloodCapacity[template]
	suitCap, isSuit := stillsuitCapacity[template]
	sack := strings.HasPrefix(template, "Bloodsack")
	if !hasKey && !inWater && !inBlood && !sack && !isSuit {
		return nil
	}
	var d struct {
		Amount *float64 `json:"CurrentAmount"`
		Kind   string   `json:"FillableType"`
	}
	if len(pair) >= 2 {
		_ = json.Unmarshal(pair[1], &d)
	}
	amount := 0.0
	if d.Amount != nil && *d.Amount > 0 {
		amount = *d.Amount
	}
	kind := d.Kind
	if kind == "" { // an empty container is recognised by its template
		switch {
		case inBlood || sack:
			kind = "Blood"
		case inWater || isSuit:
			kind = "Water"
		}
	}
	out := map[string]any{"kind": kind, "amount": amount}
	capacity, known := 0.0, false
	switch kind {
	case "Water":
		capacity, known = waterCapacity[template]
		if isSuit { // the suit keeps no liquid type in the save, so an empty kind above became Water; a suit that says Blood stays unknown
			capacity, known = suitCap, true
			out["selfFilled"] = true
		}
	case "Blood":
		capacity, known = bloodCapacity[template]
	}
	switch {
	case amount == 0:
		out["state"] = "empty"
	case !known:
		out["state"] = "unknown"
	case amount > capacity:
		out["state"] = "over"
	case amount == capacity:
		out["state"] = "full"
	default:
		out["state"] = "partial"
	}
	if known && capacity > 0 {
		out["capacity"] = capacity
		out["percent"] = float64(int64(amount/capacity*100 + 0.5))
	}
	return out
}
