package web

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The base backup routes (#475, phase 1) only read the save: the list, and one backup as a file for the browser to save.

const backupWorld = `
insert into actors(id,class,map,state) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5),(602,'/Game/BP_Building.BP_Building_C','HaggaBasin',5);
insert into fgl_entities(entity_id,components) values (5764607523034234881, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,5764607523034234881,'Actor');
insert into placeables(id,owner_entity_id,building_type) values (601,5764607523034234881,'Totem_Small_Placeable');
insert into totems(id) values (601);
insert into buildings(id) values (602);
insert into building_instances(building_id,instance_id,building_type,owner_entity_id) values (602,1,'Foundation',5764607523034234881);
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (1,1,'My base',1);
insert into base_backup_linked_actors(id,actor_id) values (1,601),(1,602);`

func backupServer(t *testing.T, world string) *Server {
	t.Helper()
	s := New(testsave.PlayerWithSQL(t, world), config.Dir{Path: t.TempDir()})
	s.EditorVersion = "9.9.9"
	s.now = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	return s
}

func TestBaseBackupRoutesListAndExportWithoutTouchingTheSave(t *testing.T) {
	s := backupServer(t, backupWorld)
	before, _ := os.ReadFile(s.Save.Path)
	w := kitGet(t, s, "/api/bases/backups")
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 || len(list) != 1 || list[0]["name"] != "My base" || list[0]["pieces"] != float64(1) || list[0]["placeables"] != float64(1) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	w = kitGet(t, s, "/api/bases/backups/export?backup=1")
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("X-Content-Type-Options") != "nosniff" ||
		w.Header().Get("Content-Disposition") != `attachment; filename="tabr-tau-base-backup-1-My_base.json"` {
		t.Fatalf("export: %d %v", w.Code, w.Header())
	}
	f, err := basefile.Check(w.Body.Bytes())
	if err != nil || f.Editor != "9.9.9" || f.ExportedAt != "2026-10-28T12:00:00Z" || f.Manifest.Summary.Pieces != 1 || f.Manifest.Summary.Actors != 2 {
		t.Fatalf("the file: %v %+v", err, f)
	}
	if !strings.Contains(w.Body.String(), "5764607523034234881") {
		t.Error("the 64-bit entity id is not in the file as written in the save")
	}
	untouched(t, s, before)
}

func TestBaseBackupExportRefusesWhatItCannotMake(t *testing.T) {
	s := backupServer(t, backupWorld)
	for route, code := range map[string]int{
		"/api/bases/backups/export":               422, // no number
		"/api/bases/backups/export?backup=abc":    422,
		"/api/bases/backups/export?backup=-1":     422,
		"/api/bases/backups/export?backup=1e3":    422,
		"/api/bases/backups/export?backup=99":     404,
		"/api/bases/backups/export?backup=1;drop": 422,
	} {
		w := kitGet(t, s, route)
		if w.Code != code || w.Header().Get("Content-Disposition") != "" || decodeBody(t, w)["error"] == nil {
			t.Errorf("%s: %d %s", route, w.Code, w.Body.String())
		}
	}
	none := backupServer(t, "")
	w := kitGet(t, none, "/api/bases/backups/export?backup=1")
	if m := decodeBody(t, w); w.Code != 404 || !strings.Contains(m["error"].(string), "This save has no base backup") {
		t.Errorf("a save with no backup: %d %s", w.Code, w.Body.String())
	}
	w = kitGet(t, none, "/api/bases/backups")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("an empty list is []: %d %s", w.Code, w.Body.String())
	}
}

func TestBaseBackupRoutesNeedTheSessionLikeEveryOtherRoute(t *testing.T) {
	s := backupServer(t, backupWorld)
	for _, route := range []string{"/api/bases/backups", "/api/bases/backups/export?backup=1"} {
		w := do(t, s, "GET", "http://127.0.0.1"+route, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
		if w.Code != 403 {
			t.Errorf("%s without the session: %d", route, w.Code)
		}
	}
}

// The Dune console's file (#534): a base that is still standing, and a backup in the console's format. Both only read the save.

const placedAt = `update actors set location_x=1,location_y=2,location_z=3,rotation_x=0,rotation_y=0,rotation_z=0,rotation_w=1; update building_instances set location_x=1,location_y=2,location_z=3,rotation_x=0,rotation_y=0,rotation_z=0,rotation_w=1;`

const standingRouteWorld = `
insert into actors(id,class,map,state) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',0),(602,'/Game/BP_Building.BP_Building_C','HaggaBasin',0);
insert into fgl_entities(entity_id,components) values (5764607523034234881, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,5764607523034234881,'Actor');
insert into placeables(id,owner_entity_id,building_type,has_buildable_support) values (601,5764607523034234881,'Totem_Small_Placeable',1);
insert into totems(id) values (601);
insert into buildings(id) values (602);
insert into building_instances(building_id,instance_id,building_type,owner_entity_id) values (602,1,'Foundation',5764607523034234881),(602,2,'Wall',5764607523034234881);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (601,'My base',1,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1);` + placedAt

func TestStandingBaseConsoleExportRouteSendsTheFileAndItsCounts(t *testing.T) {
	s := backupServer(t, standingRouteWorld)
	before, _ := os.ReadFile(s.Save.Path)
	w := kitGet(t, s, "/api/bases/console-export?base=601")
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("X-Content-Type-Options") != "nosniff" ||
		w.Header().Get("Content-Disposition") != `attachment; filename="tabr-tau-console-base-601-My_base.json"` {
		t.Fatalf("export: %d %v", w.Code, w.Header())
	}
	b, err := basefile.Load(w.Body.Bytes(), basefile.MaxFileBytes, basefile.MaxRows)
	if err == nil {
		err = b.Verify()
	}
	if err != nil || b.Origin != basefile.OriginConsole || b.Console.SourceKind != "live-base" || b.Console.Name != "My base" {
		t.Fatalf("the file: %v %+v", err, b.Console)
	}
	var counts map[string]int
	if err := json.Unmarshal([]byte(w.Header().Get("X-Base-Counts")), &counts); err != nil || counts["pieces"] != 2 || counts["placeables"] != 1 || counts["actors"] != 2 || counts["storedItems"] != 0 || counts["entries"] != 12 {
		t.Errorf("counts header %q: %v", w.Header().Get("X-Base-Counts"), err)
	}
	if h := w.Header().Get("X-Base-Notes"); h != "%5B%5D" {
		t.Errorf("notes header %q", h)
	}
	untouched(t, s, before)
}

func TestConsoleExportRoutesRefuseWhatTheyCannotMake(t *testing.T) {
	s := backupServer(t, standingRouteWorld)
	for route, want := range map[string]int{
		"/api/bases/console-export":                         422,
		"/api/bases/console-export?base=abc":                422,
		"/api/bases/console-export?base=-1":                 422,
		"/api/bases/console-export?base=1;drop":             422,
		"/api/bases/console-export?base=99":                 404,
		"/api/bases/backups/export?backup=9&format=console": 404,
	} {
		w := kitGet(t, s, route)
		if w.Code != want || w.Header().Get("Content-Disposition") != "" || decodeBody(t, w)["error"] == nil {
			t.Errorf("%s: %d %s", route, w.Code, w.Body.String())
		}
	}
	// a base that was picked up is exported from Base backups, and says so
	p := backupServer(t, backupWorld)
	w := kitGet(t, p, "/api/bases/console-export?base=601")
	if m := decodeBody(t, w); w.Code != 409 || !strings.Contains(m["error"].(string), "Export it from Base backups") {
		t.Errorf("a picked-up base: %d %s", w.Code, w.Body.String())
	}
}

func TestPickedUpBaseConsoleExportRouteSendsTheConsolesFile(t *testing.T) {
	s := backupServer(t, backupWorld+placedAt)
	before, _ := os.ReadFile(s.Save.Path)
	w := kitGet(t, s, "/api/bases/backups/export?backup=1&format=console")
	if w.Code != 200 || w.Header().Get("Content-Disposition") != `attachment; filename="tabr-tau-console-backup-1-My_base.json"` {
		t.Fatalf("export: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	b, err := basefile.Load(w.Body.Bytes(), basefile.MaxFileBytes, basefile.MaxRows)
	if err == nil {
		err = b.Verify()
	}
	if err != nil || b.Console.Kinds["BuildingInstance"] != 1 || b.Console.SourceKind != "" {
		t.Fatalf("the file: %v %+v", err, b.Console)
	}
	// without format the editor's own file, as before
	if w := kitGet(t, s, "/api/bases/backups/export?backup=1"); !strings.Contains(w.Header().Get("Content-Disposition"), "tabr-tau-base-backup-1") {
		t.Errorf("the default export changed: %v", w.Header())
	}
	untouched(t, s, before)
}

func TestConsoleExportRouteNeedsTheSession(t *testing.T) {
	s := backupServer(t, standingRouteWorld)
	w := do(t, s, "GET", "http://127.0.0.1/api/bases/console-export?base=601", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
	if w.Code != 403 {
		t.Errorf("without the session: %d", w.Code)
	}
}
