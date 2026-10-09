package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The owned vehicle of vehicleWorld (201, with a Hull_1 module at 100 of 250), plus a second module with no known maximum, its
// fuel in the vehicle component, and a cargo hold with one stack. A vehicle's per-component holds have no inventory_type.
const vehicleDetailsWorld = vehicleWorld + `
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (11,201,'SandbikeChassis_1',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":2000.0}]}')),
  (12,201,'SandbikeEngine_1',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":900.0,"DecayedMaxDurability":1000.0}]}'));
insert into fgl_entities(entity_id,components) values (-201, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":90.5,"RemainingTimeUntilDespawn":0.0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (201,-201,'Actor');
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (8001,201,0,15,500.0),(8002,201,NULL,NULL,NULL);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (8101,8001,40,2,'CopperOre',0,1790000000,'{}',0);`

func TestVehicleDetailsConditionFuelComponentsAndCargo(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleDetailsWorld)}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	live := r.(map[string]any)["vehicles"].([]map[string]any)
	if len(live) != 1 {
		t.Fatalf("one owned vehicle: %v", live)
	}
	v := live[0]
	if v["type"] != "Sandbike" {
		t.Errorf("type: %v", v["type"])
	}
	// the lowest condition is the Hull at 100 of 250 = 40 %; the chassis has no known maximum and is not guessed
	if c, ok := v["condition"].(float64); !ok || c != 40 {
		t.Errorf("condition: %v", v["condition"])
	}
	if f, ok := v["fuel"].(float64); !ok || f != 90.5 {
		t.Errorf("fuel: %v", v["fuel"])
	}
	comps := v["components"].([]map[string]any)
	by := map[string]map[string]any{}
	for _, c := range comps {
		by[c["template_id"].(string)] = c
	}
	if e := by["SandbikeEngine_1"]; e["current"].(float64) != 900 || e["max"].(float64) != 1000 || e["percent"].(float64) != 90 {
		t.Errorf("engine: %v", e)
	}
	if ch := by["SandbikeChassis_1"]; ch["current"].(float64) != 2000 || ch["max"] != nil || ch["percent"] != nil {
		t.Errorf("a module with no known maximum shows its value and no percent: %v", ch)
	}
	cargo := v["cargo"].(map[string]any)
	items := cargo["items"].([]map[string]any)
	if cargo["inventory_id"].(int64) != 8001 || cargo["slots"].(int64) != 15 || len(items) != 1 || items[0]["template_id"] != "CopperOre" || items[0]["stack_size"].(int64) != 40 {
		t.Errorf("cargo hold is the inventory_type 0 one, not a per-component hold: %v", cargo)
	}
}

func TestVehicleWithoutDetailsStillListsAndNeverGuesses(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld)} // module data but no fuel entity and no cargo hold
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	v := r.(map[string]any)["vehicles"].([]map[string]any)[0]
	if v["fuel"] != nil || v["cargo"] != nil {
		t.Errorf("no fuel and no cargo to show: fuel=%v cargo=%v", v["fuel"], v["cargo"])
	}
	if !strings.Contains(v["type"].(string), "Sandbike") {
		t.Errorf("type: %v", v["type"])
	}
}
