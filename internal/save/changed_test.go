package save

import (
	"os"
	"testing"
	"time"
)

// The game rewrites game.db while it runs; the editor must notice at once and not only when Save is pressed (UX-01 of the 2026-10-15 audit).
func TestChangedOnDiskNoticesTheFileBeingRewritten(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.ChangedOnDisk() {
		t.Fatal("a file just loaded has not changed")
	}
	// touched, not changed: the modification time moves, the content does not
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	if s.ChangedOnDisk() {
		t.Fatal("same content with a new time is not a change")
	}
	if s.ChangedOnDisk() {
		t.Fatal("and it stays unchanged on the next look")
	}
	// the game saves: different content
	other := makeSave(t, `create table t(a integer); insert into t values (9);`)
	b, _ := os.ReadFile(other)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.ChangedOnDisk() {
		t.Fatal("a different file must be noticed")
	}
	// a file that vanished is not trusted either
	os.Remove(p)
	if !s.ChangedOnDisk() {
		t.Fatal("a missing file counts as changed")
	}
}

func TestOurOwnSaveIsNotAChangeOnDisk(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	s, _ := Open(p)
	defer s.Close()
	s.ExecScript(`insert into t values (1)`)
	defer NoGameRunningForTests()()
	if _, err := s.Commit(false); err != nil {
		t.Fatal(err)
	}
	if s.ChangedOnDisk() {
		t.Fatal("the file this editor just wrote is the baseline, not a change")
	}
}
