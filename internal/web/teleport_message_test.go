package web

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #501: the answer to a teleport says what happened and what to do, by the state of the game and of the write. The editor changes the save file; a running game keeps its own
// copy of the save in memory, so a move written to the file shows only when the game next loads the save.

const (
	msgClosed    = "Position saved. You will be there when you load this save."
	msgMenu      = "Position saved. Press Continue to load it."
	msgWorldQ    = "Position queued, but the running game keeps its own copy of the save: it will not move you now, and the game may overwrite it when it saves. Leave the world, wait about 30 seconds, then teleport and press Continue."
	msgWorldSave = "Position saved in the file, but the running game keeps its own copy of the save: it will not move you now, and the game may overwrite it when it saves. Leave the world, wait about 30 seconds, then teleport and press Continue."
)

func teleportAnswer(t *testing.T, s *Server) map[string]any {
	t.Helper()
	w := postEdit(t, s, "/api/player/teleport", `{"x":100,"y":200,"z":300}`)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	return decodeBody(t, w)
}

func TestTeleportSaysYouWillBeThereWhenTheGameIsClosed(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	m := teleportAnswer(t, s)
	if m["outcome"] != "written" || m["message"] != msgClosed || m["warn"] == true {
		t.Fatalf("%v", m)
	}
}

func TestTeleportAtTheMenuSaysPressContinue(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	defer save.GameAtMenuForTests()()
	m := teleportAnswer(t, s)
	if m["outcome"] != "written" || m["message"] != msgMenu || m["warn"] == true {
		t.Fatalf("%v", m)
	}
}

// A world is running: the edit waits in the queue, and the answer says the running game will not move the character and what to do. It is a warning that stays.
func TestTeleportWhileAWorldIsRunningSaysTheGameWillNotMove(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	defer save.ActiveSessionForTests()()
	m := teleportAnswer(t, s)
	if m["outcome"] != "queued" || m["message"] != msgWorldQ || m["warn"] != true {
		t.Fatalf("%v", m)
	}
}

// Queued for another reason (right after leaving the world): it says it is queued and why, as the other edits do, and not that the game will not move.
func TestTeleportQueuedByTheGameCheckSaysWhy(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	defer save.GameJustLeftWorldForTests()()
	m := teleportAnswer(t, s)
	msg, _ := m["message"].(string)
	if m["outcome"] != "queued" || m["warn"] != true || !strings.HasPrefix(msg, "Position queued: the game wrote its database ") || !strings.HasSuffix(msg, ". It is written by itself as soon as that clears; press Continue to load it then.") || strings.Contains(msg, save.QueuedUntilClears) {
		t.Fatalf("%v", m)
	}
}

// The check cannot be made at all: queued, with that reason.
func TestTeleportQueuedWhenTheGameCheckFailsSaysWhy(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	defer save.FailedGameCheckForTests("the process list is not available")()
	m := teleportAnswer(t, s)
	msg, _ := m["message"].(string)
	if m["outcome"] != "queued" || !strings.HasPrefix(msg, "Position queued: could not check whether the game is running") {
		t.Fatalf("%v", m)
	}
}

// A copy is not the game's: nothing about a running game, and it says the game will not see the file.
func TestTeleportOnACopySaysSo(t *testing.T) {
	t.Cleanup(save.RealOwnSaveCheckForTests()) // the temp save is outside the game's folder: a copy
	t.Setenv("LOCALAPPDATA", t.TempDir())
	defer save.ActiveSessionForTests()() // a running game does not matter for a file the game never opens
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	m := teleportAnswer(t, s)
	want := "Position saved to this file. This is a copy, so the game will not see it unless this file is installed as its game.db."
	if m["outcome"] != "written" || m["message"] != want || m["warn"] == true {
		t.Fatalf("%v", m)
	}
}

// The wording by state, without the machine: a world that is running and an edit that was written (it cannot happen through the gate, but the wording follows the state).
func TestTeleportMessageByState(t *testing.T) {
	for _, c := range []struct {
		name    string
		g       save.GameState
		outcome string
		own     bool
		want    string
		warn    bool
	}{
		{"closed", save.GameState{Mode: "none"}, "written", true, msgClosed, false},
		{"menu", save.GameState{Mode: "menu"}, "written", true, msgMenu, false},
		{"world, queued", save.GameState{Mode: "single_player", Blocked: true, Reason: "x" + save.QueuedUntilClears}, "queued", true, msgWorldQ, true},
		{"world, written", save.GameState{Mode: "single_player"}, "written", true, msgWorldSave, true},
		{"queued, no reason", save.GameState{Mode: "none"}, "queued", true, "Position queued: the save cannot be written yet. It is written by itself as soon as that clears; press Continue to load it then.", true},
	} {
		got, warn := teleportMessage(c.g, c.outcome, c.own)
		if got != c.want || warn != c.warn {
			t.Errorf("%s: %q warn=%v", c.name, got, warn)
		}
	}
}
