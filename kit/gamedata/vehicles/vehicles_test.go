package vehicles

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// #401: the vehicle catalog is made from the game's own tables (tools/vehiclegen). These tests pin what it says against what real saves show.

// moduleIDsInSaves are the 98 module ids found in the vehicles of the saves studied (the world's vehicles and the character's own).
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
	if err := Check(data); err != nil {
		t.Fatal(err)
	}
	if Build() != "1.5.15.0" || data.Read == "" {
		t.Errorf("the file records the game build and the day it was read: %q %q", data.Build, data.Read)
	}
	want := []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler", "ContainerVehicle"}
	if got := ClassKeys(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the seven classes that appear in saves, and no others (Tank and Treadwheel never have): %v", got)
	}
	for id, m := range data.Modules {
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
		if _, ok := ModuleFor(id); !ok {
			t.Errorf("%s is in a real save but not in the catalog", id)
		}
	}
}

// What "Mk6" is. The module ids the game's items use for a grade-6 part end in _6 (BuggyChassis_6 is the game's Mk6 buggy chassis), a template's tier is the grade of its chassis,
// and the template name begins with that number: Mk6 is tier 6 is the T6 template of the class.
func TestMkSixIsTierSixIsTheGradeSixChassis(t *testing.T) {
	b, _ := ClassFor("Buggy")
	tpl, ok := b.Template("T6_Combat")
	if !ok || tpl.Tier != 6 {
		t.Fatalf("the buggy's top template: %+v %v", tpl, ok)
	}
	if tpl.Modules["Chassis"] != "BuggyChassis_6" {
		t.Errorf("T6 has the Mk6 chassis: %v", tpl.Modules)
	}
	for _, slot := range []string{"Chassis", "Generator", "Engine", "CockpitHull", "BodyHull", "Locomotion", "Locomotion2", "Locomotion3", "Locomotion4", "UtilitySlot1", "UtilitySlot2"} {
		id := tpl.Modules[slot]
		m, _ := ModuleFor(id)
		if m.Grade != 6 || !strings.HasSuffix(id, "_6") {
			t.Errorf("T6_Combat %s: %s is grade %d", slot, id, m.Grade)
		}
	}
	// in every class a template of tier N has the chassis of grade N
	for _, key := range ClassKeys() {
		c, _ := ClassFor(key)
		for _, t2 := range c.Templates {
			ch, _ := ModuleFor(t2.Modules["Chassis"])
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

// The four parts of a Mk6 buggy with uniques: which module row each is, and the slot it sits in.
func TestMkSixBuggyPartsAreRows(t *testing.T) {
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
		m, _ := ModuleFor(row)
		if !ok || row != want.row || m.Slot != want.slot || m.CargoSlots != want.cargo || m.Weapon != want.weapon || m.Grade != 6 {
			t.Errorf("%s: row %q slot %q cargo %d weapon %v grade %d", item, row, m.Slot, m.CargoSlots, m.Weapon, m.Grade)
		}
	}
	// both utility slots hang from the rear hull in the game's slot table, and the game's own T5 and T6 buggies fill both on the "Extra" rear hull
	b, _ := ClassFor("Buggy")
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

// goldens are vehicles as the game saves them, with the numbers it saved for each; the rules must give the same numbers from the module ids alone.
var goldens = []golden{
	{"medium ornithopter (T5_Inventory)", strings.Fields(`OrnithopterMediumChassis_5 OrnithopterMediumHullFront_5 OrnithopterMediumHullBack_5 OrnithopterMediumGenerator_5 OrnithopterMediumHull_5
		OrnithopterMediumLocomotionBackLeft_5 OrnithopterMediumLocomotionBackRight_5 OrnithopterMediumLocomotionCenterLeft_5 OrnithopterMediumLocomotionCenterRight_5
		OrnithopterMediumLocomotionFrontLeft_5 OrnithopterMediumLocomotionFrontRight_5 OrnithopterMediumInventory_5 OrnithopterMediumEngine_5 OrnithopterMediumBoost_5`), 750, 150, 750, 20, 2000},
	{"light ornithopter (a T4 with a unique)", strings.Fields(`OrnithopterLightChassis_4 OrnithopterLightHullBack_4 OrnithopterLightHullFront_4 OrnithopterLightEngine_4 OrnithopterLightLocomotionFrontLeft_4
		OrnithopterLightLocomotionBackLeft_4 OrnithopterLightLocomotionBackRight_4 OrnithopterLightLocomotionFrontRight_4 OrnithopterLightGenerator_4 OrnithopterLightScanner_4
		OrnithopterLightBoost_Unique_LessHeat_4`), 500, 100, 300, 0, 0},
	{"container vehicle", []string{"ContainerVehicle"}, 2000, 100, 0, 150, 15000},
	{"Mk5 sandbike", strings.Fields(`SandbikeChassis_5 SandbikeGenerator_5 SandbikeLocomotionBackCenter_5 SandbikeLocomotionFrontLeft_5 SandbikeLocomotionFrontRight_5 SandbikeHull_5
		SandbikeBoost_5 SandbikeEngine_5`), 400, 100, 140, 0, 0},
	{"sandcrawler (T6_Harvesting with uniques)", strings.Fields(`SandcrawlerChassis_6 SandcrawlerEngine_Unique_Speed_06 SandcrawlerGenerator_6 SandcrawlerHull_6 SandcrawlerLocomotionLeft_Unique_WormThreat_06
		SandcrawlerLocomotionRight_Unique_WormThreat_06 SandcrawlerSpiceContainer_Unique_Capacity_6 SandcrawlerSpiceHeader_6`), 850, 250, 750, 30, 11250},
	{"carrier ornithopter (T6_Boost with uniques)", strings.Fields(`OrnithopterTransportChassis_6 OrnithopterTransportEngine_6 OrnithopterTransportGenerator_6 OrnithopterTransportHullBackLeft_6
		OrnithopterTransportHullBackRight_6 OrnithopterTransportHullFrontLeft_6 OrnithopterTransportHullFrontRight_6 OrnithopterTransportHull_6 OrnithopterTransportBoost_Unique_LessHeat_06
		OrnithopterTransportLocomotionBackLeft_Unique_Speed_6 OrnithopterTransportLocomotionBackRight_Unique_Speed_6 OrnithopterTransportLocomotionCenterLeft1_Unique_Speed_6
		OrnithopterTransportLocomotionCenterLeft2_Unique_Speed_6 OrnithopterTransportLocomotionCenterRight1_Unique_Speed_6 OrnithopterTransportLocomotionCenterRight2_Unique_Speed_6
		OrnithopterTransportLocomotionFrontLeft_Unique_Speed_6 OrnithopterTransportLocomotionFrontRight_Unique_Speed_6`), 2000, 250, 2500, 0, 0},
}

func TestDeriveVehicleReproducesTheNumbersRealVehiclesSaved(t *testing.T) {
	for _, g := range goldens {
		d, err := Derive(g.modules)
		if err != nil {
			t.Errorf("%s: %v", g.name, err)
			continue
		}
		if d.MaxHealth != g.health || d.Critical != g.crit || d.Fuel != g.fuel || d.CargoSlots != g.slots || d.CargoVolume != g.volume {
			t.Errorf("%s: derived %+v, saved health %v crit %v fuel %v cargo %d/%v", g.name, d, g.health, g.crit, g.fuel, g.slots, g.volume)
		}
	}
	// a vehicle needs a chassis, and every module must be one the catalog knows
	if _, err := Derive([]string{"BuggyEngine_6"}); err == nil {
		t.Error("no chassis")
	}
	if _, err := Derive([]string{"BuggyChassis_6", "NotAModule"}); err == nil {
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
	mc, _ := ClassFor("OrnithopterMedium")
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
		c, _ := ClassFor(class)
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
		cl, _ := ClassFor(c.class)
		tp, ok := cl.Template(c.tpl)
		if !ok || len(tp.Modules) != c.modules {
			t.Errorf("%s/%s: %d modules", c.class, c.tpl, len(tp.Modules))
			continue
		}
		d, err := Derive(ids(tp.Modules))
		if err != nil || d.MaxHealth != c.health || d.Critical != c.crit || d.Fuel != c.fuel || d.CargoSlots != c.slots || d.CargoVolume != c.volume {
			t.Errorf("%s/%s: %+v %v", c.class, c.tpl, d, err)
		}
		for _, w := range c.weaponModuleIDs {
			if m, _ := ModuleFor(w); !m.Weapon {
				t.Errorf("%s is a weapon-type module (its stats carry the weapon part)", w)
			}
		}
	}
}

func TestEveryNewModuleHasADurabilityAndTheClassesHaveTheirRowShape(t *testing.T) {
	for _, key := range ClassKeys() {
		c, _ := ClassFor(key)
		if !strings.HasPrefix(c.ActorClass, "/Game/Dune/Systems/Vehicles/Blueprints/") || !strings.HasSuffix(c.ActorClass, "."+c.PropsKey) || !strings.HasPrefix(c.PermissionName, "##") {
			t.Errorf("%s: %+v", key, c)
		}
	}
	sb, _ := ClassFor("Sandbike")
	if len(sb.Holds) != 2 {
		t.Errorf("the sandbike has two component holds: %v", sb.Holds)
	}
}

// The fuel tanks of the chassis, written a second time here from the game's module table (CDT_BaseVehicleModules, field m_FuelCapacity of the Chassis rows, game build
// 1.5.15.x): 23 chassis of six vehicles. A change to the snapshot has to change this table too.
var chassisTanks = map[string]map[string]float64{
	"BP_Sandbike_CHOAM_C": {
		"SandbikeChassis_0": 90, "SandbikeChassis_1": 100, "SandbikeChassis_2": 110, "SandbikeChassis_3": 120, "SandbikeChassis_4": 130, "SandbikeChassis_5": 140, "SandbikeChassis_6": 150,
	},
	"BP_Buggy_CHOAM_C": {
		"BuggyChassis_0": 220, "BuggyChassis_3": 240, "BuggyChassis_4": 260, "BuggyChassis_5": 280, "BuggyChassis_6": 300,
	},
	"BP_LightOrnithopter_Choam_C": {
		"OrnithopterLightChassis_0": 225, "OrnithopterLightChassis_4": 300, "OrnithopterLightChassis_5": 350, "OrnithopterLightChassis_6": 400,
	},
	"BP_MediumOrnithopter_CHOAM_C": {
		"OrnithopterMediumChassis_0": 450, "OrnithopterMediumChassis_5": 750, "OrnithopterMediumChassis_6": 850,
	},
	"BP_TransportOrnithopter_CHOAM_C": {
		"OrnithopterTransportChassis_0": 1350, "OrnithopterTransportChassis_6": 2500,
	},
	"BP_SandCrawler_CHOAM_C": {
		"SandcrawlerChassis_0": 450, "SandcrawlerChassis_6": 750,
	},
}

func TestTheSnapshotHasTheCountsOfTheGamesTables(t *testing.T) {
	if len(data.Classes) != 7 || len(data.Modules) != 398 {
		t.Errorf("%d classes and %d modules, want 7 and 398", len(data.Classes), len(data.Modules))
	}
	templates, chassis, withTank := 0, 0, 0
	for _, c := range data.Classes {
		templates += len(c.Templates)
	}
	for _, m := range data.Modules {
		if m.Kind == "Chassis" {
			chassis++
			if m.Fuel > 0 {
				withTank++
			}
		}
	}
	if templates != 24 || chassis != 24 || withTank != 23 {
		t.Errorf("%d templates, %d chassis rows, %d with a tank; want 24, 24 and 23 (the Container vehicle has none)", templates, chassis, withTank)
	}
	if data.Tables == nil || data.Read == "" {
		t.Error("the snapshot names the tables and the day they were read")
	}
}

func TestChassisTanksAreTheGamesTableForEveryVehicleAndGrade(t *testing.T) {
	got := ChassisTanks()
	if len(got) != len(chassisTanks) {
		t.Errorf("%d vehicles, want %d: %v", len(got), len(chassisTanks), got)
	}
	rows := 0
	for bp, want := range chassisTanks {
		if len(got[bp]) != len(want) {
			t.Errorf("%s: %d chassis, want %d", bp, len(got[bp]), len(want))
		}
		for id, tank := range want {
			rows++
			if got[bp][id] != tank {
				t.Errorf("%s %s: %v, want %v", bp, id, got[bp][id], tank)
			}
			if f, ok := ChassisFuel(bp, id); !ok || f != tank {
				t.Errorf("ChassisFuel(%s, %s) = %v %v, want %v", bp, id, f, ok, tank)
			}
		}
	}
	if rows != 23 {
		t.Errorf("%d rows typed above, want 23", rows)
	}
	if _, ok := got["BP_ContainerVehicle_C"]; ok {
		t.Error("the Container vehicle has no tank and is not a key")
	}
	// the caller gets a copy
	got["BP_Buggy_CHOAM_C"]["BuggyChassis_6"] = 1
	if f, _ := ChassisFuel("BP_Buggy_CHOAM_C", "BuggyChassis_6"); f != 300 {
		t.Errorf("changing the copy changed the catalog: %v", f)
	}
	if ChassisTanks()["BP_Buggy_CHOAM_C"]["BuggyChassis_6"] != 300 {
		t.Error("changing the copy changed the next copy")
	}
}

func TestChassisFuelAnswersOnlyForAChassisWithATankOfThatVehicle(t *testing.T) {
	for name, c := range map[string]struct{ bp, id string }{
		"not a row":                    {"BP_Buggy_CHOAM_C", "BuggyChassis_77"},
		"a grade the game has no row":  {"BP_Buggy_CHOAM_C", "BuggyChassis_1"},
		"a module that is no chassis":  {"BP_Buggy_CHOAM_C", "BuggyEngine_6"},
		"the chassis of another":       {"BP_Sandbike_CHOAM_C", "BuggyChassis_6"},
		"the container vehicle":        {"BP_ContainerVehicle_C", "ContainerVehicle"},
		"an unknown vehicle":           {"BP_Mystery_C", "BuggyChassis_6"},
		"empty":                        {"", ""},
		"the blueprint without a tank": {"", "BuggyChassis_6"},
	} {
		if f, ok := ChassisFuel(c.bp, c.id); ok || f != 0 {
			t.Errorf("%s: %v %v", name, f, ok)
		}
	}
}

// Derive reads the same tanks: the fuel of a vehicle made from a template is the tank of its chassis.
func TestDeriveGivesTheTankOfTheChassis(t *testing.T) {
	for _, tanks := range chassisTanks {
		for id, tank := range tanks {
			m, _ := ModuleFor(id)
			if m.Kind != "Chassis" {
				t.Errorf("%s is a %s", id, m.Kind)
			}
			d, err := Derive([]string{id})
			if err != nil || d.Fuel != tank {
				t.Errorf("%s: %+v %v, want a tank of %v", id, d, err, tank)
			}
		}
	}
}

// The text of the snapshot is its canonical form, byte for byte, so a refreshed snapshot shows only what changed; the same text parses back to the same catalog.
func TestTheSnapshotIsInCanonicalFormAndRoundTrips(t *testing.T) {
	out, err := Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, snapshot) {
		t.Fatalf("vehicles.json is not the canonical text of its own content (%d bytes against %d); make it with Marshal", len(snapshot), len(out))
	}
	back, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Marshal(back)
	if !bytes.Equal(again, out) || len(back.Modules) != len(data.Modules) || len(back.Classes) != len(data.Classes) {
		t.Error("Marshal then Parse changed the catalog")
	}
	if bytes.Contains(snapshot, []byte("\r")) {
		t.Error("the snapshot has CRLF line ends (.gitattributes keeps it LF)")
	}
}

func TestParseRefusesWhatCheckRefusesAndUnknownFields(t *testing.T) {
	var d Data
	if err := json.Unmarshal(snapshot, &d); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Data){
		"no build": func(d *Data) { d.Build = "" },
		"a module of a class the file lacks": func(d *Data) {
			m := d.Modules["BuggyChassis_6"]
			m.Class = "Nope"
			d.Modules["BuggyChassis_6"] = m
		},
		"a module with no durability": func(d *Data) {
			m := d.Modules["BuggyChassis_6"]
			m.Durability = 0
			d.Modules["BuggyChassis_6"] = m
		},
		"a template naming a module that is not a row": func(d *Data) { d.Classes["Buggy"].Templates[0].Modules["Engine"] = "NoSuchEngine_0" },
	} {
		var c Data
		if err := json.Unmarshal(snapshot, &c); err != nil {
			t.Fatal(err)
		}
		mutate(&c)
		b, err := Marshal(&c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(b); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	extra := bytes.Replace(snapshot, []byte(`"build"`), []byte(`"surprise": 1, "build"`), 1)
	if _, err := Parse(extra); err == nil {
		t.Error("an unknown field was accepted")
	}
	if _, err := Parse([]byte("{}")); err == nil {
		t.Error("an empty catalog was accepted")
	}
	if _, err := Parse([]byte("not json")); err == nil {
		t.Error("garbage was accepted")
	}
}
