package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// POST /api/landsraad/mine (#380): the character (controller 1, Harkonnen) has 1,300 on House Kenola of the current term; a simulated guild has 3,000 there.
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

func TestFillingMyLandsraadContributionIsWrittenAtOnceWithABackupAndALogLine(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadMineWorld), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	w := postEdit(t, s, "/api/landsraad/mine", `{"task_id":175,"tier":6000}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["before"] != float64(1300) || m["after"] != float64(6000) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 6000 || got[50] != 3000 {
		t.Fatalf("the character's row is filled, the guild's is as it was: %v", got)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup was made before the write")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl")); err != nil || len(b) == 0 {
		t.Fatalf("the write is recorded: %v", err)
	}
}

func TestFillingMyLandsraadContributionRefusesAHouseOfAnEarlierTermWithAConflict(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadMineWorld), config.Dir{Path: t.TempDir()})
	w := postEdit(t, s, "/api/landsraad/mine", `{"task_id":149,"tier":1200}`)
	if w.Code != 409 {
		t.Fatalf("an earlier term's house is a conflict: %d %s", w.Code, w.Body.String())
	}
	if w := postEdit(t, s, "/api/landsraad/mine", `{"task_id":175,"tier":1500}`); w.Code != 422 {
		t.Fatalf("a tier the house does not have is a bad argument: %d %s", w.Code, w.Body.String())
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 {
		t.Fatalf("nothing was written: %v", got)
	}
}
