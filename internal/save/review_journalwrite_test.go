package save

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renameFailing(t *testing.T, fail func(call int) bool) {
	t.Helper()
	old, oldWait := fsRename, renameBackoff
	calls := 0
	fsRename = func(a, b string) error {
		calls++
		if fail(calls) {
			return errors.New("sharing violation")
		}
		return os.Rename(a, b)
	}
	renameBackoff = 0
	t.Cleanup(func() { fsRename, renameBackoff = old, oldWait })
}

// A scanner holding the journal for a moment must not cost the person the journal and the journaling.
func TestAJournalWriteThatFailsOnceIsTriedAgain(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	renameFailing(t, func(call int) bool { return call <= 2 })
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if why := s.JournalOff(); why != "" {
		t.Fatalf("the journal is still on: %q", why)
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); err != nil {
		t.Fatalf("the journal was written on a later attempt: %v", err)
	}
}

// When the new journal cannot replace the old for good, the old one stays on disk and the reason is reported.
func TestAJournalThatCannotBeReplacedLeavesTheOldOneInPlace(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(jp)
	if err != nil {
		t.Fatal(err)
	}
	renameFailing(t, func(int) bool { return true })
	if _, err := s.Mutate("rename two", func(m *Mut) error { _, err := m.Exec(`update jt set name='dos' where a=2`); return err }); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(jp)
	if err != nil || string(after) != string(before) {
		t.Fatalf("the journal that was kept stays as it was: %v", err)
	}
	if why := s.JournalOff(); !strings.Contains(why, "could not be written") {
		t.Fatalf("the reason is reported: %q", why)
	}
	if _, err := os.Stat(jp + ".tmp"); err == nil {
		t.Fatal("no temp file is left behind")
	}
}
