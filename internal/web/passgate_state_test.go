package web

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #819: the status the page polls, Check again and the write give the same answer. Since #499 a queue of only the automatic pass's edit has the gate of an edit made by hand: in
// the grace after leaving the world all three say the file cannot be written, for the same reason; with the game open at its menu all three say it can, and it is written.
func TestPassOnlyQueueIsShownAsTheWriteWillTreatIt(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	restore := save.GameJustLeftWorldForTests()

	if _, err := s.Save.Mutate(save.AutoPassDesc+": 1 Water Cistern", func(m *save.Mut) error {
		_, err := m.Exec(`update actors set id=id where id=(select min(id) from actors)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !s.Save.Dirty() {
		t.Fatal("the pass's edit should be queued")
	}

	const waited = "the game saved 20 seconds ago"
	st := stateOf(t, s)
	why, _ := st["blockReason"].(string)
	if st["saveBlocked"] != true || !strings.Contains(why, waited) {
		restore()
		t.Fatalf("the status must say what the write will say: %v", st)
	}
	if _, c, body := gameCheck(t, s, true); !c.Blocked || !strings.Contains(c.Message, "the game is open and edits will queue") || !strings.Contains(c.Message, waited) {
		restore()
		t.Fatalf("Check again must agree with the write: %s", body)
	}
	// Check again is a POST, which tries to write what waits: the queue is still there
	if f := s.Save.Flush(); f.Saved || !strings.Contains(f.Why, waited) || !s.Save.Dirty() {
		restore()
		t.Fatalf("the write is refused for the same reason: %+v", f)
	}
	restore()

	defer save.GameAtMenuForTests()()
	if st := stateOf(t, s); st["saveBlocked"] != false || st["gameRunning"] != true {
		t.Fatalf("at the menu a pass-only queue is writable, with the game still reported open: %v", st)
	}
	if f := s.Save.Flush(); !f.Saved || s.Save.Dirty() {
		t.Fatalf("what the page says is writable must be written: %+v", f)
	}
	if _, c, body := gameCheck(t, s, true); c.Blocked || !c.Running || !strings.Contains(c.Message, "edits are written at once") {
		t.Fatalf("Check again at the menu: %s", body)
	}
}
