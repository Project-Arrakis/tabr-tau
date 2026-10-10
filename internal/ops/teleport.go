package ops

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Teleport as the Dune Docker console has it (PlayerTeleportControls.tsx): move the character to coordinates or to a named place,
// with a facing. In single player the move is a change to the save, so it applies the next time single-player is entered: the
// game keeps its database in memory and overwrites the file while a session runs (issue #67).

// destinationLimit is the most rows one destination list returns.
const destinationLimit = 200

// Destination is one place the character can be moved to.
type Destination struct {
	Kind     string   `json:"kind"`
	Label    string   `json:"label"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Z        float64  `json:"z"`
	Distance float64  `json:"distance"` // flat distance from the character now, in game units
	Facing   *float64 `json:"facing,omitempty"`
}

// DestinationKinds are the lists the picker offers.
var DestinationKinds = []string{"base", "respawn", "vehicle", "marker"}

// where returns the character's pawn id, map and position; destinations are limited to that map because a move to another map is not
// something the editor can do from the save (it is held back until it has been checked in game).
func (o *Ops) where() (id int64, mp string, x, y float64, err error) {
	p, err := o.player()
	if err != nil {
		return 0, "", 0, 0, err
	}
	r, err := o.S.One(`select map, location_x x, location_y y from actors where id=?`, p.Pawn)
	if err != nil || r == nil {
		return 0, "", 0, 0, errors.New("the character's body was not found in this save")
	}
	fx, _ := toFloat(r["x"])
	fy, _ := toFloat(r["y"])
	return p.Pawn, fmt.Sprint(r["map"]), fx, fy, nil
}

func flatDistance(x, y, px, py float64) float64 { return math.Hypot(x-px, y-py) }

// Destinations lists places on the character's current map for one kind (base, respawn, vehicle or marker), nearest first. q narrows by
// name (a marker's type, a respawn group, a vehicle class).
func (o *Ops) Destinations(kind, q string) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	_, mp, px, py, err := o.where()
	if err != nil {
		return nil, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	var out []Destination
	add := func(kind, label string, x, y, z any) {
		fx, ok1 := toFloat(x)
		fy, ok2 := toFloat(y)
		fz, ok3 := toFloat(z)
		if !ok1 || !ok2 || !ok3 {
			return // a place without coordinates in the save cannot be a destination
		}
		if q != "" && !strings.Contains(strings.ToLower(label), q) {
			return
		}
		out = append(out, Destination{Kind: kind, Label: label, X: fx, Y: fy, Z: fz, Distance: flatDistance(fx, fy, px, py)})
	}
	switch kind {
	case "base":
		rows, err := o.S.Query(`select t.id id, t.landclaim_original_global_location_x x, t.landclaim_original_global_location_y y,
			t.landclaim_original_global_location_z z from totems t join actors a on a.id=t.id where a.map=?`, mp)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			add("base", fmt.Sprintf("Base #%v", r["id"]), r["x"], r["y"], r["z"])
		}
	case "respawn":
		rows, err := o.S.Query(`select pr."group" g, coalesce(pr.locator_location_x, a.location_x) x, coalesce(pr.locator_location_y, a.location_y) y,
			coalesce(pr.locator_location_z, a.location_z) z from player_respawn_locations pr left join actors a on a.id=pr.locator_actor_id
			where pr.character_id=? and pr.map=?`, p.Controller, mp)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			add("respawn", fmt.Sprint(r["g"]), r["x"], r["y"], r["z"])
		}
	case "vehicle":
		rows, err := o.S.Query(`select a.id id, a.class cls, a.location_x x, a.location_y y, a.location_z z from vehicles v join actors a on a.id=v.id
			where v.id in (`+ownedVehicleSQL+`) and a.map=?`, p.Controller, mp)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			add("vehicle", fmt.Sprintf("%s #%v", shortClass(r["cls"]), r["id"]), r["x"], r["y"], r["z"])
		}
	case "marker":
		rows, err := o.S.Query(`select m.marker_type t, m.x x, m.y y, m.z z from markers m
			join player_markers pm on pm.marker_hash_id=m.marker_hash_id and pm.dimension_index=m.dimension_index and pm.map_name=m.map_name
			where pm.player_id=? and pm.discovery_level>0 and m.map_name=?`, p.Controller, mp)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			add("marker", fmt.Sprint(r["t"]), r["x"], r["y"], r["z"])
		}
	default:
		return nil, errors.New("kind must be base, respawn, vehicle or marker")
	}
	sortByDistance(out)
	total := len(out)
	if len(out) > destinationLimit {
		out = out[:destinationLimit]
	}
	if out == nil {
		out = []Destination{}
	}
	return map[string]any{"map": mp, "kind": kind, "total": total, "destinations": out}, nil
}

func sortByDistance(d []Destination) {
	// insertion-sorted merge would do, but the lists are a few thousand at most
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j].Distance < d[j-1].Distance; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}

// yawQuaternion is the rotation of a character turned by yaw degrees about the vertical axis, as the actors table stores it
// (x, y, z, w; Unreal's convention, 0 degrees faces +X).
func yawQuaternion(deg float64) (x, y, z, w float64) {
	h := deg * math.Pi / 360
	return 0, 0, math.Sin(h), math.Cos(h)
}

// Teleport moves the character's body to x, y, z (game units) and, when "yaw" is given in degrees, turns it to face that way.
func (o *Ops) Teleport(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	x, e1 := a.Float("x")
	y, e2 := a.Float("y")
	z, e3 := a.Float("z")
	if err := errors.Join(e1, e2, e3); err != nil {
		return nil, err
	}
	for _, v := range []float64{x, y, z} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e7 {
			return nil, errors.New("x, y and z must be ordinary numbers (game units)")
		}
	}
	desc := fmt.Sprintf("teleport player to %.0f, %.0f, %.0f", x, y, z)
	if _, has := a["yaw"]; !has || a["yaw"] == nil || a["yaw"] == "" {
		if _, err := o.run(desc, `update actors set location_x=?, location_y=?, location_z=? where id=?`, x, y, z, p.Pawn); err != nil {
			return nil, err
		}
		return ok(), nil
	}
	yaw, err := a.Float("yaw")
	if err != nil || math.IsNaN(yaw) || math.IsInf(yaw, 0) || math.Abs(yaw) > 3600 {
		return nil, errors.New("facing must be a number of degrees")
	}
	qx, qy, qz, qw := yawQuaternion(yaw)
	if _, err := o.run(desc+fmt.Sprintf(" facing %.0f degrees", yaw),
		`update actors set location_x=?, location_y=?, location_z=?, rotation_x=?, rotation_y=?, rotation_z=?, rotation_w=? where id=?`,
		x, y, z, qx, qy, qz, qw, p.Pawn); err != nil {
		return nil, err
	}
	return ok(), nil
}
