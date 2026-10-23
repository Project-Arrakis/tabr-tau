package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Check again and Write now call this route: the game check is made at once and the answer is a sentence (#366).

func gameCheck(t *testing.T, s *Server, withToken bool) (int, save.GameCheck, string) {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	if withToken {
		h["Cookie"] = "tabr_session=" + s.token
	}
	w := do(t, s, "POST", "http://127.0.0.1:8090/api/game/check", loop, h, []byte(`{}`))
	var c save.GameCheck
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	return w.Code, c, w.Body.String()
}

func stateOf(t *testing.T, s *Server) map[string]any {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	w := do(t, s, "GET", "http://127.0.0.1:8090/api/save/state", loop, h, nil)
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGameCheckRouteSaysWhatTheCheckFoundAndTheBannerClearsWhenItWorks(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	restore := save.FailedGameCheckForTests("process snapshot: Access is denied.")
	code, c, body := gameCheck(t, s, true)
	if code != 200 || !c.Failed || !strings.HasPrefix(c.Message, "the check failed again: ") || !strings.Contains(c.Message, "Access is denied") {
		t.Fatalf("a check that fails says so with the reason: %d %s", code, body)
	}
	if st := stateOf(t, s); st["saveBlocked"] != true || st["gameCheckError"] == "" {
		t.Fatalf("while the check fails the state blocks writes: %v", st)
	}
	restore() // the next check works
	code, c, body = gameCheck(t, s, true)
	if code != 200 || c.Failed || c.Running || c.Message != "the game is not running: edits are written at once" {
		t.Fatalf("the next check that works says the game is not running: %d %s", code, body)
	}
	if st := stateOf(t, s); st["saveBlocked"] != false || st["gameCheckError"] != "" {
		t.Fatalf("the block went away without a restart: %v", st)
	}
	defer save.ActiveSessionForTests()()
	if _, c, body = gameCheck(t, s, true); !c.Running || !c.Blocked || c.Message != "the game is running: edits will queue" {
		t.Fatalf("a running game in a session queues: %s", body)
	}
}

func TestGameCheckRouteWritesWhatWaitsOnceTheCheckWorks(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	restore := save.FailedGameCheckForTests("process snapshot: boom")
	before := solariOf(t, s.Save.Path)
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if !s.Save.Dirty() || solariOf(t, s.Save.Path) != before {
		t.Fatal("with the check failing the edit waits and the file is untouched")
	}
	if code, c, body := gameCheck(t, s, true); code != 200 || !c.Failed {
		t.Fatalf("still failing: %d %s", code, body)
	}
	if !s.Save.Dirty() {
		t.Fatal("nothing was written while the check failed")
	}
	restore()
	if code, c, body := gameCheck(t, s, true); code != 200 || c.Failed {
		t.Fatalf("check works now: %d %s", code, body)
	}
	if s.Save.Dirty() || solariOf(t, s.Save.Path) != before+250 {
		t.Fatal("the check that worked also wrote the waiting edit")
	}
}

func TestGameCheckRouteNeedsThePagesTokenAndPost(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	if code, _, _ := gameCheck(t, s, false); code != http.StatusForbidden && code != http.StatusUnauthorized {
		t.Fatalf("a request without the token is refused, got %d", code)
	}
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	if w := do(t, s, "GET", "http://127.0.0.1:8090/api/game/check", loop, h, nil); w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusNotFound {
		t.Fatalf("the route does not answer a GET, got %d", w.Code)
	}
}
