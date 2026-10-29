package web

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// #352: each job of the automatic pass has its own switch. The result text names what ran and what was switched off, a job that is switched off writes
// nothing, and the switches are honoured by the pass that waited for the game to close, as they are read when it runs (not when it was queued).

func offFlag() *bool { off := false; return &off }

// The text of a pass that saved something names what was done and what was switched off.
func TestAutoPassTextNamesWhatWasSwitchedOff(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true, AutoRepairVehicles: offFlag(), AutoFillHydration: offFlag()})
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if warn || !strings.Contains(note, "Automatic refill saved ") || !strings.Contains(note, "Switched off: Repair vehicles, Fill your water.") {
		t.Fatalf("note %q warn %v", note, warn)
	}
	if strings.Contains(note, "vehicle parts repaired") || strings.Contains(note, "your water filled") {
		t.Fatalf("the text claims a job that was switched off: %q", note)
	}
	if !strings.Contains(postEditGet(t, s, "/api/settings"), "Switched off: Repair vehicles, Fill your water") {
		t.Fatal("the card's last result does not name what was switched off")
	}
}

// A pass in which nothing needed doing says so, and still names what was switched off. Nothing else is said when every job is on.
func TestAutoPassNothingNeededTextNamesWhatWasSwitchedOff(t *testing.T) {
	s, _ := autoServer(t, `select 1;`, settings.Settings{AutoRefillOnOpen: true, AutoRepairGear: offFlag()})
	s.RunStartupTasks()
	note, _ := s.takeStartup()
	if !strings.Contains(note, "Switched off: Repair worn equipment and loadout.") {
		t.Fatalf("note %q", note)
	}
	s, _ = autoServer(t, `select 1;`, settings.Settings{AutoRefillOnOpen: true})
	s.RunStartupTasks()
	if note, _ = s.takeStartup(); strings.Contains(note, "Switched off") {
		t.Fatalf("every job is on, nothing was switched off: %q", note)
	}
}

// A job that is switched off writes nothing: with the only job that has work to do switched off, the file, the backups, the queue and the write log
// stay exactly as they were.
func TestSwitchedOffJobWritesNothing(t *testing.T) {
	s, dir := autoServer(t, twoVehiclesSQL, settings.Settings{AutoRefillOnOpen: true, AutoRepairVehicles: offFlag()})
	before, _ := os.ReadFile(s.Save.Path)
	s.RunStartupTasks()
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) || s.Save.Dirty() || len(s.Save.Pending()) != 0 || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
		t.Fatal("a switched-off job wrote or queued something")
	}
	// the same save with the switch on does repair: the test would otherwise pass for the wrong reason
	worn := func(path string) any {
		chk, err := save.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer chk.Close()
		r, err := chk.One(`select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') v from vehicle_modules where id=11`)
		if err != nil || r == nil {
			t.Fatalf("%v %v", r, err)
		}
		return r["v"]
	}
	on, _ := autoServer(t, twoVehiclesSQL, settings.Settings{AutoRefillOnOpen: true})
	on.RunStartupTasks()
	if worn(s.Save.Path) == worn(on.Save.Path) {
		t.Fatalf("the switch makes no difference (%v): the fixture proves nothing", worn(s.Save.Path))
	}
}

// The pass that waited for the game to close reads the switches when it runs: a job switched off while it waited does not run, the others do, and the
// gate is not weakened on the way (nothing is written or queued while the game exists, however many jobs are on or off).
func TestQueuedPassHonoursTheSwitchesAsTheyAreWhenItRuns(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before, _ := os.ReadFile(s.Save.Path)
	restore := save.GameJustLeftWorldForTests()
	s.RunStartupTasks()
	if note, warn := s.takeStartup(); !warn || !strings.Contains(note, "queued") {
		t.Fatalf("note %q warn %v", note, warn)
	}
	// the operator switches two jobs off while the game is open
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: true, AutoRepairVehicles: offFlag(), AutoFillHydration: offFlag()}); err != nil {
		t.Fatal(err)
	}
	s.retryWaitingPass()
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) || s.Save.Dirty() || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
		t.Fatal("the gate: something was written while the game still existed")
	}
	restore() // the game closes
	s.retryWaitingPass()
	note, warn := s.takeStartup()
	if warn || !strings.Contains(note, "Automatic refill saved ") || !strings.Contains(note, "Switched off: Repair vehicles, Fill your water.") {
		t.Fatalf("note %q warn %v", note, warn)
	}
	got := autoPassFile(t, s.Save.Path)
	if got["vehicle"] != 100.0 || got["hydration"] != "70.000000" {
		t.Fatalf("a job switched off while the pass waited ran: %v", got)
	}
	if got["water"] != int64(5000) || got["gear"] != 100.0 || got["container"] != 1000.0 {
		t.Fatalf("the jobs still on must run: %v", got)
	}
	if l := logLines(t, dir); len(l) != 1 || !strings.Contains(l[0], "automatic pass") {
		t.Fatalf("one write-log line expected: %v", l)
	}
}

// With the game open and every job switched off the master switch queues nothing at all.
func TestQueuedPassWithEveryJobOffStaysQuietWhileTheGameIsOpen(t *testing.T) {
	o := offFlag()
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true, AutoRefillWater: o, AutoRefillPower: o, AutoRepairVehicles: o, AutoRepairGear: o,
		AutoFillContainers: o, AutoFillHydration: o, AutoFillBloodSacks: o, AutoRefuelVehicles: o})
	defer save.GameJustLeftWorldForTests()()
	s.RunStartupTasks()
	if note, _ := s.takeStartup(); note != "" {
		t.Fatalf("note %q", note)
	}
	s.startupMu.Lock()
	waiting := s.autoWaiting
	s.startupMu.Unlock()
	if waiting {
		t.Fatal("nothing is switched on, nothing should wait")
	}
}
