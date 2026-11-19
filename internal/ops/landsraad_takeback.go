package ops

import (
	"fmt"
	"math"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- what a tier removal takes back (#944, refs #609, #656, #777, #827)
//
// Removing a tier lowers the character's own contribution (landsraad_apply.go). It also takes back the reward of each removed tier that is still waiting at the house representative:
// the row of landsraad_house_rewards the raise wrote for that tier (landsraad_rewards.go). There is ONE rule for it, planTakeBack below. The write calls it inside the edit and the
// board's read calls it to say, before the click, what the click would do, so the sentence before the click and the write cannot disagree.
//
// The rule. The removed tiers are the tiers whose threshold lies above the new contribution and at or below the old one. A reward is a ROW (one per item and house: two tiers that give the
// same item are one row), and each row of the removed tiers has one outcome:
//
//	earned      the character's own progress events (the game's record, eventsOf) or its contribution under another faction (otherFactionGround) cover the tier: the game paid it for the
//	            character's own missions, so a reward waiting for it stays whatever the row says (a tier with no waiting row is simply "none")
//	none        the character has no row for the tier's item, or only a collected one of an earlier term: nothing waits
//	earlier     the row is waiting but was last written before this term started: it is not this term's tier, it stays
//	collected   the row is of this term and holds 0: the reward was collected and stays. Nothing here ever touches a collected reward
//	shared      the same item is also the reward of a tier that is not removed (or is earned): a waiting amount cannot be told from that tier's, it stays
//	unproven    the editor's own record (landsraad_paid.go, trusted as far as the save shows reward rows) must say that it paid the ground of the tier, or the row is not known to be the
//	            editor's and stays; for the highest tier's house swatch the character's one-time swatch row must also be of this term. The record vouches for a RANGE of ground, from the
//	            floor it started paying at (paidNote.Floor) up to what it paid, and a tier at or below the floor may hold a reward the game wrote there: it stays. A record that does not
//	            know its floor (an older build wrote it) vouches for no tier
//	short       the row holds less than the removed tiers of the item paid: it cannot be the whole of them, it stays
//	take        the row is waiting, of this term, the item is the removed tiers' alone, the editor paid the ground and the amount covers it: the removed tiers' amount comes off the row (the row
//	            is deleted at 0; what is left stays), and for the swatch the character's one-time row of this term is deleted too, so the swatch can be won again
//
// Only the character's own rows are looked at (player_id = the character), only the rows of the removed tiers' items, only the house of the change. Nothing else is written: not the
// wallets, not the specialization XP, not the faction's progress, not the progress events or the cursor (#655).
//
// A raise after a take-back. The missions the removed tiers paid (Solari, House Scrip, XP, the faction's progress) are NOT taken back, so a raise to the same ground must not pay them again,
// however often the tier is removed and raised. The editor's record of the ground it paid (landsraad_paid.go) is trusted only as far as the save shows reward rows; a take-back removes the
// rows on purpose, so it writes one more thing into that record, in the same edit: the tier it took rows up to (paidNote.Back). The record then counts as paid, for the missions and the
// faction's progress, up to the higher of the highest tier whose row stands and Back; for the reward rows it counts only up to the highest tier whose row stands, so the next raise writes the
// removed rows again and pays nothing else. Back is only ever raised.

// the outcomes of one removed tier and of one row
const (
	tbTake      = "take"
	tbCollected = "collected"
	tbNone      = "none"
	tbEarned    = "earned" // a reward waits for the tier, but the character's own missions cover it: it stays
	tbUnsure    = "unsure" // the tier's reward stays because it cannot be tied to the tier with certainty; why says why
)

// the reasons of tbEarned and tbUnsure
const (
	tbWhyEarned   = "your own missions in the game cover this tier"
	tbWhyEarlier  = "the waiting reward was written in an earlier term"
	tbWhyShared   = "the same item is also the reward of another tier"
	tbWhyShort    = "less is waiting than the tier paid"
	tbWhyUnproven = "the editor has no record that it paid this tier"
	tbWhyBelow    = "the editor only started paying above this tier, so the reward may be the game's"
	tbWhySwatch   = "the editor has no record that it wrote this swatch"
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
	paid      func() groundPaid    // what the editor's own record vouches for, as far as the save shows reward rows (rows) and from where it started paying (floor); read only when a row is in question
	rows      map[string]rewardRow // the character's rows for the house, by item
}

// tierBack is the outcome for one removed tier.
type tierBack struct {
	th      float64
	tmpl    string
	amount  int64
	outcome string
	why     string
}

// rowBack is the outcome for one row of the removed tiers (a row stands for every removed tier that gives its item); a row with nothing waiting or collected is not listed. For a take, keep
// is the amount the row keeps and swatch says that the character's one-time swatch row goes too; upTo is the highest removed tier the row stands for.
type rowBack struct {
	tmpl    string
	outcome string
	why     string
	keep    int64
	swatch  bool
	upTo    float64
}

type takeBackPlan struct {
	tiers []tierBack // the removed tiers, from the lowest
	rows  []rowBack  // one per row, in the order of the tiers that first named the item
}

// planTakeBack is the rule above for a contribution that goes from cur down to to.
func planTakeBack(c takeBackCtx, to, cur float64) takeBackPlan {
	var p takeBackPlan
	inSet := map[int]bool{} // the removed tiers that are not earned: the ones whose reward may be taken back
	at := map[int]int{}     // the index of a removed tier of c.items in p.tiers
	for i, it := range c.items {
		if !(it.th > to && it.th <= cur) {
			continue
		}
		at[i] = len(p.tiers)
		p.tiers = append(p.tiers, tierBack{th: it.th, tmpl: it.tmpl, amount: it.amount})
		if it.th > c.earned {
			inSet[i] = true
		}
	}
	waits := func(tmpl string) bool {
		row, has := c.rows[tmpl]
		return has && row.amount > 0 && row.updated >= c.termStart
	}
	sums, swatchOf, topOf, lowOf := map[string]int64{}, map[string]bool{}, map[string]float64{}, map[string]float64{}
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
		if low, ok := lowOf[it.tmpl]; !ok || it.th < low {
			lowOf[it.tmpl] = it.th
		}
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
		// only a row the editor is known to have written is taken back: its record says it paid the ground and, for the swatch, the one-time row is of this term
		if swatchOf[tmpl] && !c.held {
			return tbUnsure, tbWhySwatch
		}
		var g groundPaid
		if c.paid != nil {
			g = c.paid()
		}
		if !g.known || g.rows < topOf[tmpl] {
			if swatchOf[tmpl] {
				return tbUnsure, tbWhySwatch
			}
			return tbUnsure, tbWhyUnproven
		}
		if !g.vouches(lowOf[tmpl], topOf[tmpl]) { // a tier of the row is at or below where the editor started paying, or in ground between two of its stretches: the game may have written it
			return tbUnsure, tbWhyBelow
		}
		if row.amount < sums[tmpl] {
			return tbUnsure, tbWhyShort
		}
		return tbTake, ""
	}
	seen := map[string]bool{}
	for _, tmpl := range order {
		o, why := decide(tmpl)
		seen[tmpl] = true
		for i, it := range c.items {
			if inSet[i] && it.tmpl == tmpl {
				p.tiers[at[i]].outcome, p.tiers[at[i]].why = o, why
			}
		}
		if o != tbNone {
			p.rows = append(p.rows, rowBack{tmpl: tmpl, outcome: o, why: why, keep: c.rows[tmpl].amount - sums[tmpl], swatch: swatchOf[tmpl], upTo: topOf[tmpl]})
		}
	}
	// a removed tier the character's own missions cover: a reward that waits for it stays, and the answer says so on its own, apart from the rows that cannot be tied to a tier
	for i, it := range c.items {
		if _, removed := at[i]; !removed || inSet[i] {
			continue
		}
		if !waits(it.tmpl) {
			p.tiers[at[i]].outcome = tbNone
			continue
		}
		p.tiers[at[i]].outcome, p.tiers[at[i]].why = tbEarned, tbWhyEarned
		if !seen[it.tmpl] {
			seen[it.tmpl] = true
			p.rows = append(p.rows, rowBack{tmpl: it.tmpl, outcome: tbEarned, why: tbWhyEarned})
		}
	}
	return p
}

// count says how many ROWS had each outcome (two removed tiers that give one item are one row), and how many swatches are taken back.
func (p takeBackPlan) count() (taken, collected, unsure, earned, swatches int64) {
	for _, r := range p.rows {
		switch r.outcome {
		case tbTake:
			taken++
			if r.swatch {
				swatches++
			}
		case tbCollected:
			collected++
		case tbUnsure:
			unsure++
		case tbEarned:
			earned++
		}
	}
	return
}

// backTo is the highest removed tier whose row the plan takes back, or 0: what the editor's record is told (paidNote.Back).
func (p takeBackPlan) backTo() float64 {
	best := 0.0
	for _, r := range p.rows {
		if r.outcome == tbTake {
			best = math.Max(best, r.upTo)
		}
	}
	return best
}

// takeBackTierRewards is the write of the rule, inside the edit, for one house whose contribution goes from cur down to to (landsraad_apply.go). paidNow reads the editor's record, as the
// raise does; a record that cannot be read does not stop a removal: no row is then provably the editor's and every waiting reward stays.
func takeBackTierRewards(m *save.Mut, me myLandsraad, t save.Row, cur, to float64, paidNow func() ([]paidNote, error), wasTaken *float64) (rewardResult, error) {
	var res rewardResult
	taskID, _ := t["id"].(int64)
	house := fmt.Sprint(t["house_name"])
	termID, _ := t["term_id"].(int64)
	tierRows, err := m.Query(`select threshold, template_id, amount from landsraad_task_rewards where task_id=? order by threshold`, taskID)
	if err != nil {
		return res, err
	}
	if len(tierRows) == 0 {
		if wasTaken != nil && *wasTaken != 0 { // a house with no tiers takes nothing back: a question that said otherwise was about another house
			return res, apierr.Newf(apierr.ErrConflict, "what removing the tiers on %v would take back is not what you were shown (0 waiting rewards now), so nothing was changed. The board was read again; check it and click again", houseOf(t))
		}
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
	var refused error
	var vouched groundPaid
	asked := false
	c.paid = func() groundPaid { // asked once, before any row is changed: the record is trusted as far as the rows of this term show it
		if asked {
			return vouched
		}
		asked = true
		if m.NotesDropped() {
			return groundPaid{} // a record that lost notes of ground it paid does not say what it vouches for: the rewards stay, and the removal goes through (a raise is refused instead)
		}
		rec, err := paidNow()
		if err != nil {
			return groundPaid{} // a record that cannot be read vouches for nothing: the rewards stay
		}
		pg, err := paidGround(m, rec, me, taskID, termID, house, clock)
		if err != nil {
			refused = err
			return groundPaid{}
		}
		vouched = pg
		return vouched
	}
	p := planTakeBack(c, to, cur)
	if refused != nil {
		return res, refused // a read of the save failed: nothing was changed yet
	}
	// the question before an Unfill said how many waiting rewards it would take back (the board's read, boardUnfill, worked it out by this same plan): when this plan takes back another number
	// of rows now, the page is stale and nothing is written (the whole request is refused, one edit), #932
	if wasTaken != nil {
		if taken, _, _, _, _ := p.count(); math.Abs(float64(taken)-*wasTaken) > 1e-6 {
			return res, apierr.Newf(apierr.ErrConflict, "what removing the tiers on %v would take back is not what you were shown (%d waiting rewards now), so nothing was changed. The board was read again; check it and click again", houseOf(t), taken)
		}
	}
	for _, r := range p.rows { // the plan is made, and read what it needs, before the first row changes
		if r.outcome != tbTake {
			continue
		}
		if r.keep > 0 {
			_, err = m.Exec(`update landsraad_house_rewards set amount=? where player_id=? and house_name=? and template_id=?`, r.keep, me.player, house, r.tmpl)
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
	// the missions the removed tiers paid stay paid, and the record must say so: the rows are gone on purpose, so the next raise to this ground writes them again and pays nothing else
	if back := p.backTo(); back > 0 {
		// the floor the note carries is where the stretch that vouched for the highest row began (a row is taken only when the record vouches for it, so there is one): the lowest of every
		// faction's entries would claim the ground between two stretches for this faction's entry (#1002)
		span, _ := vouched.spanOf(back, back)
		floor := span.lo
		if err := m.Note(paidNoteKind, paidNote{Player: me.player, Term: termID, House: house, Faction: me.faction, Paid: back, Back: back, Floor: &floor}); err != nil {
			return res, err
		}
	}
	res.takenBack, res.keptCollected, res.keptUnsure, res.keptEarned, res.swatchesBack = p.count()
	return res, nil
}

// countRewards words a number of rewards: "1 waiting reward", "2 rewards" (adj goes before the noun).
func countRewards(n int64, adj string) string {
	if n == 1 {
		return "1 " + adj + "reward"
	}
	return fmt.Sprintf("%d %srewards", n, adj)
}

// boardTakeBack tells the board, for each reached tier of a house (the others carry empty strings), what removing that tier (and every tier above it) would do to its reward: r["takeback"] is take,
// collected, none, earned or unsure (r["takeback_why"] says why), worked out by planTakeBack from the rows the board's read already holds. paid is what the editor's record vouches for: the ground as far as the save shows reward rows (groundPaid.rows) and the floor it started paying from. The page's question before a removal and the tier's mouse-over say it.
func boardTakeBack(tiers []any, house string, delivered map[string]save.Row, one oneTime, termID int64, termStart, mine, earned float64, paid groundPaid) {
	c := boardTakeBackCtx(tiers, house, delivered, one, termID, termStart, earned, paid)
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

// boardTakeBackCtx is the context planTakeBack reads, built from the rows the board's read already holds (the one place the board builds it, so the per-tier outcome and the unfill preview cannot differ).
func boardTakeBackCtx(tiers []any, house string, delivered map[string]save.Row, one oneTime, termID int64, termStart, earned float64, paid groundPaid) takeBackCtx {
	c := takeBackCtx{termStart: termStart, earned: earned, paid: func() groundPaid { return paid }, rows: map[string]rewardRow{}}
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
	return c
}

// boardUnfill tells the board what "Unfill chosen" would do to the rewards of one house whose contribution is mine: every reached tier is removed at once (to 0), so it is planTakeBack(c, 0, mine),
// the same rule and the same inputs as the write (takeBackTierRewards). tiers is the number of tiers removed; take, collected, earned and unsure count ROWS (two tiers that give one item are one
// row), as the answer of the write does; rows lists each row with the amount the removed tiers gave, the outcome and the reason. The page words the question from it and sends take back as
// was_taken, so the write is refused when it would take back another number of rows than the question said.
func boardUnfill(tiers []any, house string, delivered map[string]save.Row, one oneTime, termID int64, termStart, mine, earned float64, paid groundPaid) map[string]any {
	c := boardTakeBackCtx(tiers, house, delivered, one, termID, termStart, earned, paid)
	p := planTakeBack(c, 0, mine)
	taken, collected, unsure, earnedN, swatches := p.count()
	rows := make([]any, 0, len(p.rows))
	for _, r := range p.rows {
		var amount int64
		for _, t := range p.tiers {
			if t.tmpl == r.tmpl && t.outcome == r.outcome {
				amount += t.amount
			}
		}
		rows = append(rows, map[string]any{"template_id": r.tmpl, "amount": amount, "outcome": r.outcome, "why": r.why, "swatch": r.swatch})
	}
	return map[string]any{"tiers": int64(len(p.tiers)), "take": taken, "collected": collected, "earned": earnedN, "unsure": unsure, "swatches": swatches, "rows": rows}
}
