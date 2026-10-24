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
//   - the volume (#357): the volume of what is in the inventory (each row's unit volume times its stack, the unit volume being the game's item table value, catalog.Volume, or the
//     row's own volume_override, which the game too reads per unit) plus the new units may not pass the inventory's max_item_volume. The check is a calculation only: the rows a
//     give writes keep volume_override NULL, the game's own convention (the game multiplies a non-NULL value by the stack size, so a total written there would be counted twice).
//     It is not made, and the answer says so, when the item has no known volume or the inventory has no volume limit recorded (NULL or not above 0); items already in the
//     inventory whose volume is unknown count as none and are counted in the answer's note;
//   - the bound: one give makes at most maxGiveStacks new stacks.
//
// Only a give that cannot place a single unit is a refusal. The numbers are worked out inside the edit, from rows the edit reads, so a queued copy of the edit is put back only
// onto a file where those rows are what they were (the held-queue rules, as for every other edit).

// What stopped a give that placed less than was asked.
const (
	limitNone   = ""
	limitSlots  = "slots"  // the inventory has no free slot for another stack
	limitBound  = "bound"  // one give makes at most maxGiveStacks stacks
	limitVolume = "volume" // the inventory's volume limit leaves room for fewer units
)

// giveOutcome is what one give did.
type giveOutcome struct {
	IDs       []int64 // the rows it added (none when everything went onto stacks that were there)
	Requested int64
	Given     int64
	Limit     string // limitNone when everything was given, else what stopped it
	FreeSlots int64  // the free slots the inventory had when the give began; -1 when it has no limit
	Stack     int64  // the item's stack size (0 when the catalog does not know the item)
	// VolumeChecked says whether the volume limit was part of the give; when it was not, or when it was with a caveat, VolumeNote says why in words ("" otherwise).
	VolumeChecked bool
	VolumeNote    string
}

// reason says in words what stopped the give ("" when nothing did). what names the inventory: "the backpack", "the container".
func (g giveOutcome) reason(what string) string {
	switch g.Limit {
	case limitSlots:
		return what + " is full"
	case limitBound:
		return fmt.Sprintf("at most %s stacks are made in one give", commas(maxGiveStacks))
	case limitVolume:
		return what + " is full (volume)"
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

// volumeLeftInTx works out how many units of template the volume limit of the inventory still leaves room for. fit is -1 when the volume limit is not part of this give
// (checked false, and note says why), and otherwise the whole number of units that fit (0 when it is full). note is also set when the check was made with a caveat.
func volumeLeftInTx(m *save.Mut, inventoryID int64, template string) (fit int64, checked bool, note string, err error) {
	unit, known := catalog.Volume(template)
	if !known {
		return -1, false, "Volume was not checked: the editor has no volume for this item.", nil
	}
	rows, err := m.Query(`select max_item_volume v from inventories where id=?`, inventoryID)
	if err != nil {
		return -1, false, "", err
	}
	var limit float64
	if len(rows) > 0 {
		limit, _ = rows[0]["v"].(float64)
	}
	if limit <= 0 {
		return -1, false, "Volume was not checked: this inventory has no volume limit recorded.", nil
	}
	if unit <= 0 {
		return -1, true, "", nil // the item takes no volume
	}
	// a save whose items table has no volume_override column (the game's has it; an older or hand-made file may not) counts every row at the item table's volume
	override := "null"
	if c, e := m.Query(`select count(*) n from pragma_table_info('items') where name='volume_override'`); e == nil && len(c) > 0 && c[0]["n"] == int64(1) {
		override = "volume_override"
	}
	have, err := m.Query(`select template_id t, `+override+` o, sum(stack_size) n from items where inventory_id=? group by template_id, `+override, inventoryID)
	if err != nil {
		return -1, false, "", err
	}
	var used float64
	var unknown int64
	for _, r := range have {
		n, _ := r["n"].(int64)
		if o, ok := r["o"].(float64); ok { // per unit, as the game reads it
			used += o * float64(n)
		} else if u, ok := catalog.Volume(fmt.Sprint(r["t"])); ok {
			used += u * float64(n)
		} else {
			unknown += n
		}
	}
	if unknown > 0 {
		note = fmt.Sprintf("%s units already there have no known volume and were counted as none.", commas(unknown))
	}
	if used >= limit {
		return 0, true, note, nil
	}
	return int64((limit-used)/unit + 1e-9), true, note, nil
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
		size = qty // one row with everything (cut to what fits below)
	}
	free, err := freeSlotsInTx(m, inventoryID)
	if err != nil {
		return out, err
	}
	out.FreeSlots = free
	// the volume cuts what is asked down to what fits; the rest of the give works with that amount
	target := qty
	fit, checked, note, err := volumeLeftInTx(m, inventoryID, template)
	if err != nil {
		return out, err
	}
	out.VolumeChecked, out.VolumeNote = checked, note
	volumeCut := false
	if checked && fit >= 0 && fit < qty {
		target, volumeCut = fit, true
		if !known {
			size = target
		}
	}
	left := target
	var topUps []struct{ id, to int64 }
	if known && size > 1 {
		rows, err := m.Query(`select id, stack_size from items where inventory_id=? and template_id=? and quality_level=? and stack_size<? order by position_index, id`, inventoryID, template, quality, size)
		if err != nil {
			return out, err
		}
		for _, r := range rows {
			have, _ := r["stack_size"].(int64)
			add := min(size-have, left)
			id, ok := r["id"].(int64)
			if add <= 0 || !ok {
				continue
			}
			topUps = append(topUps, struct{ id, to int64 }{id, have + add})
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
	if volumeCut && out.Limit == limitNone {
		out.Limit = limitVolume
	}
	var placed int64
	for _, s := range sizes {
		placed += s
	}
	out.Given = target - left + placed
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
	if out.Limit == limitVolume {
		return nil, fmt.Errorf("%w: %s needs room for %s units by volume and the inventory has room for %s", errNotEnoughRoom, itemLabel(template), commas(qty), commas(out.Given))
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
	return map[string]any{"ok": true, "itemId": first, "stacks": len(g.IDs), "requested": g.Requested, "given": g.Given, "reason": g.reason(what), "limit": g.Limit,
		"volumeChecked": g.VolumeChecked, "volumeNote": g.VolumeNote}
}
