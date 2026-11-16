package ops

import (
	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- what the board writes for a complete and a reopen (#451)
//
// The helpers below are the code of the board's write (landsraad_apply.go): the house read inside the edit, the term compared with the one the page showed, a completion and a
// reopening of one house.

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
		return nil, apierr.New(apierr.ErrConflict, "a chosen house belongs to an earlier term, so nothing was changed. The board was read again; check it and click again")
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
		return apierr.New(apierr.ErrConflict, "the Landsraad term is not the one you were shown, so nothing was changed. The board was read again; check it and click again")
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
