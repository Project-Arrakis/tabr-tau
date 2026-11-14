package web

import (
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// POST /api/landsraad/faction-goal (#601), on the world of landsraad_chosen_route_test.go: the character (Harkonnen, 2) has 1,300 on house 175 (Kenola, tiers 1,200 and 6,000, the
// faction at 1,000 of 4,000); 149 is a house of the earlier term.

func TestFillFactionGoalIsOneEditThatCompletesTheHouseAtTheTopTier(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/landsraad/faction-goal", `{"task_id":175,"faction_id":2,"term_id":7}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["changed"] != float64(1) || m["tier"] != float64(6000) || m["house"] != "Kenola" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if h := houseRows(t, s.Save.Path, 175, 2); h.completed != 1 || h.winner != 2 || !h.hasProgress || h.progress != 4000 {
		t.Fatalf("complete for the faction with the progress at the goal: %+v", h)
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 6000 || got[50] != 3000 {
		t.Fatalf("the character is at the top tier, the guild's row is as it was: %v", got)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup was made before the write")
	}
	if b, err := os.ReadFile(dir + string(os.PathSeparator) + "write-log.jsonl"); err != nil || strings.Count(string(b), "\n") != 1 {
		t.Fatalf("one line in the write log: %v %s", err, b)
	}
	// again: nothing to change, nothing written
	w = postEdit(t, s, "/api/landsraad/faction-goal", `{"task_id":175,"faction_id":2,"term_id":7}`)
	if m = decodeBody(t, w); w.Code != 200 || m["noop"] != true || m["changed"] != float64(0) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestFillFactionGoalWhileTheGameRunsIsQueuedAndWritesNothing(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	before, _ := os.ReadFile(s.Save.Path)
	restore := save.ActiveSessionForTests()
	defer restore()
	w := postEdit(t, s, "/api/landsraad/faction-goal", `{"task_id":175,"faction_id":2,"term_id":7}`)
	if m := decodeBody(t, w); w.Code != 200 || m["outcome"] != "queued" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Backups()) != 0 {
		t.Fatal("the save was written while the game was running")
	}
	if p := s.Save.Pending(); len(p) != 1 || !strings.HasPrefix(p[0], "landsraad Kenola: completed for Harkonnen at the top tier") {
		t.Fatalf("queued as one edit with its words: %v", p)
	}
}

func TestFillFactionGoalRefusalsCarryTheirCodesAndWriteNothing(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	for name, c := range map[string]struct {
		body, code string
		status     int
	}{
		"no faction":         {`{"task_id":175,"term_id":7}`, "bad_argument", 422},
		"the None faction":   {`{"task_id":175,"faction_id":3,"term_id":7}`, "bad_argument", 422},
		"no term":            {`{"task_id":175,"faction_id":2}`, "bad_argument", 422},
		"another term":       {`{"task_id":175,"faction_id":2,"term_id":6}`, "conflict", 409},
		"an old house":       {`{"task_id":149,"faction_id":2,"term_id":7}`, "conflict", 409},
		"a missing house":    {`{"task_id":999,"faction_id":2,"term_id":7}`, "not_found", 404},
		"a body not in JSON": {`{`, "bad_request", 400},
	} {
		w := postEdit(t, s, "/api/landsraad/faction-goal", c.body)
		if m := decodeBody(t, w); w.Code != c.status || m["code"] != c.code || m["outcome"] != nil {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if s.Save.Dirty() {
		t.Fatal("no refusal changed anything")
	}
}

// D1-1 of the delta audit (#611): the character (Harkonnen, 2) fills house 175 for Atreides (1). The faction's progress and win are written (one edit, backup, log line); the character's
// own rows are exactly as they were (1,300 under faction 2, no row under faction 1) and the answer says the personal row was skipped.
func TestFillFactionGoalForAFactionTheCharacterIsNotInWritesTheWinAndNoPersonalRow(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/landsraad/faction-goal", `{"task_id":175,"faction_id":1,"term_id":7}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["outcome"] != "written" || m["changed"] != float64(1) || m["personal_skipped"] != true || m["contribution_changed"] != false || m["faction"] != "Atreides" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if h := houseRows(t, s.Save.Path, 175, 1); h.completed != 1 || h.winner != 1 || !h.hasProgress || h.progress != 4000 {
		t.Fatalf("won by Atreides with the progress at the goal: %+v", h)
	}
	if got := landsraadMineAmounts(t, s.Save.Path); len(got) != 2 || got[1] != 1300 || got[50] != 3000 {
		t.Fatalf("no contribution row was added or changed: %v", got)
	}
	chk, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	if n, _ := chk.One(`select count(*) n from landsraad_task_player_contributions where faction_id=1`); n["n"].(int64) != 1 { // the simulated guild's row of the fixture only
		t.Fatalf("a row of the character's under Atreides: %v", n)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup was made before the write")
	}
}

// D1-2: a read of the Landsraad page that fails is an error the page can show, not a board with nothing on it.
func TestTheLandsraadPageAnswersAFailedReadWithItsErrorNotAnEmptyBoard(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld+`drop table landsraad_task_rewards;`), config.Dir{Path: t.TempDir()})
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	w := do(t, s, "GET", "http://127.0.0.1/api/landsraad", loop, h, nil)
	m := decodeBody(t, w)
	msg, _ := m["error"].(string)
	if w.Code < 400 || m["tasks"] != nil || !strings.Contains(msg, "could not read the reward tiers") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
