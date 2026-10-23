package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// POST /api/player/fill-blood-sacks and GET /api/player/blood-sacks (issue #60). The test character's backpack (inventory 1) gets three sacks:
// an empty Bloodsack_Unique_Durable_06 (21), a full Bloodsack_01 (22) and a Bloodsack_02 (23, a template whose capacity is not known).
const bloodRouteSQL = `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (21,1,1,5,'Bloodsack_Unique_Durable_06',0,1790000000,'{"FFillableItemStats":[[],{}]}',6),
  (22,1,1,6,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":3000.0,"FillableType":"Blood"}]}',1),
  (23,1,1,7,'Bloodsack_02',0,1790000000,'{"FFillableItemStats":[[],{}]}',2);`

func sackStats(t *testing.T, path string, id int) string {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select stats from items where id=?`, id)
	if err != nil || r == nil {
		t.Fatalf("item %d: %v", id, err)
	}
	return r["stats"].(string)
}

func TestFillingBloodSacksIsWrittenAtOnceWithABackupAndALogLine(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, bloodRouteSQL), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/player/fill-blood-sacks", `{}`)
	m := decodeBody(t, w)
	by, _ := m["byTemplate"].(map[string]any)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["filled"] != float64(1) || m["alreadyFull"] != float64(1) || m["unknown"] != float64(1) ||
		by["Bloodsack_Unique_Durable_06"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := sackStats(t, s.Save.Path, 21); got != `{"FFillableItemStats":[[],{"CurrentAmount":30000.0,"FillableType":"Blood"}]}` {
		t.Fatalf("the sack in the file: %s", got)
	}
	if got := sackStats(t, s.Save.Path, 23); got != `{"FFillableItemStats":[[],{}]}` {
		t.Fatalf("the unknown sack was touched: %s", got)
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
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != "Filled 1 blood sack" || l.Backup == "" {
		t.Fatalf("one line: one edit, its words, its backup: %v %s", err, b)
	}
}

func TestFillingBloodSacksWhileTheGameHoldsTheFileIsQueued(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := New(testsave.PlayerWithSQL(t, bloodRouteSQL), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/player/fill-blood-sacks", `{}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["ok"] != true || m["filled"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := sackStats(t, s.Save.Path, 21); got != `{"FFillableItemStats":[[],{}]}` {
		t.Fatalf("the file is not touched while the game holds it: %s", got)
	}
	if p := s.Save.Pending(); len(p) != 1 || p[0] != "Filled 1 blood sack" {
		t.Fatalf("the fill waits as one entry: %v", p)
	}
}

func TestFillingBloodSacksNeedsAPostWithTheSessionAndWritesNothingWhenThereIsNothingToFill(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	h := map[string]string{"Host": "127.0.0.1:8090", "Content-Type": "application/json"}
	if w := do(t, s, "POST", "http://127.0.0.1/api/player/fill-blood-sacks", loop, h, []byte(`{}`)); w.Code == 200 {
		t.Fatalf("a request without the session was served: %d", w.Code)
	}
	if w := do(t, s, "GET", "http://127.0.0.1/api/player/fill-blood-sacks", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil); w.Code == 200 {
		t.Fatalf("a write is not a GET: %d", w.Code)
	}
	w := postEdit(t, s, "/api/player/fill-blood-sacks", `{}`)
	if m := decodeBody(t, w); w.Code != 200 || m["filled"] != float64(0) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() || len(s.Save.Backups()) != 0 {
		t.Fatal("nothing to fill is not a write")
	}
}

func TestTheBloodSackCountIsAReadOnlyRoute(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, bloodRouteSQL), config.Dir{Path: t.TempDir()})
	w := do(t, s, "GET", "http://127.0.0.1/api/player/blood-sacks", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	m := decodeBody(t, w)
	if w.Code != 200 || m["total"] != float64(2) || m["notFull"] != float64(1) || m["unknown"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() {
		t.Fatal("reading the count wrote something")
	}
}
