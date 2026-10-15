package ops

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A land claim is the totem's own cell (0,0, never stored) plus rows in landclaim_segments, one per extra grid cell.
// One cell is 10x10 foundations (5,120 uu; verified on the dedicated server). Expanding a claim adds cells and may
// raise totems.landclaim_vertical_level; both tables are the same in single-player and on the server, and the Dune
// Docker console edits them the same way. Cells are added by ExpandLandClaim (a square of rings) and ApplyLandClaim (chosen cells), which
// share one write path, and removed by ShrinkLandClaim (everything outside a square of rings, never a cell that holds pieces). The game
// needs the claim connected edge to edge from the totem's own cell, so no edit may leave a cell cut off from it (see claimReach).
const (
	maxClaimRings    = 5 // a (2n+1) x (2n+1) square: 5 rings is 121 cells
	maxVerticalLevel = 5
)

// claimCell is a grid cell of a claim, {x, y}; the totem's own cell is {0, 0}.
type claimCell = [2]int64

// claimCellSet is the cells of a claim read from landclaim_segments rows, plus the totem's own cell.
func claimCellSet(rows []save.Row) map[claimCell]bool {
	have := map[claimCell]bool{{0, 0}: true}
	for _, r := range rows {
		x, okx := r["x"].(int64)
		y, oky := r["y"].(int64)
		if okx && oky {
			have[claimCell{x, y}] = true
		}
	}
	return have
}

// claimReach is every cell of cells that can be reached from the totem's own cell (0,0) by steps over a shared edge through cells of the
// set. The game needs a claim connected this way; cells outside the result are cut off from the totem.
func claimReach(cells map[claimCell]bool) map[claimCell]bool {
	reached := map[claimCell]bool{}
	if !cells[claimCell{0, 0}] {
		return reached
	}
	reached[claimCell{0, 0}] = true
	queue := []claimCell{{0, 0}}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, n := range claimNeighbours(c) {
			if cells[n] && !reached[n] {
				reached[n] = true
				queue = append(queue, n)
			}
		}
	}
	return reached
}

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
		have := claimCellSet(cells)
		rings := int64(0)
		for n := int64(1); n <= maxClaimRings+10 && squareComplete(have, n); n++ {
			rings = n
		}
		t["name"] = shortClass(t["class"])
		delete(t, "class")
		pc, err := o.pieceCells(t["totem_id"].(int64))
		if err == nil {
			var inside int64
			for c, n := range pc {
				if have[c] {
					inside += n
				}
			}
			t["piecesInClaim"] = inside
			t["piecesOutside"] = totalPieces(pc) - inside
		}
		t["grid"] = claimGrid(have, pc)
		yaw := 0.0
		if y, _ := o.S.One(`select landclaim_original_global_yaw_rotation yaw from totems where id=?`, t["totem_id"]); y != nil {
			yaw, _ = y["yaw"].(float64)
		}
		t["yaw"] = yaw
		segs := make([]map[string]any, 0, len(cells))
		for _, c := range cells {
			segs = append(segs, map[string]any{"x": c["x"], "y": c["y"]})
		}
		t["segments"] = segs
		t["cells"] = int64(len(have))
		t["rings"] = rings
		t["irregular"] = int64(len(have)) != (2*rings+1)*(2*rings+1)
		t["maxRings"] = int64(maxClaimRings)
		t["maxLevel"] = int64(maxVerticalLevel)
	}
	return totems, nil
}

// claimGrid lists the cells to draw: every cell of the claim (the totem's own cell is 0,0) and every cell outside it that holds
// building pieces, each with its piece count (0 when pieces could not be matched to cells), in row order.
func claimGrid(have map[claimCell]bool, pieces map[claimCell]int64) []map[string]any {
	cells := map[claimCell]bool{}
	for c := range have {
		cells[c] = true
	}
	for c, n := range pieces {
		if n > 0 {
			cells[c] = true
		}
	}
	keys := make([]claimCell, 0, len(cells))
	for c := range cells {
		keys = append(keys, c)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][1] != keys[j][1] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][0] < keys[j][0]
	})
	out := make([]map[string]any, 0, len(keys))
	for _, c := range keys {
		out = append(out, map[string]any{"x": c[0], "y": c[1], "in": have[c], "n": pieces[c]})
	}
	return out
}

func squareComplete(have map[claimCell]bool, n int64) bool {
	for y := -n; y <= n; y++ {
		for x := -n; x <= n; x++ {
			if !have[claimCell{x, y}] {
				return false
			}
		}
	}
	return true
}

// claimGrow is what one adding edit did to a claim.
type claimGrow struct {
	added, total int   // cells added, and cells in the claim afterwards (the totem's own included)
	level        int64 // the vertical level afterwards
	raised       bool  // the level went up
}

// growClaim is the one write path of the adding edits (ExpandLandClaim and ApplyLandClaim). In a single transaction it reads the claim's
// stored cells and level, asks plan for the cells to add and the level to end at (plan returns an error to refuse), checks that every
// added cell would be connected to the totem's own cell edge to edge, and writes. Reading, planning and writing together means two
// overlapping requests cannot plan from the same stale claim. desc names the edit in the pending list.
func (o *Ops) growClaim(id int64, desc func(added int, level int64) string, plan func(have map[claimCell]bool, curLevel int64) ([]claimCell, int64, error)) (claimGrow, error) {
	var res claimGrow
	_, err := o.S.Mutate("", func(m *save.Mut) error {
		t, err := m.One(`select coalesce(landclaim_vertical_level,0) level from totems where id=?`, id)
		if err != nil {
			return err
		}
		if t == nil {
			return errors.New("land claim (totem) not found")
		}
		curLevel := t["level"].(int64)
		rows, err := m.Query(`select grid_location_x x, grid_location_y y from landclaim_segments where totem_id=?`, id)
		if err != nil {
			return err
		}
		have := claimCellSet(rows)
		add, level, err := plan(have, curLevel)
		if err != nil {
			return err
		}
		all := map[claimCell]bool{}
		for c := range have {
			all[c] = true
		}
		for _, c := range add {
			all[c] = true
		}
		reach := claimReach(all)
		for _, c := range add {
			if !reach[c] {
				return fmt.Errorf("cell %d, %d is not connected to the claim edge to edge", c[0], c[1])
			}
		}
		for _, c := range add {
			if _, err := m.Exec(`insert into landclaim_segments(totem_id, grid_location_x, grid_location_y) values(?,?,?)`, id, c[0], c[1]); err != nil {
				return err
			}
		}
		res = claimGrow{added: len(add), total: len(have) + len(add), level: level, raised: level > curLevel}
		if res.raised {
			if _, err := m.Exec(`update totems set landclaim_vertical_level=? where id=?`, level, id); err != nil {
				return err
			}
		}
		m.Desc = desc(len(add), level)
		return nil
	})
	return res, err
}

// errLowerLevel is the refusal for a level below the current one, which neither edit supports.
func errLowerLevel(cur int64) error {
	return fmt.Errorf("the vertical level is %d; lowering it is not supported", cur)
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
	res, err := o.growClaim(id,
		func(added int, lvl int64) string {
			return fmt.Sprintf("expand land claim %d: +%d cells (rings %d), level %d", id, added, rings, lvl)
		},
		func(have map[claimCell]bool, curLevel int64) ([]claimCell, int64, error) {
			if hasLevel && level < curLevel {
				return nil, 0, errLowerLevel(curLevel)
			}
			var add []claimCell
			for y := -rings; y <= rings; y++ {
				for x := -rings; x <= rings; x++ {
					if !have[claimCell{x, y}] {
						add = append(add, claimCell{x, y})
					}
				}
			}
			raise := hasLevel && level > curLevel
			if len(add) == 0 && !raise {
				switch {
				case hasRings && hasLevel:
					return nil, 0, errors.New("the claim already covers that size and has that vertical level")
				case hasRings:
					return nil, 0, errors.New("the claim already covers that size")
				default:
					return nil, 0, fmt.Errorf("the vertical level is already %d", curLevel)
				}
			}
			return add, max(level, curLevel), nil
		})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "added": res.added, "totalCells": res.total, "level": res.level}, nil
}

// claimCellSize is the width of one claim cell in world units (10 x 10 foundations of 512, verified on the
// dedicated server). A piece's cell is found in the claim's own frame: its offset from the claim origin, turned by
// minus the claim yaw, divided by the cell size and rounded, so cell (0,0) is centred on the origin. No axis flips.
const claimCellSize = 5120.0

// A piece exactly on a cell boundary (offset +-2560) is placed by rounding half away from zero; the game's own rule for
// that edge is not known, which can only make the count differ by the pieces on the boundary.
// pieceCells counts the building pieces and placeables of one claim per cell, so a cell is never removed from under a base.
func (o *Ops) pieceCells(totemID int64) (map[claimCell]int64, error) {
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
	// A piece belongs to this claim when its owner entity is linked to the totem's actor. A piece with no owner link is
	// counted for every claim: removing a cell must err on the side of keeping it.
	rows, err := o.S.Query(`select bi.location_x x, bi.location_y y from building_instances bi
			left join actor_fgl_entities afe on afe.entity_id=bi.owner_entity_id
			where bi.location_x is not null and bi.location_y is not null and (afe.actor_id is null or afe.actor_id=?)
		union all select a.location_x, a.location_y from placeables p join actors a on a.id=p.id
			left join actor_fgl_entities afe on afe.entity_id=p.owner_entity_id
			where a.location_x is not null and a.location_y is not null and (afe.actor_id is null or afe.actor_id=?)`, totemID, totemID)
	if err != nil {
		return nil, err
	}
	sin, cos := math.Sincos(-yaw * math.Pi / 180)
	out := map[claimCell]int64{}
	for _, r := range rows {
		x, okx := r["x"].(float64)
		y, oky := r["y"].(float64)
		if !okx || !oky {
			continue
		}
		dx, dy := x-ox, y-oy
		lx, ly := dx*cos-dy*sin, dx*sin+dy*cos
		out[claimCell{int64(math.Round(lx / claimCellSize)), int64(math.Round(ly / claimCellSize))}]++
	}
	return out, nil
}

// ShrinkLandClaim removes every stored cell outside a square of the given number of rings around the totem's own cell
// (rings 0 removes all stored cells; the totem's own cell is implicit and always stays). A cell that holds building
// pieces or placeables is never removed: the request is refused and says how many pieces block it. A request that would cut a cell that
// stays off from the totem's own cell (its only link to the totem ran through a removed cell) is refused too, because the game needs the
// claim connected edge to edge; a cell that was already cut off before the request does not block it.
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
	// Building pieces are never moved by an edit made here, so they are read once, outside the transaction. The cells are read, checked and
	// deleted in one transaction, so an Expand or Apply that lands in between cannot leave the check looking at a stale claim.
	pieces, err := o.pieceCells(id)
	if err != nil {
		return nil, err
	}
	var dropped, remaining int
	_, err = o.S.Mutate("", func(m *save.Mut) error {
		rows, err := m.Query(`select grid_location_x x, grid_location_y y from landclaim_segments where totem_id=?`, id)
		if err != nil {
			return err
		}
		have := claimCellSet(rows)
		var drop []claimCell
		left := map[claimCell]bool{{0, 0}: true}
		for _, r := range rows {
			x, okx := r["x"].(int64)
			y, oky := r["y"].(int64)
			if !okx || !oky {
				continue
			}
			if max(x, -x, y, -y) > keep {
				drop = append(drop, claimCell{x, y})
			} else {
				left[claimCell{x, y}] = true
			}
		}
		if len(drop) == 0 {
			return errors.New("there are no stored cells outside that size")
		}
		blocked, blockers := 0, int64(0)
		for _, c := range drop {
			if n := pieces[c]; n > 0 {
				blocked++
				blockers += n
			}
		}
		if blocked > 0 {
			return fmt.Errorf("%d of the cells to remove hold %d building pieces or placeables; nothing was removed", blocked, blockers)
		}
		before, after := claimReach(have), claimReach(left)
		var cut []claimCell
		for c := range left {
			if before[c] && !after[c] {
				cut = append(cut, c)
			}
		}
		if len(cut) > 0 {
			sort.Slice(cut, func(i, j int) bool {
				if cut[i][1] != cut[j][1] {
					return cut[i][1] < cut[j][1]
				}
				return cut[i][0] < cut[j][0]
			})
			return fmt.Errorf("removing those cells would cut %d cells that stay (the first at %d, %d) off from the totem, and the game needs the claim connected edge to edge; choose a larger size; nothing was removed", len(cut), cut[0][0], cut[0][1])
		}
		for _, c := range drop {
			if _, err := m.Exec(`delete from landclaim_segments where totem_id=? and grid_location_x=? and grid_location_y=?`, id, c[0], c[1]); err != nil {
				return err
			}
		}
		dropped, remaining = len(drop), len(left)
		m.Desc = fmt.Sprintf("shrink land claim %d: -%d cells (keep %d rings)", id, len(drop), keep)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "removed": dropped, "remaining": remaining}, nil
}

func totalPieces(pc map[claimCell]int64) int64 {
	var n int64
	for _, v := range pc {
		n += v
	}
	return n
}

// claimNeighbours are the four cells that share an edge with a cell.
func claimNeighbours(c claimCell) []claimCell {
	return []claimCell{{c[0] + 1, c[1]}, {c[0] - 1, c[1]}, {c[0], c[1] + 1}, {c[0], c[1] - 1}}
}

const (
	maxClaimCoord   = 128 // the console's editor offers cells up to this far from the totem
	maxCellsPerEdit = 500
)

// ApplyLandClaim is the console's land claim editor: add the chosen cells (args cells, a list of {x, y}) and/or raise the vertical level
// (args level). Every new cell must share an edge with the claim or with another new cell that does, back to the totem's own cell (0,0),
// because the game needs the claim connected. A cell already in the claim is refused (the console never offers one), and the level can only
// go up, to 5.
func (o *Ops) ApplyLandClaim(a Args) (any, error) {
	id, err := a.Int("totem_id")
	if err != nil {
		return nil, err
	}
	raw, _ := a["cells"].([]any)
	_, hasLevel := a["level"]
	if len(raw) == 0 && !hasLevel {
		return nil, errors.New("nothing to change: choose cells and/or a vertical level")
	}
	if len(raw) > maxCellsPerEdit {
		return nil, fmt.Errorf("at most %d cells in one edit", maxCellsPerEdit)
	}
	var want int64
	if hasLevel {
		if want, err = a.IntRange("level", 0, maxVerticalLevel); err != nil {
			return nil, err
		}
	}
	res, err := o.growClaim(id,
		func(added int, lvl int64) string {
			return fmt.Sprintf("land claim %d: +%d cells, vertical level %d", id, added, lvl)
		},
		func(have map[claimCell]bool, curLevel int64) ([]claimCell, int64, error) {
			level := curLevel
			if hasLevel {
				level = want
				if level < curLevel {
					return nil, 0, errLowerLevel(curLevel)
				}
			}
			var add []claimCell
			adding := map[claimCell]bool{}
			for i, e := range raw {
				m, ok := e.(map[string]any)
				if !ok {
					return nil, 0, fmt.Errorf("cell %d is not {x, y}", i+1)
				}
				x, errx := Args(m).IntRange("x", -maxClaimCoord, maxClaimCoord)
				y, erry := Args(m).IntRange("y", -maxClaimCoord, maxClaimCoord)
				if errx != nil || erry != nil {
					return nil, 0, fmt.Errorf("cell %d: x and y must be whole numbers from -%d to %d", i+1, maxClaimCoord, maxClaimCoord)
				}
				c := claimCell{x, y}
				if have[c] {
					return nil, 0, fmt.Errorf("cell %d, %d is already in the claim", x, y)
				}
				if adding[c] {
					return nil, 0, fmt.Errorf("cell %d, %d is listed twice", x, y)
				}
				adding[c] = true
				add = append(add, c)
			}
			if len(add) == 0 && level <= curLevel {
				return nil, 0, fmt.Errorf("the vertical level is already %d and no cells were chosen", curLevel)
			}
			return add, level, nil
		})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "added": res.added, "level": res.level, "levelRaised": res.raised}, nil
}
