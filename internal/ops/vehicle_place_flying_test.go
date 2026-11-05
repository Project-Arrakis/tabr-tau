package ops

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Flying vehicles (#540). An aircraft is put at least 30 m ahead of the character, along the character's facing, and at least 7 m above the character's height; a ground vehicle is never
// put below the character's height unless the character stands in a base. The fixture is that of vehicle_place_test.go: the character (id 2) at (100, 200, 300) on Hagga Basin, facing
// south (+y), vehicle 578 a Medium ornithopter of the character's, vehicle 579 a buggy that is nobody's until a test gives it a rank.

var (
	flyingKeys = []string{"OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport"}
	groundKeys = []string{"Sandbike", "Buggy", "Sandcrawler"}
)

// turnTo sets the character's turn about the vertical axis, in degrees from east (+x), counter-clockwise on the page (+y is south).
func turnTo(deg float64) string {
	h := deg * math.Pi / 360
	return fmt.Sprintf("update actors set rotation_x=0, rotation_y=0, rotation_z=%v, rotation_w=%v where id=2;", math.Sin(h), math.Cos(h))
}

// ahead says how far (x, y) is from the character along the facing direction (deg) and how far to the side of it.
func ahead(x, y, cx, cy, deg float64) (along, side float64) {
	fx, fy := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
	dx, dy := x-cx, y-cy
	return dx*fx + dy*fy, -dx*fy + dy*fx
}

func ownBuggy579() string {
	return `insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (579,'##Buggy',2,3,0,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (579,1,1);`
}

func TestFlyingTheRuleIsTheConsolesNumbers(t *testing.T) {
	if flyingVehicleMinOffset != 3000 || flyingVehicleZOffset != 700 {
		t.Errorf("the console's published values are 3000 and 700 centimetres: %v %v", flyingVehicleMinOffset, flyingVehicleZOffset)
	}
}

func TestFlyingSizesTellAircraftApart(t *testing.T) {
	for k, s := range vehicleSizes {
		if s.Flies != strings.HasPrefix(k, "Ornithopter") {
			t.Errorf("%s: flies %v", k, s.Flies)
		}
	}
	for _, c := range []struct {
		class any
		flies bool
	}{
		{"/Game/Dune/Systems/Vehicles/Blueprints/FlyingVehicles/BP_MediumOrnithopter_CHOAM.BP_MediumOrnithopter_CHOAM_C", true},
		{"/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C", false},
		{"/Game/Other/BP_SomeOrnithopter_Variant.BP_SomeOrnithopter_Variant_C", true}, // not in the class table, named an ornithopter
		{"/Game/Other.Other_C", false},
		{nil, false},
	} {
		if got := sizeOfActorClass(c.class).Flies; got != c.flies {
			t.Errorf("%v: flies %v, want %v", c.class, got, c.flies)
		}
	}
	for _, key := range flyingKeys { // the same by the class key the request names
		if !sizeOfClassKey(key).Flies {
			t.Errorf("%s does not fly", key)
		}
	}
}

func TestFlyingFacingFollowsTheConsolesMaths(t *testing.T) {
	// a turn about the vertical axis only
	for _, deg := range []float64{0, 30, 90, 135, 180, -70, 250} {
		h := deg * math.Pi / 360
		fx, fy, ok := facingOf(0, 0, math.Sin(h), math.Cos(h))
		if !ok || math.Abs(fx-math.Cos(deg*math.Pi/180)) > 1e-9 || math.Abs(fy-math.Sin(deg*math.Pi/180)) > 1e-9 {
			t.Errorf("%v degrees: %v %v %v", deg, fx, fy, ok)
		}
	}
	// a turn that also leans: the facing is the rotated x axis, projected on the ground plane and normalised; check it against a rotation done the long way
	qx, qy, qz, qw := 0.1, -0.2, 0.5, 0.0
	n := math.Sqrt(qx*qx + qy*qy + qz*qz + 1)
	qx, qy, qz, qw = qx/n, qy/n, qz/n, 1/n
	// v' = v + 2w(q × v) + 2 q × (q × v), v = (1, 0, 0)
	cx1, cy1, cz1 := 0.0, qz, -qy // q × v
	vx := 1 + 2*qw*cx1 + 2*(qy*cz1-qz*cy1)
	vy := 0 + 2*qw*cy1 + 2*(qz*cx1-qx*cz1)
	l := math.Hypot(vx, vy)
	fx, fy, ok := facingOf(qx, qy, qz, qw)
	if !ok || math.Abs(fx-vx/l) > 1e-9 || math.Abs(fy-vy/l) > 1e-9 {
		t.Errorf("a leaning turn: %v %v %v, want %v %v", fx, fy, ok, vx/l, vy/l)
	}
	// unusable: no rotation saved, or one that points straight up
	for _, q := range [][4]float64{{0, 0, 0, 0}, {0, math.Sqrt2 / 2, 0, math.Sqrt2 / 2}} {
		if _, _, ok := facingOf(q[0], q[1], q[2], q[3]); ok {
			t.Errorf("%v has no facing on the ground", q)
		}
	}
}

func TestFlyingVehicleIsThirtyMetresAheadAndSevenUpWhateverTheFacing(t *testing.T) {
	for _, deg := range []float64{0, 45, 90, 135, 180, 270, -30} {
		for _, key := range flyingKeys {
			o := placeOps(t, turnTo(deg))
			id, r := spawn(t, o, Args{"class": key, "template": "T0"})
			x, y, z := xyz(t, o, id)
			along, side := ahead(x, y, 100, 200, deg)
			if along < flyingVehicleMinOffset-0.001 || math.Abs(side) > 0.001 {
				t.Errorf("%s facing %v: %v ahead and %v to the side (at %v %v)", key, deg, along, side, x, y)
			}
			if z < 300+flyingVehicleZOffset {
				t.Errorf("%s: z %v is not 7 m above the character's 300", key, z)
			}
			p := placeOf(r)
			if p["inAir"] != true || p["up"] != 700.0 || p["moved"] != false || p["distance"] != 3000.0 {
				t.Errorf("%s: the answer: %v", key, p)
			}
		}
	}
}

func TestFlyingVehicleIsAboveTheCharacterAtAnyHeight(t *testing.T) {
	for _, cz := range []float64{-20000, 0, 300, 52000} {
		o := placeOps(t, fmt.Sprintf("update actors set location_z=%v where id=2;", cz))
		id, _ := spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0"})
		if _, _, z := xyz(t, o, id); z < cz+flyingVehicleZOffset {
			t.Errorf("character at %v: aircraft at %v", cz, z)
		}
	}
}

func TestFlyingWithNoUsableFacingTheSearchStartsInItsUsualDirection(t *testing.T) {
	for _, set := range []string{"update actors set rotation_x=0, rotation_y=0, rotation_z=0, rotation_w=0 where id=2;", "update actors set rotation_x=0, rotation_y=0.7071067811865476, rotation_z=0, rotation_w=0.7071067811865476 where id=2;"} {
		o := placeOps(t, set)
		id, r := spawn(t, o, Args{"class": "OrnithopterMedium", "template": "T0"})
		x, y, z := xyz(t, o, id)
		if x != 3100 || y != 200 || z != 1000 {
			t.Errorf("the ring starts east, 30 m out: %v %v %v", x, y, z)
		}
		if w := placeOf(r)["words"]; w != "30 m east of you and 7 m above you; it starts in the air and the game settles it" {
			t.Errorf("words: %v", w)
		}
	}
}

func TestFlyingStillKeepsClearOfBasePiecesAndOtherVehicles(t *testing.T) {
	// the character faces south: a long wall across the point 30 m ahead
	var wall []testPiece
	for x := -2000.0; x <= 2200; x += 512 {
		wall = append(wall, testPiece{"Sardaukar_Wall_01", x, 3200, 300, 0})
	}
	o := placeOps(t, piecesSQL(7001, wall))
	id, r := spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0"})
	x, y, z := xyz(t, o, id)
	for _, w := range wall {
		if math.Hypot(math.Max(math.Abs(x-w.x)-256, 0), math.Max(math.Abs(y-w.y)-40, 0)) < 800+placeMargin {
			t.Fatalf("the aircraft at %v %v is against the wall piece at %v", x, y, w.x)
		}
	}
	if d := math.Hypot(x-100, y-200); d < flyingVehicleMinOffset || d > placeMaxRadius || z < 300+flyingVehicleZOffset {
		t.Errorf("%v from the character at height %v", d, z)
	}
	if p := placeOf(r); p["moved"] != true || !strings.Contains(p["words"].(string), "(base pieces at the first spot)") || !strings.HasSuffix(p["words"].(string), "; it starts in the air and the game settles it") {
		t.Errorf("the answer: %v", p)
	}
	// another vehicle at that point
	o = placeOps(t, `insert into actors(id,class,map,location_x,location_y,location_z) values (590,'/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',100,3200,1000);
		insert into vehicles(id) values (590);`)
	id, r = spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0"})
	x, y, _ = xyz(t, o, id)
	if math.Hypot(x-100, y-3200) < 400+800+placeMargin || !strings.Contains(placeOf(r)["words"].(string), "another vehicle at the first spot") {
		t.Errorf("%v %v %v", x, y, placeOf(r))
	}
}

func TestFlyingRoofCheckIsMadeAtTheAircraftsHeight(t *testing.T) {
	// a roof slab 20 m under the point 30 m ahead is no obstacle for an aircraft 7 m above the character; the same slab at the character's own height is
	o := placeOps(t, piecesSQL(7001, []testPiece{{"Sardaukar_Floor", 100, 3200, -1700, 0}}))
	id, r := spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0"})
	if x, y, z := xyz(t, o, id); math.Abs(x-100) > 0.001 || y != 3200 || z != 1000 || placeOf(r)["moved"] != false {
		t.Errorf("under the aircraft: %v %v %v %v", x, y, z, placeOf(r))
	}
	o = placeOps(t, piecesSQL(7001, []testPiece{{"Sardaukar_Floor", 100, 3200, 300, 0}}))
	id, r = spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0"})
	x, y, _ := xyz(t, o, id)
	if math.Abs(x-100) < 256+900 && math.Abs(y-3200) < 256+900 || placeOf(r)["moved"] != true {
		t.Errorf("in the aircraft's way: %v %v %v", x, y, placeOf(r))
	}
}

func TestFlyingWithNoClearSpotIsRefusedAsBefore(t *testing.T) {
	var ps []testPiece
	for x := -9500.0; x <= 9700; x += 512 {
		for y := -9300.0; y <= 9900; y += 512 {
			ps = append(ps, testPiece{"Sardaukar_Foundation", x, y, 0, 0})
		}
	}
	o := placeOps(t, piecesSQL(7001, ps))
	before, _ := o.S.One(`select count(*) n, max(id) m from actors`)
	for _, call := range []func(Args) (any, error){o.SpawnVehicle, o.SpawnVehiclePreview} {
		_, err := call(Args{"class": "OrnithopterLight", "template": "T0"})
		if err == nil || !errors.Is(err, apierr.ErrBadArgument) || !strings.Contains(err.Error(), "no clear spot for a light ornithopter within 80 m") || !strings.Contains(err.Error(), "give a place") {
			t.Fatalf("want a refusal that says so: %v", err)
		}
	}
	if _, err := o.BringVehicle(Args{"id": 578.0}); err == nil || !strings.Contains(err.Error(), "no clear spot") {
		t.Errorf("bring: %v", err)
	}
	if _, err := o.DuplicateVehicle(Args{"id": 578.0}); err == nil || !strings.Contains(err.Error(), "no clear spot") {
		t.Errorf("duplicate: %v", err)
	}
	if after, _ := o.S.One(`select count(*) n, max(id) m from actors`); before["n"] != after["n"] || before["m"] != after["m"] {
		t.Errorf("nothing was written: %v -> %v", before, after)
	}
}

func TestFlyingBringAndDuplicateFollowTheSameRule(t *testing.T) {
	for _, deg := range []float64{90, 200} {
		o := placeOps(t, turnTo(deg))
		r, err := o.BringVehicle(Args{"id": 578.0})
		if err != nil {
			t.Fatal(err)
		}
		x, y, z := xyz(t, o, 578)
		if along, side := ahead(x, y, 100, 200, deg); along < flyingVehicleMinOffset-0.001 || math.Abs(side) > 0.001 || z < 300+flyingVehicleZOffset {
			t.Errorf("bring, facing %v: %v ahead, %v aside, z %v", deg, along, side, z)
		}
		if p := placeOf(r.(map[string]any)); p["inAir"] != true || !strings.HasSuffix(p["words"].(string), "the game settles it") {
			t.Errorf("bring: %v", p)
		}
		d, err := o.DuplicateVehicle(Args{"id": 578.0})
		if err != nil {
			t.Fatal(err)
		}
		cx, cy, cz := xyz(t, o, d.(map[string]any)["id"].(int64))
		if math.Hypot(cx-100, cy-200) < flyingVehicleMinOffset-0.001 || cz < 300+flyingVehicleZOffset || math.Hypot(cx-x, cy-y) < 1200+placeMargin+1200 {
			t.Errorf("duplicate, facing %v: the copy at %v %v %v, the original at %v %v", deg, cx, cy, cz, x, y)
		}
		if p := placeOf(d.(map[string]any)); p["inAir"] != true || !strings.Contains(p["words"].(string), "m above you") || !strings.HasSuffix(p["words"].(string), "; it starts in the air and the game settles it") {
			t.Errorf("duplicate: %v", p)
		}
	}
}

func TestFlyingGroundVehiclesAreNeverBelowTheCharacter(t *testing.T) {
	for _, cz := range []float64{-20000, 0, 300, 52000} {
		for _, key := range groundKeys {
			o := placeOps(t, fmt.Sprintf("update actors set location_z=%v where id=2;", cz))
			id, r := spawn(t, o, Args{"class": key, "template": "T0"})
			if _, _, z := xyz(t, o, id); z < cz {
				t.Errorf("%s: character at %v, vehicle at %v", key, cz, z)
			}
			if placeOf(r)["inAir"] != false {
				t.Errorf("%s: %v", key, placeOf(r))
			}
		}
	}
	// Bring and Duplicate of a ground vehicle: the old point, beside the character, at the character's height and a little over
	o := placeOps(t, ownBuggy579())
	r, err := o.BringVehicle(Args{"id": 579.0})
	if err != nil {
		t.Fatal(err)
	}
	if row, err := o.S.One(`select location_x x, location_y y, location_z z from actors where id=579`); err != nil || row["x"] != 900.0 || row["y"] != 200.0 || row["z"] != 450.0 || placeOf(r.(map[string]any))["words"] != "8 m east of you" {
		t.Errorf("bring a buggy: %v %v %v", row, err, r)
	}
	d, err := o.DuplicateVehicle(Args{"id": 579.0})
	if err != nil {
		t.Fatal(err)
	}
	if row, err := o.S.One(`select location_x x, location_z z from actors where id=?`, d.(map[string]any)["id"]); err != nil || row == nil || row["z"] != 450.0 {
		t.Errorf("duplicate a buggy: %v %v", row, err)
	}
}

func TestFlyingInABaseAGroundVehicleStillGoesToTheGroundAndAnAircraftStaysAboveTheCharacter(t *testing.T) {
	// the base of TestPlaceFromInsideABaseGoesToTheGroundOutsideTheClaim: the foundations are at z 0, the character stands on them at z 300
	setup := piecesSQL(7001, smallBase()) + claimTotem + "update actors set location_x=2100, location_y=200 where id=2;" + turnTo(0)
	o := placeOps(t, setup)
	id, r := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	if _, _, z := xyz(t, o, id); z != 250 || placeOf(r)["inBase"] != true {
		t.Errorf("a ground vehicle in a base the editor knows goes to the foundation level and the drop: %v %v", z, placeOf(r))
	}
	o = placeOps(t, setup)
	id, r = spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0"})
	x, y, z := xyz(t, o, id)
	if z < 300+flyingVehicleZOffset || placeOf(r)["inBase"] != true || !strings.HasPrefix(placeOf(r)["words"].(string), "outside your base, ") {
		t.Errorf("an aircraft in a base: %v %v %v %v", x, y, z, placeOf(r))
	}
	if baseGap := math.Hypot(math.Max(math.Abs(x-1500)-768, 0), math.Max(math.Abs(y)-768, 0)); baseGap < 800+placeMargin {
		t.Errorf("the aircraft is %v from the base", baseGap)
	}
	if along, _ := ahead(x, y, 2100, 200, 0); along < flyingVehicleMinOffset {
		t.Errorf("%v ahead", along)
	}
}

func TestFlyingTheAnswerSaysWhereAndThatItSettles(t *testing.T) {
	o := placeOps(t, "")
	prev, err := o.SpawnVehiclePreview(Args{"class": "OrnithopterLight", "template": "T0"})
	if err != nil {
		t.Fatal(err)
	}
	_, r := spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0"})
	const want = "30 m south of you and 7 m above you; it starts in the air and the game settles it"
	for name, p := range map[string]map[string]any{"preview": placeOf(prev.(map[string]any)), "create": placeOf(r)} {
		if p["words"] != want || p["inAir"] != true || p["up"] != 700.0 || p["distance"] != 3000.0 {
			t.Errorf("%s: %v", name, p)
		}
	}
	// a ground vehicle says where, and nothing about the air
	_, r = spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
	if p := placeOf(r); p["words"] != "8 m east of you" || p["inAir"] != false || p["up"] != 150.0 {
		t.Errorf("sandbike: %v", p)
	}
	// a place given is used as asked, and is not in the air by the editor's doing
	_, r = spawn(t, o, Args{"class": "OrnithopterLight", "template": "T0", "place": map[string]any{"map": "HaggaBasin", "x": 0.0, "y": 0.0, "z": 100.0}})
	if p := placeOf(r); p["words"] != "where you asked" || p["inAir"] != false {
		t.Errorf("a given place: %v", p)
	}
}
