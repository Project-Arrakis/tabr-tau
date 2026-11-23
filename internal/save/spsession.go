package save

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Project-Arrakis/tabr-tau/kit/gamefolder"
	"github.com/Project-Arrakis/tabr-tau/kit/sessionlog"
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
// game.db at the menu; the rule below therefore uses the silence of the save markers, not a close marker, and since #612 also the save file's own modification time and the editor's observation of it, a second signal that does not depend on the text of the log (processState lists the whole rule).
//
// The markers and the transitions of the state machine are in the kit (kit/sessionlog, read here with the zero Policy: a marker counts wherever it stands in a line, and the write line is a marker), the log path in kit/gamefolder; the waits below are this editor's.

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
	// menuQuietAfterWrite is how long the game must also have been silent since that last write, and its save file unchanged (see processState): 60 s, a little under three
	// save intervals (the largest gap seen between two writes is 22.7 s). It was 23 s, 0.3 s above that gap, and never bound with the writes measured (#612, D2-2); now the
	// edit waits until 60 s after the last write, which is 61.5 s to 67 s after the end marker in the exits measured. Waiting costs a minute, a wrong guess costs the edit.
	menuQuietAfterWrite = time.Minute
	// maxWritesAfterEnd is how many database writes after the end marker still look like leaving a world: exactly one was seen in 19 exits. More than that means the marker
	// was not the end of the world (a local ?closed browse while the world stayed loaded, a map load without ?listen), so the short grace is not used and the flat quiet time
	// and the fallback wait apply (#612, D2-2).
	maxWritesAfterEnd = 1
	// maxWritesNearEnd and nearEndWindow are the second shape of leaving a world (#1122): the game's last periodic write (every 20 s while a world is loaded) can fall just after the end
	// marker as well as the write it makes when it leaves, so there are TWO writes, both within a few seconds of the marker. Measured in the logs of 2026-11-18 to 2026-11-23: 9 of
	// 10 exits had one write 1.8 s to 3.8 s after the marker, one had two (0.4 s and 3.6 s after it), and none had a write between 4 s and the start of the next session (see
	// docs/design/2026-10-27-menu-write-grace.md). Up to maxWritesNearEnd writes that all fall within nearEndWindow of the marker (the largest gap in the 19 exits of #470 was 6.9 s)
	// are the normal exit; a write later than that, or a third one, is not, and the flat quiet time and the fallback wait apply as before.
	maxWritesNearEnd = 2
	nearEndWindow    = 10 * time.Second
	// noMarkerQuietFor is how long the save file must have been unchanged when a game process exists and the log, though it can be read, shows no session line at all (no
	// start, no end, no write line): that is what a game that never loaded a world looks like, and also what a log looks like when a game update reworded the lines the
	// editor reads (#612, D2-1). The state is then unknown, and only the file's own time can say the game is idle: ten minutes, longer than any gap between the game's writes
	// while it holds a world (20 s in the log; the file's own interval is not measured here, see docs/design/2026-11-15-live-save-gate.md) and than the "about every five
	// minutes" autosave an older note mentions.
	noMarkerQuietFor = 10 * time.Minute
)

// spPolicy is how the editor reads the log: a marker counts wherever it stands in a line, and the line of the game's write of the database is a marker. The wide reading is
// kept for the START of a session and for the write line: a line that only contains the words and is taken for a start makes the editor wait, the safe direction. An END of
// a session read from a browse line is taken only under the strict reading (see spTracker.feed), because a line that merely contains the words must not end a session
// (#612, D2-3). The markers and the transitions are in kit/sessionlog; what the editor does with the end time and the last write (the waits above) stays here.
var spPolicy = sessionlog.Policy{}

// spStrictPolicy is the reading that counts a browse line only at the start of the line's text, after the logger's bracketed prefixes and the category.
var spStrictPolicy = sessionlog.Policy{Strict: true}

// The other three end markers, anchored to the start of the line's text (after the logger's bracketed prefixes), as the game writes them (#612, D2-3). Read anywhere in a line, a
// line that merely contained the words ended the session in the state machine, the unsafe direction. A real end line that does not look like these is a missed end, which only
// makes the editor wait longer.
const (
	anchorDelayed = "LogNet: Display: Delaying Travel (net mode Listen Server): ?closed"
	anchorExit    = "LogWindows: Log: FPlatformMisc::RequestExit("
	anchorClosed  = "Log: Log file closed"
)

// endAnchored says whether a line that the wide reading takes for a delayed close, an exit or the closing of the log is one at the start of its text.
func endAnchored(ev sessionlog.Event, line string) bool {
	b := sessionlog.Body(line)
	switch ev {
	case sessionlog.DelayedClose:
		return strings.HasPrefix(b, anchorDelayed)
	case sessionlog.Exit:
		return strings.HasPrefix(b, anchorExit) || strings.HasPrefix(b, anchorClosed)
	}
	return true
}

// spTracker is the log state machine of the kit (kit/sessionlog) read under the editor's policy: SPActive, EndedAt and LastSaveAt are the three answers the waits use. It
// also keeps how many database writes followed the end of the session.
type spTracker struct {
	sessionlog.Tracker
	writesAfterEnd int // database writes seen after the latest end marker; zero while a session is open
	writesNearEnd  int // how many of those fall within nearEndWindow of the end marker, by the log's own times (a write with no time stamp is not near)
}

// feed applies one log line. No end marker ends a session unless it stands at the start of the line's text; starts and the write line keep the wide reading (a spurious start
// or write only makes the editor wait).
func (t *spTracker) feed(line string) {
	p := spPolicy
	ev := sessionlog.Classify(p, line)
	// A browse line that ends a session under the wide reading but not under the strict one (the words stand in a line of some other category) is not an end: the line is read
	// under the strict policy instead, which still takes an exit or a closing of the log that stands in it, when that stands at the start of the text.
	if ev == sessionlog.Menu || ev == sessionlog.RemoteBrowse {
		if strict := sessionlog.Classify(spStrictPolicy, line); strict != ev {
			p, ev = spStrictPolicy, strict
		}
	}
	if !endAnchored(ev, line) {
		return // the words of an end marker inside some other line: not a marker, and (the wide reading puts it after the write and browse lines) no other marker is in it
	}
	switch {
	case ev == sessionlog.Start:
		t.writesAfterEnd, t.writesNearEnd = 0, 0
	case ev.Ends() && t.SPActive:
		t.writesAfterEnd, t.writesNearEnd = 0, 0
	case ev == sessionlog.DatabaseSaved && !t.EndedAt.IsZero():
		t.writesAfterEnd++
		if at := sessionlog.LogTime(line); !at.IsZero() && at.Sub(t.EndedAt) <= nearEndWindow {
			t.writesNearEnd++
		}
	}
	t.Tracker.Feed(p, line, nil)
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
	skip   bool   // the line in rest is past maxLogLine: the rest of it is dropped up to its end
}

// spStatus is what the log says about the single-player session.
type spStatus struct {
	Active     bool
	EndedAt    time.Time
	LastSaveAt time.Time // the game's last write of the database, from its log; zero when the log shows none
	// WritesAfterEnd is how many database writes the log shows after the latest end marker (one is what leaving a world looks like).
	WritesAfterEnd int
	// WritesNearEnd is how many of them fall within nearEndWindow of the end marker (#1122): two writes that both do are still the normal exit.
	WritesNearEnd int
}

// endWritesNormal says whether n database writes after the end marker, near of them within nearEndWindow of it, look like leaving a world: one write, whenever it came (#612), or
// up to maxWritesNearEnd writes that all fall within the window (#1122).
func endWritesNormal(n, near int) bool {
	return n <= maxWritesAfterEnd || (n <= maxWritesNearEnd && near == n)
}

// seen says whether the log currently asserts anything about a session: one is active, one ended, or the game wrote its database. None of these is a log that says nothing
// (#612, D2-1), which is not the same as a log that says no world is loaded. It is derived from what the log asserts now, not remembered from lines read before.
func (s spStatus) seen() bool { return s.Active || !s.EndedAt.IsZero() || !s.LastSaveAt.IsZero() }

// The log is read in pieces of logChunk bytes, whatever its size, so a long session (a game log grows about 70 MB an hour, #530) is never held in memory as a whole and never
// refused for its size. A line longer than maxLogLine is read as its first maxLogLine bytes (the time and the marker are at the start of a line); the log's own lines are far
// shorter. Variables so tests can use small values.
var (
	logChunk   = 4 << 20
	maxLogLine = 1 << 20
)

// absorb feeds the tracker with the complete lines in b, which follows what was read before; an unfinished last line waits in spLog.rest. The caller holds spLog's lock.
func absorb(b []byte) {
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		var seg []byte
		if i < 0 {
			seg = b
		} else {
			seg = b[:i]
		}
		if !spLog.skip { // a line past maxLogLine keeps its head only
			if room := maxLogLine - len(spLog.rest); len(seg) > room {
				seg = seg[:max(room, 0)]
				spLog.skip = true
			}
			spLog.rest += string(seg)
		}
		if i < 0 {
			return
		}
		spLog.tr.feed(strings.TrimRight(spLog.rest, "\r"))
		spLog.rest, spLog.skip = "", false
		b = b[i+1:]
	}
}

// singlePlayerStatus reads whatever the game appended to its log since the last call, in chunks, whatever the size of the file: only a log that cannot be read at all is an
// error (and then the edit is queued, see processState).
func singlePlayerStatus(savePath string) (spStatus, error) {
	lp, err := gamefolder.LogPathFor(savePath)
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
	hb := make([]byte, 160)
	hn, _ := f.ReadAt(hb, 0)
	head := string(hb[:hn])
	if lp != spLog.path || st.Size() < spLog.offset || head != spLog.head { // another file, a shorter one, or a new game run
		spLog.path, spLog.head, spLog.offset, spLog.tr, spLog.rest, spLog.skip = lp, head, 0, spTracker{}, "", false
	}
	if _, err := f.Seek(spLog.offset, io.SeekStart); err != nil {
		return spStatus{}, fmt.Errorf("cannot read the game log: %w", err)
	}
	// Up to the size seen above: what the game appends meanwhile is the next call's. Each chunk is applied before the next is read, so an error part-way keeps what was read
	// (the offset only moves over bytes that were fed) and the next call goes on from there.
	r := io.LimitReader(f, st.Size()-spLog.offset)
	buf := make([]byte, logChunk)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			spLog.offset += int64(n)
			absorb(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return spStatus{}, fmt.Errorf("cannot read the game log: %w", err)
		}
	}
	return spStatus{Active: spLog.tr.SPActive, EndedAt: spLog.tr.EndedAt, LastSaveAt: spLog.tr.LastSaveAt, WritesAfterEnd: spLog.tr.writesAfterEnd, WritesNearEnd: spLog.tr.writesNearEnd}, nil
}

// QueuedUntilClears ends the Reason of a game that may still hold the save's rows. A caller that words the wait for itself (the automatic pass at launch, #499) takes it off.
const QueuedUntilClears = "; they are written when this clears, or when the game closes"

// WrittenInMarker starts the end of the Reason of a wait that has a time left ("the game saved 21 seconds ago; they are written in about 39 seconds, or when the game closes", #1128).
const WrittenInMarker = "; they are written in about "

// WaitFact is the Reason of a blocked game without the sentence about the queue that ends it: what the editor waits for, in a few words (QueuedUntilClears, or the part from
// WrittenInMarker, is taken off). A caller that words the wait for itself (the automatic pass) uses it.
func WaitFact(reason string) string {
	if r, ok := strings.CutSuffix(reason, QueuedUntilClears); ok {
		return r
	}
	if r, _, ok := strings.Cut(reason, WrittenInMarker); ok {
		return r
	}
	return reason
}

// GameState is the answer to "may the save be written now?".
type GameState struct {
	Blocked    bool     // Save must not run
	Reason     string   // why, in a short sentence for the page (empty when saving is allowed): what is waited for and, when it is known, how long (#1128)
	Detail     string   // the technical account behind Reason (the last write, the end time, the grace values), for the collapsed Details of the banner; empty when Reason says it all
	Mode       string   // none | menu | unknown_quiet | single_player | session_ended | game_running | unknown (unknown_quiet: allowed, but the log said nothing about a session)
	Processes  []string // Dune processes found (also set when saving is allowed)
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
	// A file the game could not have loaded is not protected by the game's state (#498): no process list, no log, no wait. The location rule below still applies to it.
	if !IsGamesOwnSave(savePath) {
		return copyState(savePath)
	}
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

// UnattendedStateFor is GameStateFor for a write that nobody is watching: the automatic pass at launch and its retries (#612, G-007). An edit made by hand has a person who
// sees the banner and can stop; the pass has not, and the one input that decides "a world is loaded" is text in the game's log. So the pass does not rely on the log alone:
// while any game process exists it waits, whatever the log and the file say, and runs once the game is closed (as before #499). With no process the answer is GameStateFor's.
// A save that is not the game's own has no game process to ask and is answered as GameStateFor does.
func UnattendedStateFor(savePath string, fresh bool) GameState {
	g := GameStateFor(savePath, fresh)
	if g.Blocked || len(g.Processes) == 0 {
		return g
	}
	return GameState{Blocked: true, Mode: "game_running", Processes: g.Processes,
		Reason: "the game is open, so the automatic pass waits until the game is closed" + QueuedUntilClears,
		Detail: "the game is open, and the automatic pass runs without anyone watching, so it waits until the game is closed (it does not rely on the game's log alone to tell that no world is loaded)" + QueuedUntilClears}
}

// copyState is GameStateFor for a save that is not the game's own (#498): the game's processes, log and handle are not asked, because the game does not open this file, so
// edits are written at once. What stays is the location rule for a place other programs or computers may write (a network share, a removable drive); the folder rule
// ("not the game's own folder") is the point of a copy and is not applied.
func copyState(savePath string) GameState {
	why := locationFor(savePath)
	if why == notInGameFolderWhy {
		why = copyDriveProblem(savePath)
	}
	if why != "" && !locationConfirmed(savePath) {
		return GameState{Blocked: true, NeedsConfirm: true, Mode: "unknown",
			Reason: why + ", so the editor cannot be sure that no other program or computer is writing this file; confirm that nothing else is using it to allow saving"}
	}
	return GameState{Mode: "not_live"}
}

// processState is the part of GameStateFor that looks at the game's processes, log and the save file on this computer. With no Dune process the save may be written. With one,
// the save may be written only when ALL of these hold, checked in this order; the first that fails is the reason shown (#449, #612):
//
//  1. the process list could be read (a list that cannot be read is not "no game");
//  2. the game's log could be read (without it the editor cannot tell whether a world is loaded);
//  3. the log shows no single-player session running;
//  4. the game has not written its database for dbQuietFor (the game writes it about every 20 s while a world is loaded: this is the "unloaded" signal) - EXCEPT when the log
//     shows the session ended and the game's last write after it (#470): then menuWriteGrace has passed since the end and menuQuietAfterWrite since that write, instead;
//  5. if a session ended and no write after it is in the log, endedFallbackGrace has passed since it ended (the unmeasured fallback);
//  6. the save file's own modification time is older than the wait the log asked for (fileQuiet: 10 minutes when the log asserts nothing about a session at all, that is, the
//     state is unknown), unless the file is the one the editor itself installed (#612);
//  7. the editor has watched the file without a gap for observeQuietFor (a first evaluation, a sleep or a clock step starts the watch again) (#612);
//  8. no other program has the save file open, and that could be checked.
//
// A write allowed while the log asserted nothing is answered with the mode unknown_quiet, not menu. Evidence: 2 to 5 are read from the game's own log (see the comment at the
// top of this file), 6 and 7 from the file's time and the editor's clock, 8 from the operating system. Not established: that the game does not keep game.db
// open at its menu in a way the check cannot see, and what the game does with rows it read when a session began (#419 suspected stale rows; the log shows the game opens the
// database again when the next session starts).
func processState(savePath string, fresh bool) GameState {
	procs, perr := listProcs(fresh)
	if perr != "" {
		return GameState{Blocked: true, Mode: "unknown", CheckError: perr, Reason: "could not check whether the game is running (" + perr + ")"}
	}
	now := nowFunc().UTC()
	if len(procs) == 0 {
		forgetObserved(savePath)
		return GameState{Mode: "none"}
	}
	wait := func(mode, why string) GameState {
		return GameState{Blocked: true, Mode: mode, Processes: procs, Reason: why + QueuedUntilClears}
	}
	// waitD is a wait with no time to name: the short reason, and the technical account (detail) for the banner's Details (#1128).
	waitD := func(mode, why, detail string) GameState {
		g := wait(mode, why)
		g.Detail = detail + QueuedUntilClears
		return g
	}
	// waitFor is a wait whose end the editor can name: "<what it waits for>; they are written in about <time>, or when the game closes" (#1128), with the technical account in Detail.
	waitFor := func(mode, fact string, more time.Duration, detail string) GameState {
		return GameState{Blocked: true, Mode: mode, Processes: procs, Reason: fact + WrittenInMarker + approxDuration(more) + ", or when the game closes", Detail: detail + QueuedUntilClears}
	}
	// The save file's time is read once, at every evaluation with a game process (also when the log already blocks), so the editor can tell whether it has been watching the
	// file without a gap (observeFile, #612).
	mod, own, terr := fileTimeFor(savePath)
	watchedFrom := observeFile(savePath, now, mod, own, terr)
	sp, serr := statusFor(savePath)
	if serr != nil {
		return waitD("game_running", "the game is open and its log cannot be read ("+shortReason(serr)+")", "the game is open and its log cannot be read ("+shortReason(serr)+"), so the editor cannot tell whether a world is loaded")
	}
	if sp.Active {
		return waitD("single_player", "a single-player session is active", "a single-player session is active, and the game rewrites the save about every 20 seconds while it runs")
	}
	endSeen := !sp.EndedAt.IsZero()
	// "After", not "not before": the log's times are whole seconds, and the periodic write that comes up to a second before the end marker can carry the same second as it. The
	// game's last write came 1.5 s or more after the marker in every exit measured, so it is always a later second; a tie is read as "not seen", which waits longer. Writes beyond
	// what endWritesNormal allows (one, or two within nearEndWindow of the marker, #1122) are not what leaving a world looks like: the marker is then not trusted (#612, D2-2).
	cleanEnd := endSeen && sp.LastSaveAt.After(sp.EndedAt) && endWritesNormal(sp.WritesAfterEnd, sp.WritesNearEnd)
	fileQuiet := dbQuietFor // how long the save file must have been unchanged, whatever the log said (the second signal, #612 D2-1)
	switch {
	case !sp.seen():
		fileQuiet = noMarkerQuietFor
	case cleanEnd:
		fileQuiet = menuQuietAfterWrite
	}
	if cleanEnd {
		// The end of the session and the game's last write after it are both in the log (#470): the short, measured grace.
		wake := sp.EndedAt.Add(menuWriteGrace)
		if w := sp.LastSaveAt.Add(menuQuietAfterWrite); w.After(wake) {
			wake = w
		}
		if now.Before(wake) {
			return waitFor("session_ended", "the game saved "+approxDuration(now.Sub(sp.LastSaveAt))+" ago", wake.Sub(now),
				"the game wrote its database "+approxDuration(now.Sub(sp.LastSaveAt))+" ago, the last write after the single-player session ended "+approxDuration(now.Sub(sp.EndedAt))+" ago; the logs show it then stays silent, so the editor waits "+approxDuration(menuWriteGrace)+" after the end and "+approxDuration(menuQuietAfterWrite)+" after that write (about "+approxDuration(wake.Sub(now))+" more)")
		}
	} else {
		if !sp.LastSaveAt.IsZero() {
			if since := now.Sub(sp.LastSaveAt); since < dbQuietFor {
				return waitFor("session_ended", "the game saved "+approxDuration(since)+" ago", dbQuietFor-since,
					"the game wrote its database "+approxDuration(since)+" ago and does that about every 20 seconds while a world is loaded; it must be quiet for "+approxDuration(dbQuietFor)+" first (about "+approxDuration(dbQuietFor-since)+" more)")
			}
		}
		if endSeen {
			if since := now.Sub(sp.EndedAt); since < endedFallbackGrace {
				if !endWritesNormal(sp.WritesAfterEnd, sp.WritesNearEnd) {
					return waitFor("session_ended", "you left the world "+approxDuration(since)+" ago and the game saved "+fmt.Sprint(sp.WritesAfterEnd)+" times after that", endedFallbackGrace-since,
						"the single-player session ended "+approxDuration(since)+" ago but the game wrote its database "+fmt.Sprint(sp.WritesAfterEnd)+" times after that, which is not what leaving a world looks like, so the editor waits "+approxDuration(endedFallbackGrace)+" after the end (about "+approxDuration(endedFallbackGrace-since)+" more)")
				}
				return waitFor("session_ended", "you left the world "+approxDuration(since)+" ago", endedFallbackGrace-since,
					"the single-player session ended "+approxDuration(since)+" ago and the log does not show the game's last write after it, so the editor waits "+approxDuration(endedFallbackGrace)+" after the end (about "+approxDuration(endedFallbackGrace-since)+" more)")
			}
		}
	}
	// The second signal, independent of the log's text (#612, D2-1): the save file's own modification time. The log can say "no session" for a reason that is not true (a game
	// update that rewords the lines read above), the file cannot be reworded: a game that holds a world rewrites it, and the editor's own writes are told apart (ownWrite). With
	// no session line in the log at all the state is unknown, and the file must have been quiet much longer. A time that cannot be read is not "quiet".
	switch {
	case terr != nil:
		return waitD("game_running", "the editor could not read the modification time of the save file ("+shortReason(terr)+")", "the editor could not read the modification time of the save file ("+shortReason(terr)+"), so it cannot tell whether the game is still writing it")
	case own: // the file is the one the editor wrote itself: its time says nothing about the game
	case now.Sub(mod) < fileQuiet:
		if !sp.seen() {
			return waitFor("game_running", "the save file changed "+approxDuration(now.Sub(mod))+" ago", fileQuiet-now.Sub(mod),
				"the game is open, its log shows no sign of a session (nothing to say whether a world is loaded), and the save file changed "+approxDuration(now.Sub(mod))+" ago; the editor waits until the file has been quiet for "+approxDuration(fileQuiet)+" (about "+approxDuration(fileQuiet-now.Sub(mod))+" more)")
		}
		return waitFor("session_ended", "the save file changed "+approxDuration(now.Sub(mod))+" ago", fileQuiet-now.Sub(mod),
			"the save file changed "+approxDuration(now.Sub(mod))+" ago, whatever the log says; the editor waits until it has been quiet for "+approxDuration(fileQuiet)+" (about "+approxDuration(fileQuiet-now.Sub(mod))+" more)")
	}
	// The quiet must have been OBSERVED (#612): the editor's clock and the file's time are the whole of the second signal, and a computer that slept, or a clock that was set
	// forward, makes an old file time look like a long quiet. When the editor has not evaluated the gate with a game process for more than observeGap (the first evaluation, a
	// sleep, a clock step), or the file changed since it last looked, it holds for observeQuietFor and sees the file unchanged across it.
	if observeQuietFor > 0 && now.Sub(watchedFrom) < observeQuietFor {
		return waitFor("game_running", "the editor has only just started watching the save file", observeQuietFor-now.Sub(watchedFrom),
			"the editor has only just started watching the save file with the game open, so it cannot yet tell that the file is quiet (it waits "+approxDuration(observeQuietFor)+" and sees the file unchanged; about "+approxDuration(observeQuietFor-now.Sub(watchedFrom))+" more)")
	}
	held, herr := fileHeld(savePath)
	if herr != nil {
		return waitD("game_running", "the editor could not check whether another program has the save file open ("+shortReason(herr)+")", "the editor could not check whether another program has the save file open ("+shortReason(herr)+"), so it cannot tell whether the game still holds it")
	}
	if held {
		return wait("game_running", "another program, probably the game, has the save file open")
	}
	if !sp.seen() {
		// Allowed, but not because the log said the game is at its menu: it said nothing (#612). Its own mode, so no text says "at its menu".
		return GameState{Mode: "unknown_quiet", Processes: procs}
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

// shortReason is an error in one line without the folders of any path in it (the page shows the reason, and a folder can hold the name of the person's account), cut to 160 bytes at a character boundary.
func shortReason(err error) string {
	s := shortErr(err)
	if len(s) > 160 {
		i := 160
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
		s = s[:i]
	}
	return s
}

// SaveBlocked is GameStateFor for the checks that gate a write.
func SaveBlocked(savePath string) (bool, string) {
	g := GameStateFor(savePath, true)
	return g.Blocked, g.Reason
}
