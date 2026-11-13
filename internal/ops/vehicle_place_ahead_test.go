package ops

import (
	"math"
	"testing"
)

// A ground vehicle for a character outdoors goes 10 m ahead along the character's facing, at the character's feet and 150 up: the console's own distance for a ground vehicle (#480).
// The fixture is that of vehicle_place_test.go.

func TestPlaceGroundVehicleIsTenMetresAheadWhateverTheFacing(t *testing.T) {
	words := map[float64]string{0: "10 m east of you", 90: "10 m south of you", 180: "10 m west of you", 270: "10 m north of you", 45: "10 m south-east of you"}
	for _, deg := range []float64{0, 45, 90, 180, 270, -30} {
		for _, key := range groundKeys {
			o := placeOps(t, turnTo(deg))
			id, r := spawn(t, o, Args{"class": key, "template": "T0"})
			x, y, z := xyz(t, o, id)
			along, side := ahead(x, y, 100, 200, deg)
			if math.Abs(along-1000) > 0.001 || math.Abs(side) > 0.001 || z != 450 {
				t.Errorf("%s facing %v: %v ahead and %v to the side, height %v", key, deg, along, side, z)
			}
			p := placeOf(r)
			if w, ok := words[deg]; ok && p["words"] != w {
				t.Errorf("%s facing %v: %v", key, deg, p["words"])
			}
			if p["moved"] != false || p["distance"] != 1000.0 || p["inAir"] != false || p["up"] != 150.0 {
				t.Errorf("%s facing %v: %v", key, deg, p)
			}
		}
	}
}

func TestPlaceGroundVehicleWithNoUsableFacingKeepsTheOldPoint(t *testing.T) {
	o := placeOps(t, "update actors set rotation_x=0, rotation_y=0, rotation_z=0, rotation_w=0 where id=2;")
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	if x, y, z := xyz(t, o, id); x != 900 || y != 200 || z != 450 || placeOf(r)["words"] != "8 m east of you" {
		t.Errorf("no saved rotation: %v %v %v %v", x, y, z, placeOf(r))
	}
}

func TestPlaceGroundVehicleAheadIsSearchedFromAheadWhenSomethingIsThere(t *testing.T) {
	// facing east, a wall 10 m ahead: the vehicle is moved, keeps the buggy's radius and a margin from it, and the answer says what was at the first spot
	o := placeOps(t, turnTo(0)+piecesSQL(7001, longWall(1100, -2900, 3100, 300)))
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	x, y, _ := xyz(t, o, id)
	ey := math.Max(math.Max(-2900-256-y, y-(3100+256)), 0)
	if d := math.Hypot(math.Max(math.Abs(x-1100)-40, 0), ey); d < 500 {
		t.Errorf("the buggy is %v from the wall at (%v, %v)", d, x, y)
	}
	if p := placeOf(r); p["moved"] != true {
		t.Errorf("the answer: %v", p)
	}
}

func TestPlaceBringVehicleOfAGroundVehicleFollowsTheFacing(t *testing.T) {
	o := placeOps(t, turnTo(180)+ownBuggy579())
	r, err := o.BringVehicle(Args{"id": 579.0})
	if err != nil {
		t.Fatal(err)
	}
	row, err := o.S.One(`select location_x x, location_y y, location_z z from actors where id=579`)
	if err != nil {
		t.Fatal(err)
	}
	x, y, z := row["x"].(float64), row["y"].(float64), row["z"].(float64)
	if math.Abs(x-(100-1000)) > 1e-6 || math.Abs(y-200) > 1e-6 || z != 450 || placeOf(r.(map[string]any))["words"] != "10 m west of you" {
		t.Errorf("brought: %v %v %v %v", x, y, z, r)
	}
}
