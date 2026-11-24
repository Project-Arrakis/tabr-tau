package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// What the layout roll needs to know before it recycles the Deep Desert bases (#1167): where room is needed and how much is free, and that a recycled base is really gone. Both
// functions here only READ the save. They do not change what a recycle does, where it puts anything, or what it refuses.
//
// A recycle puts things in exactly two kinds of place: the character's Reserve (every stored item stack of the base, and the refund stacks) and, with the Solido option, one new
// blank Solido device in the backpack or, when that has no room, in the loadout. It moves no vehicle and no other item anywhere.

// InventoryRoom is one place a recycle puts something, and how much room it has now.
type InventoryRoom struct {
	Name       string `json:"name"`
	ID         int64  `json:"id"`      // 0 when the character has no such inventory
	Present    bool   `json:"present"` // the character has it
	Capacity   int64  `json:"capacity"`
	Used       int64  `json:"used"`       // item rows in it
	FreeSlots  int64  `json:"freeSlots"`  // -1 when the save records no slot limit for it
	DeviceFit  int64  `json:"deviceFit"`  // how many Solido devices the volume limit still leaves room for; -1 when no volume limit applies or it could not be checked
	VolumeNote string `json:"volumeNote"` // why the volume was not checked, when it was not
}

// RecycleRoom is the room the three destinations have now.
type RecycleRoom struct {
	Reserve  InventoryRoom `json:"reserve"`
	Backpack InventoryRoom `json:"backpack"`
	Loadout  InventoryRoom `json:"loadout"`
}

// RecycleRoom reads the Reserve (inventory type 33), the backpack (type 0) and the loadout (type 15) of the character, found by type as the recycle finds them. The Reserve's
// limit is its max_item_count: the recycle itself refuses when the rows it adds would pass it, and a value of 0 or less means the save records no limit (the saves seen have -1).
func (o *Ops) RecycleRoom() (RecycleRoom, error) {
	var out RecycleRoom
	pl, err := o.player()
	if err != nil || pl.Controller <= 0 || pl.Pawn <= 0 {
		return out, errors.New("this save has no character to receive what a base returns")
	}
	one := func(typ int, name string) (InventoryRoom, error) {
		ir := InventoryRoom{Name: name, FreeSlots: -1, DeviceFit: -1}
		rows, err := o.S.Query(`select id, coalesce(max_item_count,0) cap from inventories where actor_id=? and inventory_type=? and item_id is null order by id`, pl.Pawn, typ)
		if err != nil {
			return ir, err
		}
		if len(rows) == 0 {
			return ir, nil
		}
		if len(rows) > 1 && typ == reserveInventoryType {
			return ir, apierr.Newf(apierr.ErrConflict, "the character has %d Reserve inventories, not one", len(rows))
		}
		ir.Present = true
		ir.ID, _ = toInt(rows[0]["id"])
		ir.Capacity, _ = toInt(rows[0]["cap"])
		n, err := o.S.One(`select count(*) n from items where inventory_id=?`, ir.ID)
		if err != nil {
			return ir, err
		}
		ir.Used, _ = toInt(n["n"])
		if typ == reserveInventoryType {
			if ir.Capacity > 0 {
				ir.FreeSlots = max(ir.Capacity-ir.Used, 0)
			}
			return ir, nil
		}
		if ir.FreeSlots, err = freeSlotsInTx(o.S, ir.ID); err != nil {
			return ir, err
		}
		fit, checked, note, err := volumeLeftInTx(o.S, ir.ID, solidoDevice)
		if err != nil {
			return ir, err
		}
		ir.DeviceFit = fit
		if !checked {
			ir.DeviceFit = -1
			ir.VolumeNote = note
		}
		return ir, nil
	}
	if out.Reserve, err = one(reserveInventoryType, "Reserve"); err != nil {
		return out, err
	}
	if out.Backpack, err = one(solidoBackpackType, "backpack"); err != nil {
		return out, err
	}
	out.Loadout, err = one(solidoLoadoutType, "loadout")
	return out, err
}

// VerifyRecycledBase says whether a recycled base is really gone and its Solido is really in place, on the save as it is now: no actor, totem or permission record of the base is
// left, the blueprint row names the device, and the device's own stats name the blueprint. An error names the first thing that is not so.
func (o *Ops) VerifyRecycledBase(totem, item, blueprint int64) error {
	if totem <= 0 || item <= 0 || blueprint <= 0 {
		return errors.New("the base, the Solido device or the blueprint is not named")
	}
	count := func(what, q string, want int64, args ...any) error {
		r, err := o.S.One(q, args...)
		if err != nil {
			return err
		}
		if n, _ := toInt(r["n"]); n != want {
			return fmt.Errorf("%s: %d found, %d expected", what, n, want)
		}
		return nil
	}
	checks := []struct {
		what, q string
		want    int64
		args    []any
	}{
		{"the base's actor", `select count(*) n from actors where id=?`, 0, []any{totem}},
		{"the base's totem", `select count(*) n from totems where id=?`, 0, []any{totem}},
		{"the base's permission record", `select count(*) n from permission_actor where actor_id=?`, 0, []any{totem}},
		{"the Solido's blueprint row", `select count(*) n from building_blueprints where id=? and item_id=?`, 1, []any{blueprint, item}},
		{"the blueprints of the Solido device", `select count(*) n from building_blueprints where item_id=?`, 1, []any{item}},
		{"the Solido device's record of its blueprint", `select count(*) n from items where id=? and template_id=? and json_extract(stats,'$.` + solidoStatsKey + `[1].PlayerBlueprintId')=?`, 1, []any{item, solidoDevice, fmt.Sprintf("!!bbp#%d", blueprint)}},
	}
	for _, c := range checks {
		if err := count(c.what, c.q, c.want, c.args...); err != nil {
			return err
		}
	}
	return nil
}

// OtherDeepDesertBases lists the standing totems on the Deep Desert (or on no recorded map) that the player does NOT hold the owner rank over: owned by someone else, shared, or
// owned by nobody. DeepDesertHolds never lists them (only the player's own bases block a roll), so the roll that recycles bases asks for them separately and stops when one stands.
func (o *Ops) OtherDeepDesertBases() ([]HoldBase, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	rows, err := o.S.Query(`select t.id id, coalesce(a.map,'') map, a.location_x x, a.location_y y, coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type
		from totems t join actors a on a.id=t.id
		left join permission_actor pa on pa.actor_id=t.id left join placeables tp on tp.id=t.id
		where coalesce(a.state,0)<>? and t.id not in (select permission_actor_id from permission_actor_rank where player_id=? and rank=1) order by t.id`, pickedUpState, p.Controller)
	if err != nil {
		return nil, err
	}
	out := []HoldBase{}
	for _, b := range rows {
		m, _ := b["map"].(string)
		if m != "" && !isDeepDesertMap(m) {
			continue
		}
		id, ok := toInt(b["id"])
		if !ok {
			return nil, errors.New("a base has no usable number")
		}
		name, _ := b["name"].(string)
		tt, _ := b["totem_type"].(string)
		out = append(out, HoldBase{ID: id, Name: backupDisplayName(name, tt), Map: m, X: num(b["x"]), Y: num(b["y"])})
	}
	return out, nil
}

// IsRoomRefusal says whether a recycle (or its question) was refused for lack of room: the Reserve's slot limit, or no place for the new Solido device. The layout roll treats such a
// refusal as "this base does not fit now" and stops after the bases that did fit; every other refusal stops the whole run. It only reads the text of the refusals the recycle gives.
func IsRoomRefusal(err error) bool {
	if err == nil {
		return false
	}
	m := err.Error()
	return strings.Contains(m, "a new one has no room") || strings.Contains(m, "has no free slot") || strings.Contains(m, "The Reserve of this save has room for")
}
