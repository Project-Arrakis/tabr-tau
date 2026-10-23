package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Refuel all vehicles (issue #349). The save is vehicleWorld (201 is the character's Sandbike, 202 a world Sandbike, 203 another owner's Buggy, 204 another
// owner's Buggy that the character only co-owns) with a fuel record added to each and more owned vehicles, one of each class of the tank table and the odd ones.

// fuelRec is the entity record of a vehicle: the fuel, a time, and a second component that must come through untouched.
func fuelRec(fuel string) string {
	return `{"FHealthComponent":[1,{"m_CurrentHealth":100.0}],"FVehicleComponent":[34,{"CurrentFuel":` + fuel + `,"RemainingTimeUntilDespawn":0.0}],"FOther":[7,{"Keep":"me"}]}`
}

type fuelVehicle struct {
	id    int
	class string // blueprint name
	rec   string // entity record; "" means the vehicle has no entity
	owned bool
}

// refuelWorld builds the SQL for the vehicles below (entity id = -id).
func refuelWorld(vs []fuelVehicle) string {
	var b strings.Builder
	b.WriteString(vehicleWorld)
	for _, v := range vs {
		fmt.Fprintf(&b, "insert into actors(id,class,map) values (%d,'/Game/%s.%s','HaggaBasin'); insert into vehicles(id) values (%d);\n", v.id, strings.TrimSuffix(v.class, "_C"), v.class, v.id)
		if v.owned {
			fmt.Fprintf(&b, "insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (%d,'mine',2,3,0); insert into permission_actor_rank(permission_actor_id,player_id,rank) values (%d,1,1);\n", v.id, v.id)
		}
		if v.rec != "" {
			fmt.Fprintf(&b, "insert into fgl_entities(entity_id,components) values (%d, jsonb('%s')); insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (%d,%d,'Actor');\n", -v.id, v.rec, v.id, -v.id)
		}
	}
	return b.String()
}

// The vehicles of vehicleWorld get an entity too.
const vehicleWorldFuel = `
insert into fgl_entities(entity_id,components) values (-201, jsonb('{"FHealthComponent":[1,{"m_CurrentHealth":100.0}],"FVehicleComponent":[34,{"CurrentFuel":40.0,"RemainingTimeUntilDespawn":0.0}],"FOther":[7,{"Keep":"me"}]}')),
  (-202, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":1.0}]}')), (-203, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":1.0}]}')), (-204, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":1.0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (201,-201,'Actor'),(202,-202,'Actor'),(203,-203,'Actor'),(204,-204,'Actor');`

var refuelVehicles = []fuelVehicle{
	{301, "BP_MediumOrnithopter_CHOAM_C", fuelRec("10.0"), true},
	{302, "BP_LightOrnithopter_Choam_C", fuelRec("20.5"), true},
	{303, "BP_Buggy_CHOAM_C", fuelRec("30.0"), true},
	{305, "BP_SandCrawler_CHOAM_C", fuelRec("0.0"), true},
	{306, "BP_TransportOrnithopter_CHOAM_C", fuelRec("100.0"), true},
	{307, "BP_ContainerVehicle_CHOAM_C", fuelRec("7.0"), true},                                         // class not in the tank table
	{308, "BP_Sandbike_CHOAM_C", fuelRec("100.0"), true},                                               // exactly full
	{309, "BP_Buggy_CHOAM_C", fuelRec("300.5"), true},                                                  // above the tank
	{310, "BP_Sandbike_CHOAM_C", fuelRec("50"), true},                                                  // the game wrote a whole number
	{311, "BP_Buggy_CHOAM_C", "", true},                                                                // no entity
	{312, "BP_Sandbike_CHOAM_C", `{"FVehicleComponent":[34,{"RemainingTimeUntilDespawn":0.0}]}`, true}, // a vehicle component without fuel
	{321, "BP_Buggy_CHOAM_C", fuelRec("5.0"), false},                                                   // no permission record: a world vehicle
}

func refuelOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, refuelWorld(refuelVehicles)+vehicleWorldFuel)}
}

func fuelOf(t *testing.T, o *Ops, id int) any {
	t.Helper()
	return mustOne(t, o, `select json_extract(components,'$.FVehicleComponent[1].CurrentFuel') f from fgl_entities where entity_id=?`, -id)["f"]
}

func entityText(t *testing.T, o *Ops, id int) string {
	t.Helper()
	return mustOne(t, o, `select json(components) c from fgl_entities where entity_id=?`, -id)["c"].(string)
}

func TestRefuelFillsEveryKnownClassToItsTankAndReportsTheRest(t *testing.T) {
	o := refuelOps(t)
	before := map[int]string{}
	for _, id := range []int{201, 202, 203, 204, 301, 302, 303, 305, 306, 307, 308, 309, 310, 312, 321} {
		before[id] = entityText(t, o, id)
	}
	others := digest(t, o, map[string]bool{"fgl_entities": true}, 0)

	r, err := o.RefuelVehicles()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	// filled: 201 (Sandbike 40), 301..303, 305, 306 and 310 (a whole-number 50)
	if res["filled"].(int) != 7 || res["alreadyFull"].(int) != 2 || res["total"].(int) != 12 || res["unknown"].(int) != 1 || res["noFuelRecord"].(int) != 2 {
		t.Fatalf("counts: %v", res)
	}
	byClass := res["byClass"].(map[string]int)
	want := map[string]int{"BP_Sandbike_CHOAM_C": 2, "BP_MediumOrnithopter_CHOAM_C": 1, "BP_LightOrnithopter_Choam_C": 1, "BP_Buggy_CHOAM_C": 1, "BP_SandCrawler_CHOAM_C": 1, "BP_TransportOrnithopter_CHOAM_C": 1}
	if len(byClass) != len(want) {
		t.Fatalf("by class: %v", byClass)
	}
	for k, n := range want {
		if byClass[k] != n {
			t.Fatalf("by class %s: %v", k, byClass)
		}
	}
	if sk := res["skippedUnknown"].([]string); len(sk) != 1 || sk[0] != "BP_ContainerVehicle_CHOAM_C x1" {
		t.Fatalf("the class that is not in the table is reported: %v", sk)
	}

	// each class is filled to the tank the table says, written as a decimal like the game does
	for id, tank := range map[int]float64{201: 100, 301: 750, 302: 300, 303: 240, 305: 750, 306: 2500, 310: 100} {
		if got := fuelOf(t, o, id); got != tank {
			t.Errorf("vehicle %d: fuel %v, want %v", id, got, tank)
		}
		if ty := mustOne(t, o, `select json_type(components,'$.FVehicleComponent[1].CurrentFuel') ty from fgl_entities where entity_id=?`, -id)["ty"]; ty != "real" {
			t.Errorf("vehicle %d: the fuel is stored as %v, want a decimal", id, ty)
		}
	}
	// the other keys of the record come through byte for byte: only the one number differs
	for id, c := range map[int][2]string{201: {"40.0", "100.0"}, 301: {"10.0", "750.0"}, 302: {"20.5", "300.0"}, 303: {"30.0", "240.0"}, 305: {"0.0", "750.0"}, 306: {"100.0", "2500.0"}} {
		wantText := strings.Replace(before[id], `"CurrentFuel":`+c[0], `"CurrentFuel":`+c[1], 1)
		if got := entityText(t, o, id); got != wantText {
			t.Errorf("vehicle %d record\n got %s\nwant %s", id, got, wantText)
		}
	}
	// a full one, one above its tank, an unknown class, one without fuel and every vehicle that is not the character's: not rewritten
	for _, id := range []int{202, 203, 204, 307, 308, 309, 312, 321} {
		if got := entityText(t, o, id); got != before[id] {
			t.Errorf("vehicle %d must not be touched:\n got %s\nwant %s", id, got, before[id])
		}
	}
	if got := fuelOf(t, o, 309); got != 300.5 {
		t.Errorf("a value above the tank is never lowered: %v", got)
	}
	if digest(t, o, map[string]bool{"fgl_entities": true}, 0) != others {
		t.Fatal("something outside the vehicle records changed")
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "refuelled 7 vehicles" {
		t.Fatalf("one write-log entry: %q", p)
	}
	assertDBSound(t, o)
}

func TestRefuelOfAFullFleetWritesNothing(t *testing.T) {
	o := refuelOps(t)
	if _, err := o.RefuelVehicles(); err != nil {
		t.Fatal(err)
	}
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	r, err := o.RefuelVehicles()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["filled"].(int) != 0 || res["alreadyFull"].(int) != 9 {
		t.Fatalf("second run: %v", res)
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("nothing to fill means nothing written and no log entry")
	}
}

func TestRefuelWithNoVehiclesOfTheCharacterIsANoOp(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld+vehicleWorldFuel)}
	// 201 is the only one that is the character's: take its owner record away
	if _, err := o.S.Mutate("fixture", func(m *save.Mut) error {
		_, err := m.Exec(`delete from permission_actor_rank where permission_actor_id=201`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	was := digest(t, o, nil, 0)
	r, err := o.RefuelVehicles()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["filled"].(int) != 0 || res["total"].(int) != 0 {
		t.Fatalf("answer: %v", res)
	}
	if digest(t, o, nil, 0) != was {
		t.Fatal("another owner's and the world's vehicles are never refuelled")
	}
}

func TestVehicleFuelCountsWhatTheButtonWouldFill(t *testing.T) {
	o := refuelOps(t)
	was := digest(t, o, nil, 0)
	r, err := o.VehicleFuel()
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["total"].(int) != 12 || m["notFull"].(int) != 7 || m["unknown"].(int) != 1 || m["noFuelRecord"].(int) != 2 {
		t.Fatalf("count: %v", m)
	}
	if digest(t, o, nil, 0) != was || o.S.Dirty() {
		t.Fatal("counting changes nothing")
	}
}

// Every class in the table is a blueprint the editor knows; the numbers are the measured ones.
func TestVehicleTankTableIsTheMeasuredOne(t *testing.T) {
	want := map[string]float64{"BP_MediumOrnithopter_CHOAM_C": 750, "BP_LightOrnithopter_Choam_C": 300, "BP_Buggy_CHOAM_C": 240, "BP_Sandbike_CHOAM_C": 100, "BP_SandCrawler_CHOAM_C": 750, "BP_TransportOrnithopter_CHOAM_C": 2500}
	if len(vehicleTank) != len(want) {
		t.Fatalf("table: %v", vehicleTank)
	}
	for k, v := range want {
		if vehicleTank[k] != v {
			t.Errorf("%s: %v, want %v", k, vehicleTank[k], v)
		}
	}
}
