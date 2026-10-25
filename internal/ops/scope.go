package ops

import (
	"errors"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// The write routes that name an item or an inventory by id take what the page sends, so the id is checked against what the page may
// offer: the player's carried inventories (backpack, worn gear, loadout, unique schematics), the containers of the player's bases
// (the same allowlist the Bases > Inventory tab lists) and the cargo hold of a vehicle the player owns (#48). Anything else, such as a
// vehicle's per-component hold, the cargo hold of a vehicle that is not the player's, a world loot container or an inventory of another
// character, is refused (ARCH-9 of the 2026-10-15 audit; the narrowing of #92 and #75 had not reached these).

// carriedInventoryTypes are the player's carried inventories the Inventory tab lists (see invTypeNames).
const carriedInventoryTypes = `(0, 1, 15, 30)`

var errOutOfScope = errors.New("that is not in your backpack, worn gear, loadout or schematics, or in a container at one of your bases or the cargo hold of one of your vehicles")

// carriedInventory says whether the inventory is one the character carries.
func (o *Ops) carriedInventory(p player, inv int64) bool {
	r, _ := o.S.One(`select 1 x from inventories where id=? and actor_id=? and inventory_type in `+carriedInventoryTypes, inv, p.Pawn)
	return r != nil
}

// ownedVehicleCargo says whether the inventory is the cargo hold (inventory_type 0) of a vehicle the character owns, by the rule Bring vehicle and Repair use
// (ownedVehicleSQL). The per-component holds of a vehicle have no inventory_type and are not cargo; the character's own backpack is type 0 too but belongs to
// the body, which is not in the vehicles table.
func (o *Ops) ownedVehicleCargo(p player, inv int64) bool {
	r, _ := o.S.One(`select 1 x from inventories where id=? and inventory_type=0 and actor_id in (`+ownedVehicleSQL+`)`, inv, p.Controller)
	return r != nil
}

// baseContainerInventory says whether the inventory is a container of one of the player's bases.
func (o *Ops) baseContainerInventory(inv int64) (bool, error) {
	rows, err := o.baseContainers(0, "storage", "refining", "crafting", "other")
	if err != nil {
		return false, err
	}
	for _, r := range rows {
		if id, _ := r["inventory_id"].(int64); id == inv {
			return true, nil
		}
	}
	return false, nil
}

// requireEditableItem refuses an item that does not exist, or that is outside the inventories above.
func (o *Ops) requireEditableItem(item int64) error {
	it, err := o.S.One(`select inventory_id inv from items where id=?`, item)
	if err != nil {
		return err
	}
	if it == nil {
		return apierr.New(apierr.ErrNotFound, "item not found")
	}
	inv, _ := it["inv"].(int64)
	p, err := o.player()
	if err != nil {
		return err
	}
	if o.carriedInventory(p, inv) {
		return nil
	}
	if ok, err := o.baseContainerInventory(inv); err != nil {
		return err
	} else if ok {
		return nil
	}
	if o.ownedVehicleCargo(p, inv) {
		return nil
	}
	return errOutOfScope
}

// requireBaseContainer refuses an inventory that is not a container of one of the player's bases or the cargo hold of one of the player's vehicles.
func (o *Ops) requireBaseContainer(inv int64) error {
	ok, err := o.baseContainerInventory(inv)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	p, err := o.player()
	if err != nil {
		return err
	}
	if o.ownedVehicleCargo(p, inv) {
		return nil
	}
	return errors.New("that is not a container at one of your bases or the cargo hold of one of your vehicles")
}
