package ops

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Landsraad returns the current term, the task board with faction progress and
// rewards, and the decree list.
//
// A task carries two different scales (#363). goal_amount is the faction's goal for the house (the game shows it as "Goal Amount: 0 / 4,000"); the
// rewards are tiers of the player's own contribution (1200 to 6000 in the term read so far) and a contribution can go above the goal. Both are returned
// as the save holds them, nothing is clamped or derived. player_contributions lists the rows of landsraad_task_player_contributions for the task as they
// are, with the player id each belongs to.
//
// Whose row is the character's (#380): the player id of those tables is the id of the character's player_state row (the same number as the controller actor and as
// player_faction.actor_id); the other ids are the actors of the simulated guilds. In the save read for this (a character with 7,850 contributed to one house) the
// character's row is the only one whose id is not a LandsraadSimulatedGuild actor, it equals the sum of the three progress rows that landsraad_task_progress_player
// ties to that id, and landsraad_house_rewards holds the five delivered tiers of that house under the same id. So the character's contribution is read from that row
// ("mine"), a task carries it as my_contribution, and each tier says whether it is reached (the contribution is at least the tier) and whether the save records it as
// delivered. Only the current term's tasks are returned: the save keeps the boards of the earlier terms too, and a fill of one of those would change nothing the game uses.
func (o *Ops) Landsraad() (any, error) {
	term, _ := o.S.One(`select * from landsraad_decree_term order by term_id desc limit 1`)
	taskQ, taskArgs := `select t.id, t.board_index, t.house_name, t.goal_amount, t.completed, t.winning_faction_id, t.sysselraad, t.completion_time
		from landsraad_tasks t order by t.board_index`, []any(nil)
	if tid, ok := term["term_id"].(int64); ok {
		taskQ = `select t.id, t.board_index, t.house_name, t.goal_amount, t.completed, t.winning_faction_id, t.sysselraad, t.completion_time
		from landsraad_tasks t where t.term_id=? order by t.board_index`
		taskArgs = []any{tid}
	}
	tasks, err := o.S.Query(taskQ, taskArgs...)
	if err != nil {
		return nil, err
	}
	var character map[string]any
	var mineID, mineFaction int64
	if p, perr := o.player(); perr == nil {
		mineID = p.Controller
		character = map[string]any{"player_id": p.Controller, "name": p.Name, "faction_id": nil}
		if f, _ := o.S.One(`select pf.faction_id id from player_faction pf join factions f on f.id=pf.faction_id where pf.actor_id=? and f.name<>'None'`, p.Controller); f != nil {
			if id, ok := f["id"].(int64); ok {
				mineFaction = id
				character["faction_id"] = id
			}
		}
	}
	contrib, _ := o.S.Query(`select task_id, faction_id, amount from landsraad_task_faction_contributions`)
	byTask := map[int64][]map[string]any{}
	for _, c := range contrib {
		t, ok := c["task_id"].(int64)
		if !ok { // a row of a save built by hand that names no task belongs to none
			continue
		}
		byTask[t] = append(byTask[t], map[string]any{"faction_id": c["faction_id"], "amount": c["amount"]})
	}
	pc, _ := o.S.Query(`select task_id, player_id, faction_id, amount from landsraad_task_player_contributions order by task_id, player_id`)
	byPlayer := map[int64][]map[string]any{}
	for _, c := range pc {
		t, ok := c["task_id"].(int64)
		if !ok {
			continue
		}
		mine := false
		if pid, ok := c["player_id"].(int64); ok && mineID != 0 && pid == mineID {
			mine = true
		}
		byPlayer[t] = append(byPlayer[t], map[string]any{"player_id": c["player_id"], "faction_id": c["faction_id"], "amount": c["amount"], "mine": mine})
	}
	// what the save records as delivered to the character: one row per house and item, the amounts of every term added up, with the time of the last delivery
	delivered := map[string]save.Row{}
	if mineID != 0 {
		drows, _ := o.S.Query(`select house_name, template_id, amount, last_updated from landsraad_house_rewards where player_id=?`, mineID)
		for _, d := range drows {
			delivered[fmt.Sprint(d["house_name"])+"\x00"+fmt.Sprint(d["template_id"])] = d
		}
	}
	termStart, _ := toFloat(term["start_time"])
	rewards, _ := o.S.Query(`select task_id, threshold, template_id, amount from landsraad_task_rewards order by task_id, threshold`)
	rw := map[int64][]any{}
	for _, r := range rewards {
		t, ok := r["task_id"].(int64)
		if !ok {
			continue
		}
		rw[t] = append(rw[t], r)
	}
	for _, t := range tasks {
		id, _ := t["id"].(int64)
		t["contributions"] = byTask[id]
		t["player_contributions"] = byPlayer[id]
		// the character's own contribution: the row of the faction the character is in now (a Fill writes that row)
		my, has := 0.0, false
		for _, c := range byPlayer[id] {
			if c["mine"] == true && (mineFaction == 0 || c["faction_id"] == mineFaction) {
				if a, ok := toFloat(c["amount"]); ok {
					my, has = my+a, true
				}
			}
		}
		t["my_contribution"], t["my_recorded"] = my, has
		// a tier is reached when the contribution is at least its threshold; it is delivered when the save holds a delivery of that item for the house made in this term,
		// of at least the amounts of the reached tiers of the same item (the rows add the amounts of every term up, so this cannot tell two equal items of one house apart)
		tiers := rw[id]
		for _, x := range tiers {
			r := x.(save.Row)
			th, _ := toFloat(r["threshold"])
			reached := has && my >= th
			r["reached"] = reached
			deliv := false
			if d, ok := delivered[fmt.Sprint(t["house_name"])+"\x00"+fmt.Sprint(r["template_id"])]; ok && reached {
				if upd, ok := toFloat(d["last_updated"]); ok && upd >= termStart {
					need := 0.0
					for _, y := range tiers {
						yr := y.(save.Row)
						if yr["template_id"] == r["template_id"] {
							if yt, _ := toFloat(yr["threshold"]); yt <= th {
								ya, _ := toFloat(yr["amount"])
								need += ya
							}
						}
					}
					if have, ok := toFloat(d["amount"]); ok && have >= need {
						deliv = true
					}
				}
			}
			r["delivered"] = deliv
		}
		t["rewards"] = tiers
	}
	decrees, _ := o.S.Query(`select id, decree_name, disabled, weight from landsraad_decrees order by id`)
	rotation, _ := o.S.Query(`select decree_id from landsraad_decree_rotation`)
	// What each decree is in the current term (#448). The pool flag `disabled` is the save's own column and it does not say whether a decree is in effect: in the saves read so
	// far the term's active decree (SpecialVendorActive_Vehicles) carries disabled = 1 while the base row SpecialVendorActive carries 0. So the term and the rotation are
	// matched to the rows here by the decree id, never by the name: the five SpecialVendorActive rows are five different decrees.
	inRot := map[int64]bool{}
	for _, r := range rotation {
		if id, ok := r["decree_id"].(int64); ok {
			inRot[id] = true
		}
	}
	activeID, _ := term["active_decree_id"].(int64)
	electedID, _ := term["elected_decree_id"].(int64)
	for _, d := range decrees {
		id, _ := d["id"].(int64)
		d["active"] = term["active_decree_id"] != nil && id == activeID
		d["elected"] = term["elected_decree_id"] != nil && id == electedID
		d["in_rotation"] = inRot[id]
	}
	factions, _ := o.S.Query(`select id, name from factions where name <> 'None' order by id`)
	return map[string]any{"term": term, "tasks": tasks, "decrees": decrees, "rotation": rotation, "factions": factions, "character": character}, nil
}

// SetTaskProgress sets a faction's contribution to a task (the faction's progress towards the house goal, not a player's own contribution). The amount is
// written as given: it is not held to the goal, because the game's numbers can pass it (#363).
func (o *Ops) SetTaskProgress(a Args) (any, error) {
	t, e1 := a.Int("task_id")
	f, e2 := a.Int("faction_id")
	amt, e3 := a.Float("amount")
	if err := errors.Join(e1, e2, e3); err != nil {
		return nil, err
	}
	if amt < 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "amount must be >= 0")
	}
	if amt > maxTier { // the game's numbers pass the goal (#363) but not a billion: keep a REAL column to a value it holds exactly (CR-6)
		return nil, apierr.Newf(apierr.ErrBadArgument, "amount must be at most %s", commas(int64(maxTier)))
	}
	if _, err := o.run(fmt.Sprintf("landsraad task %d faction %d progress = %g", t, f, amt), `insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values(?,?,?)
		on conflict(faction_id, task_id) do update set amount=excluded.amount`, f, t, amt); err != nil {
		return nil, err
	}
	return ok(), nil
}

// CompleteTask marks a task complete for a faction; reset=true reopens it. A completion writes the game's own clock into completion_time (see landsraad_clock.go), and the
// completed tasks of the same term that still hold a time from outside the term's window (the Unix time older editors wrote) are brought onto that clock in the same edit (#465).
func (o *Ops) CompleteTask(a Args) (any, error) {
	t, err := a.Int("task_id")
	if err != nil {
		return nil, err
	}
	reset := a.Bool("reset", false)
	var f int64
	desc := fmt.Sprintf("landsraad task %d reopened", t)
	if !reset {
		if f, err = a.Int("faction_id"); err != nil {
			return nil, err
		}
		desc = fmt.Sprintf("landsraad task %d completed for faction %d", t, f)
	}
	var n int64
	if _, err = o.S.Mutate(desc, func(m *save.Mut) error {
		clock := clockOfTask(m, t) // read before the write, from the task's own term
		var res sql.Result
		var err error
		if reset {
			res, err = m.Exec(`update landsraad_tasks set completed=0, winning_faction_id=null, completion_time=null where id=?`, t)
		} else {
			res, err = m.Exec(`update landsraad_tasks set completed=1, winning_faction_id=?, completion_time=? where id=?`, f, clock.arg(), t)
		}
		if err != nil {
			return err
		}
		if n, _ = res.RowsAffected(); n > 0 && clock.ok {
			_, err = repairCompletionTimes(m, clock.term)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, apierr.New(apierr.ErrNotFound, "task not found")
	}
	return ok(), nil
}

// SetDecree sets or clears the disabled flag of one decree row in the pool, picked by its id (#448). The flag is the save's own column; it is not whether the decree is
// in effect (that is the term's active decree). Only the row with that id changes: SpecialVendorActive and its four variants are five rows.
func (o *Ops) SetDecree(a Args) (any, error) {
	id, err := a.Int("id")
	if err != nil {
		return nil, err
	}
	dis := int64(0)
	if a.Bool("disabled", false) {
		dis = 1
	}
	n, err := o.run(fmt.Sprintf("decree %d disabled=%d", id, dis), `update landsraad_decrees set disabled=? where id=?`, dis, id)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, apierr.New(apierr.ErrNotFound, "decree not found")
	}
	return ok(), nil
}

// SetTerm updates the active/elected decree, reigning faction and term end.
func (o *Ops) SetTerm(a Args) (any, error) {
	var err error
	type edit struct {
		col string
		val any
	}
	var edits []edit
	for _, k := range []string{"active_decree_id", "elected_decree_id", "reigning_faction_id", "end_time"} {
		v, has := a[k]
		if !has {
			continue
		}
		var val any
		if s, isStr := v.(string); v == nil || (isStr && s == "") {
			val = nil
		} else if val, err = a.Int(k); err != nil {
			return nil, err
		}
		edits = append(edits, edit{k, val})
	}
	if len(edits) == 0 {
		return ok(), nil
	}
	var parts []string
	for _, e := range edits {
		parts = append(parts, fmt.Sprintf("%s = %v", e.col, e.val))
	}
	// The term is looked up inside the edit, through its own query, so it is kept with the edit: put back onto a newer file with another term, the edit is not applied to the
	// old term's id.
	_, err = o.S.Mutate("landsraad term", func(m *save.Mut) error {
		term, err := m.One(`select term_id from landsraad_decree_term order by term_id desc limit 1`)
		if err != nil {
			return err
		}
		if term == nil {
			return errors.New("no Landsraad term in this save")
		}
		m.Desc = fmt.Sprintf("landsraad term %v: %s", term["term_id"], strings.Join(parts, ", "))
		for _, e := range edits {
			if _, err := m.Exec(`update landsraad_decree_term set `+e.col+`=? where term_id=?`, e.val, term["term_id"]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ok(), nil
}

// ---------------------------------------------------------------- the character's own contribution (#380)

// myLandsraad is what a fill of the character's contribution needs from the save: the character's player id (the player_state id, which is the id the contribution tables use)
// and the Landsraad faction the character is in. A character with no faction (Neutral) has none to contribute for, so a fill is refused.
type myLandsraad struct{ player, faction int64 }

// maxTier bounds a tier: the tiers in a save are 1,200 to 6,000, and a request names one of them, so this only keeps a hand-built save from carrying a value beyond what a
// REAL column holds exactly.
const maxTier = 1e9

// fillMine raises the character's contribution to the task up to the threshold of one of its tiers, inside the edit m, and, when withFaction is true, raises the faction's
// progress on the task to its goal. It only raises: a contribution that is already at or above the tier is left as it is. The task must belong to the current term. It returns the
// contribution before and after, and whether the faction's row changed. The rewards are not written: delivering a tier is the game's own act on the contribution.
func fillMine(m *save.Mut, me myLandsraad, taskID int64, tier float64, withFaction, write bool) (before, after float64, factionChanged bool, house string, err error) {
	term, err := m.One(`select term_id from landsraad_decree_term order by term_id desc limit 1`)
	if err != nil {
		return 0, 0, false, "", err
	}
	if term == nil {
		return 0, 0, false, "", apierr.New(apierr.ErrNotFound, "this save has no Landsraad term")
	}
	t, err := m.One(`select id, term_id, house_name, goal_amount from landsraad_tasks where id=?`, taskID)
	if err != nil {
		return 0, 0, false, "", err
	}
	if t == nil {
		return 0, 0, false, "", apierr.New(apierr.ErrNotFound, "task not found")
	}
	if t["term_id"] != term["term_id"] {
		return 0, 0, false, "", apierr.New(apierr.ErrConflict, "that house belongs to an earlier term: only the current term's houses can be filled; reload the page")
	}
	house = strings.TrimPrefix(fmt.Sprint(t["house_name"]), "DA_House")
	r, err := m.One(`select 1 x from landsraad_task_rewards where task_id=? and threshold=?`, taskID, int64(tier))
	if err != nil {
		return 0, 0, false, "", err
	}
	if r == nil || tier != float64(int64(tier)) {
		return 0, 0, false, "", apierr.Newf(apierr.ErrBadArgument, "%s has no reward tier at %g", house, tier)
	}
	cur, err := m.One(`select amount from landsraad_task_player_contributions where player_id=? and faction_id=? and task_id=?`, me.player, me.faction, taskID)
	if err != nil {
		return 0, 0, false, "", err
	}
	if cur != nil {
		var ok bool
		if before, ok = toFloat(cur["amount"]); !ok {
			return 0, 0, false, "", apierr.Newf(apierr.ErrBadArgument, "your recorded contribution to %s is not a number, so it was not changed", house)
		}
	}
	after = before
	if before < tier {
		after = tier
		if write { // without write this only says what would change
			if _, err := m.Exec(`insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values(?,?,?,?)
				on conflict(player_id, faction_id, task_id) do update set amount=excluded.amount`, me.player, me.faction, taskID, after); err != nil {
				return 0, 0, false, "", err
			}
		}
	}
	if withFaction {
		goal, _ := toFloat(t["goal_amount"])
		fc, err := m.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, me.faction, taskID)
		if err != nil {
			return 0, 0, false, "", err
		}
		have := 0.0
		if fc != nil {
			have, _ = toFloat(fc["amount"])
		}
		if have < goal {
			if write {
				if _, err := m.Exec(`insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values(?,?,?)
					on conflict(faction_id, task_id) do update set amount=excluded.amount`, me.faction, taskID, goal); err != nil {
					return 0, 0, false, "", err
				}
			}
			factionChanged = true
		}
	}
	return before, after, factionChanged, house, nil
}

// myLandsraadTx reads who the character is for the Landsraad tables, inside the edit.
func myLandsraadTx(m *save.Mut, p player) (myLandsraad, error) {
	f, err := m.One(`select pf.faction_id id from player_faction pf join factions f on f.id=pf.faction_id where pf.actor_id=? and f.name<>'None'`, p.Controller)
	if err != nil {
		return myLandsraad{}, err
	}
	id, ok := int64(0), false
	if f != nil {
		id, ok = f["id"].(int64)
	}
	if !ok {
		return myLandsraad{}, apierr.New(apierr.ErrBadArgument, "your character is in no faction (Neutral), so it has no Landsraad contribution to fill: assign Atreides or Harkonnen under Admin first")
	}
	return myLandsraad{player: p.Controller, faction: id}, nil
}

// FillMyContribution raises the character's own contribution to a house up to one of its tiers (args task_id and tier: the threshold, one of the task's tiers; with_faction:
// also raise the faction's progress to the house goal, false when absent). It writes the character's row of landsraad_task_player_contributions (the faction's progress and
// the reward rows are not touched unless with_faction asks for the first; the rewards are never written) and is one edit.
func (o *Ops) FillMyContribution(a Args) (any, error) {
	id, e1 := a.Int("task_id")
	tier, e2 := a.Float("tier")
	withFaction, e3 := strictBool(a, "with_faction", false)
	if err := errors.Join(e1, e2, e3); err != nil {
		return nil, err
	}
	if tier <= 0 || tier > maxTier {
		return nil, apierr.New(apierr.ErrBadArgument, "tier must be one of the house's reward tiers")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var before, after float64
	var factionChanged bool
	if _, err := o.S.Mutate("landsraad", func(m *save.Mut) error {
		me, err := myLandsraadTx(m, p)
		if err != nil {
			return err
		}
		var house string
		if before, after, factionChanged, house, err = fillMine(m, me, id, tier, withFaction, true); err != nil {
			return err
		}
		m.Desc = fmt.Sprintf("landsraad %s: your contribution %g -> %g", house, before, after)
		if factionChanged {
			m.Desc += ", faction goal filled"
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "before": before, "after": after, "changed": after > before || factionChanged, "faction_changed": factionChanged}, nil
}

// ---------------------------------------------------------------- several houses at once (#364)

// maxFillHouses bounds one request: a board has 25 houses; the limit only keeps a hand-built request from carrying thousands.
const maxFillHouses = 100

// fillItem is one house of a multi-house fill: the task and the tier its contribution is raised to (the highest tier chosen for it).
type fillItem struct {
	task int64
	tier float64
}

// parseFillItems reads the list of houses out of a decoded body. Every entry is checked: it must be an object with a whole task_id and a tier, and a house may appear once.
func parseFillItems(a Args) ([]fillItem, error) {
	raw, ok := a["items"].([]any)
	if !ok || len(raw) == 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "items is required: the houses to fill, each with its task_id and tier")
	}
	if len(raw) > maxFillHouses {
		return nil, apierr.Newf(apierr.ErrBadArgument, "at most %d houses can be filled at once", maxFillHouses)
	}
	seen := map[int64]bool{}
	out := make([]fillItem, 0, len(raw))
	for _, x := range raw {
		m, ok := x.(map[string]any)
		if !ok {
			return nil, apierr.New(apierr.ErrBadArgument, "each of the items must be an object with task_id and tier")
		}
		e := Args(m)
		id, e1 := e.Int("task_id")
		tier, e2 := e.Float("tier")
		if err := errors.Join(e1, e2); err != nil {
			return nil, err
		}
		if tier <= 0 || tier > maxTier {
			return nil, apierr.New(apierr.ErrBadArgument, "tier must be one of the house's reward tiers")
		}
		if seen[id] {
			return nil, apierr.Newf(apierr.ErrBadArgument, "house %d is in the list twice: send the highest tier chosen for it once", id)
		}
		seen[id] = true
		out = append(out, fillItem{task: id, tier: tier})
	}
	return out, nil
}

// FillMyContributions raises the character's own contribution on several houses of the current term in one edit (args items: [{task_id, tier}], one entry per house with the
// highest tier chosen for it; with_faction: also raise the faction's progress on each to its goal; term_id: the term the page showed; expected: the number of houses the page counted would change (the page no longer asks first, #450; the count still guards against a board that changed between the click and the edit).
// Inside the edit the term is compared with term_id and the houses that would change are counted again with the rules of the single fill; a term or a
// count that is not the expected one is refused with a conflict and nothing is written. A house whose contribution is already at or above its tier does not count and is not
// written. Either every house is filled or none is: any refusal rolls the edit back. The reward rows are never written.
func (o *Ops) FillMyContributions(a Args) (any, error) {
	items, err := parseFillItems(a)
	if err != nil {
		return nil, err
	}
	withFaction, err := strictBool(a, "with_faction", false)
	if err != nil {
		return nil, err
	}
	expected, err := expectedCount(a)
	if err != nil {
		return nil, err
	}
	termID, err := a.Int("term_id")
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var changed int64
	var factions, mine int64 // houses whose faction progress / own contribution were raised (a house counts in changed when either was)
	if _, err := o.S.Mutate("landsraad", func(m *save.Mut) error {
		changed, factions, mine = 0, 0, 0
		me, err := myLandsraadTx(m, p)
		if err != nil {
			return err
		}
		term, err := m.One(`select term_id from landsraad_decree_term order by term_id desc limit 1`)
		if err != nil {
			return err
		}
		if term == nil || term["term_id"] != termID {
			return apierr.New(apierr.ErrConflict, "the Landsraad term is not the one you were shown, so nothing was changed: reload the page and look again")
		}
		for _, it := range items { // first the count, with nothing written
			before, after, fch, _, err := fillMine(m, me, it.task, it.tier, withFaction, false)
			if err != nil {
				return err
			}
			if after > before || fch {
				changed++
			}
		}
		if changed != expected {
			return apierr.Newf(apierr.ErrConflict, "the number of houses to fill is now %d, not the %d the page counted, so nothing was changed: reload the page and look again", changed, expected)
		}
		for _, it := range items {
			before, after, fch, _, err := fillMine(m, me, it.task, it.tier, withFaction, true)
			if err != nil {
				return err
			}
			if after > before {
				mine++
			}
			if fch {
				factions++
			}
		}
		if changed > 0 {
			m.Desc = fmt.Sprintf("landsraad: your contribution filled on %d houses", mine)
			if factions > 0 {
				m.Desc += fmt.Sprintf(", the faction goal on %d", factions)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, map[string]any{"houses": int64(len(items)), "faction_changed": factions, "contribution_changed": mine}), nil
}
