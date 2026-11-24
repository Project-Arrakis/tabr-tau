package ops

import (
	"math"
	"strings"
	"testing"
)

// In front of the character (#480): the first spot is on the character's facing, the half in front is searched before anything behind, several vehicles put down together stand 12 m apart
// along the facing, and the answer says "in front of you" with the map and the coordinates. The fixture is that of vehicle_place_test.go: the character at (100, 200, 300) on Hagga Basin.

func characterPose(t *testing.T, o *Ops) charPose {
	t.Helper()
	row, err := o.S.One(`select location_x x, location_y y, location_z z, rotation_x rx, rotation_y ry, rotation_z rz, rotation_w rw from actors where id=2`)
	if err != nil || row == nil {
		t.Fatal(err)
	}
	return poseOf(row)
}

func TestFrontEastNorthWestSouthGiveThePointOnTheRightAxis(t *testing.T) {
	// the map runs east = +x, south = +y; a ground vehicle is 1,000 ahead, an aircraft 3,000 ahead and 700 up
	for _, c := range []struct {
		deg    float64
		dx, dy float64
	}{{0, 1, 0}, {270, 0, -1}, {180, -1, 0}, {90, 0, 1}} {
		for _, k := range []struct {
			key  string
			dist float64
			up   float64
		}{{"Buggy", 1000, 150}, {"OrnithopterMedium", 3000, 700}} {
			o := placeOps(t, turnTo(c.deg))
			id, _ := spawn(t, o, Args{"class": k.key, "template": "T0"})
			x, y, z := xyz(t, o, id)
			if math.Abs(x-(100+c.dx*k.dist)) > 0.01 || math.Abs(y-(200+c.dy*k.dist)) > 0.01 || math.Abs(z-(300+k.up)) > 0.01 {
				t.Errorf("%s facing %v: (%v, %v, %v)", k.key, c.deg, x, y, z)
			}
		}
	}
}

func TestFrontWordsAndCoordinates(t *testing.T) {
	o := placeOps(t, turnTo(0))
	_, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	p := placeOf(r)
	if p["words"] != "10 m in front of you (east)" || p["coords"] != "HaggaBasin, x 1100, y 200, z 450" || p["map"] != "HaggaBasin" {
		t.Errorf("the answer: %v", p)
	}
	prev, err := o.SpawnVehiclePreview(Args{"class": "OrnithopterLight", "template": "T0"})
	if err != nil {
		t.Fatal(err)
	}
	if pp := placeOf(prev.(map[string]any)); pp["words"] != "30 m in front of you (east) and 7 m above you; it starts in the air and the game settles it" || pp["coords"] != "HaggaBasin, x 3100, y 200, z 1000" {
		t.Errorf("the preview: %v", pp)
	}
}

func TestFrontSideOfSaysFrontBehindAndSide(t *testing.T) {
	east := charPose{Fx: 1, HasFacing: true}
	for _, c := range []struct {
		dx, dy float64
		want   string
	}{{1000, 0, "front"}, {1000, 900, "front"}, {1000, 1100, ""}, {0, 1000, ""}, {-1000, 1100, ""}, {-1000, 900, "behind"}, {-1000, 0, "behind"}, {0, 0, ""}} {
		if got := sideOf(east, c.dx, c.dy); got != c.want {
			t.Errorf("(%v, %v): %q, want %q", c.dx, c.dy, got, c.want)
		}
	}
	if w := (placement{Dist: 2000, Bearing: "west", Side: "behind"}).Words(); w != "20 m behind you (west)" {
		t.Errorf("behind: %q", w)
	}
	if w := (placement{Dist: 2000, Bearing: "west"}).Words(); w != "20 m west of you" {
		t.Errorf("side: %q", w)
	}
}

func TestFrontBlockedFrontIsSearchedInFrontBeforeBehind(t *testing.T) {
	// facing east; a round obstacle 10 m ahead blocks the whole front half of the 10 m ring. Only the part behind is clear at 10 m, but the front half at 18 m is clear too: it is taken.
	w := &placeWorld{mapName: "HaggaBasin", solids: []solid{{x: 1100, y: 200, hx: 1300, round: true, z0: 0, z1: 5000, kind: "placeable"}}}
	pose := charPose{X: 100, Y: 200, Z: 300, Fx: 1, HasFacing: true}
	p, err := w.choose(pose, vehicleSizes["Buggy"], "Buggy")
	if err != nil {
		t.Fatal(err)
	}
	along := (p.X-100)*1 + (p.Y-200)*0
	if along < 0 || p.Side == "behind" || p.Moved == false || len(p.Blocked) != 1 || p.Blocked[0] != "placeables" {
		t.Errorf("not in front: %+v (along %v)", p, along)
	}
	if !strings.Contains(p.Words(), "of you (placeables at the first spot)") && !strings.Contains(p.Words(), "(things you built at the first spot)") {
		t.Errorf("words: %v", p.Words())
	}
}

func TestFrontWithTheFrontShutTheBehindIsTried(t *testing.T) {
	// a row of obstacles across the whole front half to 30 m past the first spot leaves only the half behind
	w := &placeWorld{mapName: "HaggaBasin"}
	for y := -9000.0; y <= 9000; y += 800 {
		for x := 100.0; x <= 4300; x += 800 {
			w.solids = append(w.solids, solid{x: x, y: 200 + y, hx: 600, round: true, z0: 0, z1: 5000, kind: "placeable"})
		}
	}
	pose := charPose{X: 100, Y: 200, Z: 300, Fx: 1, HasFacing: true}
	p, err := w.choose(pose, vehicleSizes["Sandbike"], "Sandbike")
	if err != nil {
		t.Fatal(err)
	}
	if p.X >= 100 {
		t.Errorf("only the half behind is open: %+v", p)
	}
}

func TestFrontSeveralVehiclesStandInALineTwelveMetresApartOrMoreWhenTheyAreBig(t *testing.T) {
	for _, deg := range []float64{0, 90, 180, 270} {
		for _, k := range []struct {
			key              string
			first, gap, rise float64
		}{{"Buggy", 1000, 1200, 150}, {"OrnithopterLight", 3000, 1800, 700}} { // a light ornithopter's footprints (radius 800 and a margin of 100 each) are wider than 12 m together
			o := placeOps(t, turnTo(deg))
			pose := characterPose(t, o)
			sz := vehicleSizes[k.key]
			ps, err := placeLine(o.S.Query, "HaggaBasin", 0, pose, []vehicleSize{sz, sz, sz}, 0, k.key)
			if err != nil || len(ps) != 3 {
				t.Fatal(err, ps)
			}
			for i, p := range ps {
				along, side := ahead(p.X, p.Y, 100, 200, deg)
				if math.Abs(along-(k.first+float64(i)*k.gap)) > 0.01 || math.Abs(side) > 0.01 || math.Abs(p.Z-(300+k.rise)) > 0.01 {
					t.Errorf("%s facing %v, vehicle %d: %v ahead, %v to the side, z %v", k.key, deg, i, along, side, p.Z)
				}
				if p.Side != "front" {
					t.Errorf("%s vehicle %d: %q", k.key, i, p.Side)
				}
			}
		}
	}
}

func TestFrontALineKeepsClearOfWhatIsInTheWay(t *testing.T) {
	// facing east, a wall across the second place of the line: that vehicle is moved clear of it, and the third still keeps clear of the second
	o := placeOps(t, turnTo(0)+piecesSQL(7001, longWall(2300, -2900, 3100, 300)))
	sz := vehicleSizes["Buggy"]
	ps, err := placeLine(o.S.Query, "HaggaBasin", 0, characterPose(t, o), []vehicleSize{sz, sz, sz}, 0, "Buggy")
	if err != nil || len(ps) != 3 {
		t.Fatal(err, ps)
	}
	if ps[0].X != 1100 || ps[0].Moved {
		t.Errorf("the first: %+v", ps[0])
	}
	if !ps[1].Moved {
		t.Errorf("the second stood at the wall: %+v", ps[1])
	}
	for i, p := range ps {
		for j := i + 1; j < len(ps); j++ {
			if d := math.Hypot(p.X-ps[j].X, p.Y-ps[j].Y); d < 2*(sz.Radius+placeMargin)-1 {
				t.Errorf("vehicles %d and %d are %v apart", i, j, d)
			}
		}
		if d := wallDistanceAt(p.X, p.Y, 2300); d < sz.Radius+placeMargin {
			t.Errorf("vehicle %d is %v from the wall", i, d)
		}
	}
}

// wallDistanceAt is how far (x, y) is from longWall(wx, -2900, 3100, ..).
func wallDistanceAt(x, y, wx float64) float64 {
	ey := math.Max(math.Max(-2900-256-y, y-(3100+256)), 0)
	return math.Hypot(math.Max(math.Abs(x-wx)-40, 0), ey)
}

func TestFrontLineWithNoFacingOrInABaseKeepsTheOldFirstSpot(t *testing.T) {
	// the line needs a facing: without one, every vehicle starts from the old point and the search spreads them (they do not overlap)
	o := placeOps(t, "update actors set rotation_x=0, rotation_y=0, rotation_z=0, rotation_w=0 where id=2;")
	pose := characterPose(t, o)
	sz := vehicleSizes["Buggy"]
	ps, err := placeLine(o.S.Query, "HaggaBasin", 0, pose, []vehicleSize{sz, sz}, 0, "Buggy")
	if err != nil || len(ps) != 2 {
		t.Fatal(err, ps)
	}
	if ps[0].X != 900 || ps[0].Y != 200 || ps[0].Side != "" {
		t.Errorf("the first: %+v", ps[0])
	}
	if d := math.Hypot(ps[0].X-ps[1].X, ps[0].Y-ps[1].Y); d < 2*(sz.Radius+placeMargin)-1 {
		t.Errorf("overlap: %v", d)
	}
}
