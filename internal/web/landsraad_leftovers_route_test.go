package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Leftovers of the review of the mission grant (#753, #754, #762): the settings file as the game reads it, a file that cannot be read, a multiplier near 0, and the room of a track.

const grantSection = "[/Script/DuneSandbox.UserServerCustomSettings]\r\n"

// The game reads its settings files without regard to case: a key typed in lower case, or a section typed in another case, is the setting, not "key absent" (#753.1).
func TestTheBoardsWriteReadsTheSettingsFileWithoutRegardToCase(t *testing.T) {
	for name, ini := range map[string]string{
		"the key in lower case":     grantSection + "landsraadcontributionmultiplier=2.000000\r\nlandsraadspecializationxpmultiplier=2.000000\r\n",
		"the section in lower case": strings.ToLower(grantSection) + "LandsraadContributionMultiplier=2.000000\r\nLandsraadSpecializationXpMultiplier=2.000000\r\n",
	} {
		s := grantServer(t, ini)
		w := postEdit(t, s, "/api/landsraad/apply", grantFill)
		m := decodeBody(t, w)
		// the same as the file with the keys spelled as the game writes them (TestTheBoardsWriteScalesAMissionByTheSettingsFile): a step of 2,000, two missions
		if w.Code != 200 || m["missions"] != float64(2) || m["solari"] != float64(12000) {
			t.Fatalf("%s: %d %s", name, w.Code, w.Body.String())
		}
		if solari, scrip, xp := grantTotals(t, s.Save.Path); solari != 12000 || scrip != 2000 || xp != 6000 {
			t.Errorf("%s: the save: %d %d %d", name, solari, scrip, xp)
		}
	}
}

// A file that cannot be read is not a value to correct: the write is refused with the file's name only, no advice that does not fit and no folder of this computer (#753.2).
func TestTheBoardsWriteSaysWhenTheSettingsFileCannotBeRead(t *testing.T) {
	cfg := t.TempDir()
	if err := os.Mkdir(filepath.Join(cfg, "ServerCustomSettings.ini"), 0o700); err != nil { // a folder where the file should be: it cannot be read
		t.Fatal(err)
	}
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: cfg})
	w := postEdit(t, s, "/api/landsraad/apply", grantFill)
	b := w.Body.String()
	if w.Code != 422 || !strings.Contains(b, "ServerCustomSettings.ini could not be read, so the Landsraad amounts cannot be worked out. Nothing was changed.") {
		t.Fatalf("%d %s", w.Code, b)
	}
	if strings.Contains(b, "correct the value") || strings.Contains(b, "..") || strings.Contains(b, cfg) || strings.Contains(b, filepath.Base(cfg)) {
		t.Fatalf("the refusal gives advice that does not fit or a folder: %s", b)
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 {
		t.Fatalf("the contribution changed: %v", got)
	}
	// the board read says the same before a click
	if body := postEditGet(t, s, "/api/landsraad"); !strings.Contains(body, `"problem":"ServerCustomSettings.ini could not be read`) {
		t.Errorf("the problem is not in the read: %s", body)
	}
}

// A multiplier so small that a raise stands for over 1,000 missions is refused through the route, with nothing written (#754).
func TestTheBoardsWriteRefusesAMultiplierNearZeroThatCountsTooManyMissions(t *testing.T) {
	s := grantServer(t, grantSection+"LandsraadContributionMultiplier=0.001\r\n")
	w := postEdit(t, s, "/api/landsraad/apply", grantFill)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "more than the 1000 missions one change may grant for one house") || !strings.Contains(w.Body.String(), "Nothing was changed") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 {
		t.Fatalf("the contribution changed: %v", got)
	}
	if solari, scrip, xp := grantTotals(t, s.Save.Path); solari != 0 || scrip != 0 || xp != 0 {
		t.Fatalf("something was granted: %d %d %d", solari, scrip, xp)
	}
}

// The board read carries the experience each track still has room for, so the line before a click does not promise more than the write adds near the last level (#762.2).
func TestTheBoardReadCarriesTheRoomOfEachTrack(t *testing.T) {
	cfg := t.TempDir()
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld+`insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 2, 40000, 90);`), config.Dir{Path: cfg})
	body := postEditGet(t, s, "/api/landsraad")
	if !strings.Contains(body, `"room":{`) || !strings.Contains(body, `"Gathering":4182`) || !strings.Contains(body, `"Crafting":44182`) {
		t.Errorf("the board read has no room: %s", body)
	}
}

// The board read carries the level of each line the save holds a track row for, so the board shows Level N without the Specializations tab; a save with no track rows has no such field (#1108).
func TestTheBoardReadCarriesTheSpecializationLevels(t *testing.T) {
	cfg := t.TempDir()
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld+`insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 2, 205, 0), (1, 5, 44182, 0);`), config.Dir{Path: cfg})
	body := postEditGet(t, s, "/api/landsraad")
	if !strings.Contains(body, `"specialization_levels":{`) || !strings.Contains(body, `"Gathering":2`) || !strings.Contains(body, `"Sabotage":100`) || strings.Contains(body, `"Crafting":0`) {
		t.Errorf("the board read has no levels, or an invented one: %s", body)
	}
	bare := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	if body := postEditGet(t, bare, "/api/landsraad"); strings.Contains(body, "specialization_levels") {
		t.Errorf("a save with no track rows must carry no levels: %s", body)
	}
}
