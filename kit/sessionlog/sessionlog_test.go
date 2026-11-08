package sessionlog

import (
	"strings"
	"testing"
	"time"
)

// Lines copied in shape from real client logs; ids and addresses are synthetic.
const (
	lnStart   = "[2026.10.08-16.22.23:461][ 37][20424]LogNet: Display: Browse (net mode 0): /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen"
	lnLeave1  = "[2026.10.08-17.10.51:686][604][20424][0]LogNet: Display: Delaying Travel (net mode Listen Server): ?closed?delayed_travel"
	lnLeave2  = "[2026.10.08-17.10.52:973][656][20424][0]LogNet: Display: Browse (net mode 2): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?closed"
	lnMP      = "[2026.10.08-17.11.08:482][110][20424][0]LogNet: Display: Browse (net mode 0): 192.0.2.7:7780/Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?flowtype=Login"
	lnCave    = "[2026.10.08-16.21.07:117][297][20424]LogNet: Display: Browse (net mode 0): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?Name=Player"
	lnHub     = "[2026.10.07-17.31.04:866]LogNet: Display: Browse (net mode 2): /Game/Dune/Maps/SocialHubs/Arrakeen/SH_Arrakeen?listen?flowtype=Travel?flowid=X"
	lnRoom    = "[2026.09.29-23.18.31:480][975][74088][0]LogNet: Display: Delaying Travel (net mode Listen Server): /Game/Dune/Maps/ChallengeRoom/Levels/StandaloneLevels/ChallengeRoom_Stillsuit_Standalone?delayed_travel"
	lnExit    = "[2026.10.08-16.13.32:262]LogWindows: Log: FPlatformMisc::RequestExit(0, <NoCallSiteInfo>)"
	lnClosed  = "[2026.10.08-16.13.40:109][  9][59396][0]Log: Log file closed"
	lnNoise   = "[2026.10.08-16.30.00:000][1][2]LogBattlegroupDirectorClientSubsystem: Verbose: nothing"
	lnSave    = "[2026.10.08-16.30.20:100][ 12][3]LogLowLevelPersistence: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	lnBOMOpen = "\xef\xbb\xbfLog: Log file open, 10/08/26 09:20:16"
)

var (
	wide   = Policy{}
	strict = Policy{Strict: true, IgnoreSaves: true}
)

func TestClassify(t *testing.T) {
	chat := "[x]LogChat: Display: Mallory: "
	cases := []struct {
		name         string
		line         string
		wide, strict Event
	}{
		{"start", lnStart, Start, Start},
		{"hub with listen", lnHub, Start, Start},
		{"character cave", lnCave, NoEvent, NoEvent},
		{"menu", lnLeave2, Menu, Menu},
		{"multiplayer login", lnMP, RemoteBrowse, RemoteBrowse},
		{"delayed travel closed", lnLeave1, DelayedClose, DelayedClose},
		{"delayed travel to a map", lnRoom, NoEvent, NoEvent},
		{"exit", lnExit, Exit, Exit},
		{"log closed", lnClosed, Exit, Exit},
		{"noise", lnNoise, NoEvent, NoEvent},
		{"the byte order mark line", lnBOMOpen, NoEvent, NoEvent},
		{"the database write", lnSave, DatabaseSaved, NoEvent},
		{"empty", "", NoEvent, NoEvent},

		// the two readings differ only where a browse marker stands somewhere other than the start of the text
		{"a browse after chat text", chat + "LogNet: Display: Browse (net mode 0): /Game/X?listen", Start, NoEvent},
		{"a browse with no category", "Browse (net mode 0): /Game/X?listen", Start, NoEvent},
		{"a browse after spaces", " LogNet: Display: Browse (net mode 0): /Game/X?listen", Start, NoEvent},
		{"a remote browse after chat text", chat + "LogNet: Display: Browse (net mode 0): 1.2.3.4:7807/Game/X", RemoteBrowse, NoEvent},
		{"a bracket group over 40 characters", "[" + strings.Repeat("a", 40) + "]LogNet: Display: Browse (net mode 0): /Game/X?listen", Start, NoEvent},
		{"a bracket group of 39 characters", "[" + strings.Repeat("a", 39) + "]LogNet: Display: Browse (net mode 0): /Game/X?listen", Start, Start},
		{"four bracket groups", "[a][b][c][d]LogNet: Display: Browse (net mode 0): /Game/X?listen", Start, Start},
		{"an unclosed bracket", "[2026.10.08LogNet: Display: Browse (net mode 0): /Game/X?listen", Start, NoEvent},

		// the end markers other than the browse line count anywhere, under both readings
		{"exit text after chat", chat + "FPlatformMisc::RequestExit(0)", Exit, Exit},
		{"log closed text after chat", chat + "Log file closed", Exit, Exit},
		{"delayed travel text after chat", chat + "Delaying Travel (net mode Listen Server): ?closed", DelayedClose, DelayedClose},

		// targets
		{"closed and listen: closed wins", "LogNet: Display: Browse (net mode 0): /Game/X?listen?closed", Menu, Menu},
		{"a remote target with ?closed", "LogNet: Display: Browse (net mode 0): 1.2.3.4:7807/Game/X?closed", RemoteBrowse, RemoteBrowse},
		{"a remote target with ?listen", "LogNet: Display: Browse (net mode 0): 1.2.3.4:7807/Game/X?listen", RemoteBrowse, RemoteBrowse},
		{"no target separator", "LogNet: Display: Browse (net mode 0", RemoteBrowse, RemoteBrowse},
		{"an empty target", "LogNet: Display: Browse (net mode 0): ", RemoteBrowse, RemoteBrowse},
		{"the target starts after the first separator only", "LogNet: Display: Browse (net mode 0): /Game/X?listen): 1.2.3.4/Game", Start, Start},
		{"a local map without a marker keeps exit text out", cave("Log file closed"), NoEvent, NoEvent},
		{"a browse line wins over a delayed travel text", "LogNet: Display: Browse (net mode 0): 1.2.3.4:7807/Game/X Delaying Travel (net mode Listen Server): ?closed", RemoteBrowse, RemoteBrowse},
		{"a browse line wins over an exit text", "LogNet: Display: Browse (net mode 0): /Game/X?listen Log file closed", Start, Start},
		{"delayed travel wins over an exit text", "Delaying Travel (net mode Listen Server): ?closed FPlatformMisc::RequestExit(", DelayedClose, DelayedClose},

		// the write line comes first under the wide reading
		{"a write line that holds a browse", "LogNet: Display: Browse (net mode 0): /Game/X?listen SaveDatabase - Database saved to disk", DatabaseSaved, Start},
		{"a write line that holds an exit", "SaveDatabase - Database saved to disk Log file closed", DatabaseSaved, Exit},
	}
	for _, c := range cases {
		if got := Classify(wide, c.line); got != c.wide {
			t.Errorf("%s: wide reading %v, want %v", c.name, got, c.wide)
		}
		if got := Classify(strict, c.line); got != c.strict {
			t.Errorf("%s: strict reading %v, want %v", c.name, got, c.strict)
		}
	}
	// the two flags are independent
	if got := Classify(Policy{Strict: true}, lnSave); got != DatabaseSaved {
		t.Errorf("strict without IgnoreSaves: %v", got)
	}
	if got := Classify(Policy{IgnoreSaves: true}, chat+"LogNet: Display: Browse (net mode 0): /Game/X?listen"); got != Start {
		t.Errorf("wide with IgnoreSaves: %v", got)
	}
}

func cave(tail string) string {
	return "LogNet: Display: Browse (net mode 0): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?Name=Player " + tail
}

func TestBody(t *testing.T) {
	for in, want := range map[string]string{
		"":             "",
		"LogNet: x":    "LogNet: x",
		"[a]LogNet: x": "LogNet: x",
		"[2026.10.08-17.57.43:218][ 14][1][0]LogNet: x": "LogNet: x",
		"[a][b][c][d][e]x":                   "x",
		"[unterminated":                      "[unterminated",
		"[" + strings.Repeat("a", 40) + "]x": "[" + strings.Repeat("a", 40) + "]x",
		"[" + strings.Repeat("a", 39) + "]x": "x",
		"[a] [b]x":                           " [b]x",
	} {
		if got := Body(in); got != want {
			t.Errorf("Body(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEventStringsAndEnds(t *testing.T) {
	want := map[Event]string{
		NoEvent:       "",
		Start:         "local ?listen (single-player start)",
		Menu:          "local ?closed (menu)",
		RemoteBrowse:  "remote browse",
		DelayedClose:  "delayed travel ?closed",
		Exit:          "game exiting",
		DatabaseSaved: "database saved",
	}
	for e, s := range want {
		if e.String() != s {
			t.Errorf("event %d: %q, want %q", e, e.String(), s)
		}
	}
	ends := map[Event]bool{Menu: true, RemoteBrowse: true, DelayedClose: true, Exit: true}
	for e := range want {
		if e.Ends() != ends[e] {
			t.Errorf("event %d: Ends() = %v", e, e.Ends())
		}
	}
	if Event(200).String() != "" || Event(200).Ends() {
		t.Error("an unknown event is nothing")
	}
}

func TestStateTransitions(t *testing.T) {
	cases := []struct {
		name   string
		events []Event
		want   State
	}{
		{"nothing", nil, State{}},
		{"start", []Event{Start}, State{SPActive: true, Last: Start}},
		{"menu ends", []Event{Start, Menu}, State{Last: Menu}},
		{"menu with no session", []Event{Menu}, State{Last: Menu}},
		{"remote", []Event{Start, RemoteBrowse}, State{Remote: true, Last: RemoteBrowse}},
		{"start clears remote", []Event{RemoteBrowse, Start}, State{SPActive: true, Last: Start}},
		{"menu clears remote", []Event{RemoteBrowse, Menu}, State{Last: Menu}},
		{"delayed close ends and keeps remote", []Event{Start, DelayedClose}, State{Last: DelayedClose}},
		{"delayed close does not clear remote", []Event{RemoteBrowse, DelayedClose}, State{Remote: true, Last: DelayedClose}},
		{"exit ends and marks the end", []Event{Start, Exit}, State{Ended: true, Last: Exit}},
		{"exit with no session", []Event{Exit}, State{Ended: true, Last: Exit}},
		{"the end mark stays after a start", []Event{Start, Exit, Start}, State{SPActive: true, Ended: true, Last: Start}},
		{"no event changes nothing", []Event{Start, NoEvent}, State{SPActive: true, Last: Start}},
		{"a database write changes nothing", []Event{Start, DatabaseSaved}, State{SPActive: true, Last: Start}},
	}
	for _, c := range cases {
		var s State
		for _, e := range c.events {
			s.Apply(e)
		}
		if s != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, s, c.want)
		}
	}
}

func run(p Policy, lines ...string) Tracker {
	var t Tracker
	for _, l := range lines {
		t.Feed(p, l, nil)
	}
	return t
}

func TestTrackerSessions(t *testing.T) {
	cases := []struct {
		name   string
		lines  []string
		active bool
	}{
		{"nothing yet", []string{lnBOMOpen, lnNoise}, false},
		{"character cave at start is not single-player", []string{lnCave}, false},
		{"started", []string{lnCave, lnStart, lnNoise}, true},
		{"left to the menu", []string{lnStart, lnLeave1, lnLeave2}, false},
		{"left, then multiplayer login", []string{lnStart, lnLeave2, lnMP}, false},
		{"multiplayer login alone", []string{lnMP}, false},
		{"social hub stays in the session", []string{lnStart, lnHub}, true},
		{"a challenge room stays in the session", []string{lnStart, lnRoom}, true},
		{"quitting the game", []string{lnStart, lnExit}, false},
		{"log closed", []string{lnStart, lnClosed}, false},
		{"back in after a menu visit", []string{lnStart, lnLeave2, lnStart}, true},
	}
	for _, c := range cases {
		for name, p := range map[string]Policy{"wide": wide, "strict": strict} {
			if got := run(p, c.lines...).SPActive; got != c.active {
				t.Errorf("%s (%s): active=%v want %v", c.name, name, got, c.active)
			}
		}
	}
}

func TestTrackerTimes(t *testing.T) {
	at := func(h, m, s int) time.Time { return time.Date(2026, 10, 8, h, m, s, 0, time.UTC) }
	tr := run(wide, lnStart, lnLeave1)
	if want := at(17, 10, 51); !tr.EndedAt.Equal(want) {
		t.Fatalf("end time %v want %v", tr.EndedAt, want)
	}
	if !run(wide, lnStart).EndedAt.IsZero() {
		t.Fatal("an active session has no end time")
	}
	// an end marker with no session open changes nothing, and records no end time
	if tr := run(wide, lnLeave2, lnExit, lnClosed); tr.SPActive || !tr.EndedAt.IsZero() {
		t.Fatalf("an end with no session open must not record an end: %+v", tr)
	}
	// starting a session clears the earlier end time
	if tr := run(wide, lnStart, lnLeave2, lnStart); !tr.SPActive || !tr.EndedAt.IsZero() {
		t.Fatalf("a new session has no end time: %+v", tr)
	}
	// a repeated end marker keeps the first end time
	if tr := run(wide, lnStart, lnLeave1, lnLeave2); !tr.EndedAt.Equal(at(17, 10, 51)) {
		t.Fatalf("the first end time stands, got %v", tr.EndedAt)
	}
	// a local map without ?listen changes nothing, also while a session is open
	if tr := run(wide, lnStart, lnCave); !tr.SPActive || !tr.EndedAt.IsZero() {
		t.Fatalf("a local map must not change the session: %+v", tr)
	}
	// the last write of the database
	tr = run(wide, lnStart, lnSave, lnNoise)
	if !tr.LastSaveAt.Equal(at(16, 30, 20)) || !tr.SPActive {
		t.Fatalf("last save %v, %+v", tr.LastSaveAt, tr)
	}
	if tr := run(strict, lnStart, lnSave); !tr.LastSaveAt.IsZero() {
		t.Fatalf("a program that leaves the write line out has no write time: %v", tr.LastSaveAt)
	}
	// the Remote, Ended and Last facts are kept beside the times
	if tr := run(wide, lnStart, lnMP); !tr.Remote || tr.Last != RemoteBrowse || tr.EndedAt.IsZero() {
		t.Fatalf("%+v", tr)
	}
}

func TestTrackerStampsALineWithoutATimeFromTheClock(t *testing.T) {
	fixed := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := func() time.Time { return fixed }
	noStamp := "LogNet: Display: Browse (net mode 2): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?closed"
	var tr Tracker
	tr.Feed(wide, lnStart, clock)
	tr.Feed(wide, noStamp, clock)
	if tr.SPActive || !tr.EndedAt.Equal(fixed) {
		t.Fatalf("no stamp: the end counts as now, got %+v", tr)
	}
	tr.Feed(wide, "LogLowLevelPersistence: DB(game.db:0): SaveDatabase - Database saved to disk", clock)
	if !tr.LastSaveAt.Equal(fixed) {
		t.Fatalf("no stamp: the write counts as now, got %v", tr.LastSaveAt)
	}
	// with no clock given it is time.Now
	before := time.Now()
	tr = run(wide, lnStart, noStamp)
	if tr.SPActive || tr.EndedAt.Before(before.Add(-time.Second)) || tr.EndedAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("no stamp and no clock: the end counts as now, got %+v", tr)
	}
	// a stamped line never asks the clock
	called := false
	tr = Tracker{}
	tr.Feed(wide, lnStart, func() time.Time { called = true; return fixed })
	tr.Feed(wide, lnLeave1, func() time.Time { called = true; return fixed })
	if called {
		t.Fatal("the clock was asked for a line that has a time")
	}
	// "?closed" only means the end when the line is a browse line, and a delayed travel without ?closed (a challenge room) is no end
	if tr := run(wide, lnStart, "[2026.10.08-17.00.00:000]LogNet: Display: Something?closed"); !tr.SPActive {
		t.Fatal("?closed outside a browse line is not an end")
	}
}

func TestLogTime(t *testing.T) {
	if got, want := LogTime("[2026.10.08-17.10.51:686][604]x"), time.Date(2026, 10, 8, 17, 10, 51, 0, time.UTC); !got.Equal(want) {
		t.Errorf("%v want %v", got, want)
	}
	for _, line := range []string{"", "[2026.10.08-17.10.51", "no bracket 2026.10.08-17.10.51:686 padding", "[2026.13.08-17.10.51:686]x", "[not a time at all here]x"} {
		if !LogTime(line).IsZero() {
			t.Errorf("%q has a time", line)
		}
	}
}

// The wide reading keeps what the log said for a session in the order of the lines, whatever else the lines hold: a log read line by line
// and a log read from a text give the same tracker.
func TestFeedingTwiceTheSameLinesGivesTheSameTracker(t *testing.T) {
	lines := []string{lnBOMOpen, lnCave, lnStart, lnSave, lnNoise, lnHub, lnSave, lnLeave1, lnLeave2, lnSave, lnMP, lnStart, lnExit, lnClosed}
	a, b := run(wide, lines...), run(wide, lines...)
	if a != b {
		t.Fatalf("%+v vs %+v", a, b)
	}
	if !a.Ended || a.SPActive || a.LastSaveAt.IsZero() || a.EndedAt.IsZero() {
		t.Fatalf("%+v", a)
	}
}
