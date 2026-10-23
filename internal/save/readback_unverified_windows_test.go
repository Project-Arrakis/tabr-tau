//go:build windows

package save

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// holdAfterRename makes the save unreadable for other programs as soon as the editor has replaced it, the way a scanner or an indexer opening the fresh file exclusively does.
// release lets go of it. The handle is taken once, for the first replacement of the save itself.
func holdAfterRename(t *testing.T, p string) (release func()) {
	t.Helper()
	var h syscall.Handle
	var held bool
	old := fsRename
	fsRename = func(a, b string) error {
		if err := os.Rename(a, b); err != nil || b != p || held {
			return err
		}
		name, _ := syscall.UTF16PtrFromString(b)
		var err error
		h, err = syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			t.Fatal(err)
		}
		held = true
		return nil
	}
	released := false
	release = func() {
		fsRename = old
		if held && !released {
			released = true
			syscall.CloseHandle(h)
		}
	}
	t.Cleanup(release)
	return release
}

func countRows(t *testing.T, s *Save, n int) int64 {
	t.Helper()
	rows, err := s.Query(`select count(*) c from ev where n=?`, n)
	if err != nil {
		t.Fatal(err)
	}
	return rows[0]["c"].(int64)
}

func addEvent(t *testing.T, s *Save, n int) {
	t.Helper()
	if _, err := s.Mutate("add event", func(m *Mut) error { _, err := m.Exec(`insert into ev values (?)`, n); return err }); err != nil {
		t.Fatal(err)
	}
}

// The file was replaced but could not be read back for longer than the retries last. The edits are in the file, so the next attempt must not put them in a second time.
func TestAWriteThatCouldNotBeReadBackIsNotAppliedTwice(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	addEvent(t, s, 1)
	release := holdAfterRename(t, p)
	f := s.Flush()
	if f.Saved {
		t.Fatalf("a write that could not be checked is not reported as saved: %+v", f)
	}
	if !strings.Contains(f.Why, "could not be verified") || !strings.Contains(f.Why, "written") {
		t.Fatalf("the message says the file was written and could not be verified: %q", f.Why)
	}
	release()
	f2 := s.Flush()
	if !f2.Saved || f2.Queued != 0 {
		t.Fatalf("the written file is recognised on the next attempt: %+v", f2)
	}
	if f2.Restored != nil && f2.Restored.Applied > 0 {
		t.Fatalf("nothing is put back onto a file that already has the edits: %+v", f2.Restored)
	}
	if s.Dirty() {
		t.Fatal("nothing is pending")
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); err == nil {
		t.Fatal("the journal is gone once the edits are known to be written")
	}
	if n := countRows(t, s, 1); n != 1 {
		t.Fatalf("the working copy has the edit %d times", n)
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, s, 1); n != 1 {
		t.Fatalf("the file has the edit %d times", n)
	}
}

// An edit made while the write was unverified is written, once, together with nothing of the first.
func TestAnEditMadeWhileTheWriteWasUnverifiedIsKept(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	addEvent(t, s, 1)
	release := holdAfterRename(t, p)
	if f := s.Flush(); f.Saved {
		t.Fatalf("unverified: %+v", f)
	}
	addEvent(t, s, 2)
	release()
	f := s.Flush()
	if !f.Saved {
		t.Fatalf("the later edit is written: %+v", f)
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if a, b := countRows(t, s, 1), countRows(t, s, 2); a != 1 || b != 1 {
		t.Fatalf("each edit is in the file once: n=1 %d times, n=2 %d times", a, b)
	}
}

// A scanner that lets go after a moment: the read-back is tried again with a pause and the write is reported as it is.
func TestAReadBackThatSucceedsOnALaterTryIsASuccess(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)
	s := openJournaled(t, p, t.TempDir())
	addEvent(t, s, 1)
	release := holdAfterRename(t, p)
	go func() { time.Sleep(300 * time.Millisecond); release() }()
	f := s.Flush()
	if !f.Saved {
		t.Fatalf("the file could be read on a later try: %+v", f)
	}
	if n := countRows(t, s, 1); n != 1 {
		t.Fatalf("%d rows", n)
	}
}

// After a restart between the unverified write and the next attempt, the journal says which file the edits were written as: they are not put back.
func TestARestartAfterAnUnverifiedWriteDoesNotPutTheEditsBackAgain(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	addEvent(t, s, 1)
	release := holdAfterRename(t, p)
	if f := s.Flush(); f.Saved {
		t.Fatalf("unverified: %+v", f)
	}
	release()
	s.Close()
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil {
		t.Fatal(err)
	}
	if r != nil && r.Applied > 0 || s2.Dirty() {
		t.Fatalf("nothing to put back: %+v dirty=%v", r, s2.Dirty())
	}
	if n := countRows(t, s2, 1); n != 1 {
		t.Fatalf("%d rows", n)
	}
}

// An edit made after the unverified write is in the journal after the ones that were written: a restart puts back only that one.
func TestARestartPutsBackOnlyTheEditsMadeAfterAnUnverifiedWrite(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	addEvent(t, s, 1)
	release := holdAfterRename(t, p)
	if f := s.Flush(); f.Saved {
		t.Fatalf("unverified: %+v", f)
	}
	addEvent(t, s, 2)
	release()
	s.Close()
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("only the later edit is put back: %+v", r)
	}
	if a, b := countRows(t, s2, 1), countRows(t, s2, 2); a != 1 || b != 1 {
		t.Fatalf("n=1 %d times, n=2 %d times", a, b)
	}
}

// Reloading while the write is unverified (the page offers it when the file looks changed) must not replay the edits onto the file that has them.
func TestAReloadAfterAnUnverifiedWriteDoesNotPutTheEditsBackAgain(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)
	s := openJournaled(t, p, t.TempDir())
	addEvent(t, s, 1)
	release := holdAfterRename(t, p)
	if f := s.Flush(); f.Saved {
		t.Fatalf("unverified: %+v", f)
	}
	release()
	if _, err := s.Rebase(); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, s, 1); n != 1 {
		t.Fatalf("the edit is in the working copy %d times", n)
	}
	if s.Dirty() {
		t.Fatal("nothing is pending")
	}
}

// If the file turns out not to be the one that was written (the game saved after it, or the replacement never took effect), the usual path applies: the edits are put back onto it.
func TestAnUnverifiedWriteThatTurnsOutToBeAnotherFileIsNotTakenForWritten(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)
	s := openJournaled(t, p, t.TempDir())
	addEvent(t, s, 1)
	release := holdAfterRename(t, p)
	if f := s.Flush(); f.Saved {
		t.Fatalf("unverified: %+v", f)
	}
	release()
	newer, err := os.ReadFile(makeSave(t, `create table ev(n integer); insert into ev values (0),(99);`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, newer, 0o600); err != nil { // the game's own save
		t.Fatal(err)
	}
	f := s.Flush()
	if !f.Saved || f.Restored == nil || f.Restored.Applied != 1 {
		t.Fatalf("the edit is put back onto the game's file and written: %+v", f)
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if a, b := countRows(t, s, 1), countRows(t, s, 99); a != 1 || b != 1 {
		t.Fatalf("the edit once (%d) and the game's own row kept (%d)", a, b)
	}
}
