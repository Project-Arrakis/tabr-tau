package ops

import (
	"errors"
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A land claim is the totem's own cell (0,0, never stored) plus rows in landclaim_segments, one per extra grid cell.
// One cell is 10x10 foundations (5,120 uu; verified on the dedicated server). Expanding a claim adds cells and may
// raise totems.landclaim_vertical_level; both tables are the same in single-player and on the server, and the Dune
// Docker console edits them the same way. Expansion only: a claim is never shrunk here.
const (
	maxClaimRings    = 5 // a (2n+1) x (2n+1) square: 5 rings is 121 cells
	maxVerticalLevel = 5
)

// LandClaims lists every totem with its stored cells, the size of its full square (rings) and its vertical level.
// rings is the largest n for which every cell of the (2n+1) x (2n+1) square is part of the claim; cells that stick out
// of that square make the claim irregular (irregular is true).
func (o *Ops) LandClaims() (any, error) {
	totems, err := o.S.Query(`select t.id totem_id, a.class, coalesce(t.landclaim_vertical_level,0) level from totems t join actors a on a.id=t.id order by t.id`)
	if err != nil {
		return nil, err
	}
	for _, t := range totems {
		cells, err := o.S.Query(`select grid_location_x x, grid_location_y y from landclaim_segments where totem_id=? order by grid_location_y, grid_location_x`, t["totem_id"])
		if err != nil {
			return nil, err
		}
		have := map[[2]int64]bool{{0, 0}: true}
		for _, c := range cells {
			x, okx := c["x"].(int64)
			y, oky := c["y"].(int64)
			if okx && oky {
				have[[2]int64{x, y}] = true
			}
		}
		rings := int64(0)
		for n := int64(1); n <= maxClaimRings+10 && squareComplete(have, n); n++ {
			rings = n
		}
		t["name"] = shortClass(t["class"])
		delete(t, "class")
		t["cells"] = int64(len(have))
		t["rings"] = rings
		t["irregular"] = int64(len(have)) != (2*rings+1)*(2*rings+1)
		t["maxRings"] = int64(maxClaimRings)
		t["maxLevel"] = int64(maxVerticalLevel)
	}
	return totems, nil
}

func squareComplete(have map[[2]int64]bool, n int64) bool {
	for y := -n; y <= n; y++ {
		for x := -n; x <= n; x++ {
			if !have[[2]int64{x, y}] {
				return false
			}
		}
	}
	return true
}

// ExpandLandClaim grows a totem's claim to a square of the given number of rings around its own cell, and/or raises
// its vertical level. Cells already stored are kept; the square is always connected edge to edge, which the game requires.
func (o *Ops) ExpandLandClaim(a Args) (any, error) {
	id, err := a.Int("totem_id")
	if err != nil {
		return nil, err
	}
	_, hasRings := a["rings"]
	_, hasLevel := a["level"]
	if !hasRings && !hasLevel {
		return nil, errors.New("nothing to change: give rings and/or level")
	}
	var rings, level int64
	if hasRings {
		if rings, err = a.IntRange("rings", 1, maxClaimRings); err != nil {
			return nil, err
		}
	}
	if hasLevel {
		if level, err = a.IntRange("level", 0, maxVerticalLevel); err != nil {
			return nil, err
		}
	}
	t, err := o.S.One(`select coalesce(landclaim_vertical_level,0) level from totems where id=?`, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errors.New("land claim (totem) not found")
	}
	curLevel := t["level"].(int64)
	if hasLevel && level < curLevel {
		return nil, fmt.Errorf("the vertical level is %d; lowering it is not supported", curLevel)
	}
	have := map[[2]int64]bool{{0, 0}: true}
	rows, err := o.S.Query(`select grid_location_x x, grid_location_y y from landclaim_segments where totem_id=?`, id)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		x, okx := r["x"].(int64)
		y, oky := r["y"].(int64)
		if okx && oky {
			have[[2]int64{x, y}] = true
		}
	}
	var add [][2]int64
	for y := -rings; y <= rings; y++ {
		for x := -rings; x <= rings; x++ {
			if !have[[2]int64{x, y}] {
				add = append(add, [2]int64{x, y})
			}
		}
	}
	raise := hasLevel && level > curLevel
	if len(add) == 0 && !raise {
		switch {
		case hasRings && hasLevel:
			return nil, errors.New("the claim already covers that size and has that vertical level")
		case hasRings:
			return nil, errors.New("the claim already covers that size")
		default:
			return nil, fmt.Errorf("the vertical level is already %d", curLevel)
		}
	}
	_, err = o.S.Mutate("", func(m *save.Mut) error {
		for _, c := range add {
			if _, err := m.Exec(`insert into landclaim_segments(totem_id, grid_location_x, grid_location_y) values(?,?,?)`, id, c[0], c[1]); err != nil {
				return err
			}
		}
		if raise {
			if _, err := m.Exec(`update totems set landclaim_vertical_level=? where id=?`, level, id); err != nil {
				return err
			}
		}
		m.Desc = fmt.Sprintf("expand land claim %d: +%d cells (rings %d), level %d", id, len(add), rings, max(level, curLevel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "added": len(add), "totalCells": len(have) + len(add), "level": max(level, curLevel)}, nil
}
