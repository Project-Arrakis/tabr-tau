package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Power devices as in the Dune Docker console's refill, plus the windtrap rule of this editor:
//   - 901: a built omnidirectional wind turbine holding 100 + 16 Low-grade lubricant (becomes 100 x 4 + 99 = 499 in five rows)
//   - 902: a built omnidirectional turbine holding nothing
//   - 903: a placeholder (hologram) omnidirectional turbine: not built, must not be touched
//   - 904: a built directional turbine holding nothing (Industrial-grade)
//   - 911: a windtrap holding 5 Makeshift Filters (changed to the Standard Filter)
//   - 912: a large windtrap holding 5 Particulate Filters (changed to the Advanced Particulate Filter)
//   - 913: a windtrap holding nothing
//   - 914: a windtrap already full of Standard Filters
//   - 921: a placeholder cistern with a water component: its water must not be touched
const turbineWorld = `
insert into actors(id,class,map) values (901,'c','HaggaBasin'),(902,'c','HaggaBasin'),(903,'c','HaggaBasin'),(904,'c','HaggaBasin'),
  (911,'c','HaggaBasin'),(912,'c','HaggaBasin'),(913,'c','HaggaBasin'),(914,'c','HaggaBasin'),(921,'c','HaggaBasin');
insert into placeables(id,health,building_type,is_hologram) values
  (901,1,'WindTurbineOmnidirectional_Placeable',0),(902,1,'WindTurbineOmnidirectional_Placeable',0),(903,1,'WindTurbineOmnidirectional_Placeable',1),
  (904,1,'WindTurbineDirectional_Placeable',0),
  (911,1,'Windtrap_Placeable',0),(912,1,'LargeWindtrap_Placeable',0),(913,1,'Windtrap_Placeable',0),(914,1,'Windtrap_Placeable',0),
  (921,1,'WaterCistern_Placeable',1);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values
  (951,901,3,5,100.0),(952,902,3,5,100.0),(953,903,3,5,100.0),(954,904,3,5,100.0),
  (961,911,3,5,25.0),(962,912,3,5,25.0),(963,913,3,5,25.0),(964,914,3,5,25.0);
insert into fgl_entities(entity_id,components) values (-2001, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (921,-2001,'a');
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (971,951,100,0,'WindTurbineLubricant1',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (972,951,16,1,'WindTurbineLubricant1',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (973,961,5,0,'WindTrapFilter1',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (974,962,5,0,'WindTrapFilter3',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (975,964,5,0,'WindTrapFilter2',0,0,'{"FItemStackAndDurabilityStats":[[],{}]}',0);`

func stacks(t *testing.T, o *Ops, inv int) (tpl string, rows int64, total int64) {
	t.Helper()
	r := mustOne(t, o, `select coalesce(min(template_id),'') t, count(*) n, coalesce(sum(stack_size),0) s from items where inventory_id=?`, inv)
	return r["t"].(string), r["n"].(int64), r["s"].(int64)
}

func TestRefillFillsBuiltTurbinesLikeTheConsoleAndLeavesPlaceholdersAlone(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, turbineWorld)}
	if _, err := o.RefillGenerators(); err != nil {
		t.Fatal(err)
	}
	// 100 + 16 -> five rows of 100,100,100,100,99
	if tpl, n, total := stacks(t, o, 951); tpl != "WindTurbineLubricant1" || n != 5 || total != 499 {
		t.Fatalf("stocked omni turbine: %s %d rows %d total", tpl, n, total)
	}
	if r := mustOne(t, o, `select max(stack_size) m from items where inventory_id=951`); r["m"].(int64) != 100 {
		t.Fatalf("no stack may exceed 100: %v", r)
	}
	if tpl, n, total := stacks(t, o, 952); tpl != "WindTurbineLubricant1" || n != 5 || total != 499 {
		t.Fatalf("empty omni turbine gets Low-grade lubricant: %s %d rows %d total", tpl, n, total)
	}
	if _, n, _ := stacks(t, o, 953); n != 0 {
		t.Fatal("a placeholder turbine must stay empty")
	}
	if tpl, n, total := stacks(t, o, 954); tpl != "WindTurbineLubricant2" || n != 5 || total != 499 {
		t.Fatalf("a directional turbine gets Industrial-grade lubricant: %s %d rows %d total", tpl, n, total)
	}
	if again, _ := o.RefillGenerators(); again.(map[string]any)["filled"].(int) != 0 {
		t.Fatalf("a second run must find nothing: %v", again)
	}
	assertDBSound(t, o)
}

func TestRefillFillsWindtrapsWithTheBestFilterForTheMachine(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, turbineWorld)}
	if _, err := o.RefillGenerators(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		inv  int
		want string
	}{{961, "WindTrapFilter2"}, {962, "WindTrapFilter4"}, {963, "WindTrapFilter2"}, {964, "WindTrapFilter2"}} {
		if tpl, n, total := stacks(t, o, c.inv); tpl != c.want || n != 1 || total != 5 {
			t.Errorf("windtrap inventory %d: %s, %d rows, %d filters; want one row of 5 %s", c.inv, tpl, n, total, c.want)
		}
	}
	// the lower-grade filter was changed in place, not removed and re-added
	if r := mustOne(t, o, `select template_id t from items where id=973`); r["t"] != "WindTrapFilter2" {
		t.Fatalf("the Makeshift Filter row should now be a Standard Filter: %v", r)
	}
}

func TestRefillWaterSkipsPlaceholders(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, turbineWorld)}
	if _, err := o.RefillBaseWater(); err != nil {
		t.Fatal(err)
	}
	if water(t, o, -2001) != 0 {
		t.Fatal("a placeholder cistern must not be filled")
	}
}

// The summary and the pending-edit description name what was filled by its in-game name.
func TestRefillSaysWhatItFilledByInGameName(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, turbineWorld)}
	r, err := o.RefillGenerators()
	if err != nil {
		t.Fatal(err)
	}
	want := "2 Omnidirectional Wind Turbines, 2 Windtraps, 1 Directional Wind Turbine, 1 Large Windtrap"
	if got := r.(map[string]any)["summary"]; got != want {
		t.Fatalf("summary %q, want %q", got, want)
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "refill power: "+want {
		t.Fatalf("pending edit %v", p)
	}
	if got := countSummary(map[string]int{"Medium Water Cistern": 2, "Blood Purifier": 1}); got != "2 Medium Water Cisterns, 1 Blood Purifier" {
		t.Fatalf("water summary %q", got)
	}
	if powerName("generator_placeable") != "Fuel-Powered Generator" || waterName("bloodwaterextractionadvanced_placeable") != "Improved Blood Purifier" {
		t.Fatal("building names must be the in-game names")
	}
}
