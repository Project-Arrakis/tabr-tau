package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/kit/gamefolder"
	"github.com/Project-Arrakis/tabr-tau/kit/sessionlog"
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
		if got := run(c.lines...).SPActive; got != c.active {
			t.Errorf("%s: active=%v want %v", c.name, got, c.active)
		}
	}
	tr := run(lnStart, lnLeave1)
	if want := time.Date(2026, 10, 8, 17, 10, 51, 0, time.UTC); !tr.EndedAt.Equal(want) {
		t.Fatalf("end time %v want %v", tr.EndedAt, want)
	}
	if !run(lnStart).EndedAt.IsZero() {
		t.Fatal("an active session has no end time")
	}
}

func TestLogPathFor(t *testing.T) {
	p := filepath.Join("C:", "Users", "x", "AppData", "Local", "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "123", "game.db")
	got, err := gamefolder.LogPathFor(p)
	if err != nil || filepath.Base(got) != "DuneSandbox.log" || filepath.Base(filepath.Dir(got)) != "Logs" || filepath.Base(filepath.Dir(filepath.Dir(got))) != "Saved" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := gamefolder.LogPathFor(filepath.Join("tmp", "somewhere", "game.db")); err == nil {
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
	if tr := run(lnLeave2, lnExit, lnClosed); tr.SPActive || !tr.EndedAt.IsZero() {
		t.Fatalf("an end with no session open must not record an end: %+v", tr)
	}
	// starting a session clears the earlier end time
	if tr := run(lnStart, lnLeave2, lnStart); !tr.SPActive || !tr.EndedAt.IsZero() {
		t.Fatalf("a new session has no end time: %+v", tr)
	}
	// a repeated end marker keeps the first end time
	tr := run(lnStart, lnLeave1, lnLeave2)
	if want := time.Date(2026, 10, 8, 17, 10, 51, 0, time.UTC); !tr.EndedAt.Equal(want) {
		t.Fatalf("the first end time stands, got %v", tr.EndedAt)
	}
	// an end line without a timestamp counts as just ended
	noStamp := "LogNet: Display: Browse (net mode 2): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?closed"
	before := time.Now()
	tr = run(lnStart, noStamp)
	if tr.SPActive || tr.EndedAt.Before(before.Add(-time.Second)) {
		t.Fatalf("no timestamp: end counts as now, got %+v", tr)
	}
	// "?closed" only means the end when the line is a Browse line, and a Delaying Travel without ?closed (a challenge room) is no end
	if tr := run(lnStart, "[2026.10.08-17.00.00:000]LogNet: Display: Something?closed"); !tr.SPActive {
		t.Fatal("?closed outside a Browse line is not an end")
	}
	if tr := run(lnStart, lnRoom); !tr.SPActive {
		t.Fatal("a challenge room is no end")
	}
	// a local map without ?listen changes nothing, also while a session is open
	if tr := run(lnStart, lnCave); !tr.SPActive {
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
		got := sessionlog.LogTime(c.line)
		if c.ok && !got.Equal(want) || !c.ok && !got.IsZero() {
			t.Errorf("sessionlog.LogTime(%q) = %v, ok want %v", c.line, got, c.ok)
		}
	}
}

// The save markers of the game's log (#449), in the shape of the real lines.
const (
	lnSave1 = "[2026.10.08-17.10.54:100][604][20424][0]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	lnSave0 = "[2026.10.08-17.09.54:100][604][20424][0]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
)

func TestTrackerRecordsTheGamesLastWriteOfTheDatabase(t *testing.T) {
	tr := run(lnStart, lnSave0, lnLeave2, lnSave1)
	if want := time.Date(2026, 10, 8, 17, 10, 54, 0, time.UTC); !tr.LastSaveAt.Equal(want) {
		t.Fatalf("last write %v want %v", tr.LastSaveAt, want)
	}
	if tr.EndedAt.IsZero() || !tr.LastSaveAt.After(tr.EndedAt) {
		t.Fatalf("the write after the end marker must be seen after it: ended %v, saved %v", tr.EndedAt, tr.LastSaveAt)
	}
	if !run(lnStart, lnLeave2).LastSaveAt.IsZero() {
		t.Fatal("no write line, no write time")
	}
	if tr := run(lnStart, lnNoise); !tr.LastSaveAt.IsZero() {
		t.Fatal("other lines are not writes")
	}
	// a line without a timestamp counts as just written, which only lengthens the wait
	if tr := run("LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk."); time.Since(tr.LastSaveAt) > time.Minute || tr.LastSaveAt.IsZero() {
		t.Fatalf("an untimed write line: %v", tr.LastSaveAt)
	}
}

// The log, read from a file, reports the write time too.
func TestSinglePlayerStatusReportsTheLastWrite(t *testing.T) {
	resetSPLog()
	sp, _ := writeLog(t, strings.Join([]string{lnStart, lnLeave2, lnSave1}, "\n")+"\n")
	st, err := singlePlayerStatus(sp)
	if err != nil || st.Active || st.EndedAt.IsZero() || !st.LastSaveAt.After(st.EndedAt) {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestTheRuleConstants(t *testing.T) {
	// The numbers the page and the PR state. dbQuietFor is the flat margin used when the log does not show both the end and the last write; endedFallbackGrace is the unmeasured
	// one; menuWriteGrace and menuQuietAfterWrite come from the measurement in docs/design/2026-10-27-menu-write-grace.md (6.9 s + 22.7 s, rounded up).
	if dbQuietFor != 3*time.Minute || endedFallbackGrace != 6*time.Minute || menuWriteGrace != 30*time.Second || menuQuietAfterWrite != time.Minute || noMarkerQuietFor != 10*time.Minute || maxWritesAfterEnd != 1 {
		t.Fatalf("dbQuietFor %v, endedFallbackGrace %v, menuWriteGrace %v, menuQuietAfterWrite %v, noMarkerQuietFor %v, maxWritesAfterEnd %d: change the README, SECURITY.md, the design notes, the changelog and the PR text with them", dbQuietFor, endedFallbackGrace, menuWriteGrace, menuQuietAfterWrite, noMarkerQuietFor, maxWritesAfterEnd)
	}
	// Longer margins than the measurement need are the point of #612: waiting is cheap. Never shorter than the largest write gap seen (22.7 s) times two, and the no-marker margin
	// is longer than the flat quiet time.
	if menuQuietAfterWrite < 2*22700*time.Millisecond || noMarkerQuietFor <= dbQuietFor {
		t.Fatalf("a margin is shorter than the gate allows: %v, %v", menuQuietAfterWrite, noMarkerQuietFor)
	}
	// Never below 20 s, and the grace covers the largest end-to-last-write gap measured (6.9 s) plus the largest save interval measured (22.7 s).
	if menuWriteGrace < 20*time.Second || menuWriteGrace < 6900*time.Millisecond+22700*time.Millisecond || menuQuietAfterWrite < 22700*time.Millisecond {
		t.Fatalf("the short grace is shorter than the measurement allows: %v, %v", menuWriteGrace, menuQuietAfterWrite)
	}
}

// The rule for a game process that exists (#449), in the order of its conditions. now is fixed; every case sets what the log says and what the file check says.
func TestGameStateDecisions(t *testing.T) {
	oldList, oldStatus, oldNow, oldHeld, oldTime := listProcs, statusFor, nowFunc, fileHeld, fileTimeFor
	t.Cleanup(func() {
		listProcs, statusFor, nowFunc, fileHeld, fileTimeFor = oldList, oldStatus, oldNow, oldHeld, oldTime
	})
	now := time.Date(2026, 10, 8, 17, 30, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	game := func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	none := func(bool) ([]string, string) { return nil, "" }
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	cases := []struct {
		name    string
		list    func(bool) ([]string, string)
		status  spStatus
		statErr error
		held    bool
		heldErr error
		blocked bool
		mode    string
		reason  string
		// the save file's own time (the second signal, #612): how long ago it was last written (zero: long ago), whether it is the file the editor installed, and whether it cannot be read
		fileAge time.Duration
		fileOwn bool
		fileErr error
	}{
		{name: "no game", list: none, status: spStatus{Active: true}, mode: "none"},
		{name: "the game at its menu, no session in this log, nothing held: written", list: game, mode: "unknown_quiet"},
		{name: "the process list cannot be read", list: func(bool) ([]string, string) { return nil, "process snapshot: boom" }, blocked: true, mode: "unknown", reason: "could not check whether the game is running"},
		{name: "the log cannot be read", list: game, statErr: os.ErrPermission, blocked: true, mode: "game_running", reason: "its log cannot be read"},
		{name: "a session is active", list: game, status: spStatus{Active: true, LastSaveAt: ago(10 * time.Second)}, blocked: true, mode: "single_player", reason: "single-player session is active"},
		{name: "the #419 case: left the world 23 s ago, the game's last write 20 s ago", list: game, status: spStatus{EndedAt: ago(23 * time.Second), LastSaveAt: ago(20 * time.Second)}, blocked: true, mode: "session_ended", reason: "wrote its database 20 seconds ago"},
		// the end of the session AND the last write after it are in the log: the measured grace (#470)
		{name: "ended 10 s ago, last write 8 s ago: waits for the grace, 20 s more", list: game, status: spStatus{EndedAt: ago(10 * time.Second), LastSaveAt: ago(8 * time.Second)}, blocked: true, mode: "session_ended", reason: "(about 52 seconds more)"},
		{name: "ended 5 s ago, last write 3 s ago: the countdown says 57 seconds", list: game, status: spStatus{EndedAt: ago(5 * time.Second), LastSaveAt: ago(3 * time.Second)}, blocked: true, mode: "session_ended", reason: "(about 57 seconds more)"},
		{name: "ended 29 s ago: the 30 s are nearly over, the minute after the write is not", list: game, status: spStatus{EndedAt: ago(29 * time.Second), LastSaveAt: ago(27 * time.Second)}, blocked: true, mode: "session_ended", reason: "(about 33 seconds more)"},
		{name: "ended exactly the grace ago: still inside the minute after the write", list: game, status: spStatus{EndedAt: ago(menuWriteGrace), LastSaveAt: ago(menuWriteGrace - 3*time.Second)}, blocked: true, mode: "session_ended", reason: "(about 33 seconds more)"},
		{name: "ended 65 s ago, the write 59 s ago: one second more", list: game, status: spStatus{EndedAt: ago(65 * time.Second), LastSaveAt: ago(59 * time.Second)}, blocked: true, mode: "session_ended", reason: "(about 1 second more)"},
		{name: "ended 70 s ago with the write 5 s after the end: written", list: game, status: spStatus{EndedAt: ago(70 * time.Second), LastSaveAt: ago(65 * time.Second)}, mode: "menu"},
		{name: "the slowest exit measured (last write 6.9 s after the end): written a minute after that write", list: game, status: spStatus{EndedAt: ago(67 * time.Second), LastSaveAt: ago(60 * time.Second)}, mode: "menu"},
		{name: "a write later than anything measured (28 s after the end) holds the edit back a minute from that write", list: game, status: spStatus{EndedAt: ago(40 * time.Second), LastSaveAt: ago(12 * time.Second)}, blocked: true, mode: "session_ended", reason: "(about 48 seconds more)"},
		{name: "that late write, a minute on: written", list: game, status: spStatus{EndedAt: ago(63 * time.Second), LastSaveAt: ago(menuQuietAfterWrite)}, mode: "menu"},
		{name: "ended, the game wrote 2 minutes ago, 10 s after the end: written", list: game, status: spStatus{EndedAt: ago(130 * time.Second), LastSaveAt: ago(2 * time.Minute)}, mode: "menu"},
		{name: "ended 10 minutes ago, last write seen: written", list: game, status: spStatus{EndedAt: ago(10 * time.Minute), LastSaveAt: ago(dbQuietFor - time.Second)}, mode: "menu"},
		{name: "ended long ago with the last write seen: written", list: game, status: spStatus{EndedAt: ago(time.Hour), LastSaveAt: ago(time.Hour - 5*time.Second)}, mode: "menu"},
		{name: "a write in the same second as the end is the periodic one before it, not the last: the flat quiet time applies", list: game, status: spStatus{EndedAt: ago(time.Minute), LastSaveAt: ago(time.Minute)}, blocked: true, mode: "session_ended", reason: "must be quiet for 3 minutes"},
		// no end marker: the flat quiet time as before
		{name: "no end seen, the game wrote 2 minutes ago: still not quiet", list: game, status: spStatus{LastSaveAt: ago(2 * time.Minute)}, blocked: true, mode: "session_ended", reason: "wrote its database 2 minutes ago"},
		{name: "no end seen, quiet for just under the limit", list: game, status: spStatus{LastSaveAt: ago(dbQuietFor - time.Second)}, blocked: true, mode: "session_ended", reason: "wrote its database"},
		{name: "no end seen, quiet for exactly the limit: written", list: game, status: spStatus{LastSaveAt: ago(dbQuietFor)}, mode: "menu"},
		// ended, but the last write is not in the log: the fallback
		{name: "ended 3 s ago, no write after the end seen: the fallback wait", list: game, status: spStatus{EndedAt: ago(3 * time.Second)}, blocked: true, mode: "session_ended", reason: "does not show the game's last write"},
		{name: "ended 100 s ago, no write after the end seen: the fallback, not the short grace", list: game, status: spStatus{EndedAt: ago(100 * time.Second), LastSaveAt: ago(40 * time.Minute)}, blocked: true, mode: "session_ended", reason: "waits 6 minutes after the end"},
		{name: "ended 5 minutes ago, no write after it seen: still the fallback wait", list: game, status: spStatus{EndedAt: ago(5 * time.Minute), LastSaveAt: ago(40 * time.Minute)}, blocked: true, mode: "session_ended", reason: "waits 6 minutes after the end"},
		{name: "fallback wait over: written", list: game, status: spStatus{EndedAt: ago(endedFallbackGrace), LastSaveAt: ago(40 * time.Minute)}, mode: "menu"},
		{name: "the game has closed just after the end: written at once", list: none, status: spStatus{EndedAt: ago(2 * time.Second), LastSaveAt: ago(time.Second)}, mode: "none"},
		{name: "a session is active again after an old end: waits", list: game, status: spStatus{Active: true, EndedAt: ago(time.Hour), LastSaveAt: ago(time.Hour)}, blocked: true, mode: "single_player", reason: "single-player session is active"},
		{name: "another program has the save file open", list: game, status: spStatus{EndedAt: ago(time.Hour), LastSaveAt: ago(time.Hour)}, held: true, blocked: true, mode: "game_running", reason: "has the save file open"},
		{name: "the file check cannot be made", list: game, heldErr: os.ErrPermission, blocked: true, mode: "game_running", reason: "could not check whether another program has the save file open"},
		// #612, D2-1: the log says nothing about a session, and then the save file's own time is the only evidence
		{name: "no marker in the log at all, the file written 5 seconds ago: unknown, waits", list: game, fileAge: 5 * time.Second, blocked: true, mode: "game_running", reason: "its log shows no sign of a session"},
		{name: "no marker, the file written 9 minutes ago: still inside the ten", list: game, fileAge: 9 * time.Minute, blocked: true, mode: "game_running", reason: "(about 60 seconds more)"},
		{name: "no marker, the file quiet for exactly ten minutes: written", list: game, fileAge: noMarkerQuietFor, mode: "unknown_quiet"},
		{name: "no marker, the file quiet for hours: written", list: game, fileAge: 5 * time.Hour, mode: "unknown_quiet"},
		{name: "no marker, the file is the one the editor installed: its time says nothing", list: game, fileAge: 5 * time.Second, fileOwn: true, mode: "unknown_quiet"},
		{name: "no marker, the file time cannot be read: not quiet", list: game, fileErr: os.ErrPermission, blocked: true, mode: "game_running", reason: "could not read the modification time of the save file"},
		{name: "a file time in the future (a clock that is wrong) is not quiet", list: game, fileAge: -time.Hour, blocked: true, mode: "game_running", reason: "its log shows no sign of a session"},
		{name: "the log shows an old write only, the file written 2 minutes ago, the game open: the flat quiet time of the file", list: game, status: spStatus{LastSaveAt: ago(time.Hour)}, fileAge: 2 * time.Minute, blocked: true, mode: "session_ended", reason: "the save file changed 2 minutes ago, whatever the log says"},
		{name: "an old write in the log, the file quiet for the flat time: written", list: game, status: spStatus{LastSaveAt: ago(time.Hour)}, fileAge: dbQuietFor, mode: "menu"},
		{name: "the log says the game left the world long ago, the file was written 20 seconds ago: the file wins", list: game, status: spStatus{EndedAt: ago(time.Hour), LastSaveAt: ago(time.Hour - 5*time.Second)}, fileAge: 20 * time.Second, blocked: true, mode: "session_ended", reason: "the save file changed 20 seconds ago, whatever the log says"},
		{name: "a clean end, the file written 30 s ago: the minute of the short grace applies to the file too", list: game, status: spStatus{EndedAt: ago(70 * time.Second), LastSaveAt: ago(65 * time.Second)}, fileAge: 30 * time.Second, blocked: true, mode: "session_ended", reason: "(about 30 seconds more)"},
		{name: "a clean end, the editor's own file written 30 s ago: written", list: game, status: spStatus{EndedAt: ago(70 * time.Second), LastSaveAt: ago(65 * time.Second)}, fileAge: 30 * time.Second, fileOwn: true, mode: "menu"},
		// #612, D2-2: more than one write after the end marker is not leaving a world
		{name: "an end and two writes after it: the short grace is not trusted", list: game, status: spStatus{EndedAt: ago(100 * time.Second), LastSaveAt: ago(40 * time.Second), WritesAfterEnd: 2}, blocked: true, mode: "session_ended", reason: "must be quiet for 3 minutes"},
		{name: "an end and two writes after it, quiet for the flat time: the fallback wait says why", list: game, status: spStatus{EndedAt: ago(4 * time.Minute), LastSaveAt: ago(dbQuietFor), WritesAfterEnd: 2}, blocked: true, mode: "session_ended", reason: "2 times after that, which is not what leaving a world looks like"},
		{name: "an end and two writes after it, all quiet for the fallback time: written", list: game, status: spStatus{EndedAt: ago(endedFallbackGrace), LastSaveAt: ago(endedFallbackGrace - 10*time.Second), WritesAfterEnd: 2}, mode: "menu"},
		{name: "an end and exactly one write after it still takes the short grace", list: game, status: spStatus{EndedAt: ago(70 * time.Second), LastSaveAt: ago(65 * time.Second), WritesAfterEnd: 1}, mode: "menu"},
	}
	for _, c := range cases {
		listProcs = c.list
		statusFor = func(string) (spStatus, error) { return c.status, c.statErr }
		fileHeld = func(string) (bool, error) { return c.held, c.heldErr }
		fileTimeFor = func(string) (time.Time, bool, error) {
			if c.fileErr != nil {
				return time.Time{}, false, c.fileErr
			}
			if c.fileAge == 0 {
				return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), c.fileOwn, nil
			}
			return ago(c.fileAge), c.fileOwn, nil
		}
		g := GameStateFor("x", true)
		if g.Blocked != c.blocked || g.Mode != c.mode || !strings.Contains(g.Reason, c.reason) {
			t.Errorf("%s: %+v", c.name, g)
		}
		if !g.Blocked && g.Reason != "" {
			t.Errorf("%s: an allowed write has no reason, got %q", c.name, g.Reason)
		}
		if c.blocked && !strings.Contains(g.Reason, "queued") && c.mode != "unknown" {
			t.Errorf("%s: the reason must say the edits are queued: %q", c.name, g.Reason)
		}
		if c.list != nil && c.mode == "menu" && len(g.Processes) != 1 {
			t.Errorf("%s: the processes must be reported even when saving is allowed (the automatic pass waits on them): %+v", c.name, g)
		}
	}
}

// The wait is said in words that are true: seconds under two minutes, whole minutes after, rounded up for what is still to wait.
func TestApproxDuration(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{{0, "0 seconds"}, {-time.Second, "0 seconds"}, {time.Second, "1 second"}, {1400 * time.Millisecond, "1 second"}, {20 * time.Second, "20 seconds"}, {25 * time.Second, "25 seconds"}, {119 * time.Second, "119 seconds"}, {2 * time.Minute, "2 minutes"}, {2*time.Minute + time.Second, "3 minutes"}, {6 * time.Minute, "6 minutes"}} {
		if got := approxDuration(c.d); got != c.want {
			t.Errorf("%v: %q want %q", c.d, got, c.want)
		}
	}
}

// The same rule, end to end: a fake log on disk (with a periodic write just before the end marker, the end marker, and the game's last write 2 s after it, as in the
// real exits of #470), the file time of the last write as the log reports it, and the clock moved through the grace (#470).
func TestMenuGraceFromAFakeLog(t *testing.T) {
	oldList, oldStatus, oldNow, oldHeld := listProcs, statusFor, nowFunc, fileHeld
	t.Cleanup(func() { listProcs, statusFor, nowFunc, fileHeld = oldList, oldStatus, oldNow, oldHeld })
	listProcs = func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	statusFor = singlePlayerStatus
	fileHeld = func(string) (bool, error) { return false, nil }
	periodic := "[2026.10.08-17.10.51:100][604][20424][0]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	last := "[2026.10.08-17.10.53:700][604][20424][0]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	end := time.Date(2026, 10, 8, 17, 10, 51, 0, time.UTC)
	at := func(d time.Duration) { nowFunc = func() time.Time { return end.Add(d) } }

	// ended, the last write seen: the short grace, before and after it passes
	resetSPLog()
	save, _ := writeLog(t, strings.Join([]string{lnStart, periodic, lnLeave1, last}, "\r\n")+"\r\n")
	at(10 * time.Second)
	if g := processState(save, true); !g.Blocked || g.Mode != "session_ended" || !strings.Contains(g.Reason, "(about 52 seconds more)") {
		t.Fatalf("10 s after the end: %+v", g)
	}
	at(menuWriteGrace) // the 30 s after the end are over, the minute after the write (2 s after the end) is not
	if g := processState(save, true); !g.Blocked || !strings.Contains(g.Reason, "(about 32 seconds more)") {
		t.Fatalf("the grace after the end is over, the quiet after the write is not: %+v", g)
	}
	at(2*time.Second + menuQuietAfterWrite - time.Second)
	if g := processState(save, true); !g.Blocked || !strings.Contains(g.Reason, "(about 1 second more)") {
		t.Fatalf("1 s before the quiet time is over: %+v", g)
	}
	at(2*time.Second + menuQuietAfterWrite)
	if g := processState(save, true); g.Blocked || g.Mode != "menu" {
		t.Fatalf("the quiet time is over: %+v", g)
	}

	// ended, the log shows the periodic write before the end but not the last one: the six-minute fallback, not the grace
	resetSPLog()
	save, _ = writeLog(t, strings.Join([]string{lnStart, periodic, lnLeave1}, "\r\n")+"\r\n")
	at(4 * time.Minute) // past the flat quiet time, inside the fallback
	if g := processState(save, true); !g.Blocked || !strings.Contains(g.Reason, "does not show the game's last write") || !strings.Contains(g.Reason, "(about 2 minutes more)") {
		t.Fatalf("no write after the end, 4 minutes on: %+v", g)
	}
	at(endedFallbackGrace)
	if g := processState(save, true); g.Blocked {
		t.Fatalf("the fallback is over: %+v", g)
	}

	// the game writes once more at the menu, later than anything measured: the grace restarts from that write
	resetSPLog()
	var lp string
	save, lp = writeLog(t, strings.Join([]string{lnStart, periodic, lnLeave1, last}, "\r\n")+"\r\n")
	at(2*time.Second + menuQuietAfterWrite)
	if g := processState(save, true); g.Blocked {
		t.Fatalf("written at the end of the quiet time: %+v", g)
	}
	appendLog(t, lp, "[2026.10.08-17.11.20:000][604][20424][0]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds.\r\n")
	at(31 * time.Second) // 20:00 is 29 s after the end; the clock is 2 s after the late write: a second write after the end, so the short grace is not trusted any more (#612, D2-2)
	if g := processState(save, true); !g.Blocked || !strings.Contains(g.Reason, "must be quiet for 3 minutes") {
		t.Fatalf("a late write must hold the edit back: %+v", g)
	}

	// a new session started: waits again
	appendLog(t, lp, lnStart+"\r\n")
	at(10 * time.Minute)
	if g := processState(save, true); !g.Blocked || g.Mode != "single_player" {
		t.Fatalf("a session is active: %+v", g)
	}

	// the log cannot be read: waits, as before
	os.Remove(lp)
	resetSPLog()
	if g := processState(save, true); !g.Blocked || g.Mode != "game_running" || !strings.Contains(g.Reason, "log cannot be read") {
		t.Fatalf("no log: %+v", g)
	}
}
