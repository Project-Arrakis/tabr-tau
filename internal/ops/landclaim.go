package ops

import (
	"errors"
	"fmt"
	"math"

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
		if pc, err := o.pieceCells(t["totem_id"].(int64)); err == nil {
			var inside int64
			for c, n := range pc {
				if have[c] {
					inside += n
				}
			}
			t["piecesInClaim"] = inside
			t["piecesOutside"] = totalPieces(pc) - inside
		}
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

// claimCellSize is the width of one claim cell in world units (10 x 10 foundations of 512, verified on the
// dedicated server). A piece's cell is found in the claim's own frame: its offset from the claim origin, turned by
// minus the claim yaw, divided by the cell size and rounded, so cell (0,0) is centred on the origin. No axis flips.
const claimCellSize = 5120.0

// pieceCells counts the building pieces and placeables of one claim per cell, so a cell is never removed from under a base.
func (o *Ops) pieceCells(totemID int64) (map[[2]int64]int64, error) {
	t, err := o.S.One(`select landclaim_original_global_location_x ox, landclaim_original_global_location_y oy, landclaim_original_global_yaw_rotation yaw from totems where id=?`, totemID)
	if err != nil {
		return nil, err
	}
	ox, okx := t["ox"].(float64)
	oy, oky := t["oy"].(float64)
	yaw, okyaw := t["yaw"].(float64)
	if t == nil || !okx || !oky || !okyaw {
		return nil, errors.New("this claim has no recorded origin, so its cells cannot be matched to pieces; nothing was removed")
	}
	rows, err := o.S.Query(`select location_x x, location_y y from building_instances where location_x is not null and location_y is not null
		union all select a.location_x, a.location_y from placeables p join actors a on a.id=p.id where a.location_x is not null and a.location_y is not null`)
	if err != nil {
		return nil, err
	}
	sin, cos := math.Sincos(-yaw * math.Pi / 180)
	out := map[[2]int64]int64{}
	for _, r := range rows {
		x, okx := r["x"].(float64)
		y, oky := r["y"].(float64)
		if !okx || !oky {
			continue
		}
		dx, dy := x-ox, y-oy
		lx, ly := dx*cos-dy*sin, dx*sin+dy*cos
		out[[2]int64{int64(math.Round(lx / claimCellSize)), int64(math.Round(ly / claimCellSize))}]++
	}
	return out, nil
}

// ShrinkLandClaim removes every stored cell outside a square of the given number of rings around the totem's own cell
// (rings 0 removes all stored cells; the totem's own cell is implicit and always stays). A cell that holds building
// pieces or placeables is never removed: the request is refused and says how many pieces block it.
func (o *Ops) ShrinkLandClaim(a Args) (any, error) {
	id, err := a.Int("totem_id")
	if err != nil {
		return nil, err
	}
	keep, err := a.IntRange("rings", 0, maxClaimRings)
	if err != nil {
		return nil, err
	}
	if t, err := o.S.One(`select 1 x from totems where id=?`, id); err != nil || t == nil {
		return nil, errors.New("land claim (totem) not found")
	}
	rows, err := o.S.Query(`select grid_location_x x, grid_location_y y from landclaim_segments where totem_id=?`, id)
	if err != nil {
		return nil, err
	}
	var drop [][2]int64
	for _, r := range rows {
		x, okx := r["x"].(int64)
		y, oky := r["y"].(int64)
		if okx && oky && max(x, -x, y, -y) > keep {
			drop = append(drop, [2]int64{x, y})
		}
	}
	if len(drop) == 0 {
		return nil, errors.New("there are no stored cells outside that size")
	}
	pieces, err := o.pieceCells(id)
	if err != nil {
		return nil, err
	}
	blocked, blockers := 0, int64(0)
	for _, c := range drop {
		if n := pieces[c]; n > 0 {
			blocked++
			blockers += n
		}
	}
	if blocked > 0 {
		return nil, fmt.Errorf("%d of the cells to remove hold %d building pieces or placeables; nothing was removed", blocked, blockers)
	}
	_, err = o.S.Mutate("", func(m *save.Mut) error {
		for _, c := range drop {
			if _, err := m.Exec(`delete from landclaim_segments where totem_id=? and grid_location_x=? and grid_location_y=?`, id, c[0], c[1]); err != nil {
				return err
			}
		}
		m.Desc = fmt.Sprintf("shrink land claim %d: -%d cells (keep %d rings)", id, len(drop), keep)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "removed": len(drop), "remaining": len(rows) - len(drop) + 1}, nil
}

func totalPieces(pc map[[2]int64]int64) int64 {
	var n int64
	for _, v := range pc {
		n += v
	}
	return n
}
