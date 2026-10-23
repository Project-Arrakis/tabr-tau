package save

import (
	"testing"
	"time"
)

// The automatic refresh decides "nothing is queued" and reads the file again in one step: an edit that lands while it waits for the write lock is kept, not thrown away.
func TestDiscardIfCleanKeepsAnEditThatLandsWhileItWaits(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	s.flushMu.Lock() // an automatic write is running: DiscardIfClean waits for it
	type res struct {
		cleaned bool
		err     error
	}
	done := make(chan res, 1)
	go func() { c, err := s.DiscardIfClean(); done <- res{c, err} }()
	time.Sleep(100 * time.Millisecond)
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	s.flushMu.Unlock()
	r := <-done
	if r.err != nil || r.cleaned {
		t.Fatalf("an edit is waiting: nothing is discarded: %+v", r)
	}
	if !s.Dirty() || rowName(t, s, 1) != "uno" {
		t.Fatalf("the edit is still there: dirty=%v name=%q", s.Dirty(), rowName(t, s, 1))
	}
}

func TestDiscardIfCleanReadsTheFileWhenNothingIsQueuedAndKeepsAHeldQueue(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	other, _ := Open(p) // the game saved
	if _, err := other.Exec(`update jt set name='game' where a=2`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	if c, err := s.DiscardIfClean(); err != nil || !c {
		t.Fatalf("nothing queued: the file is read again: %v %v", c, err)
	}
	if rowName(t, s, 2) != "game" {
		t.Fatalf("the game's file is loaded: %q", rowName(t, s, 2))
	}
	if _, err := s.Mutate("add row 3", func(m *Mut) error { _, err := m.Exec(`insert into jt values (3,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	g, _ := Open(p) // the game saved and used the same id
	if _, err := g.Exec(`insert into jt values (3,'the game''s',9.5,x'09',9)`); err != nil {
		t.Fatal(err)
	}
	if _, err := g.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	g.Close()
	if f := s.Flush(); f.Saved || s.Held() == nil {
		t.Fatalf("the queue should be held: %+v", f)
	}
	if c, err := s.DiscardIfClean(); err != nil || c || s.Held() == nil {
		t.Fatalf("a held queue is kept: %v %v held=%v", c, err, s.Held())
	}
}
