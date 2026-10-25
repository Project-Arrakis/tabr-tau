package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// autoFixture is the real-schema player save with something for every job to do: base water and power (baseFixture), two owned vehicles with worn
// modules, a worn helmet, a loadout sword, an empty Literjon in the backpack and a character that is 70 % full of water.
const autoFixture = baseFixture + `
insert into actors(id,class,map) values (211,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin'),(212,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin');
insert into vehicles(id) values (211),(212);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (211,'a',2,3,0),(212,'b',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (211,1,1),(212,1,1);
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (11,211,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (12,212,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (13,212,'Gen_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":240.0,"DecayedMaxDurability":250.0}]}'));
insert into fgl_entities(entity_id,components) values
  (-211, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":10.0}]}')),(-212, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":20.0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (211,-211,'Actor'),(212,-212,'Actor');
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0),(23,2,15,8,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (201,21,1,0,'Helmet',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.0,"MaxDurability":100.0}]}',0),
  (203,23,1,0,'Sword',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":20.0,"MaxDurability":100.0}]}',0),
  (220,1,1,3,'Literjon',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (222,1,1,4,'Bloodsack_01',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{}]}',0);
update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":"70.000000","CurrentValue":"70.000000"}}}') where id=2;`

func autoOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, autoFixture)}
}

func allJobs(AutoJob) bool { return true }

func onlyJobs(ids ...string) func(AutoJob) bool {
	return func(j AutoJob) bool {
		for _, id := range ids {
			if j.ID == id {
				return true
			}
		}
		return false
	}
}

// the state each job changes, as one string, so a test can compare before and after
func autoState(t *testing.T, o *Ops) string {
	t.Helper()
	var b strings.Builder
	for _, q := range []string{
		`select entity_id k, hex(components) v from fgl_entities order by 1`,
		`select id k, stack_size||'/'||template_id v from items order by 1`,
		`select id k, hex(stats) v from items order by 1`,
		`select id k, hex(stats) v from vehicle_modules order by 1`,
		`select id k, hex(gas_attributes) v from actors order by 1`,
	} {
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			fmt.Fprintf(&b, "%v=%v\n", r["k"], r["v"])
		}
	}
	return b.String()
}

func hasDone(p AutoPass, id string) bool {
	for _, d := range p.Done {
		if d.ID == id {
			return true
		}
	}
	return false
}

func moduleCur(t *testing.T, o *Ops, id int) float64 {
	t.Helper()
	return mustOne(t, o, `select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') c from vehicle_modules where id=?`, id)["c"].(float64)
}

// The pass runs every enabled job in ONE edit: one pending edit named "automatic pass", every job's change in it.
func TestAutoPassRunsEveryEnabledJobInOneEdit(t *testing.T) {
	o := autoOps(t)
	p, err := o.RunAutoPass(allJobs)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Changed {
		t.Fatal("the pass changed rows")
	}
	pending := o.S.Pending()
	if len(pending) != 1 || !strings.HasPrefix(pending[0], "automatic pass: ") {
		t.Fatalf("one edit for the whole pass, got %v", pending)
	}
	for _, id := range []string{"water", "power", "repair-vehicles", "repair-gear", "fill-containers", "fill-hydration", "fill-blood", "refuel"} {
		if !hasDone(p, id) {
			t.Errorf("job %s did nothing: %+v", id, p.Done)
		}
	}
	if f := mustOne(t, o, `select json_extract(components,'$.FVehicleComponent[1].CurrentFuel') f from fgl_entities where entity_id=-211`)["f"].(float64); f != 100 {
		t.Errorf("sandbike tank: %v", f)
	}
	if f := mustOne(t, o, `select json_extract(components,'$.FVehicleComponent[1].CurrentFuel') f from fgl_entities where entity_id=-212`)["f"].(float64); f != 240 {
		t.Errorf("buggy tank: %v", f)
	}
	if water(t, o, -1001) != 5000 || water(t, o, -1002) != 5000 {
		t.Errorf("base water: %d %d", water(t, o, -1001), water(t, o, -1002))
	}
	if mustOne(t, o, `select stack_size s from items where id=401`)["s"].(int64) != 499 {
		t.Error("the generator was not filled")
	}
	if moduleCur(t, o, 11) != 250 || moduleCur(t, o, 12) != 250 || moduleCur(t, o, 13) != 250 {
		t.Errorf("vehicle parts: %v %v %v", moduleCur(t, o, 11), moduleCur(t, o, 12), moduleCur(t, o, 13))
	}
	cur := func(id int) float64 {
		return mustOne(t, o, `select json_extract(stats,'$.FItemStackAndDurabilityStats[1].CurrentDurability') c from items where id=?`, id)["c"].(float64)
	}
	if cur(201) != 100 || cur(203) != 100 {
		t.Errorf("worn and loadout: %v %v", cur(201), cur(203))
	}
	if r := mustOne(t, o, `select json_extract(stats,'$.FFillableItemStats[1].CurrentAmount') c, json_extract(stats,'$.FFillableItemStats[1].FillableType') k from items where id=220`); r["c"].(float64) != 1000 || r["k"] != "Water" {
		t.Errorf("Literjon: %v", r)
	}
	if r := mustOne(t, o, `select json_extract(stats,'$.FFillableItemStats[1].CurrentAmount') c, json_extract(stats,'$.FFillableItemStats[1].FillableType') k from items where id=222`); r["c"].(float64) != 3000 || r["k"] != "Blood" {
		t.Errorf("blood sack: %v", r)
	}
	if h := mustOne(t, o, `select json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') h from actors where id=2`)["h"]; h != "100.000000" {
		t.Errorf("hydration: %v", h)
	}
	sum := p.Summary()
	for _, want := range []string{"Water Cistern", "Generator", "3 vehicle parts repaired", "2 worn or loadout items repaired", "1 water container filled", "your water filled", "1 blood sack filled", "2 vehicles refuelled"} {
		if !strings.Contains(sum, want) {
			t.Errorf("summary %q lacks %q", sum, want)
		}
	}
	assertDBSound(t, o)

	// the second pass finds nothing: no change, no second edit, and the sentence names what it looked at
	again, err := o.RunAutoPass(allJobs)
	if err != nil {
		t.Fatal(err)
	}
	if again.Changed || len(again.Done) != 0 || again.Summary() != "" || len(o.S.Pending()) != 1 {
		t.Fatalf("a second pass must find nothing and add no edit: %+v, pending %v", again, o.S.Pending())
	}
	if n := again.NothingNeeded(); !strings.Contains(n, "already full or in good repair") || !strings.Contains(n, "water storage") || !strings.Contains(n, "vehicles") {
		t.Fatalf("nothing-needed sentence: %q", n)
	}
}

// A job that is switched off is not run; the master switch off (no job asked for) runs nothing and touches nothing.
func TestAutoPassSkipsJobsThatAreSwitchedOff(t *testing.T) {
	o := autoOps(t)
	p, err := o.RunAutoPass(func(j AutoJob) bool { return j.ID != "repair-gear" && j.ID != "water" })
	if err != nil {
		t.Fatal(err)
	}
	if hasDone(p, "repair-gear") || hasDone(p, "water") || !hasDone(p, "repair-vehicles") || !hasDone(p, "power") {
		t.Fatalf("done: %+v", p.Done)
	}
	if water(t, o, -1001) != 0 {
		t.Error("base water was switched off but was filled")
	}
	if mustOne(t, o, `select json_extract(stats,'$.FItemStackAndDurabilityStats[1].CurrentDurability') c from items where id=201`)["c"].(float64) != 10 {
		t.Error("gear repair was switched off but ran")
	}

	o2 := autoOps(t)
	before := autoState(t, o2)
	p2, err := o2.RunAutoPass(func(AutoJob) bool { return false })
	if err != nil || p2.Changed || len(p2.Ran) != 0 || len(o2.S.Pending()) != 0 || o2.S.Dirty() || autoState(t, o2) != before {
		t.Fatalf("no job asked for must do nothing: %+v %v pending %v", p2, err, o2.S.Pending())
	}
}

// A job that fails rolls the whole pass back: the jobs before it leave nothing behind, and the error names the job.
func TestAutoPassFailureInTheLastJobRollsEverythingBack(t *testing.T) {
	o := autoOps(t)
	before := autoState(t, o)
	boom := errors.New("boom")
	var ran []string
	jobs := AutoJobs()
	for i := range jobs {
		if jobs[i].run != nil {
			id, run := jobs[i].ID, jobs[i].run
			jobs[i].run = func(m *save.Mut, c player) (string, string, error) { ran = append(ran, id); return run(m, c) }
		}
	}
	jobs = append(jobs, AutoJob{ID: "last", Label: "The last job", Ready: true, noChar: true, run: func(m *save.Mut, c player) (string, string, error) {
		return "", "", boom
	}})
	_, err := o.runAutoJobs(jobs, allJobs)
	var je *AutoJobError
	if !errors.As(err, &je) || je.ID != "last" || je.Label != "The last job" || !errors.Is(err, boom) {
		t.Fatalf("want a job error naming the last job, got %v", err)
	}
	if len(ran) != 8 {
		t.Fatalf("the eight ready jobs ran before the failure: %v", ran)
	}
	if autoState(t, o) != before {
		t.Fatal("a failing job left the others' changes behind")
	}
	if len(o.S.Pending()) != 0 || o.S.Dirty() {
		t.Fatalf("nothing may be queued: %v", o.S.Pending())
	}
	// and the editor is not stuck: the next pass works
	if p, err := o.RunAutoPass(allJobs); err != nil || !p.Changed {
		t.Fatalf("%+v %v", p, err)
	}
}

// With only water and power on, the pass leaves the save exactly as the two refill buttons do one after the other.
func TestAutoPassWithWaterAndPowerOnlyEqualsTheTwoRefills(t *testing.T) {
	a, b := autoOps(t), autoOps(t)
	if _, err := a.RunAutoPass(onlyJobs("water", "power")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RefillBaseWater(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RefillGenerators(); err != nil {
		t.Fatal(err)
	}
	if autoState(t, a) != autoState(t, b) {
		t.Fatal("the pass and the two refill buttons left different saves")
	}
	if len(a.S.Pending()) != 1 {
		t.Fatalf("the pass is one edit: %v", a.S.Pending())
	}
}

// A save with no character: the base jobs still run and the character's jobs are skipped with a note, not an error that would take the base refill down.
func TestAutoPassWithoutACharacterSkipsTheCharacterJobs(t *testing.T) {
	o := &Ops{S: testsave.WithSQL(t, baseFixture)}
	p, err := o.RunAutoPass(allJobs)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDone(p, "water") || !hasDone(p, "power") || hasDone(p, "repair-vehicles") || hasDone(p, "fill-hydration") {
		t.Fatalf("done: %+v", p.Done)
	}
	if len(p.Notes) != 6 || !strings.Contains(strings.Join(p.Notes, "|"), "skipped") {
		t.Fatalf("one note per skipped job: %v", p.Notes)
	}
}

// A character with no hydration record is not an error (the button refuses; the pass notes it and goes on).
func TestAutoPassNotesAMissingHydrationRecord(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, autoFixture+`update actors set gas_attributes=jsonb('{}') where id=2;`)}
	p, err := o.RunAutoPass(allJobs)
	if err != nil {
		t.Fatal(err)
	}
	if hasDone(p, "fill-hydration") || !hasDone(p, "water") || !strings.Contains(strings.Join(p.Notes, "|"), "hydration record") {
		t.Fatalf("%+v", p)
	}
}

// A character that is already full of water is not written again.
func TestAutoPassDoesNotRewriteFullHydration(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":"100.000000","CurrentValue":"100.000000"}}}') where id=2;`)}
	p, err := o.RunAutoPass(onlyJobs("fill-hydration"))
	if err != nil || p.Changed || len(o.S.Pending()) != 0 || o.S.Dirty() {
		t.Fatalf("%+v %v pending %v", p, err, o.S.Pending())
	}
}

// Only the character's own vehicles are repaired, never another owner's or a world vehicle (the rule of the Repair button).
func TestAutoPassRepairsOnlyTheCharactersOwnVehicles(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, autoFixture+`
insert into actors(id,class,map) values (230,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin'),(231,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin');
insert into vehicles(id) values (230),(231);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (230,'o',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (230,99,1);
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (31,230,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":10.0,"DecayedMaxDurability":250.0}]}')),
  (32,231,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":10.0,"DecayedMaxDurability":250.0}]}'));`)}
	if _, err := o.RunAutoPass(onlyJobs("repair-vehicles")); err != nil {
		t.Fatal(err)
	}
	if moduleCur(t, o, 11) != 250 || moduleCur(t, o, 31) != 10 || moduleCur(t, o, 32) != 10 {
		t.Fatalf("own vehicle repaired, another owner's and the world's left alone: %v %v %v", moduleCur(t, o, 11), moduleCur(t, o, 31), moduleCur(t, o, 32))
	}
}

// The backpack is not part of "worn equipment and loadout" (what RepairGear covers): an item there is left alone.
func TestAutoPassGearRepairLeavesTheBackpackAlone(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, autoFixture+`
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (221,1,1,4,'Tool',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":5.0,"MaxDurability":100.0}]}',0);`)}
	if _, err := o.RunAutoPass(onlyJobs("repair-gear")); err != nil {
		t.Fatal(err)
	}
	if c := mustOne(t, o, `select json_extract(stats,'$.FItemStackAndDurabilityStats[1].CurrentDurability') c from items where id=221`)["c"].(float64); c != 5 {
		t.Fatalf("a backpack item was repaired: %v", c)
	}
}

// The registry and the settings agree: same keys in the same order, unique ids, every job described, a job that is not ready says why and has no
// function, a ready one has a function; all default to on.
func TestAutoJobRegistryMatchesSettings(t *testing.T) {
	jobs := AutoJobs()
	if len(jobs) != len(settings.JobKeys) {
		t.Fatalf("%d jobs, %d settings keys", len(jobs), len(settings.JobKeys))
	}
	ids := map[string]bool{}
	var s settings.Settings
	for i, j := range jobs {
		if j.Key != settings.JobKeys[i] {
			t.Errorf("job %s has key %s, settings has %s", j.ID, j.Key, settings.JobKeys[i])
		}
		if !s.SetJob(j.Key, !j.Default) || s.JobOn(j.Key, j.Default) == j.Default {
			t.Errorf("job %s: key %s is not a settings key", j.ID, j.Key)
		}
		if ids[j.ID] || j.ID == "" || j.Label == "" || j.Desc == "" || j.noun == "" {
			t.Errorf("job %+v is incomplete or repeated", j)
		}
		ids[j.ID] = true
		if !j.Default {
			t.Errorf("job %s must default to on", j.ID)
		}
		if j.Ready != (j.run != nil) || (!j.Ready && j.Reason == "") || (j.Ready && j.Reason != "") {
			t.Errorf("job %s: ready=%v run=%v reason=%q", j.ID, j.Ready, j.run != nil, j.Reason)
		}
	}
}

// A real failure, not an injected one: the character's water is not a number, so the job that fills it fails after the jobs before it have
// written. Nothing is left behind, and the error names the job.
func TestAutoPassRollsBackWhenARealJobFails(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, autoFixture+`update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":"x","CurrentValue":"x"}}}') where id=2;`)}
	before := autoState(t, o)
	_, err := o.RunAutoPass(allJobs)
	var je *AutoJobError
	if !errors.As(err, &je) || je.ID != "fill-hydration" || !strings.Contains(err.Error(), "Fill your water") {
		t.Fatalf("%v", err)
	}
	if autoState(t, o) != before || len(o.S.Pending()) != 0 || o.S.Dirty() {
		t.Fatal("the jobs before the failing one left changes behind")
	}
}

// If a job says it did something but no row changed, the pass claims nothing: what is shown never goes beyond what was written.
func TestAutoPassClaimsNothingWhenNoRowChanged(t *testing.T) {
	o := autoOps(t)
	jobs := []AutoJob{{ID: "liar", Label: "Liar", Ready: true, noChar: true, noun: "x", run: func(m *save.Mut, c player) (string, string, error) {
		return "10 things done", "", nil
	}}}
	p, err := o.runAutoJobs(jobs, allJobs)
	if err != nil || p.Changed || len(p.Done) != 0 || p.Summary() != "" || len(o.S.Pending()) != 0 {
		t.Fatalf("%+v %v", p, err)
	}
}

// The automatic pass keeps the type of the stored water like the button does: a number stays a number (the game's own shape in current
// saves), text stays text, and the summary (and so the write log) says which.
func TestAutoPassKeepsTheTypeOfTheHydration(t *testing.T) {
	for _, tc := range []struct {
		name, rec, jsonType string
		want                any
	}{
		{"number", `{"BaseValue":59.289321,"CurrentValue":59.289321}`, "real", 100.0},
		{"text", `{"BaseValue":"59.289321","CurrentValue":"59.289321"}`, "text", "100.000000"},
	} {
		o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":`+tc.rec+`,"ClothingCapturedWater":{"BaseValue":0.0,"CurrentValue":0.0}}}') where id=2;`)}
		p, err := o.RunAutoPass(onlyJobs("fill-hydration"))
		if err != nil || !hasDone(p, "fill-hydration") {
			t.Fatalf("%s: %+v %v", tc.name, p, err)
		}
		row := mustOne(t, o, `select json_type(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.BaseValue') bt,
			json_type(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') ct,
			json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.BaseValue') b,
			json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') c,
			json_type(gas_attributes,'$.DuneHydrationAttributeSet.ClothingCapturedWater.CurrentValue') wt from actors where id=2`)
		if row["bt"] != tc.jsonType || row["ct"] != tc.jsonType || row["b"] != tc.want || row["c"] != tc.want || row["wt"] != "real" {
			t.Errorf("%s: %v", tc.name, row)
		}
		if !strings.Contains(p.Summary(), "your water filled (kept as "+tc.name+")") {
			t.Errorf("%s: the summary does not say the type: %q", tc.name, p.Summary())
		}
	}
}
