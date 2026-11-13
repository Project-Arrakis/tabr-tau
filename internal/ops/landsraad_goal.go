package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- Fill faction goal as a completion at the top tier (#601)
//
// What a real completion leaves in a save, read from the shapes in the fixtures of this package (the character's house Maros of landsraad_mine_test.go, the game-written rows of
// landsraad_clock_test.go) and the schema:
//
//	landsraad_tasks                         completed 1, winning_faction_id = the faction, completion_time = a value of the game's play clock inside the term's window (#465)
//	landsraad_task_faction_contributions    the faction's progress at or above the house goal (goal_amount, 4,000 in the term read)
//	landsraad_task_player_contributions     the character's own contribution under that faction at or above the last reward tier (6,000: the "top tier"; the tiers are
//	                                        personal, the goal is the faction's, see #363)
//
// The game also records each contribution as a landsraad_task_progress event (tied to the player by landsraad_task_progress_player) and, when it hands a tier out, a row of
// landsraad_house_rewards. Those are the game's own acts on the contribution (#380): the editor does not write them, here as in Fill chosen, and never writes a reward row.
//
// The old Fill faction goal wrote only the second line, as a number. This one writes all three with the paths the other buttons already use: completeHouse (the progress at the
// goal, the winner and the play clock, as Complete chosen does) and the character's contribution raised to the highest tier of the house, as Fill chosen does. Nothing is ever lowered.

// topTier is the highest reward threshold of a task, 0 for a task that has no reward tier.
func topTier(m *save.Mut, taskID int64) (float64, error) {
	r, err := m.One(`select max(threshold) t from landsraad_task_rewards where task_id=?`, taskID)
	if err != nil || r == nil {
		return 0, err
	}
	t, _ := toFloat(r["t"])
	return t, nil
}

// FillFactionGoal completes one house of the current term for a faction at the highest reward (args task_id, faction_id, term_id: the term the page showed). The faction's
// progress is raised to the house goal, the house is marked complete with the faction as winner and the play clock as the time (the code of Complete chosen), and the character's own
// contribution under that faction is raised to the highest reward tier of the house (the code of Fill chosen). Nothing is lowered and no reward row is written. A house that is
// already in that state changes nothing. A save with no character gets the first two only. It is one edit.
func (o *Ops) FillFactionGoal(a Args) (any, error) {
	taskID, e1 := a.Int("task_id")
	faction, e2 := a.Int("faction_id")
	termID, e3 := a.Int("term_id")
	if err := errors.Join(e1, e2, e3); err != nil {
		return nil, err
	}
	me, perr := o.player() // no character: the faction part is still written, the personal part is not
	var progressDone, completedDone, mineDone bool
	var top, before float64
	var house, fname string
	if _, err := o.S.Mutate("landsraad", func(m *save.Mut) error {
		progressDone, completedDone, mineDone = false, false, false
		f, err := m.One(`select name from factions where id=? and name<>'None'`, faction)
		if err != nil {
			return err
		}
		if f == nil {
			return apierr.New(apierr.ErrBadArgument, "that faction is not one the Landsraad has")
		}
		fname = fmt.Sprint(f["name"])
		if err := currentTerm(m, termID); err != nil {
			return err
		}
		t, err := chosenTask(m, taskID, termID)
		if err != nil {
			return err
		}
		house = strings.TrimPrefix(fmt.Sprint(t["house_name"]), "DA_House")
		if top, err = topTier(m, taskID); err != nil {
			return err
		}
		goal, _ := toFloat(t["goal_amount"])
		if fc, err := m.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, faction, taskID); err != nil {
			return err
		} else if fc != nil {
			have, _ := toFloat(fc["amount"])
			progressDone = have < goal
		} else {
			progressDone = true
		}
		completedDone = !(t["completed"] == int64(1) && t["winning_faction_id"] == faction)
		if perr == nil && top > 0 {
			cur, err := m.One(`select amount from landsraad_task_player_contributions where player_id=? and faction_id=? and task_id=?`, me.Controller, faction, taskID)
			if err != nil {
				return err
			}
			if cur != nil {
				var ok bool
				if before, ok = toFloat(cur["amount"]); !ok {
					return apierr.Newf(apierr.ErrBadArgument, "your recorded contribution to %s is not a number, so nothing was changed", house)
				}
			}
			mineDone = before < top
		}
		if !progressDone && !completedDone && !mineDone {
			return nil
		}
		if _, err := completeHouse(m, t, faction, true); err != nil {
			return err
		}
		if mineDone {
			if _, err := m.Exec(`insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values(?,?,?,?)
				on conflict(player_id, faction_id, task_id) do update set amount=excluded.amount`, me.Controller, faction, taskID, top); err != nil {
				return err
			}
		}
		m.Desc = fmt.Sprintf("landsraad %s: completed for %s at the top tier", house, fname)
		if _, err := repairCompletionTimes(m, termID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	changed := int64(0)
	if progressDone || completedDone || mineDone {
		changed = 1
	}
	return resetAnswer(changed, map[string]any{
		"house": house, "faction": fname, "tier": top,
		"progress_changed": progressDone, "completed_changed": completedDone, "contribution_changed": mineDone, "contribution_before": before,
		"has_character": perr == nil,
	}), nil
}
