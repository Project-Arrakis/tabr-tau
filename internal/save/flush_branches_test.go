package save

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// These tests pin what Flush reports when the edits are not written: each branch that returns early, and the two that follow a write. The page shows Why and Warning
// to the person, so a branch that dropped one would leave an edit waiting (or a reload that failed) with no explanation.

// theGameSaves replaces the file at p with another one, as the game does when it saves.
func theGameSaves(t *testing.T, p, ddl string) {
	t.Helper()
	b, err := os.ReadFile(makeSave(t, ddl))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A queue that was not kept (no journal) cannot be put back onto a newer file: the error is the answer, nothing is written, and the edit keeps waiting.
func TestFlushSaysWhyWhenTheEditsCannotBePutBackOntoTheGamesNewerFile(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, err := Open(p) // no journal enabled: the edits are not kept
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	setA(t, s, 2)
	theGameSaves(t, p, `create table t(a integer); insert into t values (7);`)
	f := s.Flush()
	if f.Saved || f.Queued != 1 || f.Restored != nil {
		t.Fatalf("nothing is written and the edit waits: %+v", f)
	}
	if !strings.Contains(f.Why, "were not kept") || !strings.Contains(f.Why, "cannot be put back") {
		t.Fatalf("the person is told why the edit cannot be put back, not that the game saved while it was being written: %q", f.Why)
	}
	if !s.Dirty() {
		t.Fatal("the edit is still pending")
	}
	if got := fileA(t, p); got != 7 {
		t.Fatalf("the game's file must be left as the game saved it, found %d", got)
	}
	if l := s.LastFlush(); l == nil || l.Why != f.Why || l.Saved {
		t.Fatalf("the page reads the same answer from the last write: %+v", l)
	}
}

// A queue that conflicts with the game's newer file is held whole: the answer names the edit that conflicted and why, says what was tried, and writes nothing.
func TestFlushNamesTheConflictThatHeldTheQueue(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	if _, err := s.Mutate("add row 3", func(m *Mut) error { _, err := m.Exec(`insert into jt values (3,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	theGameSaves(t, p, journalDDL+`insert into jt values (3,'the game''s',9.5,x'09',9);`)
	f := s.Flush()
	if f.Saved || f.Queued != 1 {
		t.Fatalf("the queue is held, not written: %+v", f)
	}
	if !strings.Contains(f.Why, "conflict") || !strings.Contains(f.Why, "add row 3") {
		t.Fatalf("the answer names the edit that conflicted: %q", f.Why)
	}
	if f.Restored == nil || f.Restored.Applied != 0 || len(f.Restored.Conflicts) != 1 || f.Restored.Conflicts[0].Desc != "add row 3" || !f.Restored.Rebased {
		t.Fatalf("what was tried is reported: %+v", f.Restored)
	}
	if h := s.Held(); h == nil || len(h.Conflicts) != 1 {
		t.Fatalf("the queue is held: %+v", h)
	}
	if got := fileName(t, p, 3); got != "the game's" {
		t.Fatalf("the game's file is left as it is, found %q", got)
	}
}

func fileName(t *testing.T, p string, a int) string {
	t.Helper()
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	return rowName(t, s, a)
}

// A held queue is tried again, from the edits it holds, once the game has saved a file they apply to.
func TestAHeldQueueIsWrittenOnceTheGameSavesAFileItAppliesTo(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	if _, err := s.Mutate("add row 3", func(m *Mut) error { _, err := m.Exec(`insert into jt values (3,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	theGameSaves(t, p, journalDDL+`insert into jt values (3,'the game''s',9.5,x'09',9);`)
	if f := s.Flush(); f.Saved || s.Held() == nil {
		t.Fatalf("the first attempt is held: %+v", f)
	}
	theGameSaves(t, p, journalDDL+`update jt set n=99 where a=2;`) // the row the game used is gone again
	f := s.Flush()
	if !f.Saved || f.Restored == nil || f.Restored.Applied != 1 || len(f.Restored.Conflicts) != 0 || s.Held() != nil || s.Dirty() {
		t.Fatalf("the held edit goes onto the newer file and is written: %+v", f)
	}
	chk, _ := Open(p)
	defer chk.Close()
	if rowName(t, chk, 3) != "ours" {
		t.Fatal("the edit is in the file")
	}
	if r, _ := chk.One(`select n from jt where a=2`); r["n"] != int64(99) {
		t.Fatalf("the game's own change is kept: %v", r)
	}
}

// The game saves between the check and the write: the write is refused, the answer says what happens next, and the next attempt puts the edit onto the new file.
func TestFlushSaysSoWhenTheGameSavedWhileTheEditsWereBeingWritten(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	// Flush asks about the game once itself and the write asks once more: the game saves in between.
	oldL := listProcs
	calls := 0
	listProcs = func(bool) ([]string, string) {
		calls++
		if calls == 2 {
			theGameSaves(t, p, journalDDL+`update jt set n=99 where a=2;`)
		}
		return nil, ""
	}
	restored := false
	restore := func() {
		if !restored {
			listProcs, restored = oldL, true
		}
	}
	defer restore()
	f := s.Flush()
	restore()
	if calls < 2 {
		t.Fatalf("the write did not ask about the game a second time (%d)", calls)
	}
	if f.Saved || f.Queued != 1 || !strings.Contains(f.Why, "the game saved while the edits were being written") {
		t.Fatalf("the refusal is explained: %+v", f)
	}
	if got := fileName(t, p, 1); got != "one" {
		t.Fatalf("nothing of ours was written over the game's file: %q", got)
	}
	f = s.Flush()
	if !f.Saved || f.Restored == nil || f.Restored.Applied != 1 {
		t.Fatalf("the next attempt puts the edit onto the game's file: %+v", f)
	}
	chk, _ := Open(p)
	defer chk.Close()
	if rowName(t, chk, 1) != "uno" {
		t.Fatal("the edit is in the file")
	}
	if r, _ := chk.One(`select n from jt where a=2`); r["n"] != int64(99) {
		t.Fatalf("the game's own change is kept: %v", r)
	}
}

// A write that fails for any other reason (here the backup cannot be made) is reported with its reason, leaves the file alone, and is tried again.
func TestFlushReportsTheErrorOfAWriteThatFailedAndTriesAgain(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	if err := os.WriteFile(s.backupDir(), []byte("in the way"), 0o600); err != nil { // a file where the backup folder belongs
		t.Fatal(err)
	}
	setA(t, s, 2)
	f := s.Flush()
	if f.Saved || f.Queued != 1 || f.Why == "" || f.Backup != "" {
		t.Fatalf("the failure is reported and the edit waits: %+v", f)
	}
	if !s.Dirty() || fileA(t, p) != 1 {
		t.Fatal("the edit is pending and the file is untouched")
	}
	if l := s.LastFlush(); l == nil || l.Why != f.Why {
		t.Fatalf("the page reads the same reason: %+v", l)
	}
	if err := os.Remove(s.backupDir()); err != nil {
		t.Fatal(err)
	}
	if f := s.Flush(); !f.Saved || f.Why != "" || fileA(t, p) != 2 {
		t.Fatalf("the next attempt writes it: %+v", f)
	}
}

// A save that is read-only for this session (a reload failed, say) never writes, and the answer says why.
func TestFlushOfAReadOnlySaveSaysWhy(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	setA(t, s, 2)
	s.mu.Lock()
	s.blocked = "restart the editor"
	s.mu.Unlock()
	f := s.Flush()
	if f.Saved || f.Queued != 1 || !strings.Contains(f.Why, "restart the editor") {
		t.Fatalf("%+v", f)
	}
	if fileA(t, p) != 1 {
		t.Fatal("nothing is written")
	}
}

// The file is written but could not be read back in: the answer is a write with a warning, not a failure, and the editor refuses further edits until it is restarted.
func TestFlushCarriesTheWarningOfAFileThatWasWrittenButCouldNotBeReloaded(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	setA(t, s, 2)
	old := reloadAfterCommit
	reloadAfterCommit = func(*Save) error { return errors.New("boom") }
	t.Cleanup(func() { reloadAfterCommit = old })
	f := s.Flush()
	if !f.Saved || f.Queued != 0 || f.Why != "" {
		t.Fatalf("the file was written: %+v", f)
	}
	if !strings.Contains(f.Warning, "boom") || !strings.Contains(f.Warning, "restart the editor") {
		t.Fatalf("the warning reaches the page: %q", f.Warning)
	}
	if l := s.LastFlush(); l == nil || l.Warning != f.Warning {
		t.Fatalf("the page reads the same warning: %+v", l)
	}
	if got := fileA(t, p); got != 2 {
		t.Fatalf("the file holds %d, want 2", got)
	}
	if s.WriteBlocked() == "" {
		t.Fatal("further edits are refused until the editor is restarted")
	}
}
