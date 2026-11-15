package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The board's write scales a mission by the two Landsraad multipliers of ServerCustomSettings.ini, read from the folder the editor already edits (1 where the key or the file is
// not there), and refuses with the reason when a value is not a number from 0 to 10 (refs #609, #652).

func grantTotals(t *testing.T, path string) (solari, scrip, xp int64) {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	for cur, dst := range map[int64]*int64{0: &solari, 1: &scrip} {
		r, err := chk.One(`select balance b from player_virtual_currency_balances where player_controller_id=1 and currency_id=?`, cur)
		if err != nil {
			t.Fatal(err)
		}
		if r != nil {
			*dst = r["b"].(int64)
		}
	}
	r, err := chk.One(`select xp_amount x from specialization_tracks where player_id=1 and track_type=2`) // Kenola raises Gathering, track 2
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		xp = r["x"].(int64)
	}
	return
}

func grantServer(t *testing.T, ini string) *Server {
	t.Helper()
	cfg := t.TempDir()
	if ini != "" {
		if err := os.WriteFile(filepath.Join(cfg, "ServerCustomSettings.ini"), []byte(ini), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: cfg})
}

const grantFill = `{"term_id":7,"houses":[{"task_id":175,"was_mine":1300,"mine_to":6000}]}`

func TestTheBoardsWriteScalesAMissionByTheSettingsFile(t *testing.T) {
	s := grantServer(t, "[/Script/DuneSandbox.UserServerCustomSettings]\r\nLandsraadContributionMultiplier=2.000000\r\nLandsraadSpecializationXpMultiplier=2.000000\r\nGatheringAmount=1.000000\r\n")
	w := postEdit(t, s, "/api/landsraad/apply", grantFill)
	m := decodeBody(t, w)
	// the step is 2,000: the 2nd and the 3rd mission cross 1,300 to 6,000; each gives 6,000 Solari, 1,000 House Scrip and 1,500 x 2 XP
	if w.Code != 200 || m["missions"] != float64(2) || m["solari"] != float64(12000) || m["house_scrip"] != float64(2000) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if solari, scrip, xp := grantTotals(t, s.Save.Path); solari != 12000 || scrip != 2000 || xp != 6000 {
		t.Fatalf("the save: %d %d %d", solari, scrip, xp)
	}
}

func TestTheBoardsWriteUsesTheStockAmountsWhenTheFileHasNoKey(t *testing.T) {
	for name, ini := range map[string]string{"no file": "", "no key": "[/Script/DuneSandbox.UserServerCustomSettings]\nGatheringAmount=1.000000\n",
		"the key in another section": "[Other]\nLandsraadContributionMultiplier=5.000000\n"} {
		s := grantServer(t, ini)
		w := postEdit(t, s, "/api/landsraad/apply", grantFill)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", name, w.Code, w.Body.String())
		}
		// step 1,000: ceil(6000 / 1000) - ceil(1300 / 1000) = 4 missions, 1,500 XP each
		if solari, scrip, xp := grantTotals(t, s.Save.Path); solari != 24000 || scrip != 4000 || xp != 6000 {
			t.Errorf("%s: the save: %d %d %d", name, solari, scrip, xp)
		}
	}
}

func TestTheBoardsWriteRefusesAMultiplierItCannotReadAndWritesNothing(t *testing.T) {
	for _, v := range []string{"abc", "-1", "11", "NaN", "Inf", ""} {
		s := grantServer(t, "[/Script/DuneSandbox.UserServerCustomSettings]\nLandsraadSpecializationXpMultiplier="+v+"\n")
		w := postEdit(t, s, "/api/landsraad/apply", grantFill)
		if w.Code != 422 || !strings.Contains(w.Body.String(), "LandsraadSpecializationXpMultiplier") || !strings.Contains(w.Body.String(), "Nothing was changed") {
			t.Errorf("%q: %d %s", v, w.Code, w.Body.String())
		}
		if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 {
			t.Errorf("%q: the contribution changed: %v", v, got)
		}
		if solari, scrip, xp := grantTotals(t, s.Save.Path); solari != 0 || scrip != 0 || xp != 0 {
			t.Errorf("%q: something was granted: %d %d %d", v, solari, scrip, xp)
		}
	}
}

func TestTheBoardsWriteNamesEveryMultiplierItCannotUse(t *testing.T) {
	s := grantServer(t, "[/Script/DuneSandbox.UserServerCustomSettings]\nLandsraadContributionMultiplier=abc\nLandsraadSpecializationXpMultiplier=11\n")
	w := postEdit(t, s, "/api/landsraad/apply", grantFill)
	b := w.Body.String()
	if w.Code != 422 || !strings.Contains(b, "LandsraadContributionMultiplier") || !strings.Contains(b, "LandsraadSpecializationXpMultiplier") || !strings.Contains(b, "Nothing was changed") {
		t.Fatalf("both bad values: %d %s", w.Code, b)
	}
}

func TestTheBoardsWriteRefusesAContributionMultiplierOfZero(t *testing.T) {
	s := grantServer(t, "[/Script/DuneSandbox.UserServerCustomSettings]\nLandsraadContributionMultiplier=0.000000\n")
	w := postEdit(t, s, "/api/landsraad/apply", grantFill)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "LandsraadContributionMultiplier") || !strings.Contains(w.Body.String(), "is 0") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 {
		t.Fatalf("the contribution changed: %v", got)
	}
}

// The board read carries what a raise grants, so the page can say it before the click: the step, what a mission pays, the reason a raise would be refused, and for each house its
// line and the contribution its own events already cover.
func TestTheBoardReadCarriesWhatARaiseGrants(t *testing.T) {
	s := grantServer(t, "[/Script/DuneSandbox.UserServerCustomSettings]\nLandsraadContributionMultiplier=2.000000\nLandsraadSpecializationXpMultiplier=3.000000\n")
	body := postEditGet(t, s, "/api/landsraad")
	for _, want := range []string{`"step":2000`, `"solari":6000`, `"scrip":1000`, `"xp":4500`, `"max_missions":1000`, `"problem":""`, `"line":"Gathering"`, `"my_events":0`} {
		if !strings.Contains(body, want) {
			t.Errorf("the board read has no %s: %s", want, body)
		}
	}
	// a setting the editor cannot use is said in the read
	s = grantServer(t, "[/Script/DuneSandbox.UserServerCustomSettings]\nLandsraadContributionMultiplier=0\n")
	if body = postEditGet(t, s, "/api/landsraad"); !strings.Contains(body, `"problem":"The Landsraad amounts cannot be worked out: LandsraadContributionMultiplier in ServerCustomSettings.ini is 0`) {
		t.Errorf("the problem is not in the read: %s", body)
	}
}
