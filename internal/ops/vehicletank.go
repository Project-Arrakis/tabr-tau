package ops

import (
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
)

// tankOf is how much fuel a full tank of one vehicle holds, in the unit of FVehicleComponent.CurrentFuel (#469). The tank follows the grade of the vehicle's
// chassis, not its class: a Mk6 buggy holds 300 and a Mk3 buggy 240, a Mk5 Sandbike 140 and a Mk1 Sandbike 100. The save stores no tank size, so it is the one
// the game's module table gives the vehicle's own chassis module (vehicle_modules.template_id, looked up in the vehicle catalog). When the vehicle has no chassis
// module the catalog knows, the tank measured for its class (vehicleTank) is used; when neither knows it, known is false and the vehicle is left alone.
// moduleIDs are the template ids of the vehicle's modules; class is the blueprint name of the vehicle ("BP_Buggy_CHOAM_C").
func tankOf(class string, moduleIDs []string) (tank float64, known bool) {
	for _, id := range moduleIDs {
		m, ok := catalog.VehicleModuleFor(id)
		if !ok || m.Kind != "Chassis" || m.Fuel <= 0 {
			continue
		}
		// a chassis of another class than the vehicle's own is not this vehicle's tank
		if c, ok := catalog.VehicleClassFor(m.Class); ok && c.PropsKey != class {
			continue
		}
		return m.Fuel, true
	}
	if t, ok := vehicleTank[class]; ok && t > 0 {
		return t, true
	}
	return 0, false
}

// splitModuleIDs turns the '|'-joined template ids that vehicleFuelQuery returns into a list.
func splitModuleIDs(v any) []string {
	s, _ := v.(string)
	if s == "" {
		return nil
	}
	return strings.Split(s, "|")
}
