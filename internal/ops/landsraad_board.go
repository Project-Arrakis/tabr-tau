package ops

import (
	"fmt"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- what the board shows that the first page did not (#599, #609)
//
// The reward of a tier, and where it stands:
//
//	reward_state   not_reached | waiting | delivered | no_row
//	               waiting   the reward rows of the house (landsraad_house_rewards) hold the item with an amount above 0: it waits at the house representative
//	               delivered the row holds the item with amount 0 and was written in this term: it was collected
//	               no_row    the tier is reached and the save holds no delivery row of this term for the item (a house filled by an older editor, or a row from an earlier term)
//	shown_template_id, shown_amount, swatch
//	               what the game lists as the reward of the tier. For the highest tier it is the house swatch while the character does not hold it yet (landsraad_one_time_house_rewards
//	               names it), and the task's own item (landsraad_task_rewards) once the swatch is held; the character holds it when landsraad_player_one_time_house_rewards has a row
//	               for the house. A swatch won in the current term is still this term's top reward, so a row of the current term does not turn the tier into the task's own item.

// hasTable says whether the save has a table. The two swatch tables are not in the older layout of the game's save.
func hasTable(o *Ops, name string) bool {
	r, err := o.S.One(`select 1 x from sqlite_master where type='table' and name=?`, name)
	return err == nil && r != nil
}

// oneTime is what the swatch tables hold: the swatch item of each house, and the term in which the character was given it.
type oneTime struct {
	swatch map[string]save.Row // house_name -> row of landsraad_one_time_house_rewards
	held   map[string]int64    // house_name -> term_id of the character's row in landsraad_player_one_time_house_rewards
}

// readOneTime reads the swatch tables; a save without them gives an empty answer (no swatch rule applies there).
func readOneTime(o *Ops, player int64) (oneTime, error) {
	out := oneTime{swatch: map[string]save.Row{}, held: map[string]int64{}}
	if hasTable(o, "landsraad_one_time_house_rewards") {
		rows, err := o.S.Query(`select house_name, threshold, template_id, amount from landsraad_one_time_house_rewards`)
		if err != nil {
			return out, readFailed("the house swatches", err)
		}
		for _, r := range rows {
			out.swatch[fmt.Sprint(r["house_name"])] = r
		}
	}
	if player != 0 && hasTable(o, "landsraad_player_one_time_house_rewards") {
		rows, err := o.S.Query(`select house_name, term_id from landsraad_player_one_time_house_rewards where player_id=?`, player)
		if err != nil {
			return out, readFailed("the swatches your character holds", err)
		}
		for _, r := range rows {
			term, _ := r["term_id"].(int64)
			out.held[fmt.Sprint(r["house_name"])] = term
		}
	}
	return out, nil
}

// boardTask adds to one task what the board needs: the place, the swatch facts and the state of every tier.
func boardTask(t save.Row, tiers []any, delivered map[string]save.Row, one oneTime, termID int64, termStart float64) {
	house := fmt.Sprint(t["house_name"])
	t["location"] = locationOf(house)
	sw, hasSwatch := one.swatch[house]
	heldTerm, held := one.held[house]
	t["swatch_held"] = held
	if hasSwatch {
		t["swatch_template_id"] = sw["template_id"]
	}
	top := 0.0
	for _, x := range tiers {
		if th, _ := toFloat(x.(save.Row)["threshold"]); th > top {
			top = th
		}
	}
	for _, x := range tiers {
		r := x.(save.Row)
		th, _ := toFloat(r["threshold"])
		tmpl, amount := r["template_id"], r["amount"]
		isSwatch := false
		if th == top && hasSwatch && (!held || heldTerm == termID) {
			tmpl, amount, isSwatch = sw["template_id"], sw["amount"], true
		}
		r["shown_template_id"], r["shown_amount"], r["swatch"] = tmpl, amount, isSwatch
		state := "not_reached"
		if r["reached"] == true {
			state = "no_row"
			if d, ok := delivered[house+"\x00"+fmt.Sprint(tmpl)]; ok {
				have, _ := toFloat(d["amount"])
				upd, _ := toFloat(d["last_updated"])
				switch {
				case have > 0:
					state = "waiting"
				case upd >= termStart:
					state = "delivered"
				}
			}
		}
		r["reward_state"] = state
	}
}

// termHistory returns every term of the save (newest first) and, for each earlier term in which the character has a contribution, the houses with it: the character's total, the
// tiers reached, the status of the house and whether a reward of the house was written inside that term's window. A save that holds only the current term has no earlier terms.
func termHistory(o *Ops, player int64) ([]save.Row, []map[string]any, error) {
	terms, err := o.S.Query(`select term_id, start_time, end_time, reigning_faction_id, active_decree_id, elected_decree_id from landsraad_decree_term order by term_id desc`)
	if err != nil {
		return nil, nil, readFailed("the Landsraad terms", err)
	}
	if len(terms) < 2 || player == 0 {
		return terms, []map[string]any{}, nil
	}
	curID, _ := terms[0]["term_id"].(int64)
	tasks, err := o.S.Query(`select id, term_id, house_name, completed, winning_faction_id from landsraad_tasks where term_id<>? order by house_name`, curID)
	if err != nil {
		return nil, nil, readFailed("the earlier terms' houses", err)
	}
	mine, err := o.S.Query(`select task_id, amount from landsraad_task_player_contributions where player_id=?`, player)
	if err != nil {
		return nil, nil, readFailed("your earlier contributions", err)
	}
	my := map[int64]float64{}
	for _, c := range mine {
		if id, ok := c["task_id"].(int64); ok {
			a, _ := toFloat(c["amount"])
			my[id] += a
		}
	}
	rw, err := o.S.Query(`select task_id, threshold from landsraad_task_rewards`)
	if err != nil {
		return nil, nil, readFailed("the earlier terms' reward tiers", err)
	}
	ths := map[int64][]float64{}
	for _, r := range rw {
		if id, ok := r["task_id"].(int64); ok {
			th, _ := toFloat(r["threshold"])
			ths[id] = append(ths[id], th)
		}
	}
	paid, err := o.S.Query(`select house_name, last_updated from landsraad_house_rewards where player_id=?`, player)
	if err != nil {
		return nil, nil, readFailed("the rewards written for your character", err)
	}
	window := map[int64][2]float64{}
	for _, t := range terms {
		id, _ := t["term_id"].(int64)
		s, _ := toFloat(t["start_time"])
		e, _ := toFloat(t["end_time"])
		window[id] = [2]float64{s, e}
	}
	byTerm := map[int64][]map[string]any{}
	for _, t := range tasks {
		id, _ := t["id"].(int64)
		tid, _ := t["term_id"].(int64)
		amount, ok := my[id]
		if !ok {
			continue
		}
		reached := 0
		for _, th := range ths[id] {
			if amount >= th {
				reached++
			}
		}
		w := window[tid]
		paidHere := false
		for _, p := range paid {
			if fmt.Sprint(p["house_name"]) == fmt.Sprint(t["house_name"]) {
				if u, _ := toFloat(p["last_updated"]); u >= w[0] && u <= w[1] {
					paidHere = true
				}
			}
		}
		byTerm[tid] = append(byTerm[tid], map[string]any{"house_name": t["house_name"], "my_contribution": amount, "tiers_reached": reached, "tier_count": len(ths[id]),
			"completed": t["completed"], "winning_faction_id": t["winning_faction_id"], "paid_in_term": paidHere})
	}
	ids := make([]int64, 0, len(byTerm))
	for id := range byTerm {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	out := []map[string]any{}
	for _, id := range ids {
		w := window[id]
		out = append(out, map[string]any{"term_id": id, "start_time": w[0], "end_time": w[1], "houses": byTerm[id]})
	}
	return terms, out, nil
}
