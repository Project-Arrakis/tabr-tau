package ops

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// readFailed says which read of the Landsraad page failed. A read that fails is an error of the page, never an empty board: a board drawn from a failed read would offer Fill and
// Complete against a picture that is not the save (D1-2 of the delta audit, #611).
func readFailed(what string, err error) error {
	return fmt.Errorf("the Landsraad page could not read %s from the save: %w", what, err)
}

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
	term, err := o.S.One(`select * from landsraad_decree_term order by term_id desc limit 1`)
	if err != nil {
		return nil, readFailed("the Landsraad term", err)
	}
	taskQ, taskArgs := `select t.id, t.board_index, t.house_name, t.goal_amount, t.completed, t.winning_faction_id, t.sysselraad, t.completion_time
		from landsraad_tasks t order by t.board_index`, []any(nil)
	if tid, ok := term["term_id"].(int64); ok {
		taskQ = `select t.id, t.board_index, t.house_name, t.goal_amount, t.completed, t.winning_faction_id, t.sysselraad, t.completion_time
		from landsraad_tasks t where t.term_id=? order by t.board_index`
		taskArgs = []any{tid}
	}
	tasks, err := o.S.Query(taskQ, taskArgs...)
	if err != nil {
		return nil, readFailed("the Landsraad tasks", err)
	}
	var character map[string]any
	var mineID, mineFaction int64
	var here *charPlace // where the character stands, for the distance to a house (nil when it is not known)
	if p, perr := o.player(); perr == nil {
		mineID = p.Controller
		// where the character stands: the player's map and the body's position, none when the body is not where the player is (#654)
		if at, aerr := readCharacterPlace(o.S.One, p, true); aerr == nil && at.Parked == nil {
			x, okx := toFloat(at.Body["x"])
			y, oky := toFloat(at.Body["y"])
			if okx && oky {
				here = &charPlace{Map: at.Map, X: x, Y: y}
			}
		}
		character = map[string]any{"player_id": p.Controller, "name": p.Name, "faction_id": nil}
		f, ferr := o.S.One(`select pf.faction_id id from player_faction pf join factions f on f.id=pf.faction_id where pf.actor_id=? and `+landsraadFactionCond, p.Controller)
		if ferr != nil {
			return nil, readFailed("your character's faction", ferr)
		}
		if f != nil {
			if id, ok := f["id"].(int64); ok {
				mineFaction = id
				character["faction_id"] = id
			}
		}
	}
	contrib, err := o.S.Query(`select task_id, faction_id, amount from landsraad_task_faction_contributions`)
	if err != nil {
		return nil, readFailed("the factions' progress", err)
	}
	byTask := map[int64][]map[string]any{}
	for _, c := range contrib {
		t, ok := c["task_id"].(int64)
		if !ok { // a row of a save built by hand that names no task belongs to none
			continue
		}
		byTask[t] = append(byTask[t], map[string]any{"faction_id": c["faction_id"], "amount": c["amount"]})
	}
	pc, err := o.S.Query(`select task_id, player_id, faction_id, amount from landsraad_task_player_contributions order by task_id, player_id`)
	if err != nil {
		return nil, readFailed("the personal contributions", err)
	}
	// the name of a contributor that is a simulated guild (landsraad_simulated_guilds), for the board to say who gave what to a House: read only, and a save without the table or without the
	// row simply gives no name (the row then carries no guild key)
	guilds := map[int64]string{}
	if hasTable(o, "landsraad_simulated_guilds") {
		if gr, gerr := o.S.Query(`select guild_actor_id, guild_identifier from landsraad_simulated_guilds`); gerr == nil {
			for _, g := range gr {
				if id, ok := g["guild_actor_id"].(int64); ok {
					if name, ok := g["guild_identifier"].(string); ok && name != "" {
						guilds[id] = name
					}
				}
			}
		}
	}
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
		row := map[string]any{"player_id": c["player_id"], "faction_id": c["faction_id"], "amount": c["amount"], "mine": mine}
		if pid, ok := c["player_id"].(int64); ok && !mine {
			if name, ok := guilds[pid]; ok {
				row["guild"] = name
			}
		}
		byPlayer[t] = append(byPlayer[t], row)
	}
	// what the save records as delivered to the character: one row per house and item, the amounts of every term added up, with the time of the last delivery
	delivered := map[string]save.Row{}
	if mineID != 0 {
		drows, err := o.S.Query(`select house_name, template_id, amount, last_updated from landsraad_house_rewards where player_id=?`, mineID)
		if err != nil {
			return nil, readFailed("the rewards delivered to your character", err)
		}
		for _, d := range drows {
			delivered[fmt.Sprint(d["house_name"])+"\x00"+fmt.Sprint(d["template_id"])] = d
		}
	}
	// the sum of the character's own progress events per house: the ground a raise already counts as paid (#656), for the page to work out what a click grants
	events := map[int64]float64{}
	haveEvents := mineID != 0 && hasTable(o, "landsraad_task_progress") && hasTable(o, "landsraad_task_progress_player")
	if haveEvents {
		er, err := o.S.Query(`select p.task_id, sum(p.player_progress) s from landsraad_task_progress p join landsraad_task_progress_player l on l.progress_id=p.id where l.player_id=? group by p.task_id`, mineID)
		if err != nil {
			return nil, readFailed("the progress events of your character", err)
		}
		for _, e := range er {
			if tid, ok := e["task_id"].(int64); ok {
				events[tid], _ = toFloat(e["s"])
			}
		}
	}
	termStart, _ := toFloat(term["start_time"])
	termID, _ := term["term_id"].(int64)
	one, err := readOneTime(o, mineID)
	if err != nil {
		return nil, err
	}
	rewards, err := o.S.Query(`select task_id, threshold, template_id, amount from landsraad_task_rewards order by task_id, threshold`)
	if err != nil {
		return nil, readFailed("the reward tiers", err)
	}
	rw := map[int64][]any{}
	for _, r := range rewards {
		t, ok := r["task_id"].(int64)
		if !ok {
			continue
		}
		rw[t] = append(rw[t], r)
	}
	places, err := o.houseLocations() // where the house representatives are, from the save's map markers (#639)
	placesFromMarkers := true
	if err != nil {
		// the places are optional: a save whose markers cannot be read still gives the board, the term and the decrees, with the fixed place of each house (#820)
		places, placesFromMarkers = fixedHouseLocations(), false
	}
	// the ground the editor itself paid (landsraad_paid.go), raises not recorded yet included (queued, or written and kept for the record), for the page to work out what a click grants: it counts only up to the highest tier whose reward row of this term stands for the house (trustedPaid), as in the write
	// (the notes not yet recorded are read before the file: a write or a delivery that lands in between moves a note into the file, and the other order could see neither)
	unrecordedPaid := pendingPaid(o.S.PendingNotes(paidNoteKind))
	filePaid, _ := o.Paid.snapshot() // a record that could not be read shows as nothing paid here (a read); the write refuses instead
	for _, t := range tasks {
		id, _ := t["id"].(int64)
		t["contributions"] = byTask[id]
		t["my_events"] = events[id]
		t["line"] = lineOfHouse(fmt.Sprint(t["house_name"]))
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
		// the highest the character reached under another faction: ground a raise does not pay again (grantRaise), for the page to work out what a click grants
		elsewhere := 0.0
		for _, c := range byPlayer[id] {
			if c["mine"] == true && mineFaction != 0 && c["faction_id"] != mineFaction {
				if a, ok := toFloat(c["amount"]); ok && a > elsewhere {
					elsewhere = a
				}
			}
		}
		t["my_elsewhere"] = elsewhere
		house := fmt.Sprint(t["house_name"])
		sw, hasSw := one.swatch[house]
		var swRow save.Row
		if hasSw {
			swRow = sw
		}
		pg := trustedPaid(filePaid, unrecordedPaid, mineID, termID, house, standingTierRows(rw[id], swRow, delivered, house, termStart))
		t["my_paid"], t["my_paid_rows"] = pg.ground, pg.rows // the ground whose missions are paid, and the ground whose reward rows are delivered (they differ after a take-back, #944)
		// a tier is reached when the contribution is at least its threshold; whether it is delivered (and what stands at the house representative) is decided per tier by
		// tierState in landsraad_board.go: the reward row of this term (#656, #655)
		tiers := rw[id]
		for _, x := range tiers {
			r := x.(save.Row)
			th, _ := toFloat(r["threshold"])
			r["reached"] = has && my >= th
		}
		t["rewards"] = tiers
		boardTask(t, tiers, delivered, one, termID, termStart, places, here)
		// what removing a reached tier would do to its reward, by the rule of the write (landsraad_takeback.go): the page says it before the click
		boardTakeBack(tiers, house, delivered, one, termID, termStart, my, math.Max(events[id], elsewhere), pg)
	}
	decrees, err := o.S.Query(`select id, decree_name, disabled, weight from landsraad_decrees order by id`)
	if err != nil {
		return nil, readFailed("the decrees", err)
	}
	rotation, err := o.S.Query(`select decree_id from landsraad_decree_rotation`)
	if err != nil {
		return nil, readFailed("the decree rotation", err)
	}
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
		key, _ := d["decree_name"].(string)
		name, effect, known := decreeDisplay(key)
		d["display_name"], d["effect"], d["name_known"], d["variant"] = name, effect, known, decreeNames[key].Variant
	}
	factions, err := o.landsraadFactions()
	if err != nil {
		return nil, readFailed("the factions", err)
	}
	terms, earlier, err := termHistory(o, mineID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"term": term, "tasks": tasks, "decrees": decrees, "rotation": rotation, "factions": factions, "character": character, "terms": terms, "earlier_terms": earlier, "locations_from_markers": placesFromMarkers, "clock": saveClock(o)}, nil
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
// A request that names the active decree or the reigning faction is refused when the row would be left with a decree and no faction (#960).
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
		term, err := m.One(`select term_id, start_time, reigning_faction_id, active_decree_id from landsraad_decree_term order by term_id desc limit 1`)
		if err != nil {
			return err
		}
		if term == nil {
			return errors.New("no Landsraad term in this save")
		}
		// A decree is only in force because a faction won the term, so a write that leaves a decree set with no reigning faction is refused (#960). It is judged on the row as it
		// would be after the edit: the request's own value where it names the column, the stored value where it does not. A request that touches neither column (the end time only)
		// is not judged, so a save the game wrote with a decree and no faction can still have its end time changed, and clearing both together stays allowed.
		after := map[string]any{"active_decree_id": term["active_decree_id"], "reigning_faction_id": term["reigning_faction_id"]}
		judged := false
		for _, e := range edits {
			if e.col == "active_decree_id" || e.col == "reigning_faction_id" {
				after[e.col], judged = e.val, true
			}
		}
		if judged && after["active_decree_id"] != nil && after["reigning_faction_id"] == nil {
			return apierr.New(apierr.ErrBadArgument, "an active decree needs a reigning faction: choose Atreides or Harkonnen, or clear the decree as well")
		}
		for _, e := range edits { // a reigning faction that is set is one the Landsraad has (landsraad_factions.go); clearing it is allowed, and so is leaving the value the row already holds
			if id, isInt := e.val.(int64); e.col == "reigning_faction_id" && isInt {
				if held, isHeld := term["reigning_faction_id"].(int64); isHeld && held == id {
					continue
				}
				if _, err := landsraadFactionName(m.One, id); err != nil {
					return err
				}
			}
		}
		// a term that ends before it starts has no window: the completion times and the clock repair are worked out from it (D1-4, #611)
		if start, ok := toFloat(term["start_time"]); ok {
			for _, e := range edits {
				if end, isInt := e.val.(int64); e.col == "end_time" && isInt && float64(end) < start {
					return apierr.Newf(apierr.ErrBadArgument, "end_time %d is before the term's start_time %d: the term would end before it starts", end, int64(start))
				}
			}
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

// myLandsraad is what a change of the character's contribution needs from the save: the character's player id (the player_state id, which is the id the contribution tables use)
// and the Landsraad faction the character is in. A character with no faction (Neutral) has none to contribute for, so a change is refused.
type myLandsraad struct{ player, faction int64 }

// maxTier bounds a tier: the tiers in a save are 1,200 to 6,000, and a request names one of them, so this only keeps a hand-built save from carrying a value beyond what a
// REAL column holds exactly.
const maxTier = 1e9

// myLandsraadTx reads who the character is for the Landsraad tables, inside the edit.
func myLandsraadTx(m *save.Mut, p player) (myLandsraad, error) {
	f, err := m.One(`select pf.faction_id id from player_faction pf join factions f on f.id=pf.faction_id where pf.actor_id=? and `+landsraadFactionCond, p.Controller)
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

// ---------------------------------------------------------------- the size of one request

// maxFillHouses bounds one request: a board has 25 houses; the limit only keeps a hand-built request from carrying thousands.
const maxFillHouses = 100
