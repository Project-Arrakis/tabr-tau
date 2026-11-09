// Package sessionlog reads the lines of the game's log that mark a single-player session, as a small state machine over lines of text.
//
// The game writes its log (Saved\Logs\DuneSandbox.log, see package gamefolder) while it runs. A few lines of it say that a world was
// loaded, that the player left it, or that the game wrote its database:
//
//	start  LogNet: Display: Browse (net mode 0|2): /Game/.../Survival_1?listen      (a local URL ending in ?listen)
//	end    LogNet: Display: Browse (net mode N): ...?closed                          (the menu)
//	       LogNet: Display: Browse (net mode N): ip:port/...                         (a remote target: a multiplayer login)
//	       LogNet: Display: Delaying Travel (net mode Listen Server): ?closed?delayed_travel
//	       FPlatformMisc::RequestExit(    /    Log file closed                       (the game is quitting)
//	save   LogLowLevelPersistence: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds.
//
// A local map without ?listen (a challenge room, the character cave) changes nothing, and neither does a delayed travel to a map.
//
// The package classifies one line ([Classify]), applies the event to a state ([State.Apply]) and, with the time of the lines, keeps
// the time of the end and of the last write of the database ([Tracker]). It does not read a file, look at a process or decide what a
// program does with the answer: how long to wait after a write, or what to refuse, stays in the program. Every function is free of
// state and of any dependency but the standard library's strings and time.
package sessionlog

import (
	"strings"
	"time"
)

// Event is what one line of the log means for a session.
type Event uint8

const (
	// NoEvent is a line that is none of the markers, or a browse line that changes nothing (a local map without ?listen).
	NoEvent Event = iota
	// Start is a browse to a local URL that ends in ?listen: a single-player session begins.
	Start
	// Menu is a browse to a local URL with ?closed: the player is at the menu.
	Menu
	// RemoteBrowse is a browse to a target that is not a local URL (it does not begin with "/"): a multiplayer login.
	RemoteBrowse
	// DelayedClose is a "Delaying Travel" line of the listen server that says ?closed: the session is being closed.
	DelayedClose
	// Exit is "FPlatformMisc::RequestExit(" or "Log file closed": the game is quitting.
	Exit
	// DatabaseSaved is the game's line about a write of its database. It is recognised only when the [Policy] allows it.
	DatabaseSaved
)

// String is the event in words, as a program can show it ("" for NoEvent).
func (e Event) String() string {
	switch e {
	case Start:
		return "local ?listen (single-player start)"
	case Menu:
		return "local ?closed (menu)"
	case RemoteBrowse:
		return "remote browse"
	case DelayedClose:
		return "delayed travel ?closed"
	case Exit:
		return "game exiting"
	case DatabaseSaved:
		return "database saved"
	}
	return ""
}

// Ends says whether the event ends a session: Menu, RemoteBrowse, DelayedClose and Exit.
func (e Event) Ends() bool {
	return e == Menu || e == RemoteBrowse || e == DelayedClose || e == Exit
}

// Policy is where two programs that read the same log differ. The zero value is the wide reading: a marker counts wherever it stands in
// the line, and the database write line is a marker.
type Policy struct {
	// Strict makes the browse marker count only at the start of the line's text: after the bracketed prefixes of the logger (time,
	// frame, thread) comes the category ("LogNet: Display: "), and then "Browse (net mode". A line that only contains the words, after
	// some other category, is not a browse line. The end markers other than the browse line (the delayed travel, the exit and the closing
	// of the log) count anywhere in the line either way.
	Strict bool
	// IgnoreSaves leaves the database write line out of the markers: it is not classified as DatabaseSaved, and a line that has the text
	// in it is read as whatever else it is.
	IgnoreSaves bool
}

const (
	markSave      = "SaveDatabase - Database saved to disk"
	markBrowse    = "Browse (net mode"
	markBrowseCat = "LogNet: Display: " + markBrowse
	markDelayed   = "Delaying Travel (net mode Listen Server): ?closed"
	markExit      = "FPlatformMisc::RequestExit("
	markClosed    = "Log file closed"
)

// Body is a log line without the bracketed prefixes of the logger ("[2026.10.08-17.57.43:218][ 14][1][0]"); the category follows them. A
// bracket group that does not close within 40 characters is not a prefix, and the line is returned from there on.
func Body(line string) string {
	for strings.HasPrefix(line, "[") {
		i := strings.IndexByte(line, ']')
		if i < 0 || i > 40 {
			break
		}
		line = line[i+1:]
	}
	return line
}

// Classify says what one log line (without its line end) means for a session under the policy. The checks run in a fixed order and the
// first that matches decides, so a line that holds two markers is the first of them: the database write, then the browse line, then the
// delayed travel, then the exit and the closing of the log.
func Classify(p Policy, line string) Event {
	if !p.IgnoreSaves && strings.Contains(line, markSave) {
		return DatabaseSaved
	}
	if p.Strict {
		if b := Body(line); strings.HasPrefix(b, markBrowseCat) {
			return browse(b[len(markBrowseCat)-len("(net mode"):])
		}
	} else if i := strings.Index(line, markBrowse); i >= 0 {
		return browse(line[i:])
	}
	switch {
	case strings.Contains(line, markDelayed):
		return DelayedClose
	case strings.Contains(line, markExit), strings.Contains(line, markClosed):
		return Exit
	}
	return NoEvent
}

// browse classifies a browse line from "(net mode" or "Browse (net mode" on: the target is what follows the first "): ".
func browse(target string) Event {
	if i := strings.Index(target, "): "); i >= 0 {
		target = target[i+3:]
	}
	switch {
	case !strings.HasPrefix(target, "/"):
		return RemoteBrowse
	case strings.Contains(target, "?closed"):
		return Menu
	case strings.Contains(target, "?listen"):
		return Start
	}
	return NoEvent
}

// State is what the session lines of a log say so far. It holds no time, so two states compare with ==.
type State struct {
	SPActive bool  // the latest session marker is a local ?listen browse
	Remote   bool  // the latest browse target was remote (a local browse clears it)
	Ended    bool  // an Exit was seen; nothing clears it, because a log is one run of the game
	Last     Event // the latest event that changed the state; NoEvent when none did
}

// Apply takes one event into the state. A Start sets SPActive and clears Remote. A Menu clears both. A RemoteBrowse sets Remote and
// clears SPActive. A DelayedClose clears SPActive and leaves Remote as it was. An Exit clears SPActive and sets Ended. NoEvent and
// DatabaseSaved change nothing.
func (s *State) Apply(ev Event) {
	switch ev {
	case Start:
		s.Remote, s.SPActive = false, true
	case Menu:
		s.Remote, s.SPActive = false, false
	case RemoteBrowse:
		s.Remote, s.SPActive = true, false
	case DelayedClose:
		s.SPActive = false
	case Exit:
		s.SPActive, s.Ended = false, true
	default:
		return
	}
	s.Last = ev
}

// Tracker is a State that also keeps the log time of the end of the session and of the last write of the database.
type Tracker struct {
	State
	// EndedAt is the log time of the latest end of a session that was open; zero when none ended or a session is open again. An end with
	// no session open records nothing, and a repeated end keeps the first time.
	EndedAt time.Time
	// LastSaveAt is the log time of the latest database write line; zero when there was none.
	LastSaveAt time.Time
}

// Feed classifies one line under the policy and takes it into the tracker. A line without a time stamp that needs one is stamped with
// now() (time.Now when now is nil): counted as having just happened, which for a program that waits after these events only makes the
// wait longer.
func (t *Tracker) Feed(p Policy, line string, now func() time.Time) {
	ev := Classify(p, line)
	switch {
	case ev == DatabaseSaved:
		t.LastSaveAt = stamp(line, now)
	case ev == Start:
		t.EndedAt = time.Time{}
	case ev.Ends() && t.SPActive:
		t.EndedAt = stamp(line, now)
	}
	t.State.Apply(ev)
}

func stamp(line string, now func() time.Time) time.Time {
	if at := LogTime(line); !at.IsZero() {
		return at
	}
	if now != nil {
		return now()
	}
	return time.Now()
}

// LogTime parses the "[2026.10.08-17.10.51:686]" prefix of a log line (UTC); zero when the line has none.
func LogTime(line string) time.Time {
	if len(line) < 25 || line[0] != '[' {
		return time.Time{}
	}
	tm, err := time.Parse("2006.01.02-15.04.05", line[1:20])
	if err != nil {
		return time.Time{}
	}
	return tm
}
