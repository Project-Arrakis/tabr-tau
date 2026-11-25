package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The percent of the Vehicles tab is against the full durability a repair raises a module to (#1243): the table's value of its template,
// else the maximum its row records, the larger one when it has both. Each module of this vehicle is one case of that rule.
const durabilityModules = vehicleOwned + `
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (1,201,'BuggyEngine_6',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":1400.0}]}')),
  (2,201,'SandbikeEngine_5',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":900.0,"DecayedMaxDurability":900.0}]}')),
  (3,201,'SandbikeEngine_4',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":1010.0,"MaxDurability":1010.0,"DecayedMaxDurability":1010.0}]}')),
  (4,201,'Mystery_9',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":500.0,"DecayedMaxDurability":2000.0}]}')),
  (5,201,'Mystery_8',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":10.0}]}')),
  (6,201,'Mystery_7',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":10.0,"MaxDurability":100.0}]}')),
  (7,201,'SandbikeGenerator_1',jsonb('{"FVehicleModuleDurabilityStats":[[],{"LastDeteriorationCause":"x"}]}')),
  (8,201,'SandbikeHull_1',jsonb('{"FCustomizationStats":[[],{}]}'));`

func durabilityList(t *testing.T, o *Ops) (map[string]any, map[string]map[string]any) {
	t.Helper()
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	v := r.(map[string]any)["vehicles"].([]map[string]any)[0]
	by := map[string]map[string]any{}
	for _, c := range v["components"].([]map[string]any) {
		by[c["template_id"].(string)] = c
	}
	return v, by
}

func TestVehicleDurabilityUsesTheFullDurabilityARepairUses(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, durabilityModules)}
	v, by := durabilityList(t, o)
	num := func(x any) any {
		if f, ok := toFloat(x); ok {
			return f
		}
		return x
	}
	for _, w := range []struct {
		template              string
		current, max, percent any
	}{
		{"BuggyEngine_6", 1400.0, 1750.0, 80.0},     // only the current value is recorded: the table's full durability
		{"SandbikeEngine_5", 900.0, 1000.0, 90.0},   // a worn-down maximum is wear: a repair raises it to 1000
		{"SandbikeEngine_4", 1010.0, 1010.0, 100.0}, // a recorded maximum above the table's is the full one
		{"Mystery_9", 500.0, 2000.0, 25.0},          // a template the table does not know: its recorded maximum
		{"Mystery_8", 10.0, nil, nil},               // neither source knows the maximum
		{"Mystery_7", 10.0, nil, nil},               // a repair does not read MaxDurability, so neither does the percent
		{"SandbikeGenerator_1", 0.0, 1000.0, 0.0},   // a save leaves CurrentDurability out when it is 0
		{"SandbikeHull_1", nil, 1500.0, nil},        // no durability record: a repair leaves it out
	} {
		c := by[w.template]
		if num(c["current"]) != w.current || num(c["max"]) != w.max || c["percent"] != w.percent {
			t.Errorf("%s: current %v max %v percent %v, want %v / %v / %v", w.template, c["current"], c["max"], c["percent"], w.current, w.max, w.percent)
		}
	}
	// the vehicle's figure is that of its weakest part
	if v["condition"] != 0.0 {
		t.Errorf("weakest part: %v", v["condition"])
	}
	if o.S.Dirty() {
		t.Error("listing the vehicles changes nothing")
	}

	// agreement with the write: exactly the modules shown below 100 % are repaired, and every one with a percent then shows 100 %
	below := 0
	for _, c := range by {
		if p, ok := c["percent"].(float64); ok && p < 100 {
			below++
		}
	}
	r, err := o.RepairVehicles()
	if err != nil {
		t.Fatal(err)
	}
	if n := r.(map[string]any)["repaired"].(int64); n != int64(below) {
		t.Errorf("repaired %d modules, the list showed %d below 100 %%", n, below)
	}
	v, after := durabilityList(t, o)
	for name, c := range after {
		if (c["percent"] == nil) != (by[name]["percent"] == nil) || (c["percent"] != nil && c["percent"] != 100.0) {
			t.Errorf("%s after the repair: %v", name, c["percent"])
		}
	}
	if v["condition"] != 100.0 {
		t.Errorf("weakest part after the repair: %v", v["condition"])
	}
}
