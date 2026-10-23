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

// POST /api/player/blood-marks (#344). The test character's backpack (inventory 1, 5 slots) holds the Solari (item 10) and a Knife (item 11); the fixture here adds a stack of
// 420 Blood Marks (item 20, item HalloweenCoin).

const coinWorld = `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 1, 420, 2, 'HalloweenCoin', '{}');`

func coinOf(t *testing.T, path string) int64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select coalesce(sum(stack_size),0) n from items where template_id='HalloweenCoin'`)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	return r["n"].(int64)
}

func TestBloodMarksAreWrittenAtOnceWithABackupAndALogLine(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, coinWorld), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/player/blood-marks", `{"amount":500}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["before"] != float64(420) || m["after"] != float64(920) || m["given"] != float64(500) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := coinOf(t, s.Save.Path); got != 920 {
		t.Fatalf("the file holds %d", got)
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
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != "gave 500 Blood Marks" || l.Backup == "" {
		t.Fatalf("one line: one edit, its words, its backup: %v %s", err, b)
	}
}

func TestAPartialGiveOfBloodMarksIsWrittenOnceWithTheRealAmount(t *testing.T) {
	// a backpack of three slots, all used; the Blood Marks stack holds 49,900 of the 50,000 a slot takes
	s := New(testsave.PlayerWithSQL(t, `update inventories set max_item_count=3 where id=1;insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 1, 49900, 2, 'HalloweenCoin', '{}');`), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/player/blood-marks", `{"amount":500}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["outcome"] != "written" || m["requested"] != float64(500) || m["given"] != float64(100) || m["reason"] != "the backpack is full" || m["limit"] != "slots" || m["after"] != float64(50000) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := coinOf(t, s.Save.Path); got != 50000 {
		t.Fatalf("the file holds %d", got)
	}
	b, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Edits int      `json:"edits"`
		Ops   []string `json:"ops"`
	}
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != "gave 100 of 500 Blood Marks" {
		t.Fatalf("one line with the real amount: %v %s", err, b)
	}
}

func TestBloodMarksGivenWhileTheGameHoldsTheFileAreQueued(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := New(testsave.PlayerWithSQL(t, coinWorld), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/player/blood-marks", `{"amount":500}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["ok"] != true {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := coinOf(t, s.Save.Path); got != 420 {
		t.Fatalf("the file is not touched while the game holds it: %d", got)
	}
	if p := s.Save.Pending(); len(p) != 1 || p[0] != "gave 500 Blood Marks" {
		t.Fatalf("the give waits as one entry: %v", p)
	}
}

func TestARefusedBloodMarksGiveCarriesItsCodeAndTheStatusThatFits(t *testing.T) {
	// a backpack of three slots, all used: the Solari, the Knife and a full stack of Blood Marks
	s := New(testsave.PlayerWithSQL(t, `update inventories set max_item_count=3 where id=1;`+`insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (20, 1, 50000, 2, 'HalloweenCoin', '{}');`), config.Dir{Path: t.TempDir()})
	for _, c := range []struct {
		name, body, code string
		status           int
	}{
		{"nothing fits: the stack is full and there is no free slot", `{"amount":1}`, "no_room", http.StatusConflict},
		{"zero", `{"amount":0}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a negative amount", `{"amount":-5}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a fraction", `{"amount":1.5}`, "bad_argument", http.StatusUnprocessableEntity},
		{"more than a stack holds", `{"amount":50001}`, "bad_argument", http.StatusUnprocessableEntity},
		{"text", `{"amount":"abc"}`, "bad_argument", http.StatusUnprocessableEntity},
		{"no amount", `{}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a body that is not JSON", `{`, "bad_request", http.StatusBadRequest},
	} {
		w := postEdit(t, s, "/api/player/blood-marks", c.body)
		m := decodeBody(t, w)
		if w.Code != c.status || m["code"] != c.code || m["error"] == nil || m["outcome"] != nil {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.status, c.code)
		}
	}
	if s.Save.Dirty() || coinOf(t, s.Save.Path) != 50000 || len(s.Save.Backups()) != 0 {
		t.Fatal("no refusal changed anything or made a backup")
	}
}

func TestThePlayerAnswerCarriesTheBloodMarksTheRowShows(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, coinWorld), config.Dir{Path: t.TempDir()})
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	m := decodeBody(t, do(t, s, "GET", "http://127.0.0.1/api/player", loop, h, nil))
	if m["bloodMarks"] != float64(420) {
		t.Fatalf("%v", m["bloodMarks"])
	}
	if w := postEdit(t, s, "/api/player/blood-marks", `{"amount":80}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	m = decodeBody(t, do(t, s, "GET", "http://127.0.0.1/api/player", loop, h, nil))
	if m["bloodMarks"] != float64(500) {
		t.Fatalf("%v", m["bloodMarks"])
	}
}

func TestTheBloodMarksRouteNeedsThePostMethodThePagesSessionAndTheSameOrigin(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, coinWorld), config.Dir{Path: t.TempDir()})
	body := []byte(`{"amount":500}`)
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
	if w := do(t, s, "POST", "http://127.0.0.1/api/player/blood-marks", loop, with("Cookie", ""), body); w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", w.Code)
	}
	if w := do(t, s, "POST", "http://127.0.0.1/api/player/blood-marks", loop, with("Origin", "http://evil.example"), body); w.Code != http.StatusForbidden {
		t.Errorf("another origin: %d", w.Code)
	}
	if w := do(t, s, "GET", "http://127.0.0.1/api/player/blood-marks", loop, good, nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("a GET: %d", w.Code)
	}
	if s.Save.Dirty() || coinOf(t, s.Save.Path) != 420 {
		t.Fatal("none of these requests gave anything")
	}
}

// A queue that is held takes no new edit, and a give of Blood Marks is no exception.
func TestABloodMarksGiveIsRefusedWhileTheQueueIsHeld(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := New(testsave.PlayerWithSQL(t, coinWorld), config.Dir{Path: t.TempDir()})
	s.Save.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if w := postEdit(t, s, "/api/player/blood-marks", `{"amount":500}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	restore()
	other, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Mutate("the game's change", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=900 where id=20`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	if f := s.Save.Flush(); f.Saved || s.Save.Held() == nil {
		t.Fatalf("the queued give read a stack the game has changed: it is held: %+v", f)
	}
	w := postEdit(t, s, "/api/player/blood-marks", `{"amount":10}`)
	if m := decodeBody(t, w); w.Code != http.StatusConflict || m["code"] != "queue_held" {
		t.Fatalf("a held queue takes no new edit: %d %s", w.Code, w.Body.String())
	}
}
