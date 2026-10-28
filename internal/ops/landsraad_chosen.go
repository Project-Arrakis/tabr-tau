package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- complete or reopen the chosen houses (#451)

// parseTaskIDs reads the houses of a complete or reopen request: a list of whole task ids, none twice. A board has 25 houses; maxFillHouses only bounds a hand-built request.
func parseTaskIDs(a Args) ([]int64, error) {
	raw, ok := a["task_ids"].([]any)
	if !ok || len(raw) == 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "task_ids is required: the houses that are chosen")
	}
	if len(raw) > maxFillHouses {
		return nil, apierr.Newf(apierr.ErrBadArgument, "at most %d houses can be changed at once", maxFillHouses)
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(raw))
	for _, x := range raw {
		id, err := Args{"task_id": x}.Int("task_id")
		if err != nil {
			return nil, apierr.New(apierr.ErrBadArgument, "each of the task_ids must be a whole number")
		}
		if seen[id] {
			return nil, apierr.Newf(apierr.ErrBadArgument, "house %d is in the list twice", id)
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// chosenTask reads one house of a complete or reopen request inside the edit: it must exist and belong to the term the page showed.
func chosenTask(m *save.Mut, taskID, termID int64) (save.Row, error) {
	t, err := m.One(`select id, term_id, house_name, goal_amount, completed, winning_faction_id, completion_time from landsraad_tasks where id=?`, taskID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, apierr.New(apierr.ErrNotFound, "task not found")
	}
	if t["term_id"] != termID {
		return nil, apierr.New(apierr.ErrConflict, "a chosen house belongs to an earlier term, so nothing was changed: reload the page and look again")
	}
	return t, nil
}

// currentTerm compares the term the page showed with the save's latest term, inside the edit.
func currentTerm(m *save.Mut, termID int64) error {
	term, err := m.One(`select term_id from landsraad_decree_term order by term_id desc limit 1`)
	if err != nil {
		return err
	}
	if term == nil || term["term_id"] != termID {
		return apierr.New(apierr.ErrConflict, "the Landsraad term is not the one you were shown, so nothing was changed: reload the page and look again")
	}
	return nil
}

// completeHouse marks one house complete for the faction and raises the faction's progress on it to the house goal, inside the edit. A faction's progress that is already at or
// above the goal is not lowered. A house that is already complete for the faction with its progress at the goal is skipped (nothing is written, changed is false). With write false
// nothing is written and only the answer is given.
func completeHouse(m *save.Mut, t save.Row, faction int64, write bool) (changed bool, err error) {
	id := t["id"]
	termID, _ := t["term_id"].(int64)
	clock := clockOfTerm(m, termID)
	goal, _ := toFloat(t["goal_amount"])
	have := 0.0
	fc, err := m.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, faction, id)
	if err != nil {
		return false, err
	}
	if fc != nil {
		have, _ = toFloat(fc["amount"])
	}
	doneFor := t["completed"] == int64(1) && t["winning_faction_id"] == faction
	needProgress := have < goal
	if doneFor && !needProgress {
		return false, nil
	}
	if !write {
		return true, nil
	}
	if needProgress {
		if _, err := m.Exec(`insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values(?,?,?)
			on conflict(faction_id, task_id) do update set amount=excluded.amount`, faction, id, goal); err != nil {
			return false, err
		}
	}
	if !doneFor {
		if _, err := m.Exec(`update landsraad_tasks set completed=1, winning_faction_id=?, completion_time=? where id=?`, faction, clock.arg(), id); err != nil {
			return false, err
		}
	}
	return true, nil
}

// reopenHouse reopens one house (completed=0, winning faction and completion time cleared) inside the edit. It touches no faction progress and no personal contribution. A house
// that is not complete and carries no winner or completion time is skipped.
func reopenHouse(m *save.Mut, t save.Row, write bool) (changed bool, err error) {
	open := (t["completed"] == nil || t["completed"] == int64(0)) && t["winning_faction_id"] == nil && t["completion_time"] == nil
	if open {
		return false, nil
	}
	if write {
		if _, err := m.Exec(`update landsraad_tasks set completed=0, winning_faction_id=null, completion_time=null where id=?`, t["id"]); err != nil {
			return false, err
		}
	}
	return true, nil
}

// chosenRequest is what both routes read the same way: the houses, the term the page showed and the number of houses the page counted as changing.
type chosenRequest struct {
	ids      []int64
	term     int64
	expected int64
}

func parseChosen(a Args) (chosenRequest, error) {
	var r chosenRequest
	var err error
	if r.ids, err = parseTaskIDs(a); err != nil {
		return r, err
	}
	if r.expected, err = expectedCount(a); err != nil {
		return r, err
	}
	r.term, err = a.Int("term_id")
	return r, err
}

// CompleteChosen marks several houses of the current term complete for a faction in one edit (args task_ids; faction_id; term_id: the term the page showed; expected: the number of
// houses the page counted as changing). On each house the faction's progress is raised to the house goal (never lowered) and the house is set complete with the faction as winner
// and the time now. A house already complete for the faction with its progress at the goal is skipped. Inside the edit the term is compared and the houses that would change are
// counted again; a term or a count that is not the one shown is refused with a conflict and nothing is written. Either every house is done or none is. No personal contribution
// and no reward row is written.
func (o *Ops) CompleteChosen(a Args) (any, error) {
	req, err := parseChosen(a)
	if err != nil {
		return nil, err
	}
	faction, err := a.Int("faction_id")
	if err != nil {
		return nil, err
	}
	var changed, repaired int64
	if _, err := o.S.Mutate("landsraad", func(m *save.Mut) error {
		changed, repaired = 0, 0
		f, err := m.One(`select name from factions where id=? and name<>'None'`, faction)
		if err != nil {
			return err
		}
		if f == nil {
			return apierr.New(apierr.ErrBadArgument, "that faction is not one the Landsraad has")
		}
		if err := currentTerm(m, req.term); err != nil {
			return err
		}
		tasks := make([]save.Row, 0, len(req.ids))
		for _, id := range req.ids {
			t, err := chosenTask(m, id, req.term)
			if err != nil {
				return err
			}
			tasks = append(tasks, t)
			c, err := completeHouse(m, t, faction, false)
			if err != nil {
				return err
			}
			if c {
				changed++
			}
		}
		if changed != req.expected {
			return apierr.Newf(apierr.ErrConflict, "the number of houses to complete is now %d, not the %d you were shown, so nothing was changed: reload the page and look again", changed, req.expected)
		}
		for _, t := range tasks {
			if _, err := completeHouse(m, t, faction, true); err != nil {
				return err
			}
		}
		if changed > 0 {
			m.Desc = fmt.Sprintf("landsraad: %d houses completed for %v", changed, f["name"])
		}
		var rerr error
		repaired, rerr = repairCompletionTimes(m, req.term)
		return rerr
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, map[string]any{"houses": int64(len(req.ids)), "skipped": int64(len(req.ids)) - changed, "repaired": repaired}), nil
}

// ReopenChosen reopens several houses of the current term in one edit (args task_ids, term_id, expected as in CompleteChosen): completed is cleared, and so are the winning faction
// and the completion time. It does not touch any faction's progress or any personal contribution. A house that is not complete is skipped.
func (o *Ops) ReopenChosen(a Args) (any, error) {
	req, err := parseChosen(a)
	if err != nil {
		return nil, err
	}
	var changed, repaired int64
	if _, err := o.S.Mutate("landsraad", func(m *save.Mut) error {
		changed, repaired = 0, 0
		if err := currentTerm(m, req.term); err != nil {
			return err
		}
		tasks := make([]save.Row, 0, len(req.ids))
		for _, id := range req.ids {
			t, err := chosenTask(m, id, req.term)
			if err != nil {
				return err
			}
			tasks = append(tasks, t)
			c, err := reopenHouse(m, t, false)
			if err != nil {
				return err
			}
			if c {
				changed++
			}
		}
		if changed != req.expected {
			return apierr.Newf(apierr.ErrConflict, "the number of houses to reopen is now %d, not the %d you were shown, so nothing was changed: reload the page and look again", changed, req.expected)
		}
		for _, t := range tasks {
			if _, err := reopenHouse(m, t, true); err != nil {
				return err
			}
		}
		if changed > 0 {
			m.Desc = fmt.Sprintf("landsraad: %d houses reopened", changed)
		}
		var rerr error
		repaired, rerr = repairCompletionTimes(m, req.term)
		return rerr
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, map[string]any{"houses": int64(len(req.ids)), "skipped": int64(len(req.ids)) - changed, "repaired": repaired}), nil
}
