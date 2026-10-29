package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The state the page polls says whether the open file is the game's own save (#498): the header, the tab title and the banner read it. A save outside the game's folder is
// not live and is written at once even with a game running; the game's own save is live and still queues.
func TestStateSaysWhetherTheSaveIsLive(t *testing.T) {
	defer save.RealOwnSaveCheckForTests()()
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)

	s := newTestServer(t) // a temp folder: not the game's
	if m := stateOf(t, s); m["live"] != false {
		t.Fatalf("a copy is not live: %v", m["live"])
	}
	defer save.ActiveSessionForTests()()
	if m := stateOf(t, s); m["saveBlocked"] != false || m["live"] != false {
		t.Fatalf("a copy is not held back by the game: %v", m)
	}

	own := filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "1", "game.db")
	if err := os.MkdirAll(filepath.Dir(own), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, b, 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := save.Open(own)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	m := stateOf(t, New(g, config.Dir{Path: t.TempDir()}))
	if m["live"] != true || m["saveBlocked"] != true {
		t.Fatalf("the game's own save is live and waits for the game: %v", m)
	}
}
