package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// POST /api/vehicles/refuel and GET /api/vehicles/fuel (#349). The character (controller 1) owns a Sandbike (601, 40 of 100) and a Buggy (602, 10 of 240); a world
// Sandbike (603) and another owner's Buggy (604) are not the character's.
const refuelRouteWorld = `
insert into actors(id,class,map) values (601,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin'),(602,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin'),
  (603,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin'),(604,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin'),(9,'/Game/Other.Other_C','HaggaBasin');
insert into vehicles(id) values (601),(602),(603),(604);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (601,'a',2,3,0),(602,'b',2,3,0),(604,'c',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(602,1,1),(604,9,1);
insert into fgl_entities(entity_id,components) values
  (-601, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":40.0}]}')), (-602, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":10.0}]}')),
  (-603, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":1.0}]}')), (-604, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":1.0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor'),(602,-602,'Actor'),(603,-603,'Actor'),(604,-604,'Actor');`

func fuelsInFile(t *testing.T, path string) map[int64]float64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	rows, err := chk.Query(`select entity_id, json_extract(components,'$.FVehicleComponent[1].CurrentFuel') f from fgl_entities where entity_id between -604 and -601 order by entity_id`)
	if err != nil {
		t.Fatal(err)
	}
	out := map[int64]float64{}
	for _, r := range rows {
		out[-r["entity_id"].(int64)] = r["f"].(float64)
	}
	return out
}

func refuelServer(t *testing.T) *Server {
	t.Helper()
	return New(testsave.PlayerWithSQL(t, refuelRouteWorld), config.Dir{Path: t.TempDir()})
}

func TestRefuellingIsWrittenAtOnceWithABackupAndALogLine(t *testing.T) {
	s := refuelServer(t)
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/vehicles/refuel", `{}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["filled"] != float64(2) || m["total"] != float64(2) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	got := fuelsInFile(t, s.Save.Path)
	if got[601] != 100 || got[602] != 240 || got[603] != 1 || got[604] != 1 {
		t.Fatalf("the character's vehicles are full, the others are as they were: %v", got)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup was made before the write")
	}
	b, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl"))
	if err != nil {
		t.Fatalf("the write is recorded: %v", err)
	}
	var l struct {
		Edits  int      `json:"edits"`
		Ops    []string `json:"ops"`
		Backup string   `json:"backup"`
	}
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != "refuelled 2 vehicles" || l.Backup == "" {
		t.Fatalf("one line: one edit, its words, its backup: %v %s", err, b)
	}
}

func TestRefuellingWhileTheGameHoldsTheFileIsQueued(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := refuelServer(t)
	w := postEdit(t, s, "/api/vehicles/refuel", `{}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["ok"] != true || m["filled"] != float64(2) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := fuelsInFile(t, s.Save.Path); got[601] != 40 || got[602] != 10 {
		t.Fatalf("the file is not touched while the game holds it: %v", got)
	}
	if p := s.Save.Pending(); len(p) != 1 || p[0] != "refuelled 2 vehicles" {
		t.Fatalf("the refuel waits as one entry: %v", p)
	}
}

func TestRefuellingWithNothingToFillWritesNothing(t *testing.T) {
	s := refuelServer(t)
	if w := postEdit(t, s, "/api/vehicles/refuel", `{}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	backups := len(s.Save.Backups())
	w := postEdit(t, s, "/api/vehicles/refuel", ``)
	m := decodeBody(t, w)
	if w.Code != 200 || m["filled"] != float64(0) || m["alreadyFull"] != float64(2) || m["outcome"] != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if len(s.Save.Backups()) != backups {
		t.Fatal("a run that fills nothing makes no backup")
	}
}

func TestTheRefuelRoutesCarryTheirCodesAndNeedThePageSession(t *testing.T) {
	s := refuelServer(t)
	if w := postEdit(t, s, "/api/vehicles/refuel", `{`); w.Code != http.StatusBadRequest || decodeBody(t, w)["code"] != "bad_request" {
		t.Errorf("a body that is not JSON: %d %s", w.Code, w.Body.String())
	}
	good := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	with := func(k, v string) map[string]string {
		h := map[string]string{}
		for a, b := range good {
			h[a] = b
		}
		if v == "" {
			delete(h, k)
		} else {
			h[k] = v
		}
		return h
	}
	if w := do(t, s, "POST", "http://127.0.0.1/api/vehicles/refuel", loop, with("Cookie", ""), []byte(`{}`)); w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", w.Code)
	}
	if w := do(t, s, "POST", "http://127.0.0.1/api/vehicles/refuel", loop, with("Origin", "http://evil.example"), []byte(`{}`)); w.Code != http.StatusForbidden {
		t.Errorf("another origin: %d", w.Code)
	}
	if w := do(t, s, "GET", "http://127.0.0.1/api/vehicles/refuel", loop, good, nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("a GET: %d", w.Code)
	}
	if w := do(t, s, "POST", "http://127.0.0.1/api/vehicles/fuel", loop, good, []byte(`{}`)); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("the count is read only: %d", w.Code)
	}
	if s.Save.Dirty() || fuelsInFile(t, s.Save.Path)[601] != 40 {
		t.Fatal("none of these requests refuelled anything")
	}
	w := do(t, s, "GET", "http://127.0.0.1/api/vehicles/fuel", loop, good, nil)
	if m := decodeBody(t, w); w.Code != 200 || m["total"] != float64(2) || m["notFull"] != float64(2) || m["unknown"] != float64(0) {
		t.Errorf("the count: %d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() {
		t.Fatal("counting changes nothing")
	}
}

// A queue that is held takes no new edit, and a refuel is no exception.
func TestARefuelIsRefusedWhileTheQueueIsHeld(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := refuelServer(t)
	s.Save.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if w := postEdit(t, s, "/api/vehicles/refuel", `{}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	restore()
	other, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Mutate("the game's change", func(m *save.Mut) error {
		_, err := m.Exec(`update fgl_entities set components=jsonb_set(components,'$.FVehicleComponent[1].CurrentFuel',5.0) where entity_id=-601`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	if f := s.Save.Flush(); f.Saved || s.Save.Held() == nil {
		t.Fatalf("the queued refuel conflicts with the game's newer file and is held: %+v", f)
	}
	w := postEdit(t, s, "/api/vehicles/refuel", `{}`)
	if m := decodeBody(t, w); w.Code != http.StatusConflict || m["code"] != "queue_held" {
		t.Fatalf("a held queue takes no new edit: %d %s", w.Code, w.Body.String())
	}
}
