package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

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

// ClearContainerRequest is what the page sends: the container, what the person confirmed would be removed, and whether currency goes too. What was confirmed is the number of
// items in the container, the number of items that hang from them (inside bags, and inside those), and a hash of the ids of both sets (ClearPreview makes it): a container that
// changed to other items of the same number is refused as much as one that changed in number.
type ClearContainerRequest struct {
	InventoryID     int64  // the container's inventory id
	Count           int64  // the item rows the person was told would be removed; the clear is refused when the container holds another number now
	Nested          int64  // the item rows inside the bags among them, which go with them
	IDsHash         string // the hash of the ids of both sets, as the preview gave it
	IncludeCurrency bool   // the Solari Coin stacks are removed as well
}

var idsHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

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
	if r.Nested, err = a.IntRange("nested", 0, maxExactInt); err != nil {
		return r, err
	}
	h, ok := a["ids_hash"].(string)
	if !ok || !idsHashPattern.MatchString(h) {
		return r, apierr.New(apierr.ErrBadArgument, "ids_hash is required: it is the hash of the items the clear was shown for (the page reads it from the preview)")
	}
	r.IDsHash = h
	if r.IncludeCurrency, err = strictBool(a, "include_currency", false); err != nil {
		return r, err
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

// clearSet is what a clear of one container would remove: the item rows in it (top), the Solari stacks it would keep, and the item rows that hang from the top ones through the
// inventories of bags (nested, any depth). The two lists are in id order.
type clearSet struct {
	inv, kept   int64
	currency    bool
	top, nested []int64
}

// readClearSet reads the set with the given reader (an edit's, so it is part of the edit's read-set, or the save's own for the preview).
func readClearSet(query func(string, ...any) ([]save.Row, error), inv int64, includeCurrency bool) (clearSet, error) {
	c := clearSet{inv: inv, currency: includeCurrency}
	rows, err := query(`select id, template_id from items where inventory_id=? order by position_index, id`, inv)
	if err != nil {
		return c, err
	}
	for _, r := range rows {
		if !includeCurrency && fmt.Sprint(r["template_id"]) == solariTemplate {
			c.kept++
			continue
		}
		id, _ := r["id"].(int64)
		c.top = append(c.top, id)
	}
	sort.Slice(c.top, func(i, j int) bool { return c.top[i] < c.top[j] })
	if len(c.top) == 0 {
		return c, nil
	}
	// the inventories bags own, and the bags in those; the Solari stacks that stay are not the start of anything
	nrows, err := query(`with recursive top(id) as (select id from items where inventory_id=? and (? or template_id<>?)),
			inv(id) as (select i.id from inventories i where i.item_id in (select id from top)
				union select i.id from inventories i join items it on i.item_id=it.id join inv on it.inventory_id=inv.id)
		select id from items where inventory_id in (select id from inv) order by id`, inv, includeCurrency, solariTemplate)
	if err != nil {
		return c, err
	}
	for _, r := range nrows {
		id, _ := r["id"].(int64)
		c.nested = append(c.nested, id)
	}
	return c, nil
}

// token is the hash the page sends back: of the container, the currency flag and both lists of ids.
func (c clearSet) token() string {
	var b strings.Builder
	fmt.Fprintf(&b, "inventory %d currency %t\ntop", c.inv, c.currency)
	for _, id := range c.top {
		b.WriteString(" " + strconv.FormatInt(id, 10))
	}
	b.WriteString("\nnested")
	for _, id := range c.nested {
		b.WriteString(" " + strconv.FormatInt(id, 10))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// ClearPreview says what a clear of the container would remove right now: the items in it, the items inside the bags among them, the currency stacks that would stay, and the
// token to send with the clear. It reads and changes nothing; the clear checks the same things again inside its own edit.
func (o *Ops) ClearPreview(inv int64, includeCurrency bool) (any, error) {
	if inv <= 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "inventory_id must be a container id")
	}
	if err := o.requireClearable(inv); err != nil {
		return nil, err
	}
	c, err := readClearSet(o.S.Query, inv, includeCurrency)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": int64(len(c.top)), "kept": c.kept, "nested": int64(len(c.nested)), "token": c.token()}, nil
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
	var removed, kept, cascaded int64
	_, err = o.S.MutateCascade(fmt.Sprintf("clear container %d", req.InventoryID), func(m *save.Mut) error {
		removed, kept, cascaded = 0, 0, 0
		set, err := readClearSet(m.Query, req.InventoryID, req.IncludeCurrency)
		if err != nil {
			return err
		}
		ids := set.top
		kept = set.kept
		if int64(len(ids)) != req.Count {
			return apierr.Newf(apierr.ErrConflict, "this container holds %s to clear now, not the %s that were confirmed, so nothing was changed: open it again and check the count", nItems(int64(len(ids))), nItems(req.Count))
		}
		if int64(len(set.nested)) != req.Nested {
			return apierr.Newf(apierr.ErrConflict, "the bags in this container hold %s now, not the %s that were confirmed, so nothing was changed: open it again and check the count", nItems(int64(len(set.nested))), nItems(req.Nested))
		}
		if set.token() != req.IDsHash {
			return apierr.New(apierr.ErrConflict, "this container holds other items than the ones that were confirmed (the number is the same), so nothing was changed: open it again and check the list")
		}
		cascaded = int64(len(set.nested))
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
	res := map[string]any{"ok": true, "removed": removed, "kept": kept, "nested": cascaded}
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
