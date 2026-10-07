package ops

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Landsraad returns the current term, the task board with faction progress and
// rewards, and the decree list.
func (o *Ops) Landsraad() (any, error) {
	term, _ := o.S.One(`select * from landsraad_decree_term order by term_id desc limit 1`)
	tasks, err := o.S.Query(`select t.id, t.board_index, t.house_name, t.goal_amount, t.completed, t.winning_faction_id, t.sysselraad, t.completion_time
		from landsraad_tasks t order by t.board_index`)
	if err != nil {
		return nil, err
	}
	contrib, _ := o.S.Query(`select task_id, faction_id, amount from landsraad_task_faction_contributions`)
	byTask := map[int64][]map[string]any{}
	for _, c := range contrib {
		t := c["task_id"].(int64)
		byTask[t] = append(byTask[t], map[string]any{"faction_id": c["faction_id"], "amount": c["amount"]})
	}
	rewards, _ := o.S.Query(`select task_id, threshold, template_id, amount from landsraad_task_rewards order by task_id, threshold`)
	rw := map[int64][]any{}
	for _, r := range rewards {
		t := r["task_id"].(int64)
		rw[t] = append(rw[t], r)
	}
	for _, t := range tasks {
		id := t["id"].(int64)
		t["contributions"] = byTask[id]
		t["rewards"] = rw[id]
	}
	decrees, _ := o.S.Query(`select id, decree_name, disabled, weight from landsraad_decrees order by id`)
	rotation, _ := o.S.Query(`select decree_id from landsraad_decree_rotation`)
	factions, _ := o.S.Query(`select id, name from factions where name <> 'None' order by id`)
	return map[string]any{"term": term, "tasks": tasks, "decrees": decrees, "rotation": rotation, "factions": factions}, nil
}

// SetTaskProgress sets a faction's contribution to a task.
func (o *Ops) SetTaskProgress(a Args) (any, error) {
	t, e1 := a.Int("task_id")
	f, e2 := a.Int("faction_id")
	amt, e3 := a.Float("amount")
	if err := errors.Join(e1, e2, e3); err != nil {
		return nil, err
	}
	if amt < 0 {
		return nil, errors.New("amount must be >= 0")
	}
	if _, err := o.run(fmt.Sprintf("landsraad task %d faction %d progress = %g", t, f, amt), `insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values(?,?,?)
		on conflict(faction_id, task_id) do update set amount=excluded.amount`, f, t, amt); err != nil {
		return nil, err
	}
	return ok(), nil
}

// CompleteTask marks a task complete for a faction; reset=true reopens it.
func (o *Ops) CompleteTask(a Args) (any, error) {
	t, err := a.Int("task_id")
	if err != nil {
		return nil, err
	}
	var n int64
	if a.Bool("reset", false) {
		n, err = o.run(fmt.Sprintf("landsraad task %d reopened", t), `update landsraad_tasks set completed=0, winning_faction_id=null, completion_time=null where id=?`, t)
	} else {
		f, ferr := a.Int("faction_id")
		if ferr != nil {
			return nil, ferr
		}
		n, err = o.run(fmt.Sprintf("landsraad task %d completed for faction %d", t, f), `update landsraad_tasks set completed=1, winning_faction_id=?, completion_time=? where id=?`, f, time.Now().Unix(), t)
	}
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errors.New("task not found")
	}
	return ok(), nil
}

// SetDecree enables/disables a decree in the pool.
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
		return nil, errors.New("decree not found")
	}
	return ok(), nil
}

// SetTerm updates the active/elected decree, reigning faction and term end.
func (o *Ops) SetTerm(a Args) (any, error) {
	term, err := o.S.One(`select term_id from landsraad_decree_term order by term_id desc limit 1`)
	if err != nil || term == nil {
		return nil, errors.New("no Landsraad term in this save")
	}
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
	_, err = o.S.Mutate(fmt.Sprintf("landsraad term %v: %s", term["term_id"], strings.Join(parts, ", ")), func(m *save.Mut) error {
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
