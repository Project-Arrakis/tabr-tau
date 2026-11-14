package ops

import (
	"errors"
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- the board's write (part 2 of the tab rework, refs #599, #440)
//
// The board stages its changes on the page and writes them in one request. Each house in the request carries what the page showed (was_*) and what it wants:
//
//	mine_to       the character's own contribution to the house, under the character's own faction. It must be 0 or one of the house's reward thresholds.
//	              A value above the recorded one raises it (the code of Fill chosen). A value below it is a tier removal: it is refused unless the request says lower: true,
//	              and then it sets the contribution to that threshold, or deletes the row for 0. Nothing else in the editor lowers a contribution.
//	complete_for  a faction id: the house is completed for that faction (completeHouse: the faction's progress is raised to the goal, never lowered; the winner and the play
//	              clock are set). It never writes the character's own row.
//	reopen        the house is reopened (reopenHouse): completed, the winner and the time are cleared, no progress and no contribution is touched.
//
// Fill is mine_to = the top tier; Fill faction goal is mine_to = the top tier and complete_for = the faction; Complete is complete_for; Reopen is reopen; a tier click is
// mine_to = that tier's threshold, and the removal of the highest tier is mine_to = the tier below it, or 0, with lower: true.
//
// Before anything is written the term and every house are compared with what the page showed (was_mine, was_done, was_win); a difference is a conflict and nothing is
// written. It is one edit: either every house is done or none is. A contribution that is raised also writes the event and the reward rows (landsraad_rewards.go).

type applyItem struct {
	task        int64
	wasMine     float64
	wasDone     bool
	wasWin      any // nil or the faction id the page showed
	mineTo      *float64
	lower       bool
	completeFor *int64
	reopen      bool
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
		if v, has := it["was_win"]; has && v != nil {
			w, err := it.Int("was_win")
			if err != nil {
				return nil, err
			}
			r.wasWin = w
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

// LandsraadApply writes the staged changes of the board (args term_id: the term the page showed; houses: the list described above). It is one edit and it is refused as a whole
// when the term, or any house, is no longer what the page showed.
func (o *Ops) LandsraadApply(a Args) (any, error) {
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
	var houses, changed, raised, lowered, completed, reopened int64
	var rw rewardResult
	if _, err := o.S.Mutate("landsraad", func(m *save.Mut) error {
		houses, changed, raised, lowered, completed, reopened = 0, 0, 0, 0, 0, 0
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
			f, err := m.One(`select name from factions where id=? and name<>'None'`, *it.completeFor)
			if err != nil {
				return err
			}
			if f == nil {
				return apierr.New(apierr.ErrBadArgument, "that faction is not one the Landsraad has")
			}
			facName[*it.completeFor] = fmt.Sprint(f["name"])
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
			if it.completeFor != nil || it.reopen {
				done := t["completed"] == int64(1)
				if done != it.wasDone || t["winning_faction_id"] != it.wasWin {
					return apierr.Newf(apierr.ErrConflict, "house %v was taken or reopened since you looked, so nothing was changed: reload the page and look again", houseOf(t))
				}
			}
			if it.mineTo != nil {
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
					return apierr.Newf(apierr.ErrConflict, "your contribution to %v is not what you were shown, so nothing was changed: reload the page and look again", houseOf(t))
				}
				to := *it.mineTo
				if to != float64(int64(to)) {
					return apierr.New(apierr.ErrBadArgument, "mine_to is not a reward tier of the house")
				}
				if to > 0 {
					r, err := m.One(`select 1 x from landsraad_task_rewards where task_id=? and threshold=?`, it.task, int64(to))
					if err != nil {
						return err
					}
					if r == nil {
						return apierr.Newf(apierr.ErrBadArgument, "%v has no reward tier at %g", houseOf(t), to)
					}
				}
				if to < s.cur && !it.lower {
					return apierr.Newf(apierr.ErrBadArgument, "%v: the contribution would go down from %g to %g; only a tier removal may do that", houseOf(t), s.cur, to)
				}
			}
			steps = append(steps, s)
		}
		for _, s := range steps {
			it := s.it
			did := false
			if it.mineTo != nil {
				to := *it.mineTo
				switch {
				case to > s.cur:
					// how far the same change takes the faction's own progress (the event records it)
					facDelta := 0.0
					if it.completeFor != nil && *it.completeFor == me.faction {
						goal, _ := toFloat(s.t["goal_amount"])
						have := 0.0
						if fc, err := m.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, me.faction, it.task); err != nil {
							return err
						} else if fc != nil {
							have, _ = toFloat(fc["amount"])
						}
						facDelta = max(goal-have, 0)
					}
					if _, err := m.Exec(`insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values(?,?,?,?)
						on conflict(player_id, faction_id, task_id) do update set amount=excluded.amount`, me.player, me.faction, it.task, to); err != nil {
						return err
					}
					raised++
					did = true
					clock := clockOfTerm(m, termID)
					if !clock.ok {
						return apierr.New(apierr.ErrBadArgument, "the save has no record of this term, so the rewards cannot be timed: nothing was changed")
					}
					rr, err := deliverContribution(m, me, s.t, s.cur, to, facDelta, clock)
					if err != nil {
						return err
					}
					rw.add(rr)
				case to < s.cur && to == 0:
					if _, err := m.Exec(`delete from landsraad_task_player_contributions where player_id=? and faction_id=? and task_id=?`, me.player, me.faction, it.task); err != nil {
						return err
					}
					lowered++
					did = true
				case to < s.cur:
					if _, err := m.Exec(`update landsraad_task_player_contributions set amount=? where player_id=? and faction_id=? and task_id=?`, to, me.player, me.faction, it.task); err != nil {
						return err
					}
					lowered++
					did = true
				}
			}
			if it.completeFor != nil {
				c, err := completeHouse(m, s.t, *it.completeFor, true)
				if err != nil {
					return err
				}
				if c {
					completed++
					did = true
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
				}
			}
			houses++
			if did {
				changed++
			}
		}
		if changed > 0 {
			m.Desc = fmt.Sprintf("landsraad: %d houses changed from the board", changed)
		}
		_, rerr := repairCompletionTimes(m, termID)
		return rerr
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, map[string]any{"houses": houses, "skipped": houses - changed, "contribution_raised": raised, "contribution_lowered": lowered, "completed": completed, "reopened": reopened, "events": rw.events, "reward_rows": rw.rewardRows, "swatches": rw.swatches, "rewards_already_given": rw.skipped}), nil
}

// houseOf names a task row's house for a message.
func houseOf(t save.Row) string {
	s := fmt.Sprint(t["house_name"])
	if len(s) > 8 && s[:8] == "DA_House" {
		return s[8:]
	}
	return s
}
