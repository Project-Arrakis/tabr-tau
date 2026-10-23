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

// POST /api/player/fill-all-liquids and GET /api/player/liquids (#349). The backpack (inventory 1) gets an empty Literjon (501) and an empty Bloodsack_01 (502); the
// vehicles are those of refuelRouteWorld (two of the character's, both below their tank).
const liquidsRouteWorld = refuelRouteWorld + `
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (501,1,1,50,'Literjon',0,1790000000,'{}',0),
  (502,1,1,51,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{}]}',1);`

func liquidsServer(t *testing.T) *Server {
	t.Helper()
	return New(testsave.PlayerWithSQL(t, liquidsRouteWorld), config.Dir{Path: t.TempDir()})
}

func itemStats(t *testing.T, path string, id int) string {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select stats from items where id=?`, id)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	return r["stats"].(string)
}

func TestFillingAllLiquidsIsOneWriteWithOneBackupAndOneLogLine(t *testing.T) {
	s := liquidsServer(t)
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/player/fill-all-liquids", `{"vehicles":true}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["vehiclesIncluded"] != true {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := fuelsInFile(t, s.Save.Path); got[601] != 100 || got[602] != 240 {
		t.Fatalf("the vehicles are full: %v", got)
	}
	if st := itemStats(t, s.Save.Path, 501); st == "{}" {
		t.Fatalf("the Literjon is filled: %s", st)
	}
	if st := itemStats(t, s.Save.Path, 502); st == `{"FFillableItemStats":[[],{}]}` {
		t.Fatalf("the sack is filled: %s", st)
	}
	if len(s.Save.Backups()) != 1 {
		t.Fatalf("one backup: %v", s.Save.Backups())
	}
	b, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl"))
	if err != nil {
		t.Fatalf("the write is recorded: %v", err)
	}
	var l struct {
		Edits int      `json:"edits"`
		Ops   []string `json:"ops"`
	}
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != "filled all liquids: 1 water container, 1 blood sack, 2 vehicles" {
		t.Fatalf("one line: %v %s", err, b)
	}
}

func TestFillingAllLiquidsWithTheBoxOffLeavesTheVehicles(t *testing.T) {
	s := liquidsServer(t)
	w := postEdit(t, s, "/api/player/fill-all-liquids", `{"vehicles":false}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "written" || m["vehiclesIncluded"] != false || m["vehicles"] != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := fuelsInFile(t, s.Save.Path); got[601] != 40 || got[602] != 10 {
		t.Fatalf("the vehicles are as they were: %v", got)
	}
}

func TestFillingAllLiquidsWhileTheGameHoldsTheFileIsQueuedAsOneEntry(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := liquidsServer(t)
	w := postEdit(t, s, "/api/player/fill-all-liquids", `{}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["ok"] != true || m["vehiclesIncluded"] != true {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := fuelsInFile(t, s.Save.Path); got[601] != 40 {
		t.Fatalf("the file is not touched while the game holds it: %v", got)
	}
	if p := s.Save.Pending(); len(p) != 1 || p[0] != "filled all liquids: 1 water container, 1 blood sack, 2 vehicles" {
		t.Fatalf("one entry: %v", p)
	}
}

func TestTheLiquidsRoutesCarryTheirCodes(t *testing.T) {
	s := liquidsServer(t)
	for _, c := range []struct {
		name, body, code string
		status           int
	}{
		{"a box that is not true or false", `{"vehicles":"yes"}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a box that is a number", `{"vehicles":1}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a body that is not JSON", `{`, "bad_request", http.StatusBadRequest},
	} {
		w := postEdit(t, s, "/api/player/fill-all-liquids", c.body)
		if m := decodeBody(t, w); w.Code != c.status || m["code"] != c.code || m["outcome"] != nil {
			t.Errorf("%s: %d %s", c.name, w.Code, w.Body.String())
		}
	}
	if s.Save.Dirty() {
		t.Fatal("no refusal changed anything")
	}
	good := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	if w := do(t, s, "GET", "http://127.0.0.1/api/player/fill-all-liquids", loop, good, nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("a GET: %d", w.Code)
	}
	bad := map[string]string{"Host": "127.0.0.1:8090", "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	if w := do(t, s, "POST", "http://127.0.0.1/api/player/fill-all-liquids", loop, bad, []byte(`{}`)); w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", w.Code)
	}
	w := do(t, s, "GET", "http://127.0.0.1/api/player/liquids", loop, good, nil)
	m := decodeBody(t, w)
	water, _ := m["water"].(map[string]any)
	veh, _ := m["vehicles"].(map[string]any)
	if w.Code != 200 || water["notFull"] != float64(1) || veh["notFull"] != float64(2) || s.Save.Dirty() {
		t.Errorf("the count: %d %s", w.Code, w.Body.String())
	}
}
