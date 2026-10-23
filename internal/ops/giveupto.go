package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A give puts as much of what was asked into the inventory as fits, and says what it could not do and why. What limits a give:
//
//   - the slots: an inventory holds the number of stacks its max_item_count says (0 means no limit). A stack that is already there and has room is filled first and needs no slot;
//     every other stack needs a free slot;
//   - the stack size: no slot holds more than the item's stack size (the game's item tables, catalog.MaxStack). An item that does not stack takes one slot per copy;
//   - the bound: one give makes at most maxGiveStacks new stacks.
//
// The editor does not track an inventory's volume: the item catalog it embeds has no volume for any item, so there is nothing to add up against max_item_volume. A give is not
// limited by volume, as it never was.
//
// Only a give that cannot place a single unit is a refusal. The numbers are worked out inside the edit, from rows the edit reads, so a queued copy of the edit is put back only
// onto a file where those rows are what they were (the held-queue rules, as for every other edit).

// What stopped a give that placed less than was asked.
const (
	limitNone  = ""
	limitSlots = "slots" // the inventory has no free slot for another stack
	limitBound = "bound" // one give makes at most maxGiveStacks stacks
)

// giveOutcome is what one give did.
type giveOutcome struct {
	IDs       []int64 // the rows it added (none when everything went onto stacks that were there)
	Requested int64
	Given     int64
	Limit     string // limitNone when everything was given, else what stopped it
	FreeSlots int64  // the free slots the inventory had when the give began; -1 when it has no limit
	Stack     int64  // the item's stack size (0 when the catalog does not know the item)
}

// reason says in words what stopped the give ("" when nothing did). what names the inventory: "the backpack", "the container".
func (g giveOutcome) reason(what string) string {
	switch g.Limit {
	case limitSlots:
		return what + " is full"
	case limitBound:
		return fmt.Sprintf("at most %s stacks are made in one give", commas(maxGiveStacks))
	}
	return ""
}

// freeSlotsInTx is the number of stacks the inventory can still take: -1 when it has no limit (max_item_count 0 or unset). The slots in use are the distinct positions of its items.
func freeSlotsInTx(m *save.Mut, inventoryID int64) (int64, error) {
	rows, err := m.Query(`select coalesce(max_item_count,0) cap from inventories where id=?`, inventoryID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, apierr.New(apierr.ErrNotFound, "inventory not found")
	}
	capacity, _ := rows[0]["cap"].(int64)
	if capacity <= 0 {
		return -1, nil
	}
	used, err := m.Query(`select count(distinct position_index) n from items where inventory_id=?`, inventoryID)
	if err != nil {
		return 0, err
	}
	n, _ := used[0]["n"].(int64)
	return max(capacity-n, 0), nil
}

// giveUpToInTx gives up to qty of an item the way the game holds it: stacks of the same item and grade that are in the inventory and not full are filled first, then new stacks are
// added, each in its own slot, for as long as there are free slots and the bound allows. It gives as much as fits; outcome.Given says how much that was. It returns an error only
// for a fault, never for a lack of room: a caller that cannot accept less than it asked for turns a short outcome into a refusal (giveStacksInTx).
// An item the catalog does not know has no stack size to honor and goes in as one row.
func giveUpToInTx(m *save.Mut, inventoryID int64, template string, qty, quality int64) (giveOutcome, error) {
	out := giveOutcome{Requested: qty}
	size, known := catalog.MaxStack(template)
	if known {
		out.Stack = size
	} else {
		size = qty // one row with everything
	}
	free, err := freeSlotsInTx(m, inventoryID)
	if err != nil {
		return out, err
	}
	out.FreeSlots = free
	left := qty
	var topUps []struct{ id, to int64 }
	if known && size > 1 {
		rows, err := m.Query(`select id, stack_size from items where inventory_id=? and template_id=? and quality_level=? and stack_size<? order by position_index, id`, inventoryID, template, quality, size)
		if err != nil {
			return out, err
		}
		for _, r := range rows {
			have, _ := r["stack_size"].(int64)
			add := min(size-have, left)
			if add <= 0 {
				continue
			}
			topUps = append(topUps, struct{ id, to int64 }{r["id"].(int64), have + add})
			left -= add
		}
	}
	// the new stacks: as many as the slots and the bound allow
	var sizes []int64
	if left > 0 {
		want := (left + size - 1) / size
		allowed := want
		if free >= 0 && free < allowed {
			allowed, out.Limit = free, limitSlots
		}
		if allowed > maxGiveStacks {
			allowed, out.Limit = maxGiveStacks, limitBound
		}
		if allowed > 0 {
			sizes = make([]int64, allowed)
			for i := range sizes {
				sizes[i] = size
			}
			if allowed == want {
				sizes[allowed-1] = left - (want-1)*size
			}
		}
	}
	var placed int64
	for _, s := range sizes {
		placed += s
	}
	out.Given = qty - left + placed
	if out.Given == 0 {
		return out, nil // nothing fits: nothing is written
	}
	for _, u := range topUps {
		if _, err := m.Exec(`update items set stack_size=? where id=?`, u.to, u.id); err != nil {
			return out, err
		}
	}
	if len(sizes) > 0 {
		if out.IDs, err = giveRowsInTx(m, inventoryID, template, sizes, quality); err != nil {
			return out, err
		}
	}
	return out, nil
}

// giveStacksInTx is the give for a caller that cannot take less than it asked for (Solari): the whole of qty or nothing. A give that
// would place less is refused, and the caller's transaction undoes whatever was done before.
func giveStacksInTx(m *save.Mut, inventoryID int64, template string, qty, quality int64) ([]int64, error) {
	out, err := giveUpToInTx(m, inventoryID, template, qty, quality)
	if err != nil {
		return nil, err
	}
	if out.Given == out.Requested {
		return out.IDs, nil
	}
	if out.Limit == limitBound {
		stacks := (qty + max(out.Stack, 1) - 1) / max(out.Stack, 1)
		return nil, apierr.Newf(apierr.ErrBadArgument, "%s makes %s separate stacks (a slot holds %s); at most %s at a time: give fewer, or give them in several steps", itemLabel(template), commas(stacks), commas(max(out.Stack, 1)), commas(maxGiveStacks))
	}
	if out.FreeSlots == 0 {
		return nil, errInventoryFull
	}
	return nil, fmt.Errorf("%w: %s needs %d stacks of up to %s and the inventory has no more free slots", errNotEnoughRoom, itemLabel(template), (qty+max(out.Stack, 1)-1)/max(out.Stack, 1), commas(out.Stack))
}

// giveWords is how a give is written in the write log and in the answer: "Give 500 x Iron Bar (grade 0)" when everything went in, "Give 340 of 500 x Iron Bar (grade 0)" when
// less did. suffix follows the item ("(grade 0)", "to container 12").
func giveWords(g giveOutcome, template, suffix string) string {
	if g.Given < g.Requested {
		return fmt.Sprintf("Give %s of %s x %s %s", commas(g.Given), commas(g.Requested), itemLabel(template), suffix)
	}
	return fmt.Sprintf("Give %s x %s %s", commas(g.Given), itemLabel(template), suffix)
}

// refuseNothingFits is the refusal of a give that placed nothing (409, no_room).
func refuseNothingFits(g giveOutcome, what string) error {
	return apierr.Newf(apierr.ErrNoRoom, "Nothing was given: %s.", g.reason(what))
}

// giveAnswer is what a give answers: the first row added (0 when everything went onto stacks that were there), how many stacks were added, and what was asked and given.
// reason is empty when everything was given.
func giveAnswer(g giveOutcome, what string) map[string]any {
	var first int64
	if len(g.IDs) > 0 {
		first = g.IDs[0]
	}
	return map[string]any{"ok": true, "itemId": first, "stacks": len(g.IDs), "requested": g.Requested, "given": g.Given, "reason": g.reason(what), "limit": g.Limit}
}
