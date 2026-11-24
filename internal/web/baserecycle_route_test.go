package web

import (
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The Recycle base routes (#1176): a preview that is the edit rolled back, and the edit as an ordinary write that must carry the fingerprint of the preview, is written after a
// backup of the save, is held while a game process exists, and refuses a base that is not the character's own.
const recycleRouteWorld = `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (12,2,33,-1,0.0),(7001,701,4,10,250.0);
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,state) values
  (99,'p','HaggaBasin',0,0,0,0,0,0,1,0),
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0,0,0,1,0),(602,'/Game/BP_Building.BP_Building_C','HaggaBasin',5000,6000,7,0,0,0,1,0),
  (701,'/Game/BP_Silo.BP_Silo_C','HaggaBasin',5000,6000,7,0,0,0,1,0),
  (801,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',9000,9000,0,0,0,0,1,0),(802,'/Game/BP_Building.BP_Building_C','HaggaBasin',9000,9000,0,0,0,0,1,0);
insert into fgl_entities(entity_id,components) values (5764607523034234881, jsonb('{}')),(5764607523034234882, jsonb('{}')),(5764607523034235001, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,5764607523034234881,'Actor'),(602,5764607523034234882,'Actor'),(801,5764607523034235001,'Actor');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (601,'Home',3,3,0,0),(801,'Theirs',3,3,0,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(801,99,1);
insert into buildings(id,owner_id) values (602,null),(802,null);
insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,1,'Sardaukar_Wall_01',5764607523034234881,1.0),(802,1,'Sardaukar_Wall_01',5764607523034235001,1.0);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support) values (601,5764607523034234881,'Totem_Placeable',1),(701,5764607523034234881,'SpiceSilo_Placeable',1),(801,5764607523034235001,'Totem_Placeable',1);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (9001,7001,30,0,'CopperOre',0,1790000001,'{}',0);
insert into totems(id) values (601),(801);`

func recycleServer(t *testing.T) *Server {
	t.Helper()
	s := backupServer(t, recycleRouteWorld)
	s.KeepPendingEdits(t.TempDir())
	return s
}

func TestRecyclePreviewChangesNothingAndTheWriteNeedsItsFingerprint(t *testing.T) {
	s := recycleServer(t)
	before, _ := os.ReadFile(s.Save.Path)
	w := kitGet(t, s, "/api/bases/recycle-preview?base=601")
	pv := decodeBody(t, w)
	if w.Code != 200 || pv["dryRun"] != true || pv["name"] != "Home" || pv["pieces"] != float64(1) || pv["placeables"] != float64(1) || pv["itemStacksMoved"] != float64(1) || pv["destination"] != "Reserve" || pv["outcome"] != nil {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	// the refund is a list of item, name, count and stacks: the wall's Stone 10, the totem's and the silo's ScrapMetal 75
	if got := w.Body.String(); !strings.Contains(got, `{"item":"ScrapMetal","name":"Salvaged Metal","count":75,"stacks":1}`) || !strings.Contains(got, `{"item":"Stone","name":"Granite Stone","count":10,"stacks":1}`) {
		t.Fatalf("refund in the preview: %s", got)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Pending()) != 0 {
		t.Fatal("a preview changed or queued something")
	}
	if w := postEdit(t, s, "/api/bases/recycle", `{"base_id":601}`); w.Code != 422 || !strings.Contains(w.Body.String(), "fingerprint is missing") {
		t.Fatalf("no fingerprint: %d %s", w.Code, w.Body.String())
	}
	if w := postEdit(t, s, "/api/bases/recycle", `{"base_id":601,"fingerprint":"nope"}`); w.Code != 409 {
		t.Fatalf("wrong fingerprint: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{}`, `{"base_id":"x"}`, `{"base_id":1.5}`, `{"base_id":0,"fingerprint":"x"}`} {
		if w := postEdit(t, s, "/api/bases/recycle", body); w.Code != 422 {
			t.Errorf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
	ok := postEdit(t, s, "/api/bases/recycle", `{"base_id":601,"fingerprint":"`+pv["fingerprint"].(string)+`"}`)
	m := decodeBody(t, ok)
	if ok.Code != 200 || m["outcome"] != "written" || m["refundStacks"] != float64(2) {
		t.Fatalf("write: %d %s", ok.Code, ok.Body.String())
	}
	chk, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, _ := chk.One(`select (select count(*) from actors where id in (601,602,701)) actors, (select count(*) from totems where id=601) totem, (select count(*) from base_backups) backups,
		(select count(*) from items where inventory_id=12) reserve, (select inventory_id from items where id=9001) moved, (select count(*) from actors where id in (801,802)) theirs`)
	if r["actors"] != int64(0) || r["totem"] != int64(0) || r["backups"] != int64(0) || r["reserve"] != int64(3) || r["moved"] != int64(12) || r["theirs"] != int64(2) {
		t.Fatalf("written rows: %v", r)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup of the save is taken before the write")
	}
}

func TestRecycleRefusesAnotherPlayersBaseAndWritesNothing(t *testing.T) {
	s := recycleServer(t)
	before, _ := os.ReadFile(s.Save.Path)
	if w := kitGet(t, s, "/api/bases/recycle-preview?base=801"); w.Code != 409 || !strings.Contains(w.Body.String(), "another player") {
		t.Fatalf("preview of another player's base: %d %s", w.Code, w.Body.String())
	}
	if w := postEdit(t, s, "/api/bases/recycle", `{"base_id":801,"fingerprint":"x"}`); w.Code != 409 {
		t.Fatalf("write: %d %s", w.Code, w.Body.String())
	}
	if w := kitGet(t, s, "/api/bases/recycle-preview?base=abc"); w.Code != 422 {
		t.Fatalf("bad id: %d", w.Code)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Pending()) != 0 {
		t.Fatal("a refusal wrote or queued something")
	}
}

func TestRecycleWhileAGameProcessExistsIsQueuedAsOneEditAndWritesNothing(t *testing.T) {
	s := recycleServer(t)
	pv := decodeBody(t, kitGet(t, s, "/api/bases/recycle-preview?base=601"))
	before, _ := os.ReadFile(s.Save.Path)
	restore := save.ActiveSessionForTests()
	defer restore()
	w := postEdit(t, s, "/api/bases/recycle", `{"base_id":601,"fingerprint":"`+pv["fingerprint"].(string)+`"}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["outcome"] != "queued" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Backups()) != 0 {
		t.Fatal("the save was written while the game was running")
	}
	if p := s.Save.Pending(); len(p) != 1 || !strings.HasPrefix(p[0], `Recycle base "Home"`) {
		t.Fatalf("queued as one edit: %v", p)
	}
	// the queued edit is in the working copy, so the base is gone there; asking again queues nothing more
	if w := kitGet(t, s, "/api/bases/recycle-preview?base=601"); w.Code != 404 || len(s.Save.Pending()) != 1 {
		t.Fatalf("asking again while queued: %d %s", w.Code, w.Body.String())
	}
}
