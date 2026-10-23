package save

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
}

// fakeAuto writes a file named as the editor names the automatic backup of s, taken i seconds after 2026-01-01 00:00:00.
func fakeAuto(t *testing.T, s *Save, i int) string {
	t.Helper()
	ext := filepath.Ext(s.Path)
	base := strings.TrimSuffix(filepath.Base(s.Path), ext)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).Add(time.Duration(i) * time.Second)
	name := filepath.Join(s.backupDir(), fmt.Sprintf("%s-%s-%d%s%s", base, at.Format("20060102-150405"), 123, autoTag, ext))
	if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// The pruning keeps the newest keepBackups automatic backups, the oldest one, the first of the session and the one just taken, and touches nothing else in the folder.
func TestPruningNeverRemovesTheFirstTheOldestTheJustTakenOrAForeignFile(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	if err := os.MkdirAll(s.backupDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	var autos []string
	for i := 0; i < keepBackups+20; i++ {
		autos = append(autos, fakeAuto(t, s, 100+i)) // autos[0] is the oldest
	}
	first := fakeAuto(t, s, 50) // the session's first backup, taken after the oldest but long before the newest ones
	oldest := fakeAuto(t, s, 1)
	// a backup whose time is older than 60 others is the one just taken (the clock was set back): it must survive too
	justTaken := fakeAuto(t, s, 60)
	s.firstBackup = first
	foreign := map[string]string{
		"a person's copy":              filepath.Join(s.backupDir(), "game-20261007-120000-1.db"),
		"another save's automatic one": filepath.Join(s.backupDir(), "other-20200101-000000-5"+autoTag+".db"),
		"a note with the tag":          filepath.Join(s.backupDir(), "game-auto-notes.txt"),
		"a name that is not a time":    filepath.Join(s.backupDir(), "game-x-y"+autoTag+".db"),
	}
	for _, f := range foreign {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(f, time.Now().Add(-1000*time.Hour), time.Now().Add(-1000*time.Hour))
	}
	s.pruneBackups(justTaken)
	for what, f := range foreign {
		if !exists(f) {
			t.Fatalf("%s was removed, the editor did not make it", what)
		}
	}
	for name, f := range map[string]string{"the oldest": oldest, "the first of the session": first, "the one just taken": justTaken, "the newest": autos[len(autos)-1]} {
		if !exists(f) {
			t.Fatalf("%s was removed", name)
		}
	}
	left := 0
	for _, a := range append(autos, oldest, first, justTaken) {
		if exists(a) {
			left++
		}
	}
	if left != keepBackups+3 { // the newest keepBackups, and the three that are kept besides
		t.Fatalf("%d automatic backups left, want %d", left, keepBackups+3)
	}
	if exists(autos[0]) {
		t.Fatal("an old backup beyond the limit was kept")
	}
}

// Through the write path: the first backup of a session survives more than keepBackups bursts, and so does the one just taken.
func TestThePreSessionBackupSurvivesManyBursts(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	setA(t, s, 2)
	f := s.Flush()
	if !f.Saved || f.Backup == "" {
		t.Fatalf("%+v", f)
	}
	original, _ := os.ReadFile(f.Backup)
	for i := 0; i < keepBackups+3; i++ {
		s.lastBackupT = time.Time{} // the burst is over: the next write takes a backup of its own
		setA(t, s, 10+i)
		f2 := s.Flush()
		if !f2.Saved || f2.Backup == f.Backup {
			t.Fatalf("burst %d: %+v", i, f2)
		}
		if !exists(f2.Backup) {
			t.Fatalf("burst %d: the backup just taken is gone: %s", i, f2.Backup)
		}
		time.Sleep(2 * time.Millisecond)
	}
	now, err := os.ReadFile(f.Backup)
	if err != nil || !bytes.Equal(now, original) {
		t.Fatalf("the first backup of the session (the file before the editor wrote anything) was removed or changed: %v", err)
	}
	n := 0
	for _, b := range s.Backups() {
		if strings.Contains(fmt.Sprint(b["name"]), autoTag) {
			n++
		}
	}
	if n > keepBackups+2 {
		t.Fatalf("%d automatic backups, the limit is %d plus the oldest and the first", n, keepBackups)
	}
}

// A file with a time in the future, or one the editor did not name, does not stop a backup being taken.
func TestAForeignOrFutureDatedFileDoesNotSuppressTheBackup(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	if err := os.MkdirAll(s.backupDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(10 * 365 * 24 * time.Hour)
	for _, n := range []string{"junk.txt", "game-20990101-000000-1" + autoTag + ".db", "game-20990101-000000-2.db"} {
		f := filepath.Join(s.backupDir(), n)
		os.WriteFile(f, []byte("x"), 0o600)
		os.Chtimes(f, future, future)
	}
	setA(t, s, 2)
	f := s.Flush()
	if !f.Saved || f.Backup == "" {
		t.Fatalf("%+v", f)
	}
	b, err := os.ReadFile(f.Backup)
	if err != nil || hashOf(b) == hashOf([]byte("x")) || len(b) < 100 {
		t.Fatalf("the backup is a copy of the file before the edit: %v %d bytes", err, len(b))
	}
}

// A burst's backup is reused only while it still holds the bytes it was taken with.
func TestABurstsBackupThatChangedIsNotReused(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	setA(t, s, 2)
	f1 := s.Flush()
	if !f1.Saved {
		t.Fatalf("%+v", f1)
	}
	if err := os.WriteFile(f1.Backup, []byte("not the save"), 0o600); err != nil {
		t.Fatal(err)
	}
	setA(t, s, 3)
	f2 := s.Flush()
	if !f2.Saved || f2.Backup == f1.Backup {
		t.Fatalf("a backup that no longer holds what it held is not the restore point: %+v", f2)
	}
	if b, err := os.ReadFile(f2.Backup); err != nil || len(b) < 100 {
		t.Fatalf("the new backup is a copy of the file: %v", err)
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
