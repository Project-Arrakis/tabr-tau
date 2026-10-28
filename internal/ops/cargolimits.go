package ops

import (
	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The cargo hold of a vehicle has the room its parts give (#483). The game stores that room on the hold's own inventory row: max_item_count is the slots and max_item_volume the volume,
// and in every vehicle of the saves studied (docs/design/2026-10-28-cargo-limits.md) they equal what catalog.DeriveVehicle sums over the vehicle's modules. A vehicle with no storage part
// has a hold row with 0 and 0.0: it holds nothing. That is different from the other inventories, where 0 (and -1) mean "no limit recorded", so a give that read 0 as "no limit" put
// 5000 Carbon Ore into a Sandbike. The rule is in one place, here, for the card (vehicleDetails) and for the give (GiveToInventory).

// errNoCargoHold is the refusal of a give into the hold of a vehicle that has no storage part.
var errNoCargoHold = apierr.New(apierr.ErrNoRoom, "This vehicle has no cargo hold (no storage part).")

// holdSlots says how many slots a vehicle's cargo hold has. stored is the hold row's max_item_count (nil when the row has none) and moduleIDs the vehicle's modules. A stored count is
// the answer, as the game wrote it (0 is a hold with no room, a negative count is "no limit": known is false then, there is no number to show). A hold row with no count falls back to
// the catalog's sum over the modules, when the catalog knows all of them.
func holdSlots(stored any, moduleIDs []string) (slots int64, known bool) {
	if n, ok := stored.(int64); ok {
		if n < 0 {
			return 0, false
		}
		return n, true
	}
	if d, err := catalog.DeriveVehicle(moduleIDs); err == nil {
		return d.CargoSlots, true
	}
	return 0, false
}

// vehicleHoldEmptyInTx says whether the inventory is the cargo hold of a vehicle (inventory_type 0 on an actor in the vehicles table) that has no room at all. It reads inside the
// edit, so a queued copy of a give is put back only onto a file where the hold is as it was.
func vehicleHoldEmptyInTx(m *save.Mut, inventoryID int64) (bool, error) {
	rows, err := m.Query(`select i.actor_id actor, i.max_item_count slots from inventories i where i.id=? and i.inventory_type=0 and i.actor_id in (select id from vehicles)`, inventoryID)
	if err != nil || len(rows) == 0 {
		return false, err
	}
	mods, err := m.Query(`select template_id from vehicle_modules where vehicle_id=? order by id`, rows[0]["actor"])
	if err != nil {
		return false, err
	}
	ids := make([]string, 0, len(mods))
	for _, r := range mods {
		if id, ok := r["template_id"].(string); ok {
			ids = append(ids, id)
		}
	}
	slots, known := holdSlots(rows[0]["slots"], ids)
	return known && slots == 0, nil
}
