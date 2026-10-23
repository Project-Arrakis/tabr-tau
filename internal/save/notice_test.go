package save

import (
	"errors"
	"testing"
)

func gameSavesRow3(t *testing.T, p string) {
	t.Helper()
	other, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.Exec(`insert into jt values (3,'the game''s',9.5,x'09',9)`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Commit(); err != nil {
		t.Fatal(err)
	}
}

// Reloading with queued edits drops the ones that no longer apply and says which. That notice must outlive the write of the ones that did apply (the automatic write used to
// clear it), and the automatic refresh reading a newer file must not clear it either; only the person does.
func TestTheNoticeOfDroppedEditsSurvivesTheWriteAndTheRefresh(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	if _, err := s.Mutate("rename row 1", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate("add row 3", func(m *Mut) error { _, err := m.Exec(`insert into jt values (3,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	gameSavesRow3(t, p)
	r, err := s.Rebase()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 1 || r.Conflicts[0].Desc != "add row 3" {
		t.Fatalf("one edit applies and one is dropped: %+v %v", r, err)
	}
	if f := s.Flush(); !f.Saved {
		t.Fatalf("the edit that applied is written: %+v", f)
	}
	n := s.Restored()
	if n == nil || len(n.Conflicts) != 1 || n.Conflicts[0].Desc != "add row 3" || !n.Written {
		t.Fatalf("the notice of the dropped edit survives the write: %+v", n)
	}
	// the game saves again and, with nothing queued, the page reads the file by itself
	other, _ := Open(p)
	other.Exec(`update jt set n=77 where a=2`)
	other.Commit()
	other.Close()
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if s.Restored() == nil {
		t.Fatal("reading the newer file with nothing queued must not clear the notice of dropped edits")
	}
	s.DismissRestored()
	if s.Restored() != nil {
		t.Fatal("dismissing clears it")
	}
}

// A notice with nothing dropped is not kept once its edits are written, and throwing a queue away ends the notice about it.
func TestANoticeWithNothingDroppedGoesWithTheWriteAndWithTheDiscard(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	if _, err := s.Mutate("rename row 1", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	gameSavesRow3(t, p)
	if r, err := s.Rebase(); err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 || s.Restored() == nil {
		t.Fatalf("%+v %v", r, err)
	}
	if f := s.Flush(); !f.Saved {
		t.Fatalf("%+v", f)
	}
	if s.Restored() != nil {
		t.Fatalf("nothing was dropped, so the written edits leave no notice: %+v", s.Restored())
	}
	// a notice with a conflict, then the queue is discarded by the person
	if _, err := s.Mutate("rename row 2", func(m *Mut) error { _, err := m.Exec(`update jt set name='dos' where a=2`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate("add row 4", func(m *Mut) error { _, err := m.Exec(`insert into jt values (4,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	other, _ := Open(p)
	other.Exec(`insert into jt values (4,'the game''s',1,x'01',1)`)
	other.Commit()
	other.Close()
	if r, _ := s.Rebase(); r == nil || r.Applied != 1 || len(r.Conflicts) != 1 || s.Restored() == nil {
		t.Fatalf("%+v", r)
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if s.Restored() != nil {
		t.Fatal("the queue was discarded, so the notice about it goes")
	}
}

// Edits that were not kept (the journal is off) cannot be put back once the game has saved. The write says so (Stuck) instead of promising to try again.
func TestEditsThatWereNotKeptAreReportedAsStuckWhenTheGameSaved(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s, err := Open(p) // no journal enabled
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Mutate("rename row 1", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	gameSavesRow3(t, p)
	f := s.Flush()
	if f.Saved || !f.Stuck || f.Why == "" || f.Queued != 1 {
		t.Fatalf("the edit is stuck and says so: %+v", f)
	}
	if !errors.Is(notKeptError{"x"}, ErrNotKept) {
		t.Fatal("ErrNotKept matches")
	}
	chk, _ := Open(p)
	defer chk.Close()
	if r, _ := chk.One(`select name from jt where a=1`); r["name"] != "one" {
		t.Fatalf("the game's file is untouched: %v", r)
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if f := s.Flush(); f.Stuck || f.Saved {
		t.Fatalf("nothing queued, nothing to report: %+v", f)
	}
}
