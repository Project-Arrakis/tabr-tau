package basefile

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
)

// The values of a base file (#613, audit D3-1). Verify (load.go) checks the ids and what they point at; this checks the numbers, which a hand-edited or foreign file can set to
// anything: a stack of 9e18, two items in one slot, a health of 1e300 that the repair functions would copy onto every piece of the type. The rules are the ones the editor's own
// give paths keep (a slot holds no more than the item's stack size, one item to a slot), and a ceiling for the figures the game's tables do not give us per item. Every rule was
// run over the whole of a real save (30,627 items, 3,457 pieces, 230 placeables): none of them refuses anything the game wrote.

// The ceilings. The real figures seen are far below them (a foundation 7,000, a placeable 2,500, gear durability 100); the ceilings are there to refuse the absurd, not to model
// the game. A stack of an item the catalog does not know is held to the largest stack the catalog knows (100,000).
const (
	// MaxPieceHealth is the most health a building piece or a placeable may have in a file, and the most the repair functions will ever copy onto a piece.
	MaxPieceHealth = 1_000_000.0
	// MaxItemDurability is the most an item's durability figures (current, maximum, decayed maximum) may be.
	MaxItemDurability = 1_000_000.0
	// MaxUnknownStack is the most of one item the file may put in a slot when the catalog does not know the item.
	MaxUnknownStack = 100_000
	// MaxSlot is the highest slot an item may be in when its inventory states no slot count, and the largest slot count an inventory may state (the largest the game's own saves hold
	// is 1,000).
	MaxSlot = 100_000
	// NoSlotCount is the value the game writes in max_item_count when no slot count is stated (as do 0 and NULL).
	NoSlotCount = -1
)

// verifyValues refuses a file whose numbers are out of range. It runs after the ids and names have been checked, so every template and building type it prints is a plain name.
func (b *Base) verifyValues() error {
	// health: a finite number from 0 to MaxPieceHealth, on every piece and every placeable (a placeable may have none)
	for _, t := range []struct {
		table string
		what  string
	}{{"building_instances", "building piece"}, {"placeables", "placeable"}} {
		rs := b.Rows[t.table]
		hc := rs.Col("health")
		for i, r := range rs.R {
			h, ok := r[hc].(float64)
			if !ok {
				continue // NULL (required where the game's schema says so, by Verify)
			}
			if math.IsNaN(h) || math.IsInf(h, 0) || h < 0 || h > MaxPieceHealth {
				return fmt.Errorf("row %d of %s: the health of this %s is %v, which is not a number from 0 to %.0f", i+1, t.table, t.what, h, MaxPieceHealth)
			}
		}
	}

	// items: the stack, the slot, the durability
	inv := b.Rows["inventories"]
	ic, mc := inv.Col("id"), inv.Col("max_item_count")
	// The game's own saves hold -1 (the large majority of inventories), 0 or NULL where no slot count is stated; those are all "no limit stated". Anything below -1 or above MaxSlot is
	// not a count. The slot an item is in is NOT held to the count: a real save holds a few items a little past the end of their inventory's count (the character's own gear), so a
	// slot is checked for being one to an item and below MaxSlot only.
	for _, r := range inv.R {
		id, _ := r[ic].(int64)
		n, _ := r[mc].(int64)
		if n < NoSlotCount || n > MaxSlot {
			return fmt.Errorf("inventory %d has a slot count of %d, which is not a count", id, n)
		}
	}
	it := b.Rows["items"]
	tc, sc, pc, ivc, stc := it.Col("template_id"), it.Col("stack_size"), it.Col("position_index"), it.Col("inventory_id"), it.Col("stats")
	seen := make(map[[2]int64]bool, len(it.R))
	for i, r := range it.R {
		tpl, stack, pos, iv := r[tc].(string), r[sc].(int64), r[pc].(int64), r[ivc].(int64)
		limit, known := catalog.MaxStack(tpl)
		if !known {
			limit = MaxUnknownStack
		}
		if stack > limit {
			return fmt.Errorf("row %d of items: a stack of %d %s is more than one slot holds (%d)", i+1, stack, clip(tpl), limit)
		}
		if pos > MaxSlot {
			return fmt.Errorf("row %d of items: slot %d is above the highest slot allowed (%d)", i+1, pos, MaxSlot)
		}
		k := [2]int64{iv, pos}
		if seen[k] {
			return fmt.Errorf("row %d of items: slot %d of inventory %d already holds another item", i+1, pos, iv)
		}
		seen[k] = true
		if j, ok := r[stc].(JSONText); ok && strings.Contains(string(j), "Durability") {
			if err := durabilityOK(string(j)); err != nil {
				return fmt.Errorf("row %d of items: %v", i+1, err)
			}
		}
	}
	return nil
}

// durabilityOK checks the durability record of an item's stats text, when it has one: the numbers in it are finite and from 0 to MaxItemDurability. A record or a value of another
// shape is left to the game, as everywhere else in the stats.
func durabilityOK(stats string) error {
	var doc map[string][]json.RawMessage
	if json.Unmarshal([]byte(stats), &doc) != nil {
		return nil
	}
	pair := doc["FItemStackAndDurabilityStats"]
	if len(pair) < 2 {
		return nil
	}
	var rec map[string]json.RawMessage
	if json.Unmarshal(pair[1], &rec) != nil {
		return nil
	}
	for _, key := range []string{"CurrentDurability", "MaxDurability", "DecayedMaxDurability"} {
		raw, has := rec[key]
		if !has {
			continue
		}
		text := strings.TrimSpace(string(raw))
		if text == "" || !(text[0] == '-' || text[0] >= '0' && text[0] <= '9') {
			continue // null or text: not a figure to check
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return fmt.Errorf("%s is a number the editor cannot hold", key) // outside the range of a float
		}
		if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > MaxItemDurability {
			return fmt.Errorf("%s is %v, which is not a number from 0 to %.0f", key, f, MaxItemDurability)
		}
	}
	return nil
}
