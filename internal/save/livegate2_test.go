package save

import (
	"os"
	"strings"
	"testing"
	"time"
)

// #612, the edit of the automatic pass: nobody watches it, so it is held to the unattended rule wherever it is written from. If the pass's own Flush is refused (the game started
// meanwhile) the edit stays queued, and neither the next tick of the autosave nor a restart that restores it from the journal may write it while a game process exists.
func TestAQueuedPassEditIsNeverWrittenWhileAGameProcessExists(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	setAge(t, p, time.Hour)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	orig, _ := os.ReadFile(p)
	gateStubs(t)
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Now().Add(-time.Hour), LastSaveAt: time.Now().Add(-time.Hour + 5*time.Second)}, nil
	}
	fileTimeFor = func(string) (time.Time, bool, error) { return time.Now().Add(-time.Hour), false, nil }
	t.Cleanup(func() { forgetOwnWrite(p) })

	// the same state of the game lets a hand edit through: this is the gate of an edit made by hand
	if g := GameStateFor(p, true); g.Blocked {
		t.Fatalf("the game at its menu must let a hand edit through here: %+v", g)
	}
	if _, err := s.Mutate(AutoPassDesc+": 1 Water Cistern", func(m *Mut) error { _, err := m.Exec(`insert into t values (1)`); return err }); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ { // the ticks of the autosave
		if f := s.Flush(); f.Saved || !strings.Contains(f.Why, "until the game is closed") {
			t.Fatalf("tick %d: the pass's edit was not held back: %+v", i, f)
		}
	}
	if _, err := s.CommitForTests(); err == nil || !strings.Contains(err.Error(), "until the game is closed") {
		t.Fatalf("a commit of the pass's edit with the game open: %v", err)
	}
	if now, _ := os.ReadFile(p); string(now) != string(orig) {
		t.Fatal("the pass's edit was written with the game open")
	}

	// a restart restores the edit from the journal; it is still the pass's edit
	s.Close()
	s2 := openJournaled(t, p, dir)
	if r, err := s2.RestoreJournal(); err != nil || r == nil || r.Applied != 1 {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if got := s2.Pending(); len(got) != 1 || !strings.HasPrefix(got[0], AutoPassDesc) {
		t.Fatalf("pending after the restart: %v", got)
	}
	if f := s2.Flush(); f.Saved || !strings.Contains(f.Why, "until the game is closed") {
		t.Fatalf("a restored pass edit was written under the gate of a hand edit: %+v", f)
	}
	if now, _ := os.ReadFile(p); string(now) != string(orig) {
		t.Fatal("the restored pass edit was written with the game open")
	}

	// a person at the page makes an edit of their own: the queue is a hand queue, and the pass's edit goes with it
	if _, err := s2.Mutate("hand edit", func(m *Mut) error { _, err := m.Exec(`insert into t values (2)`); return err }); err != nil {
		t.Fatal(err)
	}
	if f := s2.Flush(); !f.Saved {
		t.Fatalf("a queue with a hand edit has the gate of a hand edit: %+v", f)
	}

	// and with the game closed the pass's edit alone is written
	if _, err := s2.Mutate(AutoPassDesc+": again", func(m *Mut) error { _, err := m.Exec(`insert into t values (3)`); return err }); err != nil {
		t.Fatal(err)
	}
	if f := s2.Flush(); f.Saved {
		t.Fatalf("still open: %+v", f)
	}
	listProcs = func(bool) ([]string, string) { return nil, "" }
	if f := s2.Flush(); !f.Saved {
		t.Fatalf("with the game closed the pass's edit must be written: %+v", f)
	}
}

// #612 (LOW 2): the quiet must be observed. A computer that slept, or a clock set forward, makes an old file time look like a long quiet; the editor holds for a minute and sees the
// file unchanged first. The clock is injected.
func TestTheQuietOfTheFileMustBeObserved(t *testing.T) {
	gateStubs(t)
	oldObs := observeQuietFor
	observeQuietFor = menuQuietAfterWrite
	t.Cleanup(func() { observeQuietFor = oldObs; forgetObserved("p") })
	forgetObserved("p")
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), LastSaveAt: time.Date(2020, 1, 1, 0, 0, 5, 0, time.UTC)}, nil
	}
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return clock }
	mod, own := clock.Add(-time.Hour), false
	fileTimeFor = func(string) (time.Time, bool, error) { return mod, own, nil }
	at := func(d time.Duration) GameState { clock = clock.Add(d); return processState("p", true) }
	held := func(g GameState) bool { return g.Blocked && strings.Contains(g.Reason, "only just started watching") }

	if g := at(0); !held(g) { // the first evaluation: nothing observed yet, however old the file's time is
		t.Fatalf("first evaluation: %+v", g)
	}
	for i := 0; i < 5; i++ { // the status poll, every ten seconds
		if g := at(10 * time.Second); !held(g) {
			t.Fatalf("after %d s: %+v", 10*(i+1), g)
		}
	}
	if g := at(10 * time.Second); g.Blocked || g.Mode != "menu" { // 60 s watched without a gap
		t.Fatalf("watched for a minute: %+v", g)
	}
	// a sleep: the computer was off for two hours; the file's time is as old as before, and that proves nothing
	if g := at(2 * time.Hour); !held(g) {
		t.Fatalf("after a sleep: %+v", g)
	}
	if g := at(observeGap + time.Second); !held(g) { // a gap just over the limit starts the watch again
		t.Fatalf("a gap over the limit: %+v", g)
	}
	for i := 0; i < 6; i++ {
		at(observeGap) // exactly the limit is not a gap
	}
	if g := at(observeGap); g.Blocked {
		t.Fatalf("evaluations at the limit are continuous: %+v", g)
	}
	// a clock that goes backward is a gap too
	if g := at(-time.Hour); !held(g) {
		t.Fatalf("after the clock went back: %+v", g)
	}
	for i := 0; i < 6; i++ {
		at(10 * time.Second)
	}
	if g := at(10 * time.Second); g.Blocked {
		t.Fatalf("watched again: %+v", g)
	}
	// the file's time changes to another time that is old enough for the file-quiet rule (a recent time would be held by that rule, not by the watch): only the watch, which sees a
	// change it cannot explain, holds the next edit, and says so
	mod = clock.Add(-30 * time.Minute)
	if g := at(10 * time.Second); !held(g) {
		t.Fatalf("a file whose time changed to another old time was not watched again: %+v", g)
	}
	for i := 0; i < 6; i++ {
		at(10 * time.Second)
	}
	if g := at(10 * time.Second); g.Blocked {
		t.Fatalf("quiet again: %+v", g)
	}
	// the editor's own write changes the time and does not start the watch again
	mod, own = clock.Add(-20*time.Minute), true
	if g := at(3 * time.Second); g.Blocked {
		t.Fatalf("the editor's own write held the next edit back: %+v", g)
	}
	// the game closes: the watch is dropped, and the next game starts a new one
	listProcs = func(bool) ([]string, string) { return nil, "" }
	if g := at(3 * time.Second); g.Mode != "none" {
		t.Fatalf("no game: %+v", g)
	}
	listProcs = func(bool) ([]string, string) { return duneProc, "" }
	own = false // the file's time is the same one across the close and the reopen: only the dropped watch can hold this
	if g := at(3 * time.Second); !held(g) {
		t.Fatalf("a new game process: %+v", g)
	}
}

// #612 (LOW 5): a state that was let through because the log said nothing is not reported as the menu.
func TestAWriteLetThroughOnAnEmptyLogIsNotCalledTheMenu(t *testing.T) {
	gateStubs(t)
	statusFor = func(string) (spStatus, error) { return spStatus{}, nil }
	fileTimeFor = func(string) (time.Time, bool, error) { return time.Now().Add(-time.Hour), false, nil }
	g := GameStateFor("x", true)
	if g.Blocked || g.Mode != "unknown_quiet" || len(g.Processes) != 1 {
		t.Fatalf("%+v", g)
	}
	if c := CheckGameNow("x"); c.Blocked || !c.Running || strings.Contains(c.Message, "at its menu") || !strings.Contains(c.Message, "no sign of a session") {
		t.Fatalf("the message of Check again says the menu for a log that said nothing: %+v", c)
	}
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Now().Add(-time.Hour), LastSaveAt: time.Now().Add(-time.Hour + 5*time.Second)}, nil
	}
	if g := GameStateFor("x", true); g.Mode != "menu" {
		t.Fatalf("a log that says the session ended is the menu: %+v", g)
	}
}

// #779 (LOW 2): the gate is asked again after the encode and before the backup. Without it, a write whose preparation outlasted the watch's gap was refused at the gate before the
// rename only after it had taken a backup of its own, and every refused attempt pushed an older automatic restore point out of the ones kept. Here the game's session starts after
// the gate of Flush: nothing may be written, and no backup may be taken.
func TestARefusedAutomaticWriteTakesNoBackup(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	setAge(t, p, time.Hour)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	orig, _ := os.ReadFile(p)
	gateStubs(t)
	fileTimeFor = func(string) (time.Time, bool, error) { return time.Now().Add(-time.Hour), false, nil }
	asked := 0
	statusFor = func(string) (spStatus, error) {
		asked++
		if asked > 1 { // the first question is the gate of Flush; a session has begun by the second
			return spStatus{Active: true}, nil
		}
		return spStatus{EndedAt: time.Now().Add(-time.Hour), LastSaveAt: time.Now().Add(-time.Hour + 5*time.Second)}, nil
	}
	setA(t, s, 2)
	f := s.Flush()
	if f.Saved || !strings.Contains(f.Why, "single-player session is active") {
		t.Fatalf("the write should be refused by the second question: %+v", f)
	}
	if asked != 2 {
		t.Fatalf("the gate was asked %d times, want 2 (Flush, then before the backup): the refusal came too late", asked)
	}
	if ents, _ := os.ReadDir(s.backupDir()); len(ents) != 0 {
		t.Fatalf("a refused write took %d backup(s)", len(ents))
	}
	if now, _ := os.ReadFile(p); string(now) != string(orig) {
		t.Fatal("the file changed")
	}
	if !s.Dirty() {
		t.Fatal("the edit must stay queued")
	}
}

// #779 (LOW 5): two evaluations that read the clock a moment apart and take the lock in the other order are not a clock that went backward and do not start the watch again. A
// real step back (a second or more) still does.
func TestAnOutOfOrderEvaluationDoesNotRestartTheWatch(t *testing.T) {
	const p = "skew"
	forgetObserved(p)
	t.Cleanup(func() { forgetObserved(p) })
	t0 := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	mod := t0.Add(-time.Hour)
	if from := observeFile(p, t0, mod, false, nil); !from.Equal(t0) {
		t.Fatalf("first evaluation starts the watch: %v", from)
	}
	observeFile(p, t0.Add(10*time.Second), mod, false, nil)
	// the poll read the clock 400 ms ago, the tick got the lock first
	if from := observeFile(p, t0.Add(10*time.Second-400*time.Millisecond), mod, false, nil); !from.Equal(t0) {
		t.Fatalf("an evaluation 0.4 s out of order restarted the watch at %v", from)
	}
	// and the next one in order continues it
	if from := observeFile(p, t0.Add(20*time.Second), mod, false, nil); !from.Equal(t0) {
		t.Fatalf("the watch did not continue: %v", from)
	}
	// the skew is one second, and the edges are pinned with literals: 999 ms early is the same moment, exactly 1 s early is a clock that went back
	if clockSkew != time.Second {
		t.Fatalf("clockSkew is %v, want 1s", clockSkew)
	}
	if from := observeFile(p, t0.Add(20*time.Second-999*time.Millisecond), mod, false, nil); !from.Equal(t0) {
		t.Fatalf("a reading 999 ms early restarted the watch at %v", from)
	}
	back := t0.Add(20*time.Second - time.Second)
	if from := observeFile(p, back, mod, false, nil); !from.Equal(back) {
		t.Fatalf("a clock that went back exactly 1 s must restart the watch: %v", from)
	}
}

// shippedObserveQuietFor is observeQuietFor as shipped: package variables are initialised before TestMain switches the watch off for the tests of the other rules.
var shippedObserveQuietFor = observeQuietFor

// #779 (LOW 4): the watch's length as shipped is one menuQuietAfterWrite. A default of 0 would pass every other test, which set the real value themselves.
func TestTheWatchIsOnAsShipped(t *testing.T) {
	if shippedObserveQuietFor != menuQuietAfterWrite || shippedObserveQuietFor <= 0 {
		t.Fatalf("observeQuietFor ships as %v, want %v", shippedObserveQuietFor, menuQuietAfterWrite)
	}
}
