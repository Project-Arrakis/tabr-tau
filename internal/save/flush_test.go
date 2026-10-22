package save

import (
	"encoding/hex"
	"encoding/json"
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
	// the backups an automatic write took are kept to the newest keepBackups; every other backup (a person's, an earlier version's) is never removed
	before := len(s.Backups())
	for i := 0; i < keepBackups+5; i++ {
		name := filepath.Join(s.backupDir(), "old-"+time.Now().Add(time.Duration(i)*time.Second).Format("150405")+"-"+string(rune('a'+i%26))+string(rune('a'+i/26))+autoTag+".db")
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour).Add(time.Duration(i) * time.Second)
		os.Chtimes(name, old, old)
	}
	manual := filepath.Join(s.backupDir(), "game-20261007-120000-1.db")
	if err := os.WriteFile(manual, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(manual, time.Now().Add(-100*24*time.Hour), time.Now().Add(-100*24*time.Hour))
	s.pruneBackups()
	if n := len(s.Backups()); n != keepBackups+1 {
		t.Fatalf("kept %d backups, want the newest %d automatic ones and the one that is not (had %d before)", n, keepBackups, before)
	}
	if _, err := os.Stat(manual); err != nil {
		t.Fatal("a backup an automatic write did not take was removed")
	}
	if newest, _ := s.newestBackup(); newest == "" {
		t.Fatal("the newest backup is kept")
	}
}

// A file the game saved since the last automatic write is overwritten only after a backup of its own: the burst's backup holds the state before the burst, not that one.
func TestABackupIsNotReusedForAFileTheGameSavedSince(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); create table u(b integer); insert into t values (1); insert into u values (5);`)
	s := openJournaled(t, p, t.TempDir())
	setA(t, s, 2)
	f1 := s.Flush()
	setA(t, s, 3)
	f2 := s.Flush()
	if !f1.Saved || !f2.Saved || f1.Backup != f2.Backup || len(s.Backups()) != 1 {
		t.Fatalf("two edits in a burst share the backup taken before the first: %+v %+v %d", f1, f2, len(s.Backups()))
	}
	other, _ := Open(p) // the game saves
	if _, err := other.Exec(`update u set b=6`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Commit(false); err != nil {
		t.Fatal(err)
	}
	other.Close()
	setA(t, s, 4)
	f3 := s.Flush()
	if !f3.Saved || f3.Backup == f1.Backup {
		t.Fatalf("the game's newer file must be backed up before it is overwritten: %+v", f3)
	}
}

// The journal outliving a write must not put the edits back a second time.
func TestAJournalThatOutlivedItsWriteIsNotReplayed(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("add a row", func(m *Mut) error { _, err := m.Exec(`insert into t values (2)`); return err }); err != nil {
		t.Fatal(err)
	}
	jp := filepath.Join(dir, "pending.json")
	kept, err := os.ReadFile(jp)
	if err != nil {
		t.Fatal(err)
	}
	if f := s.Flush(); !f.Saved {
		t.Fatalf("%+v", f)
	}
	s.Close()
	// the write happened but the journal could not be removed: put it back as it was while the file was being written
	var j journalFile
	if err := json.Unmarshal(kept, &j); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	h := hashOf(b)
	j.Writing = hex.EncodeToString(h[:])
	out, _ := json.Marshal(j)
	if err := os.WriteFile(jp, out, 0o600); err != nil {
		t.Fatal(err)
	}
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r != nil || s2.Dirty() {
		t.Fatalf("the edits are in the file already: %+v %v dirty=%v", r, err, s2.Dirty())
	}
	if _, err := os.Stat(jp); err == nil {
		t.Fatal("the journal that outlived its write is removed")
	}
	row, _ := s2.One(`select count(*) n from t`)
	if row["n"].(int64) != 2 {
		t.Fatalf("one row was added, once: %v", row)
	}
}

// An edit that does not apply to the game's newer file must not leave the ones after it acting on the game's own rows: the whole queue is held, nothing is written, and the
// game's file is left as it is.
func TestAQueueThatConflictsWithTheGamesNewerFileIsHeldWholeAndNothingIsWritten(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	if _, err := s.Mutate("add row 3", func(m *Mut) error { _, err := m.Exec(`insert into jt values (3,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate("rename row 3", func(m *Mut) error { _, err := m.Exec(`update jt set name='ours 2' where a=3`); return err }); err != nil {
		t.Fatal(err)
	}
	other, _ := Open(p) // the game saved and used the same id for a row of its own
	if _, err := other.Exec(`insert into jt values (3,'the game''s',9.5,x'09',9)`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Commit(false); err != nil {
		t.Fatal(err)
	}
	other.Close()
	f := s.Flush()
	if f.Saved || f.Why == "" || s.Held() == nil {
		t.Fatalf("the queue is held, not written: %+v", f)
	}
	if got := rowName(t, s, 3); got != "the game's" {
		t.Fatalf("the game's row must be untouched in the editor's copy: %q", got)
	}
	chk, _ := Open(p)
	defer chk.Close()
	if r, _ := chk.One(`select name from jt where a=3`); r["name"] != "the game's" {
		t.Fatalf("the file on disk is the game's: %v", r)
	}
	if !s.Dirty() || len(s.Pending()) != 2 {
		t.Fatalf("the held edits still count as the queue: dirty=%v %v", s.Dirty(), s.Pending())
	}
	if _, err := s.Mutate("another", func(m *Mut) error { _, err := m.Exec(`update jt set name='x' where a=1`); return err }); err == nil {
		t.Fatal("nothing new is accepted while a queue is held")
	}
	if again := s.Flush(); again.Saved || again.Why == "" {
		t.Fatalf("trying again with the same file changes nothing: %+v", again)
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if s.Held() != nil || s.Dirty() {
		t.Fatal("Discard drops the held queue")
	}
}
