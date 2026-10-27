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

// POST /api/landsraad/complete-many and /api/landsraad/reopen-many (#451): the character (Harkonnen, 2) with two houses of the current term (175 Kenola with 1,000 of 4,000, 176
// Vernius with no progress row) and one of an earlier term (149).
const landsraadChosenWorld = landsraadMineWorld + `
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (176, 7, 1, 'DA_HouseVernius', 4000);
insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values (2, 175, 1000);`

type houseState struct {
	completed, winner, progress float64
	hasWinner, hasProgress      bool
}

func houseRows(t *testing.T, path string, id int64, faction int64) houseState {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	var h houseState
	r, _ := chk.One(`select completed, winning_faction_id w from landsraad_tasks where id=?`, id)
	if c, ok := r["completed"].(int64); ok {
		h.completed = float64(c)
	}
	if w, ok := r["w"].(int64); ok {
		h.winner, h.hasWinner = float64(w), true
	}
	if p, _ := chk.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, faction, id); p != nil {
		h.progress, h.hasProgress = p["amount"].(float64), true
	}
	return h
}

func TestCompletingTheChosenHousesIsOneEditWithOneBackupAndOneLogLine(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/landsraad/complete-many", `{"term_id":7,"task_ids":[175,176],"faction_id":2,"expected":2}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["changed"] != float64(2) || m["skipped"] != float64(0) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	for _, id := range []int64{175, 176} {
		if h := houseRows(t, s.Save.Path, id, 2); h.completed != 1 || h.winner != 2 || !h.hasProgress || h.progress != 4000 {
			t.Errorf("house %d: %+v", id, h)
		}
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 || got[50] != 3000 {
		t.Fatalf("the personal contributions are as they were: %v", got)
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
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Backup == "" {
		t.Fatalf("one line: one edit, its words, its backup: %v %s", err, b)
	}
	// the same houses again: nothing to do, and the answer says so
	w = postEdit(t, s, "/api/landsraad/complete-many", `{"term_id":7,"task_ids":[175,176],"faction_id":2,"expected":0}`)
	m = decodeBody(t, w)
	if w.Code != 200 || m["noop"] != true || m["skipped"] != float64(2) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestCompletingTheChosenHousesWithAStaleBoardIsAConflictAndWritesNothing(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	for name, body := range map[string]string{
		"a count that is not the set": `{"term_id":7,"task_ids":[175,176],"faction_id":2,"expected":5}`,
		"another term":                `{"term_id":6,"task_ids":[175],"faction_id":2,"expected":1}`,
		"a house of an earlier term":  `{"term_id":7,"task_ids":[175,149],"faction_id":2,"expected":2}`,
	} {
		if w := postEdit(t, s, "/api/landsraad/complete-many", body); w.Code != 409 {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if h := houseRows(t, s.Save.Path, 175, 2); h.completed != 0 || h.progress != 1000 {
		t.Fatalf("nothing was written: %+v", h)
	}
	if s.Save.Dirty() {
		t.Fatal("a refused request leaves no edit")
	}
}

func TestReopeningTheChosenHousesLeavesProgressAndContributionsAlone(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	if w := postEdit(t, s, "/api/landsraad/complete-many", `{"term_id":7,"task_ids":[175,176],"faction_id":2,"expected":2}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/landsraad/reopen-many", `{"term_id":7,"task_ids":[175,176],"expected":2}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["changed"] != float64(2) || m["skipped"] != float64(0) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	for _, id := range []int64{175, 176} {
		if h := houseRows(t, s.Save.Path, id, 2); h.completed != 0 || h.hasWinner || h.progress != 4000 {
			t.Errorf("house %d is open and its progress stays: %+v", id, h)
		}
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 || got[50] != 3000 {
		t.Fatalf("the personal contributions are as they were: %v", got)
	}
	if w := postEdit(t, s, "/api/landsraad/reopen-many", `{"term_id":7,"task_ids":[175,176],"expected":2}`); w.Code != 409 {
		t.Fatalf("both are open now, so a count of 2 is stale: %d %s", w.Code, w.Body.String())
	}
}

func TestTheChosenHousesRoutesCarryTheirCodesAndNeedThePageSession(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	for _, c := range []struct {
		route, name, body, code string
		status                  int
	}{
		{"complete-many", "no houses", `{"term_id":7,"task_ids":[],"faction_id":2,"expected":0}`, "bad_argument", 422},
		{"complete-many", "no count", `{"term_id":7,"task_ids":[175],"faction_id":2}`, "bad_argument", 422},
		{"complete-many", "no faction", `{"term_id":7,"task_ids":[175],"expected":1}`, "bad_argument", 422},
		{"complete-many", "the None faction", `{"term_id":7,"task_ids":[175],"faction_id":3,"expected":1}`, "bad_argument", 422},
		{"complete-many", "a house that is not there", `{"term_id":7,"task_ids":[999],"faction_id":2,"expected":1}`, "not_found", 404},
		{"complete-many", "a body that is not JSON", `{`, "bad_request", 400},
		{"reopen-many", "no houses", `{"term_id":7,"expected":0}`, "bad_argument", 422},
		{"reopen-many", "no count", `{"term_id":7,"task_ids":[175]}`, "bad_argument", 422},
		{"reopen-many", "a house twice", `{"term_id":7,"task_ids":[175,175],"expected":0}`, "bad_argument", 422},
	} {
		w := postEdit(t, s, "/api/landsraad/"+c.route, c.body)
		if m := decodeBody(t, w); w.Code != c.status || m["code"] != c.code || m["outcome"] != nil {
			t.Errorf("%s %s: %d %s", c.route, c.name, w.Code, w.Body.String())
		}
	}
	if s.Save.Dirty() {
		t.Fatal("no refusal changed anything")
	}
	good := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	bad := map[string]string{"Host": "127.0.0.1:8090", "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	for _, route := range []string{"complete-many", "reopen-many"} {
		if w := do(t, s, "GET", "http://127.0.0.1/api/landsraad/"+route, loop, good, nil); w.Code != 405 {
			t.Errorf("%s: a GET: %d", route, w.Code)
		}
		if w := do(t, s, "POST", "http://127.0.0.1/api/landsraad/"+route, loop, bad, []byte(`{}`)); w.Code != 403 && w.Code != 401 {
			t.Errorf("%s: no session: %d", route, w.Code)
		}
	}
}
