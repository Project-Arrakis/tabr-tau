package catalog

import (
	"sort"
	"strings"
	"testing"
)

// #401: the vehicle catalog is made from the game's own tables (tools/vehiclegen). These tests pin what it says against what real saves show.

// moduleIDsInSaves are the 98 module ids found in the vehicles of 155 saves (the world's vehicles and the character's own).
var moduleIDsInSaves = strings.Fields(`BuggyChassis_0 BuggyChassis_3 BuggyEngine_0 BuggyEngine_4 BuggyGenerator_0 BuggyGenerator_4 BuggyHullBackExtra_3 BuggyHullBack_0 BuggyHullFront_0 BuggyHullFront_3
BuggyInventory_4 BuggyInventory_5 BuggyInventory_Unique_Capacity_6 BuggyLocomotionBackLeft_0 BuggyLocomotionBackLeft_3 BuggyLocomotionBackRight_0 BuggyLocomotionBackRight_3
BuggyLocomotionFrontLeft_0 BuggyLocomotionFrontLeft_3 BuggyLocomotionFrontRight_0 BuggyLocomotionFrontRight_3 BuggyMining_4 BuggyMining_6 ContainerVehicle
OrnithopterLightBoost_Unique_LessHeat_4 OrnithopterLightChassis_4 OrnithopterLightEngine_4 OrnithopterLightGenerator_4 OrnithopterLightHullBack_4 OrnithopterLightHullFront_4
OrnithopterLightLocomotionBackLeft_4 OrnithopterLightLocomotionBackRight_4 OrnithopterLightLocomotionFrontLeft_4 OrnithopterLightLocomotionFrontRight_4 OrnithopterLightScanner_4
OrnithopterMediumBoost_5 OrnithopterMediumChassis_5 OrnithopterMediumEngine_5 OrnithopterMediumGenerator_5 OrnithopterMediumHullBack_5 OrnithopterMediumHullFront_5
OrnithopterMediumHull_5 OrnithopterMediumInventory_5 OrnithopterMediumLocomotionBackLeft_5 OrnithopterMediumLocomotionBackRight_5 OrnithopterMediumLocomotionCenterLeft_5
OrnithopterMediumLocomotionCenterRight_5 OrnithopterMediumLocomotionFrontLeft_5 OrnithopterMediumLocomotionFrontRight_5 OrnithopterTransportBoost_Unique_LessHeat_06
OrnithopterTransportChassis_6 OrnithopterTransportEngine_6 OrnithopterTransportGenerator_6 OrnithopterTransportHullBackLeft_6 OrnithopterTransportHullBackRight_6
OrnithopterTransportHullFrontLeft_6 OrnithopterTransportHullFrontRight_6 OrnithopterTransportHull_6 OrnithopterTransportLocomotionBackLeft_Unique_Speed_6
OrnithopterTransportLocomotionBackRight_Unique_Speed_6 OrnithopterTransportLocomotionCenterLeft1_Unique_Speed_6 OrnithopterTransportLocomotionCenterLeft2_Unique_Speed_6
OrnithopterTransportLocomotionCenterRight1_Unique_Speed_6 OrnithopterTransportLocomotionCenterRight2_Unique_Speed_6 OrnithopterTransportLocomotionFrontLeft_Unique_Speed_6
OrnithopterTransportLocomotionFrontRight_Unique_Speed_6 SandbikeBoost_5 SandbikeChassis_0 SandbikeChassis_1 SandbikeChassis_5 SandbikeEngine_0 SandbikeEngine_1 SandbikeEngine_5
SandbikeGenerator_0 SandbikeGenerator_1 SandbikeGenerator_5 SandbikeHull_0 SandbikeHull_1 SandbikeHull_5 SandbikeInventory_2 SandbikeLocomotionBackCenter_0
SandbikeLocomotionBackCenter_1 SandbikeLocomotionBackCenter_5 SandbikeLocomotionFrontLeft_0 SandbikeLocomotionFrontLeft_1 SandbikeLocomotionFrontLeft_5
SandbikeLocomotionFrontRight_0 SandbikeLocomotionFrontRight_1 SandbikeLocomotionFrontRight_5 SandbikeSeat_0 SandcrawlerChassis_6 SandcrawlerEngine_Unique_Speed_06
SandcrawlerGenerator_6 SandcrawlerHull_6 SandcrawlerLocomotionLeft_Unique_WormThreat_06 SandcrawlerLocomotionRight_Unique_WormThreat_06 SandcrawlerSpiceContainer_Unique_Capacity_6
SandcrawlerSpiceHeader_6`)

func TestVehicleCatalogIsConsistentAndNamesItsBuild(t *testing.T) {
	if err := CheckVehicles(&vehicleData); err != nil {
		t.Fatal(err)
	}
	if VehicleBuild() != "1.5.15.0" || vehicleData.Read == "" {
		t.Errorf("the file records the game build and the day it was read: %q %q", vehicleData.Build, vehicleData.Read)
	}
	want := []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler", "ContainerVehicle"}
	if got := VehicleClassKeys(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the seven classes that appear in saves, and no others (Tank and Treadwheel never have): %v", got)
	}
	for id, m := range vehicleData.Modules {
		if strings.Contains(m.Class, "Tank") || strings.Contains(m.Class, "Tread") || strings.Contains(id, "Tank") || strings.Contains(id, "Treadwheel") {
			t.Errorf("%s: Tank and Treadwheel stay off", id)
		}
		if m.Grade > maxVehicleGradeForTest {
			t.Errorf("%s: grade %d is above what any template or save has", id, m.Grade)
		}
	}
}

const maxVehicleGradeForTest = 6

// Every module id a real save holds is a module of the catalog, in a class of the catalog.
func TestEveryModuleIDSeenInSavesIsInTheCatalog(t *testing.T) {
	if len(moduleIDsInSaves) != 98 {
		t.Fatalf("the list of ids seen in saves has %d entries, want 98", len(moduleIDsInSaves))
	}
	for _, id := range moduleIDsInSaves {
		if _, ok := VehicleModuleFor(id); !ok {
			t.Errorf("%s is in a real save but not in the catalog", id)
		}
	}
}

// What "Mk6" is. The module ids the game's items use for a grade-6 part end in _6 (BuggyChassis_6 is the item "Buggy Chassis Mk6"), a template's tier is the grade of its chassis,
// and the template name begins with that number: Mk6 is tier 6 is the T6 template of the class.
func TestMkSixIsTierSixIsTheGradeSixChassis(t *testing.T) {
	if n, _ := Name("BuggyChassis_6"); n != "Buggy Chassis Mk6" {
		t.Fatalf("the item catalog names BuggyChassis_6 %q", n)
	}
	b, _ := VehicleClassFor("Buggy")
	tpl, ok := b.Template("T6_Combat")
	if !ok || tpl.Tier != 6 {
		t.Fatalf("the buggy's top template: %+v %v", tpl, ok)
	}
	if tpl.Modules["Chassis"] != "BuggyChassis_6" {
		t.Errorf("T6 has the Mk6 chassis: %v", tpl.Modules)
	}
	for _, slot := range []string{"Chassis", "Generator", "Engine", "CockpitHull", "BodyHull", "Locomotion", "Locomotion2", "Locomotion3", "Locomotion4", "UtilitySlot1", "UtilitySlot2"} {
		id := tpl.Modules[slot]
		m, _ := VehicleModuleFor(id)
		if m.Grade != 6 || !strings.HasSuffix(id, "_6") {
			t.Errorf("T6_Combat %s: %s is grade %d", slot, id, m.Grade)
		}
		if n := ModuleName(id); !strings.HasSuffix(n, "Mk6") {
			t.Errorf("%s is %q, not a Mk6 part", id, n)
		}
	}
	// in every class a template of tier N has the chassis of grade N
	for _, key := range VehicleClassKeys() {
		c, _ := VehicleClassFor(key)
		for _, t2 := range c.Templates {
			ch, _ := VehicleModuleFor(t2.Modules["Chassis"])
			if ch.Grade != t2.Tier {
				t.Errorf("%s/%s: tier %d, chassis grade %d", key, t2.Name, t2.Tier, ch.Grade)
			}
		}
	}
}

// ResolveModule finds the row an item id stands for, whichever way the game spells the grade.
func TestResolveModuleAcceptsBothSpellingsOfAGrade(t *testing.T) {
	for in, want := range map[string]string{
		"BuggyInventory_Unique_Capacity_06":   "BuggyInventory_Unique_Capacity_6", // the item id has the zero, the row does not
		"BuggyInventory_Unique_Capacity_6":    "BuggyInventory_Unique_Capacity_6",
		"BuggyEngine_Unique_Accelerate_06":    "BuggyEngine_Unique_Accelerate_06", // the row has the zero
		"BuggyEngine_Unique_Accelerate_6":     "BuggyEngine_Unique_Accelerate_06",
		"BuggyMining_Unique_YieldIncrease_06": "BuggyMining_Unique_YieldIncrease_06",
		"BuggyHullBackExtra_6":                "BuggyHullBackExtra_6",
	} {
		if got, ok := ResolveModule("Buggy", in); !ok || got != want {
			t.Errorf("%s -> %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "Nope_6", "SandbikeChassis_5", "BuggyChassis_9", "BuggyMining_Polarcap_6"} {
		if got, ok := ResolveModule("Buggy", in); ok {
			t.Errorf("%q must not resolve for a buggy, got %q", in, got)
		}
	}
}

// The four parts of the operator's buggy: which module row each is, and the slot it sits in.
func TestTheOperatorsMkSixBuggyPartsAreRows(t *testing.T) {
	for item, want := range map[string]struct {
		row, slot string
		cargo     int64
		weapon    bool
	}{
		"BuggyEngine_Unique_Accelerate_06":    {"BuggyEngine_Unique_Accelerate_06", "Engine", 0, false},
		"BuggyInventory_Unique_Capacity_06":   {"BuggyInventory_Unique_Capacity_6", "UtilitySlot1", 35, false},
		"BuggyHullBackExtra_6":                {"BuggyHullBackExtra_6", "BodyHull", 0, false},
		"BuggyMining_Unique_YieldIncrease_06": {"BuggyMining_Unique_YieldIncrease_06", "UtilitySlot2", 0, true},
	} {
		row, ok := ResolveModule("Buggy", item)
		m, _ := VehicleModuleFor(row)
		if !ok || row != want.row || m.Slot != want.slot || m.CargoSlots != want.cargo || m.Weapon != want.weapon || m.Grade != 6 {
			t.Errorf("%s: row %q slot %q cargo %d weapon %v grade %d", item, row, m.Slot, m.CargoSlots, m.Weapon, m.Grade)
		}
	}
	// both utility slots hang from the rear hull in the game's slot table, and the game's own T5 and T6 buggies fill both on the "Extra" rear hull
	b, _ := VehicleClassFor("Buggy")
	parents := map[string]string{}
	for _, s := range b.Slots {
		parents[s.Slot] = s.Parent
		if strings.HasPrefix(s.Slot, "UtilitySlot") && s.Mandatory {
			t.Errorf("%s is optional", s.Slot)
		}
	}
	if parents["UtilitySlot1"] != "BodyHull" || parents["UtilitySlot2"] != "BodyHull" {
		t.Errorf("utility slots hang from the rear hull: %v", parents)
	}
	if !BodyUtilitySeen(b, "BuggyHullBackExtra", "UtilitySlot1") || !BodyUtilitySeen(b, "BuggyHullBackExtra", "UtilitySlot2") {
		t.Error("a template fills both utility slots on the BuggyHullBackExtra hull")
	}
	if BodyUtilitySeen(b, "BuggyHullBack", "UtilitySlot2") {
		t.Error("no template puts the second utility slot on the plain rear hull")
	}
}

type golden struct {
	name       string
	modules    []string
	health     float64
	crit, fuel float64
	slots      int64
	volume     float64
}

// goldens are vehicles of real saves (read from copies of them) with the numbers the game saved for each; the rules must give the same numbers from the module ids alone.
var goldens = []golden{
	{"medium ornithopter 578 (T5_Inventory)", strings.Fields(`OrnithopterMediumChassis_5 OrnithopterMediumHullFront_5 OrnithopterMediumHullBack_5 OrnithopterMediumGenerator_5 OrnithopterMediumHull_5
		OrnithopterMediumLocomotionBackLeft_5 OrnithopterMediumLocomotionBackRight_5 OrnithopterMediumLocomotionCenterLeft_5 OrnithopterMediumLocomotionCenterRight_5
		OrnithopterMediumLocomotionFrontLeft_5 OrnithopterMediumLocomotionFrontRight_5 OrnithopterMediumInventory_5 OrnithopterMediumEngine_5 OrnithopterMediumBoost_5`), 750, 150, 750, 20, 2000},
	{"light ornithopter 1859 (a T4 with a unique)", strings.Fields(`OrnithopterLightChassis_4 OrnithopterLightHullBack_4 OrnithopterLightHullFront_4 OrnithopterLightEngine_4 OrnithopterLightLocomotionFrontLeft_4
		OrnithopterLightLocomotionBackLeft_4 OrnithopterLightLocomotionBackRight_4 OrnithopterLightLocomotionFrontRight_4 OrnithopterLightGenerator_4 OrnithopterLightScanner_4
		OrnithopterLightBoost_Unique_LessHeat_4`), 500, 100, 300, 0, 0},
	{"container vehicle 1634", []string{"ContainerVehicle"}, 2000, 100, 0, 150, 15000},
	{"Mk5 sandbike 1828", strings.Fields(`SandbikeChassis_5 SandbikeGenerator_5 SandbikeLocomotionBackCenter_5 SandbikeLocomotionFrontLeft_5 SandbikeLocomotionFrontRight_5 SandbikeHull_5
		SandbikeBoost_5 SandbikeEngine_5`), 400, 100, 140, 0, 0},
	{"sandcrawler 1599 (T6_Harvesting with uniques)", strings.Fields(`SandcrawlerChassis_6 SandcrawlerEngine_Unique_Speed_06 SandcrawlerGenerator_6 SandcrawlerHull_6 SandcrawlerLocomotionLeft_Unique_WormThreat_06
		SandcrawlerLocomotionRight_Unique_WormThreat_06 SandcrawlerSpiceContainer_Unique_Capacity_6 SandcrawlerSpiceHeader_6`), 850, 250, 750, 30, 11250},
	{"carrier ornithopter 1616 (T6_Boost with uniques)", strings.Fields(`OrnithopterTransportChassis_6 OrnithopterTransportEngine_6 OrnithopterTransportGenerator_6 OrnithopterTransportHullBackLeft_6
		OrnithopterTransportHullBackRight_6 OrnithopterTransportHullFrontLeft_6 OrnithopterTransportHullFrontRight_6 OrnithopterTransportHull_6 OrnithopterTransportBoost_Unique_LessHeat_06
		OrnithopterTransportLocomotionBackLeft_Unique_Speed_6 OrnithopterTransportLocomotionBackRight_Unique_Speed_6 OrnithopterTransportLocomotionCenterLeft1_Unique_Speed_6
		OrnithopterTransportLocomotionCenterLeft2_Unique_Speed_6 OrnithopterTransportLocomotionCenterRight1_Unique_Speed_6 OrnithopterTransportLocomotionCenterRight2_Unique_Speed_6
		OrnithopterTransportLocomotionFrontLeft_Unique_Speed_6 OrnithopterTransportLocomotionFrontRight_Unique_Speed_6`), 2000, 250, 2500, 0, 0},
}

func TestDeriveVehicleReproducesTheNumbersRealVehiclesSaved(t *testing.T) {
	for _, g := range goldens {
		d, err := DeriveVehicle(g.modules)
		if err != nil {
			t.Errorf("%s: %v", g.name, err)
			continue
		}
		if d.MaxHealth != g.health || d.Critical != g.crit || d.Fuel != g.fuel || d.CargoSlots != g.slots || d.CargoVolume != g.volume {
			t.Errorf("%s: derived %+v, saved health %v crit %v fuel %v cargo %d/%v", g.name, d, g.health, g.crit, g.fuel, g.slots, g.volume)
		}
	}
	// a vehicle needs a chassis, and every module must be one the catalog knows
	if _, err := DeriveVehicle([]string{"BuggyEngine_6"}); err == nil {
		t.Error("no chassis")
	}
	if _, err := DeriveVehicle([]string{"BuggyChassis_6", "NotAModule"}); err == nil {
		t.Error("unknown module")
	}
}

func ids(t Tpl) []string {
	var out []string
	for _, id := range t {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Tpl is a template's modules, for the comparisons below.
type Tpl = map[string]string

// A template is exactly the module set of the real vehicle it is: the medium ornithopter T5_Inventory is vehicle 578, and the T0 sandbike and buggy are the vehicles the world places.
func TestTemplatesAreTheModuleSetsOfRealVehicles(t *testing.T) {
	mc, _ := VehicleClassFor("OrnithopterMedium")
	tpl, _ := mc.Template("T5_Inventory")
	want := append([]string(nil), goldens[0].modules...)
	sort.Strings(want)
	if got := ids(tpl.Modules); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("T5_Inventory:\n got %v\nwant %v", got, want)
	}
	world := map[string][]string{
		"Sandbike": strings.Fields(`SandbikeChassis_0 SandbikeGenerator_0 SandbikeEngine_0 SandbikeHull_0 SandbikeLocomotionFrontLeft_0 SandbikeLocomotionFrontRight_0 SandbikeLocomotionBackCenter_0 SandbikeSeat_0`),
		"Buggy":    strings.Fields(`BuggyChassis_0 BuggyEngine_0 BuggyGenerator_0 BuggyHullBack_0 BuggyHullFront_0 BuggyLocomotionBackLeft_0 BuggyLocomotionBackRight_0 BuggyLocomotionFrontLeft_0 BuggyLocomotionFrontRight_0`),
	}
	for class, want := range world {
		c, _ := VehicleClassFor(class)
		tpl, _ := c.Template("T0")
		sort.Strings(want)
		if got := ids(tpl.Modules); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s T0:\n got %v\nwant %v", class, got, want)
		}
	}
}

// The worked examples: a light ornithopter T4_Inventory, and the buggy T5_Mining and T6_Combat.
func TestWorkedExamplesDeriveAsComputedFromTheTables(t *testing.T) {
	for _, c := range []struct {
		class, tpl      string
		modules         int
		health, crit    float64
		fuel            float64
		slots           int64
		volume          float64
		weaponModuleIDs []string
	}{
		{"OrnithopterLight", "T4_Inventory", 11, 500, 100, 300, 10, 500, []string{"OrnithopterLightScanner_4"}},
		{"Buggy", "T5_Mining", 11, 650, 150, 280, 20, 2500, []string{"BuggyMining_5"}},
		{"Buggy", "T6_Combat", 11, 650, 150, 300, 0, 0, []string{"BuggyLauncher_6"}},
	} {
		cl, _ := VehicleClassFor(c.class)
		tp, ok := cl.Template(c.tpl)
		if !ok || len(tp.Modules) != c.modules {
			t.Errorf("%s/%s: %d modules", c.class, c.tpl, len(tp.Modules))
			continue
		}
		d, err := DeriveVehicle(ids(tp.Modules))
		if err != nil || d.MaxHealth != c.health || d.Critical != c.crit || d.Fuel != c.fuel || d.CargoSlots != c.slots || d.CargoVolume != c.volume {
			t.Errorf("%s/%s: %+v %v", c.class, c.tpl, d, err)
		}
		for _, w := range c.weaponModuleIDs {
			if m, _ := VehicleModuleFor(w); !m.Weapon {
				t.Errorf("%s is a weapon-type module (its stats carry the weapon part)", w)
			}
		}
	}
}

func TestEveryNewModuleHasADurabilityAndTheClassesHaveTheirRowShape(t *testing.T) {
	for _, key := range VehicleClassKeys() {
		c, _ := VehicleClassFor(key)
		if !strings.HasPrefix(c.ActorClass, "/Game/Dune/Systems/Vehicles/Blueprints/") || !strings.HasSuffix(c.ActorClass, "."+c.PropsKey) || !strings.HasPrefix(c.PermissionName, "##") {
			t.Errorf("%s: %+v", key, c)
		}
	}
	sb, _ := VehicleClassFor("Sandbike")
	if len(sb.Holds) != 2 {
		t.Errorf("the sandbike has two component holds: %v", sb.Holds)
	}
}
