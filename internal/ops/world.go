package ops

import (
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

// ownedByAnyBase is the test, for a row of the table named by alias, that some totem's entity is its owner: the piece belongs to a base of the player's (the opposite of ownedByNoBase).
func ownedByAnyBase(alias string) string {
	return alias + `.owner_entity_id in (select afe.entity_id from actor_fgl_entities afe join totems t on t.id=afe.actor_id)`
}

// notInBackup is the test, for a row of the table named by table whose actor id is in idCol, that its actor is not in the picked-up state (5, which is what an imported backup is written in):
// the write of Repair every base leaves such a row as it is (#815), the same way bestHealthOf does not take it as a source. A row with no actor is not a backup and is not left out.
func notInBackup(table, idCol string) string {
	return table + `.` + idCol + ` not in (select ba.id from actors ba where ba.state = 5)`
}

// bestHealthOf is the derived table of the repair: the repair target of each type of the table (building_instances or placeables), as "bt" (the type) and "top" (the target).
//
// The target is the best health seen on STANDING pieces of the player's own bases (#613, audit G-009): a piece counts only when a totem's entity owns it and its actor is not in the
// picked-up state (5, which is what an imported backup is written in; a backup's pieces have no owner either). A placeable that is a hologram does not count. A health outside 0 to
// basefile.MaxPieceHealth is never a "best seen". The best of a type is then held to the cap of the type (healthcaps.go): the table's own figure where it has one, else the overall cap of
// the table, so a standing piece with a health far above what the game gives is not spread to the other pieces of its type. The ceilings are written into the text (they are constants of
// the program, not arguments), so that the statement stays one that needs no arguments of its own.
func bestHealthOf(table string) string {
	idCol, overall, seen, extra := "building_id", MaxRepairPieceHealth, pieceTypeHealthSeen, ""
	if table == "placeables" {
		idCol, overall, seen, extra = "id", MaxRepairPlaceableHealth, placeableTypeHealthSeen, " and x.is_hologram=0"
	}
	return `(select s.building_type bt, min(s.top, coalesce(c.cap, ` + strconv.FormatFloat(overall, 'f', -1, 64) + `)) top
		from (select x.building_type, max(x.health) top from ` + table + ` x join actors a on a.id=x.` + idCol + `
			where x.building_type is not null and x.health >= 0 and x.health <= ` + strconv.FormatFloat(basefile.MaxPieceHealth, 'f', 0, 64) + extra + `
				and coalesce(a.state,0) <> 5 and ` + ownedByAnyBase("x") + ` group by x.building_type) s
		left join (select column1 ty, column2 cap from (values ` + capValues(seen) + `)) c on c.ty=s.building_type)`
}

// RepairBuildings raises every building piece and placeable of the whole save (a placeable hologram is left as it is, as in RepairBase, #770, and so is a piece of an imported backup, #815) to the repair target of its type (bestHealthOf: the best health seen on standing pieces of the player's own bases, held to the cap of the type, #613). It is the
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
			where building_instances.building_type=best.bt and building_instances.health < best.top
				and ` + notInBackup("building_instances", "building_id"))
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		res2, err := m.Exec(`update placeables set health=best.top
			from ` + bestHealthOf("placeables") + ` best
			where placeables.building_type=best.bt and placeables.health < best.top and placeables.is_hologram=0
				and ` + notInBackup("placeables", "id"))
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
		// the character's place: the map and dimension of the controller, the body's position, and a refusal when the body is not where the player is (#654)
		here, err := readCharacterPlace(m.One, p, true)
		if err != nil {
			return err
		}
		if here.Parked != nil {
			return here.Parked
		}
		v, err := m.One(`select class from actors where id=? and id in (`+ownedVehicleSQL+`) and id not in (`+storedVehicleSQL+`)`, id, p.Controller)
		if err != nil {
			return err
		}
		if v == nil {
			return nil
		}
		if placed, err = choosePlace(m.Query, here.Map, here.Dim, poseOf(here.Body), sizeOfActorClass(v["class"]), id, vehicleType(shortClass(v["class"]))); err != nil {
			return err
		}
		res, err := m.Exec(`update actors set map=?, dimension_index=?, location_x=?, location_y=?, location_z=?
		where id=? and id in (`+ownedVehicleSQL+`) and id not in (`+storedVehicleSQL+`)`,
			here.Map, here.Dim, placed.X, placed.Y, placed.Z, id, p.Controller)
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
