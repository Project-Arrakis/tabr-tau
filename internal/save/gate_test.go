package save

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeGame stands in for the machine: procs are the Dune processes the process list finds (perr set when it cannot be read), and the
// session is what the game's log says. The previous behaviour is put back when the test ends.
func fakeGame(t *testing.T, procs []string, perr string, sp spStatus, spErr error) {
	t.Helper()
	oldL, oldS, oldN, oldH := listProcs, statusFor, nowFunc, fileHeld
	listProcs = func(bool) ([]string, string) { return procs, perr }
	statusFor = func(string) (spStatus, error) { return sp, spErr }
	fileHeld = func(string) (bool, error) { return false, nil } // nothing holds the file unless a test says so (fakeHeld)
	t.Cleanup(func() { listProcs, statusFor, nowFunc, fileHeld = oldL, oldS, oldN, oldH })
}

// fakeHeld stands in for the check of whether another program has the save file open; call it after fakeGame.
func fakeHeld(held bool, err error) { fileHeld = func(string) (bool, error) { return held, err } }

var duneProc = []string{"DuneSandbox-Win64-Shipping.exe"}

// every situation in which the editor cannot be sure the game is not writing the save
var blockedGames = map[string]func(t *testing.T){
	"a single-player session is active": func(t *testing.T) { fakeGame(t, duneProc, "", spStatus{Active: true}, nil) },
	"the log cannot be read":            func(t *testing.T) { fakeGame(t, duneProc, "", spStatus{}, errors.New("cannot read the game log")) },
	"the process list cannot be read":   func(t *testing.T) { fakeGame(t, nil, "process snapshot: not found", spStatus{}, nil) },
	"the session ended a moment ago": func(t *testing.T) {
		fakeGame(t, duneProc, "", spStatus{EndedAt: time.Now().Add(-2 * time.Second)}, nil)
	},
}

// Commit must refuse whenever the game may be writing, leave the file and the pending edits as they were, and write no backup. The
// positive control at the end shows the same save does commit once the game is not in the way.
func TestCommitRefusesWhenTheGameMayBeWriting(t *testing.T) {
	for name, setup := range blockedGames {
		t.Run(name, func(t *testing.T) {
			p := makeSave(t, `create table t(a integer);`)
			orig, _ := os.ReadFile(p)
			s, err := Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err := s.ExecScript(`insert into t values (1)`); err != nil {
				t.Fatal(err)
			}
			setup(t)
			if _, err := s.CommitForTests(); err == nil {
				t.Fatal("Commit wrote while the game may be writing")
			}
			now, _ := os.ReadFile(p)
			if !bytes.Equal(orig, now) {
				t.Fatal("a refused Commit changed the file")
			}
			if len(s.Pending()) != 1 || !s.Dirty() {
				t.Fatalf("a refused Commit must keep the pending edit: %d pending, dirty %v", len(s.Pending()), s.Dirty())
			}
			if len(s.Backups()) != 0 {
				t.Fatal("a refused Commit made a backup")
			}
			fakeGame(t, nil, "", spStatus{}, nil) // the game is closed
			if res, err := s.CommitForTests(); err != nil || res["saved"] != true {
				t.Fatalf("with the game closed the same edit must save: %v %v", res, err)
			}
		})
	}
}

// Nothing skips the game check, and with the game closed nothing skips the changed-on-disk check either: there is no way to ask for a write that ignores either one.
func TestNothingSkipsTheGameCheckOrTheOnDiskChangeCheck(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	s, _ := Open(p)
	defer s.Close()
	s.ExecScript(`insert into t values (1)`)
	fakeGame(t, duneProc, "", spStatus{Active: true}, nil)
	if _, err := s.CommitForTests(); err == nil {
		t.Fatal("the game is running: must refuse")
	}
	other := makeSave(t, `create table t(a integer); insert into t values (9);`)
	b, _ := os.ReadFile(other)
	os.WriteFile(p, b, 0o644)             // the file changed on disk too
	fakeGame(t, nil, "", spStatus{}, nil) // the game is closed
	if _, err := s.CommitForTests(); err == nil || !errors.Is(err, ErrChangedOnDisk) {
		t.Fatalf("a file that changed on disk is refused: %v", err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, b) {
		t.Fatal("a refused commit must leave the file alone")
	}
}

// Restore uses the same gate.
func TestRestoreRefusesWhenTheGameMayBeWriting(t *testing.T) {
	for name, setup := range blockedGames {
		t.Run(name, func(t *testing.T) {
			p := makeSave(t, `create table t(a integer);`)
			s, _ := Open(p)
			defer s.Close()
			s.ExecScript(`insert into t values (1)`)
			if _, err := s.CommitForTests(); err != nil { // makes a backup of the original
				t.Fatal(err)
			}
			bk := s.Backups()
			if len(bk) != 1 {
				t.Fatalf("want one backup, have %d", len(bk))
			}
			before, _ := os.ReadFile(p)
			setup(t)
			err := s.Restore(bk[0]["name"].(string))
			if err == nil {
				t.Fatal("Restore wrote while the game may be writing")
			}
			if !strings.Contains(err.Error(), "single-player") && !strings.Contains(err.Error(), "could not check") && !strings.Contains(err.Error(), "cannot tell") && !strings.Contains(err.Error(), "just ended") && !strings.Contains(err.Error(), "is running") {
				t.Fatalf("the refusal should say why: %v", err)
			}
			after, _ := os.ReadFile(p)
			if !bytes.Equal(before, after) {
				t.Fatal("a refused Restore changed the file")
			}
		})
	}
}
