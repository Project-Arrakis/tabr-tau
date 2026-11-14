package ops

import (
	"fmt"
	"math"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- reward delivery for a raised contribution (part 3 of the tab rework, refs #609)
//
// What the game leaves in a save when a character's contribution to a house is raised, as read from real saves (the evidence is in the comments of #609):
//
//	landsraad_task_progress           one event per fill or mission: an id above every earlier id and above the cursor, the faction, the task, faction_progress (what the event
//	                                  added to the faction's progress, 0 once the faction is at the goal), guild_progress 0, player_progress (what the character added), and the
//	                                  play clock as the timestamp
//	landsraad_task_progress_player    the link of that event to the character
//	landsraad_house_rewards           one row per reward tier crossed (player, house, template, amount, last_updated = the event's time): the template and the amount come from
//	                                  landsraad_task_rewards of that threshold. An amount above 0 waits at the house representative; the game sets it to 0 when the reward is
//	                                  collected; rows are one per (player, house, template), so a second delivery of the same item adds to a waiting amount
//	the highest tier                  while landsraad_player_one_time_house_rewards has no row of the character for the house, the highest tier delivers the house swatch (the
//	                                  template and amount of landsraad_one_time_house_rewards) and the character gets that row (term_id = the current term); once it has the row,
//	                                  the highest tier delivers the task's own item and no row is added
//	the cursor                        landsraad_task_progress_processed.last_processed_id lags the newest event and rewards do not wait for it. It is left as it is: no save read
//	                                  shows what the game would do with a value the editor sets
//
// Nothing here lowers anything. A tier is delivered once per term: a (player, house, template) row written in the current term means the tier was already delivered (this also
// holds after a tier was removed and given again). Completing a house adds no reward row by itself, and neither does a contribution for a faction the character is not in.

// rewardResult says what a delivery wrote.
type rewardResult struct {
	events, rewardRows, swatches, skipped int64
}

func (r *rewardResult) add(o rewardResult) {
	r.events += o.events
	r.rewardRows += o.rewardRows
	r.swatches += o.swatches
	r.skipped += o.skipped
}

// deliverContribution writes the event and the reward rows for a character's contribution to one house raised from before to after, inside the edit. factionDelta is how much
// the faction's progress on the house was raised by the same change (0 when the change left the faction alone).
func deliverContribution(m *save.Mut, me myLandsraad, t save.Row, before, after, factionDelta float64, clock termClock) (rewardResult, error) {
	var res rewardResult
	if after <= before {
		return res, nil
	}
	taskID, _ := t["id"].(int64)
	house := fmt.Sprint(t["house_name"])
	termID, _ := t["term_id"].(int64)
	// the event: its id is above every event and above the cursor
	top, err := m.One(`select coalesce(max(id),0) a, coalesce((select last_processed_id from landsraad_task_progress_processed where id=1),0) b from landsraad_task_progress`)
	if err != nil {
		return res, err
	}
	a, _ := top["a"].(int64)
	b, _ := top["b"].(int64)
	id := max(a, b) + 1
	if _, err := m.Exec(`insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values(?,?,?,?,0,?,?)`,
		id, me.faction, taskID, int64(math.Round(factionDelta)), after-before, clock.value); err != nil {
		return res, err
	}
	if _, err := m.Exec(`insert into landsraad_task_progress_player(progress_id, player_id) values(?,?)`, id, me.player); err != nil {
		return res, err
	}
	res.events = 1
	// the tiers crossed
	tiers, err := m.Query(`select threshold, template_id, amount from landsraad_task_rewards where task_id=? and threshold>? and threshold<=? order by threshold`, taskID, before, after)
	if err != nil {
		return res, err
	}
	highest, err := m.One(`select max(threshold) t from landsraad_task_rewards where task_id=?`, taskID)
	if err != nil {
		return res, err
	}
	topTh, _ := toFloat(highest["t"])
	haveSwatchTables, err := tablesExist(m, "landsraad_one_time_house_rewards", "landsraad_player_one_time_house_rewards")
	if err != nil {
		return res, err
	}
	type item struct {
		template string
		amount   int64
		swatch   bool
	}
	var items []item
	for _, r := range tiers {
		th, _ := toFloat(r["threshold"])
		tmpl, amt := fmt.Sprint(r["template_id"]), asInt(r["amount"])
		sw := false
		if haveSwatchTables && th == topTh {
			held, err := m.One(`select term_id from landsraad_player_one_time_house_rewards where player_id=? and house_name=?`, me.player, house)
			if err != nil {
				return res, err
			}
			// the swatch is the top reward while it is not held, and also in the term in which it was won (the board shows it that way); a tier given again in that term is then the
			// swatch again, which the once-per-term rule below finds already delivered
			heldTerm, _ := held["term_id"].(int64)
			if held == nil || heldTerm == termID {
				s, err := m.One(`select template_id, amount from landsraad_one_time_house_rewards where house_name=? and threshold=?`, house, int64(th))
				if err != nil {
					return res, err
				}
				if s != nil {
					tmpl, amt, sw = fmt.Sprint(s["template_id"]), asInt(s["amount"]), held == nil
				}
			}
		}
		items = append(items, item{tmpl, amt, sw})
	}
	// the same item from two tiers is one row with the amounts added; a row written in this term means the tier was already delivered
	sum := map[string]int64{}
	swatchOf := map[string]bool{}
	var order []string
	for _, it := range items {
		if _, ok := sum[it.template]; !ok {
			order = append(order, it.template)
		}
		sum[it.template] += it.amount
		swatchOf[it.template] = swatchOf[it.template] || it.swatch
	}
	sort.Strings(order)
	for _, tmpl := range order {
		cur, err := m.One(`select amount, last_updated from landsraad_house_rewards where player_id=? and house_name=? and template_id=?`, me.player, house, tmpl)
		if err != nil {
			return res, err
		}
		if cur != nil {
			if lu, _ := cur["last_updated"].(int64); lu >= clock.start {
				res.skipped++
				continue
			}
		}
		amt := sum[tmpl]
		if cur != nil {
			if have := asInt(cur["amount"]); have > 0 {
				amt += have // a reward from an earlier term that was never collected is still waiting
			}
		}
		if _, err := m.Exec(`insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values(?,?,?,?,?)
			on conflict(player_id, house_name, template_id) do update set amount=excluded.amount, last_updated=excluded.last_updated`, me.player, house, amt, tmpl, clock.value); err != nil {
			return res, err
		}
		res.rewardRows++
		if swatchOf[tmpl] {
			if _, err := m.Exec(`insert into landsraad_player_one_time_house_rewards(player_id, house_name, template_id, term_id) values(?,?,?,?)
				on conflict do nothing`, me.player, house, tmpl, termID); err != nil {
				return res, err
			}
			res.swatches++
		}
	}
	return res, nil
}

// tablesExist says whether every named table is in the save (the two swatch tables are not in the older layout).
func tablesExist(m *save.Mut, names ...string) (bool, error) {
	for _, n := range names {
		r, err := m.One(`select 1 x from sqlite_master where type='table' and name=?`, n)
		if err != nil {
			return false, err
		}
		if r == nil {
			return false, nil
		}
	}
	return true, nil
}

func asInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	}
	return 0
}
