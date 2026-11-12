package save

import (
	"path/filepath"
	"testing"
)

// An edit with no description is written like any other (#592): it is listed, the write carries its rows into the file, and the queue is empty afterwards.
func TestAnUnnamedEditIsWrittenAndTheQueueEmptiesAfterwards(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Mutate("", func(m *Mut) error { _, err := m.Exec(`update t set a=5`); return err }); err != nil {
		t.Fatal(err)
	}
	if dirty, pending := s.QueueState(); !dirty || len(pending) != 1 {
		t.Fatalf("dirty=%v pending=%v", dirty, pending)
	}
	f := s.Flush()
	if !f.Saved || len(f.Ops) != 1 || f.Ops[0] != "unnamed change (1 rows)" {
		t.Fatalf("flush: %+v", f)
	}
	if got := fileA(t, p); got != 5 {
		t.Fatalf("the game's file holds %d, want 5: the unnamed edit must be written", got)
	}
	if dirty, pending := s.QueueState(); dirty || len(pending) != 0 {
		t.Fatalf("after the write: dirty=%v pending=%v", dirty, pending)
	}
}

// Kept in the journal and put back after a restart, as a named edit is: it is not a reason to stop keeping the queue.
func TestAnUnnamedEditIsKeptInTheJournalAndPutBack(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("", func(m *Mut) error { _, err := m.Exec(`update jt set name=? where a=?`, "uno", 1); return err }); err != nil {
		t.Fatal(err)
	}
	if why := s1.JournalOff(); why != "" {
		t.Fatalf("the journal was switched off: %s", why)
	}
	s1.Close()
	s2 := openJournaled(t, p, filepath.Clean(dir))
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if got := s2.Pending(); len(got) != 1 || got[0] != "unnamed change (1 rows)" {
		t.Fatalf("pending after restore: %v", got)
	}
	if rowName(t, s2, 1) != "uno" {
		t.Fatal("the edit should be back")
	}
}
