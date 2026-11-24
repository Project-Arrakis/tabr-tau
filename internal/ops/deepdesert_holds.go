package ops

import (
	"fmt"
	"strings"
)

// What of the player's stands in the Deep Desert (#1167). A layout change puts new terrain under the Deep Desert; a base, a vehicle or the character that is left there is left inside
// it. The weekly layout roll asks this before it changes anything. It reads the save and writes nothing, in the save or anywhere else, and it fails closed: a thing whose map cannot
// be read counts as being in the Deep Desert, and a save that does not say who the player is cannot be cleared.

// isDeepDesertMap says whether a map name is the Deep Desert's (the save writes "DeepDesert"; a name that starts with it, such as its travel map, counts as well, in any case).
func isDeepDesertMap(m string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(m)), "deepdesert")
}

// HoldBase is a base of the player that stands in the Deep Desert (or whose map cannot be read).
type HoldBase struct {
	ID   int64   `json:"id"` // the totem's number, as the Bases list shows it
	Name string  `json:"name"`
	Map  string  `json:"map"` // "" when the save does not say
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// HoldVehicle is a vehicle of the player (stored or placed) that is in the Deep Desert (or whose map cannot be read).
type HoldVehicle struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Map    string  `json:"map"` // "" when the save does not say
	Stored bool    `json:"stored"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// HoldCharacter says where the character is when that is the Deep Desert (or cannot be told).
type HoldCharacter struct {
	Map   string  `json:"map"`   // the map of the actor that is in the Deep Desert; "" when none could be read
	Which string  `json:"which"` // the actor: the controller, the body or the player state
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

// DeepDesertHolds is the answer: what stands in the Deep Desert. Problems are things the check could not read, each of which blocks like a find.
type DeepDesertHolds struct {
	Bases     []HoldBase     `json:"bases"`
	Character *HoldCharacter `json:"character,omitempty"`
	Vehicles  []HoldVehicle  `json:"vehicles"`
	Problems  []string       `json:"problems"`
	Checked   struct {       // how much was looked at, so a run can say "0 of 12" and not only "none"
		Bases    int `json:"bases"`
		Vehicles int `json:"vehicles"`
	} `json:"checked"`
}

// Blocked is true when anything was found or could not be read.
func (h DeepDesertHolds) Blocked() bool {
	return len(h.Bases) > 0 || h.Character != nil || len(h.Vehicles) > 0 || len(h.Problems) > 0
}

// DeepDesertHolds reads the player's bases, the character's place and the player's vehicles, and lists those in the Deep Desert.
//
//   - Bases: the totems the player owns (the owner rank, 1, in the totem's permission record) that stand in the world (not picked up with the game's tool: actor state 5). Other
//     players' bases, and bases the player is only a member of, never count.
//   - Character: the controller, the body and the player state; any of them in the Deep Desert counts, and so does a controller whose map is not recorded.
//   - Vehicles: those the player owns (ownedVehicleSQL: the owner rank of the vehicle's permission record; the world's own vehicles have none), stored or placed, by the map of the
//     vehicle's actor. A vehicle with no actor or no map counts. Vehicles that exist only as a row of the recovery or backup table, with no actor, have no map and are not counted.
func (o *Ops) DeepDesertHolds() (DeepDesertHolds, error) {
	h := DeepDesertHolds{Bases: []HoldBase{}, Vehicles: []HoldVehicle{}, Problems: []string{}}
	p, err := o.player()
	if err != nil {
		if err == errNoPlayer {
			h.Problems = append(h.Problems, "the save has no player, so nothing can be said about where the player's bases, character and vehicles are")
			return h, nil
		}
		return h, err
	}

	// the character
	here, err := readCharacterPlace(o.S.One, p, true)
	if err != nil {
		return h, err
	}
	var where *HoldCharacter
	type seat struct {
		which string
		row   map[string]any
	}
	for _, s := range []seat{{"controller", here.Ctl}, {"body", here.Body}} {
		if s.row == nil {
			continue
		}
		if m := mapOfRow(s.row); isDeepDesertMap(m) {
			where = &HoldCharacter{Map: m, Which: s.which, X: num(s.row["x"]), Y: num(s.row["y"])}
			break
		}
	}
	if where == nil {
		st, err := o.S.One(`select a.map map, a.location_x x, a.location_y y from player_state ps join actors a on a.id=ps.player_state_id where ps.id=?`, p.Controller)
		if err != nil {
			return h, err
		}
		if st != nil {
			if m := mapOfRow(st); isDeepDesertMap(m) {
				where = &HoldCharacter{Map: m, Which: "player state", X: num(st["x"]), Y: num(st["y"])}
			}
		}
	}
	switch {
	case where != nil:
		h.Character = where
	case here.Ctl == nil || here.Map == "":
		h.Problems = append(h.Problems, "the save does not say which map the character is on")
	}

	// the bases
	bases, err := o.S.Query(`select t.id id, coalesce(a.map,'') map, a.location_x x, a.location_y y, coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type
		from totems t join actors a on a.id=t.id
		join permission_actor_rank r on r.permission_actor_id=t.id and r.player_id=? and r.rank=1
		left join permission_actor pa on pa.actor_id=t.id left join placeables tp on tp.id=t.id
		where coalesce(a.state,0)<>? order by t.id`, p.Controller, pickedUpState)
	if err != nil {
		return h, err
	}
	for _, b := range bases {
		h.Checked.Bases++
		m, _ := b["map"].(string)
		if m != "" && !isDeepDesertMap(m) {
			continue
		}
		id, ok := toInt(b["id"])
		if !ok {
			h.Problems = append(h.Problems, "a base of the player has no usable number")
			continue
		}
		name, _ := b["name"].(string)
		tt, _ := b["totem_type"].(string)
		h.Bases = append(h.Bases, HoldBase{ID: id, Name: backupDisplayName(name, tt), Map: m, X: num(b["x"]), Y: num(b["y"])})
	}

	// the vehicles
	vs, err := o.S.Query(`select v.id id, coalesce(a.class,'') class, coalesce(a.map,'') map, a.location_x x, a.location_y y, (v.id in (`+storedVehicleSQL+`)) stored
		from vehicles v left join actors a on a.id=v.id where v.id in (`+ownedVehicleSQL+`) order by v.id`, p.Controller)
	if err != nil {
		return h, err
	}
	for _, v := range vs {
		h.Checked.Vehicles++
		m, _ := v["map"].(string)
		if m != "" && !isDeepDesertMap(m) {
			continue
		}
		id, ok := toInt(v["id"])
		if !ok {
			h.Problems = append(h.Problems, "a vehicle of the player has no usable number")
			continue
		}
		name := fmt.Sprint(vehicleType(shortClass(v["class"])))
		if cls, _ := v["class"].(string); cls == "" {
			name = "vehicle"
		}
		stored, _ := toInt(v["stored"])
		h.Vehicles = append(h.Vehicles, HoldVehicle{ID: id, Name: name, Map: m, Stored: stored == 1, X: num(v["x"]), Y: num(v["y"])})
	}
	return h, nil
}
