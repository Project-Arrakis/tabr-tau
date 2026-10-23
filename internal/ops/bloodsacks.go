package ops

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Filling blood sacks (issue #60, the second half of the Player tab's refills). A blood sack keeps its fill level exactly where a Literjon keeps its water, in the
// item's own stats: {"FFillableItemStats":[[],{"CurrentAmount":30000.0,"FillableType":"Blood"}]}. An empty sack has the key with an empty object
// ([[],{}]) or no key at all. The capacities are in bloodCapacity (fillables.go); a sack of any other template is left alone and reported.

// bloodSackQuery lists the blood sacks of the character's own inventories (backpack, worn gear, loadout: the same scope as the water refill), whatever they hold.
const bloodSackQuery = `select i.id, i.template_id,
		cast(json_extract(i.stats,'$.FFillableItemStats[1].CurrentAmount') as real) amount,
		json_extract(i.stats,'$.FFillableItemStats[1].FillableType') kind
		from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and json_valid(i.stats) and i.template_id like 'Bloodsack%' order by i.id`

// bloodSackState is what a look at the character's blood sacks finds.
type bloodSackState struct {
	todo      []bloodTarget // sacks of a known kind that are not full
	byKind    map[string]int
	full      int            // sacks of a known kind that are already at or above capacity
	other     int            // sacks of a known kind that hold something other than blood: left alone
	unknown   map[string]int // sacks of a template whose capacity is not known: left alone
	unknownN  int
	totalKnow int
}

type bloodTarget struct {
	id  any
	tpl string
	cap float64
}

// lookAtBloodSacks sorts the rows into what to fill and what to leave alone. query is the edit's own query when it runs inside an edit.
func lookAtBloodSacks(query func(string, ...any) ([]save.Row, error), pawn int64) (bloodSackState, error) {
	st := bloodSackState{byKind: map[string]int{}, unknown: map[string]int{}}
	rows, err := query(bloodSackQuery, pawn)
	if err != nil {
		return st, err
	}
	for _, r := range rows {
		tpl := fmt.Sprint(r["template_id"])
		if !strings.HasPrefix(tpl, "Bloodsack") { // 'like' is case-insensitive
			continue
		}
		capacity, known := bloodCapacity[tpl]
		if !known {
			st.unknown[tpl]++
			st.unknownN++
			continue
		}
		st.totalKnow++
		if r["kind"] != nil && r["kind"] != "Blood" {
			st.other++
			continue
		}
		if cur, ok := r["amount"].(float64); ok && cur >= capacity {
			st.full++
			continue
		}
		st.todo = append(st.todo, bloodTarget{r["id"], tpl, capacity})
		st.byKind[tpl]++
	}
	return st, nil
}

func (st bloodSackState) skippedUnknown() []string {
	out := make([]string, 0, len(st.unknown))
	for k, n := range st.unknown {
		out = append(out, fmt.Sprintf("%s x%d", k, n))
	}
	sort.Strings(out)
	return out
}

// BloodSacks says how many blood sacks the character carries and how many of them are not full, for the count beside the Fill blood sacks button.
func (o *Ops) BloodSacks() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	st, err := lookAtBloodSacks(o.S.Query, p.Pawn)
	if err != nil {
		return nil, err
	}
	return map[string]any{"total": st.totalKnow, "notFull": len(st.todo), "unknown": st.unknownN, "holdingOther": st.other}, nil
}

// FillBloodSacks fills every blood sack of a known kind that the character carries to its capacity, in one edit. A sack already at or above its capacity is not
// rewritten (a value is never lowered), a sack holding something other than blood is left alone, and so is a sack whose template is not in bloodCapacity.
// Nothing else in the sack's stats changes: json_set writes only the two keys.
func (o *Ops) FillBloodSacks() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var st bloodSackState
	_, err = o.S.Mutate("fill blood sacks", func(m *save.Mut) error {
		var ferr error
		if st, ferr = fillBloodSacksTx(m, p.Pawn); ferr != nil {
			return ferr
		}
		if n := len(st.todo); n == 1 {
			m.Desc = "Filled 1 blood sack"
		} else {
			m.Desc = fmt.Sprintf("Filled %d blood sacks", n)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "filled": len(st.todo), "byTemplate": st.byKind, "alreadyFull": st.full, "holdingOther": st.other,
		"unknown": st.unknownN, "skippedUnknown": st.skippedUnknown()}, nil
}

// fillBloodSacksTx is FillBloodSacks inside the edit m, for the character whose pawn actor is pawn, so it can share one edit with other jobs (the automatic
// pass). The sacks are looked at through the edit's own query, as the water refill does: what the edit read is kept with it, and an edit put back onto a newer file
// is not applied to a list made from the older one. It sets no description.
func fillBloodSacksTx(m *save.Mut, pawn int64) (bloodSackState, error) {
	st, err := lookAtBloodSacks(m.Query, pawn)
	if err != nil {
		return st, err
	}
	for _, t := range st.todo {
		// The record is changed in place when it has the [header, values] pair; a missing or odd record is replaced by a whole new pair.
		if _, err := m.Exec(`update items set stats=case
			when json_type(stats,'$.FFillableItemStats[1]')='object'
			then json_set(stats,'$.FFillableItemStats[1].CurrentAmount',?,'$.FFillableItemStats[1].FillableType','Blood')
			else json_set(stats,'$.FFillableItemStats',json_array(json_array(),json_object('CurrentAmount',?,'FillableType','Blood'))) end
			where id=?`, t.cap, t.cap, t.id); err != nil {
			return st, err
		}
	}
	return st, nil
}
