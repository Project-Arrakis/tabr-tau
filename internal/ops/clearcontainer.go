package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Clearing a container removes every item row in one inventory: the backpack, the worn gear or the loadout of the character, or one container of one of
// the player's bases. Each item goes through the same removal as the single-item delete (deleteItemRowTx, with the schema's cascades on), so what hangs from an
// item (the inventory of a bag, a blueprint of it) goes with it, and the whole clear is one edit: one backup, one line of the write log, one entry in the queue.
// The Solari Coin stacks stay unless the request says to remove currency too.

// clearableCarriedTypes are the carried inventories that can be cleared. The unique schematics (type 30) are left out: they are the character's unlocks, not
// stored goods.
const clearableCarriedTypes = `(0, 1, 15)`

// ClearContainerRequest is what the page sends: the container, the number of items the person confirmed would be removed, and whether currency goes too.
type ClearContainerRequest struct {
	InventoryID     int64 // the container's inventory id
	Count           int64 // the item rows the person was told would be removed; the clear is refused when the container holds another number now
	IncludeCurrency bool  // the Solari Coin stacks are removed as well
}

// ParseClearContainer reads the request out of a decoded body. Every field is checked: a count that is missing (Args.Int refuses an absent number), negative or not a whole number, or a currency flag
// that is not true or false, is refused instead of being taken as something else.
func ParseClearContainer(a Args) (ClearContainerRequest, error) {
	var r ClearContainerRequest
	var err error
	if r.InventoryID, err = a.Int("inventory_id"); err != nil {
		return r, err
	}
	if r.InventoryID <= 0 {
		return r, apierr.New(apierr.ErrBadArgument, "inventory_id must be a container's id")
	}
	if r.Count, err = a.IntRange("count", 0, maxExactInt); err != nil {
		return r, err
	}
	if v, has := a["include_currency"]; has {
		b, isBool := v.(bool)
		if !isBool {
			return r, apierr.New(apierr.ErrBadArgument, "include_currency must be true or false")
		}
		r.IncludeCurrency = b
	}
	return r, nil
}

// requireClearable refuses an inventory that does not exist or that is not a container the page may offer to clear: a carried backpack, worn gear or loadout of
// the character, or a container of one of the player's bases.
func (o *Ops) requireClearable(inv int64) error {
	row, err := o.S.One(`select 1 x from inventories where id=?`, inv)
	if err != nil {
		return err
	}
	if row == nil {
		return apierr.New(apierr.ErrNotFound, "container not found")
	}
	p, err := o.player()
	if err != nil {
		return err
	}
	if r, _ := o.S.One(`select 1 x from inventories where id=? and actor_id=? and inventory_type in `+clearableCarriedTypes, inv, p.Pawn); r != nil {
		return nil
	}
	if ok, err := o.baseContainerInventory(inv); err != nil {
		return err
	} else if ok {
		return nil
	}
	return apierr.New(apierr.ErrBadArgument, "that is not your backpack, worn gear or loadout, or a container at one of your bases")
}

// ClearContainer removes the items of one container as one edit. The container's item list is read inside the edit, so a queued copy of the edit is put back
// only onto a file where the container still holds the same items; the count the person confirmed is checked against that list, and a container that holds
// another number now is refused with nothing changed. A container with nothing to remove is not an edit: the answer says so.
func (o *Ops) ClearContainer(a Args) (any, error) {
	req, err := ParseClearContainer(a)
	if err != nil {
		return nil, err
	}
	if err := o.requireClearable(req.InventoryID); err != nil {
		return nil, err
	}
	var removed, kept int64
	_, err = o.S.MutateCascade(fmt.Sprintf("clear container %d", req.InventoryID), func(m *save.Mut) error {
		removed, kept = 0, 0
		rows, err := m.Query(`select id, template_id from items where inventory_id=? order by position_index, id`, req.InventoryID)
		if err != nil {
			return err
		}
		var ids []int64
		for _, r := range rows {
			if !req.IncludeCurrency && fmt.Sprint(r["template_id"]) == solariTemplate {
				kept++
				continue
			}
			id, _ := r["id"].(int64)
			ids = append(ids, id)
		}
		if int64(len(ids)) != req.Count {
			return apierr.Newf(apierr.ErrConflict, "this container holds %s to clear now, not the %s that were confirmed, so nothing was changed: open it again and check the count", nItems(int64(len(ids))), nItems(req.Count))
		}
		for _, id := range ids {
			if err := deleteItemRowTx(m, id); err != nil {
				return err
			}
		}
		removed = int64(len(ids))
		if removed > 0 {
			m.Desc = fmt.Sprintf("Cleared container %d (%s)", req.InventoryID, nItems(removed))
			if req.IncludeCurrency {
				m.Desc = fmt.Sprintf("Cleared container %d (%s, currency included)", req.InventoryID, nItems(removed))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	res := map[string]any{"ok": true, "removed": removed, "kept": kept}
	if removed == 0 {
		res["message"] = "Nothing to clear: this container has no items to remove."
		if kept > 0 {
			res["message"] = "Nothing to clear: this container holds only currency, which is kept unless currency is included."
		}
	}
	return res, nil
}

// nItems is "1 item" or "N items".
func nItems(n int64) string {
	if n == 1 {
		return "1 item"
	}
	return commas(n) + " items"
}
