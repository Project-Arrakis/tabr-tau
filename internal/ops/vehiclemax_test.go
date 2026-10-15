package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// vehicleOwned is the player's own vehicle 201 (see vehicleWorld); modules are added after it.
const vehicleOwned = `
insert into actors(id,class,map) values (201,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin');
insert into vehicles(id) values (201);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'mine',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1);`

func moduleDur(t *testing.T, o *Ops, id int) (cur float64, decayed any) {
	t.Helper()
	r := mustOne(t, o, `select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') c,
		json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') d from vehicle_modules where id=?`, id)
	return r["c"].(float64), r["d"]
}

// Audit L2 DBA-6 (#171): a repair only raises. A module that records a maximum above the table's value for its template (SandbikeEngine_6
// with 1200) is repaired to the recorded 1200 and its recorded maximum is not touched; it used to come out 1000/1000.
func TestVehicleRepairNeverLowersARecordedMaximum(t *testing.T) {
	st := func(body string) string {
		return `jsonb('{"FVehicleModuleDurabilityStats":[[],{` + body + `}]}')`
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleOwned+`
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (1,201,'SandbikeEngine_6',`+st(`"CurrentDurability":800.0,"DecayedMaxDurability":1200.0`)+`),
  (2,201,'SandbikeEngine_5',`+st(`"CurrentDurability":1300.0,"DecayedMaxDurability":1200.0`)+`),
  (3,201,'SandbikeEngine_4',`+st(`"CurrentDurability":1200.0,"DecayedMaxDurability":1200.0`)+`),
  (4,201,'SandbikeEngine_3',`+st(`"CurrentDurability":500.0,"DecayedMaxDurability":600.0`)+`),
  (5,201,'SandbikeEngine_2',`+st(`"CurrentDurability":500.0`)+`),
  (6,201,'SandbikeEngine_1',`+st(`"CurrentDurability":1000.0,"DecayedMaxDurability":1000.0`)+`),
  (7,201,'Mystery_9',`+st(`"CurrentDurability":10.0,"DecayedMaxDurability":2500.0`)+`);`)}
	r, err := o.RepairVehicles()
	if err != nil {
		t.Fatal(err)
	}
	// repaired: 1 (800 -> 1200), 4 (500/600 -> 1000/1000), 5 (500 -> 1000), 7 (10 -> its own 2500). Not repaired: 2 (1300 is above 1200),
	// 3 and 6 (already full)
	if got := r.(map[string]any)["repaired"].(int64); got != 4 {
		t.Fatalf("repaired %d modules, want 4: %v", got, r)
	}
	want := []struct {
		id  int
		cur float64
		dec any
	}{
		{1, 1200, 1200.0}, // raised to the larger recorded maximum, which is left as it is
		{2, 1300, 1200.0}, // above its maximum: untouched
		{3, 1200, 1200.0}, // full already
		{4, 1000, 1000.0}, // recorded maximum below the template's: restored to it
		{5, 1000, nil},    // no recorded maximum: none is added
		{6, 1000, 1000.0},
		{7, 2500, 2500.0}, // template the table does not know: its own recorded maximum
	}
	for _, w := range want {
		cur, dec := moduleDur(t, o, w.id)
		if cur != w.cur || dec != w.dec {
			t.Errorf("module %d: current %v decayed %v, want %v / %v", w.id, cur, dec, w.cur, w.dec)
		}
	}
	assertDBSound(t, o)
}

// Audit L2 QA-4 (#171): one golden (template, full durability) pair for every family-and-part rule of the durability table, written out by
// hand from the rule in upkeep.go (a family base; a chassis twice it; a hull, cabin or spice container one and a half times; every other part
// once), so a wrong base or factor cannot hide behind the same code computing the expectation. The tiers cycle through 1 to 6 because the
// full durability is the same at every tier.
func TestVehicleDurabilityTableGolden(t *testing.T) {
	parts := []string{"Chassis", "Hull", "SpiceContainer", "SpiceHeader", "Engine", "Generator", "Locomotion", "Inventory", "Boost", "Launcher", "Scanner", "Mining", "Seat", "Passenger"}
	golden := []struct {
		family string
		max    [14]float64 // in the order of parts
	}{
		{"Sandbike", [14]float64{2000, 1500, 1500, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000}},
		{"Treadwheel", [14]float64{2000, 1500, 1500, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000}},
		{"Buggy", [14]float64{3500, 2625, 2625, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750}},
		{"OrnithopterLight", [14]float64{3500, 2625, 2625, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750, 1750}},
		{"OrnithopterMedium", [14]float64{4000, 3000, 3000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000}},
		{"OrnithopterTransport", [14]float64{6000, 4500, 4500, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000}},
		{"Sandcrawler", [14]float64{5000, 3750, 3750, 2500, 2500, 2500, 2500, 2500, 2500, 2500, 2500, 2500, 2500, 2500}},
	}
	var rows []string
	type exp struct {
		template string
		max      float64
	}
	var all []exp
	id := 0
	for _, f := range golden {
		for i, p := range parts {
			id++
			tpl := fmt.Sprintf("%s%s_%d", f.family, p, 1+id%6)
			all = append(all, exp{tpl, f.max[i]})
			rows = append(rows, fmt.Sprintf(`(%d,201,'%s',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":1.0}]}'))`, id, tpl))
		}
	}
	if len(all) != 98 {
		t.Fatalf("the table has 7 families x 14 parts = 98 rules, the test lists %d", len(all))
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleOwned+`insert into vehicle_modules(id,vehicle_id,template_id,stats) values `+strings.Join(rows, ",")+`;`)}
	r, err := o.RepairVehicles()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["repaired"].(int64) != 98 || res["withoutKnownMax"].(int64) != 0 {
		t.Fatalf("every module of every rule is repaired with a known maximum: %v", res)
	}
	for i, e := range all {
		cur, dec := moduleDur(t, o, i+1)
		if cur != e.max || dec != nil {
			t.Errorf("%s: repaired to %v (decayed %v), want %v and no recorded maximum added", e.template, cur, dec, e.max)
		}
	}
}
