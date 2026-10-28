package ops

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Create a vehicle from a template (#401). The fixture is a character (pawn at 100, 200, 300 on HaggaBasin, facing 90 degrees) who owns one medium ornithopter written as the
// game wrote vehicle 578 of a real save (the same properties, entity components, 14 modules, a cargo hold of 20 slots and 2000 volume, one component hold), and a world vehicle
// that is nobody's.

func entID(prefix uint32, low uint32) int64 { return int64(uint64(prefix)<<32 | uint64(low)) }

const realMediumProps = `{"BP_MediumOrnithopter_CHOAM_C":{"m_HarnessedVehicleId":"!!act#0","m_InitialFuel":0.0,"m_InitialHealth":0.0,"m_SpawnerName":"","m_CustomizationId":"None","m_AbandonedTimestamp":"2026.10.19-03.56.03","m_bIsInSafezone":true},"DamageableActorComponent":{"m_CurrentMaxHealth":750.0,"m_TotalMaxHealth":750.0},"EncounterActorComponent":{"m_OwningEncounterUid":{"EncounterUid":"None"}}}`

const realMediumEntity = `{"FAggroControllerComponent":[0,{"m_TotalDamageDone":0.0}],"FBiomeWeatherModifierComponent":[0,{"CurrentTemperatureModifier":1.0,"CurrentSandBuildupModifier":0.0,"m_CurrentSandColor":{"r":0.428689,"G":0.145263,"B":0.06859,"A":1.0}}],"FHealthComponent":[0,{"m_CurrentHealth":750.0,"m_MaxDownButNotOutStateHealth":150.0,"m_CurrentDownButNotOutStateHealth":150.0}],"FShelterComponent":[0,{"m_IsShelteredTraceResults":[true,true,true,true,true,true,true,true,true],"m_ShelteredPercentage":1.0}],"FVehicleComponent":[0,{"CurrentFuel":750.0,"RemainingTimeUntilDespawn":0.0}]}`

var realMediumModules = strings.Fields(`OrnithopterMediumChassis_5 OrnithopterMediumHullFront_5 OrnithopterMediumHullBack_5 OrnithopterMediumGenerator_5 OrnithopterMediumHull_5
	OrnithopterMediumLocomotionBackLeft_5 OrnithopterMediumLocomotionBackRight_5 OrnithopterMediumLocomotionCenterLeft_5 OrnithopterMediumLocomotionCenterRight_5
	OrnithopterMediumLocomotionFrontLeft_5 OrnithopterMediumLocomotionFrontRight_5 OrnithopterMediumInventory_5 OrnithopterMediumEngine_5 OrnithopterMediumBoost_5`)

func spawnWorldSQL() string {
	var b strings.Builder
	prefix := uint32(0xb1270da9)
	fmt.Fprintf(&b, `
update actors set rotation_x=0.0, rotation_y=0.0, rotation_z=0.7071067811865476, rotation_w=0.7071067811865476 where id=2;
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,gas_attributes,properties,serial,state) values
 (578,'/Game/Dune/Systems/Vehicles/Blueprints/FlyingVehicles/BP_MediumOrnithopter_CHOAM.BP_MediumOrnithopter_CHOAM_C','DeepDesert',-311808.95,-587767.15,3528.84,-0.0,0.0,-1.0,0.06,0,jsonb('{}'),jsonb('%s'),679,0),
 (579,'/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',5000,6000,7000,0,0,0,1,0,jsonb('{}'),jsonb('{}'),5,0);
insert into vehicles(id) values (578),(579);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (578,'##MediumOrnithopter',2,3,0,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (578,1,1);
insert into fgl_entities(entity_id,components) values (%d, jsonb('%s')), (%d, jsonb('{}')), (%d, jsonb('{"Other":1}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (578,%d,'Actor'),(578,%d,'VehicleInventory');
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (506,578,0,20,2000.0),(507,578,NULL,NULL,NULL);
insert into actor_inventories(inventory_id,component_name_hash) values (506,-72424899),(507,462019337);`,
		realMediumProps, entID(prefix, 664255), realMediumEntity, entID(prefix, 664253), entID(0x11111111, 900000), entID(prefix, 664255), entID(prefix, 664253))
	for i, id := range realMediumModules {
		fmt.Fprintf(&b, "\ninsert into vehicle_modules(id,vehicle_id,template_id,stats) values (%d,578,'%s',jsonb('{\"FVehicleModuleDurabilityStats\":[[],{\"CurrentDurability\":1.0}],\"FCustomizationStats\":[[],{\"SwatchId\":\"Sardaukar Global\"}]}'));", 204+i, id)
	}
	return b.String()
}

func spawnOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, spawnWorldSQL())} }

func spawnID(t *testing.T, r any) int64 { t.Helper(); return newVehicleID(t, r) }

// rowsOf renders every vehicle-related row of the save, so a test can show that nothing but the new vehicle changed.
func vehicleRowsDump(t *testing.T, o *Ops, ids ...int64) string {
	t.Helper()
	var out []string
	for _, id := range ids {
		for _, q := range []string{
			`select id,class,map,location_x,location_y,location_z,json(properties),serial,state from actors where id=?`, `select * from vehicles where id=?`, `select * from permission_actor where actor_id=?`,
			`select * from permission_actor_rank where permission_actor_id=?`, `select actor_id,entity_id,slot_name from actor_fgl_entities where actor_id=?`,
			`select * from inventories where actor_id=?`, `select * from vehicle_modules where vehicle_id=? order by id`,
			`select e.entity_id, json(e.components) from fgl_entities e join actor_fgl_entities l on l.entity_id=e.entity_id where l.actor_id=?`,
			`select i.id, i.template_id, i.stack_size from items i join inventories v on v.id=i.inventory_id where v.actor_id=?`,
		} {
			rows, err := o.S.Query(q, id)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprint(rows))
		}
	}
	return strings.Join(out, "\n")
}

type vehicleView struct {
	actor    map[string]any
	modules  []string
	cargo    map[string]any
	holds    []int64
	entities map[string]map[string]any
	props    string
}

func viewOf(t *testing.T, o *Ops, id int64) vehicleView {
	t.Helper()
	v := vehicleView{entities: map[string]map[string]any{}}
	v.actor = mustOne(t, o, `select class, map, location_x x, location_y y, location_z z, rotation_x rx, rotation_y ry, rotation_z rz, rotation_w rw, dimension_index dim, serial, state, owner_account_id owner,
		json(gas_attributes) gas from actors where id=?`, id)
	v.props = mustOne(t, o, `select json(properties) p from actors where id=?`, id)["p"].(string)
	rows, _ := o.S.Query(`select template_id t from vehicle_modules where vehicle_id=? order by template_id`, id)
	for _, r := range rows {
		v.modules = append(v.modules, r["t"].(string))
	}
	v.cargo = mustOne(t, o, `select id, max_item_count n, max_item_volume vol from inventories where actor_id=? and inventory_type=0`, id)
	hs, _ := o.S.Query(`select h.component_name_hash h from inventories v join actor_inventories h on h.inventory_id=v.id where v.actor_id=? and v.inventory_type is null order by v.id`, id)
	for _, h := range hs {
		v.holds = append(v.holds, h["h"].(int64))
	}
	ents, _ := o.S.Query(`select l.slot_name s, l.entity_id e, json(f.components) c from actor_fgl_entities l join fgl_entities f on f.entity_id=l.entity_id where l.actor_id=?`, id)
	for _, e := range ents {
		v.entities[e["s"].(string)] = e
	}
	return v
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func sameSet(a, b []string) bool {
	return strings.Join(sortedCopy(a), " ") == strings.Join(sortedCopy(b), " ")
}

func spawn(t *testing.T, o *Ops, a Args) (int64, map[string]any) {
	t.Helper()
	r, err := o.SpawnVehicle(a)
	if err != nil {
		t.Fatal(err)
	}
	return spawnID(t, r), r.(map[string]any)
}

// Everything a vehicle is: the rows of a medium ornithopter T5_Inventory made from the template are those of the real vehicle 578 (the module set, the properties, the entity
// components, the cargo hold and the component hold, the permission record and its rank), apart from the place, the time and the ids.
func TestSpawnedMediumT5ReproducesTheRealVehicle578(t *testing.T) {
	o := spawnOps(t)
	id, res := spawn(t, o, Args{"class": "OrnithopterMedium", "template": "T5_Inventory"})
	real, got := viewOf(t, o, 578), viewOf(t, o, id)
	if !sameSet(real.modules, got.modules) || len(got.modules) != 14 {
		t.Errorf("modules:\n real %v\n got  %v", sortedCopy(real.modules), sortedCopy(got.modules))
	}
	if real.actor["class"] != got.actor["class"] || got.actor["serial"] != int64(0) || got.actor["state"] != int64(0) || got.actor["owner"] != nil || got.actor["gas"] != "{}" || got.actor["dim"] != int64(0) {
		t.Errorf("actor: %v vs %v", got.actor, real.actor)
	}
	// the properties are 578's but for the abandoned time
	norm := func(s string) string {
		i := strings.Index(s, `"m_AbandonedTimestamp":"`)
		return s[:i] + s[i+len(`"m_AbandonedTimestamp":"`)+19:]
	}
	if norm(real.props) != norm(got.props) {
		t.Errorf("properties:\n real %s\n got  %s", real.props, got.props)
	}
	if got.entities["Actor"]["c"] != real.entities["Actor"]["c"] || got.entities["VehicleInventory"]["c"] != real.entities["VehicleInventory"]["c"] {
		t.Errorf("entity components:\n real %v\n got  %v", real.entities, got.entities)
	}
	if got.cargo["n"] != int64(20) || got.cargo["vol"] != 2000.0 || fmt.Sprint(got.holds) != fmt.Sprint(real.holds) {
		t.Errorf("inventories: %v %v vs %v", got.cargo, got.holds, real.holds)
	}
	if h := mustOne(t, o, `select component_name_hash h from actor_inventories where inventory_id=?`, got.cargo["id"])["h"]; h != int64(-72424899) {
		t.Errorf("cargo hash %v", h)
	}
	pa := mustOne(t, o, `select actor_name n, actor_type t, access_level l, is_child c, edited_by_player_id e from permission_actor where actor_id=?`, id)
	if pa["n"] != "##MediumOrnithopter" || pa["t"] != int64(2) || pa["l"] != int64(3) || pa["c"] != int64(0) || pa["e"] != int64(0) {
		t.Errorf("permission: %v", pa)
	}
	if rk, _ := o.S.Query(`select player_id p, rank r from permission_actor_rank where permission_actor_id=?`, id); len(rk) != 1 || rk[0]["p"] != int64(1) || rk[0]["r"] != int64(1) {
		t.Errorf("rank: %v", rk)
	}
	// the answer says what it wrote and where
	if res["tier"] != 5 || res["template"] != "T5_Inventory" || res["complete"] != true {
		t.Errorf("answer: %v", res)
	}
	assertDBSound(t, o)
}

func moduleStatsOf(t *testing.T, o *Ops, id int64, tmpl string) string {
	return mustOne(t, o, `select json(stats) s from vehicle_modules where vehicle_id=? and template_id=?`, id, tmpl)["s"].(string)
}

// The light ornithopter T4_Inventory of the research: 11 modules, health 500, critical 100, fuel 300, a cargo hold of 10 slots and 500 volume; the scanner has weapon stats.
func TestSpawnLightOrnithopterT4Inventory(t *testing.T) {
	o := spawnOps(t)
	before := vehicleRowsDump(t, o, 578, 579)
	id, _ := spawn(t, o, Args{"class": "OrnithopterLight", "template": "T4_Inventory"})
	v := viewOf(t, o, id)
	if len(v.modules) != 11 || v.cargo["n"] != int64(10) || v.cargo["vol"] != 500.0 {
		t.Fatalf("%d modules, cargo %v", len(v.modules), v.cargo)
	}
	for _, want := range []string{`"m_TotalMaxHealth":500.0`, `"m_CurrentMaxHealth":500.0`, `"WeaponActorComponent":{"m_FavoriteWeaponItemDatabaseId":"!!itm#0"}`, `"BP_LightOrnithopter_Choam_C"`} {
		if !strings.Contains(v.props, want) {
			t.Errorf("properties lack %s: %s", want, v.props)
		}
	}
	c := v.entities["Actor"]["c"].(string)
	for _, want := range []string{`"m_CurrentHealth":500.0`, `"m_MaxDownButNotOutStateHealth":100.0`, `"m_CurrentDownButNotOutStateHealth":100.0`, `"CurrentFuel":300.0`} {
		if !strings.Contains(c, want) {
			t.Errorf("entity lacks %s: %s", want, c)
		}
	}
	if s := moduleStatsOf(t, o, id, "OrnithopterLightChassis_4"); s != `{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":3500.0}],"FCustomizationStats":[[],{"SwatchId":"Sardaukar Global"}]}` {
		t.Errorf("a new chassis: %s", s)
	}
	if s := moduleStatsOf(t, o, id, "OrnithopterLightScanner_4"); !strings.Contains(s, `"FVehicleWeaponModuleStats":[[],{"CharacterStateTag":{}}]`) || !strings.Contains(s, `"CurrentDurability":1750.0`) {
		t.Errorf("a new scanner: %s", s)
	}
	if after := vehicleRowsDump(t, o, 578, 579); after != before {
		t.Error("the existing vehicles are as they were")
	}
	assertDBSound(t, o)
}

func TestSpawnBuggyT5MiningAndT6Combat(t *testing.T) {
	o := spawnOps(t)
	id5, _ := spawn(t, o, Args{"class": "Buggy", "template": "T5_Mining"})
	v := viewOf(t, o, id5)
	if len(v.modules) != 11 || v.cargo["n"] != int64(20) || v.cargo["vol"] != 2500.0 || !strings.Contains(v.props, `"m_TotalMaxHealth":650.0`) || !strings.Contains(v.entities["Actor"]["c"].(string), `"CurrentFuel":280.0`) {
		t.Errorf("T5_Mining: %d modules, cargo %v, %s", len(v.modules), v.cargo, v.entities["Actor"]["c"])
	}
	if mustOne(t, o, `select actor_name n from permission_actor where actor_id=?`, id5)["n"] != "##BuggyChoam" {
		t.Error("permission name of a buggy")
	}
	id6, _ := spawn(t, o, Args{"class": "Buggy", "template": "T6_Combat"})
	v = viewOf(t, o, id6)
	if len(v.modules) != 11 || v.cargo["n"] != int64(0) || v.cargo["vol"] != 0.0 || !strings.Contains(v.entities["Actor"]["c"].(string), `"CurrentFuel":300.0`) {
		t.Errorf("T6_Combat: %d modules, cargo %v", len(v.modules), v.cargo)
	}
	if v.entities["Actor"]["e"].(int64) == viewOf(t, o, id5).entities["Actor"]["e"].(int64) {
		t.Error("distinct entities")
	}
	assertDBSound(t, o)
}

// The operator's buggy: Mk6 (tier 6, the T6_Combat template) with three uniques in place of stock parts and a cargo of items. The item ids the game's catalog uses for two of them
// spell the grade with a zero; the rows are told apart in the answer.
func TestSpawnTheOperatorsMkSixBuggyWithUniquesAndCargo(t *testing.T) {
	o := spawnOps(t)
	a := Args{
		"class": "Buggy", "template": "T6_Combat",
		"uniques": map[string]any{
			"Engine":       "BuggyEngine_Unique_Accelerate_06",
			"BodyHull":     "BuggyHullBackExtra_6",
			"UtilitySlot1": "BuggyInventory_Unique_Capacity_06",
			"UtilitySlot2": "BuggyMining_Unique_YieldIncrease_06",
		},
		"items": []any{map[string]any{"template_id": "MelangeSpice", "quantity": 40.0}, map[string]any{"template_id": "Literjon", "quantity": 2.0}},
	}
	id, res := spawn(t, o, a)
	v := viewOf(t, o, id)
	want := []string{"BuggyChassis_6", "BuggyGenerator_6", "BuggyEngine_Unique_Accelerate_06", "BuggyHullFront_6", "BuggyHullBackExtra_6", "BuggyLocomotionFrontLeft_6",
		"BuggyLocomotionFrontRight_6", "BuggyLocomotionBackLeft_6", "BuggyLocomotionBackRight_6", "BuggyInventory_Unique_Capacity_6", "BuggyMining_Unique_YieldIncrease_06"}
	if !sameSet(v.modules, want) {
		t.Fatalf("modules:\n got  %v\n want %v", sortedCopy(v.modules), sortedCopy(want))
	}
	if v.cargo["n"] != int64(35) || v.cargo["vol"] != 3500.0 || !strings.Contains(v.props, `"m_TotalMaxHealth":650.0`) || !strings.Contains(v.entities["Actor"]["c"].(string), `"CurrentFuel":300.0`) {
		t.Errorf("derived: cargo %v, %s, %s", v.cargo, v.props, v.entities["Actor"]["c"])
	}
	if s := moduleStatsOf(t, o, id, "BuggyMining_Unique_YieldIncrease_06"); !strings.Contains(s, "FVehicleWeaponModuleStats") {
		t.Errorf("the mining module is a weapon-type module: %s", s)
	}
	// the items are in the cargo hold, by the Give Items path
	items, _ := o.S.Query(`select template_id t, stack_size n, position_index p from items where inventory_id=? order by position_index`, v.cargo["id"])
	if len(items) < 2 {
		t.Fatalf("items: %v", items)
	}
	total := map[string]int64{}
	for _, it := range items {
		total[it["t"].(string)] += it["n"].(int64)
	}
	if total["MelangeSpice"] != 40 || total["Literjon"] != 2 || res["complete"] != true || res["given"] != int64(42) {
		t.Errorf("items: %v %v", total, res)
	}
	// the answer says which row the item ids became, and has no hull warning (the game's T5 and T6 buggies fill both utility slots on this hull)
	notes := fmt.Sprint(res["notes"])
	if !strings.Contains(notes, "BuggyInventory_Unique_Capacity_06 is the module row BuggyInventory_Unique_Capacity_6") || strings.Contains(notes, "has not been seen built") {
		t.Errorf("notes: %v", res["notes"])
	}
	changed := fmt.Sprint(res["changed"])
	for _, w := range []string{"BuggyBoost_6", "BuggyLauncher_6", "BuggyEngine_6"} {
		if !strings.Contains(changed, w) {
			t.Errorf("the answer says what was changed in the template (%s): %s", w, changed)
		}
	}
	assertDBSound(t, o)
}

// A mining laser on the plain rear hull is something none of the game's templates builds: it is allowed (the game's slot table does not forbid it) and said.
func TestSpawnWarnsOfAHullCombinationNoTemplateHas(t *testing.T) {
	o := spawnOps(t)
	_, res := spawn(t, o, Args{"class": "Buggy", "template": "T4_Boost", "uniques": map[string]any{"UtilitySlot2": "BuggyMining_4"}})
	if !strings.Contains(fmt.Sprint(res["notes"]), "has not been seen built") {
		t.Errorf("notes: %v", res["notes"])
	}
}

// Dry run: the answer holds the rows and the id the vehicle would get, nothing is kept, and the real run then writes the very same rows.
func TestPreviewWritesNothingAndTheApplyWritesTheSameRows(t *testing.T) {
	o := spawnOps(t)
	a := Args{"class": "OrnithopterLight", "template": "T4_Inventory", "fuel": 0.5, "items": []any{map[string]any{"template_id": "MelangeSpice", "quantity": 3.0}}}
	before := vehicleRowsDump(t, o, 578, 579)
	pr, err := o.SpawnVehiclePreview(a)
	if err != nil {
		t.Fatal(err)
	}
	if o.S.Dirty() {
		t.Fatal("a preview changes nothing")
	}
	if vehicleRowsDump(t, o, 578, 579) != before {
		t.Fatal("rows changed")
	}
	p := pr.(map[string]any)
	if p["dryRun"] != true || p["id"] == nil {
		t.Fatalf("preview answer: %v", p)
	}
	rows := p["rows"].(map[string]any)
	for _, k := range []string{"actors", "permission_actor", "permission_actor_rank", "entities", "inventories", "items", "vehicle_modules"} {
		if rows[k] == nil {
			t.Errorf("the preview lists the %s rows", k)
		}
	}
	if n := mustOne(t, o, `select count(*) c from actors where id=?`, p["id"])["c"].(int64); n != 0 {
		t.Fatal("the previewed vehicle must not exist")
	}
	r, err := o.SpawnVehicle(a)
	if err != nil {
		t.Fatal(err)
	}
	ap := r.(map[string]any)
	if ap["id"] != p["id"] || ap["dryRun"] != false {
		t.Errorf("the apply takes the id the preview named: %v vs %v", ap["id"], p["id"])
	}
	// the rows are the same, apart from the time in the properties
	strip := func(m any) string {
		s := fmt.Sprint(m)
		for {
			i := strings.Index(s, "m_AbandonedTimestamp:")
			if i < 0 {
				return s
			}
			s = s[:i] + s[i+len("m_AbandonedTimestamp:")+19:]
		}
	}
	if strip(rows) == "" || strip(rows["entities"]) != strip(ap["rows"].(map[string]any)["entities"]) || strip(rows["vehicle_modules"]) != strip(ap["rows"].(map[string]any)["vehicle_modules"]) ||
		strip(rows["items"]) != strip(ap["rows"].(map[string]any)["items"]) || strip(rows["inventories"]) != strip(ap["rows"].(map[string]any)["inventories"]) {
		t.Errorf("preview and apply differ:\n%v\n%v", rows, ap["rows"])
	}
	if c := viewOf(t, o, ap["id"].(int64)).entities["Actor"]["c"].(string); !strings.Contains(c, `"CurrentFuel":150.0`) {
		t.Errorf("half a tank of 300: %s", c)
	}
}

// Place: next to the character by default (the character's map, +800 along x, same y, +150 up, facing as the character does), or where the request says.
func TestSpawnPlacesTheVehicleNextToTheCharacterOrWhereAsked(t *testing.T) {
	o := spawnOps(t)
	id, _ := spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
	a := viewOf(t, o, id).actor
	if a["map"] != "HaggaBasin" || a["x"] != 900.0 || a["y"] != 200.0 || a["z"] != 450.0 || a["rx"] != 0.0 || a["ry"] != 0.0 {
		t.Errorf("place: %v", a)
	}
	// the character's 90 degrees about the vertical axis
	if rz, rw := a["rz"].(float64), a["rw"].(float64); rz < 0.70710 || rz > 0.70711 || rw < 0.70710 || rw > 0.70711 {
		t.Errorf("turn: %v %v", rz, rw)
	}
	id2, _ := spawn(t, o, Args{"class": "Sandbike", "template": "T0", "place": map[string]any{"map": "DeepDesert", "x": -1000.5, "y": 2000.0, "z": 3000.0}})
	a = viewOf(t, o, id2).actor
	if a["map"] != "DeepDesert" || a["x"] != -1000.5 || a["y"] != 2000.0 || a["z"] != 3000.0 {
		t.Errorf("chosen place: %v", a)
	}
	// a character with a lean (a quaternion with pitch) gets a vehicle that only turns
	testsave.Exec(t, o.S, `update actors set rotation_x=0.2, rotation_y=0.1, rotation_z=0.6, rotation_w=0.7615773105863909 where id=2`)
	id3, _ := spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
	a = viewOf(t, o, id3).actor
	if a["rx"] != 0.0 || a["ry"] != 0.0 {
		t.Errorf("only a turn: %v", a)
	}
}

// The Sandbike has two component holds, the container vehicle one module and a big cargo hold; the carrier has seventeen modules.
func TestSpawnEveryClassWritesAVehicleThatIsSound(t *testing.T) {
	o := spawnOps(t)
	for _, key := range catalog.VehicleClassKeys() {
		c, _ := catalog.VehicleClassFor(key)
		for _, tp := range c.Templates {
			id, _ := spawn(t, o, Args{"class": key, "template": tp.Name})
			v := viewOf(t, o, id)
			if len(v.modules) != len(tp.Modules) || len(v.holds) != len(c.Holds) || v.entities["Actor"] == nil || v.entities["VehicleInventory"] == nil {
				t.Errorf("%s/%s: %d modules, %v holds", key, tp.Name, len(v.modules), v.holds)
			}
		}
	}
	assertDBSound(t, o)
	if v, _ := o.Vehicles(); len(v.(map[string]any)["vehicles"].([]map[string]any)) < 20 {
		t.Errorf("the new vehicles belong to the character")
	}
	cid, _ := spawn(t, o, Args{"class": "ContainerVehicle", "template": "Container"})
	if c := viewOf(t, o, cid).cargo; c["n"] != int64(150) || c["vol"] != 15000.0 {
		t.Errorf("container: %v", c)
	}
}

// Entity ids: they take the prefix of the newest vehicle, count from above everything the prefix already has, and are distinct for every vehicle made.
func TestSpawnEntityIDsShareThePrefixAndAreUnique(t *testing.T) {
	o := spawnOps(t)
	ref := entID(0xb1270da9, 664255)
	var seen = map[int64]bool{}
	for i := 0; i < 3; i++ {
		id, _ := spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
		for slot, e := range viewOf(t, o, id).entities {
			eid := e["e"].(int64)
			if eid>>32 != ref>>32 || eid&0xFFFFFFFF <= 664255 || seen[eid] {
				t.Errorf("%s entity %x", slot, eid)
			}
			seen[eid] = true
		}
	}
	// without any vehicle, the character's own entities give the prefix; without those, the edit is refused
	o2 := &Ops{S: testsave.PlayerWithSQL(t, `insert into fgl_entities(entity_id,components) values (8300000000000001, jsonb('{}')); insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,8300000000000001,'DuneCharacter');`)}
	id, _ := spawn(t, o2, Args{"class": "Sandbike", "template": "T0"})
	for _, e := range viewOf(t, o2, id).entities {
		if e["e"].(int64)>>32 != 8300000000000001>>32 {
			t.Errorf("the character's prefix: %x", e["e"])
		}
	}
	o3 := &Ops{S: testsave.Player(t)}
	if _, err := o3.SpawnVehicle(Args{"class": "Sandbike", "template": "T0"}); err == nil || !strings.Contains(err.Error(), "entity") {
		t.Errorf("no entity to number from: %v", err)
	}
	if o3.S.Dirty() {
		t.Error("nothing may change")
	}
}

func TestSpawnRefusesBadRequestsAndChangesNothing(t *testing.T) {
	o := spawnOps(t)
	cases := map[string]Args{
		"no class":                               {"template": "T0"},
		"unknown class":                          {"class": "Tank", "template": "T0"},
		"treadwheel stays off":                   {"class": "TreadWheel", "template": "T5_Inventory"},
		"no template":                            {"class": "Buggy"},
		"a template of another class":            {"class": "Buggy", "template": "T4_Inventory"},
		"unknown slot":                           {"class": "Buggy", "template": "T0", "uniques": map[string]any{"Wing": "BuggyBoost_4"}},
		"a slot of another class":                {"class": "Sandbike", "template": "T0", "uniques": map[string]any{"UtilitySlot3": "x"}},
		"a module in the wrong slot":             {"class": "Buggy", "template": "T0", "uniques": map[string]any{"Engine": "BuggyBoost_4"}},
		"a module of another class":              {"class": "Buggy", "template": "T0", "uniques": map[string]any{"Engine": "SandbikeEngine_1"}},
		"an unknown module":                      {"class": "Buggy", "template": "T0", "uniques": map[string]any{"Engine": "BuggyEngine_77"}},
		"a placeholder module":                   {"class": "Buggy", "template": "T0", "uniques": map[string]any{"UtilitySlot2": "BuggyMining_Polarcap_6"}},
		"a mandatory slot left empty":            {"class": "Buggy", "template": "T0", "uniques": map[string]any{"Engine": ""}},
		"uniques that is not an object":          {"class": "Buggy", "template": "T0", "uniques": "BuggyEngine_4"},
		"fuel above 1":                           {"class": "Buggy", "template": "T0", "fuel": 1.5},
		"negative fuel":                          {"class": "Buggy", "template": "T0", "fuel": -0.1},
		"fuel not a number":                      {"class": "Buggy", "template": "T0", "fuel": "lots"},
		"place without a map":                    {"class": "Buggy", "template": "T0", "place": map[string]any{"x": 1.0, "y": 2.0, "z": 3.0}},
		"place with a bad map name":              {"class": "Buggy", "template": "T0", "place": map[string]any{"map": "../x", "x": 1.0, "y": 2.0, "z": 3.0}},
		"place without z":                        {"class": "Buggy", "template": "T0", "place": map[string]any{"map": "HaggaBasin", "x": 1.0, "y": 2.0}},
		"place far out":                          {"class": "Buggy", "template": "T0", "place": map[string]any{"map": "HaggaBasin", "x": 1e12, "y": 2.0, "z": 3.0}},
		"place with NaN":                         {"class": "Buggy", "template": "T0", "place": map[string]any{"map": "HaggaBasin", "x": "NaN", "y": 2.0, "z": 3.0}},
		"items for a vehicle with no cargo hold": {"class": "Buggy", "template": "T6_Combat", "items": []any{map[string]any{"template_id": "MelangeSpice", "quantity": 1.0}}},
		"an unknown item":                        {"class": "Buggy", "template": "T5_Mining", "items": []any{map[string]any{"template_id": "NotAThing", "quantity": 1.0}}},
		"an item that is not an object":          {"class": "Buggy", "template": "T5_Mining", "items": []any{"MelangeSpice"}},
		"a zero quantity":                        {"class": "Buggy", "template": "T5_Mining", "items": []any{map[string]any{"template_id": "MelangeSpice", "quantity": 0.0}}},
	}
	for name, a := range cases {
		if _, err := o.SpawnVehicle(a); err == nil {
			t.Errorf("%s must be refused", name)
		} else if _, perr := o.SpawnVehiclePreview(a); perr == nil {
			t.Errorf("%s must be refused by the preview too", name)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused request must not change the save")
	}
	_, err := o.SpawnVehicle(Args{"class": "Buggy", "template": "T0", "uniques": map[string]any{"Engine": "BuggyBoost_4"}})
	wantRefusal(t, err, apierr.ErrBadArgument, "a module in the wrong slot")
	// too many items
	var many []any
	for i := 0; i < maxSpawnItems+1; i++ {
		many = append(many, map[string]any{"template_id": "MelangeSpice", "quantity": 1.0})
	}
	_, err = o.SpawnVehicle(Args{"class": "Buggy", "template": "T5_Mining", "items": many})
	wantRefusal(t, err, apierr.ErrBadArgument, "too many items")
	// a save with no character body
	o2 := &Ops{S: testsave.PlayerWithSQL(t, `update player_state set player_pawn_id=NULL; delete from actors where id=2;`)}
	if _, err := o2.SpawnVehicle(Args{"class": "Buggy", "template": "T0"}); err == nil {
		t.Error("no character body to put it next to")
	}
	if o2.S.Dirty() {
		t.Error("nothing may change")
	}
}

// Items that do not all fit are given as far as they fit and the answer says so, as Give Items and the care package do.
func TestSpawnPutsInAsMuchCargoAsFitsAndSaysWhatDidNot(t *testing.T) {
	o := spawnOps(t)
	// a sandbike T2_Inventory has 15 slots and 250 volume; Literjon does not stack
	var items []any
	items = append(items, map[string]any{"template_id": "Literjon", "quantity": 30.0})
	id, res := spawn(t, o, Args{"class": "Sandbike", "template": "T2_Inventory", "items": items})
	if res["complete"] != false || res["given"].(int64) >= res["requested"].(int64) || len(res["notGiven"].([]map[string]any)) != 1 {
		t.Errorf("answer: %v", res)
	}
	n := mustOne(t, o, `select count(*) c from items i join inventories v on v.id=i.inventory_id where v.actor_id=?`, id)["c"].(int64)
	if n == 0 || n > 15 {
		t.Errorf("%d items", n)
	}
	assertDBSound(t, o)
}

// The checks after the write: a row that does not follow from the modules, or a foreign key broken by the rows, undoes the whole edit.
func TestSpawnUndoesItselfWhenTheRowsDoNotMatchTheirModules(t *testing.T) {
	defer func() { afterVehicleRows = nil }()
	o := spawnOps(t)
	before := vehicleRowsDump(t, o, 578, 579)
	for name, tamper := range map[string]func(m *save.Mut, id int64) error{
		"fuel": func(m *save.Mut, id int64) error {
			_, err := m.Exec(`update fgl_entities set components=jsonb_set(components,'$.FVehicleComponent[1].CurrentFuel',1.5) where entity_id in (select entity_id from actor_fgl_entities where actor_id=? and slot_name='Actor')`, id)
			return err
		},
		"health": func(m *save.Mut, id int64) error {
			_, err := m.Exec(`update actors set properties=jsonb_set(properties,'$.DamageableActorComponent.m_TotalMaxHealth',9999.0) where id=?`, id)
			return err
		},
		"cargo": func(m *save.Mut, id int64) error {
			_, err := m.Exec(`update inventories set max_item_count=99 where actor_id=? and inventory_type=0`, id)
			return err
		},
		"a module": func(m *save.Mut, id int64) error {
			_, err := m.Exec(`update vehicle_modules set template_id='BuggyChassis_0' where vehicle_id=? and template_id='BuggyEngine_4'`, id)
			return err
		},
		"a dangling reference": func(m *save.Mut, id int64) error {
			_, err := m.Exec(`insert into inventories(id,actor_id,inventory_type) values (987654, 424242, NULL)`)
			return err
		},
	} {
		afterVehicleRows = tamper
		if _, err := o.SpawnVehicle(Args{"class": "Buggy", "template": "T4_Boost"}); err == nil || !strings.Contains(err.Error(), "nothing was changed") {
			t.Errorf("%s: %v", name, err)
		}
		if _, err := o.SpawnVehiclePreview(Args{"class": "Buggy", "template": "T4_Boost"}); err == nil {
			t.Errorf("%s: the preview runs the same checks", name)
		}
		if o.S.Dirty() || vehicleRowsDump(t, o, 578, 579) != before {
			t.Fatalf("%s: the edit must be undone", name)
		}
	}
	afterVehicleRows = nil
	if n := mustOne(t, o, `select count(*) c from actors where class like '%Buggy%'`)["c"].(int64); n != 1 {
		t.Errorf("%d buggies after the undone edits", n)
	}
}

// What the editor's own reads and actions do with a created vehicle: it is listed as the character's, with its fuel and modules, Repair and Refuel accept it, and Duplicate copies it.
func TestACreatedVehicleIsTheCharactersToTheEditorsOwnActions(t *testing.T) {
	o := spawnOps(t)
	id, _ := spawn(t, o, Args{"class": "Buggy", "template": "T5_Mining", "fuel": 0.25})
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	var found map[string]any
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		if v["id"] == id {
			found = v
		}
	}
	if found == nil || len(found["modules"].([]map[string]any)) != 11 || found["fuel"] != 70.0 {
		t.Fatalf("listed: %v", found)
	}
	if _, err := o.RepairVehiclesWhere(100, id); err != nil {
		t.Errorf("repair: %v", err)
	}
	dr, err := o.DuplicateVehicle(Args{"id": float64(id)})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	cp := newVehicleID(t, dr)
	if !sameSet(viewOf(t, o, cp).modules, viewOf(t, o, id).modules) {
		t.Error("the copy has the modules")
	}
	if _, err := o.RefuelVehicles(); err != nil {
		t.Errorf("refuel: %v", err)
	}
	assertDBSound(t, o)
}

func TestVehicleCatalogAnswerNamesTheBuildClassesTiersAndOptions(t *testing.T) {
	r, err := (&Ops{}).VehicleCatalog()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["build"] != catalog.VehicleBuild() {
		t.Error("build")
	}
	classes := m["classes"].([]map[string]any)
	if len(classes) != 7 || classes[1]["key"] != "Buggy" {
		t.Fatalf("classes: %v", classes)
	}
	var util map[string]any
	for _, s := range classes[1]["slots"].([]map[string]any) {
		if s["slot"] == "UtilitySlot1" {
			util = s
		}
	}
	found := false
	for _, o := range util["modules"].([]map[string]any) {
		if o["id"] == "BuggyInventory_Unique_Capacity_6" && o["name"] == "Bigger Buggy Boot Mk6" && o["cargoSlots"] == int64(35) && o["unique"] == true {
			found = true
		}
	}
	if !found {
		t.Error("the options of a slot name the unique storage module with its in-game name")
	}
	var t6 map[string]any
	for _, tp := range classes[1]["templates"].([]map[string]any) {
		if tp["name"] == "T6_Combat" {
			t6 = tp
		}
	}
	if t6["tier"] != 6 || t6["maxHealth"] != 650.0 || t6["fuel"] != 300.0 {
		t.Errorf("T6_Combat: %v", t6)
	}
}

// Ids: above the largest id in use and above the table's sequence (a row the game deleted left its number behind), for the actor, the modules and the inventories; the new vehicle
// is owned by the character alone, and a second vehicle gets ids above the first's.
func TestSpawnIDsAreAboveEverythingInUseAndTheSequences(t *testing.T) {
	o := spawnOps(t)
	for name, seq := range map[string]int{"actors": 5000, "vehicle_modules": 7000, "inventories": 9000} {
		testsave.Exec(t, o.S, `delete from sqlite_sequence where name=?`, name)
		testsave.Exec(t, o.S, `insert into sqlite_sequence(name,seq) values (?,?)`, name, seq)
	}
	testsave.Exec(t, o.S, `insert into permission_actor_rank(permission_actor_id,player_id,rank) values (578,579,2)`) // another player's rank on 578
	id, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	if id != 5001 {
		t.Errorf("actor id %d, want 5001", id)
	}
	if n := mustOne(t, o, `select min(id) m from vehicle_modules where vehicle_id=?`, id)["m"].(int64); n != 7001 {
		t.Errorf("first module id %d, want 7001", n)
	}
	if n := mustOne(t, o, `select min(id) m from inventories where actor_id=?`, id)["m"].(int64); n != 9001 {
		t.Errorf("first inventory id %d, want 9001", n)
	}
	if rk, _ := o.S.Query(`select player_id p, rank r from permission_actor_rank where permission_actor_id=?`, id); len(rk) != 1 || rk[0]["p"] != int64(1) {
		t.Errorf("only the character's rank: %v", rk)
	}
	id2, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0"})
	if id2 != id+1 {
		t.Errorf("second vehicle %d", id2)
	}
	if n := mustOne(t, o, `select min(id) m from vehicle_modules where vehicle_id=?`, id2)["m"].(int64); n != 7001+9 {
		t.Errorf("second vehicle's modules start at %d", n)
	}
	// deleting a created vehicle's actor takes its rows with it (the foreign keys cascade) and the two entities with the links (the schema's trigger)
	testsave.Exec(t, o.S, `pragma foreign_keys=on`)
	if _, err := o.S.MutateCascade("delete the created vehicle", func(m *save.Mut) error {
		_, err := m.Exec(`delete from actors where id=?`, id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`select count(*) c from vehicle_modules where vehicle_id=?`, `select count(*) c from inventories where actor_id=?`, `select count(*) c from actor_fgl_entities where actor_id=?`} {
		if n := mustOne(t, o, q, id)["c"].(int64); n != 0 {
			t.Errorf("%s left %d rows", q, n)
		}
	}
	assertDBSound(t, o)
}
