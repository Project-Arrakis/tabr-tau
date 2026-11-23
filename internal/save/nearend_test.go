package save

import (
	"strings"
	"testing"
	"time"
)

// #1122: leaving a world can show TWO database writes within a few seconds of the end marker (the game's periodic write that falls just after it, and the write it makes when it
// leaves). The log of 2026-11-22 00:14 UTC has exactly that. Every log line here is synthetic, with the timings of that exit; every file is a temp file.

const (
	neStart   = "[2026.11.22-00.10.00:000][  1][1]LogNet: Display: Browse (net mode 0): /Game/Maps/Test_1?listen"
	neWrite   = "[2026.11.22-00.14.02:400][  2][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	neEnd     = "[2026.11.22-00.14.22:500][  3][1]LogNet: Display: Delaying Travel (net mode Listen Server): ?closed?delayed_travel"
	neWrite1  = "[2026.11.22-00.14.22:900][  4][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	neBrowse  = "[2026.11.22-00.14.24:700][  5][1]LogNet: Display: Browse (net mode 2): /Game/Maps/Menu_1?closed"
	neWrite2  = "[2026.11.22-00.14.26:100][  6][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	neWrite3  = "[2026.11.22-00.14.30:100][  6][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	neWrite4  = "[2026.11.22-00.14.32:100][  6][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	neLate    = "[2026.11.22-00.15.22:100][  7][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	neAtMenu  = "[2026.11.22-00.20.00:000][  8][1]LogLowLevelPersistence: Display: DB(game.db:0): SaveDatabase - Database saved to disk. Saving again in 60 seconds."
	endAt     = "2026-11-22T00:14:22Z" // the end marker, to the second
	oldFile   = "2000-01-01T00:00:00Z"
	neNowZone = time.RFC3339
)

// stateAt is what the gate says when the clock is d after the end marker and the log holds the lines; the save file's own time is far in the past, so only the log decides.
func stateAt(t *testing.T, d time.Duration, lines ...string) GameState {
	t.Helper()
	gateStubs(t)
	old, _ := time.Parse(neNowZone, oldFile)
	fileTimeFor = func(string) (time.Time, bool, error) { return old, false, nil }
	end, _ := time.Parse(neNowZone, endAt)
	nowFunc = func() time.Time { return end.Add(d) }
	save, _ := writeLog(t, strings.Join(lines, "\n")+"\n")
	return processState(save, true)
}

// The exact sequence of the report: the end marker, a write 0.4 s later, the browse line, a write 3.6 s after the marker, then silence. The 30 s grace applies again: held a
// moment after the exit, written once 60 s have passed since the last write (63.6 s after the marker). The fallback (3 minutes after the last write) would still hold it then.
func TestTwoWritesNearTheEndAreTheNormalExit(t *testing.T) {
	lines := []string{neStart, neWrite, neEnd, neWrite1, neBrowse, neWrite2}
	if g := stateAt(t, 30*time.Second, lines...); !g.Blocked || g.Mode != "session_ended" {
		t.Fatalf("30 s after the end the quiet after the last write is not over: %+v", g)
	}
	if g := stateAt(t, 70*time.Second, lines...); g.Blocked {
		t.Fatalf("two writes within a few seconds of the end must take the short grace, not the three minutes: %+v", g)
	}
	tr := feedAll(lines...)
	if tr.writesAfterEnd != 2 || tr.writesNearEnd != 2 {
		t.Fatalf("the tracker saw %d writes after the end, %d near it", tr.writesAfterEnd, tr.writesNearEnd)
	}
}

// A long stream of writes is still the fallback: a second write a minute after the end, or a third within the window, is not what leaving a world looks like.
func TestAStreamOfWritesAfterTheEndKeepsTheFallback(t *testing.T) {
	for name, lines := range map[string][]string{
		"a write a minute after the end":      {neStart, neWrite, neEnd, neWrite1, neLate},
		"a write a minute after the end only": {neStart, neWrite, neEnd, neLate},
		"three writes within the window":      {neStart, neWrite, neEnd, neWrite1, neWrite2, neWrite3},
		"four writes within the window":       {neStart, neWrite, neEnd, neWrite1, neWrite2, neWrite3, neWrite4},
		"a write at the menu minutes later":   {neStart, neWrite, neEnd, neWrite1, neAtMenu},
	} {
		// 130 s after the end: past the short grace (30 s after the end, 60 s after the last write) of a clean exit of the same shape; the fallback holds until 3 minutes after the last write
		g := stateAt(t, 130*time.Second, lines...)
		if name == "a write a minute after the end only" {
			// exactly one write, whenever it came, is the rule of #612 and is unchanged: it waits 60 s after that write (120 s after the end)
			if g := stateAt(t, 130*time.Second, lines...); g.Blocked {
				t.Errorf("%s: one late write keeps the rule it had: %+v", name, g)
			}
			continue
		}
		if name == "a write at the menu minutes later" {
			g = stateAt(t, 6*time.Minute+30*time.Second, lines...) // the write 5 min 38 s after the end: held for 3 minutes after it
		}
		if !g.Blocked || g.Mode != "session_ended" {
			t.Errorf("%s: the fallback must hold: %+v", name, g)
		}
	}
}

// No end marker: the game wrote its database and no session end is in the log (it was started at the menu): the three minutes since the last write apply, as before.
func TestNoEndMarkerKeepsTheFallback(t *testing.T) {
	lines := []string{neWrite, neWrite1, neWrite2}
	if g := stateAt(t, 100*time.Second, lines...); !g.Blocked || g.Mode != "session_ended" {
		t.Fatalf("no end marker, 100 s after the last write: the three minutes hold: %+v", g)
	}
	if g := stateAt(t, 4*time.Minute, lines...); g.Blocked {
		t.Fatalf("no end marker, more than three minutes after the last write: %+v", g)
	}
}

func TestEndWritesNormal(t *testing.T) {
	for _, c := range []struct {
		n, near int
		want    bool
	}{{0, 0, true}, {1, 0, true}, {1, 1, true}, {2, 2, true}, {2, 1, false}, {2, 0, false}, {3, 3, false}, {5, 5, false}} {
		if got := endWritesNormal(c.n, c.near); got != c.want {
			t.Errorf("endWritesNormal(%d, %d) = %v, want %v", c.n, c.near, got, c.want)
		}
	}
}
