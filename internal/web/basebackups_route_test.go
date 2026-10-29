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
