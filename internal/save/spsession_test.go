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
	if _, err := singlePlayerStatus(filepath.Join(t.TempDir(), "elsewhere", "game.db")); err == nil {
		t.Fatal("an unknown log location must be an error")
	}
	os.Remove(lp)
	if _, err := singlePlayerStatus(save); err == nil {
		t.Fatal("a missing log must be an error, not 'not in single-player'")
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
