package save

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func mutateJT(t *testing.T, s *Save, desc, q string) {
	t.Helper()
	if _, err := s.Mutate(desc, func(m *Mut) error { _, err := m.Exec(q); return err }); err != nil {
		t.Fatalf("%s: %v", desc, err)
	}
}

// When an edit cannot be put back, the ones that work on the rows it was to leave must not be put back either: they would act on the game's own rows with the same id. The
// edits that have nothing to do with it still go back. (This is the path of Reload now and of a restart; the automatic write holds the whole queue instead.)
func TestAnEditThatDependsOnOneThatFailedIsNotPutBack(t *testing.T) {
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	mutateJT(t, s, "add row 3", `insert into jt values (3,'ours',0.5,x'00',7)`)
	mutateJT(t, s, "rename row 3", `update jt set name='ours 2' where a=3`)
	mutateJT(t, s, "rename row 1", `update jt set name='uno' where a=1`)
	gameSavesRow3(t, p)
	r, err := s.Rebase()
	if err != nil || r == nil {
		t.Fatalf("%+v %v", r, err)
	}
	if got := rowName(t, s, 3); got != "the game's" {
		t.Fatalf("the edit that worked on the failed edit's row changed the game's row: %q", got)
	}
	if got := rowName(t, s, 1); got != "uno" {
		t.Fatalf("an edit that has nothing to do with it is still put back: %q", got)
	}
	if r.Applied != 1 || len(r.Conflicts) != 2 {
		t.Fatalf("one applied, two reported: %+v", r)
	}
	for _, c := range r.Conflicts {
		if c.Desc == "rename row 3" && !strings.Contains(c.Why, `"add row 3"`) {
			t.Fatalf("the reason names the edit it depends on: %+v", c)
		}
	}
}

func TestTheEditsOfATornDownRestartAreSubjectToTheSameRule(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	mutateJT(t, s1, "add row 3", `insert into jt values (3,'ours',0.5,x'00',7)`)
	mutateJT(t, s1, "rename row 3", `update jt set name='ours 2' where a=3`)
	s1.Close()
	gameSavesRow3(t, p)
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	if got := rowName(t, s2, 3); got != "the game's" {
		t.Fatalf("the game's row is untouched: %q", got)
	}
}

// Nothing can come in between two of the edits being put back: a new edit made while they are is made after them, not among them, and the journal file holds the queue until
// the edits that went back have replaced it.
func TestNothingComesInBetweenTheEditsBeingPutBackAndTheJournalIsKept(t *testing.T) {
	p := makeSave(t, journalDDL)
	jdir := t.TempDir()
	s := openJournaled(t, p, jdir)
	mutateJT(t, s, "rename row 1", `update jt set name='uno' where a=1`)
	mutateJT(t, s, "rename row 2", `update jt set name='dos' where a=2`)
	gameSavesRow3(t, p) // the file differs, so the edits are put back one by one
	var finished atomic.Bool
	var journalThere atomic.Bool
	done := make(chan struct{})
	started := false
	replayStep = func() {
		if _, err := os.Stat(s.journalPath); err == nil {
			journalThere.Store(true)
		}
		if started {
			return
		}
		started = true
		go func() {
			defer close(done)
			if _, err := s.Mutate("late edit", func(m *Mut) error { _, err := m.Exec(`update jt set n=99 where a=2`); return err }); err != nil {
				t.Error(err)
			}
			finished.Store(true)
		}()
		time.Sleep(100 * time.Millisecond) // long enough for it to get in if it could
		if finished.Load() {
			t.Error("an edit got in between two of the edits being put back")
		}
	}
	defer func() { replayStep = nil }()
	r, err := s.Rebase()
	if err != nil || r == nil || r.Applied != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	<-done
	got := s.Pending()
	if len(got) != 3 || got[0] != "rename row 1" || got[1] != "rename row 2" || got[2] != "late edit" {
		t.Fatalf("the late edit comes after the edits that were put back: %v", got)
	}
	if !journalThere.Load() {
		t.Fatal("the journal file was gone while the edits were being put back")
	}
}
