package web

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// The Structures tab routes (#526) take the base id: the read lists that base only, the two writes change that base only, and an id that is
// not a totem is refused with 422 before anything is read or written. The whole-save writes are separate routes.
const structuresWorld = `
insert into actors(id,class,map) values (601,'t','HaggaBasin'),(602,'t','HaggaBasin'),(900,'x','HaggaBasin'),(801,'b','HaggaBasin'),(802,'b','HaggaBasin');
insert into totems(id) values (601),(602);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}')),(-602, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor'),(602,-602,'Actor');
insert into building_instances(building_id,instance_id,building_type,health,sand_buildup,owner_entity_id) values
  (801,1,'Wall',1000.0,7,-601),(801,2,'Wall',2000.0,0,-601),(802,1,'Wall',5600.0,9,-602),(802,2,'Wall',300.0,4,NULL);
insert into placeables(id,owner_entity_id,health,building_type,last_placed_by_player_id,is_hologram) values
  (601,-601,5000.0,'Totem_Placeable',1,0),(602,-602,4000.0,'Totem_Placeable',1,0);`

func healthsOf(t *testing.T, s *Server) string {
	t.Helper()
	rows, err := s.Save.Query(`select building_id b, instance_id i, health h, sand_buildup s from building_instances order by 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(rows)
}

func TestStructuresRouteListsTheOpenBaseOnly(t *testing.T) {
	s := backupServer(t, structuresWorld)
	w := kitGet(t, s, "/api/bases/structures?base=601")
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	p := out["pieces"].(map[string]any)
	if p["n"] != float64(2) || p["minh"] != float64(1000) || p["sand"] != float64(7) {
		t.Fatalf("base 601's card: %v", p)
	}
	if n := len(out["placeables"].([]any)); n != 1 {
		t.Fatalf("base 601's placeables: %v", out["placeables"])
	}
	if up := out["unowned"].(map[string]any)["pieces"].(map[string]any); up["n"] != float64(1) {
		t.Fatalf("unowned: %v", out["unowned"])
	}
	for _, route := range []string{"/api/bases/structures", "/api/bases/structures?base=900", "/api/bases/structures?base=abc", "/api/bases/structures?base=-601", "/api/bases/structures?base=801"} {
		if w := kitGet(t, s, route); w.Code != 422 {
			t.Errorf("%s: %d %s", route, w.Code, w.Body.String())
		}
	}
}

func TestRepairAndSandRoutesAreOneBaseAndRefuseABadId(t *testing.T) {
	s := backupServer(t, structuresWorld)
	file := func() []byte { b, _ := os.ReadFile(s.Save.Path); return b }
	before, rows := file(), healthsOf(t, s)
	for _, route := range []string{"/api/bases/repair", "/api/bases/clear-sand"} {
		for _, body := range []string{`{}`, `{"base_id":0}`, `{"base_id":900}`, `{"base_id":801}`, `{"base_id":"x"}`, `{"base_id":1.5}`, `{"base_id":-601}`} {
			if w := postEdit(t, s, route, body); w.Code != 422 {
				t.Errorf("%s %s: %d %s", route, body, w.Code, w.Body.String())
			}
		}
	}
	if string(file()) != string(before) || healthsOf(t, s) != rows {
		t.Fatal("a refused request must write nothing")
	}

	if w := postEdit(t, s, "/api/bases/repair", `{"base_id":601}`); w.Code != 200 {
		t.Fatalf("repair: %d %s", w.Code, w.Body.String())
	}
	got, _ := s.Save.Query(`select health h from building_instances order by building_id, instance_id`)
	if got[0]["h"].(float64) != 5600 || got[1]["h"].(float64) != 5600 || got[2]["h"].(float64) != 5600 || got[3]["h"].(float64) != 300 {
		t.Fatalf("only base 601's walls go up to the best of the type: %v", got)
	}
	if w := postEdit(t, s, "/api/bases/clear-sand", `{"base_id":601}`); w.Code != 200 {
		t.Fatalf("sand: %d %s", w.Code, w.Body.String())
	}
	sand, _ := s.Save.Query(`select sand_buildup n from building_instances order by building_id, instance_id`)
	if sand[0]["n"].(int64) != 0 || sand[2]["n"].(int64) != 9 || sand[3]["n"].(int64) != 4 {
		t.Fatalf("only base 601's sand is cleared: %v", sand)
	}

	// the whole-save actions are their own routes
	if w := postEdit(t, s, "/api/bases/clear-sand-all", `{}`); w.Code != 200 {
		t.Fatalf("sand all: %d %s", w.Code, w.Body.String())
	}
	if w := postEdit(t, s, "/api/bases/repair-all", `{}`); w.Code != 200 {
		t.Fatalf("repair all: %d %s", w.Code, w.Body.String())
	}
	all, _ := s.Save.Query(`select health h, sand_buildup n from building_instances order by building_id, instance_id`)
	if all[3]["h"].(float64) != 5600 || all[3]["n"].(int64) != 0 || all[2]["n"].(int64) != 0 {
		t.Fatalf("the save-wide actions reach every row: %v", all)
	}
}
