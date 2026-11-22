package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// #1123: the automatic refill that waits for the game is a job of its own, not an edit. The status the page polls says that it waits and what for, so the queue view can list it,
// and Cancel stops the wait. Nothing is written and no setting changes.

func statusJSON(t *testing.T, s *Server) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(postEditGet(t, s, "/api/save/state")), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStatusSaysAWaitingRefillAndCancelStopsIt(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	restore := save.GameJustLeftWorldForTests()
	defer restore()
	// nothing waits yet
	if r, _ := statusJSON(t, s)["autoRefill"].(map[string]any); r == nil || r["waiting"] != false {
		t.Fatalf("no refill waits yet: %v", statusJSON(t, s)["autoRefill"])
	}
	s.RunStartupTasks()
	if !isWaiting(s) {
		t.Fatal("the pass should be waiting for the game")
	}
	m := statusJSON(t, s)
	r, _ := m["autoRefill"].(map[string]any)
	if r == nil || r["waiting"] != true || strings.TrimSpace(r["why"].(string)) == "" {
		t.Fatalf("the status must say that a refill waits and what for: %v", m["autoRefill"])
	}
	// the refill is not one of the queued edits: the queue itself is empty
	if p, _ := m["pending"].([]any); len(p) != 0 || m["dirty"] != false {
		t.Fatalf("a waiting refill is not a queued edit: pending=%v dirty=%v", m["pending"], m["dirty"])
	}
	// Cancel stops the wait, writes nothing and changes no setting
	before := settings.Load(s.SettingsPath)
	w := postEdit(t, s, "/api/autopass/cancel", `{}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"cancelled":true`) {
		t.Fatalf("cancel: %d %s", w.Code, w.Body.String())
	}
	if isWaiting(s) {
		t.Fatal("the refill still waits after Cancel")
	}
	if r, _ := statusJSON(t, s)["autoRefill"].(map[string]any); r["waiting"] != false {
		t.Fatalf("the status still says a refill waits: %v", r)
	}
	if n, _ := s.takeStartup(); n != "" {
		t.Fatalf("the note that promised the refill is still there: %q", n)
	}
	if s.Save.Dirty() || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
		t.Fatal("something was written")
	}
	if after := settings.Load(s.SettingsPath); after.AutoRefillOnOpen != before.AutoRefillOnOpen {
		t.Fatal("Cancel changed a setting")
	}
	// a second Cancel finds nothing waiting
	if w := postEdit(t, s, "/api/autopass/cancel", `{}`); !strings.Contains(w.Body.String(), `"cancelled":false`) {
		t.Fatalf("nothing was waiting: %s", w.Body.String())
	}
}
