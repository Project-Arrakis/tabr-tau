package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The save's factions as in a real one (Smuggler exists but is not assignable) and the character in Harkonnen.
const factionWorld = `
insert into factions(id,name) values (1,'Atreides'),(2,'Harkonnen'),(3,'None'),(4,'Smuggler');
insert into player_faction(actor_id,faction_id,utc_time_faction_change) values (1,2,1791483293175000);`

func TestFactionListsTheConsolesThreeChoices(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, factionWorld)}
	r, err := o.Faction()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if cur := m["current"].(map[string]any); cur["name"] != "Harkonnen" || cur["id"].(int64) != 2 {
		t.Fatalf("current: %v", cur)
	}
	opts := m["options"].([]map[string]any)
	if len(opts) != 3 || opts[0]["name"] != "Neutral" || opts[1]["name"] != "Atreides" || opts[2]["name"] != "Harkonnen" {
		t.Fatalf("options: %v", opts)
	}
}

func TestSetFactionWritesOneRowWithAMicrosecondStamp(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, factionWorld)}
	if _, err := o.SetFaction(Args{"faction_id": 1.0}); err != nil {
		t.Fatal(err)
	}
	r := mustOne(t, o, `select faction_id f, utc_time_faction_change t from player_faction where actor_id=1`)
	if r["f"].(int64) != 1 || r["t"].(int64) < 1_700_000_000_000_000 {
		t.Fatalf("row: %v", r)
	}
	if n := mustOne(t, o, `select count(*) c from player_faction`)["c"].(int64); n != 1 {
		t.Fatalf("exactly the character's row: %d", n)
	}
	if _, err := o.SetFaction(Args{"faction_id": 3.0}); err != nil {
		t.Fatal(err)
	}
	cur, _ := o.Faction()
	if cur.(map[string]any)["current"].(map[string]any)["name"] != "Neutral" {
		t.Fatalf("%v", cur)
	}
	assertDBSound(t, o)
}

func TestSetFactionCreatesTheRowWhenMissingAndRefusesOthers(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into factions(id,name) values (1,'Atreides'),(2,'Harkonnen'),(3,'None'),(4,'Smuggler');`)}
	if _, err := o.SetFaction(Args{"faction_id": 2.0}); err != nil {
		t.Fatal(err)
	}
	if mustOne(t, o, `select faction_id f from player_faction where actor_id=1`)["f"].(int64) != 2 {
		t.Fatal("row not created")
	}
	for _, id := range []float64{4, 0, 9, -1} {
		if _, err := o.SetFaction(Args{"faction_id": id}); err == nil {
			t.Errorf("faction %v must be refused", id)
		}
	}
	if _, err := o.SetFaction(Args{}); err == nil {
		t.Error("a missing faction must be refused")
	}
}

func TestRepairVehiclesBelowARepairThreshold(t *testing.T) {
	world := `
insert into actors(id,class,map) values (201,'c','HaggaBasin');
insert into vehicles(id) values (201);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'mine',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1);
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (1,201,'A_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (2,201,'B_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":200.0,"DecayedMaxDurability":250.0}]}')),
  (3,201,'C_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":250.0,"DecayedMaxDurability":250.0}]}'));`
	cur := func(o *Ops, id int) float64 {
		return mustOne(t, o, `select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') c from vehicle_modules where id=?`, id)["c"].(float64)
	}
	o := &Ops{S: testsave.PlayerWithSQL(t, world)}
	r, err := o.RepairVehiclesBelow(50) // 40 % is below, 80 % is not
	if err != nil || r.(map[string]any)["repaired"].(int64) != 1 {
		t.Fatalf("%v %v", r, err)
	}
	if cur(o, 1) != 250 || cur(o, 2) != 200 || cur(o, 3) != 250 {
		t.Fatalf("only the module below 50%% is raised: %v %v %v", cur(o, 1), cur(o, 2), cur(o, 3))
	}
	o2 := &Ops{S: testsave.PlayerWithSQL(t, world)}
	r, _ = o2.RepairVehiclesBelow(90)
	if r.(map[string]any)["repaired"].(int64) != 2 || cur(o2, 2) != 250 {
		t.Fatalf("90%% takes both: %v", r)
	}
	o3 := &Ops{S: testsave.PlayerWithSQL(t, world)}
	for _, bad := range []int64{0, 101, -5} {
		if _, err := o3.RepairVehiclesBelow(bad); err == nil {
			t.Errorf("threshold %d must be refused", bad)
		}
	}
	if o3.S.Dirty() {
		t.Fatal("a refused threshold must not change the save")
	}
}
