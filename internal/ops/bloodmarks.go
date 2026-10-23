package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Blood Marks are the currency of the Halloween 2026 event (the Scourge event in game; the item card reads Blood Marks). The game keeps them as an ordinary item stack in the backpack, template HalloweenCoin, the way it
// keeps Solari. The item catalog the editor embeds has no name for it, so the page calls it Blood Marks and shows the item id beside it.
//
// Giving them is the general give (giveUpToInTx): it fills the HalloweenCoin stack that is in the backpack first, up to the stack size the game's item tables give
// (catalog.MaxStack, 50,000), then starts a new stack in a free slot, and gives as much as fits. There is no second way of inserting an item here.
const (
	bloodMarksTemplate = "HalloweenCoin"
	bloodMarksName     = "Blood Marks"
)

// bloodMarksStack is the most Blood Marks one stack holds and so the most one give may add: the game's own stack size for the item (50,000).
func bloodMarksStack() int64 {
	if n, _ := catalog.MaxStack(bloodMarksTemplate); n > 0 {
		return n
	}
	return 1
}

// bloodMarksIn is the Blood Marks the backpack holds: the sum of every HalloweenCoin stack in it, 0 when there is none. query is the read function of the caller (the save,
// or the edit that is running, so that an edit reads the stacks inside itself).
func bloodMarksIn(query func(q string, args ...any) ([]map[string]any, error), pawn int64) (int64, error) {
	rows, err := query(`select coalesce(sum(i.stack_size),0) n from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and v.inventory_type=0 and i.template_id=?`, pawn, bloodMarksTemplate)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	n, _ := rows[0]["n"].(int64)
	return n, nil
}

// bloodMarks is the figure the Player tab shows.
func (o *Ops) bloodMarks(p player) int64 {
	n, _ := bloodMarksIn(o.S.Query, p.Pawn)
	return n
}

// GiveBloodMarks adds Blood Marks to the character's backpack as one edit. The amount is a whole number from 1 to the size of one stack (50,000). Like every give it puts in as
// much as fits (the stack that is there first, then new stacks in free slots) and says how much that was; only when not one fits is it refused (409, no_room) with nothing changed.
// The stacks and the free slots are read inside the edit, so a queued copy is put back only onto a file where they are what they were.
func (o *Ops) GiveBloodMarks(a Args) (any, error) {
	amount, err := a.IntRange("amount", 1, bloodMarksStack())
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var before int64
	var out giveOutcome
	_, err = o.S.Mutate(fmt.Sprintf("gave %s %s", commas(amount), bloodMarksName), func(m *save.Mut) error {
		before = 0
		bp, err := m.Query(`select id from inventories where actor_id=? and inventory_type=0 order by id limit 1`, p.Pawn)
		if err != nil {
			return err
		}
		if len(bp) == 0 {
			return apierr.New(apierr.ErrNotFound, "player backpack not found")
		}
		inv, _ := bp[0]["id"].(int64)
		if before, err = bloodMarksIn(m.Query, p.Pawn); err != nil {
			return err
		}
		if out, err = giveUpToInTx(m, inv, bloodMarksTemplate, amount, 0); err != nil {
			return err
		}
		if out.Given == 0 {
			return apierr.Newf(apierr.ErrNoRoom, "No %s were given: %s.", bloodMarksName, out.reason("the backpack"))
		}
		if out.Given < amount {
			m.Desc = fmt.Sprintf("gave %s of %s %s", commas(out.Given), commas(amount), bloodMarksName)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "requested": amount, "given": out.Given, "reason": out.reason("the backpack"), "limit": out.Limit, "before": before, "after": before + out.Given, "stacks": len(out.IDs)}, nil
}
