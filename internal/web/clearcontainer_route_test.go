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

// POST /api/inventories/clear (#338). The test character's backpack (inventory 1) holds a Solari stack (item 10) and a Knife (item 11).

func backpackIDs(t *testing.T, path string) []int64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	rows, err := chk.Query(`select id from items where inventory_id=1 order by id`)
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	for _, r := range rows {
		out = append(out, r["id"].(int64))
	}
	return out
}

func TestClearingAContainerIsWrittenAtOnceWithABackupAndALogLine(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/inventories/clear", `{"inventory_id":1,"count":1}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["removed"] != float64(1) || m["kept"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := backpackIDs(t, s.Save.Path); len(got) != 1 || got[0] != 10 {
		t.Fatalf("the file holds the Solari stack only: %v", got)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup was made before the write")
	}
	if l := s.Save.LastFlush(); l == nil || !l.Saved || len(l.Ops) != 1 || l.Ops[0] != "Cleared container 1 (1 item)" {
		t.Fatalf("one edit with a plain description: %+v", l)
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
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != "Cleared container 1 (1 item)" || l.Backup == "" {
		t.Fatalf("one line: one edit, its words, its backup: %v %s", err, b)
	}
}

func TestClearingWithCurrencyIncludedAsksForItInTheBody(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	if w := postEdit(t, s, "/api/inventories/clear", `{"inventory_id":1,"count":2,"include_currency":true}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := backpackIDs(t, s.Save.Path); len(got) != 0 {
		t.Fatalf("the backpack is empty: %v", got)
	}
}

func TestAClearWhileTheGameHoldsTheFileIsQueuedLikeTheOtherEdits(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/inventories/clear", `{"inventory_id":1,"count":1}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["ok"] != true {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := backpackIDs(t, s.Save.Path); len(got) != 2 {
		t.Fatalf("the file is not touched while the game holds it: %v", got)
	}
	if p := s.Save.Pending(); len(p) != 1 || p[0] != "Cleared container 1 (1 item)" {
		t.Fatalf("the clear waits as one entry: %v", p)
	}
}

func TestARefusedClearCarriesItsCodeAndTheStatusThatFits(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, `insert into actors(id,class,map) values (50,'/Game/BP_LootContainer.BP_LootContainer_C','HaggaBasin');
		insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (60,50,4,5,1000.0);`), config.Dir{Path: t.TempDir()})
	for _, c := range []struct {
		name, body, code string
		status           int
	}{
		{"a count that is not what the container holds", `{"inventory_id":1,"count":5}`, "conflict", http.StatusConflict},
		{"the count that includes the currency, without the flag", `{"inventory_id":1,"count":2}`, "conflict", http.StatusConflict},
		{"a container that is not in the save", `{"inventory_id":987654,"count":0}`, "not_found", http.StatusNotFound},
		{"a container the page does not offer", `{"inventory_id":60,"count":0}`, "bad_argument", http.StatusUnprocessableEntity},
		{"no count", `{"inventory_id":1}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a currency flag that is not true or false", `{"inventory_id":1,"count":1,"include_currency":"yes"}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a body that is not JSON", `{`, "bad_request", http.StatusBadRequest},
	} {
		w := postEdit(t, s, "/api/inventories/clear", c.body)
		m := decodeBody(t, w)
		if w.Code != c.status || m["code"] != c.code || m["error"] == nil || m["outcome"] != nil {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.status, c.code)
		}
	}
	if s.Save.Dirty() || len(backpackIDs(t, s.Save.Path)) != 2 {
		t.Fatal("no refusal changed anything")
	}
}

func TestClearingAnEmptyContainerChangesNothingAndSaysSo(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, `insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (2,2,1,10,1000.0);`), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/inventories/clear", `{"inventory_id":2,"count":0}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["removed"] != float64(0) || m["message"] == nil || m["outcome"] != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() || s.Save.LastFlush() != nil || len(s.Save.Backups()) != 0 {
		t.Fatal("nothing was written and no backup was made")
	}
}

func TestTheClearRouteNeedsThePostMethodThePagesSessionAndTheSameOrigin(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	body := []byte(`{"inventory_id":1,"count":1}`)
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
	if w := do(t, s, "POST", "http://127.0.0.1/api/inventories/clear", loop, with("Cookie", ""), body); w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", w.Code)
	}
	if w := do(t, s, "POST", "http://127.0.0.1/api/inventories/clear", loop, with("Origin", "http://evil.example"), body); w.Code != http.StatusForbidden {
		t.Errorf("another origin: %d", w.Code)
	}
	if w := do(t, s, "GET", "http://127.0.0.1/api/inventories/clear", loop, good, nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("a GET: %d", w.Code)
	}
	if s.Save.Dirty() || len(backpackIDs(t, s.Save.Path)) != 2 {
		t.Fatal("none of these requests cleared anything")
	}
}

// A queue that is held (the game saved something that conflicts with the waiting edits) takes no new edit, and a clear is no exception.
func TestAClearIsRefusedWhileTheQueueIsHeld(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	s.Save.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if w := postEdit(t, s, "/api/inventories/clear", `{"inventory_id":1,"count":1}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	restore()
	other, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Mutate("the game's change", func(m *save.Mut) error {
		_, err := m.Exec(`insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (12,1,1,3,'Rifle',0,1790000000,'{}',0)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	if f := s.Save.Flush(); f.Saved || s.Save.Held() == nil {
		t.Fatalf("the queued clear conflicts with the game's newer file and is held: %+v", f)
	}
	w := postEdit(t, s, "/api/inventories/clear", `{"inventory_id":1,"count":2}`)
	if m := decodeBody(t, w); w.Code != http.StatusConflict || m["code"] != "queue_held" {
		t.Fatalf("a held queue takes no new edit: %d %s", w.Code, w.Body.String())
	}
}
