package web

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// #1124 review: Cancel in the queue view must win over a retry that is already under way. The retry re-reads the save before it runs the pass; a Cancel that lands in that
// window used to answer "cancelled" while the pass still wrote (and, if the game relaunched meanwhile, the cancelled wait came back).

func waitingServer(t *testing.T) (*Server, func()) {
	t.Helper()
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	restore := save.GameJustLeftWorldForTests()
	s.RunStartupTasks()
	if !isWaiting(s) {
		restore()
		t.Fatal("the pass should be waiting for the game")
	}
	restore() // the game closes: the next retry gets as far as the re-read
	return s, func() {}
}

func TestCancelDuringARetryStopsThePassFromRunning(t *testing.T) {
	s, _ := waitingServer(t)
	before, _ := os.ReadFile(s.Save.Path)
	var cancelled bool
	s.afterReload = func() { cancelled = s.cancelWaitingPass() }
	s.retryWaitingPass()
	if !cancelled {
		t.Fatal("the cancel should have found the refill waiting")
	}
	if after, _ := os.ReadFile(s.Save.Path); !bytes.Equal(before, after) || s.Save.Dirty() || len(s.Save.Backups()) != 0 {
		t.Fatal("Cancel said the refill was cancelled, but the pass wrote")
	}
	if isWaiting(s) {
		t.Fatal("the cancelled refill still waits")
	}
	if n, _ := s.takeStartup(); strings.Contains(n, "Automatic refill saved") {
		t.Fatalf("the pass ran after Cancel: %q", n)
	}
}

func TestCancelDuringARetryIsNotUndoneByTheGameRelaunching(t *testing.T) {
	s, _ := waitingServer(t)
	var restoreGame func()
	s.afterReload = func() {
		s.cancelWaitingPass()
		restoreGame = save.ActiveSessionForTests() // the game loads a world again before the pass looks
	}
	s.retryWaitingPass()
	if restoreGame != nil {
		defer restoreGame()
	}
	if isWaiting(s) {
		t.Fatal("the cancelled refill came back when the game relaunched")
	}
	if n, _ := s.takeStartup(); strings.Contains(n, "queued") {
		t.Fatalf("a note promises the cancelled refill: %q", n)
	}
}

func TestARetryWithNoCancelStillRunsThePass(t *testing.T) {
	s, _ := waitingServer(t)
	called := false
	s.afterReload = func() { called = true }
	s.retryWaitingPass()
	if !called {
		t.Fatal("the retry never reached the re-read")
	}
	if n, _ := s.takeStartup(); !strings.Contains(n, "Automatic refill saved ") {
		t.Fatalf("a retry with no Cancel should run the pass: %q", n)
	}
	if isWaiting(s) {
		t.Fatal("the pass ran, so it no longer waits")
	}
	if got := autoPassFile(t, s.Save.Path); got["water"] != int64(5000) {
		t.Fatalf("the written file: %v", got)
	}
}

func TestACancelBeforeAnyRetryStartedDoesNotBlockALaterRefill(t *testing.T) {
	s, _ := waitingServer(t)
	s.cancelWaitingPass()
	if isWaiting(s) {
		t.Fatal("Cancel should end the wait")
	}
	// a new pass at launch waits again, and its retry (which captures the newer generation) runs
	restore := save.GameJustLeftWorldForTests()
	s.RunStartupTasks()
	restore()
	if !isWaiting(s) {
		t.Fatal("a later pass should wait again")
	}
	s.retryWaitingPass()
	if n, _ := s.takeStartup(); !strings.Contains(n, "Automatic refill saved ") {
		t.Fatalf("a retry after an old Cancel should run the pass: %q", n)
	}
}
