package ops

import (
	"fmt"
	"math"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- what completing a mission grants (refs #609, #652, #656)
//
// A board write that raises the character's contribution to a house stands for the missions that would have raised it. What one mission leaves in a save was measured on copies of a
// real save (three missions on three houses, then one more on each of the other lines; the numbers are on #609):
//
//	the house side      one landsraad_task_progress event with its player link, the cursor landsraad_task_progress_processed moved to it, the character's and the faction's contribution
//	                    raised (the card's 1,000 times LandsraadContributionMultiplier), one landsraad_house_rewards row per reward tier the character's own contribution crossed,
//	                    and the house completed by the event that takes the faction's progress to the goal
//	the wallet          +6,000 on currency 0 (Solari) and +1,000 on currency 1 (House Scrip); neither is scaled by a setting
//	the specialization  +1,500 times LandsraadSpecializationXpMultiplier XP on the track of the house's line, a row created when the line has none
//
// What is NOT written: the character's own XP (+350 on the card, level and skill points follow) and the faction standing (not seen in any save: it was at its maximum in every sample),
// and the daily and weekly limits some community guides quote (625 specialization XP a day, 35 missions a week, unverified) are ignored, as every other Landsraad write ignores them.
//
// The number of missions a change stands for is the number of mission steps it takes the character across, worked from the contribution the character's own events already
// cover: ceil(after / step) - ceil(lo / step), where step is 1,000 times the contribution multiplier and lo is the larger of the contribution before and the sum of the character's
// own events for the task (the per-tier rule of #656). The same ground is never counted twice, so a second Fill, or a Fill after a removal, grants nothing.

const (
	missionContribution = 1000.0 // the card's Landsraad contribution, before LandsraadContributionMultiplier
	missionSpecXP       = 1500.0 // the card's specialization XP, before LandsraadSpecializationXpMultiplier
	missionSolari       = 6000   // currency 0
	missionScrip        = 1000   // currency 1, House Scrip
	currencySolari      = 0
	currencyScrip       = 1
	maxMissionsAtOnce   = 1000 // a change worth more than this is refused: it is not a board tier of any save read
)

// GrantSettings are the numbers a mission's amounts depend on: the two Landsraad multipliers of ServerCustomSettings.ini (1 where the key is not in the file) and the numbers the
// editor learned for the specialization tracks (the measured ones where it learned none).
type GrantSettings struct {
	ContributionMultiplier     float64
	SpecializationXPMultiplier float64
	TrackNumbers               map[string]int64
	// Problem is set when the settings file holds a value that is not a number from 0 to 10. A write that raises a contribution is refused with this text; a write that does not
	// need the numbers (a removal, Complete, Reopen) is not.
	Problem string
}

// DefaultGrantSettings is the stock single-player settings: both multipliers 1, the measured track numbers.
func DefaultGrantSettings() GrantSettings {
	return GrantSettings{ContributionMultiplier: 1, SpecializationXPMultiplier: 1}
}

// measuredTrackNumbers are the numbers the game uses in specialization_tracks.track_type, measured on #609 (a mission on each line): 1 Crafting, 2 Gathering, 3 Exploration, 4 Combat,
// 5 Sabotage. A number the editor learned for a line (the Assign button of the Specialization tab) takes their place.
var measuredTrackNumbers = map[string]int64{"Crafting": 1, "Gathering": 2, "Exploration": 3, "Combat": 4, "Sabotage": 5}

func (g GrantSettings) track(line string) int64 {
	if n, ok := g.TrackNumbers[line]; ok {
		return n
	}
	return measuredTrackNumbers[line]
}

// houseLine is the specialization line each house raises, keyed by the name the save writes without the DA_House prefix. Source: the public community Landsraad tracker
// (htwsguild.com/Molbork/lr_tracker.html, read 2026-11-14, recorded on #609 and #646), five houses per line; the save's spelling of two names differs from the tracker's
// (Mikkarol for Mikarrol, Spinette for Spinnette). Two missions on Argosaz (Exploration, track 3) and one on Mikkarol (Combat, track 4) agree with it; the save itself holds no link
// between a house and a line.
var houseLine = map[string]string{
	"Imota": "Combat", "Mikkarol": "Combat", "Moritani": "Combat", "Novebruns": "Combat", "Thorvald": "Combat",
	"Alexin": "Crafting", "Ecaz": "Crafting", "Hagal": "Crafting", "Lindaren": "Crafting", "Maros": "Crafting",
	"Argosaz": "Exploration", "Hurata": "Exploration", "Taligari": "Exploration", "Varota": "Exploration", "Wallach": "Exploration",
	"Dyvetz": "Gathering", "Kenola": "Gathering", "Sor": "Gathering", "Tseida": "Gathering", "Wayku": "Gathering",
	"Mutelli": "Sabotage", "Richese": "Sabotage", "Spinette": "Sabotage", "Vernius": "Sabotage", "Wydras": "Sabotage",
}

// lineOfHouse is the specialization line of a house by its save name ("DA_HouseArgosaz"), or "" for a house the table does not hold.
func lineOfHouse(house string) string {
	name := house
	if len(name) > 8 && name[:8] == "DA_House" {
		name = name[8:]
	}
	return houseLine[name]
}

// ceilSteps is ceil(x / step) for x >= 0 and step > 0.
func ceilSteps(x, step float64) int64 {
	if x <= 0 || step <= 0 {
		return 0
	}
	return int64(math.Ceil(x/step - 1e-9))
}

// eventsOf is the sum of player_progress of the character's own events for the task: the contribution the game's own record of this character's missions covers.
func eventsOf(m *save.Mut, player, taskID int64) (float64, error) {
	r, err := m.One(`select coalesce(sum(p.player_progress),0) s from landsraad_task_progress p join landsraad_task_progress_player l on l.progress_id=p.id where p.task_id=? and l.player_id=?`, taskID, player)
	if err != nil {
		return 0, err
	}
	s, _ := toFloat(r["s"])
	return s, nil
}

// grantRaise writes what completing the missions a raise stands for grants, inside the edit, after the character's own contribution row was set to after (landsraad_apply.go). The
// order of the writes is: the faction's contribution and the completion, the event with its player link and the cursor, the reward rows (and the one-time swatch row), the two wallets,
// the specialization track. A change whose ground the character's own events already cover (after is not above lo) writes none of them. completeMine is true when the same request
// completes the house for the character's own faction; that completion is made here, so the event can record the faction's share. completed says whether this call completed the
// house.
func grantRaise(m *save.Mut, me myLandsraad, t save.Row, before, after float64, completeMine bool, clock termClock, g GrantSettings) (res rewardResult, completed bool, err error) {
	taskID, _ := t["id"].(int64)
	house := fmt.Sprint(t["house_name"])
	termID, _ := t["term_id"].(int64)
	goal, _ := toFloat(t["goal_amount"])
	if after <= before {
		return res, false, nil
	}
	// what the character's own events already cover: the tiers up to there are delivered (#656)
	cover, err := eventsOf(m, me.player, taskID)
	if err != nil {
		return res, false, err
	}
	lo := max(before, cover)
	if after <= lo {
		n, err := m.One(`select count(*) n from landsraad_task_rewards where task_id=? and threshold>? and threshold<=?`, taskID, before, after)
		if err != nil {
			return res, false, err
		}
		res.skipped = asInt(n["n"])
		return res, false, nil
	}
	if g.Problem != "" {
		return res, false, apierr.New(apierr.ErrBadArgument, g.Problem)
	}
	line := lineOfHouse(house)
	if line == "" && g.SpecializationXPMultiplier > 0 {
		return res, false, apierr.Newf(apierr.ErrBadArgument, "%v is not a house the editor knows a specialization line for, so the experience a mission gives cannot be written: nothing was changed", houseOf(t))
	}
	step := missionContribution * g.ContributionMultiplier
	missions := int64(1)
	if step > 0 {
		missions = ceilSteps(after, step) - ceilSteps(lo, step)
	}
	if missions > maxMissionsAtOnce {
		return res, false, apierr.Newf(apierr.ErrBadArgument, "%v: this change stands for %d missions, more than the %d one change may grant, so nothing was changed", houseOf(t), missions, maxMissionsAtOnce)
	}
	gained := after - lo

	// 1. the faction's contribution, while the house is open, and the completion the faction's progress brings
	facBefore := 0.0
	if fc, err := m.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, me.faction, taskID); err != nil {
		return res, false, err
	} else if fc != nil {
		facBefore, _ = toFloat(fc["amount"])
	}
	open := t["completed"] != int64(1)
	// a house the same request completes for the character's faction takes the faction to the goal (Fill faction goal, never lowered); otherwise the faction gains what the character
	// gained while the house is open, as a mission adds it (an event after the completion carries no faction progress)
	facNew := facBefore
	switch {
	case completeMine:
		facNew = max(facBefore, goal)
	case open:
		facNew = facBefore + gained
	}
	if facNew != facBefore {
		if _, err := m.Exec(`insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values(?,?,?)
			on conflict(faction_id, task_id) do update set amount=excluded.amount`, me.faction, taskID, facNew); err != nil {
			return res, false, err
		}
	}
	if completeMine || (open && goal > 0 && facNew >= goal) {
		if completed, err = completeHouse(m, t, me.faction, true); err != nil {
			return res, false, err
		}
	}
	facAfter := facBefore
	if fc, err := m.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, me.faction, taskID); err != nil {
		return res, false, err
	} else if fc != nil {
		facAfter, _ = toFloat(fc["amount"])
	}

	// 2. the event, its player link and the cursor. The event records after - cover as the character's progress: what this change adds beyond the events and, when the contribution
	// was ahead of them (a Fill written before the events existed), the gap too, so the sum of the character's events is again the contribution and a later removal and raise cannot
	// count the same ground twice. The event's id is above every event and above the cursor, and the cursor moves to it (the game's own saves do the same, #609).
	top, err := m.One(`select coalesce(max(id),0) a, coalesce((select last_processed_id from landsraad_task_progress_processed where id=1),0) b from landsraad_task_progress`)
	if err != nil {
		return res, false, err
	}
	a, _ := top["a"].(int64)
	b, _ := top["b"].(int64)
	id := max(a, b) + 1
	if _, err := m.Exec(`insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values(?,?,?,?,0,?,?)`,
		id, me.faction, taskID, int64(math.Round(max(facAfter-facBefore, 0))), after-cover, clock.value); err != nil {
		return res, false, err
	}
	if _, err := m.Exec(`insert into landsraad_task_progress_player(progress_id, player_id) values(?,?)`, id, me.player); err != nil {
		return res, false, err
	}
	if _, err := m.Exec(`insert into landsraad_task_progress_processed(id, last_processed_id) values(1,?)
		on conflict(id) do update set last_processed_id=max(last_processed_id, excluded.last_processed_id)`, id); err != nil {
		return res, false, err
	}
	res.events = 1

	// 3. the reward rows of the tiers the events did not cover
	rr, err := writeTierRewards(m, me, taskID, house, termID, before, lo, after, clock)
	if err != nil {
		return res, false, err
	}
	res.rewardRows, res.swatches, res.skipped = rr.rewardRows, rr.swatches, rr.skipped

	// 4. the wallets and 5. the specialization track
	res.missions = missions
	if missions > 0 {
		res.solari, res.scrip = missions*missionSolari, missions*missionScrip
		if err := addWallet(m, me.player, currencySolari, res.solari); err != nil {
			return res, false, err
		}
		if err := addWallet(m, me.player, currencyScrip, res.scrip); err != nil {
			return res, false, err
		}
		if xp := missions * int64(math.Round(missionSpecXP*g.SpecializationXPMultiplier)); xp > 0 {
			applied, err := addSpecialization(m, me.player, g.track(line), xp)
			if err != nil {
				return res, false, err
			}
			if applied > 0 {
				res.spec = map[string]int64{line: applied}
			}
		}
	}
	return res, completed, nil
}

// addWallet adds to one of the character's virtual currency balances (a wallet with no row is created).
func addWallet(m *save.Mut, controller, currency, amount int64) error {
	cur, err := m.One(`select balance from player_virtual_currency_balances where player_controller_id=? and currency_id=?`, controller, currency)
	if err != nil {
		return err
	}
	have := int64(0)
	if cur != nil {
		have = asInt(cur["balance"])
	}
	_, err = m.Exec(`insert into player_virtual_currency_balances(player_controller_id, currency_id, balance) values(?,?,?)
		on conflict(player_controller_id, currency_id) do update set balance=excluded.balance`, controller, currency, have+amount)
	return err
}

// addSpecialization adds XP to one track of the character, never past the last level, and sets the level to match; a line with no row gets one. It returns the XP that was added
// (less than asked when the track reaches its last level).
func addSpecialization(m *save.Mut, controller, number, xp int64) (int64, error) {
	cur, err := m.One(`select xp_amount xp from specialization_tracks where player_id=? and track_type=?`, controller, number)
	if err != nil {
		return 0, err
	}
	have := int64(0)
	if cur != nil {
		have = asInt(cur["xp"])
	}
	next := min(have+xp, max(maxSpecXP, have))
	if cur != nil && next == have {
		return 0, nil
	}
	// a track at its last level is stored as level 100 (Grant max does the same; the saves read show 44,182 XP at level 100)
	level := specLevel(next)
	if next >= maxSpecXP {
		level = 100
	}
	return next - have, upsertSpecInTx(m, controller, number, next, level)
}
