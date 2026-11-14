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

// hubArrivalTolerance is how far (game units) each coordinate of a move into a hub may be from the hub's arrival point: the page sends the numbers it was given.
const hubArrivalTolerance = 1.0

// Destination is one place the character can be moved to.
type Destination struct {
	Label    string   `json:"label"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Z        float64  `json:"z"`
	Distance float64  `json:"distance"`           // flat distance from the character now, in game units; 0 (not given) for a map the character is not on
	Group    string   `json:"group,omitempty"`    // the heading of the list the place is under (tradepost, base, vehicle, respawn or taxi)
	Arrival  bool     `json:"arrival,omitempty"`  // X, Y, Z are the game's arrival point for the place (#515), not the marker's position; Distance is to this point
	Map      string   `json:"map,omitempty"`      // the map the place is on, given for a hub (#521): a move to it is a move into that map, which the page asks for when it teleports
	OtherMap bool     `json:"otherMap,omitempty"` // the place is on a map other than the one the character is on now: no distance is given, and the page shows the warning about a move between maps
	Facing   *float64 `json:"facing,omitempty"`
}

// TravelMap is a map the character can be moved to: the two the console offers for an offline player (Hagga Basin and the Deep Desert).
// The social hubs and the dungeons are instances with their own entry; a position in the save is not known to be enough to enter them, so the Map box does not offer them. Arrakeen and
// Harko Village are offered as places instead (#521), to their arrival points only, with the warning that a save edit is not proven to move the character into another map.
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

// where returns where the character is (characterPlace): the map the player is on is the controller's, the position the body's when the three actors agree. An answer with no map
// means the save does not say where the character is.
func (o *Ops) where(p player) (characterPlace, error) {
	cp, err := readCharacterPlace(o.S.One, p, true)
	if err != nil {
		return cp, err
	}
	if cp.Map == "" {
		return cp, apierr.New(apierr.ErrNotFound, "the character's place was not found in this save")
	}
	return cp, nil
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
	here, err := o.where(p)
	if err != nil {
		return nil, err
	}
	cur := here.Map
	px, py := here.pos()
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
	type listed struct {
		d    Destination
		dist bool // d.Distance means something: the place is on the map the character is on
	}
	var rows []listed
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
		d := Destination{Group: r.group, Label: r.label, X: fx, Y: fy, Z: fz, Arrival: r.arrival, Map: r.onMap, OtherMap: r.onMap != "" && r.onMap != cur}
		onCur := (r.onMap == "" && same) || (r.onMap != "" && r.onMap == cur) // the place is on the map the character is on
		if onCur {
			d.Distance = flatDistance(fx, fy, px, py)
		}
		rows = append(rows, listed{d, onCur})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if gi, gj := groupIndex(a.d.Group), groupIndex(b.d.Group); gi != gj {
			return gi < gj
		}
		if a.d.OtherMap != b.d.OtherMap {
			return !a.d.OtherMap // the places on the character's map before the hubs that are another map
		}
		if a.dist != b.dist {
			return a.dist // a place with a distance before one without (the save's order is kept among the others)
		}
		return a.dist && a.d.Distance < b.d.Distance
	})
	out := make([]Destination, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.d)
	}
	total := len(out)
	if len(out) > destinationLimit {
		out = out[:destinationLimit]
	}
	res := map[string]any{"map": mp, "sameMap": same, "maps": maps, "total": total, "destinations": out, "groups": groupsOf(out)}
	if here.Parked != nil {
		res["parked"] = here.Parked.Error() // the body is not where the player is: the distances are from the player (the controller), and a move of the character is refused or made on all three actors
	}
	return res, nil
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
	// The whole move is one edit that reads where the character is inside it (#654, G-008): the map the player is on (the controller's), whether the body is there and standing in the
	// world. The reads are kept with the edit, so an edit put back onto a newer file in which the character is on another map is held, not applied with the old map's numbers.
	_, err = o.S.Mutate(desc, func(m *save.Mut) error {
		here, err := readCharacterPlace(m.One, p, false)
		if err != nil {
			return err
		}
		if here.Parked != nil {
			// a body on another map than the player is moved by the move between maps, which writes all three actors to the map the person named; a move on one map would move the
			// body only and leave it where the player is not. A body that is not standing in the world (its state is not 0) is not moved by the editor.
			if !here.ParkedMap {
				return here.Parked
			}
			if dest == "" {
				return apierr.Newf(apierr.ErrBadArgument, "this save holds the character's body on another map (%s) than the one the player is on (%s), so a move on one map would move only the body. Choose the map to move to: the editor then moves the body, the controller and the player state there together. Nothing was changed.", orNone(mapOfRow(here.Body)), here.Map)
			}
		}
		if dest != "" && (dest != here.Map || here.Parked != nil) {
			m.Desc = teleportMapDesc(dest, x, y, z, yaw, hasYaw)
			return teleportToMap(m, p, dest, x, y, z, yaw, hasYaw)
		}
		if !hasYaw {
			_, err := m.Exec(`update actors set location_x=?, location_y=?, location_z=? where id=?`, x, y, z, p.Pawn)
			return err
		}
		qx, qy, qz, qw := yawQuaternion(yaw)
		m.Desc = desc + fmt.Sprintf(" facing %.0f degrees", yaw)
		_, err = m.Exec(`update actors set location_x=?, location_y=?, location_z=?, rotation_x=?, rotation_y=?, rotation_z=?, rotation_w=? where id=?`,
			x, y, z, qx, qy, qz, qw, p.Pawn)
		return err
	})
	if err != nil {
		return nil, err
	}
	return ok(), nil
}

func mapOfRow(r save.Row) string {
	if r == nil || r["map"] == nil {
		return ""
	}
	return fmt.Sprint(r["map"])
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
		if _, hub := hubArrival(s); !hub { // Arrakeen and Harko Village are accepted too, to their arrival point only (teleportToMap, #521)
			return "", apierr.New(apierr.ErrBadArgument, "map must be HaggaBasin, DeepDesert, Arrakeen or HarkoVillage")
		}
	}
	return s, nil
}

// teleportMapDesc is the journal line of a move to another map.
func teleportMapDesc(dest string, x, y, z, yaw float64, hasYaw bool) string {
	desc := fmt.Sprintf("teleport player to %s %.0f, %.0f, %.0f", dest, x, y, z)
	if hasYaw {
		desc += fmt.Sprintf(" facing %.0f degrees", yaw)
	}
	return desc
}

// teleportToMap moves the character to another map, inside the caller's edit. A save the game wrote keeps the character's three actors (the controller, the body and
// the player state) on the same map and at the same position, so a move between maps writes all three; a move on one map writes the body
// only, which is what was checked in the game. The position has to lie inside the destination map's own bounds (Hagga Basin's are much
// smaller than the Deep Desert's, so a Deep Desert position typed for Hagga Basin is caught; the reverse is not, and the places the page lists
// for a map are the safe way to choose). A move to the Deep Desert was checked in the game once (2026-10-27, one character, a throwaway
// save): the three actors were written as here and the game loaded the character there. No other move was checked. A move into a hub (Arrakeen, Harko Village; #521) is written the
// same way but is not known to work: the game may load the character in the hub or ignore the map in the save (the operator checks it on a copy).
func teleportToMap(m *save.Mut, p player, dest string, x, y, z, yaw float64, hasYaw bool) error {
	if cfg, ok := travelMapOf(dest); ok {
		if x < cfg.MinX || x > cfg.MaxX || y < cfg.MinY || y > cfg.MaxY {
			return apierr.Newf(apierr.ErrBadArgument, "X %.0f, Y %.0f is outside the %s map (X %.0f to %.0f, Y %.0f to %.0f)", x, y, cfg.Label, cfg.MinX, cfg.MaxX, cfg.MinY, cfg.MaxY)
		}
	} else if a, hub := hubArrival(dest); !hub {
		return apierr.New(apierr.ErrBadArgument, "map must be HaggaBasin, DeepDesert, Arrakeen or HarkoVillage")
	} else if math.Abs(x-a.X) > hubArrivalTolerance || math.Abs(y-a.Y) > hubArrivalTolerance || math.Abs(z-a.Z) > hubArrivalTolerance {
		// a hub's own bounds are not known, so the editor moves the character into a hub only to the game's arrival point for it, which Go to a place gives
		return apierr.Newf(apierr.ErrBadArgument, "the editor moves the character into %s only to the game's arrival point for it: choose it in Go to a place", hubName(dest))
	}
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
}
