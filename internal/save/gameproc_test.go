package save

import (
	"errors"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMatchGameImages(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"full name", []string{"DuneSandbox-Win64-Shipping.exe"}, []string{"DuneSandbox-Win64-Shipping.exe"}},
		{"a different dune process", []string{"DuneAwakening.exe"}, []string{"DuneAwakening.exe"}},
		{"mixed case", []string{"dunesandbox.exe", "DUNELauncher.exe"}, []string{"dunesandbox.exe", "DUNELauncher.exe"}},
		{"two processes, one repeated", []string{"DuneA.exe", "DuneB.exe", "DuneA.exe"}, []string{"DuneA.exe", "DuneB.exe"}},
		{"other process", []string{"notepad.exe", "explorer.exe"}, nil},
		{"the prefix must start the name", []string{"notdune.exe", "my-dune.exe"}, nil},
		{"our own program is not the game", []string{"tabr-tau-window.exe", "tabr-tau.exe"}, nil},
		{"surrounding blanks", []string{"  DuneX.exe "}, []string{"DuneX.exe"}},
		{"empty", nil, nil},
	}
	for _, c := range cases {
		if got := matchGameImages(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// fakeImages replaces the process lister for one test and restores everything after it. The answers are used in order; the last one repeats. It counts the
// asks and records the pauses between tries (nothing really sleeps).
type fakeImages struct {
	answers []imagesAnswer
	calls   int
	sleeps  []time.Duration
}

type imagesAnswer struct {
	names []string
	err   error
}

func useImages(t *testing.T, answers ...imagesAnswer) *fakeImages {
	t.Helper()
	f := &fakeImages{answers: answers}
	oldOS, oldList, oldSleep, oldLoc := goos, listImages, sleepFunc, locationFor
	goos = "windows"
	locationFor = func(string) string { return "" }
	listImages = func() ([]string, error) {
		i := f.calls
		if i >= len(f.answers) {
			i = len(f.answers) - 1
		}
		f.calls++
		return f.answers[i].names, f.answers[i].err
	}
	sleepFunc = func(d time.Duration) { f.sleeps = append(f.sleeps, d) }
	resetGameCache()
	t.Cleanup(func() {
		goos, listImages, sleepFunc, locationFor = oldOS, oldList, oldSleep, oldLoc
		resetGameCache()
	})
	return f
}

var (
	noGame   = imagesAnswer{names: []string{"explorer.exe", "notepad.exe"}}
	theGame  = imagesAnswer{names: []string{"explorer.exe", "DuneSandbox-Win64-Shipping.exe"}}
	snapFail = imagesAnswer{err: errors.New("process snapshot: Access is denied")}
)

// A check that cannot run must block writes and say why, not pass as "game closed". The failure is injected (on every OS), so this does not depend on whether the
// real process list works. Every try fails here, so the retries are used up first.
func TestFailedGameCheckBlocks(t *testing.T) {
	f := useImages(t, snapFail)
	if !GameRunningNow() || !GameRunning() {
		t.Fatal("a failed check must count as the game running")
	}
	if m := RunningMessage(); !strings.Contains(m, "could not check") || !strings.Contains(m, "Access is denied") {
		t.Fatalf("message does not say the check failed and why: %q", m)
	}
	if f.calls < checkAttempts {
		t.Fatalf("a check that failed was tried %d times, want at least %d before it blocked", f.calls, checkAttempts)
	}
}

// One failed try (the intermittent failure of #366) must not block anything: it is tried again after a short pause and the second answer counts.
func TestOneFailedTryIsRetriedAndBlocksNothing(t *testing.T) {
	f := useImages(t, snapFail, noGame)
	if GameRunningNow() {
		t.Fatalf("a single failed try blocked: %q", GameCheckError())
	}
	if GameCheckError() != "" {
		t.Fatalf("the failed try left an error behind: %q", GameCheckError())
	}
	if f.calls != 2 || len(f.sleeps) != 1 || f.sleeps[0] <= 0 || f.sleeps[0] > time.Second {
		t.Fatalf("want 2 tries with one short pause between, got %d tries and pauses %v", f.calls, f.sleeps)
	}
	// and it also finds the game after a failed first try
	useImages(t, snapFail, snapFail, theGame)
	if n := GameProcessesNow(); len(n) != 1 || !strings.HasPrefix(n[0], "DuneSandbox") {
		t.Fatalf("the third try's answer was not used: %v", n)
	}
}

// A check that failed every try is an error; the next check that works clears it, without a restart (#366 expectation 3).
func TestTheErrorClearsWhenTheNextCheckSucceeds(t *testing.T) {
	useImages(t, snapFail, snapFail, snapFail, noGame)
	p := filepath.Join(t.TempDir(), "game.db")
	if g := GameStateFor(p, true); !g.Blocked || g.CheckError == "" {
		t.Fatalf("every try failed: the write must be refused with the reason, got %+v", g)
	}
	if g := GameStateFor(p, true); g.Blocked || g.CheckError != "" {
		t.Fatalf("the next check worked and the block must be gone, got %+v", g)
	}
}

// The status poll keeps showing the last good answer for a short time when a check fails, so one failure does not flip the banner; a write never relies on it.
func TestAStatusPollKeepsTheLastGoodAnswerButAWriteFailsClosed(t *testing.T) {
	useImages(t, theGame, snapFail)
	if n := GameProcesses(); len(n) != 1 {
		t.Fatalf("first answer: %v", n)
	}
	gameCache.Lock()
	gameCache.at = time.Now().Add(-2 * gameCacheTTL) // the cached answer is too old to reuse: the poll asks again, and the ask fails
	gameCache.Unlock()
	if n := GameProcesses(); len(n) != 1 || GameCheckError() != "" {
		t.Fatalf("one failed poll flipped the state: names=%v err=%q", n, GameCheckError())
	}
	if !GameRunningNow() || GameCheckError() == "" {
		t.Fatal("a check that gates a write must not use the old answer: it fails closed with the reason")
	}
	// after the good answer is too old to trust, the poll shows the failure too
	gameCache.Lock()
	gameCache.at, gameCache.goodAt = time.Time{}, time.Now().Add(-2*gameLastGoodFor)
	gameCache.Unlock()
	GameProcesses()
	if GameCheckError() == "" {
		t.Fatal("an answer older than the grace period must not hide a failing check")
	}
}

// A failed answer is not kept as long as a good one, so the banner is not stuck for the whole poll interval after the cause is gone.
func TestAFailedAnswerIsAskedAgainSoon(t *testing.T) {
	if gameErrTTL > 5*time.Second || gameErrTTL >= gameCacheTTL {
		t.Fatalf("a failed answer lives %v: it must be short (at most 5 s) and shorter than a good one (%v)", gameErrTTL, gameCacheTTL)
	}
	f := useImages(t, snapFail, snapFail, snapFail, noGame)
	GameProcesses()
	if GameCheckError() == "" {
		t.Fatal("the check failed and must say so")
	}
	before := f.calls
	GameProcesses() // inside the short window: the same answer, nothing asked
	if f.calls != before {
		t.Fatalf("asked again at once, %d -> %d", before, f.calls)
	}
	gameCache.Lock()
	gameCache.at = time.Now().Add(-gameErrTTL - time.Second)
	gameCache.Unlock()
	GameProcesses()
	if f.calls == before || GameCheckError() != "" {
		t.Fatalf("a failed answer was not asked again after its short life (calls %d -> %d, err %q)", before, f.calls, GameCheckError())
	}
}

// A write asks the machine again every time (fresh); only the status poll may use the answer of the last gameCacheTTL. This pins the direction of the flag (ARCH-4 of the
// 2026-10-22 audit read it as inverted; it was fixed in #242).
func TestAWriteGateAsksTheProcessListAgainAndTheStatusPollMayUseTheCache(t *testing.T) {
	f := useImages(t, noGame)
	p := filepath.Join(t.TempDir(), "game.db")
	GameStateFor(p, true)
	GameStateFor(p, true)
	GameStateFor(p, true)
	if f.calls != 3 {
		t.Fatalf("each write check asks the process list itself, asked %d times in 3", f.calls)
	}
	resetGameCache()
	f.calls = 0
	GameStateFor(p, false)
	GameStateFor(p, false)
	GameStateFor(p, false)
	if f.calls != 1 {
		t.Fatalf("the status poll reuses the last answer, asked %d times in 3", f.calls)
	}
	if blocked, _ := SaveBlocked(p); blocked || f.calls != 2 {
		t.Fatalf("SaveBlocked (the write gate) is never answered from the cache: blocked=%v calls=%d", blocked, f.calls)
	}
}

// The status poll runs every few seconds; the process list is asked at most about every 15 seconds for it (#359).
func TestTheStatusPollAsksNoMoreOftenThanTheCacheLife(t *testing.T) {
	if gameCacheTTL < 15*time.Second || gameCacheTTL > 30*time.Second {
		t.Fatalf("the poll's cache life is %v, want 15 to 30 seconds", gameCacheTTL)
	}
	f := useImages(t, noGame)
	GameProcesses()
	gameCache.Lock()
	gameCache.at = time.Now().Add(-gameCacheTTL + 2*time.Second)
	gameCache.Unlock()
	GameProcesses()
	if f.calls != 1 {
		t.Fatalf("asked again inside the cache life: %d", f.calls)
	}
	gameCache.Lock()
	gameCache.at = time.Now().Add(-gameCacheTTL - time.Second)
	gameCache.Unlock()
	GameProcesses()
	if f.calls != 2 {
		t.Fatalf("not asked again after the cache life: %d", f.calls)
	}
}

// "Check again" and "Write now": the check is made at once (never from the cache) and the answer is a sentence.
func TestCheckGameNowSaysWhatItFound(t *testing.T) {
	p := filepath.Join(t.TempDir(), "game.db")
	oldS := statusFor
	t.Cleanup(func() { statusFor = oldS })

	f := useImages(t, noGame)
	statusFor = func(string) (spStatus, error) { return spStatus{}, nil }
	c := CheckGameNow(p)
	if c.Running || c.Failed || c.Blocked || c.Message != "the game is not running: edits are written at once" {
		t.Fatalf("not running: %+v", c)
	}
	// it asks again every time, whatever is cached
	CheckGameNow(p)
	if f.calls != 2 {
		t.Fatalf("Check again must ask the machine each time, asked %d times in 2", f.calls)
	}

	useImages(t, theGame)
	statusFor = func(string) (spStatus, error) { return spStatus{Active: true}, nil }
	c = CheckGameNow(p)
	if !c.Running || !c.Blocked || c.Failed || c.Message != "the game is running: edits will queue" || len(c.Processes) != 1 {
		t.Fatalf("running in a session: %+v", c)
	}
	statusFor = func(string) (spStatus, error) { return spStatus{}, nil }
	c = CheckGameNow(p)
	if !c.Running || c.Blocked || !strings.Contains(c.Message, "not in a single-player session") {
		t.Fatalf("running at the menu: %+v", c)
	}

	useImages(t, snapFail)
	c = CheckGameNow(p)
	if !c.Failed || !c.Blocked || !strings.HasPrefix(c.Message, "the check failed again: ") || !strings.Contains(c.Message, "Access is denied") || c.CheckError == "" {
		t.Fatalf("failed: %+v", c)
	}
}

// Nothing on the path of the game check starts a program (#359): the files that make the check import neither os/exec nor a package that wraps it.
func TestTheGameCheckStartsNoProgram(t *testing.T) {
	files, err := filepath.Glob("gameproc*.go")
	if err != nil || len(files) < 2 {
		t.Fatalf("the game check's files were not found: %v %v", files, err)
	}
	fs := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fs, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			if p := strings.Trim(imp.Path.Value, `"`); p == "os/exec" || strings.HasSuffix(p, "/winpath") {
				t.Errorf("%s imports %s: the game check must not start a program", name, p)
			}
		}
	}
}

// A queued edit is written by itself once the check works again, with no restart: the check failed on every try of the first attempts to write, then the process list
// answers and the next attempt (the editor makes one every few seconds while edits wait) writes it. The game is not running at that point.
func TestAQueuedEditIsWrittenOnceTheFailingCheckWorksAgain(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// two whole attempts fail (checkAttempts tries each), then the list answers: the game is not running
	answers := make([]imagesAnswer, 0, 2*checkAttempts+1)
	for i := 0; i < 2*checkAttempts; i++ {
		answers = append(answers, snapFail)
	}
	useImages(t, append(answers, noGame)...)
	setA(t, s, 2)
	for i := 0; i < 2; i++ {
		f := s.Flush()
		if f.Saved || f.Queued != 1 || !strings.Contains(f.Why, "could not check whether the game is running") {
			t.Fatalf("attempt %d: with the check failing the edit waits and says why: %+v", i+1, f)
		}
	}
	if !s.Dirty() || fileA(t, p) != 1 {
		t.Fatal("nothing was written while the check failed")
	}
	f := s.Flush()
	if !f.Saved || f.Queued != 0 || s.Dirty() {
		t.Fatalf("the check works again: the waiting edit is written: %+v", f)
	}
	if fileA(t, p) != 2 {
		t.Fatal("the file holds the edit")
	}
}
