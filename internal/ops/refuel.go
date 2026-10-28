package ops

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// vehicleTank is how much fuel a full tank holds for each vehicle blueprint, in the unit of the save's FVehicleComponent.CurrentFuel, measured once per class
// by filling a vehicle to a known value and reading the fuel gauge in the game. It is only the fallback: the tank follows the grade of the vehicle's chassis, so
// tankOf takes it from the vehicle's own chassis module first (#469). Each number here is the tank of the grade that was measured: Sandbike Mk1, Buggy Mk3,
// Light Ornithopter Mk4, Medium Ornithopter Mk5, Sand Crawler and Transport Ornithopter Mk6. A class that is in neither is left alone and reported, never
// guessed. The Container Vehicle has no fuel and is not listed.
var vehicleTank = map[string]float64{
	"BP_MediumOrnithopter_CHOAM_C":    750,
	"BP_LightOrnithopter_Choam_C":     300,
	"BP_Buggy_CHOAM_C":                240,
	"BP_Sandbike_CHOAM_C":             100,
	"BP_SandCrawler_CHOAM_C":          750,
	"BP_TransportOrnithopter_CHOAM_C": 2500,
}

// A vehicle's fuel lives in the vehicle's entity record, {"FVehicleComponent":[34,{"CurrentFuel":90.5,...}]} in fgl_entities.components, linked to the vehicle
// through actor_fgl_entities. Nothing else is written: not the actor's properties either (the console's refuel sets a spawn-time fraction there, m_InitialFuel, which a
// vehicle that already has a fuel record in its entity does not need), and the other keys of the component stay as the game wrote them.

// vehicleFuelQuery lists the vehicles the character owns (ownedVehicleSQL, the rule of Bring vehicle and Repair) with the fuel of each, one row per vehicle. A vehicle
// with no vehicle component in its entity has no entity columns.
const vehicleFuelQuery = `select a.id, a.class, e.entity_id eid,
		(select group_concat(m.template_id,'|') from vehicle_modules m where m.vehicle_id=a.id) mods,
		json_type(e.components,'$.FVehicleComponent[1].CurrentFuel') ftype,
		json_extract(e.components,'$.FVehicleComponent[1].CurrentFuel') fuel
	from actors a left join fgl_entities e on e.entity_id=(
		select l.entity_id from actor_fgl_entities l join fgl_entities f on f.entity_id=l.entity_id
		where l.actor_id=a.id and json_valid(f.components,8) and json_type(f.components,'$.FVehicleComponent') is not null order by l.entity_id limit 1)
	where a.id in (` + ownedVehicleSQL + `) order by a.id`

type fuelTarget struct {
	entity any
	class  string
	tank   float64
}

// fuelState is what a look at the character's vehicles finds.
type fuelState struct {
	total    int            // the vehicles the character owns
	todo     []fuelTarget   // vehicles of a known class whose fuel is below the tank
	byClass  map[string]int // todo by class
	full     int            // vehicles of a known class at or above the tank
	unknown  map[string]int // vehicles whose tank neither their chassis nor vehicleTank gives: left alone
	unknownN int
	noRecord int // vehicles of a known class with no readable fuel value: left alone
}

// blueprintClass is the blueprint name of an actor class path: "/Game/.../BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C" is "BP_Sandbike_CHOAM_C".
func blueprintClass(c any) string {
	s := fmt.Sprint(c)
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// lookAtVehicleFuel sorts the character's vehicles into what to fill and what to leave alone. query is the edit's own query when it runs inside an edit.
func lookAtVehicleFuel(query func(string, ...any) ([]save.Row, error), controller int64) (fuelState, error) {
	st := fuelState{byClass: map[string]int{}, unknown: map[string]int{}}
	rows, err := query(vehicleFuelQuery, controller)
	if err != nil {
		return st, err
	}
	for _, r := range rows {
		st.total++
		class := blueprintClass(r["class"])
		tank, known := tankOf(class, splitModuleIDs(r["mods"]))
		if !known {
			st.unknown[class]++
			st.unknownN++
			continue
		}
		cur, ok := toFloat(r["fuel"])
		if !ok || r["eid"] == nil {
			st.noRecord++
			continue
		}
		if cur >= tank {
			st.full++
			continue
		}
		st.todo = append(st.todo, fuelTarget{r["eid"], class, tank})
		st.byClass[class]++
	}
	return st, nil
}

func (st fuelState) skippedUnknown() []string {
	out := make([]string, 0, len(st.unknown))
	for k, n := range st.unknown {
		out = append(out, fmt.Sprintf("%s x%d", k, n))
	}
	sort.Strings(out)
	return out
}

// refuelVehiclesTx fills every vehicle in st.todo to its own tank (tankOf), inside the edit m. A value is only ever raised (todo holds nothing at or above the tank), and
// jsonb_set writes the one number, leaving the other keys of the component as they were.
func refuelVehiclesTx(m *save.Mut, controller int64) (fuelState, error) {
	st, err := lookAtVehicleFuel(m.Query, controller)
	if err != nil {
		return st, err
	}
	for _, t := range st.todo {
		if _, err := m.Exec(`update fgl_entities set components=jsonb_set(components,'$.FVehicleComponent[1].CurrentFuel',?) where entity_id=?`, t.tank, t.entity); err != nil {
			return st, err
		}
	}
	return st, nil
}

// refuelDesc is the write-log line of a refuel.
func refuelDesc(n int) string {
	if n == 1 {
		return "refuelled 1 vehicle"
	}
	return fmt.Sprintf("refuelled %d vehicles", n)
}

// answer is the answer of a refuel, alone or as a part of filling all liquids.
func (st fuelState) answer() map[string]any {
	return map[string]any{"ok": true, "filled": len(st.todo), "byClass": st.byClass, "alreadyFull": st.full, "total": st.total,
		"unknown": st.unknownN, "skippedUnknown": st.skippedUnknown(), "noFuelRecord": st.noRecord}
}

// VehicleFuel says how many of the character's own vehicles are not full, for the count beside the Refuel button.
func (o *Ops) VehicleFuel() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	st, err := lookAtVehicleFuel(o.S.Query, p.Controller)
	if err != nil {
		return nil, err
	}
	return map[string]any{"total": st.total, "notFull": len(st.todo), "unknown": st.unknownN, "noFuelRecord": st.noRecord}, nil
}

// RefuelVehicles fills the fuel of every vehicle the character owns to its tank, in one edit. A vehicle at or above its tank is not rewritten, a vehicle whose
// tank is not known (no chassis module in the catalog and a class that is not in vehicleTank) is left alone and reported, and so is a vehicle with no fuel record. Another owner's vehicles and the world's are never touched.
// The vehicles are read inside the edit, so an edit put back onto a newer file is not applied to a list made from an older one.
func (o *Ops) RefuelVehicles() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var st fuelState
	if _, err = o.S.Mutate("refuel vehicles", func(m *save.Mut) error {
		var rerr error
		if st, rerr = refuelVehiclesTx(m, p.Controller); rerr != nil {
			return rerr
		}
		m.Desc = refuelDesc(len(st.todo))
		return nil
	}); err != nil {
		return nil, err
	}
	return st.answer(), nil
}
