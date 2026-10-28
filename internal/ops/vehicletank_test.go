package ops

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The tank follows the grade of the vehicle's chassis (#469): a Mk6 buggy holds 300, a Mk5 Sandbike 140. The numbers below are written a second time here, from the
// game's module table (CDT_BaseVehicleModules, field m_FuelCapacity of the Chassis rows), so a change to the catalog or to tankOf has to change a test.
// docs/design/2026-10-28-vehicle-fuel-tanks.md has the table and its source.
var chassisTanks = map[string]float64{
	"SandbikeChassis_0": 90, "SandbikeChassis_1": 100, "SandbikeChassis_2": 110, "SandbikeChassis_3": 120, "SandbikeChassis_4": 130, "SandbikeChassis_5": 140, "SandbikeChassis_6": 150,
	"BuggyChassis_0": 220, "BuggyChassis_3": 240, "BuggyChassis_4": 260, "BuggyChassis_5": 280, "BuggyChassis_6": 300,
	"OrnithopterLightChassis_0": 225, "OrnithopterLightChassis_4": 300, "OrnithopterLightChassis_5": 350, "OrnithopterLightChassis_6": 400,
	"OrnithopterMediumChassis_0": 450, "OrnithopterMediumChassis_5": 750, "OrnithopterMediumChassis_6": 850,
	"OrnithopterTransportChassis_0": 1350, "OrnithopterTransportChassis_6": 2500,
	"SandcrawlerChassis_0": 450, "SandcrawlerChassis_6": 750,
}

// classOfChassis is the blueprint name of the class each chassis belongs to.
func classOfChassis(t *testing.T, id string) string {
	t.Helper()
	m, ok := catalog.VehicleModuleFor(id)
	if !ok {
		t.Fatalf("%s is not in the catalog", id)
	}
	c, ok := catalog.VehicleClassFor(m.Class)
	if !ok {
		t.Fatalf("%s: no class %s", id, m.Class)
	}
	return c.PropsKey
}

func TestTankOfEveryCatalogChassisIsTheTableOfTheGame(t *testing.T) {
	for id, want := range chassisTanks {
		got, known := tankOf(classOfChassis(t, id), []string{id})
		if !known || got != want {
			t.Errorf("%s: %v (known %v), want %v", id, got, known, want)
		}
	}
	// the catalog has no chassis with a tank that this table lacks (the container vehicle has none), and none the other way round
	n := 0
	for _, key := range catalog.VehicleClassKeys() {
		for _, tpl := range mustClass(t, key).Templates {
			id := tpl.Modules["Chassis"]
			if _, ok := chassisTanks[id]; !ok && key != "ContainerVehicle" {
				t.Errorf("template %s of %s has a chassis %s that the table above lacks", tpl.Name, key, id)
			}
		}
	}
	for id := range chassisTanks {
		m, _ := catalog.VehicleModuleFor(id)
		if m.Kind != "Chassis" || m.Fuel != chassisTanks[id] {
			t.Errorf("%s: catalog kind %s fuel %v", id, m.Kind, m.Fuel)
		}
		n++
	}
	if n != 23 {
		t.Errorf("23 chassis rows, have %d", n)
	}
}

func mustClass(t *testing.T, key string) *catalog.VehicleClass {
	t.Helper()
	c, ok := catalog.VehicleClassFor(key)
	if !ok {
		t.Fatalf("no class %s", key)
	}
	return c
}

func TestTankOfFallsBackToTheClassAndNeverGuesses(t *testing.T) {
	cases := []struct {
		name  string
		class string
		mods  []string
		want  float64
		known bool
	}{
		{"no modules: the class's measured tank", "BP_Buggy_CHOAM_C", nil, 240, true},
		{"modules but no chassis", "BP_Sandbike_CHOAM_C", []string{"SandbikeHull_5", "SandbikeEngine_5"}, 100, true},
		{"a chassis the catalog does not know", "BP_Buggy_CHOAM_C", []string{"BuggyChassis_77"}, 240, true},
		{"a chassis row of the table that is no grade of a template (Mk7)", "BP_Buggy_CHOAM_C", []string{"BuggyChassis_7"}, 240, true},
		{"the chassis of another class is not this vehicle's tank", "BP_Sandbike_CHOAM_C", []string{"BuggyChassis_6"}, 100, true},
		{"the chassis wins over the class", "BP_Buggy_CHOAM_C", []string{"BuggyEngine_6", "BuggyChassis_6"}, 300, true},
		{"the container vehicle has no fuel", "BP_ContainerVehicle_C", []string{"ContainerVehicle"}, 0, false},
		{"a class neither knows", "BP_Mystery_C", []string{"MysteryChassis_3"}, 0, false},
		{"a class neither knows, no modules", "BP_Mystery_C", nil, 0, false},
	}
	for _, c := range cases {
		got, known := tankOf(c.class, c.mods)
		if got != c.want || known != c.known {
			t.Errorf("%s: %v / %v, want %v / %v", c.name, got, known, c.want, c.known)
		}
	}
}

// tankVehicle is a vehicle of the test save with the modules it has.
type tankVehicle struct {
	id    int
	class string
	mods  []string
	fuel  string
}

// tankOps is the refuel save with more vehicles, each owned by the character, with modules.
func tankOps(t *testing.T, vs []tankVehicle) *Ops {
	t.Helper()
	var fv []fuelVehicle
	var b strings.Builder
	mid := 90000
	for _, v := range vs {
		fv = append(fv, fuelVehicle{v.id, v.class, fuelRec(v.fuel), true})
		for _, m := range v.mods {
			mid++
			fmt.Fprintf(&b, "insert into vehicle_modules(id,vehicle_id,template_id,stats) values (%d,%d,'%s',jsonb('{\"FVehicleModuleDurabilityStats\":[[],{\"CurrentDurability\":100.0,\"DecayedMaxDurability\":250.0}]}'));\n", mid, v.id, m)
		}
	}
	return &Ops{S: testsave.PlayerWithSQL(t, refuelWorld(fv)+vehicleWorldFuel+b.String())}
}

// The real vehicles of the operator's saves (the rows the #401 tests use: 578, 1828, 1859, 1599, 1616, 1634) and the created Mk6 buggy, with the fuel they had.
var tankFleet = []tankVehicle{
	{578, "BP_MediumOrnithopter_CHOAM_C", []string{"OrnithopterMediumChassis_5", "OrnithopterMediumEngine_5", "OrnithopterMediumGenerator_5"}, "221.203079"},
	{1828, "BP_Sandbike_CHOAM_C", []string{"SandbikeChassis_5", "SandbikeEngine_5", "SandbikeGenerator_5"}, "0.0"},
	{1859, "BP_LightOrnithopter_Choam_C", []string{"OrnithopterLightChassis_4", "OrnithopterLightEngine_4"}, "275.75354"},
	{1599, "BP_SandCrawler_CHOAM_C", []string{"SandcrawlerChassis_6", "SandcrawlerEngine_Unique_Speed_06"}, "0.0"},
	{1616, "BP_TransportOrnithopter_CHOAM_C", []string{"OrnithopterTransportChassis_6", "OrnithopterTransportEngine_6"}, "100.0"},
	{1634, "BP_ContainerVehicle_C", []string{"ContainerVehicle"}, "0.0"},
	{143, "BP_Buggy_CHOAM_C", []string{"BuggyChassis_6", "BuggyEngine_Unique_Accelerate_06", "BuggyGenerator_6"}, "240.0"}, // the created Mk6 buggy after the old Refuel
	{144, "BP_Buggy_CHOAM_C", []string{"BuggyChassis_6", "BuggyGenerator_6"}, "75.0"},                                      // the created Mk6 buggy at a quarter
	{145, "BP_Buggy_CHOAM_C", []string{"BuggyEngine_3", "BuggyChassis_3"}, "100.0"},                                        // Mk3: the 240 the table always had
	{146, "BP_Sandbike_CHOAM_C", []string{"SandbikeChassis_1", "SandbikeHull_1"}, "50"},                                    // Mk1: still 100
	{147, "BP_Sandbike_CHOAM_C", []string{"SandbikeChassis_0"}, "13.7"},                                                    // the grade-0 vehicle the world places, owned
	{148, "BP_Sandbike_CHOAM_C", []string{"SandbikeChassis_1"}, "120.0"},                                                   // above its tank: never lowered
	{149, "BP_Buggy_CHOAM_C", []string{"BuggyChassis_77", "BuggyEngine_6"}, "10.0"},                                        // a chassis the catalog does not know: the class's 240
	{150, "BP_Mystery_C", []string{"MysteryChassis_3"}, "10.0"},                                                            // nothing knows it: left alone
	{151, "BP_Sandbike_CHOAM_C", []string{"BuggyChassis_6"}, "10.0"},                                                       // another class's chassis: the class's 100
	{152, "BP_Sandbike_CHOAM_C", []string{"SandbikeChassis_6"}, "10.0"},                                                    // Mk6 Sandbike
}

func TestRefuelFillsEachVehicleToTheTankOfItsChassis(t *testing.T) {
	o := tankOps(t, tankFleet)
	before := map[int]string{}
	for _, v := range tankFleet {
		before[v.id] = entityText(t, o, v.id)
	}
	r, err := o.RefuelVehicles()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	want := map[int]float64{578: 750, 1828: 140, 1859: 300, 1599: 750, 1616: 2500, 143: 300, 144: 300, 145: 240, 146: 100, 147: 90, 149: 240, 151: 100, 152: 150}
	for id, tank := range want {
		if got := fuelOf(t, o, id); got != tank {
			t.Errorf("vehicle %d: fuel %v, want %v", id, got, tank)
		}
	}
	// 148 is above its tank, 150 and 1634 have no tank anyone knows: not rewritten
	for _, id := range []int{148, 150, 1634} {
		if got := entityText(t, o, id); got != before[id] {
			t.Errorf("vehicle %d must not be touched:\n got %s\nwant %s", id, got, before[id])
		}
	}
	if got := fuelOf(t, o, 148); got != 120.0 {
		t.Errorf("a value above the tank is never lowered: %v", got)
	}
	// the sandbike 201 of vehicleWorld has no chassis row: the class's 100, as before
	if got := fuelOf(t, o, 201); got != 100.0 {
		t.Errorf("a vehicle with no chassis module falls back to its class: %v", got)
	}
	if res["filled"].(int) != len(want)+1 || res["unknown"].(int) != 2 {
		t.Errorf("counts: %v", res)
	}
	if sk := res["skippedUnknown"].([]string); len(sk) != 2 || sk[0] != "BP_ContainerVehicle_C x1" || sk[1] != "BP_Mystery_C x1" {
		t.Errorf("the vehicles nothing knows are reported: %v", sk)
	}
	// only the fuel number differs in a record that was written
	for _, v := range tankFleet {
		if tank, ok := want[v.id]; ok {
			w := strings.Replace(before[v.id], `"CurrentFuel":`+v.fuel, fmt.Sprintf(`"CurrentFuel":%.1f`, tank), 1)
			if got := entityText(t, o, v.id); got != w {
				t.Errorf("vehicle %d record\n got %s\nwant %s", v.id, got, w)
			}
		}
	}
	assertDBSound(t, o)

	// a second press finds nothing to do and writes nothing
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	r, err = o.RefuelVehicles()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["filled"].(int) != 0 {
		t.Fatalf("second press: %v", res)
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("a second press writes nothing and logs nothing")
	}
}

// The count beside the button uses the same tank: the Mk6 buggy at 240 is not full.
func TestVehicleFuelCountCountsAMk6BuggyAt240AsNotFull(t *testing.T) {
	o := tankOps(t, []tankVehicle{{143, "BP_Buggy_CHOAM_C", []string{"BuggyChassis_6"}, "240.0"}, {145, "BP_Buggy_CHOAM_C", []string{"BuggyChassis_3"}, "240.0"}})
	r, err := o.VehicleFuel()
	if err != nil {
		t.Fatal(err)
	}
	// 201 (Sandbike, 40 of 100) and the Mk6 buggy are not full; the Mk3 buggy at 240 is
	if m := r.(map[string]any); m["notFull"].(int) != 2 || m["total"].(int) != 3 {
		t.Fatalf("count: %v", m)
	}
}

// Every chassis of the catalog, one vehicle each, filled from empty: the tank is the chassis's, and the Vehicles tab says the same.
func TestRefuelAndTheVehiclesTabAgreeOnEveryClassAndGrade(t *testing.T) {
	var ids []string
	for id := range chassisTanks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var vs []tankVehicle
	for i, id := range ids {
		vs = append(vs, tankVehicle{500 + i, classOfChassis(t, id), []string{id}, "0.0"})
	}
	o := tankOps(t, vs)
	if _, err := o.RefuelVehicles(); err != nil {
		t.Fatal(err)
	}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	by := map[int64]map[string]any{}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		by[v["id"].(int64)] = v
	}
	for i, id := range ids {
		want := chassisTanks[id]
		if got := fuelOf(t, o, 500+i); got != want {
			t.Errorf("%s: filled to %v, want %v", id, got, want)
		}
		v := by[int64(500+i)]
		if v["fuelMax"] != want || !near(v["fuelPercent"], 100.0) {
			t.Errorf("%s: the tab says %v / %v, want %v / 100", id, v["fuelMax"], v["fuelPercent"], want)
		}
	}
}

// near compares two optional percents.
func near(a, b any) bool {
	x, ok1 := a.(float64)
	y, ok2 := b.(float64)
	if !ok1 || !ok2 {
		return a == b
	}
	return x-y < 1e-9 && y-x < 1e-9
}

func TestVehiclesTabShowsTheTankOfTheChassisAndNeverMoreThan100Percent(t *testing.T) {
	o := tankOps(t, tankFleet)
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	by := map[int64]map[string]any{}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		by[v["id"].(int64)] = v
	}
	cases := []struct {
		id      int64
		max     any
		percent any
	}{
		{143, 300.0, 80.0},        // the created Mk6 buggy at 240 of 300
		{1828, 140.0, 0.0},        // Mk5 Sandbike
		{146, 100.0, 50.0},        // Mk1 Sandbike
		{145, 240.0, 100.0 / 2.4}, // Mk3 buggy
		{148, 100.0, 100.0},       // above the tank: the percent is capped
		{149, 240.0, 10.0 / 2.4},  // chassis unknown to the catalog: the class's tank
		{150, nil, nil},           // nothing knows it: the number alone
		{1634, nil, nil},
	}
	for _, c := range cases {
		v := by[c.id]
		if v == nil {
			t.Fatalf("vehicle %d is not listed", c.id)
		}
		if v["fuelMax"] != c.max || !near(v["fuelPercent"], c.percent) {
			t.Errorf("vehicle %d: max %v percent %v, want %v / %v", c.id, v["fuelMax"], v["fuelPercent"], c.max, c.percent)
		}
		if p, ok := v["fuelPercent"].(float64); ok && p > 100 {
			t.Errorf("vehicle %d: %v percent", c.id, p)
		}
	}
	if v := by[150]; v["fuel"] == nil {
		t.Error("a vehicle with no known tank still shows its fuel")
	}
}

// The case of the issue: a Mk6 buggy built from the game's tables is refuelled to 300 from a low tank, and the tab reads 100 %.
func TestACreatedMk6BuggyIsRefuelledTo300(t *testing.T) {
	o := spawnOps(t)
	id, res := spawn(t, o, Args{"class": "Buggy", "template": "T6_Combat", "fuel": 0.25})
	if d := res["derived"].(map[string]any); d["fuelCapacity"] != 300.0 || d["fuel"] != 75.0 {
		t.Fatalf("the created vehicle: %v", d)
	}
	fuel := func() map[string]any {
		r, err := o.Vehicles()
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
			if v["id"] == id {
				return v
			}
		}
		t.Fatal("the created vehicle is not listed")
		return nil
	}
	if v := fuel(); v["fuel"] != 75.0 || v["fuelMax"] != 300.0 || !near(v["fuelPercent"], 25.0) {
		t.Fatalf("before: %v / %v / %v", v["fuel"], v["fuelMax"], v["fuelPercent"])
	}
	if _, err := o.RefuelVehicles(); err != nil {
		t.Fatal(err)
	}
	if v := fuel(); v["fuel"] != 300.0 || v["fuelMax"] != 300.0 || !near(v["fuelPercent"], 100.0) {
		t.Fatalf("after: %v / %v / %v", v["fuel"], v["fuelMax"], v["fuelPercent"])
	}
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	if _, err := o.RefuelVehicles(); err != nil {
		t.Fatal(err)
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("a second press writes nothing")
	}
	assertDBSound(t, o)
}
