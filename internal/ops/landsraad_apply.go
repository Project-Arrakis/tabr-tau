package ops

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- the board's write (part 2 of the tab rework, refs #599, #440)
//
// The board writes each click at once (#641), as one request with one house or several (a bulk button). Each house in the request carries what the page showed (was_*) and what it wants:
//
//	mine_to       the character's own contribution to the house, under the character's own faction. It must be 0 or one of the house's reward thresholds.
//	              A value above the recorded one raises it (the raise of the board's write). A value below it is a tier removal: it is refused unless the request says lower: true,
//	              and then it sets the contribution to that threshold, or deletes the row for 0, and takes back the rewards of the removed tiers that still wait at the house
//	              representative (landsraad_takeback.go; collected ones stay). Nothing else in the editor lowers a contribution.
//	complete_for  a faction id: the house is completed for that faction (completeHouse: the faction's progress is raised to the goal, never lowered; the winner and the play
//	              clock are set). It never writes the character's own row.
//	reopen        the house is reopened (reopenHouse): completed, the winner and the time are cleared, no progress and no contribution is touched.
//
// Fill is mine_to = the top tier; Fill faction goal is mine_to = the top tier and complete_for = the faction; Complete is complete_for; Reopen is reopen; a tier click is
// mine_to = that tier's threshold, and the removal of the highest tier is mine_to = the tier below it, or 0, with lower: true.
//
// Before anything is written the term and every house are compared with what the page showed (was_mine, was_done, was_win, and for a raise was_events, was_paid, was_paid_rows and was_elsewhere:
// the grounds its sentence was worked out from, each compared only when sent, #762 #805); a difference is a conflict and nothing is written. It is one edit: either every house is done or none is. A contribution that is raised also grants what the missions it stands for grant (landsraad_grant.go): the faction's share and the completion it brings, the reward rows of the tiers above the ground already paid (own events, another faction, the editor's own record), the wallets and the specialization XP; one transaction, so a failure at any step leaves nothing behind.

type applyItem struct {
	task        int64
	wasMine     float64
	wasDone     bool
	wasWin      any // nil or the faction id the page showed
	mineTo      *float64
	lower       bool
	completeFor *int64
	reopen      bool
	stateShown  bool     // the page sent was_done or was_win: the state of the house is compared for every write, not only Complete and Reopen (#762)
	wasEvents   *float64 // what the page showed as the ground the character's own events cover; compared when a contribution is raised (#762)
	// what the page showed as the two other grounds a raise is worked out from (#805): what the editor itself paid before (missions, and the ground whose reward rows are delivered) and what
	// the character reached under another faction. Each is compared when a contribution is raised and only when sent: a request without it is not refused.
	wasPaid, wasPaidRows, wasElsewhere *float64
	// what the page said an Unfill would take back on this house, in rewards (#932): compared when a contribution is lowered and only when sent, so a removal of one tier is not refused for lack of it
	wasTaken *float64
}

// optFloat reads a number that may be left out (or null): nil when it is not there.
func optFloat(it Args, key string) (*float64, error) {
	v, has := it[key]
	if !has || v == nil {
		return nil, nil
	}
	f, err := it.Float(key)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func parseApplyItems(a Args) ([]applyItem, error) {
	raw, ok := a["houses"].([]any)
	if !ok || len(raw) == 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "houses is required: the houses with a change")
	}
	if len(raw) > maxFillHouses {
		return nil, apierr.Newf(apierr.ErrBadArgument, "at most %d houses can be changed at once", maxFillHouses)
	}
	seen := map[int64]bool{}
	out := make([]applyItem, 0, len(raw))
	for _, x := range raw {
		m, ok := x.(map[string]any)
		if !ok {
			return nil, apierr.New(apierr.ErrBadArgument, "each house must be an object")
		}
		it := Args(m)
		var r applyItem
		var err error
		if r.task, err = it.Int("task_id"); err != nil {
			return nil, err
		}
		if seen[r.task] {
			return nil, apierr.Newf(apierr.ErrBadArgument, "house %d is in the list twice", r.task)
		}
		seen[r.task] = true
		if _, has := it["was_mine"]; has {
			if r.wasMine, err = it.Float("was_mine"); err != nil {
				return nil, err
			}
		}
		if r.wasDone, err = strictBool(it, "was_done", false); err != nil {
			return nil, err
		}
		_, hasDone := it["was_done"]
		if v, has := it["was_win"]; has && v != nil {
			w, err := it.Int("was_win")
			if err != nil {
				return nil, err
			}
			r.wasWin = w
		}
		_, hasWin := it["was_win"]
		r.stateShown = hasDone || hasWin
		if r.wasEvents, err = optFloat(it, "was_events"); err != nil {
			return nil, err
		}
		if r.wasPaid, err = optFloat(it, "was_paid"); err != nil {
			return nil, err
		}
		if r.wasPaidRows, err = optFloat(it, "was_paid_rows"); err != nil {
			return nil, err
		}
		if r.wasElsewhere, err = optFloat(it, "was_elsewhere"); err != nil {
			return nil, err
		}
		if r.wasTaken, err = optFloat(it, "was_taken"); err != nil {
			return nil, err
		}
		if r.wasTaken != nil && (*r.wasTaken < 0 || *r.wasTaken != float64(int64(*r.wasTaken))) {
			return nil, apierr.New(apierr.ErrBadArgument, "was_taken is not a number of rewards")
		}
		if _, has := it["mine_to"]; has {
			v, err := it.Float("mine_to")
			if err != nil {
				return nil, err
			}
			if v < 0 || v > maxTier {
				return nil, apierr.New(apierr.ErrBadArgument, "mine_to is not a reward tier of the house")
			}
			r.mineTo = &v
		}
		if r.lower, err = strictBool(it, "lower", false); err != nil {
			return nil, err
		}
		if _, has := it["complete_for"]; has {
			f, err := it.Int("complete_for")
			if err != nil {
				return nil, err
			}
			r.completeFor = &f
		}
		if r.reopen, err = strictBool(it, "reopen", false); err != nil {
			return nil, err
		}
		if r.completeFor != nil && r.reopen {
			return nil, apierr.Newf(apierr.ErrBadArgument, "house %d cannot be completed and reopened in one change", r.task)
		}
		if r.mineTo == nil && r.completeFor == nil && !r.reopen {
			return nil, apierr.Newf(apierr.ErrBadArgument, "house %d has no change in the request", r.task)
		}
		out = append(out, r)
	}
	return out, nil
}

// LandsraadApply is LandsraadApplyWith with the stock settings (both multipliers 1, the measured track numbers).
func (o *Ops) LandsraadApply(a Args) (any, error) {
	return o.LandsraadApplyWith(DefaultGrantSettings(), a)
}

// LandsraadApplyWith writes the board's change (args term_id: the term the page showed; houses: the list described above). It is one edit and it is refused as a whole
// when the term, or any house, is no longer what the page showed. g gives the amounts a mission stands for (landsraad_grant.go).
func (o *Ops) LandsraadApplyWith(g GrantSettings, a Args) (any, error) {
	termID, err := a.Int("term_id")
	if err != nil {
		return nil, err
	}
	items, err := parseApplyItems(a)
	if err != nil {
		return nil, err
	}
	needMe := false
	for _, it := range items {
		if it.mineTo != nil {
			needMe = true
		}
	}
	var p player
	if needMe {
		if p, err = o.player(); err != nil {
			if errors.Is(err, errNoPlayer) {
				return nil, apierr.New(apierr.ErrBadArgument, "this save has no character, so there is no contribution of yours to change")
			}
			return nil, err
		}
	}
	var houses, changed, raised, lowered, completed, reopened, repaired int64
	var replaced []replacedWin
	var names []string // the houses that changed, and what was done to each, for the description (D1-6, #611)
	var done []string // what was done to each of them, in the same order
	var rw rewardResult
	if _, err := o.S.Mutate("landsraad", func(m *save.Mut) error {
		houses, changed, raised, lowered, completed, reopened, repaired = 0, 0, 0, 0, 0, 0, 0
		// the editor's record of the ground it paid (landsraad_paid.go), read at most once for the request, under the lock the sink writes it under, and only when a house is raised: a
		// removal pays nothing and needs no record, so a record that cannot be read does not stop it (it only keeps a house swatch from being taken back, landsraad_takeback.go). A raise that cannot read it is refused with nothing written.
		var recorded []paidNote
		recordedRead := false
		paidNow := func() ([]paidNote, error) {
			if !recordedRead {
				es, err := o.Paid.snapshot()
				if err != nil {
					return nil, err
				}
				recorded, recordedRead = es, true
			}
			return recorded, nil
		}
		replaced, names, done = nil, nil, nil
		rw = rewardResult{}
		if err := currentTerm(m, termID); err != nil {
			return err
		}
		var me myLandsraad
		if needMe {
			if me, err = myLandsraadTx(m, p); err != nil {
				return err
			}
		}
		facName := map[int64]string{}
		for _, it := range items {
			if it.completeFor == nil {
				continue
			}
			name, err := landsraadFactionName(m.One, *it.completeFor) // the one rule for a faction the Landsraad has (landsraad_factions.go)
			if err != nil {
				return err
			}
			facName[*it.completeFor] = clipName(name) // a name from a save built by hand must not fill a description
		}
		type step struct {
			it  applyItem
			t   save.Row
			cur float64
		}
		steps := make([]step, 0, len(items))
		for _, it := range items { // first look at every house, then write: a refusal leaves nothing behind
			t, err := chosenTask(m, it.task, termID)
			if err != nil {
				return err
			}
			s := step{it: it, t: t}
			if it.completeFor != nil || it.reopen || it.stateShown { // a raise or a tier removal on a house taken or reopened since the page was drawn is refused too (#762)
				isDone := t["completed"] == int64(1)
				if isDone != it.wasDone || t["winning_faction_id"] != it.wasWin {
					return apierr.Newf(apierr.ErrConflict, "house %v was taken or reopened since you looked, so nothing was changed. The board was read again; check it and click again", houseOf(t))
				}
			}
			if it.mineTo != nil {
				// the contribution is the character's, under its own faction: a request that fills it while completing the house for another faction is refused (Fill faction goal under
				// another faction records that faction's win and skips the personal row, and no reward row is written)
				if it.completeFor != nil && *it.completeFor != me.faction {
					return apierr.Newf(apierr.ErrBadArgument, "%v: your own contribution counts for your own faction, so it cannot be changed in the same step as completing the house for another faction", houseOf(t))
				}
				row, err := m.One(`select amount from landsraad_task_player_contributions where player_id=? and faction_id=? and task_id=?`, me.player, me.faction, it.task)
				if err != nil {
					return err
				}
				if row != nil {
					var ok bool
					if s.cur, ok = toFloat(row["amount"]); !ok {
						return apierr.Newf(apierr.ErrBadArgument, "your recorded contribution to %v is not a number, so nothing was changed", houseOf(t))
					}
				}
				if s.cur != it.wasMine {
					return apierr.Newf(apierr.ErrConflict, "your contribution to %v is not what you were shown, so nothing was changed. The board was read again; check it and click again", houseOf(t))
				}
				to := *it.mineTo
				if to != float64(int64(to)) {
					return apierr.New(apierr.ErrBadArgument, "mine_to is not a reward tier of the house")
				}
				// the ground the character's own events cover decides what a raise pays: a raise whose page showed another number would pay what the sentence before the click did not say (#762)
				if to > s.cur {
					if it.wasEvents != nil {
						ev, err := eventsOf(m, me.player, it.task)
						if err != nil {
							return err
						}
						if math.Abs(ev-*it.wasEvents) > 1e-6 {
							return apierr.Newf(apierr.ErrConflict, "your earlier progress on %v is not what you were shown, so nothing was changed. The board was read again; check it and click again", houseOf(t))
						}
					}
					// the other two grounds the sentence before the click is worked out from (#805), read the way the raise reads them below
					if it.wasElsewhere != nil {
						el, err := otherFactionGround(m, me, it.task)
						if err != nil {
							return err
						}
						if math.Abs(el-*it.wasElsewhere) > 1e-6 {
							return apierr.Newf(apierr.ErrConflict, "what you reached on %v under another faction is not what you were shown, so nothing was changed. The board was read again; check it and click again", houseOf(t))
						}
					}
					if it.wasPaid != nil || it.wasPaidRows != nil {
						if clock := clockOfTerm(m, termID); clock.ok { // a save with no record of the term is refused below, with its own reason
							recordedNow, err := paidNow()
							if err != nil {
								return apierr.New(apierr.ErrWriteFailed, err.Error())
							}
							pg, err := paidGround(m, recordedNow, me, it.task, termID, fmt.Sprint(t["house_name"]), clock)
							if err != nil {
								return err
							}
							if (it.wasPaid != nil && math.Abs(pg.ground-*it.wasPaid) > 1e-6) || (it.wasPaidRows != nil && math.Abs(pg.rows-*it.wasPaidRows) > 1e-6) {
								return apierr.Newf(apierr.ErrConflict, "the ground the editor paid before on %v is not what you were shown, so nothing was changed. The board was read again; check it and click again", houseOf(t))
							}
						}
					}
				}
				if to > 0 {
					r, err := m.One(`select 1 x from landsraad_task_rewards where task_id=? and threshold=?`, it.task, int64(to))
					if err != nil {
						return err
					}
					if r == nil {
						return apierr.Newf(apierr.ErrBadArgument, "%v has no reward tier at %s", houseOf(t), numText(to))
					}
				}
				if to < s.cur && !it.lower {
					return apierr.Newf(apierr.ErrBadArgument, "%v: the contribution would go down from %s to %s; only a tier removal may do that", houseOf(t), numText(s.cur), numText(to))
				}
			}
			steps = append(steps, s)
		}
		for _, s := range steps {
			it := s.it
			if it.completeFor != nil { // a win of another faction that this completion takes away is named in the answer and the write log (D1-3, #611)
				prev, err := replacedWinner(m, s.t, *it.completeFor)
				if err != nil {
					return err
				}
				if prev != "" {
					replaced = append(replaced, replacedWin{house: houseOf(s.t), winner: prev})
				}
			}
			did := false
			// what was done to this house, in words
			var what []string
			doneByRaise := false // grantRaise took the house for the character's faction: the completion below must not be made, or counted, a second time (#751)
			if it.mineTo != nil {
				to := *it.mineTo
				switch {
				case to > s.cur:
					recordedNow, err := paidNow()
					if err != nil {
						return apierr.New(apierr.ErrWriteFailed, err.Error())
					}
					// the character's own row is raised to the tier; the missions it stands for are then granted (landsraad_grant.go): the faction's share and the completion it
					// brings, the reward rows, the two wallets and the specialization track
					if _, err := m.Exec(`insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values(?,?,?,?)
						on conflict(player_id, faction_id, task_id) do update set amount=excluded.amount`, me.player, me.faction, it.task, to); err != nil {
						return err
					}
					raised++
					did = true
					what = append(what, fmt.Sprintf("your contribution %s -> %s", numText(s.cur), numText(to)))
					clock := clockOfTerm(m, termID)
					if !clock.ok {
						return apierr.New(apierr.ErrBadArgument, "the save has no record of this term, so the rewards cannot be timed: nothing was changed")
					}
					rr, took, err := grantRaise(m, me, s.t, s.cur, to, it.completeFor != nil && *it.completeFor == me.faction, clock, g, recordedNow)
					if err != nil {
						return err
					}
					if took {
						completed++
						doneByRaise = true
						if it.completeFor != nil {
							what = append(what, "completed for "+facName[*it.completeFor])
						} else {
							what = append(what, "completed, its faction reached the goal") // the raise itself took the house (the faction's progress)
						}
					}
					rw.add(rr)
					// the cap is on the whole request, not only on each house: many houses, each under it, must not add up to more than one change may grant
					if rw.missions > maxMissionsAtOnce {
						return apierr.Newf(apierr.ErrBadArgument, "this request stands for more than the %d missions one change may grant, counting every house in it. Nothing was changed; fill fewer houses at once, or set LandsraadContributionMultiplier in ServerCustomSettings.ini (Config tab) nearer to 1.", maxMissionsAtOnce)
					}
				case to < s.cur:
					var werr error
					if to == 0 {
						_, werr = m.Exec(`delete from landsraad_task_player_contributions where player_id=? and faction_id=? and task_id=?`, me.player, me.faction, it.task)
					} else {
						_, werr = m.Exec(`update landsraad_task_player_contributions set amount=? where player_id=? and faction_id=? and task_id=?`, to, me.player, me.faction, it.task)
					}
					if werr != nil {
						return werr
					}
					lowered++
					did = true
					what = append(what, fmt.Sprintf("your contribution %s -> %s", numText(s.cur), numText(to)))
					// the rewards of the removed tiers that wait at the house representative go with them (landsraad_takeback.go: the one rule); collected ones stay
					rr, err := takeBackTierRewards(m, me, s.t, s.cur, to, paidNow, it.wasTaken)
					if err != nil {
						return err
					}
					rw.add(rr) // the description of the edit says what was taken back (grantDesc)
				}
			}
			if it.completeFor != nil && !doneByRaise {
				c, err := completeHouse(m, s.t, *it.completeFor, true)
				if err != nil {
					return err
				}
				if c {
					completed++
					did = true
					what = append(what, "completed for "+facName[*it.completeFor])
				}
			}
			if it.reopen {
				c, err := reopenHouse(m, s.t, true)
				if err != nil {
					return err
				}
				if c {
					reopened++
					did = true
					what = append(what, "reopened")
				}
			}
			houses++
			if did {
				changed++
				names = append(names, houseOf(s.t))
				done = append(done, strings.Join(what, ", "))
			}
		}
		var rerr error
		repaired, rerr = repairCompletionTimes(m, termID)
		switch {
		case changed == 1: // one house: what was done to it
			m.Desc = landsraadDescG(fmt.Sprintf("landsraad %s: %s", names[0], done[0]), grantDesc(rw, false), replaced, repaired, nil)
		case changed > 1:
			m.Desc = landsraadDescG(severalHead(changed, done), grantDesc(rw, true), replaced, repaired, names)
		case repaired > 0:
			m.Desc = landsraadDesc("landsraad", nil, repaired, nil)
		}
		return rerr
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, map[string]any{"houses": houses, "skipped": houses - changed, "contribution_raised": raised, "contribution_lowered": lowered, "completed": completed, "reopened": reopened, "repaired": repaired, "replaced": replacedAnswer(replaced), "reward_rows": rw.rewardRows, "swatches": rw.swatches, "rewards_taken_back": rw.takenBack, "rewards_kept_collected": rw.keptCollected, "rewards_kept_unsure": rw.keptUnsure, "rewards_kept_earned": rw.keptEarned, "swatches_taken_back": rw.swatchesBack, "rewards_already_given": rw.skipped, "ground_paid_before": rw.paidGround, "missions_paid_before": rw.paidMissions,
		"missions": rw.missions, "solari": rw.solari, "house_scrip": rw.scrip, "specialization_xp": specAnswer(rw.spec), "specialization_xp_not_added": specAnswer(rw.specLeft)}), nil
}

// severalHead is the head of the description of an edit that changed several houses. When every house had the same plain thing done to it (completed for one faction, or reopened) the
// head says it ("landsraad: 3 houses completed for Atreides"), so the list of queued changes names the action and the faction; otherwise it says only that the houses were changed from the board.
func severalHead(n int64, done []string) string {
	generic := fmt.Sprintf("landsraad: %d houses changed from the board", n)
	if len(done) == 0 {
		return generic
	}
	for _, d := range done {
		if d != done[0] {
			return generic
		}
	}
	switch {
	case done[0] == "reopened":
		return fmt.Sprintf("landsraad: %d houses reopened", n)
	case strings.HasPrefix(done[0], "completed for ") && !strings.ContainsAny(done[0], ",;"):
		return fmt.Sprintf("landsraad: %d houses %s", n, done[0])
	}
	return generic
}

// specAnswer is the specialization XP that was added, by line, as the answer carries it (never nil, so the page can read it).
func specAnswer(m map[string]int64) map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	return m
}

// houseOf names a task row's house for a message.
func houseOf(t save.Row) string {
	return clipName(houseShort(fmt.Sprint(t["house_name"])))
}

// clipName bounds a house or faction name in a message or a description: a name from a save built by hand must not fill it.
func clipName(s string) string {
	if r := []rune(s); len(r) > maxHouseName {
		s = string(r[:maxHouseName]) + "..."
	}
	return s
}

// numText writes a number of a message or a description in full: %g would print 1000000 as 1e+06.
func numText(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// maxHouseName bounds a house or faction name in a message or a description.
const maxHouseName = 40
