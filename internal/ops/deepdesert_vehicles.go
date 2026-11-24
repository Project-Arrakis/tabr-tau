package ops

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/vehicles"
)

// Taking the player's vehicles out of the Deep Desert before a layout roll (#1174). A vehicle left there when the layout changes stands inside new terrain, so the roll gets each
// vehicle of the player that stands in the Deep Desert out first, in one of two ways, both measured on real saves:
//
//   - A Scout (the light ornithopter) or a Sandbike goes into the game's own vehicle backup. A real before/after pair of a Scout stored by the game shows exactly this and nothing else:
//     the vehicle's actor goes from state 0 to state 2, and ONE row is added to backup_vehicles (the character, the vehicle, and the vehicle's customization id, which is the
//     m_CustomizationId the vehicle's own properties already hold). The table allows one row per character (UNIQUE(character_id)), so only one vehicle can be stored this way, and
//     only when the character has no row there yet; a vehicle that cannot be stored is moved (below), and the answer says why.
//   - Every other vehicle (a buggy, a crawler, a carrier, an assault ornithopter, a transport) is moved to the character's own map by editing its actor's map, dimension and place,
//     exactly as Bring vehicle does it (a real test moved a vehicle from the Deep Desert to the Hagga Basin that way). The places come from placeLine: in front of the character's saved
//     facing, 10 m ahead for a ground vehicle, 30 m ahead and 7 m up for an aircraft, several in a line at least 12 m apart, clear of base pieces, claims and other vehicles. The
//     terrain is not in a save, so the place can still be inside rock; the roll's help page tells the person to log out facing open ground with room in front.
//
// Only the player's own vehicles are touched (ownedVehicleSQL, the owner rank of the permission record), only those that stand in the world (not storedVehicleSQL), and only those on the
// Deep Desert. The cargo and the inventories belong to the vehicle and are not touched: both ways are edits of the actor row (and one backup row). Everything is one edit of the save
// (Mutate: one transaction), read back inside it, and the edit fails as a whole, changing nothing, if anything read back differs from what was meant.

// vehicleStoreClasses are the class keys (kit/gamedata/vehicles) of the vehicles the game's own backup takes: the Scout and the Sandbike.
var vehicleStoreClasses = map[string]bool{"Sandbike": true, "OrnithopterLight": true}

// VehicleMove is one vehicle that leaves the Deep Desert: stored in the vehicle backup, or moved to the character's map.
type VehicleMove struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`             // the type, as the Vehicles list names it: Sandbike, Buggy, LightOrnithopter ...
	Action string `json:"action"`           // "store" or "move"
	Reason string `json:"reason,omitempty"` // a Scout or Sandbike that is moved instead of stored: why
	Class  string `json:"class"`            // the actor's class, as saved
	From   Spot   `json:"from"`
	Custom string `json:"customization,omitempty"` // store: the customization id written to backup_vehicles
	To     *Spot  `json:"to,omitempty"`            // move: where it goes
	Words  string `json:"words"`                   // one sentence for the person
	State  int64  `json:"state"`                   // the actor's state as read (a move keeps it, a store sets 2)
	Dim    int64  `json:"dimension"`               // the dimension a moved vehicle is written into: the character's
	prior  int64  // how many rows the character had in backup_vehicles before the edit (the same for every entry of a plan)
}

// Spot is a place in the save: a map and a point in centimetres.
type Spot struct {
	Map    string  `json:"map"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Phrase string  `json:"words,omitempty"` // for a spot the editor chose: where it is against the character
}

func (s Spot) coords() string {
	return fmt.Sprintf("%s, x %.0f, y %.0f, z %.0f", s.Map, s.X, s.Y, s.Z)
}

// classKeyOfActorClass is the key of the game's vehicle table for an actors.class ("" when it is none of them). The exact class path decides; the blueprint's own name decides for
// the same blueprint under another path (the editor's made-up test saves).
func classKeyOfActorClass(class any) string {
	c, _ := class.(string)
	if c == "" {
		return ""
	}
	short := strings.ToLower(shortClass(c))
	byName := ""
	for _, k := range vehicles.ClassKeys() {
		vc, ok := vehicles.ClassFor(k)
		if !ok {
			continue
		}
		if vc.ActorClass == c {
			return k
		}
		if strings.ToLower(shortClass(vc.ActorClass)) == short {
			byName = k
		}
	}
	return byName
}

// sizeOfVehicle is the footprint of the vehicle whose actors.class is class.
func sizeOfVehicle(class any) vehicleSize {
	if k := classKeyOfActorClass(class); k != "" {
		return sizeOfClassKey(k)
	}
	return sizeOfActorClass(class)
}

type oneReader = func(q string, args ...any) (save.Row, error)

// planDeepDesertVehicles works out what to do for each vehicle of the player that stands in the Deep Desert, with the readers it is given (the edit's own inside the edit, the save's
// for a dry run). It reads only. It refuses (an error, and nothing else is done) when it cannot place the character, when the character is in the Deep Desert, when a vehicle's map is
// not recorded, or when a vehicle has no clear spot.
func planDeepDesertVehicles(p player, query rowReader, one oneReader) ([]VehicleMove, error) {
	here, err := readCharacterPlace(one, p, true)
	if err != nil {
		return nil, err
	}
	if here.Parked != nil {
		return nil, apierr.Newf(apierr.ErrBadArgument, "the vehicles cannot be placed by the character: %s", here.Parked.Error())
	}
	if here.Map == "" || isDeepDesertMap(here.Map) {
		return nil, apierr.New(apierr.ErrBadArgument, "the character is in the Deep Desert (or the save does not say where), so the vehicles would be put there again. Take the character out first")
	}
	rows, err := query(`select v.id id, coalesce(a.class,'') class, coalesce(a.map,'') map, a.location_x x, a.location_y y, a.location_z z, coalesce(a.state,0) st
		from vehicles v left join actors a on a.id=v.id
		where v.id in (`+ownedVehicleSQL+`) and v.id not in (`+storedVehicleSQL+`) order by v.id`, p.Controller)
	if err != nil {
		return nil, err
	}
	hasBackup, err := one(`select 1 x from backup_vehicles where character_id=?`, p.Controller)
	if err != nil {
		return nil, err
	}
	slotTaken := hasBackup != nil
	prior := int64(0)
	if slotTaken {
		prior = 1 // one row for each character, by the table's own rule
	}
	var plan []VehicleMove
	var moving []int // the indexes in plan of the vehicles that are moved
	sizeOf := map[int]vehicleSize{}
	for _, r := range rows {
		m, _ := r["map"].(string)
		if m != "" && !isDeepDesertMap(m) {
			continue
		}
		id, ok := toInt(r["id"])
		if !ok {
			return nil, apierr.New(apierr.ErrBadArgument, "a vehicle of the player has no usable number; nothing was changed")
		}
		class, _ := r["class"].(string)
		key := classKeyOfActorClass(class)
		name := vehicleType(shortClass(class))
		if class == "" {
			name = "vehicle"
		}
		if m == "" {
			return nil, apierr.Newf(apierr.ErrBadArgument, "vehicle %d (%s) has no map recorded in the save, so it cannot be placed or told apart from one in the Deep Desert; nothing was changed", id, name)
		}
		st, _ := toInt(r["st"])
		v := VehicleMove{ID: id, Name: name, Class: class, State: st, Dim: here.Dim, prior: prior,
			From: Spot{Map: m, X: num(r["x"]), Y: num(r["y"]), Z: num(r["z"])}}
		v.Action = "move"
		if vehicleStoreClasses[key] {
			custom, why, err := customizationOf(one, id, key)
			switch {
			case err != nil:
				return nil, err
			case why != "":
				v.Reason = why
			case slotTaken:
				v.Reason = "the vehicle backup (one place for each character) already holds a vehicle"
			default:
				v.Action, v.Custom = "store", custom
				slotTaken = true
			}
		}
		plan = append(plan, v)
		if v.Action == "move" {
			moving = append(moving, len(plan)-1)
			sizeOf[len(plan)-1] = sizeOfVehicle(class)
		}
	}
	if len(moving) > 0 {
		// One line, ground vehicles first and aircraft after them (by number within each): an aircraft is put down in the air and the game settles it on the ground, so each
		// vehicle is placed clear of the one before it in the plan, and an aircraft goes 20 m farther out than a ground vehicle at the same step, so it never ends up over one.
		sort.SliceStable(moving, func(a, b int) bool { return !sizeOf[moving[a]].Flies && sizeOf[moving[b]].Flies })
		var sizes []vehicleSize
		var labels []string
		for _, k := range moving {
			sizes = append(sizes, sizeOf[k])
			labels = append(labels, fmt.Sprintf("%s (vehicle %d)", plan[k].Name, plan[k].ID))
		}
		ps, err := placeLineNamed(query, here.Map, here.Dim, poseOf(here.Body), sizes, labels, 0)
		if err != nil {
			return nil, err
		}
		for i, k := range moving {
			pl := ps[i]
			plan[k].To = &Spot{Map: pl.Map, X: pl.X, Y: pl.Y, Z: pl.Z, Phrase: pl.Words()}
		}
	}
	for i := range plan {
		plan[i].Words = plan[i].words()
	}
	return plan, nil
}

// customizationOf reads the customization id a vehicle of class key carries in its own properties (the id the game copies to backup_vehicles). why is a sentence when it cannot be read.
func customizationOf(one oneReader, id int64, key string) (custom, why string, err error) {
	vc, ok := vehicles.ClassFor(key)
	if !ok || vc.PropsKey == "" {
		return "", "its customization cannot be read", nil
	}
	r, err := one(`select json_extract(properties, ?) c from actors where id=?`, `$."`+vc.PropsKey+`".m_CustomizationId`, id)
	if err != nil {
		return "", "", err
	}
	c, _ := r["c"].(string)
	if strings.TrimSpace(c) == "" {
		return "", "the save does not record its customization, which the game's backup row carries", nil
	}
	return c, "", nil
}

// words is the sentence for one vehicle: what is done and where.
func (v VehicleMove) words() string {
	from := "from " + v.From.coords()
	if v.Action == "store" {
		return fmt.Sprintf("vehicle %d (%s): stored in your vehicle backup, the game's own store (it keeps its place, %s, but is out of the world)", v.ID, v.Name, from)
	}
	s := fmt.Sprintf("vehicle %d (%s): moved %s to %s, %s", v.ID, v.Name, from, v.To.coords(), v.To.Phrase)
	if v.Reason != "" {
		s += fmt.Sprintf(" (it is not stored because %s)", v.Reason)
	}
	return s
}

// PlanDeepDesertVehicles says what the roll would do with the player's vehicles in the Deep Desert. It reads and writes nothing (a dry run).
func (o *Ops) PlanDeepDesertVehicles() ([]VehicleMove, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	return planDeepDesertVehicles(p, o.S.Query, o.S.One)
}

// vehicleReadBack checks, with the edit's own reader, that the rows are what the plan meant. A variable only so a test can make it fail; nothing else changes it.
var vehicleReadBack = verifyVehicleMoves

// MoveDeepDesertVehicles takes the player's vehicles out of the Deep Desert: the plan of PlanDeepDesertVehicles, written in ONE edit and read back inside it. An error, from the plan,
// from a write or from the read-back, undoes the whole edit. It returns what was done. It changes the working copy; writing the save to disk is the caller's step (save.Flush).
func (o *Ops) MoveDeepDesertVehicles() ([]VehicleMove, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var done []VehicleMove
	_, err = o.S.Mutate("", func(m *save.Mut) error {
		plan, err := planDeepDesertVehicles(p, m.Query, m.One)
		if err != nil {
			return err
		}
		if len(plan) == 0 {
			return nil
		}
		stored, moved := 0, 0
		for _, v := range plan {
			switch v.Action {
			case "store":
				// the game's own store, as measured: the actor's state 0 -> 2 and one backup_vehicles row
				res, err := m.Exec(`update actors set state=? where id=? and id in (`+ownedVehicleSQL+`) and id not in (`+storedVehicleSQL+`) and state=?`, stateVehicleBackup, v.ID, p.Controller, v.State)
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n != 1 {
					return apierr.Newf(apierr.ErrBadArgument, "vehicle %d could not be stored (the save changed under the edit); nothing was changed", v.ID)
				}
				if _, err := m.Exec(`insert into backup_vehicles(character_id, vehicle_id, customization_id) values (?,?,?)`, p.Controller, v.ID, v.Custom); err != nil {
					return err
				}
				stored++
			case "move":
				// exactly what Bring vehicle writes: the map, the dimension and the place
				res, err := m.Exec(`update actors set map=?, dimension_index=?, location_x=?, location_y=?, location_z=?
					where id=? and id in (`+ownedVehicleSQL+`) and id not in (`+storedVehicleSQL+`)`, v.To.Map, v.Dim, v.To.X, v.To.Y, v.To.Z, v.ID, p.Controller)
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n != 1 {
					return apierr.Newf(apierr.ErrBadArgument, "vehicle %d could not be moved (the save changed under the edit); nothing was changed", v.ID)
				}
				moved++
			default:
				return fmt.Errorf("vehicle %d has an unknown action %q (a bug); nothing was changed", v.ID, v.Action)
			}
		}
		if err := vehicleReadBack(m.Query, p, plan); err != nil {
			return err
		}
		m.Desc = fmt.Sprintf("took %d vehicle(s) out of the Deep Desert (%d stored, %d moved)", len(plan), stored, moved)
		done = plan
		return nil
	})
	if err != nil {
		return nil, err
	}
	return done, nil
}

// VerifyVehicleMoves reads the save again and says whether every row of the plan is as it was meant: the check MoveDeepDesertVehicles makes inside its edit, made on the save as it is
// now (the layout roll makes it on the file it has written, opened afresh).
func (o *Ops) VerifyVehicleMoves(plan []VehicleMove) error {
	p, err := o.player()
	if err != nil {
		return err
	}
	return verifyVehicleMoves(o.S.Query, p, plan)
}

// verifyVehicleMoves reads back the actors rows and the backup rows of the plan and returns an error naming the first difference. A stored vehicle must be at state 2 at the place it
// stood, with one backup row (the character, the vehicle, the customization); a moved one must stand where the plan put it, in the plan's dimension, with its state as it was and no
// backup row; and the character's backup rows must be exactly those of the vehicles stored.
func verifyVehicleMoves(query rowReader, p player, plan []VehicleMove) error {
	stored := 0
	for _, v := range plan {
		rows, err := query(`select map, dimension_index d, location_x x, location_y y, location_z z, state st from actors where id=?`, v.ID)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return fmt.Errorf("read back: vehicle %d has %d actor rows, not 1", v.ID, len(rows))
		}
		a := rows[0]
		bak, err := query(`select character_id, customization_id from backup_vehicles where vehicle_id=?`, v.ID)
		if err != nil {
			return err
		}
		switch v.Action {
		case "store":
			st, _ := toInt(a["st"])
			if mapOfRow(a) != v.From.Map || num(a["x"]) != v.From.X || num(a["y"]) != v.From.Y || num(a["z"]) != v.From.Z || st != int64(stateVehicleBackup) {
				return fmt.Errorf("read back: the actor row of vehicle %d is not the stored one (it should be state %d at its old place)", v.ID, stateVehicleBackup)
			}
			if len(bak) != 1 {
				return fmt.Errorf("read back: vehicle %d has %d backup rows, not 1", v.ID, len(bak))
			}
			c, _ := bak[0]["customization_id"].(string)
			cid, _ := toInt(bak[0]["character_id"])
			if cid != p.Controller || c != v.Custom {
				return fmt.Errorf("read back: the backup row of vehicle %d is not (character %d, customization %q)", v.ID, p.Controller, v.Custom)
			}
			stored++
		case "move":
			st, _ := toInt(a["st"])
			d, _ := toInt(a["d"])
			if mapOfRow(a) != v.To.Map || d != v.Dim || num(a["x"]) != v.To.X || num(a["y"]) != v.To.Y || num(a["z"]) != v.To.Z || st != v.State {
				return fmt.Errorf("read back: the actor row of vehicle %d is not where it was put (%s, dimension %d, state %d)", v.ID, v.To.coords(), v.Dim, v.State)
			}
			if len(bak) != 0 {
				return fmt.Errorf("read back: vehicle %d was moved but has a backup row", v.ID)
			}
		default:
			return fmt.Errorf("read back: vehicle %d has an unknown action %q", v.ID, v.Action)
		}
	}
	n, err := query(`select count(*) n from backup_vehicles where character_id=?`, p.Controller)
	if err != nil {
		return err
	}
	prior := int64(0)
	if len(plan) > 0 {
		prior = plan[0].prior
	}
	if got, _ := toInt(n[0]["n"]); got != prior+int64(stored) {
		return fmt.Errorf("read back: the character has %d backup row(s), %d were meant", got, prior+int64(stored))
	}
	return nil
}

// VehicleMoveLines is one line for each vehicle of a plan, in the order of the plan, for a person to read.
func VehicleMoveLines(plan []VehicleMove) []string {
	var out []string
	for _, v := range plan {
		out = append(out, v.Words)
	}
	return out
}
