package ops

import (
	"fmt"
	"math"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- what a tier removal takes back (#944, refs #609, #656, #777, #827)
//
// Removing a tier lowers the character's own contribution (landsraad_apply.go). It also takes back the reward of each removed tier that is still waiting at the house representative:
// the row of landsraad_house_rewards the raise wrote for that tier (landsraad_rewards.go). There is ONE rule for it, planTakeBack below. The write calls it inside the edit and the
// board's read calls it to say, before the click, what the click would do, so the sentence before the click and the write cannot disagree.
//
// The rule. The removed tiers are the tiers whose threshold lies above the new contribution and at or below the old one. For each of them:
//
//	earned      the character's own progress events (the game's record, eventsOf) or its contribution under another faction (otherFactionGround) cover the tier: the game paid it for the
//	            character's own missions, so it stays whatever the row says
//	none        the character has no row for the tier's item, or only a collected one of an earlier term: nothing waits
//	earlier     the row is waiting but was last written before this term started: it is not this term's tier, it stays
//	collected   the row is of this term and holds 0: the reward was collected and stays. Nothing here ever touches a collected reward
//	shared      the same item is also the reward of a tier that is not removed (or is earned): a waiting amount cannot be told from that tier's, it stays
//	unproven    the highest tier's house swatch is taken back only when the editor's own record (landsraad_paid.go, trusted as far as the save shows it) says it paid that ground and the
//	            character's one-time swatch row is of this term: only then is the swatch known to be the editor's and not the game's
//	short       the row holds less than the removed tiers of the item paid: it cannot be the whole of them, it stays
//	take        the row is waiting, of this term, the item is the removed tiers' alone and the amount covers them: the removed tiers' amount comes off the row (the row is deleted at 0; what
//	            is left stays), and for the swatch the character's one-time row of this term is deleted too, so the swatch can be won again
//
// Only the character's own rows are looked at (player_id = the character), only the rows of the removed tiers' items, only the house of the change. Nothing else is written: not the
// wallets, not the specialization XP, not the faction's progress, not the progress events or the cursor (#655), and not the editor's paid record (landsraad_paid.go). That record is
// trusted only as far as the save shows reward rows, so after a take-back it is read as paid up to the highest tier whose row still stands: a raise after a take-back pays the
// taken-back tiers again, rows and missions (their Solari, House Scrip and XP were not taken back, so they are paid a second time).

// the outcomes of one removed tier
const (
	tbTake      = "take"
	tbCollected = "collected"
	tbNone      = "none"
	tbUnsure    = "unsure" // the tier's reward stays because it cannot be tied to the tier with certainty; why says why
)

// the reasons of tbUnsure
const (
	tbWhyEarned   = "your own missions in the game cover this tier"
	tbWhyEarlier  = "the waiting reward was written in an earlier term"
	tbWhyShared   = "the same item is also the reward of another tier"
	tbWhyShort    = "less is waiting than the tier paid"
	tbWhyUnproven = "the editor has no record that it wrote this swatch"
)

// tierItem is the reward of one tier of a house: the item the board shows for it (the swatch for the highest tier while it is this term's) and the task's own item at that threshold.
type tierItem struct {
	th     float64
	tmpl   string
	own    string
	amount int64
	swatch bool
}

// rewardRow is the character's row of landsraad_house_rewards for one house and item.
type rewardRow struct {
	amount  int64
	updated float64
}

type takeBackCtx struct {
	termStart float64
	items     []tierItem           // every tier of the task
	held      bool                 // the character's one-time swatch row for the house is of this term
	earned    float64              // the ground the character's own events and its contribution under another faction cover
	paid      func() float64       // the ground the editor's own record says it paid, as far as the save shows it; read only when a swatch is in question
	rows      map[string]rewardRow // the character's rows for the house, by item
}

type tierBack struct {
	th      float64
	tmpl    string
	amount  int64
	outcome string
	why     string
}

// rowBack is what the plan does to one row: set the amount (delete at 0) and, for the swatch, delete the character's one-time row.
type rowBack struct {
	tmpl   string
	amount int64 // the amount the row keeps
	swatch bool
}

type takeBackPlan struct {
	tiers []tierBack // the removed tiers, from the lowest
	rows  []rowBack
}

// planTakeBack is the rule above for a contribution that goes from cur down to to.
func planTakeBack(c takeBackCtx, to, cur float64) takeBackPlan {
	var p takeBackPlan
	inSet := map[int]bool{} // the removed tiers that are not earned: the ones whose reward may be taken back
	at := map[int]int{}     // the index of a tier of c.items in p.tiers
	for i, it := range c.items {
		if !(it.th > to && it.th <= cur) {
			continue
		}
		at[i] = len(p.tiers)
		p.tiers = append(p.tiers, tierBack{th: it.th, tmpl: it.tmpl, amount: it.amount})
		if it.th <= c.earned {
			p.tiers[at[i]].outcome, p.tiers[at[i]].why = tbUnsure, tbWhyEarned
			continue
		}
		inSet[i] = true
	}
	sums, swatchOf, topOf := map[string]int64{}, map[string]bool{}, map[string]float64{}
	var order []string
	for i, it := range c.items {
		if !inSet[i] {
			continue
		}
		if _, ok := sums[it.tmpl]; !ok {
			order = append(order, it.tmpl)
		}
		sums[it.tmpl] += it.amount
		swatchOf[it.tmpl] = swatchOf[it.tmpl] || it.swatch
		topOf[it.tmpl] = math.Max(topOf[it.tmpl], it.th)
	}
	decide := func(tmpl string) (string, string) {
		row, has := c.rows[tmpl]
		switch {
		case !has || (row.amount <= 0 && row.updated < c.termStart):
			return tbNone, ""
		case row.updated < c.termStart:
			return tbUnsure, tbWhyEarlier
		case row.amount <= 0:
			return tbCollected, ""
		}
		for j, x := range c.items {
			if !inSet[j] && (x.tmpl == tmpl || x.own == tmpl) {
				return tbUnsure, tbWhyShared
			}
		}
		if swatchOf[tmpl] && !(c.held && c.paid != nil && c.paid() >= topOf[tmpl]) {
			return tbUnsure, tbWhyUnproven
		}
		if row.amount < sums[tmpl] {
			return tbUnsure, tbWhyShort
		}
		return tbTake, ""
	}
	for _, tmpl := range order {
		o, why := decide(tmpl)
		for i, it := range c.items {
			if inSet[i] && it.tmpl == tmpl {
				p.tiers[at[i]].outcome, p.tiers[at[i]].why = o, why
			}
		}
		if o == tbTake {
			p.rows = append(p.rows, rowBack{tmpl: tmpl, amount: c.rows[tmpl].amount - sums[tmpl], swatch: swatchOf[tmpl]})
		}
	}
	return p
}

// count says how many removed tiers had each outcome, and how many swatches are taken back.
func (p takeBackPlan) count() (taken, collected, unsure, swatches int64) {
	for _, t := range p.tiers {
		switch t.outcome {
		case tbTake:
			taken++
		case tbCollected:
			collected++
		case tbUnsure:
			unsure++
		}
	}
	for _, r := range p.rows {
		if r.swatch {
			swatches++
		}
	}
	return
}

// takeBackTierRewards is the write of the rule, inside the edit, for one house whose contribution goes from cur down to to (landsraad_apply.go). paidNow reads the editor's record, as the
// raise does; a record that cannot be read does not stop a removal: the swatch is then not provably the editor's and stays.
func takeBackTierRewards(m *save.Mut, me myLandsraad, t save.Row, cur, to float64, paidNow func() ([]paidNote, error)) (rewardResult, error) {
	var res rewardResult
	taskID, _ := t["id"].(int64)
	house := fmt.Sprint(t["house_name"])
	termID, _ := t["term_id"].(int64)
	tierRows, err := m.Query(`select threshold, template_id, amount from landsraad_task_rewards where task_id=? order by threshold`, taskID)
	if err != nil {
		return res, err
	}
	if len(tierRows) == 0 {
		return res, nil
	}
	top := 0.0
	for _, r := range tierRows {
		th, _ := toFloat(r["threshold"])
		top = math.Max(top, th)
	}
	c := takeBackCtx{termStart: math.Inf(1), rows: map[string]rewardRow{}}
	clock := clockOfTerm(m, termID)
	if clock.ok { // a term the save does not hold has no start: every waiting row is then "earlier" and stays
		c.termStart = float64(clock.start)
	}
	haveSwatch, err := tablesExist(m, "landsraad_one_time_house_rewards", "landsraad_player_one_time_house_rewards")
	if err != nil {
		return res, err
	}
	var sw save.Row
	held, heldTerm := false, int64(0)
	if haveSwatch {
		if sw, err = m.One(`select template_id, amount from landsraad_one_time_house_rewards where house_name=? and threshold=?`, house, int64(top)); err != nil {
			return res, err
		}
		hr, err := m.One(`select term_id from landsraad_player_one_time_house_rewards where player_id=? and house_name=?`, me.player, house)
		if err != nil {
			return res, err
		}
		if hr != nil {
			held, heldTerm = true, asInt(hr["term_id"])
		}
	}
	for _, r := range tierRows {
		th, _ := toFloat(r["threshold"])
		tmpl, amount, isSw := tierShows(r, th, top, sw, sw != nil, held, heldTerm, termID)
		it := tierItem{th: th, tmpl: fmt.Sprint(tmpl), own: fmt.Sprint(r["template_id"]), amount: asInt(amount), swatch: isSw}
		c.items = append(c.items, it)
		if _, seen := c.rows[it.tmpl]; seen {
			continue
		}
		row, err := m.One(`select amount, last_updated from landsraad_house_rewards where player_id=? and house_name=? and template_id=?`, me.player, house, it.tmpl)
		if err != nil {
			return res, err
		}
		if row != nil {
			upd, _ := toFloat(row["last_updated"])
			c.rows[it.tmpl] = rewardRow{amount: asInt(row["amount"]), updated: upd}
		}
	}
	c.held = held && heldTerm == termID
	ev, err := eventsOf(m, me.player, taskID)
	if err != nil {
		return res, err
	}
	el, err := otherFactionGround(m, me, taskID)
	if err != nil {
		return res, err
	}
	c.earned = math.Max(ev, el)
	c.paid = func() float64 { // before any row is changed: the record is trusted as far as the rows of this term show it
		rec, err := paidNow()
		if err != nil {
			return 0
		}
		p, _, err := paidGround(m, rec, me, taskID, termID, house, clock)
		if err != nil {
			return 0
		}
		return p
	}
	p := planTakeBack(c, to, cur)
	for _, r := range p.rows { // the plan is made, and read what it needs, before the first row changes
		if r.amount > 0 {
			_, err = m.Exec(`update landsraad_house_rewards set amount=? where player_id=? and house_name=? and template_id=?`, r.amount, me.player, house, r.tmpl)
		} else {
			_, err = m.Exec(`delete from landsraad_house_rewards where player_id=? and house_name=? and template_id=?`, me.player, house, r.tmpl)
		}
		if err != nil {
			return res, err
		}
		if r.swatch {
			if _, err = m.Exec(`delete from landsraad_player_one_time_house_rewards where player_id=? and house_name=? and template_id=? and term_id=?`, me.player, house, r.tmpl, termID); err != nil {
				return res, err
			}
		}
	}
	res.takenBack, res.keptCollected, res.keptUnsure, res.swatchesBack = p.count()
	return res, nil
}

// countRewards words a number of rewards: "1 waiting reward", "2 rewards" (adj goes before the noun).
func countRewards(n int64, adj string) string {
	if n == 1 {
		return "1 " + adj + "reward"
	}
	return fmt.Sprintf("%d %srewards", n, adj)
}

// boardTakeBack tells the board, for each reached tier of a house (the others carry empty strings), what removing that tier (and every tier above it) would do to its reward: r["takeback"] is take, collected, none or
// unsure (r["takeback_why"] says why), worked out by planTakeBack from the rows the board's read already holds. The page's question before a removal and the tier's mouse-over say it.
func boardTakeBack(tiers []any, house string, delivered map[string]save.Row, one oneTime, termID int64, termStart, mine, earned, paid float64) {
	c := takeBackCtx{termStart: termStart, earned: earned, paid: func() float64 { return paid }, rows: map[string]rewardRow{}}
	if held, ok := one.held[house]; ok && held == termID {
		c.held = true
	}
	for _, x := range tiers {
		r := x.(save.Row)
		th, _ := toFloat(r["threshold"])
		it := tierItem{th: th, tmpl: fmt.Sprint(r["shown_template_id"]), own: fmt.Sprint(r["template_id"]), amount: asInt(r["shown_amount"]), swatch: r["swatch"] == true}
		c.items = append(c.items, it)
		if d, ok := delivered[house+"\x00"+it.tmpl]; ok {
			upd, _ := toFloat(d["last_updated"])
			c.rows[it.tmpl] = rewardRow{amount: asInt(d["amount"]), updated: upd}
		}
	}
	prev := 0.0
	for _, x := range tiers {
		r := x.(save.Row)
		th, _ := toFloat(r["threshold"])
		r["takeback"], r["takeback_why"] = "", "" // every tier carries both keys; only a reached tier has an outcome
		if r["reached"] == true {
			for _, b := range planTakeBack(c, prev, mine).tiers {
				if b.th == th {
					r["takeback"], r["takeback_why"] = b.outcome, b.why
				}
			}
		}
		prev = th
	}
}
