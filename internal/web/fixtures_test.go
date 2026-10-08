package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// uiRoutes are the GET endpoints the UI reads. Responses are produced by the REAL handlers over a real-schema
// save in which every TEXT column holds testsave.HostileText, then replayed by web-tests/ in a browser engine.
var uiRoutes = []string{
	"/api/overview", "/api/save/state", "/api/player", "/api/player/inventory", "/api/player/factions", "/api/player/specs",
	"/api/player/tutorials", "/api/player/tags", "/api/player/recipes", "/api/player/journey",
	"/api/catalog/items", "/api/settings", "/api/bases", "/api/bases/claim", "/api/bases/storage", "/api/bases/storage/items?inventory=1", "/api/vehicles", "/api/vendors", "/api/landsraad",
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

	ok := 0
	for _, v := range out {
		if v.(map[string]any)["status"].(int) == http.StatusOK {
			ok++
		}
	}
	t.Logf("fixture routes: %d, answered 200: %d", len(out), ok)
	if ok < 10 {
		t.Fatalf("only %d routes answered 200 over the hostile real-schema save; the UI test would not exercise enough of the UI", ok)
	}
	b, _ := json.MarshalIndent(out, "", " ")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ui-fixtures.json"), b, 0o600); err != nil {
		t.Fatal(err)
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
}
