package ops

import (
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Fill all liquids (issue #349): the water refill, the blood-sack fill and (unless the page's box is off) the refuel of the character's vehicles as ONE edit. The
// three parts are the same functions the single buttons use (refillContainersTx, fillBloodSacksTx, refuelVehiclesTx), run one after the other inside one
// transaction: one backup, one write-log line, and a failure in any part puts back all of them.

// countOf renders "1 water container" / "3 water containers".
func countOf(n int, one string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %ss", n, one)
}

// liquidsDesc is the write-log line of Fill all liquids; the vehicles part is left out when it was not asked for.
func liquidsDesc(water, blood, vehicles int, withVehicles bool) string {
	parts := []string{countOf(water, "water container"), countOf(blood, "blood sack")}
	if withVehicles {
		parts = append(parts, countOf(vehicles, "vehicle"))
	}
	return "filled all liquids: " + strings.Join(parts, ", ")
}

// FillAllLiquids fills the character's water containers and blood sacks and, unless args vehicles is false, refuels the vehicles the character owns, in one edit.
// Each part follows the rules of its own button (capacities, what is left alone, nothing lowered); the answer carries each part's own answer.
func (o *Ops) FillAllLiquids(a Args) (any, error) {
	withVehicles := true
	if v, has := a["vehicles"]; has {
		b, ok := v.(bool)
		if !ok {
			return nil, apierr.New(apierr.ErrBadArgument, "vehicles must be true or false")
		}
		withVehicles = b
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var water waterState
	var blood bloodSackState
	var fuel fuelState
	if _, err := o.S.Mutate("fill all liquids", func(m *save.Mut) error {
		fuel = fuelState{}
		var perr error
		if water, perr = refillContainersTx(m, p.Pawn); perr != nil {
			return perr
		}
		if blood, perr = fillBloodSacksTx(m, p.Pawn); perr != nil {
			return perr
		}
		if withVehicles {
			if fuel, perr = refuelVehiclesTx(m, p.Controller); perr != nil {
				return perr
			}
		}
		m.Desc = liquidsDesc(len(water.todo), len(blood.todo), len(fuel.todo), withVehicles)
		return nil
	}); err != nil {
		return nil, err
	}
	out := map[string]any{"ok": true, "water": water.answer(), "bloodSacks": blood.answer(), "vehiclesIncluded": withVehicles}
	if withVehicles {
		out["vehicles"] = fuel.answer()
	}
	return out, nil
}

// LiquidsStatus is the read-only count beside the Fill all liquids button: how many containers, sacks and vehicles are not full.
func (o *Ops) LiquidsStatus() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	water, err := lookAtWater(o.S.Query, p.Pawn)
	if err != nil {
		return nil, err
	}
	blood, err := lookAtBloodSacks(o.S.Query, p.Pawn)
	if err != nil {
		return nil, err
	}
	fuel, err := lookAtVehicleFuel(o.S.Query, p.Controller)
	if err != nil {
		return nil, err
	}
	unknownWater := 0
	for _, n := range water.unknown {
		unknownWater += n
	}
	return map[string]any{
		"water":      map[string]any{"total": len(water.todo) + water.full, "notFull": len(water.todo), "unknown": unknownWater},
		"bloodSacks": map[string]any{"total": blood.totalKnow, "notFull": len(blood.todo), "unknown": blood.unknownN, "holdingOther": blood.other},
		"vehicles":   map[string]any{"total": fuel.total, "notFull": len(fuel.todo), "unknown": fuel.unknownN, "noFuelRecord": fuel.noRecord},
	}, nil
}
