package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The Structures tab of one base (#526). A building piece or a placeable belongs to a base when its owner entity is one of the base
// totem's entities (ownedByBase, the link the containers, the Power tab and the Water tab use). A piece whose owner is empty, or is not
// an entity of any totem, belongs to no base: it is listed apart and never counted into a base.

// ownedByNoBase is the test, for a row of the table named by alias, that no totem's entity is its owner.
func ownedByNoBase(alias string) string {
	return `not exists (select 1 from actor_fgl_entities afe join totems t on t.id=afe.actor_id where afe.entity_id=` + alias + `.owner_entity_id)`
}

// requireTotem refuses an id that is not a base (a totem) with a bad-argument error (422), like every route that takes a base.
func (o *Ops) requireTotem(baseID int64) error {
	if baseID > 0 {
		if r, err := o.S.One(`select 1 x from totems where id=?`, baseID); err != nil {
			return err
		} else if r != nil {
			return nil
		}
	}
	return apierr.Newf(apierr.ErrBadArgument, "%d is not a base (the id of a totem)", baseID)
}

// structureSummary is the health card and the piece-type table of the building pieces chosen by where (a condition on building_instances
// bi, with its arguments).
func (o *Ops) structureSummary(where string, args ...any) (map[string]any, []map[string]any, error) {
	pieces, err := o.S.One(`select count(*) n, coalesce(min(health),0) minh, coalesce(avg(health),0) avgh, coalesce(sum(sand_buildup),0) sand
		from building_instances bi where `+where, args...)
	if err != nil {
		return nil, nil, err
	}
	types, err := o.S.Query(`select building_type, count(*) n, min(health) minh, max(health) maxh, round(avg(health)) avgh
		from building_instances bi where `+where+` group by building_type order by n desc, building_type`, args...)
	if err != nil {
		return nil, nil, err
	}
	return pieces, types, nil
}

// placeableRows lists the built placeables (not holograms) chosen by where (a condition on placeables p).
func (o *Ops) placeableRows(where string, args ...any) ([]map[string]any, error) {
	return o.S.Query(`select p.id, a.class, p.building_type, p.health, a.location_x x, a.location_y y, a.location_z z
		from placeables p left join actors a on a.id=p.id where p.is_hologram=0 and `+where+` order by p.building_type, p.id`, args...)
}

// BaseStructures is the Structures tab of one base: the health card, the piece-type table and the placeables of the pieces and
// placeables the base owns, and apart from them "unowned" (pieces and placeables no base owns). "save" holds the two whole-save counts,
// so the page can say how many pieces the other bases have. Nothing else in the answer is a whole-save figure.
func (o *Ops) BaseStructures(baseID int64) (any, error) {
	if err := o.requireTotem(baseID); err != nil {
		return nil, err
	}
	pieces, types, err := o.structureSummary("bi."+ownedByBase, baseID)
	if err != nil {
		return nil, err
	}
	placeables, err := o.placeableRows("p." + ownedByBase, baseID)
	if err != nil {
		return nil, err
	}
	uPieces, uTypes, err := o.structureSummary(ownedByNoBase("bi"))
	if err != nil {
		return nil, err
	}
	uPlaceables, err := o.placeableRows(ownedByNoBase("p"))
	if err != nil {
		return nil, err
	}
	allPieces, err := o.S.One(`select count(*) n from building_instances`)
	if err != nil {
		return nil, err
	}
	allPlaceables, err := o.S.One(`select count(*) n from placeables where is_hologram=0`)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"base_id": baseID, "pieces": pieces, "types": types, "placeables": placeables,
		"unowned": map[string]any{"pieces": uPieces, "types": uTypes, "placeables": uPlaceables},
		"save":    map[string]any{"pieces": allPieces["n"], "placeables": allPlaceables["n"]},
	}, nil
}

// RepairBase raises the building pieces and the built placeables owned by one base to the highest health seen for their type.
//
// The rule for the target: the highest health of a type is looked up over the whole save (the same figure RepairBuildings uses), but only
// rows owned by this base are written. A base whose every wall is worn is therefore still repaired, which a maximum taken from the base's
// own pieces would not do. Nothing is ever lowered, and the other bases' rows and the unowned ones are not touched.
func (o *Ops) RepairBase(baseID int64) (any, error) {
	if err := o.requireTotem(baseID); err != nil {
		return nil, err
	}
	var n, mm int64
	_, err := o.S.Mutate("", func(m *save.Mut) error {
		res, err := m.Exec(`update building_instances set health=best.top
			from (select building_type bt, max(health) top from building_instances where building_type is not null group by building_type) best
			where building_instances.building_type=best.bt and building_instances.health < best.top and building_instances.`+ownedByBase, baseID)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		res2, err := m.Exec(`update placeables set health=best.top
			from (select building_type bt, max(health) top from placeables where building_type is not null group by building_type) best
			where placeables.building_type=best.bt and placeables.health < best.top and placeables.is_hologram=0 and placeables.`+ownedByBase, baseID)
		if err != nil {
			return err
		}
		mm, _ = res2.RowsAffected()
		m.Desc = fmt.Sprintf("repair base %d: %d pieces, %d placeables", baseID, n, mm)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "base_id": baseID, "pieces": n, "placeables": mm}, nil
}

// ClearSandBase clears the sand buildup of the building pieces owned by one base.
func (o *Ops) ClearSandBase(baseID int64) (any, error) {
	if err := o.requireTotem(baseID); err != nil {
		return nil, err
	}
	var n int64
	_, err := o.S.Mutate("", func(m *save.Mut) error {
		res, err := m.Exec(`update building_instances set sand_buildup=0 where sand_buildup<>0 and `+ownedByBase, baseID)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		m.Desc = fmt.Sprintf("cleared sand buildup on %d pieces of base %d", n, baseID)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "base_id": baseID, "pieces": n}, nil
}
