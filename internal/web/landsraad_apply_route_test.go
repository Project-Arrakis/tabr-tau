package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// POST /api/landsraad/apply (part 2 of the Landsraad tab rework): the staged changes of the board are one edit, with one backup and one log line, and a house that is no longer what
// the page showed refuses the whole request with a conflict.
func TestTheBoardsWriteIsOneEditAndAStaleHouseRefusesIt(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	body := `{"term_id":7,"houses":[{"task_id":175,"was_mine":1300,"mine_to":6000},{"task_id":176,"complete_for":2}]}`
	w := postEdit(t, s, "/api/landsraad/apply", body)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["changed"] != float64(2) || m["contribution_raised"] != float64(1) || m["completed"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 6000 || got[50] != 3000 {
		t.Fatalf("the character is at the top tier of house 175 and the simulated guild is as it was: %v", got)
	}
	if h := houseRows(t, s.Save.Path, 176, 2); h.completed != 1 || h.winner != 2 || h.progress != 4000 {
		t.Fatalf("house 176 is complete for Harkonnen: %+v", h)
	}
	if len(s.Save.Backups()) == 0 {
		t.Fatal("a backup was made before the write")
	}
	if _, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl")); err != nil {
		t.Fatalf("the write is recorded: %v", err)
	}
	// the same request again: the contribution is no longer 1,300 as the page showed, so nothing is written
	w = postEdit(t, s, "/api/landsraad/apply", body)
	if w.Code != 409 {
		t.Fatalf("a stale request is a conflict: %d %s", w.Code, w.Body.String())
	}
	// a lowering needs the tier-removal flag (the tiers of the test house are 1,200 and 6,000)
	w = postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":175,"was_mine":6000,"mine_to":1200}]}`)
	if w.Code != 422 {
		t.Fatalf("a lowering without the flag is refused: %d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":175,"was_mine":6000,"mine_to":1200,"lower":true}]}`)
	if w.Code != 200 {
		t.Fatalf("a tier removal: %d %s", w.Code, w.Body.String())
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1200 {
		t.Fatalf("the contribution drops to the tier below: %v", got)
	}
}
