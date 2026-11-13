package ops

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
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

// bestHealthOf is the derived table of the repair: the best health seen for each type of the table (building_instances or placeables), counting only a health that is a number from 0
// to basefile.MaxPieceHealth. A piece with a health outside that range (a file or an edit put 1e300 there) is not a "best seen" and is never copied: what the repair writes is always
// inside the range. The ceiling is written into the text (it is a constant of the program, not an argument), so that the statement stays one that needs no arguments of its own.
func bestHealthOf(table string) string {
	return `(select building_type bt, max(health) top from ` + table + ` where building_type is not null and health >= 0 and health <= ` + strconv.FormatFloat(basefile.MaxPieceHealth, 'f', 0, 64) + ` group by building_type)`
}

// RepairBuildings sets every building piece and placeable of the whole save to the highest health seen for its type, among the healths from 0 to basefile.MaxPieceHealth (#613). It is the
// save-wide action; the Structures tab offers it apart from the open base's own, RepairBase (#526).
//
// The best health of each type is worked out once, by one grouped scan, and the pieces are joined to it. The statement used to look the
// maximum up again for every piece with a correlated subquery over the whole table (and again in its own WHERE), so it grew with the square
// of the piece count: 4.0 s at 5,000 pieces and 66 s at 20,000, with the save locked for all of it (audit L2 G13).
func (o *Ops) RepairBuildings() (any, error) {
	var n, mm int64
	_, err := o.S.Mutate("", func(m *save.Mut) error {
		res, err := m.Exec(`update building_instances set health=best.top
			from ` + bestHealthOf("building_instances") + ` best
			where building_instances.building_type=best.bt and building_instances.health < best.top`)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		res2, err := m.Exec(`update placeables set health=best.top
			from ` + bestHealthOf("placeables") + ` best
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

// ClearSand clears the sand buildup of every building piece of the whole save (the save-wide action; ClearSandBase is one base's).
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

// Vehicles lists the player's own vehicles and every stored vehicle of the character ("stored": the recovered ones, the one in the vehicle backup and the ones whose state says
// stored, from storedVehicles; "recovered" and "backups" are the raw rows, kept for compatibility). World vehicles are not listed; their
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
	stored := storedVehicles(o.S.Query, o.S.One, p.Controller)
	kinds := map[int64]string{}
	for _, sv := range stored {
		kinds[sv["vehicle_id"].(int64)] = sv["kind"].(string)
	}
	for _, v := range live {
		v["name"] = shortClass(v["class"])
		v["type"] = vehicleType(fmt.Sprint(v["name"]))
		if m, ok := v["map"].(string); ok {
			rf.label(v, m, "x", "y")
		}
		if kind := kinds[v["id"].(int64)]; kind != "" {
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
	return map[string]any{"vehicles": live, "recovered": rec, "backups": bak, "stored": stored, "hidden": hidden["n"]}, nil
}

// BringVehicle moves one of the player's own vehicles next to the player: to the point 10 m ahead along the facing (800 along x when the facing is unknown), 150 up, when it is clear, else to the nearest clear spot, outside a base
// when the player is in one (#480). An aircraft goes at least 30 m ahead and at least 7 m up, in the air (#540). The answer says where it went.
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
	var placed placement
	_, err = o.S.Mutate(fmt.Sprintf("moved vehicle %d next to player", id), func(m *save.Mut) error {
		me, err := m.One(`select map, location_x x, location_y y, location_z z, rotation_x rx, rotation_y ry, rotation_z rz, rotation_w rw, dimension_index d from actors where id=?`, p.Pawn)
		if err != nil {
			return err
		}
		if me == nil {
			return errors.New("player position unknown")
		}
		v, err := m.One(`select class from actors where id=? and id in (`+ownedVehicleSQL+`) and id not in (`+storedVehicleSQL+`)`, id, p.Controller)
		if err != nil {
			return err
		}
		if v == nil {
			return nil
		}
		dim, _ := toInt(me["d"])
		if placed, err = choosePlace(m.Query, fmt.Sprint(me["map"]), dim, poseOf(me), sizeOfActorClass(v["class"]), id, vehicleType(shortClass(v["class"]))); err != nil {
			return err
		}
		res, err := m.Exec(`update actors set map=?, location_x=?, location_y=?, location_z=?
		where id=? and id in (`+ownedVehicleSQL+`) and id not in (`+storedVehicleSQL+`)`,
			me["map"], placed.X, placed.Y, placed.Z, id, p.Controller)
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
	return map[string]any{"ok": true, "place": placed.Info()}, nil
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
