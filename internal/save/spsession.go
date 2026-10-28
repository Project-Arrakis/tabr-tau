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

// While a game process exists, a write to the save is allowed only when the evidence below says the game is at its menu with the save not loaded (#449; #419 had refused
// whatever the evidence said, after a write at the character-selection screen 23 s after the player left the world). The session is read from the game's own log
// (Saved/Logs/DuneSandbox.log), whose markers were checked against 11 real logs (issue #65) and, for the save markers, against 10 leave-the-world events in 11 logs (#449):
//
//	start  LogNet: Display: Browse (net mode 0|2): /Game/.../Survival_1?listen        (a local URL ending in ?listen)
//	end    Browse (net mode N): ...?closed, or a remote  ip:port/...  target          (menu, multiplayer login)
//	       Delaying Travel (net mode Listen Server): ?closed?delayed_travel
//	       FPlatformMisc::RequestExit(  /  Log file closed                            (the game is quitting)
//	save   LogLowLevelPersistence: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds.
//
// A local map without ?listen (a challenge room, the character cave) does not change the state. What the logs show about the save markers: while a world is loaded the game
// writes the database about every 20 s (the line says "Saving again in 60 seconds"; the measured gaps of 4,900 writes are 20.1 s to 22.7 s). After the end marker there was exactly
// one more write, between 1.5 s and 6.9 s later (19 exits), and then none until the next session began (menu stays of 41 s to 381 s, and up to 760 s before a quit), when the log
// shows "Using persistence implementation Sqlite" and "ApplyPatches" and an immediate write: the game opens the database when a session starts, not at the menu. What the logs do not show is a line saying the database was closed or unloaded, nor whether the game keeps a handle on
// game.db at the menu; the rule below therefore uses the silence of the save markers, not a close marker.

const (
	// dbQuietFor is how long after the game's last write of the database the editor still treats it as loaded, when the log does not show both the end of a session and the
	// game's last write after it (#470 gives that case the shorter menuWriteGrace). The game writes about every 20 s while it holds a world (the log lines say 60 s; measured
	// 20.1 s median), so three minutes is many missed intervals. The margin itself is a choice, not a measurement; it also keeps the #419 case queued.
	dbQuietFor = 3 * time.Minute
	// endedFallbackGrace is used only when the log shows a session that ended but no write after the end marker (the line this rule relies on was not seen, so the
	// evidence is missing): wait this long after the end marker. NOT MEASURED. It is as long as it is because nothing observed says how long the game may still hold the
	// save's rows after leaving the world, the incident of #419 shows that seconds are too short, and a wrong guess costs a few minutes of waiting where a wrong guess the
	// other way costs items that cannot be moved. Six minutes is longer than an earlier note's "about every five minutes" autosave; the logs say 60 s.
	endedFallbackGrace = 6 * time.Minute
	// menuWriteGrace is how long after the end marker the editor waits when the log shows BOTH the end of the session and the game's last write after it (#470). Measured in 19
	// exits from a world: that last write came 1.5 s to 6.9 s after the end marker, once, and no later write was seen at the menu (stays of up to 760 s). The grace is that
	// maximum (6.9 s) plus one save interval (the largest gap between two writes while a world is loaded, 22.7 s), 29.6 s, rounded up to 30 s.
	menuWriteGrace = 30 * time.Second
	// menuQuietAfterWrite is how long the game must also have been silent since that last write: one save interval, the largest gap seen between two writes (22.7 s) rounded up.
	// With the writes seen it never binds (the last write is 6.9 s after the end at most, so 30 s after the end is 23 s after it); it only holds the edit back when a write
	// comes later than anything measured.
	menuQuietAfterWrite = 23 * time.Second
)

// spTracker is the log state machine. It sees the log line by line and is the only place the markers live.
type spTracker struct {
	active     bool
	endedAt    time.Time // log time of the last end marker; zero when none was seen or a session is active
	lastSaveAt time.Time // log time of the last database write line; zero when none was seen
}

// feed applies one log line.
func (t *spTracker) feed(line string) {
	switch {
	case strings.Contains(line, "SaveDatabase - Database saved to disk"):
		t.lastSaveAt = logTime(line)
		if t.lastSaveAt.IsZero() { // no timestamp: count it as just written, which only makes the wait longer
			t.lastSaveAt = time.Now()
		}
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
	head   string // first bytes of the log: unique per game run, so a rotated log is noticed even when it is longer
	offset int64
	tr     spTracker
	rest   string // an unfinished last line
}

// spStatus is what the log says about the single-player session.
type spStatus struct {
	Active     bool
	EndedAt    time.Time
	LastSaveAt time.Time // the game's last write of the database, from its log; zero when the log shows none
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
	const maxLog = 256 << 20
	if st.Size() > maxLog {
		return spStatus{}, fmt.Errorf("the game log is larger than %d MiB", maxLog>>20)
	}
	hb := make([]byte, 160)
	hn, _ := f.ReadAt(hb, 0)
	head := string(hb[:hn])
	if lp != spLog.path || st.Size() < spLog.offset || head != spLog.head { // another file, a shorter one, or a new game run
		spLog.path, spLog.head, spLog.offset, spLog.tr, spLog.rest = lp, head, 0, spTracker{}, ""
	}
	if _, err := f.Seek(spLog.offset, io.SeekStart); err != nil {
		return spStatus{}, fmt.Errorf("cannot read the game log: %w", err)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxLog))
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
	return spStatus{Active: spLog.tr.active, EndedAt: spLog.tr.endedAt, LastSaveAt: spLog.tr.lastSaveAt}, nil
}

// GameState is the answer to "may the save be written now?".
type GameState struct {
	Blocked    bool     // Save must not run
	Reason     string   // why, in a sentence for the page (empty when saving is allowed)
	Mode       string   // none | menu | single_player | session_ended | game_running | unknown
	Processes  []string // Dune processes found (also set when saving is allowed: the automatic pass at launch waits for these to be gone)
	CheckError string   // set when we could not tell
	// NeedsConfirm is set when the only thing blocking the write is that the save's location cannot be vouched for (a network share, a removable
	// drive, a folder outside the game's own). ConfirmLocation lifts it.
	NeedsConfirm bool
}

// listProcs, statusFor and fileHeld are variables so tests can stand in for the machine.
var (
	listProcs = func(fresh bool) ([]string, string) {
		n := gameProcesses(!fresh) // fresh asks the process list again; the status poll (not fresh) may use the answer of the last gameCacheTTL
		gameCache.Lock()
		defer gameCache.Unlock()
		return n, gameCache.err
	}
	statusFor = singlePlayerStatus
	fileHeld  = saveFileHeld // gameproc_windows.go / gameproc_other.go
	nowFunc   = time.Now
)

// GameStateFor decides for the save at savePath. fresh bypasses the process cache (writes use it).
// Anything it cannot establish blocks the write: a guess must not decide.
func GameStateFor(savePath string, fresh bool) GameState {
	g := processState(savePath, fresh)
	if g.Blocked {
		return g
	}
	// The process list and the log are this computer's. For a save that may be used from elsewhere, "no game here" proves nothing.
	if why := locationFor(savePath); why != "" && !locationConfirmed(savePath) {
		return GameState{Blocked: true, NeedsConfirm: true, Mode: "unknown", Processes: g.Processes,
			Reason: why + ", so this computer cannot tell whether the game is using it (here or on another computer); confirm that the game is closed everywhere that can open this save to allow saving"}
	}
	return g
}

// processState is the part of GameStateFor that looks at the game's processes, log and the save file on this computer. With no Dune process the save may be written. With one,
// the save may be written only when ALL of these hold, checked in this order; the first that fails is the reason shown (#449):
//
//  1. the process list could be read (a list that cannot be read is not "no game");
//  2. the game's log could be read (without it the editor cannot tell whether a world is loaded);
//  3. the log shows no single-player session running;
//  4. the game has not written its database for dbQuietFor (the game writes it about every 20 s while a world is loaded: this is the "unloaded" signal) - EXCEPT when the log
//     shows the session ended and the game's last write after it (#470): then menuWriteGrace has passed since the end and menuQuietAfterWrite since that write, instead;
//  5. if a session ended and no write after it is in the log, endedFallbackGrace has passed since it ended (the unmeasured fallback);
//  6. no other program has the save file open, and that could be checked.
//
// Evidence: 2 to 5 are read from the game's own log (see the comment at the top of this file), 6 from the operating system. Not established: that the game does not keep game.db
// open at its menu in a way the check cannot see, and what the game does with rows it read when a session began (#419 suspected stale rows; the log shows the game opens the
// database again when the next session starts).
func processState(savePath string, fresh bool) GameState {
	procs, perr := listProcs(fresh)
	if perr != "" {
		return GameState{Blocked: true, Mode: "unknown", CheckError: perr, Reason: "could not check whether the game is running (" + perr + ")"}
	}
	if len(procs) == 0 {
		return GameState{Mode: "none"}
	}
	wait := func(mode, why string) GameState {
		return GameState{Blocked: true, Mode: mode, Processes: procs, Reason: why + "; edits stay queued until this clears, or the game closes"}
	}
	sp, serr := statusFor(savePath)
	if serr != nil {
		return wait("game_running", "the game is open and its log cannot be read ("+shortReason(serr)+"), so the editor cannot tell whether a world is loaded")
	}
	now := nowFunc().UTC()
	if sp.Active {
		return wait("single_player", "a single-player session is active, and the game rewrites the save about every 20 seconds while it runs")
	}
	endSeen := !sp.EndedAt.IsZero()
	// "After", not "not before": the log's times are whole seconds, and the periodic write that comes up to a second before the end marker can carry the same second as it. The
	// game's last write came 1.5 s or more after the marker in every exit measured, so it is always a later second; a tie is read as "not seen", which waits longer.
	if endSeen && sp.LastSaveAt.After(sp.EndedAt) {
		// The end of the session and the game's last write after it are both in the log (#470): the short, measured grace.
		wake := sp.EndedAt.Add(menuWriteGrace)
		if w := sp.LastSaveAt.Add(menuQuietAfterWrite); w.After(wake) {
			wake = w
		}
		if now.Before(wake) {
			return wait("session_ended", "the game wrote its database "+approxDuration(now.Sub(sp.LastSaveAt))+" ago, the last write after the single-player session ended "+approxDuration(now.Sub(sp.EndedAt))+" ago; the logs show it then stays silent, so the editor waits "+approxDuration(menuWriteGrace)+" after the end and "+approxDuration(menuQuietAfterWrite)+" after that write (about "+approxDuration(wake.Sub(now))+" more)")
		}
	} else {
		if !sp.LastSaveAt.IsZero() {
			if since := now.Sub(sp.LastSaveAt); since < dbQuietFor {
				return wait("session_ended", "the game wrote its database "+approxDuration(since)+" ago and does that about every 20 seconds while a world is loaded; it must be quiet for "+approxDuration(dbQuietFor)+" first (about "+approxDuration(dbQuietFor-since)+" more)")
			}
		}
		if endSeen {
			if since := now.Sub(sp.EndedAt); since < endedFallbackGrace {
				return wait("session_ended", "the single-player session ended "+approxDuration(since)+" ago and the log does not show the game's last write after it, so the editor waits "+approxDuration(endedFallbackGrace)+" after the end (about "+approxDuration(endedFallbackGrace-since)+" more)")
			}
		}
	}
	held, herr := fileHeld(savePath)
	if herr != nil {
		return wait("game_running", "the editor could not check whether another program has the save file open ("+shortReason(herr)+")")
	}
	if held {
		return wait("game_running", "another program, probably the game, has the save file open")
	}
	return GameState{Mode: "menu", Processes: procs}
}

// approxDuration says a wait in words: seconds under two minutes, else whole minutes (rounded up for a time still to wait).
func approxDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < 2*time.Minute {
		n := int(d.Round(time.Second) / time.Second)
		if n == 1 {
			return "1 second"
		}
		return fmt.Sprintf("%d seconds", n)
	}
	return fmt.Sprintf("%d minutes", int((d+59*time.Second)/time.Minute)) // at least 2 here: d is two minutes or more
}

// shortReason is an error in one line without the paths of the machine.
func shortReason(err error) string {
	s := err.Error()
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

// SaveBlocked is GameStateFor for the checks that gate a write.
func SaveBlocked(savePath string) (bool, string) {
	g := GameStateFor(savePath, true)
	return g.Blocked, g.Reason
}
