package ops

import (
	"fmt"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- reward delivery for a raised contribution (part 3 of the tab rework, refs #609, #656)
//
// What the game leaves in a save when a character's contribution to a house is raised, as read from real saves (the evidence is in the comments of #609). The whole set of writes of
// a raise (the faction's share, the rewards, the wallets and the specialization XP; no event and no cursor move, #655) is in landsraad_grant.go; this file writes the reward rows:
//
//	landsraad_house_rewards           one row per reward tier crossed (player, house, template, amount, last_updated = the time of the term's play clock): the template and the amount come from
//	                                  landsraad_task_rewards of that threshold. An amount above 0 waits at the house representative; the game sets it to 0 when the reward is
//	                                  collected; rows are one per (player, house, template), so a second delivery of the same item adds to a waiting amount and sets a collected one
//	the highest tier                  while landsraad_player_one_time_house_rewards has no row of the character for the house, the highest tier delivers the house swatch (the
//	                                  template and amount of landsraad_one_time_house_rewards) and the character gets that row (term_id = the current term); once it has the row,
//	                                  the highest tier delivers the task's own item and no row is added
//
// A tier is delivered once: the tiers that the character's own events for the task already crossed (the sum of their player_progress, see grantRaise) are delivered, and only the
// tiers above that ground are written (#656). The editor writes no event (#655), so what it delivered itself is remembered by the editor's own record of the ground it paid (landsraad_paid.go, #777) and, for the top tier, by the
// one-time swatch row of this term (a swatch won in this term is not delivered again). A row of the same house and item that is already there says nothing about a tier: a reward collected in a later term, or the same item
// at two tiers of one task, no longer hides a tier that was not delivered. Nothing here lowers anything. Completing a house adds no reward row by itself, and neither does a
// contribution for a faction the character is not in.

// rewardResult says what a delivery wrote.
type rewardResult struct {
	rewardRows, swatches, skipped int64
	missions, solari, scrip       int64
	spec                          map[string]int64 // the specialization XP that was added, by line
	specLeft                      map[string]int64 // the specialization XP a mission stood for that did not fit because the track is at its last level, by line (#752)
	// the ground of the raise that was not paid because the editor paid it before (landsraad_paid.go), and the missions it stands for
	paidGround   float64
	paidMissions int64
	// what a tier removal did to the rewards of the removed tiers (landsraad_takeback.go): tiers whose waiting reward was taken back, tiers whose reward was already collected and stays, tiers whose
	// reward stays because it cannot be tied to the tier with certainty, rows that wait for a tier the character's own missions cover (they stay), and the house swatches among those taken back
	takenBack, keptCollected, keptUnsure, keptEarned, swatchesBack int64
}

func (r *rewardResult) add(o rewardResult) {
	r.rewardRows += o.rewardRows
	r.swatches += o.swatches
	r.skipped += o.skipped
	r.missions += o.missions
	r.solari += o.solari
	r.scrip += o.scrip
	r.paidGround += o.paidGround
	r.paidMissions += o.paidMissions
	r.takenBack += o.takenBack
	r.keptCollected += o.keptCollected
	r.keptUnsure += o.keptUnsure
	r.keptEarned += o.keptEarned
	r.swatchesBack += o.swatchesBack
	for l, x := range o.spec {
		if r.spec == nil {
			r.spec = map[string]int64{}
		}
		r.spec[l] += x
	}
	for l, x := range o.specLeft {
		if r.specLeft == nil {
			r.specLeft = map[string]int64{}
		}
		r.specLeft[l] += x
	}
}

// writeTierRewards writes the reward rows of the tiers in (lo, after] of one house; the tiers in (before, lo] were crossed by the character's own events already and only counted as
// skipped.
func writeTierRewards(m *save.Mut, me myLandsraad, taskID int64, house string, termID int64, before, lo, after float64, clock termClock) (rewardResult, error) {
	var res rewardResult
	if lo > before {
		n, err := m.One(`select count(*) n from landsraad_task_rewards where task_id=? and threshold>? and threshold<=?`, taskID, before, lo)
		if err != nil {
			return res, err
		}
		res.skipped = asInt(n["n"])
	}
	tiers, err := m.Query(`select threshold, template_id, amount from landsraad_task_rewards where task_id=? and threshold>? and threshold<=? order by threshold`, taskID, lo, after)
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
			// A swatch won in this term is still this term's top reward (the board shows it so): the top tier was delivered in this term, and with no event of the editor to
			// remember it (#655) the one-time row is what says so, so the tier is not delivered again, as the task's own item or otherwise.
			if held != nil && asInt(held["term_id"]) == termID {
				res.skipped++
				continue
			}
			// the swatch is the top reward while the character does not hold it; once it does, the task's own item is
			if held == nil {
				s, err := m.One(`select template_id, amount from landsraad_one_time_house_rewards where house_name=? and threshold=?`, house, int64(th))
				if err != nil {
					return res, err
				}
				if s != nil {
					tmpl, amt, sw = fmt.Sprint(s["template_id"]), asInt(s["amount"]), true
				}
			}
		}
		items = append(items, item{tmpl, amt, sw})
	}
	// the same item from two tiers of this change is one row with the amounts added
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
		cur, err := m.One(`select amount from landsraad_house_rewards where player_id=? and house_name=? and template_id=?`, me.player, house, tmpl)
		if err != nil {
			return res, err
		}
		amt := sum[tmpl]
		if cur != nil {
			if have := asInt(cur["amount"]); have > 0 {
				amt += have // a reward that was never collected is still waiting: the game adds to it
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
