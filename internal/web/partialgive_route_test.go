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

// A give that does not fit whole gives what fits (POST /api/player/give). The test character's backpack (inventory 1) has 5 slots with 2 in use, so 3 are free; Spice-infused Fuel
// Cells stack to 500.

func fuelIn(t *testing.T, path string) int64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select coalesce(sum(stack_size),0) n from items where template_id='SpicedFuelCell'`)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	return r["n"].(int64)
}

func TestAPartialGiveIsWrittenOnceWithTheRealAmountInTheAnswerAndTheLog(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/player/give", `{"template_id":"SpicedFuelCell","quantity":2000}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["requested"] != float64(2000) || m["given"] != float64(1500) || m["reason"] != "the backpack is full" || m["limit"] != "slots" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := fuelIn(t, s.Save.Path); got != 1500 {
		t.Fatalf("the file holds %d", got)
	}
	if len(s.Save.Backups()) != 1 {
		t.Fatalf("one backup: %d", len(s.Save.Backups()))
	}
	b, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Edits int      `json:"edits"`
		Ops   []string `json:"ops"`
	}
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != "Give 1,500 of 2,000 x Spice-infused Fuel Cell (grade 0)" {
		t.Fatalf("one line with the real amount: %v %s", err, b)
	}
}

func TestAWholeGiveSaysNothingWasLeftOut(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	m := decodeBody(t, postEdit(t, s, "/api/player/give", `{"template_id":"SpicedFuelCell","quantity":300}`))
	if m["requested"] != float64(300) || m["given"] != float64(300) || m["reason"] != "" || m["limit"] != "" {
		t.Fatalf("%v", m)
	}
}

func TestAGiveThatFitsNothingIsA409WithTheReasonAndWritesNothing(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;`), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/player/give", `{"template_id":"SpicedFuelCell","quantity":10}`)
	m := decodeBody(t, w)
	if w.Code != http.StatusConflict || m["code"] != "no_room" || m["error"] == nil || m["outcome"] != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() || len(s.Save.Backups()) != 0 || fuelIn(t, s.Save.Path) != 0 {
		t.Fatal("nothing was written and no backup was made")
	}
	// a request that is wrong is still a request that is wrong
	for _, c := range []struct{ body, code string }{
		{`{"template_id":"SpicedFuelCell","quantity":0}`, "bad_argument"},
		{`{"template_id":"NoSuchThing","quantity":1}`, "bad_argument"},
	} {
		if w := postEdit(t, s, "/api/player/give", c.body); w.Code != http.StatusUnprocessableEntity || decodeBody(t, w)["code"] != c.code {
			t.Errorf("%s: %d %s", c.body, w.Code, w.Body.String())
		}
	}
}

func TestAPartialGiveWhileTheGameHoldsTheFileIsQueuedWithTheRealAmount(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/player/give", `{"template_id":"SpicedFuelCell","quantity":2000}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["given"] != float64(1500) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if p := s.Save.Pending(); len(p) != 1 || p[0] != "Give 1,500 of 2,000 x Spice-infused Fuel Cell (grade 0)" {
		t.Fatalf("%v", p)
	}
	if fuelIn(t, s.Save.Path) != 0 {
		t.Fatal("the file is not touched while the game holds it")
	}
}

func TestAPartialQueueAnswersTheSameWayAndAFullBackpackIsRefused(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	m := decodeBody(t, postEdit(t, s, "/api/player/give-items", `{"items":[{"template_id":"SpicedFuelCell","quantity":2000}]}`))
	if m["requested"] != float64(2000) || m["given"] != float64(1500) || m["reason"] != "the backpack is full" {
		t.Fatalf("%v", m)
	}
	w := postEdit(t, s, "/api/player/give-items", `{"items":[{"template_id":"SpicedFuelCell","quantity":5}]}`)
	if m := decodeBody(t, w); w.Code != http.StatusConflict || m["code"] != "no_room" {
		t.Fatalf("the backpack is full now: %d %s", w.Code, w.Body.String())
	}
}
