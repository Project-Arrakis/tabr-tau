package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name string, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

type obj = map[string]any

// dumps builds small dumps of the five tables: every class has a chassis (grade 0 and 6), a utility module with cargo, and a template T0 with the grade-0 chassis; the buggy also has
// a unique and a grade-9 row, a faction row with no grade and a placeholder row, which are not offered; the container vehicle is one module and a template with no decoded list.
func dumps(t *testing.T) Inputs {
	dir := t.TempDir()
	mods, schem, tpls, wpn := obj{}, obj{}, obj{}, obj{}
	decoded := obj{}
	row := func(model, typ, class string, health, crit, fuel, slots, vol, dur float64) obj {
		return obj{"m_StaticData": obj{"VehicleModuleType": "EVehicleModuleType::" + typ, "VehicleModuleClass": "EVehicleModuleClass::" + class, "VehicleModel": "EVehicleModel::" + model,
			"Health": health, "CriticalStateHealth": crit, "m_FuelCapacity": fuel, "NumInventorySlots": slots, "m_MaxInventoryVolume": vol}, "m_DurabilityStats": obj{"MaxDurability": dur}}
	}
	for _, sh := range shapes {
		if sh.key == "ContainerVehicle" {
			mods["ContainerVehicle"] = row("Container", "Chassis", "Chassis", 2000, 0, 0, 150, 15000, 100)
			tpls[sh.templateRow] = obj{"m_VehicleActorClass": obj{"AssetPathName": "/Game/X/BP_ContainerVehicle.BP_ContainerVehicle_C"}, "m_Templates": []obj{{"Key": "Container"}}}
			schem[sh.schematic] = obj{"m_CraftingSchematic": obj{"m_VehicleSlots": []obj{{"Key": "EVehicleModuleType::Chassis", "Value": obj{"m_bIsMandatory": true, "m_ParentSlot": "EVehicleModuleType::Uninitialized"}}}}}
			continue
		}
		m := sh.model
		mods[m+"Chassis_0"] = row(m, "Chassis", "Chassis", 100, 40, 90, 0, 100, 1000)
		mods[m+"Chassis_6"] = row(m, "Chassis", "Chassis", 300, 100, 150, 0, 100, 3500)
		mods[m+"Inventory_6"] = row(m, "UtilitySlot1", "Utility", 0, 0, 1000, 10, 500, 1750)
		mods[m+"Inventory_Unique_Capacity_6"] = row(m, "UtilitySlot1", "Utility", 0, 0, 1000, 12, 700, 1750)
		wpn[m+"Inventory_6"] = obj{}
		tpls[sh.templateRow] = obj{"m_VehicleActorClass": obj{"AssetPathName": "/Game/X/BP_" + m + ".BP_" + m + "_C"}, "m_Templates": []obj{{"Key": "T0"}, {"Key": "T6_Inventory"}}}
		schem[sh.schematic] = obj{"m_CraftingSchematic": obj{"m_VehicleSlots": []obj{
			{"Key": "EVehicleModuleType::Chassis", "Value": obj{"m_bIsMandatory": true, "m_ParentSlot": "EVehicleModuleType::Uninitialized"}},
			{"Key": "EVehicleModuleType::UtilitySlot1", "Value": obj{"m_bIsMandatory": false, "m_ParentSlot": "EVehicleModuleType::Chassis"}}}}}
		decoded[sh.templateRow+"__slots"] = obj{
			"T0":           [][]string{{"Chassis", m + "Chassis_0"}},
			"T6_Inventory": [][]string{{"Chassis", m + "Chassis_6"}, {"UtilitySlot2", m + "Inventory_6"}}, // the decode's slot key is not the row's: the row's slot (UtilitySlot1) is used
		}
		if sh.key == "Buggy" {
			mods["BuggyChassis_9"] = row(m, "Chassis", "Chassis", 1, 1, 1, 0, 100, 1)
			mods["BuggyChoamShield"] = row(m, "UtilitySlot1", "Utility", 0, 0, 0, 0, 100, 100)
			mods["BuggyMining_Polarcap_6"] = row(m, "UtilitySlot1", "Utility", 0, 0, 0, 0, 100, 100)
		}
	}
	return Inputs{
		Modules: write(t, dir, "m.json", []obj{{"Rows": mods}}), Schematics: write(t, dir, "s.json", []obj{{"Rows": schem}}), Templates: write(t, dir, "t.json", []obj{{"Rows": tpls}}),
		Decoded: write(t, dir, "d.json", decoded), Weapons: write(t, dir, "w.json", []obj{{"Rows": wpn}}), Build: "9.9.9.9", Read: "2026-01-01",
	}
}

func TestGenerateMakesAConsistentCatalogFromDumps(t *testing.T) {
	d, err := Generate(dumps(t))
	if err != nil {
		t.Fatal(err)
	}
	if d.Build != "9.9.9.9" || len(d.Classes) != 7 {
		t.Fatalf("%q %d classes", d.Build, len(d.Classes))
	}
	b := d.Classes["Buggy"]
	if b.PropsKey != "BP_Buggy_C" || b.PermissionName != "##BuggyChoam" || len(b.Templates) != 2 {
		t.Errorf("buggy: %+v", b)
	}
	tpl := b.Templates[1]
	if tpl.Name != "T6_Inventory" || tpl.Tier != 6 || tpl.Modules["UtilitySlot1"] != "BuggyInventory_6" || tpl.Modules["UtilitySlot2"] != "" {
		t.Errorf("the module's own slot is used: %+v", tpl)
	}
	for _, id := range []string{"BuggyChassis_9", "BuggyChoamShield", "BuggyMining_Polarcap_6"} {
		if _, ok := d.Modules[id]; ok {
			t.Errorf("%s must not be offered", id)
		}
	}
	m := d.Modules["BuggyInventory_Unique_Capacity_6"]
	if !m.Unique || m.CargoSlots != 12 || m.CargoVolume != 700 || m.Durability != 1750 || m.Weapon {
		t.Errorf("unique: %+v", m)
	}
	if w := d.Modules["BuggyInventory_6"]; !w.Weapon {
		t.Errorf("a module in the weapon table is a weapon-type module: %+v", w)
	}
	if c := d.Modules["BuggyChassis_6"]; c.Health != 300 || c.Critical != 100 || c.Fuel != 150 {
		t.Errorf("chassis: %+v", c)
	}
	// the container vehicle: one template with no decoded list is the model's one module
	cv := d.Classes["ContainerVehicle"]
	if len(cv.Templates) != 1 || cv.Templates[0].Modules["Chassis"] != "ContainerVehicle" {
		t.Errorf("container: %+v", cv)
	}
	if _, ok := d.Classes["Tank"]; ok {
		t.Error("Tank stays off")
	}
}

func TestGenerateRefusesWhatItCannotPlaceOrCheck(t *testing.T) {
	in := dumps(t)
	// a template whose decoded list is missing
	b, _ := os.ReadFile(in.Decoded)
	var dec map[string]any
	_ = json.Unmarshal(b, &dec)
	delete(dec, "Buggy__slots")
	in.Decoded = write(t, t.TempDir(), "d.json", dec)
	if _, err := Generate(in); err == nil || !strings.Contains(err.Error(), "no decoded module list") {
		t.Errorf("a template without modules: %v", err)
	}
	// a module the table does not have
	in = dumps(t)
	b, _ = os.ReadFile(in.Decoded)
	_ = json.Unmarshal(b, &dec)
	dec["Buggy__slots"].(map[string]any)["T0"] = [][]string{{"Chassis", "NotAModule"}}
	in.Decoded = write(t, t.TempDir(), "d.json", dec)
	if _, err := Generate(in); err == nil || !strings.Contains(err.Error(), "not in the module table") {
		t.Errorf("a module the table lacks: %v", err)
	}
}
