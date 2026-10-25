package ops

import (
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
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
	stateVehicleBackup   = 2
	stateVehicleRecovery = 4
)

// storedVehicleSQL selects the ids of the vehicles that are stored (no parameters). A vehicle in the recovery table is "recovery" and one in the backup table
// "backup"; the state decides when there is no row.
const storedVehicleSQL = `select id from actors where id in (select id from vehicles) and state in (2, 4)
	union select vehicle_id from backup_vehicles union select vehicle_id from recovered_vehicles`

// storedKind says how a vehicle is stored: "backup" (in the vehicle backup), "recovery" (stored for recovery) or "" (not stored). The table row decides first, the state next.
func (o *Ops) storedKind(id int64) string { return storedKindBy(o.S.One, id) }

// storedKindBy is storedKind with the reader to use: inside an edit the save is locked, so the edit's own reader must be used (a read through the save would wait for itself).
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
