package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// uiRoutes are the GET endpoints the UI reads. Responses are produced by the REAL handlers over a real-schema
// save in which every TEXT column holds testsave.HostileText, then replayed by web-tests/ in a browser engine.
var uiRoutes = []string{
	"/api/overview", "/api/save/state", "/api/player", "/api/player/summary", "/api/player/inventory", "/api/player/blood-sacks", "/api/player/factions", "/api/player/building-sets", "/api/player/customizations", "/api/player/research", "/api/player/crafting", "/api/player/skills", "/api/player/journey/browse", "/api/player/faction", "/api/player/specs",
	"/api/player/tutorials", "/api/player/tags", "/api/player/recipes", "/api/player/journey",
	"/api/catalog/items", "/api/settings", "/api/bases", "/api/bases/list", "/api/bases/power?base=900001", "/api/bases/water?base=900001", "/api/bases/inventory?base=900001", "/api/bases/claim", "/api/bases/storage", "/api/bases/storage/items?inventory=1", "/api/livemap", "/api/vehicles", "/api/vendors", "/api/landsraad",
	"/api/vehicles/fuel", "/api/player/liquids",
	"/api/db/tables", "/api/save/backups",
	"/api/config/validate", "/api/config/files",
}

// TestDumpUIFixtures is a generator, not an assertion: it runs only when UI_FIXTURES_DIR is set (CI sets it for the
// browser test job). It fails if the hostile save makes a route return something other than a response, so the
// fixtures cannot silently become empty.
func TestDumpUIFixtures(t *testing.T) {
	dir := os.Getenv("UI_FIXTURES_DIR")
	if dir == "" {
		t.Skip("UI_FIXTURES_DIR not set")
	}
	cfgDir := t.TempDir()
	hostileINI := "[Section" + testsave.HostileText + "]\nKey" + "x" + "=" + testsave.HostileText + "\nOther=" + testsave.HostileText + "\n"
	for _, n := range []string{"ServerCustomSettings.ini", "Game.ini", "GameUserSettings.ini", "Engine.ini", "Input.ini"} {
		if err := os.WriteFile(filepath.Join(cfgDir, n), []byte(hostileINI), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sv := testsave.Hostile(t)
	addHostileStorage(t, sv)
	addHostileCharacter(t, sv)
	s := New(sv, config.Dir{Path: cfgDir})
	cookie := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}

	out := map[string]any{}
	get := func(route string) {
		t.Helper()
		w := do(t, s, "GET", "http://127.0.0.1"+route, loop, cookie, nil)
		var body any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: not JSON (%d): %.200s", route, w.Code, w.Body.String())
		}
		u, _ := url.Parse(route)
		out[u.Path] = map[string]any{"status": w.Code, "body": body}
	}
	for _, r := range uiRoutes {
		get(r)
	}
	// data-dependent routes: use the first table / first config file exactly as the UI does
	var tables []map[string]any
	json.Unmarshal([]byte(mustJSON(t, out["/api/db/tables"].(map[string]any)["body"])), &tables)
	if len(tables) == 0 {
		t.Fatal("no tables in the hostile save")
	}
	get("/api/db/table?name=" + tables[0]["name"].(string) + "&limit=100&offset=0")
	var files []map[string]any
	json.Unmarshal([]byte(mustJSON(t, out["/api/config/files"].(map[string]any)["body"])), &files)
	if len(files) > 0 {
		name := url.QueryEscape(files[0]["name"].(string))
		get("/api/config/file?name=" + name)
		get("/api/config/backups?name=" + name)
	}

	// Every route must answer 200 over the hostile save, except the ones named here with the reason. The list is empty on purpose:
	// a route that fails or comes back empty means the browser test is rendering a view that was never fed hostile text.
	allowedNon200 := map[string]string{}
	ok := 0
	for route, v := range out {
		status := v.(map[string]any)["status"].(int)
		if status == http.StatusOK {
			ok++
		} else if allowedNon200[route] == "" {
			t.Errorf("%s answered %d over the hostile save: %v", route, status, v.(map[string]any)["body"])
		}
	}
	t.Logf("fixture routes: %d, answered 200: %d", len(out), ok)
	// views whose rows come from text in the save must actually carry the hostile text, or the browser test checks nothing there
	for _, route := range []string{"/api/player/skills", "/api/player/research", "/api/player/crafting", "/api/player/inventory", "/api/bases/water", "/api/bases/inventory", "/api/vehicles"} {
		v, found := out[route]
		if !found || !strings.Contains(mustJSON(t, v.(map[string]any)["body"]), "__pwned") {
			t.Errorf("%s does not carry the hostile text; the fixture is not feeding that view", route)
		}
	}
	// the power tab names only known devices, so it carries no save text, but it must list the generator that was seeded
	if body := mustJSON(t, out["/api/bases/power"].(map[string]any)["body"]); !strings.Contains(body, "Fuel-Powered Generator") {
		t.Errorf("/api/bases/power lists no generator: %s", body)
	}
	b, _ := json.MarshalIndent(out, "", " ")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ui-fixtures.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// addHostileCharacter gives the hostile save what the Player views need to list anything: the character's body (found by its
// class and owner, which the generic one-row-per-table fill cannot satisfy), its research and known recipes, its skill record
// with a module the catalog does not know, a backpack with a hostile item, and a generator and a water building in the base
// that addHostileStorage made. Without these the skills route answers 400 and research, crafting, power, water and inventory
// answer empty lists, so the browser test would pass over views that were never fed hostile text (audit QA-9).
func addHostileCharacter(t *testing.T, sv *save.Save) {
	t.Helper()
	h := testsave.HostileText
	skillID := "Skills.Perk." + strings.TrimPrefix(h, `">`) // a module key cannot hold a double quote (the key is `(TagName="...")`)
	props := mustJSON(t, map[string]any{
		"TechKnowledgePlayerComponent": map[string]any{"m_TechKnowledge": map[string]any{"m_TechKnowledgeData": []any{
			map[string]any{"ItemKey": "RCP_" + h, "UnlockedState": h, "bIsNewEntry": true},
			map[string]any{"ItemKey": "BLD_" + h + "_Patent", "UnlockedState": "Purchased", "bIsNewEntry": false},
		}}},
		"CraftingRecipesLibraryActorComponent": map[string]any{"m_KnownItemRecipes": []any{
			map[string]any{"BaseRecipeId": map[string]any{"Name": h}, "m_Source": h, "m_NumberOfRecipeUses": 1, "m_bIsLimitedUseRecipe": true},
		}},
	})
	level := mustJSON(t, map[string]any{"FLevelComponent": []any{[]any{}, map[string]any{
		"ModuleData":    map[string]any{`(TagName="` + skillID + `")`: map[string]any{"SkillPointsSpent": 3}},
		"TotalXPEarned": 0, "TotalSkillPoints": 10, "UnspentSkillPoints": 2}}})
	water := mustJSON(t, map[string]any{"FWaterStorageComponent": []any{[]any{}, map[string]any{"m_WaterStored": 100}}})
	for _, q := range [][]any{
		{`insert into actors(id,class,map,owner_account_id,properties) values (900010,'/Game/Dune/Characters/Player/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C','HaggaBasin',1,jsonb(?))`, props},
		{`update player_state set player_pawn_id=900010 where id=1`}, // the game links the player to this pawn; the editor follows that link
		{`insert into fgl_entities(entity_id,components) values (-900010, jsonb(?))`, level},
		{`insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (900010,-900010,'DuneCharacter')`},
		{`insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (900011,900010,0,20,1000.0)`},
		{`insert into items(id,inventory_id,stack_size,position_index,template_id,stats) values (900012,900011,1,0,?,?)`, h, mustJSON(t, map[string]any{"Note": h})}, // stats is a JSON document: the editor refuses to store text that is not JSON there
		// a generator holding fuel, and a water building of a kind the editor does not know (so its name is the save's own text)
		{`insert into actors(id,class,map) values (900020,'/Game/Water.Water_C','HaggaBasin'),(900021,'/Game/Generator.Generator_C','HaggaBasin')`},
		{`insert into fgl_entities(entity_id,components) values (-900020, jsonb(?))`, water},
		{`insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (900020,-900020,'Water')`},
		{`insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id) values (900020,-900001,?,1),(900021,-900001,'Generator_Placeable',1)`, h},
		{`insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (900022,900021,3,5,100.0)`},
		{`insert into items(id,inventory_id,stack_size,position_index,template_id,stats) values (900023,900022,5,0,'Oil','{}')`},
	} {
		testsave.Exec(t, sv, q[0].(string), q[1:]...)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// addHostileStorage gives the hostile save what the Storage view needs to list anything: a base (a totem with its entity), and
// a real storage container in it whose own name is the hostile text. It reuses inventory 1, so the items route the UI calls
// (inventory=1) returns the hostile items. Without this the base-linked storage list is empty and the view is not exercised.
func addHostileStorage(t *testing.T, sv *save.Save) {
	t.Helper()
	for _, q := range []string{
		`insert into actors(id,class,map) values (900001,'/Game/BP_Totem.BP_Totem_C','HaggaBasin'),(900002,'/Game/Container.Container_C','HaggaBasin')`,
		`insert into totems(id) values (900001)`,
		`insert into fgl_entities(entity_id,components) values (-900001, jsonb('{}'))`,
		`insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (900001,-900001,'Actor')`,
		`insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id) values (900002,-900001,'GenericContainer_Placeable',1)`,
		`update inventories set actor_id=900002, max_item_count=20 where id=1`,
	} {
		testsave.Exec(t, sv, q)
	}
	testsave.Exec(t, sv, `insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (900002,?,1,3,1)`, testsave.HostileText)
	// a map marker on the player's map whose type is the hostile text, so the Live Map view is exercised
	testsave.Exec(t, sv, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (900003,0,'HaggaBasin',?,1000,2000,0)`, testsave.HostileText)
}
