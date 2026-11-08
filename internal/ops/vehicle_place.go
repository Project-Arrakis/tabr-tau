package ops

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/vehicles"
)

// Where a vehicle goes (#480). Create a vehicle, Bring vehicle and Duplicate vehicle all put a vehicle "next to the character": 800 along x and 150 up. Taken blind that point can be
// inside a base wall, in the ground-floor room of a base, or on another storey. This file chooses the spot by search instead: the old point is the first candidate, then rings of
// growing radius around the character, and the first candidate with room for the vehicle is taken. Everything it knows comes from the save (the base pieces and placeables, the other
// vehicles, the land claims); the terrain and the level's own props are not in a save, so a point can still be inside a rock. The design note says what each number rests on.
//
// The model is deliberately coarse and errs on the side of keeping away:
//
//   - A base piece is a box: a foundation, floor or roof is a 512 x 512 cell (the pieces of a real base are 512 apart), a wall, window or door frame is a 512 x 80 slab on the edge of
//     a cell turned by the piece's own yaw, a column is 120 x 120, anything of an unknown kind (or a corner, round or sloped piece) counts as a whole cell. Walls, foundations, stairs
//     and ramps stand 384 high from the piece's z (a storey of walls is 384 in the saves); a floor or roof slab is thin and hangs on both sides of its z.
//   - A vehicle is a circle of a radius per class, plus a margin, and a height. A candidate is refused when a piece lies within the circle at any height from 800 below the vehicle to
//     2,500 above it (that keeps the vehicle out of a room, a garage under a floor and the ground floor of a base, not only out of the walls), when a placeable or another vehicle
//     lies within it, or when it is shut in: sixteen lines out from it all end at a piece within 30 m.
//   - A character who stands on, in or under a base piece is "in a base". The vehicle then goes to the ground outside the base: the level is that of the nearest foundation's
//     bottom (the stairs and ramps of a base start there, so the ground is there) plus a drop, and a point outside the land claim the character is in is preferred when there is one
//     within 30 m. A character outdoors keeps the old height, 150 above their feet.
//   - An aircraft (an ornithopter) is the exception (#540): it is put at least 30 m ahead of the character, along the character's facing, and at least 7 m above the character's height,
//     in the air, for the game to settle. The same clear-spot search runs from that point, with the vehicle's height as it is there.

// vehicleSize is the footprint of a vehicle class: the radius of a circle that holds the vehicle at any heading, and its height, in centimetres. The game's tables carry no sizes, so the
// numbers are chosen (see the design note): about the vehicle's half-diagonal with its wings out, a little generous, larger for the big ones.
//
// Flies says the vehicle is an aircraft: an ornithopter. Create, Bring and Duplicate put an aircraft in the air, ahead of the character (#540, below). The class table has no flag for it,
// so it is told here, once, by the class key (every ornithopter key begins with "Ornithopter") and, for a vehicle whose class is not in the table, by "ornithopter" in its class name.
type vehicleSize struct {
	Radius, Height float64
	Flies          bool
}

var vehicleSizes = map[string]vehicleSize{
	"Sandbike":             {250, 200, false},
	"Buggy":                {400, 300, false},
	"OrnithopterLight":     {800, 450, true},
	"OrnithopterMedium":    {1200, 550, true},
	"OrnithopterTransport": {1800, 800, true},
	"Sandcrawler":          {2200, 1000, false},
	"ContainerVehicle":     {450, 350, false},
}

// unknownVehicleSize is for a vehicle whose class is not one of the seven (a world vehicle of another kind).
var unknownVehicleSize = vehicleSize{500, 400, false}

// unknownFlyingSize is for a vehicle whose class is not in the table but whose name says it is an ornithopter: the largest ornithopter's footprint, and it flies.
var unknownFlyingSize = vehicleSize{1800, 800, true}

func sizeOfClassKey(key string) vehicleSize {
	if s, ok := vehicleSizes[key]; ok {
		return s
	}
	return unknownVehicleSize
}

// sizeOfActorClass is the size of the vehicle whose actors.class is class.
func sizeOfActorClass(class any) vehicleSize {
	c, _ := class.(string)
	for _, k := range vehicles.ClassKeys() {
		if vc, ok := vehicles.ClassFor(k); ok && vc.ActorClass == c {
			return sizeOfClassKey(k)
		}
	}
	if strings.Contains(strings.ToLower(c), "ornithopter") {
		return unknownFlyingSize
	}
	return unknownVehicleSize
}

const (
	placeStartRadius  = 800.0  // the first candidate is the old default: 800 along x
	placeStep         = 400.0  // the rings grow by this much
	placeMaxRadius    = 8000.0 // the farthest candidate from the character: 80 m
	placeDrop         = 150.0  // above the character's feet, as before
	placeGroundDrop   = 250.0  // above the foundation level when the character is in a base
	placeMargin       = 100.0  // added to the vehicle's radius
	placeBelow        = 800.0  // a piece this far below the vehicle's z still counts as being under it
	placeAbove        = 2500.0 // a piece this far above the vehicle's z still counts as being over it
	placeCharRadius   = 60.0   // the character is on a piece when this circle touches it
	placeClaimReach   = 3000.0 // a point outside the land claim is preferred up to this far from the character
	placeShutRayLen   = 3000.0
	placeShutRays     = 16
	placeShutStep     = 40.0   // finer than the thinnest wall, 80
	placeGroundSearch = 6000.0 // a foundation farther than this from the vehicle does not tell the ground there
	pieceHalfCell     = 256.0
	pieceStorey       = 384.0
)

// Flying vehicles (#540). The placement cannot see the terrain or the level's rocks, so a point 22 m under a standing character, inside rock, was once chosen for an ornithopter. The
// console's published rule for a flying vehicle is followed instead: the vehicle is put well ahead of the character and well above, in the air, and the game settles it. The values are
// those of the console's admin script: DUNE_ADMIN_FLYING_VEHICLE_MIN_OFFSET is 3000 (centimetres, at least 30 m ahead along the character's facing) and
// DUNE_ADMIN_FLYING_VEHICLE_Z_OFFSET is 700 (7 m above the character's height).
const (
	flyingVehicleMinOffset = 3000.0 // at least this far ahead of the character, in the plan
	flyingVehicleZOffset   = 700.0  // at least this far above the character's height
)

// solid is something a vehicle cannot be put in.
type solid struct {
	x, y       float64
	cos, sin   float64 // of the yaw
	hx, hy     float64 // half sizes in the piece's own frame; a round solid has hx as its radius
	round      bool
	z0, z1     float64
	kind       string // "piece", "placeable", "vehicle"
	building   int64
	foundation bool
}

// quatYaw is the turn about the vertical axis of a rotation; the pieces of a base are never tilted.
func quatYaw(x, y, z, w float64) float64 {
	if x == 0 && y == 0 && z == 0 && w == 0 {
		return 0
	}
	yaw := math.Atan2(2*(w*z+x*y), 1-2*(y*y+z*z))
	if math.IsNaN(yaw) {
		return 0
	}
	return yaw
}

// shapeOf says how a building piece of this type is modelled: half sizes, how far it reaches below and above its z, and whether it is a foundation.
func shapeOf(typ string) (hx, hy, below, above float64, foundation bool) {
	n := strings.ToLower(typ)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(n, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("foundation"):
		return pieceHalfCell, pieceHalfCell, 0, pieceStorey, true
	case has("column", "pillar"):
		return 60, 60, 0, pieceStorey, false
	case has("floor", "roof", "ceiling"):
		return pieceHalfCell, pieceHalfCell, 100, 30, false
	case has("stairs", "ramp"):
		return pieceHalfCell, pieceHalfCell, 0, pieceStorey, false
	case has("wall", "window", "door", "passageway", "railing", "gate") && !has("corner", "round", "inclined", "triangle", "wedge"):
		return pieceHalfCell, 40, 0, pieceStorey, false
	}
	return pieceHalfCell, pieceHalfCell, 0, pieceStorey, false
}

// touches says whether a circle of radius r at (cx, cy) reaches the solid in the plan.
func (s *solid) touches(cx, cy, r float64) bool {
	dx, dy := cx-s.x, cy-s.y
	if s.round {
		return math.Hypot(dx, dy) <= r+s.hx
	}
	lx, ly := dx*s.cos+dy*s.sin, -dx*s.sin+dy*s.cos
	qx, qy := math.Max(math.Abs(lx)-s.hx, 0), math.Max(math.Abs(ly)-s.hy, 0)
	return qx*qx+qy*qy <= r*r
}

// placeWorld is what the save holds around the character.
type placeWorld struct {
	solids  []solid
	claims  []placeClaim
	mapName string
}

// placeClaim is a land claim: the totem's origin and yaw, and the cells.
type placeClaim struct {
	ox, oy, sin, cos float64
	cells            map[claimCell]bool
}

// holds says whether (x, y) is in one of the cells of the claim.
func (c placeClaim) holds(x, y float64) bool {
	dx, dy := x-c.ox, y-c.oy
	lx, ly := dx*c.cos-dy*c.sin, dx*c.sin+dy*c.cos
	return c.cells[claimCell{int64(math.Round(lx / claimCellSize)), int64(math.Round(ly / claimCellSize))}]
}

type rowReader func(q string, args ...any) ([]save.Row, error)

// loadPlaceWorld reads the pieces, placeables and vehicles within reach of (cx, cy) on the map, and the land claims. exclude is a vehicle that is being moved (its own old place is not an obstacle).
func loadPlaceWorld(query rowReader, mapName string, dim int64, cx, cy float64, exclude int64) (*placeWorld, error) {
	reach := placeMaxRadius + 4000
	x0, x1, y0, y1 := cx-reach, cx+reach, cy-reach, cy+reach
	w := &placeWorld{mapName: mapName}
	pieces, err := query(`select bi.building_id b, bi.building_type t, bi.location_x x, bi.location_y y, bi.location_z z, bi.rotation_x qx, bi.rotation_y qy, bi.rotation_z qz, bi.rotation_w qw
		from building_instances bi join actors a on a.id=bi.building_id
		where a.map=? and a.dimension_index=? and bi.location_x between ? and ? and bi.location_y between ? and ?`, mapName, dim, x0, x1, y0, y1)
	if err != nil {
		return nil, err
	}
	for _, r := range pieces {
		x, okx := toFloat(r["x"])
		y, oky := toFloat(r["y"])
		z, okz := toFloat(r["z"])
		if !okx || !oky || !okz {
			continue
		}
		hx, hy, below, above, found := shapeOf(fmt.Sprint(r["t"]))
		yaw := quatYaw(num(r["qx"]), num(r["qy"]), num(r["qz"]), num(r["qw"]))
		b, _ := toInt(r["b"])
		w.solids = append(w.solids, solid{x: x, y: y, cos: math.Cos(yaw), sin: math.Sin(yaw), hx: hx, hy: hy, z0: z - below, z1: z + above, kind: "piece", building: b, foundation: found})
	}
	pl, err := query(`select a.location_x x, a.location_y y, a.location_z z from placeables p join actors a on a.id=p.id
		where p.is_hologram=0 and a.map=? and a.dimension_index=? and a.location_x between ? and ? and a.location_y between ? and ?`, mapName, dim, x0, x1, y0, y1)
	if err != nil {
		return nil, err
	}
	for _, r := range pl {
		x, okx := toFloat(r["x"])
		y, oky := toFloat(r["y"])
		z, okz := toFloat(r["z"])
		if okx && oky && okz {
			w.solids = append(w.solids, solid{x: x, y: y, hx: 250, round: true, z0: z - 50, z1: z + 400, kind: "placeable"})
		}
	}
	vs, err := query(`select a.id id, a.class c, a.location_x x, a.location_y y, a.location_z z from actors a join vehicles v on v.id=a.id
		where a.map=? and a.dimension_index=? and a.id<>? and a.location_x between ? and ? and a.location_y between ? and ?
		and a.id not in (`+storedVehicleSQL+`)`, mapName, dim, exclude, x0, x1, y0, y1)
	if err != nil {
		return nil, err
	}
	for _, r := range vs {
		x, okx := toFloat(r["x"])
		y, oky := toFloat(r["y"])
		z, okz := toFloat(r["z"])
		if okx && oky && okz {
			sz := sizeOfActorClass(r["c"])
			w.solids = append(w.solids, solid{x: x, y: y, hx: sz.Radius, round: true, z0: z - 150, z1: z + sz.Height, kind: "vehicle"})
		}
	}
	tot, err := query(`select t.id id, t.landclaim_original_global_location_x ox, t.landclaim_original_global_location_y oy, t.landclaim_original_global_yaw_rotation yaw
		from totems t join actors a on a.id=t.id where a.map=?`, mapName)
	if err != nil {
		return nil, err
	}
	for _, t := range tot {
		ox, okx := toFloat(t["ox"])
		oy, oky := toFloat(t["oy"])
		yaw, okyaw := toFloat(t["yaw"])
		if !okx || !oky || !okyaw {
			continue
		}
		segs, err := query(`select grid_location_x x, grid_location_y y from landclaim_segments where totem_id=?`, t["id"])
		if err != nil {
			return nil, err
		}
		s, c := math.Sincos(-yaw * math.Pi / 180)
		w.claims = append(w.claims, placeClaim{ox: ox, oy: oy, sin: s, cos: c, cells: claimCellSet(segs)})
	}
	return w, nil
}

// blockers lists what is in the way of a vehicle of this size centred at (cx, cy) with its bottom at zc: "pieces", "placeables" or "vehicles", sorted.
func (w *placeWorld) blockers(cx, cy, zc float64, sz vehicleSize) []string {
	r := sz.Radius + placeMargin
	seen := map[string]bool{}
	for i := range w.solids {
		s := &w.solids[i]
		var lo, hi float64
		switch s.kind {
		case "piece":
			lo, hi = zc-placeBelow, zc+placeAbove
		default:
			lo, hi = zc, zc+sz.Height
		}
		if s.z1 < lo || s.z0 > hi || seen[s.kind] || !s.touches(cx, cy, r) {
			continue
		}
		seen[s.kind] = true
	}
	var out []string
	for k := range seen {
		out = append(out, k+"s")
	}
	sort.Strings(out)
	return out
}

// shutIn says whether every line out from (cx, cy) ends at a piece within placeShutRayLen: a courtyard with no way out, which a roof test does not see.
func (w *placeWorld) shutIn(cx, cy, zc float64, sz vehicleSize) bool {
	var near []*solid
	for i := range w.solids {
		s := &w.solids[i]
		if s.kind == "piece" && s.z1 >= zc && s.z0 <= zc+sz.Height && math.Hypot(cx-s.x, cy-s.y) <= placeShutRayLen+pieceHalfCell*1.5 {
			near = append(near, s)
		}
	}
	if len(near) == 0 {
		return false
	}
	for k := 0; k < placeShutRays; k++ {
		a := 2 * math.Pi * float64(k) / placeShutRays
		sa, ca := math.Sincos(a)
		hit := false
		for d := placeShutStep; d <= placeShutRayLen && !hit; d += placeShutStep {
			for _, s := range near {
				if s.touches(cx+ca*d, cy+sa*d, 0) {
					hit = true
					break
				}
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// onPieces says whether the character at (x, y, z) stands on, in or under a piece, and the buildings those pieces belong to.
func (w *placeWorld) onPieces(x, y, z float64) (bool, map[int64]bool) {
	b := map[int64]bool{}
	for i := range w.solids {
		s := &w.solids[i]
		if s.kind == "piece" && s.z1 >= z-placeBelow && s.z0 <= z+placeAbove && s.touches(x, y, placeCharRadius) {
			b[s.building] = true
		}
	}
	return len(b) > 0, b
}

// groundAt is the level of the ground near (x, y) judged from the foundations: the bottom of the nearest foundation (a base's stairs and ramps start there). ok is false when there is no foundation.
func (w *placeWorld) groundAt(x, y float64, buildings map[int64]bool) (float64, bool) {
	best, bz, found := math.MaxFloat64, 0.0, false
	for i := range w.solids {
		s := &w.solids[i]
		if s.kind != "piece" || !s.foundation {
			continue
		}
		if d := math.Hypot(x-s.x, y-s.y); d < best {
			best, bz, found = d, s.z0, true
		}
	}
	if found && best <= placeGroundSearch {
		return bz, true
	}
	// no foundation near: the lowest piece of the buildings the character is on
	low, ok := math.MaxFloat64, false
	for i := range w.solids {
		s := &w.solids[i]
		if s.kind == "piece" && buildings[s.building] && s.z0 < low {
			low, ok = s.z0, true
		}
	}
	return low, ok
}

// characterClaims lists the claims that hold the point.
func (w *placeWorld) characterClaims(x, y float64) []placeClaim {
	var out []placeClaim
	for _, c := range w.claims {
		if c.holds(x, y) {
			out = append(out, c)
		}
	}
	return out
}

// outsideClaims says whether a vehicle at (cx, cy) with radius r is outside every one of the claims.
func outsideClaims(cl []placeClaim, cx, cy, r float64) bool {
	for _, c := range cl {
		if c.holds(cx, cy) {
			return false
		}
		for k := 0; k < 8; k++ {
			sa, ca := math.Sincos(float64(k) * math.Pi / 4)
			if c.holds(cx+ca*r, cy+sa*r) {
				return false
			}
		}
	}
	return true
}

// charPose is where the character is and which way it faces. Fx, Fy is a unit vector on the ground plane; HasFacing is false when the saved rotation cannot tell (all zero, or
// pointing straight up or down), and then the search starts from its usual direction.
type charPose struct {
	X, Y, Z   float64
	Fx, Fy    float64
	HasFacing bool
}

// facingOf reads the facing from a saved rotation (x, y, z, w): forward = (1 - 2(qy² + qz²), 2(qx·qy + qw·qz)), normalised on the ground plane, which is the maths the console uses.
func facingOf(qx, qy, qz, qw float64) (fx, fy float64, ok bool) {
	if qx == 0 && qy == 0 && qz == 0 && qw == 0 { // no rotation was saved
		return 0, 0, false
	}
	fx, fy = 1-2*(qy*qy+qz*qz), 2*(qx*qy+qw*qz)
	n := math.Hypot(fx, fy)
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 1e-6 {
		return 0, 0, false
	}
	return fx / n, fy / n, true
}

// poseOf reads a character's pose from an actors row with the columns x, y, z and rx, ry, rz, rw.
func poseOf(row map[string]any) charPose {
	p := charPose{X: num(row["x"]), Y: num(row["y"]), Z: num(row["z"])}
	p.Fx, p.Fy, p.HasFacing = facingOf(num(row["rx"]), num(row["ry"]), num(row["rz"]), num(row["rw"]))
	return p
}

// placement is where a vehicle goes and what was done to find the spot.
type placement struct {
	Map     string
	X, Y, Z float64
	Moved   bool     // not the old default point
	InBase  bool     // the character is on a base piece
	Blocked []string // what was in the way at the first spot
	Claim   string   // "outside": the spot is outside the land claim the character is in; "inside": it could not be; "": the character is in no claim, or not in a base
	Dist    float64  // from the character, in the plan, in centimetres
	Bearing string
	Up      float64 // above the character's height, in centimetres (negative: below it)
	Air     bool    // an aircraft, set down in the air for the game to settle (#540)
}

var compass8 = []string{"north", "north-east", "east", "south-east", "south", "south-west", "west", "north-west"}

// bearingWord is the compass point of the direction (dx, dy) as the map runs: east is +x, south is +y.
func bearingWord(dx, dy float64) string {
	deg := math.Atan2(dy, dx) * 180 / math.Pi
	b := math.Mod(math.Mod(deg+90, 360)+360, 360)
	return compass8[int(math.Round(b/45))%8]
}

var blockerWords = map[string]string{"pieces": "base pieces", "placeables": "things you built", "vehicles": "another vehicle"}

// listWords joins "a", "b" and "c" as "a, b and c".
func listWords(ks []string) string {
	var ws []string
	for _, k := range ks {
		ws = append(ws, blockerWords[k])
	}
	if len(ws) > 1 {
		return strings.Join(ws[:len(ws)-1], ", ") + " and " + ws[len(ws)-1]
	}
	return strings.Join(ws, "")
}

// Words says in one sentence where the vehicle goes, in the editor's own style: how far and which way from the character, how high for an aircraft, and that the game settles it.
func (p placement) Words() string {
	var b strings.Builder
	if p.InBase {
		b.WriteString("outside your base, ")
	}
	fmt.Fprintf(&b, "%d m %s of you", int(math.Round(p.Dist/100)), p.Bearing)
	if p.Air {
		fmt.Fprintf(&b, " and %d m above you", int(math.Round(p.Up/100)))
	}
	switch p.Claim {
	case "outside":
		b.WriteString(", outside the land claim")
	case "inside":
		b.WriteString(", still inside the land claim")
	}
	if len(p.Blocked) > 0 {
		b.WriteString(" (" + listWords(p.Blocked) + " at the first spot)")
	} else if p.InBase {
		b.WriteString(" (you are inside a base)")
	}
	if p.Air {
		b.WriteString("; it starts in the air and the game settles it")
	}
	return b.String()
}

// Info is the answer's description of the place.
func (p placement) Info() map[string]any {
	return map[string]any{"map": p.Map, "x": p.X, "y": p.Y, "z": p.Z, "words": p.Words(), "moved": p.Moved, "inBase": p.InBase,
		"blocked": append([]string{}, p.Blocked...), "claim": p.Claim, "distance": math.Round(p.Dist), "up": math.Round(p.Up), "inAir": p.Air}
}

// choosePlace finds the place for a vehicle of the given size next to the character, which stands at pose on the map. exclude is the vehicle that is being moved.
func choosePlace(query rowReader, mapName string, dim int64, pose charPose, sz vehicleSize, exclude int64, label string) (placement, error) {
	w, err := loadPlaceWorld(query, mapName, dim, pose.X, pose.Y, exclude)
	if err != nil {
		return placement{}, err
	}
	return w.choose(pose, sz, label)
}

func (w *placeWorld) choose(pose charPose, sz vehicleSize, label string) (placement, error) {
	cx, cy, cz := pose.X, pose.Y, pose.Z
	inBase, buildings := w.onPieces(cx, cy, cz)
	start := 0.0
	if inBase {
		// the character's own side: away from the middle of the base
		var sx, sy, n float64
		for i := range w.solids {
			if s := &w.solids[i]; s.kind == "piece" && buildings[s.building] {
				sx, sy, n = sx+s.x, sy+s.y, n+1
			}
		}
		if n > 0 && math.Hypot(cx-sx/n, cy-sy/n) > 150 {
			start = math.Atan2(cy-sy/n, cx-sx/n)
		}
	}
	// the first ring and the height. A ground vehicle starts at the old point, 800 along the start direction and 150 above the character's feet (in a base: the ground outside, below).
	// An aircraft (#540) starts on the console's rule: at least 30 m ahead along the character's facing and at least 7 m above the character's height, so it begins in the air.
	firstRing := placeStartRadius
	claimReach := placeClaimReach
	if sz.Flies {
		firstRing = math.Max(placeStartRadius, flyingVehicleMinOffset)
		claimReach += firstRing - placeStartRadius
		if pose.HasFacing {
			start = math.Atan2(pose.Fy, pose.Fx)
		}
	}
	zAt := func(x, y float64) float64 {
		if sz.Flies {
			z := cz + flyingVehicleZOffset
			if inBase {
				if g, ok := w.groundAt(x, y, buildings); ok {
					z = math.Max(z, g+placeGroundDrop)
				}
			}
			return z
		}
		if inBase {
			if g, ok := w.groundAt(x, y, buildings); ok {
				return g + placeGroundDrop
			}
		}
		return cz + placeDrop
	}
	// what is at the first spot, which is what the person is told was in the way
	lx, ly, lz := cx+placeStartRadius, cy, cz+placeDrop // the old default point: always east of the character
	if sz.Flies {
		lx, ly = cx+firstRing*math.Cos(start), cy+firstRing*math.Sin(start)
		lz = zAt(lx, ly)
	}
	legacy := w.blocked(lx, ly, lz, sz)
	var claims []placeClaim
	if inBase {
		claims = w.characterClaims(cx, cy)
	}
	r := sz.Radius + placeMargin
	var first *placement
	for ring := firstRing; ring <= placeMaxRadius; ring += placeStep {
		count := int(math.Max(12, math.Ceil(2*math.Pi*ring/400)))
		for i := 0; i < count; i++ {
			// the ring's own order: the start direction, then alternately to either side of it
			k := (i + 1) / 2
			if i%2 == 0 {
				k = -k
			}
			a := start + 2*math.Pi*float64(k)/float64(count)
			x, y := cx+ring*math.Cos(a), cy+ring*math.Sin(a)
			z := zAt(x, y)
			if len(w.blocked(x, y, z, sz)) > 0 {
				continue
			}
			p := placement{Map: w.mapName, X: x, Y: y, Z: z, InBase: inBase, Blocked: legacy, Dist: ring, Bearing: bearingWord(x-cx, y-cy), Up: z - cz, Air: sz.Flies}
			p.Moved = math.Abs(x-lx) > 0.5 || math.Abs(y-ly) > 0.5 || math.Abs(z-lz) > 0.5
			if len(claims) == 0 {
				return p, nil
			}
			if outsideClaims(claims, x, y, r) {
				p.Claim = "outside"
				return p, nil
			}
			p.Claim = "inside"
			if first == nil {
				first = &p
			}
			if ring >= claimReach {
				return *first, nil
			}
		}
		if first != nil && ring+placeStep > claimReach {
			return *first, nil
		}
	}
	if first != nil {
		return *first, nil
	}
	return placement{}, apierr.Newf(apierr.ErrBadArgument, "there is no clear spot for a %s within %d m of the character: base pieces, buildings or other vehicles are in the way. Walk to open ground, or give a place", strings.ToLower(label), int(placeMaxRadius/100))
}

// blocked is what keeps a vehicle out of (x, y, z): what is in the way, or "pieces" when it is shut in.
func (w *placeWorld) blocked(x, y, z float64, sz vehicleSize) []string {
	bl := w.blockers(x, y, z, sz)
	if len(bl) == 0 && w.shutIn(x, y, z, sz) {
		return []string{"pieces"}
	}
	return bl
}

// placeNote says that an explicit place is inside something, or "" when it is clear or the editor cannot tell.
func (w *placeWorld) placeNote(x, y, z float64, sz vehicleSize) string {
	bl := w.blocked(x, y, z, sz)
	if len(bl) == 0 {
		return ""
	}
	return "the place given has " + listWords(bl) + " within reach of the vehicle; it was put there as asked. Bring vehicle moves it next to you"
}
