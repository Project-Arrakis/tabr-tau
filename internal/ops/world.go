package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

func shortClass(c any) string {
	s := fmt.Sprint(c)
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSuffix(s, "_C")
}

// ---------------------------------------------------------------- bases

// Bases summarises claimed bases (totems), building pieces and placeables.
func (o *Ops) Bases() (any, error) {
	totems, err := o.S.Query(`select t.id totem_id, a.map, t.landclaim_original_global_location_x x, t.landclaim_original_global_location_y y,
		t.landclaim_original_global_location_z z, t.landclaim_vertical_level level from totems t join actors a on a.id=t.id`)
	if err != nil {
		return nil, err
	}
	pieces, _ := o.S.One(`select count(*) n, coalesce(min(health),0) minh, coalesce(avg(health),0) avgh, coalesce(sum(sand_buildup),0) sand from building_instances`)
	types, _ := o.S.Query(`select building_type, count(*) n, min(health) minh, max(health) maxh, round(avg(health)) avgh
		from building_instances group by building_type order by n desc`)
	placeables, _ := o.S.Query(`select p.id, a.class, p.building_type, p.health, a.location_x x, a.location_y y, a.location_z z
		from placeables p join actors a on a.id=p.id order by p.building_type`)
	return map[string]any{"totems": totems, "pieces": pieces, "types": types, "placeables": placeables}, nil
}

// Storage lists every non-player inventory (containers, machines, vehicles) that holds items.
func (o *Ops) StorageItems(inv int64) (any, error) {
	items, err := o.S.Query(`select id, template_id, stack_size, quality_level, position_index, stats from items where inventory_id=? order by position_index`, inv)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		it["fill"] = fillOf(fmt.Sprint(it["template_id"]), fmt.Sprint(it["stats"]))
		delete(it, "stats")
	}
	return items, nil
}

// RepairBuildings sets every building piece to the highest health seen for its type.
//
// The best health of each type is worked out once, by one grouped scan, and the pieces are joined to it. The statement used to look the
// maximum up again for every piece with a correlated subquery over the whole table (and again in its own WHERE), so it grew with the square
// of the piece count: 4.0 s at 5,000 pieces and 66 s at 20,000, with the save locked for all of it (audit L2 G13).
func (o *Ops) RepairBuildings() (any, error) {
	var n, mm int64
	_, err := o.S.Mutate("", func(m *save.Mut) error {
		res, err := m.Exec(`update building_instances set health=best.top
			from (select building_type bt, max(health) top from building_instances where building_type is not null group by building_type) best
			where building_instances.building_type=best.bt and building_instances.health < best.top`)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		res2, err := m.Exec(`update placeables set health=best.top
			from (select building_type bt, max(health) top from placeables where building_type is not null group by building_type) best
			where placeables.building_type=best.bt and placeables.health < best.top`)
		if err != nil {
			return err
		}
		mm, _ = res2.RowsAffected()
		m.Desc = fmt.Sprintf("repair buildings: %d pieces, %d placeables", n, mm)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "pieces": n, "placeables": mm}, nil
}

func (o *Ops) ClearSand() (any, error) {
	var n int64
	_, err := o.S.Mutate("", func(m *save.Mut) error {
		res, err := m.Exec(`update building_instances set sand_buildup=0 where sand_buildup<>0`)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		m.Desc = fmt.Sprintf("cleared sand buildup on %d pieces", n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "pieces": n}, nil
}

// ---------------------------------------------------------------- vehicles

// ownedVehicleSQL selects the vehicle actors a character owns: the owner rank (1) of the vehicle's permission record. A vehicle's
// actors.owner_account_id is NULL in real saves, so it cannot be used; the many grade-0 vehicles the game places in the world have no
// permission record and are never "owned". It takes the character id (player_state.id) as its one parameter.
const ownedVehicleSQL = `select pa.actor_id from permission_actor pa join permission_actor_rank r on r.permission_actor_id=pa.actor_id
	where r.rank=1 and r.player_id=? and pa.actor_id in (select id from vehicles)`

// Vehicles lists the player's own vehicles (and the character's recovered and backup vehicles). World vehicles are not listed; their
// number is returned so the page can say how many are hidden.
func (o *Ops) Vehicles() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	live, err := o.S.Query(`select a.id, a.class, a.map, a.location_x x, a.location_y y, a.location_z z
		from vehicles v join actors a on a.id=v.id where v.id in (`+ownedVehicleSQL+`) order by a.id`, p.Controller)
	if err != nil {
		return nil, err
	}
	rf := o.regionFinder()
	for _, v := range live {
		v["name"] = shortClass(v["class"])
		v["type"] = vehicleType(fmt.Sprint(v["name"]))
		if m, ok := v["map"].(string); ok {
			rf.label(v, m, "x", "y")
		}
		if kind := o.storedKind(v["id"].(int64)); kind != "" {
			v["stored"], v["storedLabel"] = kind, storedLabel(kind)
		}
		o.vehicleDetails(v)
		mods, _ := o.S.Query(`select id, template_id from vehicle_modules where vehicle_id=?`, v["id"])
		v["modules"] = mods
		inv, _ := o.S.Query(`select id inventory_id, inventory_type from inventories where actor_id=?`, v["id"])
		v["inventories"] = inv
	}
	hidden, _ := o.S.One(`select count(*) n from vehicles where id not in (`+ownedVehicleSQL+`)`, p.Controller)
	rec, _ := o.S.Query(`select vehicle_id, vehicle_name, time_stored, chassis_durability, customization_id, reason from recovered_vehicles
		where character_id=? order by time_stored`, p.Controller)
	bak, _ := o.S.Query(`select vehicle_id, customization_id from backup_vehicles where character_id=?`, p.Controller)
	return map[string]any{"vehicles": live, "recovered": rec, "backups": bak, "hidden": hidden["n"]}, nil
}

// BringVehicle moves one of the player's own vehicles next to the player.
func (o *Ops) BringVehicle(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	id, err := a.Int("id")
	if err != nil {
		return nil, err
	}
	if err := refuseStored(o.S.One, id, "brought to you"); err != nil {
		return nil, err
	}
	// The player's position is read inside the edit, through its own query: it is kept with the edit, so an edit put back onto a newer file (the player has moved since)
	// is not applied to the old position.
	found := false
	_, err = o.S.Mutate(fmt.Sprintf("moved vehicle %d next to player", id), func(m *save.Mut) error {
		me, err := m.One(`select map, location_x x, location_y y, location_z z from actors where id=?`, p.Pawn)
		if err != nil {
			return err
		}
		if me == nil {
			return errors.New("player position unknown")
		}
		res, err := m.Exec(`update actors set map=?, location_x=?+800, location_y=?, location_z=?+150
		where id=? and id in (`+ownedVehicleSQL+`) and id not in (`+storedVehicleSQL+`)`,
			me["map"], me["x"], me["y"], me["z"], id, p.Controller)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		found = n > 0
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apierr.New(apierr.ErrNotFound, "vehicle not found among your vehicles")
	}
	return ok(), nil
}

// ---------------------------------------------------------------- vendors

// Vendors covers what the single-player save stores about vendors: per-vendor
// purchase counters and restock cycles. (The Solari balance lives on the Player tab.)
func (o *Ops) Vendors() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	stock, _ := o.S.Query(`select vendor_id, template_id, amount_bought from vendor_stock_state where player_id=? order by vendor_id, template_id`, p.Controller)
	cycle, _ := o.S.Query(`select vendor_id, last_interacted_timestamp from vendor_stock_cycle where player_id=? order by vendor_id`, p.Controller)
	return map[string]any{"stock": stock, "cycles": cycle}, nil
}

// ResetVendors clears purchase limits (all vendors, or one when vendor_id is set).
func (o *Ops) ResetVendors(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	v := a.Str("vendor_id")
	q1, q2, args := `delete from vendor_stock_state where player_id=?`, `delete from vendor_stock_cycle where player_id=?`, []any{p.Controller}
	if v != "" {
		q1 += ` and vendor_id=?`
		q2 += ` and vendor_id=?`
		args = append(args, v)
	}
	var n int64
	_, err = o.S.Mutate("", func(m *save.Mut) error {
		r1, err := m.Exec(q1, args...)
		if err != nil {
			return err
		}
		n, _ = r1.RowsAffected()
		if _, err := m.Exec(q2, args...); err != nil {
			return err
		}
		m.Desc = fmt.Sprintf("reset vendor purchase limits (%d rows)", n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "rows": n}, nil
}
