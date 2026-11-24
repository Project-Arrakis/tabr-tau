package basefile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/policy"
)

// The values of a base file (#613, audit D3-1). Verify (load.go) checks the ids and what they point at; this checks the numbers, which a hand-edited or foreign file can set to
// anything: a stack of 9e18, two items in one slot, a health of 1e300 that the repair functions would copy onto every piece of the type. The rules are the ones the editor's own
// give paths keep (a slot holds no more than the item's stack size, one item to a slot), and a ceiling for the figures the game's tables do not give us per item. Every rule was
// run over the whole of a real save (30,627 items, 3,457 pieces, 230 placeables): none of them refuses anything the game wrote. The per-type health caps are the figures measured on that
// save (policy/health.go); the volume ceilings were set from the largest figures the editor documents and then run over the same save (the largest inventory volume limit there is 50,000
// against 1,000,000, the largest item volume 6.0).

// The limits are in internal/policy, which the repair functions share (#613, audit G-045): the health caps per building type, the ceilings of durability, volume and slot, and the stack a
// file may put in a slot for an item the catalog does not know.

// verifyValues refuses a file whose numbers are out of range. It runs after the ids and names have been checked, so every template and building type it prints is a plain name.
func (b *Base) verifyValues() error {
	// health: a finite number from 0 to the cap of the piece's type (policy.PieceHealthCap, policy.PlaceableHealthCap: the most health seen on any piece of the type in a real save, with room
	// above; a type with no figure of its own is held to the overall cap of its table), on every piece and every placeable (a placeable may have none). The game's tables give no maximum
	// health, so the caps are the highest figure seen, and a base whose piece is above one is refused whole with the row and the limit. A placeable hologram has no measured figure and is
	// held to the outer ceiling policy.MaxPieceHealth only. Everything the game wrote in a real save is below the caps: the highest piece is 7,000 (cap 8,750), the highest placeable 20,000
	// (cap 25,000).
	for _, t := range []struct {
		table string
		what  string
		cap   func(string) float64
	}{{"building_instances", "building piece", policy.PieceHealthCap}, {"placeables", "placeable", policy.PlaceableHealthCap}} {
		rs := b.Rows[t.table]
		hc, tc, hg := rs.Col("health"), rs.Col("building_type"), -1
		if t.table == "placeables" {
			hg = rs.Col("is_hologram")
		}
		for i, r := range rs.R {
			h, ok := r[hc].(float64)
			if !ok {
				continue // NULL (required where the game's schema says so, by Verify)
			}
			limit, kind := policy.MaxPieceHealth, "the outer ceiling"
			if holo := hg >= 0 && r[hg] != nil && r[hg] != int64(0); !holo {
				ty, _ := r[tc].(string)
				limit, kind = t.cap(ty), "the most the game gives this type of "+t.what // ty is never empty: Verify requires the name (required, namePattern)
			}
			if math.IsNaN(h) || math.IsInf(h, 0) || h < 0 || h > limit {
				return fmt.Errorf("row %d of %s: the health of this %s is %s, which is not a number from 0 to %.0f (%s)", i+1, t.table, t.what, healthText(h), limit, kind)
			}
		}
	}

	// items: the stack, the slot, the durability
	inv := b.Rows["inventories"]
	ic, mc, vc := inv.Col("id"), inv.Col("max_item_count"), inv.Col("max_item_volume")
	// The game's own saves hold -1 (the large majority of inventories), 0 or NULL where no slot count is stated; those are all "no limit stated". Anything below -1 or above MaxSlot is
	// not a count. The slot an item is in is NOT held to the count: a real save holds a few items a little past the end of their inventory's count (the character's own gear), so a
	// slot is checked for being one to an item and below MaxSlot only.
	for _, r := range inv.R {
		id, _ := r[ic].(int64)
		n, _ := r[mc].(int64)
		if n < policy.NoSlotCount || n > policy.MaxSlot {
			return fmt.Errorf("inventory %d has a slot count of %d, which is not a count", id, n)
		}
		if err := volumeOK(r[vc]); err != nil {
			return fmt.Errorf("inventory %d: the volume limit %v", id, err)
		}
	}
	it := b.Rows["items"]
	tc, sc, pc, ivc, stc, voc := it.Col("template_id"), it.Col("stack_size"), it.Col("position_index"), it.Col("inventory_id"), it.Col("stats"), it.Col("volume_override")
	seen := make(map[[2]int64]bool, len(it.R))
	for i, r := range it.R {
		tpl, stack, pos, iv := r[tc].(string), r[sc].(int64), r[pc].(int64), r[ivc].(int64)
		limit, known := catalog.MaxStack(tpl)
		if !known {
			limit = policy.MaxUnknownStack
		}
		if stack > limit {
			return fmt.Errorf("row %d of items: a stack of %d %s is more than one slot holds (%d)", i+1, stack, clip(tpl), limit)
		}
		if pos > policy.MaxSlot {
			return fmt.Errorf("row %d of items: slot %d is above the highest slot allowed (%d)", i+1, pos, policy.MaxSlot)
		}
		k := [2]int64{iv, pos}
		if seen[k] {
			return fmt.Errorf("row %d of items: slot %d of inventory %d already holds another item", i+1, pos, iv)
		}
		seen[k] = true
		if err := volumeOK(r[voc]); err != nil {
			return fmt.Errorf("row %d of items: the volume %v", i+1, err)
		}
		if j, ok := r[stc].(JSONText); ok && strings.Contains(string(j), "Durability") {
			if err := durabilityOK(string(j)); err != nil {
				return fmt.Errorf("row %d of items: %v", i+1, err)
			}
		}
	}
	return nil
}

// volumeOK checks a volume cell (an inventory's max_item_volume, an item's volume_override): NULL is fine, a number is from 0 to policy.MaxVolume. The text it returns finishes a
// sentence that begins with what the volume is.
func volumeOK(v any) error {
	f, ok := v.(float64)
	if !ok {
		return nil
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > policy.MaxVolume {
		return fmt.Errorf("is %v, which is not a number from 0 to %.0f", f, policy.MaxVolume)
	}
	return nil
}

// durabilityOK checks the durability record of an item's stats text, when it has one. It fails closed (#613, audit G-021): the text is only looked at when it mentions a durability, and
// then it must be what the game writes, an object whose "FItemStackAndDurabilityStats" is a pair ([[], {fields}]) when it has that key at all, and the three figures in the record
// (current, maximum and decayed maximum durability) are, when present, null or a finite number from 0 to policy.MaxItemDurability; text, a flag, a list or an object there is refused,
// not left to the game. Only the durability key is decoded as an array: another key of any shape (a number, text, an object) beside it does not make the text undecodable and never
// lets a bad durability through.
//
// The two objects it reads (the stats and the record) are walked key by key, not decoded into a map, because a map keeps the LAST of a repeated key while SQLite's json_extract on the
// saved text reads the FIRST: a file with the key twice, one good and one bad, would pass this check and be read as the bad one. So a key repeated in either object is refused, and so is
// a key that is one of the four names checked spelled in another case (the game's own readers are case sensitive, but nothing here may rest on that).
func durabilityOK(stats string) error {
	doc, err := recordFields([]byte(stats), "FItemStackAndDurabilityStats")
	if errors.Is(err, errNotRecord) {
		return fmt.Errorf("the stats mention a durability but are not a record of groups")
	} else if err != nil {
		return err
	}
	raw, has := doc["FItemStackAndDurabilityStats"]
	if !has {
		return nil // another group that has "Durability" in its name; none of the three figures below
	}
	var pair []json.RawMessage
	if json.Unmarshal(raw, &pair) != nil || len(pair) < 2 {
		return fmt.Errorf("the durability record is not a pair of a list and a record of fields")
	}
	rec, err := recordFields(pair[1], "CurrentDurability", "MaxDurability", "DecayedMaxDurability")
	if errors.Is(err, errNotRecord) {
		return fmt.Errorf("the durability record is not a record of fields")
	} else if err != nil {
		return err
	}
	for _, key := range []string{"CurrentDurability", "MaxDurability", "DecayedMaxDurability"} {
		raw, has := rec[key]
		if !has {
			continue
		}
		text := strings.TrimSpace(string(raw))
		if text == "null" {
			continue
		}
		if text == "" || !(text[0] == '-' || text[0] >= '0' && text[0] <= '9') {
			return fmt.Errorf("%s is not a number", key)
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return fmt.Errorf("%s is a number the editor cannot hold", key) // outside the range of a float
		}
		if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > policy.MaxItemDurability {
			return fmt.Errorf("%s is %v, which is not a number from 0 to %.0f", key, f, policy.MaxItemDurability)
		}
	}
	return nil
}

// errNotRecord says the text is not a JSON object.
var errNotRecord = errors.New("not a record")

// recordFields reads a JSON object into its keys and the raw text of each value, walking it with the token reader so that every key is seen. A key that comes twice is refused, and so is a
// key that is not one of the names in checked but is the same name in another case.
func recordFields(raw []byte, checked ...string) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errNotRecord
	}
	out := map[string]json.RawMessage{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, errNotRecord
		}
		key, ok := t.(string)
		if !ok {
			return nil, errNotRecord
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("the key %s is given twice in one record, and the two readings would differ", clip(key))
		}
		for _, name := range checked {
			if key != name && strings.EqualFold(key, name) {
				return nil, fmt.Errorf("the key %s is %s spelled in another case, which the editor does not read", clip(key), name)
			}
		}
		var v json.RawMessage
		if dec.Decode(&v) != nil {
			return nil, errNotRecord
		}
		out[key] = v
	}
	if t, err := dec.Token(); err != nil || t != json.Delim('}') {
		return nil, errNotRecord
	}
	return out, nil
}

// healthText prints a health the way a person reads it: whole figures and fractions in full (1000000.5, not 1.0000005e+06), and only a figure too large to read as digits (1e300) in
// the short form.
func healthText(h float64) string {
	if math.Abs(h) < 1e15 {
		return strconv.FormatFloat(h, 'f', -1, 64)
	}
	return strconv.FormatFloat(h, 'g', -1, 64)
}
