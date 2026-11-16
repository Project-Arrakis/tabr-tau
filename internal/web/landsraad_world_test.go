package web

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The world the Landsraad route tests share: the character (controller 1, Harkonnen) has 1,300 on House Kenola of the current term; a simulated guild has 3,000 there.
const landsraadMineWorld = `
insert or ignore into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen'), (3, 'None');
insert into player_faction(actor_id, faction_id, utc_time_faction_change) values (1, 2, 1);
insert into actors(id, class) values (50, 'LandsraadSimulatedGuild');
insert into landsraad_decree_term(term_id, start_time, end_time) values (6, 0, 100), (7, 100, 200);
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (149, 6, 0, 'DA_HouseKenola', 4000), (175, 7, 0, 'DA_HouseKenola', 4000);
insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values (149, 1200, 'SolarisCoin', 10000), (175, 1200, 'SolarisCoin', 10000), (175, 6000, 'Swatch.Kenola', 1);
insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 2, 175, 1300), (50, 1, 175, 3000);`

func landsraadMineAmounts(t *testing.T, path string) map[int64]float64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	rows, err := chk.Query(`select player_id, amount from landsraad_task_player_contributions where task_id=175`)
	if err != nil {
		t.Fatal(err)
	}
	out := map[int64]float64{}
	for _, r := range rows {
		out[r["player_id"].(int64)] = r["amount"].(float64)
	}
	return out
}

// landsraadChosenWorld adds a second house of the current term (176, Vernius, no progress row) and the faction's progress on the first (175, 1,000 of 4,000); 149 is a house of the earlier term.
const landsraadChosenWorld = landsraadMineWorld + `
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (176, 7, 1, 'DA_HouseVernius', 4000);
insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values (2, 175, 1000);`

type houseState struct {
	completed, winner, progress float64
	hasWinner, hasProgress      bool
}

func houseRows(t *testing.T, path string, id int64, faction int64) houseState {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	var h houseState
	r, _ := chk.One(`select completed, winning_faction_id w from landsraad_tasks where id=?`, id)
	if c, ok := r["completed"].(int64); ok {
		h.completed = float64(c)
	}
	if w, ok := r["w"].(int64); ok {
		h.winner, h.hasWinner = float64(w), true
	}
	if p, _ := chk.One(`select amount from landsraad_task_faction_contributions where faction_id=? and task_id=?`, faction, id); p != nil {
		h.progress, h.hasProgress = p["amount"].(float64), true
	}
	return h
}

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

// The seven routes of the older write model (#657) are gone: the board writes through /api/landsraad/apply only. A request to any of them, with the page's session and a body that
// would have been valid, finds no route and writes nothing.
func TestTheRetiredLandsraadRoutesAreGone(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	for route, body := range map[string]string{
		"progress":      `{"task_id":175,"faction_id":2,"amount":4000}`,
		"mine":          `{"task_id":175,"tier":6000}`,
		"mine-many":     `{"term_id":7,"expected":1,"items":[{"task_id":175,"tier":6000}]}`,
		"complete":      `{"task_id":175,"faction_id":2}`,
		"faction-goal":  `{"task_id":175,"faction_id":2,"term_id":7}`,
		"complete-many": `{"term_id":7,"task_ids":[175],"faction_id":2,"expected":1}`,
		"reopen-many":   `{"term_id":7,"task_ids":[175],"expected":0}`,
	} {
		if w := postEdit(t, s, "/api/landsraad/"+route, body); w.Code != 404 {
			t.Errorf("%s: %d %s, want 404", route, w.Code, w.Body.String())
		}
	}
	if s.Save.Dirty() {
		t.Fatal("a request to a retired route changed the save")
	}
	// the routes the page uses are still there: a house that is not there is refused by the handler (a 404 of its own kind, with a code), not by the router
	if w := postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":999,"reopen":true}]}`); w.Code != 404 || !strings.Contains(w.Body.String(), "not_found") {
		t.Errorf("apply: %d %s", w.Code, w.Body.String())
	}
}
