package ops

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Teleport as the Dune Docker console has it (PlayerTeleportControls.tsx): move the character to coordinates or to a named place,
// with a facing. In single player the move is a change to the save, so it applies the next time single-player is entered: the
// game keeps its database in memory and overwrites the file while a session runs (issue #67).

// destinationLimit is the most rows one destination list returns.
const destinationLimit = 200

// Destination is one place the character can be moved to.
type Destination struct {
	Label    string   `json:"label"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Z        float64  `json:"z"`
	Distance float64  `json:"distance"`          // flat distance from the character now, in game units; 0 (not given) for a map the character is not on
	Group    string   `json:"group,omitempty"`   // the heading of the list the place is under (tradepost, base, vehicle, respawn or taxi)
	Arrival  bool     `json:"arrival,omitempty"` // X, Y, Z are the game's arrival point for the place (#515), not the marker's position; Distance is to this point
	Facing   *float64 `json:"facing,omitempty"`
}

// TravelMap is a map the character can be moved to: the two the console offers for an offline player (Hagga Basin and the Deep Desert).
// The social hubs and the dungeons are instances with their own entry; a position in the save is not enough to enter them, so they are not offered.
type TravelMap struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Current bool   `json:"current"`
}

// travelMapOf returns the map named key, or false when the editor does not move the character to it.
func travelMapOf(key string) (mapConfig, bool) {
	for _, m := range liveMaps {
		if m.Key == key {
			return m, true
		}
	}
	return mapConfig{}, false
}

// where returns the character's pawn id, map and position.
func (o *Ops) where() (id int64, mp string, x, y float64, err error) {
	p, err := o.player()
	if err != nil {
		return 0, "", 0, 0, err
	}
	r, err := o.S.One(`select map, location_x x, location_y y from actors where id=?`, p.Pawn)
	if err != nil || r == nil {
		return 0, "", 0, 0, apierr.New(apierr.ErrNotFound, "the character's body was not found in this save")
	}
	fx, _ := toFloat(r["x"])
	fy, _ := toFloat(r["y"])
	return p.Pawn, fmt.Sprint(r["map"]), fx, fy, nil
}

func flatDistance(x, y, px, py float64) float64 { return math.Hypot(x-px, y-py) }

// Destinations lists the places the character can go to on the map named dest, or on the character's current map when dest is empty: one list with every group in it
// (places.go: trading posts, the character's deployed bases, the character's vehicles in the world, the character's respawn points, taxi hubs), each place under its group, the
// groups in that order. On the character's own map the places of a group are nearest first; on another map a distance from the character means nothing, so none is given and the
// places are in the order of the save's tables. q narrows by name (a place's name or the other name of a trading post); the page does not send it.
// The page used to ask for one kind at a time (base, respawn, vehicle or marker, #519); the route no longer reads that parameter, so an old caller gets the whole list.
func (o *Ops) Destinations(q, dest string) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	_, cur, px, py, err := o.where()
	if err != nil {
		return nil, err
	}
	mp := cur
	if dest != "" {
		if _, ok := travelMapOf(dest); !ok && dest != cur {
			return nil, apierr.New(apierr.ErrBadArgument, "map must be HaggaBasin or DeepDesert")
		}
		mp = dest
	}
	same := mp == cur
	maps := make([]TravelMap, 0, len(liveMaps))
	for _, m := range liveMaps {
		maps = append(maps, TravelMap{Key: m.Key, Label: m.Label, Current: m.Key == cur})
	}
	q = strings.ToLower(strings.TrimSpace(q))
	places, err := o.namedPlaces(p, mp)
	if err != nil {
		return nil, err
	}
	var out []Destination
	for _, r := range places {
		fx, ok1 := toFloat(r.x)
		fy, ok2 := toFloat(r.y)
		fz, ok3 := toFloat(r.z)
		if !ok1 || !ok2 || !ok3 {
			continue // a place without coordinates in the save cannot be a destination
		}
		if q != "" && !strings.Contains(strings.ToLower(r.label), q) && !strings.Contains(strings.ToLower(r.alias), q) {
			continue
		}
		d := Destination{Group: r.group, Label: r.label, X: fx, Y: fy, Z: fz, Arrival: r.arrival}
		if same {
			d.Distance = flatDistance(fx, fy, px, py)
		}
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if gi, gj := groupIndex(out[i].Group), groupIndex(out[j].Group); gi != gj {
			return gi < gj
		}
		return same && out[i].Distance < out[j].Distance
	})
	total := len(out)
	if len(out) > destinationLimit {
		out = out[:destinationLimit]
	}
	if out == nil {
		out = []Destination{}
	}
	return map[string]any{"map": mp, "sameMap": same, "maps": maps, "total": total, "destinations": out, "groups": groupsOf(out)}, nil
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
	if p.Pawn == 0 { // no body to move: say so instead of reporting a move that changed nothing (ARCH-4)
		return nil, errors.New("this save has no character body to move")
	}
	for _, v := range []float64{x, y, z} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e7 {
			return nil, apierr.New(apierr.ErrBadArgument, "x, y and z must be ordinary numbers (game units)")
		}
	}
	desc := fmt.Sprintf("teleport player to %.0f, %.0f, %.0f", x, y, z)
	yaw, hasYaw, err := teleportYaw(a)
	if err != nil {
		return nil, err
	}
	dest, err := teleportMapArg(a)
	if err != nil {
		return nil, err
	}
	if dest != "" {
		_, cur, _, _, err := o.where()
		if err != nil {
			return nil, err
		}
		if dest != cur {
			return o.teleportToMap(p, dest, x, y, z, yaw, hasYaw)
		}
	}
	if !hasYaw {
		if _, err := o.run(desc, `update actors set location_x=?, location_y=?, location_z=? where id=?`, x, y, z, p.Pawn); err != nil {
			return nil, err
		}
		return ok(), nil
	}
	qx, qy, qz, qw := yawQuaternion(yaw)
	if _, err := o.run(desc+fmt.Sprintf(" facing %.0f degrees", yaw),
		`update actors set location_x=?, location_y=?, location_z=?, rotation_x=?, rotation_y=?, rotation_z=?, rotation_w=? where id=?`,
		x, y, z, qx, qy, qz, qw, p.Pawn); err != nil {
		return nil, err
	}
	return ok(), nil
}

// teleportYaw reads the optional facing: absent, null or empty means "keep the way the character faces".
func teleportYaw(a Args) (yaw float64, has bool, err error) {
	if _, ok := a["yaw"]; !ok || a["yaw"] == nil || a["yaw"] == "" {
		return 0, false, nil
	}
	yaw, err = a.Float("yaw")
	if err != nil || math.IsNaN(yaw) || math.IsInf(yaw, 0) || math.Abs(yaw) > 3600 {
		return 0, false, apierr.New(apierr.ErrBadArgument, "facing must be a number of degrees")
	}
	return yaw, true, nil
}

// teleportMapArg reads the optional map to move to: absent, null or empty means "the map the character is on". Anything but text is refused.
func teleportMapArg(a Args) (string, error) {
	v, has := a["map"]
	if !has || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", apierr.New(apierr.ErrBadArgument, "map must be text: HaggaBasin or DeepDesert")
	}
	if s == "" {
		return "", nil
	}
	if _, ok := travelMapOf(s); !ok {
		return "", apierr.New(apierr.ErrBadArgument, "map must be HaggaBasin or DeepDesert")
	}
	return s, nil
}

// teleportToMap moves the character to another map. A save the game wrote keeps the character's three actors (the controller, the body and
// the player state) on the same map and at the same position, so a move between maps writes all three; a move on one map writes the body
// only, which is what was checked in the game. The position has to lie inside the destination map's own bounds (Hagga Basin's are much
// smaller than the Deep Desert's, so a Deep Desert position typed for Hagga Basin is caught; the reverse is not, and the places the page lists
// for a map are the safe way to choose). A move to the Deep Desert was checked in the game once (2026-10-27, one character, a throwaway
// save): the three actors were written as here and the game loaded the character there. No other move was checked.
func (o *Ops) teleportToMap(p player, dest string, x, y, z, yaw float64, hasYaw bool) (any, error) {
	cfg, _ := travelMapOf(dest)
	if x < cfg.MinX || x > cfg.MaxX || y < cfg.MinY || y > cfg.MaxY {
		return nil, apierr.Newf(apierr.ErrBadArgument, "X %.0f, Y %.0f is outside the %s map (X %.0f to %.0f, Y %.0f to %.0f)", x, y, cfg.Label, cfg.MinX, cfg.MaxX, cfg.MinY, cfg.MaxY)
	}
	desc := fmt.Sprintf("teleport player to %s %.0f, %.0f, %.0f", dest, x, y, z)
	if hasYaw {
		desc += fmt.Sprintf(" facing %.0f degrees", yaw)
	}
	if _, err := o.S.Mutate(desc, func(m *save.Mut) error {
		r, err := m.Query(`select ps.player_controller_id c, ps.player_pawn_id b, ps.player_state_id s,
			(select count(*) from actors where id in (ps.player_controller_id, ps.player_pawn_id, ps.player_state_id)) n from player_state ps where ps.id=?`, p.Controller)
		if err != nil {
			return err
		}
		if len(r) == 0 {
			return errors.New("the character's record was not found in this save")
		}
		if n, _ := toInt(r[0]["n"]); n != 3 {
			return errors.New("this save does not hold all three of the character's actors (controller, body and player state), so a move to another map is refused: moving only some of them would split the character between two maps")
		}
		c, _ := toInt(r[0]["c"])
		b, _ := toInt(r[0]["b"])
		s, _ := toInt(r[0]["s"])
		if _, err := m.Exec(`update actors set map=?, dimension_index=0, location_x=?, location_y=?, location_z=? where id in (?,?,?)`, dest, x, y, z, c, b, s); err != nil {
			return err
		}
		if hasYaw {
			qx, qy, qz, qw := yawQuaternion(yaw)
			if _, err := m.Exec(`update actors set rotation_x=?, rotation_y=?, rotation_z=?, rotation_w=? where id=?`, qx, qy, qz, qw, b); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return ok(), nil
}
