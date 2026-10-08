package save

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The game process stays alive in multiplayer and at the menu; only an active single-player session holds the
// save (it rewrites game.db about every five minutes while it runs, and once more when the session ends).
// So Save is blocked only while a single-player session is active. The session is read from the game's own log
// (Saved/Logs/DuneSandbox.log), whose markers were checked against 11 real logs (issue #65):
//
//	start  LogNet: Display: Browse (net mode 0|2): /Game/.../Survival_1?listen        (a local URL ending in ?listen)
//	end    Browse (net mode N): ...?closed, or a remote  ip:port/...  target          (menu, multiplayer login)
//	       Delaying Travel (net mode Listen Server): ?closed?delayed_travel
//	       FPlatformMisc::RequestExit(  /  Log file closed                            (the game is quitting)
//
// The game writes the save for the last time about 3 s after the end marker, so Save stays blocked for spEndGrace
// after it. A local map without ?listen (a challenge room, the character cave) does not change the state.

const spEndGrace = 15 * time.Second

// spTracker is the log state machine. It sees the log line by line and is the only place the markers live.
type spTracker struct {
	active  bool
	endedAt time.Time // log time of the last end marker; zero when none was seen or a session is active
}

// feed applies one log line.
func (t *spTracker) feed(line string) {
	switch {
	case strings.Contains(line, "Browse (net mode"):
		target := line[strings.Index(line, "Browse (net mode"):]
		if i := strings.Index(target, "): "); i >= 0 {
			target = target[i+3:]
		}
		switch {
		case strings.Contains(target, "?closed") || !strings.HasPrefix(target, "/"):
			t.end(line)
		case strings.Contains(target, "?listen"):
			t.active, t.endedAt = true, time.Time{}
		}
	case strings.Contains(line, "Delaying Travel (net mode Listen Server): ?closed"),
		strings.Contains(line, "FPlatformMisc::RequestExit("),
		strings.Contains(line, "Log file closed"):
		t.end(line)
	}
}

func (t *spTracker) end(line string) {
	if !t.active {
		return
	}
	t.active = false
	t.endedAt = logTime(line)
	if t.endedAt.IsZero() { // no timestamp: count it as just ended, which only makes the wait longer
		t.endedAt = time.Now()
	}
}

// logTime parses the "[2026.10.08-17.10.51:686]" prefix (UTC); zero when absent.
func logTime(line string) time.Time {
	if len(line) < 25 || line[0] != '[' {
		return time.Time{}
	}
	tm, err := time.Parse("2006.01.02-15.04.05", line[1:20])
	if err != nil {
		return time.Time{}
	}
	return tm
}

// spLog follows one log file and keeps the tracker between reads, so the 15 MB log is read once and then only
// what the game appended.
var spLog struct {
	sync.Mutex
	path   string
	offset int64
	tr     spTracker
	rest   string // an unfinished last line
}

// spStatus is what the log says about the single-player session.
type spStatus struct {
	Active  bool
	EndedAt time.Time
}

// logPathFor derives Saved/Logs/DuneSandbox.log from the standard save location
// (Saved/Cloud/PlayerClientStorage/FLS_retail/<id>/game.db). A save kept anywhere else has no known log.
func logPathFor(savePath string) (string, error) {
	d := filepath.Dir(savePath)
	for i := 0; i < 4; i++ {
		d = filepath.Dir(d)
	}
	if !strings.EqualFold(filepath.Base(d), "Saved") {
		return "", fmt.Errorf("the save is not in the game's standard folder, so its log cannot be found")
	}
	return filepath.Join(d, "Logs", "DuneSandbox.log"), nil
}

// singlePlayerStatus reads whatever the game appended to its log since the last call.
func singlePlayerStatus(savePath string) (spStatus, error) {
	lp, err := logPathFor(savePath)
	if err != nil {
		return spStatus{}, err
	}
	spLog.Lock()
	defer spLog.Unlock()
	f, err := os.Open(lp)
	if err != nil {
		return spStatus{}, fmt.Errorf("cannot read the game log: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return spStatus{}, fmt.Errorf("cannot read the game log: %w", err)
	}
	if lp != spLog.path || st.Size() < spLog.offset { // another file, or the game started a new log
		spLog.path, spLog.offset, spLog.tr, spLog.rest = lp, 0, spTracker{}, ""
	}
	if _, err := f.Seek(spLog.offset, io.SeekStart); err != nil {
		return spStatus{}, fmt.Errorf("cannot read the game log: %w", err)
	}
	b, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		return spStatus{}, fmt.Errorf("cannot read the game log: %w", err)
	}
	spLog.offset += int64(len(b))
	text := spLog.rest + string(b)
	nl := strings.LastIndexByte(text, '\n')
	if nl < 0 {
		spLog.rest = text
	} else {
		spLog.rest = text[nl+1:]
		for _, line := range strings.Split(text[:nl], "\n") {
			spLog.tr.feed(strings.TrimRight(line, "\r"))
		}
	}
	return spStatus{Active: spLog.tr.active, EndedAt: spLog.tr.endedAt}, nil
}

// GameState is the answer to "may the save be written now?".
type GameState struct {
	Blocked    bool     // Save must not run
	Reason     string   // why, in a sentence (also set when the game is open but saving is allowed)
	Mode       string   // none | menu_or_multiplayer | single_player | unknown
	Processes  []string // Dune processes found
	CheckError string   // set when we could not tell
}

// listProcs and statusFor are variables so tests can stand in for the machine.
var (
	listProcs = func(fresh bool) ([]string, string) {
		n := gameProcesses(fresh)
		gameCache.Lock()
		defer gameCache.Unlock()
		return n, gameCache.err
	}
	statusFor = singlePlayerStatus
	nowFunc   = time.Now
)

// GameStateFor decides for the save at savePath. fresh bypasses the three-second process cache (writes use it).
// Anything it cannot establish blocks the write: a guess must not decide.
func GameStateFor(savePath string, fresh bool) GameState {
	procs, perr := listProcs(fresh)
	if perr != "" {
		return GameState{Blocked: true, Mode: "unknown", CheckError: perr, Reason: "could not check whether the game is running (" + perr + ")"}
	}
	if len(procs) == 0 {
		return GameState{Mode: "none"}
	}
	names := strings.Join(procs, ", ")
	sp, err := statusFor(savePath)
	if err != nil {
		return GameState{Blocked: true, Mode: "unknown", Processes: procs, CheckError: err.Error(),
			Reason: names + " is running and the editor cannot tell whether a single-player session is active (" + err.Error() + "); quit the game to save"}
	}
	if sp.Active {
		return GameState{Blocked: true, Mode: "single_player", Processes: procs,
			Reason: "a single-player session is active (" + names + "); go to the menu or multiplayer, or quit the game, then save"}
	}
	if !sp.EndedAt.IsZero() && nowFunc().UTC().Sub(sp.EndedAt) < spEndGrace {
		return GameState{Blocked: true, Mode: "single_player", Processes: procs,
			Reason: "the single-player session just ended; wait a few seconds for the game to finish writing the save"}
	}
	return GameState{Mode: "menu_or_multiplayer", Processes: procs, Reason: names + " is running but not in single-player; saving is allowed"}
}

// SaveBlocked is GameStateFor for the checks that gate a write.
func SaveBlocked(savePath string) (bool, string) {
	g := GameStateFor(savePath, true)
	return g.Blocked, g.Reason
}
