package ops

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Where a vehicle goes (#480). The fixture is the one of the Create a vehicle tests: the character at (100, 200, 300) on Hagga Basin, so the old default point is (900, 200) and 450 up.

type testPiece struct {
	typ     string
	x, y, z float64
	yawDeg  float64
}

// piecesSQL makes building actor bid with the pieces.
func piecesSQL(bid int64, ps []testPiece) string {
	var b strings.Builder
	fmt.Fprintf(&b, "insert into actors(id,class,map,location_x,location_y,location_z) values (%d,'/Game/Dune/Systems/Building/Pieces/BP_DuneBuildingBase.BP_DuneBuildingBase_C','HaggaBasin',0,0,0);\ninsert into buildings(id) values (%d);\n", bid, bid)
	for i, p := range ps {
		h := p.yawDeg * math.Pi / 360
		fmt.Fprintf(&b, "insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_z,rotation_w) values (%d,%d,'%s',%v,%v,%v,%v,%v);\n",
			bid, i+1, p.typ, p.x, p.y, p.z, math.Sin(h), math.Cos(h))
	}
	return b.String()
}

func placeOps(t *testing.T, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, spawnWorldSQL()+"\n"+extra)}
}

// longWall is a wall along y at x, from y0 to y1, 512 a piece.
func longWall(x, y0, y1, z float64) []testPiece {
	var ps []testPiece
	for y := y0; y <= y1; y += 512 {
		ps = append(ps, testPiece{"Sardaukar_Wall_01", x, y, z, 90})
	}
	return ps
}

// wallDistance is how far (x, y) is from the long wall of longWall(900, -2900, 3100, ..): the thickness is 80 and the pieces are 512 long.
func wallDistance(x, y float64) float64 {
	ey := math.Max(math.Max(-2900-256-y, y-(3100+256)), 0)
	return math.Hypot(math.Max(math.Abs(x-900)-40, 0), ey)
}

func xyz(t *testing.T, o *Ops, id int64) (float64, float64, float64) {
	t.Helper()
	a := viewOf(t, o, id).actor
	return a["x"].(float64), a["y"].(float64), a["z"].(float64)
}

func placeOf(r map[string]any) map[string]any { return r["place"].(map[string]any) }

func TestPlaceOutdoorsWithNothingNearIsTheOldPoint(t *testing.T) {
	o := placeOps(t, "")
	id, r := spawn(t, o, Args{"class": "OrnithopterTransport", "template": "T0"})
	x, y, z := xyz(t, o, id)
	if x != 900 || y != 200 || z != 450 {
		t.Errorf("outdoors the vehicle goes where it always did, whatever its size: %v %v %v", x, y, z)
	}
	p := placeOf(r)
	if p["words"] != "next to you" || p["moved"] != false || p["inBase"] != false || p["nextToCharacter"] != true || len(p["blocked"].([]string)) != 0 {
		t.Errorf("the answer: %v", p)
	}
}

func TestPlaceAWallAtTheOldPointMovesTheVehicleAway(t *testing.T) {
	o := placeOps(t, piecesSQL(7001, longWall(900, -2900, 3100, 300)))
	prev, err := o.SpawnVehiclePreview(Args{"class": "Buggy", "template": "T0"})
	if err != nil {
		t.Fatal(err)
	}
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	x, y, z := xyz(t, o, id)
	if d := wallDistance(x, y); d < 500 {
		t.Errorf("the buggy is %v from the wall at (%v, %v); it needs its radius of 400 and a margin of 100", d, x, y)
	}
	if z != 450 {
		t.Errorf("outdoors the height stays the character's feet and 150: %v", z)
	}
	p := placeOf(r)
	if p["moved"] != true || p["inBase"] != false || !strings.Contains(p["words"].(string), "of you (base pieces at the first spot)") {
		t.Errorf("the answer says what it moved away from: %v", p)
	}
	if pp := placeOf(prev.(map[string]any)); pp["x"] != p["x"] || pp["y"] != p["y"] || pp["z"] != p["z"] || pp["words"] != p["words"] {
		t.Errorf("the preview names the place the create takes: %v vs %v", pp, p)
	}
}

func TestPlaceAFloorAtOneAndAHalfMetresUpIsNotDrivenInto(t *testing.T) {
	// a floor slab 150 above the character's feet, at the old point: the vehicle's body would be in it
	o := placeOps(t, piecesSQL(7001, []testPiece{{"Sardaukar_Floor", 900, 200, 450, 0}}))
	id, r := spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
	x, y, z := xyz(t, o, id)
	if math.Abs(x-900) < 256+350 && math.Abs(y-200) < 256+350 {
		t.Errorf("the sandbike is under or on the floor: %v %v", x, y)
	}
	if z != 450 || placeOf(r)["moved"] != true {
		t.Errorf("height %v, answer %v", z, placeOf(r))
	}
}

func TestPlaceAClosedRingOfWallsIsNotAPlaceToPutAVehicle(t *testing.T) {
	// the character stands in a ring of walls 15 m across with no way out and no roof: the points inside have room, but the vehicle would be shut in
	var ps []testPiece
	for i := 0; i < 24; i++ {
		a := 2 * math.Pi * float64(i) / 24
		ps = append(ps, testPiece{"Sardaukar_Wall_01", 100 + 1500*math.Cos(a), 200 + 1500*math.Sin(a), 300, a*180/math.Pi + 90})
	}
	o := placeOps(t, piecesSQL(7001, ps))
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	x, y, _ := xyz(t, o, id)
	if d := math.Hypot(x-100, y-200); d < 1500+500 {
		t.Errorf("the buggy is %v from the character: inside the ring or against it", d)
	}
	if w := placeOf(r)["words"].(string); !strings.Contains(w, "base pieces at the first spot") {
		t.Errorf("words: %s", w)
	}
	// with a gap in the ring there is a way out and the old point is fine
	o = placeOps(t, piecesSQL(7001, ps[:12]))
	id, r = spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	if x, y, _ = xyz(t, o, id); x != 900 || y != 200 {
		t.Errorf("a ring with a gap of half its length: %v %v (%v)", x, y, placeOf(r))
	}
}

func TestPlaceBiggerVehiclesKeepMoreClearance(t *testing.T) {
	var got []float64
	var dist []float64
	for _, key := range []string{"Sandbike", "Buggy", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler"} {
		o := placeOps(t, piecesSQL(7001, longWall(900, -2900, 3100, 300)))
		tpl := "T0"
		id, _ := spawn(t, o, Args{"class": key, "template": tpl})
		x, y, _ := xyz(t, o, id)
		sz := vehicleSizes[key]
		d := wallDistance(x, y)
		if d < sz.Radius+placeMargin {
			t.Errorf("%s: %v from the wall, needs %v", key, d, sz.Radius+placeMargin)
		}
		got = append(got, d)
		dist = append(dist, math.Hypot(x-100, y-200))
	}
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Errorf("a bigger vehicle keeps more clearance: %v", got)
		}
	}
	if got[1] >= got[2] || got[2] >= got[3] || got[3] >= got[4] {
		t.Errorf("from the buggy up each class keeps more: %v", got)
	}
	if got[0] >= 1000 || got[4] < 2300 || dist[4] > placeMaxRadius {
		t.Errorf("the sandbike keeps close (%v) and the crawler is beyond its radius (%v), within the search (%v)", got[0], got[4], dist[4])
	}
}

func TestPlaceAnotherVehicleAtTheOldPointIsAvoided(t *testing.T) {
	o := placeOps(t, "")
	first, _ := spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
	second, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	fx, fy, _ := xyz(t, o, first)
	sx, sy, sz := xyz(t, o, second)
	if d := math.Hypot(fx-sx, fy-sy); d < 250+400+placeMargin {
		t.Errorf("the buggy is %v from the sandbike", d)
	}
	if sz != 450 || !strings.Contains(placeOf(r)["words"].(string), "another vehicle at the first spot") {
		t.Errorf("z %v, answer %v", sz, placeOf(r))
	}
	// a vehicle that is stored is not in the world
	o = placeOps(t, `insert into actors(id,class,map,location_x,location_y,location_z,state) values (590,'/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',900,200,450,2);
		insert into vehicles(id) values (590);`)
	id, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	if x, y, _ := xyz(t, o, id); x != 900 || y != 200 {
		t.Errorf("a stored vehicle keeps only the place it was last in: %v %v", x, y)
	}
}

// A character on a small base (3 x 3 cells about (1500, 0)) that stands in a land claim: the vehicle goes to the ground outside the base, at the foundations' level, and outside the claim when that is near.
func smallBase() []testPiece {
	var ps []testPiece
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			ps = append(ps, testPiece{"Sardaukar_Foundation", 1500 + float64(i)*512, float64(j) * 512, 0, 0})
			ps = append(ps, testPiece{"Sardaukar_Floor", 1500 + float64(i)*512, float64(j) * 512, 768, 0})
		}
	}
	return ps
}

const claimTotem = `insert into actors(id,class,map,location_x,location_y,location_z) values (7002,'/Game/Dune/Systems/Building/Pieces/BP_TotemSmall.BP_TotemSmall_C','HaggaBasin',0,0,0);
insert into totems(id,landclaim_vertical_level,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (7002,0,0.0,0.0,0.0,0.0);
`

func TestPlaceFromInsideABaseGoesToTheGroundOutsideTheClaim(t *testing.T) {
	// the character is on the foundations (z 300 is on a foundation 0..384 high), near the east edge of the claim, which is one cell, 5,120 across, round the origin
	o := placeOps(t, piecesSQL(7001, smallBase())+claimTotem+"update actors set location_x=2100, location_y=200 where id=2;")
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	x, y, z := xyz(t, o, id)
	p := placeOf(r)
	if z != 250 {
		t.Errorf("the ground is the foundations' bottom (0) and a drop of 250: %v", z)
	}
	if math.Abs(x) < 2560+500 && math.Abs(y) < 2560+500 {
		t.Errorf("the vehicle is inside the claim (or on its edge): %v %v", x, y)
	}
	if p["inBase"] != true || p["claim"] != "outside" || !strings.Contains(p["words"].(string), "outside your base, ") || !strings.Contains(p["words"].(string), "outside the land claim") {
		t.Errorf("the answer: %v", p)
	}
	if d := math.Hypot(x-2100, y-200); d > placeClaimReach+placeStep {
		t.Errorf("preferring the outside of the claim stops at 30 m: %v", d)
	}
}

func TestPlaceWhenTheClaimIsTooBigTheVehicleStaysInItButOutsideTheBase(t *testing.T) {
	var cells strings.Builder
	for i := -5; i <= 5; i++ {
		for j := -5; j <= 5; j++ {
			if i != 0 || j != 0 {
				fmt.Fprintf(&cells, "insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (7002,%d,%d);\n", i, j)
			}
		}
	}
	o := placeOps(t, piecesSQL(7001, smallBase())+claimTotem+cells.String()+"update actors set location_x=2100, location_y=200 where id=2;")
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	x, y, z := xyz(t, o, id)
	p := placeOf(r)
	// the base is 3 x 3 cells of 512 about (1500, 0); the buggy is outside it by its radius and margin
	if d := math.Hypot(math.Max(math.Abs(x-1500)-768, 0), math.Max(math.Abs(y)-768, 0)); d < 500 || z != 250 {
		t.Errorf("%v %v %v: %v from the base", x, y, z, d)
	}
	if p["claim"] != "inside" || !strings.Contains(p["words"].(string), "still inside the land claim") {
		t.Errorf("the answer says the claim could not be left: %v", p)
	}
}

func TestPlaceWithNoClearPointIsRefusedAndNothingIsWritten(t *testing.T) {
	// foundations everywhere within 95 m
	var ps []testPiece
	for x := -9500.0; x <= 9700; x += 512 {
		for y := -9300.0; y <= 9900; y += 512 {
			ps = append(ps, testPiece{"Sardaukar_Foundation", x, y, 0, 0})
		}
	}
	o := placeOps(t, piecesSQL(7001, ps))
	before, _ := o.S.One(`select count(*) n, max(id) m from actors`)
	for _, call := range []func(Args) (any, error){o.SpawnVehicle, o.SpawnVehiclePreview} {
		_, err := call(Args{"class": "Sandbike", "template": "T0"})
		if err == nil || !errors.Is(err, apierr.ErrBadArgument) || !strings.Contains(err.Error(), "no clear spot for a sandbike within 80 m") || !strings.Contains(err.Error(), "give a place") {
			t.Fatalf("want a refusal that says so: %v", err)
		}
	}
	if _, err := o.BringVehicle(Args{"id": 578.0}); err == nil || !strings.Contains(err.Error(), "no clear spot") {
		t.Errorf("bring: %v", err)
	}
	if _, err := o.DuplicateVehicle(Args{"id": 578.0}); err == nil || !strings.Contains(err.Error(), "no clear spot") {
		t.Errorf("duplicate: %v", err)
	}
	after, _ := o.S.One(`select count(*) n, max(id) m from actors`)
	if before["n"] != after["n"] || before["m"] != after["m"] {
		t.Errorf("nothing was written: %v -> %v", before, after)
	}
	// a place given is used as it is, with a note
	id, r := spawn(t, o, Args{"class": "Sandbike", "template": "T0", "place": map[string]any{"map": "HaggaBasin", "x": 0.0, "y": 0.0, "z": 100.0}})
	if x, y, z := xyz(t, o, id); x != 0 || y != 0 || z != 100 {
		t.Errorf("the place given: %v %v %v", x, y, z)
	}
	if notes, _ := r["notes"].([]string); len(notes) != 1 || !strings.Contains(notes[0], "put there as asked") || placeOf(r)["words"] != "where you asked" || placeOf(r)["nextToCharacter"] != false {
		t.Errorf("the note: %v %v", r["notes"], placeOf(r))
	}
}

func TestPlaceAGivenPlaceInTheOpenHasNoNote(t *testing.T) {
	o := placeOps(t, piecesSQL(7001, longWall(900, -2900, 3100, 300)))
	_, r := spawn(t, o, Args{"class": "Sandbike", "template": "T0", "place": map[string]any{"map": "HaggaBasin", "x": -3000.0, "y": 0.0, "z": 100.0}})
	if notes, _ := r["notes"].([]string); len(notes) != 0 {
		t.Errorf("notes: %v", notes)
	}
}

func TestPlaceBringVehicleAndDuplicateUseTheSameSearch(t *testing.T) {
	// outdoors, nothing near: as before
	o := placeOps(t, "")
	r, err := o.BringVehicle(Args{"id": 578.0})
	if err != nil {
		t.Fatal(err)
	}
	if x, y, z := xyz(t, o, 578); x != 900 || y != 200 || z != 450 || placeOf(r.(map[string]any))["words"] != "next to you" {
		t.Errorf("bring outdoors: %v %v %v %v", x, y, z, r)
	}
	// the copy of a vehicle that stands at the old point is put beside it, not on it
	d, err := o.DuplicateVehicle(Args{"id": 578.0})
	if err != nil {
		t.Fatal(err)
	}
	cx, cy, cz := xyz(t, o, d.(map[string]any)["id"].(int64))
	if dd := math.Hypot(cx-900, cy-200); dd < 1200+placeMargin+1200 || cz != 450 {
		t.Errorf("the copy is %v from the original, at %v", dd, cz)
	}
	if w := placeOf(d.(map[string]any))["words"].(string); !strings.Contains(w, "another vehicle at the first spot") {
		t.Errorf("words: %s", w)
	}
	// a wall at the old point: the medium ornithopter keeps its radius from it, and is not counted as in its own way
	o = placeOps(t, piecesSQL(7001, longWall(900, -2900, 3100, 300)))
	r, err = o.BringVehicle(Args{"id": 578.0})
	if err != nil {
		t.Fatal(err)
	}
	x, y, _ := xyz(t, o, 578)
	if wd := wallDistance(x, y); wd < 1200+placeMargin {
		t.Errorf("brought %v from the wall", wd)
	}
	if w := placeOf(r.(map[string]any))["words"].(string); !strings.Contains(w, "base pieces at the first spot") {
		t.Errorf("words: %s", w)
	}
	// bring the vehicle again from where it stands: it is not its own obstacle
	if _, err = o.BringVehicle(Args{"id": 578.0}); err != nil {
		t.Fatal(err)
	}
	if x2, y2, _ := xyz(t, o, 578); x2 != x || y2 != y {
		t.Errorf("bringing it again moves it to the same place: %v %v -> %v %v", x, y, x2, y2)
	}
	d, err = o.DuplicateVehicle(Args{"id": 578.0})
	if err != nil {
		t.Fatal(err)
	}
	cx, cy, _ = xyz(t, o, d.(map[string]any)["id"].(int64))
	if wd := wallDistance(cx, cy); wd < 1200+placeMargin || math.Hypot(cx-x, cy-y) < 1200+placeMargin+1200 {
		t.Errorf("the copy is %v from the wall and %v from the original", wd, math.Hypot(cx-x, cy-y))
	}
}

// ---- the operator's base

func operatorBaseOps(t *testing.T) *Ops {
	t.Helper()
	var b strings.Builder
	b.WriteString(`insert into actors(id,class,map,location_x,location_y,location_z) values (7001,'/Game/Dune/Systems/Building/Pieces/BP_DuneBuildingBase.BP_DuneBuildingBase_C','HaggaBasin',180036.51,-36637.14,8165.93);
insert into buildings(id) values (7001);
insert into actors(id,class,map,location_x,location_y,location_z) values (7002,'/Game/Dune/Systems/Building/Pieces/BP_TotemSmall.BP_TotemSmall_C','HaggaBasin',180160.77,-35163.03,8552.27);
insert into totems(id,landclaim_vertical_level,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (7002,0,179799.65625,-36676.69140625,9321.4833984375,-140.0);
insert into actors(id,class,map,location_x,location_y,location_z,rotation_z,rotation_w) values (129,'/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',180762.18,-36755.99,8648.34,0.644,0.765);
insert into vehicles(id) values (129);
`)
	n := 0
	for _, ln := range strings.Split(strings.TrimSpace(operatorBasePieces), "\n") {
		f := strings.Fields(ln)
		n++
		fmt.Fprintf(&b, "insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_z,rotation_w) values (7001,%d,'%s',%s,%s,%s,%s,%s);\n", n, f[0], f[1], f[2], f[3], f[4], f[5])
	}
	n = 0
	for _, ln := range strings.Split(strings.TrimSpace(operatorBasePlaceables), "\n") {
		f := strings.Fields(ln)
		fmt.Fprintf(&b, "insert into actors(id,class,map,location_x,location_y,location_z) values (%d,'/Game/Dune/Systems/Building/Placeables/BP_%s.BP_%s_C','HaggaBasin',%s,%s,%s);\ninsert into placeables(id,building_type,is_hologram) values (%d,'%s',0);\n", 7100+n, f[0], f[0], f[1], f[2], f[3], 7100+n, f[0])
		n++
	}
	fmt.Fprintf(&b, "update actors set location_x=%v, location_y=%v, location_z=%v, rotation_z=-0.9731622855552838, rotation_w=0.2301198947783881 where id=2;\n", operatorCharX, operatorCharY, operatorCharZ)
	return placeOps(t, b.String())
}

const (
	operatorCharX, operatorCharY, operatorCharZ = 179628.34, -36141.70, 8554.42
	operatorFoundationZ                         = 8165.93
	operatorGridX, operatorGridY                = 180036.51, -36637.14 // the middle foundation; the grid is turned 40 degrees about it
	operatorHalfBase                            = 1280.0               // 5 cells of 512 across
)

// gridUV is a point in the base's own frame: along the foundation rows and across them, in centimetres from the middle foundation.
func gridUV(x, y float64) (float64, float64) {
	s, c := math.Sincos(40 * math.Pi / 180)
	dx, dy := x-operatorGridX, y-operatorGridY
	return dx*c + dy*s, -dx*s + dy*c
}

// baseGap is how far (x, y) is from the 5 x 5 block of foundations (0 on it).
func baseGap(x, y float64) float64 {
	u, v := gridUV(x, y)
	return math.Hypot(math.Max(math.Abs(u)-operatorHalfBase, 0), math.Max(math.Abs(v)-operatorHalfBase, 0))
}

func TestPlaceOperatorBaseTheOldPointWasInsideTheBase(t *testing.T) {
	o := operatorBaseOps(t)
	// the evidence: the old point (character + 800 along x, + 150 up) is in the ground-floor room of the 5 x 5 base, under two floors, and no vehicle fits there as a way out
	ox, oy, oz := operatorCharX+800, operatorCharY, operatorCharZ+150
	if u, v := gridUV(ox, oy); math.Abs(u) > operatorHalfBase || math.Abs(v) > operatorHalfBase {
		t.Fatalf("the old point is outside the base: %v %v", u, v)
	}
	w, err := loadPlaceWorld(o.S.Query, "HaggaBasin", 0, operatorCharX, operatorCharY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if in, _ := w.onPieces(operatorCharX, operatorCharY, operatorCharZ); !in {
		t.Fatalf("the character is on the base")
	}
	for _, key := range []string{"Sandbike", "Buggy", "OrnithopterTransport"} {
		if bl := w.blocked(ox, oy, oz, vehicleSizes[key]); len(bl) == 0 {
			t.Errorf("%s: the old point is not refused", key)
		}
	}
}

func TestPlaceOperatorBaseEveryClassGoesOutsideAtTheFoundationLevel(t *testing.T) {
	var gaps []float64
	w, err := loadPlaceWorld(operatorBaseOps(t).S.Query, "HaggaBasin", 0, operatorCharX, operatorCharY, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler"} {
		o := operatorBaseOps(t)
		id, r := spawn(t, o, Args{"class": key, "template": "T0"})
		x, y, z := xyz(t, o, id)
		sz := vehicleSizes[key]
		gap := baseGap(x, y)
		if gap < sz.Radius+placeMargin {
			t.Errorf("%s: %v from the base, needs %v", key, gap, sz.Radius+placeMargin)
		}
		if math.Abs(z-(operatorFoundationZ+placeGroundDrop)) > 0.01 {
			t.Errorf("%s: z %v, want the foundation level %v and the drop", key, z, operatorFoundationZ+placeGroundDrop)
		}
		if d := math.Hypot(x-operatorCharX, y-operatorCharY); d > placeMaxRadius {
			t.Errorf("%s: %v from the character", key, d)
		}
		p := placeOf(r)
		if p["inBase"] != true || p["moved"] != true || !strings.HasPrefix(p["words"].(string), "outside your base, ") || !strings.Contains(p["words"].(string), "base pieces") || !strings.Contains(p["words"].(string), "at the first spot)") {
			t.Errorf("%s: %v", key, p)
		}
		// the claim is one cell of 5,120 about the totem's origin: the small ones get out of it within 30 m, the big ones cannot and stay just outside the base
		inClaim := false
		cl := w.characterClaims(x, y)
		for _, c := range cl {
			inClaim = inClaim || !outsideClaims([]placeClaim{c}, x, y, sz.Radius+placeMargin)
		}
		if (key == "Sandbike" || key == "Buggy") && (p["claim"] != "outside" || inClaim) {
			t.Errorf("%s: %v", key, p)
		}
		if (key == "OrnithopterMedium" || key == "OrnithopterTransport" || key == "Sandcrawler") && p["claim"] != "inside" {
			t.Errorf("%s: %v", key, p)
		}
	}
	// without the claim each class is as near the base as its size lets it be, and a bigger one is no nearer
	for _, key := range []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler"} {
		o := operatorBaseOps(t)
		testsave.Exec(t, o.S, `delete from totems`)
		id, _ := spawn(t, o, Args{"class": key, "template": "T0"})
		x, y, _ := xyz(t, o, id)
		gaps = append(gaps, baseGap(x, y))
	}
	for i := 1; i < len(gaps); i++ {
		if gaps[i] < gaps[i-1] {
			t.Errorf("a bigger vehicle is not nearer the base: %v", gaps)
		}
	}
	if gaps[0] > 1000 {
		t.Errorf("a sandbike goes just outside the walls: %v", gaps[0])
	}
}

func TestPlaceOperatorBaseBringAndDuplicate(t *testing.T) {
	o := operatorBaseOps(t)
	if _, err := o.BringVehicle(Args{"id": 578.0}); err != nil {
		t.Fatal(err)
	}
	x, y, z := xyz(t, o, 578)
	if baseGap(x, y) < 1200+placeMargin || math.Abs(z-(operatorFoundationZ+placeGroundDrop)) > 0.01 {
		t.Errorf("brought to %v %v %v: %v from the base", x, y, z, baseGap(x, y))
	}
	d, err := o.DuplicateVehicle(Args{"id": 578.0})
	if err != nil {
		t.Fatal(err)
	}
	cx, cy, cz := xyz(t, o, d.(map[string]any)["id"].(int64))
	if baseGap(cx, cy) < 1200+placeMargin || math.Abs(cz-(operatorFoundationZ+placeGroundDrop)) > 0.01 || math.Hypot(cx-x, cy-y) < 1200+1200 {
		t.Errorf("the copy at %v %v %v: %v from the base, %v from the original", cx, cy, cz, baseGap(cx, cy), math.Hypot(cx-x, cy-y))
	}
}

func TestPlaceOnTheSecondFloorTheVehicleStillGoesToTheGround(t *testing.T) {
	o := operatorBaseOps(t)
	// the character on the second floor (floor top 9317.93 + 4), at the middle of the base
	testsave.Exec(t, o.S, `update actors set location_x=180036.51, location_y=-36637.14, location_z=9322 where id=2`)
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T6_Combat"})
	x, y, z := xyz(t, o, id)
	if baseGap(x, y) < 500 || math.Abs(z-(operatorFoundationZ+placeGroundDrop)) > 0.01 || placeOf(r)["inBase"] != true {
		t.Errorf("%v %v %v %v", x, y, z, placeOf(r))
	}
}

func TestPlaceTheSearchIsFast(t *testing.T) {
	o := operatorBaseOps(t)
	w, err := loadPlaceWorld(o.S.Query, "HaggaBasin", 0, operatorCharX, operatorCharY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.solids) < 200 || len(w.claims) != 1 {
		t.Fatalf("solids %d, claims %d", len(w.solids), len(w.claims))
	}
	// the worst case the base gives: the biggest vehicle, from the middle of the base
	for i := 0; i < 20; i++ {
		if _, err := w.choose(180036.51, -36637.14, 9322, vehicleSizes["Sandcrawler"], "sandcrawler"); err != nil {
			t.Fatal(err)
		}
	}
}

// ---- the model

func TestPlaceShapesOfTheKindsOfPiece(t *testing.T) {
	cases := []struct {
		typ         string
		hx, hy      float64
		below, high float64
		foundation  bool
	}{
		{"Sardaukar_Foundation", 256, 256, 0, 384, true},
		{"Sardaukar_Foundation_Wedge", 256, 256, 0, 384, true},
		{"Sardaukar_Floor_02", 256, 256, 100, 30, false},
		{"Sardaukar_Rooftop_01", 256, 256, 100, 30, false},
		{"Sardaukar_Wall_01", 256, 40, 0, 384, false},
		{"Sardaukar_Window_L", 256, 40, 0, 384, false},
		{"Sardaukar_Garage_Door_Frame", 256, 40, 0, 384, false},
		{"Sardaukar_Wall_Round_Corner", 256, 256, 0, 384, false},
		{"Sardaukar_Wall_Triangle_Top_Left", 256, 256, 0, 384, false},
		{"Sardaukar_Column_corner", 60, 60, 0, 384, false},
		{"Sardaukar_Stairs_Half", 256, 256, 0, 384, false},
		{"Sardaukar_Ramp_Corner", 256, 256, 0, 384, false},
		{"Something_Never_Seen", 256, 256, 0, 384, false},
	}
	for _, c := range cases {
		hx, hy, below, high, f := shapeOf(c.typ)
		if hx != c.hx || hy != c.hy || below != c.below || high != c.high || f != c.foundation {
			t.Errorf("%s: %v %v %v %v %v", c.typ, hx, hy, below, high, f)
		}
	}
	// every kind of piece of the operator's base is modelled
	for _, ln := range strings.Split(strings.TrimSpace(operatorBasePieces), "\n") {
		if hx, _, _, _, _ := shapeOf(strings.Fields(ln)[0]); hx <= 0 {
			t.Errorf("%s", ln)
		}
	}
}

func TestPlaceBearingWordsFollowTheMap(t *testing.T) {
	// east is +x and south is +y, as the land claim editor draws them
	for _, c := range []struct {
		dx, dy float64
		want   string
	}{{1, 0, "east"}, {0, 1, "south"}, {-1, 0, "west"}, {0, -1, "north"}, {1, 1, "south-east"}, {-1, -1, "north-west"}} {
		if got := bearingWord(c.dx, c.dy); got != c.want {
			t.Errorf("%v,%v: %s, want %s", c.dx, c.dy, got, c.want)
		}
	}
}

func TestPlaceEveryClassHasASize(t *testing.T) {
	for _, key := range []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler", "ContainerVehicle"} {
		if s, ok := vehicleSizes[key]; !ok || s.Radius <= 0 || s.Height <= 0 {
			t.Errorf("%s: %v", key, s)
		}
	}
	if s := sizeOfActorClass("/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C"); s != vehicleSizes["Buggy"] {
		t.Errorf("a buggy: %v", s)
	}
	if s := sizeOfActorClass("/Game/Other.Other_C"); s != unknownVehicleSize {
		t.Errorf("an unknown class: %v", s)
	}
	// the order the design note gives
	order := []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler"}
	for i := 1; i < len(order); i++ {
		if vehicleSizes[order[i]].Radius <= vehicleSizes[order[i-1]].Radius {
			t.Errorf("%s is not larger than %s", order[i], order[i-1])
		}
	}
}

func TestPlaceABaseWithNoFoundationUsesItsLowestPieceAsTheGround(t *testing.T) {
	// a platform of floors with no foundation under it: the ground is taken from the lowest piece (a slab reaches 100 below its z), and the character is still "in a base"
	var ps []testPiece
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			ps = append(ps, testPiece{"Sardaukar_Floor", float64(i) * 512, float64(j) * 512, 50, 0})
		}
	}
	o := placeOps(t, piecesSQL(7001, ps)+"update actors set location_z=60 where id=2;")
	id, r := spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
	x, y, z := xyz(t, o, id)
	if z != -50+placeGroundDrop || placeOf(r)["inBase"] != true || math.Hypot(math.Max(math.Abs(x)-768, 0), math.Max(math.Abs(y)-768, 0)) < 350 {
		t.Errorf("%v %v %v %v", x, y, z, placeOf(r))
	}
}
