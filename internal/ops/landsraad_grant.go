package ops

import (
	"fmt"
	"math"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- what completing a mission grants (refs #609, #652, #655, #656)
//
// A board write that raises the character's contribution to a house stands for the missions that would have raised it. What one mission leaves in a save was measured on copies of a
// real save (three missions on three houses, then one more on each of the other lines; the numbers are on #609):
//
//	the house side      one landsraad_task_progress event with its player link, the cursor landsraad_task_progress_processed moved to it, the character's and the faction's contribution
//	                    raised (the card's 1,000 times LandsraadContributionMultiplier), one landsraad_house_rewards row per reward tier the character's own contribution crossed,
//	                    and the house completed by the event that takes the faction's progress to the goal
//	                    (the editor writes the contribution rows and the reward rows, and the completion; NOT the event, its player link or the cursor, see below)
//	the wallet          +6,000 on currency 0 (Solari) and +1,000 on currency 1 (House Scrip); neither is scaled by a setting. The mission card gives the House Scrip and the specialization XP as the
//	                    Recollection reward (#609); whether a mission without a Recollection pays them is not known (#745), so the editor writes them as a mission with a Recollection turned in pays
//	the specialization  +1,500 times LandsraadSpecializationXpMultiplier XP on the track of the house's line, a row created when the line has none
//
// What is NOT written: any progress event, its player link and the cursor (#655, below), the character's own XP (+350 on the card, level and skill points follow) and the faction standing (not seen in any save: it was at its maximum in every sample),
// and the daily and weekly limits some community guides quote (625 specialization XP a day, 35 missions a week, unverified) are ignored, as every other Landsraad write ignores them.
//
// No progress event (#655). The game saves an event above its cursor (landsraad_task_progress_processed) with its contribution and rewards already applied, but what it does with
// an event it has not processed is not known, and one written in a shape it never writes could be applied a second time (the contribution added again, a reward handed out again). The
// editor therefore writes no event, no player link and no cursor move: the contribution row, the faction's row and the completion it brings, the reward rows, the wallets and the
// specialization XP are the whole raise. At its next cursor step the game should find only events of its own, so nothing the editor wrote should be applied or delivered twice by
// it; whether the game then keeps a contribution raised here, or works it out again from its own events, is not checked in the game. Events an earlier build wrote stay in the
// save and still count as ground paid (eventsOf).
// What the event also did, remember the ground, is done elsewhere (#777): the editor records the ground it paid in a small file of its own (landsraad_paid.go), outside the save and the
// game's tables, attached to the edit that paid it as a note (save.Mut.Note) and written once that edit is in the game's file. A tier removal lowers the contribution and not that record, so
// the same raise after a removal pays nothing again (the missions, the reward rows of the tiers crossed, the faction's progress), a higher raise pays only the new tiers, and the clicking of
// the highest tier twice pays once. The answer says how much ground and how many missions were not paid for that reason.
// A removal that takes back the waiting reward rows of the removed tiers (landsraad_takeback.go, #944) leaves the missions paid and records, in the same record, the tier it took rows up to
// (paidNote.Back). The reward rows are then the one thing a later raise to the same ground writes again: the missions, the wallets, the specialization XP and the faction's progress are not
// paid a second time, however often the tier is removed and raised.
// Ground that stays covered: the character's own events, the highest contribution the character reached under another faction (otherFactionGround; the contribution row is per faction
// and a faction change writes only player_faction) and the editor's own record, which counts only as far as the save shows it (up to the highest tier whose reward row of this term stands; a raise that pays when the cap cut it replaces the record; a restored backup keeps it as far as the restored file's own rows stand). A record that cannot be used is read as nothing paid and these guards stay.
//
// The number of missions a change stands for is the number of mission steps it takes the character across, worked from the contribution the character's own events already
// cover: ceil(after / step) - ceil(lo / step), where step is 1,000 times the contribution multiplier and lo is the larger of the contribution before and the sum of the character's
// own events for the task (the per-tier rule of #656; the events are the game's, the editor writes none, #655). The same ground is not counted twice while the contribution stands, so a second Fill grants nothing; a Fill after a removal grants again what the character's own events do not cover.

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

// specializationLines are the five lines in the order the Specialization tab lists them; the order only fixes which two lines a refusal names first.
var specializationLines = []string{"Crafting", "Gathering", "Exploration", "Combat", "Sabotage"}

// checkTracks refuses the settings when two lines would write to the same track number. The number of a line is the learned one where there is one and the measured one otherwise, so
// one learned number can land on the measured number of another line (#750); the XP of both lines would then go to one row and the other row would never be written.
func (g GrantSettings) checkTracks() error {
	seen := map[int64]string{}
	for _, line := range specializationLines {
		n := g.track(line)
		if other, dup := seen[n]; dup {
			return apierr.Newf(apierr.ErrBadArgument, "The specialization track numbers are not usable: %s and %s both use number %d, so the experience of a mission could be written to the wrong line. Nothing was changed; correct the assignment on the Specialization tab and try again.", other, line, n)
		}
		seen[n] = line
	}
	return nil
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
	return houseLine[houseShort(house)]
}

// housePrefix is what the save writes before a house's name.
const housePrefix = "DA_House"

// houseShort is a house's save name without the prefix ("DA_HouseArgosaz" is "Argosaz"); a name that is the prefix alone, or has none, is kept as it is. It is the one place the prefix is
// cut: houseOf (for a message) and lineOfHouse (for the table) both go through it.
func houseShort(house string) string {
	if len(house) > len(housePrefix) && strings.HasPrefix(house, housePrefix) {
		return house[len(housePrefix):]
	}
	return house
}

// ceilSteps is ceil(x / step) as a float, for x >= 0 and step > 0. It stays a float so that the caller compares it with the cap before it is turned into a whole number: a step so
// small that the quotient does not fit an int64 must be refused, not wrapped round.
func ceilSteps(x, step float64) float64 {
	if x <= 0 || step <= 0 {
		return 0
	}
	return math.Ceil(x/step - 1e-9)
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

// otherFactionGround is the character's highest contribution to the task under any faction but the one it is in now (0 when there is none). Called after the row of the current
// faction was set, so that row is left out.
func otherFactionGround(m *save.Mut, me myLandsraad, taskID int64) (float64, error) {
	r, err := m.One(`select coalesce(max(amount),0) a from landsraad_task_player_contributions where player_id=? and task_id=? and faction_id<>?`, me.player, taskID, me.faction)
	if err != nil {
		return 0, err
	}
	a, _ := toFloat(r["a"])
	return a, nil
}

// grantRaise writes what completing the missions a raise stands for grants, inside the edit, after the character's own contribution row was set to after (landsraad_apply.go). The
// order of the writes is: the faction's contribution and the completion, the reward rows (and the one-time swatch row), the two wallets, the specialization track. No event is
// written (#655). A change whose ground the character's own events already cover (after is not above lo) writes none of them. completeMine is true when the same request
// completes the house for the character's own faction; that completion is made here, so the faction's share is the goal at least. completed says whether this call completed the
// house.
func grantRaise(m *save.Mut, me myLandsraad, t save.Row, before, after float64, completeMine bool, clock termClock, g GrantSettings, recorded []paidNote) (res rewardResult, completed bool, err error) {
	taskID, _ := t["id"].(int64)
	house := fmt.Sprint(t["house_name"])
	termID, _ := t["term_id"].(int64)
	goal, _ := toFloat(t["goal_amount"])
	if after <= before {
		return res, false, nil
	}
	// what the character's own events already cover: the ground their missions paid (the board reads a tier from its reward row, not from the events, #655)
	cover, err := eventsOf(m, me.player, taskID)
	if err != nil {
		return res, false, err
	}
	// what the character reached under another faction counts too: the contribution row is per faction, the editor writes no event (#655) that would cover the ground for the player
	// whatever the faction, and a faction change (which writes only player_faction) would otherwise let the same raise pay again
	elsewhere, err := otherFactionGround(m, me, taskID)
	if err != nil {
		return res, false, err
	}
	// what the editor itself paid before and remembers (landsraad_paid.go): a tier removal does not take it back, so the same raise pays only the ground above it (#777); lo0 is the
	// ground covered without that record
	pg, err := paidGround(m, recorded, me, taskID, termID, house, clock)
	if err != nil {
		return res, false, err
	}
	// Two grounds (#944). lo is the ground whose missions and faction progress are paid already: the record counts there up to the highest tier whose reward row stands or the tier a
	// take-back removed rows up to. loRows is the ground whose reward rows are delivered already: the record counts only up to the highest tier whose row stands, so the rows a take-back
	// removed are written again, and only they: a raise after a take-back pays no mission and no progress twice.
	lo0 := max(before, cover, elsewhere)
	lo := max(lo0, pg.ground)
	loRows := max(lo0, pg.rows)
	if after <= lo {
		if after > loRows { // rows a take-back removed, and nothing else
			rr, err := writeTierRewards(m, me, taskID, house, termID, before, loRows, after, clock)
			if err != nil {
				return res, false, err
			}
			res.rewardRows, res.swatches, res.skipped = rr.rewardRows, rr.swatches, rr.skipped
		} else {
			n, err := m.One(`select count(*) n from landsraad_task_rewards where task_id=? and threshold>? and threshold<=?`, taskID, before, after)
			if err != nil {
				return res, false, err
			}
			res.skipped = asInt(n["n"])
		}
		res.notePaidBefore(after, lo0, lo, g)
		return res, false, nil
	}
	if g.Problem != "" {
		return res, false, apierr.New(apierr.ErrBadArgument, g.Problem)
	}
	line := lineOfHouse(house)
	step := missionContribution * g.ContributionMultiplier
	if step <= 0 {
		return res, false, apierr.New(apierr.ErrBadArgument, zeroContributionProblem)
	}
	// compared as a float, before it becomes a whole number: a tiny step makes the quotient overflow, and NaN (infinity minus infinity) is refused too
	if span := ceilSteps(after, step) - ceilSteps(lo, step); !(span <= maxMissionsAtOnce) {
		return res, false, apierr.Newf(apierr.ErrBadArgument, "%v: this change stands for more than the %d missions one change may grant for one house. Nothing was changed; click the tiers one at a time, or set LandsraadContributionMultiplier in ServerCustomSettings.ini (Config tab) nearer to 1.", houseOf(t), maxMissionsAtOnce)
	}
	missions := int64(ceilSteps(after, step) - ceilSteps(lo, step))
	// the experience of a mission needs a line and a usable track only when a mission is written and it carries experience (#762): a raise inside one step, or a multiplier whose
	// experience rounds to nothing, writes none and is not refused for what it would not write
	if missions > 0 && g.xpPerMission() > 0 {
		if line == "" {
			return res, false, apierr.Newf(apierr.ErrBadArgument, "%v is not a house the editor knows a specialization line for, so the experience a mission gives cannot be written. Nothing was changed; choose only other houses, or use Complete for this one.", houseOf(t))
		}
		if err := g.checkTracks(); err != nil {
			return res, false, err
		}
	}
	gained := after - lo
	res.notePaidBefore(after, lo0, lo, g)

	// 1. the faction's contribution, while the house is open, and the completion the faction's progress brings
	facBefore := 0.0
	if fc, err := m.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, me.faction, taskID); err != nil {
		return res, false, err
	} else if fc != nil {
		facBefore, _ = toFloat(fc["amount"])
	}
	open := t["completed"] != int64(1)
	// the faction gains what the character gained while the house is open, as a mission adds it (an event after the completion carries no faction progress); a house the same request
	// completes for the character's faction (Fill faction goal) is also taken to the goal at least, never lowered
	facNew := facBefore
	switch {
	case completeMine && open:
		// the same as a plain Fill (the faction gains what the character gained, as the game adds a whole event), and never less than the goal (#751)
		facNew = max(facBefore+gained, goal)
	// a house that is not open gains nothing from the raise; completeHouse below takes the faction to the goal at least and never lowers it, so it needs no case here
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
	// 2. no progress event. The editor writes none and does not touch the game's cursor (#655): an event above the cursor is a record the game has not processed, and what the game
	// does with one is not known, so the contribution and the reward rows below are the whole write. The game's own events are only read (eventsOf), as the ground the character's
	// own missions already paid.

	// 3. the reward rows of the tiers above the ground already paid (own events, another faction, the editor's own record)
	rr, err := writeTierRewards(m, me, taskID, house, termID, before, loRows, after, clock)
	if err != nil {
		return res, false, err
	}
	res.rewardRows, res.swatches, res.skipped = rr.rewardRows, rr.swatches, rr.skipped

	// 4. the wallets and 5. the specialization track
	res.missions = missions
	if missions > 0 {
		res.solari, res.scrip = missions*missionSolari, missions*missionScrip
		if _, err := addWallet(m, me.player, currencySolari, res.solari); err != nil {
			return res, false, err
		}
		if _, err := addWallet(m, me.player, currencyScrip, res.scrip); err != nil {
			return res, false, err
		}
		if xp := missions * g.xpPerMission(); xp > 0 {
			applied, err := addSpecialization(m, me.player, g.track(line), xp)
			if err != nil {
				return res, false, err
			}
			if applied > 0 {
				res.spec = map[string]int64{line: applied}
			}
			if applied < xp { // the track is at its last level: what did not fit is said, so a filled house is not a success line with no word about the experience (#752)
				res.specLeft = map[string]int64{line: xp - applied}
			}
		}
	}
	// 6. what was paid is remembered, outside the save (landsraad_paid.go): the note goes with this edit, so it is kept only if the edit is, and is handed to the editor's own file
	// once the game's file has been written. The ground is the whole of what this raise took the contribution to, whatever part of it an earlier raise had already paid; when the record was held back by what the
	// save shows (landsraad_paid.go), the note replaces it instead of being merged into it.
	// the ground the editor began paying from: the rows are written from loRows, so the tiers above it are the editor's (#944); a record never has a lower start than this raise's own
	floor := loRows
	if err := m.Note(paidNoteKind, paidNote{Player: me.player, Term: termID, House: house, Faction: me.faction, Paid: after, Back: pg.back, Floor: &floor, Replace: pg.untrusted}); err != nil {
		return res, false, err
	}
	return res, completed, nil
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

// Preview is what the board page needs to say, before a click, what a raise will grant: the size of a mission step in contribution, what one mission pays and the reason a raise would be
// refused (the same reasons grantRaise gives that do not depend on the house). The page works out the missions of a click from these and from the contribution and the character's own
// events of the house, with the arithmetic of grantRaise.
func (g GrantSettings) Preview() map[string]any {
	p := map[string]any{"step": missionContribution * g.ContributionMultiplier, "solari": int64(missionSolari), "scrip": int64(missionScrip),
		"xp": g.xpPerMission(), "max_missions": int64(maxMissionsAtOnce), "problem": ""}
	switch {
	case g.Problem != "":
		p["problem"] = g.Problem
	case g.ContributionMultiplier <= 0:
		p["problem"] = zeroContributionProblem
	case g.xpPerMission() > 0:
		if err := g.checkTracks(); err != nil {
			p["problem"] = err.Error()
		}
	}
	return p
}

const zeroContributionProblem = "LandsraadContributionMultiplier is 0, so a mission adds no contribution and the missions a click stands for cannot be counted. Nothing was changed; set it to a number above 0 in ServerCustomSettings.ini (Config tab) and try again."

// xpPerMission is the specialization XP one mission stands for with these settings (0 when the multiplier makes it round to nothing).
func (g GrantSettings) xpPerMission() int64 {
	return int64(math.Round(missionSpecXP * g.SpecializationXPMultiplier))
}

// grantText is what the missions of a raise wrote, for the description of the edit (the list of pending changes and the write log), in two parts: left is what did not fit (the
// experience of a track at its last level), which goes with the short facts of the description, and paid is the amounts, which come after them. The write log cuts a description at a
// fixed length, so what a person needs to decide on a restore (the wins taken away, the times moved, the experience left out) must come before the amounts (#752).
type grantText struct{ left, paid string }

// grantDesc words a raise's pay-out: "; 3 Sabotage XP not added (last level)" and "; 12 missions: +72,000 Solari, +12,000 House Scrip, +18,000 Gathering XP; 9 reward rows". For
// several houses (several) the experience is one total, to keep the line short. Both parts are empty for a change that wrote none of it.
func grantDesc(r rewardResult, several bool) grantText {
	var g grantText
	total := func(m map[string]int64) (n int64) {
		for _, x := range m {
			n += x
		}
		return n
	}
	switch {
	case total(r.specLeft) > 0 && several:
		g.left = "; " + commas(total(r.specLeft)) + " XP not added"
	case total(r.specLeft) > 0:
		for _, l := range sortedLines(r.specLeft) {
			g.left += fmt.Sprintf("; %s %s XP not added (last level)", commas(r.specLeft[l]), l)
		}
	}
	if r.keptCollected > 0 { // facts before the amounts: what a removal left, for a person who decides on a restore
		g.left += "; " + countRewards(r.keptCollected, "collected ") + stayWord(r.keptCollected)
	}
	if r.keptEarned > 0 {
		g.left += "; " + countRewards(r.keptEarned, "waiting ") + stayWord(r.keptEarned) + " (your own missions cover the tier)"
	}
	if r.keptUnsure > 0 {
		g.left += "; " + countRewards(r.keptUnsure, "") + " left alone (not certain they belong to the tier)"
	}
	var parts []string
	if r.takenBack > 0 {
		parts = append(parts, "took back "+countRewards(r.takenBack, "waiting "))
	}
	if r.missions > 0 {
		list := []string{"+" + commas(r.solari) + " Solari", "+" + commas(r.scrip) + " House Scrip"}
		if several {
			if xp := total(r.spec); xp > 0 {
				list = append(list, "+"+commas(xp)+" XP")
			}
		} else {
			for _, l := range sortedLines(r.spec) {
				list = append(list, "+"+commas(r.spec[l])+" "+l+" XP")
			}
		}
		word := "missions"
		if r.missions == 1 {
			word = "mission"
		}
		parts = append(parts, fmt.Sprintf("%s %s: %s", commas(r.missions), word, strings.Join(list, ", ")))
	}
	if r.rewardRows > 0 {
		word := "reward rows"
		if r.rewardRows == 1 {
			word = "reward row"
		}
		parts = append(parts, fmt.Sprintf("%s %s", commas(r.rewardRows), word))
	}
	if len(parts) > 0 {
		g.paid = "; " + strings.Join(parts, "; ")
	}
	return g
}

// stayWord is " stays" for one and " stay" for more.
func stayWord(n int64) string {
	if n == 1 {
		return " stays"
	}
	return " stay"
}

// sortedLines is the keys of a by-line map in the order of the Specialization tab's lines.
func sortedLines(m map[string]int64) []string {
	var out []string
	for _, l := range specializationLines {
		if _, ok := m[l]; ok {
			out = append(out, l)
		}
	}
	return out
}

// SpecRoom is, for each specialization line, the experience the character's track still has room for before its last level (the board shows what a raise adds, and the server adds
// no more than fits, #762). Every line is in the map; a save with no character, or with no track table, gives no map at all (nil), and the page then shows the uncapped number.
func (o *Ops) SpecRoom(g GrantSettings) map[string]int64 {
	p, err := o.player()
	if err != nil {
		return nil
	}
	room := map[string]int64{}
	for _, line := range specializationLines {
		r, err := o.S.One(`select xp_amount xp from specialization_tracks where player_id=? and track_type=?`, p.Controller, g.track(line))
		if err != nil {
			return nil
		}
		have := int64(0)
		if r != nil {
			have = asInt(r["xp"])
		}
		room[line] = max(maxSpecXP-have, 0)
	}
	return room
}
