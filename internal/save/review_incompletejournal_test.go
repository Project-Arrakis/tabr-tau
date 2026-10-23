package save

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func evSave(t *testing.T) string {
	t.Helper()
	return makeSave(t, `create table ev(n integer); insert into ev values (0);`)
}

func evEdit(t *testing.T, s *Save, n int) {
	t.Helper()
	if _, err := s.Mutate("add event "+strconv.Itoa(n), func(m *Mut) error { _, err := m.Exec(`insert into ev values (?)`, n); return err }); err != nil {
		t.Fatal(err)
	}
}

func evCount(t *testing.T, s *Save, n int) int64 {
	t.Helper()
	rows, err := s.Query(`select count(*) c from ev where n=?`, n)
	if err != nil {
		t.Fatal(err)
	}
	return rows[0]["c"].(int64)
}

// failJournalRenames makes the replacement of the journal fail (a scanner holds it) for as long as the returned function has not been called; renames of the save itself work.
func failJournalRenames(t *testing.T, p string) (stop func()) {
	t.Helper()
	old, oldWait := fsRename, renameBackoff
	renameBackoff = 0
	fsRename = func(a, b string) error {
		if strings.HasSuffix(b, "pending.json") {
			return fmt.Errorf("sharing violation")
		}
		return os.Rename(a, b)
	}
	stop = func() { fsRename, renameBackoff = old, oldWait }
	t.Cleanup(stop)
	return stop
}

// Edits 1 to 5 are kept, the journal write of edit 6 fails, the editor is closed with 6 pending. The restart must not report the five as the queue: it says that the kept list is
// not complete and puts nothing back.
func TestARestartDoesNotReplayAJournalThatCouldNotBeKeptUpToDate(t *testing.T) {
	defer NoGameRunningForTests()()
	p := evSave(t)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	for i := 1; i <= 5; i++ {
		evEdit(t, s, i)
	}
	failJournalRenames(t, p)
	evEdit(t, s, 6)
	if why := s.JournalOff(); !strings.Contains(why, "could not be written") {
		t.Fatalf("the page is told: %q", why)
	}
	s.Close()

	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.Applied != 0 || s2.Dirty() {
		t.Fatalf("nothing is put back from a list that is not the whole queue: %+v dirty=%v", r, s2.Dirty())
	}
	if len(r.Conflicts) != 1 || !strings.Contains(r.Conflicts[0].Why, "not the whole queue") || !strings.Contains(r.Conflicts[0].Why, "may be missing") {
		t.Fatalf("the report says the list is incomplete: %+v", r.Conflicts)
	}
	if !strings.Contains(r.Conflicts[0].Why, "add event 5") {
		t.Fatalf("the report names what was kept: %q", r.Conflicts[0].Why)
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); err == nil {
		t.Fatal("the stale journal is set aside, not left to be put back at the start after")
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "pending.json.skipped*")); len(m) != 1 {
		t.Fatalf("the journal is kept aside: %v", m)
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json.incomplete")); err == nil {
		t.Fatal("the mark is consumed by the report")
	}
	if evCount(t, s2, 3) != 0 {
		t.Fatal("no edit was applied")
	}
	// and the next start is quiet
	s2.Close()
	s3 := openJournaled(t, p, dir)
	if r3, err := s3.RestoreJournal(); err != nil || r3 != nil {
		t.Fatalf("reported once: %+v %v", r3, err)
	}
}

// When no journal could be written at all (the first edit), the restart still says that the queue was not kept.
func TestARestartReportsAQueueThatWasNeverKept(t *testing.T) {
	defer NoGameRunningForTests()()
	p := evSave(t)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	failJournalRenames(t, p)
	evEdit(t, s, 1)
	s.Close()
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || len(r.Conflicts) != 1 || !strings.Contains(r.Conflicts[0].Why, "none of the queued changes may have been kept") {
		t.Fatalf("reported: %+v", r)
	}
}

// With the journal off the marker of the write must still reach disk: a stop between the rename and the removal of the journal must not put the old edits onto a file that has them.
func TestAStopAfterTheRenameWithTheJournalOffDoesNotReplayTheOldEdits(t *testing.T) {
	defer NoGameRunningForTests()()
	p := evSave(t)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	s := openJournaled(t, p, dir)
	for i := 1; i <= 5; i++ {
		evEdit(t, s, i)
	}
	stop := failJournalRenames(t, p)
	evEdit(t, s, 6)
	// the editor is stopped right after the save was replaced: what the journal folder held at that moment is what a restart finds
	snap := map[string][]byte{}
	renameBefore := fsRename
	fsRename = func(a, b string) error {
		err := renameBefore(a, b)
		if err == nil && b == p {
			for _, f := range []string{jp, jp + ".incomplete"} {
				if data, rerr := os.ReadFile(f); rerr == nil {
					snap[f] = data
				}
			}
		}
		return err
	}
	f := s.Flush()
	stop()
	if !f.Saved {
		t.Fatalf("written: %+v", f)
	}
	s.Close()
	for _, name := range []string{jp, jp + ".incomplete"} {
		os.Remove(name)
	}
	for name, data := range snap {
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if len(snap) == 0 {
		t.Fatal("the journal folder held nothing at the rename")
	}
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil {
		t.Fatal(err)
	}
	if r != nil || s2.Dirty() {
		t.Fatalf("the edits are in the file already: %+v dirty=%v", r, s2.Dirty())
	}
	for i := 1; i <= 6; i++ {
		if n := evCount(t, s2, i); n != 1 {
			t.Fatalf("edit %d is in the file %d times", i, n)
		}
	}
}

// A journal that was set aside before is never overwritten by the next one, however many there are.
func TestSkippedJournalsAreNeverOverwritten(t *testing.T) {
	p := evSave(t)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	jp := filepath.Join(dir, "pending.json")
	names := []string{jp + ".skipped"}
	for i := 2; i <= 99; i++ {
		names = append(names, jp+".skipped."+strconv.Itoa(i))
	}
	for _, n := range names {
		if err := os.WriteFile(n, []byte("old "+filepath.Base(n)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(jp, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.skipJournal("test")
	for _, n := range names {
		if b, err := os.ReadFile(n); err != nil || string(b) != "old "+filepath.Base(n) {
			t.Fatalf("%s was overwritten: %q %v", n, b, err)
		}
	}
	if b, err := os.ReadFile(jp + ".skipped.100"); err != nil || string(b) != "new" {
		t.Fatalf("the new one has the next free name: %q %v", b, err)
	}
}
