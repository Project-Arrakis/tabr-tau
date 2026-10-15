package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Lines copied in shape from real client logs (issue #65); ids and addresses are synthetic.
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
	lnBOMOpen = "\xef\xbb\xbfLog: Log file open, 10/08/26 09:20:16"
)

func run(lines ...string) spTracker {
	var t spTracker
	for _, l := range lines {
		t.feed(l)
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
		if got := run(c.lines...).active; got != c.active {
			t.Errorf("%s: active=%v want %v", c.name, got, c.active)
		}
	}
	tr := run(lnStart, lnLeave1)
	if want := time.Date(2026, 10, 8, 17, 10, 51, 0, time.UTC); !tr.endedAt.Equal(want) {
		t.Fatalf("end time %v want %v", tr.endedAt, want)
	}
	if !run(lnStart).endedAt.IsZero() {
		t.Fatal("an active session has no end time")
	}
}

func TestLogPathFor(t *testing.T) {
	p := filepath.Join("C:", "Users", "x", "AppData", "Local", "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "123", "game.db")
	got, err := logPathFor(p)
	if err != nil || filepath.Base(got) != "DuneSandbox.log" || filepath.Base(filepath.Dir(got)) != "Logs" || filepath.Base(filepath.Dir(filepath.Dir(got))) != "Saved" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := logPathFor(filepath.Join("tmp", "somewhere", "game.db")); err == nil {
		t.Fatal("a save outside the standard folder has no known log")
	}
}

// writeLog builds Saved/Logs/DuneSandbox.log under a fake standard folder and returns the save path.
func writeLog(t *testing.T, text string) (savePath, logPath string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Saved")
	savePath = filepath.Join(root, "Cloud", "PlayerClientStorage", "FLS_retail", "1", "game.db")
	logPath = filepath.Join(root, "Logs", "DuneSandbox.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

func resetSPLog() {
	spLog.Lock()
	spLog.path, spLog.offset, spLog.tr, spLog.rest = "", 0, spTracker{}, ""
	spLog.Unlock()
}

func TestSinglePlayerStatusFollowsTheLog(t *testing.T) {
	resetSPLog()
	save, lp := writeLog(t, strings.Join([]string{lnBOMOpen, lnStart}, "\r\n")+"\r\n")
	st, err := singlePlayerStatus(save)
	if err != nil || !st.Active {
		t.Fatalf("expected active: %+v %v", st, err)
	}
	f, _ := os.OpenFile(lp, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(lnLeave1[:60]) // half a line: must not count yet
	f.Close()
	if st, _ = singlePlayerStatus(save); !st.Active {
		t.Fatal("an unfinished line must be ignored")
	}
	f, _ = os.OpenFile(lp, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(lnLeave1[60:] + "\r\n")
	f.Close()
	if st, _ = singlePlayerStatus(save); st.Active || st.EndedAt.IsZero() {
		t.Fatalf("the completed end line must end the session: %+v", st)
	}
	os.WriteFile(lp, []byte(lnNoise+"\r\n"), 0o644) // game restarted: new, shorter log
	if st, _ = singlePlayerStatus(save); st.Active || !st.EndedAt.IsZero() {
		t.Fatalf("a new log starts clean: %+v", st)
	}
	// a rotated log that is already longer than the old offset must still be read from its start
	os.WriteFile(lp, []byte("\xef\xbb\xbfLog: Log file open, 10/08/26 09:20:16\r\n"+strings.Repeat(lnNoise+"\r\n", 50)), 0o644)
	singlePlayerStatus(save)
	old, _ := os.Stat(lp)
	os.WriteFile(lp, []byte("\xef\xbb\xbfLog: Log file open, 10/08/26 11:00:00\r\n"+lnStart+"\r\n"+strings.Repeat(lnNoise+"\r\n", 80)), 0o644)
	if now, _ := os.Stat(lp); now.Size() <= old.Size() {
		t.Fatal("test setup: the new log must be longer than the old one")
	}
	if st, _ = singlePlayerStatus(save); !st.Active {
		t.Fatal("a longer, rotated log must be read from its start, so its single-player start is seen")
	}
	if _, err := singlePlayerStatus(filepath.Join(t.TempDir(), "elsewhere", "game.db")); err == nil {
		t.Fatal("an unknown log location must be an error")
	}
	os.Remove(lp)
	if _, err := singlePlayerStatus(save); err == nil {
		t.Fatal("a missing log must be an error, not 'not in single-player'")
	}
}

func appendLog(t *testing.T, lp, text string) {
	t.Helper()
	f, err := os.OpenFile(lp, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

// QA-6: the game appends while the editor reads, so a read can end in the middle of a line, also in the middle of the
// start marker itself, or hold whole lines followed by the beginning of the next one.
func TestSinglePlayerStatusAcrossSplitReads(t *testing.T) {
	resetSPLog()
	save, lp := writeLog(t, lnBOMOpen+"\r\n")
	if st, err := singlePlayerStatus(save); err != nil || st.Active {
		t.Fatalf("a log with only the open line is not single-player: %+v %v", st, err)
	}
	// the start marker "Browse (net mode" is cut in two: nothing counts until the line is finished
	cut := strings.Index(lnStart, "Browse (net mode") + len("Browse (")
	appendLog(t, lp, lnStart[:cut])
	if st, _ := singlePlayerStatus(save); st.Active {
		t.Fatal("half of the start marker must not start a session")
	}
	appendLog(t, lp, lnStart[cut:]) // the rest, still no newline
	if st, _ := singlePlayerStatus(save); st.Active {
		t.Fatal("a line without its newline must not count yet")
	}
	appendLog(t, lp, "\r\n")
	if st, _ := singlePlayerStatus(save); !st.Active {
		t.Fatal("the start marker, once complete across reads, starts the session")
	}
	// whole lines plus the beginning of the next one in a single read: the whole lines count, the tail waits
	appendLog(t, lp, lnNoise+"\r\n"+lnLeave2+"\r\n"+lnStart[:70])
	st, _ := singlePlayerStatus(save)
	if st.Active || st.EndedAt.IsZero() {
		t.Fatalf("the complete end line before the partial tail must end the session: %+v", st)
	}
	appendLog(t, lp, lnStart[70:]+"\r\n")
	if st, _ = singlePlayerStatus(save); !st.Active || !st.EndedAt.IsZero() {
		t.Fatalf("the tail, once finished, must be read as the line it was: %+v", st)
	}
	// reading again with nothing new changes nothing (no line is applied twice)
	before := st
	if st, _ = singlePlayerStatus(save); st != before {
		t.Fatalf("a read with no new text changed the state: %+v then %+v", before, st)
	}
	// an end marker split between its timestamp and its text
	appendLog(t, lp, lnLeave1[:30])
	if st, _ = singlePlayerStatus(save); !st.Active {
		t.Fatal("half an end line must not end the session")
	}
	appendLog(t, lp, lnLeave1[30:]+"\n")
	if st, _ = singlePlayerStatus(save); st.Active {
		t.Fatal("the finished end line must end the session")
	}
}

// QA-6: the tracker's smaller rules.
func TestTrackerEdgeRules(t *testing.T) {
	// an end marker with no session open changes nothing, and records no end time
	if tr := run(lnLeave2, lnExit, lnClosed); tr.active || !tr.endedAt.IsZero() {
		t.Fatalf("an end with no session open must not record an end: %+v", tr)
	}
	// starting a session clears the earlier end time
	if tr := run(lnStart, lnLeave2, lnStart); !tr.active || !tr.endedAt.IsZero() {
		t.Fatalf("a new session has no end time: %+v", tr)
	}
	// a repeated end marker keeps the first end time
	tr := run(lnStart, lnLeave1, lnLeave2)
	if want := time.Date(2026, 10, 8, 17, 10, 51, 0, time.UTC); !tr.endedAt.Equal(want) {
		t.Fatalf("the first end time stands, got %v", tr.endedAt)
	}
	// an end line without a timestamp counts as just ended
	noStamp := "LogNet: Display: Browse (net mode 2): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?closed"
	before := time.Now()
	tr = run(lnStart, noStamp)
	if tr.active || tr.endedAt.Before(before.Add(-time.Second)) {
		t.Fatalf("no timestamp: end counts as now, got %+v", tr)
	}
	// "?closed" only means the end when the line is a Browse line, and a Delaying Travel without ?closed (a challenge room) is no end
	if tr := run(lnStart, "[2026.10.08-17.00.00:000]LogNet: Display: Something?closed"); !tr.active {
		t.Fatal("?closed outside a Browse line is not an end")
	}
	if tr := run(lnStart, lnRoom); !tr.active {
		t.Fatal("a challenge room is no end")
	}
	// a local map without ?listen changes nothing, also while a session is open
	if tr := run(lnStart, lnCave); !tr.active {
		t.Fatal("the character cave inside a session is not an end")
	}
}

func TestLogTime(t *testing.T) {
	want := time.Date(2026, 10, 8, 17, 10, 51, 0, time.UTC)
	for _, c := range []struct {
		line string
		ok   bool
	}{
		{"[2026.10.08-17.10.51:686]rest", true},
		{"[2026.10.08-17.10.51:686]", true},
		{"[2026.10.08-17.10.51]", false}, // shorter than the game's prefix: not a log timestamp
		{" [2026.10.08-17.10.51:686]rest", false},
		{"2026.10.08-17.10.51:686]rest.....", false},
		{"[2026.13.08-17.10.51:686]rest", false}, // month 13
		{"[abcdefghijklmnopqrs:686]rest", false},
		{"", false},
	} {
		got := logTime(c.line)
		if c.ok && !got.Equal(want) || !c.ok && !got.IsZero() {
			t.Errorf("logTime(%q) = %v, ok want %v", c.line, got, c.ok)
		}
	}
}

func TestGameStateGraceBoundary(t *testing.T) {
	oldList, oldStatus, oldNow := listProcs, statusFor, nowFunc
	t.Cleanup(func() { listProcs, statusFor, nowFunc = oldList, oldStatus, oldNow })
	now := time.Date(2026, 10, 8, 17, 30, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	listProcs = func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	if spEndGrace != 15*time.Second {
		t.Fatalf("the documented grace after the end marker is 15 s, got %v", spEndGrace)
	}
	for _, c := range []struct {
		ago     time.Duration
		blocked bool
	}{
		{0, true}, {spEndGrace - time.Millisecond, true}, {spEndGrace, false}, {spEndGrace + time.Millisecond, false},
	} {
		statusFor = func(string) (spStatus, error) { return spStatus{EndedAt: now.Add(-c.ago)}, nil }
		if g := GameStateFor("x", true); g.Blocked != c.blocked {
			t.Errorf("ended %v ago: blocked=%v want %v", c.ago, g.Blocked, c.blocked)
		}
	}
}

func TestGameStateDecisions(t *testing.T) {
	oldList, oldStatus, oldNow := listProcs, statusFor, nowFunc
	t.Cleanup(func() { listProcs, statusFor, nowFunc = oldList, oldStatus, oldNow })
	now := time.Date(2026, 10, 8, 17, 30, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	procs := func(n ...string) func(bool) ([]string, string) { return func(bool) ([]string, string) { return n, "" } }
	cases := []struct {
		name    string
		list    func(bool) ([]string, string)
		status  spStatus
		statErr error
		blocked bool
		mode    string
		reason  string
	}{
		{"no game", procs(), spStatus{Active: true}, nil, false, "none", ""},
		{"menu or multiplayer", procs("DuneSandbox-Win64-Shipping.exe"), spStatus{}, nil, false, "menu_or_multiplayer", "saving is allowed"},
		{"single-player active", procs("DuneSandbox-Win64-Shipping.exe"), spStatus{Active: true}, nil, true, "single_player", "single-player session is active"},
		{"just ended", procs("DuneSandbox-Win64-Shipping.exe"), spStatus{EndedAt: now.Add(-3 * time.Second)}, nil, true, "single_player", "just ended"},
		{"ended long ago", procs("DuneSandbox-Win64-Shipping.exe"), spStatus{EndedAt: now.Add(-time.Minute)}, nil, false, "menu_or_multiplayer", ""},
		{"log unreadable while the game runs", procs("DuneSandbox-Win64-Shipping.exe"), spStatus{}, os.ErrPermission, true, "unknown", "cannot tell whether a single-player session"},
		{"process check failed", func(bool) ([]string, string) { return nil, "tasklist: boom" }, spStatus{}, nil, true, "unknown", "could not check"},
	}
	for _, c := range cases {
		listProcs = c.list
		statusFor = func(string) (spStatus, error) { return c.status, c.statErr }
		g := GameStateFor("x", true)
		if g.Blocked != c.blocked || g.Mode != c.mode || !strings.Contains(g.Reason, c.reason) {
			t.Errorf("%s: %+v", c.name, g)
		}
	}
}
