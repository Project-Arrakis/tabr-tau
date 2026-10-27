package web

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #449: with the game open at its menu (no world loaded, the game quiet, nothing holding the file) an edit made by hand is written; right after leaving the world (the #419
// case) it stays queued, says what is being waited for, and the page state says the save is blocked for that reason.
func TestAnEditIsWrittenAtTheMenuAndQueuedRightAfterLeavingTheWorld(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})

	restore := save.GameJustLeftWorldForTests()
	before := solariOf(t, s.Save.Path)
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if !s.Save.Dirty() || solariOf(t, s.Save.Path) != before {
		t.Fatal("right after leaving the world the edit is queued and the file is untouched")
	}
	st := stateOf(t, s)
	why, _ := st["blockReason"].(string)
	if st["saveBlocked"] != true || !strings.Contains(why, "wrote its database") || strings.Contains(why, "is running") {
		t.Fatalf("the state must say what is being waited for: %v", st)
	}
	restore()

	defer save.GameAtMenuForTests()()
	if st := stateOf(t, s); st["saveBlocked"] != false || st["gameRunning"] != true {
		t.Fatalf("at the menu the save is writable (and the game is still reported open): %v", st)
	}
	if f := s.Save.Flush(); !f.Saved {
		t.Fatalf("the queued edit is written once the evidence allows: %+v", f)
	}
	if solariOf(t, s.Save.Path) != before+250 {
		t.Fatal("the written file does not hold the edit")
	}
	if w := postEdit(t, s, "/api/player/solari", `{"amount":10}`); w.Code != 200 || s.Save.Dirty() {
		t.Fatalf("a new edit at the menu is written at once: %d dirty=%v", w.Code, s.Save.Dirty())
	}
}
