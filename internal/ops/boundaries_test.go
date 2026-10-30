package ops

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// QA-11: values that tests used to compute with the code under test, and limits that were never tried at their edge. Here the
// expected numbers are written out.

func TestXPTableAnchors(t *testing.T) {
	if len(xpByLevel) != 201 {
		t.Fatalf("the table has %d entries, the highest level is meant to be 200", len(xpByLevel))
	}
	for lvl, want := range map[int]int64{0: 0, 1: 40, 2: 215, 3: 440, 4: 740, 5: 1240, 10: 4190, 100: 58190, 150: 93099, 199: 331314, 200: 344440} {
		if xpByLevel[lvl] != want {
			t.Errorf("xpByLevel[%d] = %d, want %d", lvl, xpByLevel[lvl], want)
		}
	}
	for i := 1; i < len(xpByLevel); i++ {
		if xpByLevel[i] <= xpByLevel[i-1] {
			t.Fatalf("level %d needs %d XP, not more than level %d (%d)", i, xpByLevel[i], i-1, xpByLevel[i-1])
		}
	}
}

func TestXPToLevelAtEveryEdge(t *testing.T) {
	for _, c := range []struct {
		xp    int64
		level int
	}{
		{-100, 0}, {0, 0}, {1, 0}, {39, 0}, {40, 1}, {41, 1}, {214, 1}, {215, 2}, {439, 2}, {440, 3},
		{58189, 99}, {58190, 100}, {344439, 199}, {344440, 200}, {344441, 200}, {1 << 40, 200},
	} {
		if got := xpToLevel(c.xp); got != c.level {
			t.Errorf("xpToLevel(%d) = %d, want %d", c.xp, got, c.level)
		}
	}
}

// The ids are written out by hand: x in bits 0-20, y in 21-41, z in 42-62, each two's complement. They do not come from packField.
func TestDecodeFieldPositionGoldenIds(t *testing.T) {
	for _, c := range []struct {
		id      int64
		x, y, z int64
	}{
		{0, 0, 0, 0},
		{1, 1, 0, 0},
		{2097152, 0, 1, 0},
		{4398046511104, 0, 0, 1},
		{2097151, -1, 0, 0},
		{4398044413952, 0, -1, 0},
		{9223367638808264704, 0, 0, -1},
		{1048575, 1048575, 0, 0},  // the largest positive coordinate
		{1048576, -1048576, 0, 0}, // the next bit pattern wraps to the most negative one
		{396234508054280, -413944, 195656, 90},
		{7617665411863667, 283763, 118663, 1732},
	} {
		x, y, z := decodeFieldPosition(c.id)
		if x != c.x || y != c.y || z != c.z {
			t.Errorf("id %d decoded as %d %d %d, want %d %d %d", c.id, x, y, z, c.x, c.y, c.z)
		}
	}
}

// Spice sizes by what is left (over 150,000 Large, over 5,000 Medium, else Small), flour sand at exactly 60,000, and the map's
// own bounds are inclusive at the integer edge. The fields are placed at distinct spots so each is told apart by x.
func TestResourceFieldThresholdsAndMapBounds(t *testing.T) {
	type fld struct {
		x, left int64
		kind    string
		size    string
	}
	want := []fld{
		{1, 0, "spice", "Small"}, {2, 4999, "spice", "Small"}, {3, 5000, "spice", "Small"}, {4, 5001, "spice", "Medium"},
		{5, 59999, "spice", "Medium"}, {6, 60000, "flour", ""}, {7, 60001, "spice", "Medium"},
		{8, 149999, "spice", "Medium"}, {9, 150000, "spice", "Medium"}, {10, 150001, "spice", "Large"}, {11, 1 << 30, "spice", "Large"},
	}
	var rows []string
	for _, f := range want {
		rows = append(rows, fmt.Sprintf("(%d,'HaggaBasin',0,1.0,%d)", packField(f.x, 0, 0), f.left))
	}
	// Hagga Basin spans x -456752.21 .. 354547.46 and y -450630.14 .. 353821.95: the integer coordinates just inside are shown, just outside are not
	inside := [][2]int64{{-456752, 0}, {354547, 0}, {0, -450630}, {0, 353821}}
	outside := [][2]int64{{-456753, 0}, {354548, 0}, {0, -450631}, {0, 353822}}
	for _, c := range append(append([][2]int64{}, inside...), outside...) {
		rows = append(rows, fmt.Sprintf("(%d,'HaggaBasin',0,1.0,100)", packField(c[0], c[1], 0)))
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into resourcefield_state(field_id,map,dimension_index,spawn_time,value_remaining) values `+strings.Join(rows, ",")+`;`)}
	r, err := o.LiveMap("HaggaBasin")
	if err != nil {
		t.Fatal(err)
	}
	got := map[[2]int64]map[string]any{}
	for _, f := range r.(map[string]any)["resourceFields"].([]map[string]any) {
		got[[2]int64{f["x"].(int64), f["y"].(int64)}] = f
	}
	for _, f := range want {
		g := got[[2]int64{f.x, 0}]
		if g == nil || g["kind"] != f.kind || g["size"] != f.size {
			t.Errorf("a field with %d left: want %s %q, got %v", f.left, f.kind, f.size, g)
		}
	}
	for _, c := range inside {
		if got[c] == nil {
			t.Errorf("%v is inside the map and must be shown", c)
		}
	}
	for _, c := range outside {
		if got[c] != nil {
			t.Errorf("%v is outside the map and must be dropped", c)
		}
	}
	if len(got) != len(want)+len(inside) {
		t.Errorf("%d fields shown, want %d", len(got), len(want)+len(inside))
	}
}

func TestTeleportLimitsAtTheEdge(t *testing.T) {
	o := newPlayerOps(t)
	for _, c := range []struct {
		name string
		a    Args
		ok   bool
	}{
		{"x at the limit", Args{"x": 1e7, "y": 0.0, "z": 0.0}, true},
		{"x just past the limit", Args{"x": 1e7 + 1, "y": 0.0, "z": 0.0}, false},
		{"y at the negative limit", Args{"x": 0.0, "y": -1e7, "z": 0.0}, true},
		{"y just past the negative limit", Args{"x": 0.0, "y": -1e7 - 1, "z": 0.0}, false},
		{"z just past the limit", Args{"x": 0.0, "y": 0.0, "z": 1e7 + 1}, false},
		{"facing at the limit", Args{"x": 0.0, "y": 0.0, "z": 0.0, "yaw": 3600.0}, true},
		{"facing just past the limit", Args{"x": 0.0, "y": 0.0, "z": 0.0, "yaw": 3601.0}, false},
		{"facing at the negative limit", Args{"x": 0.0, "y": 0.0, "z": 0.0, "yaw": -3600.0}, true},
		{"facing just past the negative limit", Args{"x": 0.0, "y": 0.0, "z": 0.0, "yaw": -3601.0}, false},
		{"infinite z", Args{"x": 0.0, "y": 0.0, "z": math.Inf(-1)}, false},
	} {
		_, err := o.Teleport(c.a)
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestDestinationListIsCappedAt200AndSaysHowManyThereAre(t *testing.T) {
	// markers a player has discovered, nearest first; the list is capped but the total is the real count
	for _, n := range []int{200, 201} {
		sql := fmt.Sprintf(`with recursive n(i) as (select 1 union all select i+1 from n where i<%d)
			insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) select i,0,'HaggaBasin','TradingPost',100.0+i,200.0,0.0 from n;
			with recursive n(i) as (select 1 union all select i+1 from n where i<%d)
			insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) select 1,i,0,'HaggaBasin',3,1 from n;`, n, n)
		o := &Ops{S: testsave.PlayerWithSQL(t, sql)}
		res, err := o.Destinations("", "")
		if err != nil {
			t.Fatal(err)
		}
		m := res.(map[string]any)
		d := m["destinations"].([]Destination)
		if m["total"].(int) != n+2 || len(d) != 200 { // the 2 more are the two hubs, which every map offers (#521)
			t.Errorf("%d markers: total %v, listed %d, want total %d and 200 listed", n, m["total"], len(d), n+2)
		}
		if d[0].X != 101 || d[199].X != 300 {
			t.Errorf("%d markers: nearest first, got x %v ... %v", n, d[0].X, d[199].X)
		}
	}
}

const claimEdgeFixture = `
insert into actors(id,class,map) values (601,'/Game/Dune/Systems/Building/Pieces/BP_Totem.BP_Totem_C','HaggaBasin');
insert into totems(id,landclaim_vertical_level,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (601,1,0.0,0.0,0.0,0.0);`

func edgeCells(xs, ys []int) []any {
	var out []any
	for _, y := range ys {
		for _, x := range xs {
			out = append(out, map[string]any{"x": float64(x), "y": float64(y)})
		}
	}
	return out
}

func rangeInts(from, to int) []int {
	var out []int
	for i := from; i <= to; i++ {
		out = append(out, i)
	}
	return out
}

func TestLandClaimLimitsAtTheEdge(t *testing.T) {
	cellsN := func(o *Ops) int64 {
		return mustOne(t, o, `select count(*) n from landclaim_segments where totem_id=601`)["n"].(int64)
	}
	// a chain along the x axis reaches the farthest cell the editor offers (128) and not one beyond it
	o := &Ops{S: testsave.PlayerWithSQL(t, claimEdgeFixture)}
	if _, err := o.ApplyLandClaim(Args{"totem_id": 601.0, "cells": edgeCells(rangeInts(1, 129), []int{0})}); err == nil {
		t.Fatal("a cell 129 away must be refused")
	}
	if cellsN(o) != 0 {
		t.Fatal("a refused edit must change nothing")
	}
	if _, err := o.ApplyLandClaim(Args{"totem_id": 601.0, "cells": edgeCells(rangeInts(1, 128), []int{0})}); err != nil {
		t.Fatalf("a chain out to 128 is allowed: %v", err)
	}
	if cellsN(o) != 128 {
		t.Fatalf("%d cells stored", cellsN(o))
	}
	// the negative edge
	o = &Ops{S: testsave.PlayerWithSQL(t, claimEdgeFixture)}
	if _, err := o.ApplyLandClaim(Args{"totem_id": 601.0, "cells": edgeCells([]int{0}, rangeInts(-128, -1))}); err != nil {
		t.Fatalf("a chain out to -128 is allowed: %v", err)
	}
	// 500 cells in one edit are allowed, 501 are not
	o = &Ops{S: testsave.PlayerWithSQL(t, claimEdgeFixture)}
	five00 := edgeCells(rangeInts(1, 125), rangeInts(0, 3))
	if len(five00) != 500 {
		t.Fatalf("test setup: %d cells", len(five00))
	}
	if _, err := o.ApplyLandClaim(Args{"totem_id": 601.0, "cells": append(append([]any{}, five00...), edgeCells([]int{126}, []int{0})...)}); err == nil || !strings.Contains(err.Error(), "at most 500") {
		t.Fatalf("501 cells must be refused for their number, got %v", err)
	}
	if _, err := o.ApplyLandClaim(Args{"totem_id": 601.0, "cells": five00}); err != nil {
		t.Fatalf("500 cells are allowed: %v", err)
	}
	if cellsN(o) != 500 {
		t.Fatalf("%d cells stored", cellsN(o))
	}
	// rings 1..5 and levels 0..5
	for _, c := range []struct {
		a  Args
		ok bool
	}{
		{Args{"totem_id": 601.0, "rings": 0.0}, false}, {Args{"totem_id": 601.0, "rings": 1.0}, true},
		{Args{"totem_id": 601.0, "rings": 5.0, "level": 5.0}, true}, {Args{"totem_id": 601.0, "rings": 6.0}, false},
		{Args{"totem_id": 601.0, "level": 6.0}, false},
	} {
		o := &Ops{S: testsave.PlayerWithSQL(t, claimEdgeFixture)}
		if _, err := o.ExpandLandClaim(c.a); (err == nil) != c.ok {
			t.Errorf("%v: err=%v, want ok=%v", c.a, err, c.ok)
		}
	}
	o = &Ops{S: testsave.PlayerWithSQL(t, claimEdgeFixture)}
	if _, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "rings": 5.0}); err != nil {
		t.Fatal(err)
	}
	if n := cellsN(o); n != 120 { // 11 x 11 = 121 cells, the totem's own cell is not stored
		t.Fatalf("5 rings is a 11 x 11 square: %d stored cells, want 120", n)
	}
}
