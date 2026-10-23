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

// The route the page calls to ask for another attempt at writing the waiting edits, and the write that every edit request makes on its way out.

func flushReq(t *testing.T, s *Server, method string, withToken bool, body string) *httpResult {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Content-Type": "application/json"}
	if withToken {
		h["Cookie"] = "tabr_session=" + s.token
	}
	w := do(t, s, method, "http://127.0.0.1/api/save/flush", loop, h, []byte(body))
	return &httpResult{code: w.Code, body: w.Body.String()}
}

type httpResult struct {
	code int
	body string
}

func (r *httpResult) flush(t *testing.T) save.Flush {
	t.Helper()
	var f save.Flush
	if err := json.Unmarshal([]byte(r.body), &f); err != nil {
		t.Fatalf("not a Flush: %v %s", err, r.body)
	}
	return f
}

func TestFlushRouteWritesTheWaitingEditsAndAnswersWithWhatBecameOfThem(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	before := solariOf(t, s.Save.Path)
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// the game is still in a session: asking again changes nothing and says why. A body that asks for the check to be skipped is not looked at.
	r := flushReq(t, s, "POST", true, `{"force":true,"ignoreGame":true}`)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.body)
	}
	f := r.flush(t)
	if f.Saved || f.Queued != 1 || !strings.Contains(f.Why, "single-player session") {
		t.Fatalf("the answer says the edit waits, and why: %s", r.body)
	}
	if !s.Save.Dirty() || solariOf(t, s.Save.Path) != before {
		t.Fatal("the game-running check cannot be switched off from the page: the file is untouched")
	}
	restore() // the game was closed
	r = flushReq(t, s, "POST", true, "")
	f = r.flush(t)
	if r.code != 200 || !f.Saved || f.Queued != 0 || f.Backup == "" || len(f.Ops) != 1 {
		t.Fatalf("the waiting edit is written and the answer says so: %d %s", r.code, r.body)
	}
	if s.Save.Dirty() || solariOf(t, s.Save.Path) != before+250 {
		t.Fatal("the file holds the edit")
	}
}

func TestFlushRouteWithNothingWaitingChangesNothing(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	before := solariOf(t, s.Save.Path)
	r := flushReq(t, s, "POST", true, "")
	f := r.flush(t)
	if r.code != 200 || f.Saved || f.Queued != 0 || f.Why != "" || f.Backup != "" || s.Save.LastFlush() != nil {
		t.Fatalf("nothing was pending, so there is nothing to report: %d %s", r.code, r.body)
	}
	if solariOf(t, s.Save.Path) != before || len(s.Save.Backups()) != 0 {
		t.Fatal("no write, no backup")
	}
}

func TestFlushRouteNeedsThePostMethodAndThePagesToken(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	restore() // the file could be written now: only the token and the method stand in the way
	before := solariOf(t, s.Save.Path)
	if r := flushReq(t, s, "POST", false, ""); r.code != http.StatusForbidden && r.code != http.StatusUnauthorized {
		t.Fatalf("a request without the token is refused, got %d %s", r.code, r.body)
	}
	if r := flushReq(t, s, "GET", true, ""); r.code != http.StatusMethodNotAllowed {
		t.Fatalf("the route does not answer a GET, got %d %s", r.code, r.body)
	}
	if !s.Save.Dirty() || solariOf(t, s.Save.Path) != before {
		t.Fatal("neither request caused a write")
	}
}

// Every edit request ends with an attempt to write; one that fails does not, and one that changed nothing does not either.
func TestAnEditRequestFlushesAfterItselfWhenItChangedSomething(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	before := solariOf(t, s.Save.Path)
	if w := postEdit(t, s, "/api/items/delete", `{"id":99999}`); w.Code == 200 {
		t.Fatalf("deleting an item that is not there is an error: %s", w.Body.String())
	}
	if s.Save.LastFlush() != nil || len(s.Save.Backups()) != 0 {
		t.Fatal("a refused edit does not write anything")
	}
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if l := s.Save.LastFlush(); l == nil || !l.Saved || len(l.Ops) != 1 {
		t.Fatalf("the edit was flushed by its own request: %+v", l)
	}
	if solariOf(t, s.Save.Path) != before+250 {
		t.Fatal("the file holds the edit")
	}
}
