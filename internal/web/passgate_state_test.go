package web

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #819: a queue of only the automatic pass's edit is written under the unattended rule (it waits for every game process). The status the page polls and Check again answer
// with that same rule, so with the game open at its menu they say the file cannot be written, with the reason the write gives, instead of "edits are written at once".
func TestPassOnlyQueueIsShownAsBlockedWhileTheGameIsOpenAtTheMenu(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	restore := save.GameAtMenuForTests()
	defer restore()

	// nothing queued: the rule of an edit made by hand, which lets a write through at the menu
	if st := stateOf(t, s); st["saveBlocked"] != false {
		t.Fatalf("an empty queue at the menu is writable: %v", st)
	}

	// the pass's edit is refused (a game process exists) and stays queued
	if _, err := s.Save.Mutate(save.AutoPassDesc+": 1 Water Cistern", func(m *save.Mut) error {
		_, err := m.Exec(`update actors set id=id where id=(select min(id) from actors)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !s.Save.Dirty() {
		t.Fatal("the pass's edit should be queued")
	}

	st := stateOf(t, s)
	why, _ := st["blockReason"].(string)
	if st["saveBlocked"] != true || !strings.Contains(why, "until the game is closed") {
		t.Fatalf("the status must say what the write will say: %v", st)
	}
	if _, c, body := gameCheck(t, s, true); !c.Blocked || !strings.Contains(c.Message, "the game is open and edits will queue") || !strings.Contains(c.Message, "until the game is closed") {
		t.Fatalf("Check again must agree with the write: %s", body)
	}
	if f := s.Save.Flush(); f.Saved || !strings.Contains(f.Why, "until the game is closed") {
		t.Fatalf("the write is refused for the same reason: %+v", f)
	}

	// an edit made by hand joins the queue: it is a hand queue, and the page says it can be written
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if st := stateOf(t, s); st["saveBlocked"] != false {
		t.Fatalf("a queue with an edit made by hand has the rule of a hand edit: %v", st)
	}
	if s.Save.Dirty() {
		t.Fatal("the hand queue should have been written at the menu")
	}
}
