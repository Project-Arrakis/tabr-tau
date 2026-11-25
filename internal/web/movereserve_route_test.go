package web

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The Move the Reserve routes (#1191): a preview that is the edit rolled back, and the edit as an ordinary write that must carry the fingerprint of the preview, is written after a backup
// of the save, is held while a game process exists, and refuses a box it cannot find.
const moveReserveRouteWorld = `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (12,2,33,-1,0.0),(7001,701,4,3,1000.0),(7002,702,4,3,1000.0);
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,state) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0,0,0,1,0),
  (701,'/Game/BP_Box.BP_Box_C','HaggaBasin',5001,6000,7,0,0,0,1,0),(702,'/Game/BP_Box.BP_Box_C','HaggaBasin',5002,6000,7,0,0,0,1,0);
insert into fgl_entities(entity_id,components) values (5764607523034234881, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,5764607523034234881,'Actor');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (601,'Home',3,3,0,0),(701,'Box A',1,3,1,0),(702,'Box B',1,3,1,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support) values (601,5764607523034234881,'Totem_Placeable',1),
  (701,5764607523034234881,'Developer_StorageContainer_Placeable',1),(702,5764607523034234881,'Developer_StorageContainer_Placeable',1);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (9001,12,30,0,'Stone',0,1790000001,'{}',0),(9002,12,20,1,'Stone',0,1790000002,'{}',0),(9003,12,10,2,'ScrapMetal',0,1790000003,'{}',0),(9004,7001,5,0,'Stone',0,1790000004,'{}',0);
insert into totems(id) values (601);`

func moveReserveServer(t *testing.T) *Server { return moveReserveServerWith(t, "") }

func moveReserveServerWith(t *testing.T, extra string) *Server {
	t.Helper()
	s := backupServer(t, moveReserveRouteWorld+extra)
	s.KeepPendingEdits(t.TempDir())
	return s
}

const moveReserveQuery = "boxes=Box%20A,Box%20B"

func TestMoveReservePreviewChangesNothingAndTheWriteNeedsItsFingerprint(t *testing.T) {
	s := moveReserveServer(t)
	before, _ := os.ReadFile(s.Save.Path)
	w := kitGet(t, s, "/api/bases/move-reserve-preview?"+moveReserveQuery)
	pv := decodeBody(t, w)
	if w.Code != 200 || pv["dryRun"] != true || pv["stacks"] != float64(3) || pv["items"] != float64(60) || pv["outcome"] != nil || pv["fingerprint"] == "" {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Pending()) != 0 {
		t.Fatal("a preview changed or queued something")
	}
	if w := postEdit(t, s, "/api/bases/move-reserve", `{"boxes":"Box A,Box B"}`); w.Code != 422 || !strings.Contains(w.Body.String(), "fingerprint is missing") {
		t.Fatalf("no fingerprint: %d %s", w.Code, w.Body.String())
	}
	if w := postEdit(t, s, "/api/bases/move-reserve", `{"boxes":"Box A,Box B","fingerprint":"nope"}`); w.Code != 409 {
		t.Fatalf("wrong fingerprint: %d %s", w.Code, w.Body.String())
	}
	ok := postEdit(t, s, "/api/bases/move-reserve", `{"boxes":"Box A,Box B","fingerprint":"`+pv["fingerprint"].(string)+`"}`)
	m := decodeBody(t, ok)
	if ok.Code != 200 || m["outcome"] != "written" || m["stacks"] != float64(3) {
		t.Fatalf("write: %d %s", ok.Code, ok.Body.String())
	}
	chk, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, _ := chk.One(`select (select count(*) from items where inventory_id=12) reserve, (select count(*) from items where inventory_id=7001) a, (select count(*) from items where inventory_id=7002) b`)
	// Box A holds one stack and has 3 slots, so it takes two of the three and Box B the third
	if r["reserve"] != int64(0) || r["a"] != int64(3) || r["b"] != int64(1) {
		t.Fatalf("written rows: %v", r)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup of the save is taken before the write")
	}
	// the Reserve is empty now: asking again is nothing to do, and the write of that writes nothing
	again := decodeBody(t, kitGet(t, s, "/api/bases/move-reserve-preview?"+moveReserveQuery))
	if again["nothingToDo"] != true {
		t.Fatalf("asking again: %v", again)
	}
	n := len(s.Save.Backups())
	w2 := postEdit(t, s, "/api/bases/move-reserve", `{"boxes":"Box A,Box B","fingerprint":"`+again["fingerprint"].(string)+`"}`)
	if m2 := decodeBody(t, w2); w2.Code != 200 || m2["outcome"] != nil || m2["nothingToDo"] != true || len(s.Save.Backups()) != n {
		t.Fatalf("nothing to do writes nothing: %d %s", w2.Code, w2.Body.String())
	}
}

func TestMoveReserveRefusesABoxItCannotFindAndWritesNothing(t *testing.T) {
	s := moveReserveServer(t)
	before, _ := os.ReadFile(s.Save.Path)
	if w := kitGet(t, s, "/api/bases/move-reserve-preview?boxes=Box%20A,Nowhere"); w.Code != 404 || !strings.Contains(w.Body.String(), `named \"Nowhere\"`) {
		t.Fatalf("no such box: %d %s", w.Code, w.Body.String())
	}
	// no names: the boxes DD-1, DD-2 ... are looked for, and this world has none
	if w := kitGet(t, s, "/api/bases/move-reserve-preview"); w.Code != 404 || !strings.Contains(w.Body.String(), "No Developer storage box named DD-1") {
		t.Fatalf("no box named DD-1: %d %s", w.Code, w.Body.String())
	}
	if w := postEdit(t, s, "/api/bases/move-reserve", `{"boxes":"Box A,Box A","fingerprint":"x"}`); w.Code != 422 {
		t.Fatalf("the same box twice: %d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Pending()) != 0 {
		t.Fatal("a refusal wrote or queued something")
	}
}

func TestMoveReserveWhileAGameProcessExistsIsQueuedAsOneEditAndWritesNothing(t *testing.T) {
	s := moveReserveServer(t)
	pv := decodeBody(t, kitGet(t, s, "/api/bases/move-reserve-preview?"+moveReserveQuery))
	before, _ := os.ReadFile(s.Save.Path)
	restore := save.ActiveSessionForTests()
	defer restore()
	w := postEdit(t, s, "/api/bases/move-reserve", `{"boxes":"Box A,Box B","fingerprint":"`+pv["fingerprint"].(string)+`"}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["outcome"] != "queued" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Backups()) != 0 {
		t.Fatal("the save was written while the game was running")
	}
	if p := s.Save.Pending(); len(p) != 1 || !strings.HasPrefix(p[0], "Move the Reserve into storage") {
		t.Fatalf("queued as one edit: %v", p)
	}
}

// The boxes are found by the editor (#1214): the line "Storage boxes" reads them, the preview and the write take no names, the box that stands behind a gap is ignored, and a write
// carries only the fingerprint.
func TestMoveReserveRoutesFindTheBoxesDD1ToDD6Themselves(t *testing.T) {
	s := moveReserveServerWith(t, `update permission_actor set actor_name='DD-1' where actor_id=701; update permission_actor set actor_name='DD-3' where actor_id=702;`)
	d := decodeBody(t, kitGet(t, s, "/api/bases/move-reserve-boxes"))
	if fmt.Sprint(d["found"]) != "[DD-1]" || fmt.Sprint(d["ignored"]) != "[DD-3]" || d["gap"] != "DD-2" || !strings.Contains(d["text"].(string), "Ignored: DD-3") {
		t.Fatalf("the line: %v", d)
	}
	editsBefore := len(s.Save.Pending())
	if editsBefore != 0 {
		t.Fatal("reading the boxes queued something")
	}
	// DD-2 is named: both boxes are found, and the Reserve (3 stacks) needs both (DD-1 has 3 slots and 1 stack in it)
	s2 := moveReserveServerWith(t, `update permission_actor set actor_name='DD-1' where actor_id=701; update permission_actor set actor_name='DD-2' where actor_id=702;`)
	d2 := decodeBody(t, kitGet(t, s2, "/api/bases/move-reserve-boxes"))
	if fmt.Sprint(d2["found"]) != "[DD-1 DD-2]" || d2["max"] != float64(6) {
		t.Fatalf("the line: %v", d2)
	}
	w := kitGet(t, s2, "/api/bases/move-reserve-preview")
	pv := decodeBody(t, w)
	if w.Code != 200 || pv["stacks"] != float64(3) || fmt.Sprint(pv["boxNames"]) != "[DD-1 DD-2]" || pv["autoDetected"] != true {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	ok := postEdit(t, s2, "/api/bases/move-reserve", `{"fingerprint":"`+pv["fingerprint"].(string)+`"}`)
	if m := decodeBody(t, ok); ok.Code != 200 || m["outcome"] != "written" || m["stacks"] != float64(3) {
		t.Fatalf("write: %d %s", ok.Code, ok.Body.String())
	}
	chk, err := save.Open(s2.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, _ := chk.One(`select (select count(*) from items where inventory_id=12) reserve, (select count(*) from items where inventory_id=7001) a, (select count(*) from items where inventory_id=7002) b`)
	if r["reserve"] != int64(0) || r["a"] != int64(3) || r["b"] != int64(1) {
		t.Fatalf("written rows: %v", r)
	}
}
