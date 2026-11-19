package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Unfill chosen (#932) through the route: the board's read says what an unfill would take back (the "unfill" of each house), the request carries that number as was_taken, and the
// answer says what was done. A stale number is refused as a conflict and nothing is written.

func unfillOfHouse(t *testing.T, s *Server, id float64) map[string]any {
	t.Helper()
	var read struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(postEditGet(t, s, "/api/landsraad")), &read); err != nil {
		t.Fatal(err)
	}
	for _, task := range read.Tasks {
		if task["id"] == id {
			u, ok := task["unfill"].(map[string]any)
			if !ok {
				t.Fatalf("house %v carries no unfill: %v", id, task)
			}
			return u
		}
	}
	t.Fatalf("no house %v", id)
	return nil
}

func TestUnfillThroughTheRouteTakesBackWhatTheBoardSaidAndRefusesAStaleNumber(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	s.KeepPendingEdits(t.TempDir())
	if w := postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":175,"was_mine":1300,"mine_to":6000}]}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	u := unfillOfHouse(t, s, 175)
	take := u["take"].(float64)
	if take != 1 || u["tiers"] != float64(2) {
		t.Fatalf("the board's preview of an unfill: %v", u) // the world has two tiers; the editor wrote the reward of the top one
	}
	solari, scrip, _ := grantTotals(t, s.Save.Path)

	// a stale number: the question said one reward more than the write would take back
	w := postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":175,"was_mine":6000,"was_done":true,"was_win":2,"mine_to":0,"lower":true,"was_taken":2}]}`)
	if w.Code != 409 || decodeBody(t, w)["code"] != "conflict" {
		t.Fatalf("a stale number must be a conflict: %d %s", w.Code, w.Body.String())
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 6000 {
		t.Fatalf("a refused unfill lowered the contribution: %v", got)
	}

	w = postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":175,"was_mine":6000,"was_done":true,"was_win":2,"mine_to":0,"lower":true,"was_taken":1}]}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["outcome"] != "written" || m["contribution_lowered"] != float64(1) || m["rewards_taken_back"] != take {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	chk, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	if left, err := chk.Query(`select template_id from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'`); err != nil || len(left) != 0 {
		t.Fatalf("the waiting rewards are gone: %v %v", left, err)
	}
	if rows, err := chk.Query(`select 1 from landsraad_task_player_contributions where player_id=1 and task_id=175`); err != nil || len(rows) != 0 {
		t.Fatalf("the contribution is 0, no row: %v %v", rows, err)
	}
	if h, err := chk.One(`select completed c, winning_faction_id w from landsraad_tasks where id=175`); err != nil || h["c"] != int64(1) || h["w"] != int64(2) {
		t.Fatalf("the House stays taken: %v %v", h, err)
	}
	if solari2, scrip2, _ := grantTotals(t, s.Save.Path); solari2 != solari || scrip2 != scrip {
		t.Fatalf("a wallet changed: %d %d, was %d %d", solari2, scrip2, solari, scrip)
	}
	if strings.Contains(w.Body.String(), "was_taken") {
		t.Fatalf("the answer must not echo the request: %s", w.Body.String())
	}
}
