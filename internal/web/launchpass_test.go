package web

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// #460: the automatic pass at launch waits for a countdown the page shows, and Skip, the switch, or -no-auto ends it before it acts. "Run the automatic pass now" is a button.

func launchState(t *testing.T, s *Server) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(postEditGet(t, s, "/api/autopass/launch")), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func fileBytes(t *testing.T, s *Server) []byte {
	t.Helper()
	b, err := os.ReadFile(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// nothingWritten says the pass did not act: the file is as it was, no backup, no queue, no write-log line.
func nothingWritten(t *testing.T, s *Server, dir string, before []byte) {
	t.Helper()
	if !bytes.Equal(before, fileBytes(t, s)) || s.Save.Dirty() || len(s.Save.Pending()) != 0 || len(s.Save.Backups()) != 0 || len(logLines(t, dir)) != 0 {
		t.Fatal("the automatic pass wrote or queued something")
	}
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

const shortDelay = 150 * time.Millisecond

// Nothing happens at launch itself, and nothing happens until a page has asked: the clock starts with the page.
func TestLaunchPassDoesNotStartUntilThePageAsks(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(shortDelay, false)
	nothingWritten(t, s, dir, before)
	time.Sleep(3 * shortDelay) // far longer than the countdown: still nothing, nobody has looked
	nothingWritten(t, s, dir, before)
	if st := launchState(t, s); st["pending"] != true {
		t.Fatalf("the first ask must start the countdown: %v", st)
	}
	nothingWritten(t, s, dir, before)
	waitUntil(t, "the pass after the countdown", func() bool { return !bytes.Equal(before, fileBytes(t, s)) })
	if st := launchState(t, s); st["pending"] != false {
		t.Fatalf("the countdown is over: %v", st)
	}
	waitUntil(t, "the note", func() bool { n, _ := s.takeStartup(); return strings.Contains(n, "Automatic refill saved ") })
}

func TestLaunchCountdownReportsTheSecondsLeft(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	clock := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	s.ScheduleLaunchPass(LaunchDelay, false)
	st := launchState(t, s)
	if st["pending"] != true || st["secondsLeft"] != 15.0 || st["seconds"] != 15.0 {
		t.Fatalf("%v", st)
	}
	clock = clock.Add(11*time.Second + 400*time.Millisecond)
	if st = launchState(t, s); st["secondsLeft"] != 4.0 {
		t.Fatalf("%v", st)
	}
	s.SkipLaunchPass()
}

func TestSkipEndsTheCountdownAndNoPassRuns(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(shortDelay, false)
	launchState(t, s)
	out := postEdit(t, s, "/api/autopass/skip", `{}`)
	if out.Code != 200 || !strings.Contains(out.Body.String(), `"skipped":true`) || !strings.Contains(out.Body.String(), `"pending":false`) || !strings.Contains(out.Body.String(), `"started":false`) {
		t.Fatalf("%d %s", out.Code, out.Body.String())
	}
	time.Sleep(3 * shortDelay)
	nothingWritten(t, s, dir, before)
	if st := launchState(t, s); st["pending"] != false {
		t.Fatalf("%v", st)
	}
	if n, _ := s.takeStartup(); n != "" {
		t.Fatalf("a skipped pass leaves no note about a pass: %q", n)
	}
}

// Skip before any page asked (the countdown never started) also ends it.
func TestSkipBeforeTheClockStartedEndsIt(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(shortDelay, false)
	if r := s.SkipLaunchPass(); r["skipped"] != true {
		t.Fatalf("%v", r)
	}
	if st := launchState(t, s); st["pending"] != false {
		t.Fatalf("%v", st)
	}
	time.Sleep(3 * shortDelay)
	nothingWritten(t, s, dir, before)
}

// Turning the setting off while it counts down ends the countdown at once, through the route the page uses, and no pass runs.
func TestTurningTheSettingOffEndsTheCountdown(t *testing.T) {
	allOff := `{"autoRefillWater":false,"autoRefillPower":false,"autoRepairVehicles":false,"autoRepairGear":false,"autoFillContainers":false,"autoFillHydration":false,"autoFillBloodSacks":false,"autoRefuelVehicles":false}`
	for name, body := range map[string]string{"master switch": `{"autoRefillOnOpen":false}`, "every job": allOff} {
		t.Run(name, func(t *testing.T) {
			s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
			before := fileBytes(t, s)
			s.ScheduleLaunchPass(shortDelay, false)
			if st := launchState(t, s); st["pending"] != true {
				t.Fatalf("%v", st)
			}
			if w := postEdit(t, s, "/api/settings", body); w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if st := launchState(t, s); st["pending"] != false {
				t.Fatalf("the banner must end when the setting is turned off: %v", st)
			}
			time.Sleep(3 * shortDelay)
			nothingWritten(t, s, dir, before)
		})
	}
}

// The setting turned off behind the page's back (the file edited, or another window) in the last moment: the timer's end reads the switches again.
func TestTheEndOfTheCountdownReadsTheSwitchesAgain(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(shortDelay, false)
	launchState(t, s)
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: false}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * shortDelay)
	nothingWritten(t, s, dir, before)
}

func TestNoAutoSkipsTheLaunchPassAndSaysSo(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(0, true) // even with no wait at all
	if st := launchState(t, s); st["pending"] != false {
		t.Fatalf("%v", st)
	}
	time.Sleep(2 * shortDelay)
	nothingWritten(t, s, dir, before)
	note, warn := s.takeStartup()
	if !strings.Contains(note, "-no-auto") || warn {
		t.Fatalf("note %q warn %v", note, warn)
	}
	// the setting itself is not changed
	if !settings.Load(s.SettingsPath).AutoRefillOnOpen {
		t.Fatal("-no-auto must not change the setting")
	}
	// with nothing switched on the flag has nothing to skip and says nothing
	s, _ = autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: false})
	s.ScheduleLaunchPass(shortDelay, true)
	if note, _ = s.takeStartup(); note != "" {
		t.Fatalf("%q", note)
	}
}

func TestNothingWantedMeansNoCountdown(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: false})
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(shortDelay, false)
	if st := launchState(t, s); st["pending"] != false {
		t.Fatalf("%v", st)
	}
	time.Sleep(3 * shortDelay)
	nothingWritten(t, s, dir, before)
}

// A launch delay of zero runs the pass at once (the callers that have their own wait; tests).
func TestZeroDelayRunsAtOnce(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(0, false)
	if bytes.Equal(before, fileBytes(t, s)) {
		t.Fatal("the pass did not run")
	}
}

// The menu-write rule is the pass's own and is not loosened by the countdown: with a game process at the menu the pass that the countdown starts still waits and writes nothing.
func TestTheCountdownKeepsTheGameRule(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	restore := save.GameAtMenuForTests()
	defer restore()
	before := fileBytes(t, s)
	s.ScheduleLaunchPass(shortDelay, false)
	launchState(t, s)
	waitUntil(t, "the pass to be held back", func() bool { return isWaiting(s) })
	nothingWritten(t, s, dir, before)
}

// Run now: one request, no question. It runs the jobs that are switched on although the master switch for the launch is off, and a countdown that is still on does not run it a second time.
func TestRunNowRunsThePassWithTheMasterSwitchOff(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: false})
	before := fileBytes(t, s)
	w := postEdit(t, s, "/api/autopass/run", `{}`)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var r struct {
		Note string
		Warn bool
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Note, "Automatic refill saved ") || r.Warn {
		t.Fatalf("%+v", r)
	}
	if bytes.Equal(before, fileBytes(t, s)) || len(s.Save.Backups()) != 1 || len(logLines(t, dir)) != 1 {
		t.Fatal("the pass did not write once")
	}
	if settings.Load(s.SettingsPath).AutoRefillOnOpen {
		t.Fatal("running the pass by hand must not turn the setting on")
	}
	if n, _ := s.takeStartup(); n != "" {
		t.Fatalf("the note was given in the answer and must not be shown again: %q", n)
	}
}

func TestRunNowEndsTheCountdownSoThePassRunsOnce(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	s.ScheduleLaunchPass(shortDelay, false)
	launchState(t, s)
	if w := postEdit(t, s, "/api/autopass/run", `{}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	time.Sleep(3 * shortDelay)
	if len(s.Save.Backups()) != 1 || len(logLines(t, dir)) != 1 {
		t.Fatalf("the pass must run once: %d backups, %d log lines", len(s.Save.Backups()), len(logLines(t, dir)))
	}
	if st := launchState(t, s); st["pending"] != false {
		t.Fatalf("%v", st)
	}
}

// Run now with a game process: answered with the reason, nothing written, and nothing queued for later (unlike the pass at launch).
func TestRunNowWithTheGameOpenWritesNothingAndQueuesNothing(t *testing.T) {
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	restore := save.GameAtMenuForTests()
	defer restore()
	before := fileBytes(t, s)
	w := postEdit(t, s, "/api/autopass/run", `{}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "the game is open") || !strings.Contains(w.Body.String(), `"warn":true`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	nothingWritten(t, s, dir, before)
	if isWaiting(s) {
		t.Fatal("a pass asked for by hand must not wait for the game")
	}
}

func TestRunNowWithEveryJobOffSaysSo(t *testing.T) {
	no := false
	s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillWater: &no, AutoRefillPower: &no, AutoRepairVehicles: &no, AutoRepairGear: &no,
		AutoFillContainers: &no, AutoFillHydration: &no, AutoFillBloodSacks: &no, AutoRefuelVehicles: &no})
	before := fileBytes(t, s)
	w := postEdit(t, s, "/api/autopass/run", `{}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "every job of the automatic pass is switched off") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	nothingWritten(t, s, dir, before)
}

// Skip after the pass has started says so and undoes nothing.
func TestSkipAfterTheStartSaysItStarted(t *testing.T) {
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	s.ScheduleLaunchPass(0, false)
	r := s.SkipLaunchPass()
	if r["skipped"] != false || r["started"] != true {
		t.Fatalf("%v", r)
	}
}
