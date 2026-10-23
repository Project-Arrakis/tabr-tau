package save

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// DBA-15 of the 2026-10-22 audit: the game-running check is made again right before the file is replaced. A single-player session that starts after the first check (while the
// edits are encoded, backed up and written to a temp file) must leave the edits queued, not be written over the game's file.
func TestAGameThatStartsAfterTheFirstCheckStopsTheWriteBeforeTheRename(t *testing.T) {
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
	// The game is not running until the editor has written its temp file next to the save (the step right before the rename): from then on a session is active.
	sawTemp := false
	oldL, oldS := listProcs, statusFor
	t.Cleanup(func() { listProcs, statusFor = oldL, oldS })
	listProcs = func(bool) ([]string, string) {
		if m, _ := filepath.Glob(p + ".tabr-*.tmp"); len(m) > 0 {
			sawTemp = true
			return duneProc, ""
		}
		return nil, ""
	}
	statusFor = func(string) (spStatus, error) { return spStatus{Active: true}, nil }

	f := s.Flush()
	if f.Saved || f.Queued != 1 || f.Why == "" {
		t.Fatalf("the edit stays queued, with the reason: %+v", f)
	}
	if !sawTemp {
		t.Fatal("the game was not looked for again once the temp file was written: it must be looked for right before the rename")
	}
	now, _ := os.ReadFile(p)
	if !bytes.Equal(orig, now) {
		t.Fatal("the game's file was replaced although a session had started")
	}
	if !s.Dirty() {
		t.Fatal("the edit is still pending")
	}
	if m, _ := filepath.Glob(p + ".tabr-*.tmp"); len(m) != 0 {
		t.Fatalf("no temp file is left behind: %v", m)
	}
	// and it is written when the session ends
	listProcs = func(bool) ([]string, string) { return nil, "" }
	if f := s.Flush(); !f.Saved {
		t.Fatalf("written once the game is closed: %+v", f)
	}
}

// DBA-17: the file's size and time are taken before its content is read. A game that rewrites the file between the two must not be taken for the file that was loaded.
func TestAFileRewrittenWhileItWasBeingLoadedIsSeenAsChanged(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	other := makeSave(t, `create table t(a integer); insert into t values (1),(2),(3),(4),(5),(6);`)
	newer, _ := os.ReadFile(other)
	s := &Save{Path: p}
	done := false
	afterLoadRead = func() {
		if done {
			return
		}
		done = true
		time.Sleep(20 * time.Millisecond)
		if err := os.WriteFile(p, newer, 0o644); err != nil { // the game saves after this editor read the file
			t.Error(err)
		}
	}
	t.Cleanup(func() { afterLoadRead = nil })
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.ChangedOnDisk() {
		t.Fatal("the content that was loaded is the old file; the file on disk is newer, and the editor must say so")
	}
}
