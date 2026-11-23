package save

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// #1139: the watch of the save file (observeFile) used to be kept only by whatever evaluated the gate, and with nothing queued that was the page's status poll alone. The page stops
// polling while its window is hidden or minimised, so a Tau window behind the game for more than observeGap broke the watch and the first poll after the person came back started a new
// observeQuietFor wait however old the file was. KeepObserving is the editor's own evaluation, made from its autosave loop. These tests keep the clock and the game's log in their own hands
// (as watchkeep_test.go does), and look at the one thing that matters: what the gate says afterwards, and that no rule of it has moved.
type obsWorld struct {
	s       *Save
	p       string
	stepped atomic.Int64 // the test's clock, in nanoseconds past obsBase
	asked   atomic.Int64 // how many times the gate read the game's log: one for each evaluation that got that far
	mod     atomic.Int64 // the save file's modification time, in nanoseconds past obsBase
	active  atomic.Bool  // the game's log shows a single-player session running
	procs   atomic.Bool  // a Dune process exists
}

var obsBase = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func newObsWorld(t *testing.T) *obsWorld {
	t.Helper()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	setAge(t, p, time.Hour)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	gateStubs(t)
	oldObs, oldEvery := observeQuietFor, observeEvery
	observeQuietFor = menuQuietAfterWrite // the real wait: the tests of this rule use the real value
	observeEvery = observeGap / 3
	t.Cleanup(func() { observeQuietFor, observeEvery = oldObs, oldEvery; forgetObserved(p); forgetOwnWrite(p) })
	forgetObserved(p)
	w := &obsWorld{s: s, p: p}
	w.mod.Store(int64(-48 * time.Hour)) // one time for the whole test unless it moves it: a changing time restarts the watch itself
	w.procs.Store(true)
	clock := func() time.Time { return obsBase.Add(time.Duration(w.stepped.Load())) }
	nowFunc = clock
	listProcs = func(bool) ([]string, string) {
		if w.procs.Load() {
			return duneProc, ""
		}
		return nil, ""
	}
	fileTimeFor = func(string) (time.Time, bool, error) { return obsBase.Add(time.Duration(w.mod.Load())), false, nil }
	statusFor = func(string) (spStatus, error) {
		w.asked.Add(1)
		now := clock()
		if w.active.Load() {
			return spStatus{Active: true}, nil
		}
		return spStatus{EndedAt: now.Add(-time.Hour), LastSaveAt: now.Add(-time.Hour + 5*time.Second)}, nil // at the menu for an hour
	}
	return w
}

func (w *obsWorld) step(d time.Duration) { w.stepped.Add(int64(d)) }

// run moves the clock in steps of 5 s for total, and calls the editor's own observation after each step, as the autosave loop does (it ticks every 3 s). No page asks anything.
func (w *obsWorld) run(total time.Duration, keep bool) {
	for el := time.Duration(0); el < total; el += 5 * time.Second {
		w.step(5 * time.Second)
		if keep {
			w.s.KeepObserving()
		}
	}
}

func TestAHiddenPageDoesNotRestartTheWatch(t *testing.T) {
	w := newObsWorld(t)
	orig, _ := os.ReadFile(w.p)
	w.run(5*time.Minute, true) // the window is hidden for five minutes: nothing but the editor looks
	g := processState(w.p, true)
	if g.Blocked {
		t.Fatalf("five minutes of unbroken watching with the file unchanged must pass the watch: %+v", g)
	}
	// the control: the same five minutes with nothing looking is the gap that used to restart the watch
	c := newObsWorld(t)
	c.run(5*time.Minute, false)
	if g := processState(c.p, true); !g.Blocked || !strings.Contains(g.Reason, "only just started watching") {
		t.Fatalf("with nothing looking the watch must start again: %+v", g)
	}
	if now, _ := os.ReadFile(w.p); string(now) != string(orig) {
		t.Fatal("KeepObserving changed the file")
	}
	if w.s.Dirty() {
		t.Fatal("KeepObserving queued something")
	}
}

// What it records is only what is true: it looks at most once in observeEvery, however often the loop calls it, and it asks nothing while no game process exists.
func TestKeepObservingLooksOncePerIntervalAndOnlyWithAGameProcess(t *testing.T) {
	w := newObsWorld(t)
	for i := 0; i < 100; i++ {
		w.s.KeepObserving() // 100 calls in the same second
	}
	if n := w.asked.Load(); n != 1 {
		t.Fatalf("100 calls in one second made %d evaluations, want 1", n)
	}
	w.step(observeEvery - time.Second)
	w.s.KeepObserving()
	if n := w.asked.Load(); n != 1 {
		t.Fatalf("a call before observeEvery made an evaluation (%d)", n)
	}
	w.step(time.Second)
	w.s.KeepObserving()
	if n := w.asked.Load(); n != 2 {
		t.Fatalf("a call at observeEvery made %d evaluations in all, want 2", n)
	}
	// no game process: nothing is read, and no watch is kept (the watch of an earlier game is dropped, as when the gate itself finds no process)
	w.procs.Store(false)
	w.step(time.Minute)
	w.s.KeepObserving()
	if n := w.asked.Load(); n != 2 {
		t.Fatalf("with no game process the log was read (%d evaluations)", n)
	}
	observed.Lock()
	_, has := observed.m[ownKey(w.p)]
	observed.Unlock()
	if has {
		t.Fatal("a watch was kept with no game process")
	}
}

// No gate is weakened: a game whose log shows a session running blocks for as long as it runs, however long the editor has been watching.
func TestAStillActiveGameStillBlocksWhileTheEditorWatches(t *testing.T) {
	w := newObsWorld(t)
	w.active.Store(true)
	w.run(5*time.Minute, true)
	if g := processState(w.p, true); !g.Blocked || g.Mode != "single_player" {
		t.Fatalf("an active session must block: %+v", g)
	}
	setA(t, w.s, 2)
	if f := w.s.Flush(); f.Saved || !strings.Contains(f.Why, "single-player session is active") {
		t.Fatalf("an edit made while the session runs must stay queued: %+v", f)
	}
	if !w.s.Dirty() {
		t.Fatal("the edit must stay queued")
	}
	// the session ends: the watch has been unbroken all along, and the rest of the gate (the log's grace after the end, the file's quiet) is as it was
	w.active.Store(false)
	if g := processState(w.p, true); g.Blocked {
		t.Fatalf("a session that ended an hour ago, watched for five minutes, passes: %+v", g)
	}
}

// A real gap still starts the watch again: a sleep, a hibernation, a clock set forward, or a loop that did not run for more than observeGap.
func TestAGapStillRestartsTheWatchWhateverKeepObservingDid(t *testing.T) {
	w := newObsWorld(t)
	w.run(5*time.Minute, true)
	w.step(2 * time.Minute) // the computer slept: nothing ran for two minutes
	if g := processState(w.p, true); !g.Blocked || !strings.Contains(g.Reason, "only just started watching") {
		t.Fatalf("a gap of two minutes must restart the watch: %+v", g)
	}
	w.run(40*time.Second, true) // looking again, but not yet for a whole observeQuietFor
	if g := processState(w.p, true); !g.Blocked || !strings.Contains(g.Reason, "only just started watching") {
		t.Fatalf("40 s into the new watch it must still be refused: %+v", g)
	}
	w.run(30*time.Second, true)
	if g := processState(w.p, true); g.Blocked {
		t.Fatalf("70 s of unbroken watching after the gap passes: %+v", g)
	}
	// the clock stepped backward is a gap too
	w.step(-time.Minute)
	w.s.KeepObserving()
	if g := processState(w.p, true); !g.Blocked || !strings.Contains(g.Reason, "only just started watching") {
		t.Fatalf("a clock that went backward must restart the watch: %+v", g)
	}
}

// A change of the file's time that is not the editor's own write still starts the watch again.
func TestAChangedFileStillRestartsTheWatch(t *testing.T) {
	w := newObsWorld(t)
	w.run(5*time.Minute, true)
	w.mod.Store(int64(-47 * time.Hour)) // the game touched the file (it still looks old, so only the watch can refuse)
	w.step(5 * time.Second)
	w.s.KeepObserving()
	if g := processState(w.p, true); !g.Blocked || !strings.Contains(g.Reason, "only just started watching") {
		t.Fatalf("a changed file time must restart the watch: %+v", g)
	}
	w.run(60*time.Second, true)
	if g := processState(w.p, true); g.Blocked {
		t.Fatalf("a minute of watching the changed file unchanged passes: %+v", g)
	}
}

// The effect the person sees: an edit made by hand after a long time with the window hidden is written (the game is at its menu and has been for an hour), where it used to be refused with
// "the editor has only just started watching".
func TestAnEditAfterAHiddenWindowIsWrittenWhenTheGameIsAtItsMenu(t *testing.T) {
	for _, keep := range []bool{true, false} {
		w := newObsWorld(t)
		w.run(10*time.Minute, keep)
		setA(t, w.s, 2)
		f := w.s.Flush()
		if keep && !f.Saved {
			t.Fatalf("the editor watched all along, so the edit goes through: %+v", f)
		}
		if !keep && (f.Saved || !strings.Contains(f.Why, "only just started watching")) {
			t.Fatalf("with nothing watching the first evaluation after the gap refuses (the rule is unchanged): %+v", f)
		}
	}
}

// A copy that is not the game's own save has no gate to watch: nothing is read for it.
func TestKeepObservingLeavesACopyAlone(t *testing.T) {
	w := newObsWorld(t)
	old := ownSaveFor
	ownSaveFor = func(string) bool { return false }
	t.Cleanup(func() { ownSaveFor = old })
	w.s.KeepObserving()
	if n := w.asked.Load(); n != 0 {
		t.Fatalf("the log was read for a copy (%d)", n)
	}
}
