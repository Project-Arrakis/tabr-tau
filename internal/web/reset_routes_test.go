package web

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The reset buttons (#339, part of #112): each reset is a POST that carries the count the person confirmed. A count that is not what the save
// holds is a 409 "conflict" and writes nothing; no count is a 422; a reset with nothing to do answers a no-op with no write; a reset made while
// the game holds the file is queued like every other edit.

// resetCharacter is a character with two learned skills (Hypersprint 3 points, BodyShots 2) beside a Hidden module and XP.
const resetCharacter = `insert into fgl_entities(entity_id,components) values (-10, jsonb('{"FLevelComponent":[[],{"ModuleData":{` +
	`"(TagName=\"Skills.Ability.Hypersprint\")":{"SkillPointsSpent":3},"(TagName=\"Skills.Perk.BodyShots\")":{"SkillPointsSpent":2},` +
	`"(TagName=\"Skills.Ability.ControlSpace\")":{"SkillPointsSpent":2}},"TotalXPEarned":1000,"TotalSkillPoints":10,"UnspentSkillPoints":5}]}'));
	insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');`

func resetServer(t *testing.T) *Server {
	t.Helper()
	return New(testsave.PlayerWithSQL(t, resetCharacter), config.Dir{Path: t.TempDir()})
}

func TestResetSkillsRoute(t *testing.T) {
	s := resetServer(t)
	// the page's number comes from the skills read
	w := do(t, s, "GET", "http://127.0.0.1/api/player/skills", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	if m := decodeBody(t, w); w.Code != 200 || m["resetCount"] != float64(2) {
		t.Fatalf("the read says how many a reset would change: %d %s", w.Code, w.Body.String())
	}
	for _, c := range []struct {
		name, body, code string
		status           int
	}{
		{"no count", `{}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a count that is not a number", `{"expected":"two"}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a stale count", `{"expected":5}`, "conflict", http.StatusConflict},
		{"zero when there is something to reset", `{"expected":0}`, "conflict", http.StatusConflict},
	} {
		w := postEdit(t, s, "/api/player/skills/reset", c.body)
		if m := decodeBody(t, w); w.Code != c.status || m["code"] != c.code || m["outcome"] != nil {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.status, c.code)
		}
	}
	if s.Save.Dirty() {
		t.Fatal("a refused reset must not write")
	}
	w = postEdit(t, s, "/api/player/skills/reset", `{"expected":2}`)
	if m := decodeBody(t, w); w.Code != 200 || m["ok"] != true || m["changed"] != float64(2) || m["noop"] != false || m["outcome"] != "written" {
		t.Fatalf("the reset: %d %s", w.Code, w.Body.String())
	}
	// the same page, pressed again, is stale; the page after the reload shows 0 and a reset with 0 is a no-op that writes nothing
	w = postEdit(t, s, "/api/player/skills/reset", `{"expected":2}`)
	if m := decodeBody(t, w); w.Code != 409 || m["code"] != "conflict" {
		t.Fatalf("a second press of the old page: %d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/player/skills/reset", `{"expected":0}`)
	if m := decodeBody(t, w); w.Code != 200 || m["noop"] != true || m["changed"] != float64(0) || m["outcome"] != nil {
		t.Fatalf("nothing to do: %d %s", w.Code, w.Body.String())
	}
}

func TestResetSkillsIsQueuedWhileTheGameHoldsTheFile(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := resetServer(t)
	w := postEdit(t, s, "/api/player/skills/reset", `{"expected":2}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" || m["changed"] != float64(2) {
		t.Fatalf("a reset while the game runs waits in the queue like any edit: %d %s", w.Code, w.Body.String())
	}
	if p := s.Save.Pending(); len(p) != 1 || p[0] != "reset skills (2 changed)" {
		t.Fatalf("the queue lists it in words: %q", p)
	}
	restore()
}

func TestResetLevelRoute(t *testing.T) {
	s := resetServer(t)
	for _, c := range []struct {
		name, body, code string
		status           int
	}{
		{"no count", `{}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a stale count", `{"expected":999}`, "conflict", http.StatusConflict},
	} {
		w := postEdit(t, s, "/api/player/level/reset", c.body)
		if m := decodeBody(t, w); w.Code != c.status || m["code"] != c.code || m["outcome"] != nil {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.status, c.code)
		}
	}
	if s.Save.Dirty() {
		t.Fatal("a refused reset must not write")
	}
	w := postEdit(t, s, "/api/player/level/reset", `{"expected":1000}`)
	if m := decodeBody(t, w); w.Code != 200 || m["changed"] != float64(1000) || m["noop"] != false || m["outcome"] != "written" {
		t.Fatalf("the reset: %d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/player/level/reset", `{"expected":0}`)
	if m := decodeBody(t, w); w.Code != 200 || m["noop"] != true || m["outcome"] != nil {
		t.Fatalf("nothing to do: %d %s", w.Code, w.Body.String())
	}
}

func TestResetSpecTracksRoute(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, resetCharacter+`
		insert into specialization_tracks(player_id,track_type,xp_amount,level) values (1,2,3000,19.0),(1,7,900,8.0);`), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, settings.Settings{TrackNumbers: map[string]int64{"Combat": 2}}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s, "GET", "http://127.0.0.1/api/player/specs", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	if m := decodeBody(t, w); w.Code != 200 || m["resetCount"] != float64(1) {
		t.Fatalf("the read says how many rows a reset would change: %d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/player/specs/reset-all", `{"expected":2}`)
	if m := decodeBody(t, w); w.Code != 409 || m["code"] != "conflict" {
		t.Fatalf("a stale count: %d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/player/specs/reset-all", `{"expected":1}`)
	if m := decodeBody(t, w); w.Code != 200 || m["changed"] != float64(1) || m["outcome"] != "written" {
		t.Fatalf("the reset: %d %s", w.Code, w.Body.String())
	}
	if n, err := s.Save.One(`select count(*) c from specialization_tracks`); err != nil || n["c"].(int64) != 1 {
		t.Fatalf("the row of the unknown track stays: %v %v", n, err)
	}
	w = postEdit(t, s, "/api/player/specs/reset-all", `{"expected":0}`)
	if m := decodeBody(t, w); w.Code != 200 || m["noop"] != true || m["outcome"] != nil {
		t.Fatalf("nothing to do: %d %s", w.Code, w.Body.String())
	}
}

func TestResetJourneyRoute(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, `insert into journey_story_node(character_id,story_node_id,complete_condition_state,reveal_condition_state,fail_condition_state,metadata_state,has_pending_reward) values
		(1,'Quest.A',x'01',x'01',x'0c',x'0c',1),(1,'Quest.B',x'01',x'01',x'0c',x'0c',0),(1,'DA_CT_SomeContract',x'01',x'01',x'0c',x'0c',0);
		insert into tutorials(id,name) values (11,'TUT_One');
		insert into tutorial_per_player(player_id,tutorial_id,tutorial_state) values (1,11,2);`), config.Dir{Path: t.TempDir()})
	w := do(t, s, "GET", "http://127.0.0.1/api/player/journey/browse", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	if m := decodeBody(t, w); w.Code != 200 || m["resetCount"] != float64(3) || m["resetNodes"] != float64(2) || m["resetTutorials"] != float64(1) {
		t.Fatalf("the read says what a reset would change: %d %s", w.Code, w.Body.String())
	}
	for _, c := range []struct {
		name, body, code string
		status           int
	}{
		{"no count", `{}`, "bad_argument", http.StatusUnprocessableEntity},
		{"a stale count", `{"expected":2}`, "conflict", http.StatusConflict},
	} {
		w := postEdit(t, s, "/api/player/journey/reset", c.body)
		if m := decodeBody(t, w); w.Code != c.status || m["code"] != c.code || m["outcome"] != nil {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.status, c.code)
		}
	}
	if s.Save.Dirty() {
		t.Fatal("a refused reset must not write")
	}
	w = postEdit(t, s, "/api/player/journey/reset", `{"expected":3}`)
	if m := decodeBody(t, w); w.Code != 200 || m["changed"] != float64(3) || m["nodes"] != float64(2) || m["tutorials"] != float64(1) || m["outcome"] != "written" {
		t.Fatalf("the reset: %d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/player/journey/reset", `{"expected":0}`)
	if m := decodeBody(t, w); w.Code != 200 || m["noop"] != true || m["outcome"] != nil {
		t.Fatalf("nothing to do: %d %s", w.Code, w.Body.String())
	}
}
