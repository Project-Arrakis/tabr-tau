package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// POST /api/vehicles/spawn-preview, POST /api/vehicles/spawn and GET /api/vehicles/catalog (#401). The character's body has one entity (the prefix a created vehicle's entities
// are numbered from when the save has no vehicle with entities).
const spawnRouteWorld = `
insert into fgl_entities(entity_id,components) values (8300000000000001, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,8300000000000001,'DuneCharacter');`

func spawnServer(t *testing.T) *Server {
	t.Helper()
	return New(testsave.PlayerWithSQL(t, spawnRouteWorld), config.Dir{Path: t.TempDir()})
}

func vehiclesInFile(t *testing.T, path string) int64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select count(*) c from vehicles`)
	if err != nil {
		t.Fatal(err)
	}
	return r["c"].(int64)
}

const mk6Body = `{"class":"Buggy","template":"T6_Combat","uniques":{"Engine":"BuggyEngine_Unique_Accelerate_06","UtilitySlot1":"BuggyInventory_Unique_Capacity_06","UtilitySlot2":"BuggyMining_Unique_YieldIncrease_06"},"items":[{"template_id":"MelangeSpice","quantity":10}]}`

func TestAPreviewOfAVehicleKeepsNothingAndTheCreateIsWrittenAtOnce(t *testing.T) {
	s := spawnServer(t)
	w := postEdit(t, s, "/api/vehicles/spawn-preview", mk6Body)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["dryRun"] != true || m["outcome"] != nil || m["rows"] == nil || m["id"] == nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if len(s.Save.Backups()) != 0 || s.Save.Dirty() || vehiclesInFile(t, s.Save.Path) != 0 {
		t.Fatal("a preview writes nothing and makes no backup")
	}
	w = postEdit(t, s, "/api/vehicles/spawn", mk6Body)
	a := decodeBody(t, w)
	if w.Code != 200 || a["ok"] != true || a["dryRun"] != false || a["outcome"] != "written" || a["id"] != m["id"] || a["complete"] != true {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup was made before the write")
	}
	if vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatal("the vehicle is in the game's file")
	}
	// the page can list it as the character's
	if body := postEditGet(t, s, "/api/vehicles"); !strings.Contains(body, "BP_Buggy_CHOAM_C") {
		t.Errorf("the vehicles list: %s", body)
	}
}

func TestACreatedVehicleWaitsWhileTheGameHoldsTheFile(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := spawnServer(t)
	w := postEdit(t, s, "/api/vehicles/spawn", `{"class":"Sandbike","template":"T0"}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["ok"] != true {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if vehiclesInFile(t, s.Save.Path) != 0 {
		t.Fatal("the file is not touched while the game holds it")
	}
	if p := s.Save.Pending(); len(p) != 1 || !strings.Contains(p[0], "created vehicle") {
		t.Fatalf("the vehicle waits as one entry: %v", p)
	}
}

func TestTheVehicleRoutesRefuseWhatIsWrongWithACodeAndChangeNothing(t *testing.T) {
	s := spawnServer(t)
	for name, body := range map[string]string{
		"a module in the wrong slot": `{"class":"Buggy","template":"T0","uniques":{"Engine":"BuggyBoost_4"}}`,
		"an unknown class":           `{"class":"Tank","template":"T0"}`,
		"fuel above one":             `{"class":"Buggy","template":"T0","fuel":2}`,
	} {
		for _, route := range []string{"/api/vehicles/spawn", "/api/vehicles/spawn-preview"} {
			w := postEdit(t, s, route, body)
			if w.Code < 400 || w.Code >= 500 || decodeBody(t, w)["code"] != "bad_argument" {
				t.Errorf("%s %s: %d %s", name, route, w.Code, w.Body.String())
			}
		}
	}
	if w := postEdit(t, s, "/api/vehicles/spawn", `{`); w.Code != http.StatusBadRequest {
		t.Errorf("a body that is not JSON: %d", w.Code)
	}
	if s.Save.Dirty() || len(s.Save.Backups()) != 0 {
		t.Fatal("refusals change nothing")
	}
}

func TestTheVehicleCatalogRouteListsTheClassesTheirTiersAndModules(t *testing.T) {
	s := spawnServer(t)
	body := postEditGet(t, s, "/api/vehicles/catalog")
	for _, want := range []string{`"build":"1.5.15.0"`, `"T6_Combat"`, `"BuggyInventory_Unique_Capacity_6"`, `"Bigger Buggy Boot Mk6"`, `"OrnithopterLight"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the catalog answer lacks %s", want)
		}
	}
	if strings.Contains(body, "Tank") || strings.Contains(body, "TreadWheel") {
		t.Error("Tank and Treadwheel are not offered")
	}
}
