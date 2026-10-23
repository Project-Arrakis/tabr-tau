package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The journal keeps the pending edits in a file, and stops (for the session, removing the file) rather than keep only some of them. These two tests run an edit past each of the
// limits and check what the person is left with: the edit itself is still made and waits to be written, and the journal says why it is off.

func TestTheJournalStopsWhenThePendingEditsAreTooLargeToKeep(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a 65 MB value")
	}
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	jp := filepath.Join(dir, "pending.json")
	if _, err := s.Mutate("small edit", func(m *Mut) error { _, err := m.Exec(`update jt set name='small' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jp); err != nil {
		t.Fatalf("a small edit is kept: %v", err)
	}
	big := strings.Repeat("x", journalMaxBytes+1)
	if _, err := s.Mutate("large edit", func(m *Mut) error { _, err := m.Exec(`update jt set name=? where a=2`, big); return err }); err != nil {
		t.Fatal(err)
	}
	if why := s.JournalOff(); !strings.Contains(why, "too large") {
		t.Fatalf("the journal says why it stopped: %q", why)
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatalf("a journal that holds only some of the edits is removed: %v", err)
	}
	if got := s.Pending(); len(got) != 2 || !s.Dirty() {
		t.Fatalf("both edits are still pending: %v", got)
	}
	if _, err := s.Mutate("after", func(m *Mut) error { _, err := m.Exec(`update jt set name='after' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatal("once off, the journal is not started again by a later edit")
	}
}

func TestTheJournalStopsWhenThereAreTooManyStatementsToKeep(t *testing.T) {
	if testing.Short() {
		t.Skip("runs 200,000 statements")
	}
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	n := 0
	if _, err := s.Mutate("many statements", func(m *Mut) error {
		for ; n <= journalMaxStmts; n++ { // one more than the limit
			if _, err := m.Exec(`update jt set n=? where a=1`, n); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if why := s.JournalOff(); !strings.Contains(why, "too many statements") {
		t.Fatalf("the journal says why it stopped: %q", why)
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); !os.IsNotExist(err) {
		t.Fatalf("no journal file is left: %v", err)
	}
	if !s.Dirty() || len(s.Pending()) != 1 {
		t.Fatalf("the edit is still pending: %v", s.Pending())
	}
}
