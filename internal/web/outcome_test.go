package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// ARCH-15 of the 2026-10-22 audit: an edit request says whether the edit was written or queued, and a refusal carries a stable code (and a status that fits) so that the
// page never has to read the words of a message.

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("not a JSON object: %v %s", err, w.Body.String())
	}
	return m
}

func TestAnEditAnswerSaysItWasWrittenAndKeepsItsOwnFields(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/player/solari", `{"amount":250}`)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	m := decodeBody(t, w)
	if m["outcome"] != "written" {
		t.Fatalf("outcome: %s", w.Body.String())
	}
	if m["ok"] != true || m["solari"] == nil {
		t.Fatalf("the fields the route always had are still there: %s", w.Body.String())
	}
}

func TestAnEditAnswerSaysItIsQueuedWhileTheGameHoldsTheFile(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/player/solari", `{"amount":250}`)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if m := decodeBody(t, w); m["outcome"] != "queued" || m["ok"] != true || m["solari"] == nil {
		t.Fatalf("a queued edit says so: %s", w.Body.String())
	}
}

func TestARequestThatChangedNothingSaysNothingAboutAnOutcome(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/save/restored/dismiss", `{}`)
	if m := decodeBody(t, w); w.Code != 200 || m["ok"] != true || m["outcome"] != nil {
		t.Fatalf("no edit, no outcome: %d %s", w.Code, w.Body.String())
	}
}

func TestRefusalsCarryAStableCodeAndTheStatusThatFits(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	for _, c := range []struct {
		name, path, body, code string
		status                 int
	}{
		{"a value of the wrong type", "/api/player/solari", `{"amount":"abc"}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a value out of range", "/api/player/teleport", `{"x":"NaN","y":0,"z":0}`, "bad_argument", http.StatusUnprocessableEntity},
		{"an item that is not in the save", "/api/items/delete", `{"id":987654}`, "not_found", http.StatusNotFound},
		{"a table that is not in the save", "/api/db/update", `{"table":"no_such_table","key":[1],"column":"x","value":"1"}`, "not_found", http.StatusNotFound},
		{"a body that is not JSON", "/api/player/solari", `{`, "bad_request", http.StatusBadRequest},
	} {
		w := postEdit(t, s, c.path, c.body)
		m := decodeBody(t, w)
		if w.Code != c.status || m["code"] != c.code || m["error"] == nil || m["outcome"] != nil {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.status, c.code)
		}
	}
}

func TestARefusalWithoutAKindKeepsTheOldStatusAndHasNoCode(t *testing.T) {
	w := httptest.NewRecorder()
	respond(w, nil, errors.New("something the editor does not classify"))
	m := decodeBody(t, w)
	if w.Code != http.StatusBadRequest || m["code"] != nil || m["error"] == nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestEveryKindMapsToItsCodeAndStatus(t *testing.T) {
	for _, c := range []struct {
		err    error
		code   string
		status int
	}{
		{save.ErrChangedOnDisk, "changed_on_disk", 409},
		{fmt.Errorf("wrapped: %w", save.ErrChangedOnDisk), "changed_on_disk", 409},
		{apierr.New(apierr.ErrConflict, "changed"), "conflict", 409},
		{apierr.New(apierr.ErrQueueHeld, "held"), "queue_held", 409},
		{apierr.New(apierr.ErrQueued, "waiting"), "queued", 409},
		{apierr.New(apierr.ErrGameBlocked, "the game is running"), "game_running", 400},
		{apierr.New(apierr.ErrReadOnly, "read-only"), "read_only", 400},
		{apierr.New(apierr.ErrBadArgument, "bad"), "bad_argument", 422},
		{apierr.New(apierr.ErrNotFound, "gone"), "not_found", 404},
		{apierr.New(apierr.ErrBadRequest, "not json"), "bad_request", 400},
	} {
		w := httptest.NewRecorder()
		respond(w, nil, c.err)
		if m := decodeBody(t, w); w.Code != c.status || m["code"] != c.code {
			t.Errorf("%v: %d %s, want %d %s", c.err, w.Code, w.Body.String(), c.status, c.code)
		}
	}
}

// Restoring a backup while edits are waiting is refused as "queued"; a held queue refuses the next edit as "queue_held"; a save the editor
// opened read-only refuses as "read_only"; a game that may be running refuses a restore as "game_running". The words of each stay as they were.
func TestTheRefusalsOfTheSaveLifecycleCarryTheirCodes(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	if _, err := s.Save.Mutate("a waiting edit", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=stack_size+1 where template_id='SolarisCoin'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	w := postEdit(t, s, "/api/save/restore", `{"name":"x.db"}`)
	if m := decodeBody(t, w); w.Code != 409 || m["code"] != "queued" {
		t.Fatalf("restore with edits waiting: %d %s", w.Code, w.Body.String())
	}

	restore := save.ActiveSessionForTests()
	defer restore()
	w = postEdit(t, s, "/api/save/restore", `{"name":"x.db"}`)
	if m := decodeBody(t, w); w.Code != 400 || m["code"] != "game_running" {
		t.Fatalf("restore while the game may be writing: %d %s", w.Code, w.Body.String())
	}

	ro := New(testsave.PlayerWithSQL(t, `create trigger sneaky after update on items begin update items set stack_size=stack_size; end;`), config.Dir{Path: t.TempDir()})
	if ro.Save.WriteBlocked() == "" {
		t.Skip("this fixture is not opened read-only")
	}
	w = postEdit(t, ro, "/api/player/solari", `{"amount":1}`)
	if m := decodeBody(t, w); w.Code != 400 || m["code"] != "read_only" {
		t.Fatalf("an edit of a read-only save: %d %s", w.Code, w.Body.String())
	}
}

// NET-13 of the 2026-10-22 audit: an edit runs the whole write inside its request, so the answer may not be cut by the two-minute write timeout of the server.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.deadlines = append(d.deadlines, t)
	return nil
}

func TestEditRequestsAskForALongerWriteWindowThanTheServersDefault(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	for _, path := range []string{"/api/player/solari", "/api/save/flush"} {
		req := httptest.NewRequest("POST", "http://127.0.0.1"+path, strings.NewReader(`{"amount":1}`))
		req.RemoteAddr = loop
		req.Host = "127.0.0.1:8090"
		req.Header.Set("Cookie", "tabr_session="+s.token)
		req.Header.Set("Content-Type", "application/json")
		rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
		start := time.Now()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != 200 || len(rec.deadlines) == 0 {
			t.Fatalf("%s: %d, deadlines %v", path, rec.Code, rec.deadlines)
		}
		if got := rec.deadlines[0].Sub(start); got < 5*time.Minute || got > 11*time.Minute {
			t.Fatalf("%s: the window is %s, longer than the server's %s default and bounded", path, got, NewHTTPServer(nil).WriteTimeout)
		}
	}
	rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest("GET", "http://127.0.0.1/api/overview", nil)
	req.RemoteAddr = loop
	req.Host = "127.0.0.1:8090"
	req.Header.Set("Cookie", "tabr_session="+s.token)
	s.Handler().ServeHTTP(rec, req)
	if len(rec.deadlines) != 0 {
		t.Fatal("a read keeps the server's timeout")
	}
}
