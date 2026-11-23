package web

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// #1135: "Run the automatic pass now" while the gate of an edit made by hand blocks a write used to answer in red "was not run ... Run it again once that clears", although the page
// says that changes made now are queued and every other edit by hand queues. It now queues the same way the pass at launch does (autoWaiting), says one short line, and runs by
// itself on the file as the game left it once the check clears. A pass that is queued has nobody watching when it runs, so it has the rules of the pass at launch (it waits for
// every game process, and it is written as "automatic pass", not as "run by hand"). A by-hand pass the gate lets through is unchanged.

const queuedByHandLine = "Queued: runs by itself when the save is free."

func TestByHandPassBlockedByTheGateIsQueuedNotRefused(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	defer save.GameJustLeftWorldForTests()()
	before := fileBytes(t, s)
	w := postEdit(t, s, "/api/autopass/run", `{}`)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, queuedByHandLine) || !strings.Contains(body, `"warn":false`) {
		t.Fatalf("%d %s", w.Code, body)
	}
	if strings.Contains(body, "was not run") || strings.Contains(body, "Run it again") {
		t.Fatalf("the old refusal is still said: %s", body)
	}
	if !isWaiting(s) {
		t.Fatal("a by-hand pass the gate blocked must wait, as the pass at launch does")
	}
	nothingWritten(t, s, dir, before)
	// the queue view and the card read the same wait, with the reason of the gate
	if st := postEditGet(t, s, "/api/settings"); !strings.Contains(st, `"waiting":true`) || !strings.Contains(st, `"waitingWhy":"the game wrote its database 20 seconds ago`) || !strings.Contains(st, `"waitingByHand":true`) {
		t.Fatalf("the card does not carry the wait: %s", st)
	}
}

func TestQueuedByHandPassRunsOnceTheGateClearsAndNotBefore(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	restore := save.GameJustLeftWorldForTests()
	s.RunPassNow()
	if note, warn := s.takeStartup(); warn || note != queuedByHandLine {
		t.Fatalf("note %q warn %v", note, warn)
	}
	before := fileBytes(t, s)
	s.retryWaitingPass() // the grace has not ended
	if !bytes.Equal(before, fileBytes(t, s)) || !isWaiting(s) {
		t.Fatal("the retry wrote inside the grace")
	}
	restore()
	restoreMenu := save.GameAtMenuForTests() // the game is still open, at its menu: the gate of a by-hand edit would let a write through, but nobody is watching this one
	s.retryWaitingPass()
	if after := fileBytes(t, s); !bytes.Equal(before, after) || s.Save.Dirty() || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 || !isWaiting(s) {
		t.Fatal("the queued pass wrote while a game process existed: it must have the rules of the pass at launch")
	}
	restoreMenu() // the game closes
	s.retryWaitingPass()
	if note, _ := s.takeStartup(); !strings.Contains(note, "Automatic refill saved ") {
		t.Fatalf("after the game closed the pass should run: %q", note)
	}
	if isWaiting(s) {
		t.Fatal("the pass ran, so it no longer waits")
	}
	if got := autoPassFile(t, s.Save.Path); got["water"] != int64(5000) || got["gear"] != 100.0 {
		t.Fatalf("the written file: %v", got)
	}
	lines := logLines(t, dir)
	if len(lines) != 1 || !strings.Contains(lines[0], "automatic pass") || strings.Contains(lines[0], "run by hand") {
		t.Fatalf("the queued pass is written as the unattended pass: %v", lines)
	}
	s.retryWaitingPass() // nothing waits any more: it must not run twice
	if len(logLines(t, dir)) != 1 {
		t.Fatal("the pass ran twice")
	}
}

func TestQueuedByHandPassRunsWithTheMasterSwitchOffToo(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: false})
	restore := save.GameJustLeftWorldForTests()
	s.RunPassNow() // a by-hand pass ignores the master switch, so the queued one does too
	if !isWaiting(s) {
		t.Fatal("not queued")
	}
	s.retryWaitingPass()
	if !isWaiting(s) {
		t.Fatal("the master switch is off, but the by-hand pass asked for must stay queued")
	}
	restore()
	s.retryWaitingPass()
	if note, _ := s.takeStartup(); !strings.Contains(note, "Automatic refill saved ") {
		t.Fatalf("the queued by-hand pass should run with the master switch off: %q", note)
	}
	// but the pass at launch is still off with the master switch off, and leaves no by-hand wait behind
	restore = save.GameJustLeftWorldForTests()
	defer restore()
	s.RunStartupTasks()
	if isWaiting(s) {
		t.Fatal("the master switch is off: the pass at launch must not queue")
	}
}

func TestQueuedByHandPassStopsWhenEveryJobIsSwitchedOffWhileItWaits(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: false})
	restore := save.GameJustLeftWorldForTests()
	s.RunPassNow()
	if !isWaiting(s) {
		t.Fatal("not queued")
	}
	no := false
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillWater: &no, AutoRefillPower: &no, AutoRepairVehicles: &no, AutoRepairGear: &no,
		AutoFillContainers: &no, AutoFillHydration: &no, AutoFillBloodSacks: &no, AutoRefuelVehicles: &no}); err != nil {
		t.Fatal(err)
	}
	restore()
	before := fileBytes(t, s)
	s.retryWaitingPass()
	if isWaiting(s) {
		t.Fatal("every job is off: nothing is left to wait for")
	}
	nothingWritten(t, s, dir, before)
}

func TestCancelStopsAQueuedByHandPass(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	restore := save.GameJustLeftWorldForTests()
	s.RunPassNow()
	s.takeStartup()
	if !s.cancelWaitingPass() {
		t.Fatal("Cancel did not find the queued pass")
	}
	if isWaiting(s) {
		t.Fatal("Cancel left the wait")
	}
	if note, _ := s.takeStartup(); note != "" {
		t.Fatalf("a note still promises the cancelled pass: %q", note)
	}
	before := fileBytes(t, s)
	restore() // the game closes
	s.retryWaitingPass()
	nothingWritten(t, s, dir, before)
	if note, _ := s.takeStartup(); note != "" {
		t.Fatalf("the cancelled pass ran: %q", note)
	}
	// and a cancel that lands while a retry is under way wins (#1124), for a by-hand wait too
	restore = save.GameJustLeftWorldForTests()
	s.RunPassNow()
	restore()
	s.afterReload = func() { s.cancelWaitingPass() }
	s.retryWaitingPass()
	if after, _ := os.ReadFile(s.Save.Path); !bytes.Equal(before, after) || s.Save.Dirty() || len(s.Save.Backups()) != 0 || isWaiting(s) {
		t.Fatal("Cancel during the retry did not stop the by-hand pass")
	}
}

// the card is told whether the wait began by hand (it shows the wait with the master switch off only then), and a wait of the pass at launch is not by hand
func TestCardIsToldOnlyAByHandWaitIsByHand(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	defer save.GameJustLeftWorldForTests()()
	s.RunStartupTasks()
	if st := postEditGet(t, s, "/api/settings"); !strings.Contains(st, `"waiting":true`) || !strings.Contains(st, `"waitingByHand":false`) {
		t.Fatalf("a wait of the pass at launch is not by hand: %s", st)
	}
	s.RunPassNow()
	if st := postEditGet(t, s, "/api/settings"); !strings.Contains(st, `"waitingByHand":true`) {
		t.Fatalf("a wait asked for by hand is: %s", st)
	}
	s.cancelWaitingPass()
	if st := postEditGet(t, s, "/api/settings"); !strings.Contains(st, `"waiting":false`) || !strings.Contains(st, `"waitingByHand":false`) {
		t.Fatalf("after Cancel nothing waits: %s", st)
	}
}

// the cancel of a by-hand wait must not leave the by-hand mark behind: a later launch pass with the master switch off must not run because of it
func TestCancelledByHandWaitDoesNotLetALaterLaunchPassRunWithTheMasterSwitchOff(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: false})
	restore := save.GameJustLeftWorldForTests()
	defer restore()
	s.RunPassNow()
	s.cancelWaitingPass()
	s.runPassGuarded(false, true, s.cancelGen)
	if isWaiting(s) {
		t.Fatal("a launch pass with the master switch off queued after a cancelled by-hand pass")
	}
}

// A by-hand pass the gate lets through is as it was: it runs now, is written as "run by hand", and nothing waits.
func TestUnblockedByHandPassIsUnchanged(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	defer save.GameAtMenuForTests()()
	before := fileBytes(t, s)
	s.RunPassNow()
	note, warn := s.takeStartup()
	if warn || !strings.Contains(note, "Automatic refill saved ") || strings.Contains(note, "Queued") {
		t.Fatalf("note %q warn %v", note, warn)
	}
	if isWaiting(s) {
		t.Fatal("the pass ran: nothing waits")
	}
	lines := logLines(t, dir)
	if bytes.Equal(before, fileBytes(t, s)) || len(lines) != 1 || !strings.Contains(lines[0], "run by hand: automatic pass") {
		t.Fatalf("the by-hand pass at the menu is written as run by hand: %v", lines)
	}
}
