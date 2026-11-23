package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tests for the live-save gate's inputs (#612, audit D2). Every log line here is synthetic: it has the shape of the lines the editor reads, with made-up maps and times. Every file
// is a temp file; nothing is the game's save.

const (
	lgStart  = "[2026.11.01-10.00.00:000][  1][1]LogNet: Display: Browse (net mode 0): /Game/Maps/Test_1?listen"
	lgSave   = "[2026.11.01-10.00.20:000][  2][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	lgSave2  = "[2026.11.01-10.05.02:000][  2][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	lgSave3  = "[2026.11.01-10.05.22:000][  2][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	lgSave4  = "[2026.11.01-10.05.42:000][  2][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	lgMenu   = "[2026.11.01-10.05.00:000][  3][1]LogNet: Display: Browse (net mode 2): /Game/Maps/Menu_1?closed"
	lgRemote = "[2026.11.01-10.05.00:000][  3][1]LogNet: Display: Browse (net mode 0): 192.0.2.7:7000/Game/Maps/Menu_1?flowtype=Login"
	// the words of a browse line inside a line of some other category: not a browse line
	lgChatMenu   = "[2026.11.01-10.02.00:000][  3][1]LogOther: Display: note: Browse (net mode 2): /Game/Maps/Menu_1?closed"
	lgChatRemote = "[2026.11.01-10.02.00:000][  3][1]LogOther: Display: note: Browse (net mode 0): 192.0.2.7:7000/Game/Maps/Menu_1"
	// a line of another category that holds a browse end and also the closing of the log: neither is at the start of the text
	lgChatThenClosed = "[2026.11.01-10.02.00:000][  3][1]LogOther: Display: note: Browse (net mode 2): /Game/Maps/Menu_1?closed; Log file closed"
	// the other end markers as the game writes them, and the same words inside lines of some other category
	lgExit        = "[2026.11.01-10.06.00:000]LogWindows: Log: FPlatformMisc::RequestExit(0, <NoCallSiteInfo>)"
	lgClosed      = "[2026.11.01-10.06.01:000][  9][5][0]Log: Log file closed, 11/01/26 10:06:01"
	lgDelayed     = "[2026.11.01-10.05.00:000][  3][1]LogNet: Display: Delaying Travel (net mode Listen Server): ?closed?delayed_travel"
	lgChatExit    = "[2026.11.01-10.02.00:000][  3][1]LogOther: Display: note: FPlatformMisc::RequestExit( was not called"
	lgChatClosed  = "[2026.11.01-10.02.00:000][  3][1]LogOther: Display: note: the Log file closed message is not a log line"
	lgChatDelayed = "[2026.11.01-10.02.00:000][  3][1]LogOther: Display: note: Delaying Travel (net mode Listen Server): ?closed?delayed_travel"
)

func feedAll(lines ...string) spTracker {
	var tr spTracker
	for _, l := range lines {
		tr.feed(l)
	}
	return tr
}

// gateStubs puts the machine in a fixed state for a test of processState: one game process, the log read from disk, nothing holds the file, the file time as the shipped check
// reads it. The previous values are put back when the test ends.
func gateStubs(t *testing.T) {
	t.Helper()
	oldList, oldStatus, oldHeld, oldTime, oldNow := listProcs, statusFor, fileHeld, fileTimeFor, nowFunc
	t.Cleanup(func() {
		listProcs, statusFor, fileHeld, fileTimeFor, nowFunc = oldList, oldStatus, oldHeld, oldTime, oldNow
	})
	listProcs = func(bool) ([]string, string) { return duneProc, "" }
	statusFor = singlePlayerStatus
	fileHeld = func(string) (bool, error) { return false, nil }
	fileTimeFor = statFileTime
	nowFunc = time.Now
	resetSPLog()
}

// setAge sets the modification time of a file to age ago.
func setAge(t *testing.T, path string, age time.Duration) {
	t.Helper()
	at := time.Now().Add(-age)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// D2-3: a browse line ends a session only at the start of the line's text, after the logger's prefixes and the category.
func TestStrictEndsOnly(t *testing.T) {
	if tr := feedAll(lgStart, lgChatMenu); !tr.SPActive || !tr.EndedAt.IsZero() {
		t.Fatalf("a line that only contains a closed browse ended the session: %+v", tr.State)
	}
	if tr := feedAll(lgStart, lgChatRemote); !tr.SPActive || !tr.EndedAt.IsZero() {
		t.Fatalf("a line that only contains a remote browse ended the session: %+v", tr.State)
	}
	if tr := feedAll(lgStart, lgMenu); tr.SPActive || tr.EndedAt.IsZero() {
		t.Fatalf("a real closed browse did not end the session: %+v", tr.State)
	}
	if tr := feedAll(lgStart, lgRemote); tr.SPActive || tr.EndedAt.IsZero() {
		t.Fatalf("a real remote browse did not end the session: %+v", tr.State)
	}
	// the exit, the closing of the log and the delayed close end a session at the start of a line's text, as the game writes them, and not inside another line (D2-3)
	for name, l := range map[string]string{"exit": lgExit, "log closed": lgClosed, "delayed close": lgDelayed} {
		if tr := feedAll(lgStart, l); tr.SPActive || tr.EndedAt.IsZero() {
			t.Fatalf("%s: a real end line did not end the session: %+v", name, tr.State)
		}
	}
	for name, l := range map[string]string{"exit": lgChatExit, "log closed": lgChatClosed, "delayed close": lgChatDelayed, "browse and closed": lgChatThenClosed} {
		if tr := feedAll(lgStart, l); !tr.SPActive || !tr.EndedAt.IsZero() {
			t.Fatalf("%s: a line that only contains the words ended the session: %+v", name, tr.State)
		}
	}
	// a start still counts wherever the words stand (the unsafe mistake is a missed start, not a spurious one)
	other := "[2026.11.01-10.00.00:000][  1][1]LogOther: Display: note: Browse (net mode 0): /Game/Maps/Test_1?listen"
	if tr := feedAll(other); !tr.SPActive {
		t.Fatalf("a start in a line of another category was missed: %+v", tr.State)
	}
}

// D2-1 and D2-2: what the log currently asserts (spStatus.seen), and the count of writes after the end.
func TestTrackerSeenAndWritesAfterEnd(t *testing.T) {
	status := func(tr spTracker) spStatus {
		return spStatus{Active: tr.SPActive, EndedAt: tr.EndedAt, LastSaveAt: tr.LastSaveAt}
	}
	if st := status(feedAll("[2026.11.01-10.00.00:000]LogOther: nothing of interest", lgChatMenu)); st.seen() {
		t.Fatal("lines that are none of the markers made the log assert something")
	}
	if st := status(feedAll(lgStart)); !st.seen() {
		t.Fatal("a start is asserted")
	}
	// a remote browse with no session open ends nothing and asserts nothing: it is not remembered as a marker (LOW 4: nothing is sticky)
	if st := status(feedAll(lgRemote)); st.seen() {
		t.Fatal("a remote browse with no session open must not make the state known")
	}
	tr := feedAll(lgSave)
	if st := status(tr); !st.seen() || tr.writesAfterEnd != 0 {
		t.Fatalf("a write line is asserted, and with no end it is not a write after the end: %+v", tr)
	}
	tr = feedAll(lgStart, lgSave, lgMenu)
	if tr.writesAfterEnd != 0 {
		t.Fatalf("no write after the end yet: %d", tr.writesAfterEnd)
	}
	tr.feed(lgSave2)
	if tr.writesAfterEnd != 1 {
		t.Fatalf("one write after the end: %d", tr.writesAfterEnd)
	}
	tr.feed(lgSave3)
	if tr.writesAfterEnd != 2 {
		t.Fatalf("two writes after the end: %d", tr.writesAfterEnd)
	}
	tr.feed(lgStart) // a new session
	if tr.writesAfterEnd != 0 || !tr.EndedAt.IsZero() {
		t.Fatalf("a new session clears the count: %+v", tr)
	}
}

// D2-2: a local ?closed browse while the world stays loaded (the game goes on writing) does not give the short grace.
func TestEndThatIsFollowedByWritesIsNotTrusted(t *testing.T) {
	gateStubs(t)
	fileTimeFor = func(string) (time.Time, bool, error) { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), false, nil }
	// 90 s after the end the short rule (30 s after the end and 60 s after the last write) would be over for a clean exit; here the game went on writing
	nowFunc = func() time.Time { return time.Date(2026, 11, 1, 10, 7, 0, 0, time.UTC) }
	save, _ := writeLog(t, strings.Join([]string{lgStart, lgSave, lgMenu, lgSave2, lgSave3, lgSave4}, "\n")+"\n")
	g := processState(save, true)
	if !g.Blocked || !strings.Contains(g.Detail, "must be quiet for 3 minutes") || !strings.Contains(g.Reason, "they are written in about") {
		t.Fatalf("a game that goes on writing after a closed browse was let through: %+v", g)
	}
	// and a clean exit of the same shape (one write after the end) is let through after the same time
	resetSPLog()
	save, _ = writeLog(t, strings.Join([]string{lgStart, lgSave, lgMenu, lgSave2}, "\n")+"\n")
	if g := processState(save, true); g.Blocked {
		t.Fatalf("a clean exit was held: %+v", g)
	}
}

// D2-1: the log can be read, a game process exists, and no line is a marker (a game update reworded the lines): the editor does not take that for "no world is loaded". The save
// file's own time decides, through the real file check on a temp file.
func TestRewordedLogFallsBackToTheFileTime(t *testing.T) {
	gateStubs(t)
	reworded := strings.Join([]string{
		"[2026.11.01-10.00.00:000][  1][1]LogNetwork: Display: LoadMap (mode 0): /Game/Maps/Test_1 listen=1",
		"[2026.11.01-10.00.20:000][  2][1]LogPersistence: Display: Database written. Next write in 60 s.",
	}, "\n") + "\n"
	save, _ := writeLog(t, reworded)
	if err := os.MkdirAll(filepath.Dir(save), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(save, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	setAge(t, save, 20*time.Second) // the game rewrites the file every 20 s while a world is loaded
	if g := processState(save, true); !g.Blocked || g.Mode != "game_running" || !strings.Contains(g.Detail, "its log shows no sign of a session") || !strings.Contains(g.Reason, "the save file changed 20 seconds ago") {
		t.Fatalf("a reworded log with a file written 20 s ago let a write through: %+v", g)
	}
	setAge(t, save, 5*time.Minute)
	if g := processState(save, true); !g.Blocked {
		t.Fatalf("a reworded log with a file written 5 minutes ago (the game may write every five minutes) let a write through: %+v", g)
	}
	setAge(t, save, noMarkerQuietFor+time.Minute)
	if g := processState(save, true); g.Blocked {
		t.Fatalf("a file quiet for longer than the margin must be written: %+v", g)
	}
	// a sidecar the database engine writes instead of the file: its time counts
	if err := os.WriteFile(save+"-wal", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	setAge(t, save+"-wal", time.Second)
	if g := processState(save, true); !g.Blocked {
		t.Fatalf("a write-ahead file written a second ago was ignored: %+v", g)
	}
	os.Remove(save + "-wal")
	// the file cannot be looked at
	os.Remove(save)
	if g := processState(save, true); !g.Blocked || !strings.Contains(g.Reason, "could not read the modification time") {
		t.Fatalf("a file that cannot be looked at is not quiet: %+v", g)
	}
}

// D2-1: the log does say a session ended long ago, and the game is writing the file anyway (the end line was not the end of a world): the file's time wins.
func TestFileWrittenRecentlyBeatsAnOldEnd(t *testing.T) {
	gateStubs(t)
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Now().Add(-time.Hour), LastSaveAt: time.Now().Add(-time.Hour + 5*time.Second)}, nil
	}
	p := makeSave(t, `create table t(a integer);`)
	setAge(t, p, 10*time.Second)
	if g := processState(p, true); !g.Blocked || !strings.Contains(g.Reason, "the save file changed") {
		t.Fatalf("a file written 10 s ago, whatever the log says: %+v", g)
	}
	setAge(t, p, dbQuietFor+time.Second)
	if g := processState(p, true); g.Blocked {
		t.Fatalf("a file quiet for the flat time with the log at the menu: %+v", g)
	}
}

// D2-1: the editor's own writes move the file's time and must not hold the editor's next edit back; a write by anything else does.
func TestOwnWriteIsNotTheGames(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	setAge(t, p, time.Hour)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	gateStubs(t)
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Now().Add(-time.Hour), LastSaveAt: time.Now().Add(-time.Hour + 5*time.Second)}, nil
	}
	t.Cleanup(func() { forgetOwnWrite(p) })

	if _, err := s.ExecScript(`insert into t values (1)`); err != nil {
		t.Fatal(err)
	}
	if res, err := s.CommitForTests(); err != nil || res["saved"] != true {
		t.Fatalf("the first write, the file quiet for an hour: %v %v", res, err)
	}
	if _, own, err := fileTimeFor(p); err != nil || !own {
		t.Fatalf("the file the editor just installed is not recognised as its own: own %v, %v", own, err)
	}
	// seconds later, the game open at its menu: the editor's own time is not the game's
	if _, err := s.ExecScript(`insert into t values (2)`); err != nil {
		t.Fatal(err)
	}
	if res, err := s.CommitForTests(); err != nil || res["saved"] != true {
		t.Fatalf("a second edit seconds after the first must be written: %v %v", res, err)
	}
	// something else writes the file (the game): size or time differs from what the editor wrote, and the next edit waits
	at := time.Now().Add(time.Second)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
	if _, own, _ := fileTimeFor(p); own {
		t.Fatal("a file touched by someone else is still taken for the editor's own")
	}
	if _, err := s.ExecScript(`insert into t values (3)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitForTests(); err == nil || !strings.Contains(err.Error(), "the save file changed") {
		t.Fatalf("an edit after the file was written by something else must wait: %v", err)
	}
}

// The record of the editor's own write is dropped on request and is told apart by size and time.
func TestOwnWriteRecord(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	t.Cleanup(func() { forgetOwnWrite(p) })
	noteOwnWrite(p)
	if _, own, _ := statFileTime(p); !own {
		t.Fatal("a recorded file is not its own")
	}
	forgetOwnWrite(p)
	if _, own, _ := statFileTime(p); own {
		t.Fatal("a forgotten record still counts")
	}
	noteOwnWrite(p)
	if err := os.WriteFile(p, []byte("a longer file than before, written by someone else"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, own, _ := statFileTime(p); own {
		t.Fatal("a file that changed after the record counts as the editor's own")
	}
	noteOwnWrite(p + "-missing") // a file that cannot be looked at is not recorded, and nothing panics
	if _, own, err := statFileTime(p + "-missing"); err == nil || own {
		t.Fatalf("a missing file: own %v, %v", own, err)
	}
}

// G-007: the pass nobody watches waits for every game process, whatever the log and the file say; a hand edit does not.
func TestUnattendedStateWaitsForEveryGameProcess(t *testing.T) {
	gateStubs(t)
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Now().Add(-time.Hour), LastSaveAt: time.Now().Add(-time.Hour + 5*time.Second)}, nil
	}
	fileTimeFor = func(string) (time.Time, bool, error) { return time.Now().Add(-time.Hour), false, nil }

	if g := GameStateFor("x", true); g.Blocked || g.Mode != "menu" {
		t.Fatalf("a hand edit with the game at its menu: %+v", g)
	}
	g := UnattendedStateFor("x", true)
	if !g.Blocked || g.Mode != "game_running" || len(g.Processes) != 1 || !strings.HasSuffix(g.Reason, QueuedUntilClears) || !strings.Contains(g.Reason, "closed") {
		t.Fatalf("the unattended pass with the game at its menu: %+v", g)
	}
	// a session is active: the reason of the ordinary gate is kept
	statusFor = func(string) (spStatus, error) { return spStatus{Active: true}, nil }
	if g := UnattendedStateFor("x", true); !g.Blocked || g.Mode != "single_player" {
		t.Fatalf("the reason of the ordinary gate must be kept: %+v", g)
	}
	// no process: the ordinary answer
	listProcs = func(bool) ([]string, string) { return nil, "" }
	if g := UnattendedStateFor("x", true); g.Blocked || g.Mode != "none" {
		t.Fatalf("no game: %+v", g)
	}
	// the process list cannot be read: blocked as before
	listProcs = func(bool) ([]string, string) { return nil, "boom" }
	if g := UnattendedStateFor("x", true); !g.Blocked || g.Mode != "unknown" {
		t.Fatalf("an unreadable process list: %+v", g)
	}
	// a copy that is not the game's own has no game to wait for
	listProcs = func(bool) ([]string, string) { return duneProc, "" }
	restore := RealOwnSaveCheckForTests()
	defer restore()
	if g := UnattendedStateFor(t.TempDir()+"/copy.db", true); g.Blocked && g.Mode == "game_running" {
		t.Fatalf("a copy waited for the game: %+v", g)
	}
}
