package save

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// holdAQueue leaves s with a queue held because the game used an id the queue also adds.
func holdAQueue(t *testing.T, s *Save, p string) {
	t.Helper()
	if _, err := s.Mutate("add row 3", func(m *Mut) error { _, err := m.Exec(`insert into jt values (3,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	other, _ := Open(p)
	if _, err := other.Exec(`insert into jt values (3,'the game''s',9.5,x'09',9)`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	if f := s.Flush(); f.Saved || s.Held() == nil {
		t.Fatalf("the queue should be held: %+v", f)
	}
}

// Reload (Rebase) on a held queue used to read the file again and put nothing back: the hold and the journal were gone and the answer was ok.
func TestRebaseOfAHeldQueueIsRefusedAndKeepsTheQueue(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	holdAQueue(t, s, p)
	r, err := s.Rebase()
	if err == nil || !errors.Is(err, apierr.ErrQueueHeld) {
		t.Fatalf("Rebase of a held queue must be refused with queue_held: %+v %v", r, err)
	}
	if s.Held() == nil || !s.Dirty() || len(s.Pending()) != 1 {
		t.Fatalf("the queue is still held: held=%v dirty=%v pending=%v", s.Held(), s.Dirty(), s.Pending())
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); err != nil {
		t.Fatalf("the journal is still there: %v", err)
	}
}
