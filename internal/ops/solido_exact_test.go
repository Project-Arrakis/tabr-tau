package ops

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The conversion of a base to a Solido against the game's own (#1187). It is for the person who has a pair of saves from the game (one taken before and one after making a Solido
// of a standing base) and it skips unless the variables name them. Both are copied first and never changed. The test makes the Solido of the base from the save before and
// compares it with the rows the game wrote in the save after, one row at a time, with the transforms to within 0.01.
//
// The numbers a Solido's rows carry (instance_id, placeable_id) are not compared: the game numbers both in an order that is not reproducible from the base's rows (see solido.go).
// A row is matched by its type and place, so the test says that every row of the game has exactly one row here and the other way round, with the same stability flag, health,
// hologram flag, angles and (for a Pentashield) scale.
//
//	TABR_SOLIDO_BEFORE  the save before the Solido was made (it is only read)
//	TABR_SOLIDO_AFTER   the save after
//	TABR_SOLIDO_TOTEM   the totem of the base that the Solido was made of
func TestSolidoRowsMatchTheGamesOwnRowByRow(t *testing.T) {
	bs, as, tid := os.Getenv("TABR_SOLIDO_BEFORE"), os.Getenv("TABR_SOLIDO_AFTER"), os.Getenv("TABR_SOLIDO_TOTEM")
	if bs == "" || as == "" || tid == "" {
		t.Skip("no pair of saves named (TABR_SOLIDO_BEFORE, TABR_SOLIDO_AFTER, TABR_SOLIDO_TOTEM)")
	}
	totem, err := strconv.ParseInt(tid, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	open := func(src string) *save.Save {
		f, err := os.Open(src)
		if err != nil {
			t.Skipf("the save is not readable here: %v", err)
		}
		defer f.Close()
		dst := filepath.Join(t.TempDir(), "copy.sav")
		g, err := os.Create(dst)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(g, f); err != nil {
			t.Fatal(err)
		}
		g.Close()
		_ = testsave.HostileText // the package's init makes the process check report no game
		s, err := save.Open(dst)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	}
	before, after := open(bs), open(as)

	ent, actors, buildings := baseParts(t, before, totem)
	src, err := loadSolidoSource(before, totem, ent, actors, buildings)
	if err != nil {
		t.Fatal(err)
	}
	mine, err := convertSolido(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("base %d: %d pieces and %d placeables in the base; the Solido made here holds %d pieces, %d placeables, %d pentashields (left out: %d pieces, %d doors, %d totems)",
		totem, len(src.Pieces), len(src.Placeables), len(mine.Instances), len(mine.Placeables), len(mine.Pentas), mine.SkippedPieces, mine.SkippedDoors, mine.SkippedTotems)

	const tol = 0.01
	angle := func(a, b float64) float64 { d := math.Mod(a-b, 360); return math.Abs(math.Remainder(d, 360)) }

	// pieces
	gi, err := after.Query(`select instance_id id, building_type t, transform_x x, transform_y y, transform_z z, transform_yaw yaw, provides_stability s, health h, hologram g from building_blueprint_instances order by instance_id`)
	if err != nil {
		t.Fatal(err)
	}
	if len(gi) != len(mine.Instances) {
		t.Fatalf("the game wrote %d pieces and this makes %d", len(gi), len(mine.Instances))
	}
	used := make([]bool, len(mine.Instances))
	var worst float64
	byType := map[string][]int{}
	for k, m := range mine.Instances {
		byType[m.Type] = append(byType[m.Type], k)
	}
	for _, g := range gi {
		gt, gx, gy, gz, gyaw := g["t"].(string), g["x"].(float64), g["y"].(float64), g["z"].(float64), g["yaw"].(float64)
		best, bestErr := -1, math.Inf(1)
		for _, k := range byType[gt] {
			if used[k] {
				continue
			}
			m := mine.Instances[k]
			e := math.Max(math.Max(math.Abs(m.X-gx), math.Abs(m.Y-gy)), math.Max(math.Abs(m.Z-gz), angle(m.Yaw, gyaw)))
			if e < bestErr {
				best, bestErr = k, e
			}
		}
		if best < 0 || bestErr > tol {
			t.Fatalf("the game's piece %v (%s at %.3f %.3f %.3f, yaw %.3f) has no piece here within %v (nearest off by %v)", g["id"], gt, gx, gy, gz, gyaw, tol, bestErr)
		}
		used[best] = true
		worst = math.Max(worst, bestErr)
		m := mine.Instances[best]
		if s, _ := toInt(g["s"]); s != m.ProvidesStability {
			t.Errorf("piece %v (%s): provides_stability is %d in the game and %d here", g["id"], gt, s, m.ProvidesStability)
		}
		if fmt.Sprint(g["h"]) != "0" || fmt.Sprint(g["g"]) != "1" {
			t.Errorf("piece %v: the game has health %v and hologram %v; this writes 0 and 1", g["id"], g["h"], g["g"])
		}
	}
	t.Logf("pieces: %d of %d match; the largest difference in a transform is %.6f", len(gi), len(mine.Instances), worst)

	// placeables
	gp, err := after.Query(`select placeable_id id, building_type t, transform_x x, transform_y y, transform_z z, transform_yaw pitch, transform_pitch yaw, transform_roll roll, hologram g from building_blueprint_placeables order by placeable_id`)
	if err != nil {
		t.Fatal(err)
	}
	if len(gp) != len(mine.Placeables) {
		t.Fatalf("the game wrote %d placeables and this makes %d", len(gp), len(mine.Placeables))
	}
	usedP := make([]bool, len(mine.Placeables))
	byTypeP := map[string][]int{}
	for k, m := range mine.Placeables {
		byTypeP[m.Type] = append(byTypeP[m.Type], k)
	}
	mineOf := map[int64]int{} // the game's placeable number -> index here
	worst = 0
	for _, g := range gp {
		gt, gx, gy, gz := g["t"].(string), g["x"].(float64), g["y"].(float64), g["z"].(float64)
		gyaw, gpitch, groll := g["yaw"].(float64), g["pitch"].(float64), g["roll"].(float64)
		best, bestErr := -1, math.Inf(1)
		for _, k := range byTypeP[gt] {
			if usedP[k] {
				continue
			}
			m := mine.Placeables[k]
			e := math.Max(math.Max(math.Abs(m.X-gx), math.Abs(m.Y-gy)), math.Max(math.Abs(m.Z-gz), angle(m.Yaw, gyaw)))
			e = math.Max(e, math.Max(math.Abs(m.Pitch-gpitch), math.Abs(m.Roll-groll)))
			if e < bestErr {
				best, bestErr = k, e
			}
		}
		if best < 0 || bestErr > tol {
			t.Fatalf("the game's placeable %v (%s at %.3f %.3f %.3f) has no placeable here within %v (nearest off by %v)", g["id"], gt, gx, gy, gz, tol, bestErr)
		}
		usedP[best] = true
		id, _ := toInt(g["id"])
		mineOf[id] = best
		worst = math.Max(worst, bestErr)
		if fmt.Sprint(g["g"]) != "1" {
			t.Errorf("placeable %v: the game has hologram %v; this writes 1", g["id"], g["g"])
		}
	}
	t.Logf("placeables: %d of %d match; the largest difference in a transform or angle is %.6f", len(gp), len(mine.Placeables), worst)

	// pentashields: the scale of each, through the placeable it belongs to
	gs, err := after.Query(`select placeable_id id, scale_x x, scale_y y, scale_z z from building_blueprint_pentashields order by placeable_id`)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != len(mine.Pentas) {
		t.Fatalf("the game wrote %d pentashields and this makes %d", len(gs), len(mine.Pentas))
	}
	scaleOf := map[int64][3]int64{}
	for _, s := range mine.Pentas {
		scaleOf[s.Placeable] = [3]int64{s.X, s.Y, s.Z}
	}
	for _, g := range gs {
		id, _ := toInt(g["id"])
		k, ok := mineOf[id]
		if !ok {
			t.Fatalf("the game's pentashield %d names a placeable that was not matched", id)
		}
		x, _ := toInt(g["x"])
		y, _ := toInt(g["y"])
		z, _ := toInt(g["z"])
		if got := scaleOf[mine.Placeables[k].ID]; got != [3]int64{x, y, z} {
			t.Errorf("the pentashield %d: scale %v in the game, %v here", id, [3]int64{x, y, z}, got)
		}
	}
	t.Logf("pentashields: %d of %d have the same scale", len(gs), len(mine.Pentas))

	// the numbers: both are 0..n-1 with no gap, here and in the game
	ids := func(rows []save.Row) []int64 {
		var out []int64
		for _, r := range rows {
			v, _ := toInt(r["id"])
			out = append(out, v)
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	for name, got := range map[string][]int64{"pieces": ids(gi), "placeables": ids(gp)} {
		for i, v := range got {
			if v != int64(i) {
				t.Fatalf("the game's %s are not numbered 0..n-1 with no gap (%d at %d)", name, v, i)
			}
		}
	}
	// the blueprint row and the item, as the game wrote them
	bp, _ := after.One(`select count(*) n, min(id) id, min(item_id) item, count(player_id) owner, min(building_blueprint_map) m from building_blueprints`)
	if bp["n"] != int64(1) || bp["owner"] != int64(0) || bp["m"] != "" {
		t.Errorf("the game's blueprint row: %v", bp)
	}
	item, _ := after.One(`select stats from items where id=?`, bp["item"])
	want, err := withBlueprintStats(solidoBlankStats, bp["id"].(int64))
	if err != nil {
		t.Fatal(err)
	}
	if item["stats"] != want {
		t.Errorf("the device's stats:\n game %v\n here %v", item["stats"], want)
	}
}

// baseParts finds the entity, the actors and the building actors of a standing base the way the recycle does.
func baseParts(t *testing.T, q baseQ, totem int64) (int64, []int64, []int64) {
	t.Helper()
	e, err := q.One(`select entity_id from actor_fgl_entities where actor_id=? and slot_name='Actor' order by entity_id limit 1`, totem)
	if err != nil || e == nil {
		t.Fatalf("the totem has no entity: %v", err)
	}
	ent, _ := toInt(e["entity_id"])
	a, err := q.Query(`select ? id union select id from placeables where owner_entity_id=?`, totem, ent)
	if err != nil {
		t.Fatal(err)
	}
	b, err := q.Query(`select distinct building_id id from building_instances where owner_entity_id=?`, ent)
	if err != nil {
		t.Fatal(err)
	}
	return ent, rowsInt64(a, "id"), rowsInt64(b, "id")
}
