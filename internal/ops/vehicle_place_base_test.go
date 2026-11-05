package ops

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// A synthetic base for the placement tests (#480, #540): invented coordinates, built by the code below, not copied from any save. It has the shapes the placement search has to
// cope with: a 5 x 5 block of foundations (512 apart) on a grid turned 30 degrees, a ring of walls, windows and door frames in two storeys, a full floor at the top of each storey,
// a rooftop piece, stairs inside, ramps on the outside, and placeables on both floors. The foundations stand at synthFoundationZ, a wall course is 384, so the first course is at +384,
// the second at +768, the first floor at +1152 and the roof floor at +1920 (a storey is 768).
//
// The grid is described in its own frame (u along the foundation rows, v across them, in centimetres from the middle foundation), and synthXY turns that into the map's x and y.

const (
	synthGridX, synthGridY = 40000.0, 25000.0 // the middle foundation
	synthGridTurnDeg       = 30.0             // the grid is turned this much about it
	synthFoundationZ       = 2000.0
	synthHalfBase          = 1280.0 // 5 cells of 512 across, half of it

	// the character stands in the ground-floor room, a few centimetres above the foundation top, facing out of the base along the grid's own -u
	synthCharU, synthCharV = -300.0, 450.0
	synthCharZ             = synthFoundationZ + 388.0
	synthFaceDeg           = 210.0 // the character's yaw: the grid's -u (the grid is turned 30)
)

// synthXY is the map position of the point (u, v) of the base's own frame.
func synthXY(u, v float64) (float64, float64) {
	s, c := math.Sincos(synthGridTurnDeg * math.Pi / 180)
	return synthGridX + u*c - v*s, synthGridY + u*s + v*c
}

var synthCharX, synthCharY = synthXY(synthCharU, synthCharV)

// synthPiece is a piece of the synthetic base in the base's own frame.
type synthPiece struct {
	typ     string
	u, v, z float64
	turn    float64 // degrees about the grid's own axes
}

// synthPlaceable is a placeable of the synthetic base; level is 0 on the ground floor and 1 on the first floor.
type synthPlaceable struct {
	typ   string
	u, v  float64
	level int
}

// synthBasePieces builds the pieces: foundations, the wall ring of two storeys, the floors, a rooftop, stairs and ramps.
func synthBasePieces() []synthPiece {
	var ps []synthPiece
	cells := []float64{-1024, -512, 0, 512, 1024}
	for _, u := range cells {
		for _, v := range cells {
			ps = append(ps, synthPiece{"Sardaukar_Foundation", u, v, synthFoundationZ, 0})
		}
	}
	// the wall ring: 20 edge positions, two courses to a storey, the course types vary so every kind of wall piece is in it
	type edge struct{ u, v, turn float64 }
	var ring []edge
	for _, c := range cells {
		ring = append(ring, edge{c, -synthHalfBase, 0}, edge{c, synthHalfBase, 0}, edge{-synthHalfBase, c, 90}, edge{synthHalfBase, c, 90})
	}
	for i, e := range ring {
		for _, z := range []float64{384, 768, 1152, 1536} {
			typ := "Sardaukar_Window"
			switch {
			case z == 384 && i == 3:
				typ = "Sardaukar_Door_Frame_Tall"
			case z == 384 && i == 6:
				typ = "Sardaukar_Garage_Door_Frame"
			case (i+int(z/384))%3 == 0:
				typ = "Sardaukar_Wall_01"
			case (i+int(z/384))%3 == 1:
				typ = "Sardaukar_Window_L"
			}
			ps = append(ps, synthPiece{typ, e.u, e.v, synthFoundationZ + z, e.turn})
		}
	}
	// a full floor at the top of each storey, of both floor kinds
	for _, z := range []float64{1152, 1920} {
		for i, u := range cells {
			for j, v := range cells {
				typ := "Sardaukar_Floor"
				if (i+j)%4 == 0 {
					typ = "Sardaukar_Floor_02"
				}
				ps = append(ps, synthPiece{typ, u, v, synthFoundationZ + z, 0})
			}
		}
	}
	ps = append(ps, synthPiece{"Sardaukar_Rooftop_01", 0, 0, synthFoundationZ + 2688, 0})
	// stairs inside, at the ground floor
	ps = append(ps, synthPiece{"Sardaukar_Stairs", -1024, -1024, synthFoundationZ + 384, 90}, synthPiece{"Sardaukar_Stairs", -1024, -512, synthFoundationZ + 384, 90})
	// ramps on the outside, along one side and turning the corner
	for _, u := range []float64{-512, 0, 512} {
		ps = append(ps, synthPiece{"Sardaukar_Ramp", u, 1536, synthFoundationZ, 0})
	}
	ps = append(ps, synthPiece{"Sardaukar_Ramp_Corner", 1024, 1536, synthFoundationZ, 0})
	return ps
}

// synthBasePlaceables builds the placeables: storage, water, power and refining things on both floors.
func synthBasePlaceables() []synthPlaceable {
	var ps []synthPlaceable
	for k := 0; k < 9; k++ {
		ps = append(ps, synthPlaceable{"WaterCistern_Placeable", 1000, -1000 + 250*float64(k), 0}, synthPlaceable{"WaterCistern_Placeable", 760, -1000 + 250*float64(k), 0})
	}
	for k := 0; k < 6; k++ {
		ps = append(ps, synthPlaceable{"Generator_Placeable", -1000 + 400*float64(k), -700, 1}, synthPlaceable{"Generator_Placeable", -1000 + 400*float64(k), 700, 1})
	}
	ps = append(ps,
		synthPlaceable{"BloodWaterExtractor_Placeable", -600, 1050, 0}, synthPlaceable{"BloodWaterExtractor_Placeable", -300, 1050, 0},
		synthPlaceable{"Developer_StorageContainer_Placeable", -900, -200, 0}, synthPlaceable{"Developer_StorageContainer_Placeable", -900, 200, 1},
		synthPlaceable{"Fabricator_Placeable", 300, -1100, 0}, synthPlaceable{"Recycler_Placeable", 600, -1100, 0},
		synthPlaceable{"SpiceSilo_Placeable", 0, -1100, 0}, synthPlaceable{"SpiceSilo_Placeable", 100, 1100, 0}, synthPlaceable{"SpiceSilo_Placeable", -300, 1100, 1},
		synthPlaceable{"SmallChemicalRefinery_Placeable", 400, 0, 1}, synthPlaceable{"SmallOreRefinery_Placeable", 0, 0, 1}, synthPlaceable{"SmallOreRefinery_Placeable", -300, 0, 1},
		// at the two door frames of the ring
		synthPlaceable{"MTX_Sardaukar_GarageDoor_Placeable", -1280, -512, 0}, synthPlaceable{"MTX_Sardaukar_TallDoor_Placeable", 1280, -1024, 0},
	)
	return ps
}

const (
	synthBuildingID  = 7001
	synthTotemID     = 7002
	synthBikeID      = 129
	synthPlaceableID = 7100 // the first placeable's actor id
)

// synthBaseOps is a save with the synthetic base, a land claim around it, a sandbike inside it and the character in the ground-floor room.
func synthBaseOps(t *testing.T) *Ops {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, `insert into actors(id,class,map,location_x,location_y,location_z) values (%d,'/Game/Dune/Systems/Building/Pieces/BP_DuneBuildingBase.BP_DuneBuildingBase_C','HaggaBasin',%v,%v,%v);
insert into buildings(id) values (%d);
`, synthBuildingID, synthGridX, synthGridY, synthFoundationZ, synthBuildingID)
	// the totem stands in a corner of the ground floor; its claim is one cell of 5,120 about an origin near the middle of the base, turned with the grid
	tx, ty := synthXY(900, -950)
	ox, oy := synthGridX-250, synthGridY-50
	fmt.Fprintf(&b, `insert into actors(id,class,map,location_x,location_y,location_z) values (%d,'/Game/Dune/Systems/Building/Pieces/BP_TotemSmall.BP_TotemSmall_C','HaggaBasin',%v,%v,%v);
insert into totems(id,landclaim_vertical_level,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (%d,0,%v,%v,%v,%v);
`, synthTotemID, tx, ty, synthFoundationZ+386, synthTotemID, ox, oy, synthFoundationZ+1155, synthGridTurnDeg-180)
	bx, by := synthXY(460, -520)
	sh, sc := math.Sincos(synthGridTurnDeg * math.Pi / 360)
	fmt.Fprintf(&b, `insert into actors(id,class,map,location_x,location_y,location_z,rotation_z,rotation_w) values (%d,'/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',%v,%v,%v,%v,%v);
insert into vehicles(id) values (%d);
`, synthBikeID, bx, by, synthFoundationZ+480, sh, sc, synthBikeID)
	for i, p := range synthBasePieces() {
		x, y := synthXY(p.u, p.v)
		h := (synthGridTurnDeg + p.turn) * math.Pi / 360
		fmt.Fprintf(&b, "insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_z,rotation_w) values (%d,%d,'%s',%v,%v,%v,%v,%v);\n",
			synthBuildingID, i+1, p.typ, x, y, p.z, math.Sin(h), math.Cos(h))
	}
	for i, p := range synthBasePlaceables() {
		x, y := synthXY(p.u, p.v)
		z := synthFoundationZ + 386 + 769*float64(p.level)
		fmt.Fprintf(&b, "insert into actors(id,class,map,location_x,location_y,location_z) values (%d,'/Game/Dune/Systems/Building/Placeables/BP_%s.BP_%s_C','HaggaBasin',%v,%v,%v);\ninsert into placeables(id,building_type,is_hologram) values (%d,'%s',0);\n",
			synthPlaceableID+i, p.typ, p.typ, x, y, z, synthPlaceableID+i, p.typ)
	}
	// the character, facing along the grid's -u
	fh, fc := math.Sincos(synthFaceDeg * math.Pi / 360)
	fmt.Fprintf(&b, "update actors set location_x=%v, location_y=%v, location_z=%v, rotation_z=%v, rotation_w=%v where id=2;\n", synthCharX, synthCharY, synthCharZ, fh, fc)
	return placeOps(t, b.String())
}

// gridUV is a point in the base's own frame: along the foundation rows and across them, in centimetres from the middle foundation.
func gridUV(x, y float64) (float64, float64) {
	s, c := math.Sincos(synthGridTurnDeg * math.Pi / 180)
	dx, dy := x-synthGridX, y-synthGridY
	return dx*c + dy*s, -dx*s + dy*c
}

// baseGap is how far (x, y) is from the 5 x 5 block of foundations (0 on it).
func baseGap(x, y float64) float64 {
	u, v := gridUV(x, y)
	return math.Hypot(math.Max(math.Abs(u)-synthHalfBase, 0), math.Max(math.Abs(v)-synthHalfBase, 0))
}

func TestSynthBaseIsAsBuilt(t *testing.T) {
	ps, pl := synthBasePieces(), synthBasePlaceables()
	kinds := map[string]bool{}
	for _, p := range ps {
		kinds[p.typ] = true
	}
	for _, k := range []string{"Sardaukar_Foundation", "Sardaukar_Wall_01", "Sardaukar_Window", "Sardaukar_Window_L", "Sardaukar_Door_Frame_Tall", "Sardaukar_Garage_Door_Frame",
		"Sardaukar_Floor", "Sardaukar_Floor_02", "Sardaukar_Rooftop_01", "Sardaukar_Stairs", "Sardaukar_Ramp", "Sardaukar_Ramp_Corner"} {
		if !kinds[k] {
			t.Errorf("no %s in the synthetic base", k)
		}
	}
	if len(ps)+len(pl) < 200 {
		t.Errorf("the synthetic base is too small to be a fair test of the search: %d pieces, %d placeables", len(ps), len(pl))
	}
	x, y := synthXY(300, -700)
	if u, v := gridUV(x, y); math.Abs(u-300) > 1e-6 || math.Abs(v+700) > 1e-6 {
		t.Errorf("the frame does not round-trip: %v %v", u, v)
	}
}

// The reach of what keeps a vehicle out, on a hand-made world of single solids (no save): a piece a long way above a vehicle's top is no obstacle and one close above is; a margin is
// kept around a wall; a placeable blocks at the vehicle's own height.
func TestPlaceWhatReachesAVehicle(t *testing.T) {
	bike := vehicleSizes["Sandbike"]
	floor := func(z float64) *placeWorld {
		hx, hy, below, above, f := shapeOf("Sardaukar_Floor")
		return &placeWorld{solids: []solid{{cos: 1, hx: hx, hy: hy, z0: z - below, z1: z + above, kind: "piece", foundation: f}}}
	}
	if bl := floor(1000).blockers(0, 0, 300, bike); len(bl) != 1 || bl[0] != "pieces" {
		t.Errorf("a floor 7 m over a sandbike: %v", bl)
	}
	if bl := floor(5000).blockers(0, 0, 300, bike); len(bl) != 0 {
		t.Errorf("a floor 47 m over a sandbike: %v", bl)
	}
	hx, hy, below, above, _ := shapeOf("Sardaukar_Wall_01")
	wall := &placeWorld{solids: []solid{{cos: 1, hx: hx, hy: hy, z0: 500 - below, z1: 500 + above, kind: "piece"}}}
	reach := hy + bike.Radius + placeMargin // from the wall's middle line to the nearest centre that is clear
	if bl := wall.blockers(0, reach-5, 500, bike); len(bl) == 0 {
		t.Errorf("5 cm inside the margin of a wall is not refused")
	}
	if bl := wall.blockers(0, reach+5, 500, bike); len(bl) != 0 {
		t.Errorf("5 cm outside the margin of a wall is refused: %v", bl)
	}
	pl := &placeWorld{solids: []solid{{x: 0, y: 0, hx: 250, round: true, z0: 450, z1: 900, kind: "placeable"}}}
	if bl := pl.blockers(300, 0, 500, bike); len(bl) != 1 || bl[0] != "placeables" {
		t.Errorf("a placeable at the vehicle's height: %v", bl)
	}
	if bl := pl.blockers(250+bike.Radius+placeMargin+5, 0, 500, bike); len(bl) != 0 {
		t.Errorf("a placeable out of reach: %v", bl)
	}
}
