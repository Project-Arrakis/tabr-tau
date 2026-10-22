package save

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setA(t *testing.T, s *Save, v int) {
	t.Helper()
	if _, err := s.Exec(`update t set a=?`, v); err != nil {
		t.Fatal(err)
	}
}

func fileA(t *testing.T, p string) int64 {
	t.Helper()
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.One(`select a from t`)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	return r["a"].(int64)
}

func TestFlushWritesTheEditAtOnceWhenTheFileCanBeWritten(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	if f := s.Flush(); f.Saved || f.Queued != 0 || s.LastFlush() != nil {
		t.Fatalf("nothing is pending, so there is nothing to write: %+v", f)
	}
	setA(t, s, 2)
	f := s.Flush()
	if !f.Saved || f.Queued != 0 || f.Backup == "" || len(f.Ops) != 1 {
		t.Fatalf("flush: %+v", f)
	}
	if s.Dirty() {
		t.Fatal("a written edit is no longer pending")
	}
	if got := fileA(t, p); got != 2 {
		t.Fatalf("the game's file holds %d, want 2", got)
	}
	if l := s.LastFlush(); l == nil || !l.Saved {
		t.Fatalf("the last write is kept for the page: %+v", l)
	}
}

func TestFlushQueuesTheEditWhileASinglePlayerSessionIsActiveAndWritesItWhenItEnds(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	setA(t, s, 2)
	restore := ActiveSessionForTests()
	f := s.Flush()
	if f.Saved || f.Queued != 1 || f.Why == "" {
		t.Fatalf("an active session blocks the write: %+v", f)
	}
	if !s.Dirty() || fileA(t, p) != 1 {
		t.Fatal("the edit waits, and the file is untouched")
	}
	restore() // the game was closed
	if f := s.Flush(); !f.Saved || f.Queued != 0 {
		t.Fatalf("the waiting edit is written once the file can be: %+v", f)
	}
	if got := fileA(t, p); got != 2 {
		t.Fatalf("file %d", got)
	}
}

func TestFlushPutsTheEditsBackOntoAFileTheGameSavedSince(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); create table u(b integer); insert into t values (1); insert into u values (5);`)
	s := openJournaled(t, p, t.TempDir())
	setA(t, s, 2)
	// the game saves (another table changes) while the editor holds its edit
	other, _ := Open(p)
	if _, err := other.Exec(`update u set b=6`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Commit(false); err != nil {
		t.Fatal(err)
	}
	other.Close()
	f := s.Flush()
	if !f.Saved || f.Restored == nil || f.Restored.Applied != 1 {
		t.Fatalf("the edit is put back onto the newer file and written: %+v", f)
	}
	chk, _ := Open(p)
	defer chk.Close()
	a, _ := chk.One(`select a from t`)
	b, _ := chk.One(`select b from u`)
	if a["a"] != int64(2) || b["b"] != int64(6) {
		t.Fatalf("both the game's change and the edit are in the file: %v %v", a, b)
	}
}

func TestAutomaticWritesShareOneBackupPerBurstAndKeepTheNewest(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	for i := 2; i <= 4; i++ {
		setA(t, s, i)
		if f := s.Flush(); !f.Saved {
			t.Fatalf("flush %d: %+v", i, f)
		}
	}
	if n := len(s.Backups()); n != 1 {
		t.Fatalf("three edits in a burst take one backup, found %d", n)
	}
	// older backups beyond the newest keepBackups are removed
	for i := 0; i < keepBackups+5; i++ {
		name := filepath.Join(s.backupDir(), "old-"+time.Now().Add(time.Duration(i)*time.Second).Format("150405")+"-"+string(rune('a'+i%26))+string(rune('a'+i/26))+".db")
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour).Add(time.Duration(i) * time.Second)
		os.Chtimes(name, old, old)
	}
	s.pruneBackups()
	if n := len(s.Backups()); n != keepBackups {
		t.Fatalf("kept %d backups, want %d", n, keepBackups)
	}
	if newest, _ := s.newestBackup(); newest == "" {
		t.Fatal("the newest backup is kept")
	}
}
