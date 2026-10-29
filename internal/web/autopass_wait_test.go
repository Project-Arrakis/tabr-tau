package web

import (
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #441: an automatic pass that waits for the game to close (#419) must stop waiting when its switches are turned off meanwhile. Before, retryWaitingPass decoded the whole
// save and built a new working copy at every 3-second tick (under the save's lock) because RunStartupTasks returned at the switch check without clearing autoWaiting.

func isWaiting(s *Server) bool {
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	return s.autoWaiting
}

// theGameSaved puts a different, valid save in place of the file, as the game does while it runs: the editor's copy is then out of date until it reads the file again.
func theGameSaved(t *testing.T, s *Server) {
	t.Helper()
	other := testsave.PlayerWithSQL(t, "")
	b, err := os.ReadFile(other.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Save.Path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if !s.Save.ChangedOnDisk() {
		t.Fatal("the test needs the file to differ from what the editor loaded")
	}
}

func TestSwitchesTurnedOffWhileTheAutomaticPassWaitsEndTheWaitWithoutReloadingTheSave(t *testing.T) {
	off := false
	allOff := settings.Settings{AutoRefillOnOpen: true, AutoRefillWater: &off, AutoRefillPower: &off, AutoRepairVehicles: &off, AutoRepairGear: &off,
		AutoFillContainers: &off, AutoFillHydration: &off, AutoFillBloodSacks: &off, AutoRefuelVehicles: &off}
	for name, changed := range map[string]settings.Settings{
		"master switch off": {AutoRefillOnOpen: false},
		"every job off":     allOff,
	} {
		t.Run(name, func(t *testing.T) {
			s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
			restore := save.GameJustLeftWorldForTests()
			defer restore()
			s.RunStartupTasks()
			if !isWaiting(s) {
				t.Fatal("the pass should be waiting for the game")
			}
			if err := settings.Save(s.SettingsPath, changed); err != nil { // switched off while it waits
				t.Fatal(err)
			}
			theGameSaved(t, s)
			restore() // the game closes
			for i := 0; i < 3; i++ {
				s.retryWaitingPass()
				if isWaiting(s) {
					t.Fatalf("tick %d: still waiting although nothing is switched on", i+1)
				}
			}
			if !s.Save.ChangedOnDisk() {
				t.Fatal("the save was decoded again for a pass that will not run")
			}
			if n, _ := s.takeStartup(); n != "" {
				t.Fatalf("the note that promised the pass is still there: %q", n)
			}
			if s.Save.Dirty() || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
				t.Fatal("something was written")
			}
		})
	}
}

// RunStartupTasks itself clears the wait at both of its early returns, for a caller that reaches them without the tick's check.
func TestRunStartupTasksClearsTheWaitWhenItFindsNothingSwitchedOn(t *testing.T) {
	off := false
	for name, changed := range map[string]settings.Settings{
		"master switch off": {AutoRefillOnOpen: false},
		"every job off": {AutoRefillOnOpen: true, AutoRefillWater: &off, AutoRefillPower: &off, AutoRepairVehicles: &off, AutoRepairGear: &off,
			AutoFillContainers: &off, AutoFillHydration: &off, AutoFillBloodSacks: &off, AutoRefuelVehicles: &off},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
			restore := save.GameJustLeftWorldForTests()
			defer restore()
			s.RunStartupTasks()
			if !isWaiting(s) {
				t.Fatal("the pass should be waiting for the game")
			}
			settings.Save(s.SettingsPath, changed)
			s.RunStartupTasks()
			if isWaiting(s) {
				t.Fatal("still waiting")
			}
		})
	}
}

// The control: a switch changed while the pass waits, with a job still on, keeps the wait, and the pass runs once on the fresh file when the game has closed (the jobs that are off
// do not run).
func TestAJobStillOnKeepsTheWaitingPass(t *testing.T) {
	off := false
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	restore := save.GameJustLeftWorldForTests()
	defer restore()
	s.RunStartupTasks()
	settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: true, AutoRepairVehicles: &off, AutoFillHydration: &off})
	s.retryWaitingPass() // the game is still there
	if !isWaiting(s) {
		t.Fatal("a pass with jobs left must keep waiting")
	}
	restore()
	s.retryWaitingPass()
	if isWaiting(s) {
		t.Fatal("the pass ran, so it is no longer waiting")
	}
	if note, _ := s.takeStartup(); !strings.Contains(note, "Automatic refill saved ") {
		t.Fatalf("the pass should have run: %q", note)
	}
	if got := autoPassFile(t, s.Save.Path); got["vehicle"] != 100.0 || got["hydration"] != "70.000000" || got["water"] != int64(5000) {
		t.Fatalf("only the jobs left on run: %v", got)
	}
}
