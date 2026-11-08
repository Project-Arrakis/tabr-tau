package ops

import (
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/schema"
)

// The game's lifecycle state of a vehicle (#403). Every actor has an integer state (actors.state, 0 for an ordinary actor). Read from real saves (24
// saves of one played character, the state of every vehicle listed in backup_vehicles or recovered_vehicles): a vehicle in the character's vehicle backup
// has state 2 and a vehicle stored for recovery has state 4, in every save in which the table row exists. The Dune Docker console names the states
// VehicleBackup and VehicleRecovery (and Travel, a vehicle in transit); the single-player file stores integers, and only these two are confirmed here. One
// older save has a vehicle with state 1 that the character still owned; it is not interpreted.
//
// A stored vehicle keeps its last position, and a vehicle in the vehicle backup keeps its owner record, so by ownership alone it looks like a vehicle standing in
// the world. It is stored when its state is 2 or 4, or when the character has a row for it in backup_vehicles or recovered_vehicles.
const (
	stateVehicleBackup   = schema.StateVehicleBackup
	stateVehicleRecovery = schema.StateVehicleRecovery
)

// storedVehicleSQL selects the ids of the vehicles that are stored (no parameters). A vehicle in the recovery table is "recovery" and one in the backup table
// "backup"; the state decides when there is no row.
const storedVehicleSQL = `select id from actors where id in (select id from vehicles) and state in (` + schema.StoredStates + `)
	union select vehicle_id from backup_vehicles union select vehicle_id from recovered_vehicles`

// storedKindBy says how a vehicle is stored: "backup" (in the vehicle backup), "recovery" (stored for recovery) or "" (not stored). The table row decides first, the state next.
// It takes the reader to use: inside an edit the save is locked, so the edit's own reader must be used (a read through the save would wait for itself).
func storedKindBy(one func(string, ...any) (save.Row, error), id int64) string {
	if r, _ := one(`select 1 x from recovered_vehicles where vehicle_id=?`, id); r != nil {
		return "recovery"
	}
	if r, _ := one(`select 1 x from backup_vehicles where vehicle_id=?`, id); r != nil {
		return "backup"
	}
	r, _ := one(`select state s from actors where id=?`, id)
	switch r["s"] {
	case int64(stateVehicleBackup):
		return "backup"
	case int64(stateVehicleRecovery):
		return "recovery"
	}
	return ""
}

// storedVehicles lists every vehicle of a character that is stored, one entry each, from the three places the save says so: the character's rows in recovered_vehicles, the
// character's row in backup_vehicles, and the character's own vehicles (ownedVehicleSQL) whose actor state says stored with no row for the character (#528). The kind of each
// entry is storedKindBy's, so the label on a vehicle's row and this list cannot differ. An entry carries vehicle_id, kind, label, type (from the actor's class, else the row's
// name) and, only where the save has them, vehicle_name, chassis_durability, time_stored, reason and customization_id. Recovered ones come first, oldest stored first.
func storedVehicles(q func(string, ...any) ([]save.Row, error), one func(string, ...any) (save.Row, error), character any) []map[string]any {
	out := []map[string]any{}
	seen := map[int64]bool{}
	add := func(r save.Row) {
		id, ok := r["vehicle_id"].(int64)
		if !ok || seen[id] {
			return
		}
		seen[id] = true
		kind := storedKindBy(one, id)
		e := map[string]any{"vehicle_id": id, "kind": kind, "label": storedLabel(kind)}
		if c, ok := r["class"].(string); ok && c != "" {
			e["type"] = vehicleType(shortClass(c))
		} else if n, ok := r["vehicle_name"].(string); ok && n != "" {
			e["type"] = vehicleType(n)
		}
		for _, k := range []string{"vehicle_name", "chassis_durability", "time_stored", "reason", "customization_id"} {
			if v, ok := r[k]; ok && v != nil {
				e[k] = v
			}
		}
		out = append(out, e)
	}
	rec, _ := q(`select r.vehicle_id, r.vehicle_name, r.time_stored, r.chassis_durability, r.customization_id, r.reason, a.class
		from recovered_vehicles r left join actors a on a.id=r.vehicle_id where r.character_id=? order by r.time_stored, r.vehicle_id`, character)
	for _, r := range rec {
		add(r)
	}
	bak, _ := q(`select b.vehicle_id, b.customization_id, a.class from backup_vehicles b left join actors a on a.id=b.vehicle_id where b.character_id=? order by b.vehicle_id`, character)
	for _, r := range bak {
		add(r)
	}
	st, _ := q(`select a.id vehicle_id, a.class from actors a where a.id in (`+ownedVehicleSQL+`) and a.id in (`+storedVehicleSQL+`)
		and a.id not in (select vehicle_id from recovered_vehicles where character_id=?) and a.id not in (select vehicle_id from backup_vehicles where character_id=?) order by a.id`, character, character, character)
	for _, r := range st {
		add(r)
	}
	return out
}

func storedLabel(kind string) string {
	switch kind {
	case "backup":
		return "In the vehicle backup"
	case "recovery":
		return "Stored for recovery"
	}
	return ""
}

// refuseStored is the refusal for an action that treats a vehicle as one standing in the world.
func refuseStored(one func(string, ...any) (save.Row, error), id int64, what string) error {
	kind := storedKindBy(one, id)
	if kind == "" {
		return nil
	}
	return apierr.New(apierr.ErrBadArgument, fmt.Sprintf("vehicle %d is stored (%s) and keeps only the place it was last in the world, so it cannot be %s. Take it out of storage in the game first.", id, strings.ToLower(storedLabel(kind)), what))
}
